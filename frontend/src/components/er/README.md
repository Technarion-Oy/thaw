# frontend/src/components/er

> Entity-relationship diagram viewer and interactive table designer for a Snowflake database.

## Responsibility

Renders an interactive ER diagram using `@xyflow/react` from Snowflake table/column/FK metadata fetched by the backend. Supports schema filtering, drag-to-rearrange, zoom, pan, minimap, and clipboard copy of Mermaid source. Also hosts `ERDesigner`, a side-by-side modal for visually defining new tables (with inline editing, FK connection drawing, and generated DDL execution).

## Files

| File | Purpose |
|------|---------|
| `erTypes.ts` | Shared types (`DesignerColumn`, `DesignerTable`), `SF_TYPES` constant, and node dimension constants (`ER_NODE_WIDTH`, etc.). |
| `erCanvasLayout.ts` | Pure layout utilities: `tablesToNodesAndEdges()`, `applyERLayout()` (dagre), `initFromERData()`, `normalizeDataType()`. No React imports. |
| `erLayoutStore.ts` | localStorage persistence for node positions, keyed by `thaw-er-layout:{DATABASE}` and `SCHEMA.TABLE`. Debounced writes. |
| `ERTableNode.tsx` | Custom XYFlow node component. Renders table header + column rows with per-column source/target handles, PK/NN/FK badges, and inline rename on double-click (edit mode). Wrapped in `React.memo`. |
| `ERCanvas.tsx` | Shared `ReactFlow` canvas used by both `ERDiagramModal` (readonly) and `ERDesigner` (edit). Manages layout (dagre + saved positions), node dragging, FK connection, selection, auto-layout, and reset-layout buttons. |
| `ERDiagramModal.tsx` | Primary viewer: interactive canvas, schema filter checkboxes, "Copy Mermaid" button, and a "Design Tables…" button that opens `ERDesigner`. |
| `ERDesigner.tsx` | Interactive table designer. Left sidebar with table/column CRUD forms, right panel with `ERCanvas` in edit mode. Generates diff-based SQL (`CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`) and executes via `ExecuteQuery`. |
| `buildMermaid.ts` | Pure function `buildMermaid(data, visibleSchemas)` that converts `snowflake.ERDiagramData` into a Mermaid `erDiagram` string. Used for the "Copy Mermaid" clipboard export. |

## Patterns & integration

**IPC calls:**
- `ERDiagramModal` receives `snowflake.ERDiagramData` as a prop (pre-fetched by the caller)
- `ERDesigner` calls `ListSchemas(database)` for the schema picker and `ExecuteQuery(sql)` to run generated DDL
- All other files are pure TypeScript/React with no IPC

**XYFlow canvas:** Uses `@xyflow/react` v12 with a custom `ERTableNode` registered via module-level `nodeTypes` (required by XYFlow to prevent re-registration). Layout is computed by `@dagrejs/dagre` with `rankdir: "TB"`, `nodesep: 60`, `ranksep: 120`. Node heights are dynamic based on column count.

**Position persistence:** Node positions are saved to localStorage via `erLayoutStore`, keyed by `SCHEMA.TABLE` (uppercase). Positions are loaded on mount and written back (debounced 500ms) on drag. "Reset Layout" clears saved positions and re-runs dagre.

**FK connections (edit mode):** Per-column handles (`col-source-{colId}` / `col-target-{colId}`) enable dragging edges between specific columns. The `onConnect` callback resolves table/column references and updates `fkRef` on the source column.

**Selection sync:** Clicking a table on the canvas highlights it and scrolls the sidebar to the corresponding card (and vice versa).

## Gotchas

- `nodeTypes` must be defined at module level, not inside a component — XYFlow re-registers node types on every reference change, causing flickering.
- FK edges are derived from `column.fkRef` fields, not stored separately. Adding/removing FK edges always goes through column state updates.
- The "Copy Mermaid" button in `ERDiagramModal` uses `buildMermaid` from `buildMermaid.ts` (works on raw `ERDiagramData`), while `ERDesigner` uses its own `buildDesignerMermaid` (works on `DesignerTable[]`).
- Column cap at 30 (`ER_COL_LIMIT`) prevents excessively tall nodes — overflow shows "+N more columns".
- The copy-Mermaid button calls `navigator.clipboard.writeText` directly. In WKWebView contexts, switch to `ClipboardSetText` if needed.
