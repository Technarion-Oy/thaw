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
CI, so tests that call generated `wailsjs/` IPC bindings must mock them — mock
the exact module the code imports (e.g. `wailsjs/go/sqleditor/Service`), or the
binding throws and the code under test silently hits its fallback path.

## Integration tests

Live-connection tests are in `internal/integration/`, gated behind the `integration` build tag and excluded from normal runs and docs generation. They require Snowflake credentials via environment variables (key-pair auth). The formatter dialect tests need no CREATE privileges; export and migration tests require a live account.

```bash
go test -tags integration ./internal/integration/...
```

## Generated-artifact guards

- `TestSemanticMapAccuracy` (in `internal/architecture`) fails if an annotated domain path no longer exists — regenerate with `go generate ./internal/architecture/` after moving/removing annotated files.
- `TestGeneratedObjectKindsInSync` (in `internal/objectkind`) fails if `frontend/src/generated/objectKinds.ts` drifts from the object-kind registry — regenerate with `go generate ./internal/objectkind/`. Its companions guard the consumers that can't be generated: `snowflake.TestEveryKindIsSearchable` / `TestEveryKindIsListable` / `TestEveryKindHasDDLDisposition`, `objects.TestEveryKindHasPropertiesQuery`, and the frontend `objectIcons.test.ts` (every kind has an icon, a colour, and a colour variable declared in `global.css`). Together they make a half-wired object kind a test failure rather than a runtime surprise.
- `staticModal.test.ts` (in `frontend/src/components`) globs every `src/**` source file and fails on a static `Modal.confirm` / `.info` / `.warning` / `.error` / `.success` call. Those helpers render outside the `<ConfigProvider>` and come up white in dark mode (issue #884) — use `const { modal } = AntApp.useApp()` instead. See [`gotchas.md`](gotchas.md).
- `TestGeneratedDataTypesInSync` (in `internal/snowflake`) does the same for `frontend/src/generated/snowflakeDataTypes.ts` — regenerate with `go generate ./internal/snowflake/`.
- `TestThirdPartyNoticesUpToDate` (in the root `main` package) re-runs the notices generator into a temp file and diffs it against the committed `THIRD_PARTY_NOTICES.md`, catching a stale license list after a dependency bump — regenerate with `scripts/regen_third_party_notices.sh`, which installs `frontend/node_modules` first and then runs `scripts/gen_third_party_notices.go` (Renovate branches run the same script as a post-upgrade task — see [`CONTRIBUTING.md`](../../CONTRIBUTING.md)). The generator refuses to write a file with zero npm packages, since a missing `node_modules` would otherwise silently drop every frontend license. It skips when `go`/`npm`/`frontend/node_modules` are unavailable (or in `-short` mode) so toolchain-light CI stays green.

## Quality gates

Run before pushing (also enforced weekly in CI):

```bash
golangci-lint run ./...
govulncheck ./...
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...
```
