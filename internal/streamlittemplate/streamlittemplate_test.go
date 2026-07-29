// SPDX-License-Identifier: GPL-3.0-or-later

package streamlittemplate

import (
	"context"
	"encoding/json"
	"fmt"
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

// rateLimitHandler responds like GitHub when the unauthenticated rate limit is
// exhausted: 403 with X-RateLimit-Remaining: 0.
func rateLimitHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	})
}

func TestListTemplatesRateLimited(t *testing.T) {
	withServer(t, rateLimitHandler())
	cat := ListTemplates(context.Background())
	if !cat.Degraded {
		t.Fatal("expected degraded catalog on rate limit")
	}
	if !strings.Contains(cat.Note, "rate limit") {
		t.Errorf("degraded note should mention the rate limit, got %q", cat.Note)
	}
	if len(cat.Templates) != len(embeddedTemplateNames) {
		t.Errorf("fallback templates = %d, want %d", len(cat.Templates), len(embeddedTemplateNames))
	}
}

func TestDownloadTemplateRateLimited(t *testing.T) {
	withServer(t, rateLimitHandler())
	err := DownloadTemplate(context.Background(), "Inventory Tracker", filepath.Join(t.TempDir(), "app"))
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected a clear rate-limit error, got %v", err)
	}
}

func TestDownloadTemplateTruncatedTree(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(treeResponse{
				Tree:      []treeEntry{{Path: "Inventory Tracker/streamlit_app.py", Type: "blob"}},
				Truncated: true,
			})
			return
		}
		http.NotFound(w, r)
	}))
	err := DownloadTemplate(context.Background(), "Inventory Tracker", filepath.Join(t.TempDir(), "app"))
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected a truncated-tree error, got %v", err)
	}
}

func TestDownloadTemplateNotFound(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(treeResponse{
				Tree: []treeEntry{{Path: "Some Other App/streamlit_app.py", Type: "blob"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	err := DownloadTemplate(context.Background(), "Missing App", filepath.Join(t.TempDir(), "app"))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected a not-found error, got %v", err)
	}
}

// TestDownloadTemplateRejectsHostileTreeEntry drives a traversal attempt through
// the whole HTTP-mocked flow, not just safeJoin: a tree entry that climbs out of
// the template folder must fail the download and write nothing anywhere.
func TestDownloadTemplateRejectsHostileTreeEntry(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(treeResponse{Tree: []treeEntry{
				{Path: "Inventory Tracker/streamlit_app.py", Type: "blob"},
				{Path: "Inventory Tracker/../../evil.py", Type: "blob"},
			}})
			return
		}
		_, _ = w.Write([]byte("payload"))
	}))

	parent := t.TempDir()
	dest := filepath.Join(parent, "sub", "app")
	err := DownloadTemplate(context.Background(), "Inventory Tracker", dest)
	if err == nil || !strings.Contains(err.Error(), "unsafe template path") {
		t.Fatalf("expected an unsafe-path error, got %v", err)
	}
	if _, serr := os.Stat(filepath.Join(parent, "evil.py")); !os.IsNotExist(serr) {
		t.Error("hostile tree entry escaped the destination")
	}
	if _, serr := os.Stat(dest); !os.IsNotExist(serr) {
		t.Error("failed download left the destination behind")
	}
}

// TestDownloadTemplateRollsBackPartialDownload covers the retry trap: a file that
// fails mid-download used to leave the destination non-empty, which the
// empty-destination rule then rejected forever.
func TestDownloadTemplateRollsBackPartialDownload(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(treeResponse{Tree: []treeEntry{
				{Path: "Inventory Tracker/streamlit_app.py", Type: "blob"},
				{Path: "Inventory Tracker/pages/page_1.py", Type: "blob"},
			}})
			return
		}
		if strings.HasSuffix(r.URL.Path, "page_1.py") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("import streamlit as st\n"))
	}))

	// Destination the user picked, already existing and empty: it survives, its
	// contents do not.
	existing := t.TempDir()
	if err := DownloadTemplate(context.Background(), "Inventory Tracker", existing); err == nil {
		t.Fatal("expected a download error")
	}
	entries, err := os.ReadDir(existing)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("partial scaffold left behind: %d entries", len(entries))
	}

	// Destination created by the download: removed entirely.
	fresh := filepath.Join(t.TempDir(), "app")
	if err := DownloadTemplate(context.Background(), "Inventory Tracker", fresh); err == nil {
		t.Fatal("expected a download error")
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Error("a destination created by the failed download was not removed")
	}
}

// TestDownloadTemplatePreservesTemplateNotice checks the Apache-2.0 §4(d) path: a
// NOTICE shipped by the template is kept and the provenance appended, matching how
// a template-provided LICENSE is left alone.
func TestDownloadTemplatePreservesTemplateNotice(t *testing.T) {
	files := map[string]string{
		"Inventory Tracker/streamlit_app.py": "app\n",
		"Inventory Tracker/NOTICE":           "Upstream notice: contains third-party code.",
		"LICENSE":                            "Apache License 2.0 ...\n",
	}
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			var entries []treeEntry
			for p := range files {
				entries = append(entries, treeEntry{Path: p, Type: "blob"})
			}
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
	notice, err := os.ReadFile(filepath.Join(dest, "NOTICE"))
	if err != nil {
		t.Fatalf("read NOTICE: %v", err)
	}
	if !strings.Contains(string(notice), "Upstream notice") {
		t.Errorf("template NOTICE was overwritten: %q", notice)
	}
	if !strings.Contains(string(notice), RepoURL) {
		t.Errorf("provenance missing from NOTICE: %q", notice)
	}
}

// TestListTemplatesPaginates walks more than one page of the contents endpoint.
func TestListTemplatesPaginates(t *testing.T) {
	// Page 1 is full (contentsPageSize entries), so a second page is requested.
	page1 := make([]contentsEntry, contentsPageSize)
	for i := range page1 {
		page1[i] = contentsEntry{Name: fmt.Sprintf("App %03d", i), Type: "dir"}
	}
	page2 := []contentsEntry{{Name: "Zebra App", Type: "dir"}}

	var pagesServed []string
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/contents") {
			http.NotFound(w, r) // READMEs: no descriptions needed here
			return
		}
		page := r.URL.Query().Get("page")
		pagesServed = append(pagesServed, page)
		if page == "1" {
			_ = json.NewEncoder(w).Encode(page1)
			return
		}
		_ = json.NewEncoder(w).Encode(page2)
	}))

	cat := ListTemplates(context.Background())
	if cat.Degraded {
		t.Fatalf("unexpected degraded catalog: %s", cat.Note)
	}
	if len(cat.Templates) != contentsPageSize+1 {
		t.Errorf("templates = %d, want %d (both pages)", len(cat.Templates), contentsPageSize+1)
	}
	if cat.Templates[len(cat.Templates)-1].Name != "Zebra App" {
		t.Errorf("second page missing from the catalog: %v", cat.Templates[len(cat.Templates)-1])
	}
	if len(pagesServed) < 2 {
		t.Errorf("pages requested = %v, want at least two", pagesServed)
	}
}

// TestListTemplatesStopsOnRepeatedPage guards the loop against a server that
// ignores the pagination parameters and replays the same full page forever.
func TestListTemplatesStopsOnRepeatedPage(t *testing.T) {
	page := make([]contentsEntry, contentsPageSize)
	for i := range page {
		page[i] = contentsEntry{Name: fmt.Sprintf("App %03d", i), Type: "dir"}
	}
	requests := 0
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/contents") {
			http.NotFound(w, r)
			return
		}
		requests++
		_ = json.NewEncoder(w).Encode(page)
	}))

	cat := ListTemplates(context.Background())
	if cat.Degraded {
		t.Fatalf("unexpected degraded catalog: %s", cat.Note)
	}
	if len(cat.Templates) != contentsPageSize {
		t.Errorf("templates = %d, want %d (duplicates dropped)", len(cat.Templates), contentsPageSize)
	}
	if requests > 2 {
		t.Errorf("kept requesting pages that added nothing: %d requests", requests)
	}
}

// TestSecondaryRateLimitDetected covers GitHub's secondary/abuse limit responses,
// which don't carry X-RateLimit-Remaining: 0 and used to surface as a generic
// "GitHub returned ..." error instead of the rate-limit message.
func TestSecondaryRateLimitDetected(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"429 with Retry-After", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"message":"You have exceeded a secondary rate limit"}`, http.StatusTooManyRequests)
		}},
		{"403 secondary limit body", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"You have exceeded a secondary rate limit. Please wait."}`, http.StatusForbidden)
		}},
		{"403 with Retry-After only", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "30")
			http.Error(w, `{"message":"nope"}`, http.StatusForbidden)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withServer(t, tc.handler)
			if err := DownloadTemplate(context.Background(), "Inventory Tracker", filepath.Join(t.TempDir(), "app")); err == nil ||
				!strings.Contains(err.Error(), "rate limit") {
				t.Errorf("download: expected a rate-limit error, got %v", err)
			}
			cat := ListTemplates(context.Background())
			if !cat.Degraded || !strings.Contains(cat.Note, "rate limit") {
				t.Errorf("list: expected a degraded rate-limit note, got %+v", cat)
			}
		})
	}
}

// TestForbiddenWithoutRateLimitStaysGeneric keeps the mapping honest: a plain 403
// (e.g. a private repo) must not be reported as a rate limit.
func TestForbiddenWithoutRateLimitStaysGeneric(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Must have admin rights"}`, http.StatusForbidden)
	}))
	err := DownloadTemplate(context.Background(), "Inventory Tracker", filepath.Join(t.TempDir(), "app"))
	if err == nil || strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected a generic 403 error, got %v", err)
	}
}

func TestRawURLEscapesSpaces(t *testing.T) {
	got := rawURL("Business Intelligence Dashboard/pages/page 1.py")
	for _, want := range []string{
		"Business%20Intelligence%20Dashboard",
		"page%201.py",
		"/" + repoOwner + "/" + repoName + "/" + repoRef + "/",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rawURL()=%q missing %q", got, want)
		}
	}
}
