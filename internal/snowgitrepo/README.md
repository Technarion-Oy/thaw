# internal/snowgitrepo

> SQL DDL builders for Snowflake-native `GIT REPOSITORY` objects (CREATE and ALTER).

## Responsibility

Generates `CREATE GIT REPOSITORY` and `ALTER GIT REPOSITORY` SQL statements from typed
configuration structs. This package concerns Snowflake's **server-side** Git integration
objects (stored in a Snowflake database schema), not local filesystem Git operations
(which live in `internal/gitrepo`).

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `GitRepositoryConfig`, `BuildCreateGitRepositorySql`, `BuildModifyGitRepositorySql`. |
| `sql_test.go` | Unit tests for both builders. |

## Key types & functions

| Symbol | Description |
|---|---|
| `GitRepositoryConfig` | Parameters for create/alter: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `originUrl`, `apiIntegration`, `gitCredentials`, `comment`, `tags` (`[]snowflake.TagPair`). |
| `snowflake.TagClause` | Shared `TAG (...)` clause builder (in `internal/snowflake`); git repositories use the `WITH TAG (...)` form, so the builder prepends `WITH `. |
| `BuildCreateGitRepositorySql(db, schema, cfg)` | Returns a fully-qualified `CREATE [OR REPLACE] GIT REPOSITORY` statement. Validates that `originUrl` and `apiIntegration` are non-empty. |
| `BuildModifyGitRepositorySql(db, schema, name, cfg, originalComment, originalIntegration, originalCredentials)` | Returns a slice of `ALTER GIT REPOSITORY … SET …` and/or `… UNSET …` statements (one per change type). Returns empty slice if nothing changed. |

## Patterns & integration

- Called from `internal/app/git.go` (or equivalent) thin delegators.
- String literals are escaped via `escLit` (single-quote doubling). Identifier names use `snowflake.QuoteOrBare` (case-sensitive toggle) and `snowflake.QuoteIdent` from `internal/snowflake`.
- `BuildModifyGitRepositorySql` compares new values against originals and emits `SET` or `UNSET` clauses only for changed fields, matching Snowflake's requirement that `API_INTEGRATION` cannot be UNSET.

## Gotchas

- `ORIGIN` cannot be altered after creation (Snowflake limitation) — it is only included in `BuildCreateGitRepositorySql`.
- `API_INTEGRATION` can only be SET, never UNSET — the builder skips the UNSET path for that field.
- Do not confuse this package with `internal/gitrepo` (local Git via go-git) — they serve entirely different purposes despite similar names.
