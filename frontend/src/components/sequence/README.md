# frontend/src/components/sequence

> Modals for managing Snowflake Sequence objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake Sequence objects.
`CreateSequenceModal` follows the standard debounced SQL preview pattern.
`SequencePropertiesModal` shows `SHOW SEQUENCES` metadata with an inline-editable
Comment. Rename / Drop are driven from the sidebar context menu via
`App.AlterSequence`.

## Files

| File | Purpose |
|------|---------|
| `CreateSequenceModal.tsx` | `CREATE SEQUENCE` form — name/options (OR REPLACE / IF NOT EXISTS), case sensitivity, `Start With` / `Increment By` (`InputNumber`), an `Ordering` select (Default NOORDER / ORDER / NOORDER), and a comment. Uses `BuildCreateSequenceSql` for live SQL preview and `ExecDDL` on submit. |
| `SequencePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "SEQUENCE", name)`; renders an inline-editable Comment (via `AlterSequence … SET/UNSET COMMENT`) and the remaining `SHOW SEQUENCES` properties. Sequences have no lifecycle or secure/definition sections. |

## Patterns & integration

**IPC calls:**
- `BuildCreateSequenceSql(db, schema, cfg)` — live SQL preview
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `GetObjectProperties(db, schema, "SEQUENCE", name)` — properties panel data
- `AlterSequence(db, schema, name, clause)` — `SET COMMENT …` / `UNSET COMMENT`

**`sequence.SequenceConfig` type** from `wailsjs/go/models`: `name`,
`caseSensitive`, `orReplace`, `ifNotExists`, `start`, `increment`, `ordered`
(`""` / `"ORDER"` / `"NOORDER"`), `comment`. The Create modal keeps form state in
a local `SeqConfig` and casts to the generated type only at the IPC boundary.

**Shared components:** `CreateModalShell`, `NameWithReplaceOptions`,
`ObjectNameCaseControl`, `SqlPreview`; hooks `useQuotedIdentifiers`,
`useSqlPreview`, `useCreateSubmit` from `createModalHooks`.

## Gotchas

- `start` / `increment` default to `1`; the builder emits them verbatim.
- The default `ordered` value `""` maps to Snowflake's default (NOORDER) and
  emits no ORDER/NOORDER clause.
