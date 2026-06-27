# frontend/src/components/git

> Local git for the DDL export directory: an embedded Source-Control view in the file explorer, plus a full-featured Git Operations modal (commit & push, branches, clone/init, GitHub OAuth).

## Responsibility

Provides the git UI: the full-featured Git Operations modal. There is **no standalone
Git panel** and **no embedded changes list** — git was folded into the Files panel, which
shows working-tree status purely through **tree color-coding** (see `components/files/`).
The Files header carries a branch chip + Git Operations button that open this modal.
Operates on the **local filesystem** git repository at the user-selected export directory;
all git operations are delegated to `internal/gitrepo` via `gitStore`. This folder is
distinct from `gitrepoobj/`, which manages Snowflake-native GIT REPOSITORY objects.

## Files

| File | Purpose |
|---|---|
| `GitOperationsDialog.tsx` | Full-featured modal (620 px wide, `destroyOnClose: false`) split into a vertical column: **Repository** (local path picker, remote URL display and edit, clone form, or empty-repo init form), **GitHub Authentication** (OAuth token — stored in memory only), **Changes** (the `ChangesView` — see below), and **Branches** (local + remote branch list with Switch, Merge, Push, Pull, Checkout, Delete per branch; Create branch input). Only shows Changes and Branches sections when `status.isRepo` is true. Opened from the Files panel header (branch chip / Git Operations button) or the **Git** menu (`⌘G`). |
| `ChangesView.tsx` | VS Code-style Source Control view inside the dialog. Two collapsible, **paginated** groups — **Staged changes** and **Changes** — of `ChangeRow`s. The header carries the global actions: **Stage all** (`git add -A`), **Unstage all** (reset the index, keep edits), and **Reset to commit** (`git reset --hard` — discards all working-tree changes, behind a Popconfirm). A commit summary box commits the **staged set** (`gitStore.commitStaged` → `stagedOnly` push). The paginator surfaces the backend's 500-file cap honestly. |
| `ChangeRow.tsx` | The signature row. Reads like a typeset `git status` line: colored status spine + mono sigil, the path (directory prefix as faint mono data, filename as the object in Inter), the Snowflake object-type ledger column in its `--icon-*` color, and reveal-on-hover/focus Stage/Unstage + Discard actions overlaid so the resting row stays tight. |
| `gitStatusUtil.ts` | Shared helpers reused by `ChangeRow` and the `FileBrowser` tree: `sigilColor` (status letter → theme token), `objectTypeFromPath` (derive Snowflake object type from the export path's `{object_type}` directory), `splitPath`. |

## Patterns & integration

- **State**: all components consume `useGitStore` exclusively. The store owns `exportDir`, `status` (incl. `staged`/`unstaged` `FileChange` lists), `branches`, `oauthToken`, `cloning`, `resetting`, `staging`, `committing`, and all action methods (`stageFile`, `unstageFile`, `stageAll`, `unstageAll`, `discardFile`, `commitStaged`, `clone`, `resetHard`, branch actions, etc.). `FileBrowser` owns calling `gitStore.loadConfig()` on first mount (it used to be the Git panel's job).
- **IPC**: `GitOperationsDialog` calls `PickDirectory` and `GitInitWithRemote` directly (the rest go through `gitStore` actions).
- **Staging model**: commit operates on the real git index. Files are staged/unstaged individually (from the `FileBrowser` context menu) or all at once (the dialog header), and `commitStaged` runs a `stagedOnly` push that commits whatever is in the index. Staging is whole-file (no per-hunk — see `internal/gitrepo` gotchas).
- **Unstage vs reset**: **Unstage all** (`gitStore.unstageAll` → `git reset`) clears the index but keeps the file edits; **Reset to commit** (`gitStore.resetHard` → `git reset --hard`) discards every working-tree change. These are deliberately separate, distinctly labeled actions so "discard" is never ambiguous.
- **Where status is shown**: the file explorer reflects status only through tree color-coding (no embedded list); the dialog's `ChangesView` is the full control surface.
- **Gutter decorations**: git line-diff gutter decorations in `SqlEditor` use `sqleditor.Service.ComputeGitLineDiff` on the backend — that path does not go through this folder. (Interactive per-hunk gutter staging is deferred — issue #563 #4.)
- **thaw:domain annotation**: `GitOperationsDialog.tsx`, `ChangesView.tsx`, `ChangeRow.tsx`, and `gitStatusUtil.ts` carry `// @thaw-domain: Git Integration`.

## Gotchas

- Only GitHub remote URLs are accepted (validated by `isGithubURL()`). The clone and remote-URL inputs reject non-GitHub URLs with an inline error message.
- When `clone()` fails because the remote repository is empty, the store error is cleared and the UI switches to an "init mode" form (`GitInitWithRemote` IPC) rather than showing an error.
- The OAuth token is **memory-only** — it is not persisted and is lost on app restart. Push and Delete-remote buttons are disabled when `oauthToken` is absent.
- `destroyOnClose: false` on `GitOperationsDialog` means the modal's internal state (commit message, clone URL, branch name input) survives close/reopen within the same session.
