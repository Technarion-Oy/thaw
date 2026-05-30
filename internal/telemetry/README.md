# internal/telemetry

> Anonymous usage event tracking — currently log-only, with a remote backend stub.

## Responsibility

Records structured usage events (app lifecycle, connection, query execution, feature usage)
enriched with a per-session anonymous ID, app version, and OS. Events are currently written
to the application log at DEBUG level. A `sendRemote` stub is provided to wire in a remote
analytics backend (PostHog, Segment, Mixpanel, custom HTTP) when one is chosen.

No PII, SQL query content, credentials, or account-specific identifiers are ever recorded.

## Key files

| File | Purpose |
|---|---|
| `telemetry.go` | `Client`, `Default`, `Init`, `Track`, `SessionDuration`, event name constants, `newSessionID`, `sendRemote` stub. |

## Key types & functions

| Symbol | Description |
|---|---|
| `Event string` | Named event constant (e.g. `EventAppStarted`, `EventQueryCompleted`, `EventFeatureGitCommit`). |
| `Props map[string]any` | Caller-supplied event properties; merged with session-level context before logging. |
| `Client` | Holds session ID, version, OS, and start time. Thread-safe via `sync.Mutex`. |
| `Default *Client` | Package-level client initialised by `Init`. |
| `Init(version)` | Creates `Default`; call once at startup. |
| `Track(event, props)` | Package-level convenience — delegates to `Default.Track`. No-op before `Init`. |
| `Client.Track(event, props)` | Safe to call on a nil receiver. Logs at DEBUG via `logger.L`. |
| `SessionDuration()` | Returns time elapsed since `Init` was called. |

## Event catalogue

| Category | Events |
|---|---|
| Lifecycle | `app.started`, `app.stopped` |
| Connection | `snowflake.connected`, `snowflake.connection_failed`, `snowflake.disconnected` |
| Query | `query.started`, `query.completed`, `query.failed`, `query.cancelled` |
| Features | `feature.er_diagram`, `feature.er_designer`, `feature.time_travel`, `feature.export_ddl`, `feature.export_data`, `feature.import_data`, `feature.git_commit`, `feature.undrop` |

## Patterns & integration

- `internal/app/run.go` calls `telemetry.Init(version.Version)` at startup.
- `internal/app/app.go` calls `telemetry.Track(telemetry.EventAppStarted, nil)` on connect and `EventDisconnected` on disconnect.
- The session ID is a random 16-byte hex string generated at `Init` time; it is not persisted across restarts, making it non-identifying across sessions.

## Gotchas

- `sendRemote` is intentionally a dead-code stub (`//nolint:unused,deadcode`). Do not remove it — it is the integration point for when a remote backend is chosen.
- `Track` is safe to call before `Init` (nil receiver guard), so call sites do not need to check whether telemetry has been initialised.
