# internal/dbtproject

> SQL builders for Snowflake-native DBT PROJECT objects: CREATE, ALTER SET/UNSET, EXECUTE, ADD VERSION, and DESCRIBE.

## Responsibility

Owns all SQL string construction for Snowflake-native DBT PROJECT objects. This is distinct from `internal/dbt`, which scaffolds local dbt project file trees. `internal/dbtproject` never executes SQL itself; it only builds and returns strings. The `*App` in `internal/app/dbtproject.go` calls these builders and delegates execution to `a.client`.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | All config types, `DbtVersionInfo`, and `Build*` builder functions |
| `sql_test.go` | Unit tests for the builders |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

### Config types

| Type | Used by |
|---|---|
| `CreateConfig` | `BuildCreateDbtProjectSql` — name, source location, dbt version, default target, external access integrations, comment, `OrReplace`/`IfNotExists`/`CaseSensitive` flags |
| `AlterSetConfig` | `BuildAlterDbtProjectSetSql` — dbt version, default target, external access integrations, comment |
| `ExecuteConfig` | `BuildExecuteDbtProjectSql` — args, dbt version, `FromWorkspace`, project root |
| `DbtVersionInfo` | Row type returned by `SYSTEM$SUPPORTED_DBT_VERSIONS()` — `DbtVersion`, `Type` |

### SQL builders

| Function | Emits |
|---|---|
| `BuildDescribeSql(db, schema, name)` | `DESCRIBE DBT PROJECT <fqn>;` |
| `BuildCreateDbtProjectSql(db, schema, cfg)` | `CREATE [OR REPLACE] DBT PROJECT [IF NOT EXISTS] <fqn> FROM '<location>' [...];` |
| `BuildAlterDbtProjectSetSql(db, schema, name, cfg, origComment, ...)` | One or two statements: `ALTER DBT PROJECT <fqn> SET ...;` and/or `UNSET ...;` |
| `BuildExecuteDbtProjectSql(db, schema, name, cfg)` | `EXECUTE DBT PROJECT <fqn> [ARGS=...] [DBT_VERSION=...];` or workspace variant |
| `BuildAddVersionSql(db, schema, name, alias, sourceLocation)` | `ALTER DBT PROJECT <fqn> ADD VERSION [<alias>] FROM '<location>';` |

## Patterns & integration

`*App` delegators in `internal/app/dbtproject.go`:
- `DescribeDbtProject` — calls `BuildDescribeSql`, executes, calls `snowflake.ResultToPairs`
- `ListSupportedDbtVersions` — calls `SYSTEM$SUPPORTED_DBT_VERSIONS()`, parses into `[]DbtVersionInfo`
- `ListDbtProjectVersions` — delegates to `client` directly (uses `snowflake.DbtProjectVersion`)
- `ListDbtProjectEntries` — delegates to `client.ListStageEntries`
- `CreateDbtProject` — delegates to `dbt.CreateProject` (the local scaffolding package)

`BuildAlterDbtProjectSetSql` compares new vs. original values (passed by the caller) to emit only changed SET clauses and required UNSET clauses in separate statements. Integration name comparison is case-insensitive to account for Snowflake's identifier uppercasing.

The frontend modals (`CreateDbtProjectModal`, `AlterDbtProjectModal`, `ExecuteDbtProjectModal`) call the corresponding `Build*Sql` IPC method on each keystroke to render a live SQL preview, following the same debounced-preview pattern used by `AddColumnModal`.

## Gotchas

`BuildCreateDbtProjectSql` uses `"project_name"` as a placeholder when `cfg.Name` is empty so the preview SQL remains syntactically valid. The frontend's `canSubmit` guard prevents submission with an empty name.

`BuildAlterDbtProjectSetSql` returns `([]string, error)` — it may return two statements (SET and UNSET) or an empty slice if nothing changed. The caller in `internal/app/dbtproject.go` must execute each statement in sequence.

`sourceLocation` is required for both `BuildCreateDbtProjectSql` and `BuildAddVersionSql`; both return an error if it is empty.
