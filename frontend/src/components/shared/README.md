# frontend/src/components/shared

> Small, reusable UI primitives shared across multiple feature components.

## Responsibility

Provides the low-level building blocks used by modal forms throughout the application — a Snowflake identifier case-sensitivity control, a SQL preview pane, a Snowflake data-type picker, plus the shared skeleton for object-creation modals (a modal shell, a name+create-options row, a tag editor, a SQL editor with source picker, and the create-modal state hooks). None of these components contain domain logic except where noted; IPC calls are limited to `ObjectNameCaseControl` (reserved keywords), `MonacoSqlField` (object/column listing), and `createModalHooks` (the QUOTED_IDENTIFIERS flag).

## Files

| File | Purpose |
|---|---|
| `ObjectNameCaseControl.tsx` | Radio group for "Case insensitive" vs "Case sensitive" quoting of a Snowflake object name; exports `needsQuoting(name)`, `quoteIdent(name)`, and `identToken(name, caseSensitive)` helpers; loads the Snowflake reserved-keyword list once at module level via `GetSnowflakeKeywords` (from `wailsjs/go/sqleditor/Service`); shows an amber warning when quoting is forced by the name content, and an Ant Design `Alert` when `QUOTED_IDENTIFIERS_IGNORE_CASE` is TRUE for the session. |
| `SqlPreview.tsx` | Read-only `<pre>` block styled as a code box; props: `sql`, optional `placeholder`, `label` (default "SQL Preview"), `variant` (`compact` default, or `prominent` for the larger "Generated SQL" panel used inside the file-format / stage inline builders), and a `style` passthrough. Used by creation/alter modals to show the backend-generated SQL before execution. Tagged `@thaw-domain: Object Browser & Administration`. |
| `DataTypeSelect.tsx` | Ant Design `Select` for Snowflake base types plus inline `InputNumber` fields for precision+scale (NUMBER, DECIMAL, NUMERIC) or max length (VARCHAR, CHAR, STRING, TEXT, BINARY); parses and reconstructs type strings like `NUMBER(10,2)` or `VARCHAR(255)` using pure local logic; emits the full type string (e.g. `NUMBER(10,2)`) via `onChange`. |
| `CreateModalShell.tsx` | The shared chrome for object-creation modals: tinted-icon + title + muted subtitle header, the Cancel / Create footer (with `creating` loading + `canSubmit` disabling), and the dismissible error banner. Props include `icon`, `title`, `subtitle?`, `error?`/`errorTitle?`/`onErrorClose?`, `creating`, `canSubmit?`, `okText?`, `okIcon?` (footer button icon, defaults to `icon`), `width?`, `bodyMaxHeight?`, `onClose`, `onSubmit`, `children`. The form body is supplied as `children`. |
| `NameWithReplaceOptions.tsx` | The "object name + OR REPLACE / IF NOT EXISTS" header row. OR REPLACE and IF NOT EXISTS are mutually exclusive (matching Snowflake DDL). Accepts an `extra` slot for additional checkboxes (e.g. TRANSIENT for dynamic tables). |
| `TagInput.tsx` | Object-level tag editor — removable chips plus name/value draft inputs and an Add button. Owns the draft inputs internally; surfaces only the committed `{name,value}[]` via `onChange`. Adding a duplicate name replaces that tag's value. Exports the `TagItem` type. |
| `MonacoSqlField.tsx` | A Monaco SQL editor with a built-in "Insert from table" database/schema/table source picker; picking a table and clicking Insert SELECT drops a fully-qualified, column-listed SELECT into the editor (replacing an untouched `placeholder` body, else inserting at the cursor). IPC: `ListDatabases`, `ListSchemas`, `ListObjects`, `GetTableColumns`. `objectKinds` filters the table picker; `extraPickerRow` is a render-prop (receives `{db, schema, objects, loading, insert}`) used by the alert modal to add its procedure-CALL picker. Exports the `ExtraPickerCtx` type. |
| `createModalHooks.ts` | Create-modal state hooks: `useQuotedIdentifiers()` (reads QUOTED_IDENTIFIERS_IGNORE_CASE once), `useSqlPreview(build, deps)` (keeps a live preview string in sync with form state; `build` may be sync or async), and `useCreateSubmit()` (returns `{creating, error, setError, submit}`; `submit(run)` toggles the flag, clears errors, runs the action, captures throws — the action does its own `onSuccess`/`onClose`). |

## Patterns & integration

- **`ObjectNameCaseControl`** — IPC: `GetSnowflakeKeywords` via `wailsjs/go/sqleditor/Service` (not `app/App`) called once as an IIFE on module load; result cached in `_reservedKeywords`. The `quotedIdentifiersIgnoreCase` prop must be fetched by the parent via `GetQuotedIdentifiersIgnoreCase` IPC (from `app/App`) and passed in.
- **`SqlPreview`** — no IPC, no stores; purely presentational.
- **`DataTypeSelect`** — no IPC, no stores; all type parsing is local. The backend `IsNumeric`, `IsBoolean`, `NeedsQuotes` predicates exist for other contexts (e.g. `AddColumnModal` uses `IsNumeric` to decide whether to show the collation field) but are not used inside `DataTypeSelect` itself.
- **Import path**: `import ObjectNameCaseControl, { needsQuoting, quoteIdent, identToken } from "../shared/ObjectNameCaseControl"`. The named exports are used directly by `CreateDatabaseModal` and other modals that construct SQL identifier tokens without rendering the full control.

## Gotchas

- `ObjectNameCaseControl` forces "Case sensitive" mode (disables the "Case insensitive" radio) when the name requires quoting. The `forced` state is derived from `needsQuoting(name)`, which uses the lazily-loaded keyword set — for the brief window before `GetSnowflakeKeywords` resolves, only the character-pattern check (`UNQUOTED_IDENT_RE`) is applied. This is adequate for the vast majority of inputs.
- `DataTypeSelect` does NOT validate that the user-entered precision/scale combination is legal for Snowflake (e.g. scale must be ≤ precision). Downstream validation is expected from Snowflake at DDL execution time.
- Do not use `navigator.clipboard` anywhere in these components. Any future clipboard integration must use `ClipboardSetText` from `wailsjs/runtime/runtime`.
- `SqlPreview` uses `white-space: pre-wrap` + `word-break: break-all` so long SQL lines wrap rather than overflow the modal.
