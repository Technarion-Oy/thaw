// SPDX-License-Identifier: GPL-3.0-or-later

package snowpark

import (
	"fmt"
	"regexp"
	"strings"
)

// Classification of a failed `pip install`, derived purely from pip's own
// output. The point is to turn "exit status 1" plus a 200-line build trace into
// one sentence the user can act on.
//
// The dominant real-world failure (issue #885) is a dependency file that pins a
// version predating the environment's interpreter: no wheel exists for that
// CPython, pip falls back to the source tarball, and the sdist build dies with
// something unrelated-looking (`No module named 'pkg_resources'`, a missing
// compiler, Cython errors). Users read that as "conflict with what Thaw already
// installed", which it never is.

const (
	// pipFailWheelMismatch — the pinned version exists on the index but ships no
	// file installable on this interpreter.
	pipFailWheelMismatch = "wheel-mismatch"
	// pipFailSourceBuild — pip fell back to an sdist and the build failed.
	pipFailSourceBuild = "source-build"
	// pipFailNotFound — the index has no such project/version at all.
	pipFailNotFound = "not-found"
)

// pipFailure is the classified shape of a pip failure. Kind is "" when the
// output matches no known signature (network errors, permission errors, …), in
// which case callers must surface pip's own message unchanged.
type pipFailure struct {
	Kind     string
	Spec     string   // best-effort requirement spec, e.g. "pandas==2.0.3"
	Packages []string // package names implicated by the failure
}

var (
	// `ERROR: Could not find a version that satisfies the requirement pandas==2.0.3 (from versions: 2.1.1, 2.2.0)`
	// The version list is the discriminator: "none" means the index has nothing
	// under that name at all, a populated list means the project exists but no
	// file matches this interpreter/platform.
	reNoVersionSatisfies = regexp.MustCompile(`Could not find a version that satisfies the requirement (\S+)[^()]*\(from versions: ([^)]*)\)`)
	// `ERROR: No matching distribution found for pandas==2.0.3`
	reNoMatchingDist = regexp.MustCompile(`No matching distribution found for (\S+)`)
	// pip's sdist fallback: `Using cached pandas-2.0.3.tar.gz (5.3 MB)`. Seeing
	// this at all means no wheel matched — pip only downloads a tarball when it
	// has no usable wheel for the running interpreter.
	reSdistDownload = regexp.MustCompile(`(?:Using cached|Downloading|Saved) .*?([A-Za-z0-9][A-Za-z0-9._-]*)-(\d[^-/\\]*)\.(?:tar\.gz|zip)`)
	// Build-failure summaries across pip versions. Each names its packages
	// either quoted, bare-and-space-separated, or parenthesised.
	reFailedToBuildQuoted = regexp.MustCompile(`Failed to build '([^']+)'`)
	reFailedBuildingWheel = regexp.MustCompile(`Failed building wheel for (\S+)`)
	reCouldNotBuildWheels = regexp.MustCompile(`Could not build wheels for ([^,]+(?:, [^,]+)*), which`)
	reFailedWheelsProject = regexp.MustCompile(`Failed to build installable wheels for some pyproject\.toml based projects \(([^)]*)\)`)
	reFailedToBuildBare   = regexp.MustCompile(`^ERROR: Failed to build ([A-Za-z0-9][A-Za-z0-9._\- ]*)$`)
)

// classifyPipOutput inspects the combined stdout+stderr of a failed pip run and
// reports what went wrong. Line order is not relied upon: stdout and stderr are
// captured separately and concatenated, so the "Using cached …tar.gz" progress
// line (stdout) and the "ERROR: Failed to build …" summary (stderr) are matched
// independently rather than as a sequence.
func classifyPipOutput(lines []string) pipFailure {
	// One entry per source distribution pip fell back to, in the order it did.
	type sdist struct{ name, version string }
	var (
		sdists      []sdist
		buildPkgs   []string
		sawSubproc  bool
		resolveSpec string
		resolveHas  bool // the index listed other versions → project exists
	)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		if m := reNoVersionSatisfies.FindStringSubmatch(line); m != nil {
			resolveSpec = m[1]
			resolveHas = strings.TrimSpace(m[2]) != "none" && strings.TrimSpace(m[2]) != ""
			continue
		}
		if m := reNoMatchingDist.FindStringSubmatch(line); m != nil {
			if resolveSpec == "" {
				resolveSpec = m[1]
			}
			continue
		}
		if m := reSdistDownload.FindStringSubmatch(line); m != nil {
			sdists = append(sdists, sdist{name: m[1], version: m[2]})
			continue
		}
		if strings.Contains(line, "subprocess-exited-with-error") {
			sawSubproc = true
			continue
		}
		buildPkgs = append(buildPkgs, matchBuildFailure(line)...)
	}

	// Resolution failures are unambiguous — report them first.
	if resolveSpec != "" {
		kind := pipFailNotFound
		if resolveHas {
			kind = pipFailWheelMismatch
		}
		return pipFailure{Kind: kind, Spec: resolveSpec, Packages: []string{specName(resolveSpec)}}
	}

	// A build failure only counts when pip actually fell back to a source
	// distribution; a subprocess error with no sdist in sight is some other
	// problem (a local project build, a broken hook) and is passed through.
	if len(sdists) == 0 {
		return pipFailure{}
	}
	if len(buildPkgs) == 0 && !sawSubproc {
		return pipFailure{}
	}

	// Prefer a package the failure summary named; otherwise blame the last sdist
	// pip touched, which is the one it was building when it died.
	for _, name := range buildPkgs {
		for _, sd := range sdists {
			if normalizeDistName(sd.name) == normalizeDistName(name) {
				return pipFailure{Kind: pipFailSourceBuild, Spec: sd.name + "==" + sd.version, Packages: []string{sd.name}}
			}
		}
	}
	if len(buildPkgs) > 0 {
		return pipFailure{Kind: pipFailSourceBuild, Spec: buildPkgs[0], Packages: buildPkgs}
	}
	last := sdists[len(sdists)-1]
	return pipFailure{Kind: pipFailSourceBuild, Spec: last.name + "==" + last.version, Packages: []string{last.name}}
}

// matchBuildFailure returns the package names a single pip build-failure
// summary line blames, or nil if the line is not such a summary.
func matchBuildFailure(line string) []string {
	for _, re := range []*regexp.Regexp{
		reFailedToBuildQuoted,
		reFailedBuildingWheel,
		reCouldNotBuildWheels,
		reFailedWheelsProject,
		reFailedToBuildBare,
	} {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// The multi-package forms separate names by ", " or " ".
		fields := strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == ' ' })
		names := make([]string, 0, len(fields))
		for _, f := range fields {
			if f = strings.TrimSpace(f); f != "" {
				names = append(names, f)
			}
		}
		return names
	}
	return nil
}

// specName strips the version operator from a requirement spec ("pandas==2.0.3"
// → "pandas").
func specName(spec string) string {
	if i := strings.IndexAny(spec, "=<>!~["); i > 0 {
		return spec[:i]
	}
	return spec
}

// normalizeDistName applies PEP 503 name normalization so "ruamel.yaml",
// "ruamel-yaml" and "Ruamel_YAML" compare equal.
func normalizeDistName(name string) string {
	return strings.ToLower(strings.NewReplacer(".", "-", "_", "-").Replace(name))
}

// pipFailureAdvice renders a classified failure as one actionable paragraph, or
// "" when the failure matched no known signature (the caller then shows pip's
// own error unchanged). pyVersion is the active environment's Python version
// ("3.14"); it may be empty when detection failed.
func pipFailureAdvice(f pipFailure, pyVersion string) string {
	interp := "the Snowpark environment's Python"
	if pyVersion != "" {
		interp = fmt.Sprintf("Python %s (the Snowpark environment's interpreter)", pyVersion)
	}
	switch f.Kind {
	case pipFailSourceBuild:
		return fmt.Sprintf(
			"%s ships no prebuilt wheel for %s, so pip fell back to building it from source and the build failed. "+
				"This is not a conflict with the packages already installed. "+
				"Fix it by relaxing the pin to a release that has wheels for this interpreter, "+
				"or by recreating the environment on an older Python (Snowpark → Setup Environment).",
			backtick(f.Spec), interp)
	case pipFailWheelMismatch:
		return fmt.Sprintf(
			"pip found no installable distribution of %s for %s — the index lists other versions of the project, "+
				"so the pinned one simply has no file for this interpreter or platform. "+
				"Fix it by relaxing the pin, or by recreating the environment on a Python version the pin supports "+
				"(Snowpark → Setup Environment).",
			backtick(f.Spec), interp)
	case pipFailNotFound:
		return fmt.Sprintf(
			"pip found no project matching %s on the configured package index. "+
				"Check the name and version; if you install from a private registry, confirm it is reachable "+
				"and configured under \"Configure pip Registry…\".",
			backtick(f.Spec))
	default:
		return ""
	}
}

func backtick(s string) string { return "`" + s + "`" }

// describePipFailure is the one call sites use: classify the captured pip
// output and render the advice, or "" when nothing is recognized.
func describePipFailure(lines []string, pyVersion string) string {
	return pipFailureAdvice(classifyPipOutput(lines), pyVersion)
}
