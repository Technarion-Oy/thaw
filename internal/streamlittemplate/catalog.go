// SPDX-License-Identifier: GPL-3.0-or-later

package streamlittemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	repoOwner = "Snowflake-Labs"
	repoName  = "snowflake-demo-streamlit"
	repoRef   = "main"
	userAgent = "Thaw-StreamlitTemplates"

	// RepoURL is the human-facing source URL, surfaced in the UI attribution line
	// and the scaffolded NOTICE file.
	RepoURL = "https://github.com/" + repoOwner + "/" + repoName

	// maxResponseBytes caps a single HTTP response body (tree JSON or a raw file),
	// a guard against a pathologically large download.
	maxResponseBytes = 32 << 20 // 32 MiB
)

// Base URLs are package vars (not consts) so tests can point them at an httptest
// server.
var (
	githubAPIBase = "https://api.github.com"
	rawBase       = "https://raw.githubusercontent.com"
)

// httpClient is shared by all template requests. Individual calls are additionally
// bounded by the caller's context deadline.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// excludedTopLevel are top-level entries that are not deployable apps.
var excludedTopLevel = map[string]bool{
	"shared_assets": true,
}

// embeddedTemplateNames is the names-only fallback returned (as a Degraded
// Catalog) when the live listing can't be fetched. It is intentionally a small,
// stable subset — not an attempt to mirror the whole repo.
var embeddedTemplateNames = []string{
	"Inventory Tracker",
	"Business Intelligence Dashboard",
	"Chat app using Snowflake Cortex",
}

// Template is a single deployable app folder from the demo repo.
type Template struct {
	Name        string `json:"name"`        // top-level folder name (also the deploy default app name)
	Description string `json:"description"` // first paragraph of the folder's README.md; "" when unavailable
}

// Catalog is the result of listing templates. When the live GitHub listing can't
// be reached, Degraded is true, Templates holds the embedded names-only fallback,
// and Note explains why — so the UI can show a usable list plus a clear warning
// rather than breaking the (additive) feature.
type Catalog struct {
	Templates []Template `json:"templates"`
	Degraded  bool       `json:"degraded"`
	Note      string     `json:"note"`
}

// ListTemplates fetches the deployable top-level folders from the demo repo, each
// with a short description taken from its README.md first paragraph (best-effort,
// fetched in parallel). It never returns an error for network/rate-limit failures:
// those yield a Degraded Catalog carrying the embedded fallback names.
func ListTemplates(ctx context.Context) Catalog {
	names, err := fetchTopLevelDirs(ctx)
	if err != nil {
		return Catalog{
			Templates: fallbackTemplates(),
			Degraded:  true,
			Note:      fmt.Sprintf("Showing a limited built-in list — couldn't reach %s: %v", RepoURL, err),
		}
	}

	templates := make([]Template, len(names))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i, name := range names {
		templates[i] = Template{Name: name}
		g.Go(func() error {
			// Best-effort: a failed/empty README just leaves the description blank.
			if desc := fetchDescription(gctx, name); desc != "" {
				templates[i].Description = desc
			}
			return nil
		})
	}
	_ = g.Wait() // goroutines never return an error (descriptions are best-effort)

	return Catalog{Templates: templates}
}

func fallbackTemplates() []Template {
	out := make([]Template, len(embeddedTemplateNames))
	for i, n := range embeddedTemplateNames {
		out[i] = Template{Name: n}
	}
	return out
}

// contentsEntry is one item from the GitHub "list repository contents" response.
type contentsEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "dir" | "file"
}

// fetchTopLevelDirs returns the deployable top-level folder names, sorted,
// excluding shared_assets and hidden entries.
func fetchTopLevelDirs(ctx context.Context) ([]string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents?ref=%s",
		githubAPIBase, url.PathEscape(repoOwner), url.PathEscape(repoName), url.QueryEscape(repoRef))
	body, err := httpGet(ctx, u, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var entries []contentsEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode repo contents: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.Type != "dir" || excludedTopLevel[e.Name] || strings.HasPrefix(e.Name, ".") {
			continue
		}
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names, nil
}

// fetchDescription downloads a template's README.md and returns its first
// paragraph as a one-line description. Returns "" on any failure.
func fetchDescription(ctx context.Context, name string) string {
	body, err := httpGet(ctx, rawURL(path.Join(name, "README.md")), "")
	if err != nil {
		return ""
	}
	return firstParagraph(string(body))
}

// firstParagraph extracts a one-line summary from Markdown: it skips leading
// headings, badges, and blank lines, then joins the first non-empty text block
// into a single whitespace-collapsed line with light Markdown stripping. The
// result is truncated to a sentence-ish length for list display.
func firstParagraph(md string) string {
	lines := strings.Split(md, "\n")
	var para []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			if len(para) > 0 {
				break // end of the first paragraph
			}
			continue // still skipping leading blanks
		}
		// Skip headings, blockquotes, images/badges, and HTML comment lines.
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") ||
			strings.HasPrefix(line, "![") || strings.HasPrefix(line, "<!--") {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, line)
	}
	text := stripInlineMarkdown(strings.Join(para, " "))
	text = strings.Join(strings.Fields(text), " ")
	return truncate(text, 200)
}

// stripInlineMarkdown removes the most common inline Markdown so a README's first
// paragraph reads as plain text: [label](url) → label, and *, _, `, ** markers.
func stripInlineMarkdown(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '[':
			// [label](url) → label
			close := strings.IndexByte(s[i:], ']')
			if close > 0 && i+close+1 < len(s) && s[i+close+1] == '(' {
				label := s[i+1 : i+close]
				b.WriteString(label)
				end := strings.IndexByte(s[i+close+1:], ')')
				if end >= 0 {
					i = i + close + 1 + end
					continue
				}
			}
			b.WriteByte(c)
		case '*', '_', '`':
			// drop emphasis / code markers
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// httpGet performs a GET with the shared client, returning the body bytes. It
// maps a GitHub rate-limit 403 and other non-2xx statuses to clear errors.
func httpGet(ctx context.Context, u, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return nil, fmt.Errorf("GitHub API rate limit exceeded; try again later")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
}

// rawURL builds a raw.githubusercontent.com URL for a repo-relative path,
// percent-encoding each path segment (folder names may contain spaces).
func rawURL(relPath string) string {
	segs := append([]string{repoOwner, repoName, repoRef}, strings.Split(relPath, "/")...)
	escaped := make([]string, 0, len(segs))
	for _, s := range segs {
		if s == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(s))
	}
	return rawBase + "/" + strings.Join(escaped, "/")
}
