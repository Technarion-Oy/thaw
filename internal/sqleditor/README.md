# internal/sqleditor

> SQL diagnostics, syntax validation, JOIN suggestion engine, and autocomplete helpers — exposed to the frontend via a dedicated Wails-bound `Service`.

## Responsibility

This package implements the proprietary SQL analysis logic that backs Thaw's editor experience. All non-trivial editor intelligence lives here (in Go, not TypeScript) to protect it from reverse-engineering. The package is entirely stateless — no Snowflake connection is required.

Key capabilities:
- Structural syntax validation (unclosed strings, unmatched parens, bad scripting syntax)
- Grammar-conformance validation via the recursive-descent state machine in [`internal/sqlgrammar`](../sqlgrammar/README.md) (replaced the legacy regex/token-scanning anti-pattern & preamble checks)
- Data-type validation in DDL and CAST expressions
- Object-existence validation — a referenced table/view/schema/database must exist in the catalog **or be created earlier in the same script** (`ValidateTablesExist`)
- JOIN ON / USING condition suggestions (3-tier: FK → PK heuristic → type-compatible same-name columns)
- Autocomplete context bundling (statement ranges, scripting variables, CTE columns, table refs, ref resolution)
- LCS-based line diff for git gutter decorations
- SQL keyword/identifier casing transformation

## Key files

| File | Purpose |
|------|---------|
| `service.go` | `Service` struct (Wails-bound, stateless); thin delegators to package-level functions |
| `sqleditor.go` | Core types (`DiagMarker`, `JoinTableRef`, `ResolvedRef`, `ColInfo`, `ColEntry`, `JoinCondition`, `AutocompleteContext`, `UseContext`, `LineDiff`, etc.) and main analysis functions |
| `patterns.go` | `ValidateDataTypes`; regex constants `ReIdentifier`, `_ident`, `_identPath`; `ApplyCasing`; the `matchesSnowflakeFP` false-positive guard shared by the bare-column-ref and table-existence validators |
| `grammar.go` | `ValidateGrammar` — per-statement check against the recursive-descent grammar in `internal/sqlgrammar`; rebases failure positions to absolute doc coordinates |
| `antipatterns.go` | `ValidateAntiPatterns` — semantic checks the grammar can't perform: MERGE clause-action validity, QUALIFY placement, FLATTEN/LATERAL usage, variant-path traversal, unknown Cortex functions, PIVOT/UNPIVOT/MATCH_RECOGNIZE/ASOF JOIN clause shape, INSERT OVERWRITE/ALL/FIRST structure, Time Travel `AT`/`BEFORE`, and cross-statement transaction tracking (nested `BEGIN`, stray `COMMIT`/`ROLLBACK`, uncommitted transaction) |
| `barecolrefs.go` | `ValidateBareColumnRefs`, `ExtractInEditorTableDefs` — validates INSERT column lists and CREATE TABLE REFERENCES; extracts in-editor table columns for pre-execution autocomplete |
| `tableexist.go` | `ValidateTablesExist` — checks SELECT/CREATE/ALTER/DROP/UNDROP for unresolvable table/schema/database references; emits quick-fix `Code` JSON when a table exists in another schema |
| `diaghelpers.go` | Shared internal helpers: `stripCommentsSQL`, `stripStringLiterals`, `getFirstSQLToken` |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Wails binding
`Service` (`service.go`) is registered in `internal/app/run.go`'s `Bind` array. The frontend imports its methods from `wailsjs/go/sqleditor/Service` — **not** from `wailsjs/go/app/App`. Every method is a one-liner that calls the corresponding package-level function.

### Syntax & semantic validation
- `ValidateSyntax(sql) []DiagMarker` — walks the `internal/sqltok` token stream (recursing into `$$` scripting bodies and rebasing line/col); flags unclosed strings/parens/comments, bad `$$` scripting syntax, placeholder tokens, wrong `:=`/`=` assignments, undeclared variables
- `ValidateSemantics(sql, resolvedRefs, colEntries) []DiagMarker` — alias.column reference validator
- `ValidateDataTypes(sql, stmtRanges) []DiagMarker` — unrecognised Snowflake type names in CREATE TABLE, CAST, `::`
- `ValidateAntiPatterns(sql, stmtRanges) []DiagMarker` — semantic Snowflake anti-patterns the grammar engine can't see (it consumes clause bodies permissively): `INSERT` in a MERGE `WHEN MATCHED` clause (and `UPDATE`/`DELETE` in `WHEN NOT MATCHED`, plus the unsupported `WHEN NOT MATCHED BY SOURCE`), `QUALIFY` placed after `ORDER BY`, `FLATTEN` used as a table function without `LATERAL`/`TABLE(...)`, the `LATERALFLATTEN` typo, dotted variant-path traversal that should use `:`, unknown `SNOWFLAKE.CORTEX.<fn>` names, malformed `PIVOT`/`UNPIVOT`/`MATCH_RECOGNIZE`/`ASOF JOIN` clauses, `INSERT OVERWRITE`/`ALL`/`FIRST` structure, Time Travel `AT`/`BEFORE` clause shape, and **cross-statement** transaction tracking (nested `BEGIN`, stray `COMMIT`/`ROLLBACK`, transaction left open at end of script). All **Warnings**. (Re-homed from the removed `ValidateSnowflakePatterns`.) Statement-type dispatch uses `sqlgrammar.IdentifyStatement` (CTE-aware) and the clause validators are gated on significant-token presence (`sigWordSet`), never `strings.Contains` — which mis-fires on `AT` inside `CREATE`/`DATE` or `PIVOT` inside `UNPIVOT`.
- `ValidateGrammar(sql, stmtRanges) []DiagMarker` — validates each statement against the recursive-descent Snowflake grammar in [`internal/sqlgrammar`](../sqlgrammar/README.md). Only statements whose leading keyword maps to an implemented grammar (`sqlgrammar.Validator.Recognized`) are checked; a non-conforming one yields a single **Warning** at the furthest position the grammar reached (with its `expected …` message). The generic catch-all rules are excluded from dispatch, so it flags unknown object types and malformed statements — e.g. a `CREATE TABLE` with no body, an empty column list, or a column without a data type (the data-type *name* itself is checked by `ValidateDataTypes`, not duplicated here)
- `ValidateTablesExist(req ValidateTablesExistRequest) []DiagMarker` — checks tables/schemas/databases against resolved refs; populates `DiagMarker.Code` with JSON quick-fix metadata (`{"kind":"qualify-table","original":"FOO","suggestions":["DB.SCHEMA.FOO"]}`)
- `ValidateBareColumnRefs(req ValidateBareColsRequest) []DiagMarker` — validates INSERT and CREATE TABLE REFERENCES column lists

### JOIN suggestions
- `ParseJoinTables(sql) []JoinTableRef` — regex-based FROM/JOIN extractor (3/2/1-part + alias)
- `ComputeJoinOnConditions(req JoinOnSuggestionsReq) []JoinCondition` — three-tier engine: (1) FK constraints, (2) PK naming heuristic, (3) type-compatible same-name columns + USING
- `ResolveTableRefs(refs, storeObjects, useCtx, session) []ResolvedRef` — qualifies unresolved refs against store objects, `UseContext`, and session context (priority: fully-qualified → store match → UseContext → session)

### Autocomplete context
- `GetAutocompleteContext(sql, cursorOffset) AutocompleteContext` — bundles statement ranges, scripting completions, table refs, CTE column projections, and `UseContext` in one IPC round-trip
- `GetAutocompleteContextFull(req AutocompleteContextRequest) AutocompleteContext` — extends the above with backend ref resolution (`ResolvedRefs`), in-editor CREATE TABLE extraction (`InEditorTables`), and context-detection flags (`IsDatatypeCtx`, `IsInJoinOnClause`, `UsingClause`)
- `IsDatatypeContext(textToCursor, lineUpToWord) bool` — detects cursor position after `::`, `CAST AS`, `DECLARE`, or column definition in `CREATE/ALTER TABLE`
- `IsInJoinOnClause(textToCursor) bool` — detects cursor inside a JOIN … ON … not yet terminated
- `DetectUsingClause(textToCursor) UsingClauseInfo` — `InUsing` (empty USING) vs `IsPartial` (partial column list)
- `GetScriptingCompletions(sql, cursorOffset) ScriptingCompletionResult` — declared Snowflake Scripting variables visible at cursor
- `GetStatementRanges(sql) []StatementRange` — per-statement line ranges and byte offsets
- `GetIdentifierAtColumn(line, col) []string` — dot-separated identifier parts under cursor
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
- `GetAutocompleteContextFull` supersedes `GetAutocompleteContext` for the main completion provider; `GetAutocompleteContext` remains for lighter-weight hover/diagnostics paths that already have resolved refs.
