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
| `SchemaPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "SCHEMA", name)` (SHOW SCHEMAS) plus `GetSchemaParameters(db, schema)` (SHOW PARAMETERS, the source for the parameters SHOW SCHEMAS omits). Renders inline-editable Comment, Data retention (days), Max data extension (days), Default DDL collation, a Managed access toggle (`ENABLE`/`DISABLE MANAGED ACCESS`), and a Rename row, plus a **Parameters** section of fixed-choice `SelectRow`s (Log level, Trace level, Storage serialization policy, Replace invalid characters) that `SET` on pick / `UNSET` to reset — all applied via `AlterSchema(db, schema, clause)`. Remaining SHOW SCHEMAS columns render read-only. |

## Patterns & integration

**IPC calls:**
- `GetObjectProperties(db, schema, "SCHEMA", name)` — properties panel data
- `GetSchemaParameters(db, schema)` — SHOW PARAMETERS fallback for parameters the
  SHOW dump omits
- `AlterSchema(db, schema, clause)` — `SET/UNSET COMMENT`, `SET/UNSET
  DATA_RETENTION_TIME_IN_DAYS` / `MAX_DATA_EXTENSION_TIME_IN_DAYS` /
  `DEFAULT_DDL_COLLATION`, `ENABLE`/`DISABLE MANAGED ACCESS`, `RENAME TO`

## Gotchas

- Schemas are **two-level** (`<db>.<schema>`), so `AlterSchema` takes `(db,
  schema, clause)` — no third `name`, and it cannot reuse the three-level
  `alterObject` helper on the backend.
- `RENAME TO` makes the modal's `name` prop stale, so a successful rename closes
  the modal; refresh the sidebar to see the new name.
- The modal mirrors `EventTablePropertiesModal` (same numeric retention /
  extension + SHOW-PARAMETERS-fallback shape).
