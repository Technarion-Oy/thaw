# Critical gotchas

Non-obvious traps. Read this before debugging anything that "should just work."

## MCP SDK `AddTool` rejects non-object output schemas

The Go MCP SDK's generic `AddTool[In, Out]` infers an output JSON schema from `Out` and **panics at registration** if that schema's type isn't `"object"` — so tools returning `[]string`, `string`, or a slice of structs crash on startup. Fix (used throughout `internal/mcp/tools.go`): declare every handler's `Out` type as `any` (the SDK then omits the output schema) and return `nil` for the structured result, delivering the payload as indented-JSON text content via `jsonResult` / `textResult`. Never give an MCP tool a concrete non-struct `Out` type.

## gosnowflake driver logs errors before throwing

The gosnowflake driver logs ALL query errors at ERROR level via slog, even when the caller catches them. Do **not** call `GetObjectDDL` with a guessed object kind (TABLE vs VIEW) — always determine the kind first (from the objects store or a `ListObjects` call) to avoid noisy error logs from failed `GET_DDL` attempts.

## gosnowflake `sf.WithQueryIDChan`

The driver writes the query ID to the channel and **then closes it**. Never call `close(qidChan)` manually — that panics. Use `case qid := <-ch:` to drain, with `case <-ctx.Done():` as the cancellation fallback.

## WKWebView clipboard

`navigator.clipboard` is blocked in WKWebView. All clipboard operations use Wails' `ClipboardGetText` / `ClipboardSetText` native APIs. Monaco's built-in copy/paste is overridden via a `_commandService` patch + capture-phase keydown listeners (`utils/monacoClipboard.ts`).

## Multi-statement execution

For multi-statement SQL, `Execute` uses an inner `execCtx` (fresh context), so the outer `qidChan` (single-statement async mode) never fires. Per-statement query IDs are tracked via per-statement goroutines + a `sync.WaitGroup` in `internal/app/query.go`'s `StartQuery`.

## `wailsjs/` is auto-generated

Never edit files under `frontend/wailsjs/` by hand — they are overwritten by `wails generate module`.

## `frontend/dist/.gitkeep` must stay committed

`//go:embed all:frontend/dist` in `main.go` is evaluated during `wails generate module` (binding generation), which runs **before** the frontend build. If `frontend/dist` is empty or missing, the Go build fails with "contains no embeddable files". The committed `.gitkeep` satisfies the embed on clean checkouts — never delete it.

## `internal/architecture/semantic_map.go` is generated

Do not edit it by hand. Annotate source files (`// thaw:domain:`, `// thaw:file-domain:`, `// @thaw-domain:`) and run `go generate ./internal/architecture/`. The CI test `TestSemanticMapAccuracy` fails if any annotated path no longer exists on disk.

## `runDiagnostics` must stay race-safe and exception-safe

`runDiagnostics` in `SqlEditor.tsx` is async with three IPC `await` points. Two invariants must hold:

1. **Race safety** — capture `model.getVersionId()` before any async work and check it after each `await`; `return` early if the version advanced (user edited mid-flight). The `return` still runs `finally`, but the version check inside `finally` prevents overwriting a newer run's markers.
2. **Exception safety** — wrap the whole body in `try/catch/finally`. If any IPC call rejects, `catch` logs and `finally` guarantees `setModelMarkers` is called with whatever was collected, so stale markers never stick.

Also use `editor.onDidChangeModelContent` (not `editor.getModel()?.onDidChangeContent`) — the latter silently skips registration if the model is null at mount.

## `gridStore` is a singleton

Formatting, search state, selection range, conditional-formatting rules, and the `tableRows` reference are **shared across tabs** and reset when switching tabs or running a query in another tab. During tab switches there's a brief window where stale state is visible. Notebook SQL cells use `ResultGrid` with the `standalone` prop to suppress `setTableRows()`/`resetGrid()`/`resetNavigation()` and avoid contaminating the main tab. In side-by-side compare mode both `ResultGrid` instances call `setTableRows()` — the last to render wins. The `reset()` on column-schema change mitigates most cases.

## Frontend bundle obfuscation

The production build runs `javascript-obfuscator` after Terser (`vite.config.ts`); vendor and Monaco chunks are skipped. The build passes `--max-old-space-size=6144` to Node to avoid V8 OOM. `controlFlowFlattening` and `deadCodeInjection` are disabled to keep peak memory in budget; RC4 string-array encoding is the primary IP protection.
