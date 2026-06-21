# frontend/src/components/stream

> Modals for managing Snowflake Stream objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake Stream (CDC) objects.
`CreateStreamModal` follows the standard debounced SQL preview pattern.
`StreamPropertiesModal` shows `SHOW STREAMS` metadata and an inline-editable
comment. The remaining operations (Drop / Rename) are driven from the sidebar
context menu in `layout/Sidebar.tsx` via `App.AlterStream`.

## Files

| File | Purpose |
|------|---------|
| `CreateStreamModal.tsx` | `CREATE STREAM` form — name/options (OR REPLACE / IF NOT EXISTS), a source-type `Select` (TABLE / VIEW / EXTERNAL TABLE / STAGE / DYNAMIC TABLE), a **database → schema → object picker** for the source (`ListDatabases`/`ListSchemas`/`ListObjects`, the object dropdown filtered to the chosen source type; the picked object's quoted FQN becomes `source`), comment, and an **Advanced options** `Collapse` whose CDC flags are gated by source type (APPEND_ONLY / SHOW_INITIAL_ROWS for table/view/dynamic-table, INSERT_ONLY for external-table; COPY GRANTS always) and which are reset when the source type changes. Uses `BuildCreateStreamSql` for live SQL preview. |
| `StreamPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "STREAM", name)`; renders an inline-editable Comment (via `AlterStream … SET/UNSET COMMENT`) and the remaining `SHOW STREAMS` properties (source_type, mode, stale, etc.). |

## Patterns & integration

**IPC calls:**
- `BuildCreateStreamSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency)
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `GetObjectProperties(db, schema, "STREAM", name)` — properties panel data
- `AlterStream(db, schema, name, clause)` — `SET COMMENT …` / `UNSET COMMENT`

**`stream.StreamConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`,
`orReplace`, `ifNotExists`, `copyGrants`, `sourceType`, `source`, `appendOnly`,
`showInitialRows`, `insertOnly`, `comment`. The Create modal keeps form state in a
local `StrConfig` (the generated class carries a `convertValues` method that a
plain literal can't satisfy) and casts to the generated type only at the IPC
boundary (`cfg as any`).

**Shared components:** `CreateModalShell`, `NameWithReplaceOptions`,
`ObjectNameCaseControl`, `SqlPreview`, and the `createModalHooks` hooks.

## Gotchas

- Streams have no lifecycle (suspend/resume) and no secure / defining-query
  surface; the properties panel only exposes an editable comment plus the raw
  `SHOW STREAMS` metadata.
- The create modal sets `source` to the picked object's fully-qualified quoted
  name (`"db"."schema"."object"`); the Go builder passes a dotted name through
  verbatim and only qualifies a bare (dot-less) `source` with the active
  db/schema.
