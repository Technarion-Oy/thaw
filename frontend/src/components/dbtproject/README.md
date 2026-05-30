# frontend/src/components/dbtproject

> Modals for managing Snowflake-native DBT PROJECT objects (CREATE, modify, add version, execute).

## Responsibility

Provides the full lifecycle UI for Snowflake-native `DBT PROJECT` objects that live inside
a schema. Each modal follows the debounced SQL preview pattern: form state changes call a
backend `Build*Sql` method with a 200 ms debounce; the resulting DDL is shown in a read-only
preview and executed via `ExecDDL` on submit.

## Files

| File | Purpose |
|------|---------|
| `CreateDbtProjectModal.tsx` | `CREATE DBT PROJECT` form. Uses `dbtproject.CreateConfig`, `SourceLocationPicker`, and `BuildCreateDbtProjectSql`. |
| `AddDbtProjectVersionModal.tsx` | Adds a new version to an existing DBT project. Calls `BuildAddDbtProjectVersionSql`. |
| `ExecuteDbtProjectModal.tsx` | Executes a DBT project run (version picker, target, flags). Calls `BuildExecuteDbtProjectSql`. |
| `ModifyDbtProjectModal.tsx` | `ALTER DBT PROJECT` form. Loads current properties via `GetObjectProperties`, uses `BuildModifyDbtProjectSql`. |
| `SourceLocationPicker.tsx` | Reusable stage/path picker used by Create and Modify modals to select `@stage/path` source locations. |

## Patterns & integration

**IPC calls (shared pattern):**
- `Build*DbtProjectSql(db, schema, cfg)` — 200 ms debounced; produces DDL string shown in `<SqlPreview>`
- `ExecDDL(sql)` — executes the previewed DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `<ObjectNameCaseControl>` for case-sensitivity display
- `ListExternalAccessIntegrations()` — populates the EAI multi-select in Create/Modify
- `ListSupportedDbtVersions()` — populates the dbt version picker
- `GetObjectProperties(db, schema, "DBT PROJECT", name)` — loads current values for Modify modal

**Shared components:**
- `ObjectNameCaseControl` — shows quoted-identifier quoting preview
- `SqlPreview` — read-only monospace SQL preview block
- `SourceLocationPicker` — lists available stages in the schema, lets user pick a path

**Domain types:** `dbtproject.CreateConfig`, `dbtproject.DbtVersionInfo` from `wailsjs/go/models`.

## Gotchas

- The `setCfg` spread (`{ ...prev, [key]: value }`) loses the Wails class prototype, which is intentional — Wails IPC uses JSON serialisation so only field values matter.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive; the Create modal enforces this by resetting `ifNotExists` when `orReplace` is checked.
