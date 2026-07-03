# frontend/src/styles

> Global CSS reset, design token definitions, and shared component-level styles for the entire Thaw frontend.

## Responsibility

Contains a single stylesheet, `global.css`, that is loaded once at application startup. It owns the CSS custom property (variable) system for both light and dark themes, global resets, scrollbar styling, accessibility rules, and all reusable CSS class patterns that are shared across multiple components (editor decorations, the results grid, notebook cells, the app toolbar, etc.).

## Files

| File | Purpose |
|------|---------|
| `global.css` | The sole global stylesheet. See the section breakdown below. |

## Patterns & integration

`global.css` is imported in the app entry point and applies universally. Component-specific styles that appear here are those which cannot be colocated with a React component because they target third-party DOM structures (Ant Design, Monaco, ReactFlow) or require `!important` overrides unavailable in CSS Modules.

### Section breakdown

| Section | What it defines |
|---------|----------------|
| Box-sizing reset | Universal `border-box`, zero margin/padding |
| `:root` dark theme | All `--bg-*`, `--border-*`, `--text-*`, `--accent`, `--success`, `--warning`, `--danger`, `--icon-*`, `--col-*`, `--cell-*` tokens |
| `[data-theme="light"]` | Light-theme overrides for every token defined in `:root`; toggled by `themeStore` setting `data-theme` on `<html>` |
| `html, body, #root` | Full-viewport layout, font stack (Inter / SF Pro Text / system-ui), WKWebView antialiasing |
| `.titlebar-drag` | Sets `--wails-draggable: drag` for the macOS traffic-light area |
| `::-webkit-scrollbar` | 10px scrollbars with inset thumb effect |
| `:focus-visible` | WCAG 2.4.7 keyboard focus ring using `--accent` |
| Ant Design overrides | `--border-strong` on inputs, selects, pickers; tree indentation, node title layout, font size |
| UI density tokens | `--row-height` / `--header-height` per `[data-density]` attribute (`compact`, default, `comfortable`) |
| `.ai-chat-selectable`, `.ddl-tooltip`, `.thaw-grid` | Re-enable `user-select: text` and set `--wails-draggable: no-drag` to unblock WKWebView pointer events |
| Draggable/resizable modals | `.ant-modal-header` gets `--wails-draggable: no-drag` (drag driven by `utils/modalDragResize.ts`); `.ant-modal` gets `resize: horizontal` (width-only, the box that owns antd's `width`) with `overflow: hidden` + the dialog `box-shadow`/`border-radius` moved up from `.ant-modal-content` so `overflow` doesn't clip the shadow; content shows `cursor: move`, body/footer reset it (#572) |
| Monaco decorations | `.sql-occurrence-highlight`, `.sql-active-stmt-bg/indicator`, `.git-gutter-added/modified/deleted`, `.sql-token-builtin/udf`, squiggle SVG overrides, hover widget theme |
| `.ctx-item` | Tab-bar right-click context menu items |
| ReactFlow controls | Maps `--xy-controls-*` to app palette so ReactFlow buttons respect the theme |
| Notebook cell styles | `.thaw-nb-cell`, gutter, kind tag, editor/output areas, toolbar buttons, hover-reveal add-cell bars |
| App toolbar styles | `.thaw-tb-icon-btn`, `.thaw-tb-text-btn`, `.thaw-tb-primary-btn`, `.thaw-tb-group`, `.thaw-tb-sep`, vstack layout for notebook deploy action |
| Sidebar column icons | `.thaw-col-icon[data-family]` — maps type families to `--col-*` tokens |
| Kernel status dot | `.thaw-kernel-dot` with ready/error/starting states |
| Debug breakpoint / current-line | `.thaw-debug-breakpoint`, `.thaw-debug-current-line`, `.thaw-debug-current-line-arrow` |
| Task history row | `.task-row-failed td` — red tint on failed rows |

### Design token conventions

- All surface colors are `--bg`, `--bg-raised`, `--bg-overlay`, `--bg-hover` (elevation order).
- Two border tiers: `--border` (decorative, exempt from WCAG 1.4.11) and `--border-strong` (controls, 3:1 non-text contrast minimum).
- Text tiers: `--text` (AAA), `--text-muted` (AAA), `--text-faint` (AA).
- Object-browser icon colors are all at the same lightness band so no icon dominates a row.
- Column type icons use `--col-*` tokens and are applied via `[data-family]` on `.thaw-col-icon`.

## Gotchas

- `!important` is used deliberately for WKWebView `user-select` overrides and Ant Design border colors. Do not add new `!important` rules without documenting why CSS specificity alone cannot solve the problem.
- Monaco squiggle colors are embedded as inline SVG data URIs because Monaco renders them via `background-image`; the color values must match `--danger`, `--warning`, and `--accent` manually — they are not connected to the CSS variable system.
- The `color-mix(in oklab, ...)` function is used for tinted backgrounds and focus rings. It requires a modern browser engine; WKWebView on macOS 13+ supports it.
- Notebook cell selection uses `color-mix` for the border and box-shadow, both derived from `--cell-accent` which is set per `[data-kind]` on the cell element.
