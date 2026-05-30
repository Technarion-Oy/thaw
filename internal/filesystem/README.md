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
| `watcher.go` | `Watcher` struct (fsnotify-based): recursive watch, 200 ms debounce per directory, auto-add new directories, `FSChangeEvent`. |
| `export.go` | `WriteBinaryFile` (base64-decode then write, used for Excel export) and `SanitizeFilename`. |
| `search.go` | `SearchFiles`: recursive file-content search (substring or regex), capped at 200 results. |

## Key types & functions

| Symbol | Description |
|---|---|
| `FileEntry` | `{ name, path, isDir, size }` — single directory entry returned by `ListDir`. |
| `FSChangeEvent` | `{ dir string }` — emitted by `Watcher` to the frontend via the callback. |
| `Watcher` | Wraps `fsnotify.Watcher`; owns debounce timers per directory. |
| `NewWatcher(dir, emit)` | Creates and starts the watcher; caller must call `Close()`. |
| `SearchMatch` | `{ path, lineNumber, lineContent, matchStart, matchEnd }` returned by `SearchFiles`. |
| `SearchFiles(dir, query, useRegex)` | Walks `dir` recursively, skipping hidden directories, returns up to 200 matches. |
| `RevealInFinder(path, allowedRoot)` | Opens the native file manager: `open -R` (macOS), `explorer /select,` (Windows), `xdg-open` (Linux). |

## Patterns & integration

- IPC entry points live in `internal/app/filesystem.go`; the package functions are called as thin delegators.
- `Watcher` is started by `StartFileWatcher(dir)` and stopped by `StopFileWatcher()` IPC methods. The frontend (`FileBrowser.tsx`) listens for the `"fs:changed"` Wails event.
- Write-only fsnotify events on existing files are intentionally skipped — only create/delete/rename events trigger directory refresh, since `ListDir` output does not change on file content edits.
- Hidden files and directories (names starting with `.`) are excluded from both the watcher and `SearchFiles`.
- The `DuplicateFile` copy name follows the pattern `stem_copy.ext`, `stem_copy_2.ext`, etc., up to 999 attempts.

## Gotchas

- Path validation uses `filepath.EvalSymlinks` on the existing ancestor, not the full target path (which may not exist yet). There is a narrow TOCTOU window between validation and the actual OS call — acceptable on a single-user desktop app.
- Case-only renames (e.g. `File.sql` → `file.sql`) are handled via `os.SameFile` so they work correctly on both case-sensitive and case-insensitive filesystems.
- On Linux, inotify watch limits may cause `NewWatcher` to log warnings for directories it cannot watch; this is non-fatal.
