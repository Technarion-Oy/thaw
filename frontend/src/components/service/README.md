# components/service

> Modals for creating and managing Snowflake SERVICE (Snowpark Container Services) objects.

## Components

| File | Purpose |
|---|---|
| `CreateServiceModal.tsx` | Create form with a live `CREATE SERVICE` SQL preview. Fields: name, IF NOT EXISTS (no OR REPLACE — Snowflake doesn't support it), compute pool (picker from `ListComputePools`), specification (inline YAML textarea **or** staged file via `StageFilePicker`), a **Template** toggle that switches to `SPECIFICATION_TEMPLATE[_FILE]` and reveals a **Template variables** (`USING`) key/value editor, min/max instances, auto resume, query warehouse (picker), external access integrations, and a comment. |
| `StageFilePicker.tsx` | Browse-and-pick widget for the staged specification file: database/schema selectors, an **internal-only** stage `Select` (`ListStages` filtered to `INTERNAL`, per the Snowflake docs), and a lazily-expanded file `Tree` (`ListStageEntries`). Picking a file fills the modal's stage (fully-qualified, quoted) and file-path fields, which remain manually editable. |
| `ServicePropertiesModal.tsx` | `SHOW SERVICES` + `DESCRIBE SERVICE` metadata: a **Status** tag, inline-editable **Settings** (comment, min/max instances, auto resume, query warehouse via `ALTER SERVICE SET/UNSET`), a read-only **Specification** (YAML), lazily-loaded **Endpoints** (`SHOW ENDPOINTS IN SERVICE`), **Containers** (`SHOW SERVICE CONTAINERS IN SERVICE`), and **Logs** (`SYSTEM$GET_SERVICE_LOGS`, with container/instance/lines inputs), plus the generic property rows. |

## Integration

- Create delegates to `BuildCreateServiceSql` / `ExecDDL` and reads
  `ListComputePools` / `ListWarehouses` for the pickers.
- Properties delegates to `GetObjectProperties` (SHOW + DESCRIBE), `AlterService`
  (lifecycle/edit clauses), `ListServiceEndpoints`, `GetServiceContainers`, and
  `GetServiceLogs`.
- `AlterService(db, schema, name, clause)` runs free-form `ALTER SERVICE …
  <clause>` for SUSPEND/RESUME (from the sidebar) and SET/UNSET of the mutable
  properties (from the modal).
- The lazy tables build an antd `Table` directly from the raw
  `snowflake.QueryResult` `columns`/`rows`, so they adapt to whatever columns the
  Snowflake edition reports. They load on demand (not on open) to avoid extra
  round-trips.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Services** group (kind `"SERVICE"`), with **Suspend** / **Resume** lifecycle
  actions. Services are not queryable tables, so there is no **Select Top 1000
  Rows**; `ALTER SERVICE` has no `RENAME TO`, so **Rename** is not offered.
- The form shape mirrors the Wails-generated `service.ServiceConfig`; a plain
  object literal is cast `as any` only at the IPC boundary.

## Gotchas

- **No `OR REPLACE`, no `RENAME`, no `GET_DDL`** — `CREATE SERVICE` has no OR
  REPLACE, services can't be renamed, and `GET_DDL` doesn't support the kind, so
  there's no DDL/View-Definition path; the properties panel relies on `SHOW
  SERVICES` + `DESCRIBE SERVICE`.
- **`SHOW SERVICES` omits the spec** — the YAML specification is fetched via
  `DESCRIBE SERVICE` (the `spec` column) and merged into the properties.
- **Suspend deletes containers** — suspending a service shuts down and removes its
  containers; resuming reconstructs them from the spec.
