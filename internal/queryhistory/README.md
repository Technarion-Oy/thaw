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

### `buildQueryHistorySql(filterType, sessionID, userName, warehouseName, endTimeStart, endTimeEnd string, resultLimit int, includeClientGenerated bool) string`
Unexported — all external access goes through `GetQueryHistory`, which validates
the session ID first. Selects the Snowflake table function based on `filterType`
(`"session"`, `"user"`, `"warehouse"`, or default `"all"`), builds named-argument
clauses for whichever filters are non-empty, and returns a `SELECT` ordered by
`START_TIME DESC`. Date strings are cast to `TIMESTAMP_LTZ` inline. A SESSION_ID
that is not a bare int64 is silently dropped here as an injection guard, so
callers must pre-validate (`GetQueryHistory` does).

### `ParseQueryHistory(res *snowflake.QueryResult) []QueryHistoryRow`
Uses `snowflake.ColIdx` for position-independent column lookup (safe against
column-order changes) and `snowflake.CellString` / `snowflake.CellInt64` for
nil-safe value extraction.

### `GetQueryHistory(ctx, client, ...) ([]QueryHistoryRow, error)`
Convenience wrapper: rejects an invalid (non-int64) session ID with an error,
calls `buildQueryHistorySql`, executes via
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
  10 000 per Snowflake's own limits; no clamping is done here.
- `ParseQueryHistory` returns an empty (non-nil) slice when `res` is nil, matching
  the zero-value expectation of the frontend.
