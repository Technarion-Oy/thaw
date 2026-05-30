# frontend/src/components/common

> Shared modal primitives used across multiple feature areas.

## Responsibility

Provides general-purpose modals for displaying and editing Snowflake object properties and session state. These components are consumed by `AccountPanel`, `Sidebar`, and other feature components and are intentionally free of domain-specific logic.

## Files

| File | Purpose |
|---|---|
| `PropertiesModal.tsx` | Displays a searchable key-value property table from `snowflake.PropertyPair[]`; optionally renders editable `TableSettingsSection` (calls `GetTableSettings`, `AlterTableProperty`) and `ColumnCommentsSection` (calls `GetColumnComments`, `SetColumnComment`) when a `tableContext` prop is provided; includes Copy-all to clipboard. |
| `SessionPropertiesModal.tsx` | Displays and allows inline editing of `snowflake.SessionParam[]` (session parameters) and `snowflake.SessionVar[]` (session variables); calls `SetSessionParameter` and `SetSessionVariable` IPC; Boolean params/vars are rendered as `Switch` toggles; non-boolean as editable text inputs; includes search and Copy-all. |

## Patterns & integration

- **IPC** (both files): `wailsjs/go/app/App` — `GetColumnComments`, `SetColumnComment`, `GetTableSettings`, `AlterTableProperty`, `SetSessionParameter`, `SetSessionVariable`.
- **Backend types**: `snowflake.PropertyPair`, `snowflake.SessionParam`, `snowflake.SessionVar` (from `wailsjs/go/models`); `objects.ColumnComment`, `table.TableSettings` when the optional sections are active.
- **Props pattern**: callers pass pre-fetched data (`rows`, `parameters`, `variables`) as `null` while loading, the real arrays when ready, or an error string. The modals handle all three states (spinner, data, error) internally.
- **Clipboard**: uses Wails `ClipboardSetText`; not `navigator.clipboard` (blocked in WKWebView).
- **No Zustand stores**: both components are stateless with respect to global state — all data flows in via props and changes are notified via `onParamChange`/`onVarChange` callbacks so the caller can update its local state.

## Gotchas

- `PropertiesModal`'s `TableSettingsSection` issues a separate `GetTableSettings` IPC call on mount whenever `tableContext` is provided — the parent does not need to pre-fetch table settings, but it must pass `{ db, schema, table }` as the `tableContext` prop.
- `ColumnCommentsSection` mutates comments optimistically in local state after a successful `SetColumnComment` call; on error the change is rolled back by not updating state.
- `SessionPropertiesModal` requires the caller to provide `onParamChange` / `onVarChange` callbacks so the owning component can keep its own copy of parameters and variables in sync (e.g. for display in the toolbar or status bar).
- The property search filter is client-side only (filters the already-fetched `rows` array); there is no server-side filtering.
