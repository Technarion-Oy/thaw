# Critical gotchas

Non-obvious traps. Read this before debugging anything that "should just work."

## MCP SDK `AddTool` rejects non-object output schemas

The Go MCP SDK's generic `AddTool[In, Out]` infers an output JSON schema from `Out` and **panics at registration** if that schema's type isn't `"object"` — so tools returning `[]string`, `string`, or a slice of structs crash on startup. Fix (used throughout `internal/mcp/tools.go`): declare every handler's `Out` type as `any` (the SDK then omits the output schema) and return `nil` for the structured result, delivering the payload as indented-JSON text content via `jsonResult` / `textResult`. Never give an MCP tool a concrete non-struct `Out` type.

## gosnowflake driver logs errors before throwing

The gosnowflake driver logs ALL query errors at ERROR level via slog, even when the caller catches them. Do **not** call `GetObjectDDL` with a guessed object kind (TABLE vs VIEW) — always determine the kind first (from the objects store or a `ListObjects` call) to avoid noisy error logs from failed `GET_DDL` attempts.

## gosnowflake `sf.WithQueryIDChan`

The driver writes the query ID to the channel and **then closes it**. Never call `close(qidChan)` manually — that panics. Use `case qid := <-ch:` to drain, with `case <-ctx.Done():` as the cancellation fallback.

## WKWebView clipboard

`navigator.clipboard` is blocked in WKWebView. All clipboard operations use Wails' `ClipboardGetText` / `ClipboardSetText` native APIs. Clipboard routing is split by target:

- **Monaco's code buffer** — overridden per-editor via a `_commandService` patch + a capture-phase keydown listener (`utils/monacoClipboard.ts`, `patchMonacoClipboard`). The listener acts **only** when the focused element is the code surface (`isMonacoCodeSurface`, i.e. Monaco's `.inputarea`).
- **Every other native `<input>`/`<textarea>`** — including Monaco's find/replace/rename fields, which live inside `.monaco-editor` but are plain fields — handled by a single global Cmd/Ctrl+V/C/X listener in `App.tsx`. The code-buffer listener lets these events bubble to it (no `stopPropagation`), so they work even on Monaco mounts that never call `patchMonacoClipboard`.

`patchMonacoClipboard` is also where the find-widget tooltip fix is installed (`utils/monacoTooltipFix.ts`), since every Monaco mount routes through it.

## WKWebView drops the first keystroke typed over a selection (#575)

Two workarounds in `SqlEditor.tsx`'s `handleMount`, both load-bearing — don't "simplify" them away:

1. The `onDidChangeCursorSelection` handler defers its `setSelectedSql` store write via `setTimeout(0)`. Running it synchronously makes the Zustand re-render land mid-keystroke and the first character typed over a keyboard/double-click selection is dropped. `refreshOccurrences` stays synchronous so occurrence highlights update live during a drag.
2. A capture-phase `mouseup`/`keydown` pair (`onDragMouseUp`/`onDragKeyDown`) intercepts the first printable key after a **mouse drag-select** and re-issues it via `editor.trigger("keyboard", "type", …)`. WKWebView wedges Monaco's hidden-`<textarea>` input deduction after a drag — the model never sees the first input — so without this the character is silently lost until the second press. Using `trigger("type")` (not `executeEdits`) preserves auto-surround/auto-close and undo coalescing.

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

The vendor-skip test runs against the chunk's **basename**, not `chunk.fileName` — the latter is `assets/vendor-….js`, so a `/^vendor/` test silently never matches and every vendor chunk gets obfuscated anyway (each inflates ~5× in the binary). `manualChunks` names every `node_modules` output `vendor-*` so the skip catches it by prefix; a Monaco `moduleIds` fallback is the belt-and-suspenders for Monaco specifically.

## Monaco: import the slim editor API, never the `monaco-editor` barrel

`import … from "monaco-editor"` resolves to `editor.main.js`, which eagerly pulls every language service (TS/HTML/CSS/JSON) **and ~80 basic-language grammars**, each referencing a web worker — embedding ~9 MB of worker bundles (`ts.worker` alone is 6.9 MB) that are never executed (the `getWorker` switch in `monacoSetup.ts` only ever returns the YAML or base editor worker). Always import the slim API instead:

```ts
import * as monacoLib from "monaco-editor/esm/vs/editor/editor.api.js"; // namespace, no languages
import "monaco-editor/esm/vs/editor/editor.all.js";                     // all editor features, no languages
```

`editor.all.js` is exactly `editor.main` minus the language contributions (find, folding, comment, suggest, multicursor, … all present). We register SQL (custom Monarch), Python (inline Monarch), and YAML (via monaco-yaml's own worker) ourselves, so no built-in language service is needed. The `.js` extension is **required** for TS `bundler` resolution to find the sibling `.d.ts` (monaco's `exports` map is `"./*": "./*"`). All three Monaco value-importers (`monacoSetup.ts`, `SqlEditor.tsx`, `NotebookTab.tsx`) must use this path or Vite re-resolves the full barrel. Type-only importers that get the slim value passed in (e.g. `snowflakeSnippets.ts`) must also type against `editor.api.js`, else the full-barrel type is not assignable.

**Critical:** dropping the basic-language contributions also drops their `languages.register({ id })` calls, so `ensureMonacoSetup` must register `sql` and `python` itself **before** calling `setMonarchTokensProvider` / `setLanguageConfiguration` — otherwise Monaco throws `Cannot set configuration for unknown language sql`, which crashes the editor (white screen in dev; a swallowed unhandled-rejection in the obfuscated prod build, where the editor silently loses SQL config). `yaml` is registered by monaco-yaml's `configureMonacoYaml`, so it is not registered manually.

## Lazy-load heavy panels/modals to protect cold start

`manualChunks` previously lumped **all** of `node_modules` into one eager `vendor` chunk, forcing on-demand-only libraries (xlsx, recharts + d3, xterm, @xyflow/@dagrejs) to load at boot. They are now split into `vendor-xlsx`/`vendor-xterm`/`vendor-viz` and reached only through `React.lazy` boundaries (terminal, notebook, and the chart / ER / task-graph / migration / dbt / function-catalog modals), plus a dynamic `import("xlsx")` for Excel export. When adding a feature that pulls a heavy dependency used only by a modal/panel, `React.lazy` it (wrap the render site in `<Suspense>`) and, if the dep is new, add it to `VIZ_DEPS` or its own `vendor-*` chunk so it stays out of the boot bundle.
