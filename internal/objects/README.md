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
| `BuildObjectPropertiesQuery(db, schema, kind, name)` | `SHOW <OBJECTS> LIKE '<name>' IN [SCHEMA/DATABASE] <fqn>` — supports 27 object kinds |
| `BuildDescribeStageQuery(db, schema, name)` | `DESCRIBE STAGE <fqn>` — used to supplement SHOW STAGES output |
| `BuildDescribeMaskingPolicyQuery(db, schema, name)` | `DESCRIBE MASKING POLICY <fqn>` — used to supplement SHOW MASKING POLICIES with the signature, return type, and body |
| `BuildDescribeRowAccessPolicyQuery(db, schema, name)` | `DESCRIBE ROW ACCESS POLICY <fqn>` — used to supplement SHOW ROW ACCESS POLICIES with the signature, return type, and body |
| `BuildDescribeNetworkRuleQuery(db, schema, name)` | `DESCRIBE NETWORK RULE <fqn>` — used to supplement SHOW NETWORK RULES with the value_list (SHOW reports only a count) |
| `BuildDescribeServiceQuery(db, schema, name)` | `DESCRIBE SERVICE <fqn>` — used to supplement SHOW SERVICES with the `spec` / `dns_name` |
| `BuildDescribeStreamlitQuery(db, schema, name)` | `DESCRIBE STREAMLIT <fqn>` — used to supplement SHOW STREAMLITS with the `root_location` / `main_file` (SHOW omits both) |
| `BuildGetColumnCommentsQuery(db, schema, table)` | `SELECT COLUMN_NAME, COALESCE(COMMENT,'') FROM <db>.INFORMATION_SCHEMA.COLUMNS WHERE ...` |
| `BuildSetColumnCommentSql(db, schema, table, column, comment)` | `ALTER TABLE ... MODIFY COLUMN ... COMMENT '...'` |

### Parsers

| Function | Input | Output |
|---|---|---|
| `ParseColumnComments(res)` | `*snowflake.QueryResult` | `[]ColumnComment` in ordinal order |

### High-level functions

| Function | Description |
|---|---|
| `GetObjectProperties(ctx, client, db, schema, kind, name)` | Runs `BuildObjectPropertiesQuery`, calls `snowflake.ResultToPairs`; for STAGE also runs `BuildDescribeStageQuery`, for MASKING POLICY also runs `BuildDescribeMaskingPolicyQuery`, for ROW ACCESS POLICY also runs `BuildDescribeRowAccessPolicyQuery`, for NETWORK RULE also runs `BuildDescribeNetworkRuleQuery`, for SERVICE also runs `BuildDescribeServiceQuery` (merging the `spec` / `dns_name`), and for STREAMLIT also runs `BuildDescribeStreamlitQuery` (merging the `root_location` / `main_file`), appending the additional properties |
| `GetColumnComments(ctx, client, db, schema, table)` | Runs `BuildGetColumnCommentsQuery`, calls `ParseColumnComments` |
| `SetColumnComment(ctx, client, db, schema, table, column, comment)` | Runs `BuildSetColumnCommentSql` via `client.Execute` |

## Patterns & integration

`*App` in `internal/app/objects.go`:
- `App.GetObjectProperties(db, schema, kind, name)` → `objects.GetObjectProperties`
- `App.GetColumnComments(db, schema, table)` → `objects.GetColumnComments`
- `App.SetColumnComment(db, schema, table, column, comment)` → `objects.SetColumnComment`

`GetObjectProperties` is the single entry point for the Properties side-panel. It handles all 35 supported object kinds (DATABASE, SCHEMA, TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE, ICEBERG TABLE, HYBRID TABLE, EVENT TABLE, MATERIALIZED VIEW, ALERT, TAG, MASKING POLICY, ROW ACCESS POLICY, PASSWORD POLICY, NETWORK RULE, IMAGE REPOSITORY, SERVICE, STREAMLIT, FUNCTION, EXTERNAL FUNCTION, DATA METRIC FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, PIPE, SECRET, GIT REPOSITORY, DBT PROJECT, WAREHOUSE, ROLE, USER) through a `switch` on `kind`. STAGE additionally merges `DESCRIBE STAGE` rows (keyed as `parent.property`), MASKING POLICY and ROW ACCESS POLICY merge the corresponding `DESCRIBE` signature / return type / body, NETWORK RULE merges the `DESCRIBE NETWORK RULE` value_list, SERVICE merges the `DESCRIBE SERVICE` spec / dns_name, and STREAMLIT merges the `DESCRIBE STREAMLIT` root_location / main_file, to expose configuration the corresponding `SHOW` omits.

`snowflake.ResultToPairs` (from `internal/snowflake/result.go`) converts a `SHOW` result row into `[]PropertyPair{Key, Value}` by pairing column names with cell values. `objects` does not duplicate this logic.

## Gotchas

`BuildObjectPropertiesQuery` escapes the `name` argument for use in a `LIKE` clause (backslash and single-quote escaping) but does **not** URI-encode it. The LIKE pattern uses exact match via `'<name>'` — no `%` wildcards — so names containing SQL LIKE metacharacters (`%`, `_`) will not match. This is generally safe because object names are retrieved from prior `ListObjects` calls.

`BuildSetColumnCommentSql` uses `MODIFY COLUMN ... COMMENT` (the older syntax compatible with all Snowflake versions) rather than `ALTER COLUMN ... COMMENT`. This is intentional for broader compatibility. Note that `internal/column.BuildSetColumnCommentSql` uses the newer `ALTER COLUMN ... COMMENT / UNSET COMMENT` form — the two builders coexist because `objects` targets the Properties panel "Set Comment" action while `column` targets the sidebar column context menu.
