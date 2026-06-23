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

// ── Statement guard factories ─────────────────────────────────────────────────
//
// These functions create guard predicates that check whether the first few
// significant tokens of a statement match a specific keyword pattern.

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

// ── Policy / clause helpers ──────────────────────────────────────────────────

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
