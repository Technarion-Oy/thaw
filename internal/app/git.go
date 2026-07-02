// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"context"

	"thaw/internal/config"
	"thaw/internal/gitrepo"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetGitConfig returns the persisted git / export settings.
//
// In an "Open Folder in New Window" instance (workdirOverridden), the per-repository
// fields are corrected so this window operates on its own folder, not the shared
// config's project: ExportDir is this window's folder, and RemoteURL/Branch are
// blanked so git operations fall back to the live status of the actual repo at that
// folder — otherwise a pull/push here would target (and CommitAndPush would even
// repoint) the main window's origin. Author/ExportPathTemplate/RecentDirs are
// genuinely shared (global identity/prefs) and pass through unchanged.
func (a *App) GetGitConfig() (config.GitConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.GitConfig{}, err
	}
	if a.workdirOverridden {
		cfg.Git.ExportDir = a.currentWorkdir()
		cfg.Git.RemoteURL = ""
		cfg.Git.Branch = ""
	}
	return cfg.Git, nil
}

// SaveGitConfig persists git / export settings to disk.
// The token field is intentionally absent — it must never be written.
//
// RecentDirs is owned exclusively by AddRecentDir/ClearRecentDirs (atomic
// read-modify-write), so it is always preserved from disk here regardless of what
// the caller sent — a whole-struct write of a stale snapshot must never drop the
// other window's recent entries.
func (a *App) SaveGitConfig(gitCfg config.GitConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gitCfg.RecentDirs = cfg.Git.RecentDirs
	// An override window's snapshot of the per-repo/instance-local fields (ExportDir,
	// RemoteURL, Branch) belongs to a different repo, so preserve the on-disk values
	// and only persist the genuinely-shared prefs it may have edited (author, export
	// path template). This tracks its own folder in memory without clobbering the
	// main window's remote/branch/export dir.
	if a.workdirOverridden {
		a.setExportDir(gitCfg.ExportDir)
		cfg.Git.AuthorName = gitCfg.AuthorName
		cfg.Git.AuthorEmail = gitCfg.AuthorEmail
		cfg.Git.ExportPathTemplate = gitCfg.ExportPathTemplate
		return config.Save(cfg)
	}
	cfg.Git = gitCfg
	a.setExportDir(gitCfg.ExportDir)
	return config.Save(cfg)
}

// AddRecentDir prepends dir to the shared recent-folders list (newest first,
// deduped, capped) with an atomic read-modify-write, so concurrent updates from
// another window aren't lost to a stale snapshot. Returns the updated list.
func (a *App) AddRecentDir(dir string) ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	cfg.Git.RecentDirs = unionRecentDirs([]string{dir}, cfg.Git.RecentDirs)
	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	return cfg.Git.RecentDirs, nil
}

// ClearRecentDirs empties the shared recent-folders list.
func (a *App) ClearRecentDirs() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Git.RecentDirs = nil
	return config.Save(cfg)
}

// unionRecentDirs concatenates primary then extra, dropping empties and
// duplicates (first occurrence wins) and capping the result. primary carries the
// caller's newest entry at the front; extra backfills anything only on disk so a
// concurrent write from another window isn't silently dropped.
func unionRecentDirs(primary, extra []string) []string {
	const maxRecent = 8
	seen := make(map[string]bool, len(primary)+len(extra))
	out := make([]string, 0, maxRecent)
	for _, list := range [][]string{primary, extra} {
		for _, d := range list {
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			if out = append(out, d); len(out) == maxRecent {
				return out
			}
		}
	}
	return out
}

// GitStatus returns the git status for the given directory.
// Safe to call on any directory — non-repos return IsRepo=false without error.
func (a *App) GitStatus(dir string) (gitrepo.RepoStatus, error) {
	return gitrepo.GetStatus(dir)
}

// GitCommitAndPush stages all changes, commits, and pushes to the remote.
// The Token field is used only in-memory for the push URL and is never persisted.
func (a *App) GitCommitAndPush(params gitrepo.PushParams) error {
	return gitrepo.CommitAndPush(a.ctx, params)
}

// GitStageFile stages a single file in the working tree (git add <file>).
func (a *App) GitStageFile(dir string, file string) error {
	return gitrepo.StageFile(dir, file)
}

// GitUnstageFile removes a file from the index, restoring it to HEAD (git reset HEAD -- <file>).
func (a *App) GitUnstageFile(dir string, file string) error {
	return gitrepo.UnstageFile(dir, file)
}

// GitStageAll stages every working-tree change (git add -A), skipping OS junk files.
func (a *App) GitStageAll(dir string) error {
	return gitrepo.StageAll(dir)
}

// GitUnstageAll resets the whole index to HEAD, leaving the working tree untouched.
func (a *App) GitUnstageAll(dir string) error {
	return gitrepo.UnstageAll(dir)
}

// GitDiscardFile reverts a file to its HEAD state (tracked) or deletes it (untracked). Cannot be undone.
func (a *App) GitDiscardFile(dir string, file string) error {
	return gitrepo.DiscardFile(dir, file)
}

// GitPull fetches and merges changes from the remote branch.
// The Token field is used only in-memory for the pull URL and is never persisted.
func (a *App) GitPull(params gitrepo.PullParams) error {
	return gitrepo.Pull(a.ctx, params)
}

// GitClone clones a remote repository into the given local path.
// The Token field is used only in-memory and is never persisted.
func (a *App) GitClone(params gitrepo.CloneParams) error {
	return gitrepo.Clone(a.ctx, params)
}

// GitInitWithRemote initializes a git repository at dir (creating it if
// necessary), sets origin to remoteURL, and configures the default branch.
// The repo is left empty — ready for the user's first commit and push.
func (a *App) GitInitWithRemote(dir string, remoteURL string, branch string) error {
	return gitrepo.InitWithRemote(dir, remoteURL, branch)
}

// GitListBranches returns all local and remote branches for the repository at dir.
func (a *App) GitListBranches(dir string) ([]gitrepo.BranchInfo, error) {
	return gitrepo.ListBranches(dir)
}

// GitCheckoutBranch checks out an existing local branch in the repository at dir.
func (a *App) GitCheckoutBranch(dir string, branchName string) error {
	return gitrepo.CheckoutBranch(dir, branchName)
}

// GitCreateBranch creates and checks out a new branch in the repository at dir.
func (a *App) GitCreateBranch(dir string, branchName string) error {
	return gitrepo.CreateBranch(dir, branchName)
}

// GitGetHeadFileContent returns the content of filePath as stored in the HEAD commit.
// Returns an empty string (no error) when the file is not yet tracked in HEAD.
func (a *App) GitGetHeadFileContent(filePath string) (string, error) {
	return gitrepo.GetHeadFileContent(filePath)
}

// GitLookupCredentials probes OS credential stores (keychain, credential manager,
// ~/.git-credentials, ~/.netrc) for the given remote URL.
// The result never contains the secret — only discovery metadata safe for the UI.
func (a *App) GitLookupCredentials(remoteURL string) (gitrepo.CredentialResult, error) {
	return gitrepo.LookupCredentials(remoteURL), nil
}

// GitFetch updates all remote-tracking refs from "origin".
// Pass the OAuth token so private repos are accessible; empty token tries stored credentials.
func (a *App) GitFetch(dir string, token string) error {
	return gitrepo.Fetch(a.ctx, dir, token)
}

// GitDeleteRemoteBranch deletes a branch on the "origin" remote (git push origin --delete <branch>).
// branch is the short name without the "origin/" prefix.
func (a *App) GitDeleteRemoteBranch(dir string, branch string, token string) error {
	return gitrepo.DeleteRemoteBranch(a.ctx, dir, branch, token)
}

// GitCheckoutRemoteBranch creates a local branch from a remote-tracking ref and checks it out.
// remoteName must be in "origin/<branch>" form (as returned by GitListBranches).
func (a *App) GitCheckoutRemoteBranch(dir string, remoteName string) error {
	return gitrepo.CheckoutRemoteBranch(dir, remoteName)
}

// GitDeleteBranch deletes a local branch. Returns an error if the branch is currently checked out.
func (a *App) GitDeleteBranch(dir string, branchName string) error {
	return gitrepo.DeleteBranch(dir, branchName)
}

// GitMergeBranch merges sourceBranch into the current branch in the repository at dir.
func (a *App) GitMergeBranch(dir string, sourceBranch string) error {
	return gitrepo.MergeBranch(dir, sourceBranch)
}

// GitResetHard discards all uncommitted changes, resetting the worktree to HEAD.
func (a *App) GitResetHard(dir string) error {
	return gitrepo.ResetHard(dir)
}

// GitUpdateRemoteURL sets or replaces the "origin" remote URL for the repository at dir.
func (a *App) GitUpdateRemoteURL(dir string, remoteURL string) error {
	return gitrepo.UpdateRemoteURL(dir, remoteURL)
}

// GitPushBranch pushes the given branch to "origin" without staging or committing.
func (a *App) GitPushBranch(dir string, branch string, token string) error {
	return gitrepo.PushBranch(a.ctx, dir, branch, token)
}

// GitLoginWithOAuth starts the local loopback OAuth flow for the specified provider
// ("github", "gitlab", etc.) and returns the obtained access token.
//
// Rather than opening a browser, it emits the authorization URL via the
// "git:oauth-url" event so the frontend can let the user open it in their chosen
// browser or copy it (useful when the default browser is signed into a different
// account). The loopback callback still completes the flow.
func (a *App) GitLoginWithOAuth(provider string) (string, error) {
	// Per-flow cancelable context so GitCancelOAuth (dialog dismiss) can unblock the
	// loopback wait and trigger the deferred server shutdown — otherwise the
	// goroutine + port 3456 leak until app quit. Abort any prior in-flight flow
	// first so reopening the dialog can't double-bind the port.
	a.oauthMu.Lock()
	if a.oauthCancel != nil {
		a.oauthCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.oauthCancel = cancel
	a.oauthMu.Unlock()
	defer cancel() // free our flow's server/goroutine on return (idempotent)

	return gitrepo.PerformOAuthFlow(ctx, provider, func(authURL string) {
		wailsruntime.EventsEmit(a.ctx, "git:oauth-url", authURL)
	})
}

// GitCancelOAuth aborts an in-flight OAuth flow (called when the user closes the
// auth dialog without completing it), freeing the loopback server + port 3456.
func (a *App) GitCancelOAuth() {
	a.oauthMu.Lock()
	defer a.oauthMu.Unlock()
	if a.oauthCancel != nil {
		a.oauthCancel()
		a.oauthCancel = nil
	}
}
