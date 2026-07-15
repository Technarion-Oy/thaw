// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"fmt"
	"strings"

	sf "thaw/internal/snowflake"
	"thaw/internal/sqltok"
)

// ── Precompiled regexes ───────────────────────────────────────────────────────

const (
	// ReIdentifier matches a Snowflake identifier part: either a double-quoted
	// string with escaped quotes (""""), or a bare word containing [a-zA-Z0-9_$].
	ReIdentifier = `(?:"(?:""|[^"])*"|[\w$]+)`

	_ident          = `(?:[a-zA-Z_][a-zA-Z0-9_$]*|"[^"]+")`
	_identPath      = _ident + `(?:\.` + _ident + `){0,2}`
	_balancedParens = `\([^()]*(?:(?:\([^()]*\))[^()]*)*\)`
)

var (
	// Snowflake false-positive guard: matchesSnowflakeFP uses these object-noun
	// sets to recognize CREATE/ALTER/DROP of objects whose statements the bare-
	// column and table-existence validators can't handle and should skip. The
	// two-word nouns IMAGE REPOSITORY and GIT REPOSITORY are matched separately
	// in fpObjectNoun.
	fpCreateNouns = toUpperSet([]string{
		"STAGE", "REPLICATION", "FAILOVER", "APPLICATION", "DATASHARE", "SERVICE",
	})
	fpAlterNouns = toUpperSet([]string{
		"TABLE", "VIEW", "STREAM", "DATABASE", "STAGE", "PIPE", "PROCEDURE", "FUNCTION",
		"ALERT", "EXTERNAL", "NOTIFICATION", "STORAGE", "SECURITY", "MASKING", "NETWORK",
		"REPLICATION", "FAILOVER", "APPLICATION", "DATASHARE", "SERVICE",
	})
	fpDropNouns = toUpperSet([]string{
		"TABLE", "VIEW", "STREAM", "STAGE", "PIPE", "PROCEDURE", "FUNCTION",
		"APPLICATION", "DATASHARE", "SERVICE",
	})
)

// Precomputed token-based guard closures replacing regex guards.
var (
	// isCreateTableGuard matches CREATE [modifiers...] TABLE in any modifier order,
	// mirroring the old regex (?i)CREATE\s+(?:(?:OR\s+(?:REPLACE|ALTER)|LOCAL|GLOBAL|TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)*TABLE.
	isCreateTableGuard = func() func([]sqltok.Token, string) bool {
		tableMods := map[string]bool{
			"OR": true, "REPLACE": true, "ALTER": true,
			"LOCAL": true, "GLOBAL": true,
			"TEMP": true, "TEMPORARY": true, "VOLATILE": true, "TRANSIENT": true,
		}
		return func(sig []sqltok.Token, sql string) bool {
			if len(sig) == 0 || tokUpper(sig[0], sql) != "CREATE" {
				return false
			}
			for i := 1; i < len(sig); i++ {
				u := tokUpper(sig[i], sql)
				if u == "TABLE" {
					return true
				}
				if !tableMods[u] {
					return false
				}
			}
			return false
		}
	}()
	isCreateViewGuard     = guardCreateWithMods([][]string{{"SECURE"}, {"LOCAL", "GLOBAL"}, {"TEMP", "TEMPORARY", "VOLATILE"}, {"RECURSIVE"}, {"INTERACTIVE"}, {"MATERIALIZED"}}, "VIEW")
	isCreateDynTableGuard = guardCreate("DYNAMIC", "TABLE")
)

// isCreateTable reports whether sql is a CREATE TABLE statement (token-based).
func isCreateTable(sql string) bool {
	sig := sigTokens(sql)
	return isCreateTableGuard(sig, sql)
}

// isCreateView reports whether sql is a CREATE VIEW statement (token-based).
func isCreateView(sql string) bool {
	sig := sigTokens(sql)
	return isCreateViewGuard(sig, sql)
}

// isCreateDynTable reports whether sql is a CREATE DYNAMIC TABLE statement (token-based).
func isCreateDynTable(sql string) bool {
	sig := sigTokens(sql)
	return isCreateDynTableGuard(sig, sql)
}

// fpObjectNoun reports whether sig[i] (optionally with sig[i+1]) is one of the
// object nouns in single, or the two-word IMAGE REPOSITORY / GIT REPOSITORY.
func fpObjectNoun(sig []sqltok.Token, sql string, i int, single map[string]bool) bool {
	if i >= len(sig) {
		return false
	}
	n := tokUpper(sig[i], sql)
	if single[n] {
		return true
	}
	return (n == "IMAGE" || n == "GIT") && i+1 < len(sig) && tokUpper(sig[i+1], sql) == "REPOSITORY"
}

// matchesSnowflakeFP reports whether a statement contains Snowflake-specific
// syntax that the bare-column-ref and table-existence validators cannot analyze
// and should therefore skip (to avoid emitting noise). It is the token-based
// replacement for the old reSnowflakeFP regex guard.
//
// Working on the significant-token stream means keywords inside string literals,
// comments, and dollar-quoted bodies are never matched (the regex, applied to
// comment-stripped-but-not-string-stripped text, could mis-fire on e.g.
// SELECT 'DROP TABLE x'). CLUSTER BY (...) clauses are skipped wholesale so their
// contents cannot trigger a match, replacing the prior reClusterBy pre-strip.
func matchesSnowflakeFP(sig []sqltok.Token, sql string) bool {
	// Statement-initial INSERT [OVERWRITE] ALL|FIRST (the old ^INSERT anchor).
	if len(sig) > 0 && tokUpper(sig[0], sql) == "INSERT" {
		j := 1
		if j < len(sig) && tokUpper(sig[j], sql) == "OVERWRITE" {
			j++
		}
		if j < len(sig) {
			if u := tokUpper(sig[j], sql); u == "ALL" || u == "FIRST" {
				return true
			}
		}
	}

	for i := 0; i < len(sig); i++ {
		switch tokUpper(sig[i], sql) {
		case "CLUSTER":
			// Skip CLUSTER BY ( ... ) so its contents don't trigger a match.
			if i+2 < len(sig) && tokUpper(sig[i+1], sql) == "BY" && sig[i+2].Kind == sqltok.LParen {
				if _, closeIdx, ok := parenInnerRange(sig, i+2); ok {
					i = closeIdx
				}
			}
		case "TABLESAMPLE", "INFER_SCHEMA", "UNPIVOT":
			return true
		case "SAMPLE", "PIVOT", "MATCH_RECOGNIZE", "AT", "BEFORE":
			if i+1 < len(sig) && sig[i+1].Kind == sqltok.LParen {
				return true
			}
		case "WITHIN":
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == "GROUP" {
				return true
			}
		case "CONNECT":
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == "BY" {
				return true
			}
		case "IN":
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == "TABLE" {
				return true
			}
		case "LATERAL":
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == "FLATTEN" {
				return true
			}
		case "ASOF":
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == "JOIN" {
				return true
			}
		case "EXECUTE":
			j := i + 1
			if j < len(sig) && tokUpper(sig[j], sql) == "JOB" {
				j++
			}
			if j < len(sig) && tokUpper(sig[j], sql) == "SERVICE" {
				return true
			}
		case "UNDROP":
			if i+1 < len(sig) {
				if u := tokUpper(sig[i+1], sql); u == "DATABASE" || u == "SCHEMA" || u == "TABLE" {
					return true
				}
			}
		case "TRUNCATE":
			// TRUNCATE <name-word> IF  (TRUNCATE [TABLE] IF EXISTS …).
			if i+2 < len(sig) && isIdent(sig[i+1]) && tokUpper(sig[i+2], sql) == "IF" {
				return true
			}
		case "CREATE":
			j := i + 1
			if j+1 < len(sig) && tokUpper(sig[j], sql) == "OR" && tokUpper(sig[j+1], sql) == "REPLACE" {
				j += 2
			}
			if j < len(sig) && tokUpper(sig[j], sql) == "TRANSIENT" {
				j++
			}
			if fpObjectNoun(sig, sql, j, fpCreateNouns) {
				return true
			}
		case "ALTER":
			if fpObjectNoun(sig, sql, i+1, fpAlterNouns) {
				return true
			}
		case "DROP":
			if fpObjectNoun(sig, sql, i+1, fpDropNouns) {
				return true
			}
		}
	}
	return false
}

// ── Shared validation helpers (DRY) ───────────────────────────────────────────

// cleanParseText strips SQL comments and string literals, returning a trimmed
// result suitable for regex-based property/keyword detection. Comments are
// replaced with whitespace (preserving newlines) and string literals are
// replaced with a single space each. The tokenizer handles interaction between
// comments and strings correctly (e.g. apostrophes inside comments, comment
// markers inside strings).
// toUpperSet builds a map[string]bool from a slice of strings, upper-casing each key.
func toUpperSet(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[strings.ToUpper(k)] = true
	}
	return m
}

// nonColumnDefKeywords are the leading words of a table-level clause that is
// NOT a column definition, so the token after them must not be treated as a
// column name / data type. Shared by walkColumnDefTypes (CREATE TABLE body) and
// the ALTER TABLE ADD handler. ROW / SEARCH are intentionally absent: they only
// begin table-level clauses in the ALTER ADD form (ROW ACCESS POLICY, SEARCH
// OPTIMIZATION) and are handled there; inside a CREATE TABLE body "row" / "search"
// can be a real column name, so they must still be type-checked.
var nonColumnDefKeywords = toUpperSet([]string{
	"CONSTRAINT", "PRIMARY", "UNIQUE", "FOREIGN", "INDEX", "CHECK",
})

// ── ValidateDataTypes ─────────────────────────────────────────────────────────

// ValidateDataTypes checks that explicit data type declarations within
// CREATE TABLE, ALTER TABLE, and CAST() functions exist in Snowflake's registry.
func ValidateDataTypes(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker

	validTypes := make(map[string]bool)
	for _, dt := range sf.AllDataTypes() {
		validTypes[strings.ToUpper(dt.Name)] = true
	}

	offsetToLineCol := func(offset int) (int, int) {
		line, col := 1, 1
		for i := 0; i < offset && i < len(sql); i++ {
			if sql[i] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		return line, col
	}

	checkType := func(typeName string, typeOffset int) {
		up := strings.ToUpper(typeName)
		if !validTypes[up] {
			line, col := offsetToLineCol(typeOffset)
			markers = append(markers, DiagMarker{
				StartLineNumber: line,
				StartColumn:     col,
				EndLineNumber:   line,
				EndColumn:       col + len(typeName),
				Message:         fmt.Sprintf("Unknown data type '%s'", up),
				Severity:        4, // Warning
			})
		}
	}

	for _, r := range stmtRanges {
		rawText := sqlStmt(sql, r)
		stmtOffset := r.StartOffset

		sig := sigTokens(rawText)

		// rel reports a type whose offset is relative to rawText, translating it
		// to an absolute document offset for checkType.
		rel := func(name string, relOffset int) { checkType(name, stmtOffset+relOffset) }

		// 1. Shorthand cast (::TYPE) — the "::" operator followed by a bare word.
		for i := 0; i+1 < len(sig); i++ {
			if sig[i].Kind == sqltok.Operator && sig[i].Text(rawText) == "::" {
				nt := sig[i+1]
				if nt.Kind == sqltok.Keyword || nt.Kind == sqltok.Identifier {
					rel(nt.Text(rawText), nt.Start)
				}
			}
		}

		// 2. CAST / TRY_CAST ( … AS TYPE ) — the CAST's own AS, which sits at the
		// depth of the opening paren. A nested AS alias (e.g. an inner subquery's
		// SELECT MAX(id) AS mx) is at a deeper depth and must be ignored.
		for i := 0; i+1 < len(sig); i++ {
			u := tokUpper(sig[i], rawText)
			if (u != "CAST" && u != "TRY_CAST") || sig[i+1].Kind != sqltok.LParen {
				continue
			}
			start, closeIdx, ok := parenInnerRange(sig, i+1)
			if !ok {
				continue
			}
			depth := 0 // relative to the inside of the CAST's "("
			for j := start; j < closeIdx; j++ {
				switch sig[j].Kind {
				case sqltok.LParen:
					depth++
					continue
				case sqltok.RParen:
					depth--
					continue
				}
				if depth != 0 || tokUpper(sig[j], rawText) != "AS" {
					continue
				}
				if j+1 < closeIdx {
					tt := sig[j+1]
					// The old regex required a 2+ char type ([a-zA-Z_][a-zA-Z0-9_]+).
					if (tt.Kind == sqltok.Keyword || tt.Kind == sqltok.Identifier) && tt.End-tt.Start >= 2 {
						rel(tt.Text(rawText), tt.Start)
					}
				}
				break
			}
		}

		// 3. ALTER TABLE <name> ADD [COLUMN] [IF NOT EXISTS] <col> <type>
		if kwAt(sig, rawText, 0, "ALTER") && kwAt(sig, rawText, 1, "TABLE") {
			_, pos := readIdentPath(sig, rawText, 2)
			if kwAt(sig, rawText, pos, "ADD") {
				pos++
				sawColumn := kwAt(sig, rawText, pos, "COLUMN")
				if sawColumn {
					pos++
				}
				if pos+2 < len(sig) && kwAt(sig, rawText, pos, "IF") &&
					kwAt(sig, rawText, pos+1, "NOT") && kwAt(sig, rawText, pos+2, "EXISTS") {
					pos += 3
				}
				// Column name (one ident token), then the declared type. An explicit
				// COLUMN keyword makes the next ident unambiguously a column, so it is
				// always validated. Without COLUMN, ADD PRIMARY KEY / CONSTRAINT /
				// FOREIGN KEY / ROW ACCESS POLICY / SEARCH OPTIMIZATION / … are
				// table-level clauses, not column defs, and are skipped. ROW / SEARCH
				// are ALTER-ADD-only markers (kept out of nonColumnDefKeywords so a
				// column literally named "row"/"search" still gets type-checked).
				addWord := ""
				if pos < len(sig) {
					addWord = strings.ToUpper(sig[pos].Text(rawText))
				}
				isColumnClause := sawColumn ||
					(!nonColumnDefKeywords[addWord] && addWord != "ROW" && addWord != "SEARCH")
				if pos < len(sig) && isIdent(sig[pos]) && isColumnClause {
					pos++
					if pos < len(sig) && (sig[pos].Kind == sqltok.Keyword || sig[pos].Kind == sqltok.Identifier) {
						rel(sig[pos].Text(rawText), sig[pos].Start)
					}
				}
			}
		}

		isCreate := kwAt(sig, rawText, 0, "CREATE")

		// 4. CREATE TABLE column definitions.
		if isCreate {
			if lp := createTableColParen(sig, rawText); lp >= 0 {
				walkColumnDefTypes(sig, rawText, lp, rel)
			}
		}

		// 5. CREATE PROCEDURE / FUNCTION parameter list.
		if isCreate {
			if lp := createProcParamParen(sig, rawText); lp >= 0 {
				walkColumnDefTypes(sig, rawText, lp, rel)
			} else if lp := createFuncParamParen(sig, rawText); lp >= 0 {
				walkColumnDefTypes(sig, rawText, lp, rel)
			}
		}

		// 6. RETURNS type (CREATE PROCEDURE / FUNCTION). "NULL" (RETURNS NULL ON
		// NULL INPUT) and "TABLE" (RETURNS TABLE(...)) are not data types.
		if isCreate {
			for i := 0; i+1 < len(sig); i++ {
				if tokUpper(sig[i], rawText) != "RETURNS" {
					continue
				}
				tt := sig[i+1]
				if tt.Kind != sqltok.Keyword && tt.Kind != sqltok.Identifier {
					continue
				}
				if u := strings.ToUpper(tt.Text(rawText)); u != "NULL" && u != "TABLE" {
					rel(tt.Text(rawText), tt.Start)
				}
			}
		}
	}

	return markers
}

// createTableColParen returns the index in sig of the "(" that opens the column
// list of a CREATE [scope] TABLE statement, or -1. Mirrors the old reCreateTableExt
// regex: CREATE [OR REPLACE] [LOCAL|GLOBAL] [TEMP|TEMPORARY|VOLATILE|TRANSIENT]
// TABLE [IF NOT EXISTS] <ident_path> "(".
func createTableColParen(sig []sqltok.Token, sql string) int {
	if !kwAt(sig, sql, 0, "CREATE") {
		return -1
	}
	i := 1
	if kwAt(sig, sql, i, "OR") && kwAt(sig, sql, i+1, "REPLACE") {
		i += 2
	}
	if kwAtAny(sig, sql, i, "LOCAL", "GLOBAL") != "" {
		i++
	}
	if kwAtAny(sig, sql, i, "TEMP", "TEMPORARY", "VOLATILE", "TRANSIENT") != "" {
		i++
	}
	if !kwAt(sig, sql, i, "TABLE") {
		return -1
	}
	i++
	if i+2 < len(sig) && kwAt(sig, sql, i, "IF") && kwAt(sig, sql, i+1, "NOT") && kwAt(sig, sql, i+2, "EXISTS") {
		i += 3
	}
	return identPathThenParen(sig, sql, i)
}

// createProcParamParen mirrors reCreateProcExt:
// CREATE [OR REPLACE] PROCEDURE <ident_path> "(".
func createProcParamParen(sig []sqltok.Token, sql string) int {
	if !kwAt(sig, sql, 0, "CREATE") {
		return -1
	}
	i := 1
	if kwAt(sig, sql, i, "OR") && kwAt(sig, sql, i+1, "REPLACE") {
		i += 2
	}
	if !kwAt(sig, sql, i, "PROCEDURE") {
		return -1
	}
	return identPathThenParen(sig, sql, i+1)
}

// createFuncParamParen mirrors reCreateFuncExt:
// CREATE [OR REPLACE] [SECURE] [TEMPORARY|TEMP] [AGGREGATE] FUNCTION <ident_path> "(".
func createFuncParamParen(sig []sqltok.Token, sql string) int {
	if !kwAt(sig, sql, 0, "CREATE") {
		return -1
	}
	i := 1
	if kwAt(sig, sql, i, "OR") && kwAt(sig, sql, i+1, "REPLACE") {
		i += 2
	}
	if kwAt(sig, sql, i, "SECURE") {
		i++
	}
	if kwAtAny(sig, sql, i, "TEMPORARY", "TEMP") != "" {
		i++
	}
	if kwAt(sig, sql, i, "AGGREGATE") {
		i++
	}
	if !kwAt(sig, sql, i, "FUNCTION") {
		return -1
	}
	return identPathThenParen(sig, sql, i+1)
}

// identPathThenParen reads an ident path starting at sig[pos]; if the token
// immediately following the path is "(", it returns that paren's index, else -1.
func identPathThenParen(sig []sqltok.Token, sql string, pos int) int {
	if pos >= len(sig) || !isIdent(sig[pos]) {
		return -1
	}
	_, next := readIdentPath(sig, sql, pos)
	if next < len(sig) && sig[next].Kind == sqltok.LParen {
		return next
	}
	return -1
}

// walkColumnDefTypes iterates the comma-separated definitions inside the
// parenthesised group whose opening "(" is sig[lparenIdx], reporting the
// declared data type of each column definition — the second word token of the
// segment — via onType(text, relOffset). Constraint definitions (CONSTRAINT,
// PRIMARY, UNIQUE, FOREIGN, INDEX, CHECK) and quoted type tokens are skipped.
// This replaces the old extractBalancedBlockPat + parseColumnDefs + processColumnDef
// helpers; the tokenizer handles strings, comments, and nested parens correctly.
func walkColumnDefTypes(sig []sqltok.Token, sql string, lparenIdx int, onType func(string, int)) {
	if lparenIdx < 0 || lparenIdx >= len(sig) || sig[lparenIdx].Kind != sqltok.LParen {
		return
	}
	// Locate the matching close paren. If the group is unterminated, skip
	// entirely (the old extractBalancedBlockPat returned "" for unbalanced parens,
	// so malformed input produced no type warnings).
	depth := 0
	closeIdx := -1
	for i := lparenIdx; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return
	}

	var w0, w1 sqltok.Token
	wn := 0
	flush := func() {
		if wn >= 2 {
			if nonColumnDefKeywords[strings.ToUpper(w0.Text(sql))] {
				// table-level constraint / policy, not a column definition
			} else if w1.Kind != sqltok.QuotedIdent {
				onType(w1.Text(sql), w1.Start)
			}
		}
		wn = 0
	}
	depth = 0
	for i := lparenIdx; i <= closeIdx; i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				flush()
				return
			}
		case sqltok.Comma:
			if depth == 1 {
				flush()
			}
		case sqltok.Keyword, sqltok.Identifier, sqltok.QuotedIdent:
			// Collect the first two word tokens of the current segment (the
			// column name and its type), regardless of nesting depth.
			switch wn {
			case 0:
				w0, wn = sig[i], 1
			case 1:
				w1, wn = sig[i], 2
			}
		}
	}
}
