---
trigger: glob
globs: "frontend/src/**/*.ts, frontend/src/**/*.tsx"
description: "React, TypeScript, Ant Design, Monaco Editor, WKWebView, Zustand, and Vitest invariants for Thaw's frontend."
---

# Thaw Frontend Rules

Follow these invariants whenever creating or editing files under `frontend/src/`. Full context: `docs/concepts/patterns.md` and `docs/concepts/gotchas.md`.

## Generated Files & IPC Safety
- **Never hand-edit** `frontend/wailsjs/` (run `wails generate module` from repo root) or `frontend/src/generated/` (`objectKinds.ts` / `snowflakeDataTypes.ts` — run `go generate ./internal/objectkind/` or `./internal/snowflake/`).
- Defend every array returned from Go IPC with `?? []` (Wails marshals Go nil slices as `null`).
- Never construct Snowflake DDL inline in React/TS components — collect form values and call Go `Build*Sql` builders over IPC.
- Add `// @thaw-domain: <Domain Name>` to new domain TS/TSX files and run `go generate ./internal/architecture/`.

## Ant Design & Modals
- **Never call static modal helpers** (`Modal.confirm`, `Modal.info`, `Modal.warning`, `Modal.error`, `Modal.success`) — they render outside `<ConfigProvider>` and appear white in dark mode (#884). Always use `const { modal } = AntApp.useApp()` at the top of the component (and include `modal` in `useCallback` deps). `frontend/src/components/staticModal.test.ts` fails the suite if any static call appears.
- Prefer conditionally mounting modals (`{open && <Modal open destroyOnClose … />}`) so they unmount on close. Global drag/resize (`utils/modalDragResize.ts`) relies on unmount to reset position/width on reopen; do not add per-modal drag/resize props.
- `antd` `Alert` has no `size` prop; use `showIcon` and `message`. Use `@ant-design/icons` and `message.success`/`error`.
- In `Tree` `onSelect(keys, info)`, `info.event` is the literal string `"select"` — read modifier keys (`ctrlKey`, `metaKey`, `shiftKey`) from `info.nativeEvent`.

## React State & Bundle Performance
- **Never compute or throw inside a React `setState(prev => …)` updater** — exceptions during render bypass `try`/`catch` around `setState` and crash the root (#875). Compute values before calling `setState` and keep the updater a pure merge.
- Wrap heavy on-demand modals/panels (xlsx, recharts/d3, xterm, `@xyflow`/`@dagrejs`) behind `React.lazy` + `<Suspense>` (or dynamic `import("xlsx")`) and keep heavy deps in `vendor-*` chunks in `vite.config.ts` to protect cold start and prevent obfuscating vendor bundles.
- Remember `gridStore` is a **singleton** shared across tabs; use `standalone` on `ResultGrid` for notebook SQL cells.
- Use store-held visibility (`tagManagementStore` / `containerServicesStore` pattern) for dialogs reachable from multiple entry points (menu, sidebar, toolbar) and mount them once in `QueryPage.tsx`.

## Monaco Editor & WKWebView
- **Never import the `monaco-editor` barrel** (`import … from "monaco-editor"` pulls ~9 MB of unused language workers). Always import the slim API with `.js` extensions:
  ```ts
  import * as monacoLib from "monaco-editor/esm/vs/editor/editor.api.js";
  import "monaco-editor/esm/vs/editor/editor.all.js";
  ```
  Type-only consumers receiving the slim namespace must also type against `editor.api.js`.
- Never register Monaco completion/hover providers inside render — use module-level disposable refs.
- Never call `navigator.clipboard` (blocked in macOS WKWebView) — use Wails `ClipboardGetText` / `ClipboardSetText` and preserve the split routing in `utils/monacoClipboard.ts` + `App.tsx`.
- Do not remove the `setTimeout(0)` selection defer or capture-phase drag-select workaround in `SqlEditor.tsx` (`handleMount`, #575), and keep `runDiagnostics` guarded by both `model.getVersionId()` and monotonic `diagRunRef` inside `try/catch/finally` (#718).

## Frontend Testing (Vitest)
- Run `cd frontend && npx tsc --noEmit && npm test` after edits.
- There is no Wails runtime in Vitest/CI: mock the **exact** `wailsjs/` module path imported by the code under test (e.g. `wailsjs/go/sqleditor/Service`), otherwise calls throw and silently fall back.
