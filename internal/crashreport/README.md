# internal/crashreport

> Panic handler that captures unexpected crashes and writes timestamped JSON crash files alongside the rotating log.

## Responsibility

Catches top-level goroutine panics via a `defer`-based recover, writes a structured JSON
crash file to the application log directory, logs the event at ERROR level, and then
re-panics so the process terminates with a non-zero exit code. A `sendRemote` stub is
provided to wire in a remote crash-reporting backend (Sentry, Bugsnag, custom HTTP) when
one is chosen.

## Key files

| File | Purpose |
|---|---|
| `crashreport.go` | Entire package: `Init`, `Recover`, internal `report`, and `sendRemote` stub. |

## Key types & functions

| Symbol | Description |
|---|---|
| `Init(version string)` | Records the application version to embed in crash reports. Call once at startup, before deferring `Recover`. |
| `Recover()` | Must be called with `defer` at the top of `main()`. Calls `recover()`, invokes `report()`, then re-panics. |
| `report(panicMsg, stack)` | Writes JSON to `<logger.Dir>/crash_<timestamp>.json` and logs at ERROR. |
| `sendRemote(payload)` | Stub — implement when a remote reporting service is chosen. |

## Patterns & integration

- `internal/app/run.go` calls `crashreport.Init(version.Version)` then `defer crashreport.Recover()` at the very top of the Wails entry point.
- Crash files are co-located with the rotating log files (`logger.Dir`), so they are easy to find alongside the application log.
- No PII, SQL text, credentials, or account-specific identifiers are included in crash payloads.

## Gotchas

- `Recover` **re-panics** after writing the report. This is intentional — it ensures a non-zero process exit and a visible crash in any surrounding process supervisor.
- The crash file is written synchronously inside the panic path; keep `report` fast.
