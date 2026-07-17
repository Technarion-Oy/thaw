//go:build ignore

// gen_third_party_notices scans the third-party code that ships inside the Thaw
// binary — the Go modules compiled into the backend and the npm packages bundled
// into the embedded frontend — collects each dependency's license text, and
// regenerates THIRD_PARTY_NOTICES.md at the repository root.
//
// The generated file is embedded into the binary (see main.go) and surfaced in
// the "About Thaw" dialog so users receive the required copyright notices and
// license texts of every package Thaw redistributes.
//
// Data sources:
//
//	Go   — `go list -deps -json ./...` yields exactly the modules whose code is
//	       compiled into the binary (not the full module graph), each with the
//	       on-disk directory of its cached source, from which the LICENSE file is
//	       read.
//	npm  — `npm ls --omit=dev --all --json` (run in ./frontend) yields the
//	       production dependency tree that Vite bundles into frontend/dist; each
//	       package's version and LICENSE file are read from node_modules.
//
// Usage:
//
//	go run scripts/gen_third_party_notices.go   (from the repository root)
//	go generate ./...                            (via the directive in main.go)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// dep is a single third-party package with its resolved license text.
type dep struct {
	Name    string
	Version string
	License string // best-effort SPDX-ish identifier
	Text    string // full license text, or "" when no license file was found
	Dir     string // resolved source directory (for diagnostics)
}

// licensePrefixes matches (case-insensitively, by prefix) the files that
// conventionally hold a package's license text, in priority order. A NOTICE
// file is not a license and is handled separately (appended, per Apache-2.0
// §4(d)), so it is not listed here.
var licensePrefixes = []string{"license", "licence", "copying", "unlicense"}

func main() {
	// -o overrides the output path; the freshness test (main_test.go) uses it to
	// generate into a temp file and diff against the committed copy.
	outFlag := flag.String("o", "", "output path (default: THIRD_PARTY_NOTICES.md at the repo root)")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	goDeps, err := collectGoDeps(root)
	if err != nil {
		fatal(fmt.Errorf("collecting Go modules: %w", err))
	}

	npmDeps, err := collectNpmDeps(root)
	if err != nil {
		fatal(fmt.Errorf("collecting npm packages: %w", err))
	}

	md := render(goDeps, npmDeps)

	out := *outFlag
	if out == "" {
		out = filepath.Join(root, "THIRD_PARTY_NOTICES.md")
	}
	if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
		fatal(fmt.Errorf("writing %s: %w", out, err))
	}

	missing := 0
	for _, d := range append(append([]dep{}, goDeps...), npmDeps...) {
		if d.Text == "" {
			missing++
		}
	}
	fmt.Printf("wrote %s (%d Go modules, %d npm packages, %d without a license file)\n",
		out, len(goDeps), len(npmDeps), missing)
}

// repoRoot walks up from the working directory to the directory containing go.mod.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from working directory upwards")
		}
		dir = parent
	}
}

// ── Go modules ──────────────────────────────────────────────────────────────

// goModule is the subset of `go list -json` we consume.
type goListPackage struct {
	Standard bool
	Module   *struct {
		Path    string
		Version string
		Dir     string
		Main    bool
	}
}

func collectGoDeps(root string) ([]dep, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	seen := map[string]dep{}
	decoder := json.NewDecoder(strings.NewReader(string(stdout)))
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, err
		}
		m := pkg.Module
		if m == nil || m.Main || m.Dir == "" {
			continue // stdlib, the main module, or a module without local source
		}
		if _, ok := seen[m.Path]; ok {
			continue
		}
		text := readLicenseFile(m.Dir)
		seen[m.Path] = dep{
			Name:    m.Path,
			Version: m.Version,
			License: detectLicense(text),
			Text:    text,
			Dir:     m.Dir,
		}
	}

	return sortedDeps(seen), nil
}

// ── npm packages ────────────────────────────────────────────────────────────

// npmNode mirrors the recursive shape of `npm ls --json`.
type npmNode struct {
	Version      string             `json:"version"`
	Dependencies map[string]npmNode `json:"dependencies"`
}

type npmTree struct {
	Dependencies map[string]npmNode `json:"dependencies"`
}

func collectNpmDeps(root string) ([]dep, error) {
	frontend := filepath.Join(root, "frontend")
	cmd := exec.Command("npm", "ls", "--omit=dev", "--all", "--json")
	cmd.Dir = frontend
	// `npm ls` exits non-zero on peer-dependency warnings even when it prints a
	// complete tree, so we ignore the exit status and parse stdout regardless.
	stdout, _ := cmd.Output()
	if len(stdout) == 0 {
		return nil, fmt.Errorf("npm ls produced no output (is frontend/node_modules installed?)")
	}

	var tree npmTree
	if err := json.Unmarshal(stdout, &tree); err != nil {
		return nil, err
	}
	// `npm ls`'s non-zero exit on peer-dependency warnings is expected and
	// ignored above, but a genuine failure (e.g. a corrupted lockfile) can still
	// emit partial JSON on stdout. An empty dependency tree means we got no usable
	// data, so fail loudly rather than silently shipping an incomplete notices
	// file. On success this project always has a populated tree.
	if len(tree.Dependencies) == 0 {
		return nil, fmt.Errorf("npm ls returned an empty dependency tree (corrupted lockfile or failed install?)")
	}

	nodeModules := filepath.Join(frontend, "node_modules")
	// Key by name@version, not name: a package can be bundled at several versions
	// simultaneously (e.g. zustand@5 directly and zustand@4 nested under
	// @xyflow/react), and all of them ship. Keying by name alone would drop all
	// but one, non-deterministically, since the tree is walked over a Go map.
	seen := map[string]dep{}
	var walk func(name string, node npmNode)
	walk = func(name string, node npmNode) {
		// `npm ls` also lists optional peer dependencies that are declared but not
		// installed (e.g. rc-picker names date-fns/luxon/moment as alternative date
		// backends). Those carry no version and ship nothing, so skip them.
		if node.Version == "" {
			return
		}
		key := name + "@" + node.Version
		if _, ok := seen[key]; !ok {
			dir := resolveNpmDir(nodeModules, name, node.Version)
			text := readLicenseFile(dir)
			license := detectLicense(text)
			if pkgLicense := readPackageJSONLicense(dir); pkgLicense != "" {
				license = pkgLicense
			}
			seen[key] = dep{
				Name:    name,
				Version: node.Version,
				License: license,
				Text:    text,
				Dir:     dir,
			}
		}
		for child, childNode := range node.Dependencies {
			walk(child, childNode)
		}
	}
	for name, node := range tree.Dependencies {
		walk(name, node)
	}

	return sortedDeps(seen), nil
}

// resolveNpmDir finds the installed directory for a specific package version.
// npm hoists most packages to the top-level node_modules but keeps conflicting
// versions nested under their dependents, so when the hoisted copy is a
// different version we search nested node_modules for the exact version,
// falling back to any install of the package.
func resolveNpmDir(nodeModules, name, version string) string {
	top := filepath.Join(nodeModules, filepath.FromSlash(name))
	topVersion := readPackageJSONVersion(top)
	if topVersion == version {
		return top
	}

	// The hoisted copy (if any) is the fallback; a nested copy may match exactly.
	fallback := ""
	if topVersion != "" {
		fallback = top
	}
	suffix := filepath.FromSlash("node_modules/" + name)
	found := ""
	_ = filepath.WalkDir(nodeModules, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries
		}
		if !strings.HasSuffix(path, suffix) {
			return nil
		}
		v := readPackageJSONVersion(path)
		if v == version {
			found = path
			return filepath.SkipAll
		}
		if fallback == "" && v != "" {
			fallback = path
		}
		return nil
	})
	if found != "" {
		return found
	}
	return fallback
}

// readPackageJSONVersion returns the `version` field of dir/package.json, or ""
// if the file is absent or unreadable.
func readPackageJSONVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Version
}

// readPackageJSONLicense extracts the SPDX license identifier from a package's
// package.json, handling both the modern `license` string and the legacy
// `licenses` array.
func readPackageJSONLicense(dir string) string {
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		License  json.RawMessage `json:"license"`
		Licenses []struct {
			Type string `json:"type"`
		} `json:"licenses"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	if len(pkg.License) > 0 {
		var s string
		if json.Unmarshal(pkg.License, &s) == nil && s != "" {
			return s
		}
		var obj struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(pkg.License, &obj) == nil && obj.Type != "" {
			return obj.Type
		}
	}
	if len(pkg.Licenses) > 0 {
		types := make([]string, 0, len(pkg.Licenses))
		for _, l := range pkg.Licenses {
			if l.Type != "" {
				types = append(types, l.Type)
			}
		}
		if len(types) > 0 {
			return strings.Join(types, ", ")
		}
	}
	return ""
}

// ── shared helpers ──────────────────────────────────────────────────────────

// readLicenseFile returns the license text for a package: the canonical LICENSE
// file, with any accompanying NOTICE file appended (Apache-2.0 §4(d) requires
// redistributing the NOTICE contents). Returns "" when no license file exists.
func readLicenseFile(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Rank license files by prefix priority, then shortest name (e.g. "LICENSE"
	// over "LICENSE-MIT"), so the canonical text wins.
	bestName, bestRank := "", len(licensePrefixes)
	var noticeName string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasPrefix(lower, "notice") {
			if noticeName == "" || len(e.Name()) < len(noticeName) {
				noticeName = e.Name()
			}
			continue
		}
		for rank, prefix := range licensePrefixes {
			if !strings.HasPrefix(lower, prefix) {
				continue
			}
			if rank < bestRank || (rank == bestRank && (bestName == "" || len(e.Name()) < len(bestName))) {
				bestName, bestRank = e.Name(), rank
			}
			break
		}
	}

	text := readTextFile(filepath.Join(dir, bestName))
	if bestName == "" {
		text = ""
	}
	if notice := readTextFile(filepath.Join(dir, noticeName)); noticeName != "" && notice != "" {
		if text != "" {
			text += "\n\n---\nNOTICE:\n\n"
		}
		text += notice
	}
	return text
}

// readTextFile reads a file and normalizes CRLF line endings. A read error
// (including a missing file or a directory path) yields "".
func readTextFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// detectLicense makes a best-effort guess at the SPDX identifier from license
// text, used only when package metadata does not declare one (always the case
// for Go modules).
func detectLicense(text string) string {
	if text == "" {
		return "Unknown"
	}
	head := strings.ToLower(text)
	if len(head) > 4000 {
		head = head[:4000]
	}
	switch {
	case strings.Contains(head, "apache license") && strings.Contains(head, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(head, "gnu general public license") && strings.Contains(head, "version 3"):
		return "GPL-3.0"
	case strings.Contains(head, "mozilla public license") && strings.Contains(head, "2.0"):
		return "MPL-2.0"
	case strings.Contains(head, "redistribution and use") && strings.Contains(head, "neither the name"):
		return "BSD-3-Clause"
	case strings.Contains(head, "redistribution and use"):
		return "BSD-2-Clause"
	case strings.Contains(head, "permission is hereby granted, free of charge") && strings.Contains(head, "the software is provided"):
		return "MIT"
	case strings.Contains(head, "permission to use, copy, modify"):
		return "ISC"
	case strings.Contains(head, "this is free and unencumbered software released into the public domain"):
		return "Unlicense"
	default:
		return "See license text"
	}
}

func sortedDeps(m map[string]dep) []dep {
	out := make([]dep, 0, len(m))
	for _, d := range m {
		out = append(out, d)
	}
	// Sort by name, then version, so a package bundled at multiple versions has a
	// stable, deterministic order (the freshness test diffs on exact bytes).
	sort.Slice(out, func(i, j int) bool {
		ni, nj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		return out[i].Version < out[j].Version
	})
	return out
}

func render(goDeps, npmDeps []dep) string {
	var b strings.Builder
	b.WriteString("# Third-Party Notices & Acknowledgements\n\n")
	b.WriteString("Thaw is built on the work of many open-source projects. This file lists every\n")
	b.WriteString("third-party package that ships inside the Thaw binary — the Go modules compiled\n")
	b.WriteString("into the backend and the npm packages bundled into the embedded frontend — along\n")
	b.WriteString("with each project's copyright notice and license text.\n\n")
	b.WriteString("> **This file is generated.** Do not edit it by hand. Regenerate it with\n")
	b.WriteString("> `go run scripts/gen_third_party_notices.go` (or `go generate ./...`) after\n")
	b.WriteString("> changing dependencies in `go.mod` or `frontend/package.json`.\n\n")

	fmt.Fprintf(&b, "Thaw itself is free software licensed under the GNU General Public License v3.0\n")
	fmt.Fprintf(&b, "or later. The licenses below apply only to the corresponding third-party packages.\n\n")

	b.WriteString("## Contents\n\n")
	fmt.Fprintf(&b, "- [Backend — Go modules](#backend--go-modules) (%d)\n", len(goDeps))
	fmt.Fprintf(&b, "- [Frontend — npm packages](#frontend--npm-packages) (%d)\n\n", len(npmDeps))

	renderSection(&b, "Backend — Go modules", goDeps)
	renderSection(&b, "Frontend — npm packages", npmDeps)

	return b.String()
}

func renderSection(b *strings.Builder, title string, deps []dep) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(deps) == 0 {
		b.WriteString("_None._\n\n")
		return
	}
	// Summary table.
	b.WriteString("| Package | Version | License |\n")
	b.WriteString("|---------|---------|---------|\n")
	for _, d := range deps {
		fmt.Fprintf(b, "| `%s` | %s | %s |\n", d.Name, valueOrDash(d.Version), valueOrDash(d.License))
	}
	b.WriteString("\n")

	// Full texts.
	for _, d := range deps {
		fmt.Fprintf(b, "### %s\n\n", d.Name)
		fmt.Fprintf(b, "- **Version:** %s\n", valueOrDash(d.Version))
		fmt.Fprintf(b, "- **License:** %s\n\n", valueOrDash(d.License))
		if d.Text == "" {
			b.WriteString("_No license file was found in the distributed package. Refer to the project's " +
				"repository for its license terms._\n\n")
			continue
		}
		b.WriteString("```\n")
		b.WriteString(strings.TrimRight(d.Text, "\n"))
		b.WriteString("\n```\n\n")
	}
}

func valueOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen_third_party_notices:", err)
	os.Exit(1)
}
