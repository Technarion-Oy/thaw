# internal/warehouse

> ALTER WAREHOUSE SQL builder, warehouse lifecycle operations, parameter retrieval, and metering-history query/parse for Snowflake warehouse administration.

## Responsibility

Provides all backend logic needed by the warehouse management UI:

- **ALTER WAREHOUSE property builder** — validates enum and integer property values
  and constructs type-safe `ALTER WAREHOUSE ... SET` statements.
- **Lifecycle operations** — `SUSPEND`, `RESUME IF SUSPENDED`, `ABORT ALL QUERIES`,
  and `RENAME TO` wrappers.
- **Parameter retrieval** — reads per-warehouse parameter overrides from `SHOW
  PARAMETERS IN WAREHOUSE` and returns a filtered `[]snowflake.PropertyPair`.
- **Metering history** — builds the `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`
  query with optional filters and parses the result into typed `WarehouseMeteringRow`
  slices.

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (Object Browser & Administration) |
| `warehouse.go` | All types and all functions |
| `warehouse_test.go` | Unit tests for the property SQL builder |

## Key types & functions

### `WarehouseMeteringRow`
```go
type WarehouseMeteringRow struct {
    StartTime                string  `json:"startTime"`
    EndTime                  string  `json:"endTime"`
    WarehouseName            string  `json:"warehouseName"`
    CreditsUsed              float64 `json:"creditsUsed"`
    CreditsUsedCompute       float64 `json:"creditsUsedCompute"`
    CreditsUsedCloudServices float64 `json:"creditsUsedCloudServices"`
}
```

### `BuildAlterWarehousePropertySQL(name, property, value string) (string, error)`
The primary builder. Supports 14 property keys:

| Key | Validation | SQL clause |
|-----|-----------|-----------|
| `size` | enum allowlist (X-SMALL … 6X-LARGE) | `WAREHOUSE_SIZE = <SIZE>` |
| `warehouseType` | enum (`STANDARD`, `SNOWPARK-OPTIMIZED`) | `WAREHOUSE_TYPE = <TYPE>` |
| `autoSuspend` | non-negative integer; `0`/`""` → NULL | `AUTO_SUSPEND = N` or `= NULL` |
| `autoResume` | enum (`TRUE`, `FALSE`) | `AUTO_RESUME = TRUE/FALSE` |
| `comment` | string literal (escaped) | `COMMENT = '...'` |
| `maxClusterCount` | non-negative integer | `MAX_CLUSTER_COUNT = N` |
| `minClusterCount` | non-negative integer | `MIN_CLUSTER_COUNT = N` |
| `scalingPolicy` | enum (`STANDARD`, `ECONOMY`) | `SCALING_POLICY = <POLICY>` |
| `resourceMonitor` | identifier or empty → NULL | `RESOURCE_MONITOR = <NAME>` or `= NULL` |
| `enableQueryAcceleration` | enum (`TRUE`, `FALSE`) | `ENABLE_QUERY_ACCELERATION = TRUE/FALSE` |
| `queryAccelerationMaxScaleFactor` | non-negative integer | `QUERY_ACCELERATION_MAX_SCALE_FACTOR = N` |
| `maxConcurrencyLevel` | non-negative integer | `MAX_CONCURRENCY_LEVEL = N` |
| `statementQueuedTimeout` | non-negative integer | `STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = N` |
| `statementTimeout` | non-negative integer | `STATEMENT_TIMEOUT_IN_SECONDS = N` |

Returns an error for unknown property keys or invalid values.

### Lifecycle functions

| Function | SQL |
|----------|-----|
| `AlterProperty(ctx, client, name, property, value)` | Executes `BuildAlterWarehousePropertySQL` result |
| `Suspend(ctx, client, name)` | `ALTER WAREHOUSE <name> SUSPEND` |
| `Resume(ctx, client, name)` | `ALTER WAREHOUSE <name> RESUME IF SUSPENDED` |
| `AbortAllQueries(ctx, client, name)` | `ALTER WAREHOUSE <name> ABORT ALL QUERIES` |
| `Rename(ctx, client, name, newName)` | `ALTER WAREHOUSE <name> RENAME TO <newName>` |

### Parameter and metering functions

| Function | Description |
|----------|-------------|
| `GetParameters(ctx, client, name) ([]snowflake.PropertyPair, error)` | `SHOW PARAMETERS IN WAREHOUSE`; returns only `MAX_CONCURRENCY_LEVEL`, `STATEMENT_QUEUED_TIMEOUT_IN_SECONDS`, `STATEMENT_TIMEOUT_IN_SECONDS` |
| `BuildMeteringHistoryQuery(warehouse, startDate, endDate string) string` | Builds the ACCOUNT_USAGE query with optional WHERE clauses |
| `ParseMeteringHistory(res *snowflake.QueryResult) []WarehouseMeteringRow` | Projects result into typed rows using `snowflake.ColIdx` / `CellString` / `CellFloat` |
| `GetMeteringHistory(ctx, client, warehouse, startDate, endDate) ([]WarehouseMeteringRow, error)` | Convenience: build + query + parse |

## Patterns & integration (thin-delegator)

```go
// internal/app/warehouse.go
func (a *App) GetWarehouseMeteringHistory(wh, start, end string) ([]warehouse.WarehouseMeteringRow, error) {
    if a.client == nil { return nil, apperrors.ErrNotConnected }
    return warehouse.GetMeteringHistory(a.ctx, a.client, wh, start, end)
}
```

All SQL builders are pure functions. Only `AlterProperty`, `Suspend`, `Resume`,
`AbortAllQueries`, `Rename`, `GetParameters`, and `GetMeteringHistory` perform
network I/O.

## Gotchas

- Enum values for `size`, `warehouseType`, `autoResume`, `scalingPolicy`, and
  `enableQueryAcceleration` are validated against an explicit allowlist before
  being interpolated into SQL without quoting. Never skip this validation.
- `autoSuspend = 0` or empty string maps to `AUTO_SUSPEND = NULL` (disables
  auto-suspend) rather than zero seconds; this matches Snowflake's semantics.
- `GetParameters` filters the `SHOW PARAMETERS` output to only the three keys
  relevant to the warehouse settings panel. All other parameters are silently
  discarded.
- `ParseMeteringHistory` returns an empty (non-nil) slice when `res` is nil,
  consistent with the pattern used in `queryhistory.ParseQueryHistory`.
- Date strings passed to `BuildMeteringHistoryQuery` are cast to `TIMESTAMP_LTZ`
  inline (`'...'::TIMESTAMP_LTZ`) but are not validated by this package — invalid
  strings produce a Snowflake runtime error.
