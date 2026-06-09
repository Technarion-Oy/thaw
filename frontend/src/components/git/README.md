# frontend/src/components/git

> Local git panel for the DDL export directory: status summary, commit & push to GitHub, branch management, clone/init, and GitHub OAuth.

## Responsibility

Provides the sidebar Git panel and the full-featured Git Operations modal. Operates on
the **local filesystem** git repository at the user-selected export directory. All git
operations are delegated to `internal/gitrepo` via `gitStore`. This folder is
distinct from `gitrepoobj/`, which manages Snowflake-native GIT REPOSITORY objects.

## Files

| File | Purpose |
|---|---|
| `GitPanel.tsx` | Collapsible sidebar section (Ant Design `Collapse`). Displays current branch name and ahead-commit count in the header, a changed-file badge, the export directory path, a Refresh button, and a "Git Operations…" button that opens `GitOperationsDialog` via `gitStore.openGitOps()`. Loads config on first render via `gitStore.loadConfig()`. |
| `GitOperationsDialog.tsx` | Full-featured modal (620 px wide, `destroyOnClose: false`) split into four sections rendered as a vertical column: **Repository** (local path picker, remote URL display and edit, clone form, or empty-repo init form), **GitHub Authentication** (OAuth token — stored in memory only), **Working Tree** (virtualised changed-file list with select-all/none/by-extension, commit message textarea, Commit & Push button, Discard Changes popconfirm), and **Branches** (local + remote branch list with Switch, Merge, Push, Pull, Checkout, Delete per branch; Create branch input). Only shows Working Tree and Branches sections when `status.isRepo` is true. |

## Patterns & integration

- **State**: both components consume `useGitStore` exclusively. The store owns `exportDir`, `status`, `branches`, `oauthToken`, `pushing`, `cloning`, `resetting`, and all action methods (`push`, `clone`, `resetHard`, `checkoutBranch`, `pushBranch`, etc.).
- **IPC**: `GitPanel` has no direct IPC calls. `GitOperationsDialog` calls `PickDirectory` and `GitInitWithRemote` directly (the rest go through `gitStore` actions).
- **Virtual file list**: `VirtualFileList` inside `GitOperationsDialog` is a custom windowed list (row height 24 px, visible window 200 px, 8-row scroll buffer) built without a third-party virtualisation library.
- **Gutter decorations**: git line-diff gutter decorations in `SqlEditor` use `sqleditor.Service.ComputeGitLineDiff` on the backend — that path does not go through this folder.
- **thaw:domain annotation**: `GitOperationsDialog.tsx` carries `// @thaw-domain: Git Integration`.

## Gotchas

- Only GitHub remote URLs are accepted (validated by `isGithubURL()`). The clone and remote-URL inputs reject non-GitHub URLs with an inline error message.
- When `clone()` fails because the remote repository is empty, the store error is cleared and the UI switches to an "init mode" form (`GitInitWithRemote` IPC) rather than showing an error.
- The OAuth token is **memory-only** — it is not persisted and is lost on app restart. Push and Delete-remote buttons are disabled when `oauthToken` is absent.
- `destroyOnClose: false` on `GitOperationsDialog` means the modal's internal state (commit message, clone URL, branch name input) survives close/reopen within the same session.
