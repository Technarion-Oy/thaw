# frontend/src/components/shared

> Small, reusable UI primitives shared across multiple feature components.

## Responsibility

Provides three low-level building blocks used by modal forms throughout the application: a Snowflake identifier case-sensitivity control, a SQL preview pane, and a Snowflake data-type picker with parameter inputs. None of these components contain domain logic or make IPC calls on their own (except `ObjectNameCaseControl`, which loads reserved keywords once at module level).

## Files

| File | Purpose |
|---|---|
| `ObjectNameCaseControl.tsx` | Radio group for "Case insensitive" vs "Case sensitive" quoting of a Snowflake object name; exports `needsQuoting(name)`, `quoteIdent(name)`, and `identToken(name, caseSensitive)` helpers; loads the Snowflake reserved-keyword list once at module level via `GetSnowflakeKeywords` (from `wailsjs/go/sqleditor/Service`); shows an amber warning when quoting is forced by the name content, and an Ant Design `Alert` when `QUOTED_IDENTIFIERS_IGNORE_CASE` is TRUE for the session. |
| `SqlPreview.tsx` | Read-only `<pre>` block styled as a dark code box; accepts a `sql` string and an optional `placeholder`; used by creation/alter modals to show the backend-generated SQL before execution. Tagged `@thaw-domain: Object Browser & Administration`. |
| `DataTypeSelect.tsx` | Ant Design `Select` for Snowflake base types plus inline `InputNumber` fields for precision+scale (NUMBER, DECIMAL, NUMERIC) or max length (VARCHAR, CHAR, STRING, TEXT, BINARY); parses and reconstructs type strings like `NUMBER(10,2)` or `VARCHAR(255)` using pure local logic; emits the full type string (e.g. `NUMBER(10,2)`) via `onChange`. |

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
