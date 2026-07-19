# internal/logger

> Application-wide structured logger with file rotation and a gosnowflake driver noise filter.

## Responsibility

Initialises a `slog.Logger` backed by a rotating log file (`lumberjack`) and exposes it
as the package-level `L` variable. Dev builds additionally echo to stderr and enable
`DEBUG` level. Production builds use an OS-specific log directory. A `driverNoiseFilter`
`slog.Handler` wrapper suppresses known-harmless ERROR messages emitted by the
gosnowflake driver as side-effects of query cancellation, row-cap truncation, and
— **only on records tagged as a serialized single-use-credential login** (via the
`SerializedLoginLogKey` context attribute `internal/snowflake` puts on the login
context) — the expected per-connection re-auth churn. The tagging is per
connection, so auth failures for other authenticators, other connections, or
outside such a login still reach the log; the real connect error is also surfaced
separately by `App.Connect`.

The minimum level is **runtime-adjustable**: the handler is wired to a package-level
`slog.LevelVar` (seeded to the build default — DEBUG in dev, INFO in production), and
`SetLevel("debug"|"info"|"warn"|"error")` changes it without rebuilding the handler.
`internal/app` calls `SetLevel` from the user's `LogPrefs` on startup and whenever the
logging preferences are updated. An empty/unrecognized name is a no-op, so the build
default is preserved when the user has expressed no preference.

Rotation is **age-driven, not just size-driven**. `lumberjack` on its own only rotates
when a write pushes the active file past `MaxSize`, and its `MaxAge`/`MaxBackups` cleanup
applies to rotated backups only — never to the active file. Because Thaw logs at INFO in
production and produces little volume, the active file would take months to reach
`MaxSize`, so nothing would ever rotate and >30-day-old entries would live forever. To
fix this, `Init()` forces an age-based rotation on startup (`maybeRotateByAge`) when the
active file's oldest entry is older than `rotationInterval` (24 h), and a background
ticker keeps rotating on that interval for long-running sessions. Each rotation triggers
`lumberjack`'s `MaxAge` cleanup, so backups older than 30 days are deleted.

Retention is **close to 30 days, not a strict bound.** `MaxAge` cleanup is anchored to a
backup's *rotation* time, not to the age of the entries inside it. For a user who runs
Thaw roughly daily, each backup spans ~1 day and the oldest entries live ~30 days. But
whenever the gap between launches exceeds `rotationInterval`, the entries accumulated
during that gap are rotated into a backup dated *now* and then survive another 30 days —
so the effective retention for the oldest entries is roughly `30 days + longest gap
between launches` (e.g. ~37 days for a weekly user, ~60 for a monthly one). This is
inherent to rotating the file wholesale rather than pruning individual stale lines; it is
still bounded and vastly better than the previous "grows forever" behaviour. If a strict
30-day cap is ever required, prune stale lines from the active file at rotation time
instead of rotating it as a whole.

## Key files

| File | Purpose |
|---|---|
| `logger.go` | `Init()`, `L`, `Dir`, `Path`, `SetLevel`, the runtime `levelVar`, `driverNoiseFilter`. |
| `path_prod.go` | `logFilePath()` for production builds (`//go:build !dev`): macOS `~/Library/Logs/thaw/thaw.log`, Windows `%APPDATA%\thaw\logs\thaw.log`, Linux `$XDG_STATE_HOME/thaw/thaw.log`. |
| `path_dev.go` | `logFilePath()` for dev builds (`//go:build dev`): writes to `./logs/thaw.log` relative to the working directory. |

## Key types & functions

| Symbol | Description |
|---|---|
| `L *slog.Logger` | Package-level logger; safe for concurrent use. Defaults to a no-op (`io.Discard`) logger so it is never nil before `Init()` (or in tests that don't call `Init`); `Init()` installs the real file-backed handler. |
| `Dir string` | Directory of the log file; set by `Init()`; used by `crashreport` to co-locate crash JSON files. |
| `Path string` | Absolute path to the active log file; set by `Init()`; used by `App.RevealLogFile` and the logging-preferences UI. |
| `Init() func()` | Sets up rotation and returns a cleanup function to defer (stops the rotation ticker and closes the file). |
| `SetLevel(name string)` | Changes the minimum severity at runtime (`debug`/`info`/`warn`/`error`); empty or unrecognized names are a no-op. |
| `maybeRotateByAge` | Rotates the active file on startup when its oldest entry is older than `rotationInterval`, so age-based cleanup has backups to prune. |
| `startRotationTicker` | Rotates on `rotationInterval` until stopped, keeping retention bounded during long sessions. |
| `firstEntryTime` / `parseSlogTime` | Read the first log line and parse its `time=<RFC3339>` prefix to find the oldest entry's timestamp. |
| `driverNoiseFilter` | Drops `slog.LevelError` records that are handled-but-noisy gosnowflake output: message containing `"failed to extract HTTP response body"` (Arrow chunk download errors, always), or — **only when the record carries the `SerializedLoginLogKey` attribute** — beginning with `"Authentication FAILED"` / `"Failed to authenticate. Connection failed after"` (single-use-credential re-auth churn). `internal/snowflake` registers that key with the driver (append, preserving `LOG_SESSION_ID`/`LOG_USER`) and tags each serialized login's context, so suppression is scoped per connection and genuine failures elsewhere stay visible; the detailed connect error is also logged/surfaced by `App.Connect`. |

## Patterns & integration

- `internal/app/run.go` calls `cleanup := logger.Init()` then `defer cleanup()` as one of the very first things at startup.
- `internal/crashreport` reads `logger.Dir` to place crash JSON files alongside the log.
- `slog.SetDefault(L)` is called inside `Init`, so the gosnowflake driver (which defaults to `slog.Default()`) automatically routes its log output through the same file and filter without explicit redirection.

## Gotchas

- Rotation settings: 10 MB size safety valve, 24 h age-based rotation interval, 30-day retention (`MaxAge`), unbounded backup count (`MaxBackups: 0`, so `MaxAge` alone governs retention), gzip compression enabled.
- `MaxAge`/`MaxBackups` never prune the active `thaw.log` — only rotated backups — so the age-based rotation in `Init()` is what actually enforces the 30-day limit. Removing it re-introduces the "log grows forever" bug.
- The rotation ticker runs a background goroutine; the deferred cleanup from `Init()` stops it before closing the file.
- The `driverNoiseFilter` is applied at `slog.Handler` level, not at the logger level, so it cannot be bypassed by callers who use `slog.Default()` directly.
- Dev and production `logFilePath` functions are selected at compile time via build tags, not at runtime.
