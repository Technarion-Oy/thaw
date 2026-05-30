# frontend/src/components/er

> Entity-relationship diagram viewer and interactive table designer for a Snowflake database.

## Responsibility

Renders a Mermaid `erDiagram` from Snowflake table/column/FK metadata fetched by the backend.
Supports schema filtering, zoom, pan, and clipboard copy of the raw Mermaid source.
Also hosts `ERDesigner`, a side-by-side modal for visually defining new tables and executing
the generated `CREATE TABLE` DDL directly against Snowflake.

## Files

| File | Purpose |
|------|---------|
| `ERDiagramModal.tsx` | Primary viewer: Mermaid rendering, zoom controls, drag-to-pan, schema filter checkboxes, and a "Design Tables…" button that opens `ERDesigner`. |
| `ERDesigner.tsx` | Interactive table designer. Manages a list of `DesignerTable` objects, renders a live Mermaid preview, and executes `CREATE TABLE` SQL via `ExecuteQuery`. |
| `buildMermaid.ts` | Pure function `buildMermaid(data, visibleSchemas)` that converts `snowflake.ERDiagramData` into a Mermaid `erDiagram` string. Caps columns at 30 per entity. |

## Patterns & integration

**IPC calls:**
- `ERDiagramModal` receives `snowflake.ERDiagramData` as a prop (pre-fetched by the caller before opening the modal; no IPC inside the modal itself)
- `ERDesigner` calls `ListSchemas(database)` for the schema picker and `ExecuteQuery(sql)` to run generated `CREATE TABLE` statements
- `buildMermaid` is a pure TypeScript helper with no IPC

**Mermaid rendering:** `mermaid.initialize` is called at module load with `securityLevel: "loose"` and `theme: "dark"`. Each render uses a unique ID derived from `useId()` + a `renderCount` ref to avoid ID collisions on re-render. The `applyZoom` function rewrites the SVG `width`/`max-width` attributes directly (rather than CSS transform) so the scroll container responds to the natural element size.

**Zoom & pan:** Zoom uses `±0.25` steps clamped to `[0.25, 4]`. Pan is implemented with `mousedown`/`mousemove`/`mouseup` on the scroll container; a `panningRef` (not state) prevents stale closures in the `mousemove` handler.

## Gotchas

- FK relationships are filtered to only include edges where both endpoints are in `visibleSchemas`; this prevents dangling references in the Mermaid output.
- `ERDesigner` uses `mermaid.initialize` again at module load — both files call it with identical options, so the second call is effectively a no-op but must stay consistent.
- The copy-Mermaid button calls `navigator.clipboard.writeText` directly. In other contexts Thaw uses Wails clipboard APIs (WKWebView blocks `navigator.clipboard`); ensure this modal is opened only from contexts where clipboard access works, or switch to `ClipboardSetText`.
