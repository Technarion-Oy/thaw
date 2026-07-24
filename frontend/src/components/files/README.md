# frontend/src/components/files

> Local file browser with an Ant Design tree, full CRUD operations, content search, diff comparison, and a FS watcher integration.

## Responsibility

`FileBrowser` renders a tree view of the local export directory. It supports lazy
directory expansion, inline rename and file/folder creation, content search, diff
comparison, reveal-in-file-manager, clipboard copy of file paths (absolute **and**
project-root-relative), **multi-select** (Cmd/Ctrl+click toggle, Shift+click range),
an **internal cut/copy/paste clipboard** (move via `RenameFile`, copy via `CopyFile`,
auto-resolving name conflicts), and **bulk** delete / cut / copy / git stage / unstage /
discard across the selection. When the `fileWatcher` feature flag is enabled it starts a
backend FS watcher and incrementally refreshes only the changed directory node on
`fs:changed` events.

**Root-level creation** is reachable without an existing folder: right-clicking the
**header title area** (top-left) or the **empty area** of the panel opens a minimal root
context menu (New Folder…, New SQL File…, Paste) targeting the **workspace root**. It
deliberately omits destructive actions on the root directory itself. (No toolbar buttons —
the header row is icon-dense already.)

The header also carries a **folder-switch button** (open-folder icon) whose dropdown offers
**Open Folder…** (native directory picker → `gitStore.pickExportDir`), **Open Folder in New
Window…** (`gitStore.openInNewWindow` → `OpenFolderInNewInstance` IPC — spawns a second Thaw
instance rooted at the folder), a **Recent** list of the last working directories
(`gitStore.recentDirs` → `openFolder`), and **Clear Recent**. This is the discoverable,
always-visible way to change the operating folder without opening Git Operations; the same
**Open Folder…** action is bound to **File → Open Folder…** (`⌘⇧O`) and **Open Folder in New
Window…** to its File-menu twin.

The header is laid out in **two rows** so a narrow sidebar never crushes the folder name:
row 1 is the folder title (caret + name) plus the action strip (paste, folder-switch,
search, refresh); row 2 is a dedicated **git status row** — a branch pill (with ↑ahead)
and a changed-file count pill, each opening Git Operations — shown only in a repo, where
the branch name finally has room to display in full. The standalone Git Operations icon
survives on row 1 **only for non-repo folders** (a repo's entry point is the row-2 pills),
so the action strip stays uncluttered in the common repo case.

The **git surface is folded into this panel** (there is no separate Git panel): the tree itself is
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
| `FileBrowser.tsx` | Main file browser component. Renders an Ant Design `Tree` with lazy `onLoadData` directory expansion. Implements tree-level helpers: `entriesToNodes`, `mergeNodes` (preserves expanded children), `updateNode`, `removeNode`, `renameTreeNode` / `reKeyChildren`, `addChild` (dirs-first alphabetical insert). **Git status overlay**: `gitOverlay` maps the repo-relative `gitStore.status` paths onto absolute tree keys to color-code changed files (VS Code sigil colors via `sigilColor`, with a trailing status letter) and emphasize directories containing nested changes. Context menu per node: Open in editor tab, Copy path, **Copy Relative Path** (relative to `exportDir`), **Cut / Copy / Paste** (internal clipboard — see below), Rename (inline Input with Enter/Escape), **Compare with last commit** (tracked changed files — diffs working tree vs HEAD via `GitGetHeadFileContent` + `queryStore.openDiff`), **Stage / Unstage / Discard changes** (changed files only), **Discard all changes (reset to last commit)** (repo-wide `git reset --hard`, shown whenever the repo has changes), New File, New Folder, Duplicate, Delete, Reveal in Finder, Diff with another file. **Multi-select**: `selKeys`/`anchorKey` state; Cmd/Ctrl+click toggles a node, Shift+click selects the range between the anchor and the click across the **visible** order (read from the DOM via `data-fbkey` on each rendered title — `visibleKeysInOrder()` — so it honors collapse state without controlling expansion). Right-clicking a node inside a multi-selection shows bulk variants (Cut/Copy/Delete/Stage/Unstage/Discard *N* items); right-clicking outside it collapses the selection to that node. **Internal clipboard** (`clipboard` state, never touches the OS text clipboard): Cut marks paths for move and dims them until pasted; Copy marks them for duplication; Paste appears on folder context menus and a header toolbar button when the clipboard is non-empty. Paste lists the target dir, resolves name conflicts (`_copy`, `_copy_2`, …) via `uniqueDstPath`, then moves (`RenameFile`, with a cross-volume `CopyFile`+delete fallback) or copies (`CopyFile`); open tabs are re-pointed via `remapTabsForMove`. **Preview tabs (#849, VS Code style):** a single click on a file (`onSelect`) opens it in the reusable *preview* tab via `openFileInTab(path, previewTabsEnabled)`; a **double-click** promotes it to a permanent tab. rc-tree has no double-click prop, so `onTreeDoubleClick` is bound on the tree wrapper and recovers the node key from the `data-fbkey` attribute (set in `titleRender`). Because rc-tree fires `onSelect` on *every* click (both clicks of a double-click) and the native `dblclick` fires on top, one open-and-pin gesture could otherwise trigger up to three concurrent `ReadFile`s for one file; two refs coalesce this — `openingPathsRef` skips a same-path open already in flight, and `pendingPromoteRef` lets a double-click that lands before the open resolves record its promote intent so `onSelect` promotes the tab (`queryStore.promoteTab`) once it exists, instead of firing its own extra read. When the tab already exists, `onTreeDoubleClick` promotes it directly. Search-result clicks open as preview too. The behavior is gated on `editorTabPrefsStore.previewTabsEnabled` (Editor Preferences toggle); when off, opens go straight to permanent tabs. Inline search mode (`SearchFiles` IPC) shows grouped match results with highlighted snippets. FS watcher: the `StartFileWatcher`/`StopFileWatcher` lifecycle lives in `QueryPage` (always mounted, so ⌘B-hiding the sidebar doesn't stop it); `FileBrowser` only *consumes* `fs:changed` events (gated by the `fileWatcher` flag) to refresh the affected directory node, and suppresses self-change flicker via a `selfChangedDirs` Set with 500 ms timeout — populated by its own mutations and by editor saves (`thaw:file-saved` carries the saved path, so the watcher's echo of our own write doesn't trigger a redundant re-list). |
| `platformUtil.ts` | Module-level singleton cache for the host OS string (fetched once via `GetPlatformOS` IPC, retried on failure). Exports `getPlatformOS()` (async, caching), `getCachedPlatformOS()` (synchronous, nullable), and `revealLabel(os)` ("Reveal in Finder" / "Show in Explorer" / "Show in File Manager"). Eagerly fetches on module load. |

## Patterns & integration

- **IPC**: `ListDirectory`, `ReadFile`, `SearchFiles`, `RevealInFinder`, `DeleteFile`, `DeleteDirectory`, `RenameFile`, `CopyFile`, `CreateDirectory`, `CreateFile`, `DuplicateFile`, `GetPlatformOS` — all from `wailsjs/go/app/App`. (`StartFileWatcher`/`StopFileWatcher` now live in `QueryPage`.)
- **Events**: subscribes to `fs:changed` Wails events (`EventsOn`); payload is `{ dir: string }`. On receipt, calls `ListDirectory(dir)` and calls `mergeNodes` on the affected subtree to preserve expanded state.
- **Live git colors**: a debounced `scheduleGitRefresh` (400 ms) re-runs `gitStore.refreshStatus()` so the tree's status colors update without a manual refresh. It fires on the `thaw:file-saved` DOM event (dispatched by `queryStore.markSaved` after every editor save — watcher-independent; its `detail.path` also marks the saved dir self-changed so the watcher echo skips the re-list) and at the top of the `fs:changed` handler (covering external edits and the app's own file ops, even when the tree update is self-change-suppressed).
- **Stores**: reads `gitStore.exportDir` (directory root) and `gitStore.status` (for the git-status overlay), plus `gitStore.stageFile`/`unstageFile`/`discardFile` actions; `queryStore` (to open files as new SQL tabs), `diffStore` (to queue files for DDL diff), `featureFlagsStore` (to gate FS watcher).
- **Feature flag**: `fileWatcher` flag gates the `fs:changed` event listener here (and the watcher lifecycle in `QueryPage`). When the flag is off, the tree is still functional but does not auto-refresh on external changes.
- **Self-change suppression**: mutations performed by `FileBrowser` itself (create, rename, delete, duplicate) — and editor saves, via the `thaw:file-saved` path — record the parent directory in `selfChangedDirs` for 500 ms; incoming `fs:changed` events for those directories are ignored during that window. The key is normalized through `suppressKey` (strips a leading `/private` on macOS auto-symlink roots) so a canonical dialog-save path matches the watcher's pre-resolution `evt.dir`.

## Gotchas

- `mergeNodes` preserves existing `children` arrays only for nodes that are still directories. If an external tool replaces a directory with a file of the same name, stale children are dropped.
- Watcher is stopped (`StopFileWatcher`) on component unmount and whenever `exportDir` changes to a new value; a new watcher is started after the old one is stopped.
- `addChild` does nothing when the parent node has no `children` array yet (not expanded) — the new node will appear naturally on expansion. This is intentional to avoid forcing an expansion.
- `renameTreeNode` recursively re-keys all descendants so that the tree keys (which equal file paths) remain correct after a parent directory is renamed.
- Shift+range relies on `data-fbkey` being present on **every** rendered node title; `titleRender` wraps all (non-editing) nodes in a `<span data-fbkey>`. Nodes scrolled out of the DOM are not in `visibleKeysInOrder()`, so a range whose anchor is off-screen falls back to a single selection.
- The internal clipboard is local component state (not a store) and is cleared when `exportDir` changes; Cut is one-shot (cleared after a successful paste), Copy persists. After paste, `refresh()` rebuilds affected directories rather than surgically inserting nodes.
