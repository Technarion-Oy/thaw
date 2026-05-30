// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"thaw/internal/config"
	"thaw/internal/gitrepo"
)

// GetGitConfig returns the persisted git / export settings.
func (a *App) GetGitConfig() (config.GitConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.GitConfig{}, err
	}
	return cfg.Git, nil
}

// SaveGitConfig persists git / export settings to disk.
// The token field is intentionally absent — it must never be written.
func (a *App) SaveGitConfig(gitCfg config.GitConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Git = gitCfg
	a.setExportDir(gitCfg.ExportDir)
	return config.Save(cfg)
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
func (a *App) GitLoginWithOAuth(provider string) (string, error) {
	return gitrepo.PerformOAuthFlow(a.ctx, provider)
}
