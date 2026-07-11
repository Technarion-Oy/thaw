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
2. **Autocomplete** — `Validator.ExpectedAt(cursorOffset)` answers "what is valid
   next at the cursor?" by parsing the prefix before the cursor and returning the
   `furthest`/`expected` set. `internal/sqleditor.GrammarExpectedAt` classifies
   that set into keyword vs token-kind completions for the frontend provider.

## Key files

| File | Purpose |
|------|---------|
| `engine.go` | The `Validator` type, terminals (`Match`/`MatchKeyword`/`MatchWord`/`MatchOp`), combinators (`Sequence`/`Choice`/`Optional`/`ZeroOrMore`/`unorderedOnce` — the last for order-independent parameter lists where each option may appear at most once, so duplicates are rejected and `ExpectedAt` stops offering a parameter already set), `furthest`/`expected` tracking + `Failure`, and shared helpers (`parseIdentPath`, `option`, `wordsValue`, `phrase`, `tagClause`, `consumeBalancedParens`, `consumeRest`, `showTrailers`, …) |
| `dispatch.go` | `Recognized()` + `ParseTopLevel()`: a leading-keyword → candidate-rules registry (bulk families by `Parse*` prefix via reflection, DML/misc leaders enumerated explicitly); `ExpectedAt(cursorOffset)` — the autocomplete "valid next" accessor; plus `IdentifyStatement()` — the effective-verb classifier that looks past a leading `WITH`/CTE prefix |
| `create.go`, `alter.go`, `drop.go`, `show.go`, `describe.go`, `undrop.go`, `dml.go`, `grant_revoke.go`, `query_constructs.go`, `data_loading.go`, `execute.go`, `session.go`, `transactions.go`, `scripting.go` | One `func (v *Validator) ParseXxx() bool` per Snowflake command reference; the doc-comment header carries the command's documented syntax. ~716 rules total |
| `scripting.go` | Snowflake Scripting procedural constructs — a separate grammar layer. `ParseScriptingBlock` is the structural core: `[ DECLARE … ] BEGIN <stmt>; … [ EXCEPTION … ] END`, dispatched under `BEGIN` (alongside the transaction `ParseBegin`) and `DECLARE`, and recursing as a nested block. `parseScriptingStatement` is the **shared block-body `Choice`** the loop/branch/cursor issues extend by inserting their construct ahead of the permissive statement-span catch-all. Other constructs (`AWAIT`, `BREAK`/`EXIT` — the latter wired into the block-body `Choice`, …) live here too |
| `ctenames.go` | `CollectCTENames(src, sig)` — the single structural CTE-alias-name scanner (handles `WITH RECURSIVE`, CTE column lists, nested WITH, unterminated bodies mid-typing). Shared by `internal/snowflake` lineage extraction and `internal/sqleditor` table-existence diagnostics (issue #559); callers apply their own normalisation |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Engine API

```go
v := sqlgrammar.New(stmt)        // tokenizes stmt into significant tokens
if v.Recognized() && !v.ParseTopLevel() {
    f := v.Failure()             // furthest token reached + expected labels
    msg := f.Message()           // "unexpected 'GROUP', expected FROM" / "unexpected end of statement, expected one of: …"
}

kind := v.IdentifyStatement()    // StmtSelect/Insert/Update/Delete/Merge (past a WITH/CTE prefix), else StmtOther

expected := v.ExpectedAt(cursor) // autocomplete: keywords/kinds valid at byte offset `cursor`
                                 // (parses the prefix before the cursor; drops the half-typed word abutting it)
```

The message names both what was **found** (the furthest token, quoted, or
`end of statement`) and what was **expected** there — naming the offending token is
what makes it actionable, since backtracking would otherwise rewind to token 0.

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

The grammar is lenient about **free-form spans** — `AS <query>`, procedure bodies,
policy expressions, ALTER actions are consumed via `consumeBalancedParens`/
`consumeRest` rather than modelled token-for-token. But it is **strict about
statement skeletons**: the generic `CREATE/ALTER/DROP/… <object>` index-page rules
(`ParseCreateObj`, `ParseAlterObj`, …) are excluded from dispatch (see
`dispatchExclude` in `dispatch.go`) so the specific per-command rules govern.
Consequently the validator flags unknown object types, missing required
actions/bodies, and malformed column lists — e.g. `CREATE TABLE t` (no body),
`CREATE TABLE t (dsdfssf)` (column with no data type), and `CREATE WIDGET w` are all
reported. `CREATE TABLE` requires a real column-definition list (each column
`<name> <datatype>`, the data-type *name* validated by `sqleditor.ValidateDataTypes`),
a CTAS column-alias list followed by `AS <query>`, or `AS`/`LIKE`/`CLONE`/
`USING TEMPLATE`/`FROM ARCHIVE`. The `CREATE OR ALTER` form is accepted everywhere
via `orReplace`.

`SELECT` is modelled as a statement skeleton (`ParseSelect` in `dml.go`, helpers in
`query_constructs.go`): `SELECT [ ALL | DISTINCT ] [ TOP <n> ] <projection>` followed
by the ordered optional `FROM` / `WHERE` / `GROUP BY` / `HAVING` / `QUALIFY` clauses,
the trailing `ORDER BY` / `LIMIT` / `OFFSET` / `FETCH` / `FOR UPDATE` clauses, and
set operators (`UNION` / `INTERSECT` / `EXCEPT` / `MINUS`) chaining further blocks.
Each clause **body** is consumed permissively up to the next clause boundary
(`consumeClauseBody`, skipping balanced parens so a boundary keyword nested in a
subquery or function call — `EXTRACT(YEAR FROM dt)` — does not end the clause), so
valid queries are accepted while the clause keywords are surfaced at every boundary
for `ExpectedAt` autocomplete. A non-empty projection is required, so `SELECT` with
zero columns (`SELECT`, `SELECT FROM t`) and a dangling `FROM`/`GROUP` are flagged.
A comma-list body that ends in a **trailing comma** (`SELECT a, <cursor>`, `FROM t1,
<cursor>`) is likewise treated as incomplete: the clause stays "still being typed",
so `ExpectedAt` reports the item label (`expression`, `identifier`) instead of the
next clause's keyword — that is what lets autocomplete offer another column/table at
the cursor (e.g. on a blank line mid-`SELECT`) rather than `FROM`/`WHERE`.

## Tests

- Per-family `*_test.go` files (e.g. `create_batch_*_test.go`, `show_batch_*_test.go`)
  with valid + invalid cases per rule, via the shared `assertValid`/`assertInvalid`
  helpers in `grammar_test.go`.
- `create_meta_test.go` — reflection meta-test asserting **every** `Parse*` rule
  rejects input with no significant tokens or a non-command leading word (guards
  against stubs and over-leniency).
- `dispatch_test.go` — `Recognized`/`ParseTopLevel` across all families.
- `classify_test.go` — `IdentifyStatement` (incl. the WITH/CTE bypass) and the
  `Failure.Message` token naming.

## Gotchas

- `sqltok.Token` has **no `Value` field** — recover text with `tok.Text(src)`.
- `->` lexes as **two** operator tokens (`-` then `>`); match with `MatchOp("-")`
  then `MatchOp(">")`.
- `=` and `=>` are `Operator` tokens distinguished by text (`MatchOp`).
- New per-command rules go in the matching `<verb>.go` file; sub-rules should be
  **function-local closures** (package-level helpers live in `engine.go`).
