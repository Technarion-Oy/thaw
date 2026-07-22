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
| `fs.go` | Core CRUD helpers: `ReadFile`, `ReadFileHead`, `WriteFile`, `WriteFileAtomic` (temp-file + rename; shared atomic writer used by `config.Save`, `gitrepo`, and `sfconfig` so a concurrent/second-process reader never sees a torn file), `ListDir`, `RevealInFinder`, `DeleteFile`, `DeleteDirectory`, `RenameFile`, `MkDir`, `WriteFileInRoot`, `DuplicateFile`, `CopyFile`, and the path-validation internals. |
| `watcher.go` | `Watcher` struct (`rjeczalik/notify`-based): a single recursive watch over the whole tree, 200 ms debounce per directory, `FSChangeEvent`. Applies user-configurable `WatchOptions` (exclude globs, a distinct-directory cap) by dropping matching events post-hoc. |
| `fdlimit_unix.go` / `fdlimit_windows.go` | `RaiseFDLimit()` — bumps the process file-descriptor soft limit toward the hard limit (`RLIMIT_NOFILE` via `setrlimit`; no-op on Windows). Opt-in mitigation for FD-hungry workspaces, invoked by `StartFileWatcher` when enabled. |
| `reveal_windows.go` / `reveal_other.go` | Platform implementations of `revealInFileManager(abs)`, the OS half of `RevealInFinder`. Windows needs `syscall.SysProcAttr{CmdLine}` to hand Explorer an unescaped `/select,"…"` argument (issue #294), which only exists on Windows builds; the `!windows` file covers macOS (`open -R`) and Linux (`xdg-open`). |
| `export.go` | `WriteBinaryFile` (base64-decode then write, used for Excel export) and `SanitizeFilename`. |
| `search.go` | `SearchFiles`: recursive file-content search (substring or regex), capped at 200 results. |

## Key types & functions

| Symbol | Description |
|---|---|
| `FileEntry` | `{ name, path, isDir, size }` — single directory entry returned by `ListDir`. |
| `FSChangeEvent` | `{ dir string }` — emitted by `Watcher` to the frontend via the callback. |
| `Watcher` | Wraps a `rjeczalik/notify` recursive watch; owns debounce timers per directory. |
| `NewWatcher(dir, opts, emit)` | Installs one recursive watch on `dir` and starts the watcher; caller must call `Close()`. `opts` (`WatchOptions`) carries exclude globs and a distinct-directory cap; the zero value keeps the historical no-exclusion behavior. |
| `WatchOptions` | `{ ExcludeGlobs []string, MaxWatchedDirs int }` — user-tunable resource controls, sourced from `config.FileWatchConfig`. |
| `RaiseFDLimit()` | Raises `RLIMIT_NOFILE` soft→hard; returns `(soft, hard, err)`. No-op on Windows. |
| `SearchMatch` | `{ path, lineNumber, lineContent, matchStart, matchEnd }` returned by `SearchFiles`. |
| `SearchFiles(dir, query, useRegex)` | Walks `dir` recursively, skipping hidden directories, returns up to 200 matches. |
| `RevealInFinder(path, allowedRoot)` | Validates `path` inside `allowedRoot`, then delegates to the platform-specific `revealInFileManager`: `open -R` (macOS), `explorer /select,` (Windows), `xdg-open` (Linux). |

## Patterns & integration

- IPC entry points live in `internal/app/filesystem.go`; the package functions are called as thin delegators.
- `ReadFile` refuses files that look binary (a NUL byte in the first 8 KB, the git heuristic) so the editor never opens garbage; all open paths (native dialog, file-tree click, diff) route through it.
- `ReadFile` and `ReadFileHead` map an `os.ErrNotExist` to an error containing `NotFoundMarker` (`"file not found"`). The raw OS "no such file" message is localized (e.g. on non-English Windows), so consumers — including the frontend over the Wails bridge — must match this stable marker to detect a deleted file. `snowpark.ReadNotebook` mirrors the same marker.
- `Watcher` is started by `StartFileWatcher(dir)` and stopped by `StopFileWatcher()` IPC methods. The frontend (`FileBrowser.tsx`) listens for the `"fs:changed"` Wails event.
- The watcher uses a **single recursive watch** (`rjeczalik/notify`): FSEvents on macOS, `ReadDirectoryChangesW` on Windows, inotify on Linux. On macOS/Windows this is one OS subscription for the entire tree, so a large/deep tree (a `venv`, `node_modules`, …) no longer registers one watch per directory or exhausts the file-descriptor limit (issue #485). New subdirectories are covered automatically.
- Write events on existing files are emitted too (coalesced by directory like any other change) so open editor tabs can re-read externally edited files. A pure content change doesn't alter `ListDir` output, so the resulting tree re-list is a harmless no-op.
- Hidden files and directories (names starting with `.`) are excluded from both the watcher and `SearchFiles`.
- **User-configurable watch controls** (issue #488, surfaced in **View → File Watching…**): `WatchOptions.ExcludeGlobs` drops change events whose tree-relative path matches a glob — a single-name pattern (`node_modules`) matches that directory at any depth, a slashed pattern (`build/generated`) excludes that subtree. Hidden entries (dot-prefixed) are already dropped by the hidden-directory filter before exclusion runs, so a dot-prefixed pattern is redundant and the defaults list only non-hidden dirs. `WatchOptions.MaxWatchedDirs` caps the number of distinct directories emitted for (0 = unlimited), bounding the debounce-timer map and re-list churn on pathological trees. Because the backend is a single recursive watch, both are enforced by dropping events after the fact, not by declining to install a watch. Defaults and validation live in `config.FileWatchConfig`; `internal/app/filesystem.go` wires them into `NewWatcher` and optionally calls `RaiseFDLimit()`.
- **Watcher failure is non-fatal**: `StartFileWatcher` logs and swallows a watch-setup error so opening a folder never fails on the watcher — only the auto-refresh-on-external-change convenience is lost.
- The `DuplicateFile` copy name follows the pattern `stem_copy.ext`, `stem_copy_2.ext`, etc., up to 999 attempts.
- `CopyFile(src, dst, allowedRoot)` copies a file (`io.Copy` with `O_EXCL`) or a directory (recursive `os.CopyFS`); both endpoints are validated inside `allowedRoot`, `dst` must not already exist (never a silent overwrite), and copying a directory into itself/a descendant is rejected. The frontend resolves name conflicts before calling, so the move/paste flow stays backend-stateless. Cross-volume **move** is the frontend's `RenameFile`-then-`CopyFile`+delete fallback (effectively dead on a single-root export dir).

## Gotchas

- Path validation uses `filepath.EvalSymlinks` on the existing ancestor, not the full target path (which may not exist yet). There is a narrow TOCTOU window between validation and the actual OS call — acceptable on a single-user desktop app.
- Case-only renames (e.g. `File.sql` → `file.sql`) are handled via `os.SameFile` so they work correctly on both case-sensitive and case-insensitive filesystems.
- **Windows Explorer `/select` cannot be fixed by quoting inside an `Args` string.** Go's `os/exec` runs every `Args` entry through `syscall.EscapeArg`, which backslash-escapes quotes; Explorer's non-standard parser has no `\"` escape, so the corrupted path makes it "open one level up" (issue #294). The only reliable fixes bypass `EscapeArg`: set `SysProcAttr.CmdLine` (what `reveal_windows.go` does) or call the Win32 `SHOpenFolderAndSelectItems` shell API. Because `SysProcAttr` fields are OS-specific, `revealInFileManager` must live in a `//go:build windows` file. This is verifiable only on real Windows — CI cross-compiles but never executes it (manual-verification checklist: issue #840).
- The recursive watch reports events for the **entire** tree, including hidden directories. Hidden entries (any path component starting with `.`) are therefore filtered out per-event in `handleEvent`, not excluded from the watch itself.
- macOS FSEvents reports **canonical** paths (e.g. `/private/var/…` for a `/var/…` symlink). `NewWatcher` resolves the root with `EvalSymlinks` and translates event paths back into the caller's namespace before emitting, so emitted `Dir` values match the path the caller passed in.
- On Linux, `rjeczalik/notify` still uses inotify (one watch per directory); a very large tree can exhaust the inotify watch limit. The macOS/Windows backends are recursive and do not have this limit.
