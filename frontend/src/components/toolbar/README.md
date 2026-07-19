# frontend/src/components/toolbar

> Unified application toolbar rendering execution controls, tab management, session selectors, and connection info.

## Responsibility

Provides the single `Toolbar` component that appears at the top of `QueryPage`. It is stateless
with respect to session data — it reads `connectionStore` and `sessionStore` directly, avoiding
prop-drilling for role/warehouse/database/schema selectors. The caller supplies only the
tab-specific handlers and optional extension slots.

## Files

| File | Purpose |
|------|---------|
| `Toolbar.tsx` | The unified toolbar component. Renders: Run/Cancel execution buttons, New SQL / New Notebook / Save icon buttons, session selectors (role, warehouse, database, schema dropdowns), connection info (username tag, account/region, Open Snowsight link, Disconnect button). Exposes `contextSlot` and `primaryAction` extension points. |
| `MCPIndicator.tsx` | Compact "MCP: N active" tag rendered in the toolbar's left group when one or more MCP sessions are running and the `mcpServer` flag is enabled. Subscribes to `mcpStore`; refreshes on the `thaw:mcp-changed` event; clicking dispatches `thaw:open-mcp-sessions` to open the MCP Sessions modal. Renders nothing when no sessions are active. |

## Patterns & integration

**Props interface (`ToolbarProps`):**

| Prop | Type | Notes |
|------|------|-------|
| `isRunning` | `boolean` | Switches Run button to Cancel. |
| `isCancelling` | `boolean` | Shows cancelling state on the Stop button. |
| `selectedSql` | `string` | Used to label the Run button ("Run Selection" vs "Run"). |
| `currentUser` | `string \| null` | Displayed as a tag. |
| `currentRegion` | `string \| null` | Displayed next to the account info. |
| `onRun` | `() => void` | Executes the current query. |
| `onCancel` | `() => void` | Cancels the running query. |
| `onDisconnect` | `() => void` | Disconnects from Snowflake. |
| `onOpenSessionProperties` | `() => void` | Opens the session properties modal. |
| `onOpenAccountParameters` | `() => void` | Opens the account parameters modal (`SHOW PARAMETERS IN ACCOUNT`; editable via `ALTER ACCOUNT SET`, ACCOUNTADMIN required). |
| `onOpenSnowsight` | `() => void` | Opens Snowsight in the browser. |
| `onNewSql` | `() => void` | Opens a new SQL tab. |
| `onNewNotebook` | `() => void` | Opens a new notebook tab. |
| `onSave` | `() => void` | Saves the current tab. |
| `contextSlot` | `ReactNode?` | Rendered after a vertical separator; used for notebook kernel status via `NotebookToolbarSlot`. |
| `primaryAction` | `ReactNode?` | Rendered above the three global icon buttons (New SQL / Notebook / Save). When present, globals collapse to a compact two-row stack. |

**Stores read directly (no props):**
- `connectionStore` — `roles`, `warehouses`, `databases`, `schemas`, `selectedRole`,
  `selectedWarehouse`, `selectedDatabase`, `selectedSchema`, and their setters.
- `sessionStore` — persisted session state used by the session selectors.

**No IPC calls** are made inside `Toolbar.tsx`. IPC for USE ROLE/WAREHOUSE/DATABASE/SCHEMA is
handled upstream in the store actions or in `QueryPage`.

**Extension points:**
- `contextSlot` — `NotebookToolbarSlot` (`frontend/src/components/notebook/NotebookToolbarSlot.tsx`)
  is the primary consumer; it renders the kernel status dot and Restart Kernel button.
- `primaryAction` — Notebook Deploy button is passed here from `QueryPage`.

**Responsive layout (issue #603):** the root strip (`.thaw-tb` in `global.css`) is a CSS
container (`container-type: inline-size`), so breakpoints track the centre-column width — which
depends on the side panels — not the window. As the column narrows: the "⌘↵ to run" hint hides
(≤880px), the session selects slim from 130px to 104px (≤760px), and finally `flex-wrap` moves
the session cluster onto its own right-aligned row (`.thaw-tb-right { margin-left: auto }`).
Nothing is ever clipped. Don't put fixed widths back as inline `style` props — they'd override
the container-query rules.

## Gotchas

- The Toolbar is the single source of truth for the session selector row. Do not replicate
  role/warehouse/database/schema dropdowns elsewhere in the layout.
- `contextSlot` and `primaryAction` are `ReactNode` — pass a fragment or `null`; omitting them
  keeps the toolbar in its single-row default layout.
- `notebookToolbarStore` is the bridge between `NotebookTab`'s internal kernel state and the
  `contextSlot` content; `Toolbar.tsx` itself does not import it.
- The toolbar reads `connectionStore` directly, so session selector changes (role, warehouse,
  database, schema) trigger Zustand re-renders on any subscriber; keep selectors granular to avoid
  unnecessary rerenders in parent components.
