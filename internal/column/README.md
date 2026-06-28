# internal/column

> SQL builders for table column DDL: ADD COLUMN, DROP COLUMN, RENAME COLUMN, and ALTER COLUMN operations.

## Responsibility

Owns every SQL string for mutating table columns via `ALTER TABLE`. No column DDL is constructed inline in the frontend or in `internal/app`; all builders live here and are exposed over IPC through thin delegators in `internal/app/builders.go`. The builders are pure functions — no Snowflake connection is required — which makes them directly unit-testable.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `AddColumnConfig` type + all `Build*ColumnSql` builder functions |
| `sql_test.go` | Unit tests for every builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

### Config type

`AddColumnConfig` — mirrors the frontend's `AddColumnModal` form fields; maps cleanly to the Wails-generated TypeScript model. Fields cover name, data type, value mode (`none` / `default` / `autoincrement` / `computed`), identity parameters, inline constraint (UNIQUE / PRIMARY KEY / FOREIGN KEY), collation, and comment.

### SQL builders

| Function | Emits |
|---|---|
| `BuildAddColumnSql(db, schema, table, cfg AddColumnConfig)` | `ALTER TABLE ... ADD COLUMN [IF NOT EXISTS] ...;` |
| `BuildDropColumnSql(db, schema, table, column)` | `ALTER TABLE ... DROP COLUMN ...;` |
| `BuildRenameColumnSql(db, schema, table, old, new, caseSensitive)` | `ALTER TABLE ... RENAME COLUMN ... TO ...;` |
| `BuildSetNotNullSql(db, schema, table, column)` | `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL;` |
| `BuildDropNotNullSql(db, schema, table, column)` | `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL;` |
| `BuildSetColumnCommentSql(db, schema, table, column, comment)` | `ALTER TABLE ... ALTER COLUMN ... COMMENT ...;` or `UNSET COMMENT;` when empty |
| `BuildChangeDataTypeSql(db, schema, table, column, dataType)` | `ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE ...;` |
| `BuildSetColumnDefaultSql(db, schema, table, column, expr)` | `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT <expr>;` |
| `BuildDropColumnDefaultSql(db, schema, table, column)` | `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT;` |
| `BuildSetColumnMaskingPolicySql(db, schema, table, column, policyDb, policySchema, policyName)` | `ALTER TABLE ... ALTER COLUMN ... SET MASKING POLICY <policy>;` |
| `BuildUnsetColumnMaskingPolicySql(db, schema, table, column)` | `ALTER TABLE ... ALTER COLUMN ... UNSET MASKING POLICY;` |

All builders return a semicolon-terminated string. `BuildAddColumnSql` also returns an `error` for IPC signature symmetry, though it currently always returns `nil`.

## Patterns & integration

The `*App` in `internal/app/builders.go` exposes each builder as a public IPC method (e.g. `BuildAddColumnSql`, `BuildDropColumnSql`). These methods are pure SQL generators — they require no live connection and return strings the frontend executes via the separate `ExecDDL` IPC call.

The altering actions (Rename, Change Type, Default, Comment, Set/Drop NOT NULL, Masking Policy, Tags) are surfaced together in the frontend's `ColumnPropertiesModal` (opened via the column context menu's **Properties…** item); **Add Column…** and **Drop Column…** stay as standalone actions. All of these are gated behind the `columnManagement` feature flag in `featureFlagsStore`. Safe edits run immediately; only a data-loss-risk edit (a data-type change) shows a confirmation dialog. The current default expression and masking policy a column carries are read via `App.GetColumnDetails` (`snowflake.Client.GetColumnDetails`); the masking-policy and tag pick lists come from `App.ListAccountMaskingPolicies` and `App.ListAccountTags`.

## Gotchas

`BuildAddColumnSql` uses `"column_name"` as a placeholder when `cfg.Name` is empty so the live SQL preview remains valid SQL while the user is still typing. The frontend's `canSubmit` guard prevents actual submission with an empty name.

Collation (`COLLATE`) is emitted between the data type and `DEFAULT`/`AUTOINCREMENT` clauses to match Snowflake's column-definition grammar and `GET_DDL` output. It is skipped entirely for computed (`AS (expr)`) columns, where it is not valid.

Inline constraints (`NOT NULL`, `CONSTRAINT`, `UNIQUE`, `PRIMARY KEY`, `REFERENCES`) are also skipped for computed columns.
