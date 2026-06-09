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
| `Entry` | A single log record: timestamp, SQL, query ID, status, duration, error, source, tab ID. |
| `Status` | `RUNNING`, `SUCCESS`, `FAIL`, `CANCELED`. |
| `Source` | `user` (editor), `internal` (object listing, DDL fetch, etc.). |

## API

| Method | Description |
|--------|-------------|
| `New() *Log` | Creates a new disabled log with default settings. |
| `Record(Entry) int` | Appends an entry and returns its ID. Evicts oldest if over capacity. |
| `UpdateStatus(id, status, durationMs, errMsg, queryID)` | Updates a RUNNING entry to its final state. |
| `Entries() []Entry` | Returns a snapshot copy of all entries. |
| `Clear()` | Removes all entries and resets the ID counter. |
| `SetEnabled(bool)` / `IsEnabled() bool` | Enable/disable logging. |
| `SetFilter(string)` / `Filter() string` | Source filter: `"all"`, `"user"`, `"internal"`. |

### Context helpers

| Function | Description |
|----------|-------------|
| `WithSource(ctx, Source) ctx` | Annotates context with query source. |
| `GetSource(ctx) Source` | Extracts source (defaults to `SourceInternal`). |
| `WithTabID(ctx, string) ctx` | Annotates context with tab ID. |
| `GetTabID(ctx) string` | Extracts tab ID (defaults to `""`). |

## Integration

- **`internal/app/app.go`**: `queryLog *querylog.Log` field on `App`, initialized in `NewApp()`.
  The `OnQuery` hook on `snowflake.Client` records internal queries. Cleared on `Disconnect()`.
- **`internal/app/query.go`**: `StartQuery` records a RUNNING entry; `WaitForQueryResult` updates
  it to final status. `ExecuteQuery` records with immediate final status.
- **`internal/app/querylog.go`**: Thin delegator IPC methods exposed to the frontend.
- **Frontend**: `QueryLogPane.tsx` subscribes to `querylog:entry` and `querylog:update` Wails events.

## Design decisions

- **User queries skip the OnQuery hook**: User-initiated queries are tracked with a two-phase
  RUNNING → final flow in `app/query.go`. The `OnQuery` hook on `snowflake.Client` checks
  `GetSource(ctx)` and skips `SourceUser` to avoid double-logging.
- **Feature flag**: Disabled by default (`QueryLog: false` in `DefaultFeatureFlags`). Opt-in via
  View → Enabled Features or View → Query Log → Enable Query Log.
