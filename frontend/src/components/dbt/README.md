# frontend/src/components/dbt

> Three-step wizard for scaffolding a local dbt project from Snowflake schemas.

## Responsibility

Guides the user through configuring a new dbt project (name, profile, output directory),
selecting source schemas with cross-dependency hints, and triggering file generation via
the Go backend. Writes `profiles.yml`, `dbt_project.yml`, `_sources.yml`, stub models,
and supporting files to the local filesystem.

## Files

| File | Purpose |
|------|---------|
| `DbtProjectModal.tsx` | Three-step modal: Configure → Select Sources → Generate. Calls `CreateDbtProject` and displays the resulting file list. |

## Patterns & integration

**IPC calls:**
- `GetGitConfig` — pre-fills output directory from the git export path on mount
- `ListDatabases` / `ListSchemas` — populate the database/schema picker in Step 1 (lazy, on panel expand)
- `GetSchemaCrossDeps(db, schema)` — per-schema cross-dependency hints; triggers automatically when a schema is selected
- `GetDatabaseCrossDeps(db, uncachedSchemas)` — batched variant used by "Select all" to avoid concurrent connection-pool exhaustion
- `ListDirectory` — checks whether the target project directory already exists (warns user)
- `PickDirectory` — native OS directory picker via Wails
- `CreateDbtProject(req, schemasMap)` — backend generator that writes all project files; returns `dbt.CreateResult` (file list + warnings)

**Dependency hints:** Cross-schema references are computed lazily and cached per `"DB||SCHEMA"` key. A `useMemo`-derived `suggestedSet` shows which unselected schemas are referenced by the current selection. `selectAllSchemas` uses the batched `GetDatabaseCrossDeps` call and marks all schemas as `fetchingDeps` upfront to prevent duplicate concurrent requests.

**Mount safety:** `mountedRef` guards all async callbacks against state updates after unmount, including the React 18 Strict Mode remount cycle.

## Gotchas

- `CreateDbtProject` accepts a `schemasMap` (`Record<db, schema[]>`) built from the `selectedSchemas` state. The `Record<string, Set<string>>` is serialised to `Record<string, string[]>` at call time.
- `INFORMATION_SCHEMA` is flagged as a system schema with a warning tooltip; it can be selected but no staging stubs are generated for it by the backend.
