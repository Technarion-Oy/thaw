# internal/integration

> Live-connection end-to-end tests requiring a real Snowflake account; excluded from regular `go test` runs.

## Responsibility

Contains integration tests that exercise the full stack against an actual Snowflake
instance: query execution, DDL export, DBT project operations, schema migrations, and SQL
formatting. They are opt-in only and are never run in normal CI without explicit setup.

## Key files

| File | Purpose |
|---|---|
| `basic_test.go` | Core connectivity and query tests; defines the shared `keyPairConnFromEnv` helper. |
| `export_test.go` | DDL export pipeline tests. |
| `dbt_test.go` | DBT project SQL builder tests against a live account. |
| `migration_test.go` | Schema migration engine tests. |
| `formatter_test.go` | SQL formatter round-trip tests. |

## Patterns & integration

All tests carry the `//go:build integration` build tag and belong to `package integration_test`.

Run with:

```bash
go test -v -tags integration -timeout 5m ./internal/integration/
```

Required environment variables:

| Variable | Description |
|---|---|
| `SNOWFLAKE_ACCOUNT` | Account identifier, e.g. `myorg-myaccount` |
| `SNOWFLAKE_USER` | Login name |
| `SNOWFLAKE_PRIVATE_KEY` | PEM-encoded PKCS#8 RSA private key (unencrypted) |
| `SNOWFLAKE_WAREHOUSE` | Warehouse to use, e.g. `COMPUTE_WH` |

Tests are **skipped** (not failed) when any required variable is absent, so the suite
degrades gracefully in CI environments without Snowflake credentials.

## Gotchas

- The tests are designed for a freshly created user with no custom grants (PUBLIC role only). No `CREATE DATABASE` or elevated privileges are required.
- This package is excluded from documentation generation and from the normal `go test ./internal/...` sweep.
