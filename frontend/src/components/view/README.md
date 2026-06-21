# frontend/src/components/view

> Modals for managing Snowflake View objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake View objects.
`CreateViewModal` follows the standard debounced SQL preview pattern.
`ViewPropertiesModal` shows `SHOW VIEWS` metadata, the rendered defining query,
and inline-editable settings (Comment and a SECURE toggle). Drop / Rename are
driven from the sidebar context menu via `App.AlterView`.

## Files

| File | Purpose |
|------|---------|
| `CreateViewModal.tsx` | `CREATE VIEW` form — name/options (OR REPLACE / IF NOT EXISTS), comment, and an embedded Monaco editor for the defining query with an **Insert from table** database→schema→table picker; an **Advanced options** `Collapse` covers explicit Columns, SECURE, RECURSIVE, COPY GRANTS, and view-level Tags. Uses `BuildCreateViewSql` for live SQL preview. |
| `ViewPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "VIEW", name)`; renders inline-editable Comment and a SECURE toggle (via `AlterView … SET/UNSET`), the remaining `SHOW VIEWS` properties, and the rendered defining query (`text` column). Views have no lifecycle (no Suspend/Resume). |

## Patterns & integration

**IPC calls:**
- `BuildCreateViewSql(db, schema, cfg)` — live SQL preview
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `GetObjectProperties(db, schema, "VIEW", name)` — properties panel data
- `AlterView(db, schema, name, clause)` — `SET SECURE` / `UNSET SECURE` / `SET COMMENT …` / `UNSET COMMENT`

**`view.ViewConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`,
`orReplace`, `secure`, `recursive`, `ifNotExists`, `copyGrants`, `comment`,
`columns`, `tags` (`{name, value}[]`), `query`. The Create modal keeps form state
in a local `VConfig` (the generated class carries a `convertValues` method that a
plain literal can't satisfy) and casts to the generated type only at the IPC
boundary.

**Shared components:** `CreateModalShell`, `NameWithReplaceOptions`,
`ObjectNameCaseControl`, `TagInput`, `MonacoSqlField`, `SqlPreview`.

## Gotchas

- **Create** stays disabled until the defining query is edited away from the seeded
  `DEFAULT_QUERY` placeholder — submitting the untouched template would `CREATE …
  AS SELECT * FROM my_source_table` and fail server-side.
- The properties panel reads the defining query from the `text` column of `SHOW
  VIEWS`; `comment`, `is_secure`, and `text` are excluded from the generic
  Properties table because they are surfaced in dedicated sections.
- Views have no lifecycle: there is no Suspend/Resume or behind-by status.
