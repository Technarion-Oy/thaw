# frontend/src/components/files

> Local file browser with an Ant Design tree, full CRUD operations, content search, diff comparison, and a FS watcher integration.

## Responsibility

`FileBrowser` renders a tree view of the local export directory. It supports lazy
directory expansion, inline rename and file/folder creation, content search, diff
comparison, reveal-in-file-manager, and clipboard copy of file paths. When the
`fileWatcher` feature flag is enabled it starts a backend FS watcher and incrementally
refreshes only the changed directory node on `fs:changed` events.

The **git surface is folded into this panel** (there is no separate Git panel): the header
shows a branch chip + changed-file count + a Git Operations button, and the tree itself is
**color-coded by git status**. The `gitOverlay` memo builds its color map from the status's
**uncapped `changedPaths`** map (so the whole tree is covered even in huge change sets) and
matches it to absolute tree node keys via `relOf` — an **exact** export-dir prefix strip
(suffix matching was rejected: in large repos files that merely share a basename would
false-match). Files get a sigil + status color; folders take the color of the most
significant change beneath them (A/U both count as "new"/green, so an all-new folder stays
green rather than reading as modified). The capped
`staged`/`unstaged` lists drive the precise Stage/Unstage context menu. The full
staged/unstaged control lives in the Git Operations dialog. `FileBrowser` also owns calling
`gitStore.loadConfig()` and refreshing git status on first mount (idempotent), since the
former Git panel used to do that.

`platformUtil.ts` is a shared utility that resolves and caches the host OS string for
platform-specific labels.

## Files

| File | Purpose |
|---|---|
| `FileBrowser.tsx` | Main file browser component. Renders an Ant Design `Tree` with lazy `onLoadData` directory expansion. Implements tree-level helpers: `entriesToNodes`, `mergeNodes` (preserves expanded children), `updateNode`, `removeNode`, `renameTreeNode` / `reKeyChildren`, `addChild` (dirs-first alphabetical insert). **Git status overlay**: `gitOverlay` maps the repo-relative `gitStore.status` paths onto absolute tree keys to color-code changed files (VS Code sigil colors via `sigilColor`, with a trailing status letter) and emphasize directories containing nested changes. Context menu per node: Open in editor tab, Copy path, Rename (inline Input with Enter/Escape), **Compare with last commit** (tracked changed files — diffs working tree vs HEAD via `GitGetHeadFileContent` + `queryStore.openDiff`), **Stage / Unstage / Discard changes** (changed files only), New File, New Folder, Duplicate, Delete, Reveal in Finder, Diff with another file. Inline search mode (`SearchFiles` IPC) shows grouped match results with highlighted snippets. FS watcher: starts `StartFileWatcher(exportDir)` when `exportDir` changes (gated by `fileWatcher` flag); listens for `fs:changed` events and refreshes the affected directory node; suppresses self-change flicker via a `selfChangedDirs` Set with 500 ms timeout. |
| `platformUtil.ts` | Module-level singleton cache for the host OS string (fetched once via `GetPlatformOS` IPC, retried on failure). Exports `getPlatformOS()` (async, caching), `getCachedPlatformOS()` (synchronous, nullable), and `revealLabel(os)` ("Reveal in Finder" / "Show in Explorer" / "Show in File Manager"). Eagerly fetches on module load. |

## Patterns & integration

- **IPC**: `ListDirectory`, `ReadFile`, `SearchFiles`, `RevealInFinder`, `DeleteFile`, `DeleteDirectory`, `RenameFile`, `CreateDirectory`, `CreateFile`, `DuplicateFile`, `StartFileWatcher`, `StopFileWatcher`, `GetPlatformOS` — all from `wailsjs/go/app/App`.
- **Events**: subscribes to `fs:changed` Wails events (`EventsOn`); payload is `{ dir: string }`. On receipt, calls `ListDirectory(dir)` and calls `mergeNodes` on the affected subtree to preserve expanded state.
- **Stores**: reads `gitStore.exportDir` (directory root) and `gitStore.status` (for the git-status overlay), plus `gitStore.stageFile`/`unstageFile`/`discardFile` actions; `queryStore` (to open files as new SQL tabs), `diffStore` (to queue files for DDL diff), `featureFlagsStore` (to gate FS watcher).
- **Feature flag**: `fileWatcher` flag gates `StartFileWatcher` / `StopFileWatcher` calls and the `fs:changed` event listener. When the flag is off, the tree is still functional but does not auto-refresh on external changes.
- **Self-change suppression**: mutations performed by `FileBrowser` itself (create, rename, delete, duplicate) record the parent directory in `selfChangedDirs` for 500 ms; incoming `fs:changed` events for those directories are ignored during that window.

## Gotchas

- `mergeNodes` preserves existing `children` arrays only for nodes that are still directories. If an external tool replaces a directory with a file of the same name, stale children are dropped.
- Watcher is stopped (`StopFileWatcher`) on component unmount and whenever `exportDir` changes to a new value; a new watcher is started after the old one is stopped.
- `addChild` does nothing when the parent node has no `children` array yet (not expanded) — the new node will appear naturally on expansion. This is intentional to avoid forcing an expansion.
- `renameTreeNode` recursively re-keys all descendants so that the tree keys (which equal file paths) remain correct after a parent directory is renamed.
