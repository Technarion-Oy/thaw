# internal/sqlutil

> **Deprecated** — `Split` now delegates to [`internal/sqltok.Split`](../sqltok/README.md). New callers should use `sqltok.Split` directly.

## Responsibility

Provides a thin wrapper around `sqltok.Split` for backward compatibility. `Split` delegates directly to `sqltok.Split`; the test suite verifies that the wrapper produces correct, non-empty statement splits across a comprehensive corpus and a fuzz test.

## Key files

| File | Purpose |
|------|---------|
| `split.go` | `Split(src) []string` — deprecated wrapper delegating to `sqltok.Split` |
| `split_test.go` | Comprehensive test suite and fuzz test (validates the sqltok implementation) |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Gotchas

- Block comments **nest** (per Snowflake spec): in `/* outer /* inner */ still */`, the inner `*/` only closes the inner comment; the whole span is one comment.
- Backslash is **not** an escape character in Snowflake strings: `'a\'` closes the string.
- Dollar-quote tags are case-sensitive: `$BODY$` does not close `$body$`.
