# Testing

Tests live alongside production code in each package, using Go's standard `testing` package (backend) and Vitest (frontend).

## Go unit tests

```bash
go test ./...                          # everything
go test ./internal/ddl/...             # one package
go test -v ./internal/ddl/...          # verbose (sub-test names)
go test -v -run TestSplit ./internal/ddl/   # a single test
go test -race ./...                    # with the race detector
```

The race detector is especially valuable for concurrency-sensitive code (e.g. the object-cache and parallel DDL export in `internal/ddl` / `internal/snowflake`). The thin-delegator pattern means most SQL building and result parsing is testable without a live connection — write `Build*Sql` / `Parse*` tests in the domain package.

## Frontend

```bash
cd frontend
npx tsc --noEmit     # type-check (fast, no emit)
npm test             # Vitest unit tests (e.g. utils/sqlFormatter.test.ts)
npm run build        # full production build (also catches type/obfuscation issues)
```

The Vitest suite runs in CI on every push/PR that touches `frontend/**` via
`.github/workflows/frontend-tests.yml` (self-hosted Linux container, `npm ci`
+ `vitest run`, JUnit results published to the PR). There is no Wails runtime in
CI, so tests that call generated `bindings/` IPC bindings must mock them — mock
the exact module the code imports (e.g. `bindings/thaw/internal/sqleditor/service`
or `bindings/thaw/internal/app/app`), or the binding loads `@wailsio/runtime` in
Node and throws.

## Integration tests

Live-connection tests are in `internal/integration/`, gated behind the `integration` build tag and excluded from normal runs and docs generation. They require Snowflake credentials via environment variables (key-pair auth). The formatter dialect tests need no CREATE privileges; export and migration tests require a live account.

```bash
go test -tags integration ./internal/integration/...
```

## Real-app Wails v3 E2E smoke tests (`e2e/`)

Unit tests mock the generated `frontend/bindings/` layer; `e2e/` exercises the **real compiled desktop binary** (Go backend + embedded React/TypeScript frontend + native Wails v3 runtime) using Wails v3's built-in app-control driver (`-tags mcp`).

> **Unrelated to `internal/mcp/`:** Wails v3's upstream `-tags mcp` app-control server is purely a local test/automation driver and has zero overlap with Thaw's own Snowflake MCP server (`internal/mcp/`). The suite lives under `e2e/` and uses the distinct Go build tag `wails_e2e` so the two are never confused.

What the smoke suite (`e2e/smoke_test.go`) verifies against a live app instance in an isolated temp profile directory:

1. **App startup** — binary launches cleanly, Wails app-control server initializes, main window opens, React mounts the first-launch License Agreement gate (`GetLicenseText`), and clicking Accept persists `AcceptLicense` via generated bindings and reveals the workspace.
2. **Native menu event delivery** — triggers real native menu callbacks wired in `internal/app/menu.go` through a non-production event hook (`wails_e2e_driver.go`): verifies connection-gated items stay disabled offline (`Tag Management…`), radio items with payloads switch `data-theme` (`View → Appearance → Light / Dark`), and `Help → About Thaw…` opens `AboutModal` with live `GetAppInfo()` data.
3. **Live IPC round-trips** — invokes bound methods on both `thaw/internal/app.App` (`GetAppInfo`, `IsConnected`, `IsLicenseAccepted`, `SaveEditorPrefs`/`GetEditorPrefs`) and `thaw/internal/sqleditor.Service` (`AnalyzeSqlSyntax`, `GetSqlStatementRanges`, `GetSnowflakeKeywords`) across the real Wails v3 runtime bridge.
4. **Window state persistence & restore** — resizes the window, closes cleanly so `WindowClosing` persists `session.json`, relaunches against the same isolated state directory, and asserts the resized dimensions are restored.

### Running locally

```bash
# 1. Build the embedded frontend bundle once (THAW_FAST_BUILD skips slow minification)
cd frontend && THAW_FAST_BUILD=1 npm run build && cd ..

# 2. Run the E2E smoke suite (auto-builds a temporary `-tags mcp` binary)
go test -tags wails_e2e -v -count=1 ./e2e/...
```

To reuse a prebuilt non-production test binary (as CI does), pass `THAW_E2E_BINARY`:

```bash
go build -tags mcp -buildvcs=false -o build/bin/thaw-wails-e2e .
THAW_E2E_BINARY=$PWD/build/bin/thaw-wails-e2e go test -tags wails_e2e -v -count=1 ./e2e/...
```

> **Warning:** Never launch a `-tags mcp` binary manually outside the E2E harness against your real profile — upstream Wails v3 leaves loopback (`127.0.0.1:9099`) unauthenticated unless `WAILS_MCP_TOKEN` is set. The harness isolates `HOME`/`XDG_*`/`APPDATA`/`USERPROFILE` and always sets a per-run bearer token.

### CI and release-build security guards

- `.github/workflows/e2e-smoke.yml` runs the full smoke suite on PRs, `main` pushes, and Renovate Wails bumps inside the Ubuntu 26.04 container under `dbus-run-session` + `xvfb-run` + `openbox` (GTK4 + WebKitGTK 6.0).
- Three layers protect release builds from enabling Wails's `-tags mcp` app-control driver:
  1. `internal/app/wails_e2e_guard.go` (`//go:build production && mcp`) fails compilation if both tags are ever combined.
  2. All platform Taskfiles (`build/*/Taskfile.yml`) compile non-dev builds with `-tags production`.
  3. `e2e/release_guard_test.go` (no build tag — runs in `go test ./...` and `e2e-smoke.yml`) scans every `.github/workflows/*.yml` (allowlisting only `e2e-smoke.yml`), asserts `build.yml`, `manual-release.yml`, and `macos-sign-test.yml` never pass `EXTRA_TAGS` or `DEV=true`, proves `go build -tags production,mcp ./internal/app` fails to compile, and enforces zero overlap between `e2e/` and `internal/mcp/`.

## Generated-artifact guards

- `TestSemanticMapAccuracy` (in `internal/architecture`) fails if an annotated domain path no longer exists — regenerate with `go generate ./internal/architecture/` after moving/removing annotated files.
- `TestGeneratedObjectKindsInSync` (in `internal/objectkind`) fails if `frontend/src/generated/objectKinds.ts` drifts from the object-kind registry — regenerate with `go generate ./internal/objectkind/`. Its companions guard the consumers that can't be generated: `snowflake.TestEveryKindIsSearchable` / `TestEveryKindIsListable` / `TestEveryKindHasDDLDisposition`, `objects.TestEveryKindHasPropertiesQuery`, and the frontend `objectIcons.test.ts` (every kind has an icon, a colour, and a colour variable declared in `global.css`). Together they make a half-wired object kind a test failure rather than a runtime surprise.
- `staticModal.test.ts` (in `frontend/src/components`) globs every `src/**` source file and fails on a static `Modal.confirm` / `.info` / `.warning` / `.error` / `.success` call. Those helpers render outside the `<ConfigProvider>` and come up white in dark mode (issue #884) — use `const { modal } = AntApp.useApp()` instead. See [`gotchas.md`](gotchas.md).
- `TestGeneratedDataTypesInSync` (in `internal/snowflake`) does the same for `frontend/src/generated/snowflakeDataTypes.ts` — regenerate with `go generate ./internal/snowflake/`.
- `TestThirdPartyNoticesUpToDate` (in the root `main` package) re-runs the notices generator into a temp file and diffs it against the committed `THIRD_PARTY_NOTICES.md`, catching a stale license list after a dependency bump — regenerate with `scripts/regen_third_party_notices.sh`, which installs `frontend/node_modules` first and then runs `scripts/gen_third_party_notices.go` (Renovate branches run the same script as a post-upgrade task — see [`CONTRIBUTING.md`](../../CONTRIBUTING.md)). The generator lists Go modules once per shipped platform (`darwin`, `linux`, `windows` — with `CGO_ENABLED=1`, as Wails builds them) and merges the sets, so its output covers every binary we ship and is byte-identical on any host: regenerating on macOS or Windows produces the same file CI does, and this test is stable everywhere. The generator refuses to write a file with zero npm packages, since a missing `node_modules` would otherwise silently drop every frontend license. It skips when `go`/`npm`/`frontend/node_modules` are unavailable (or in `-short` mode) so toolchain-light CI stays green.

## Quality gates

Run before pushing (also enforced weekly in CI):

```bash
golangci-lint run ./...
govulncheck ./...
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...
```
