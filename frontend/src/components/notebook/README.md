# frontend/src/components/notebook

> Native Snowpark notebook UI: per-cell Monaco editors, Python/SQL kernel management, DAP debugger, and notebook lifecycle actions (deploy, execute, preferences).

## Responsibility

Provides the full interactive notebook experience for Snowflake-native notebooks running
on a Snowpark (Jupyter) kernel. Handles cell rendering and execution, kernel lifecycle
(start / stop / restart), per-cell Python debugging via Debug Adapter Protocol, and
notebook deployment to Snowflake.

## Files

| File | Purpose |
|---|---|
| `NotebookTab.tsx` | Root notebook component. Renders the ordered list of cells, per-cell Monaco editors (Python and SQL), hover-reveal "Add Cell" bars, cell type toggle, run/debug controls, output display, and breakpoint gutter. Manages kernel state (`kernelReady`, `kernelStarting`, `kernelError`) and publishes it upward via `notebookToolbarStore`. Registers the Python snippet context menu (module-level IIFE, mirrors SQL snippet pattern). |
| `NotebookToolbarSlot.tsx` | Stateless slot component rendered inside the unified Toolbar `contextSlot`. Shows a kernel status dot (`KernelDot` — spinning, error, or green) and a Restart Kernel icon button. Deploy moved to `Toolbar.primaryAction`; Add Cell is handled via hover bars inside `NotebookTab`. |
| `DeployNotebookModal.tsx` | Deploys a local `.ipynb` file (or unsaved notebook serialized to nbformat JSON) as a Snowflake NOTEBOOK object. Collects `DATABASE`, `SCHEMA`, `NAME`, `OR REPLACE / IF NOT EXISTS`, optional `QUERY_WAREHOUSE`, `WAREHOUSE`, `IDLE_AUTO_SHUTDOWN_TIME_SECONDS`, `RUNTIME_NAME`, and `COMPUTE_POOL`. Calls `DeployNotebook` IPC method. |
| `CreateNotebookModal.tsx` | Schema context-menu **Create Object → Projects → Notebook…** form (gated by the `snowparkNotebooks` feature flag). Creates a NOTEBOOK object from scratch — name (+ case control), `OR REPLACE / IF NOT EXISTS`, optional `QUERY_WAREHOUSE`, an optional stage-file picker seeding `FROM '<source>'` + `MAIN_FILE`, and `COMMENT` — with a live SQL preview from `BuildCreateNotebookSql` (backend `internal/notebook`), executed via `ExecDDL`. Distinct from `DeployNotebookModal`, which uploads local notebook bytes. |
| `ExecuteNotebookModal.tsx` | Issues `EXECUTE NOTEBOOK db.schema.name(params…)`. Loads the current `QUERY_WAREHOUSE` via `GetNotebookQueryWarehouse` and warns if none is set. Supports an arbitrary list of positional string parameters with a live SQL preview. Opens `SetNotebookWarehouseModal` inline if the user needs to set a warehouse first. |
| `SetNotebookWarehouseModal.tsx` | Quick modal to call `SetNotebookQueryWarehouse` for an already-deployed notebook. Pre-selects the current session warehouse. |
| `NotebookPrefsModal.tsx` | Persists `NotebookPrefs` (currently `syntaxMode`: kernel-aware / static-only / off). Reads/writes via `GetNotebookPrefs` / `SaveNotebookPrefs` IPC and reloads `notebookPrefsStore` on save. |
| `debugClient.ts` | Minimal DAP (Debug Adapter Protocol) client. Communicates via Wails events (`dap:client-to-backend`, `dap:backend-to-client`). Handles Base64 binary framing over the IPC bridge, full DAP handshake (initialize → fire-and-forget attach → setBreakpoints → configurationDone), stopped-event variable capture, and step/continue/disconnect commands. Exported types: `DapClient`, `CellBreakpoints`, `DebugVariable`. |

## Patterns & integration

- **IPC**: all kernel calls (`StartNotebookSession`, `RunNotebookCell`, `RunNotebookCellSql`, `StopNotebookSession`, `GetNotebookCompletions`, `GetNotebookHover`, `CheckPythonSyntax`, `DebugNotebookCell`, `StopDapProxy`, `SaveNotebookBreakpoints`, `LoadNotebookBreakpoints`, `GetKernelPythonVersion`) come from `wailsjs/go/app/App`.
- **Kernel state bridge**: `NotebookTab` pushes `{ kernelReady, kernelStarting, kernelError, kernelName, onRestartKernel }` into `notebookToolbarStore` so the unified `Toolbar` (in `QueryPage`) can render `NotebookToolbarSlot` without prop-drilling.
- **SQL cell results**: SQL cells render `ResultGrid` with the `standalone` prop, which suppresses `setTableRows()` / `resetGrid()` to avoid contaminating the main query tab's `gridStore`.
- **Clipboard**: a module-level `installCopyHandler` patches `Cmd+C` on each cell container to route markdown/output text-selection copies through `ClipboardSetText` (WKWebView blocks native clipboard API). A cell's Monaco **code buffer** goes through the shared `patchMonacoClipboard(editor)` (called from the cell's `onMount`), which gates on `codeEditor.hasTextFocus()` — so Cmd/Ctrl+V/C/X in a cell's find/replace/rename fields bubbles to the global handler in `App.tsx` instead of hitting the code buffer.
- **Python snippets**: registered once per session via `MenuRegistry` + `CommandsRegistry` internal Monaco APIs; same pattern as SQL snippets in `SqlEditor`.
- **DAP transport**: `debugClient.ts` uses `EventsEmit("dap:client-to-backend")` and `EventsOn("dap:backend-to-client")` (Base64-encoded). The attach request is fire-and-forget to avoid a deadlock with debugpy's `wait_for_client()`.

## Gotchas

- The DAP `attach` request **must not be awaited** before sending `configurationDone` — debugpy deadlocks if you wait for the attach response first (see `debugClient.ts` comments).
- `NotebookToolbarSlot` is a pure presentational component with no store access — `NotebookTab` owns all state and pushes it via `notebookToolbarStore`.
- SQL cells use `ResultGrid` with `standalone: true` — omitting this causes `gridStore` to reset formatting for any other open SQL tab.
- `thaw:editor-ready` is only emitted by the primary `SqlEditor` (not by per-cell notebook editors), so `CrossTabSearch` navigation cannot scroll to a match inside a notebook cell.
