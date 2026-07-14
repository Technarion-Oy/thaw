# internal/table

> Table-summary queries, modifiable-settings retrieval, and ALTER TABLE SQL builders for Snowflake table administration.

## Responsibility

Provides four related capabilities for managing Snowflake tables:

1. **Database-wide table summaries** — queries `INFORMATION_SCHEMA.TABLES` for all
   physical tables in a database and parses the result into typed `TableSummary` rows.
2. **Table settings retrieval** — reads the current values of modifiable table
   properties via `SHOW TABLES` (with a `SHOW PARAMETERS` fallback for
   `DEFAULT_DDL_COLLATION`) and returns a typed `TableSettings` struct.
3. **ALTER TABLE SQL builders** — constructs a single-property `ALTER TABLE SET`
   statement for any supported table property.
4. **Single-row INSERT builder** — renders an `INSERT INTO … (cols) VALUES (…)`
   statement from per-column form values, quoting each value as a typed literal
   (or emitting NULL / DEFAULT / a raw expression). Backs the Insert Row modal.

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (Object Browser & Administration) |
| `table.go` | Table-summary / settings types and functions |
| `insert.go` | `InsertRowConfig` / `InsertRowValue` types + `BuildInsertRowSql` and its per-type literal rendering |
| `table_test.go` | Unit tests for the summary/settings builders and parsers |
| `insert_test.go` | Unit tests for `BuildInsertRowSql` (typed literals, NULL/DEFAULT/expression, escaping) |

## Key types & functions

### `TableSummary`
```go
type TableSummary struct {
    Name          string `json:"name"`
    Schema        string `json:"schema"`
    Kind          string `json:"kind"`      // BASE TABLE, TRANSIENT, TEMPORARY
    Rows          int64  `json:"rows"`
    Bytes         int64  `json:"bytes"`
    Owner         string `json:"owner"`
    RetentionTime int    `json:"retentionTime"`
    Created       string `json:"created"`   // RFC3339 string
    LastAltered   string `json:"lastAltered"` // RFC3339 string
    Comment       string `json:"comment"`
}
```
`time.Time` values are converted to RFC3339 strings for Wails IPC compatibility
(the Wails type system does not handle `time.Time` cleanly in all configurations).

### `TableSettings`
```go
type TableSettings struct {
    ClusterBy             string `json:"clusterBy"`
    EnableSchemaEvolution bool   `json:"enableSchemaEvolution"`
    DataRetentionDays     int    `json:"dataRetentionDays"`
    MaxDataExtensionDays  int    `json:"maxDataExtensionDays"`
    ChangeTracking        bool   `json:"changeTracking"`
    DefaultDDLCollation   string `json:"defaultDDLCollation"`
    Comment               string `json:"comment"`
}
```

### `InsertRowConfig` / `InsertRowValue`
```go
type InsertRowValue struct {
    Column   string `json:"column"`   // column name (quoted via QuoteIdent)
    DataType string `json:"dataType"` // e.g. "NUMBER(38,0)" — drives literal rendering
    Mode     string `json:"mode"`     // "value" | "null" | "default" | "expression"
    Value    string `json:"value"`    // literal text, or raw expression in "expression" mode
}

type InsertRowConfig struct {
    Values []InsertRowValue `json:"values"`
}
```
`Mode` selects rendering: `value` renders a typed literal per `DataType` (numeric
and boolean literals bare when valid, everything else single-quoted; an invalid
numeric is quoted so no injection escapes the literal); `null`/`default` emit the
`NULL`/`DEFAULT` keyword; `expression` emits `Value` verbatim (function-picker
values such as `CURRENT_TIMESTAMP()`). Entries with an empty column name are
skipped so a partially-filled form still yields valid preview SQL.

### Functions

| Function | Description |
|----------|-------------|
| `BuildInsertRowSql(db, schema, tableName string, cfg InsertRowConfig) (string, error)` | Builds a single-row `INSERT INTO … (cols) VALUES (…)`; always returns a nil error (IPC symmetry) |
| `BuildDatabaseTableSummaryQuery(database string) string` | Returns `INFORMATION_SCHEMA.TABLES` SELECT for all physical tables |
| `ParseDatabaseTableSummary(res *snowflake.QueryResult) []TableSummary` | Projects query result into `[]TableSummary` by positional column index |
| `GetDatabaseTableSummary(ctx, client, database) ([]TableSummary, error)` | Convenience wrapper: build + query + parse |
| `GetTableSettings(ctx, client, database, schema, tbl) (TableSettings, error)` | `SHOW TABLES LIKE '...' IN SCHEMA` + optional `SHOW PARAMETERS` fallback |
| `BuildAlterTablePropertySQL(database, schema, tbl, property, value string) (string, error)` | Single-property ALTER TABLE SET builder |
| `AlterProperty(ctx, client, database, schema, tbl, property, value string) error` | Executes `BuildAlterTablePropertySQL` result via `client.Execute` |

### `BuildAlterTablePropertySQL` — supported properties

| `property` key | SQL clause |
|----------------|-----------|
| `clusterBy` | `CLUSTER BY (...)` or `DROP CLUSTERING KEY` when value is empty |
| `enableSchemaEvolution` | `SET ENABLE_SCHEMA_EVOLUTION = TRUE/FALSE` |
| `dataRetentionDays` | `SET DATA_RETENTION_TIME_IN_DAYS = N` |
| `maxDataExtensionDays` | `SET MAX_DATA_EXTENSION_TIME_IN_DAYS = N` |
| `changeTracking` | `SET CHANGE_TRACKING = TRUE/FALSE` |
| `defaultDDLCollation` | `SET DEFAULT_DDL_COLLATION = '...'` |
| `comment` | `SET COMMENT = '...'` |

Returns an error for any unknown property key.

## Patterns & integration (thin-delegator)

```go
// internal/app/table.go (illustrative)
func (a *App) GetTableSettings(db, schema, tbl string) (table.TableSettings, error) {
    if a.client == nil { return table.TableSettings{}, apperrors.ErrNotConnected }
    return table.GetTableSettings(a.ctx, a.client, db, schema, tbl)
}
```

The builder functions (`BuildDatabaseTableSummaryQuery`, `BuildAlterTablePropertySQL`)
are pure and unit-testable without a live connection. Only `GetDatabaseTableSummary`,
`GetTableSettings`, and `AlterProperty` require a `*snowflake.Client`.

## Gotchas

- `ParseDatabaseTableSummary` accesses columns by fixed positional index (0–9)
  rather than name lookup. If the `INFORMATION_SCHEMA.TABLES` column set ever
  changes, this parser will silently misread values. `GetTableSettings` uses a
  name-keyed map and is immune to this issue.
- `GetTableSettings` does a case-insensitive exact-name match against `SHOW TABLES
  LIKE` results because `LIKE` can match partial names. This requires a second
  scan of the result set after the driver returns rows.
- `DEFAULT_DDL_COLLATION` may not appear in older `SHOW TABLES` output; the
  `SHOW PARAMETERS LIKE 'DEFAULT_DDL_COLLATION' IN TABLE` fallback handles this.
  Both queries are executed even when the first succeeds and returns an empty string.
- `time.Time` fields from the Snowflake driver are type-asserted with `ok` checks;
  non-`time.Time` values (e.g. string timestamps in some driver versions) result
  in empty `Created` / `LastAltered` strings rather than an error.
- `BuildInsertRowSql` does no per-row-type validation beyond quoting — a `NULL`
  mode on a `NOT NULL` column, or a `DEFAULT` mode on a column without a default,
  produces valid SQL that Snowflake rejects at execution time. The form surfaces
  that error via `ExecDDL`; the live `SqlPreview` shows exactly what will run.
- `"expression"` mode is emitted verbatim: it is the intentional raw-SQL escape
  hatch (function picker / hand-typed expressions), so its correctness and safety
  are the caller's responsibility — never route untrusted input through it.
