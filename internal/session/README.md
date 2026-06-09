# internal/session

> Window geometry persistence — saves and restores the main window position and size across application launches.

## Responsibility

Serialises a `WindowState` struct (position, dimensions, maximised flag) to a JSON file
in an OS-specific user-data directory and reads it back on the next launch so the window
reopens where the user left it.

## Key files

| File | Purpose |
|---|---|
| `session.go` | `WindowState`, `LoadWindowState()`, `SaveWindowState()`. |
| `path_prod.go` | `StatePath()` for production builds (`//go:build !dev`): macOS `~/Library/Application Support/thaw/session.json`, Windows `%LOCALAPPDATA%\thaw\session.json`, Linux `$XDG_DATA_HOME/thaw/session.json`. |
| `path_dev.go` | `StatePath()` for dev builds: local `./session.json`. |

## Key types & functions

| Symbol | Description |
|---|---|
| `WindowState` | `{ x, y, width, height int; maximised bool }` — JSON tag for `maximised` uses British spelling for backward-compatibility with existing session files. |
| `LoadWindowState()` | Returns `(WindowState, true)` on success; `(zero, false)` if file is missing, unparseable, or has implausible dimensions (`width < 100` or `height < 100`). |
| `SaveWindowState(s)` | Marshals to indented JSON and writes to `StatePath()`. |
| `StatePath()` | Returns the OS-specific file path, creating the parent directory as needed. |

## Patterns & integration

- `internal/app/app.go` calls `LoadWindowState()` during `startup` to set the initial Wails window geometry.
- `internal/app/app.go` calls `SaveWindowState()` during `shutdown` to persist the final window geometry.
- Dev and production `StatePath` functions are selected at compile time via build tags.

## Gotchas

- The JSON key for the maximised field is `"maximised"` (British spelling) — do not change it, as existing session files on user machines use this key.
- Implausible dimensions (`width < 100`, `height < 100`) are rejected and treated as a missing file, preventing a zero-size window on corrupt or zeroed state files.
