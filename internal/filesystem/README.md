# internal/filesystem

> File read/write/delete/rename helpers with path-containment guards, a recursive FS watcher, and file-content search.

## Responsibility

Provides all local-filesystem operations exposed to the Wails frontend through
`internal/app/filesystem.go`. Every mutating operation validates that the target path
stays inside an `allowedRoot` (symlink-resolved, case-aware on macOS/Windows) to
prevent path-traversal attacks. A separate `Watcher` component monitors a directory
tree for changes and emits debounced Wails events to refresh the file browser UI.

## Key files

| File | Purpose |
|---|---|
| `fs.go` | Core CRUD helpers: `ReadFile`, `ReadFileHead`, `WriteFile`, `ListDir`, `RevealInFinder`, `DeleteFile`, `DeleteDirectory`, `RenameFile`, `MkDir`, `WriteFileInRoot`, `DuplicateFile`, and the path-validation internals. |
| `watcher.go` | `Watcher` struct (`rjeczalik/notify`-based): a single recursive watch over the whole tree, 200 ms debounce per directory, `FSChangeEvent`. |
| `export.go` | `WriteBinaryFile` (base64-decode then write, used for Excel export) and `SanitizeFilename`. |
| `search.go` | `SearchFiles`: recursive file-content search (substring or regex), capped at 200 results. |

## Key types & functions

| Symbol | Description |
|---|---|
| `FileEntry` | `{ name, path, isDir, size }` — single directory entry returned by `ListDir`. |
| `FSChangeEvent` | `{ dir string }` — emitted by `Watcher` to the frontend via the callback. |
| `Watcher` | Wraps a `rjeczalik/notify` recursive watch; owns debounce timers per directory. |
| `NewWatcher(dir, emit)` | Installs one recursive watch on `dir` and starts the watcher; caller must call `Close()`. |
| `SearchMatch` | `{ path, lineNumber, lineContent, matchStart, matchEnd }` returned by `SearchFiles`. |
| `SearchFiles(dir, query, useRegex)` | Walks `dir` recursively, skipping hidden directories, returns up to 200 matches. |
| `RevealInFinder(path, allowedRoot)` | Opens the native file manager: `open -R` (macOS), `explorer /select,` (Windows), `xdg-open` (Linux). |

## Patterns & integration

- IPC entry points live in `internal/app/filesystem.go`; the package functions are called as thin delegators.
- `Watcher` is started by `StartFileWatcher(dir)` and stopped by `StopFileWatcher()` IPC methods. The frontend (`FileBrowser.tsx`) listens for the `"fs:changed"` Wails event.
- The watcher uses a **single recursive watch** (`rjeczalik/notify`): FSEvents on macOS, `ReadDirectoryChangesW` on Windows, inotify on Linux. On macOS/Windows this is one OS subscription for the entire tree, so a large/deep tree (a `venv`, `node_modules`, …) no longer registers one watch per directory or exhausts the file-descriptor limit (issue #485). New subdirectories are covered automatically.
- Write-only events on existing files are intentionally skipped — only create/delete/rename events trigger directory refresh, since `ListDir` output does not change on file content edits.
- Hidden files and directories (names starting with `.`) are excluded from both the watcher and `SearchFiles`.
- The `DuplicateFile` copy name follows the pattern `stem_copy.ext`, `stem_copy_2.ext`, etc., up to 999 attempts.

## Gotchas

- Path validation uses `filepath.EvalSymlinks` on the existing ancestor, not the full target path (which may not exist yet). There is a narrow TOCTOU window between validation and the actual OS call — acceptable on a single-user desktop app.
- Case-only renames (e.g. `File.sql` → `file.sql`) are handled via `os.SameFile` so they work correctly on both case-sensitive and case-insensitive filesystems.
- The recursive watch reports events for the **entire** tree, including hidden directories. Hidden entries (any path component starting with `.`) are therefore filtered out per-event in `handleEvent`, not excluded from the watch itself.
- macOS FSEvents reports **canonical** paths (e.g. `/private/var/…` for a `/var/…` symlink). `NewWatcher` resolves the root with `EvalSymlinks` and translates event paths back into the caller's namespace before emitting, so emitted `Dir` values match the path the caller passed in.
- On Linux, `rjeczalik/notify` still uses inotify (one watch per directory); a very large tree can exhaust the inotify watch limit. The macOS/Windows backends are recursive and do not have this limit.
