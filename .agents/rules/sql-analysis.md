---
trigger: glob
globs: "internal/sqltok/**/*.go, internal/sqlgrammar/**/*.go, internal/sqleditor/**/*.go"
description: "Snowflake SQL tokenizer, recursive-descent grammar engine, and diagnostics/autocomplete invariants."
---

# Thaw SQL Analysis Rules (`sqltok` / `sqlgrammar` / `sqleditor`)

Follow these invariants when modifying Thaw's SQL tokenizer, grammar engine, or editor diagnostics. Read the package READMEs (`internal/sqltok/README.md`, `internal/sqlgrammar/README.md`, `internal/sqleditor/README.md`) before making changes.

## Package Dependency Boundaries
- Strict one-way layering: `internal/sqltok` (leaf, no Thaw imports) ← `internal/sqlgrammar` (imports ONLY `internal/sqltok`) ← `internal/sqleditor` (stateless Wails-bound `Service`, no Snowflake connection).
- Never introduce reverse imports or hardcode Snowflake data-type lists in `sqltok` (they are injected from `internal/snowflake` via `RegisterDataTypeKeywords`).

## Tokenizer (`internal/sqltok`)
- `sqltok.Token` has **no `Value` field** — always recover text with `tok.Text(src)`.
- `->` lexes as **two** operator tokens (`-` then `>`); match with `MatchOp("-")` followed by `MatchOp(">")`.
- `=` and `=>` are both `Operator` tokens distinguished by text (`MatchOp`).
- Remember `//` is a Snowflake line comment and `/* /* */ */` block comments nest; never replace `sqltok` with ad-hoc string/regex scanning.

## Grammar (`internal/sqlgrammar`)
- Place new `func (v *Validator) ParseXxx() bool` rules in the matching `<verb>.go` file (`create.go`, `alter.go`, `drop.go`, `show.go`, `dml.go`, `scripting.go`, etc.) with the documented Snowflake syntax in the doc comment; keep sub-rules as function-local closures and shared combinators in `engine.go`.
- Use `MatchWord` (not `MatchKeyword`) for option/clause words, because many Snowflake words (`SHOW`, `SET`, `LISTING`, option names) lex as `Identifier`.
- Use `unorderedOnce` for order-independent option lists so duplicates are rejected and `ExpectedAt` stops suggesting already-set options.
- Keep generic catch-all rules excluded in `dispatchExclude` (`dispatch.go`) so specific per-command rules govern diagnostics and `ExpectedAt` completion.
- Every `Parse*` rule is checked by `create_meta_test.go` — it must reject empty input and non-command leading words.

## Diagnostics & Autocomplete (`internal/sqleditor`)
- **Never re-find a matched span via raw `strings.Index(sql, rawPath)`** — it can latch onto comments or string literals earlier in the statement. Pass `pathIdx` in `sig` directly or convert byte offsets via `sigIndexAtOffset(sig, off)`.
- Missing catalog data must **never** produce false positives (`hasSchemaDataForDB`, `FetchedObjectSchemas`, `isAlwaysPresentSchema`).
- `DiagMarker.Severity` follows Monaco constants: `8` = Error, `4` = Warning.
- `GetAutocompleteContextFull` is the single autocomplete IPC entry point on `sqleditor.Service`; keep helper steps as package-level functions.
- After editing any method signature on `sqleditor.Service`, run `wails3 generate bindings -ts -i -clean=true`.
