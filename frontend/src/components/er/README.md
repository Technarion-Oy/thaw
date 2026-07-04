# frontend/src/components/er

> Entity-relationship diagram viewer and interactive table designer for a Snowflake database.

## Responsibility

Renders an interactive ER diagram using `@xyflow/react` from Snowflake table/column/FK metadata fetched by the backend. Supports schema filtering, drag-to-rearrange, zoom, pan, minimap, and clipboard copy of Mermaid source. Also hosts `ERDesigner`, a side-by-side modal for visually defining new tables (with inline editing, FK connection drawing, and generated DDL execution).

## Files

| File | Purpose |
|------|---------|
| `erTypes.ts` | Shared types (`DesignerColumn`, `DesignerTable`, `JoinQueryState`, etc.), the `SF_DATA_TYPES` / `SF_TYPES` constants (re-derived from the generated artifact `src/generated/snowflakeDataTypes.ts`, whose source of truth is the Go registry `internal/snowflake/datatypes.go` — not hand-maintained here), and node dimension constants (`ER_NODE_WIDTH`, etc.). |
| `erCanvasLayout.ts` | Pure layout utilities: `tablesToNodesAndEdges()`, `applyERLayout()` (dagre), `initFromERData()`, `normalizeDataType()`, `mergeAITablesIntoDesigner()`. No React imports. |
| `erLayoutStore.ts` | localStorage persistence for node positions, keyed by `thaw-er-layout:{DATABASE}` and `SCHEMA.TABLE`. Debounced writes. |
| `ERTableNode.tsx` | Custom XYFlow node component. Renders table header + column rows with per-column source/target handles, PK/NN/FK badges, and inline rename on double-click (edit mode). Wrapped in `React.memo`. |
| `ERCanvas.tsx` | Shared `ReactFlow` canvas used by both `ERDiagramModal` (readonly) and `ERDesigner` (edit). Manages layout (dagre + saved positions), node dragging, FK connection, selection, auto-layout, and reset-layout buttons. |
| `ERDiagramModal.tsx` | Primary viewer: interactive canvas, schema filter checkboxes, "Copy Mermaid" button, and a "Design Tables…" button that opens `ERDesigner`. Delegates join pathfinding and SQL generation to Go backend via `FindJoinPaths` and `BuildJoinState` IPC calls. |
| `ERDesigner.tsx` | Interactive table designer. Left sidebar with table/column CRUD forms, right panel with `ERCanvas` in edit mode. Generates diff-based SQL (`CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`) and executes via `ExecuteQuery`. Syncs state to backend MCP cache (mount/unmount/debounced changes) and listens for `mcp:modify-er-designer` events. Each column has a free-text DEFAULT `Input` emitted as `DEFAULT <expr>` on `CREATE TABLE`/`ADD COLUMN`; the `DefaultFunctionPicker` (ƒ) shortcut from `shared/` is shown **only for new-table columns** (`!isExistingTable`) — Snowflake rejects function-expression defaults on `ALTER … ADD COLUMN`, so only literals are offered there. Existing columns' default changes are not diffed (Snowflake restricts `ALTER COLUMN … SET DEFAULT`). |
| `buildMermaid.ts` | Pure function `buildMermaid(tables, visibleSchemas?)` that converts `DesignerTable[]` into a Mermaid `erDiagram` string. Used by both `ERDiagramModal` and `ERDesigner` for the "Copy Mermaid" clipboard export. Also exports shared helpers `sanitiseId`, `entityId`, `shortType`. |
| `JoinQueryPanel.tsx` | Bottom panel UI for the visual join builder. Shows join configuration (type selector, ON conditions), column picker, live SQL preview, and "Open in Editor" button. Also exports `JoinPathDisambiguation` for choosing between multiple candidate join paths. Delegates SQL generation to Go backend via `BuildJoinSQL` IPC call. |

## Patterns & integration

**IPC calls:**
- `ERDiagramModal` receives `snowflake.ERDiagramData` as a prop (pre-fetched by the caller) and calls `ListSchemas(database)` on mount to populate the schema filter with all database schemas (not just those present in the ER data). Join pathfinding (`FindJoinPaths`) and state building (`BuildJoinState`) are delegated to Go via IPC — see `internal/erdesigner/`.
- `JoinQueryPanel` delegates SQL generation to Go via `BuildJoinSQL` IPC — the Go backend uses `snowflake.QuoteIdent` for proper identifier quoting.
- `ERDesigner` calls `ListSchemas(database)` for the schema picker and `ExecuteQuery(sql)` to run generated DDL
- All other files are pure TypeScript/React with no IPC

**MCP integration:** The `open_er_designer` MCP tool (in `internal/mcp/er_tools.go`) emits a `mcp:open-er-designer` Wails event. `Sidebar.tsx` listens for this event and opens `ERDesigner` with two data sets: `mergedData` (AI tables merged into live schema, used for initial canvas population) and `initialData` (the original live schema, used as the baseline for diff SQL generation). The `mergedData` prop takes priority over `initialData` for table initialization, while `initialData` continues to drive `generateDiffSQL`.

**MCP state sync:** `ERDesigner.tsx` pushes its table state to the backend's `ERDesignerStateStore` via `UpdateERDesignerState` IPC on mount, on debounced (300ms) table changes, and clears via `ClearERDesignerState` on unmount. The `get_er_designer_state` MCP tool reads from this cache. The `modify_er_designer` MCP tool emits a `mcp:modify-er-designer` Wails event, which `ERDesigner.tsx` listens for and merges into its current state via `mergeAITablesIntoDesigner` — matching tables (by `SCHEMA.NAME`) are replaced (preserving canvas-positioning UUIDs), new tables are appended.

**XYFlow canvas:** Uses `@xyflow/react` v12 with a custom `ERTableNode` registered via module-level `nodeTypes` (required by XYFlow to prevent re-registration). Layout is computed by `@dagrejs/dagre` with `rankdir: "TB"`, `nodesep: 60`, `ranksep: 120`. Node heights are dynamic based on column count.

**Position persistence:** Node positions are saved to localStorage via `erLayoutStore`, keyed by `SCHEMA.TABLE` (case-preserved). Positions are loaded on mount and written back (debounced 500ms) on drag. "Reset Layout" clears saved positions and re-runs dagre.

**FK connections (edit mode):** Per-column handles (`col-source-{colId}` / `col-target-{colId}`) enable dragging edges between specific columns. The `onConnect` callback resolves table/column references and updates `fkRef` on the source column.

**Selection sync:** Clicking a table on the canvas highlights it and scrolls the sidebar to the corresponding card (and vice versa).

**Visual join builder (readonly mode):** Select 2+ tables on the canvas (Cmd/Ctrl+click), right-click → "Build Query". `ERDiagramModal` calls `FindJoinPaths` (Go IPC) to find FK paths connecting the selected tables. If multiple equal-length paths exist (e.g. two FKs between the same tables), a disambiguation panel appears. The `JoinQueryPanel` shows join configuration (adjustable join types), column selection, and a live SQL preview generated by `BuildJoinSQL` (Go IPC with proper identifier quoting). "Open in Editor" calls `loadInNewTab(sql)` from `queryStore` and closes the modal. The canvas highlights edges in the join path and marks intermediate tables with a dashed border via `highlightedEdgeIds`/`highlightedNodeIds` props on `ERCanvas`. The BFS pathfinder, SQL generator, and all associated types live in `internal/erdesigner/`.

## Gotchas

- `nodeTypes` must be defined at module level, not inside a component — XYFlow re-registers node types on every reference change, causing flickering.
- FK edges are derived from `column.fkRef` fields, not stored separately. Adding/removing FK edges always goes through column state updates.
- Both `ERDiagramModal` and `ERDesigner` use the shared `buildMermaid(tables, visibleSchemas?)` from `buildMermaid.ts` for Mermaid clipboard export.
- Column cap at 30 (`ER_COL_LIMIT`) prevents excessively tall nodes — overflow shows "+N more columns".
- Clipboard operations use `ClipboardSetText` from `wailsjs/runtime/runtime` (required because WKWebView blocks `navigator.clipboard`).
