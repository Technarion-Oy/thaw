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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// maxStatusFiles is the maximum number of file paths returned per category in
// RepoStatus. When the actual count exceeds this, the remainder is reflected
// only in TotalChanged so the UI can show an accurate badge without needing to
// render thousands of rows.
const maxStatusFiles = 500

// FileChange is a single working-tree change with its VS Code-style status
// letter: "A" added/untracked-new, "M" modified, "D" deleted, "R" renamed,
// "C" copied, "U" untracked. Status is derived from the index side (staged)
// or the worktree side (unstaged) depending on which list it appears in.
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// RepoStatus describes the current state of a git repository directory.
type RepoStatus struct {
	IsRepo bool   `json:"isRepo"`
	Branch string `json:"branch"`

	// Legacy flat arrays (union of staged + unstaged), kept for compatibility.
	Modified []string `json:"modified"`
	Added    []string `json:"added"`
	Deleted  []string `json:"deleted"`

	// Staged holds files whose index differs from HEAD (git add'ed).
	// Unstaged holds files whose worktree differs from the index (incl. untracked).
	// A partially-staged file appears in both. Each is capped at maxStatusFiles;
	// StagedTotal / UnstagedTotal carry the exact counts for the paginator.
	Staged        []FileChange `json:"staged"`
	Unstaged      []FileChange `json:"unstaged"`
	StagedTotal   int          `json:"stagedTotal"`
	UnstagedTotal int          `json:"unstagedTotal"`

	HasRemote    bool   `json:"hasRemote"`
	RemoteURL    string `json:"remoteURL"`
	Ahead        int    `json:"ahead"`
	TotalChanged int    `json:"totalChanged"` // exact total distinct changed paths

	// ChangedPaths maps every changed path (repo-relative, forward-slash) to its
	// single-letter status — uncapped, so the file-explorer color overlay can
	// cover the whole tree even when the capped arrays above can't. Worktree
	// status wins; staged-only files use their staging status.
	ChangedPaths map[string]string `json:"changedPaths"`
}

// statusLetter maps a go-git StatusCode to the single-letter sigil the UI uses.
// Untracked ('?') is surfaced as "U".
func statusLetter(c gogit.StatusCode) string {
	switch c {
	case gogit.Untracked:
		return "U"
	case gogit.Added:
		return "A"
	case gogit.Deleted:
		return "D"
	case gogit.Renamed:
		return "R"
	case gogit.Copied:
		return "C"
	default:
		// Modified and UpdatedButUnmerged both read as a modification.
		return "M"
	}
}

// PushParams holds all parameters needed for a commit-and-push operation.
type PushParams struct {
	Dir         string   `json:"dir"`
	RemoteURL   string   `json:"remoteURL"`
	Branch      string   `json:"branch"`
	AuthMethod  string   `json:"authMethod"` // "pat" | "bearer" | "stored" | ""
	Token       string   `json:"token"`
	Message     string   `json:"message"`
	AuthorName  string   `json:"authorName"`
	AuthorEmail string   `json:"authorEmail"`
	Files       []string `json:"files"` // if empty (and StagedOnly false), stages all changes
	StagedOnly  bool     `json:"stagedOnly"`
}

// PullParams holds parameters needed for a git pull operation.
type PullParams struct {
	Dir        string `json:"dir"`
	RemoteURL  string `json:"remoteURL"`
	Branch     string `json:"branch"`
	AuthMethod string `json:"authMethod"` // "pat" | "bearer" | "stored" | ""
	Token      string `json:"token"`
}

// BranchInfo describes a local or remote git branch.
type BranchInfo struct {
	Name      string `json:"name"`
	IsRemote  bool   `json:"isRemote"`
	IsCurrent bool   `json:"isCurrent"`
}

// CloneParams holds parameters needed for a git clone operation.
type CloneParams struct {
	URL        string `json:"url"`
	Path       string `json:"path"`
	AuthMethod string `json:"authMethod"` // "pat" | "bearer" | "stored" | ""
	Token      string `json:"token"`
}

// normaliseHTTPS converts SSH URLs (like git@github.com:org/repo.git) to
// HTTPS form. This is required when using token-based authentication (PAT/OAuth),
// as those credentials only apply to the HTTPS transport.
func normaliseHTTPS(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	// Handle git@host:path/repo.git
	if strings.HasPrefix(remoteURL, "git@") && strings.Contains(remoteURL, ":") {
		parts := strings.SplitN(strings.TrimPrefix(remoteURL, "git@"), ":", 2)
		if len(parts) == 2 {
			return "https://" + parts[0] + "/" + parts[1]
		}
	}
	// Handle ssh://git@host/path/repo.git
	if strings.HasPrefix(remoteURL, "ssh://git@") {
		return "https://" + strings.TrimPrefix(remoteURL, "ssh://git@")
	}
	return remoteURL
}

// osJunkFiles is the set of OS-generated file names that should never be committed.
var osJunkFiles = map[string]bool{
	".DS_Store":   true,
	"Thumbs.db":   true,
	"desktop.ini": true,
}

// ensureGitignore writes a .gitignore in dir that covers common OS junk files.
func ensureGitignore(dir string) error {
	required := []string{".DS_Store", "Thumbs.db", "desktop.ini"}
	path := filepath.Join(dir, ".gitignore")
	existing, _ := os.ReadFile(path)

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

// GetStatus inspects dir and returns a RepoStatus.
// Non-repo directories return IsRepo=false without error.
func GetStatus(dir string) (RepoStatus, error) {
	var s RepoStatus

	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: false})
	if err != nil {
		return s, nil // not a repo
	}
	s.IsRepo = true

	// Branch name
	head, err := repo.Head()
	if err == nil {
		s.Branch = head.Name().Short()
	}

	// Working tree status
	wt, err := repo.Worktree()
	if err == nil {
		st, err := wt.Status()
		if err == nil {
			s.ChangedPaths = make(map[string]string, len(st))
			for path, fs := range st {
				x := fs.Staging
				y := fs.Worktree
				s.TotalChanged++

				// Uncapped color overlay: worktree status wins, else staging.
				disp := y
				if y == gogit.Unmodified {
					disp = x
				}
				s.ChangedPaths[filepath.ToSlash(path)] = statusLetter(disp)

				// Staged side: index differs from HEAD. Untracked is never staged.
				if x != gogit.Unmodified && x != gogit.Untracked {
					s.StagedTotal++
					if len(s.Staged) < maxStatusFiles {
						s.Staged = append(s.Staged, FileChange{Path: path, Status: statusLetter(x)})
					}
				}
				// Unstaged side: worktree differs from the index (includes untracked).
				if y != gogit.Unmodified {
					s.UnstagedTotal++
					if len(s.Unstaged) < maxStatusFiles {
						s.Unstaged = append(s.Unstaged, FileChange{Path: path, Status: statusLetter(y)})
					}
				}

				// Legacy union arrays.
				switch {
				case x == gogit.Deleted || y == gogit.Deleted:
					if len(s.Deleted) < maxStatusFiles {
						s.Deleted = append(s.Deleted, path)
					}
				case x == gogit.Added || x == gogit.Untracked || y == gogit.Untracked:
					if len(s.Added) < maxStatusFiles {
						s.Added = append(s.Added, path)
					}
				default:
					if len(s.Modified) < maxStatusFiles {
						s.Modified = append(s.Modified, path)
					}
				}
			}
		}
	}

	// Remote URL
	remote, err := repo.Remote("origin")
	if err == nil && remote != nil {
		urls := remote.Config().URLs
		if len(urls) > 0 {
			s.HasRemote = true
			s.RemoteURL = urls[0]
		}
	}

	// Ahead count: commits in HEAD not reachable from upstream tracking ref
	if s.HasRemote && head != nil {
		cfg, _ := repo.Config()
		var trackingRef plumbing.ReferenceName
		if cfg != nil {
			branchName := head.Name().Short()
			if bc, ok := cfg.Branches[branchName]; ok && bc.Remote != "" {
				trackingRef = plumbing.NewRemoteReferenceName(bc.Remote, bc.Merge.Short())
			}
		}
		if trackingRef == "" {
			// Fallback: guess origin/<branch>
			trackingRef = plumbing.NewRemoteReferenceName("origin", head.Name().Short())
		}
		upstreamRef, err := repo.Reference(trackingRef, true)
		if err == nil {
			logs, err := repo.Log(&gogit.LogOptions{From: head.Hash()})
			if err == nil {
				_ = logs.ForEach(func(c *object.Commit) error {
					if c.Hash == upstreamRef.Hash() {
						return errors.New("stop")
					}
					s.Ahead++
					return nil
				})
			}
		}
	}

	return s, nil
}

// CommitAndPush stages files, commits, and pushes to the remote.
// "Nothing to commit" / "already up-to-date" are treated as success.
func CommitAndPush(ctx context.Context, p PushParams) error {
	// Init repo if directory is not yet a repo.
	repo, err := gogit.PlainOpen(p.Dir)
	if err != nil {
		if !errors.Is(err, gogit.ErrRepositoryNotExists) {
			return fmt.Errorf("open repo: %w", err)
		}
		repo, err = gogit.PlainInit(p.Dir, false)
		if err != nil {
			return fmt.Errorf("git init: %w", err)
		}
	}

	// Ensure OS junk files are gitignored before staging.
	if err := ensureGitignore(p.Dir); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	remoteURL := p.RemoteURL
	if remoteURL == "" {
		if remote, err := repo.Remote("origin"); err == nil && remote != nil {
			if urls := remote.Config().URLs; len(urls) > 0 {
				remoteURL = urls[0]
			}
		}
	}

	// Set or update origin remote (plain URL — token injected only for push).
	if p.RemoteURL != "" {
		normalised := normaliseHTTPS(p.RemoteURL)
		existing, err := repo.Remote("origin")
		if err == nil {
			// Update URL if different.
			if len(existing.Config().URLs) == 0 || existing.Config().URLs[0] != normalised {
				_ = repo.DeleteRemote("origin")
				if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{normalised}}); err != nil {
					return fmt.Errorf("set remote: %w", err)
				}
			}
		} else {
			if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{normalised}}); err != nil {
				return fmt.Errorf("add remote: %w", err)
			}
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	// Stage specified files or everything. When StagedOnly is set the commit
	// operates on whatever is already in the index — nothing new is staged.
	if p.StagedOnly {
		// no-op: commit the existing index as-is
	} else if len(p.Files) > 0 {
		var stageable []string
		for _, f := range p.Files {
			if !osJunkFiles[filepath.Base(f)] {
				stageable = append(stageable, f)
			}
		}
		if len(stageable) == 0 {
			return nil
		}
		for _, f := range stageable {
			rel, err := filepath.Rel(p.Dir, f)
			if err != nil {
				rel = f
			}
			if err := wt.AddWithOptions(&gogit.AddOptions{Path: rel}); err != nil {
				return fmt.Errorf("git add %s: %w", rel, err)
			}
		}
	} else {
		if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
			return fmt.Errorf("git add -A: %w", err)
		}
	}

	msg := p.Message
	if msg == "" {
		msg = "chore: export Snowflake DDL"
	}

	authorName := p.AuthorName
	authorEmail := p.AuthorEmail
	if authorName == "" {
		authorName = "Thaw"
	}
	if authorEmail == "" {
		authorEmail = "thaw@local"
	}

	commitHash, err := wt.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		if errors.Is(err, gogit.ErrEmptyCommit) ||
			strings.Contains(err.Error(), "nothing to commit") ||
			strings.Contains(err.Error(), "nothing added to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %w", err)
	}
	_ = commitHash

	branch := p.Branch
	if branch == "" {
		branch = "main"
	}

	pushOpts := &gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))},
		Auth:       resolveAuth(normaliseHTTPS(remoteURL), p.AuthMethod, p.Token),
	}
	// Force using the normalised (HTTPS) URL if we resolved one,
	// even if the remote "origin" is SSH.
	if remoteURL != "" {
		pushOpts.RemoteURL = normaliseHTTPS(remoteURL)
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// ── Index (staging) operations ──────────────────────────────────────────────
//
// These give the UI a real git index to work with: stage a single file, unstage
// it, stage/unstage everything, or discard a file's changes. Commit then runs
// over the staged set (CommitAndPush with StagedOnly) rather than re-selecting
// files. go-git exposes no "git add -p", so staging is whole-file only.

// openWorktree opens the repository at dir and returns its worktree.
func openWorktree(dir string) (*gogit.Repository, *gogit.Worktree, error) {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, nil, fmt.Errorf("not a git repository")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, fmt.Errorf("worktree: %w", err)
	}
	return repo, wt, nil
}

// repoRelPath normalises file (absolute or already-relative) into a
// forward-slash path relative to the worktree root, the form go-git's index and
// tree APIs expect.
func repoRelPath(wt *gogit.Worktree, dir, file string) string {
	if !filepath.IsAbs(file) {
		return filepath.ToSlash(file)
	}
	root := wt.Filesystem.Root()
	if rel, err := filepath.Rel(root, file); err == nil {
		return filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(dir, file); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(file)
}

// headTreeEntry returns the HEAD tree entry for rel. found is false when there
// is no HEAD commit yet, or rel is not tracked in HEAD (i.e. newly added).
func headTreeEntry(repo *gogit.Repository, rel string) (entry *object.TreeEntry, found bool) {
	head, err := repo.Head()
	if err != nil {
		return nil, false
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, false
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, false
	}
	te, err := tree.FindEntry(rel)
	if err != nil {
		return nil, false
	}
	return te, true
}

// StageFile stages a single file (git add <file>). Deletions are staged too.
// OS junk files are silently skipped.
func StageFile(dir, file string) error {
	_, wt, err := openWorktree(dir)
	if err != nil {
		return err
	}
	rel := repoRelPath(wt, dir, file)
	if osJunkFiles[filepath.Base(rel)] {
		return nil
	}
	if err := wt.AddWithOptions(&gogit.AddOptions{Path: rel}); err != nil {
		return fmt.Errorf("git add %s: %w", rel, err)
	}
	return nil
}

// StageAll stages every working-tree change (git add -A), skipping OS junk.
func StageAll(dir string) error {
	_, wt, err := openWorktree(dir)
	if err != nil {
		return err
	}
	_ = ensureGitignore(dir)
	st, err := wt.Status()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	for p, fs := range st {
		if fs.Worktree == gogit.Unmodified {
			continue // already matches the index — nothing to stage
		}
		if osJunkFiles[filepath.Base(p)] {
			continue
		}
		if err := wt.AddWithOptions(&gogit.AddOptions{Path: p}); err != nil {
			return fmt.Errorf("git add %s: %w", p, err)
		}
	}
	return nil
}

// UnstageFile removes a file from the index, restoring its staged state to HEAD
// (git reset HEAD -- <file>). Newly-added files are dropped from the index
// entirely. The working-tree copy is left untouched.
//
// It delegates to a path-constrained mixed reset rather than hand-editing the
// index so the entry's stat cache is reset too — otherwise go-git's status
// fast-path would still see the file as matching the (stale) index.
func UnstageFile(dir, file string) error {
	repo, wt, err := openWorktree(dir)
	if err != nil {
		return err
	}
	rel := repoRelPath(wt, dir, file)

	head, err := repo.Head()
	if err != nil {
		// No commits yet — there is no HEAD to reset to; just drop the entry.
		idx, err := repo.Storer.Index()
		if err != nil {
			return fmt.Errorf("read index: %w", err)
		}
		if _, err := idx.Remove(rel); err != nil && !errors.Is(err, index.ErrEntryNotFound) {
			return fmt.Errorf("unstage %s: %w", rel, err)
		}
		if err := repo.Storer.SetIndex(idx); err != nil {
			return fmt.Errorf("write index: %w", err)
		}
		return nil
	}

	if err := wt.Reset(&gogit.ResetOptions{
		Commit: head.Hash(),
		Mode:   gogit.MixedReset,
		Files:  []string{rel},
	}); err != nil {
		return fmt.Errorf("unstage %s: %w", rel, err)
	}
	return nil
}

// UnstageAll resets the whole index to HEAD (git reset HEAD), leaving the
// working tree untouched.
func UnstageAll(dir string) error {
	repo, wt, err := openWorktree(dir)
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		// No commits yet — clear the index entirely.
		if err := repo.Storer.SetIndex(&index.Index{Version: 2}); err != nil {
			return fmt.Errorf("write index: %w", err)
		}
		return nil
	}
	return wt.Reset(&gogit.ResetOptions{Commit: head.Hash(), Mode: gogit.MixedReset})
}

// DiscardFile reverts a file to its HEAD state: tracked files are restored from
// HEAD (and unstaged); untracked / newly-added files are removed from the index
// and deleted from disk. This cannot be undone.
func DiscardFile(dir, file string) error {
	repo, wt, err := openWorktree(dir)
	if err != nil {
		return err
	}
	rel := repoRelPath(wt, dir, file)
	absPath := filepath.Join(wt.Filesystem.Root(), filepath.FromSlash(rel))

	te, found := headTreeEntry(repo, rel)
	if !found {
		// Untracked or newly-added: drop from index and delete from disk.
		if idx, err := repo.Storer.Index(); err == nil {
			if _, err := idx.Remove(rel); err == nil || errors.Is(err, index.ErrEntryNotFound) {
				_ = repo.Storer.SetIndex(idx)
			}
		}
		if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("discard %s: %w", rel, err)
		}
		return nil
	}

	// Tracked: restore the worktree copy from the HEAD blob, then unstage.
	blob, err := repo.BlobObject(te.Hash)
	if err != nil {
		return fmt.Errorf("read HEAD blob: %w", err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return fmt.Errorf("read HEAD blob: %w", err)
	}
	defer reader.Close()
	contents, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read HEAD blob: %w", err)
	}
	if err := os.WriteFile(absPath, contents, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", rel, err)
	}
	return UnstageFile(dir, file)
}

// Pull fetches and merges changes from the remote branch.
func Pull(ctx context.Context, p PullParams) error {
	repo, err := gogit.PlainOpen(p.Dir)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	branch := p.Branch
	if branch == "" {
		branch = "main"
	}

	remoteURL := p.RemoteURL
	if remoteURL == "" {
		if remote, err := repo.Remote("origin"); err == nil && remote != nil {
			if urls := remote.Config().URLs; len(urls) > 0 {
				remoteURL = urls[0]
			}
		}
	}

	pullOpts := &gogit.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          resolveAuth(normaliseHTTPS(remoteURL), p.AuthMethod, p.Token),
	}
	if remoteURL != "" {
		pullOpts.RemoteURL = normaliseHTTPS(remoteURL)
	}

	err = wt.PullContext(ctx, pullOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git pull: %w", err)
	}
	return nil
}

// Fetch updates all remote-tracking refs from "origin".
// "already up-to-date" is treated as success.
// If token is empty the repo's stored credentials (if any) are tried.
func Fetch(ctx context.Context, dir, token string) error {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	remote, err := repo.Remote("origin")
	if err != nil || remote == nil {
		return fmt.Errorf("no remote 'origin' configured")
	}
	remoteURL := ""
	if urls := remote.Config().URLs; len(urls) > 0 {
		remoteURL = urls[0]
	}

	fetchOpts := &gogit.FetchOptions{
		RemoteName: "origin",
		Auth:       resolveAuth(normaliseHTTPS(remoteURL), "oauth", token),
	}
	if remoteURL != "" {
		fetchOpts.RemoteURL = normaliseHTTPS(remoteURL)
	}

	err = repo.FetchContext(ctx, fetchOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

// InitWithRemote initializes a new git repository at dir, sets the "origin"
// remote URL, and configures HEAD to point to branch. The repository is left
// empty (no commits) — ready for the user's first commit and push.
// If dir is already a git repository it is re-used; only the remote and HEAD
// are updated.
func InitWithRemote(dir, remoteURL, branch string) error {
	if branch == "" {
		branch = "main"
	}
	normalised := normaliseHTTPS(remoteURL)

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		if !errors.Is(err, gogit.ErrRepositoryAlreadyExists) {
			return fmt.Errorf("git init: %w", err)
		}
		repo, err = gogit.PlainOpen(dir)
		if err != nil {
			return fmt.Errorf("open repo: %w", err)
		}
	}

	// Set or replace the origin remote.
	_ = repo.DeleteRemote("origin")
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{normalised},
	}); err != nil {
		return fmt.Errorf("set remote: %w", err)
	}

	// Point HEAD at the desired branch name.
	headRef := plumbing.NewSymbolicReference(
		plumbing.HEAD,
		plumbing.NewBranchReferenceName(branch),
	)
	if err := repo.Storer.SetReference(headRef); err != nil {
		return fmt.Errorf("set HEAD: %w", err)
	}

	return nil
}

// Clone clones a remote repository into the given local path.
// If the clone fails part-way (e.g. the remote is an empty repository with no
// commits), any .git directory created during the attempt is removed so the
// caller can safely retry without hitting ErrRepositoryAlreadyExists.
func Clone(ctx context.Context, p CloneParams) error {
	url := normaliseHTTPS(p.URL)

	// Remember whether .git already existed before we start, so we don't
	// delete something the user already had.
	gitDir := filepath.Join(p.Path, ".git")
	_, statErr := os.Stat(gitDir)
	gitExistedBefore := statErr == nil

	_, err := gogit.PlainCloneContext(ctx, p.Path, false, &gogit.CloneOptions{
		URL:      url,
		Auth:     resolveAuth(url, p.AuthMethod, p.Token),
		Progress: nil,
	})
	if err != nil {
		// Clean up the partial .git we created so the user can retry.
		if !gitExistedBefore {
			_ = os.RemoveAll(gitDir)
		}
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return fmt.Errorf("git clone: the remote repository is empty (no commits yet)")
		}
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

// ListBranches returns all local and remote branches in the repo at dir.
func ListBranches(dir string) ([]BranchInfo, error) {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("not a git repository")
	}

	head, _ := repo.Head()
	var currentBranch string
	if head != nil {
		currentBranch = head.Name().Short()
	}

	var branches []BranchInfo

	// Local branches
	localIter, err := repo.Branches()
	if err == nil {
		_ = localIter.ForEach(func(ref *plumbing.Reference) error {
			name := ref.Name().Short()
			branches = append(branches, BranchInfo{
				Name:      name,
				IsRemote:  false,
				IsCurrent: name == currentBranch,
			})
			return nil
		})
	}

	// Remote-tracking refs
	refs, err := repo.References()
	if err == nil {
		_ = refs.ForEach(func(ref *plumbing.Reference) error {
			if strings.HasPrefix(string(ref.Name()), "refs/remotes/") {
				// Strip "refs/remotes/" prefix and skip HEAD pseudo-refs
				shortName := strings.TrimPrefix(string(ref.Name()), "refs/remotes/")
				if strings.HasSuffix(shortName, "/HEAD") {
					return nil
				}
				branches = append(branches, BranchInfo{
					Name:      shortName,
					IsRemote:  true,
					IsCurrent: false,
				})
			}
			return nil
		})
	}

	return branches, nil
}

// CheckoutBranch checks out an existing local branch.
func CheckoutBranch(dir, name string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	return wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Create: false,
	})
}

// CreateBranch creates and checks out a new branch.
func CreateBranch(dir, name string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	return wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Create: true,
	})
}

// CheckoutRemoteBranch creates a local branch from a remote-tracking ref and
// checks it out. remoteName must be in "origin/<branch>" form (the format
// returned by ListBranches for remote entries).
func CheckoutRemoteBranch(dir, remoteName string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	// Split "origin/feature/xyz" → remote="origin", local="feature/xyz"
	idx := strings.IndexByte(remoteName, '/')
	if idx < 0 {
		return fmt.Errorf("expected remote branch in '<remote>/<branch>' form, got %q", remoteName)
	}
	remotePart := remoteName[:idx]
	localName := remoteName[idx+1:]

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName(remotePart, localName), true)
	if err != nil {
		return fmt.Errorf("remote branch not found: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	return wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(localName),
		Hash:   remoteRef.Hash(),
		Create: true,
	})
}

// DeleteRemoteBranch deletes a branch on the "origin" remote by pushing a
// delete refspec (equivalent to git push origin --delete <branch>).
// branch must be the short local name (e.g. "feature/xyz", not "origin/feature/xyz").
func DeleteRemoteBranch(ctx context.Context, dir, branch, token string) error {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	remote, err := repo.Remote("origin")
	if err != nil || remote == nil {
		return fmt.Errorf("no remote 'origin' configured")
	}
	remoteURL := ""
	if urls := remote.Config().URLs; len(urls) > 0 {
		remoteURL = urls[0]
	}

	pushOpts := &gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(":refs/heads/" + branch)},
		Auth:       resolveAuth(normaliseHTTPS(remoteURL), "oauth", token),
	}
	if remoteURL != "" {
		pushOpts.RemoteURL = normaliseHTTPS(remoteURL)
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("delete remote branch: %w", err)
	}
	return nil
}

// DeleteBranch deletes a local branch by name.
// Returns an error if the branch is currently checked out.
func DeleteBranch(dir, name string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	head, err := repo.Head()
	if err == nil && head.Name().Short() == name {
		return fmt.Errorf("cannot delete the currently checked-out branch")
	}

	if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(name)); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	return nil
}

// MergeBranch merges sourceBranch into the current branch of the repository at dir.
func MergeBranch(dir, sourceBranch string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	srcRef, err := repo.Reference(plumbing.NewBranchReferenceName(sourceBranch), true)
	if err != nil {
		return fmt.Errorf("source branch %q not found: %w", sourceBranch, err)
	}

	err = repo.Merge(*srcRef, gogit.MergeOptions{
		Strategy: gogit.FastForwardMerge,
	})
	if err != nil {
		return fmt.Errorf("git merge: %w", err)
	}
	return nil
}

// ResetHard discards all uncommitted working-tree changes,
// resetting the worktree to the HEAD commit (git reset --hard HEAD).
func ResetHard(dir string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("no commits yet: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	return wt.Reset(&gogit.ResetOptions{
		Commit: head.Hash(),
		Mode:   gogit.HardReset,
	})
}

// UpdateRemoteURL sets or replaces the "origin" remote URL for the repo at dir.
func UpdateRemoteURL(dir, remoteURL string) error {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	normalised := normaliseHTTPS(remoteURL)
	_ = repo.DeleteRemote("origin")
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{normalised},
	})
	return err
}

// PushBranch pushes the given branch to "origin" without staging or committing.
func PushBranch(ctx context.Context, dir, branch, token string) error {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	remote, err := repo.Remote("origin")
	if err != nil || remote == nil {
		return fmt.Errorf("no remote 'origin' configured")
	}
	remoteURL := ""
	if urls := remote.Config().URLs; len(urls) > 0 {
		remoteURL = urls[0]
	}

	pushOpts := &gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))},
		Auth:       resolveAuth(normaliseHTTPS(remoteURL), "oauth", token),
	}
	if remoteURL != "" {
		pushOpts.RemoteURL = normaliseHTTPS(remoteURL)
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// GetHeadFileContent returns the content of filePath as it exists in the HEAD
// commit of the repository. Returns an empty string (no error) when the file
// is not present in HEAD (i.e. newly added, not yet committed).
func GetHeadFileContent(filePath string) (string, error) {
	repo, err := gogit.PlainOpenWithOptions(filepath.Dir(filePath), &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", nil // not in a repo — treat as new file
	}

	head, err := repo.Head()
	if err != nil {
		return "", nil // no commits yet
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", nil
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", nil
	}

	// Determine the repository root so we can compute a relative path.
	// go-git opens the repo at the .git parent; we need the worktree root.
	wt, err := repo.Worktree()
	if err != nil {
		return "", nil
	}
	repoRoot := wt.Filesystem.Root()

	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", nil
	}
	relPath, err := filepath.Rel(repoRoot, absFile)
	if err != nil {
		return "", nil
	}
	// go-git uses forward slashes internally.
	relPath = filepath.ToSlash(relPath)

	f, err := tree.File(relPath)
	if err != nil {
		return "", nil // file not tracked in HEAD — it's a new file
	}

	contents, err := f.Contents()
	if err != nil {
		return "", fmt.Errorf("read HEAD file: %w", err)
	}
	return contents, nil
}
