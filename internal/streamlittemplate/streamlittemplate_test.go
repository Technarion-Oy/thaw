// SPDX-License-Identifier: GPL-3.0-or-later

package streamlittemplate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// withServer points the package's GitHub API + raw base URLs at a test server
// for the duration of the test, restoring them afterwards.
func withServer(t *testing.T, h http.Handler) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	origAPI, origRaw := githubAPIBase, rawBase
	githubAPIBase, rawBase = srv.URL, srv.URL
	t.Cleanup(func() { githubAPIBase, rawBase = origAPI, origRaw })
}

func TestFirstParagraph(t *testing.T) {
	cases := []struct {
		name, md, want string
	}{
		{
			name: "skips heading and badges, strips a link",
			md:   "# Inventory Tracker\n\n![badge](x.svg)\n\nTrack inventory with a [Streamlit](https://streamlit.io) app.\n\nMore text.",
			want: "Track inventory with a Streamlit app.",
		},
		{
			name: "collapses whitespace across wrapped lines",
			md:   "A dashboard\nfor  business   intelligence.\n\nSecond para.",
			want: "A dashboard for business intelligence.",
		},
		{
			name: "strips emphasis and code markers",
			md:   "Use **Cortex** and `SQL` together.",
			want: "Use Cortex and SQL together.",
		},
		{
			name: "empty readme yields empty",
			md:   "# Only a title\n",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstParagraph(tc.md); got != tc.want {
				t.Errorf("firstParagraph() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFirstParagraphTruncates(t *testing.T) {
	long := strings.Repeat("word ", 100)
	got := firstParagraph(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected truncation ellipsis, got %q", got)
	}
	if len([]rune(got)) > 201 {
		t.Errorf("truncated length = %d, want <= 201", len([]rune(got)))
	}
}

func TestListTemplates(t *testing.T) {
	readmes := map[string]string{
		"Inventory Tracker/README.md":               "# Inventory Tracker\n\nTrack inventory levels.",
		"Business Intelligence Dashboard/README.md": "# BI\n\nAnalyze the business.",
		// "Chat app" intentionally has no README → blank description.
	}
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/contents"):
			_ = json.NewEncoder(w).Encode([]contentsEntry{
				{Name: "Inventory Tracker", Type: "dir"},
				{Name: "Business Intelligence Dashboard", Type: "dir"},
				{Name: "Chat app using Snowflake Cortex", Type: "dir"},
				{Name: "shared_assets", Type: "dir"},   // excluded
				{Name: ".github", Type: "dir"},          // hidden, excluded
				{Name: "README.md", Type: "file"},       // not a dir, excluded
			})
		default: // raw README fetch
			trimmed := strings.TrimPrefix(r.URL.Path, "/"+repoOwner+"/"+repoName+"/"+repoRef+"/")
			// r.URL.Path is already percent-decoded by net/http.
			if body, ok := readmes[trimmed]; ok {
				_, _ = w.Write([]byte(body))
				return
			}
			http.NotFound(w, r)
		}
	}))

	cat := ListTemplates(context.Background())
	if cat.Degraded {
		t.Fatalf("unexpected degraded catalog: %s", cat.Note)
	}

	got := map[string]string{}
	var names []string
	for _, tmpl := range cat.Templates {
		got[tmpl.Name] = tmpl.Description
		names = append(names, tmpl.Name)
	}

	wantNames := []string{"Business Intelligence Dashboard", "Chat app using Snowflake Cortex", "Inventory Tracker"}
	if !sort.StringsAreSorted(names) {
		t.Errorf("templates not sorted: %v", names)
	}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names = %v, want %v (shared_assets/.github/file excluded)", names, wantNames)
	}
	if got["Inventory Tracker"] != "Track inventory levels." {
		t.Errorf("Inventory Tracker description = %q", got["Inventory Tracker"])
	}
	if got["Chat app using Snowflake Cortex"] != "" {
		t.Errorf("expected blank description for README-less template, got %q", got["Chat app using Snowflake Cortex"])
	}
}

func TestListTemplatesDegraded(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	cat := ListTemplates(context.Background())
	if !cat.Degraded {
		t.Fatal("expected degraded catalog on listing failure")
	}
	if len(cat.Templates) != len(embeddedTemplateNames) {
		t.Errorf("fallback templates = %d, want %d", len(cat.Templates), len(embeddedTemplateNames))
	}
	if cat.Note == "" {
		t.Error("expected a non-empty degraded note")
	}
}

func TestDownloadTemplate(t *testing.T) {
	files := map[string]string{
		"Inventory Tracker/streamlit_app.py":  "import streamlit as st\n",
		"Inventory Tracker/environment.yml":   "name: env\n",
		"Inventory Tracker/pages/page_1.py":   "# page 1\n",
		"Inventory Tracker/README.md":         "# Inventory Tracker\n",
		"Other App/streamlit_app.py":          "# other\n",
		"LICENSE":                             "Apache License 2.0 ...\n",
	}
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			var entries []treeEntry
			for p := range files {
				entries = append(entries, treeEntry{Path: p, Type: "blob"})
			}
			entries = append(entries, treeEntry{Path: "Inventory Tracker/pages", Type: "tree"})
			_ = json.NewEncoder(w).Encode(treeResponse{Tree: entries})
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/"+repoOwner+"/"+repoName+"/"+repoRef+"/")
		if body, ok := files[trimmed]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))

	dest := filepath.Join(t.TempDir(), "app")
	if err := DownloadTemplate(context.Background(), "Inventory Tracker", dest); err != nil {
		t.Fatalf("DownloadTemplate: %v", err)
	}

	// Only the chosen folder's files land, relative to the folder root.
	want := []string{"README.md", "environment.yml", "pages/page_1.py", "streamlit_app.py"}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected file %s: %v", rel, err)
		}
	}
	// The other app's files must NOT be here.
	if _, err := os.Stat(filepath.Join(dest, "Other App")); !os.IsNotExist(err) {
		t.Error("unrelated template files were downloaded")
	}
	// License carry-along + provenance.
	if _, err := os.Stat(filepath.Join(dest, "LICENSE")); err != nil {
		t.Errorf("expected LICENSE: %v", err)
	}
	notice, err := os.ReadFile(filepath.Join(dest, "NOTICE"))
	if err != nil {
		t.Fatalf("expected NOTICE: %v", err)
	}
	if !strings.Contains(string(notice), "Inventory Tracker") || !strings.Contains(string(notice), repoName) {
		t.Errorf("NOTICE missing provenance: %q", notice)
	}
}

func TestDownloadTemplateRefusesNonEmptyDest(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := DownloadTemplate(context.Background(), "Inventory Tracker", dest)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected non-empty-destination error, got %v", err)
	}
}

func TestValidTemplateName(t *testing.T) {
	valid := []string{"Inventory Tracker", "Business Intelligence Dashboard", "app"}
	for _, n := range valid {
		if !validTemplateName(n) {
			t.Errorf("validTemplateName(%q) = false, want true", n)
		}
	}
	invalid := []string{"", ".", "..", "shared_assets", ".github", "a/b", `a\b`, "../etc"}
	for _, n := range invalid {
		if validTemplateName(n) {
			t.Errorf("validTemplateName(%q) = true, want false", n)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	if _, err := safeJoin(base, "pages/page.py"); err != nil {
		t.Errorf("safeJoin rejected a valid path: %v", err)
	}
	for _, rel := range []string{"../escape.py", "../../etc/passwd", "pages/../../out.py"} {
		if _, err := safeJoin(base, rel); err == nil {
			t.Errorf("safeJoin(%q) accepted an escaping path", rel)
		}
	}
}
