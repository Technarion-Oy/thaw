# internal/queryhistory

> SQL builder and row parser for Snowflake's `INFORMATION_SCHEMA.QUERY_HISTORY*` table functions.

## Responsibility

Constructs the appropriate `QUERY_HISTORY`, `QUERY_HISTORY_BY_SESSION`,
`QUERY_HISTORY_BY_USER`, or `QUERY_HISTORY_BY_WAREHOUSE` table-function call based
on a filter type and optional parameters, then parses the raw `QueryResult` into
typed `QueryHistoryRow` slices. Also provides a convenience `GetQueryHistory`
function that combines both steps.

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (SQL Editor & Diagnostics) |
| `queryhistory.go` | `QueryHistoryRow` type, `buildQueryHistorySql` (unexported), `ParseQueryHistory`, `GetQueryHistory` |
| `queryhistory_test.go` | Unit tests for the SQL builder covering all filter types and edge cases |

## Key types & functions

### `QueryHistoryRow`
```go
type QueryHistoryRow struct {
    QueryID       string `json:"queryId"`
    SessionID     string `json:"sessionId"`
    QueryText     string `json:"queryText"`
    QueryType     string `json:"queryType"`
    UserName      string `json:"userName"`
    WarehouseName string `json:"warehouseName"`
    DatabaseName  string `json:"databaseName"`
    SchemaName    string `json:"schemaName"`
    StartTime     string `json:"startTime"`
    EndTime       string `json:"endTime"`
    ElapsedMs     int64  `json:"elapsedMs"`
    Status        string `json:"status"`
    ErrorMessage  string `json:"errorMessage"`
    RowsProduced  int64  `json:"rowsProduced"`
    BytesScanned  int64  `json:"bytesScanned"`
}
```

### `QueryHistoryFilters`
```go
type QueryHistoryFilters struct {
    Statuses      []string `json:"statuses"`      // EXECUTION_STATUS IN (…), case-insensitive
    QueryTypes    []string `json:"queryTypes"`    // QUERY_TYPE IN (…), case-insensitive
    MinDurationMs int64    `json:"minDurationMs"` // TOTAL_ELAPSED_TIME >= N ms (0 = off)
    Database      string   `json:"database"`      // DATABASE_NAME = … (case-insensitive)
    Schema        string   `json:"schema"`        // SCHEMA_NAME = … (case-insensitive)
}
```
Optional server-side `WHERE`-clause filters (issue #827). The `QUERY_HISTORY*`
table functions do **not** accept these as named arguments, so they are rendered
as a `WHERE` clause wrapping the function call. Every field is optional; a zero
value adds no predicate. String literals are escaped via `snowflake.QuoteStringLit`;
the status/type IN-lists and the db/schema comparisons are uppercased for
case-insensitive matching.

### `buildQueryHistorySql(filterType, sessionID, userName, warehouseName, endTimeStart, endTimeEnd string, resultLimit int, includeClientGenerated bool, filters QueryHistoryFilters) string`
Unexported — all external access goes through `GetQueryHistory`, which validates
the session ID first. Selects the Snowflake table function based on `filterType`
(`"session"`, `"user"`, `"warehouse"`, or default `"all"`), builds named-argument
clauses for whichever filters are non-empty, appends a `WHERE` clause for any
`filters`, and returns a `SELECT` ordered by `START_TIME DESC`. Date strings are
cast to `TIMESTAMP_LTZ` inline. For `filterType "session"`, a `SESSION_ID` that is
not a bare int64 violates the precondition and **panics** (a programmer error —
callers must pre-validate; `GetQueryHistory` does), rather than silently emitting
a wrong query.

**Limit re-application:** the table function applies `RESULT_LIMIT` itself, *before*
the wrapping `WHERE` runs. So when any `filters` predicate is active, the builder
requests the full window (`RESULT_LIMIT => 10000`, Snowflake's max) from the
function and re-applies the caller's `resultLimit` as an outer `LIMIT` — making the
limit count *matching* rows rather than rows scanned before the filter. Without
filters the `resultLimit` is passed straight to the function as before.

### `ParseQueryHistory(res *snowflake.QueryResult) []QueryHistoryRow`
Uses `snowflake.ColIdx` for position-independent column lookup (safe against
column-order changes) and `snowflake.CellString` / `snowflake.CellInt64` for
nil-safe value extraction.

### `GetQueryHistory(ctx, client, …, filters QueryHistoryFilters) ([]QueryHistoryRow, error)`
Convenience wrapper: rejects an invalid (non-int64) session ID with an error,
calls `buildQueryHistorySql` (forwarding `filters`), executes via
`client.QuerySingle`, and parses the result. This is the function called by the
`*App` thin delegator in `internal/app`.

## Patterns & integration (thin-delegator)

```go
// internal/app/query.go (illustrative)
func (a *App) GetQueryHistory(...) ([]queryhistory.QueryHistoryRow, error) {
    if a.client == nil { return nil, apperrors.ErrNotConnected }
    return queryhistory.GetQueryHistory(a.ctx, a.client, ...)
}
```

The builder and parser are independently unit-testable without a live connection.
Column lookup via `snowflake.ColIdx` avoids positional fragility if Snowflake
changes the column order in a future `QUERY_HISTORY` schema revision.

## Gotchas

- The date parameters (`endTimeStart`, `endTimeEnd`) are expected to be RFC3339
  strings. They are cast to `TIMESTAMP_LTZ` in the SQL but are not validated by
  this package — invalid date strings will produce a Snowflake runtime error.
- `resultLimit` is passed directly as `RESULT_LIMIT =>` and must be between 1 and
  10 000 per Snowflake's own limits; no clamping is done here. **Exception:** when a
  `QueryHistoryFilters` predicate is active the function is asked for the full
  window (`RESULT_LIMIT => 10000`) and `resultLimit` becomes an outer `LIMIT` — see
  "Limit re-application" above.
- The `filters` predicates run in a `WHERE` clause *within* the window the table
  function returns. Without the limit re-application (i.e. if a future change drops
  it) a small `resultLimit` would filter an already-truncated page — keep the outer
  `LIMIT` / bumped inner limit together.
- `ParseQueryHistory` returns an empty (non-nil) slice when `res` is nil, matching
  the zero-value expectation of the frontend.
