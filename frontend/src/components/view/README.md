# frontend/src/components/view

> Modals for managing Snowflake View objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake View objects.
`CreateViewModal` follows the standard debounced SQL preview pattern.
`ViewPropertiesModal` shows `SHOW VIEWS` metadata, the rendered defining query,
and inline-editable settings (Rename, Comment, a SECURE toggle, a Change
Tracking on/off, and Tags). All edits route through `App.AlterView`. Drop is
driven from the sidebar context menu.

## Files

| File | Purpose |
|------|---------|
| `CreateViewModal.tsx` | `CREATE VIEW` form — name/options (OR REPLACE / IF NOT EXISTS), comment, and an embedded Monaco editor for the defining query with an **Insert from table** database→schema→table picker; an **Advanced options** `Collapse` covers explicit Columns, SECURE, RECURSIVE, COPY GRANTS, and view-level Tags. Uses `BuildCreateViewSql` for live SQL preview. |
| `ViewPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "VIEW", name)` and `GetObjectTagReferences("VIEW", …)`; renders inline-editable Rename, Comment, a SECURE toggle, a Change Tracking on/off `Select`, and the shared `TagsRow` editor (all via `AlterView … RENAME TO / SET / UNSET`), plus the remaining `SHOW VIEWS` properties and the rendered defining query (`text` column). Rename is in-place within the same schema, using the shared `identToken`/`quoteIdent` (case-folding by default) so it matches the sidebar Rename dialog; it fires `onSuccess` (sidebar refresh) and closes. Inherited tags are shown but not removable. Views have no lifecycle (no Suspend/Resume). |

## Patterns & integration

**IPC calls:**
- `BuildCreateViewSql(db, schema, cfg)` — live SQL preview
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `GetObjectProperties(db, schema, "VIEW", name)` — properties panel data
- `GetObjectTagReferences("VIEW", db, schema, name, "")` — current tag applications (no-latency `INFORMATION_SCHEMA.TAG_REFERENCES`)
- `AlterView(db, schema, name, clause)` — `RENAME TO …` / `SET SECURE` / `UNSET SECURE` / `SET COMMENT …` / `UNSET COMMENT` / `SET CHANGE_TRACKING = TRUE|FALSE` / `SET TAG …` / `UNSET TAG …`

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
  VIEWS`; `comment`, `is_secure`, `text`, and `change_tracking` are excluded from
  the generic Properties table because they are surfaced in dedicated sections.
- Rename is in-place within the current schema (bare name only) via the shared
  `identToken(name, false)`, so it folds to uppercase by default exactly like the
  sidebar's Rename dialog — the two entry points can't diverge on stored casing.
  Cross-schema moves stay in the SQL editor.
- Policy attach/detach (masking / row-access / aggregation / projection / join)
  and column-level ALTER COLUMN edits are deliberately left to follow-ups — they
  need a policy picker (issue #618).
- Views have no lifecycle: there is no Suspend/Resume or behind-by status.
