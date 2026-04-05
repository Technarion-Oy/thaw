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

// ── ValidateSyntax ────────────────────────────────────────────────────────────

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
		if ch == ' ' || ch == '\t' || ch == '\r' {
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
				for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r') {
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
						runes[j] == '\n' || runes[j] == '\r') {
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
								runes[j] == '\n' || runes[j] == '\r') {
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
									runes[j] == '\n' || runes[j] == '\r') {
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
						runes[j] == '\n' || runes[j] == '\r') {
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

						// Now check for assignment after the variable name
						for j < n && (runes[j] == ' ' || runes[j] == '\t' ||
							runes[j] == '\n' || runes[j] == '\r') {
							if runes[j] == '\n' {
								line++
								jCol = 1
							} else {
								jCol++
							}
							j++
						}
						if j < n && runes[j] == '=' {
							prev := runes[j-1]
							next := rune(0)
							if j+1 < n {
								next = runes[j+1]
							}
							if prev != ':' && next != '=' {
								addError("Expected ':=' for assignment", line, jCol, line, jCol+1)
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
						for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r') {
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
						} else {
							// Not a scripting keyword and not an assignment —
							// bare unrecognised identifier at statement start.
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

const idPat = `(?:"[^"]+"|` + `\w+)`

var tableRefRe = regexp.MustCompile(
	`(?i)(?:FROM|JOIN)\s+` +
		`(?:` +
		`(` + idPat + `)\.(` + idPat + `)\.(` + idPat + `)` +
		`|(` + idPat + `)\.(` + idPat + `)` +
		`|(` + idPat + `)` +
		`)` +
		`(?:[ \t]+(?:AS[ \t]+)?(` + idPat + `))?`,
)

// normID normalises a raw captured identifier:
//   - Quoted identifiers ("name") have quotes stripped and case preserved.
//   - Unquoted identifiers are uppercased (Snowflake convention).
func normID(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return strings.ToUpper(s)
}

// stripQ strips surrounding double-quotes from an identifier, leaving case intact.
func stripQ(s string) string {
	if strings.HasPrefix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}

// ParseJoinTables extracts all FROM/JOIN table references (with optional aliases)
// from the given SQL text.  Three-part (db.schema.table), two-part (schema.table),
// and one-part (table) references are all recognised.
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
			parts[idx] = fkAlias + "." + fk.FKColumn + " = " + pkAlias + "." + fk.PKColumn
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
					results = append(results, lastAlias+"."+col+" = "+otherAlias+"."+pkCol)
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
					results = append(results, otherAlias+"."+col+" = "+lastAlias+"."+pkCol)
					break
				}
			}
		}
	}
	return results
}

// ── TypeCategory ──────────────────────────────────────────────────────────────

var (
	numericRe  = regexp.MustCompile(`^(NUMBER|INT|INTEGER|FLOAT|DECIMAL|NUMERIC|BIGINT|SMALLINT|TINYINT|BYTEINT|DOUBLE|REAL)$`)
	textRe     = regexp.MustCompile(`^(VARCHAR|CHAR|STRING|TEXT|NCHAR|NVARCHAR|CHARACTER VARYING)$`)
	datetimeRe = regexp.MustCompile(`^(DATE|TIME|TIMESTAMP|DATETIME|TIMESTAMP_NTZ|TIMESTAMP_LTZ|TIMESTAMP_TZ)$`)
	semiRe     = regexp.MustCompile(`^(VARIANT|OBJECT|ARRAY)$`)
)

// TypeCategory maps a Snowflake data-type string to a broad compatibility category.
// Returns one of: "numeric", "text", "datetime", "boolean", "semi", "other".
func TypeCategory(dt string) string {
	t := strings.ToUpper(dt)
	// Strip type parameters (e.g. VARCHAR(255) → VARCHAR)
	if idx := strings.Index(t, "("); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	switch {
	case numericRe.MatchString(t):
		return "numeric"
	case textRe.MatchString(t):
		return "text"
	case datetimeRe.MatchString(t):
		return "datetime"
	case t == "BOOLEAN":
		return "boolean"
	case semiRe.MatchString(t):
		return "semi"
	default:
		return "other"
	}
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
			sharedCompatible = append(sharedCompatible, info.Name)

			// Standardize order: smaller alias first to keep suggestions stable
			a1, a2 := lastRef.Alias, otherRef.Alias
			if a1 > a2 {
				a1, a2 = a2, a1
			}
			cond := a1 + "." + info.Name + " = " + a2 + "." + info.Name
			addSugg(cond, "SAME-NAME COLUMN", "1")
		}
		if len(sharedCompatible) > 0 {
			usingCond := "USING (" + strings.Join(sharedCompatible, ", ") + ")"
			addSugg(usingCond, "USING", "1.5")
		}
	}

	return suggestions
}
