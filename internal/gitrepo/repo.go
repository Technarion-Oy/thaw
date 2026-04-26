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
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// maxStatusFiles is the maximum number of file paths returned per category in
// RepoStatus. When the actual count exceeds this, the remainder is reflected
// only in TotalChanged so the UI can show an accurate badge without needing to
// render thousands of rows.
const maxStatusFiles = 500

// RepoStatus describes the current state of a git repository directory.
type RepoStatus struct {
	IsRepo       bool     `json:"isRepo"`
	Branch       string   `json:"branch"`
	Modified     []string `json:"modified"`
	Added        []string `json:"added"`
	Deleted      []string `json:"deleted"`
	HasRemote    bool     `json:"hasRemote"`
	RemoteURL    string   `json:"remoteURL"`
	Ahead        int      `json:"ahead"`
	TotalChanged int      `json:"totalChanged"` // exact total (may exceed len of arrays above)
}

// PushParams holds all parameters needed for a commit-and-push operation.
type PushParams struct {
	Dir         string   `json:"dir"`
	RemoteURL   string   `json:"remoteURL"`
	Branch      string   `json:"branch"`
	AuthMethod  string   `json:"authMethod"`  // "pat" | "bearer" | "stored" | ""
	Token       string   `json:"token"`
	Message     string   `json:"message"`
	AuthorName  string   `json:"authorName"`
	AuthorEmail string   `json:"authorEmail"`
	Files       []string `json:"files"` // if empty, stages all changes
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

// normaliseHTTPS converts git@github.com:org/repo.git to https form.
func normaliseHTTPS(remoteURL string) string {
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		return "https://github.com/" + strings.TrimPrefix(remoteURL, "git@github.com:")
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
			for path, fs := range st {
				x := fs.Staging
				y := fs.Worktree
				s.TotalChanged++
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

	// Set or update origin remote (plain URL — token injected only for push).
	if p.RemoteURL != "" {
		remoteURL := normaliseHTTPS(p.RemoteURL)
		existing, err := repo.Remote("origin")
		if err == nil {
			// Update URL if different.
			if len(existing.Config().URLs) == 0 || existing.Config().URLs[0] != remoteURL {
				_ = repo.DeleteRemote("origin")
				if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remoteURL}}); err != nil {
					return fmt.Errorf("set remote: %w", err)
				}
			}
		} else {
			if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remoteURL}}); err != nil {
				return fmt.Errorf("add remote: %w", err)
			}
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	// Stage specified files or everything.
	if len(p.Files) > 0 {
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
		if strings.Contains(err.Error(), "nothing to commit") ||
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
		Auth:       resolveAuth(normaliseHTTPS(p.RemoteURL), p.AuthMethod, p.Token),
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
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

	pullOpts := &gogit.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          resolveAuth(normaliseHTTPS(p.RemoteURL), p.AuthMethod, p.Token),
	}
	if p.RemoteURL != "" {
		pullOpts.RemoteURL = normaliseHTTPS(p.RemoteURL)
	}

	err = wt.PullContext(ctx, pullOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git pull: %w", err)
	}
	return nil
}

// Clone clones a remote repository into the given local path.
func Clone(ctx context.Context, p CloneParams) error {
	_, err := gogit.PlainCloneContext(ctx, p.Path, false, &gogit.CloneOptions{
		URL:      p.URL,
		Auth:     resolveAuth(p.URL, p.AuthMethod, p.Token),
		Progress: nil,
	})
	if err != nil {
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
