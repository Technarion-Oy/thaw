# Wails v3 Real-App E2E Smoke Suite (`e2e/`)

End-to-end smoke tests that launch the **real compiled Thaw desktop application** (Go backend + embedded React/TypeScript frontend + native Wails v3 webview/menu runtime) and drive it over Wails v3's built-in app-control driver (`-tags mcp`).

> **Naming note:** Wails v3's upstream app-control driver uses the Go build tag `mcp` (`WAILS_MCP_*` environment variables). It is **completely unrelated** to Thaw's own Snowflake MCP server (`internal/mcp/`). This directory and its test tag (`wails_e2e`) are named distinctly so the two are never confused, and `release_guard_test.go` enforces zero overlap between them.

## Files

| File | Build tag | Purpose |
|------|-----------|---------|
| `release_guard_test.go` | *(none — runs in `go test ./...`)* | Security guards asserting release/signing workflows never enable Wails's `mcp` tag, `go build -tags production,mcp ./internal/app` fails at compile time, and `e2e/` has zero overlap with `internal/mcp/`. |
| `driver_test.go` | `//go:build wails_e2e` | Process launcher (isolated `HOME` / `XDG_*` / `APPDATA` state dir, per-run bearer token, ephemeral loopback port) and JSON-RPC HTTP client for Wails v3's app-control driver. |
| `smoke_test.go` | `//go:build wails_e2e` | Smoke checks: app startup & license gate, live IPC round-trips (`App` + `sqleditor.Service`), native menu event delivery to the frontend, and window state persistence/restore across relaunch. |

## Running locally

1. Build the frontend bundle once (fast mode is fine for E2E):
   ```bash
   cd frontend && THAW_FAST_BUILD=1 npm run build && cd ..
   ```
2. Run the smoke suite (auto-builds a temporary `-tags mcp` binary and cleans it up):
   ```bash
   go test -tags wails_e2e -v -count=1 ./e2e/...
   ```

To reuse a prebuilt `-tags mcp` binary (as CI does), set `THAW_E2E_BINARY`:
```bash
go build -tags mcp -buildvcs=false -o build/bin/thaw-wails-e2e .
THAW_E2E_BINARY=$PWD/build/bin/thaw-wails-e2e go test -tags wails_e2e -v -count=1 ./e2e/...
```

On headless Linux (CI), wrap the test command with `dbus-run-session` and `xvfb-run` plus a lightweight window manager (see `.github/workflows/e2e-smoke.yml`).

## Security boundary

Wails v3's `-tags mcp` app-control server exposes arbitrary JavaScript evaluation, DOM inspection, and bound-method invocation over localhost HTTP. Three independent layers guarantee it can never ship in a release binary:

1. **Compile-time refusal (`internal/app/wails_e2e_guard.go`):** `//go:build production && mcp` intentionally fails compilation if both tags are ever combined.
2. **Taskfile enforcement (`build/*/Taskfile.yml`):** all non-dev builds pass `-tags production`.
3. **CI & unit-test guard (`e2e/release_guard_test.go`):** `TestReleaseWorkflowsNeverEnableWailsMCPTag` and `TestProductionAndMCPTagsFailToCompile` run on every PR in `build-check.yml` and `e2e-smoke.yml`.

## Gotcha (Wails v3.1.0 `call_bound_method` & Vite bundles)

Wails v3's `call_bound_method` tool runs `await import('/wails/runtime.js')` inside the page. Because Thaw bundles `@wailsio/runtime` with Vite, that dynamic import evaluates a second copy of `@wailsio/runtime` which overwrites `window._wails.dispatchWailsEvent` with a fresh empty listener map. Keep event-delivery assertions (`NativeMenuEvents`) before `call_bound_method` assertions (`LiveIPCRoundTrip`) within the same window session.
