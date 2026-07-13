# components/column

> Table-column administration UI.

## Components

| File | Purpose |
|---|---|
| `ColumnPropertiesModal.tsx` | The single **Column Properties** modal opened from a table column's **Properties…** context-menu item. Consolidates every column-modification action into inline-editable sections. |

## ColumnPropertiesModal

Receives `{ db, schema, table, column, parentKind, initial, onClose, onChanged }`. The `initial` metadata (data type, nullable, primary-key flag, comment) comes from the sidebar column node; the current **default** and **masking policy** are fetched on open via `GetColumnDetails`, and current column **tags** via `GetColumnTagReferences`.

Each editable section:

- **Name** — `RENAME COLUMN` with an `ObjectNameCaseControl` case-sensitivity toggle. Closes the modal afterwards, since the column identity changes.
- **Data type** — `SET DATA TYPE` via `DataTypeSelect`.
- **Nullable** — a switch issuing `SET NOT NULL` / `DROP NOT NULL`; disabled (shown as NOT NULL) for primary-key columns.
- **Default value** — `SET DEFAULT <expr>` / `DROP DEFAULT` (empty clears). Free-text plus a **sequence picker** (`SequenceDefaultPicker`, fed by `ListAccountSequences` → `SHOW SEQUENCES IN ACCOUNT`) that inserts a `<seq>.NEXTVAL` reference. On an *existing* column Snowflake's `ALTER … ALTER COLUMN … SET DEFAULT` accepts only a sequence reference — function defaults like `CURRENT_TIMESTAMP()` are valid solely at table-create time, so the built-in-function ƒ picker used by Create Table / the ER Designer is deliberately not offered here (#614).
- **Comment** — `COMMENT …` / `UNSET COMMENT` (empty clears).
- **Masking policy** — `SET / UNSET MASKING POLICY`, chosen from a searchable dropdown populated by `ListAccountMaskingPolicies` (`SHOW MASKING POLICIES IN ACCOUNT`); clearing the dropdown unsets the policy.
- **Tags** — removable chips plus an add row whose tag-name field is a dropdown populated by `ListAccountTags` (`SHOW TAGS IN ACCOUNT`); applied via the shared tag governance IPC (`SetObjectTag` / `UnsetObjectTag`) with an `ObjectTagRef` of domain `COLUMN`. The chip list is loaded once on open from `GetColumnTagReferences` and updated optimistically after a set/unset — `TAG_REFERENCES_ALL_COLUMNS` has propagation latency, so refetching after a mutation would show stale rows. Selecting a tag name reads that tag's `allowed_values` from the `SHOW TAGS` catalog already loaded by `ListAccountTags` (parsed with the shared `components/tag/allowedValues` helper): if the tag defines an allowed-values whitelist the value field becomes a dropdown of those values, otherwise it stays a free-text input.

Every mutating section builds its SQL through the backend `internal/column` `Build*Sql` IPC methods and runs it with `ExecDDL`; on success it calls `onChanged` to refresh the table's columns in the sidebar. Safe edits execute immediately — only an edit that can lose or truncate data (currently just **Change data type**) shows a confirmation dialog with a warning and a theme-aware SQL preview. Tags reuse the existing tag-governance flow, which executes directly.

The whole modal is gated behind the `columnManagement` feature flag at its call site (`Sidebar.tsx`).
