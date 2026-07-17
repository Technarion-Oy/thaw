// SPDX-License-Identifier: GPL-3.0-or-later

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/mod/semver"
)

// releasesLatestURL is GitHub's "latest published, non-draft, non-prerelease
// release" endpoint for the Thaw repository. No authentication is required for a
// public repo; unauthenticated calls are limited to 60/hour, well above what the
// throttled background check consumes.
const releasesLatestURL = "https://api.github.com/repos/Technarion-Oy/thaw/releases/latest"

// userAgent is sent on every request — the GitHub API rejects requests without a
// User-Agent header.
const userAgent = "Thaw-UpdateChecker"

// CheckResult is the outcome of an update check, returned to the frontend by the
// CheckForUpdate IPC method and emitted on the "update:available" event.
type CheckResult struct {
	// Available is true when LatestVersion is a strictly newer semantic version
	// than the running build.
	Available bool `json:"available"`
	// CurrentVersion is the running build's version (internal/version.Version).
	CurrentVersion string `json:"currentVersion"`
	// LatestVersion is the latest release's version, with any leading "v"
	// stripped for display.
	LatestVersion string `json:"latestVersion"`
	// ReleaseNotes is the release body (Markdown) shown in the update modal.
	ReleaseNotes string `json:"releaseNotes"`
	// ReleasePageURL is the GitHub release page, opened in the default browser by
	// the "Download update" button.
	ReleasePageURL string `json:"releasePageURL"`
}

// githubRelease is the subset of the GitHub release JSON we consume.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// Check fetches the latest release and compares it against currentVersion. It
// never returns Available=true for a "dev" build — callers should skip the check
// entirely for dev builds, but this is a defensive second line. A network,
// proxy, or parse failure is returned as an error; the caller decides whether to
// surface or silently log it.
func Check(ctx context.Context, currentVersion string) (CheckResult, error) {
	rel, err := fetchLatest(ctx)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion}, err
	}
	res := CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  strings.TrimPrefix(rel.TagName, "v"),
		ReleaseNotes:   rel.Body,
		ReleasePageURL: rel.HTMLURL,
	}
	res.Available = IsNewer(rel.TagName, currentVersion)
	return res, nil
}

// IsNewer reports whether latest is a strictly newer semantic version than
// current. Both accept an optional leading "v". A "dev" (or otherwise
// non-semver) current version never counts as older, so IsNewer returns false —
// this keeps local/dev builds from being nagged to "update". An unparseable
// latest tag also yields false.
func IsNewer(latest, current string) bool {
	lv := normalizeSemver(latest)
	cv := normalizeSemver(current)
	if !semver.IsValid(lv) || !semver.IsValid(cv) {
		return false
	}
	return semver.Compare(lv, cv) > 0
}

// normalizeSemver ensures a leading "v" so the string is acceptable to
// golang.org/x/mod/semver, which requires it.
func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// fetchLatest performs the proxy-aware GET and decodes the release JSON.
func fetchLatest(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesLatestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := doWithProxyFallback(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		// Read a little of the body for diagnostics without slurping a huge page.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("update check: GitHub returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("update check: decode release: %w", err)
	}
	return &rel, nil
}
