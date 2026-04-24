// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package sqleditor provides SQL analysis utilities that are executed in the
// Go backend to protect proprietary logic from frontend reverse-engineering.
// These functions mirror the TypeScript counterparts previously in
// sqlDiagnostics.ts and SqlEditor.tsx.
package sqleditor

import (
	"regexp"
	"sort"
	"strings"

	sf "thaw/internal/snowflake"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// DiagMarker represents a Monaco editor marker (error or warning).
type DiagMarker struct {
	StartLineNumber int    `json:"startLineNumber"`
	StartColumn     int    `json:"startColumn"`
	EndLineNumber   int    `json:"endLineNumber"`
	EndColumn       int    `json:"endColumn"`
	Message         string `json:"message"`
	Severity        int    `json:"severity"` // 8 = Error (red), 4 = Warning (yellow)
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

// TokenMatch is a located occurrence of a target token in a SQL string,
// as returned by FindTokenPositions.
type TokenMatch struct {
	Name   string `json:"name"`   // bare word text, or inner content of a double-quoted identifier
	Line   int    `json:"line"`   // 1-indexed line number
	Col    int    `json:"col"`    // 1-indexed start column (includes opening '"' for quoted tokens)
	EndCol int    `json:"endCol"` // 1-indexed end column, exclusive (includes closing '"' for quoted tokens)
	Quoted bool   `json:"quoted"` // true for double-quoted identifiers, false for bare words
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
	// Data loading
	"COPY": true, "PUT": true, "GET": true, "LIST": true, "REMOVE": true,
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

// JOIN clause stop keywords used to detect accidental alias capture.
var joinStopKW = map[string]bool{
	"ON": true, "WHERE": true, "SET": true, "GROUP": true, "ORDER": true,
	"HAVING": true, "LIMIT": true, "UNION": true, "EXCEPT": true,
	"INTERSECT": true, "CROSS": true, "INNER": true, "LEFT": true,
	"RIGHT": true, "FULL": true, "OUTER": true, "NATURAL": true, "JOIN": true,
	"SELECT": true, "WITH": true, "FROM": true,
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
	StartOffset int `json:"startOffset"` // rune offset of trimmed statement start
	EndOffset   int `json:"endOffset"`   // rune offset just past ';' (or end of string)
}

// GetStatementRanges splits sql into per-statement ranges by scanning
// character-by-character.  Semicolons inside string literals, quoted
// identifiers, block comments, line comments, and dollar-quoted blocks are
// correctly ignored.  No Snowflake connection is required.
func GetStatementRanges(sql string) []StatementRange {
	var ranges []StatementRange

	runes := []rune(sql)
	n := len(runes)

	line := 1 // current 1-indexed line number

	// Position of the current statement's trimmed start (-1 = not yet started).
	stmtStartLine := -1
	stmtStartOffset := 0
	inStmt := false

	// Record the start of a new statement at rune index i.
	// Called once per statement on the first non-whitespace, non-comment char.
	startStmt := func(i int) {
		if !inStmt {
			stmtStartLine = line
			stmtStartOffset = i
			inStmt = true
		}
	}

	// Emit the current statement ending at rune index endOffset (exclusive).
	emit := func(endLine, endOffset int) {
		if inStmt {
			ranges = append(ranges, StatementRange{
				StartLine:   stmtStartLine,
				EndLine:     endLine,
				StartOffset: stmtStartOffset,
				EndOffset:   endOffset,
			})
			inStmt = false
			stmtStartLine = -1
		}
	}

	i := 0
	for i < n {
		ch := runes[i]

		// ── Newline ────────────────────────────────────────────────────────
		if ch == '\n' {
			line++
			i++
			continue
		}

		// ── Whitespace ────────────────────────────────────────────────────
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\u00a0' {
			i++
			continue
		}

		// ── Line comment -- ───────────────────────────────────────────────
		if ch == '-' && i+1 < n && runes[i+1] == '-' {
			i += 2
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}

		// ── Block comment /* */ ───────────────────────────────────────────
		if ch == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i < n {
				if runes[i] == '\n' {
					line++
					i++
				} else if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					break
				} else {
					i++
				}
			}
			continue
		}

		// All remaining chars belong to a statement; record its start.
		startStmt(i)

		// ── Single-quoted string '...' ────────────────────────────────────
		if ch == '\'' {
			i++
			for i < n {
				if runes[i] == '\n' {
					line++
					i++
				} else if runes[i] == '\'' && i+1 < n && runes[i+1] == '\'' {
					i += 2
				} else if runes[i] == '\'' {
					i++
					break
				} else {
					i++
				}
			}
			continue
		}

		// ── Double-quoted identifier "..." ────────────────────────────────
		if ch == '"' {
			i++
			for i < n {
				if runes[i] == '\n' {
					line++
					i++
				} else if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
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

		// ── Dollar-quoted block $tag$...$tag$ ─────────────────────────────
		if ch == '$' {
			tag := extractDollarTag(runes, i)
			if tag != "" {
				tagRunes := []rune(tag)
				tagLen := len(tagRunes)
				i += tagLen // skip opening tag
				// Scan for the matching closing tag.
				for i < n {
					if runes[i] == '\n' {
						line++
						i++
					} else if runes[i] == tagRunes[0] && i+tagLen <= n {
						match := true
						for k := 1; k < tagLen; k++ {
							if runes[i+k] != tagRunes[k] {
								match = false
								break
							}
						}
						if match {
							i += tagLen
							break
						}
						i++
					} else {
						i++
					}
				}
				continue
			}
		}

		// ── Semicolon: end of statement ───────────────────────────────────
		if ch == ';' {
			emit(line, i+1)
			i++
			continue
		}

		i++
	}

	// Emit trailing statement with no semicolon.
	emit(line, n)

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

// ── FindTokenPositions ────────────────────────────────────────────────────────

// FindTokenPositions walks sql and returns the line/column positions of:
//   - every unquoted bare word (not preceded by '.', not followed by '(')
//     whose upper-cased form is in bareTargets
//   - every double-quoted identifier whose upper-cased inner name is in quotedTargets
//
// Single-quoted strings ('...'), line comments (--), and block comments (/* */)
// are transparently skipped so tokens inside them are never reported.
// Line and Col are 1-indexed, matching Monaco editor coordinates.
func FindTokenPositions(sql string, bareTargets []string, quotedTargets []string) []TokenMatch {
	bareSet := make(map[string]struct{}, len(bareTargets))
	for _, t := range bareTargets {
		bareSet[strings.ToUpper(t)] = struct{}{}
	}
	quotedSet := make(map[string]struct{}, len(quotedTargets))
	for _, t := range quotedTargets {
		quotedSet[strings.ToUpper(t)] = struct{}{}
	}

	var results []TokenMatch
	runes := []rune(sql)
	n := len(runes)
	line, col := 1, 1
	i := 0

	for i < n {
		r := runes[i]

		// Newline
		if r == '\n' {
			line++
			col = 1
			i++
			continue
		}

		// Line comment: --
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			i += 2
			col += 2
			for i < n && runes[i] != '\n' {
				i++
				col++
			}
			continue
		}

		// Block comment: /* */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
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

		// Single-quoted string: '...'
		if r == '\'' {
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
			continue
		}

		// Double-quoted identifier: "..."
		if r == '"' {
			startLine, startCol := line, col
			i++
			col++
			var name []rune
			closed := false
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
					name = append(name, '\n')
				} else if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
					name = append(name, '"')
					i += 2
					col += 2
				} else if runes[i] == '"' {
					i++
					col++
					closed = true
					break
				} else {
					name = append(name, runes[i])
					i++
					col++
				}
			}
			if closed && len(quotedSet) > 0 {
				nameStr := string(name)
				if _, ok := quotedSet[strings.ToUpper(nameStr)]; ok {
					results = append(results, TokenMatch{
						Name: nameStr, Line: startLine, Col: startCol, EndCol: col, Quoted: true,
					})
				}
			}
			continue
		}

		// Bare word: [a-zA-Z_]\w*
		if isLetterOrUnderscore(r) {
			wLine, wCol, wStart := line, col, i
			for i < n && isWordRune(runes[i]) {
				i++
				col++
			}
			if len(bareSet) > 0 {
				word := string(runes[wStart:i])
				prevCh := rune(0)
				if wStart > 0 {
					prevCh = runes[wStart-1]
				}
				nextCh := rune(0)
				if i < n {
					nextCh = runes[i]
				}
				if prevCh != '.' && nextCh != '(' {
					if _, ok := bareSet[strings.ToUpper(word)]; ok {
						results = append(results, TokenMatch{
							Name: word, Line: wLine, Col: wCol, EndCol: col, Quoted: false,
						})
					}
				}
			}
			continue
		}

		i++
		col++
	}
	return results
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

// sqlFormatterKeywords is the complete set of SQL reserved words for the
// token-level casing pass (ApplyCasing).  Tokens in this set receive
// keywordCase treatment unless they are also in builtinFunctions.
var sqlFormatterKeywords = map[string]bool{
	"ADD": true, "ALL": true, "ALTER": true, "AND": true, "ANY": true,
	"AS": true, "ASC": true, "AT": true, "BEFORE": true, "BETWEEN": true,
	"BY": true, "CALL": true, "CASCADE": true, "CASE": true, "CAST": true,
	"CHANGES": true, "CLUSTER": true, "COLUMN": true, "COMMENT": true,
	"COMMIT": true, "CONNECT": true, "CONSTRAINT": true, "COPY": true,
	"CREATE": true, "CROSS": true, "CURRENT": true, "CURRENT_DATE": true,
	"CURRENT_ROLE": true, "CURRENT_SCHEMA": true, "CURRENT_TIME": true,
	"CURRENT_TIMESTAMP": true, "CURRENT_USER": true, "CURRENT_WAREHOUSE": true,
	"DATABASE": true, "DEFAULT": true, "DELETE": true, "DESC": true,
	"DESCRIBE": true, "DISTINCT": true, "DROP": true, "ELSE": true,
	"END": true, "EXCEPT": true, "EXECUTE": true, "EXISTS": true,
	"EXPLAIN": true, "EXTRACT": true, "FALSE": true, "FILE": true,
	"FIRST": true, "FLATTEN": true, "FOLLOWING": true, "FOR": true,
	"FORCE": true, "FOREIGN": true, "FROM": true, "FULL": true,
	"FUNCTION": true, "DEFINE": true, "GRANT": true, "GROUP": true,
	"GROUPING": true, "HAVING": true, "IF": true, "ILIKE": true,
	"IN": true, "INDEX": true, "INNER": true, "INSERT": true,
	"INTERSECT": true, "INTO": true, "IS": true, "JOIN": true,
	"KEY": true, "LAST": true, "LATERAL": true, "LEFT": true,
	"LIKE": true, "LIMIT": true, "MATCH_RECOGNIZE": true, "MEASURES": true,
	"MERGE": true, "MINUS": true, "NATURAL": true, "NOT": true,
	"NULL": true, "NULLS": true, "OF": true, "OFFSET": true,
	"ON": true, "OR": true, "ORDER": true, "OUTER": true, "OVER": true,
	"OVERWRITE": true, "PARTITION": true, "PATTERN": true, "PIPE": true,
	"PRECEDING": true, "PRIMARY": true, "PROCEDURE": true, "PURGE": true,
	"QUALIFY": true, "RANGE": true, "RECURSIVE": true, "REFERENCES": true,
	"REPLACE": true, "RESTRICT": true, "REVOKE": true, "RIGHT": true,
	"ROLLBACK": true, "ROW": true, "ROWS": true, "SAMPLE": true,
	"SCHEMA": true, "SELECT": true, "SEMI": true, "SEQUENCE": true,
	"SET": true, "SHOW": true, "SOME": true, "STAGE": true,
	"START": true, "STREAM": true, "TABLE": true, "TABLESAMPLE": true,
	"TASK": true, "THEN": true, "TO": true, "TOP": true,
	"TRANSACTION": true, "TRUE": true, "TRUNCATE": true, "UNBOUNDED": true,
	"UNION": true, "UNIQUE": true, "UPDATE": true, "USING": true,
	"VALUES": true, "VIEW": true, "VOLATILE": true, "WAREHOUSE": true,
	"WHEN": true, "WHERE": true, "WINDOW": true, "WITH": true,
	"WITHIN": true, "WITHOUT": true,
	// Snowflake-specific
	"ANTI": true, "ASOF": true, "CLONE": true, "CRON": true,
	"DYNAMIC": true, "ENABLE": true, "EXTERNAL": true, "FINALIZE": true,
	"FORMAT": true, "ICEBERG": true, "MASKING": true, "NETWORK": true,
	"NOTIFY": true, "POLICY": true, "PROJECTION": true, "RECOVER": true,
	"REPLICATION": true, "RESUME": true, "ROLE": true, "SCHEDULE": true,
	"SECURE": true, "SHARE": true, "SUSPEND": true, "TABULAR": true,
	"TRANSIENT": true, "TRIGGER": true, "UNDROP": true,
	// Additional Snowflake / Standard keywords
	"IMMEDIATE": true, "NUMBER": true, "VARCHAR": true, "BOOLEAN": true, "DATE": true, "STRING": true, "FLOAT": true, "INTEGER": true, "INT": true,
	"DOUBLE": true, "REAL": true, "DECIMAL": true, "NUMERIC": true, "BIGINT": true, "SMALLINT": true, "TINYINT": true, "BYTEINT": true,
	"TIMESTAMP_NTZ": true, "TIMESTAMP_LTZ": true, "TIMESTAMP_TZ": true, "VARIANT": true, "OBJECT": true, "ARRAY": true,
	"GEOGRAPHY": true, "GEOMETRY": true, "INPUT": true,
	// Snowflake Scripting keywords
	"DECLARE": true, "BEGIN": true, "LET": true, "RETURN": true, "VAR": true, "WHILE": true, "REPEAT": true, "LOOP": true, "EXCEPTION": true,
}

// builtinFunctions is the set of Snowflake built-in function names.
// A token that is in both sqlFormatterKeywords and builtinFunctions and is
// followed by "(" receives functionCase treatment (not keywordCase).
var builtinFunctions = map[string]bool{
	"ABS": true, "ACOS": true, "ACOSH": true, "ADD_MONTHS": true,
	"ANY_VALUE": true, "APPROX_COUNT_DISTINCT": true, "APPROX_PERCENTILE": true,
	"APPROX_TOP_K": true, "ARRAY_AGG": true, "ARRAY_APPEND": true,
	"ARRAY_CAT": true, "ARRAY_COMPACT": true, "ARRAY_CONSTRUCT": true,
	"ARRAY_CONTAINS": true, "ARRAY_DISTINCT": true, "ARRAY_EXCEPT": true,
	"ARRAY_FLATTEN": true, "ARRAY_GENERATE_RANGE": true, "ARRAY_INSERT": true,
	"ARRAY_INTERSECTION": true, "ARRAY_MAX": true, "ARRAY_MIN": true,
	"ARRAY_PREPEND": true, "ARRAY_REMOVE": true, "ARRAY_REMOVE_AT": true,
	"ARRAY_SIZE": true, "ARRAY_SLICE": true, "ARRAY_SORT": true,
	"ARRAY_TO_STRING": true, "ARRAY_UNION_AGG": true, "ARRAY_UNIQUE_AGG": true,
	"AS_ARRAY": true, "AS_BINARY": true, "AS_BOOLEAN": true, "AS_CHAR": true,
	"AS_DATE": true, "AS_DECIMAL": true, "AS_DOUBLE": true, "AS_INTEGER": true,
	"AS_NUMBER": true, "AS_OBJECT": true, "AS_REAL": true, "AS_TIME": true,
	"AS_TIMESTAMP_LTZ": true, "AS_TIMESTAMP_NTZ": true, "AS_TIMESTAMP_TZ": true,
	"AS_TINYINT": true, "AS_VARCHAR": true, "ASIN": true, "ASINH": true,
	"ATAN": true, "ATAN2": true, "ATANH": true, "AVG": true,
	"BASE64_DECODE_STRING": true, "BASE64_ENCODE": true, "BITNOT": true,
	"BITSHIFTLEFT": true, "BITSHIFTRIGHT": true, "BITAND": true,
	"BITOR": true, "BITXOR": true, "BOOLAND": true, "BOOLAND_AGG": true,
	"BOOLNOT": true, "BOOLOR": true, "BOOLOR_AGG": true, "BOOLXOR": true,
	"BOOLXOR_AGG": true,
	"CASE":        true, "CAST": true, "CBRT": true, "CEIL": true, "CEILING": true,
	"CHARINDEX": true, "CHR": true, "CHAR": true, "COALESCE": true,
	"COLLATE": true, "COLLATION": true, "COMPRESS": true, "CONCAT": true,
	"CONCAT_WS": true, "CONDITIONAL_CHANGE_EVENT": true,
	"CONDITIONAL_TRUE_EVENT": true, "CONTAINS": true, "CONVERT_TIMEZONE": true,
	"COS": true, "COSH": true, "COUNT": true, "COUNT_IF": true,
	"COVAR_POP": true, "COVAR_SAMP": true, "CUME_DIST": true,
	"DATE_FROM_PARTS": true, "DATE_PART": true, "DATE_TRUNC": true,
	"DATEADD": true, "DATEDIFF": true, "DAYNAME": true, "DAYOFMONTH": true,
	"DAYOFWEEK": true, "DAYOFWEEKISO": true, "DAYOFYEAR": true,
	"DECODE": true, "DECOMPRESS": true, "DENSE_RANK": true, "DIV0": true,
	"DIV0NULL":     true,
	"EDITDISTANCE": true, "ENDSWITH": true, "EQUAL_NULL": true, "EXP": true,
	"FIRST_VALUE": true, "FLATTEN": true, "FLOOR": true, "FORMAT_DATE": true,
	"FORMAT_NUMBER": true,
	"GENERATOR":     true, "GET": true, "GET_ABSOLUTE_PATH": true, "GET_DDL": true,
	"GET_PATH": true, "GET_PRESIGNED_URL": true, "GET_STAGE_LOCATION": true,
	"GETBIT": true, "GREATEST": true, "GROUPING": true, "GROUPING_ID": true,
	"HASH": true, "HASH_AGG": true, "HAVERSINE": true,
	"HEX_DECODE_BINARY": true, "HEX_DECODE_STRING": true, "HEX_ENCODE": true,
	"HOUR": true, "HOURS": true,
	"IFF": true, "IFNULL": true, "IN": true, "INITCAP": true,
	"INSERT": true, "IS_ARRAY": true, "IS_BINARY": true, "IS_BOOLEAN": true,
	"IS_CHAR": true, "IS_DATE": true, "IS_DATE_VALUE": true,
	"IS_DECIMAL": true, "IS_DOUBLE": true, "IS_GRANTED_TO_INVOKER_ROLE": true,
	"IS_INTEGER": true, "IS_NULL_VALUE": true, "IS_OBJECT": true,
	"IS_REAL": true, "IS_TIME": true, "IS_TIMESTAMP_LTZ": true,
	"IS_TIMESTAMP_NTZ": true, "IS_TIMESTAMP_TZ": true, "IS_VARCHAR": true,
	"JAROWINKLER_SIMILARITY": true, "JSON_EXTRACT_PATH_TEXT": true,
	"KURTOSIS": true, "LAG": true, "LAST_DAY": true, "LAST_VALUE": true,
	"LEAD": true, "LEAST": true, "LEFT": true, "LENGTH": true, "LEN": true,
	"LISTAGG": true, "LN": true, "LOG": true, "LOWER": true, "LPAD": true,
	"LTRIM": true,
	"MAX":   true, "MAX_BY": true, "MEDIAN": true, "MIN": true, "MIN_BY": true,
	"MINUTE": true, "MINUTES": true, "MOD": true, "MODE": true,
	"MONTH": true, "MONTHNAME": true, "MONTHS_BETWEEN": true,
	"NORMAL": true, "NTH_VALUE": true, "NTILE": true, "NULLIF": true,
	"NULLIFZERO": true, "NVL": true, "NVL2": true,
	"OBJECT_AGG": true, "OBJECT_CONSTRUCT": true,
	"OBJECT_CONSTRUCT_KEEP_NULL": true, "OBJECT_DELETE": true,
	"OBJECT_INSERT": true, "OBJECT_KEYS": true, "OBJECT_PICK": true,
	"PARSE_IP": true, "PARSE_JSON": true, "PARSE_URL": true,
	"PARSE_XML": true, "PERCENT_RANK": true, "PERCENTILE_CONT": true,
	"PERCENTILE_DISC": true, "PI": true, "POSITION": true, "POW": true,
	"POWER":   true,
	"RANDSTR": true, "RANDOM": true, "RANK": true, "RATIO_TO_REPORT": true,
	"REGEXP": true, "REGEXP_COUNT": true, "REGEXP_EXTRACT": true,
	"REGEXP_EXTRACT_ALL": true, "REGEXP_INSTR": true, "REGEXP_LIKE": true,
	"REGEXP_REPLACE": true, "REGEXP_SUBSTR": true, "REPEAT": true,
	"REPLACE": true, "REVERSE": true, "RIGHT": true, "ROUND": true,
	"ROW_NUMBER": true, "RPAD": true, "RTRIM": true,
	"SECOND": true, "SECONDS": true, "SHA1": true, "SHA1_BINARY": true,
	"SHA1_HEX": true, "SHA2": true, "SHA2_BINARY": true, "SHA2_HEX": true,
	"SIGN": true, "SIN": true, "SINH": true, "SKEW": true,
	"SOUNDEX": true, "SPACE": true, "SPLIT": true, "SPLIT_PART": true,
	"SPLIT_TO_TABLE": true, "SQL_VARIANT_PROPERTY": true, "SQRT": true,
	"SQUARE": true, "STARTSWITH": true, "STDDEV": true, "STDDEV_POP": true,
	"STDDEV_SAMP": true, "STRIP_NULL_VALUE": true, "STRTOK": true,
	"STRTOK_SPLIT_TO_TABLE": true, "STRTOK_TO_ARRAY": true,
	"SUBSTR": true, "SUBSTRING": true, "SUM": true,
	"SYSTEM$ABORT_TRANSACTION": true, "SYSTEM$CANCEL_ALL_QUERIES": true,
	"SYSTEM$CANCEL_QUERY": true, "SYSTEM$CLUSTERING_DEPTH": true,
	"SYSTEM$CLUSTERING_INFORMATION":       true,
	"SYSTEM$GET_PREDECESSOR_RETURN_VALUE": true,
	"SYSTEM$STREAM_GET_TABLE_TIMESTAMP":   true, "SYSTEM$STREAM_HAS_DATA": true,
	"SYSTEM$TASK_DEPENDENTS_ENABLE": true, "SYSTEM$TYPEOF": true,
	"SYSTEM$WAIT": true,
	"TAN":         true, "TANH": true, "TIME_FROM_PARTS": true, "TIMEADD": true,
	"TIMEDIFF": true, "TIMESTAMPADD": true, "TIMESTAMPDIFF": true,
	"TIMESTAMP_FROM_PARTS": true, "TIMESTAMP_LTZ_FROM_PARTS": true,
	"TIMESTAMP_NTZ_FROM_PARTS": true, "TIMESTAMP_TZ_FROM_PARTS": true,
	"TO_ARRAY": true, "TO_BINARY": true, "TO_BOOLEAN": true,
	"TO_CHAR": true, "TO_DATE": true, "TO_DECIMAL": true, "TO_DOUBLE": true,
	"TO_GEOGRAPHY": true, "TO_GEOMETRY": true, "TO_JSON": true,
	"TO_NUMBER": true, "TO_OBJECT": true, "TO_REAL": true,
	"TO_TIME": true, "TO_TIMESTAMP": true, "TO_TIMESTAMP_LTZ": true,
	"TO_TIMESTAMP_NTZ": true, "TO_TIMESTAMP_TZ": true, "TO_VARIANT": true,
	"TO_VARCHAR": true, "TO_XML": true, "TRANSLATE": true, "TRIM": true,
	"TRUNCATE": true, "TRUNC": true, "TRY_BASE64_DECODE_BINARY": true,
	"TRY_BASE64_DECODE_STRING": true, "TRY_CAST": true,
	"TRY_HEX_DECODE_BINARY": true, "TRY_HEX_DECODE_STRING": true,
	"TRY_PARSE_JSON": true, "TRY_TO_BINARY": true, "TRY_TO_BOOLEAN": true,
	"TRY_TO_DATE": true, "TRY_TO_DECIMAL": true, "TRY_TO_DOUBLE": true,
	"TRY_TO_NUMBER": true, "TRY_TO_TIME": true, "TRY_TO_TIMESTAMP": true,
	"TRY_TO_TIMESTAMP_LTZ": true, "TRY_TO_TIMESTAMP_NTZ": true,
	"TRY_TO_TIMESTAMP_TZ": true, "TYPEOF": true,
	"UNIFORM": true, "UPPER": true, "UNISTR": true,
	"VAR_POP": true, "VAR_SAMP": true, "VARIANCE": true,
	"VARIANCE_POP": true, "VARIANCE_SAMP": true,
	"WEEK": true, "WEEKISO": true, "WEEKOFYEAR": true,
	"XMLGET": true, "YEAR": true, "YEAROFWEEK": true,
	"YEAROFWEEKISO": true, "ZEROIFNULL": true,
}

// ApplyCasing walks a formatted SQL string token by token and applies
// per-role casing preferences.  Double-quoted identifiers, single-quoted
// strings, dollar-quoted strings, and comments are passed through unchanged.
// keywordCase: "UPPER" | "lower" | "Title" | "Preserve"
// identifierCase: "Preserve" | "UPPER" | "lower"
// functionCase: "UPPER" | "lower"
func ApplyCasing(sql, keywordCase, identifierCase, functionCase string) string {
	if sql == "" {
		return sql
	}
	runes := []rune(sql)
	n := len(runes)
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

	i := 0
	for i < n {
		ch := runes[i]

		// ── Double-quoted identifier — pass through unchanged ──────────────────
		if ch == '"' {
			j := i + 1
			for j < n {
				if runes[j] == '"' {
					if j+1 < n && runes[j+1] == '"' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			sb.WriteString(string(runes[i:j]))
			i = j
			continue
		}

		// ── Single-quoted string — pass through unchanged ──────────────────────
		if ch == '\'' {
			j := i + 1
			for j < n {
				if runes[j] == '\'' {
					if j+1 < n && runes[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			sb.WriteString(string(runes[i:j]))
			i = j
			continue
		}

		// ── Dollar-quoted string — recursively apply casing unless tag is $query$ ──
		if ch == '$' {
			tagEnd := i + 1
			for tagEnd < n && runes[tagEnd] != '$' && runes[tagEnd] != '\n' {
				tagEnd++
			}
			if tagEnd < n && runes[tagEnd] == '$' {
				tag := string(runes[i : tagEnd+1]) // e.g. "$$" or "$body$"
				rest := string(runes[tagEnd+1:])
				closeByteIdx := strings.Index(rest, tag)
				if closeByteIdx >= 0 {
					innerStart := tagEnd + 1
					closeRuneOff := len([]rune(rest[:closeByteIdx]))
					innerEnd := innerStart + closeRuneOff
					innerSql := string(runes[innerStart:innerEnd])

					sb.WriteString(tag)
					tagUpper := strings.ToUpper(tag)
					if tagUpper == "$QUERY$" {
						// Pass through $query$ blocks unchanged (likely contains a query string literal)
						sb.WriteString(innerSql)
					} else {
						// Recursively process scripting bodies ($$, $body$, etc.)
						sb.WriteString(ApplyCasing(innerSql, keywordCase, identifierCase, functionCase))
					}

					sb.WriteString(tag)

					i = innerEnd + len([]rune(tag))
					continue
				}
			}
			// Not a valid dollar-quoted string — pass the '$' through.
			sb.WriteRune(ch)
			i++
			continue
		}

		// ── Line comment -- … newline — pass through unchanged ─────────────────
		if ch == '-' && i+1 < n && runes[i+1] == '-' {
			j := i
			for j < n && runes[j] != '\n' {
				j++
			}
			if j < n {
				j++ // include the newline
			}
			sb.WriteString(string(runes[i:j]))
			i = j
			continue
		}

		// ── Block comment /* … */ — pass through unchanged ─────────────────────
		if ch == '/' && i+1 < n && runes[i+1] == '*' {
			j := i + 2
			for j+1 < n && !(runes[j] == '*' && runes[j+1] == '/') {
				j++
			}
			if j+1 < n {
				j += 2 // skip past */
			} else {
				j = n // unclosed comment — consume to end
			}
			sb.WriteString(string(runes[i:j]))
			i = j
			continue
		}

		// ── Word token (identifier / keyword / function name) ──────────────────
		if isAlpha(ch) {
			j := i + 1
			for j < n && (isWordChar(runes[j]) || runes[j] == '$') {
				j++
			}
			word := string(runes[i:j])
			upper := strings.ToUpper(word)

			// Peek past whitespace to determine if this is a function call.
			k := j
			for k < n && (runes[k] == ' ' || runes[k] == '\t' || runes[k] == '\n' || runes[k] == '\r') {
				k++
			}
			isCall := k < n && runes[k] == '('

			var result string
			if isCall {
				// Keywords that use '(' structurally (OVER, IN, …) keep keyword casing.
				// Built-in functions and UDFs get function casing.
				if sqlFormatterKeywords[upper] && !builtinFunctions[upper] {
					result = applyCase(word, keywordCase)
				} else {
					result = applyCase(word, functionCase)
				}
			} else if sqlFormatterKeywords[upper] {
				result = applyCase(word, keywordCase)
			} else {
				result = applyCase(word, identifierCase)
			}

			// For function tokens (not pure keyword constructs), strip the space
			// sql-formatter inserted before '(' so e.g. "COUNT (" → "COUNT(".
			isFunctionToken := isCall && (!sqlFormatterKeywords[upper] || builtinFunctions[upper])
			sb.WriteString(result)
			if isFunctionToken {
				i = k // advance past the whitespace before '('
			} else {
				i = j
			}
			continue
		}

		// Numbers, operators, whitespace, punctuation — pass through unchanged.
		sb.WriteRune(ch)
		i++
	}

	return sb.String()
}

// ValidateSyntax is a character-by-character Snowflake SQL tokenizer that catches
// structural errors:
//   - Unclosed single-quoted strings
//   - Unclosed double-quoted identifiers
//   - Unclosed dollar-quoted strings
//   - Unclosed block comments
//   - Unmatched / extra closing parens and brackets
//   - Unclosed opening parens and brackets
//   - Missing ':' in scripting assignments (var = expr → var := expr)
func ValidateSyntax(sql string) []DiagMarker {
	var markers []DiagMarker

	runes := []rune(sql)
	n := len(runes)

	line, col := 1, 1
	atStmtStart := true
	atScriptStmtStart := false
	inDeclareBlock := false

	var parenStack []parenEntry
	var dollarStack []string
	declaredVars := map[string]bool{}

	addError := func(msg string, sl, sc, el, ec int) {
		markers = append(markers, DiagMarker{sl, sc, el, ec, msg, 8})
	}

	i := 0
	for i < n {
		ch := runes[i]

		// Newline
		if ch == '\n' {
			line++
			col = 1
			i++
			continue
		}

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\u00a0' {
			i++
			col++
			continue
		}

		// Line comment --
		if ch == '-' && i+1 < n && runes[i+1] == '-' {
			i += 2
			col += 2
			for i < n && runes[i] != '\n' {
				i++
				col++
			}
			continue
		}

		// Block comment /* */
		if ch == '/' && i+1 < n && runes[i+1] == '*' {
			openLine, openCol := line, col
			i += 2
			col += 2
			closed := false
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
				} else if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					col += 2
					closed = true
					break
				} else {
					i++
					col++
				}
			}
			if !closed {
				addError("Unclosed block comment", openLine, openCol, openLine, openCol+2)
			}
			continue
		}

		// Single-quoted string '...' ('' escapes a literal quote)
		if ch == '\'' {
			openLine, openCol := line, col
			i++
			col++
			closed := false
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
					closed = true
					break
				} else {
					i++
					col++
				}
			}
			if !closed {
				addError("Unclosed string literal", openLine, openCol, openLine, openCol+1)
			}
			atStmtStart = false
			atScriptStmtStart = false
			continue
		}

		// Double-quoted identifier "..." ("" escapes a literal quote)
		if ch == '"' {
			openLine, openCol := line, col
			i++
			col++
			closed := false
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
				} else if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
					i += 2
					col += 2
				} else if runes[i] == '"' {
					i++
					col++
					closed = true
					break
				} else {
					i++
					col++
				}
			}
			if !closed {
				addError("Unclosed quoted identifier", openLine, openCol, openLine, openCol+1)
			}

			// In script context after a statement start, look for bare '=' (should be ':=')
			if len(dollarStack) > 0 && atScriptStmtStart && closed {
				j, jCol := i, col
				for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r' || runes[j] == '\u00a0') {
					if runes[j] == '\n' {
						break
					}
					j++
					jCol++
				}
				if j < n && runes[j] == '=' {
					prev := rune(0)
					if j > 0 {
						prev = runes[j-1]
					}
					next := rune(0)
					if j+1 < n {
						next = runes[j+1]
					}
					if prev != ':' && prev != '<' && prev != '>' && prev != '!' && next != '=' {
						addError("Expected ':=' for assignment", line, jCol, line, jCol+1)
					}
				}
			}

			atStmtStart = false
			atScriptStmtStart = false
			continue
		}

		// Dollar-quoted marker $tag$...$tag$
		if ch == '$' {
			tag := extractDollarTag(runes, i)
			if tag != "" {
				if len(dollarStack) > 0 && dollarStack[len(dollarStack)-1] == tag {
					dollarStack = dollarStack[:len(dollarStack)-1]
					atScriptStmtStart = false
					atStmtStart = true // After a $$ block, we are ready for the next SQL statement
					inDeclareBlock = false
					declaredVars = map[string]bool{}
				} else {
					dollarStack = append(dollarStack, tag)
					atScriptStmtStart = true
					atStmtStart = false // Inside $$, we are in scripting context
					declaredVars = map[string]bool{}
				}
				i += len([]rune(tag))
				col += len([]rune(tag))
				continue
			}
		}

		// Opening paren or bracket
		if ch == '(' || ch == '[' {
			parenStack = append(parenStack, parenEntry{string(ch), line, col})
			i++
			col++
			atStmtStart = false
			atScriptStmtStart = false
			continue
		}

		// Closing paren or bracket
		if ch == ')' || ch == ']' {
			expected := "("
			if ch == ']' {
				expected = "["
			}
			if len(parenStack) == 0 || parenStack[len(parenStack)-1].char != expected {
				addError("Unmatched '"+string(ch)+"'", line, col, line, col+1)
			} else {
				parenStack = parenStack[:len(parenStack)-1]
			}
			i++
			col++
			atStmtStart = false
			atScriptStmtStart = false
			continue
		}

		// Semicolon marks end of statement
		if ch == ';' {
			if len(dollarStack) == 0 {
				atStmtStart = true
			} else {
				atScriptStmtStart = true
			}
			i++
			col++
			continue
		}

		// Word token at a statement start position
		if (atStmtStart || atScriptStmtStart) && isAlpha(ch) {
			wordLine, wordCol := line, col
			wordStart := i
			for i < n && isWordChar(runes[i]) {
				i++
				col++
			}
			wordRaw := string(runes[wordStart:i])
			word := strings.ToUpper(wordRaw)

			if atStmtStart {
				atStmtStart = false
				if !sqlStmtKeywords[word] {
					addError("Unexpected token '"+wordRaw+"'",
						wordLine, wordCol, wordLine, wordCol+len(wordRaw))
				}
			} else if atScriptStmtStart {
				atScriptStmtStart = false

				switch word {
				case "DECLARE":
					inDeclareBlock = true
					atScriptStmtStart = true
				case "BEGIN":
					inDeclareBlock = false
					atScriptStmtStart = true
				case "THEN", "ELSE", "DO", "EXCEPTION":
					atScriptStmtStart = true
				case "RETURN", "FOR":
					// Peek ahead for variable usage
					j := i
					jCol := col
					// Skip whitespace/newlines
					for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
						runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
						if runes[j] == '\n' {
							line++
							jCol = 1
						} else {
							jCol++
						}
						j++
					}
					if j < n && isAlpha(runes[j]) {
						varStart := j
						for j < n && isWordChar(runes[j]) {
							j++
							jCol++
						}
						varNameRaw := string(runes[varStart:j])
						varName := strings.ToUpper(varNameRaw)

						if word == "FOR" {
							// FOR record IN cursor DO
							declaredVars[varName] = true
							// Skip IN
							for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
								runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
								if runes[j] == '\n' {
									line++
									jCol = 1
								} else {
									jCol++
								}
								j++
							}
							if j+2 < n && strings.ToUpper(string(runes[j:j+2])) == "IN" {
								j += 2
								jCol += 2
								// Skip whitespace to cursor name
								for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
									runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
									if runes[j] == '\n' {
										line++
										jCol = 1
									} else {
										jCol++
									}
									j++
								}
								if j < n && isAlpha(runes[j]) {
									curStart := j
									for j < n && isWordChar(runes[j]) {
										j++
										jCol++
									}
									curNameRaw := string(runes[curStart:j])
									curName := strings.ToUpper(curNameRaw)
									if !scriptStmtKeywords[curName] && !declaredVars[curName] {
										addError("Variable '"+curNameRaw+"' is not declared", line, jCol-len(curNameRaw), line, jCol)
									}
								}
							}
						} else {
							// RETURN expr
							// If it's a known keyword, it's not a variable usage (e.g. RETURN SELECT ...)
							if !scriptStmtKeywords[varName] && !declaredVars[varName] {
								addError("Variable '"+varNameRaw+"' is not declared", line, jCol-len(varNameRaw), line, jCol)
							}
						}
						// Advance main loop counters to where we peeked
						i = j
						col = jCol
					}
					// FOR and RETURN themselves don't start a new statement, but they consume the start position
					atScriptStmtStart = false
				case "LET", "VAR":
					// Peek ahead for the variable name being declared
					j := i
					jCol := col
					for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
						runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
						if runes[j] == '\n' {
							line++
							jCol = 1
						} else {
							jCol++
						}
						j++
					}
					if j < n && isAlpha(runes[j]) {
						varStart := j
						for j < n && isWordChar(runes[j]) {
							j++
							jCol++
						}
						varNameRaw := string(runes[varStart:j])
						varName := strings.ToUpper(varNameRaw)
						declaredVars[varName] = true

						// Skip whitespace after the variable name.
						for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
							runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
							if runes[j] == '\n' {
								line++
								jCol = 1
							} else {
								jCol++
							}
							j++
						}

						// Skip optional type annotation (e.g. FLOAT, VARCHAR(100))
						// that may appear between the variable name and ':' or '='.
						if j < n && isAlpha(runes[j]) {
							typeStart := j
							for j < n && isWordChar(runes[j]) {
								j++
								jCol++
							}
							typeWord := strings.ToUpper(string(runes[typeStart:j]))
							if typeWord != "DEFAULT" {
								// Skip optional parenthesised type parameters: VARCHAR(100), NUMBER(10,2)
								for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\u00a0') {
									j++
									jCol++
								}
								if j < n && runes[j] == '(' {
									depth := 1
									j++
									jCol++
									for j < n && depth > 0 {
										switch runes[j] {
										case '(':
											depth++
										case ')':
											depth--
										}
										if runes[j] == '\n' {
											line++
											jCol = 1
										} else {
											jCol++
										}
										j++
									}
								}
								// Skip whitespace before the assignment operator.
								for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
									runes[j] == '\n' || runes[j] == '\r' || runes[j] == '\u00a0') {
									if runes[j] == '\n' {
										line++
										jCol = 1
									} else {
										jCol++
									}
									j++
								}
							}
						}

						// Check for := (valid) or bare = (error) and detect missing expression.
						isLetColon := j < n && runes[j] == ':' && j+1 < n && runes[j+1] == '='
						isLetBareEq := j < n && runes[j] == '=' && (j == 0 || runes[j-1] != ':')
						if isLetColon || isLetBareEq {
							if isLetBareEq {
								addError("Expected ':=' for assignment", line, jCol, line, jCol+1)
							}

							// --- Missing expression check ---
							opEnd := j + 1
							if isLetColon {
								opEnd = j + 2 // skip both : and =
							}
							k := opEnd
							for k < n && (runes[k] == ' ' || runes[k] == '\t' || runes[k] == '\n' || runes[k] == '\r' || runes[k] == '\u00a0') {
								k++
							}

							missingExpr := false
							if k >= n || runes[k] == ';' {
								missingExpr = true
							} else if isAlpha(runes[k]) {
								wordStart := k
								for k < n && isWordChar(runes[k]) {
									k++
								}
								firstWord := strings.ToUpper(string(runes[wordStart:k]))
								switch firstWord {
								case "LET", "DECLARE", "BEGIN", "RETURN", "FOR", "WHILE", "LOOP", "IF":
									missingExpr = true
								}
							}

							if missingExpr {
								addError("Missing expression after assignment", line, jCol, line, jCol+(opEnd-j))
							}
						}
						// Advance main loop counters to where we peeked
						i = j
						col = jCol
					}

				default:
					if inDeclareBlock {
						// Inside DECLARE every non-keyword identifier is a variable declaration
						if !scriptStmtKeywords[word] {
							declaredVars[word] = true
						}
						atScriptStmtStart = true // Each line in DECLARE is a new start
					} else if !scriptStmtKeywords[word] {
						// Look ahead for assignment operator
						j, jCol := i, col
						for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r' || runes[j] == '\u00a0') {
							if runes[j] == '\n' {
								break
							}
							j++
							jCol++
						}
						isColonAssign := j < n && runes[j] == ':' && j+1 < n && runes[j+1] == '='
						isBareEq := false
						if j < n && runes[j] == '=' {
							prev := rune(0)
							if j > 0 {
								prev = runes[j-1]
							}
							next := rune(0)
							if j+1 < n {
								next = runes[j+1]
							}
							isBareEq = prev != ':' && prev != '<' && prev != '>' && prev != '!' && next != '='
						}
						isAssignment := isColonAssign || isBareEq

						if isAssignment {
							if isBareEq {
								addError("Expected ':=' for assignment",
									line, jCol, line, jCol+1)
							}
							if !declaredVars[word] {
								addError("Variable '"+wordRaw+"' is not declared",
									wordLine, wordCol, wordLine, wordCol+len(wordRaw))
							}

							// --- Missing expression check ---
							opEnd := j
							if isColonAssign {
								opEnd += 2
							} else {
								opEnd += 1
							}

							k := opEnd
							for k < n && (runes[k] == ' ' || runes[k] == '\t' || runes[k] == '\n' || runes[k] == '\r' || runes[k] == '\u00a0') {
								k++
							}

							missingExpr := false
							if k >= n || runes[k] == ';' {
								missingExpr = true
							} else if isAlpha(runes[k]) {
								wordStart := k
								for k < n && isWordChar(runes[k]) {
									k++
								}
								firstWord := strings.ToUpper(string(runes[wordStart:k]))
								switch firstWord {
								case "LET", "DECLARE", "BEGIN", "RETURN", "FOR", "WHILE", "LOOP", "IF":
									missingExpr = true
								}
							}

							if missingExpr {
								addError("Missing expression after assignment", line, jCol, line, jCol+(opEnd-j))
							}

						} else {
							// Not a scripting keyword and not an assignment —
							// bare unrecognized identifier at statement start.
							addError("Unexpected token '"+wordRaw+"'",
								wordLine, wordCol, wordLine, wordCol+len(wordRaw))
						}
					}
				}
			}
			continue
		}

		// Any other character resets statement-start flags.
		// If we are at a statement-start position and encounter a character that
		// can never open a SQL or Snowflake Scripting statement, flag it as an
		// error.  This catches placeholder/template text like <wrong_text> or
		// {placeholder} that is accidentally left inside a script body.
		//
		// Angle brackets and curly braces are chosen because they are
		// syntactically invalid at statement level in both outer SQL and
		// Snowflake Scripting, yet common in template/placeholder patterns.
		if (atStmtStart || atScriptStmtStart) &&
			(ch == '<' || ch == '>' || ch == '{' || ch == '}') {
			addError("Unexpected token '"+string(ch)+"'",
				line, col, line, col+1)
		}
		atStmtStart = false
		atScriptStmtStart = false
		i++
		col++
	}

	// Report unclosed opening parens/brackets
	for _, open := range parenStack {
		addError("Unclosed '"+open.char+"'", open.line, open.col, open.line, open.col+1)
	}

	return markers
}

// ── ParseJoinTables ───────────────────────────────────────────────────────────

const idPat = `(?:"(?:""|[^"])*"|[\w$]+)`

var tableRefRe = regexp.MustCompile(
	`(?i)(?:FROM|JOIN|MERGE\s+INTO|USING)\s+` +
		`(?:` +
		`(` + idPat + `)\.(` + idPat + `)\.(` + idPat + `)` +
		`|(` + idPat + `)\.(` + idPat + `)` +
		`|(` + idPat + `)` +
		`)` +
		`(?:[ \t]+(?:AS[ \t]+)?(` + idPat + `))?`,
)

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

// stripQ strips surrounding double-quotes from an identifier and unescapes embedded quotes, leaving case intact.
func stripQ(s string) string {
	if strings.HasPrefix(s, `"`) {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, `""`, `"`)
	}
	return s
}

// ParseJoinTables extracts all FROM/JOIN table references (with optional aliases)
// from the given SQL text.  Three-part (db.schema.table), two-part (schema.table),
// and one-part (table) references are all recognized.
func ParseJoinTables(sql string) []JoinTableRef {
	var result []JoinTableRef
	start := 0
	for {
		m := tableRefRe.FindStringSubmatchIndex(sql[start:])
		if m == nil {
			break
		}

		// Adjust indices based on the current search start position
		for i := range m {
			if m[i] != -1 {
				m[i] += start
			}
		}

		var db, schema, name, alias string

		if m[2] != -1 && m[4] != -1 && m[6] != -1 {
			// Three-part: db.schema.table
			db = normID(sql[m[2]:m[3]])
			schema = normID(sql[m[4]:m[5]])
			name = normID(sql[m[6]:m[7]])
			start = m[7]
		} else if m[8] != -1 && m[10] != -1 {
			// Two-part: schema.table
			schema = normID(sql[m[8]:m[9]])
			name = normID(sql[m[10]:m[11]])
			start = m[11]
		} else if m[12] != -1 {
			// One-part: table
			name = normID(sql[m[12]:m[13]])
			start = m[13]
		}

		alias = name
		if m[14] != -1 {
			rawAlias := stripQ(sql[m[14]:m[15]])
			if !joinStopKW[strings.ToUpper(rawAlias)] {
				alias = rawAlias
				// Successfully matched an alias - continue AFTER the alias
				start = m[15]
			} else {
				// Matched a keyword instead of an alias - continue BEFORE the keyword
				start = m[14]
			}
		}

		result = append(result, JoinTableRef{DB: db, Schema: schema, Name: name, Alias: alias})
	}
	return result
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
			parts[idx] = sf.QuoteIdent(fkAlias) + "." + sf.QuoteOrBare(fk.FKColumn, false) +
				" = " + sf.QuoteIdent(pkAlias) + "." + sf.QuoteOrBare(fk.PKColumn, false)
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
					results = append(results, sf.QuoteIdent(lastAlias)+"."+sf.QuoteOrBare(col, false)+" = "+sf.QuoteIdent(otherAlias)+"."+sf.QuoteOrBare(pkCol, false))
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
					results = append(results, sf.QuoteIdent(otherAlias)+"."+sf.QuoteOrBare(col, false)+" = "+sf.QuoteIdent(lastAlias)+"."+sf.QuoteOrBare(pkCol, false))
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
	return ScriptingCompletionResult{
		Variables:  scriptingExtractVars(sql, cursorOffset),
		NeedsColon: scriptingNeedsColon(sql, cursorOffset),
	}
}

// scriptingExtractVars mirrors extractDeclaredVariables from snowflakeScriptingUtils.ts.
// Returns uppercased variable names declared before cursorOffset inside the current $$ block.
func scriptingExtractVars(sql string, cursorOffset int) []string {
	runes := []rune(sql)
	n := len(runes)
	if cursorOffset > n {
		cursorOffset = n
	}
	beforeCursor := string(runes[:cursorOffset])

	// Count $$ occurrences before cursor to determine if we are inside a block.
	ddCount := strings.Count(beforeCursor, "$$")
	if ddCount%2 == 0 {
		return nil // cursor is in plain SQL, not inside a $$ block
	}

	// Isolate text from the opening $$ to the cursor.
	blockStart := strings.LastIndex(beforeCursor, "$$")
	textToScan := beforeCursor[blockStart:]

	seen := make(map[string]struct{})
	var vars []string
	addVar := func(name string) {
		up := strings.ToUpper(name)
		if _, ok := seen[up]; !ok {
			seen[up] = struct{}{}
			vars = append(vars, up)
		}
	}

	skipWords := map[string]bool{
		"CURSOR": true, "EXCEPTION": true, "TYPE": true,
		"LET": true, "VAR": true,
	}

	// 1. DECLARE … BEGIN blocks
	declareRe := regexp.MustCompile(`(?i)\bDECLARE\b`)
	blockBoundRe := regexp.MustCompile(`(?i)\b(?:BEGIN|END)\b`)
	for _, loc := range declareRe.FindAllStringIndex(textToScan, -1) {
		after := textToScan[loc[1]:]
		if bound := blockBoundRe.FindStringIndex(after); bound != nil {
			after = after[:bound[0]]
		}
		for _, seg := range strings.Split(after, ";") {
			// Strip line comments and block comments
			clean := regexp.MustCompile(`--[^\n]*`).ReplaceAllString(seg, "")
			clean = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(clean, "")
			clean = strings.TrimSpace(clean)
			if clean == "" {
				continue
			}
			wordRe := regexp.MustCompile(`[a-zA-Z0-9_$]+`)
			if m := wordRe.FindString(clean); m != "" {
				up := strings.ToUpper(m)
				if !skipWords[up] {
					addVar(m)
				}
			}
		}
	}

	// 2. LET / VAR declarations
	letVarRe := regexp.MustCompile(`(?i)\b(?:LET|VAR)\s+([a-zA-Z0-9_$]+)`)
	for _, m := range letVarRe.FindAllStringSubmatch(textToScan, -1) {
		addVar(m[1])
	}

	// 3. FOR loop variables
	forRe := regexp.MustCompile(`(?i)\bFOR\s+([a-zA-Z0-9_$]+)\s+IN\b`)
	for _, m := range forRe.FindAllStringSubmatch(textToScan, -1) {
		addVar(m[1])
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

// scriptingNeedsColon mirrors isColonRequired from snowflakeScriptingUtils.ts.
func scriptingNeedsColon(sql string, offset int) bool {
	runes := []rune(sql)
	n := len(runes)
	if offset > n {
		offset = n
	}
	beforeCursor := string(runes[:offset])

	// 1. Find where the current word starts and check for a preceding colon.
	wordStartRe := regexp.MustCompile(`[a-zA-Z0-9_$]+$`)
	wordLoc := wordStartRe.FindStringIndex(beforeCursor)
	posToEval := len(beforeCursor)
	if wordLoc != nil {
		posToEval = wordLoc[0]
	}
	textBeforeWord := strings.TrimRight(beforeCursor[:posToEval], " \t\r\n")
	if strings.HasSuffix(textBeforeWord, ":") {
		return false
	}

	// 2. Strip comments and quoted strings.
	clean := regexp.MustCompile(`--[^\n]*`).ReplaceAllString(textBeforeWord, " ")
	clean = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(clean, " ")
	clean = regexp.MustCompile(`'[^']*'`).ReplaceAllString(clean, " ")
	clean = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(clean, " ")

	// 3. Find the most recent context-setting token.
	// SQL statement keywords require a colon. Scripting assignments/control-flow do not.
	contextRe := regexp.MustCompile(`(?i)(:=|;|\b(?:SELECT|INSERT|UPDATE|DELETE|MERGE|CREATE|ALTER|DROP|TRUNCATE|COPY|CALL|WITH|SHOW|DESCRIBE|GRANT|REVOKE|LET|RETURN|IF|WHILE|UNTIL|DO|LOOP|BEGIN)\b)`)

	matches := contextRe.FindAllString(clean, -1)
	if len(matches) == 0 {
		return false
	}

	lastMatch := strings.ToUpper(matches[len(matches)-1])

	// If the last context setter is a standard SQL keyword, it needs a colon.
	return colonRequiredKeywords[lastMatch]
}

// ── TypeCategory ──────────────────────────────────────────────────────────────

// typeCategoryMap maps canonical upper-case Snowflake type names to the broad
// JOIN-suggestion compatibility category used by ComputeJoinOnConditions.
// It is built once from snowflake.AllDataTypes so that any type added to the
// authoritative registry is automatically visible here (defaulting to "other").
var typeCategoryMap = func() map[string]string {
	// Category assignment is sqleditor-specific (JOIN compatibility buckets).
	// Types absent from this map fall through to "other".
	explicit := map[string]string{
		"NUMBER": "numeric", "DECIMAL": "numeric", "NUMERIC": "numeric",
		"INT": "numeric", "INTEGER": "numeric", "BIGINT": "numeric",
		"SMALLINT": "numeric", "TINYINT": "numeric", "BYTEINT": "numeric",
		"FLOAT": "numeric", "FLOAT4": "numeric", "FLOAT8": "numeric",
		"DOUBLE": "numeric", "DOUBLE PRECISION": "numeric", "REAL": "numeric",
		"VARCHAR": "text", "CHAR": "text", "CHARACTER": "text",
		"STRING": "text", "TEXT": "text",
		"BINARY": "text", "VARBINARY": "text",
		"BOOLEAN": "boolean",
		"DATE": "datetime", "DATETIME": "datetime", "TIME": "datetime",
		"TIMESTAMP": "datetime", "TIMESTAMP_LTZ": "datetime",
		"TIMESTAMP_NTZ": "datetime", "TIMESTAMP_TZ": "datetime",
		"VARIANT": "semi", "OBJECT": "semi", "ARRAY": "semi",
	}
	m := make(map[string]string, len(sf.AllDataTypes()))
	for _, dt := range sf.AllDataTypes() {
		if cat, ok := explicit[dt.Name]; ok {
			m[dt.Name] = cat
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

// ── ValidateSemantics ─────────────────────────────────────────────────────────

// ValidateSemantics walks the SQL text and for every alias.column two-part
// reference where the alias is in resolvedRefs, checks whether column exists
// in the cached column list.  Unknown columns emit Warning markers.
func ValidateSemantics(sql string, resolvedRefs []ResolvedRef, colEntries []ColEntry) []DiagMarker {
	var markers []DiagMarker

	// Build colInfoCache: "UC(DB)\x00UC(SCHEMA)\x00UC(NAME)" → []ColInfo
	colInfoCache := map[string][]ColInfo{}
	for _, e := range colEntries {
		key := strings.ToUpper(e.DB) + "\x00" + strings.ToUpper(e.Schema) + "\x00" + strings.ToUpper(e.Name)
		colInfoCache[key] = e.Cols
	}

	// Build aliasMap: UC(alias) → cache key
	aliasMap := map[string]string{}
	for _, ref := range resolvedRefs {
		key := strings.ToUpper(ref.DB) + "\x00" + strings.ToUpper(ref.Schema) + "\x00" + strings.ToUpper(ref.Name)
		aliasMap[strings.ToUpper(ref.Alias)] = key
	}

	runes := []rune(sql)
	n := len(runes)
	line, col := 1, 1
	i := 0

	for i < n {
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
			continue
		}

		// Skip double-quoted identifiers (not treated as alias words)
		if ch == '"' {
			i++
			col++
			for i < n {
				if runes[i] == '\n' {
					line++
					col = 1
					i++
				} else if runes[i] == '"' && i+1 < n && runes[i+1] == '"' {
					i += 2
					col += 2
				} else if runes[i] == '"' {
					i++
					col++
					break
				} else {
					i++
					col++
				}
			}
			continue
		}

		// Skip dollar-quoted tag delimiters
		if ch == '$' {
			tag := extractDollarTag(runes, i)
			if tag != "" {
				i += len([]rune(tag))
				col += len([]rune(tag))
				continue
			}
		}

		// Bare word — look for alias.column patterns
		if isAlpha(ch) {
			word1Start := i
			for i < n && isWordChar(runes[i]) {
				i++
				col++
			}
			word1 := string(runes[word1Start:i])

			j, jCol := i, col
			if j < n && runes[j] == '.' {
				afterDot := j + 1
				afterDotCol := jCol + 1
				if afterDot < n && isAlpha(runes[afterDot]) {
					word2Col := afterDotCol
					word2Line := line
					k, kCol := afterDot, afterDotCol
					for k < n && isWordChar(runes[k]) {
						k++
						kCol++
					}
					word2 := string(runes[afterDot:k])

					// Only validate two-part references (skip three-part db.schema.col)
					if !(k < n && runes[k] == '.') {
						if cacheKey, ok := aliasMap[strings.ToUpper(word1)]; ok {
							if cols, ok := colInfoCache[cacheKey]; ok {
								found := false
								for _, c := range cols {
									if strings.EqualFold(c.Name, word2) {
										found = true
										break
									}
								}
								if !found {
									tableName := cacheKey
									if parts := strings.Split(cacheKey, "\x00"); len(parts) == 3 {
										tableName = parts[2]
									}
									markers = append(markers, DiagMarker{
										StartLineNumber: word2Line,
										StartColumn:     word2Col,
										EndLineNumber:   word2Line,
										EndColumn:       word2Col + len(word2),
										Message:         "Column '" + word2 + "' does not exist in " + tableName,
										Severity:        4,
									})
								}
							}
						}
					}
				}
			}
			continue
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
			cond := sf.QuoteIdent(a1) + "." + sf.QuoteOrBare(info.Name, false) + " = " + sf.QuoteIdent(a2) + "." + sf.QuoteOrBare(info.Name, false)
			addSugg(cond, "SAME-NAME COLUMN", "1")
		}
		if len(sharedCompatible) > 0 {
			usingCond := "USING (" + strings.Join(sharedCompatible, ", ") + ")"
			addSugg(usingCond, "USING", "1.5")
		}
	}

	return suggestions
}
