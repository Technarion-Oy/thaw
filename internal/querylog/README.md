# internal/querylog

> Session-scoped, thread-safe log of all SQL queries Thaw sends to Snowflake.

## Responsibility

Provides an in-memory, FIFO-evicting log of every SQL statement that Thaw
executes against Snowflake during a session — both user-initiated (editor) and
internal (object listing, DDL fetching, session setup). Used for debugging and
issue reporting via the "Query Log" result pane tab.

## Key types

| Type | Purpose |
|------|---------|
| `Log` | Thread-safe log container with FIFO eviction (default 5 000 entries). |
| `Entry` | A single log record: timestamp, SQL, query ID, status, duration, error, source, feature, tab ID. |
| `Status` | `RUNNING`, `SUCCESS`, `FAIL`, `CANCELED`. |
| `Source` | `user` (editor), `internal` (object listing, DDL fetch, etc.). |
| `Feature` | Free-form label of the Thaw feature that issued the query (e.g. `"Object Browser"`, `"SQL Editor"`, `"DDL Export"`). Empty when unknown. |

## API

| Method | Description |
|--------|-------------|
| `New() *Log` | Creates a new disabled log with default settings. |
| `Record(Entry) int` | Appends an entry and returns its ID. Evicts oldest if over capacity. |
| `UpdateStatus(id, status, durationMs, errMsg, queryID)` | Updates a RUNNING entry to its final state. |
| `Entries() []Entry` | Returns a snapshot copy of all entries. |
| `Clear()` | Removes all entries and resets the ID counter. |
| `SetEnabled(bool)` / `IsEnabled() bool` | Enable/disable logging. |

### Context helpers

| Function | Description |
|----------|-------------|
| `WithSource(ctx, Source) ctx` | Annotates context with query source. |
| `GetSource(ctx) Source` | Extracts source (defaults to `SourceInternal`). |
| `WithFeature(ctx, string) ctx` | Annotates context with the originating Thaw feature. |
| `GetFeature(ctx) string` | Extracts the feature (defaults to `""`). |
| `WithTabID(ctx, string) ctx` | Annotates context with tab ID. |
| `GetTabID(ctx) string` | Extracts tab ID (defaults to `""`). |

## Integration

- **`internal/app/app.go`**: `queryLog *querylog.Log` field on `App`, initialized in `NewApp()`.
  The `OnQuery` hook on `snowflake.Client` records internal queries, reading the
  feature from the context via `GetFeature`. Cleared on `Disconnect()`.
- **`internal/app/features.go`**: defines the `Feature*` name constants and the
  `App.fctx(feature)` helper. Each IPC entry point passes `a.fctx("…")` (instead of
  `a.ctx`) into client/domain calls so the resulting internal queries are attributed
  to the feature that issued them. User editor queries are tagged `"SQL Editor"`.
- **`internal/app/query.go`**: `StartQuery` records a RUNNING entry; `WaitForQueryResult` updates
  it to final status. `ExecuteQuery` records with immediate final status.
- **`internal/app/querylog.go`**: Thin delegator IPC methods exposed to the frontend.
- **Frontend**: `QueryLogPane.tsx` subscribes to `querylog:entry` and `querylog:update` Wails
  events. It renders a **Feature** column and a Feature filter dropdown (options derived from
  the features present in the log) alongside the existing Source/Status filters.

## Design decisions

- **User queries skip the OnQuery hook**: User-initiated queries are tracked with a two-phase
  RUNNING → final flow in `app/query.go`. The `OnQuery` hook on `snowflake.Client` checks
  `GetSource(ctx)` and skips `SourceUser` to avoid double-logging.
- **Feature flag**: Disabled by default (`QueryLog: false` in `DefaultFeatureFlags`). Opt-in via
  View → Enabled Features or Tools → Query Log → Enable Query Log.
