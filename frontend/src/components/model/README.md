# components/model

> Modals for creating and managing Snowflake MODEL (ML Model Registry) objects.

## Components

| File | Purpose |
|---|---|
| `CreateModelModal.tsx` | Create form with a live `CREATE MODEL` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other), optional `WITH VERSION` name, and the shared **`ModelSourcePicker`**. |
| `ModelSourcePicker.tsx` | Reusable FROM-clause source selector for `CREATE MODEL` and `ALTER MODEL … ADD VERSION`: a **Source** toggle between *copy an existing model* (a searchable model dropdown from `ListModels` + optional source-version input) and *load from an internal stage* (an editable stage-path input plus the shared `components/shared/StageFilePicker` browser). The `ListModels` result (`SHOW MODELS IN ACCOUNT`, an account-wide scan) is cached at module scope so it runs at most once per session despite the picker remounting on every modal open; `invalidateModelsCache()` (called after a successful `CreateModel`) clears it. Props: `db`, `schema`, `value: ModelSourceValue`, `onChange(patch)`. Used by both the create modal and the Add-version dialog so the model dropdown + stage browser live in one place. |
| `ModelPropertiesModal.tsx` | `SHOW MODELS` metadata: an **Overview** (model type, aliases), inline-editable **Default version** (`SET DEFAULT_VERSION`) and **Comment** (`SET`/`UNSET COMMENT`), a **Tags** editor (chips from `GetModelTags`; add → `SET TAG`, chip close → `UNSET TAG`), a lazily-loaded **Versions** table (`SHOW VERSIONS IN MODEL`) with a per-row Actions menu (`VERSION … SET/UNSET ALIAS`, `MODIFY VERSION … SET COMMENT/METADATA`, `DROP VERSION`) and an **Add version…** dialog (`ADD VERSION … FROM MODEL/stage`, reusing `ModelSourcePicker`), and the generic property rows. This surfaces every `ALTER MODEL` clause (RENAME is the one exception — it lives in the sidebar context menu). |

## Integration

- Both delegate to IPC: `BuildCreateModelSql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterModel` / `ListModelVersions` (properties + edits +
  version listing).
- `AlterModel(db, schema, name, clause)` runs free-form `ALTER MODEL … <clause>`
  and is the single entry point for every mutation: `SET COMMENT`,
  `SET DEFAULT_VERSION`, `SET`/`UNSET TAG`, `VERSION … SET`/`UNSET ALIAS`,
  `MODIFY VERSION … SET COMMENT`/`METADATA`, `ADD VERSION`, `DROP VERSION`, and the
  context-menu **Rename** (`RENAME TO`).
- `GetModelTags(db, schema, name)` reads the currently-applied tags from
  `INFORMATION_SCHEMA.TAG_REFERENCES` (object domain MODEL); the read is
  best-effort (an error shows a warning but `SET`/`UNSET TAG` still work).
- `ListModels()` (`SHOW MODELS IN ACCOUNT`) backs the source-model dropdown in
  `ModelSourcePicker` — every model the role can see, as quoted FQNs. A
  user-typed value not in the list is preserved as a selectable option.
- `ListModelVersions(db, schema, name)` returns the raw `snowflake.QueryResult`
  from `SHOW VERSIONS IN MODEL`; the modal builds an antd `Table` directly from
  `columns`/`rows`, adapting to whatever columns the Snowflake edition reports
  (typically created_on, name, is_default_version, is_last_version, aliases,
  comment). It is loaded on demand (not on open) to avoid an extra round-trip.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Models** group (kind `"MODEL"`). Models are not queryable tables, so there is
  no **Select Top 1000 Rows**; `ALTER MODEL` *does* support `RENAME TO`, so
  **Rename** is offered.
- The form shape mirrors the Wails-generated `model.ModelConfig`; a plain object
  literal is cast `as any` only at the IPC boundary.

## Gotchas

- **No `GET_DDL`** — the get_ddl object-type enumeration omits `MODEL`, so there's
  no DDL/View-Definition/comparison path; the properties panel relies on
  `SHOW MODELS` + `SHOW VERSIONS IN MODEL`.
- **CREATE MODEL has no `COMMENT`/`TAG` clause** — those are applied afterwards
  via `ALTER MODEL`, so the create modal omits them.
- Most models are registered through the Snowpark ML Python API; SQL
  `CREATE MODEL` only copies an existing model or loads from a stage.
