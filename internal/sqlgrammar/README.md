# internal/sqlgrammar

Recursive-descent grammar engine for Snowflake SQL. Consumes the significant-token
stream from [`internal/sqltok`](../sqltok/README.md) and validates that a statement
conforms to a known Snowflake grammar.

## Responsibility

SQL allows nested structures (parenthesised expressions, subqueries, nested option
lists), so a plain finite-state machine isn't enough — this is a **pushdown
automaton realised as recursive descent**: Go's call stack is the automaton's
memory, the `pos` cursor is the state pointer, and the per-command `Parse*` rules
are the state transitions.

It is a **leaf package**: it imports only `internal/sqltok` and must never import
`internal/sqleditor` (import cycle). It has two consumers:

1. **Diagnostics** — `internal/sqleditor.ValidateGrammar` flags statements that
   don't conform (`[]DiagMarker`).
2. **Autocomplete** (future) — the `furthest`/`expected` machinery answers "what is
   valid next at the cursor?".

## Key files

| File | Purpose |
|------|---------|
| `engine.go` | The `Validator` type, terminals (`Match`/`MatchKeyword`/`MatchWord`/`MatchOp`), combinators (`Sequence`/`Choice`/`Optional`/`ZeroOrMore`), `furthest`/`expected` tracking + `Failure`, and shared helpers (`parseIdentPath`, `option`, `wordsValue`, `phrase`, `tagClause`, `consumeBalancedParens`, `consumeRest`, `showTrailers`, …) |
| `dispatch.go` | `Recognized()` + `ParseTopLevel()`: a leading-keyword → candidate-rules registry (bulk families by `Parse*` prefix via reflection, DML/misc leaders enumerated explicitly) |
| `create.go`, `alter.go`, `drop.go`, `show.go`, `describe.go`, `undrop.go`, `dml.go`, `grant_revoke.go`, `query_constructs.go`, `data_loading.go`, `execute.go`, `session.go`, `transactions.go` | One `func (v *Validator) ParseXxx() bool` per Snowflake command reference; the doc-comment header carries the command's documented syntax. ~716 rules total |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Engine API

```go
v := sqlgrammar.New(stmt)        // tokenizes stmt into significant tokens
if v.Recognized() && !v.ParseTopLevel() {
    f := v.Failure()             // furthest token reached + expected labels
    msg := f.Message()           // "expected FROM" / "expected one of: …"
}
```

- **Terminals** advance on match and record an `expect` label on miss. `MatchKeyword`
  matches a lexer-classified `Keyword`; **`MatchWord`** matches any identifier-like
  token by text — use it for option/clause words, since many Snowflake words
  (`SHOW`, `SET`, `LISTING`, option names, …) are lexed as `Identifier`, not `Keyword`.
- **Combinators** save & restore `pos` so a failed attempt consumes nothing
  (backtracking) — the capability the single-pass `sqleditor/tokmatch.go` scanners lack.
- `furthest`/`expected` track the furthest position reached across all attempts, so
  a failed parse yields a useful `Failure` for diagnostics and completion.

## Dispatch & conservatism

`ParseTopLevel` tries **every** rule the leading keyword selects and accepts the
statement if any rule consumes it to the end (a single trailing `;` is tolerated),
so overlapping variants (`CREATE DATABASE` vs `CREATE DATABASE ROLE`, the several
`CREATE TABLE` forms) disambiguate by longest full match.

The grammar is **deliberately lenient**: free-form spans (column/constraint lists,
`AS <query>`, procedure bodies, policy expressions, ALTER actions) are accepted via
`consumeBalancedParens`/`consumeRest`, and generic `CREATE/ALTER/DROP/… <object>`
catch-all rules accept any roughly-well-formed statement. The validator therefore
only flags clearly-broken statements (missing names, dangling keywords, unbalanced
parens) and never flags valid-but-unmodelled SQL.

## Tests

- Per-family `*_test.go` files (e.g. `create_batch_*_test.go`, `show_batch_*_test.go`)
  with valid + invalid cases per rule, via the shared `assertValid`/`assertInvalid`
  helpers in `grammar_test.go`.
- `create_meta_test.go` — reflection meta-test asserting **every** `Parse*` rule
  rejects input with no significant tokens or a non-command leading word (guards
  against stubs and over-leniency).
- `dispatch_test.go` — `Recognized`/`ParseTopLevel` across all families.

## Gotchas

- `sqltok.Token` has **no `Value` field** — recover text with `tok.Text(src)`.
- `->` lexes as **two** operator tokens (`-` then `>`); match with `MatchOp("-")`
  then `MatchOp(">")`.
- `=` and `=>` are `Operator` tokens distinguished by text (`MatchOp`).
- New per-command rules go in the matching `<verb>.go` file; sub-rules should be
  **function-local closures** (package-level helpers live in `engine.go`).
