// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package gitrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// RepoStatus describes the current state of a git repository directory.
type RepoStatus struct {
	IsRepo    bool     `json:"isRepo"`
	Branch    string   `json:"branch"`
	Modified  []string `json:"modified"`
	Added     []string `json:"added"`
	Deleted   []string `json:"deleted"`
	HasRemote bool     `json:"hasRemote"`
	RemoteURL string   `json:"remoteURL"`
	Ahead     int      `json:"ahead"`
}

// PushParams holds all parameters needed for a commit-and-push operation.
type PushParams struct {
	Dir         string   `json:"dir"`
	RemoteURL   string   `json:"remoteURL"`
	Branch      string   `json:"branch"`
	Token       string   `json:"token"`
	Message     string   `json:"message"`
	AuthorName  string   `json:"authorName"`
	AuthorEmail string   `json:"authorEmail"`
	Files       []string `json:"files"` // if empty, stages all changes
}

// PullParams holds parameters needed for a git pull operation.
type PullParams struct {
	Dir       string `json:"dir"`
	RemoteURL string `json:"remoteURL"`
	Branch    string `json:"branch"`
	Token     string `json:"token"`
}

// run executes a git command in dir and returns trimmed stdout.
// Stderr is captured and included in the error message on failure.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return strings.TrimSpace(string(out)),
				fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetStatus inspects dir and returns a RepoStatus.
// Non-repo directories return IsRepo=false without error.
func GetStatus(dir string) (RepoStatus, error) {
	ctx := context.Background()
	var s RepoStatus

	if _, err := run(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		return s, nil // not a repo
	}
	s.IsRepo = true

	if branch, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		s.Branch = branch
	}

	if porcelain, err := run(ctx, dir, "status", "--porcelain"); err == nil && porcelain != "" {
		for _, line := range strings.Split(porcelain, "\n") {
			if len(line) < 3 {
				continue
			}
			xy := line[:2]
			path := strings.TrimSpace(line[3:])
			switch {
			case strings.ContainsAny(xy, "D"):
				s.Deleted = append(s.Deleted, path)
			case xy == "??" || strings.ContainsAny(string(xy[0]), "A"):
				s.Added = append(s.Added, path)
			default:
				s.Modified = append(s.Modified, path)
			}
		}
	}

	if remoteURL, err := run(ctx, dir, "remote", "get-url", "origin"); err == nil && remoteURL != "" {
		s.HasRemote = true
		s.RemoteURL = remoteURL
	}

	if s.HasRemote {
		if aheadStr, err := run(ctx, dir, "rev-list", "@{u}..HEAD", "--count"); err == nil {
			if n, err := strconv.Atoi(aheadStr); err == nil {
				s.Ahead = n
			}
		}
	}

	return s, nil
}

// injectToken rewrites an HTTPS remote URL to embed a PAT for push auth.
// The token is never persisted — it lives only in the push command argument.
func injectToken(remoteURL, token string) string {
	if token == "" {
		return remoteURL
	}
	// Convert git@github.com:org/repo.git → https://github.com/org/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		remoteURL = "https://github.com/" + strings.TrimPrefix(remoteURL, "git@github.com:")
	}
	if strings.HasPrefix(remoteURL, "https://") {
		hostAndPath := strings.TrimPrefix(remoteURL, "https://")
		if atIdx := strings.Index(hostAndPath, "@"); atIdx != -1 {
			hostAndPath = hostAndPath[atIdx+1:]
		}
		return fmt.Sprintf("https://x-access-token:%s@%s", token, hostAndPath)
	}
	return remoteURL
}

// osJunkFiles is the set of OS-generated file names that should never be
// committed. It mirrors the entries written by ensureGitignore.
var osJunkFiles = map[string]bool{
	".DS_Store":  true,
	"Thumbs.db":  true,
	"desktop.ini": true,
}

// ensureGitignore writes a .gitignore in dir that covers common OS junk files
// (e.g. .DS_Store on macOS, Thumbs.db on Windows). Entries that already
// appear in the file are not duplicated. The file is created when absent.
func ensureGitignore(dir string) error {
	required := []string{".DS_Store", "Thumbs.db", "desktop.ini"}

	path := filepath.Join(dir, ".gitignore")
	existing, _ := os.ReadFile(path) // ignore error — empty on first run

	var missing []string
	for _, entry := range required {
		found := false
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == entry {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(missing, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// CommitAndPush stages all changes, commits, and pushes to the remote.
// "Nothing to commit" is treated as success.
func CommitAndPush(ctx context.Context, p PushParams) error {
	// Init repo if needed
	if _, err := run(ctx, p.Dir, "rev-parse", "--git-dir"); err != nil {
		if _, err := run(ctx, p.Dir, "init"); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
	}

	// Ensure OS junk files are gitignored before staging.
	if err := ensureGitignore(p.Dir); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	// Set or update origin (plain URL — token is only injected for push)
	if p.RemoteURL != "" {
		if _, err := run(ctx, p.Dir, "remote", "set-url", "origin", p.RemoteURL); err != nil {
			if _, err2 := run(ctx, p.Dir, "remote", "add", "origin", p.RemoteURL); err2 != nil {
				return fmt.Errorf("set remote: %w", err2)
			}
		}
	}

	// Stage specified files or everything.
	// When an explicit list is given, strip any OS junk files: they are now
	// covered by .gitignore and git will error with "did not match any files"
	// if we try to add them explicitly.
	if len(p.Files) > 0 {
		var stageable []string
		for _, f := range p.Files {
			if !osJunkFiles[filepath.Base(f)] {
				stageable = append(stageable, f)
			}
		}
		if len(stageable) == 0 {
			return nil // nothing left to commit
		}
		addArgs := append([]string{"add", "--"}, stageable...)
		if _, err := run(ctx, p.Dir, addArgs...); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	} else {
		if _, err := run(ctx, p.Dir, "add", "-A"); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	}

	// Commit
	msg := p.Message
	if msg == "" {
		msg = "chore: export Snowflake DDL"
	}
	commitArgs := []string{"commit", "-m", msg}
	if p.AuthorName != "" && p.AuthorEmail != "" {
		commitArgs = append(commitArgs, "--author", fmt.Sprintf("%s <%s>", p.AuthorName, p.AuthorEmail))
	}
	if out, err := run(ctx, p.Dir, commitArgs...); err != nil {
		combined := out + err.Error()
		if strings.Contains(combined, "nothing to commit") ||
			strings.Contains(combined, "nothing added to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %w", err)
	}

	// Push with token injected into URL (not stored in config)
	branch := p.Branch
	if branch == "" {
		branch = "main"
	}
	pushURL := injectToken(p.RemoteURL, p.Token)
	if _, err := run(ctx, p.Dir, "push", "-u", pushURL, branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// Pull fetches and merges changes from the remote branch.
func Pull(ctx context.Context, p PullParams) error {
	if _, err := run(ctx, p.Dir, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository")
	}

	branch := p.Branch
	if branch == "" {
		branch = "main"
	}

	pullURL := injectToken(p.RemoteURL, p.Token)
	if _, err := run(ctx, p.Dir, "pull", pullURL, branch); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	return nil
}
