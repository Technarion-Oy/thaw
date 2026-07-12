# frontend/src/components/schema

> Modal for viewing and editing Snowflake Schema properties.

## Responsibility

Provides the editable properties UI for Snowflake Schema objects, following the
same "consistent model" as the other per-object properties modals (editable
Settings section + read-only Properties section). Opened from the sidebar schema
context-menu "Properties" item.

## Files

| File | Purpose |
|------|---------|
| `SchemaPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "SCHEMA", name)` (SHOW SCHEMAS) plus `GetSchemaParameters(db, schema)` (SHOW PARAMETERS, the source for the parameters SHOW SCHEMAS omits). Renders inline-editable Comment, Data retention (days), Max data extension (days), Default DDL collation, a Managed access toggle (`ENABLE`/`DISABLE MANAGED ACCESS`), and a Rename row; a **Tags** section (shared `TagsRow`, current tags read via `GetObjectTagReferences("SCHEMA", …)`); a **Storage & Iceberg** section with live-list `PickerRow`s (External volume, Catalog, Catalog sync) plus Iceberg params (default DDL collation, version default, merge-on-read behavior/enable, base location prefix); a **Notebook & Streamlit** section of `PickerRow`s (notebook compute pool CPU/GPU, Streamlit warehouse); a **Parameters** section of fixed-choice `SelectRow`s (Log level, Trace level, Storage serialization policy, Replace invalid characters, Object visibility, Enable data compaction, Replicable with failover groups) that `SET` on pick / `UNSET` to reset; and a **Danger zone** `SWAP WITH` (sibling-schema picker + confirm dialog). All applied via `AlterSchema(db, schema, clause)`. Remaining SHOW SCHEMAS columns render read-only. **Deferred** (need backend list IPC or a bespoke editor): `CLASSIFICATION_PROFILE`, `SET/UNSET CONTACT`, `UNSET DCM PROJECT`. |
| Row components | `EditRow` / `InfoRow` / `SelectRow` / `PickerRow` and the `q1` / `opts` / `paramValue` helpers come from `../shared/PropertyRows.tsx` (shared with `DatabasePropertiesModal`). `PickerRow` is the identifier-valued row: a searchable `Select` populated from a live list loader (`ListExternalVolumes` / `ListIntegrations("CATALOG")` / `ListComputePools` / `ListWarehouses`); sets the picked name double-quoted, or unsets, falling back to showing the current value (unsettable) if the list read fails. |

## Patterns & integration

**IPC calls:**
- `GetObjectProperties(db, schema, "SCHEMA", name)` — properties panel data
- `GetSchemaParameters(db, schema)` — SHOW PARAMETERS fallback for parameters the
  SHOW dump omits
- `AlterSchema(db, schema, clause)` — every SET/UNSET clause plus `ENABLE`/`DISABLE
  MANAGED ACCESS`, `RENAME TO`, `SET/UNSET TAG`, and `SWAP WITH` (the backend
  accepts any trailing clause; the modal owns the SQL quoting)
- `GetObjectTagReferences("SCHEMA", db, schema, name, "")` — current tags (no-latency
  `INFORMATION_SCHEMA.TAG_REFERENCES` read; inherited rows shown non-removable)
- `ListExternalVolumes` / `ListIntegrations("CATALOG")` / `ListComputePools` /
  `ListWarehouses` — `PickerRow` option lists
- `ListUserSchemas(db)` — sibling targets for `SWAP WITH` (excludes the read-only
  INFORMATION_SCHEMA; current schema also filtered out)

## Gotchas

- Schemas are **two-level** (`<db>.<schema>`), so `AlterSchema` takes `(db,
  schema, clause)` — no third `name`, and it cannot reuse the three-level
  `alterObject` helper on the backend.
- `RENAME TO` makes the modal's `name` prop stale, so a successful rename closes
  the modal; refresh the sidebar to see the new name.
- The modal mirrors `EventTablePropertiesModal` (same numeric retention /
  extension + SHOW-PARAMETERS-fallback shape).
- `SWAP WITH` is destructive (exchanges all contents of the two schemas) and, like
  `RENAME TO`, invalidates the modal's `name` prop — it confirms first, then closes.
- Quoting follows the authoritative `ParseAlterSchema` grammar in
  `internal/sqlgrammar/alter.go`, **not** whether the value looks like an identifier:
  only `EXTERNAL_VOLUME` / `CATALOG` accept a bare identifier (`parseScalar` → double-
  quoted via `quoteIdent`, same as `SWAP WITH` / `RENAME TO`). `CATALOG_SYNC`, the
  notebook compute pools, the Streamlit warehouse, `ICEBERG_MERGE_ON_READ_BEHAVIOR`,
  the Iceberg text params, `BASE_LOCATION_PREFIX`, tag values, and the `'YES'`/`'NO'`
  of `REPLICABLE_WITH_FAILOVER_GROUPS` all require a **string literal** (`parseString`
  → single-quoted via `q1`). The `TRUE`/`FALSE` / `AUTO` / `PRIVILEGED` / policy enums
  are interpolated raw from their closed option list. When adding a row, check the
  matching `v.option(...)` in `alter.go` for `parseScalar` vs `parseString`.
