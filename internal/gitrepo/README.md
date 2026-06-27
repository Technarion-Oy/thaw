# internal/gitrepo

> Local Git repository operations (clone, status, commit/push, pull, branch management) via the go-git library, with OAuth flow support.

## Responsibility

Implements all local git operations used by the DDL export workflow and the file-browser
git panel. Uses `github.com/go-git/go-git/v5` (pure Go, no system `git` binary required).
Handles credential resolution across multiple sources (PAT, OAuth bearer, OS keychain,
`~/.git-credentials`, `~/.netrc`) and normalises SSH remote URLs to HTTPS for
token-based authentication.

## Key files

| File | Purpose |
|---|---|
| `repo.go` | Core operations: `GetStatus`, `CommitAndPush`, `StageFile`, `UnstageFile`, `StageAll`, `UnstageAll`, `DiscardFile`, `Pull`, `Fetch`, `Clone`, `InitWithRemote`, `ListBranches`, `CheckoutBranch`, `CheckoutRemoteBranch`, `CreateBranch`, `DeleteBranch`, `DeleteRemoteBranch`, `MergeBranch`, `ResetHard`, `UpdateRemoteURL`, `PushBranch`, `GetHeadFileContent`. |
| `credentials.go` | `resolveAuth` (selects go-git `AuthMethod`), `LookupCredentials` (IPC-safe, no secrets), `lookupStoredCredentials` (priority: `~/.git-credentials` → `~/.netrc` → OS keychain). |
| `credentials_darwin.go` / `_windows.go` / `_other.go` | Platform-specific OS keychain lookups. |
| `oauth.go` | `PerformOAuthFlow` (loopback redirect on `127.0.0.1:3456`), `GetProviderConfig` (GitHub / GitLab), `exchangeCodeForToken`. |

## Key types & functions

| Symbol | Description |
|---|---|
| `RepoStatus` | `{ isRepo, branch, modified, added, deleted, staged[], unstaged[], stagedTotal, unstagedTotal, changedPaths, hasRemote, remoteURL, ahead, totalChanged }`. `staged`/`unstaged` are `FileChange` lists distinguishing the index side (git add'ed) from the worktree side; a partially-staged file appears in both. `changedPaths` is an **uncapped** `map[path]letter` (worktree status wins, else staging) used by the file-explorer color overlay so the whole tree can be colored even when the capped arrays can't cover a huge change set. The flat `modified/added/deleted` arrays are kept as a legacy union. |
| `FileChange` | `{ path, status }` — `status` is a single VS Code-style letter: `A` added, `M` modified, `D` deleted, `R` renamed, `C` copied, `U` untracked. |
| `PushParams` | All fields needed for `CommitAndPush`: dir, remoteURL, branch, authMethod, token, message, author, files, `stagedOnly`, `noPush`. When `stagedOnly` is set, the commit runs over the existing index (nothing is staged first); when `noPush` is set, it commits locally and returns without touching the remote (no auth/remote required). |
| `StageFile` / `UnstageFile` / `StageAll` / `UnstageAll` | Manipulate the git index (whole-file granularity — go-git has no `git add -p`). `UnstageFile` uses a path-constrained mixed reset so the index stat-cache is refreshed and status re-detects the file. **Perf:** `StageAll` does one bulk `AddOptions{All:true}` (single status scan + single index write) — never a per-file loop, which is O(N²) because go-git rewrites the whole index and re-scans status on every `Add`. `StageFile` sets `AddOptions{SkipStatus:true}` so staging one file doesn't trigger a full-repo status scan. |
| `DiscardFile(dir, file)` | Reverts a tracked file to its HEAD content (and unstages it); deletes an untracked/newly-added file from disk and the index. Cannot be undone. |
| `PullParams` | Dir, remoteURL, branch, authMethod, token. |
| `CloneParams` | URL, path, authMethod, token. |
| `BranchInfo` | `{ name, isRemote, isCurrent }` |
| `CredentialResult` | IPC-safe: `{ found, username, source }` — never includes the secret. |
| `GetStatus(dir)` | Returns `RepoStatus`; non-repos return `IsRepo: false` without error. |
| `CommitAndPush(ctx, p)` | Init-if-needed → ensure `.gitignore` → stage → commit → push. "Nothing to commit" and "already up-to-date" are success. |
| `GetHeadFileContent(filePath)` | Returns file content at HEAD; returns `""` (no error) for untracked files or repos with no commits. |
| `PerformOAuthFlow(ctx, provider)` | Opens browser, runs loopback callback server, exchanges code for token. |

## Patterns & integration

- IPC delegators live in `internal/app/git.go`; all logic stays in this package.
- `resolveAuth` is the central credential dispatcher: `"bearer"/"oauth"` → `TokenAuth` (or `BasicAuth` for github.com), `"stored"` → OS keychain lookup, `"pat"/""` → `BasicAuth{x-access-token, token}`.
- SSH remote URLs are automatically normalised to HTTPS via `normaliseHTTPS` before any token-based push/pull, since PAT/OAuth credentials only apply to the HTTPS transport.
- `ensureGitignore` is called before every commit to keep `.DS_Store`, `Thumbs.db`, and `desktop.ini` out of the repository.
- File status is capped at `maxStatusFiles = 500` paths per category; `TotalChanged` / `StagedTotal` / `UnstagedTotal` always reflect the exact counts so the UI can paginate honestly.
- Per-hunk staging is **not** supported — go-git has no `git add -p` and partial blob/index construction was deferred (see issue #563 #4). Staging is whole-file only.

## Gotchas

- Clone of an empty remote repository (no commits) returns a user-friendly error and cleans up the partial `.git` directory so the user can retry.
- `MergeBranch` only supports fast-forward merges (`gogit.FastForwardMerge`). Conflict resolution is not supported.
- The OAuth loopback server listens on a fixed port `127.0.0.1:3456`. If that port is occupied, the flow will fail.
- GitHub does not accept the `Authorization: Bearer` header for Git-over-HTTPS; the code special-cases `github.com` to always use `BasicAuth` even for OAuth tokens.
