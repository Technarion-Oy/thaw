// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"strconv"
	"strings"

	"thaw/internal/sqltok"
)

// ── Token-stream pattern matching utilities ──────────────────────────────────
//
// These helpers replace regex-based statement classification in tableexist.go
// and barecolrefs.go with O(N) token-stream scanning.

// sigToks returns only significant tokens (everything except whitespace,
// newlines, comments, and EOF), preserving their original positions.
func sigToks(tokens []sqltok.Token) []sqltok.Token {
	return sqltok.Significant(tokens)
}

// sigTokens tokenizes sql and returns only its significant tokens. It is the
// string-input shorthand for sigToks(sqltok.Tokenize(sql)) — the setup used by
// nearly every validator.
func sigTokens(sql string) []sqltok.Token {
	return sqltok.SignificantTokens(sql)
}

// tokUpper returns the uppercased text of a keyword/identifier token.
// Returns "" for any other token kind.
func tokUpper(tok sqltok.Token, sql string) string {
	if tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier {
		return strings.ToUpper(tok.Text(sql))
	}
	return ""
}

// isIdent reports whether tok is a keyword, identifier, or quoted identifier —
// i.e. something that can appear in a qualified name.
func isIdent(tok sqltok.Token) bool {
	return tok.Kind.IsIdentLike()
}

// isAliasTok reports whether tok can be a table alias: an unquoted identifier or
// a "quoted" identifier, but not a bare keyword. This prevents a following clause
// keyword (WHERE, GROUP, JOIN, …) from being captured as an implicit alias.
func isAliasTok(tok sqltok.Token) bool {
	return tok.Kind == sqltok.Identifier || tok.Kind == sqltok.QuotedIdent
}

// isNonEmptyIdent checks that tok is an identifier token with actual content.
// Unlike isIdent, it rejects empty quoted identifiers ("") and unclosed quotes.
func isNonEmptyIdent(tok sqltok.Token, sql string) bool {
	if !isIdent(tok) {
		return false
	}
	text := tok.Text(sql)
	if tok.Kind == sqltok.QuotedIdent {
		// Must be properly closed and have content: "x" → len >= 3.
		// Unclosed quotes don't end with '"'.
		return len(text) >= 3 && text[len(text)-1] == '"'
	}
	return text != ""
}

// findPreambleEnd scans sig for the target keyword (e.g. "TABLE"),
// skips optional "IF NOT EXISTS" after it, reads an ident path, and returns
// the byte position in sql immediately after the ident path. Returns -1 if
// the target keyword or ident path is not found.
func findPreambleEnd(sig []sqltok.Token, sql, targetKW string) int {
	// Find the target keyword.
	idx := -1
	for i, t := range sig {
		if t.Kind == sqltok.Keyword && strings.EqualFold(t.Text(sql), targetKW) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return -1
	}
	pos := idx + 1
	// Skip optional IF NOT EXISTS.
	if pos+2 < len(sig) &&
		kwAt(sig, sql, pos, "IF") &&
		kwAt(sig, sql, pos+1, "NOT") &&
		kwAt(sig, sql, pos+2, "EXISTS") {
		pos += 3
	}
	// Read ident path.
	if pos >= len(sig) || !isIdent(sig[pos]) {
		return -1
	}
	_, nextPos := readIdentPath(sig, sql, pos)
	// Return byte position after the last consumed token.
	return sig[nextPos-1].End
}

// findCreateTablePreambleEnd validates the strict modifier order for CREATE TABLE:
//
//	CREATE [OR (REPLACE|ALTER)] [LOCAL|GLOBAL] [TEMP|TEMPORARY|VOLATILE|TRANSIENT] TABLE [IF NOT EXISTS] <identPath>
//
// Returns the byte position after the ident path, or -1 if the preamble is invalid.
func findCreateTablePreambleEnd(sig []sqltok.Token, sql string) int {
	if len(sig) == 0 || tokUpper(sig[0], sql) != "CREATE" {
		return -1
	}
	i := 1
	// Optional OR (REPLACE|ALTER)
	if i < len(sig) && tokUpper(sig[i], sql) == "OR" {
		i++
		if i < len(sig) {
			u := tokUpper(sig[i], sql)
			if u == "REPLACE" || u == "ALTER" {
				i++
			}
		}
	}
	// Optional LOCAL|GLOBAL
	if i < len(sig) {
		u := tokUpper(sig[i], sql)
		if u == "LOCAL" || u == "GLOBAL" {
			i++
		}
	}
	// Optional TEMP|TEMPORARY|VOLATILE|TRANSIENT
	if i < len(sig) {
		u := tokUpper(sig[i], sql)
		if u == "TEMP" || u == "TEMPORARY" || u == "VOLATILE" || u == "TRANSIENT" {
			i++
		}
	}
	// TABLE
	if i >= len(sig) || tokUpper(sig[i], sql) != "TABLE" {
		return -1
	}
	i++
	// Optional IF NOT EXISTS
	if i+2 < len(sig) && kwAt(sig, sql, i, "IF") && kwAt(sig, sql, i+1, "NOT") && kwAt(sig, sql, i+2, "EXISTS") {
		i += 3
	}
	// ident path
	if i >= len(sig) || !isIdent(sig[i]) {
		return -1
	}
	_, nextPos := readIdentPath(sig, sql, i)
	return sig[nextPos-1].End
}

// readIdentPath reads a dot-separated identifier path from sig[pos:] and returns
// the raw substring and the position after the last consumed token. sig is a
// significant-token slice (trivia removed), so parts join across original
// whitespace. The raw substring can be passed to extractIdentParts.
func readIdentPath(sig []sqltok.Token, sql string, pos int) (string, int) {
	raw, next, _ := sqltok.ReadIdentPath(sig, sql, pos, 0)
	return raw, next
}

// readIdentParts reads a dot-separated identifier path and returns the
// individual raw token texts (un-normalised).
func readIdentParts(sig []sqltok.Token, sql string, pos int) ([]string, int) {
	return sqltok.ReadIdentParts(sig, sql, pos, 0)
}

// kwAt checks if sig[pos] is a keyword/identifier matching kw (case-insensitive).
func kwAt(sig []sqltok.Token, sql string, pos int, kw string) bool {
	return pos < len(sig) && tokUpper(sig[pos], sql) == kw
}

// kwAtAny checks if sig[pos] matches any of kws. Returns the matched keyword or "".
func kwAtAny(sig []sqltok.Token, sql string, pos int, kws ...string) string {
	if pos >= len(sig) {
		return ""
	}
	u := tokUpper(sig[pos], sql)
	for _, kw := range kws {
		if u == kw {
			return kw
		}
	}
	return ""
}

// ── Statement prefix matchers ────────────────────────────────────────────────
//
// Each function matches a specific SQL statement prefix and extracts the
// identifier path. They operate on "significant tokens" (sig) — the output
// of sigToks.

// skipCreateOr consumes an optional "OR REPLACE" / "OR ALTER" prefix at sig[i]
// (Snowflake's CREATE OR ALTER is valid for tables, tasks, etc.). ok is false
// only when "OR" is present but not followed by REPLACE or ALTER — a malformed
// prefix the caller should reject.
func skipCreateOr(sig []sqltok.Token, sql string, i int) (next int, ok bool) {
	if !kwAt(sig, sql, i, "OR") {
		return i, true
	}
	if kwAtAny(sig, sql, i+1, "REPLACE", "ALTER") == "" {
		return i, false
	}
	return i + 2, true
}

// skipIfNotExists consumes an "IF NOT EXISTS" clause at sig[i] only when all
// three keywords are present, so a partial sequence such as the DROP-style
// "IF EXISTS" is left unconsumed rather than silently treated as IF NOT EXISTS.
func skipIfNotExists(sig []sqltok.Token, sql string, i int) int {
	if kwAt(sig, sql, i, "IF") && kwAt(sig, sql, i+1, "NOT") && kwAt(sig, sql, i+2, "EXISTS") {
		return i + 3
	}
	return i
}

// skipIfExists consumes an "IF EXISTS" clause at sig[i] only when both keywords
// are present, returning the new index and whether the clause was present.
func skipIfExists(sig []sqltok.Token, sql string, i int) (next int, present bool) {
	if kwAt(sig, sql, i, "IF") && kwAt(sig, sql, i+1, "EXISTS") {
		return i + 2, true
	}
	return i, false
}

// matchCreateTV matches:
//
//	CREATE [OR REPLACE] [SECURE] [INTERACTIVE]
//	  [{LOCAL|GLOBAL}] [{TEMP|TEMPORARY|VOLATILE|TRANSIENT}]
//	  [RECURSIVE] [MATERIALIZED]
//	  {TABLE|VIEW} [IF NOT EXISTS] <ident_path>
//
// Returns the raw ident path text, the object keyword (TABLE or VIEW), and ok.
func matchCreateTV(sig []sqltok.Token, sql string) (rawPath, objKw string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return
	}
	if kwAt(sig, sql, i, "SECURE") {
		i++
	}
	if kwAt(sig, sql, i, "INTERACTIVE") {
		i++
	}
	if kwAtAny(sig, sql, i, "LOCAL", "GLOBAL") != "" {
		i++
	}
	if kwAtAny(sig, sql, i, "TEMP", "TEMPORARY", "VOLATILE", "TRANSIENT") != "" {
		i++
	}
	if kwAt(sig, sql, i, "RECURSIVE") {
		i++
	}
	if kwAt(sig, sql, i, "MATERIALIZED") {
		i++
	}
	objKw = kwAtAny(sig, sql, i, "TABLE", "VIEW")
	if objKw == "" {
		return
	}
	i++
	i = skipIfNotExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchCreateDbSch matches:
//
//	CREATE [OR REPLACE] [TRANSIENT] {DATABASE|SCHEMA} [IF NOT EXISTS] <ident_path>
//
// Returns the raw ident path and ok.
func matchCreateDbSch(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return
	}
	if kwAt(sig, sql, i, "TRANSIENT") {
		i++
	}
	if kwAtAny(sig, sql, i, "DATABASE", "SCHEMA") == "" {
		return
	}
	i++
	i = skipIfNotExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchCreateSchema matches:
//
//	CREATE [OR REPLACE] [TRANSIENT] SCHEMA [IF NOT EXISTS] <ident_path>
func matchCreateSchema(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return
	}
	if kwAt(sig, sql, i, "TRANSIENT") {
		i++
	}
	if !kwAt(sig, sql, i, "SCHEMA") {
		return
	}
	i++
	i = skipIfNotExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchCreateAnyDB matches:
//
//	CREATE [OR REPLACE] [TRANSIENT] DATABASE
//
// Returns true if the prefix matches (no ident path capture needed).
func matchCreateAnyDB(sig []sqltok.Token, sql string) bool {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return false
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return false
	}
	if kwAt(sig, sql, i, "TRANSIENT") {
		i++
	}
	return kwAt(sig, sql, i, "DATABASE")
}

// matchCreateAnySchema matches:
//
//	CREATE [OR REPLACE] [TRANSIENT] SCHEMA
func matchCreateAnySchema(sig []sqltok.Token, sql string) bool {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return false
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return false
	}
	if kwAt(sig, sql, i, "TRANSIENT") {
		i++
	}
	return kwAt(sig, sql, i, "SCHEMA")
}

// matchDropTable matches:
//
//	DROP TABLE [IF EXISTS] <ident_path>
func matchDropTable(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "DROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "TABLE") {
		return
	}
	i++
	i, _ = skipIfExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchDropDbSch matches:
//
//	DROP {DATABASE|SCHEMA} [IF EXISTS] <ident_path>
func matchDropDbSch(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "DROP") {
		return
	}
	i++
	if kwAtAny(sig, sql, i, "DATABASE", "SCHEMA") == "" {
		return
	}
	i++
	i, _ = skipIfExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchDropDB matches DROP DATABASE [IF EXISTS] <ident> and returns
// whether IF EXISTS was present.
func matchDropDB(sig []sqltok.Token, sql string) (rawPath string, hasIfExists, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "DROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "DATABASE") {
		return
	}
	i++
	i, hasIfExists = skipIfExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchDropSchema matches DROP SCHEMA [IF EXISTS] <ident_path> and returns
// whether IF EXISTS was present.
func matchDropSchema(sig []sqltok.Token, sql string) (rawPath string, hasIfExists, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "DROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "SCHEMA") {
		return
	}
	i++
	i, hasIfExists = skipIfExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchUndropTable matches UNDROP TABLE <ident_path>.
func matchUndropTable(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "UNDROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "TABLE") {
		return
	}
	i++
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchUndropDbSch matches UNDROP {DATABASE|SCHEMA} <ident_path>.
func matchUndropDbSch(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "UNDROP") {
		return
	}
	i++
	if kwAtAny(sig, sql, i, "DATABASE", "SCHEMA") == "" {
		return
	}
	i++
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchUndropDB matches UNDROP DATABASE <ident>.
func matchUndropDB(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "UNDROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "DATABASE") {
		return
	}
	i++
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchUndropSchema matches UNDROP SCHEMA <ident_path>.
func matchUndropSchema(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "UNDROP") {
		return
	}
	i++
	if !kwAt(sig, sql, i, "SCHEMA") {
		return
	}
	i++
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// matchAlterTV matches:
//
//	ALTER {TABLE|VIEW} [IF EXISTS] <ident_path>
//
// Returns the raw ident path, the object keyword, whether IF EXISTS was
// present, and ok.
func matchAlterTV(sig []sqltok.Token, sql string) (rawPath, objKw string, hasIfExists, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "ALTER") {
		return
	}
	i++
	objKw = kwAtAny(sig, sql, i, "TABLE", "VIEW")
	if objKw == "" {
		return
	}
	i++
	i, hasIfExists = skipIfExists(sig, sql, i)
	rawPath, _ = readIdentPath(sig, sql, i)
	ok = rawPath != ""
	return
}

// useStmt holds the result of parsing a USE statement.
type useStmt struct {
	kind   string // "DATABASE", "SCHEMA", or "" for bare USE
	parts  int    // 1 or 2 (number of ident parts)
	ident1 string // raw text of first ident
	ident2 string // raw text of second ident (if 2-part)
}

// matchUse parses USE statements in all their forms:
//
//	USE DATABASE <ident>
//	USE SCHEMA <ident>[.<ident>]
//	USE <ident>[.<ident>]    (bare — no keyword)
//
// Returns the parsed result and ok.
func matchUse(sig []sqltok.Token, sql string) (u useStmt, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "USE") {
		return
	}
	i++
	if kwAt(sig, sql, i, "DATABASE") {
		i++
		u.kind = "DATABASE"
	} else if kwAt(sig, sql, i, "SCHEMA") {
		i++
		u.kind = "SCHEMA"
	}
	if i >= len(sig) || !isIdent(sig[i]) {
		return
	}
	u.ident1 = sig[i].Text(sql)
	u.parts = 1
	i++
	// Check for dot-separated second part
	if i+1 < len(sig) && sig[i].Kind == sqltok.Dot && isIdent(sig[i+1]) {
		u.ident2 = sig[i+1].Text(sql)
		u.parts = 2
		i += 2
	}
	// For bare USE <ident> (no keyword), reject reserved sub-commands
	if u.kind == "" {
		first := strings.ToUpper(u.ident1)
		if first == "DATABASE" || first == "SCHEMA" || first == "ROLE" || first == "WAREHOUSE" {
			return useStmt{}, false
		}
	}
	// Must be at end-of-statement (only semicolon or nothing left)
	if u.kind == "" || u.kind == "SCHEMA" {
		// For USE SCHEMA <ident> (1-part) and bare USE, the regex required [;\s]*$
		// This means no further significant tokens except maybe semicolons.
		for i < len(sig) {
			if sig[i].Kind != sqltok.Semicolon {
				// More tokens follow — for bare 1-part USE and 1-part USE SCHEMA
				// the old regex required end of string. For 2-part forms, trailing
				// tokens are OK.
				if u.parts == 1 {
					// Check the kind: bare USE and USE SCHEMA (1-part) required end
					if u.kind == "" || u.kind == "SCHEMA" {
						return useStmt{}, false
					}
				}
				break
			}
			i++
		}
	}
	ok = true
	return
}

// fromJoinKeywords are the keywords that precede a table reference in DML
// statements (for fallback table extraction).
var fromJoinKeywords = map[string]bool{
	"FROM": true, "JOIN": true, "USING": true, "UPDATE": true,
	"CLONE": true, "LIKE": true,
}

// fromJoinTwoPartKeywords are two-keyword combinations that precede table refs.
var fromJoinTwoPartKeywords = map[string]string{
	"MERGE":  "INTO",
	"INSERT": "INTO",
	"COPY":   "INTO",
	"THEN":   "INTO",
	"ELSE":   "INTO",
}

// findFromJoinTables scans significant tokens for FROM/JOIN/MERGE INTO/etc.
// keywords followed by identifier paths. Returns the raw path text for each.
// This replaces reFromJoinFallback.
func findFromJoinTables(sig []sqltok.Token, sql string) []string {
	var paths []string
	for i := 0; i < len(sig); i++ {
		u := tokUpper(sig[i], sql)
		if u == "" {
			continue
		}
		matched := false
		if fromJoinKeywords[u] {
			matched = true
			i++
		} else if second, ok := fromJoinTwoPartKeywords[u]; ok {
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == second {
				matched = true
				i += 2
			}
		}
		if matched && i < len(sig) && isIdent(sig[i]) {
			path, end := readIdentPath(sig, sql, i)
			if path != "" {
				paths = append(paths, path)
				i = end - 1 // loop will i++
			}
		}
	}
	return paths
}

// findDynAsSelect finds AS SELECT or AS WITH in the significant tokens
// (for CREATE DYNAMIC TABLE). Returns the byte offset of SELECT/WITH, or -1.
func findDynAsSelect(sig []sqltok.Token, sql string) int {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == "AS" {
			next := tokUpper(sig[i+1], sql)
			if next == "SELECT" || next == "WITH" {
				return sig[i+1].Start
			}
		}
	}
	return -1
}

// findCTENames finds all CTE names in a WITH query by looking for the
// pattern <ident> AS ( in the significant tokens. Returns a normalised
// set of CTE names.
func findCTENames(sig []sqltok.Token, sql string, ic bool) map[string]struct{} {
	names := make(map[string]struct{})
	for i := 0; i+2 < len(sig); i++ {
		if !isIdent(sig[i]) {
			continue
		}
		if tokUpper(sig[i+1], sql) == "AS" && sig[i+2].Kind == sqltok.LParen {
			names[normIdent(sig[i].Text(sql), ic)] = struct{}{}
		}
	}
	return names
}

// findSwapWith finds SWAP WITH <ident_path> in significant tokens.
// Returns the raw path text and ok.
func findSwapWith(sig []sqltok.Token, sql string) (rawPath string, ok bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) == "SWAP" && tokUpper(sig[i+1], sql) == "WITH" {
			path, _ := readIdentPath(sig, sql, i+2)
			if path != "" {
				return path, true
			}
		}
	}
	return
}

// matchCreateTablePre matches CREATE TABLE prefix and returns:
//   - rawPath: the raw ident path text
//   - parenOff: byte offset of the opening '(' after the table name, or -1
//   - ok: whether the pattern matched
//
// This replaces reCreateTablePreScan.
func matchCreateTablePre(sig []sqltok.Token, sql string) (rawPath string, parenOff int, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return
	}
	if kwAtAny(sig, sql, i, "LOCAL", "GLOBAL") != "" {
		i++
	}
	if kwAtAny(sig, sql, i, "TEMP", "TEMPORARY", "VOLATILE", "TRANSIENT") != "" {
		i++
	}
	if !kwAt(sig, sql, i, "TABLE") {
		return
	}
	i++
	i = skipIfNotExists(sig, sql, i)
	rawPath, i = readIdentPath(sig, sql, i)
	if rawPath == "" {
		return
	}
	if i < len(sig) && sig[i].Kind == sqltok.LParen {
		parenOff = sig[i].Start
		ok = true
	}
	return
}

// matchCreateTableGuard checks if the statement starts with CREATE TABLE
// (without requiring a paren). Replaces reCreateTableGuard.
func matchCreateTableGuard(sig []sqltok.Token, sql string) bool {
	i := 0
	if !kwAt(sig, sql, i, "CREATE") {
		return false
	}
	i++
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return false
	}
	if kwAtAny(sig, sql, i, "LOCAL", "GLOBAL") != "" {
		i++
	}
	if kwAtAny(sig, sql, i, "TEMP", "TEMPORARY", "VOLATILE", "TRANSIENT") != "" {
		i++
	}
	return kwAt(sig, sql, i, "TABLE")
}

// matchInsertColList matches INSERT [OVERWRITE] INTO <ident_path> (<col_list>)
// and returns the raw table path and column list text.
func matchInsertColList(sig []sqltok.Token, sql string) (tablePath, colListRaw string, ok bool) {
	i := 0
	if !kwAt(sig, sql, i, "INSERT") {
		return
	}
	i++
	if kwAt(sig, sql, i, "OVERWRITE") {
		i++
	}
	if !kwAt(sig, sql, i, "INTO") {
		return
	}
	i++
	tablePath, i = readIdentPath(sig, sql, i)
	if tablePath == "" {
		return
	}
	if i >= len(sig) || sig[i].Kind != sqltok.LParen {
		return
	}
	// Find the matching ')' and extract the column list text.
	parenStart := sig[i].Start
	depth := 0
	for j := i; j < len(sig); j++ {
		switch sig[j].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				// Column list is between the parens (exclusive)
				colListRaw = sql[parenStart+1 : sig[j].Start]
				ok = true
				return
			}
		}
	}
	return
}

// findReferences finds all REFERENCES <ident_path> [(<cols>)] patterns
// in the significant tokens. Returns a slice of (tablePath, colListRaw) pairs.
type refMatch struct {
	tablePath  string
	colListRaw string
}

func findReferences(sig []sqltok.Token, sql string) []refMatch {
	var matches []refMatch
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], sql) != "REFERENCES" {
			continue
		}
		i++
		path, end := readIdentPath(sig, sql, i)
		if path == "" {
			continue
		}
		i = end
		var cols string
		if i < len(sig) && sig[i].Kind == sqltok.LParen {
			parenStart := sig[i].Start
			depth := 0
			for j := i; j < len(sig); j++ {
				if sig[j].Kind == sqltok.LParen {
					depth++
				} else if sig[j].Kind == sqltok.RParen {
					depth--
					if depth == 0 {
						cols = sql[parenStart+1 : sig[j].Start]
						i = j + 1
						break
					}
				}
			}
		}
		matches = append(matches, refMatch{tablePath: path, colListRaw: cols})
	}
	return matches
}

// findSelectKWOffset finds the byte offset of the first SELECT keyword
// in the significant tokens. Returns -1 if not found.
func findSelectKWOffset(sig []sqltok.Token, sql string) int {
	for _, tok := range sig {
		if tokUpper(tok, sql) == "SELECT" {
			return tok.Start
		}
	}
	return -1
}

// findFromJoinWithAlias scans for FROM/JOIN keywords and extracts
// (tablePath, alias) pairs.
type tableAlias struct {
	tablePath string
	alias     string
}

func findFromJoinWithAlias(sig []sqltok.Token, sql string) []tableAlias {
	// Keywords that start a FROM/JOIN clause (single-word)
	singleKW := map[string]bool{
		"FROM": true, "JOIN": true, "UPDATE": true,
	}
	twoPartKW := map[string]string{
		"CROSS":  "JOIN",
		"INSERT": "INTO",
		"DELETE": "FROM",
		"MERGE":  "INTO",
	}

	var results []tableAlias
	for i := 0; i < len(sig); i++ {
		u := tokUpper(sig[i], sql)
		matched := false
		if singleKW[u] {
			matched = true
			i++
		} else if second, ok := twoPartKW[u]; ok {
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == second {
				matched = true
				i += 2
			}
		}
		if !matched || i >= len(sig) || !isIdent(sig[i]) {
			continue
		}
		path, end := readIdentPath(sig, sql, i)
		if path == "" {
			continue
		}
		i = end

		// Check for optional alias: [AS] <alias>. An implicit alias must be an
		// Identifier or QuotedIdent — never a bare keyword — so a following clause
		// keyword (FROM mytable WHERE …) is not captured as the alias.
		var alias string
		if i < len(sig) {
			if tokUpper(sig[i], sql) == "AS" {
				i++
				if i < len(sig) && isAliasTok(sig[i]) {
					alias = sig[i].Text(sql)
					i++
				}
			} else if isAliasTok(sig[i]) {
				alias = sig[i].Text(sql)
				i++
			}
		}
		results = append(results, tableAlias{tablePath: path, alias: alias})
		i-- // loop will i++
	}
	return results
}

// findFromJoinTables2 is like findFromJoinTables but uses the barecolrefs
// FROM/JOIN keyword set (includes TRUNCATE TABLE, DESCRIBE TABLE, etc.)
func findFromJoinTables2(sig []sqltok.Token, sql string) []string {
	singleKW := map[string]bool{
		"FROM": true, "JOIN": true, "UPDATE": true,
	}
	twoPartKW := map[string]string{
		"CROSS":    "JOIN",
		"INSERT":   "INTO",
		"TRUNCATE": "TABLE",
		"DELETE":   "FROM",
		"MERGE":    "INTO",
		"DESCRIBE": "TABLE",
		"DESC":     "TABLE",
	}
	// DESCRIBE/DESC VIEW is also checked
	twoPartKW2 := map[string]string{
		"DESCRIBE": "VIEW",
		"DESC":     "VIEW",
	}

	var paths []string
	for i := 0; i < len(sig); i++ {
		u := tokUpper(sig[i], sql)
		if u == "" {
			continue
		}
		matched := false
		if singleKW[u] {
			matched = true
			i++
		} else if second, ok := twoPartKW[u]; ok {
			if i+1 < len(sig) && tokUpper(sig[i+1], sql) == second {
				matched = true
				i += 2
			}
		}
		if !matched {
			if second, ok := twoPartKW2[u]; ok {
				if i+1 < len(sig) && tokUpper(sig[i+1], sql) == second {
					matched = true
					i += 2
				}
			}
		}
		if matched && i < len(sig) && isIdent(sig[i]) {
			path, end := readIdentPath(sig, sql, i)
			if path != "" {
				paths = append(paths, path)
				i = end - 1
			}
		}
	}
	return paths
}

// findAsAliases finds all AS <alias> patterns and returns the byte offsets
// of the AS keyword start and the alias identifier start.
type asAliasLoc struct {
	asStart    int // byte offset of AS keyword
	aliasStart int // byte offset of alias identifier
	aliasEnd   int // byte offset past alias identifier
}

func findAsAliases(sig []sqltok.Token, sql string) []asAliasLoc {
	var locs []asAliasLoc
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == "AS" && isIdent(sig[i+1]) {
			locs = append(locs, asAliasLoc{
				asStart:    sig[i].Start,
				aliasStart: sig[i+1].Start,
				aliasEnd:   sig[i+1].End,
			})
		}
	}
	return locs
}

// ── Dispatch guard factories ─────────────────────────────────────────────────
//
// These functions create guard predicates for the parseTextRoutes dispatch table.
// Each guard checks whether the first few significant tokens of a statement
// match a specific keyword pattern.

// guardKW returns a guard that matches when the first N significant tokens
// equal the given keywords (case-insensitive). For single-keyword guards
// like GRANT, REVOKE, CALL, SHOW, BEGIN, COMMIT, etc.
func guardKW(keywords ...string) func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		if len(sig) < len(keywords) {
			return false
		}
		for i, kw := range keywords {
			if tokUpper(sig[i], sql) != kw {
				return false
			}
		}
		return true
	}
}

// guardKWAlt returns a guard that matches when the first significant token
// equals any of the given alternatives. For guards like LIST|LS, REMOVE|RM,
// DESCRIBE|DESC.
func guardKWAlt(alts ...string) func([]sqltok.Token, string) bool {
	set := make(map[string]bool, len(alts))
	for _, a := range alts {
		set[a] = true
	}
	return func(sig []sqltok.Token, sql string) bool {
		return len(sig) > 0 && set[tokUpper(sig[0], sql)]
	}
}

// guardCreate returns a guard for CREATE [OR REPLACE] <objKWs...>.
// The OR REPLACE modifier is optional. For CREATE TASK, CREATE ALERT, etc.
func guardCreate(objKWs ...string) func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		i := 0
		if i >= len(sig) || tokUpper(sig[i], sql) != "CREATE" {
			return false
		}
		i++
		if i < len(sig) && tokUpper(sig[i], sql) == "OR" {
			i++ // skip OR
			if i < len(sig) {
				u := tokUpper(sig[i], sql)
				if u == "REPLACE" || u == "ALTER" {
					i++
				}
			}
		}
		for _, kw := range objKWs {
			if i >= len(sig) || tokUpper(sig[i], sql) != kw {
				return false
			}
			i++
		}
		return true
	}
}

// guardCreateWithMods returns a guard for CREATE with optional modifiers
// before the final object keyword(s). Used for patterns like:
// CREATE [OR REPLACE] [LOCAL|GLOBAL] [TEMP|TEMPORARY|VOLATILE|TRANSIENT] TABLE
func guardCreateWithMods(modSets [][]string, objKWs ...string) func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		i := 0
		if i >= len(sig) || tokUpper(sig[i], sql) != "CREATE" {
			return false
		}
		i++
		// Skip OR REPLACE / OR ALTER
		if i < len(sig) && tokUpper(sig[i], sql) == "OR" {
			i++
			if i < len(sig) {
				u := tokUpper(sig[i], sql)
				if u == "REPLACE" || u == "ALTER" {
					i++
				}
			}
		}
		// Check optional modifier groups
		for _, modSet := range modSets {
			if i < len(sig) {
				u := tokUpper(sig[i], sql)
				for _, m := range modSet {
					if u == m {
						i++
						break
					}
				}
			}
		}
		// Check required object keywords
		for _, kw := range objKWs {
			if i >= len(sig) || tokUpper(sig[i], sql) != kw {
				return false
			}
			i++
		}
		return true
	}
}

// guardAlter returns a guard for ALTER <objKWs...>.
func guardAlter(objKWs ...string) func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		i := 0
		if i >= len(sig) || tokUpper(sig[i], sql) != "ALTER" {
			return false
		}
		i++
		for _, kw := range objKWs {
			if i >= len(sig) || tokUpper(sig[i], sql) != kw {
				return false
			}
			i++
		}
		return true
	}
}

// guardDrop returns a guard for DROP <objKWs...>.
func guardDrop(objKWs ...string) func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		i := 0
		if i >= len(sig) || tokUpper(sig[i], sql) != "DROP" {
			return false
		}
		i++
		for _, kw := range objKWs {
			if i >= len(sig) || tokUpper(sig[i], sql) != kw {
				return false
			}
			i++
		}
		return true
	}
}

// guardCreateIntegration matches CREATE [OR REPLACE] (STORAGE|API|NOTIFICATION|SECURITY|EXTERNAL ACCESS) INTEGRATION.
func guardCreateIntegration() func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		i := 0
		if i >= len(sig) || tokUpper(sig[i], sql) != "CREATE" {
			return false
		}
		i++
		if i < len(sig) && tokUpper(sig[i], sql) == "OR" {
			i++
			if i < len(sig) && tokUpper(sig[i], sql) == "REPLACE" {
				i++
			}
		}
		if i >= len(sig) {
			return false
		}
		u := tokUpper(sig[i], sql)
		switch u {
		case "STORAGE", "API", "NOTIFICATION", "SECURITY":
			i++
		case "EXTERNAL":
			i++
			if i >= len(sig) || tokUpper(sig[i], sql) != "ACCESS" {
				return false
			}
			i++
		default:
			return false
		}
		return i < len(sig) && tokUpper(sig[i], sql) == "INTEGRATION"
	}
}

// guardExecuteService matches EXECUTE [JOB] SERVICE.
func guardExecuteService() func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		if len(sig) < 2 || tokUpper(sig[0], sql) != "EXECUTE" {
			return false
		}
		if tokUpper(sig[1], sql) == "SERVICE" {
			return true
		}
		return len(sig) >= 3 && tokUpper(sig[1], sql) == "JOB" && tokUpper(sig[2], sql) == "SERVICE"
	}
}

// guardWithProcAlias matches WITH <ident> AS PROCEDURE.
func guardWithProcAlias() func([]sqltok.Token, string) bool {
	return func(sig []sqltok.Token, sql string) bool {
		return len(sig) >= 4 &&
			tokUpper(sig[0], sql) == "WITH" &&
			isIdent(sig[1]) &&
			tokUpper(sig[2], sql) == "AS" &&
			tokUpper(sig[3], sql) == "PROCEDURE"
	}
}

// guardInsertVariant matches INSERT [OVERWRITE] <target> where target is
// ALL or FIRST.
func guardInsertVariant(sig []sqltok.Token, sql string, target string) bool {
	if len(sig) < 2 || tokUpper(sig[0], sql) != "INSERT" {
		return false
	}
	i := 1
	if tokUpper(sig[i], sql) == "OVERWRITE" {
		i++
	}
	return i < len(sig) && tokUpper(sig[i], sql) == target
}

// guardAlterTableAction matches ALTER TABLE [IF EXISTS] <ident_path> <actionKWs...>.
func guardAlterTableAction(sig []sqltok.Token, sql string, actionKWs ...string) bool {
	if len(sig) < 2 || tokUpper(sig[0], sql) != "ALTER" || tokUpper(sig[1], sql) != "TABLE" {
		return false
	}
	i := 2
	// Skip optional IF EXISTS
	if i < len(sig) && tokUpper(sig[i], sql) == "IF" {
		i++
		if i < len(sig) && tokUpper(sig[i], sql) == "EXISTS" {
			i++
		}
	}
	// Skip identifier path
	if i >= len(sig) || !isIdent(sig[i]) {
		return false
	}
	_, i = readIdentPath(sig, sql, i)
	// Check action keywords
	for _, kw := range actionKWs {
		if i >= len(sig) || tokUpper(sig[i], sql) != kw {
			return false
		}
		i++
	}
	return true
}

// guardAlterTableSearchOpt matches ALTER TABLE [IF EXISTS] <ident_path> (ADD|DROP) SEARCH OPTIMIZATION.
func guardAlterTableSearchOpt(sig []sqltok.Token, sql string) bool {
	if len(sig) < 2 || tokUpper(sig[0], sql) != "ALTER" || tokUpper(sig[1], sql) != "TABLE" {
		return false
	}
	i := 2
	if i < len(sig) && tokUpper(sig[i], sql) == "IF" {
		i++
		if i < len(sig) && tokUpper(sig[i], sql) == "EXISTS" {
			i++
		}
	}
	if i >= len(sig) || !isIdent(sig[i]) {
		return false
	}
	_, i = readIdentPath(sig, sql, i)
	if i >= len(sig) {
		return false
	}
	u := tokUpper(sig[i], sql)
	if u != "ADD" && u != "DROP" {
		return false
	}
	i++
	return i+1 < len(sig) && tokUpper(sig[i], sql) == "SEARCH" && tokUpper(sig[i+1], sql) == "OPTIMIZATION"
}

// checkOptionValue scans tokens for a <keyword> = <value> pattern and calls
// validate with the value text. Returns a DiagMarker if validate returns a
// non-empty error string. This is a generic helper for PUT/GET option checks
// (PARALLEL, SOURCE_COMPRESSION, OVERWRITE, AUTO_COMPRESS, etc.).
func checkOptionValue(toks []sqltok.Token, sql string, r StatementRange, optionKW string, validate func(string) string) []DiagMarker {
	for i, t := range toks {
		if (t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier) &&
			strings.EqualFold(t.Text(sql), optionKW) {
			// Look for = then value token.
			j := sqltok.SkipTrivia(toks, i+1)
			if j < len(toks) && toks[j].Kind == sqltok.Operator && toks[j].Text(sql) == "=" {
				j = sqltok.SkipTrivia(toks, j+1)
				if j < len(toks) {
					val := toks[j].Text(sql)
					if msg := validate(val); msg != "" {
						return []DiagMarker{diagMarkerSpan(r, msg)}
					}
				}
			}
			break
		}
	}
	return nil
}

// ── OR REPLACE / IF NOT EXISTS conflict ────────────────────────────────────────

// checkOrReplaceConflictTok returns a diagnostic and true if both OR REPLACE and
// IF NOT EXISTS modifiers are present in the significant token stream.
func checkOrReplaceConflictTok(sig []sqltok.Token, sql string, r StatementRange, stmtType string) (DiagMarker, bool) {
	hasOrReplace := hasKWPair(sig, sql, "OR", "REPLACE")
	hasIfNotExists := hasKWSeq(sig, sql, "IF", "NOT", "EXISTS")
	if hasOrReplace && hasIfNotExists {
		return diagMarkerSpan(r,
				"Conflict between OR REPLACE and IF NOT EXISTS in "+stmtType+" statement."),
			true
	}
	return DiagMarker{}, false
}

// ── Property value extraction helpers ─────────────────────────────────────────

// findKWAssign scans significant tokens for KEYWORD = VALUE and returns the
// raw value token text and true. Returns ("", false) if not found.
// This replaces regex patterns like `\bKEYWORD\s*=\s*(\w+)`.
func findKWAssign(sig []sqltok.Token, sql, keyword string) (string, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" {
			// Return the raw text of the value token.
			return sig[i+2].Text(sql), true
		}
	}
	return "", false
}

// hasKWAssign checks if KEYWORD = is present in significant tokens (any value).
// This replaces regex patterns like `\bKEYWORD\s*=`.
func hasKWAssign(sig []sqltok.Token, sql, keyword string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == keyword &&
			sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" {
			return true
		}
	}
	return false
}

// findKWAssignInt scans significant tokens for KEYWORD = [-]<number> and returns
// the integer value and true. Handles optional minus sign (which is a separate
// Operator token). Returns (0, false) if not found.
func findKWAssignInt(sig []sqltok.Token, sql, keyword string) (int, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" {
			j := i + 2
			neg := false
			if j < len(sig) && sig[j].Kind == sqltok.Operator && sig[j].Text(sql) == "-" {
				neg = true
				j++
			}
			if j < len(sig) && sig[j].Kind == sqltok.NumberLit {
				v, err := strconv.Atoi(sig[j].Text(sql))
				if err == nil {
					if neg {
						v = -v
					}
					return v, true
				}
			}
		}
	}
	return 0, false
}

// findKWAssignStr scans significant tokens for KEYWORD = 'value' and returns
// the unquoted string value and true. Returns ("", false) if not found or if
// the value is not a string literal.
func findKWAssignStr(sig []sqltok.Token, sql, keyword string) (string, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" {
			if sig[i+2].Kind == sqltok.StringLit {
				raw := sig[i+2].Text(sql)
				// Strip surrounding quotes.
				if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
					return raw[1 : len(raw)-1], true
				}
			}
			return "", false
		}
	}
	return "", false
}

// checkNameSwallowedByIFTok detects the case where the name extraction captured
// "IF" as the object name because the IF [NOT] EXISTS clause consumed the
// actual name slot. Returns the error marker and true if the name was swallowed.
func checkNameSwallowedByIFTok(name string, sig []sqltok.Token, sql string, r StatementRange, errMsg string) (DiagMarker, bool) {
	if strings.EqualFold(name, "IF") &&
		(hasKWSeq(sig, sql, "IF", "NOT", "EXISTS") || hasKWPair(sig, sql, "IF", "EXISTS")) {
		return diagMarkerSpan(r, errMsg), true
	}
	return DiagMarker{}, false
}

// ── DROP / CREATE name extraction helpers ─────────────────────────────────────

// extractNameAfterKeywords scans sig for the keyword sequence kws, skips
// optional IF [NOT] EXISTS, and returns the identifier path and its index.
// Returns ("", -1) if the name is not found.
func extractNameAfterKeywords(sig []sqltok.Token, sql string, kws ...string) (name string, nameIdx int) {
	i := 0
	for _, kw := range kws {
		if i >= len(sig) || tokUpper(sig[i], sql) != kw {
			return "", -1
		}
		i++
	}
	// Skip optional IF [NOT] EXISTS.
	if i < len(sig) && tokUpper(sig[i], sql) == "IF" {
		i++
		if i < len(sig) && tokUpper(sig[i], sql) == "NOT" {
			i++
		}
		if i < len(sig) && tokUpper(sig[i], sql) == "EXISTS" {
			i++
		}
	}
	// Read identifier path — reject empty quoted identifiers.
	if i >= len(sig) || !isNonEmptyIdent(sig[i], sql) {
		return "", -1
	}
	path, end := readIdentPath(sig, sql, i)
	if path == "" {
		return "", -1
	}
	_ = end
	return path, i
}

// extractNameAfterCreate handles CREATE [OR REPLACE] [modifiers...] <obj_kws> [IF NOT EXISTS] <name>.
// modKWs are optional modifier keywords (e.g., "TRANSIENT", "TEMPORARY").
// objKWs are the required object type keywords (e.g., "TABLE", "IMAGE", "REPOSITORY").
func extractNameAfterCreate(sig []sqltok.Token, sql string, modKWs []string, objKWs ...string) (name string, nameIdx int) {
	i := 0
	if i >= len(sig) || tokUpper(sig[i], sql) != "CREATE" {
		return "", -1
	}
	i++
	// Optional OR REPLACE / OR ALTER.
	i, orOK := skipCreateOr(sig, sql, i)
	if !orOK {
		return "", -1
	}
	// Optional modifiers.
	for _, mod := range modKWs {
		if kwAt(sig, sql, i, mod) {
			i++
		}
	}
	// Required object type keywords.
	for _, kw := range objKWs {
		if i >= len(sig) || tokUpper(sig[i], sql) != kw {
			return "", -1
		}
		i++
	}
	// Skip an IF NOT EXISTS clause — only the full three-keyword sequence, and
	// only when a non-empty ident follows (so "IF EXISTS" or a bare "IF" is not
	// mistaken for the clause).
	if kwAt(sig, sql, i, "IF") && kwAt(sig, sql, i+1, "NOT") && kwAt(sig, sql, i+2, "EXISTS") &&
		i+3 < len(sig) && isNonEmptyIdent(sig[i+3], sql) {
		i += 3
	}
	// Read identifier path — reject empty quoted identifiers.
	if i >= len(sig) || !isNonEmptyIdent(sig[i], sql) {
		return "", -1
	}
	path, end := readIdentPath(sig, sql, i)
	if path == "" {
		return "", -1
	}
	_ = end
	return path, i
}

// ── GRANT/REVOKE helpers ──────────────────────────────────────────────────────

// hasKW checks if a keyword appears anywhere in the significant token stream.
func hasKW(sig []sqltok.Token, sql, kw string) bool {
	for _, t := range sig {
		if tokUpper(t, sql) == kw {
			return true
		}
	}
	return false
}

// hasKWPair checks if keyword kw1 is immediately followed by kw2 in the
// significant token stream.
func hasKWPair(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			return true
		}
	}
	return false
}

// hasKWPairAny checks if keyword kw1 is immediately followed by any of alts
// in the significant token stream.
func hasKWPairAny(sig []sqltok.Token, sql, kw1 string, alts []string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 {
			u := tokUpper(sig[i+1], sql)
			for _, a := range alts {
				if u == a {
					return true
				}
			}
		}
	}
	return false
}

// hasKWSeq checks if a keyword sequence kw1 kw2 kw3 appears in sig.
func hasKWSeq(sig []sqltok.Token, sql, kw1, kw2, kw3 string) bool {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 && tokUpper(sig[i+2], sql) == kw3 {
			return true
		}
	}
	return false
}

// hasKWSeq4 reports whether four consecutive keyword tokens kw1 kw2 kw3 kw4
// appear in sig in that order.
func hasKWSeq4(sig []sqltok.Token, sql, kw1, kw2, kw3, kw4 string) bool {
	for i := 0; i+3 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 &&
			tokUpper(sig[i+2], sql) == kw3 && tokUpper(sig[i+3], sql) == kw4 {
			return true
		}
	}
	return false
}

// hasFromSpecKW checks for FROM [@stage] <specKW> pattern where specKW is one
// of the given keywords. The optional @stage reference between FROM and the
// keyword is skipped. Since stage paths are identifier sequences that cannot
// be distinguished from the specification keyword by kind alone, we scan
// forward from FROM looking for any matching keyword within a reasonable
// window (up to 8 tokens covers @db.schema.stage + the spec keyword).
func hasFromSpecKW(sig []sqltok.Token, sql string, specKWs []string) bool {
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], sql) != "FROM" {
			continue
		}
		// Scan forward up to 8 tokens for a matching specification keyword.
		limit := i + 9
		if limit > len(sig) {
			limit = len(sig)
		}
		for j := i + 1; j < limit; j++ {
			u := tokUpper(sig[j], sql)
			for _, kw := range specKWs {
				if u == kw {
					return true
				}
			}
		}
	}
	return false
}

// findKWFollowedByIdent finds KEYWORD <ident> and returns the ident text.
// Used for patterns like LANGUAGE PYTHON.
func findKWFollowedByIdent(sig []sqltok.Token, sql, keyword string) (string, bool) {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == keyword && isIdent(sig[i+1]) {
			return sig[i+1].Text(sql), true
		}
	}
	return "", false
}

// hasKWPairAssignIdent checks for KW1 KW2 = <ident> pattern.
// Used for patterns like ADD ACCOUNTS = org.acct.
func hasKWPairAssignIdent(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+3 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			if sig[i+2].Kind == sqltok.Operator && sig[i+2].Text(sql) == "=" {
				if isIdent(sig[i+3]) {
					return true
				}
			}
		}
	}
	return false
}

// hasKWPairFollowedByIdent checks for KW1 KW2 <ident> pattern (no = sign).
// Used for patterns like ADD DATABASES mydb.
func hasKWPairFollowedByIdent(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			if isIdent(sig[i+2]) {
				return true
			}
		}
	}
	return false
}

// hasGrantee checks for TO/FROM ROLE|USER|DATABASE ROLE|SHARE pattern.
func hasGrantee(sig []sqltok.Token, sql, preposition string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == preposition {
			u := tokUpper(sig[i+1], sql)
			if u == "ROLE" || u == "USER" || u == "SHARE" {
				return true
			}
			if u == "DATABASE" && i+2 < len(sig) && tokUpper(sig[i+2], sql) == "ROLE" {
				return true
			}
		}
	}
	return false
}

// grantObjTypeKWs lists compound object types used in GRANT/REVOKE ON clauses.
// Order matters: longer (multi-word) types are checked first.
var grantObjTypeKWs = [][]string{
	{"EXTERNAL", "TABLE"},
	{"MATERIALIZED", "VIEW"},
	{"HYBRID", "TABLE"},
	{"ICEBERG", "TABLE"},
	{"DYNAMIC", "TABLE"},
	{"FILE", "FORMAT"},
}

// findOnObjectClause scans sig for the pattern:
//
//	[GRANT OPTION FOR] <privileges> ON [ALL|FUTURE] <object_type>
//
// and returns the raw privilege substring, the ALL/FUTURE modifier (or ""),
// the normalised object type, and ok=true.
func findOnObjectClause(sig []sqltok.Token, sql string) (privListRaw, allFuture, objectType string, ok bool) {
	// Find ON keyword position.
	onIdx := -1
	for i := 1; i < len(sig); i++ {
		if tokUpper(sig[i], sql) == "ON" {
			onIdx = i
			break
		}
	}
	if onIdx < 0 || onIdx+1 >= len(sig) {
		return
	}

	// Extract privilege list: text between first token after GRANT/REVOKE and ON.
	// Skip leading GRANT OPTION FOR if present (REVOKE only).
	privStart := 1
	if privStart < onIdx && tokUpper(sig[privStart], sql) == "GRANT" &&
		privStart+1 < onIdx && tokUpper(sig[privStart+1], sql) == "OPTION" &&
		privStart+2 < onIdx && tokUpper(sig[privStart+2], sql) == "FOR" {
		privStart += 3
	}
	if privStart >= onIdx {
		return
	}
	privListRaw = sql[sig[privStart].Start:sig[onIdx-1].End]

	// After ON: optional ALL/FUTURE, then object type.
	j := onIdx + 1
	u := tokUpper(sig[j], sql)
	if u == "ALL" || u == "FUTURE" {
		allFuture = u
		j++
		if j >= len(sig) {
			return
		}
	}

	// Try compound types first.
	for _, compound := range grantObjTypeKWs {
		match := true
		for k, kw := range compound {
			if j+k >= len(sig) || tokUpper(sig[j+k], sql) != kw {
				match = false
				break
			}
		}
		if match {
			raw := sql[sig[j].Start:sig[j+len(compound)-1].End]
			objectType = normalizeGrantObjectType(raw)
			ok = true
			return
		}
	}

	// Single-word type.
	if isIdent(sig[j]) {
		objectType = normalizeGrantObjectType(sig[j].Text(sql))
		ok = true
	}
	return
}

// ── Policy / clause helpers ──────────────────────────────────────────────────

// findKWLParen returns the index of the first occurrence of KEYWORD (
// in sig, or -1 if not found. This is useful for finding AS (, FROM (, etc.
func findKWLParen(sig []sqltok.Token, sql, keyword string) int {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == keyword && sig[i+1].Kind == sqltok.LParen {
			return i
		}
	}
	return -1
}

// extractParenContentTok extracts the raw SQL text inside the parentheses
// following the keyword at index kwIdx in sig. It handles nested parens.
// For example, given "AS ( a VARCHAR, b NUMBER(10,2) )", it returns
// " a VARCHAR, b NUMBER(10,2) ".
func extractParenContentTok(sig []sqltok.Token, sql string, kwIdx int) string {
	if kwIdx+1 >= len(sig) || sig[kwIdx+1].Kind != sqltok.LParen {
		return ""
	}
	openPos := sig[kwIdx+1].End // byte position after the opening (
	depth := 1
	for i := kwIdx + 2; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return sql[openPos:sig[i].Start]
			}
		}
	}
	// Unterminated — return everything after the opening paren.
	return sql[openPos:]
}

// parenInnerRange returns the half-open token range [start, end) strictly inside
// the parenthesised group whose opening "(" is sig[openIdx], plus closeIdx (the
// index of the matching ")"). start == openIdx+1 and end == closeIdx. ok is
// false if sig[openIdx] is not "(" or the group is unterminated. It replaces the
// recurring inline "scan for the matching close paren and slice the body" loop.
func parenInnerRange(sig []sqltok.Token, openIdx int) (start, closeIdx int, ok bool) {
	if openIdx < 0 || openIdx >= len(sig) || sig[openIdx].Kind != sqltok.LParen {
		return 0, 0, false
	}
	depth := 0
	for j := openIdx; j < len(sig); j++ {
		switch sig[j].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return openIdx + 1, j, true
			}
		}
	}
	return 0, 0, false
}

// findKWAssignParenContent scans for KEYWORD = ( ... ) and returns the raw
// text inside the parentheses. This replaces regex patterns like
// `\bKEYWORD\s*=\s*\(([^)]*)\)`.
func findKWAssignParenContent(sig []sqltok.Token, sql, keyword string) string {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" &&
			sig[i+2].Kind == sqltok.LParen {
			return extractParenContentTok(sig, sql, i+1)
		}
	}
	return ""
}

// findKWFatArrowInt scans for KEYWORD => <integer> and returns the integer value.
// This replaces regex patterns like `\bKEYWORD\s*=>\s*(-?\d+)`.
func findKWFatArrowInt(sig []sqltok.Token, sql, keyword string) (int, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=>" {
			// Check for optional negative sign.
			j := i + 2
			neg := false
			if j < len(sig) && sig[j].Kind == sqltok.Operator && sig[j].Text(sql) == "-" {
				neg = true
				j++
			}
			if j < len(sig) && sig[j].Kind == sqltok.NumberLit {
				val, err := strconv.Atoi(sig[j].Text(sql))
				if err == nil {
					if neg {
						val = -val
					}
					return val, true
				}
			}
		}
	}
	return 0, false
}

// findKWFatArrowStr scans for KEYWORD => '<string>' and returns the unquoted string.
func findKWFatArrowStr(sig []sqltok.Token, sql, keyword string) (string, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=>" {
			if sig[i+2].Kind == sqltok.StringLit {
				raw := sig[i+2].Text(sql)
				if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
					return raw[1 : len(raw)-1], true
				}
				return raw, true
			}
		}
	}
	return "", false
}

// hasKWPairArrow checks if KEYWORD1 KEYWORD2 -> is present in sig.
// This replaces regex patterns like `RETURNS\s+BOOLEAN\s*->`.
func hasKWPairArrow(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			// Check that next token is the -> operator.
			// Note: the tokenizer emits - and > as separate Operator tokens,
			// so we check for "-" followed by ">" or a single "->" token.
			if i+2 < len(sig) && sig[i+2].Kind == sqltok.Operator {
				op := sig[i+2].Text(sql)
				if op == "-" {
					// Check for > as next token
					if i+3 < len(sig) && sig[i+3].Kind == sqltok.Operator && sig[i+3].Text(sql) == ">" {
						return true
					}
				}
				// Unlikely but handle if tokenizer produces -> as one token
				if op == "->" {
					return true
				}
			}
			// Also check => (fat arrow)
			if i+2 < len(sig) && sig[i+2].Kind == sqltok.Operator && sig[i+2].Text(sql) == "=>" {
				return true
			}
		}
	}
	return false
}

// isValidTargetLagDuration checks if s is a valid TARGET_LAG duration string
// (without the surrounding quotes). Valid format: '<positive_int> <unit>'
// where unit is one of seconds, minutes, hours, days (singular or plural).
func isValidTargetLagDuration(s string) bool {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 1 {
		return false
	}
	unit := strings.ToLower(parts[1])
	switch unit {
	case "second", "seconds", "minute", "minutes", "hour", "hours", "day", "days":
		return true
	}
	return false
}

// extractObjectTypesValue extracts the OBJECT_TYPES = <value> portion from sig,
// collecting all tokens after the = sign until the next known boundary keyword
// (ALLOWED_*, IGNORE, REPLICATION_SCHEDULE) or end of tokens. Returns the
// uppercased concatenation of those tokens.
func extractObjectTypesValue(sig []sqltok.Token, sql string) string {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) != "OBJECT_TYPES" {
			continue
		}
		if sig[i+1].Kind != sqltok.Operator || sig[i+1].Text(sql) != "=" {
			continue
		}
		// Collect tokens after = until a boundary keyword.
		var parts []string
		for j := i + 2; j < len(sig); j++ {
			u := tokUpper(sig[j], sql)
			if strings.HasPrefix(u, "ALLOWED_") || u == "IGNORE" || u == "REPLICATION_SCHEDULE" {
				break
			}
			if u != "" {
				parts = append(parts, u)
			} else {
				// Non-ident tokens (commas, parens, etc.) — include raw text.
				parts = append(parts, strings.ToUpper(sig[j].Text(sql)))
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// hasKWSeqFollowedByIdent checks if KW1 KW2 KW3 <ident> is present in sig.
func hasKWSeqFollowedByIdent(sig []sqltok.Token, sql, kw1, kw2, kw3 string) bool {
	for i := 0; i+3 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 &&
			tokUpper(sig[i+1], sql) == kw2 &&
			tokUpper(sig[i+2], sql) == kw3 &&
			isIdent(sig[i+3]) {
			return true
		}
	}
	return false
}
