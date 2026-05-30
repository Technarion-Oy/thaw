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

## Integration tests

Live-connection tests are in `internal/integration/`, gated behind the `integration` build tag and excluded from normal runs and docs generation. They require Snowflake credentials via environment variables (key-pair auth). The formatter dialect tests need no CREATE privileges; export and migration tests require a live account.

```bash
go test -tags integration ./internal/integration/...
```

## Generated-artifact guards

- `TestSemanticMapAccuracy` (in `internal/architecture`) fails if an annotated domain path no longer exists — regenerate with `go generate ./internal/architecture/` after moving/removing annotated files.

## Quality gates

Run before pushing (also enforced weekly in CI):

```bash
golangci-lint run ./...
govulncheck ./...
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...
```
