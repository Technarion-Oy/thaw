# internal/objects

> Generic Snowflake object metadata retrieval: SHOW/DESCRIBE query builders, column-comment list/set operations, and the STAGE two-query merge.

## Responsibility

Provides object-property queries and column-comment helpers that are not specific to any single Snowflake object type. The package builds the correct `SHOW` or `DESCRIBE` SQL for each object kind, executes it, and returns the result as `[]snowflake.PropertyPair` key/value pairs for the Properties panel. It also owns the `ColumnComment` type and the `INFORMATION_SCHEMA.COLUMNS` query that backs the "Set Column Comment" dialog.

## Key files

| File | Purpose |
|---|---|
| `objects.go` | All types, query builders, parsers, and high-level functions |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

### Types

| Type | Purpose |
|---|---|
| `ColumnComment` | `Column string`, `Comment string` — one row from the column-comments query |

### Query builders

| Function | Emits |
|---|---|
| `BuildObjectPropertiesQuery(db, schema, kind, name)` | `SHOW <OBJECTS> LIKE '<name>' IN [SCHEMA/DATABASE] <fqn>` — supports 18 object kinds |
| `BuildDescribeStageQuery(db, schema, name)` | `DESCRIBE STAGE <fqn>` — used to supplement SHOW STAGES output |
| `BuildGetColumnCommentsQuery(db, schema, table)` | `SELECT COLUMN_NAME, COALESCE(COMMENT,'') FROM <db>.INFORMATION_SCHEMA.COLUMNS WHERE ...` |
| `BuildSetColumnCommentSql(db, schema, table, column, comment)` | `ALTER TABLE ... MODIFY COLUMN ... COMMENT '...'` |

### Parsers

| Function | Input | Output |
|---|---|---|
| `ParseColumnComments(res)` | `*snowflake.QueryResult` | `[]ColumnComment` in ordinal order |

### High-level functions

| Function | Description |
|---|---|
| `GetObjectProperties(ctx, client, db, schema, kind, name)` | Runs `BuildObjectPropertiesQuery`, calls `snowflake.ResultToPairs`; for STAGE also runs `BuildDescribeStageQuery` and appends the additional properties |
| `GetColumnComments(ctx, client, db, schema, table)` | Runs `BuildGetColumnCommentsQuery`, calls `ParseColumnComments` |
| `SetColumnComment(ctx, client, db, schema, table, column, comment)` | Runs `BuildSetColumnCommentSql` via `client.Execute` |

## Patterns & integration

`*App` in `internal/app/objects.go`:
- `App.GetObjectProperties(db, schema, kind, name)` → `objects.GetObjectProperties`
- `App.GetColumnComments(db, schema, table)` → `objects.GetColumnComments`
- `App.SetColumnComment(db, schema, table, column, comment)` → `objects.SetColumnComment`

`GetObjectProperties` is the single entry point for the Properties side-panel. It handles all 20 supported object kinds (DATABASE, SCHEMA, TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, PIPE, SECRET, GIT REPOSITORY, DBT PROJECT, WAREHOUSE, ROLE, USER) through a `switch` on `kind`. STAGE additionally merges `DESCRIBE STAGE` rows (keyed as `parent.property`) to expose stage-specific configuration that `SHOW STAGES` omits.

`snowflake.ResultToPairs` (from `internal/snowflake/result.go`) converts a `SHOW` result row into `[]PropertyPair{Key, Value}` by pairing column names with cell values. `objects` does not duplicate this logic.

## Gotchas

`BuildObjectPropertiesQuery` escapes the `name` argument for use in a `LIKE` clause (backslash and single-quote escaping) but does **not** URI-encode it. The LIKE pattern uses exact match via `'<name>'` — no `%` wildcards — so names containing SQL LIKE metacharacters (`%`, `_`) will not match. This is generally safe because object names are retrieved from prior `ListObjects` calls.

`BuildSetColumnCommentSql` uses `MODIFY COLUMN ... COMMENT` (the older syntax compatible with all Snowflake versions) rather than `ALTER COLUMN ... COMMENT`. This is intentional for broader compatibility. Note that `internal/column.BuildSetColumnCommentSql` uses the newer `ALTER COLUMN ... COMMENT / UNSET COMMENT` form — the two builders coexist because `objects` targets the Properties panel "Set Comment" action while `column` targets the sidebar column context menu.
