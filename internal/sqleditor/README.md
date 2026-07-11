# internal/sqleditor

> SQL diagnostics, syntax validation, JOIN suggestion engine, and autocomplete helpers — exposed to the frontend via a dedicated Wails-bound `Service`.

## Responsibility

This package implements the proprietary SQL analysis logic that backs Thaw's editor experience. All non-trivial editor intelligence lives here (in Go, not TypeScript) to protect it from reverse-engineering. The package is entirely stateless — no Snowflake connection is required.

Key capabilities:
- Structural syntax validation (unclosed strings, unmatched parens, bad scripting syntax)
- Grammar-conformance validation via the recursive-descent state machine in [`internal/sqlgrammar`](../sqlgrammar/README.md) (replaced the legacy regex/token-scanning anti-pattern & preamble checks)
- Data-type validation in DDL and CAST expressions
- Object-existence validation — a referenced table/view/schema/database must exist in the catalog **or be created earlier in the same script** (`ValidateTablesExist`). The "no database/schema selected" check on an unqualified CREATE name covers all schema-scoped object types (TABLE, VIEW, SEQUENCE, STAGE, STREAM, TASK, PIPE, FILE FORMAT, FUNCTION, the policy family, …) via `matchCreateSchemaScoped`, which reads the schema-scoped phrase list from `snowflake.SchemaScopedObjectTypes()` (the single source of truth for object scope). Account-level objects (DATABASE, WAREHOUSE, ROLE, NETWORK POLICY, integrations, …) are excluded there.
- JOIN ON / USING condition suggestions (3-tier: FK → PK heuristic → type-compatible same-name columns)
- Autocomplete context bundling (statement ranges, scripting variables, CTE columns, table refs, ref resolution)
- LCS-based line diff for git gutter decorations
- SQL keyword/identifier casing transformation

## Key files

| File | Purpose |
|------|---------|
| `service.go` | `Service` struct (Wails-bound, stateless); thin delegators to package-level functions |
| `sqleditor.go` | Core types (`DiagMarker`, `JoinTableRef`, `ResolvedRef`, `ColInfo`, `ColEntry`, `JoinCondition`, `AutocompleteContext`, `GrammarExpectation`, `UseContext`, `LineDiff`, etc.) and main analysis functions (incl. `GrammarExpectedAt`, the grammar-driven completion bridge to `internal/sqlgrammar`) |
| `patterns.go` | `ValidateDataTypes`; regex constants `ReIdentifier`, `_ident`, `_identPath`; `ApplyCasing`; the `matchesSnowflakeFP` false-positive guard shared by the bare-column-ref and table-existence validators |
| `grammar.go` | `ValidateGrammar` — per-statement check against the recursive-descent grammar in `internal/sqlgrammar`; rebases failure positions to absolute doc coordinates |
| `antipatterns.go` | `ValidateAntiPatterns` — semantic checks the grammar can't perform: MERGE clause-action validity, QUALIFY placement, FLATTEN/LATERAL usage, variant-path traversal, unknown Cortex functions, stray token / dangling `AS` after a FROM/JOIN table reference, PIVOT/UNPIVOT/MATCH_RECOGNIZE/ASOF JOIN clause shape, INSERT OVERWRITE/ALL/FIRST structure, Time Travel `AT`/`BEFORE`, and cross-statement transaction tracking (nested `BEGIN`, stray `COMMIT`/`ROLLBACK`, uncommitted transaction) |
| `barecolrefs.go` | `ValidateBareColumnRefs`, `ExtractInEditorTableDefs` — validates INSERT column lists and CREATE TABLE REFERENCES; extracts in-editor table columns for pre-execution autocomplete |
| `tableexist.go` | `ValidateTablesExist` — checks SELECT/CREATE/ALTER/DROP/UNDROP for unresolvable table/schema/database references and unqualified schema-scoped CREATEs with no active database/schema (`matchCreateSchemaScoped`); emits quick-fix `Code` JSON when a table exists in another schema |
| `starselect.go` | `StarSelectAt(sql, line, col) *StarSelect` — tokenizer-based (`sqltok`) detection of a select-list wildcard (`*`/`alias.*`) at a Monaco cursor position; returns its replace span + qualifier or nil. Ignores a `*` inside a quoted identifier and a `*` multiplication; walks the full dotted chain so a multi-part `db.schema.tbl.*` replace range starts at the first segment; converts sqltok byte columns to Monaco UTF-16 columns (`utf16Col`) so non-ASCII earlier on the line doesn't shift the range. `FromSourceCount(sql) int` — number of *plain-table* top-level FROM sources (JOIN/comma separated), or -1 when a bare `*` can't be safely expanded: nested SELECT/CTE, no FROM, or a non-table source (subquery, table function `TABLE(…)`, or a `PIVOT`/`UNPIVOT`/`SAMPLE` clause — all detected as a `(` opening at a source position, i.e. not inside an `ON`/`USING` condition). The "Expand \*" command compares it to the resolved-ref count to refuse incomplete/wrong expansions. Backs the editor's "Expand \*" context menu |
| `diaghelpers.go` | Shared internal helpers: `stripCommentsSQL`, `stripStringLiterals`, `getFirstSQLToken` |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Wails binding
`Service` (`service.go`) is registered in `internal/app/run.go`'s `Bind` array. The frontend imports its methods from `wailsjs/go/sqleditor/Service` — **not** from `wailsjs/go/app/App`. Every method is a one-liner that calls the corresponding package-level function.

### Syntax & semantic validation
- `ValidateSyntax(sql) []DiagMarker` — walks the `internal/sqltok` token stream (recursing into `$$` scripting bodies and rebasing line/col); flags unclosed strings/parens/comments, bad `$$` scripting syntax, placeholder tokens, wrong `:=`/`=` assignments, undeclared variables
- `ValidateSemantics(sql, resolvedRefs, colEntries) []DiagMarker` — alias.column reference validator
- `ValidateDataTypes(sql, stmtRanges) []DiagMarker` — unrecognised Snowflake type names in CREATE TABLE, CAST, `::`
- `ValidateAntiPatterns(sql, stmtRanges) []DiagMarker` — semantic Snowflake anti-patterns the grammar engine can't see (it consumes clause bodies permissively): `INSERT` in a MERGE `WHEN MATCHED` clause (and `UPDATE`/`DELETE` in `WHEN NOT MATCHED`, plus the unsupported `WHEN NOT MATCHED BY SOURCE`), `QUALIFY` placed after `ORDER BY`, `FLATTEN` used as a table function without `LATERAL`/`TABLE(...)`, the `LATERALFLATTEN` typo, dotted variant-path traversal that should use `:`, unknown `SNOWFLAKE.CORTEX.<fn>` names, a stray token or dangling `AS` after a FROM/JOIN table reference (`FROM t 1000`, `FROM t AS`, `FROM t a AS`), malformed `PIVOT`/`UNPIVOT`/`MATCH_RECOGNIZE`/`ASOF JOIN` clauses, `INSERT OVERWRITE`/`ALL`/`FIRST` structure, Time Travel `AT`/`BEFORE` clause shape, and **cross-statement** transaction tracking (nested `BEGIN`, stray `COMMIT`/`ROLLBACK`, transaction left open at end of script). All **Warnings**. (Re-homed from the removed `ValidateSnowflakePatterns`.) Statement-type dispatch uses `sqlgrammar.IdentifyStatement` (CTE-aware) and the clause validators are gated on significant-token presence (`sigWordSet`), never `strings.Contains` — which mis-fires on `AT` inside `CREATE`/`DATE` or `PIVOT` inside `UNPIVOT`.
- `ValidateGrammar(sql, stmtRanges) []DiagMarker` — validates each statement against the recursive-descent Snowflake grammar in [`internal/sqlgrammar`](../sqlgrammar/README.md). Only statements whose leading keyword maps to an implemented grammar (`sqlgrammar.Validator.Recognized`) are checked; a non-conforming one yields a single **Warning** at the furthest position the grammar reached (with its `expected …` message). The generic catch-all rules are excluded from dispatch, so it flags unknown object types and malformed statements — e.g. a `CREATE TABLE` with no body, an empty column list, or a column without a data type (the data-type *name* itself is checked by `ValidateDataTypes`, not duplicated here)
- `ValidateTablesExist(req ValidateTablesExistRequest) []DiagMarker` — checks tables/schemas/databases against resolved refs; populates `DiagMarker.Code` with JSON quick-fix metadata (`{"kind":"qualify-table","original":"FOO","suggestions":["DB.SCHEMA.FOO"]}`)
- `ValidateBareColumnRefs(req ValidateBareColsRequest) []DiagMarker` — validates INSERT and CREATE TABLE REFERENCES column lists

### JOIN suggestions
- `ParseJoinTables(sql) []JoinTableRef` — FROM/JOIN/USING/MERGE INTO + USE extractor (3/2/1-part + alias); a keyword-anchored scan over the `sqltok` significant-token stream (mirrors `snowflake/lineage.go`), so comments/string literals never yield phantom refs
- `ComputeJoinOnConditions(req JoinOnSuggestionsReq) []JoinCondition` — three-tier engine: (1) FK constraints, (2) PK naming heuristic, (3) type-compatible same-name columns + USING
- `ResolveTableRefs(refs, storeObjects, useCtx, session) []ResolvedRef` — qualifies unresolved refs against store objects, `UseContext`, and session context (priority: fully-qualified → store match → UseContext → session)

### Autocomplete context
- `GetAutocompleteContextFull(req AutocompleteContextRequest) AutocompleteContext` — **the IPC entry point** the completion provider calls. Bundles statement ranges, scripting completions, table refs, CTE column projections, and `UseContext`, then extends them with backend ref resolution (`ResolvedRefs`), in-editor CREATE TABLE extraction (`InEditorTables`), the context-detection flags (`IsDatatypeCtx`, `IsInJoinOnClause`, `UsingClause`), and the grammar-driven `GrammarExpected`
- The following are **package-level helpers only** (no IPC wrapper): `GetAutocompleteContextFull` computes them server-side so the frontend needs a single round-trip —
  - `GetAutocompleteContext(sql, cursorOffset)` — the un-resolved bundle `…Full` builds on; also computes `GrammarExpected`
  - `GetScriptingCompletions(sql, cursorOffset)` — declared Snowflake Scripting variables visible at cursor
  - `GrammarExpectedAt(stmt, localOffset) *GrammarExpectation` — **grammar-driven completion.** Parses the statement prefix up to the cursor with [`internal/sqlgrammar`](../sqlgrammar/README.md)'s `Validator.ExpectedAt` and classifies its "valid next" set into `Keywords` (literal keyword/option words the provider offers verbatim — e.g. `FROM` after `COPY INTO <table>`, the object types after `CREATE`/`DROP`, the verbs after `ALTER TABLE <name>`) and `Kinds` (token-kind expectations like `Identifier`/`StringLit` that the existing catalog/column/stage sources already fill). Returns nil for unmodelled leading keywords, so completion is **leading-keyword-gated** — unmodelled SQL keeps the legacy detector behavior. Migrates the ad-hoc detectors below over time (issue #557)
  - `IsDatatypeContext(textToCursor, lineUpToWord)` — cursor after `::`, `CAST AS`, `DECLARE`, or a column definition
  - `IsInJoinOnClause(textToCursor)` — cursor inside a JOIN … ON … not yet terminated
  - `DetectUsingClause(textToCursor)` — `InUsing` (empty USING) vs `IsPartial` (partial column list)
- `GetStatementRanges(sql) []StatementRange` — per-statement line ranges and byte offsets
- `GetIdentifierAtColumn(line, col) []string` — dot-separated identifier parts under cursor
- `StarSelectAt(sql, line, col) *StarSelect` — select-list wildcard (`*`/`alias.*`) at a cursor position, with its replace span + qualifier (nil when not a wildcard)
- `FromSourceCount(sql) int` — plain-table top-level FROM source count, or -1 when a bare `*` can't be safely expanded (nested SELECT/CTE, no FROM, or a non-table source: subquery / table function / PIVOT etc.)
- `GetActiveFunctionCall(prefix) *FunctionCallContext` — innermost open function call + active parameter index
- `ParseSignatureParams(sig) []SignatureParam` — byte spans of parameters for Monaco parameter highlighting
- `ExtractInEditorTableDefs(sql) []InEditorTableDef` — reuses `parseCreateTableColDefs` from `barecolrefs.go`

### Other
- `ComputeGitLineDiff(headLines, currentLines, maxLines) LineDiff` — LCS-based line diff; returns 1-based line numbers for added/modified/deleted regions; returns empty slices when either input exceeds `maxLines`
- `ApplyCasing(sql, keywordCase, identifierCase, functionCase) string` — token-level casing; quoted identifiers, string literals, dollar-quoted blocks, and comments pass through unchanged
- `GetSnowflakeKeywords() []string` — delegates to `snowflake.ReservedKeywords()`

## Patterns & integration

- Registered as a second Wails-bound struct alongside `*app.App`; frontend calls `wailsjs/go/sqleditor/Service.*` methods.
- All functions are pure (no shared mutable state); safe to call concurrently.
- `DiagMarker.Severity` follows Monaco constants: `8` = Error (red), `4` = Warning (yellow).
- `DiagMarker.Code` carries JSON quick-fix metadata when `ValidateTablesExist` finds a table in another schema; the frontend `CodeActionProvider` parses this to offer lightbulb qualification actions.

## Gotchas

- The `validateWithParser` and `validateBareColumnRefs` paths in the frontend (`sqlDiagnostics.ts`) still use `node-sql-parser` for checks that have no Go equivalent; this package does not replace those paths.
- `ComputeGitLineDiff` returns empty slices (not an error) when either input exceeds `maxLines`. Callers must check for this case to avoid rendering stale gutter decorations.
- `GetAutocompleteContextFull` is the **only** autocomplete IPC entry point; `GetAutocompleteContext`, `GetScriptingCompletions`, `IsDatatypeContext`, `IsInJoinOnClause`, and `DetectUsingClause` are package-level helpers it calls server-side (their standalone `Service` wrappers were removed once the frontend consolidated onto `…Full`).
