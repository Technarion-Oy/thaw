# internal/sqlutil

> Shared SQL utilities for Snowflake-aware statement processing.

## Responsibility

Provides a single, canonical SQL statement splitter used by all packages that need to break a multi-statement SQL string into individual statements. The splitter correctly handles every quoting and comment style that Snowflake supports.

## Key files

| File | Purpose |
|------|---------|
| `split.go` | `Split(src) []string` — byte-level SQL statement splitter; handles all Snowflake quoting and comment styles |
| `split_test.go` | Comprehensive test suite and fuzz test for `Split` |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key functions

`Split(src string) []string` tokenises at the byte level and flushes on each unquoted `;`. It correctly skips semicolons inside:
- line comments (`-- ... \n`)
- block comments (`/* ... */`)
- single-quoted strings (`'...'` with `''` escaping)
- double-quoted identifiers (`"..."` with `""` escaping)
- dollar-quoted bodies (`$$...$$` or `$tag$...$tag$`)

SIMD-accelerated `strings.Index`/`strings.IndexByte` make large procedure bodies very fast to skip.

## Patterns & integration

- `internal/snowflake` uses `Split` in `Execute` and `ExecuteOnConn`, then applies `normalizePutGet` to each statement before execution.
- `internal/ddl` previously contained this splitter; `ddl.Parse` callers now use `sqlutil.Split` for the splitting step.
- `internal/migration` uses `Split` when scanning `.sql` source files and when parsing remote DDL.
- `internal/mcp` uses `Split` in the read-only gate to verify single-statement input.
- `internal/pipe` uses `Split` in `validateCopyStatement` to reject multi-statement pipe definitions.

## Gotchas

- Block comments are **not** nested (per Snowflake spec): `/* outer /* inner */` ends at the first `*/`.
- Backslash is **not** an escape character in Snowflake strings: `'a\'` closes the string.
- Dollar-quote tags are case-sensitive: `$BODY$` does not close `$body$`.
