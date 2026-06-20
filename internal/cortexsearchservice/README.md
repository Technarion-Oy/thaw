# internal/cortexsearchservice

> SQL builder for Snowflake CORTEX SEARCH SERVICE objects.

## Responsibility

Builds the `CREATE CORTEX SEARCH SERVICE` DDL from a structured config. A Cortex
Search Service is a schema-level object that indexes a text column over the rows
of a source query to provide low-latency hybrid (keyword + semantic) search and
retrieval — the backbone of Snowflake RAG applications.

Both documented CREATE shapes are modeled by the visual builder, selected by
`IndexMode`:

- **Single-index** (`IndexMode = "single"`): `ON <search_column>` plus an optional
  `EMBEDDING_MODEL` (vectorization model, cannot be altered later).
- **Multi-index** (`IndexMode = "multi"`): `TEXT INDEXES <col>, …` (optional) and
  `VECTOR INDEXES <spec>, …` (at least one required; each spec is a vector column
  or a managed-embedding text column such as `BODY (model='snowflake-arctic-embed-m')`).
  This form does **not** support `EMBEDDING_MODEL` or `IF NOT EXISTS`, so the
  builder drops them in this mode.

Shared by both shapes: optional `PRIMARY KEY ( <col>, … )`, filterable
`ATTRIBUTES <col>, …`, the required `WAREHOUSE` / `TARGET_LAG`, the tuning options
`REFRESH_MODE` (FULL/INCREMENTAL), `INITIALIZE` (ON_CREATE/ON_SCHEDULE),
`FULL_INDEX_BUILD_INTERVAL_DAYS`, `REQUEST_LOGGING`, `AUTO_SUSPEND`, a `COMMENT`,
and the defining query (`AS <query>`). Scoring profiles are added post-create via
`ALTER` rather than in `CREATE`.

Every `ALTER CORTEX SEARCH SERVICE` option is applied afterwards via free-form
`ALTER CORTEX SEARCH SERVICE <fqn> <clause>` statements from
`internal/app/cortexsearchservice.go` (`App.AlterCortexSearchService`), without a
dedicated builder, and is reachable from the properties modal:

- Lifecycle: `SUSPEND` / `RESUME` (optionally scoped to `INDEXING` or `SERVING`)
  and a manual `REFRESH`.
- `SET` / `UNSET`: `TARGET_LAG`, `WAREHOUSE`, `ATTRIBUTES`, `PRIMARY KEY`,
  `AUTO_SUSPEND` (NULL clears it), `FULL_INDEX_BUILD_INTERVAL_DAYS`,
  `REQUEST_LOGGING`, `COMMENT`.
- `SET` / `UNSET TAG` (current tags read via `App.GetCortexSearchServiceTags`).
- `ADD` / `DROP SCORING PROFILE` (raw scoring-profile definition).

`EMBEDDING_MODEL` is fixed at creation and so is not alterable.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `CortexSearchServiceConfig`, `BuildCreateCortexSearchServiceSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `CortexSearchServiceConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `IndexMode` (`"single"`/`"multi"`), `SearchColumn` (ON, single), `TextIndexes` / `VectorIndexes` (multi), `PrimaryKey`, `Attributes`, `Warehouse`, `TargetLag`, `EmbeddingModel` (single), `RefreshMode`, `Initialize`, `FullIndexBuildIntervalDays`, `RequestLogging`, `AutoSuspend`, `Comment`, `Query` (AS) |
| `IndexModeSingle` / `IndexModeMulti` | String constants for `IndexMode` |
| `BuildCreateCortexSearchServiceSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] CORTEX SEARCH SERVICE [IF NOT EXISTS] <fqn> ON <col> [ATTRIBUTES …] WAREHOUSE = … TARGET_LAG = '…' [EMBEDDING_MODEL = '…'] [COMMENT = '…'] AS <query>;` |
| `App.GetCortexSearchServiceTags(db, schema, name)` | Reads currently applied tags via `INFORMATION_SCHEMA.TAG_REFERENCES` for the properties modal's tag chips (best-effort; needs privileges) |

The column-list joining for the `SET ATTRIBUTES ( … )` / `SET PRIMARY KEY ( … )`
ALTER clauses is exposed over IPC by `App.FormatCortexSearchAttributes`, which
delegates to the shared `snowflake.JoinCleanList(cols, ", ")` helper (trim, drop
blanks, comma-join).

## Patterns & integration

- Blank required fields emit placeholders (`search_service_name`,
  `<search_column>`, `<warehouse>`, a default `1 hour` target lag, and a
  `SELECT id, text_column FROM <source_table>` query), so the live SQL preview
  reads as a completable template while the user is still typing.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `EMBEDDING_MODEL` names contain hyphens, so they are emitted as quoted string
  literals.
- `App.BuildCreateCortexSearchServiceSql` (in `internal/app/builders.go`) is the
  thin IPC delegator; `App.AlterCortexSearchService` (in
  `internal/app/cortexsearchservice.go`) runs the edit/lifecycle clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW CORTEX SEARCH SERVICES IN
  SCHEMA` with the fixed kind `"CORTEX SEARCH SERVICE"`. Cortex search services
  are not surfaced by `SHOW OBJECTS`, so — like services, image repositories,
  and models — no dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW CORTEX SEARCH SERVICES LIKE …`
  for the `CORTEX SEARCH SERVICE` kind and enriches it with `DESCRIBE CORTEX
  SEARCH SERVICE` (search column, attributes, embedding model, definition,
  target lag, warehouse, serving/indexing state, plus the mutable primary key,
  auto-suspend, request-logging, and full-index-build-interval values), which the
  SHOW output omits.

## Gotchas

- **`GET_DDL` is not supported** for cortex search services (the get_ddl
  object-type enumeration omits them), so there is no DDL export / "View
  Definition" / comparison path and no `buildGetDDLQuery` mapping for this kind.
  `App.GetObjectDDL` rejects the `CORTEX SEARCH SERVICE` kind up front, and the
  sidebar excludes it from the DDL-driven menu actions. The properties panel
  relies on `SHOW CORTEX SEARCH SERVICES` + `DESCRIBE CORTEX SEARCH SERVICE`.
- **`RENAME` is not supported** — `ALTER CORTEX SEARCH SERVICE` has no `RENAME
  TO`, so this kind is added to the sidebar's Rename-exclusion and given a
  dedicated "Properties…" item.
- **`EMBEDDING_MODEL` and `REFRESH_MODE`/`INITIALIZE` are immutable** after
  creation; the properties panel covers every other `ALTER` option
  (`TARGET_LAG`, `WAREHOUSE`, `ATTRIBUTES`, `PRIMARY KEY`, `AUTO_SUSPEND`,
  `FULL_INDEX_BUILD_INTERVAL_DAYS`, `REQUEST_LOGGING`, `COMMENT`, tags, and
  scoring profiles).
- **Lifecycle** — the service supports `SUSPEND`/`RESUME` (optionally scoped to
  `INDEXING` or `SERVING`, surfaced as a split dropdown) and a manual `REFRESH`.
