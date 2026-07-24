// SPDX-License-Identifier: GPL-3.0-or-later

package streamlit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFiles materializes a set of relative paths (empty content) under a fresh
// temp directory and returns it. Intermediate directories are created.
func writeFiles(t *testing.T, rels ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range rels {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

func TestDetectStreamlitMainFile(t *testing.T) {
	cases := []struct {
		name           string
		files          []string
		wantMain       string
		wantCandidates []string
	}{
		{
			name:           "prefers streamlit_app.py",
			files:          []string{"streamlit_app.py", "helpers.py", "environment.yml", "pages/page1.py"},
			wantMain:       "streamlit_app.py",
			wantCandidates: []string{"helpers.py", "streamlit_app.py"},
		},
		{
			name:           "falls back to app.py",
			files:          []string{"app.py", "utils.py"},
			wantMain:       "app.py",
			wantCandidates: []string{"app.py", "utils.py"},
		},
		{
			name:           "streamlit_app.py wins over app.py",
			files:          []string{"app.py", "streamlit_app.py"},
			wantMain:       "streamlit_app.py",
			wantCandidates: []string{"app.py", "streamlit_app.py"},
		},
		{
			name:           "ambiguous: no preferred name, multiple candidates",
			files:          []string{"dashboard.py", "main.py", "config.yml"},
			wantMain:       "",
			wantCandidates: []string{"dashboard.py", "main.py"},
		},
		{
			name:           "single non-preferred candidate is not auto-selected",
			files:          []string{"dashboard.py"},
			wantMain:       "",
			wantCandidates: []string{"dashboard.py"},
		},
		{
			name:           "no python files",
			files:          []string{"environment.yml", "README.md"},
			wantMain:       "",
			wantCandidates: []string{},
		},
		{
			name:           "ignores hidden files and subdirectory pages",
			files:          []string{"streamlit_app.py", ".hidden.py", "pages/page1.py", "assets/logo.png"},
			wantMain:       "streamlit_app.py",
			wantCandidates: []string{"streamlit_app.py"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFiles(t, tc.files...)
			got, err := DetectStreamlitMainFile(root)
			if err != nil {
				t.Fatalf("DetectStreamlitMainFile: %v", err)
			}
			if got.MainFile != tc.wantMain {
				t.Errorf("MainFile = %q, want %q", got.MainFile, tc.wantMain)
			}
			if !reflect.DeepEqual(got.Candidates, tc.wantCandidates) {
				t.Errorf("Candidates = %#v, want %#v", got.Candidates, tc.wantCandidates)
			}
		})
	}
}

func TestDetectStreamlitMainFile_Errors(t *testing.T) {
	// A file path (not a directory) is rejected.
	root := writeFiles(t, "streamlit_app.py")
	filePath := filepath.Join(root, "streamlit_app.py")
	if _, err := DetectStreamlitMainFile(filePath); err == nil {
		t.Error("expected error for non-directory path, got nil")
	}

	// A missing path is rejected.
	if _, err := DetectStreamlitMainFile(filepath.Join(root, "does-not-exist")); err == nil {
		t.Error("expected error for missing path, got nil")
	}
}
