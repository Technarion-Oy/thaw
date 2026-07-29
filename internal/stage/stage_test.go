// SPDX-License-Identifier: GPL-3.0-or-later

package stage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"thaw/internal/filesystem"
)

// writeTree materializes a map of relative path → content under a fresh temp
// directory and returns it. Intermediate directories are created; empty content
// still creates the file.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// relPlan re-expresses each planned upload's absolute Path relative to root, for
// stable, platform-independent assertions.
func relPlan(t *testing.T, root string, ups []dirUpload) []dirUpload {
	t.Helper()
	out := make([]dirUpload, len(ups))
	for i, u := range ups {
		rel, err := filepath.Rel(root, u.Path)
		if err != nil {
			t.Fatalf("rel %s: %v", u.Path, err)
		}
		out[i] = dirUpload{Path: filepath.ToSlash(rel), RelDir: u.RelDir}
	}
	return out
}

func TestPlanDirUploads_PreservesTreeAndSkipsJunk(t *testing.T) {
	root := writeTree(t, map[string]string{
		"streamlit_app.py":          "app",
		"environment.yml":           "deps",
		".DS_Store":                 "junk", // skipped
		".git/config":               "vcs",  // junk dir → skipped
		"pages/page1.py":            "p1",
		"pages/page2.py":            "p2",
		"assets/logo.png":           "img",
		"assets/__pycache__/x.pyc":  "cache", // junk dir → skipped
		"utils/helper.py":           "help",
		"utils/.secret.py":          "hidden", // hidden file → skipped
		"emptyish/.DS_Store":        "junk",   // only junk → no uploads
		".streamlit/config.toml":    "theme",  // config dir → KEPT
		"venv/bin/activate":         "venv",   // virtualenv → skipped
		"node_modules/pkg/index.js": "dep",    // JS deps → skipped
	})

	ups, err := planDirUploads(root)
	if err != nil {
		t.Fatalf("planDirUploads: %v", err)
	}

	want := []dirUpload{
		{Path: ".streamlit/config.toml", RelDir: ".streamlit"},
		{Path: "assets/logo.png", RelDir: "assets"},
		{Path: "environment.yml", RelDir: ""},
		{Path: "pages/page1.py", RelDir: "pages"},
		{Path: "pages/page2.py", RelDir: "pages"},
		{Path: "streamlit_app.py", RelDir: ""},
		{Path: "utils/helper.py", RelDir: "utils"},
	}
	if got := relPlan(t, root, ups); !reflect.DeepEqual(got, want) {
		t.Errorf("plan mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPlanDirUploads_AllJunkIsEmpty(t *testing.T) {
	root := writeTree(t, map[string]string{
		".DS_Store":         "junk",
		".git/config":       "vcs",
		"__pycache__/x.pyc": "cache",
	})
	ups, err := planDirUploads(root)
	if err != nil {
		t.Fatalf("planDirUploads: %v", err)
	}
	if len(ups) != 0 {
		t.Errorf("expected empty plan, got %#v", relPlan(t, root, ups))
	}
}

// TestPlanDirUploads_SymlinkEscapesAreSkipped is the regression test for the
// exfiltration hole: PUT opens the local path through a symlink, so a link
// planted under the app folder — by an AI client driving deploy_streamlit, or
// just present in the user's project — would otherwise copy a file from outside
// the sandbox into the stage, where it is readable back out of the deployed app.
func TestPlanDirUploads_SymlinkEscapesAreSkipped(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "id_rsa")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	root := writeTree(t, map[string]string{
		"streamlit_app.py":  "app",
		"pages/page1.py":    "p1",
		"assets/shared.css": "css",
	})

	// A link to a file outside the app folder, at the root and nested.
	mustSymlink(t, secret, filepath.Join(root, "leak.txt"))
	mustSymlink(t, secret, filepath.Join(root, "pages", "leak.py"))
	// A link to the enclosing directory outside the app folder.
	mustSymlink(t, outside, filepath.Join(root, "elsewhere"))
	// A link to a directory inside the app folder: not uploadable as a file, and
	// its contents are already walked at their real location.
	mustSymlink(t, filepath.Join(root, "assets"), filepath.Join(root, "assets_link"))
	// A dangling link.
	mustSymlink(t, filepath.Join(outside, "does-not-exist"), filepath.Join(root, "broken.txt"))
	// A link to a regular file inside the app folder: legitimate, kept in place.
	mustSymlink(t, filepath.Join(root, "assets", "shared.css"), filepath.Join(root, "pages", "shared.css"))

	ups, err := planDirUploads(root)
	if err != nil {
		t.Fatalf("planDirUploads: %v", err)
	}

	want := []dirUpload{
		{Path: "assets/shared.css", RelDir: "assets"},
		{Path: "pages/page1.py", RelDir: "pages"},
		{Path: "pages/shared.css", RelDir: "pages"},
		{Path: "streamlit_app.py", RelDir: ""},
	}
	got := relPlan(t, root, ups)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	// Belt and braces: no planned upload may resolve outside the app folder.
	for _, u := range ups {
		resolved, err := filepath.EvalSymlinks(u.Path)
		if err != nil {
			t.Fatalf("resolve %s: %v", u.Path, err)
		}
		if err := filesystem.ValidateInsideOrEqual(resolved, root); err != nil {
			t.Errorf("planned upload escapes the app folder: %s → %s", u.Path, resolved)
		}
	}
}

// mustSymlink creates target←link, skipping the test on platforms/accounts that
// can't (unprivileged Windows).
func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func TestIsJunk(t *testing.T) {
	for _, n := range []string{".git", "__pycache__", ".venv", ".idea", "venv", "env", "node_modules"} {
		if !isJunkDir(n) {
			t.Errorf("isJunkDir(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"pages", "assets", "data", ".streamlit"} {
		if isJunkDir(n) {
			t.Errorf("isJunkDir(%q) = true, want false", n)
		}
	}
	for _, n := range []string{".DS_Store", ".env", ".gitignore"} {
		if !isJunkFile(n) {
			t.Errorf("isJunkFile(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"streamlit_app.py", "environment.yml", "README.md"} {
		if isJunkFile(n) {
			t.Errorf("isJunkFile(%q) = true, want false", n)
		}
	}
}
