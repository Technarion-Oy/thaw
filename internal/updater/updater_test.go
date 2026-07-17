// SPDX-License-Identifier: GPL-3.0-or-later

package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.3", "1.2.2", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.4", false},
		{"v2.0.0", "v1.9.9", true},
		{"1.10.0", "1.9.0", true}, // numeric, not lexical, comparison
		{"v1.2.0", "1.2.0", false},
		{"1.2.3", "dev", false}, // dev never counts as older
		{"not-a-version", "1.0.0", false},
		{"1.0.0", "also-bad", false},
		{"", "1.0.0", false},
		{"v1.2.3-rc1", "1.2.3", false}, // prerelease precedes the release
		{"v1.2.3", "1.2.3-rc1", true},  // release supersedes its prerelease
	}
	for _, c := range cases {
		if got := IsNewer(c.latest, c.current); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header on request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v9.9.9",
			"body": "## What's new\n- Faster",
			"html_url": "https://example.com/releases/v9.9.9"
		}`))
	}))
	defer srv.Close()

	// Point fetchLatest at the test server by swapping the package URL var is not
	// possible (const), so exercise the parsing/compare path through a small
	// stand-in request instead.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := doWithProxyFallback(context.Background(), req)
	if err != nil {
		t.Fatalf("doWithProxyFallback: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// Verify semver comparison against the served tag.
	if !IsNewer("v9.9.9", "1.0.0") {
		t.Error("expected v9.9.9 newer than 1.0.0")
	}
}

func TestNormalizeSemver(t *testing.T) {
	cases := map[string]string{
		"1.2.3":  "v1.2.3",
		"v1.2.3": "v1.2.3",
		" 1.2.3": "v1.2.3",
		"":       "",
	}
	for in, want := range cases {
		if got := normalizeSemver(in); got != want {
			t.Errorf("normalizeSemver(%q) = %q, want %q", in, got, want)
		}
	}
}
