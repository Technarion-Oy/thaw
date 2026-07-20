// SPDX-License-Identifier: GPL-3.0-or-later

// Package sqleditor provides SQL analysis utilities that are executed in the
// Go backend to protect proprietary logic from frontend reverse-engineering.
// These functions mirror the TypeScript counterparts previously in
// sqlDiagnostics.ts and SqlEditor.tsx.
//
// # SQL validation data flow (backend)
//
// There is no central validator. The frontend diagnostics provider
// (sqlDiagnostics.ts) is the orchestrator: on every edit it fans the document
// out to a set of INDEPENDENT backend checks over the Wails IPC bridge, then
// merges all the returned []DiagMarker into one Monaco marker set. Each method
// on the bound Service (service.go) is a thin delegator to a package-level
// Validate*/Get* function; the validators never call one another.
//
//	┌──────────────────────────────────────────────────────────────────────────┐
//	│ frontend  sqlDiagnostics.ts   (Monaco onDidChangeContent → fan-out/merge)  │
//	└──────────────────────────────────────────────────────────────────────────┘
//	             │  Wails IPC  (wailsjs/go/sqleditor/Service.*)
//	             ▼
//	   sqleditor.Service          (service.go — stateless thin delegators)
//	             │
//	  IPC method ├─ GetSqlStatementRanges ─► GetStatementRanges      (sqleditor.go)
//	             │                              └─► []StatementRange ─┐ split once,
//	             │                                                    │ then reused
//	             ├─ AnalyzeSqlSyntax ─────────► ValidateSyntax        (sqleditor.go)
//	             ├─ AnalyzeSqlSemantics ──────► ValidateSemantics     (sqleditor.go)
//	             ├─ ValidateDataTypes ────────► ValidateDataTypes     (patterns.go)
//	             ├─ ValidateGrammar ──────────► ValidateGrammar       (grammar.go)
//	             ├─ ValidateAntiPatterns ─────► ValidateAntiPatterns  (antipatterns.go)
//	             ├─ ValidateTablesExist ──────► ValidateTablesExist   (tableexist.go)
//	             └─ ValidateBareColumnRefs ───► ValidateBareColumnRefs (barecolrefs.go)
//	                          │
//	                          ▼  each returns []DiagMarker (Severity 8=Error, 4=Warning)
//	            ◄── merged by the frontend ──►  monaco.setModelMarkers
//
// Reference resolution (a prerequisite for the two reference-aware checks) is its
// own IPC round-trip: the frontend calls ParseJoinTableRefs → ResolveTableRefs and
// passes the resulting []ResolvedRef into AnalyzeSqlSemantics and ValidateTablesExist.
//
// Shared lower layers each validator builds on (none reach the frontend directly):
//
//	ValidateSyntax         → internal/sqltok        (tokenizer: walks the token stream)
//	ValidateSemantics      → tokmatch.go            (findFromJoinWithAlias, aliases, CTEs)
//	ValidateDataTypes      → internal/snowflake     (sf.AllDataTypes) + patterns.go
//	ValidateGrammar        → internal/sqlgrammar    (New → ParseTopLevel → Failure.Message)
//	ValidateAntiPatterns   → internal/sqlgrammar    (IdentifyStatement) + tokmatch.go
//	ValidateTablesExist    → tokmatch.go            (matchCreate*/Drop*/Alter*, findFromJoinTables2)
//	ValidateBareColumnRefs → tokmatch.go            (matchInsertColList, findReferences)
//
// tokmatch.go + diaghelpers.go (sigTokens, sqlStmt, stripCommentsSQL, …) are the
// common token-scanning toolkit layered on internal/sqltok; only ValidateGrammar
// and ValidateAntiPatterns reach into the internal/sqlgrammar grammar engine, and
// only ValidateDataTypes reaches into internal/snowflake. Every check re-derives
// its per-statement ranges from GetStatementRanges, so the checks stay independent
// and order-free.
package sqleditor

import (
	"regexp"
	"sort"
	"strings"

	sf "thaw/internal/snowflake"
	"thaw/internal/sqlgrammar"
	"thaw/internal/sqltok"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// DiagMarker represents a Monaco editor marker (error or warning).
// Monaco marker severity constants. These match the MarkerSeverity enum in
// monaco-editor (monaco.MarkerSeverity.Error = 8, Warning = 4).
const (
	SeverityError   = 8
	SeverityWarning = 4
)

type DiagMarker struct {
	StartLineNumber int    `json:"startLineNumber"`
	StartColumn     int    `json:"startColumn"`
	EndLineNumber   int    `json:"endLineNumber"`
	EndColumn       int    `json:"endColumn"`
	Message         string `json:"message"`
	Severity        int    `json:"severity"`       // SeverityError (red) or SeverityWarning (yellow)
	Code            string `json:"code,omitempty"` // JSON quick-fix metadata (e.g. qualify-table suggestions)
}

// JoinTableRef is a table reference parsed from a FROM/JOIN clause.
type JoinTableRef struct {
	DB     string `json:"db"`
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Alias  string `json:"alias"`
}

// ResolvedRef is a fully-qualified table reference with alias.
type ResolvedRef struct {
	Alias  string `json:"alias"`
	DB     string `json:"db"`
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

// ColInfo holds a column name and its Snowflake data type.
type ColInfo struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// ColEntry pairs a fully-qualified table with its column info list.
type ColEntry struct {
	DB     string    `json:"db"`
	Schema string    `json:"schema"`
	Name   string    `json:"name"`
	Cols   []ColInfo `json:"cols"`
}

// FKEntry represents one row from a foreign key constraint (child-side view).
type FKEntry struct {
	PKDatabase     string `json:"pkDatabase"`
	PKSchema       string `json:"pkSchema"`
	PKTable        string `json:"pkTable"`
	PKColumn       string `json:"pkColumn"`
	FKColumn       string `json:"fkColumn"`
	ConstraintName string `json:"constraintName"`
	KeySequence    int    `json:"keySequence"`
}

// TableFKEntry pairs a fully-qualified table with its FK entries.
type TableFKEntry struct {
	DB     string    `json:"db"`
	Schema string    `json:"schema"`
	Name   string    `json:"name"`
	FKs    []FKEntry `json:"fks"`
}

// JoinOnSuggestionsReq is the input for ComputeJoinOnConditions.
type JoinOnSuggestionsReq struct {
	ResolvedRefs []ResolvedRef  `json:"resolvedRefs"`
	FKEntries    []TableFKEntry `json:"fkEntries"`
	ColEntries   []ColEntry     `json:"colEntries"`
	// Prefix is prepended to each condition string (e.g. "ON " for trigger-C).
	Prefix string `json:"prefix"`
}

// JoinCondition is a single join condition suggestion with Monaco sort/detail metadata.
type JoinCondition struct {
	Condition string `json:"condition"`
	Detail    string `json:"detail"`
	SortText  string `json:"sortText"`
}

// CTEColumnEntry represents a CTE name and its projected columns for autocomplete.
type CTEColumnEntry struct {
	Name string    `json:"name"` // Uppercase CTE name
	Cols []ColInfo `json:"cols"`
}

// UseContext holds the accumulated DATABASE/SCHEMA context from USE statements
// that appear before the current cursor position in a multi-statement editor.
type UseContext struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

// StoreObject represents a known database object from the frontend object store.
type StoreObject struct {
	DB     string `json:"db"`
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

// SessionContext holds the live Snowflake session's database/schema for fallback resolution.
type SessionContext struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

// InEditorTableDef represents a table defined via CREATE TABLE in the editor text
// whose columns can be used for autocomplete before the statement is executed.
type InEditorTableDef struct {
	DB     string    `json:"db"`
	Schema string    `json:"schema"`
	Name   string    `json:"name"`
	Cols   []ColInfo `json:"cols"`
}

// AutocompleteContextRequest bundles all inputs for the extended autocomplete context.
type AutocompleteContextRequest struct {
	SQL          string          `json:"sql"`
	CursorOffset int             `json:"cursorOffset"`
	StoreObjects []StoreObject   `json:"storeObjects"`
	Session      *SessionContext `json:"session,omitempty"`
	LineUpToWord string          `json:"lineUpToWord"`
}

// UsingClauseInfo describes whether the cursor is inside a USING(...) clause.
type UsingClauseInfo struct {
	InUsing   bool `json:"inUsing"`   // Cursor is right after USING(
	IsPartial bool `json:"isPartial"` // Cursor is after USING(col, ...
}

// LineDiff holds 1-based line numbers for added, modified, and deleted lines.
type LineDiff struct {
	Added    []int `json:"added"`
	Modified []int `json:"modified"`
	Deleted  []int `json:"deleted"`
}

// AutocompleteContext bundles all server-side context needed by the frontend
// completion provider in a single IPC round-trip.
type AutocompleteContext struct {
	StatementRanges  []StatementRange          `json:"statementRanges"`
	CurrentStmt      string                    `json:"currentStmt"`
	CurrentStmtIdx   int                       `json:"currentStmtIdx"`
	Scripting        ScriptingCompletionResult `json:"scripting"`
	TableRefs        []JoinTableRef            `json:"tableRefs"`
	CTEColumns       []CTEColumnEntry          `json:"cteColumns"`
	UseContext       *UseContext               `json:"useContext,omitempty"`
	ResolvedRefs     []ResolvedRef             `json:"resolvedRefs,omitempty"`
	InEditorTables   []InEditorTableDef        `json:"inEditorTables,omitempty"`
	IsDatatypeCtx    bool                      `json:"isDatatypeContext"`
	IsInJoinOnClause bool                      `json:"isInJoinOnClause"`
	UsingClause      *UsingClauseInfo          `json:"usingClause,omitempty"`
	GrammarExpected  *GrammarExpectation       `json:"grammarExpected,omitempty"`
}

// GrammarExpectation is the recursive-descent grammar's "valid next" set at the
// cursor (sqlgrammar.Validator.ExpectedAt), split into the two buckets an
// autocomplete provider treats differently:
//
//   - Keywords — literal keyword/option words (FROM, TAG, DATA_RETENTION_TIME_IN_DAYS)
//     the provider can offer verbatim, ranked above the generic keyword list.
//   - Kinds — token-kind expectations (Identifier, StringLit, …) that the existing
//     completion sources already fill (object/column catalogs, stage lists,
//     scripting variables); surfaced so the provider can tell, e.g., "a name is
//     expected here" from "a keyword is expected here".
//
// It is nil when the statement's leading keyword is not modeled by sqlgrammar,
// keeping grammar-driven completion leading-keyword-gated.
type GrammarExpectation struct {
	Keywords []string `json:"keywords,omitempty"`
	Kinds    []string `json:"kinds,omitempty"`
}

// FunctionCallContext is returned by GetActiveFunctionCall and identifies the
// innermost open function call at the cursor position.
type FunctionCallContext struct {
	Name       string `json:"name"`       // function name (unquoted identifier)
	ParamIndex int    `json:"paramIndex"` // 0-indexed parameter the cursor is on
}

// SignatureParam is the [Start, End) byte span of one parameter within a
// function signature string, as returned by ParseSignatureParams.
type SignatureParam struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// SQL statement-starting keywords (outer, non-scripting context).
var sqlStmtKeywords = map[string]bool{
	// DML
	"SELECT": true, "WITH": true, "INSERT": true, "UPDATE": true,
	"DELETE": true, "MERGE": true,
	// DDL
	"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true,
	"UNDROP": true, "COMMENT": true,
	// DCL / session
	"GRANT": true, "REVOKE": true, "USE": true, "SET": true, "UNSET": true,
	// Info
	"SHOW": true, "DESCRIBE": true, "DESC": true, "EXPLAIN": true,
	// TCL
	"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "SAVEPOINT": true,
	// Execution
	"CALL": true, "EXECUTE": true, "RETURN": true,
	// Data loading (LS/RM are the documented abbreviations of LIST/REMOVE)
	"COPY": true, "PUT": true, "GET": true, "LIST": true, "REMOVE": true,
	"LS": true, "RM": true,
	// Snowflake scripting
	"DECLARE": true, "LET": true, "FOR": true, "WHILE": true, "IF": true,
	"CASE": true, "RAISE": true, "END": true, "LOOP": true,
	// Misc
	"ANALYZE": true,
}

// Snowflake scripting keywords that can start a statement inside $$.
var scriptStmtKeywords = map[string]bool{
	"BEGIN": true, "END": true, "DECLARE": true, "IF": true, "ELSE": true,
	"ELSEIF": true, "THEN": true, "CASE": true, "WHEN": true,
	"FOR": true, "WHILE": true, "LOOP": true, "REPEAT": true, "UNTIL": true,
	"DO": true, "RETURN": true, "RAISE": true,
	"EXCEPTION": true, "CALL": true, "LET": true, "VAR": true, "EXIT": true,
	"CONTINUE": true, "OPEN": true, "FETCH": true, "CLOSE": true,
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"MERGE": true, "CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true,
	"EXECUTE": true, "NULL": true,
	// TABLE is valid after RETURN (RETURN TABLE(resultset)) — not a variable.
	"TABLE": true,
}

// scriptBuiltinVars are the implicit variables Snowflake Scripting exposes
// inside a block (chiefly an EXCEPTION handler) without a DECLARE — the built-in
// exception variables. They must not be flagged as undeclared (issue #793 E1).
// https://docs.snowflake.com/en/developer-guide/snowflake-scripting/exceptions
var scriptBuiltinVars = map[string]bool{
	"SQLERRM": true, "SQLCODE": true, "SQLSTATE": true,
}

// JOIN clause stop keywords used to detect accidental alias capture.
var joinStopKW = map[string]bool{
	"ON": true, "WHERE": true, "SET": true, "GROUP": true, "ORDER": true,
	"HAVING": true, "LIMIT": true, "UNION": true, "EXCEPT": true,
	"INTERSECT": true, "CROSS": true, "INNER": true, "LEFT": true,
	"RIGHT": true, "FULL": true, "OUTER": true, "NATURAL": true, "JOIN": true,
	"SELECT": true, "WITH": true, "FROM": true,
	"AT": true, "BEFORE": true,
	"ASOF": true, "MATCH_CONDITION": true,
	// MERGE: `MERGE INTO t USING s …` — without USING/WHEN as stop words the
	// alias scan swallows USING (giving the target a bogus "USING" alias) and
	// skips past it, so the source table `s` is never extracted and later
	// `s.col` references are wrongly flagged as unknown columns.
	"USING": true, "WHEN": true,
}

func isWordChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// extractDollarTag extracts a $tag$ delimiter starting at position i in runes.
// Returns the full tag string (e.g. "$$" or "$body$") or "" if none matches.
func extractDollarTag(runes []rune, i int) string {
	if i >= len(runes) || runes[i] != '$' {
		return ""
	}
	j := i + 1
	for j < len(runes) && isWordChar(runes[j]) {
		j++
	}
	if j < len(runes) && runes[j] == '$' {
		return string(runes[i : j+1])
	}
	return ""
}

type parenEntry struct {
	char string
	line int
	col  int
}

// ── StatementRange / GetStatementRanges ───────────────────────────────────────

// StatementRange is the position of one SQL statement within a multi-statement string.
type StatementRange struct {
	StartLine   int `json:"startLine"`   // 1-indexed line of trimmed statement start
	EndLine     int `json:"endLine"`     // 1-indexed line of trailing ';' (or last char)
	StartOffset int `json:"startOffset"` // byte offset of trimmed statement start
	EndOffset   int `json:"endOffset"`   // byte offset just past ';' (or end of string)
}

// GetStatementRanges splits sql into per-statement ranges.  Semicolons inside
// string literals, quoted identifiers, block comments, line comments, and
// dollar-quoted blocks are correctly ignored.  No Snowflake connection is
// required.
//
// Delegates to [sqltok.SplitRanges] for the actual tokenization. Bare (non-$$)
// Snowflake Scripting anonymous blocks ([DECLARE …] BEGIN … END) that SplitRanges
// shreds on their inner `;` are re-glued into one range so the grammar validates
// them intact (issue #793 A1).
func GetStatementRanges(sql string) []StatementRange {
	tokRanges := sqltok.SplitRanges(sql)
	if len(tokRanges) == 0 {
		return nil
	}
	spans := bareScriptingBlockSpans(sql, tokRanges)
	var ranges []StatementRange
	for i := 0; i < len(tokRanges); i++ {
		r := tokRanges[i]
		// A range that begins a bare scripting block is emitted as one merged range
		// spanning the whole block; the fragments it absorbs are skipped.
		if span, ok := spanStartingAt(spans, r.StartOffset); ok {
			end := r
			for i+1 < len(tokRanges) && tokRanges[i+1].StartOffset < span.end {
				i++
				end = tokRanges[i]
			}
			ranges = append(ranges, StatementRange{
				StartLine:   r.StartLine,
				EndLine:     end.EndLine,
				StartOffset: r.StartOffset,
				EndOffset:   span.end,
			})
			continue
		}
		ranges = append(ranges, StatementRange{
			StartLine:   r.StartLine,
			EndLine:     r.EndLine,
			StartOffset: r.StartOffset,
			EndOffset:   r.EndOffset,
		})
	}
	return ranges
}

// ── GetIdentifierAtColumn ─────────────────────────────────────────────────────

// GetIdentifierAtColumn parses a single line of SQL and returns the
// dot-separated identifier parts (e.g. ["DB","SCHEMA","TABLE"]) when the
// zero-indexed cursor column col falls on or between any of those parts,
// including the dot separators.  Double-quoted identifiers (e.g. "My Table")
// are unquoted before being returned.  Returns nil when the column is not on
// any identifier.
func GetIdentifierAtColumn(line string, col int) []string {
	runes := []rune(line)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		if r != '"' && !isWordRune(r) {
			i++
			continue
		}

		// Gather one dot-separated identifier chain starting at i.
		parts := []string{}
		containsCol := false

		for i < n {
			partStart := i
			var partName []rune

			if runes[i] == '"' {
				i++ // skip opening quote
				for i < n {
					if runes[i] == '"' {
						if i+1 < n && runes[i+1] == '"' {
							partName = append(partName, '"')
							i += 2
							continue
						}
						i++ // skip closing quote
						break
					}
					partName = append(partName, runes[i])
					i++
				}
				parts = append(parts, string(partName))
			} else if isWordRune(runes[i]) {
				for i < n && isWordRune(runes[i]) {
					partName = append(partName, runes[i])
					i++
				}
				parts = append(parts, strings.ToUpper(string(partName)))
			} else {
				break
			}
			if col >= partStart && col < i {
				containsCol = true
			}

			// Continue chain if followed by '.'
			if i < n && runes[i] == '.' {
				if col == i {
					containsCol = true
				}
				i++ // skip '.'

				// If cursor is exactly after the dot, it also belongs to this chain.
				if col == i {
					containsCol = true
				}

				if i < n && (runes[i] == '"' || isWordRune(runes[i])) {
					continue
				}
				break
			}
			break
		}

		if containsCol && len(parts) > 0 {
			return parts
		}
	}
	return nil
}

// isWordRune reports whether r is a SQL word character (\w equivalent).
func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '$'
}

// isLetterOrUnderscore reports whether r can start an unquoted SQL identifier.
func isLetterOrUnderscore(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

// ── GetActiveFunctionCall ─────────────────────────────────────────────────────

// GetActiveFunctionCall parses prefix — the SQL text from the document start up
// to the cursor — and returns the innermost function call that is still open,
// together with which parameter (0-indexed) the cursor is on.
// Returns nil when the cursor is not inside any named function call.
//
// Improvements over the original TypeScript implementation:
//   - Handles ” escaped single quotes correctly (no false string-close)
//   - Skips double-quoted identifiers so they never pollute the paren stack
//   - Skips -- line comments and /* */ block comments so commas inside them
//     are not counted as parameter separators
func GetActiveFunctionCall(prefix string) *FunctionCallContext {
	type frame struct {
		name   string
		commas int
	}
	var stack []frame
	runes := []rune(prefix)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		// Line comment: --
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			i += 2
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment: /* */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i < n {
				if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// Single-quoted string '...' with '' escape
		if r == '\'' {
			i++
			for i < n {
				if runes[i] == '\'' && i+1 < n && runes[i+1] == '\'' {
					i += 2 // escaped quote
				} else if runes[i] == '\'' {
					i++
					break
				} else {
					i++
				}
			}
			continue
		}

		// Double-quoted identifier "..." — skip without affecting the paren stack
		if r == '"' {
			i++
			for i < n {
				if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
					i += 2
				} else if runes[i] == '"' {
					i++
					break
				} else {
					i++
				}
			}
			continue
		}

		if r == '(' {
			// Scan backwards past whitespace to find the function name.
			name := ""
			j := i - 1
			for j >= 0 && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
				j--
			}
			if j >= 0 && isWordRune(runes[j]) {
				end := j + 1
				for j >= 0 && isWordRune(runes[j]) {
					j--
				}
				start := j + 1
				if isLetterOrUnderscore(runes[start]) {
					name = string(runes[start:end])
				}
			}
			stack = append(stack, frame{name: name})
			i++
			continue
		}

		if r == ')' {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			i++
			continue
		}

		if r == ',' && len(stack) > 0 {
			stack[len(stack)-1].commas++
			i++
			continue
		}

		i++
	}

	if len(stack) == 0 {
		return nil
	}
	top := stack[len(stack)-1]
	if top.name == "" {
		return nil
	}
	return &FunctionCallContext{Name: top.name, ParamIndex: top.commas}
}

// ── ParseSignatureParams ──────────────────────────────────────────────────────

// ParseSignatureParams extracts the byte spans of each parameter in a function
// signature string such as "CONCAT(str1, str2)" or "DATEADD(datepart, val, dt)".
// The returned [Start, End) offsets index directly into the signature string,
// matching the format expected by Monaco's parameter-label highlighting.
// Returns nil when the signature has no '(' or no parameters.
func ParseSignatureParams(sig string) []SignatureParam {
	openIdx := strings.IndexByte(sig, '(')
	if openIdx < 0 {
		return nil
	}

	// Find the matching closing ')'.
	depth, closeIdx := 0, -1
	for i := openIdx; i < len(sig); i++ {
		if sig[i] == '(' {
			depth++
		} else if sig[i] == ')' {
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
	}
	if closeIdx < 0 || closeIdx == openIdx+1 {
		return nil
	}

	var params []SignatureParam
	start := openIdx + 1
	d := 0
	for i := openIdx + 1; i <= closeIdx; i++ {
		ch := sig[i]
		switch ch {
		case '(':
			d++
		case ')':
			d--
		}

		if (ch == ',' && d == 0) || i == closeIdx {
			rawEnd := i
			ps, pe := start, rawEnd
			for ps < pe && sig[ps] == ' ' {
				ps++
			}
			for pe > ps && sig[pe-1] == ' ' {
				pe--
			}
			if ps < pe {
				params = append(params, SignatureParam{Start: ps, End: pe})
			}
			start = i + 1
		}
	}
	return params
}

// ── ValidateSyntax ────────────────────────────────────────────────────────────

// ── Token-level casing ────────────────────────────────────────────────────────

// ApplyCasing walks a formatted SQL string token by token and applies
// per-role casing preferences.  Double-quoted identifiers, single-quoted
// strings, dollar-quoted strings, and comments are passed through unchanged.
// keywordCase: "UPPER" | "lower" | "Title" | "Preserve"
// identifierCase: "Preserve" | "UPPER" | "lower"
// functionCase: "UPPER" | "lower"
//
// Uses [sqltok.Tokenize] for tokenization.
func ApplyCasing(sql, keywordCase, identifierCase, functionCase string) string {
	if sql == "" {
		return sql
	}

	tokens := sqltok.Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))

	applyCase := func(word, casing string) string {
		switch casing {
		case "UPPER":
			return strings.ToUpper(word)
		case "lower":
			return strings.ToLower(word)
		case "Title":
			r := []rune(word)
			if len(r) == 0 {
				return word
			}
			return strings.ToUpper(string(r[:1])) + strings.ToLower(string(r[1:]))
		default: // "Preserve"
			return word
		}
	}

	// peekNextNonWS finds the next non-whitespace, non-newline token after index i.
	peekNextNonWS := func(i int) (sqltok.Token, bool) {
		for j := i + 1; j < len(tokens); j++ {
			k := tokens[j].Kind
			if k != sqltok.Whitespace && k != sqltok.Newline {
				return tokens[j], true
			}
		}
		return sqltok.Token{}, false
	}

	for i, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		text := tok.Text(sql)

		switch tok.Kind {
		case sqltok.QuotedIdent, sqltok.StringLit,
			sqltok.LineComment, sqltok.BlockComment:
			// Pass through unchanged.
			sb.WriteString(text)

		case sqltok.DollarQuoted:
			// Recursively apply casing to dollar-quoted bodies (except $query$).
			tag := tok.Tag
			tagLen := len(tag)
			inner := text[tagLen : len(text)-tagLen]
			sb.WriteString(tag)
			if strings.ToUpper(tag) == "$QUERY$" {
				sb.WriteString(inner)
			} else {
				sb.WriteString(ApplyCasing(inner, keywordCase, identifierCase, functionCase))
			}
			sb.WriteString(tag)

		case sqltok.Keyword, sqltok.Identifier:
			word := text
			upper := strings.ToUpper(word)

			// Peek past whitespace to check if followed by '(' (function call).
			nextTok, hasNext := peekNextNonWS(i)
			isCall := hasNext && nextTok.Kind == sqltok.LParen

			var result string
			if isCall {
				if sqltok.IsKeyword(upper) && !sqltok.IsBuiltinFunction(upper) {
					result = applyCase(word, keywordCase)
				} else {
					result = applyCase(word, functionCase)
				}
			} else if sqltok.IsKeyword(upper) {
				result = applyCase(word, keywordCase)
			} else {
				result = applyCase(word, identifierCase)
			}

			// For function tokens, strip whitespace before '(' that sql-formatter inserted.
			isFunctionToken := isCall && (!sqltok.IsKeyword(upper) || sqltok.IsBuiltinFunction(upper))
			sb.WriteString(result)
			if isFunctionToken {
				// Skip whitespace tokens between the word and '('.
				// Zero the text range so they emit nothing, but keep Kind
				// unchanged to avoid triggering the EOF break.
				for j := i + 1; j < len(tokens); j++ {
					k := tokens[j].Kind
					if k == sqltok.Whitespace || k == sqltok.Newline {
						tokens[j].End = tokens[j].Start
					} else {
						break
					}
				}
			}

		default:
			// Numbers, operators, whitespace, newlines, punctuation — pass through.
			sb.WriteString(text)
		}
	}

	return sb.String()
}

// ValidateSyntax catches structural Snowflake SQL errors by walking the
// [sqltok] token stream:
//   - Unclosed single-quoted strings, double-quoted identifiers, dollar-quoted
//     strings, and block comments
//   - Unmatched / extra closing parens and brackets, and unclosed opening ones
//   - Missing ':' in scripting assignments (var = expr → var := expr)
//   - Missing right-hand expression after an assignment
//   - Undeclared variables referenced in RETURN / FOR / assignment
//   - Unexpected token at a statement start
//
// Snowflake Scripting lives inside dollar-quoted bodies, which the tokenizer
// surfaces as a single opaque DollarQuoted token. validateSyntaxScope recurses
// into a body (re-tokenizing it, rebasing inner positions via baseLine/baseCol)
// only when it opens with BEGIN or DECLARE — an anonymous scripting block.
// Plain string constants (SELECT $$hi$$) and non-SQL UDF bodies
// (LANGUAGE PYTHON/JAVASCRIPT … AS $$…$$) are left opaque (#704).
func ValidateSyntax(sql string) []DiagMarker {
	var markers []DiagMarker
	add := func(msg string, sl, sc, el, ec int) {
		markers = append(markers, DiagMarker{
			StartLineNumber: sl, StartColumn: sc,
			EndLineNumber: el, EndColumn: ec,
			Message: msg, Severity: 8,
		})
	}
	validateSyntaxScope(sql, 1, 1, false, add, nil)
	markers = append(markers, validateScriptExprIdents(sql)...)
	markers = append(markers, validateBindVarColons(sql)...)
	return dedupMarkers(markers)
}

// dedupMarkers drops exact-duplicate markers (identical span, message, and
// severity). ValidateSyntax runs several overlapping passes — the scope walk and
// validateScriptExprIdents both flag the bare token of `RETURN missing;` — and an
// identical marker twice is redundant to the user. First-occurrence order is kept.
func dedupMarkers(markers []DiagMarker) []DiagMarker {
	if len(markers) < 2 {
		return markers
	}
	seen := make(map[DiagMarker]struct{}, len(markers))
	out := make([]DiagMarker, 0, len(markers))
	for _, m := range markers {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

// missingExprKeywords are statement keywords whose appearance immediately after
// an assignment operator means the right-hand expression is missing.
var missingExprKeywords = map[string]bool{
	"LET": true, "DECLARE": true, "BEGIN": true, "RETURN": true,
	"FOR": true, "WHILE": true, "LOOP": true, "IF": true,
}

// validateSyntaxScope validates one lexical scope (the whole input, or the body
// of a dollar-quoted block when inScript is true). baseLine/baseCol give the
// absolute position of this scope's first character so emitted markers use
// absolute coordinates.
// seedVars, when non-nil, pre-populates the scope's declaredVars — used to
// carry a CREATE PROCEDURE/FUNCTION argument list into its $$ scripting body,
// which would otherwise start with no declarations and flag every argument
// reference as undeclared (issue #705).
func validateSyntaxScope(src string, baseLine, baseCol int, inScript bool, add func(string, int, int, int, int), seedVars map[string]bool) {
	toks := sqltok.Tokenize(src)

	// Bare (non-$$) scripting blocks are only recognized at the outer (non-script)
	// level — inside a $$ body everything is already scripting. Detect their spans
	// once so the walk can validate each as a scripting scope instead of tripping
	// the outer statement-start gate on EXCEPTION/END fragments (issue #793 A1).
	var blockSpans []byteSpan
	if !inScript && hasBlockLeaderToken(toks, src) {
		blockSpans = bareScriptingBlockSpans(src, sqltok.SplitRanges(src))
	}

	// abs converts an intra-scope (line,col) to absolute coordinates. Only the
	// first line of the scope is horizontally offset by baseCol.
	abs := func(line, col int) (int, int) {
		if line == 1 {
			return baseLine, baseCol + col - 1
		}
		return baseLine + line - 1, col
	}
	absT := func(t sqltok.Token) (int, int) { return abs(t.Line, t.Col) }

	isBareWord := func(t sqltok.Token) bool {
		return t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier
	}
	// skipWS advances over whitespace only (stops at a newline, comment, or any
	// other token) — used by peeks that must stay on the current line.
	skipWS := func(k int) int {
		for k < len(toks) && toks[k].Kind == sqltok.Whitespace {
			k++
		}
		return k
	}
	// skipWSNL advances over whitespace and newlines, but stops at comments (a
	// comment is treated as "something is there", mirroring the original
	// character scanner which never skipped comments during look-ahead).
	skipWSNL := func(k int) int {
		for k < len(toks) && (toks[k].Kind == sqltok.Whitespace || toks[k].Kind == sqltok.Newline) {
			k++
		}
		return k
	}
	// skipParens skips a balanced (...) group starting at an LParen token and
	// returns the index just past the matching ')'.
	skipParens := func(k int) int {
		depth := 0
		for k < len(toks) {
			switch toks[k].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
				if depth == 0 {
					return k + 1
				}
			}
			k++
		}
		return k
	}

	// detectAssign reports the assignment operator at toks[k], if any: "colon"
	// for ':=' (Colon immediately followed by '='), "bare" for a lone '=' (an
	// error). opTok is the operator's first token (for positioning) and after is
	// the index just past the operator.
	detectAssign := func(k int) (kind string, opTok sqltok.Token, after int) {
		if k >= len(toks) {
			return "", sqltok.Token{}, k
		}
		t := toks[k]
		if t.Kind == sqltok.Colon && k+1 < len(toks) &&
			toks[k+1].Kind == sqltok.Operator && toks[k+1].Text(src) == "=" &&
			t.End == toks[k+1].Start {
			return "colon", t, k + 2
		}
		if t.Kind == sqltok.Operator && t.Text(src) == "=" {
			return "bare", t, k + 1
		}
		return "", sqltok.Token{}, k
	}

	// checkMissingExpr emits "Missing expression after assignment" when the
	// operator (first token opTok, length opLen) is followed only by ';', EOF,
	// or a statement keyword.
	checkMissingExpr := func(after int, opTok sqltok.Token, opLen int) {
		k := skipWSNL(after)
		missing := false
		if k >= len(toks) || toks[k].Kind == sqltok.EOF || toks[k].Kind == sqltok.Semicolon {
			missing = true
		} else if isBareWord(toks[k]) && missingExprKeywords[strings.ToUpper(toks[k].Text(src))] {
			missing = true
		}
		if missing {
			l, c := absT(opTok)
			add("Missing expression after assignment", l, c, l, c+opLen)
		}
	}

	var parenStack []parenEntry
	// flushOpenParens reports every still-open paren/bracket in push order and
	// clears the stack. Used both at each statement boundary (per-statement
	// balance) and at end of scope.
	flushOpenParens := func() {
		for _, open := range parenStack {
			add("Unclosed '"+open.char+"'", open.line, open.col, open.line, open.col+1)
		}
		parenStack = parenStack[:0]
	}
	declaredVars := map[string]bool{}
	for v := range seedVars {
		declaredVars[v] = true
	}
	// Built-in exception variables (SQLERRM/SQLCODE/SQLSTATE) are always in scope
	// inside a scripting block — seed them so they never read as undeclared (#793 E1).
	for v := range scriptBuiltinVars {
		declaredVars[v] = true
	}
	inDeclareBlock := false
	atStart := true

	i := 0
	for i < len(toks) {
		t := toks[i]
		switch t.Kind {
		case sqltok.EOF:
			i++

		case sqltok.Whitespace, sqltok.Newline, sqltok.LineComment:
			// Trivia; never resets statement start (a line comment can't be
			// unterminated).
			i++

		case sqltok.BlockComment:
			if t.Unterminated {
				l, c := absT(t)
				add("Unclosed block comment", l, c, l, c+2)
			}
			i++

		case sqltok.StringLit:
			if t.Unterminated {
				l, c := absT(t)
				add("Unclosed string literal", l, c, l, c+1)
			}
			atStart = false
			i++

		case sqltok.QuotedIdent:
			if t.Unterminated {
				l, c := absT(t)
				add("Unclosed quoted identifier", l, c, l, c+1)
			} else if inScript && atStart {
				// A quoted identifier at a script statement start followed by a
				// bare '=' is a mis-typed assignment.
				if k := skipWS(i + 1); k < len(toks) &&
					toks[k].Kind == sqltok.Operator && toks[k].Text(src) == "=" {
					l, c := absT(toks[k])
					add("Expected ':=' for assignment", l, c, l, c+1)
				}
			}
			atStart = false
			i++

		case sqltok.DollarQuoted:
			// A $$…$$ body is only Snowflake Scripting when it's an anonymous
			// block — i.e. it opens with BEGIN or DECLARE (EXECUTE IMMEDIATE $$…$$
			// and a LANGUAGE SQL procedure body both do). A plain string constant
			// (SELECT $$hi$$) or a non-SQL UDF body (LANGUAGE PYTHON/JAVASCRIPT …
			// AS $$…$$) is opaque — validating it as scripting produces phantom
			// errors (#704). The body starts len(tag) columns after the token.
			tag := t.Tag
			text := t.Text(src)
			var inner string
			if t.Unterminated {
				inner = text[len(tag):]
			} else {
				inner = text[len(tag) : len(text)-len(tag)]
			}
			if t.Unterminated {
				l, c := absT(t)
				add("Unclosed dollar-quoted string", l, c, l, c+len(tag))
			}
			if kw := getFirstSQLToken(inner); kw == "BEGIN" || kw == "DECLARE" {
				sl, sc := absT(t)
				validateSyntaxScope(inner, sl, sc+len(tag), true, add, procFuncArgNames(toks, src, i))
				atStart = true
			} else if fnArgs, ok := sqlScalarFuncArgs(toks, src, i); ok {
				// A scalar LANGUAGE SQL function body is a plain SQL expression, not
				// a scripting block: every bare identifier in it must resolve to a
				// function argument (or a builtin/keyword). Anything else is an
				// invalid identifier Snowflake rejects at creation time (#509).
				sl, sc := absT(t)
				validateSQLExprBodyIdents(inner, sl, sc+len(tag), fnArgs, add)
				atStart = false
			} else {
				// Opaque string constant — like StringLit, it doesn't reset start.
				atStart = false
			}
			i++

		case sqltok.LParen, sqltok.LBracket:
			ch := "("
			if t.Kind == sqltok.LBracket {
				ch = "["
			}
			l, c := absT(t)
			parenStack = append(parenStack, parenEntry{ch, l, c})
			atStart = false
			i++

		case sqltok.RParen, sqltok.RBracket:
			ch, expected := ")", "("
			if t.Kind == sqltok.RBracket {
				ch, expected = "]", "["
			}
			if len(parenStack) == 0 || parenStack[len(parenStack)-1].char != expected {
				l, c := absT(t)
				add("Unmatched '"+ch+"'", l, c, l, c+1)
			} else {
				parenStack = parenStack[:len(parenStack)-1]
			}
			atStart = false
			i++

		case sqltok.Semicolon:
			// Paren balance is per-statement: flush/report any parens left open
			// by this statement so a stray ')' in the next one can't silently pop
			// them (cross-statement leak).
			flushOpenParens()
			atStart = true
			i++

		case sqltok.Keyword, sqltok.Identifier:
			if !atStart {
				i++
				break
			}
			if !inScript {
				// A bare Snowflake Scripting anonymous block ([DECLARE …] BEGIN … END
				// without $$): validate the whole block as a scripting scope and skip
				// past it, mirroring the $$ anonymous-block handling above (#793 A1).
				if span, ok := spanStartingAt(blockSpans, t.Start); ok {
					sl, sc := absT(t)
					validateSyntaxScope(src[span.start:span.end], sl, sc, true, add, nil)
					for i < len(toks) && toks[i].Start < span.end {
						i++
					}
					atStart = true
					break
				}
				word := strings.ToUpper(t.Text(src))
				atStart = false
				if !sqlStmtKeywords[word] {
					l, c := absT(t)
					add("Unexpected token '"+t.Text(src)+"'", l, c, l, c+(t.End-t.Start))
				}
				i++
				break
			}
			i = validateScriptWord(toks, src, i, absT, isBareWord, skipWS, skipWSNL,
				skipParens, detectAssign, checkMissingExpr, add, declaredVars,
				&inDeclareBlock, &atStart)

		case sqltok.Operator:
			// '<' / '>' at a statement start is invalid in both outer SQL and
			// scripting (catches template/placeholder text like <foo>).
			if atStart {
				if txt := t.Text(src); len(txt) > 0 && (txt[0] == '<' || txt[0] == '>') {
					l, c := absT(t)
					add("Unexpected token '"+string(txt[0])+"'", l, c, l, c+1)
				}
			}
			atStart = false
			i++

		case sqltok.Other:
			if atStart {
				if txt := t.Text(src); txt == "{" || txt == "}" {
					l, c := absT(t)
					add("Unexpected token '"+txt+"'", l, c, l, c+1)
				}
			}
			atStart = false
			i++

		default:
			// NumberLit, Dot, Comma, Colon, At, etc. reset statement start.
			atStart = false
			i++
		}
	}

	// Report unclosed opening parens/brackets in push order.
	flushOpenParens()
}

// procFuncArgNames extracts the argument names of the CREATE PROCEDURE/FUNCTION
// signature whose $$ body is toks[dq], so those args seed the body's declared
// variables (issue #705). It walks back to the current statement start (the
// prior ';'), finds the PROCEDURE/FUNCTION keyword, then collects the first
// bare word of each comma-separated segment inside the following (...) arg
// list. Returns nil when the statement isn't a proc/func definition.
func procFuncArgNames(toks []sqltok.Token, src string, dq int) map[string]bool {
	start := 0
	for j := dq - 1; j >= 0; j-- {
		if toks[j].Kind == sqltok.Semicolon {
			start = j + 1
			break
		}
	}
	kw := -1
	for j := start; j < dq; j++ {
		if toks[j].Kind == sqltok.Keyword || toks[j].Kind == sqltok.Identifier {
			switch strings.ToUpper(toks[j].Text(src)) {
			case "PROCEDURE", "FUNCTION":
				kw = j
			}
		}
		if kw >= 0 {
			break
		}
	}
	if kw < 0 {
		return nil
	}
	lp := -1
	for j := kw + 1; j < dq; j++ {
		if toks[j].Kind == sqltok.LParen {
			lp = j
			break
		}
	}
	if lp < 0 {
		return nil
	}
	vars := map[string]bool{}
	depth, expectName := 0, true
	for j := lp; j < dq; j++ {
		switch toks[j].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return vars
			}
		case sqltok.Comma:
			if depth == 1 {
				expectName = true
			}
		case sqltok.Keyword, sqltok.Identifier:
			if depth == 1 && expectName {
				vars[strings.ToUpper(toks[j].Text(src))] = true
				expectName = false
			}
		}
	}
	return vars
}

// sqlScalarFuncArgs reports whether the $$ body at toks[dq] belongs to a scalar
// LANGUAGE SQL function definition and, if so, returns its argument names. Such
// a body is a plain SQL expression (not a Snowflake Scripting block), so its
// bare identifiers can be checked against the argument list (issue #509).
// Returns (nil, false) when the enclosing statement isn't a CREATE FUNCTION, is
// a PROCEDURE, or declares a non-SQL LANGUAGE (PYTHON/JAVASCRIPT/JAVA/SCALA)
// whose body must stay opaque (#704).
func sqlScalarFuncArgs(toks []sqltok.Token, src string, dq int) (map[string]bool, bool) {
	start := 0
	for j := dq - 1; j >= 0; j-- {
		if toks[j].Kind == sqltok.Semicolon {
			start = j + 1
			break
		}
	}
	isFunc := false
	for j := start; j < dq; j++ {
		if toks[j].Kind != sqltok.Keyword && toks[j].Kind != sqltok.Identifier {
			continue
		}
		switch strings.ToUpper(toks[j].Text(src)) {
		case "PROCEDURE":
			return nil, false
		case "FUNCTION":
			isFunc = true
		case "LANGUAGE":
			k := j + 1
			for k < dq && (toks[k].Kind == sqltok.Whitespace || toks[k].Kind == sqltok.Newline) {
				k++
			}
			if k < dq && (toks[k].Kind == sqltok.Keyword || toks[k].Kind == sqltok.Identifier) &&
				strings.ToUpper(toks[k].Text(src)) != "SQL" {
				return nil, false
			}
		}
	}
	if !isFunc {
		return nil, false
	}
	args := procFuncArgNames(toks, src, dq)
	if args == nil {
		return nil, false
	}
	return args, true
}

// sqlExprBareWords are words (beyond keywords and builtin functions) that
// legitimately appear as non-argument identifiers inside a scalar SQL
// expression: date-part names in DATEADD/DATEDIFF/EXTRACT/…, INTERVAL units,
// and sequence pseudo-columns. Listed here so validateSQLExprBodyIdents does
// not flag them as invalid identifiers.
var sqlExprBareWords = map[string]bool{
	"YEAR": true, "YEARS": true, "QUARTER": true, "QUARTERS": true,
	"MONTH": true, "MONTHS": true, "WEEK": true, "WEEKS": true, "WEEKISO": true,
	"DAY": true, "DAYS": true, "DAYOFWEEK": true, "DAYOFWEEKISO": true,
	"DAYOFYEAR": true, "DOW": true, "DOY": true,
	"HOUR": true, "HOURS": true, "MINUTE": true, "MINUTES": true,
	"SECOND": true, "SECONDS": true,
	"MILLISECOND": true, "MILLISECONDS": true, "MICROSECOND": true,
	"MICROSECONDS": true, "NANOSECOND": true, "NANOSECONDS": true,
	"EPOCH_SECOND": true, "EPOCH_MILLISECOND": true, "EPOCH_MICROSECOND": true,
	"EPOCH_NANOSECOND": true, "TIMEZONE_HOUR": true, "TIMEZONE_MINUTE": true,
	"YEAROFWEEK": true, "YEAROFWEEKISO": true,
	"INTERVAL": true, "NEXTVAL": true,
}

// validateSQLExprBodyIdents validates the bare identifiers of a scalar SQL
// function's expression body (src, positioned at baseLine/baseCol). Each
// identifier must be a function argument, a call (name followed by '('), a
// qualified-name part (adjacent to '.'), a builtin/keyword, or a date-part /
// interval word; anything else is flagged as not a function argument (#509).
// A body that queries tables (SELECT/FROM/…) is skipped — its column references
// can't be resolved without a catalog, mirroring ValidateBareColumnRefs's stance
// on SELECT clauses.
func validateSQLExprBodyIdents(src string, baseLine, baseCol int, args map[string]bool, add func(string, int, int, int, int)) {
	toks := sqltok.Tokenize(src)
	for _, t := range toks {
		if t.Kind == sqltok.Keyword {
			switch strings.ToUpper(t.Text(src)) {
			case "SELECT", "FROM", "WITH", "JOIN", "WHERE", "TABLE":
				return
			}
		}
	}
	abs := func(line, col int) (int, int) {
		if line == 1 {
			return baseLine, baseCol + col - 1
		}
		return baseLine + line - 1, col
	}
	// sig returns the nearest non-trivia token index scanning from k in the
	// given direction (+1 forward, -1 back), or -1 if none.
	sig := func(k, dir int) int {
		for k >= 0 && k < len(toks) {
			switch toks[k].Kind {
			case sqltok.Whitespace, sqltok.Newline, sqltok.LineComment, sqltok.BlockComment:
				k += dir
			default:
				return k
			}
		}
		return -1
	}
	for i := range toks {
		t := toks[i]
		if t.Kind != sqltok.Identifier {
			continue
		}
		upper := strings.ToUpper(t.Text(src))
		if args[upper] || sqlExprBareWords[upper] ||
			sqltok.IsBuiltinFunction(upper) || sqltok.IsReserved(upper) || sqltok.IsKeyword(upper) {
			continue
		}
		// A name immediately followed by '(' is a function call (a UDF or a
		// builtin outside our lists); a name adjacent to '.' is a qualified-name
		// part (db.schema.obj, seq.NEXTVAL, variant.path). Neither is a bare
		// value reference, so neither can be validated against the arg list.
		if n := sig(i+1, 1); n >= 0 && (toks[n].Kind == sqltok.LParen || toks[n].Kind == sqltok.Dot) {
			continue
		}
		if p := sig(i-1, -1); p >= 0 && toks[p].Kind == sqltok.Dot {
			continue
		}
		l, c := abs(t.Line, t.Col)
		add("Identifier '"+t.Text(src)+"' is not a function argument", l, c, l, c+(t.End-t.Start))
	}
}

// validateScriptWord handles a keyword/identifier token at a Snowflake Scripting
// statement start (toks[i]) and returns the index to resume scanning from.
// declaredVars, inDeclareBlock, and atStart are updated in place.
func validateScriptWord(
	toks []sqltok.Token, src string, i int,
	absT func(sqltok.Token) (int, int),
	isBareWord func(sqltok.Token) bool,
	skipWS, skipWSNL, skipParens func(int) int,
	detectAssign func(int) (string, sqltok.Token, int),
	checkMissingExpr func(int, sqltok.Token, int),
	add func(string, int, int, int, int),
	declaredVars map[string]bool, inDeclareBlock, atStart *bool,
) int {
	t := toks[i]
	word := strings.ToUpper(t.Text(src))
	*atStart = false // default; specific keywords below re-arm a statement start

	switch word {
	case "DECLARE":
		*inDeclareBlock = true
		*atStart = true
		return i + 1
	case "BEGIN":
		*inDeclareBlock = false
		*atStart = true
		return i + 1
	case "THEN", "ELSE", "DO", "EXCEPTION":
		*atStart = true
		return i + 1

	case "RETURN", "FOR":
		k := skipWSNL(i + 1)
		if k >= len(toks) || !isBareWord(toks[k]) {
			return i + 1
		}
		varTok := toks[k]
		if word == "FOR" {
			declaredVars[strings.ToUpper(varTok.Text(src))] = true // loop variable
			// FOR <var> IN <cursor> DO
			k = skipWSNL(k + 1)
			if k < len(toks) && isBareWord(toks[k]) && strings.ToUpper(toks[k].Text(src)) == "IN" {
				k = skipWSNL(k + 1)
				if k < len(toks) && isBareWord(toks[k]) {
					curTok := toks[k]
					curName := strings.ToUpper(curTok.Text(src))
					// FOR i IN REVERSE <lo> TO <hi> — REVERSE is a range modifier,
					// not a cursor; skip it (issue #705).
					if curName == "REVERSE" {
						return k + 1
					}
					if !scriptStmtKeywords[curName] && !declaredVars[curName] {
						l, c := absT(curTok)
						add("Variable '"+curTok.Text(src)+"' is not declared", l, c, l, c+(curTok.End-curTok.Start))
					}
					return k + 1
				}
			}
			return k
		}
		// RETURN <var> — flag an undeclared, non-keyword identifier. The scope
		// checker only validates variables, so the sole exemption is an
		// identifier immediately followed by '(': that's a function/procedure
		// call (builtin or not), not a variable reference. A bare identifier
		// matching a builtin name (e.g. `RETURN count;`) is still a variable
		// reference and must be flagged if undeclared (issue #705).
		varName := strings.ToUpper(varTok.Text(src))
		isCall := skipWS(k+1) < len(toks) && toks[skipWS(k+1)].Kind == sqltok.LParen
		if !isCall && !scriptStmtKeywords[varName] && !declaredVars[varName] {
			l, c := absT(varTok)
			add("Variable '"+varTok.Text(src)+"' is not declared", l, c, l, c+(varTok.End-varTok.Start))
		}
		return k + 1

	case "LET", "VAR":
		k := skipWSNL(i + 1)
		if k >= len(toks) || !isBareWord(toks[k]) {
			return i + 1
		}
		declaredVars[strings.ToUpper(toks[k].Text(src))] = true // the declared variable
		k = skipWSNL(k + 1)
		// Optional type annotation (FLOAT, VARCHAR(100), NUMBER(10,2), …).
		if k < len(toks) && isBareWord(toks[k]) {
			k = skipWS(k + 1)
			if k < len(toks) && toks[k].Kind == sqltok.LParen {
				k = skipParens(k)
			}
			k = skipWSNL(k)
		}
		if kind, opTok, after := detectAssign(k); kind != "" {
			opLen := 1
			if kind == "colon" {
				opLen = 2
			} else { // bare '='
				l, c := absT(opTok)
				add("Expected ':=' for assignment", l, c, l, c+1)
			}
			checkMissingExpr(after, opTok, opLen)
		}
		return k

	default:
		if *inDeclareBlock {
			// Inside DECLARE, every non-keyword identifier declares a variable.
			if !scriptStmtKeywords[word] {
				declaredVars[word] = true
			}
			*atStart = true
			return i + 1
		}
		if !scriptStmtKeywords[word] {
			// A non-keyword word at a statement start is an assignment target,
			// otherwise an unexpected token. The assignment operator must be on
			// the same line (skipWS, not skipWSNL).
			if kind, opTok, after := detectAssign(skipWS(i + 1)); kind != "" {
				opLen := 1
				if kind == "colon" {
					opLen = 2
				} else {
					l, c := absT(opTok)
					add("Expected ':=' for assignment", l, c, l, c+1)
				}
				if !declaredVars[word] {
					l, c := absT(t)
					add("Variable '"+t.Text(src)+"' is not declared", l, c, l, c+(t.End-t.Start))
				}
				checkMissingExpr(after, opTok, opLen)
			} else {
				l, c := absT(t)
				add("Unexpected token '"+t.Text(src)+"'", l, c, l, c+(t.End-t.Start))
			}
		}
		return i + 1
	}
}

// ── ParseJoinTables ───────────────────────────────────────────────────────────

// normID normalises a raw captured identifier:
//   - Quoted identifiers ("name") have quotes stripped, escaped quotes unescaped, and case preserved.
//   - Unquoted identifiers are uppercased (Snowflake convention).
func normID(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, `""`, `"`)
	}
	return strings.ToUpper(s)
}

// ParseJoinTables extracts all FROM/JOIN table references (with optional aliases)
// AND database/schema references from USE statements from the given SQL text.
// Three-part (db.schema.table), two-part (schema.table), and one-part (table)
// references are all recognized.
//
// It is a keyword-anchored scan over the significant-token stream (mirroring
// internal/snowflake/lineage.go extractObjectRefs): FROM/JOIN/USING/MERGE INTO
// introduce a table path, USE introduces a database/schema. Scanning tokens
// rather than the raw string means comments and string literals never produce
// phantom refs, comments between a keyword and its identifier are tolerated,
// and dotted paths + quoted identifiers are read by the tokenizer's own
// sqltok.ReadIdentParts.
func ParseJoinTables(sql string) []JoinTableRef {
	toks := sqltok.SignificantTokens(sql)
	var result []JoinTableRef

	for i := 0; i < len(toks); {
		if toks[i].Kind != sqltok.Keyword {
			i++
			continue
		}
		kw := strings.ToUpper(toks[i].Text(sql))

		// tableAt is the index where the table path begins, or -1 if this
		// keyword does not introduce one.
		tableAt := -1
		switch kw {
		case "FROM", "JOIN", "USING":
			tableAt = i + 1
		case "MERGE":
			if j := i + 1; j < len(toks) && strings.EqualFold(toks[j].Text(sql), "INTO") {
				tableAt = j + 1 // MERGE INTO <target>
			}
		case "USE":
			i = parseUseRef(toks, sql, i+1, &result)
			continue
		}
		if tableAt < 0 {
			i++
			continue
		}

		parts, next := sqltok.ReadIdentParts(toks, sql, tableAt, 3)
		if parts == nil {
			i++
			continue
		}
		ref := partsToRef(parts)
		ref.Alias = ref.Name

		// Optional alias, with an optional leading AS. A stop keyword in alias
		// position is not an alias: leave it for the main loop to re-process
		// (so `FROM a JOIN b` and `MERGE INTO t USING s` chain correctly).
		aliasAt := next
		if aliasAt < len(toks) && toks[aliasAt].Kind == sqltok.Keyword &&
			strings.EqualFold(toks[aliasAt].Text(sql), "AS") {
			aliasAt++
		}
		resume := next
		if aliasAt < len(toks) && toks[aliasAt].Kind.IsIdentLike() &&
			!joinStopKW[strings.ToUpper(toks[aliasAt].Text(sql))] {
			ref.Alias = normID(toks[aliasAt].Text(sql))
			resume = aliasAt + 1
		}

		result = append(result, ref)
		i = resume
	}

	return result
}

// partsToRef maps a 1-, 2-, or 3-part identifier path (as returned by
// sqltok.ReadIdentParts) onto a JoinTableRef's DB/Schema/Name, normalising each
// part. Alias is left unset for the caller to fill.
func partsToRef(parts []string) JoinTableRef {
	switch len(parts) {
	case 3:
		return JoinTableRef{DB: normID(parts[0]), Schema: normID(parts[1]), Name: normID(parts[2])}
	case 2:
		return JoinTableRef{Schema: normID(parts[0]), Name: normID(parts[1])}
	default:
		return JoinTableRef{Name: normID(parts[len(parts)-1])}
	}
}

// parseUseRef parses a USE statement whose body starts at toks[at] (just past
// the USE keyword), appending a DB/schema ref to result when applicable. It
// returns the token index at which the main scan should resume. USE ROLE and
// USE WAREHOUSE are recognized but produce no ref.
func parseUseRef(toks []sqltok.Token, src string, at int, result *[]JoinTableRef) int {
	if at >= len(toks) {
		return at
	}
	keyword := strings.ToUpper(toks[at].Text(src))
	nameAt := at
	switch keyword {
	case "DATABASE", "SCHEMA":
		nameAt = at + 1
	case "ROLE", "WAREHOUSE":
		return at + 1
	default:
		keyword = ""
	}

	parts, next := sqltok.ReadIdentParts(toks, src, nameAt, 2)
	if parts == nil {
		return nameAt
	}

	var db, schema string
	if len(parts) == 2 {
		db, schema = normID(parts[0]), normID(parts[1])
	} else if keyword == "SCHEMA" {
		schema = normID(parts[0])
	} else {
		db = normID(parts[0]) // USE DATABASE <db> or bare USE <db>
	}

	if db != "" || schema != "" {
		*result = append(*result, JoinTableRef{DB: db, Schema: schema})
	}
	return next
}

// ── BuildCompositeConditions ──────────────────────────────────────────────────

// BuildCompositeConditions groups FKs by constraint name, sorts each group by
// key sequence, and returns one condition string per constraint.
// Multi-column constraints produce AND-joined pairs (e.g. "a.x = b.x AND a.y = b.y").
func BuildCompositeConditions(fks []FKEntry, fkAlias, pkAlias string) []string {
	// Preserve insertion order of constraint groups (mirrors JS Map iteration order).
	type group struct {
		rows []FKEntry
	}
	var groupOrder []string
	groupMap := map[string]*group{}

	for _, fk := range fks {
		key := fk.ConstraintName
		if key == "" {
			key = fk.FKColumn
		}
		if _, ok := groupMap[key]; !ok {
			groupOrder = append(groupOrder, key)
			groupMap[key] = &group{}
		}
		groupMap[key].rows = append(groupMap[key].rows, fk)
	}

	result := make([]string, 0, len(groupOrder))
	for _, k := range groupOrder {
		cols := groupMap[k].rows
		sort.Slice(cols, func(a, b int) bool {
			return cols[a].KeySequence < cols[b].KeySequence
		})
		parts := make([]string, len(cols))
		for idx, fk := range cols {
			parts[idx] = sf.QuoteOrBare(fkAlias, fkAlias != strings.ToUpper(fkAlias)) + "." + sf.QuoteOrBare(fk.FKColumn, false) +
				" = " + sf.QuoteOrBare(pkAlias, pkAlias != strings.ToUpper(pkAlias)) + "." + sf.QuoteOrBare(fk.PKColumn, false)
		}
		result = append(result, strings.Join(parts, " AND "))
	}
	return result
}

// ── PkHeuristicConditions ─────────────────────────────────────────────────────

// PkHeuristicConditions suggests join conditions using the TABLE_B.TABLE_A_ID ↔ TABLE_A.ID
// naming convention when no explicit FK constraints are present.
func PkHeuristicConditions(
	lastName, lastAlias, otherName, otherAlias string,
	lastCols, otherCols []string,
) []string {
	var results []string
	ln := strings.ToUpper(lastName)
	on := strings.ToUpper(otherName)

	for _, col := range lastCols {
		uc := strings.ToUpper(col)
		if uc == on+"_ID" || uc == on+"ID" {
			for _, pkCol := range otherCols {
				if strings.ToUpper(pkCol) == "ID" {
					results = append(results, sf.QuoteOrBare(lastAlias, lastAlias != strings.ToUpper(lastAlias))+"."+sf.QuoteOrBare(col, false)+" = "+sf.QuoteOrBare(otherAlias, otherAlias != strings.ToUpper(otherAlias))+"."+sf.QuoteOrBare(pkCol, false))
					break
				}
			}
		}
	}
	for _, col := range otherCols {
		uc := strings.ToUpper(col)
		if uc == ln+"_ID" || uc == ln+"ID" {
			for _, pkCol := range lastCols {
				if strings.ToUpper(pkCol) == "ID" {
					results = append(results, sf.QuoteOrBare(otherAlias, otherAlias != strings.ToUpper(otherAlias))+"."+sf.QuoteOrBare(col, false)+" = "+sf.QuoteOrBare(lastAlias, lastAlias != strings.ToUpper(lastAlias))+"."+sf.QuoteOrBare(pkCol, false))
					break
				}
			}
		}
	}
	return results
}

// ── GetScriptingCompletions ───────────────────────────────────────────────────

// ScriptingCompletionResult is the combined result of variable-extraction and
// colon-detection for Snowflake Scripting autocompletion at a cursor position.
type ScriptingCompletionResult struct {
	Variables  []string `json:"variables"`  // Uppercased declared variable names in scope
	NeedsColon bool     `json:"needsColon"` // Whether variables need a ':' prefix in SQL context
}

// GetScriptingCompletions extracts declared Snowflake Scripting variables visible
// at cursorOffset and determines whether a ':' prefix is required. Both results
// are returned together to avoid two IPC round-trips. No Snowflake connection
// is required. cursorOffset is a Unicode codepoint count (matches Monaco's
// model.getOffsetAt for ASCII SQL).
func GetScriptingCompletions(sql string, cursorOffset int) ScriptingCompletionResult {
	// Resolve the analysis scope once — both helpers need it, and it tokenizes
	// the whole document, so we avoid doing that (and the O(n) offset scan) twice.
	text, inBlock := scriptingContext(sql, runeOffsetToByte(sql, cursorOffset))
	return ScriptingCompletionResult{
		Variables:  scriptingExtractVars(text, inBlock),
		NeedsColon: scriptingNeedsColon(text),
	}
}

// runeOffsetToByte converts a Unicode-codepoint offset (as produced by Monaco's
// model.getOffsetAt) into a byte offset into s. Values <= 0 map to 0; values past
// the end map to len(s).
func runeOffsetToByte(s string, runeOff int) int {
	if runeOff <= 0 {
		return 0
	}
	n := 0
	for i := range s {
		if n == runeOff {
			return i
		}
		n++
	}
	return len(s)
}

// scriptingContext returns the text to analyze for scripting autocomplete at the
// given byte cursor. When the cursor sits inside a dollar-quoted block it returns
// that block's body (from just after the opening delimiter up to the cursor) with
// inBlock=true; otherwise it returns sql up to the cursor with inBlock=false.
// Detecting the block via sqltok's DollarQuoted token means "$$" markers inside
// string literals or comments no longer flip block detection.
func scriptingContext(sql string, cursor int) (text string, inBlock bool) {
	if cursor > len(sql) {
		cursor = len(sql)
	}
	for _, tok := range sqltok.Tokenize(sql) {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind != sqltok.DollarQuoted {
			continue
		}
		openLen := len(tok.Tag) // Tag is the full "$$" / "$tag$" opening delimiter
		bodyStart := tok.Start + openLen
		bodyEnd := tok.End
		if !tok.Unterminated {
			bodyEnd -= openLen // closing delimiter matches the opening one
		}
		if cursor >= bodyStart && cursor <= bodyEnd {
			return sql[bodyStart:cursor], true
		}
	}
	return sql[:cursor], false
}

// scriptingExtractVars mirrors extractDeclaredVariables from snowflakeScriptingUtils.ts.
// body/inBlock come from scriptingContext: body is the $$ block content up to the
// cursor, inBlock reports whether the cursor is inside a block at all. Variables are
// scanned over sqltok significant tokens so keywords or ';' inside string literals
// and comments are never mistaken for declarations.
func scriptingExtractVars(body string, inBlock bool) []string {
	if !inBlock {
		return nil // cursor is in plain SQL, not inside a $$ block
	}
	sig := sqltok.SignificantTokens(body)

	seen := make(map[string]struct{})
	var vars []string
	addVar := func(name string) {
		up := strings.ToUpper(name)
		if _, ok := seen[up]; !ok {
			seen[up] = struct{}{}
			vars = append(vars, up)
		}
	}
	upper := func(t sqltok.Token) string { return strings.ToUpper(t.Text(body)) }

	// Phases run in the legacy order (DECLARE, then LET/VAR, then FOR) so dedup
	// preserves the same first-seen ordering the regex passes produced.

	// 1. DECLARE … BEGIN sections: first identifier of each ';'-separated entry.
	skipWords := map[string]bool{"CURSOR": true, "EXCEPTION": true, "TYPE": true, "LET": true, "VAR": true}
	inDeclare, expectName := false, false
	for _, tok := range sig {
		switch up := upper(tok); {
		case up == "DECLARE":
			inDeclare, expectName = true, true
		case up == "BEGIN" || up == "END":
			inDeclare, expectName = false, false
		case tok.Kind == sqltok.Semicolon:
			expectName = inDeclare
		case inDeclare && expectName && tok.Kind.IsIdentLike():
			expectName = false
			if !skipWords[up] {
				addVar(tok.Text(body))
			}
		}
	}

	// 2. LET / VAR declarations: the identifier following the keyword.
	for i, tok := range sig {
		if up := upper(tok); up == "LET" || up == "VAR" {
			if i+1 < len(sig) && sig[i+1].Kind.IsIdentLike() {
				addVar(sig[i+1].Text(body))
			}
		}
	}

	// 3. FOR loop variables: FOR <ident> IN.
	for i, tok := range sig {
		if upper(tok) == "FOR" && i+2 < len(sig) &&
			sig[i+1].Kind.IsIdentLike() && strings.ToUpper(sig[i+2].Text(body)) == "IN" {
			addVar(sig[i+1].Text(body))
		}
	}

	return vars
}

// colonRequiredKeywords is the set of SQL DQL/DML/DDL keywords that require a
// ':' prefix on scripting variable references (mirrors isColonRequired in TS).
var colonRequiredKeywords = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true,
	"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true, "COPY": true,
	"CALL": true, "WITH": true, "SHOW": true, "DESCRIBE": true,
	"GRANT": true, "REVOKE": true,
}

// scriptingContextKeywords is the full set of keywords that "set the context" for
// colon detection: the colon-requiring SQL keywords plus the scripting/control-flow
// keywords that do NOT require a colon. The most recent one before the cursor wins.
var scriptingContextKeywords = func() map[string]bool {
	m := map[string]bool{
		"LET": true, "RETURN": true, "IF": true, "WHILE": true,
		"UNTIL": true, "DO": true, "LOOP": true, "BEGIN": true,
	}
	for k := range colonRequiredKeywords {
		m[k] = true
	}
	return m
}()

// scriptingNeedsColon mirrors isColonRequired from snowflakeScriptingUtils.ts,
// scanning sqltok significant tokens (comment/string-aware) instead of stripping
// the text with per-call regexes. text is the scriptingContext-scoped scan region
// (the $$ block body when inside one), so context keywords before an enclosing $$
// never leak into the decision.
func scriptingNeedsColon(text string) bool {
	sig := sqltok.SignificantTokens(text)

	// Ignore the word currently being typed: an ident-like token ending at the cursor.
	if k := len(sig) - 1; k >= 0 && sig[k].Kind.IsIdentLike() && sig[k].End == len(text) {
		sig = sig[:k]
	}
	// A ':' immediately before the word means the reference is already prefixed.
	if k := len(sig) - 1; k >= 0 && sig[k].Kind == sqltok.Colon {
		return false
	}
	// Walk back to the most recent context-setting token.
	for i := len(sig) - 1; i >= 0; i-- {
		tok := sig[i]
		// ':=' assignment (a Colon immediately followed by '=') needs no colon.
		if tok.Kind == sqltok.Operator && tok.Text(text) == "=" &&
			i > 0 && sig[i-1].Kind == sqltok.Colon && sig[i-1].End == tok.Start {
			return false
		}
		if tok.Kind == sqltok.Semicolon {
			return false // a ';' closed the previous statement
		}
		if tok.Kind.IsIdentLike() {
			if up := strings.ToUpper(tok.Text(text)); scriptingContextKeywords[up] {
				return colonRequiredKeywords[up]
			}
		}
	}
	return false
}

// validateBindVarColons flags a Snowflake Scripting variable (or a stored
// procedure/function argument) that is referenced inside an embedded SQL
// statement without the required ':' bind prefix — e.g. `WHERE id = amount`
// where `amount` is a declared variable and should read `:amount`. Emitted as a
// Warning (not an Error): without the catalog we can't know whether a table
// actually has a column of the same name, and when it does the bare form is the
// legitimate column reference — so this is a "did you mean :amount?" hint, not a
// hard error. Only scripting blocks (`$$…$$` opening with BEGIN/DECLARE) are
// scanned; plain SQL and non-SQL UDF bodies have no scripting variables.
func validateBindVarColons(sql string) []DiagMarker {
	var out []DiagMarker
	outer := sqltok.Tokenize(sql)
	for idx, tok := range outer {
		if tok.Kind != sqltok.DollarQuoted || tok.Unterminated {
			continue
		}
		text := tok.Text(sql)
		tag := tok.Tag
		if len(text) < 2*len(tag) {
			continue
		}
		inner := text[len(tag) : len(text)-len(tag)]
		if kw := getFirstSQLToken(inner); kw != "BEGIN" && kw != "DECLARE" {
			continue // not a scripting block
		}
		// Names that need a ':' inside embedded SQL: declared scripting variables
		// plus the enclosing procedure/function's argument names.
		vars := map[string]bool{}
		for _, v := range scriptingExtractVars(inner, true) {
			vars[v] = true
		}
		for v := range procFuncArgNames(outer, sql, idx) {
			vars[v] = true
		}
		if len(vars) == 0 {
			continue
		}
		out = append(out, scanBindVarColons(inner, tok.Line, tok.Col+len(tag), vars)...)
	}
	return out
}

// scanBindVarColons walks a scripting body's significant tokens and returns a
// Warning marker for each bare reference to a name in vars that sits in a
// colon-requiring SQL context and in an operand position (right of an operator,
// after '(', or after THEN/ELSE). Restricting to operand positions keeps the
// classic `= myvar` bug in scope while avoiding the column positions (projection
// heads, predicate left-hand sides) where a same-named column is most likely
// intended. baseLine/baseCol locate the body's first character for rebasing.
func scanBindVarColons(inner string, baseLine, baseCol int, vars map[string]bool) []DiagMarker {
	var out []DiagMarker
	sig := sqltok.SignificantTokens(inner)
	for i, t := range sig {
		if !t.Kind.IsIdentLike() || !vars[strings.ToUpper(t.Text(inner))] {
			continue
		}
		// Already ':'-prefixed, a call `name(`, or a qualified-name part
		// (`t.name` / `name.field`) — none is a bare bind reference.
		if i > 0 && sig[i-1].Kind == sqltok.Colon && sig[i-1].End == t.Start {
			continue
		}
		if i > 0 && sig[i-1].Kind == sqltok.Dot && sig[i-1].End == t.Start {
			continue
		}
		if i+1 < len(sig) && (sig[i+1].Kind == sqltok.LParen || sig[i+1].Kind == sqltok.Dot) &&
			sig[i+1].Start == t.End {
			continue
		}
		// Operand position: preceded by an operator, '(', or THEN/ELSE.
		if i == 0 {
			continue
		}
		prev := sig[i-1]
		operand := prev.Kind == sqltok.Operator || prev.Kind == sqltok.LParen ||
			(prev.Kind.IsIdentLike() && bindOperandKeywords[strings.ToUpper(prev.Text(inner))])
		if !operand {
			continue
		}
		// Confirm the enclosing statement is a colon-requiring SQL context.
		if !scriptingNeedsColon(inner[:t.End]) {
			continue
		}
		l, c := baseLine+t.Line-1, t.Col
		if t.Line == 1 {
			c = baseCol + t.Col - 1
		}
		name := t.Text(inner)
		out = append(out, DiagMarker{
			StartLineNumber: l, StartColumn: c,
			EndLineNumber: l, EndColumn: c + (t.End - t.Start),
			Message:  "Scripting variable '" + name + "' must be referenced with a leading colon (':" + name + "') inside a SQL statement",
			Severity: SeverityWarning,
		})
	}
	return out
}

// bindOperandKeywords are keywords after which a following identifier is a value
// operand (so a variable there needs a ':'), used by scanBindVarColons.
var bindOperandKeywords = map[string]bool{"THEN": true, "ELSE": true}

// validateScriptExprIdents flags an identifier referenced in a Snowflake
// Scripting control-flow expression — an IF/ELSEIF/WHILE/REPEAT-UNTIL condition,
// an assignment right-hand side, or a RETURN expression — that resolves to
// neither a declared variable, a procedure/function argument, a call, a
// qualified-name part, nor a builtin/keyword. It is the scripting analog of
// the scalar-function body check (#509): the scope walk only inspects the first
// token after RETURN and bare assignment targets, so an undeclared name buried
// in an expression (e.g. `IF (missing <= 0) THEN`) went unreported. Only
// scripting blocks (`$$…$$` opening with BEGIN/DECLARE) are scanned. Emitted as
// Errors, matching the scope walk's other "Variable '…' is not declared" markers.
func validateScriptExprIdents(sql string) []DiagMarker {
	var out []DiagMarker
	outer := sqltok.Tokenize(sql)
	for idx, tok := range outer {
		if tok.Kind != sqltok.DollarQuoted || tok.Unterminated {
			continue
		}
		text := tok.Text(sql)
		tag := tok.Tag
		if len(text) < 2*len(tag) {
			continue
		}
		inner := text[len(tag) : len(text)-len(tag)]
		if kw := getFirstSQLToken(inner); kw != "BEGIN" && kw != "DECLARE" {
			continue
		}
		vars := map[string]bool{}
		for _, v := range scriptingExtractVars(inner, true) {
			vars[v] = true
		}
		for v := range procFuncArgNames(outer, sql, idx) {
			vars[v] = true
		}
		out = append(out, scanScriptExprIdents(inner, tok.Line, tok.Col+len(tag), vars)...)
	}
	return out
}

// scriptStmtStarters are the keywords after which a new Snowflake Scripting
// statement begins, so the following significant token sits at a statement start.
// Used to gate control-flow keyword matching: a bare `IF`/`WHILE`/`UNTIL`/`RETURN`
// is only a control-flow keyword when it opens a statement, never mid-statement.
// This is what keeps `CREATE TABLE IF NOT EXISTS` / `DROP TABLE IF EXISTS` guards
// from being misread as scripting `IF … THEN` conditions.
var scriptStmtStarters = map[string]bool{
	"BEGIN": true, "THEN": true, "ELSE": true, "LOOP": true, "DO": true, "REPEAT": true,
}

// scanScriptExprIdents walks a scripting body's significant tokens, delimits each
// control-flow expression (IF/ELSEIF/WHILE condition up to THEN/DO, REPEAT-UNTIL
// condition up to END, and RETURN and ':=' right-hand side up to ';'), and
// reports undeclared identifiers in it. baseLine/baseCol locate the body's first
// character for rebasing. `DO`/`ELSEIF` are not tokenizer keywords, so control
// words are matched via IsIdentLike rather than by kind. The IF/ELSEIF/WHILE/
// UNTIL/RETURN keywords are recognized only at a statement-start position (the
// body start, or just after a depth-0 ';' or a block keyword) so guarded DDL
// like `CREATE TABLE IF NOT EXISTS …` is not mistaken for a condition.
func scanScriptExprIdents(inner string, baseLine, baseCol int, vars map[string]bool) []DiagMarker {
	var out []DiagMarker
	sig := sqltok.SignificantTokens(inner)
	word := func(i int) string { return strings.ToUpper(sig[i].Text(inner)) }
	depth, atStart := 0, true
	for i := 0; i < len(sig); i++ {
		t := sig[i]
		curStart := atStart && depth == 0
		// A new statement begins after a depth-0 ';' or a block keyword; nested
		// tokens inside a '(' … ')' are never statement starts.
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		}
		if t.Kind == sqltok.Semicolon {
			atStart = true
		} else if t.Kind.IsIdentLike() {
			atStart = scriptStmtStarters[word(i)]
		} else {
			atStart = false
		}

		var lo, hi int
		switch {
		case curStart && t.Kind.IsIdentLike() && (word(i) == "IF" || word(i) == "ELSEIF"):
			lo, hi = i+1, scriptStopAt(sig, inner, i+1, "THEN")
		case curStart && t.Kind.IsIdentLike() && word(i) == "WHILE":
			lo, hi = i+1, scriptStopAt(sig, inner, i+1, "DO")
		case curStart && t.Kind.IsIdentLike() && word(i) == "UNTIL":
			lo, hi = i+1, scriptStopAt(sig, inner, i+1, "END")
		case curStart && t.Kind.IsIdentLike() && word(i) == "RETURN":
			lo, hi = i+1, scriptStopSemicolon(sig, i+1)
		case t.Kind == sqltok.Colon && i+1 < len(sig) && sig[i+1].Kind == sqltok.Operator &&
			sig[i+1].Text(inner) == "=" && t.End == sig[i+1].Start: // ':=' assignment
			lo, hi = i+2, scriptStopSemicolon(sig, i+2)
		default:
			continue
		}
		out = append(out, scanExprRange(sig, inner, lo, hi, vars, baseLine, baseCol)...)
	}
	return out
}

// scriptStopAt returns the index of the first token from `from` (at paren depth 0)
// whose word equals stop, or that terminates the statement (';'), or len(sig).
func scriptStopAt(sig []sqltok.Token, inner string, from int, stop string) int {
	depth := 0
	for k := from; k < len(sig); k++ {
		switch sig[k].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		case sqltok.Semicolon:
			if depth == 0 {
				return k
			}
		default:
			if depth == 0 && sig[k].Kind.IsIdentLike() && strings.ToUpper(sig[k].Text(inner)) == stop {
				return k
			}
		}
	}
	return len(sig)
}

// scriptStopSemicolon returns the index of the first depth-0 ';' from `from`, or
// len(sig) if the statement runs to the end of the body.
func scriptStopSemicolon(sig []sqltok.Token, from int) int {
	depth := 0
	for k := from; k < len(sig); k++ {
		switch sig[k].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		case sqltok.Semicolon:
			if depth == 0 {
				return k
			}
		}
	}
	return len(sig)
}

// scanExprRange reports each identifier in sig[lo:hi] that is not a declared
// variable/argument, a call (`name(`), a qualified-name part (adjacent to '.'),
// or a builtin/keyword/date-part word. A range that embeds a query (SELECT/WITH)
// is skipped, since such an expression can legitimately reference columns — the
// same conservative stance as validateSQLExprBodyIdents (#509). Adjacency peeks
// use the full sig bounds so a call/qualifier just past `hi` still exempts.
func scanExprRange(sig []sqltok.Token, inner string, lo, hi int, vars map[string]bool, baseLine, baseCol int) []DiagMarker {
	if lo >= hi {
		return nil
	}
	for k := lo; k < hi; k++ {
		if sig[k].Kind == sqltok.Keyword {
			if up := strings.ToUpper(sig[k].Text(inner)); up == "SELECT" || up == "WITH" {
				return nil
			}
		}
	}
	var out []DiagMarker
	for k := lo; k < hi; k++ {
		t := sig[k]
		if t.Kind != sqltok.Identifier {
			continue
		}
		up := strings.ToUpper(t.Text(inner))
		if vars[up] || sqlExprBareWords[up] || scriptBuiltinVars[up] ||
			sqltok.IsBuiltinFunction(up) || sqltok.IsReserved(up) || sqltok.IsKeyword(up) {
			continue
		}
		if k+1 < len(sig) && (sig[k+1].Kind == sqltok.LParen || sig[k+1].Kind == sqltok.Dot) {
			continue
		}
		if k > 0 && sig[k-1].Kind == sqltok.Dot {
			continue
		}
		l := baseLine + t.Line - 1
		c := t.Col
		if t.Line == 1 {
			c = baseCol + t.Col - 1
		}
		name := t.Text(inner)
		out = append(out, DiagMarker{
			StartLineNumber: l, StartColumn: c,
			EndLineNumber: l, EndColumn: c + (t.End - t.Start),
			Message:  "Variable '" + name + "' is not declared",
			Severity: SeverityError,
		})
	}
	return out
}

// ── TypeCategory ──────────────────────────────────────────────────────────────

// registryCategoryToJoinBucket maps a registry DataTypeCategory to the broad
// JOIN-suggestion compatibility bucket used by ComputeJoinOnConditions.
// Categories without an entry fall through to "other" (e.g. structured,
// geospatial, vector).  Note BINARY collapses into "text" for JOIN purposes.
var registryCategoryToJoinBucket = map[sf.DataTypeCategory]string{
	sf.CategoryNumeric:        "numeric",
	sf.CategoryString:         "text",
	sf.CategoryBinary:         "text",
	sf.CategoryBoolean:        "boolean",
	sf.CategoryDatetime:       "datetime",
	sf.CategorySemiStructured: "semi",
}

// typeCategoryMap maps canonical upper-case Snowflake type names to the broad
// JOIN-suggestion compatibility bucket.  It is built once from
// snowflake.AllDataTypes using each type's authoritative Category, so any type
// added to the registry is automatically visible here (defaulting to "other").
var typeCategoryMap = func() map[string]string {
	m := make(map[string]string, len(sf.AllDataTypes()))
	for _, dt := range sf.AllDataTypes() {
		if bucket, ok := registryCategoryToJoinBucket[dt.Category]; ok {
			m[dt.Name] = bucket
		} else {
			m[dt.Name] = "other"
		}
	}
	return m
}()

// TypeCategory maps a Snowflake data-type string to a broad compatibility
// category used by the JOIN suggestion engine.
// Returns one of: "numeric", "text", "datetime", "boolean", "semi", "other".
func TypeCategory(dt string) string {
	t := strings.ToUpper(dt)
	// Strip type parameters (e.g. VARCHAR(255) → VARCHAR).
	if idx := strings.Index(t, "("); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	if cat, ok := typeCategoryMap[t]; ok {
		return cat
	}
	return "other"
}

// ── CTE projection helpers ────────────────────────────────────────────────────

// reAsAliasExpr matches a trailing "AS <alias>" at the end of a SELECT-list
// expression (anchored with $).
var reAsAliasExpr = regexp.MustCompile(`(?i)\bAS\s+(` + _ident + `)\s*$`)

// reSimpleQualifiedIdent matches a simple (optionally dot-qualified) identifier
// with no operators, parens, or spaces — e.g. "col", "t.col", "db.sch.col".
var reSimpleQualifiedIdent = regexp.MustCompile(`^(` + _ident + `)(\.` + _ident + `){0,2}\s*$`)

// splitTopLevelCommas splits s by commas that are not nested inside
// parentheses or string literals.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	inSingle := false
	inDouble := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			switch c {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					parts = append(parts, s[start:i])
					start = i + 1
				}
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// extractProjectedColName returns the column name that a SELECT-list expression
// projects, or "" if it cannot be determined without a full AST.
//
// Rules (in order):
//  1. Trailing "AS alias" → use alias.
//  2. Simple bare/qualified identifier (no operators, no parens) → use last part.
//  3. Everything else (functions without alias, arithmetic, etc.) → "".
func extractProjectedColName(expr string) string {
	expr = strings.TrimSpace(expr)
	// Rule 1: explicit AS alias at the end of the expression.
	if m := reAsAliasExpr.FindStringSubmatch(expr); m != nil {
		return strings.ToUpper(normIdent(m[1], true))
	}
	// Rule 1.5: implicit (AS-less) trailing alias — a bare/quoted identifier
	// closing the item, preceded by an expression terminator (e.g. `COUNT(*) cnt`,
	// `ID employee_id`). A single bare/qualified ident has no such predecessor and
	// falls through to rule 2; `db.sch.col` has a Dot before the last part, not a
	// terminator, so it is never misread as an alias.
	if sig := sigTokens(expr); len(sig) >= 2 {
		last := sig[len(sig)-1]
		if isAliasTok(last) && isExprEndAt(sig, len(sig)-2, expr) {
			return strings.ToUpper(normIdent(last.Text(expr), true))
		}
	}
	// Rule 2: simple identifier — must not contain operators or function calls.
	if strings.ContainsAny(expr, "()+-*/%|&^!<>=") {
		return ""
	}
	if reSimpleQualifiedIdent.MatchString(expr) {
		// Use the last dot-component as the column name.
		parts := strings.Split(expr, ".")
		return strings.ToUpper(normIdent(strings.TrimSpace(parts[len(parts)-1]), true))
	}
	return ""
}

// extractSelectProjections returns the projected column names from a SELECT
// statement by parsing its SELECT-list.
//
// If the statement is SELECT *, it attempts to expand columns based on the
// table(s) found in the immediate FROM/JOIN of this SELECT block.
func extractSelectProjections(sql string, localScope map[string][]ColInfo) []ColInfo {
	stripped := stripCommentsSQL(sql)
	strippedToks := sqltok.Tokenize(stripped)
	strippedSig := sigToks(strippedToks)
	selOff := findSelectKWOffset(strippedSig, stripped)
	if selOff < 0 {
		return nil
	}

	// 1. Determine the context for this SELECT (Step A: Source Resolution)
	// We extract table references only from THIS select block.
	activeContext := make(map[string][]ColInfo)
	for _, tablePath := range findFromJoinTables2(strippedSig, stripped) {
		parts := extractIdentParts(tablePath, true)
		if len(parts) > 0 {
			tableNameU := parts[len(parts)-1]
			if cols, ok := localScope[tableNameU]; ok {
				activeContext[tableNameU] = cols
			}
		}
	}

	// 2. Evaluate the SELECT clause (Step C: Output Registration)
	afterSelect := strings.TrimSpace(stripped[selOff+6:])
	upAfter := strings.ToUpper(afterSelect)
	if strings.HasPrefix(upAfter, "DISTINCT ") || strings.HasPrefix(upAfter, "DISTINCT\t") || strings.HasPrefix(upAfter, "DISTINCT\n") {
		afterSelect = strings.TrimSpace(afterSelect[8:])
	}
	selectClause := extractSelectClause(afterSelect)

	var cols []ColInfo
	for _, expr := range splitTopLevelCommas(selectClause) {
		trimmed := strings.TrimSpace(expr)
		if trimmed == "*" {
			// Resolve wildcard expansions
			for _, tableCols := range activeContext {
				cols = append(cols, tableCols...)
			}
			continue
		}
		if name := extractProjectedColName(trimmed); name != "" {
			cols = append(cols, ColInfo{Name: name, DataType: "UNKNOWN"})
		}
	}
	return cols
}

// isSimpleCTESelect returns true when every item in the CTE's SELECT list is a
// bare or qualified column reference with no function calls, arithmetic operators,
// or AS-renamed aliases.  For such CTEs the effective schema equals the source
// table's actual columns, allowing us to detect bare column typos in the CTE body.
func isSimpleCTESelect(innerSQL string) bool {
	stripped := stripCommentsSQL(innerSQL)
	strippedToks := sqltok.Tokenize(stripped)
	strippedSig := sigToks(strippedToks)
	selOff := findSelectKWOffset(strippedSig, stripped)
	if selOff < 0 {
		return false
	}
	afterSelect := strings.TrimSpace(stripped[selOff+6:])
	upAfter := strings.ToUpper(afterSelect)
	if strings.HasPrefix(upAfter, "DISTINCT ") || strings.HasPrefix(upAfter, "DISTINCT\t") || strings.HasPrefix(upAfter, "DISTINCT\n") {
		afterSelect = strings.TrimSpace(afterSelect[8:])
	}
	selectClause := extractSelectClause(afterSelect)
	for _, expr := range splitTopLevelCommas(selectClause) {
		trimmed := strings.TrimSpace(expr)
		if trimmed == "*" {
			continue // wildcard — source columns will be used as-is
		}
		// reSimpleQualifiedIdent only matches bare/qualified identifiers with no
		// operators, parentheses, spaces, or AS aliases.
		if !reSimpleQualifiedIdent.MatchString(trimmed) {
			return false
		}
	}
	return true
}

// getSimpleSelectColumnNames extracts bare column names from a simple SELECT
// statement (one that already passes isSimpleCTESelect).  Returns the last
// dot-component of each SELECT-list item, uppercased.  Returns ["*"] if the
// select list contains a wildcard.
func getSimpleSelectColumnNames(innerSQL string) []string {
	stripped := stripCommentsSQL(innerSQL)
	strippedToks := sqltok.Tokenize(stripped)
	strippedSig := sigToks(strippedToks)
	selOff := findSelectKWOffset(strippedSig, stripped)
	if selOff < 0 {
		return nil
	}
	afterSelect := strings.TrimSpace(stripped[selOff+6:])
	upAfter := strings.ToUpper(afterSelect)
	if strings.HasPrefix(upAfter, "DISTINCT ") || strings.HasPrefix(upAfter, "DISTINCT\t") || strings.HasPrefix(upAfter, "DISTINCT\n") {
		afterSelect = strings.TrimSpace(afterSelect[8:])
	} else if strings.HasPrefix(upAfter, "ALL ") {
		afterSelect = strings.TrimSpace(afterSelect[4:])
	}
	selectClause := extractSelectClause(afterSelect)

	var names []string
	for _, expr := range splitTopLevelCommas(selectClause) {
		trimmed := strings.TrimSpace(expr)
		if trimmed == "*" {
			return []string{"*"}
		}
		parts := strings.Split(trimmed, ".")
		last := strings.TrimSpace(parts[len(parts)-1])
		names = append(names, strings.ToUpper(normIdent(last, true)))
	}
	return names
}

// extractCTEProjections processes a WITH clause sequentially (Step 3).
func extractCTEProjections(stripped string, globalRegistry map[string][]ColInfo) map[string][]ColInfo {
	localScope := make(map[string][]ColInfo)
	for k, v := range globalRegistry {
		localScope[k] = v
	}

	result := make(map[string][]ColInfo)

	// CollectCTEDefs is the single structural CTE scanner (issue #673): it
	// returns each member's name/column-list/body spans in source order, so
	// later CTEs can reference earlier ones. It also descends into nested WITH
	// clauses; we skip those (cursor guard) to keep this the top-level
	// WITH-clause scope the projection registry has always tracked.
	sig := sqltok.SignificantTokens(stripped)
	cursor := 0
	for _, def := range sqlgrammar.CollectCTEDefs(stripped, sig) {
		if def.BodyStart < cursor {
			continue // nested inside an already-processed body
		}

		cteName := def.Name

		// Explicit column aliases, e.g. cte(col_a, col_b) AS (...).
		var explicitCols []string
		if def.ColsStart >= 0 {
			for _, part := range splitTopLevelCommas(stripped[def.ColsStart+1 : def.ColsEnd-1]) {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					explicitCols = append(explicitCols, strings.ToUpper(normIdent(trimmed, true)))
				}
			}
		}

		if !def.Closed {
			continue // unterminated body (statement still being typed)
		}
		cursor = def.BodyEnd
		innerSQL := strings.TrimSpace(stripped[def.BodyStart+1 : def.BodyEnd-1])

		// Process this CTE using current localScope.
		firstTok := getFirstSQLToken(innerSQL)
		var cteCols []ColInfo
		if firstTok == "SELECT" || firstTok == "WITH" || firstTok == "VALUES" {
			// For simple CTEs (bare/qualified column refs only, no function calls or
			// AS-renamed aliases), use the SOURCE table's actual columns as the CTE's
			// effective schema.  This makes bare column typos inside the CTE body
			// detectable: if a SELECT item doesn't exist in the source table the bare
			// column scanner will flag it, and alias.col refs in the outer query are
			// validated against the real schema instead of the (possibly typo-laden)
			// projection list.
			if isSimpleCTESelect(innerSQL) {
				innerStripped := stripCommentsSQL(innerSQL)
				innerToks := sqltok.Tokenize(innerStripped)
				innerSig := sigToks(innerToks)
				var allSourceCols []ColInfo
				for _, tablePath := range findFromJoinTables2(innerSig, innerStripped) {
					parts := extractIdentParts(tablePath, true)
					if len(parts) > 0 {
						tableNameU := parts[len(parts)-1]
						if cols, ok := localScope[tableNameU]; ok {
							allSourceCols = append(allSourceCols, cols...)
						}
					}
				}
				if len(allSourceCols) > 0 {
					// Extract projected column names from the SELECT list.
					// Only filter when every projected name exists in the
					// source — this handles chained CTEs (e.g. SELECT id
					// FROM base) correctly while preserving typo detection
					// when a projected name doesn't match any source column.
					projNames := getSimpleSelectColumnNames(innerSQL)
					if len(projNames) > 0 && projNames[0] != "*" {
						sourceMap := make(map[string]ColInfo)
						for _, c := range allSourceCols {
							sourceMap[c.Name] = c
						}
						allExist := true
						for _, name := range projNames {
							if _, ok := sourceMap[name]; !ok {
								allExist = false
								break
							}
						}
						if allExist {
							for _, name := range projNames {
								cteCols = append(cteCols, sourceMap[name])
							}
						} else {
							// Some projected names don't exist in source
							// (potential typos) — use all source columns to
							// preserve typo detection in alias.col validation.
							cteCols = allSourceCols
						}
					} else {
						// SELECT * — return all source columns
						cteCols = allSourceCols
					}
				}
			}
			// Fall back to SELECT-list projections when the CTE is complex or the
			// source table's columns are not available in the local scope.
			if len(cteCols) == 0 {
				cteCols = extractSelectProjections(innerSQL, localScope)
			}
		}

		// If CTE has explicit column aliases, override with those names.
		if len(explicitCols) > 0 {
			overridden := make([]ColInfo, len(explicitCols))
			for i, name := range explicitCols {
				dt := "UNKNOWN"
				if i < len(cteCols) {
					dt = cteCols[i].DataType
				}
				overridden[i] = ColInfo{Name: name, DataType: dt}
			}
			cteCols = overridden
		}

		if len(cteCols) > 0 {
			nameU := strings.ToUpper(normIdent(cteName, true))
			result[nameU] = cteCols
			localScope[nameU] = cteCols // Update local scope for next CTE in sequence
		}
	}
	return result
}

// ── ValidateSemantics ─────────────────────────────────────────────────────────

// ValidateSemantics walks the SQL text and for every alias.column two-part
// reference where the alias is in resolvedRefs, checks whether column exists
// in the cached column list.  Unknown columns emit Warning markers.
func ValidateSemantics(sql string, resolvedRefs []ResolvedRef, colEntries []ColEntry) []DiagMarker {
	var markers []DiagMarker
	stmtRanges := GetStatementRanges(sql)

	// colInfoCacheGlobal: "DB\x00SCHEMA\x00NAME" -> []ColInfo
	colInfoCacheGlobal := map[string][]ColInfo{}
	for _, e := range colEntries {
		key := strings.ToUpper(e.DB) + "\x00" + strings.ToUpper(e.Schema) + "\x00" + strings.ToUpper(e.Name)
		colInfoCacheGlobal[key] = e.Cols
	}

	// globalAliasMap for objects already resolved by frontend.
	globalAliasMap := map[string]string{}
	for _, ref := range resolvedRefs {
		key := strings.ToUpper(ref.DB) + "\x00" + strings.ToUpper(ref.Schema) + "\x00" + strings.ToUpper(ref.Name)
		globalAliasMap[strings.ToUpper(ref.Alias)] = key
	}

	// ── Pre-calculate per-statement context ──────────────────────────────
	type stmtContext struct {
		aliasMap          map[string]string
		colInfoCache      map[string][]ColInfo
		activeKeys        []string // ordered list of tables in scope for bare col lookup
		bareColValidation bool     // true only when every FROM/JOIN source table has known columns
	}
	stmtContexts := make([]stmtContext, len(stmtRanges))
	localColCache := make(map[string][]ColInfo)

	for idx, r := range stmtRanges {
		raw := sqlStmt(sql, r)
		stripped := stripCommentsSQL(raw)
		rawToks := sqltok.Tokenize(raw)
		rawSig := sigToks(rawToks)
		strippedToks := sqltok.Tokenize(stripped)
		strippedSig := sigToks(strippedToks)

		// 1. Update localColCache if this is a CREATE TABLE
		if nameStr, parenStart, ok := matchCreateTablePre(rawSig, raw); ok {
			parts := extractIdentParts(nameStr, true)
			if len(parts) > 0 {
				colsRaw := extractBalancedBlock(raw, parenStart)
				if len(colsRaw) >= 2 {
					colsRaw = colsRaw[1 : len(colsRaw)-1]
					columns := parseCreateTableColDefs(colsRaw, true)
					tableNameU := strings.ToUpper(parts[len(parts)-1])
					localColCache[tableNameU] = columns
				}
			}
		} else if aPath, aCols, aok := parseAlterAddColumns(rawSig, raw, true); aok {
			// ALTER TABLE … ADD [COLUMN] on a table created in-script: merge the
			// added columns so later references resolve (issue #715).
			if parts := extractIdentParts(aPath, true); len(parts) > 0 {
				tableNameU := strings.ToUpper(parts[len(parts)-1])
				if existing, ok := localColCache[tableNameU]; ok {
					merged := make([]ColInfo, 0, len(existing)+len(aCols))
					merged = append(merged, existing...)
					merged = append(merged, aCols...)
					localColCache[tableNameU] = merged
				}
			}
		}

		// 2. CTE projections in this statement
		var cteProjMap map[string][]ColInfo
		if strings.Contains(strings.ToUpper(stripped), "WITH") {
			cteProjMap = extractCTEProjections(stripped, localColCache)
		}

		// 3. Build stmtContext
		ctx := stmtContext{
			aliasMap:     make(map[string]string),
			colInfoCache: make(map[string][]ColInfo),
			activeKeys:   make([]string, 0),
		}

		// Add the object being created to the aliasMap so it's not flagged as a missing column.
		if rawPath, _, ok := matchCreateTV(rawSig, raw); ok {
			if parts := extractIdentParts(rawPath, true); len(parts) > 0 {
				objNameU := strings.ToUpper(parts[len(parts)-1])
				ctx.aliasMap[objNameU] = "__object__"
			}
		} else if rawPath, ok := matchCreateDbSch(rawSig, raw); ok {
			if parts := extractIdentParts(rawPath, true); len(parts) > 0 {
				objNameU := strings.ToUpper(parts[len(parts)-1])
				ctx.aliasMap[objNameU] = "__object__"
			}
		}

		// Pre-scan for column aliases (AS alias) and add them to the aliasMap.
		// This prevents false positives on alias names within the same statement.
		for _, loc := range findAsAliases(strippedSig, stripped) {
			aliasText := stripped[loc.aliasStart:loc.aliasEnd]
			aliasU := strings.ToUpper(normIdent(aliasText, true))
			if !sqltok.IsKeyword(aliasU) {
				ctx.aliasMap[aliasU] = "__alias__"
			}
		}

		// Inherit global state
		for k, v := range globalAliasMap {
			ctx.aliasMap[k] = v
		}
		for k, v := range colInfoCacheGlobal {
			ctx.colInfoCache[k] = v
		}

		// Add CTEs
		for cteNameU, cols := range cteProjMap {
			cacheKey := "__cte__\x00\x00" + cteNameU
			ctx.colInfoCache[cacheKey] = cols
			ctx.aliasMap[cteNameU] = cacheKey
		}

		// 4. Resolve FROM/JOIN aliases against local tables and CTEs.
		// hasUnknownTable becomes true when any referenced table has no known columns.
		// In that case bareColValidation is disabled for the whole statement to prevent
		// false positives on column references from those unknown tables.
		hasUnknownTable := false
		hasValuesSource := false
		for _, ta := range findFromJoinWithAlias(strippedSig, stripped) {
			parts := extractIdentParts(ta.tablePath, true)
			if len(parts) == 0 {
				continue
			}
			tableNameU := parts[len(parts)-1]

			// `FROM VALUES (...)` is a table literal exposing implicit columns
			// (column1..columnN, or an explicit `AS v (a,b,c)` list). Their names
			// aren't known here, so disable bare-column validation for the whole
			// statement rather than flag them as out-of-scope.
			// ponytail: whole-statement disable; register synthetic column1..N /
			// explicit-list scope if a VALUES statement ever also needs a real
			// table's bare columns validated.
			if strings.EqualFold(tableNameU, "VALUES") {
				hasValuesSource = true
				continue
			}

			cacheKey := ""
			// Priority: 1. CTE, 2. Local Table, 3. Global resolvedRef (already in aliasMap)
			if key, isCTE := ctx.aliasMap[tableNameU]; isCTE && strings.HasPrefix(key, "__cte__") {
				cacheKey = key
			} else if cols, isLocal := localColCache[tableNameU]; isLocal {
				cacheKey = "__local__\x00\x00" + tableNameU
				ctx.colInfoCache[cacheKey] = cols
				if _, already := ctx.aliasMap[tableNameU]; !already {
					ctx.aliasMap[tableNameU] = cacheKey
				}
			} else {
				// Search in global colInfoCacheGlobal if it wasn't a CTE or local table
				// and if the path matches a known table.
				if len(parts) == 3 {
					key := strings.ToUpper(parts[0]) + "\x00" + strings.ToUpper(parts[1]) + "\x00" + strings.ToUpper(parts[2])
					if _, ok := colInfoCacheGlobal[key]; ok {
						cacheKey = key
					}
				} else if len(parts) == 1 {
					// Search all global tables for a matching name
					for k := range colInfoCacheGlobal {
						if strings.HasSuffix(k, "\x00"+tableNameU) {
							cacheKey = k
							break
						}
					}
				}
			}

			if cacheKey != "" {
				ctx.activeKeys = append(ctx.activeKeys, cacheKey)

				// Register explicit alias if present.
				if ta.alias != "" {
					aliasU := strings.ToUpper(ta.alias)
					if !joinStopKW[aliasU] {
						ctx.aliasMap[aliasU] = cacheKey
					}
				}
				// Always register the table name itself in aliasMap so the scanner does
				// not mistake it for a bare column reference.
				if _, already := ctx.aliasMap[tableNameU]; !already {
					ctx.aliasMap[tableNameU] = cacheKey
				}
			} else if !sqltok.IsKeyword(tableNameU) {
				// Unknown table (not a CTE, not local, not in global registry, not a SQL
				// keyword like TABLE).  Disable bare-column validation for this statement
				// to prevent false positives on columns from the unknown source table.
				hasUnknownTable = true
				if _, already := ctx.aliasMap[tableNameU]; !already {
					ctx.aliasMap[tableNameU] = "__unknown__"
				}
			}
		}
		// Bare-column validation is only safe when every source table has known columns
		// (so we can definitively say whether a column exists) and there is at least one
		// table in scope to validate against. It is also disabled for statements whose
		// clause bodies expose columns/keywords our flat scan can't model — PIVOT/UNPIVOT
		// (FOR/value literals), CONNECT BY (LEVEL/PRIOR pseudo-columns), LATERAL FLATTEN
		// (SEQ/KEY/VALUE… output), etc. — mirroring ValidateBareColumnRefs' matchesSnowflakeFP
		// guard, which ValidateSemantics previously lacked (issue #793 D2/D4/D5).
		// A derived table (FROM/JOIN (<subquery>) …) exposes columns our flat
		// scanner can't resolve, and its inner FROM leaks into the table-in-scope
		// set — so the alias after the subquery, and the subquery's own tables, were
		// misread as columns (issue #793 D3). Treat it like an unknown source.
		hasDerivedTable := hasSubquerySource(strippedSig, stripped)

		ctx.bareColValidation = !hasUnknownTable && !hasValuesSource && !hasDerivedTable &&
			len(ctx.activeKeys) > 0 && !matchesSnowflakeFP(rawSig, raw)

		stmtContexts[idx] = ctx
	}

	runes := []rune(sql)
	n := len(runes)
	line, col := 1, 1
	i := 0
	stmtIdx := 0

	// prevSigRune is the last significant rune consumed — the last rune that is
	// neither whitespace nor part of a comment or string literal. It lets us
	// answer "is this identifier immediately preceded by `*`?" without a raw
	// backward scan, which would incorrectly see comment characters (e.g. the
	// `*` closing a `/* … */` block, or a `*` inside a line comment). The
	// forward walk below already skips comments/strings, so tracking the last
	// significant rune here mirrors the token-based lookback used by
	// scanSelectClauseForUnknownCols in barecolrefs.go.
	var prevSigRune rune

	for i < n {
		// Advance to the current statement context
		for stmtIdx < len(stmtRanges) && i >= stmtRanges[stmtIdx].EndOffset {
			stmtIdx++
		}

		ch := runes[i]
		if ch == '\n' {
			line++
			col = 1
			i++
			continue
		}

		// Skip line comments
		if ch == '-' && i+1 < n && runes[i+1] == '-' {
			i += 2
			col += 2
			for i < n && runes[i] != '\n' {
				i++
				col++
			}
			continue
		}

		// Skip block comments
		if ch == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			col += 2
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
				} else if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					col += 2
					break
				} else {
					i++
					col++
				}
			}
			continue
		}

		// Skip single-quoted strings
		if ch == '\'' {
			i++
			col++
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
				} else if runes[i] == '\'' && i+1 < n && runes[i+1] == '\'' {
					i += 2
					col += 2
				} else if runes[i] == '\'' {
					i++
					col++
					break
				} else {
					i++
					col++
				}
			}
			prevSigRune = '\''
			continue
		}

		// Skip dollar-quoted tag delimiters
		if ch == '$' {
			tag := extractDollarTag(runes, i)
			if tag != "" {
				i += len([]rune(tag))
				col += len([]rune(tag))
				prevSigRune = '$'
				continue
			}
		}

		// Identifier (bare or quoted)
		if ch == '"' || isAlpha(ch) {
			// Capture whether this identifier directly follows a `*` before we
			// consume it (used for the paren-less `SELECT * EXCLUDE col` case).
			prevIsStar := prevSigRune == '*'
			// An identifier directly after a `:` is a variant-path segment
			// (v:field) or a cast target (x::type) / bind ref (:var) — not a
			// column, so it must not be validated as one (issue #793 D1).
			prevIsColon := prevSigRune == ':'
			word1Start := i
			word1Line := line
			word1Col := col
			word1Quoted := (ch == '"')

			// Extract first identifier
			if word1Quoted {
				i++
				col++
				for i < n {
					if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
						i += 2
						col += 2
					} else if runes[i] == '"' {
						i++
						col++
						break
					} else if runes[i] == '\n' {
						line++
						col = 1
						i++
					} else {
						i++
						col++
					}
				}
			} else {
				for i < n && isWordChar(runes[i]) {
					i++
					col++
				}
			}
			word1Raw := string(runes[word1Start:i])
			word1Norm := normIdent(word1Raw, true)

			// Look for dot
			if i < n && runes[i] == '.' {
				i++
				col++

				// Look for second identifier
				if i < n && (runes[i] == '"' || isAlpha(runes[i])) {
					word2Start := i
					word2Line := line
					word2Col := col
					word2Quoted := (runes[i] == '"')

					if word2Quoted {
						i++
						col++
						for i < n {
							if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
								i += 2
								col += 2
							} else if runes[i] == '"' {
								i++
								col++
								break
							} else if runes[i] == '\n' {
								line++
								col = 1
								i++
							} else {
								i++
								col++
							}
						}
					} else {
						for i < n && isWordChar(runes[i]) {
							i++
							col++
						}
					}
					word2Raw := string(runes[word2Start:i])
					word2Norm := normIdent(word2Raw, true)

					// Only validate two-part references (skip db.schema.col)
					if !(i < n && runes[i] == '.') {
						if stmtIdx < len(stmtContexts) {
							ctx := stmtContexts[stmtIdx]
							if cacheKey, ok := ctx.aliasMap[word1Norm]; ok {
								if cols, ok := ctx.colInfoCache[cacheKey]; ok {
									found := false
									for _, c := range cols {
										if strings.EqualFold(c.Name, word2Norm) {
											found = true
											break
										}
									}
									if !found {
										tableName := cacheKey
										if parts := strings.Split(cacheKey, "\x00"); len(parts) == 3 {
											tableName = parts[2]
										} else if strings.HasPrefix(cacheKey, "__cte__") || strings.HasPrefix(cacheKey, "__local__") {
											parts := strings.Split(cacheKey, "\x00")
											tableName = parts[len(parts)-1]
										}
										markers = append(markers, DiagMarker{
											StartLineNumber: word2Line,
											StartColumn:     word2Col,
											EndLineNumber:   word2Line,
											EndColumn:       word2Col + len(word2Raw),
											Message:         "Column '" + word2Raw + "' does not exist in " + tableName,
											Severity:        4,
										})
									}
								}
							}
						}
					}
				}
			} else {
				// Bare identifier without dot. Validate against ALL tables in scope.
				if stmtIdx < len(stmtContexts) {
					ctx := stmtContexts[stmtIdx]
					// Paren-less `SELECT * EXCLUDE col`: EXCLUDE is not a global
					// keyword (it is a valid identifier name), so recognize the
					// clause keyword contextually — only when it follows a `*`
					// (prevIsStar was captured above from prevSigRune, which
					// correctly ignores intervening comments and whitespace).
					// Skip if it's a known SQL keyword.
					if !sqltok.IsKeyword(word1Norm) && !isStarExcludeCol(word1Norm, prevIsStar) && !prevIsColon {
						// Heuristic: skip if followed by '(' (likely a function call).
						isFunction := false
						k := i
						for k < n && (runes[k] == ' ' || runes[k] == '\t' || runes[k] == '\r' || runes[k] == '\n') {
							k++
						}
						if k < n && runes[k] == '(' {
							isFunction = true
						}

						// NEW: Skip if it is a date part used as the first argument of a date function
						isDatePartUsage := false
						// Note: Reusing bcrDateParts and bcrDateFuncs defined in barecolrefs.go
						if bcrDateParts[word1Norm] {
							// CRITICAL FIX: Slice the 'runes' array, not the 'sql' byte string,
							// to prevent multi-byte characters (like emojis or em-dashes) from
							// misaligning the index and truncating the function prefix.
							if fn := GetActiveFunctionCall(string(runes[:word1Start])); fn != nil {
								if bcrDateFuncs[strings.ToUpper(fn.Name)] && fn.ParamIndex == 0 {
									isDatePartUsage = true
								}
							}
						}

						if !isFunction && !isDatePartUsage && ctx.bareColValidation {
							// Check if this column exists in ANY of the active tables.
							foundInAny := false
							for _, cacheKey := range ctx.activeKeys {
								if cols, ok := ctx.colInfoCache[cacheKey]; ok {
									for _, c := range cols {
										if strings.EqualFold(c.Name, word1Norm) {
											foundInAny = true
											break
										}
									}
								}
								if foundInAny {
									break
								}
							}

							// Trapdoor: if not found in any active table, it might be an incorrect column name.
							// To avoid false positives on table names themselves or other constructs,
							// we only emit if there is an active context and it doesn't match an alias.
							if !foundInAny {
								if _, isAlias := ctx.aliasMap[word1Norm]; !isAlias {
									markers = append(markers, DiagMarker{
										StartLineNumber: word1Line,
										StartColumn:     word1Col,
										EndLineNumber:   word1Line,
										EndColumn:       word1Col + len(word1Raw),
										Message:         "Column '" + word1Raw + "' not found in any of the tables in scope.",
										Severity:        4,
									})
								}
							}
						}
					}
				}
			}
			// The last consumed rune is an identifier char (or closing quote),
			// never `*`, so record it as the previous significant rune.
			prevSigRune = runes[i-1]
			continue
		}

		if ch != ' ' && ch != '\t' && ch != '\r' {
			prevSigRune = ch
		}
		i++
		col++
	}

	return markers
}

// ── ComputeJoinOnConditions ───────────────────────────────────────────────────

// filterFKsByPK returns only those FKs whose pk_table/schema/database match pkRef.
func filterFKsByPK(fks []FKEntry, pkRef ResolvedRef) []FKEntry {
	var result []FKEntry
	for _, fk := range fks {
		if !strings.EqualFold(fk.PKTable, pkRef.Name) {
			continue
		}
		if fk.PKSchema != "" && !strings.EqualFold(fk.PKSchema, pkRef.Schema) {
			continue
		}
		if fk.PKDatabase != "" && !strings.EqualFold(fk.PKDatabase, pkRef.DB) {
			continue
		}
		result = append(result, fk)
	}
	return result
}

func colNames(infos []ColInfo) []string {
	names := make([]string, len(infos))
	for i, c := range infos {
		names[i] = c.Name
	}
	return names
}

// tableKey returns the canonical cache key for a ResolvedRef.
func tableKey(ref ResolvedRef) string {
	return strings.ToUpper(ref.DB) + "\x00" +
		strings.ToUpper(ref.Schema) + "\x00" +
		strings.ToUpper(ref.Name)
}

// ComputeJoinOnConditions computes JOIN ON / USING condition suggestions using
// three tiers:
//
//	Tier 1a — Explicit FK constraints (composite-aware, FK→PK and PK→FK).
//	Tier 1b — PK naming heuristic (TABLE_B.TABLE_A_ID ↔ TABLE_A.ID).
//	Tier 2  — Type-compatible same-name columns + USING clause.
//
// The req.Prefix is prepended to each condition (use "ON " for trigger-C mode).
func ComputeJoinOnConditions(req JoinOnSuggestionsReq) []JoinCondition {
	if len(req.ResolvedRefs) < 2 {
		return nil
	}

	// Build FK lookup: tableKey → []FKEntry
	fkMap := map[string][]FKEntry{}
	for _, entry := range req.FKEntries {
		k := strings.ToUpper(entry.DB) + "\x00" +
			strings.ToUpper(entry.Schema) + "\x00" +
			strings.ToUpper(entry.Name)
		fkMap[k] = entry.FKs
	}

	// Build column info lookup: tableKey → []ColInfo
	colMap := map[string][]ColInfo{}
	for _, entry := range req.ColEntries {
		k := strings.ToUpper(entry.DB) + "\x00" +
			strings.ToUpper(entry.Schema) + "\x00" +
			strings.ToUpper(entry.Name)
		colMap[k] = entry.Cols
	}

	lastRef := req.ResolvedRefs[len(req.ResolvedRefs)-1]
	otherRefs := req.ResolvedRefs[:len(req.ResolvedRefs)-1]
	prefix := req.Prefix

	var suggestions []JoinCondition
	seen := map[string]bool{}

	addSugg := func(cond, detail, sortPfx string) {
		pfx := prefix
		if strings.HasPrefix(cond, "USING") && strings.HasPrefix(prefix, "ON ") {
			pfx = "USING "
			cond = cond[len("USING "):]
		}
		full := pfx + cond
		if !seen[full] {
			seen[full] = true
			suggestions = append(suggestions, JoinCondition{
				Condition: full,
				Detail:    detail,
				SortText:  sortPfx + full,
			})
		}
	}

	lastKey := tableKey(lastRef)
	lastFKs := fkMap[lastKey]

	// ── Tier 1a: FK constraints ────────────────────────────────────────────
	for _, otherRef := range otherRefs {
		// lastRef is the FK child → otherRef is the PK parent
		fksForPK := filterFKsByPK(lastFKs, otherRef)
		for _, cond := range BuildCompositeConditions(fksForPK, lastRef.Alias, otherRef.Alias) {
			addSugg(cond, "FK RELATION", "0a")
		}

		// otherRef is the FK child → lastRef is the PK parent
		otherFKs := fkMap[tableKey(otherRef)]
		fksForLast := filterFKsByPK(otherFKs, lastRef)
		for _, cond := range BuildCompositeConditions(fksForLast, otherRef.Alias, lastRef.Alias) {
			addSugg(cond, "FK RELATION", "0b")
		}
	}

	// ── Tier 1b: PK naming heuristic (only when no FK suggestions) ─────────
	if len(suggestions) == 0 {
		lastColNames := colNames(colMap[lastKey])
		for _, otherRef := range otherRefs {
			otherColNames := colNames(colMap[tableKey(otherRef)])
			for _, cond := range PkHeuristicConditions(
				lastRef.Name, lastRef.Alias,
				otherRef.Name, otherRef.Alias,
				lastColNames, otherColNames,
			) {
				addSugg(cond, "PK HEURISTIC", "0c")
			}
		}
	}

	// ── Tier 2: Type-compatible same-name columns + USING ──────────────────
	lastColInfos := colMap[lastKey]
	lastColInfoMap := map[string]string{}
	for _, c := range lastColInfos {
		lastColInfoMap[strings.ToUpper(c.Name)] = c.DataType
	}

	for _, otherRef := range otherRefs {
		otherColInfos := colMap[tableKey(otherRef)]
		var sharedCompatible []string
		for _, info := range otherColInfos {
			dt1, ok := lastColInfoMap[strings.ToUpper(info.Name)]
			if !ok {
				continue
			}
			cat1 := TypeCategory(dt1)
			cat2 := TypeCategory(info.DataType)
			// Allow if same category, or either side is "other" (unknown → permissive)
			if cat1 != "other" && cat2 != "other" && cat1 != cat2 {
				continue
			}
			sharedCompatible = append(sharedCompatible, sf.QuoteOrBare(info.Name, false))

			// Standardize order: smaller alias first to keep suggestions stable
			a1, a2 := lastRef.Alias, otherRef.Alias
			if a1 > a2 {
				a1, a2 = a2, a1
			}
			cond := sf.QuoteOrBare(a1, a1 != strings.ToUpper(a1)) + "." + sf.QuoteOrBare(info.Name, false) + " = " + sf.QuoteOrBare(a2, a2 != strings.ToUpper(a2)) + "." + sf.QuoteOrBare(info.Name, false)
			addSugg(cond, "SAME-NAME COLUMN", "1")
		}
		if len(sharedCompatible) > 0 {
			usingCond := "USING (" + strings.Join(sharedCompatible, ", ") + ")"
			addSugg(usingCond, "USING", "1.5")
		}
	}

	return suggestions
}

// ── GetAutocompleteContext ────────────────────────────────────────────────────

// GetAutocompleteContext bundles statement ranges, scripting completions, table
// references, and CTE column projections for the current cursor position into a
// single response. This replaces multiple sequential IPC calls from the frontend.
func GetAutocompleteContext(sql string, cursorOffset int) AutocompleteContext {
	ranges := GetStatementRanges(sql)

	// Identify which statement contains the cursor. stmtStart is the chosen
	// statement's start offset, so the cursor's statement-local offset (for the
	// grammar) is cursorOffset - stmtStart.
	currentIdx := -1
	currentStmt := sql
	stmtStart := 0
	for i, r := range ranges {
		if cursorOffset >= r.StartOffset && cursorOffset <= r.EndOffset {
			currentIdx = i
			stmtStart = r.StartOffset
			runes := []rune(sql)
			currentStmt = string(runes[r.StartOffset:r.EndOffset])
			break
		}
	}
	// If cursor is past all ranges, use the last statement.
	if currentIdx == -1 && len(ranges) > 0 {
		last := ranges[len(ranges)-1]
		currentIdx = len(ranges) - 1
		stmtStart = last.StartOffset
		runes := []rune(sql)
		if last.EndOffset <= len(runes) {
			currentStmt = string(runes[last.StartOffset:last.EndOffset])
		}
	}

	// Grammar-driven "valid next" set at the cursor (nil for unmodelled leaders).
	// The grammar is fed the current statement's text from its start up to the
	// cursor — NOT the trimmed currentStmt — so trailing whitespace before the
	// cursor is preserved. That distinction matters: it is what tells the grammar
	// the word before the cursor is finished (offer the next clause) rather than
	// still being typed (the half-typed word it must drop).
	runesAll := []rune(sql)
	cur := max(stmtStart, min(cursorOffset, len(runesAll)))
	stmtPrefix := string(runesAll[stmtStart:cur])
	grammarExpected := GrammarExpectedAt(stmtPrefix, len(stmtPrefix))

	scripting := GetScriptingCompletions(sql, cursorOffset)
	tableRefs := ParseJoinTables(currentStmt)
	cteColumns := getCTEColumnsAtCursor(currentStmt)

	// Scan statements 0..currentIdx (inclusive) for USE DATABASE/SCHEMA context.
	var useCtx *UseContext
	runes := []rune(sql)
	scanEnd := currentIdx + 1
	if currentIdx < 0 {
		scanEnd = 0
	}
	for i := 0; i < scanEnd && i < len(ranges); i++ {
		r := ranges[i]
		end := r.EndOffset
		if end > len(runes) {
			end = len(runes)
		}
		stmtText := string(runes[r.StartOffset:end])
		refs := ParseJoinTables(stmtText)
		for _, ref := range refs {
			if ref.Name == "" { // USE statement ref (Name is always empty)
				if useCtx == nil {
					useCtx = &UseContext{}
				}
				if ref.DB != "" {
					useCtx.Database = ref.DB
				}
				if ref.Schema != "" {
					useCtx.Schema = ref.Schema
				}
			}
		}
	}

	return AutocompleteContext{
		StatementRanges: ranges,
		CurrentStmt:     currentStmt,
		CurrentStmtIdx:  currentIdx,
		Scripting:       scripting,
		TableRefs:       tableRefs,
		CTEColumns:      cteColumns,
		UseContext:      useCtx,
		GrammarExpected: grammarExpected,
	}
}

// getCTEColumnsAtCursor extracts CTE column projections from the given statement
// text, suitable for autocomplete suggestions. It uses the existing
// extractCTEProjections machinery with an empty global registry.
func getCTEColumnsAtCursor(stmtText string) []CTEColumnEntry {
	// FirstToken skips leading comments and matches whole tokens, so a leading
	// comment does not hide the WITH and an identifier like WITHDRAWALS does
	// not look like one.
	if sqltok.FirstToken(stmtText) != "WITH" {
		return nil
	}
	stripped := stripCommentsSQL(stmtText)

	projections := extractCTEProjections(stripped, make(map[string][]ColInfo))
	if len(projections) == 0 {
		return nil
	}

	// Convert map to sorted slice for deterministic output.
	entries := make([]CTEColumnEntry, 0, len(projections))
	for name, cols := range projections {
		entries = append(entries, CTEColumnEntry{Name: name, Cols: cols})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// ── Ref resolution & in-editor table defs ─────────────────────────────────

// ResolveTableRefs resolves unqualified/partially-qualified table references
// against the provided store objects, UseContext, and session context.
// Resolution order for each ref:
//  1. Already fully qualified (db+schema+name) → return as-is
//  2. Search storeObjects for matching TABLE/VIEW (case-insensitive)
//  3. Apply UseContext (editor-level USE DATABASE/SCHEMA)
//  4. Apply session context (live Snowflake connection)
//  5. If still incomplete → skip
//
// USE refs (Name == "") are skipped.
func ResolveTableRefs(
	refs []JoinTableRef,
	storeObjects []StoreObject,
	useCtx *UseContext,
	session *SessionContext,
) []ResolvedRef {
	var resolved []ResolvedRef
	for _, ref := range refs {
		// Skip USE refs (ParseJoinTables returns these with Name == "")
		if ref.Name == "" {
			continue
		}

		r := ResolvedRef{
			Alias:  ref.Alias,
			DB:     ref.DB,
			Schema: ref.Schema,
			Name:   ref.Name,
		}

		// 1. Already fully qualified
		if r.DB != "" && r.Schema != "" {
			resolved = append(resolved, r)
			continue
		}

		// 2. Search store objects
		found := false
		for _, o := range storeObjects {
			if !strings.EqualFold(o.Kind, "TABLE") && !strings.EqualFold(o.Kind, "VIEW") {
				continue
			}
			if !strings.EqualFold(o.Name, ref.Name) {
				continue
			}
			if ref.DB != "" && !strings.EqualFold(o.DB, ref.DB) {
				continue
			}
			if ref.Schema != "" && !strings.EqualFold(o.Schema, ref.Schema) {
				continue
			}
			r.DB = o.DB
			r.Schema = o.Schema
			r.Name = o.Name
			found = true
			break
		}
		if found {
			resolved = append(resolved, r)
			continue
		}

		// 3. Apply UseContext
		if useCtx != nil {
			if r.DB == "" && useCtx.Database != "" {
				r.DB = useCtx.Database
			}
			if r.Schema == "" && useCtx.Schema != "" {
				r.Schema = useCtx.Schema
			}
		}
		if r.DB != "" && r.Schema != "" {
			resolved = append(resolved, r)
			continue
		}

		// 4. Apply session context
		if session != nil {
			if r.DB == "" && session.Database != "" {
				r.DB = session.Database
			}
			if r.Schema == "" && session.Schema != "" {
				r.Schema = session.Schema
			}
		}
		if r.DB != "" && r.Schema != "" {
			resolved = append(resolved, r)
		}
		// 5. Still incomplete → skip
	}
	return resolved
}

// ExtractInEditorTableDefs scans all statements for CREATE TABLE definitions
// and extracts their column definitions. UseContext and session context are
// applied to qualify unqualified table names.
func ExtractInEditorTableDefs(
	sql string,
	stmtRanges []StatementRange,
	useCtx *UseContext,
	session *SessionContext,
) []InEditorTableDef {
	var defs []InEditorTableDef

	for _, r := range stmtRanges {
		raw := sqlStmt(sql, r)

		// Must be a CREATE TABLE (not CTAS/CLONE/LIKE)
		rawToks := sqltok.Tokenize(raw)
		rawSig := sigToks(rawToks)
		if !matchCreateTableGuard(rawSig, raw) {
			continue
		}

		nameStr, parenStart, ok := matchCreateTablePre(rawSig, raw)
		if !ok {
			continue
		}

		// Check for CTAS: if AS SELECT follows the column block or there is no column block
		colsRaw := extractBalancedBlock(raw, parenStart)
		if colsRaw == "" {
			continue
		}

		// Check if this is CTAS: the first significant token after the closing
		// paren is AS. Matching the token rather than a text prefix means a
		// comment between ')' and AS does not hide it.
		if kwAt(rawSig, raw, sigIndexAtOffset(rawSig, parenStart+len(colsRaw)), "AS") {
			continue
		}

		// Strip surrounding parens and parse columns
		if len(colsRaw) >= 2 {
			colsRaw = colsRaw[1 : len(colsRaw)-1]
		}
		columns := parseCreateTableColDefs(colsRaw, false)
		if len(columns) == 0 {
			continue
		}

		// Extract name parts
		parts := extractIdentParts(nameStr, false)
		if len(parts) == 0 {
			continue
		}

		var db, schema, name string
		switch len(parts) {
		case 3:
			db, schema, name = parts[0], parts[1], parts[2]
		case 2:
			schema, name = parts[0], parts[1]
		default:
			name = parts[0]
		}

		// Qualify using UseContext then session
		if db == "" && useCtx != nil && useCtx.Database != "" {
			db = useCtx.Database
		}
		if schema == "" && useCtx != nil && useCtx.Schema != "" {
			schema = useCtx.Schema
		}
		if db == "" && session != nil && session.Database != "" {
			db = session.Database
		}
		if schema == "" && session != nil && session.Schema != "" {
			schema = session.Schema
		}

		defs = append(defs, InEditorTableDef{
			DB:     db,
			Schema: schema,
			Name:   name,
			Cols:   columns,
		})
	}

	return defs
}

// ── ComputeGitLineDiff ─────────────────────────────────────────────────────

// ComputeGitLineDiff computes a line-level diff between HEAD lines and current
// lines using an LCS dynamic-programming approach. Returns 1-based line numbers
// for added, modified, and deleted regions. Returns empty slices if either
// input exceeds maxLines.
func ComputeGitLineDiff(headLines, currentLines []string, maxLines int) LineDiff {
	if len(headLines) > maxLines || len(currentLines) > maxLines {
		return LineDiff{Added: []int{}, Modified: []int{}, Deleted: []int{}}
	}

	H := len(headLines)
	C := len(currentLines)

	// DP LCS on line arrays.
	dp := make([][]int, H+1)
	for i := range dp {
		dp[i] = make([]int, C+1)
	}
	for i := H - 1; i >= 0; i-- {
		for j := C - 1; j >= 0; j-- {
			if headLines[i] == currentLines[j] {
				dp[i][j] = 1 + dp[i+1][j+1]
			} else if dp[i+1][j] > dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	// Backtrack to find diff operations.
	var added, deleted []int
	i, j := 0, 0
	for i < H || j < C {
		if i < H && j < C && headLines[i] == currentLines[j] {
			i++
			j++
		} else if j < C && (i >= H || dp[i+1][j] <= dp[i][j+1]) {
			added = append(added, j+1) // 1-based
			j++
		} else {
			// Head line i was deleted. Record deletion point as line j in current (1-based, min 1).
			pos := j
			if pos < 1 {
				pos = 1
			}
			deleted = append(deleted, pos)
			i++
		}
	}

	// Reclassify: a line in current that appears in both added and deleted
	// at the same position was in HEAD but changed — show as "modified".
	deletedSet := make(map[int]bool, len(deleted))
	for _, l := range deleted {
		deletedSet[l] = true
	}
	addedSet := make(map[int]bool, len(added))
	for _, l := range added {
		addedSet[l] = true
	}

	var modified []int
	for _, l := range added {
		if deletedSet[l] {
			modified = append(modified, l)
		}
	}

	filteredAdded := make([]int, 0, len(added))
	for _, l := range added {
		if !deletedSet[l] {
			filteredAdded = append(filteredAdded, l)
		}
	}
	filteredDeleted := make([]int, 0, len(deleted))
	for _, l := range deleted {
		if !addedSet[l] {
			filteredDeleted = append(filteredDeleted, l)
		}
	}

	if filteredAdded == nil {
		filteredAdded = []int{}
	}
	if modified == nil {
		modified = []int{}
	}
	if filteredDeleted == nil {
		filteredDeleted = []int{}
	}

	return LineDiff{Added: filteredAdded, Modified: modified, Deleted: filteredDeleted}
}

// ── autocomplete context detectors ─────────────────────────────────────────
//
// IsDatatypeContext, IsInJoinOnClause, and DetectUsingClause scan the sqltok
// significant-token stream of the current statement rather than matching keyword
// regexes over raw text. Tokenizing drops comments and string literals, so a
// keyword in a comment or a "::" inside a string no longer produces a false
// context, and the backwards scan is linear instead of regex backtracking over
// the whole buffer.

// currentStmtSig returns the significant tokens of the statement ending at the
// cursor: SignificantTokens(textToCursor) with everything up to and including the
// last statement-terminating ";" dropped. A ";" inside a string or comment is not
// a Semicolon token, so it never splits the statement.
func currentStmtSig(textToCursor string) []sqltok.Token {
	sig := sqltok.SignificantTokens(textToCursor)
	start := 0
	for i, t := range sig {
		if t.Kind == sqltok.Semicolon {
			start = i + 1
		}
	}
	return sig[start:]
}

// tokKeyword reports whether t is the keyword kw (case-insensitive).
func tokKeyword(src string, t sqltok.Token, kw string) bool {
	return t.Kind == sqltok.Keyword && strings.EqualFold(t.Text(src), kw)
}

// IsDatatypeContext returns true when the cursor position suggests a Snowflake
// data type name is expected — after ::, CAST(x AS, DECLARE varname, or
// CREATE/ALTER TABLE (..., col_name. lineUpToWord is unused (kept for the caller
// signature); detection now works off the tokenized current statement.
func IsDatatypeContext(textToCursor string, _ string) bool {
	sig := currentStmtSig(textToCursor)
	if len(sig) == 0 {
		return false
	}
	last := sig[len(sig)-1]

	// After a "::" cast operator.
	if last.Kind == sqltok.Operator && last.Text(textToCursor) == "::" {
		return true
	}

	// CAST(x AS / TRY_CAST(x AS — the "AS" must sit directly inside the (still
	// open) CAST paren: walk back to the enclosing "(" without crossing a ")",
	// then check the word before it is CAST/TRY_CAST.
	if tokKeyword(textToCursor, last, "AS") {
		for i := len(sig) - 2; i >= 0; i-- {
			if sig[i].Kind == sqltok.RParen {
				break
			}
			if sig[i].Kind == sqltok.LParen {
				if i > 0 {
					w := strings.ToUpper(sig[i-1].Text(textToCursor))
					if w == "CAST" || w == "TRY_CAST" {
						return true
					}
				}
				break
			}
		}
	}

	// The remaining contexts fire when the cursor sits right after a bare word —
	// the just-typed variable/column name, with the type coming next.
	if !last.Kind.IsIdentLike() {
		return false
	}

	// Scan the preceding tokens once for a DECLARE, a CREATE/ALTER, and the paren
	// depth at the cursor.
	prior := sig[:len(sig)-1]
	depth, hasDeclare, hasCreateAlter := 0, false, false
	for _, t := range prior {
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		case sqltok.Keyword:
			switch strings.ToUpper(t.Text(textToCursor)) {
			case "DECLARE":
				hasDeclare = true
			case "CREATE", "ALTER":
				hasCreateAlter = true
			}
		}
	}

	// DECLARE varname.
	if hasDeclare {
		return true
	}

	// CREATE/ALTER (... col — inside an unclosed paren, the word is the first
	// token of a column definition (immediately after "(" or ",").
	if hasCreateAlter && depth > 0 {
		prev := prior[len(prior)-1]
		if prev.Kind == sqltok.LParen || prev.Kind == sqltok.Comma {
			return true
		}
	}
	return false
}

// IsInJoinOnClause returns true when the cursor is inside a JOIN ... ON ...
// clause that has not been terminated by a subsequent keyword.
func IsInJoinOnClause(textToCursor string) bool {
	sig := currentStmtSig(textToCursor)

	// Last JOIN before the cursor.
	lastJoin := -1
	for i, t := range sig {
		if tokKeyword(textToCursor, t, "JOIN") {
			lastJoin = i
		}
	}
	if lastJoin < 0 {
		return false
	}

	// First ON after that JOIN.
	on := -1
	for i := lastJoin + 1; i < len(sig); i++ {
		if tokKeyword(textToCursor, sig[i], "ON") {
			on = i
			break
		}
	}
	if on < 0 {
		return false
	}

	// A terminator keyword after the ON closes the clause.
	for i := on + 1; i < len(sig); i++ {
		if sig[i].Kind != sqltok.Keyword {
			continue
		}
		switch strings.ToUpper(sig[i].Text(textToCursor)) {
		case "JOIN", "WHERE", "GROUP", "ORDER", "HAVING", "UNION", "INTERSECT", "EXCEPT":
			return false
		}
	}
	return true
}

// DetectUsingClause checks whether the cursor is inside a USING(...) clause.
// InUsing is true when right after "USING(" with no columns yet.
// IsPartial is true when after "USING(col1, " with at least one column listed.
func DetectUsingClause(textToCursor string) UsingClauseInfo {
	sig := currentStmtSig(textToCursor)

	// Last USING immediately followed by "(".
	open := -1
	for i := 0; i+1 < len(sig); i++ {
		if tokKeyword(textToCursor, sig[i], "USING") && sig[i+1].Kind == sqltok.LParen {
			open = i + 1
		}
	}
	if open < 0 {
		return UsingClauseInfo{}
	}

	rest := sig[open+1:]
	if len(rest) == 0 {
		return UsingClauseInfo{InUsing: true}
	}
	// Partial column list: (ident, ident, ...) ending with a trailing comma.
	if len(rest)%2 == 0 {
		partial := true
		for i, t := range rest {
			want := sqltok.Comma
			if i%2 == 0 {
				if !t.Kind.IsIdentLike() {
					partial = false
					break
				}
				continue
			}
			if t.Kind != want {
				partial = false
				break
			}
		}
		if partial {
			return UsingClauseInfo{IsPartial: true}
		}
	}
	return UsingClauseInfo{}
}

// ── GrammarExpectedAt ──────────────────────────────────────────────────────

// reGrammarKeyword matches a grammar expected-label that is a literal keyword or
// option word (FROM, TAG, DATA_RETENTION_TIME_IN_DAYS) — all-uppercase, as the
// grammar emits them. Token-kind names (Identifier, StringLit, …), the lowercase
// "identifier" / "column assignment" placeholders, and operators (=, ::) do not
// match, so they fall into the Kinds bucket.
var reGrammarKeyword = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// GrammarExpectedAt parses stmt up to localOffset (a byte offset within stmt)
// with the recursive-descent grammar and returns its classified "valid next" set
// — the keywords/token-kinds the grammar expects at the cursor. It returns nil
// when the statement's leading keyword is unmodelled (so grammar-driven
// completion stays leading-keyword-gated and unmodelled SQL is unaffected) or
// when the grammar has no expectation at that position (e.g. a free-form clause
// body the grammar consumes permissively).
func GrammarExpectedAt(stmt string, localOffset int) *GrammarExpectation {
	v := sqlgrammar.New(stmt)
	if !v.Recognized() {
		return nil
	}
	expected := v.ExpectedAt(localOffset)
	if len(expected) == 0 {
		return nil
	}
	exp := &GrammarExpectation{}
	for _, label := range expected {
		if label == "EOF" {
			continue // end-of-input sentinel — not a completion candidate
		}
		if reGrammarKeyword.MatchString(label) {
			exp.Keywords = append(exp.Keywords, label)
		} else {
			exp.Kinds = append(exp.Kinds, label)
		}
	}
	if len(exp.Keywords) == 0 && len(exp.Kinds) == 0 {
		return nil
	}
	return exp
}

// GetAutocompleteContextFull extends GetAutocompleteContext with ref resolution
// and in-editor CREATE TABLE column extraction, so the frontend completion
// provider becomes a thin wrapper.
func GetAutocompleteContextFull(req AutocompleteContextRequest) AutocompleteContext {
	ctx := GetAutocompleteContext(req.SQL, req.CursorOffset)
	ctx.ResolvedRefs = ResolveTableRefs(ctx.TableRefs, req.StoreObjects, ctx.UseContext, req.Session)
	ctx.InEditorTables = ExtractInEditorTableDefs(req.SQL, ctx.StatementRanges, ctx.UseContext, req.Session)

	// Compute context-detection fields from text-to-cursor.
	textToCursor := req.SQL
	runes := []rune(req.SQL)
	if req.CursorOffset >= 0 && req.CursorOffset <= len(runes) {
		textToCursor = string(runes[:req.CursorOffset])
	}
	ctx.IsDatatypeCtx = IsDatatypeContext(textToCursor, req.LineUpToWord)
	ctx.IsInJoinOnClause = IsInJoinOnClause(textToCursor)

	usingInfo := DetectUsingClause(textToCursor)
	if usingInfo.InUsing || usingInfo.IsPartial {
		ctx.UsingClause = &usingInfo
	}

	return ctx
}
