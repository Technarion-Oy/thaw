// SPDX-License-Identifier: GPL-3.0-or-later

package e2e_test

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// releaseWorkflowFiles are the CI workflows that build distributable or signed
// binaries. None of them may ever pass the Wails v3 `mcp` app-control tag.
var releaseWorkflowFiles = []string{
	".github/workflows/build.yml",
	".github/workflows/manual-release.yml",
	".github/workflows/macos-sign-test.yml",
}

// platformTaskfiles are the OS build definitions invoked by release workflows.
var platformTaskfiles = []string{
	"build/darwin/Taskfile.yml",
	"build/linux/Taskfile.yml",
	"build/windows/Taskfile.yml",
}

// forbiddenMCPTagPattern matches attempts to enable Wails v3's `mcp` build tag
// or configure its runtime env vars in release workflows / platform Taskfiles.
var forbiddenMCPTagPattern = regexp.MustCompile(
	`(?i)(` +
		`-tags(?:=|\s+)[^\n]*\bmcp\b` +
		`|EXTRA_TAGS\s*[:=][^\n]*\bmcp\b` +
		`|\bWAILS_MCP_[A-Z0-9_]+` +
		`)`,
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", wd)
		}
		dir = parent
	}
}

func stripYAMLComments(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// TestReleaseWorkflowsNeverEnableWailsMCPTag enforces the security invariant
// from issue #1053: release and signing workflows must never compile with
// Wails v3's `mcp` app-control build tag, and every platform Taskfile's
// production build must carry `-tags production` (which pairs with the
// compile-time guard in internal/app/wails_e2e_guard.go).
func TestReleaseWorkflowsNeverEnableWailsMCPTag(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range releaseWorkflowFiles {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		cleaned := stripYAMLComments(string(data))
		if match := forbiddenMCPTagPattern.FindString(cleaned); match != "" {
			t.Errorf("%s must never enable the Wails `mcp` build tag or WAILS_MCP_* env vars (matched %q)", rel, match)
		}
		// Ensure none of the release workflows pass EXTRA_TAGS at all, which could
		// otherwise smuggle `mcp` into platform Taskfiles.
		if strings.Contains(cleaned, "EXTRA_TAGS") {
			t.Errorf("%s references EXTRA_TAGS — release workflows must not inject arbitrary build tags", rel)
		}
	}

	for _, rel := range platformTaskfiles {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		cleaned := stripYAMLComments(string(data))
		if match := forbiddenMCPTagPattern.FindString(cleaned); match != "" {
			t.Errorf("%s must not hardcode the Wails `mcp` build tag (matched %q)", rel, match)
		}
		if !strings.Contains(cleaned, "-tags production") {
			t.Errorf("%s must compile non-dev builds with `-tags production` so internal/app/wails_e2e_guard.go blocks `mcp` at compile time", rel)
		}
	}
}

// TestProductionAndMCPTagsFailToCompile proves that combining the `production`
// and `mcp` build tags fails at compile time via internal/app/wails_e2e_guard.go.
func TestProductionAndMCPTagsFailToCompile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not found on PATH")
	}
	root := repoRoot(t)
	cmd := exec.Command("go", "build", "-tags", "production,mcp", "./internal/app")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected `go build -tags production,mcp ./internal/app` to fail at compile time, but it succeeded")
	}
	if !strings.Contains(string(out), "productionBuildsMustNotEnableWailsAppControlDriver") {
		t.Errorf("expected compile failure from wails_e2e_guard.go, got unexpected error:\n%s", string(out))
	}
}

// TestE2ENoOverlapWithSnowflakeMCP verifies that the Wails v3 E2E harness under
// e2e/ is strictly decoupled from Thaw's own Snowflake MCP server under
// internal/mcp/ (zero shared imports or build tags).
func TestE2ENoOverlapWithSnowflakeMCP(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	// 1. No file in e2e/ may import thaw/internal/mcp.
	e2eEntries, err := os.ReadDir(filepath.Join(root, "e2e"))
	if err != nil {
		t.Fatalf("ReadDir(e2e): %v", err)
	}
	for _, entry := range e2eEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(root, "e2e", entry.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if p == "thaw/internal/mcp" || strings.HasPrefix(p, "thaw/internal/mcp/") {
				t.Errorf("e2e/%s imports Snowflake MCP package %q — keep Wails E2E completely separate from internal/mcp", entry.Name(), p)
			}
		}
	}

	// 2. No file in internal/mcp/ may use Wails's `mcp` or `wails_e2e` build tag
	// or reference WAILS_MCP_* env vars.
	mcpEntries, err := os.ReadDir(filepath.Join(root, "internal", "mcp"))
	if err != nil {
		t.Fatalf("ReadDir(internal/mcp): %v", err)
	}
	buildTagRe := regexp.MustCompile(`(?m)^//go:build\s+.*\b(mcp|wails_e2e)\b`)
	for _, entry := range mcpEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(root, "internal", "mcp", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		text := string(data)
		if buildTagRe.MatchString(text) {
			t.Errorf("internal/mcp/%s carries a Wails E2E/app-control build tag — internal/mcp must stay unrelated to Wails's mcp tag", entry.Name())
		}
		if strings.Contains(text, "WAILS_MCP_") {
			t.Errorf("internal/mcp/%s references WAILS_MCP_* — keep Snowflake MCP separate from Wails app-control driver", entry.Name())
		}
	}
}
