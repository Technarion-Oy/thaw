# internal/logger

> Application-wide structured logger with file rotation and a gosnowflake driver noise filter.

## Responsibility

Initialises a `slog.Logger` backed by a rotating log file (`lumberjack`) and exposes it
as the package-level `L` variable. Dev builds additionally echo to stderr and enable
`DEBUG` level. Production builds use an OS-specific log directory. A `driverNoiseFilter`
`slog.Handler` wrapper suppresses known-harmless ERROR messages emitted by the
gosnowflake driver as side-effects of query cancellation and row-cap truncation.

## Key files

| File | Purpose |
|---|---|
| `logger.go` | `Init()`, `L`, `Dir`, `driverNoiseFilter`. |
| `path_prod.go` | `logFilePath()` for production builds (`//go:build !dev`): macOS `~/Library/Logs/thaw/thaw.log`, Windows `%APPDATA%\thaw\logs\thaw.log`, Linux `$XDG_STATE_HOME/thaw/thaw.log`. |
| `path_dev.go` | `logFilePath()` for dev builds (`//go:build dev`): writes to `./logs/thaw.log` relative to the working directory. |

## Key types & functions

| Symbol | Description |
|---|---|
| `L *slog.Logger` | Package-level logger; safe for concurrent use; set by `Init()`. |
| `Dir string` | Directory of the log file; set by `Init()`; used by `crashreport` to co-locate crash JSON files. |
| `Init() func()` | Sets up rotation and returns a cleanup function to defer. |
| `driverNoiseFilter` | Drops `slog.LevelError` records whose message contains `"failed to extract HTTP response body"` (Arrow chunk download errors from gosnowflake). |

## Patterns & integration

- `internal/app/run.go` calls `cleanup := logger.Init()` then `defer cleanup()` as one of the very first things at startup.
- `internal/crashreport` reads `logger.Dir` to place crash JSON files alongside the log.
- `slog.SetDefault(L)` is called inside `Init`, so the gosnowflake driver (which defaults to `slog.Default()`) automatically routes its log output through the same file and filter without explicit redirection.

## Gotchas

- Rotation settings: 10 MB max per file, 5 old files retained, 30-day age limit, gzip compression enabled.
- The `driverNoiseFilter` is applied at `slog.Handler` level, not at the logger level, so it cannot be bypassed by callers who use `slog.Default()` directly.
- Dev and production `logFilePath` functions are selected at compile time via build tags, not at runtime.
