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
| `repo.go` | Core operations: `GetStatus`, `CommitAndPush`, `Pull`, `Fetch`, `Clone`, `InitWithRemote`, `ListBranches`, `CheckoutBranch`, `CheckoutRemoteBranch`, `CreateBranch`, `DeleteBranch`, `DeleteRemoteBranch`, `MergeBranch`, `ResetHard`, `UpdateRemoteURL`, `PushBranch`, `GetHeadFileContent`. |
| `credentials.go` | `resolveAuth` (selects go-git `AuthMethod`), `LookupCredentials` (IPC-safe, no secrets), `lookupStoredCredentials` (priority: `~/.git-credentials` → `~/.netrc` → OS keychain). |
| `credentials_darwin.go` / `_windows.go` / `_other.go` | Platform-specific OS keychain lookups. |
| `oauth.go` | `PerformOAuthFlow` (loopback redirect on `127.0.0.1:3456`), `GetProviderConfig` (GitHub / GitLab), `exchangeCodeForToken`. |

## Key types & functions

| Symbol | Description |
|---|---|
| `RepoStatus` | `{ isRepo, branch, modified, added, deleted, hasRemote, remoteURL, ahead, totalChanged }` |
| `PushParams` | All fields needed for `CommitAndPush`: dir, remoteURL, branch, authMethod, token, message, author, files. |
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
- File status is capped at `maxStatusFiles = 500` paths per category; `TotalChanged` always reflects the exact count.

## Gotchas

- Clone of an empty remote repository (no commits) returns a user-friendly error and cleans up the partial `.git` directory so the user can retry.
- `MergeBranch` only supports fast-forward merges (`gogit.FastForwardMerge`). Conflict resolution is not supported.
- The OAuth loopback server listens on a fixed port `127.0.0.1:3456`. If that port is occupied, the flow will fail.
- GitHub does not accept the `Authorization: Bearer` header for Git-over-HTTPS; the code special-cases `github.com` to always use `BasicAuth` even for OAuth tokens.
