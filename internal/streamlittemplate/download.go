// SPDX-License-Identifier: GPL-3.0-or-later

package streamlittemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DownloadTemplate scaffolds a single template folder from the demo repo into
// destDir, preserving its relative structure. It fetches only the chosen folder's
// files (git-tree API + raw downloads), never a full clone. The repo's Apache-2.0
// LICENSE and a NOTICE provenance line are written alongside the template files
// for attribution.
//
// destDir must be empty or not yet exist — DownloadTemplate refuses to overwrite a
// non-empty target (the caller confirms destination choice in the UI).
func DownloadTemplate(ctx context.Context, name, destDir string) error {
	if !validTemplateName(name) {
		return fmt.Errorf("invalid template name %q", name)
	}
	if strings.TrimSpace(destDir) == "" {
		return fmt.Errorf("destination directory is required")
	}

	empty, err := destIsEmpty(destDir)
	if err != nil {
		return fmt.Errorf("check destination: %w", err)
	}
	if !empty {
		return fmt.Errorf("destination folder is not empty: %s", destDir)
	}

	rels, err := fetchTemplateFiles(ctx, name)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		return fmt.Errorf("template %q not found in %s/%s", name, repoOwner, repoName)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	for _, rel := range rels {
		data, err := httpGet(ctx, rawURL(path.Join(name, rel)), "")
		if err != nil {
			return fmt.Errorf("download %s: %w", rel, err)
		}
		target, err := safeJoin(destDir, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}

	if err := writeLicenseAndNotice(ctx, name, destDir); err != nil {
		return err
	}
	return nil
}

// writeLicenseAndNotice carries the Apache-2.0 attribution into the scaffolded
// folder: the repo's LICENSE (best-effort download; a bundled fallback header if
// unreachable) and a NOTICE recording the provenance. An existing LICENSE from
// the template itself is not overwritten.
func writeLicenseAndNotice(ctx context.Context, name, destDir string) error {
	licensePath := filepath.Join(destDir, "LICENSE")
	if _, err := os.Stat(licensePath); os.IsNotExist(err) {
		license, err := httpGet(ctx, rawURL("LICENSE"), "")
		if err != nil || len(license) == 0 {
			license = []byte(apacheLicenseFallback)
		}
		if err := os.WriteFile(licensePath, license, 0o644); err != nil {
			return fmt.Errorf("write LICENSE: %w", err)
		}
	}

	notice := fmt.Sprintf(
		"Based on \"%s\" from %s/%s (Apache-2.0).\nSource: %s\n",
		name, repoOwner, repoName, RepoURL)
	if err := os.WriteFile(filepath.Join(destDir, "NOTICE"), []byte(notice), 0o644); err != nil {
		return fmt.Errorf("write NOTICE: %w", err)
	}
	return nil
}

// treeResponse is the GitHub "get a tree (recursive)" response.
type treeResponse struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" | "tree"
}

// fetchTemplateFiles returns the file paths of the chosen template folder,
// relative to that folder (e.g. "streamlit_app.py", "pages/page_1.py").
func fetchTemplateFiles(ctx context.Context, name string) ([]string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1",
		githubAPIBase, url.PathEscape(repoOwner), url.PathEscape(repoName), url.PathEscape(repoRef))
	body, err := httpGet(ctx, u, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var tree treeResponse
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("decode repo tree: %w", err)
	}
	if tree.Truncated {
		return nil, fmt.Errorf("repository tree was truncated by GitHub; please try again")
	}

	prefix := name + "/"
	var rels []string
	for _, e := range tree.Tree {
		if e.Type == "blob" && strings.HasPrefix(e.Path, prefix) {
			rels = append(rels, strings.TrimPrefix(e.Path, prefix))
		}
	}
	return rels, nil
}

// validTemplateName guards against path traversal and non-app entries: a template
// name must be a single, non-hidden path segment that isn't an excluded folder.
func validTemplateName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return false
	}
	return !excludedTopLevel[name]
}

// destIsEmpty reports whether dir does not exist or is an empty directory.
func destIsEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// safeJoin joins a repo-relative file path onto base, rejecting any path that
// would escape base (defense against a malicious/odd tree entry).
func safeJoin(base, rel string) (string, error) {
	joined := filepath.Join(base, filepath.FromSlash(rel))
	within, err := filepath.Rel(base, joined)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe template path %q", rel)
	}
	return joined, nil
}

// apacheLicenseFallback is written when the repo's LICENSE can't be fetched, so a
// scaffolded folder always records the governing license.
const apacheLicenseFallback = `This template is derived from ` + RepoURL + `,
which is licensed under the Apache License, Version 2.0.
The full license text is at http://www.apache.org/licenses/LICENSE-2.0
`
