// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeTree materializes a map of relative path → file content under a fresh
// temp directory and returns the directory. Intermediate directories are
// created automatically. An entry whose content is the empty string still
// creates the file.
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

// relPut re-expresses a stagePut's absolute Source relative to root so tests can
// assert on stable, platform-independent paths.
func relPut(t *testing.T, root string, p stagePut) stagePut {
	t.Helper()
	rel, err := filepath.Rel(root, p.Source)
	if err != nil {
		t.Fatalf("rel %s: %v", p.Source, err)
	}
	return stagePut{Source: filepath.ToSlash(rel), RelDir: p.RelDir, Glob: p.Glob}
}

func relPlan(t *testing.T, root string, plan []stagePut) []stagePut {
	t.Helper()
	out := make([]stagePut, len(plan))
	for i, p := range plan {
		out[i] = relPut(t, root, p)
	}
	return out
}

func TestPlanStageUploads_GroupsAndSkipsJunk(t *testing.T) {
	root := writeTree(t, map[string]string{
		"streamlit_app.py":         "app",
		"environment.yml":          "deps",
		".DS_Store":                "junk",   // junk file at root → root can't glob
		".git/config":              "vcs",    // junk dir → skipped, blocks root glob
		"pages/page1.py":           "p1",     // clean leaf dir → single glob
		"pages/page2.py":           "p2",     // clean leaf dir → single glob
		"assets/logo.png":          "img",    // dir with a junk subdir → per-file
		"assets/__pycache__/x.pyc": "cache",  // junk dir → skipped, blocks assets glob
		"utils/helper.py":          "help",   // dir with a hidden file → per-file
		"utils/.secret.py":         "hidden", // hidden file → skipped, blocks utils glob
		"emptyish/.DS_Store":       "junk",   // only junk → directory omitted entirely
	})

	plan, err := planStageUploads(root)
	if err != nil {
		t.Fatalf("planStageUploads: %v", err)
	}

	got := relPlan(t, root, plan)
	want := []stagePut{
		{Source: "environment.yml", RelDir: "", Glob: false},
		{Source: "streamlit_app.py", RelDir: "", Glob: false},
		{Source: "assets/logo.png", RelDir: "assets", Glob: false},
		{Source: "pages", RelDir: "pages", Glob: true},
		{Source: "utils/helper.py", RelDir: "utils", Glob: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPlanStageUploads_CleanFlatDirIsSingleGlob(t *testing.T) {
	root := writeTree(t, map[string]string{
		"streamlit_app.py": "app",
		"environment.yml":  "deps",
		"README.md":        "docs",
	})

	plan, err := planStageUploads(root)
	if err != nil {
		t.Fatalf("planStageUploads: %v", err)
	}

	got := relPlan(t, root, plan)
	want := []stagePut{{Source: ".", RelDir: "", Glob: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPlanStageUploads_NestedCleanDirsEachGlob(t *testing.T) {
	// Root has subdirs (so root files go per-file), but each leaf dir is clean.
	root := writeTree(t, map[string]string{
		"streamlit_app.py": "app",
		"pages/a.py":       "a",
		"pages/b.py":       "b",
		"assets/logo.png":  "img",
	})

	plan, err := planStageUploads(root)
	if err != nil {
		t.Fatalf("planStageUploads: %v", err)
	}

	got := relPlan(t, root, plan)
	want := []stagePut{
		{Source: "streamlit_app.py", RelDir: "", Glob: false},
		{Source: "assets", RelDir: "assets", Glob: true},
		{Source: "pages", RelDir: "pages", Glob: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPlanStageUploads_EmptyOrAllJunk(t *testing.T) {
	// A tree containing only junk yields no PUTs (uploadDirToStage turns this
	// into a clear error).
	root := writeTree(t, map[string]string{
		".DS_Store":         "junk",
		".git/config":       "vcs",
		"__pycache__/x.pyc": "cache",
	})

	plan, err := planStageUploads(root)
	if err != nil {
		t.Fatalf("planStageUploads: %v", err)
	}
	if len(plan) != 0 {
		t.Errorf("expected empty plan, got %#v", relPlan(t, root, plan))
	}
}

func TestIsJunk(t *testing.T) {
	junkDirs := []string{".git", "__pycache__", ".venv", ".idea"}
	for _, n := range junkDirs {
		if !isJunkDir(n) {
			t.Errorf("isJunkDir(%q) = false, want true", n)
		}
	}
	keepDirs := []string{"pages", "assets", "data", "components"}
	for _, n := range keepDirs {
		if isJunkDir(n) {
			t.Errorf("isJunkDir(%q) = true, want false", n)
		}
	}

	junkFiles := []string{".DS_Store", ".env", ".gitignore"}
	for _, n := range junkFiles {
		if !isJunkFile(n) {
			t.Errorf("isJunkFile(%q) = false, want true", n)
		}
	}
	keepFiles := []string{"streamlit_app.py", "environment.yml", "README.md"}
	for _, n := range keepFiles {
		if isJunkFile(n) {
			t.Errorf("isJunkFile(%q) = true, want false", n)
		}
	}
}
