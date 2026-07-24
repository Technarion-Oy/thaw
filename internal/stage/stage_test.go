// SPDX-License-Identifier: GPL-3.0-or-later

package stage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
		"streamlit_app.py":         "app",
		"environment.yml":          "deps",
		".DS_Store":                "junk",   // skipped
		".git/config":              "vcs",    // junk dir → skipped
		"pages/page1.py":           "p1",
		"pages/page2.py":           "p2",
		"assets/logo.png":          "img",
		"assets/__pycache__/x.pyc": "cache",  // junk dir → skipped
		"utils/helper.py":          "help",
		"utils/.secret.py":         "hidden", // hidden file → skipped
		"emptyish/.DS_Store":       "junk",   // only junk → no uploads
	})

	ups, err := planDirUploads(root)
	if err != nil {
		t.Fatalf("planDirUploads: %v", err)
	}

	want := []dirUpload{
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

func TestIsJunk(t *testing.T) {
	for _, n := range []string{".git", "__pycache__", ".venv", ".idea"} {
		if !isJunkDir(n) {
			t.Errorf("isJunkDir(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"pages", "assets", "data"} {
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
