# frontend/src/components/files

> Local file browser with an Ant Design tree, full CRUD operations, content search, diff comparison, and a FS watcher integration.

## Responsibility

`FileBrowser` renders a tree view of the local export directory. It supports lazy
directory expansion, inline rename and file/folder creation, content search, diff
comparison, reveal-in-file-manager, and clipboard copy of file paths. When the
`fileWatcher` feature flag is enabled it starts a backend FS watcher and incrementally
refreshes only the changed directory node on `fs:changed` events.

`platformUtil.ts` is a shared utility that resolves and caches the host OS string for
platform-specific labels.

## Files

| File | Purpose |
|---|---|
| `FileBrowser.tsx` | Main file browser component. Renders an Ant Design `Tree` with lazy `onLoadData` directory expansion. Implements tree-level helpers: `entriesToNodes`, `mergeNodes` (preserves expanded children), `updateNode`, `removeNode`, `renameTreeNode` / `reKeyChildren`, `addChild` (dirs-first alphabetical insert). Context menu per node: Open in editor tab, Copy path, Rename (inline Input with Enter/Escape), New File, New Folder, Duplicate, Delete, Reveal in Finder, Diff with another file. Inline search mode (`SearchFiles` IPC) shows grouped match results with highlighted snippets. FS watcher: starts `StartFileWatcher(exportDir)` when `exportDir` changes (gated by `fileWatcher` flag); listens for `fs:changed` events and refreshes the affected directory node; suppresses self-change flicker via a `selfChangedDirs` Set with 500 ms timeout. |
| `platformUtil.ts` | Module-level singleton cache for the host OS string (fetched once via `GetPlatformOS` IPC, retried on failure). Exports `getPlatformOS()` (async, caching), `getCachedPlatformOS()` (synchronous, nullable), and `revealLabel(os)` ("Reveal in Finder" / "Show in Explorer" / "Show in File Manager"). Eagerly fetches on module load. |

## Patterns & integration

- **IPC**: `ListDirectory`, `ReadFile`, `SearchFiles`, `RevealInFinder`, `DeleteFile`, `DeleteDirectory`, `RenameFile`, `CreateDirectory`, `CreateFile`, `DuplicateFile`, `StartFileWatcher`, `StopFileWatcher`, `GetPlatformOS` — all from `wailsjs/go/app/App`.
- **Events**: subscribes to `fs:changed` Wails events (`EventsOn`); payload is `{ dir: string }`. On receipt, calls `ListDirectory(dir)` and calls `mergeNodes` on the affected subtree to preserve expanded state.
- **Stores**: reads `gitStore.exportDir` (directory root), `queryStore` (to open files as new SQL tabs), `diffStore` (to queue files for DDL diff), `featureFlagsStore` (to gate FS watcher).
- **Feature flag**: `fileWatcher` flag gates `StartFileWatcher` / `StopFileWatcher` calls and the `fs:changed` event listener. When the flag is off, the tree is still functional but does not auto-refresh on external changes.
- **Self-change suppression**: mutations performed by `FileBrowser` itself (create, rename, delete, duplicate) record the parent directory in `selfChangedDirs` for 500 ms; incoming `fs:changed` events for those directories are ignored during that window.

## Gotchas

- `mergeNodes` preserves existing `children` arrays only for nodes that are still directories. If an external tool replaces a directory with a file of the same name, stale children are dropped.
- Watcher is stopped (`StopFileWatcher`) on component unmount and whenever `exportDir` changes to a new value; a new watcher is started after the old one is stopped.
- `addChild` does nothing when the parent node has no `children` array yet (not expanded) — the new node will appear naturally on expansion. This is intentional to avoid forcing an expansion.
- `renameTreeNode` recursively re-keys all descendants so that the tree keys (which equal file paths) remain correct after a parent directory is renamed.
