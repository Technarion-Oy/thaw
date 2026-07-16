# frontend/src/components/common

> Shared modal primitives used across multiple feature areas.

## Responsibility

Provides general-purpose modals for displaying and editing Snowflake object properties and session state. These components are consumed by `AccountPanel`, `Sidebar`, and other feature components and are intentionally free of domain-specific logic.

## Files

| File | Purpose |
|---|---|
| `PropertiesModal.tsx` | Displays a searchable key-value property table from `snowflake.PropertyPair[]`; optionally renders the editable `TableSettingsSection` (calls `GetTableSettings`, `AlterTableProperty`) when a `tableContext` prop is provided; includes Copy-all to clipboard. Booleans use the staged `ConfirmSwitch`. (Column-comment editing has moved to `components/column/ColumnPropertiesModal`.) |
| `SessionPropertiesModal.tsx` | Displays and allows inline editing of `snowflake.SessionParam[]` (session parameters) and `snowflake.SessionVar[]` (session variables); calls `SetSessionParameter` and `SetSessionVariable` IPC; Boolean params/vars are rendered as staged `ConfirmSwitch` toggles; non-boolean as editable text inputs; includes search and Copy-all. |
| `PropertyRows.tsx` | Shared building blocks for the per-property "Properties: <OBJECT>" modals (`WarehousePropertiesModal`, `UserPropertiesModal`): `EditRow` (typed inline-editable row — text / number / select / boolean — with a no-op save guard, inline error display, and optional `loadOptions` for lazily-fetched select lists; the boolean variant renders a `ConfirmSwitch`), `InfoRow` (read-only, searchable, optional `extra` node), and `SECTION_HEAD` / `LABEL_TD` styles. Re-exports `friendlyError` from `errors.ts` for back-compat. Pure presentational — the caller supplies `onSave`, which should route through a backend `Alter*Property` builder. |
| `ConfirmSwitch.tsx` | A boolean toggle that **stages** its change instead of committing on flip: flipping to a value different from the committed `checked` reveals inline Save / Cancel controls (Cancel reverts with no write; toggling back to the committed value also clears the pending state), and `onConfirm(next)` runs the ALTER on Save. Gives every Properties-modal boolean an in-place "unselect" path (issue #519). Used by `PropertiesModal`, `SessionPropertiesModal`, `PropertyRows`, and the per-type `*PropertiesModal` booleans (View Secure/Change-tracking, Materialized View / Function / Procedure Secure, Column Nullable). |
| `errors.ts` | `friendlyError` — strips gosnowflake noise down to the human-readable message. Extracted to its own module so `PropertyRows` and `ConfirmSwitch` can share it without a circular import. |

## Patterns & integration

- **IPC** (both files): `wailsjs/go/app/App` — `GetTableSettings`, `AlterTableProperty`, `SetSessionParameter`, `SetSessionVariable`.
- **Backend types**: `snowflake.PropertyPair`, `snowflake.SessionParam`, `snowflake.SessionVar` (from `wailsjs/go/models`); `table.TableSettings` when the optional `TableSettingsSection` is active.
- **Props pattern**: callers pass pre-fetched data (`rows`, `parameters`, `variables`) as `null` while loading, the real arrays when ready, or an error string. The modals handle all three states (spinner, data, error) internally.
- **Clipboard**: uses Wails `ClipboardSetText`; not `navigator.clipboard` (blocked in WKWebView).
- **No Zustand stores**: both components are stateless with respect to global state — all data flows in via props and changes are notified via `onParamChange`/`onVarChange` callbacks so the caller can update its local state.

## Gotchas

- `PropertiesModal`'s `TableSettingsSection` issues a separate `GetTableSettings` IPC call on mount whenever `tableContext` is provided — the parent does not need to pre-fetch table settings, but it must pass `{ db, schema, table }` as the `tableContext` prop.
- `SessionPropertiesModal` requires the caller to provide `onParamChange` / `onVarChange` callbacks so the owning component can keep its own copy of parameters and variables in sync (e.g. for display in the toolbar or status bar).
- The property search filter is client-side only (filters the already-fetched `rows` array); there is no server-side filtering.
