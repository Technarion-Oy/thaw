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
	"regexp"
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
	// CLUSTER BY (...) – remove before FP check
	reClusterBy = regexp.MustCompile(`(?i)\bCLUSTER\s+BY\s*\([^)]+\)`)

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
//     if the table is not yet in Snowflake).
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
		firstTok := getFirstSQLToken(raw)

		if firstTok != "SELECT" && firstTok != "WITH" &&
			firstTok != "INSERT" && firstTok != "CREATE" && firstTok != "UNDROP" {
			continue
		}

		// False-positive guard: skip statements with Snowflake-specific syntax
		// that would produce noise.
		stripped := strings.TrimSpace(stripCommentsSQL(raw))
		checkText := reClusterBy.ReplaceAllString(stripped, "")
		if reSnowflakeFP.MatchString(checkText) {
			continue
		}

		switch firstTok {
		case "INSERT":
			markers = append(markers,
				validateInsertCols(raw, r, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)

		case "CREATE":
			markers = append(markers,
				validateReferencesCols(raw, r, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
			if isCreateView(raw) {
				markers = append(markers,
					validateSelectCols(raw, r, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
			}

		case "SELECT", "WITH":
			markers = append(markers,
				validateSelectCols(raw, r, req.ResolvedRefs, colInfoCache, localColCache, checkEq, ic)...)
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
// It handles comments (-- and /* */), double-quoted identifiers with special
// characters, and single-quoted string literals so that commas and parentheses
// inside those contexts do not interfere with column splitting.
func parseCreateTableColDefs(colsRaw string, ic bool) []ColInfo {
	var columns []ColInfo
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
				if col, ok := parseFirstIdentAsCol(def, ic); ok {
					columns = append(columns, col)
				}
			}
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	if def := strings.TrimSpace(current.String()); def != "" {
		if col, ok := parseFirstIdentAsCol(def, ic); ok {
			columns = append(columns, col)
		}
	}
	return columns
}

func parseFirstIdentAsCol(def string, ic bool) (ColInfo, bool) {
	// Tokenize the column definition and take the first identifier.
	tokens := sqltok.Tokenize(def)
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind == sqltok.Whitespace || tok.Kind == sqltok.Newline ||
			tok.Kind == sqltok.LineComment || tok.Kind == sqltok.BlockComment {
			continue
		}
		if isIdent(tok) {
			return ColInfo{Name: normIdent(tok.Text(def), ic), DataType: "UNKNOWN"}, true
		}
		break // first non-WS token is not an identifier
	}
	return ColInfo{}, false
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
	raw string, r StatementRange,
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

	return colRefMarkers(raw, r, missing, tableName, ic)
}

// validateReferencesCols validates REFERENCES (col) lists inside CREATE TABLE.
func validateReferencesCols(
	raw string, r StatementRange,
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
		markers = append(markers, colRefMarkers(raw, r, missing, tableName, ic)...)
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

// buildSingleQuoteMask returns a boolean slice where true indicates the byte
// position is inside a single-quoted SQL string literal (including the quotes
// themselves).  Handles SQL-style escaped quotes ('').
func buildSingleQuoteMask(s string) []bool {
	mask := make([]bool, len(s))
	inSingle := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			if inSingle && i+1 < len(s) && s[i+1] == '\'' {
				// Escaped quote '' — mark both bytes and skip ahead.
				mask[i] = true
				mask[i+1] = true
				i++
			} else {
				// Opening or closing quote.
				mask[i] = true
				inSingle = !inSingle
			}
		} else if inSingle {
			mask[i] = true
		}
	}
	return mask
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
	locs := reIdentOrQuoted.FindAllStringIndex(clause, -1)
	if len(locs) == 0 {
		return nil
	}

	// Pre-compute which positions are inside single-quoted string literals so
	// that e.g. 'month' in DATE_TRUNC('month', col) is not flagged as an
	// unknown column reference.
	inStr := buildSingleQuoteMask(clause)

	// Build set of start positions that are AS aliases (or the AS keyword itself).
	aliasStarts := make(map[int]struct{})
	// Use token-based AS alias detection.
	clauseSig := sigToks(sqltok.Tokenize(clause))
	for _, a := range findAsAliases(clauseSig, clause) {
		aliasStarts[a.asStart] = struct{}{}
		aliasStarts[a.aliasStart] = struct{}{}
	}

	var missing []string
	seen := make(map[string]struct{})
	for _, loc := range locs {
		start, end := loc[0], loc[1]

		// Skip identifiers inside single-quoted string literals.
		if start < len(inStr) && inStr[start] {
			continue
		}

		// Skip AS keyword and AS aliases
		if _, skip := aliasStarts[start]; skip {
			continue
		}
		raw := clause[start:end]
		// Skip numeric literals (can't be column names)
		if raw[0] >= '0' && raw[0] <= '9' {
			continue
		}
		// Skip if preceded by "." (qualified column reference)
		if start > 0 && clause[start-1] == '.' {
			continue
		}
		// Skip if followed by "." (schema/table qualifier)
		if end < len(clause) && clause[end] == '.' {
			continue
		}
		// Skip if followed by "(" possibly with whitespace (function call)
		after := strings.TrimLeft(clause[end:], " \t\r\n")
		if len(after) > 0 && after[0] == '(' {
			continue
		}

		// Normalize the identifier: bare identifiers are uppercased (Snowflake
		// convention), quoted identifiers preserve case (unless ic=true).
		normName := normIdent(raw, ic)

		// Skip known SQL keywords to prevent flagging things like FROM, WHERE, etc.
		normUpper := strings.ToUpper(normName)
		if sqltok.IsKeyword(normUpper) {
			continue
		}

		// Skip date parts used as the first argument of date functions
		if bcrDateParts[normUpper] {
			if fn := GetActiveFunctionCall(clause[:start]); fn != nil {
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
	locs := reIdentOrQuoted.FindAllStringIndex(clause, -1)
	if len(locs) == 0 {
		return nil
	}
	inStr := buildSingleQuoteMask(clause)
	var missing []string
	seen := make(map[string]struct{})

	for i, loc := range locs {
		start, end := loc[0], loc[1]
		// Skip positions inside single-quoted string literals.
		if start < len(inStr) && inStr[start] {
			continue
		}
		// We want tokens that are directly preceded by a dot (the column part
		// of alias.column).
		if start == 0 || clause[start-1] != '.' {
			continue
		}
		// The previous identifier token must be immediately before the dot
		// (i.e., its end == start-1) so we know it is the alias token, not
		// a different token that happens to sit near a dot.
		if i == 0 || locs[i-1][1] != start-1 {
			continue
		}
		// Skip three-part qualifiers like db.schema.table — the token before
		// the alias would itself be preceded by a dot.
		prevStart := locs[i-1][0]
		if prevStart > 0 && clause[prevStart-1] == '.' {
			continue
		}
		// Skip if the column itself is followed by a dot (e.g. schema.table.col
		// where this token is not the final component).
		if end < len(clause) && clause[end] == '.' {
			continue
		}
		prevRaw := clause[prevStart:locs[i-1][1]]
		aliasU := strings.ToUpper(normIdent(prevRaw, false))
		sets, hasAlias := aliasMap[aliasU]
		if !hasAlias {
			continue // Alias not resolved to a cached table; skip.
		}
		colRaw := clause[start:end]
		normName := normIdent(colRaw, ic)
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
	raw string, r StatementRange,
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
	return colRefMarkers(raw, r, missing, label, ic)
}

// colRefMarkers locates tokens in raw for each missing column and returns
// DiagMarker entries (severity 4 = warning).
func colRefMarkers(raw string, r StatementRange, missing []string, tableLabel string, ic bool) []DiagMarker {
	var markers []DiagMarker
	for _, t := range findTokensLocally(raw, missing, r.StartLine, ic) {
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
