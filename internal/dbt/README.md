# internal/dbt

> Scaffolds a new local dbt project pre-wired to the active Snowflake connection, writing the full file tree to disk.

## Responsibility

This package is a **pure file-generation** package for classic (local) dbt projects — not to be confused with `internal/dbtproject`, which manages Snowflake-native DBT PROJECT objects. `internal/dbt` fetches live Snowflake metadata (session context, table/view lists, view DDL bodies), builds in-memory data structures, then writes the complete dbt project tree (`dbt_project.yml`, `profiles.yml`, `models/staging/_sources.yml`, per-table staging model stubs) to the user's local filesystem via `internal/filesystem`.

## Key files

| File | Purpose |
|---|---|
| `generator.go` | `Generate(req, session, objects)` — pure file writer; plus the unexported naming/dedup helpers (`sourceName`, `stagingBase`, `sourceNameMap`, `stagingNameMap`) |
| `create.go` | `CreateProject(ctx, client, req, schemasMap)` — fetches live Snowflake data, builds `SchemaObjects`, optionally rewrites view SQL references into `{{ source(...) }}` / `{{ ref(...) }}` macros, then calls `Generate` |
| `generator_test.go` | Unit tests for generation logic |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `CreateRequest` | User-supplied parameters: project name, output dir, profile name, `InlineViewDefs`, `DatabaseVars` |
| `SessionInfo` | Live session values used to populate `profiles.yml` |
| `SchemaObjects` | Tables and views discovered in one `(db, schema)` pair; `ViewDefs` map holds inlined SELECT bodies when `InlineViewDefs` is true; `IsSystem` marks schemas skipped for object discovery |
| `CreateResult` | Return value: `ProjectDir`, `FilesCreated []string`, `Warnings []string` |
| `Generate(req, session, objects)` | Pure function — writes all project files; no Snowflake connection needed |
| `CreateProject(ctx, client, req, schemasMap)` | Orchestrator — queries Snowflake, builds `SchemaObjects`, rewrites view refs, calls `Generate` |

`Generate` and `CreateProject` are the package's only exported entry points. The naming helpers (`sourceName`/`stagingModelName` base names, `sourceNameMap`/`stagingNameMap` dedup maps) are unexported — there is no consistent standalone name without the full object set, so callers must go through `Generate`/`CreateProject`.

### Name uniqueness

dbt source and model names may only contain `[A-Za-z0-9_]`, so `_` is the only available separator. Because Snowflake identifiers can themselves contain `_`, the readable base names (`sourceName`/`stagingModelName`) are **not injective** — distinct scopes can map to the same string (`"A_B"."C"` and `"A"."B_C"` both → `a_b_c`; the same applies at the scope/table boundary and for leading/trailing underscores). No `_`-based scheme avoids this, so uniqueness is enforced at the **project level**: `sourceNameMap`/`stagingNameMap` walk `objects` in order and append a numeric suffix (`_2`, `_3`, …) to any base name already taken. `Generate` and `CreateProject` (for inlined `{{ source }}`/`{{ ref }}` rewrites) build the same maps, so on-disk names and references stay consistent. Discovery is iterated in sorted database order so the tie-break (and thus the generated names) is deterministic across runs. `multiScope` (the `stg_source_table` prefix) is decided from the count of schemas that actually produce stubs — system and empty schemas don't inflate it.

> **Migration note:** since the previous generator keyed multi-scope naming on `len(objects)`, a project regenerated after this change may rename staging files (e.g. a single data schema discovered alongside `INFORMATION_SCHEMA` now yields `stg_orders.sql` instead of `stg_db_public_orders.sql`). Update any hand-written `{{ ref(...) }}` accordingly after regenerating.

## Patterns & integration

`App.CreateDbtProject(req, schemasMap)` in `internal/app/dbtproject.go` is the thin delegator: nil-check → `dbt.CreateProject(...)` → return `*dbt.CreateResult`. All Snowflake queries and file I/O happen inside this package.

`CreateProject` is also exposed via the MCP tool `generate_dbt_project` in `internal/mcp/migration_tools.go`. The MCP tool is workspace-gated (only registered when a workspace root is configured) and validates that the output directory is inside the workspace root before delegating.

`InlineViewDefs` mode: when enabled, `CreateProject` fetches each view's DDL via `client.GetObjectDDL`, extracts the SELECT body with `snowflake.ExtractDDLBody`, then rewrites three-part Snowflake object references into `{{ source('...', '...') }}` (for tables) or `{{ ref('...') }}` (for views) using `snowflake.RewriteSQLReferences`. The rewrite is a best-effort text transformation — references not found in the discovered object set are left as-is.

`DatabaseVars` mode: when enabled, each database used in a source entry becomes a `vars:` entry in `dbt_project.yml` and the `database:` fields in `_sources.yml` reference them via `{{ var('db_mydb', 'MYDB') }}`.

## Gotchas

`INFORMATION_SCHEMA` is flagged as `IsSystem = true`: a source entry is written with `tables: []` and a note, but no object discovery or staging stubs are generated.

Schemas that yield no tables or views are skipped with a warning appended to `CreateResult.Warnings`, not a fatal error, so a partially-populated project is still returned.

File writes go through `filesystem.WriteFile`, which creates intermediate directories as needed. If any write fails the function returns immediately without cleaning up partially-written files.
