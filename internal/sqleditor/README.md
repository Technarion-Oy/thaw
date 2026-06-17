# internal/sqleditor

> SQL diagnostics, syntax validation, JOIN suggestion engine, and autocomplete helpers — exposed to the frontend via a dedicated Wails-bound `Service`.

## Responsibility

This package implements the proprietary SQL analysis logic that backs Thaw's editor experience. All non-trivial editor intelligence lives here (in Go, not TypeScript) to protect it from reverse-engineering. The package is entirely stateless — no Snowflake connection is required.

Key capabilities:
- Structural syntax validation (unclosed strings, unmatched parens, bad scripting syntax)
- Anti-pattern and statement-preamble validation
- Data-type validation in DDL and CAST expressions
- JOIN ON / USING condition suggestions (3-tier: FK → PK heuristic → type-compatible same-name columns)
- Autocomplete context bundling (statement ranges, scripting variables, CTE columns, table refs, ref resolution)
- LCS-based line diff for git gutter decorations
- SQL keyword/identifier casing transformation

## Key files

| File | Purpose |
|------|---------|
| `service.go` | `Service` struct (Wails-bound, stateless); thin delegators to package-level functions |
| `sqleditor.go` | Core types (`DiagMarker`, `JoinTableRef`, `ResolvedRef`, `ColInfo`, `ColEntry`, `JoinCondition`, `AutocompleteContext`, `UseContext`, `LineDiff`, etc.) and main analysis functions |
| `patterns.go` | `ValidateSnowflakePatterns`, `ValidateDataTypes`; regex constants `ReIdentifier`, `_ident`, `_identPath`; `ApplyCasing` |
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
- `ValidateSnowflakePatterns(sql, stmtRanges) []DiagMarker` — anti-pattern checks, preamble validation
- `ValidateDataTypes(sql, stmtRanges) []DiagMarker` — unrecognised Snowflake type names in CREATE TABLE, CAST, `::`
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
