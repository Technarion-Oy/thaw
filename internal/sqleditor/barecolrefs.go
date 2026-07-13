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

// ── Types ─────────────────────────────────────────────────────────────────────

// ValidateBareColsRequest is the input to ValidateBareColumnRefs.
type ValidateBareColsRequest struct {
	SQL                         string           `json:"sql"`
	StmtRanges                  []StatementRange `json:"stmtRanges"`
	ResolvedRefs                []ResolvedRef    `json:"resolvedRefs"`
	ColEntries                  []ColEntry       `json:"colEntries"`
	QuotedIdentifiersIgnoreCase bool             `json:"quotedIdentifiersIgnoreCase"`
}

// ── Precompiled regexes & Maps ────────────────────────────────────────────────

var (
	// Date parts and functions for context-aware bare column skipping
	bcrDateParts = map[string]bool{
		"YEAR": true, "MONTH": true, "DAY": true, "HOUR": true, "MINUTE": true,
		"SECOND": true, "QUARTER": true, "WEEK": true, "MILLISECOND": true,
		"MICROSECOND": true, "NANOSECOND": true, "DAYOFWEEK": true,
		"DAYOFMONTH": true, "DAYOFYEAR": true, "EPOCH": true,
	}

	bcrDateFuncs = map[string]bool{
		"DATEADD": true, "DATEDIFF": true, "DATE_TRUNC": true, "DATE_PART": true,
		"EXTRACT": true, "TIMEADD": true, "TIMEDIFF": true, "TIMESTAMPADD": true,
		"TIMESTAMPDIFF": true,
	}
)

// ── ValidateBareColumnRefs ────────────────────────────────────────────────────

// ValidateBareColumnRefs is the Go port of validateBareColumnRefs in
// sqlDiagnostics.ts.  Because no SQL AST parser is available in Go, the
// function performs:
//
//  1. A pre-scan that builds a local column cache from CREATE TABLE statements
//     in the script (so subsequent INSERT or REFERENCES can find columns even
//     if the table is not yet in Snowflake). ALTER TABLE … ADD [COLUMN] effects
//     are merged into that cache so later references to added columns resolve
//     (issue #715).
//  2. Column validation for INSERT column lists:
//     INSERT INTO t (col1, col2) VALUES (...)
//  3. Column validation for FK REFERENCES column lists inside CREATE TABLE:
//     col1 INT REFERENCES other_table (ref_col)
//
// SELECT column validation is intentionally skipped: without an AST it cannot
// be done reliably (table-alias false-positives, AS aliases, sub-selects, etc.)
// — matching the TS fallback behavior when node-sql-parser fails to parse.
//
// Severity: 4 = Monaco Warning (yellow squiggles).
func ValidateBareColumnRefs(req ValidateBareColsRequest) []DiagMarker {
	ic := req.QuotedIdentifiersIgnoreCase
	checkEq := func(a, b string) bool {
		if ic {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	// Build colInfoCache from caller-supplied ColEntries.
	colInfoCache := make(map[string][]ColInfo, len(req.ColEntries))
	for _, e := range req.ColEntries {
		key := bcrCacheKey(strings.ToUpper(e.DB), strings.ToUpper(e.Schema), strings.ToUpper(e.Name))
		colInfoCache[key] = e.Cols
	}

	// ── Pre-scan: extract columns from CREATE TABLE statements ────────────
	localColCache := make(map[string][]ColInfo)
	for _, r := range req.StmtRanges {
		raw := sqlStmt(req.SQL, r)
		tokens := sqltok.Tokenize(raw)
		sig := sigToks(tokens)

		rawPath, parenOff, ok := matchCreateTablePre(sig, raw)
		if !ok {
			// Apply ALTER TABLE … ADD [COLUMN] to tables already created in-script
			// so later INSERT/SELECT can find the added columns (issue #715).
			if aPath, aCols, aok := parseAlterAddColumns(sig, raw, ic); aok {
				applyAlterAddToLocalCache(aPath, aCols, localColCache, ic)
			}
			continue
		}
		parts := extractIdentParts(rawPath, ic)
		if len(parts) == 0 {
			continue
		}

		// Extract balanced column block starting at the opening paren.
		colsRaw := extractBalancedBlock(raw, parenOff)
		if colsRaw == "" {
			continue
		}
		// Strip the surrounding parens.
		if len(colsRaw) >= 2 {
			colsRaw = colsRaw[1 : len(colsRaw)-1]
		}
		columns := parseCreateTableColDefs(colsRaw, ic)

		tableName := parts[len(parts)-1]
		// 1-part key (table name only)
		localColCache[bcrCacheKey("", "", tableName)] = columns
		if len(parts) >= 2 {
			schema := parts[len(parts)-2]
			// 2-part key (schema.table)
			localColCache[bcrCacheKey("", schema, tableName)] = columns
		}
		if len(parts) >= 3 {
			db := parts[len(parts)-3]
			schema := parts[len(parts)-2]
			// 3-part key (db.schema.table)
			localColCache[bcrCacheKey(db, schema, tableName)] = columns
		}
	}

	// ── Second pass: validate column refs ─────────────────────────────────
	var markers []DiagMarker

	for _, r := range req.StmtRanges {
		raw := sqlStmt(req.SQL, r)
		baseCol := stmtStartCol(req.SQL, r) // doc column of the statement's first char
		firstTok := getFirstSQLToken(raw)

		if firstTok != "SELECT" && firstTok != "WITH" &&
			firstTok != "INSERT" && firstTok != "CREATE" && firstTok != "UNDROP" {
			continue
		}

		// False-positive guard: skip statements with Snowflake-specific syntax
		// that would produce noise.
		if matchesSnowflakeFP(sigTokens(raw), raw) {
			continue
		}

		switch firstTok {
		case "INSERT":
			markers = append(markers,
				validateInsertCols(raw, r, baseCol, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)

		case "CREATE":
			markers = append(markers,
				validateReferencesCols(raw, r, baseCol, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
			if isCreateView(raw) {
				markers = append(markers,
					validateSelectCols(raw, r, baseCol, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
			}

		case "SELECT", "WITH":
			markers = append(markers,
				validateSelectCols(raw, r, baseCol, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
		}
	}

	return markers
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// bcrCacheKey returns the null-separated cache key used by colInfoCache /
// localColCache.  All parts should already be normalised (upper-cased when
// applicable) by the caller.
func bcrCacheKey(db, schema, table string) string {
	return db + "\x00" + schema + "\x00" + table
}

// extractBalancedBlock returns the substring of s starting at openIdx
// (which must be '(') up to and including the matching closing ')'.
// Returns "" if the parentheses are unbalanced.
func extractBalancedBlock(s string, openIdx int) string {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '(' {
		return ""
	}
	depth := 0
	inSingle := false
	inDouble := false
	for i := openIdx; i < len(s); i++ {
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
				depth--
				if depth == 0 {
					return s[openIdx : i+1]
				}
			}
		}
	}
	return "" // unbalanced
}

// parseCreateTableColDefs splits a raw column-definition block (the text
// between CREATE TABLE name( ... )) into individual ColInfo entries.
func parseCreateTableColDefs(colsRaw string, ic bool) []ColInfo {
	var columns []ColInfo
	for _, def := range splitColDefs(colsRaw) {
		if col, ok := parseFirstIdentAsCol(def, ic); ok {
			columns = append(columns, col)
		}
	}
	return columns
}

// splitColDefs splits a raw column-definition block into individual
// comma-separated definition strings. It handles comments (-- and /* */),
// double-quoted identifiers with special characters, and single-quoted string
// literals so that commas and parentheses inside those contexts do not
// interfere with column splitting.
func splitColDefs(colsRaw string) []string {
	var defs []string
	depth := 0
	inDouble := false
	inSingle := false
	var current strings.Builder

	for i := 0; i < len(colsRaw); i++ {
		c := colsRaw[i]

		// Skip line comments (-- to end of line).
		if !inDouble && !inSingle && c == '-' && i+1 < len(colsRaw) && colsRaw[i+1] == '-' {
			for i < len(colsRaw) && colsRaw[i] != '\n' {
				i++
			}
			// Write the newline so line structure is preserved.
			if i < len(colsRaw) {
				current.WriteByte('\n')
			}
			continue
		}

		// Skip block comments (/* ... */).
		// NOTE: if the comment is never closed, the rest of the column definitions
		// are silently consumed — acceptable since the SQL is already invalid.
		if !inDouble && !inSingle && c == '/' && i+1 < len(colsRaw) && colsRaw[i+1] == '*' {
			i += 2
			for i < len(colsRaw) {
				if colsRaw[i] == '*' && i+1 < len(colsRaw) && colsRaw[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			current.WriteByte(' ') // Replace comment with space.
			continue
		}

		// Track double-quoted identifiers.  Handle escaped quotes ("") inside
		// quoted identifiers — Snowflake uses "" to embed a literal double-quote.
		if !inSingle && c == '"' {
			if inDouble && i+1 < len(colsRaw) && colsRaw[i+1] == '"' {
				// Escaped quote inside identifier: write both and stay in double-quote mode.
				current.WriteByte(c)
				current.WriteByte(c)
				i++
				continue
			}
			inDouble = !inDouble
			current.WriteByte(c)
			continue
		}

		// Track single-quoted string literals (for DEFAULT values etc.).
		// Handle escaped quotes ('') inside string literals.
		if !inDouble && c == '\'' {
			if inSingle && i+1 < len(colsRaw) && colsRaw[i+1] == '\'' {
				// Escaped quote inside string: write both and stay in single-quote mode.
				current.WriteByte(c)
				current.WriteByte(c)
				i++
				continue
			}
			inSingle = !inSingle
			current.WriteByte(c)
			continue
		}

		// Inside a quoted context, write everything verbatim.
		if inDouble || inSingle {
			current.WriteByte(c)
			continue
		}

		switch {
		case c == '(':
			depth++
			current.WriteByte(c)
		case c == ')':
			if depth > 0 {
				depth--
				current.WriteByte(c)
			}
			// depth == 0 means we're past the outer paren — shouldn't happen if
			// the caller already stripped the outer parens, but guard anyway.
		case c == ',' && depth == 0:
			if def := strings.TrimSpace(current.String()); def != "" {
				defs = append(defs, def)
			}
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	if def := strings.TrimSpace(current.String()); def != "" {
		defs = append(defs, def)
	}
	return defs
}

func parseFirstIdentAsCol(def string, ic bool) (ColInfo, bool) {
	// Tokenize the column definition and take the first identifier.
	tokens := sqltok.Tokenize(def)
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind.IsTrivia() {
			continue
		}
		if isIdent(tok) {
			return ColInfo{Name: normIdent(tok.Text(def), ic), DataType: "UNKNOWN"}, true
		}
		break // first non-WS token is not an identifier
	}
	return ColInfo{}, false
}

// nonColumnAddKw are the words that begin a non-column ALTER TABLE … ADD clause
// (ADD CONSTRAINT / PRIMARY KEY / …). A parsed "column" whose name is one of
// these is dropped so constraints aren't cached as columns.
var nonColumnAddKw = map[string]bool{
	"CONSTRAINT": true, "PRIMARY": true, "FOREIGN": true,
	"UNIQUE": true, "CHECK": true,
}

// stripAddColItemPrefix removes a leading COLUMN keyword and/or IF NOT EXISTS
// from a single comma-separated ALTER TABLE … ADD item, so the repeated
// "ADD COLUMN a INT, COLUMN b INT" form yields the real names "a" and "b"
// instead of discarding the COLUMN-prefixed items (issue #715).
func stripAddColItemPrefix(def string) string {
	sig := sigToks(sqltok.Tokenize(def))
	i := 0
	if kwAt(sig, def, i, "COLUMN") {
		i++
	}
	if kwAt(sig, def, i, "IF") && kwAt(sig, def, i+1, "NOT") && kwAt(sig, def, i+2, "EXISTS") {
		i += 3
	}
	if i == 0 || i >= len(sig) {
		return def
	}
	return def[sig[i].Start:]
}

// parseAlterAddColumns matches
//
//	ALTER TABLE [IF EXISTS] <name> ADD [COLUMN] [IF NOT EXISTS] <col> <type> [, …]
//
// and returns the table's ident-path text and the added columns. Non-column ADD
// clauses (CONSTRAINT / PRIMARY KEY / …) yield no columns. ok is false when the
// statement is not an ALTER TABLE … ADD that introduces at least one column.
// sig is the caller's already-computed significant tokens for raw.
func parseAlterAddColumns(sig []sqltok.Token, raw string, ic bool) (tablePath string, cols []ColInfo, ok bool) {
	if !kwAt(sig, raw, 0, "ALTER") || !kwAt(sig, raw, 1, "TABLE") {
		return "", nil, false
	}
	i := 2
	if kwAt(sig, raw, i, "IF") && kwAt(sig, raw, i+1, "EXISTS") {
		i += 2
	}
	path, pos := readIdentPath(sig, raw, i)
	if path == "" || !kwAt(sig, raw, pos, "ADD") {
		return "", nil, false
	}
	pos++
	if kwAt(sig, raw, pos, "COLUMN") {
		pos++
	}
	if kwAt(sig, raw, pos, "IF") && kwAt(sig, raw, pos+1, "NOT") && kwAt(sig, raw, pos+2, "EXISTS") {
		pos += 3
	}
	if pos >= len(sig) {
		return "", nil, false
	}
	// The column-def block is everything from the first def token to end of stmt.
	// ponytail: reuse the CREATE TABLE column splitter — it already handles commas,
	// quotes, comments and nested parens — then strip each item's optional
	// COLUMN / IF NOT EXISTS prefix and drop non-column ADD clauses.
	for _, def := range splitColDefs(raw[sig[pos].Start:]) {
		col, cok := parseFirstIdentAsCol(stripAddColItemPrefix(def), ic)
		if !cok || nonColumnAddKw[strings.ToUpper(col.Name)] {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return "", nil, false
	}
	return path, cols, true
}

// applyAlterAddToLocalCache appends the columns added by an ALTER TABLE … ADD
// COLUMN to the in-script column cache, but only for tables already created
// in-script (whose keys already exist) — never inventing a partial cache entry
// for a table that lives only in Snowflake metadata.
func applyAlterAddToLocalCache(tablePath string, cols []ColInfo, localColCache map[string][]ColInfo, ic bool) {
	parts := extractIdentParts(tablePath, ic)
	if len(parts) == 0 {
		return
	}
	tableName := parts[len(parts)-1]
	keys := []string{bcrCacheKey("", "", tableName)}
	if len(parts) >= 2 {
		keys = append(keys, bcrCacheKey("", parts[len(parts)-2], tableName))
	}
	if len(parts) >= 3 {
		keys = append(keys, bcrCacheKey(parts[len(parts)-3], parts[len(parts)-2], tableName))
	}
	for _, k := range keys {
		existing, ok := localColCache[k]
		if !ok {
			continue
		}
		// Fresh slice: CREATE stores the same backing array under several keys.
		merged := make([]ColInfo, 0, len(existing)+len(cols))
		merged = append(merged, existing...)
		merged = append(merged, cols...)
		localColCache[k] = merged
	}
}

// lookupColsForRef finds the ColInfo slice for a table identified by
// (name, db, schema).  It checks the caller-supplied colInfoCache first
// (populated from Snowflake metadata), then the script-local localColCache.
// Returns (nil, false) if the table is not found in either cache, which
// triggers the caller to skip column validation for that statement.
func lookupColsForRef(
	name, db, schema string,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool,
) ([]ColInfo, bool) {
	cols, found, _ := lookupColsForRefTagged(name, db, schema, resolvedRefs, colInfoCache, localColCache, checkEq)
	return cols, found
}

// lookupColsForRefTagged is like lookupColsForRef but additionally reports
// whether the returned columns came from the localColCache (fromLocal=true)
// or from the colInfoCache/resolvedRefs metadata (fromLocal=false).
func lookupColsForRefTagged(
	name, db, schema string,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool,
) (cols []ColInfo, found bool, fromLocal bool) {
	nameU := strings.ToUpper(name)

	// If db+schema are fully qualified, look up directly.
	if db != "" && schema != "" {
		key := bcrCacheKey(strings.ToUpper(db), strings.ToUpper(schema), nameU)
		if c, ok := colInfoCache[key]; ok {
			return c, true, false
		}
		if c, ok := localColCache[key]; ok {
			return c, true, true
		}
		return nil, false, false
	}

	// Try to resolve via resolvedRefs (Snowflake live objects).
	for _, ref := range resolvedRefs {
		if checkEq(ref.Name, name) &&
			(db == "" || checkEq(ref.DB, db)) &&
			(schema == "" || checkEq(ref.Schema, schema)) {
			key := bcrCacheKey(strings.ToUpper(ref.DB), strings.ToUpper(ref.Schema), strings.ToUpper(ref.Name))
			if c, ok := colInfoCache[key]; ok {
				return c, true, false
			}
			break
		}
	}

	// Fall back to local cache with schema.table or table-only key.
	if schema != "" {
		key := bcrCacheKey("", strings.ToUpper(schema), nameU)
		if c, ok := localColCache[key]; ok {
			return c, true, true
		}
	}
	key := bcrCacheKey("", "", nameU)
	if c, ok := localColCache[key]; ok {
		return c, true, true
	}

	return nil, false, false
}

// validateInsertCols validates the explicit column list in an INSERT statement.
// Only fires when the INSERT names columns: INSERT INTO t (c1, c2) VALUES ...
func validateInsertCols(
	raw string, r StatementRange, baseCol int,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool, ic bool,
) []DiagMarker {
	tokens := sqltok.Tokenize(raw)
	sig := sigToks(tokens)
	tablePath, colListRaw, ok := matchInsertColList(sig, raw)
	if !ok {
		return nil // No explicit column list.
	}

	parts := extractIdentParts(tablePath, ic)
	if len(parts) == 0 {
		return nil
	}
	var db, schema string
	tableName := parts[len(parts)-1]
	if len(parts) >= 3 {
		db = parts[len(parts)-3]
		schema = parts[len(parts)-2]
	} else if len(parts) == 2 {
		schema = parts[0]
	}

	cols, ok := lookupColsForRef(tableName, db, schema, resolvedRefs, colInfoCache, localColCache, checkEq)
	if !ok {
		return nil // Table not in cache; skip to avoid false-positives.
	}

	knownCols := buildKnownColSet(cols, ic)

	colIdents := extractIdentParts(colListRaw, ic)
	var missing []string
	for _, normName := range colIdents {
		if _, found := knownCols[normName]; !found {
			missing = append(missing, normName)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return colRefMarkers(raw, r, missing, tableName, baseCol, ic)
}

// validateReferencesCols validates REFERENCES (col) lists inside CREATE TABLE.
func validateReferencesCols(
	raw string, r StatementRange, baseCol int,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool, ic bool,
) []DiagMarker {
	tokens := sqltok.Tokenize(raw)
	sig := sigToks(tokens)
	if !matchCreateTableGuard(sig, raw) {
		return nil
	}
	var markers []DiagMarker

	for _, rm := range findReferences(sig, raw) {
		if rm.colListRaw == "" {
			continue // REFERENCES without an explicit column list; skip.
		}
		parts := extractIdentParts(rm.tablePath, ic)
		if len(parts) == 0 {
			continue
		}
		var db, schema string
		tableName := parts[len(parts)-1]
		if len(parts) >= 3 {
			db = parts[len(parts)-3]
			schema = parts[len(parts)-2]
		} else if len(parts) == 2 {
			schema = parts[0]
		}

		cols, ok := lookupColsForRef(tableName, db, schema, resolvedRefs, colInfoCache, localColCache, checkEq)
		if !ok {
			continue
		}
		knownCols := buildKnownColSet(cols, ic)

		colIdents := extractIdentParts(rm.colListRaw, ic)
		var missing []string
		for _, normName := range colIdents {
			if _, found := knownCols[normName]; !found {
				missing = append(missing, normName)
			}
		}
		if len(missing) == 0 {
			continue
		}
		markers = append(markers, colRefMarkers(raw, r, missing, tableName, baseCol, ic)...)
	}
	return markers
}

// buildKnownColSet builds a set of normalised column names from a ColInfo slice.
func buildKnownColSet(cols []ColInfo, ic bool) map[string]struct{} {
	m := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		key := c.Name
		if ic {
			key = strings.ToUpper(key)
		}
		m[key] = struct{}{}
	}
	return m
}

// extractSelectClause returns the text immediately after a SELECT keyword up to
// (but not including) the first FROM keyword found at paren depth 0.
// The input s is the text AFTER the SELECT keyword has been consumed.
// If no FROM is found at depth 0, the whole input is returned.
func extractSelectClause(s string) string {
	depth := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && (c == 'F' || c == 'f') {
				upper := strings.ToUpper(s[i:min(i+5, len(s))])
				if len(upper) >= 4 && upper[:4] == "FROM" &&
					(len(upper) < 5 || !isWordCharByte2(upper[4])) {
					return s[:i]
				}
			}
		}
	}
	return s
}

// isWordCharByte2 reports whether c can continue a SQL word.
func isWordCharByte2(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '$'
}

// scanSelectClauseForUnknownCols finds identifiers in the SELECT clause that
// are not qualified (preceded/followed by "."), not function calls (followed
// by "("), not AS aliases, not numeric literals, and not inside single-quoted
// string literals.  It returns each unknown name normalised via normIdent (for
// use as a lookup key and search target).
//
// Two known-column sets are used:
//   - metaCols: columns from Snowflake metadata (case-insensitive, all uppercase keys)
//   - localCols: columns from in-script CREATE TABLE (case-sensitive, keys from normIdent)
//
// A reference is valid if it matches either set with the appropriate semantics.
func scanSelectClauseForUnknownCols(clause string, metaCols, localCols map[string]struct{}, ic bool) []string {
	// The clause is tokenized once: significant tokens carry positions, so
	// identifiers, string literals, and neighboring punctuation are all read
	// from the same stream (no regex + hand-built quote mask to keep in sync).
	clauseSig := sigTokens(clause)
	if len(clauseSig) == 0 {
		return nil
	}

	// Build set of start positions that are AS aliases (or the AS keyword itself).
	aliasStarts := make(map[int]struct{})
	for _, a := range findAsAliases(clauseSig, clause) {
		aliasStarts[a.asStart] = struct{}{}
		aliasStarts[a.aliasStart] = struct{}{}
	}

	var missing []string
	seen := make(map[string]struct{})
	for i, tok := range clauseSig {
		// Only bare/quoted identifiers (and keywords, filtered below) can be
		// column names; string literals, numbers, and operators are skipped by
		// virtue of not being ident-like. String literals in particular — e.g.
		// 'month' in DATE_TRUNC('month', col) — are their own tokens, never
		// identifiers, so no quote mask is needed.
		if !isIdent(tok) {
			continue
		}

		// Skip AS keyword and AS aliases
		if _, skip := aliasStarts[tok.Start]; skip {
			continue
		}
		// Skip if preceded by "." (qualified column reference)
		if i > 0 && clauseSig[i-1].Kind == sqltok.Dot {
			continue
		}
		// Skip if followed by "." (schema/table qualifier)
		if i+1 < len(clauseSig) && clauseSig[i+1].Kind == sqltok.Dot {
			continue
		}
		// Skip if followed by "(" (function call)
		if i+1 < len(clauseSig) && clauseSig[i+1].Kind == sqltok.LParen {
			continue
		}

		// Normalize the identifier: bare identifiers are uppercased (Snowflake
		// convention), quoted identifiers preserve case (unless ic=true).
		normName := normIdent(tok.Text(clause), ic)

		// Skip known SQL keywords to prevent flagging things like FROM, WHERE, etc.
		normUpper := strings.ToUpper(normName)
		if sqltok.IsKeyword(normUpper) {
			continue
		}

		// Skip date parts used as the first argument of date functions
		if bcrDateParts[normUpper] {
			if fn := GetActiveFunctionCall(clause[:tok.Start]); fn != nil {
				if bcrDateFuncs[strings.ToUpper(fn.Name)] && fn.ParamIndex == 0 {
					continue
				}
			}
		}

		// A reference is valid if:
		//  - it matches the local in-script set exactly (case-sensitive), OR
		//  - its uppercased form matches the metadata set (case-insensitive).
		// When ic=true, both sets are already uppercased and normName is also
		// uppercased, so the distinction disappears.
		_, inLocal := localCols[normName]
		_, inMeta := metaCols[strings.ToUpper(normName)]
		if !inLocal && !inMeta {
			if _, already := seen[normName]; !already {
				seen[normName] = struct{}{}
				missing = append(missing, normName)
			}
		}
	}
	return missing
}

// aliasColSets holds the meta (case-insensitive) and local (case-sensitive)
// known-column sets for a single table alias.
type aliasColSets struct{ meta, local map[string]struct{} }

// scanAliasedColRefs scans clause for alias.column occurrences and returns
// the column names that are NOT found in the alias's known-column sets.
// aliasMap maps UPPERCASE alias → meta/local column sets (same distinction as
// scanSelectClauseForUnknownCols).  Only aliases present in aliasMap are
// checked; all others are silently skipped so that CTE aliases (whose column
// lists are unknown without an AST) never produce false positives.
func scanAliasedColRefs(clause string, aliasMap map[string]*aliasColSets, ic bool) []string {
	clauseSig := sigTokens(clause)
	var missing []string
	seen := make(map[string]struct{})

	for i, tok := range clauseSig {
		// We want the column part of alias.column: an ident directly preceded
		// by a dot, whose preceding token is the alias ident.
		if !isIdent(tok) {
			continue
		}
		if i < 2 || clauseSig[i-1].Kind != sqltok.Dot || !isIdent(clauseSig[i-2]) {
			continue
		}
		// Skip three-part qualifiers like db.schema.table — the token before
		// the alias would itself be preceded by a dot.
		if i >= 3 && clauseSig[i-3].Kind == sqltok.Dot {
			continue
		}
		// Skip if the column itself is followed by a dot (e.g. schema.table.col
		// where this token is not the final component).
		if i+1 < len(clauseSig) && clauseSig[i+1].Kind == sqltok.Dot {
			continue
		}
		aliasU := strings.ToUpper(normIdent(clauseSig[i-2].Text(clause), false))
		sets, hasAlias := aliasMap[aliasU]
		if !hasAlias {
			continue // Alias not resolved to a cached table; skip.
		}
		normName := normIdent(tok.Text(clause), ic)
		key := aliasU + "\x00" + normName
		_, inLocal := sets.local[normName]
		_, inMeta := sets.meta[strings.ToUpper(normName)]
		if !inLocal && !inMeta {
			if _, already := seen[key]; !already {
				seen[key] = struct{}{}
				missing = append(missing, normName)
			}
		}
	}
	return missing
}

// validateSelectCols validates column references in SELECT clauses (and in
// CREATE VIEW ... AS SELECT).
func validateSelectCols(
	raw string, r StatementRange, baseCol int,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool, ic bool,
) []DiagMarker {
	stripped := stripCommentsSQL(raw)

	// Find the SELECT keyword in the statement.
	strippedTokens := sqltok.Tokenize(stripped)
	strippedSig := sigToks(strippedTokens)
	selOffset := findSelectKWOffset(strippedSig, stripped)
	if selOffset < 0 {
		return nil
	}
	// selOffset is the byte offset of SELECT; advance past it.
	selEnd := selOffset + len("SELECT")

	// Extract FROM/JOIN table refs from the full stripped statement.
	type tableRef struct{ db, schema, name string }
	var tables []tableRef
	for _, path := range findFromJoinTables2(strippedSig, stripped) {
		parts := extractIdentParts(path, ic)
		switch len(parts) {
		case 3:
			tables = append(tables, tableRef{parts[0], parts[1], parts[2]})
		case 2:
			tables = append(tables, tableRef{"", parts[0], parts[1]})
		case 1:
			tables = append(tables, tableRef{"", "", parts[0]})
		}
	}

	// Build known-column sets from all FROM/JOIN tables.
	// metaCols: columns from Snowflake metadata — uppercased (case-insensitive matching).
	// localCols: columns from in-script CREATE TABLE — as-is from normIdent
	// (case-sensitive for quoted, uppercase for bare).
	metaCols := make(map[string]struct{})
	localCols := make(map[string]struct{})
	foundAnyTable := false
	for _, t := range tables {
		cols, found, fromLocal := lookupColsForRefTagged(t.name, t.db, t.schema, resolvedRefs, colInfoCache, localColCache, checkEq)
		if !found {
			continue
		}
		foundAnyTable = true
		if fromLocal {
			for _, c := range cols {
				key := c.Name
				if ic {
					key = strings.ToUpper(key)
				}
				localCols[key] = struct{}{}
			}
		} else {
			for _, c := range cols {
				metaCols[strings.ToUpper(c.Name)] = struct{}{}
			}
		}
	}
	noFromClause := len(tables) == 0
	if !foundAnyTable && !noFromClause {
		return nil // FROM/JOIN tables present but none resolved; skip to avoid false positives.
	}

	// Extract the SELECT clause (text between SELECT and the first depth-0 FROM).
	selectClause := extractSelectClause(stripped[selEnd:])

	// Scan for unknown bare (unqualified) column refs.
	missing := scanSelectClauseForUnknownCols(selectClause, metaCols, localCols, ic)

	// Skip the name of the object being created (e.g. the view name)
	// to prevent false positives when a view has the same name as a column.
	rawToks := sqltok.Tokenize(raw)
	rawSig := sigToks(rawToks)
	if rawPath, _, ok := matchCreateTV(rawSig, raw); ok {
		if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
			objName := parts[len(parts)-1] // already normalised by extractIdentParts
			filtered := make([]string, 0, len(missing))
			for _, m := range missing {
				if m != objName {
					filtered = append(filtered, m)
				}
			}
			missing = filtered
		}
	}

	// Also validate qualified column refs (alias.column) for aliases whose
	// tables are in the column cache.  Build alias → per-table column sets.
	// Skip when there is no FROM clause — no aliases to check.
	if !noFromClause {
		aliasMap := make(map[string]*aliasColSets)
		for _, ta := range findFromJoinWithAlias(strippedSig, stripped) {
			rawAlias := ta.alias
			if rawAlias == "" {
				continue
			}
			aliasU := strings.ToUpper(normIdent(rawAlias, ic))
			// Filter out SQL keywords that the matcher may capture as aliases.
			if joinStopKW[aliasU] {
				continue
			}
			parts := extractIdentParts(ta.tablePath, ic)
			var tRef struct{ db, schema, name string }
			switch len(parts) {
			case 3:
				tRef = struct{ db, schema, name string }{parts[0], parts[1], parts[2]}
			case 2:
				tRef = struct{ db, schema, name string }{"", parts[0], parts[1]}
			case 1:
				tRef = struct{ db, schema, name string }{"", "", parts[0]}
			default:
				continue
			}
			cols, found, fromLocal := lookupColsForRefTagged(tRef.name, tRef.db, tRef.schema, resolvedRefs, colInfoCache, localColCache, checkEq)
			if !found {
				continue // Table not cached; cannot validate — skip to avoid false positives.
			}
			sets := &aliasColSets{meta: make(map[string]struct{}), local: make(map[string]struct{})}
			if fromLocal {
				for _, c := range cols {
					key := c.Name
					if ic {
						key = strings.ToUpper(key)
					}
					sets.local[key] = struct{}{}
				}
			} else {
				for _, c := range cols {
					sets.meta[strings.ToUpper(c.Name)] = struct{}{}
				}
			}
			aliasMap[aliasU] = sets
		}
		if len(aliasMap) > 0 {
			missing = append(missing, scanAliasedColRefs(selectClause, aliasMap, ic)...)
		}
	}

	if len(missing) == 0 {
		return nil
	}
	label := "query"
	if noFromClause {
		label = "query (no FROM clause)"
	}
	return colRefMarkers(raw, r, missing, label, baseCol, ic)
}

// colRefMarkers locates tokens in raw for each missing column and returns
// DiagMarker entries (severity 4 = warning).
func colRefMarkers(raw string, r StatementRange, missing []string, tableLabel string, baseCol int, ic bool) []DiagMarker {
	var markers []DiagMarker
	for _, t := range findTokensLocally(raw, missing, r.StartLine, baseCol, ic) {
		var msg string
		if t.quoted {
			msg = `Column '"` + t.name + `"' not found in ` + tableLabel
		} else {
			msg = "Column '" + t.name + "' not found in " + tableLabel
		}
		markers = append(markers, diagMarkerAt(t, msg, 4))
	}
	return markers
}
