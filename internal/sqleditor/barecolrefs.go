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
	// FROM keyword at start (for SELECT clause extraction)
	reFromKW = regexp.MustCompile(`(?i)^FROM\b`)
	// SELECT keyword (to locate start of SELECT clause)
	reSelectKW = regexp.MustCompile(`(?i)\bSELECT\b`)
	// AS <alias> pattern (to mark alias names for skipping)
	reAsAliasSel = regexp.MustCompile(`(?i)\bAS\s+([a-zA-Z0-9_$]+|"[^"]+")`)
	// FROM/JOIN without trailing \b – handles quoted identifiers like "DB"."SCH"."TABLE"
	// (reFromJoinFallback in tableexist.go has \b which fails after closing '"')
	reFromJoinSel = regexp.MustCompile(`(?i)(?:FROM|JOIN|CROSS\s+JOIN|INSERT\s+INTO|UPDATE|TRUNCATE\s+TABLE|DELETE\s+FROM|MERGE\s+INTO|DESCRIBE\s+TABLE|DESC\s+TABLE|DESCRIBE\s+VIEW|DESC\s+VIEW)\s+(` + _ident + `(?:\.` + _ident + `){0,2})`)

	// reFromJoinWithAlias captures (tablePath, optional_alias) from FROM/JOIN.
	// The optional alias may be preceded by AS or appear bare (e.g. FROM t AS a
	// or FROM t a).  SQL stop-words that look like aliases (ON, WHERE, …) are
	// filtered out in Go code using joinStopKW.
	reFromJoinWithAlias = regexp.MustCompile(
		`(?i)(?:FROM|JOIN|CROSS\s+JOIN|INSERT\s+INTO|UPDATE|DELETE\s+FROM|MERGE\s+INTO)\s+(` + _ident + `(?:\.` + _ident + `){0,2})` +
			`(?:\s+(?:AS\s+)?(` + _ident + `))?`)

	// CREATE TABLE (for pre-scan): capture name + column block.
	// The column block is captured as everything after the opening paren;
	// balanced-paren matching is done in Go code.
	reCreateTablePreScan = regexp.MustCompile(
		`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?` +
			`(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)?` +
			`TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)\s*\(`)

	// INSERT INTO table (columns) VALUES ...
	reInsertColList = regexp.MustCompile(
		`(?i)^\s*INSERT\s+(?:OVERWRITE\s+)?INTO\s+(` + _identPath + `)\s*\(([^)]+)\)`)

	// REFERENCES table [(columns)]
	reReferencesClause = regexp.MustCompile(
		`(?i)\bREFERENCES\s+(` + _identPath + `)\s*(?:\(([^)]+)\))?`)

	// CREATE TABLE guard (not CTAS/CLONE/LIKE/USING TEMPLATE)
	reCreateTableGuard = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?` +
			`(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)?TABLE\b`)

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
		m := reCreateTablePreScan.FindStringSubmatchIndex(raw)
		if m == nil {
			continue
		}
		// m[2:4] = capture group 1 (table name/path)
		nameStr := raw[m[2]:m[3]]
		parts := extractIdentParts(nameStr, ic)
		if len(parts) == 0 {
			continue
		}

		// Extract balanced column block starting at the opening paren.
		parenStart := m[1] - 1 // index of '(' in raw
		colsRaw := extractBalancedBlock(raw, parenStart)
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
			if reIsCreateView.MatchString(raw) {
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
func parseCreateTableColDefs(colsRaw string, ic bool) []ColInfo {
	var columns []ColInfo
	depth := 0
	var current strings.Builder

	for i := 0; i < len(colsRaw); i++ {
		c := colsRaw[i]
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

var reFirstColIdent = regexp.MustCompile(`^([a-zA-Z0-9_$]+|"[^"]+")`)

func parseFirstIdentAsCol(def string, ic bool) (ColInfo, bool) {
	m := reFirstColIdent.FindString(def)
	if m == "" {
		return ColInfo{}, false
	}
	return ColInfo{Name: normIdent(m, ic), DataType: "UNKNOWN"}, true
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
	nameU := strings.ToUpper(name)

	// If db+schema are fully qualified, look up directly.
	if db != "" && schema != "" {
		key := bcrCacheKey(strings.ToUpper(db), strings.ToUpper(schema), nameU)
		if cols, ok := colInfoCache[key]; ok {
			return cols, true
		}
		if cols, ok := localColCache[key]; ok {
			return cols, true
		}
		return nil, false
	}

	// Try to resolve via resolvedRefs (Snowflake live objects).
	for _, ref := range resolvedRefs {
		if checkEq(ref.Name, name) &&
			(db == "" || checkEq(ref.DB, db)) &&
			(schema == "" || checkEq(ref.Schema, schema)) {
			key := bcrCacheKey(strings.ToUpper(ref.DB), strings.ToUpper(ref.Schema), strings.ToUpper(ref.Name))
			if cols, ok := colInfoCache[key]; ok {
				return cols, true
			}
			break
		}
	}

	// Fall back to local cache with schema.table or table-only key.
	if schema != "" {
		key := bcrCacheKey("", strings.ToUpper(schema), nameU)
		if cols, ok := localColCache[key]; ok {
			return cols, true
		}
	}
	key := bcrCacheKey("", "", nameU)
	if cols, ok := localColCache[key]; ok {
		return cols, true
	}

	return nil, false
}

// validateInsertCols validates the explicit column list in an INSERT statement.
// Only fires when the INSERT names columns: INSERT INTO t (c1, c2) VALUES ...
func validateInsertCols(
	raw string, r StatementRange,
	resolvedRefs []ResolvedRef,
	colInfoCache, localColCache map[string][]ColInfo,
	checkEq func(string, string) bool, ic bool,
) []DiagMarker {
	m := reInsertColList.FindStringSubmatch(raw)
	if m == nil {
		return nil // No explicit column list.
	}
	tablePath := m[1]
	colListRaw := m[2]

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

	colIdents := reIdentOrQuoted.FindAllString(colListRaw, -1)
	var missing []string
	for _, ident := range colIdents {
		normName := normIdent(ident, ic)
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
	if !reCreateTableGuard.MatchString(raw) {
		return nil
	}
	var markers []DiagMarker

	for _, m := range reReferencesClause.FindAllStringSubmatch(raw, -1) {
		colListRaw := m[2]
		if colListRaw == "" {
			continue // REFERENCES without an explicit column list; skip.
		}
		tablePath := m[1]
		parts := extractIdentParts(tablePath, ic)
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

		colIdents := reIdentOrQuoted.FindAllString(colListRaw, -1)
		var missing []string
		for _, ident := range colIdents {
			normName := normIdent(ident, ic)
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
				if reFromKW.MatchString(s[i:]) {
					return s[:i]
				}
			}
		}
	}
	return s
}

// buildSingleQuoteMask returns a boolean slice where true indicates the byte
// position is inside a single-quoted SQL string literal (including the quotes
// themselves).  Handles SQL-style escaped quotes (”).
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
// string literals.  It returns each unknown name normalised to uppercase (for
// use as a lookup key and search target).
func scanSelectClauseForUnknownCols(clause string, knownCols map[string]struct{}) []string {
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
	for _, m := range reAsAliasSel.FindAllStringSubmatchIndex(clause, -1) {
		aliasStarts[m[0]] = struct{}{} // the AS keyword start
		aliasStarts[m[2]] = struct{}{} // the alias identifier start
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

		// Normalize to uppercase for case-insensitive column lookup
		normName := strings.ToUpper(normIdent(raw, false))

		// NEW: Skip known SQL keywords to prevent flagging things like FROM, WHERE, etc.
		if sqlAllKeywords[normName] {
			continue
		}

		// NEW: Skip date parts used as the first argument of date functions
		if bcrDateParts[normName] {
			if fn := GetActiveFunctionCall(clause[:start]); fn != nil {
				if bcrDateFuncs[strings.ToUpper(fn.Name)] && fn.ParamIndex == 0 {
					continue
				}
			}
		}

		if _, found := knownCols[normName]; !found {
			if _, already := seen[normName]; !already {
				seen[normName] = struct{}{}
				missing = append(missing, normName)
			}
		}
	}
	return missing
}

// scanAliasedColRefs scans clause for alias.column occurrences and returns
// the column names that are NOT found in the alias's known-column set.
// aliasColMap maps UPPERCASE alias → set of UPPERCASE known column names.
// Only aliases that appear in aliasColMap are checked; all others are silently
// skipped so that CTE aliases (whose column lists are unknown without an AST)
// never produce false positives.
func scanAliasedColRefs(clause string, aliasColMap map[string]map[string]struct{}) []string {
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
		knownCols, hasAlias := aliasColMap[aliasU]
		if !hasAlias {
			continue // Alias not resolved to a cached table; skip.
		}
		colRaw := clause[start:end]
		normName := strings.ToUpper(normIdent(colRaw, false))
		key := aliasU + "\x00" + normName
		if _, found := knownCols[normName]; !found {
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
	selLoc := reSelectKW.FindStringIndex(stripped)
	if selLoc == nil {
		return nil
	}

	// Extract FROM/JOIN table refs from the full stripped statement.
	type tableRef struct{ db, schema, name string }
	var tables []tableRef
	for _, fm := range reFromJoinSel.FindAllStringSubmatch(stripped, -1) {
		parts := extractIdentParts(fm[1], ic)
		switch len(parts) {
		case 3:
			tables = append(tables, tableRef{parts[0], parts[1], parts[2]})
		case 2:
			tables = append(tables, tableRef{"", parts[0], parts[1]})
		case 1:
			tables = append(tables, tableRef{"", "", parts[0]})
		}
	}

	// Build combined known columns from all FROM/JOIN tables.
	knownCols := make(map[string]struct{})
	foundAnyTable := false
	for _, t := range tables {
		cols, ok := lookupColsForRef(t.name, t.db, t.schema, resolvedRefs, colInfoCache, localColCache, checkEq)
		if ok {
			foundAnyTable = true
			for _, c := range cols {
				knownCols[strings.ToUpper(c.Name)] = struct{}{}
			}
		}
	}
	if !foundAnyTable {
		return nil // No table metadata; skip to avoid false positives.
	}

	// Extract the SELECT clause (text between SELECT and the first depth-0 FROM).
	selectClause := extractSelectClause(stripped[selLoc[1]:])

	// Scan for unknown bare (unqualified) column refs.
	missing := scanSelectClauseForUnknownCols(selectClause, knownCols)

	// NEW: Skip the name of the object being created (e.g. the view name)
	// to prevent false positives when a view has the same name as a column.
	if m := reCreateTVMatch.FindStringSubmatch(raw); m != nil {
		if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
			objNameU := strings.ToUpper(parts[len(parts)-1])
			filtered := make([]string, 0, len(missing))
			for _, m := range missing {
				if m != objNameU {
					filtered = append(filtered, m)
				}
			}
			missing = filtered
		}
	}

	// Also validate qualified column refs (alias.column) for aliases whose
	// tables are in the column cache.  Build alias → per-table column set.
	aliasColMap := make(map[string]map[string]struct{})
	for _, fm := range reFromJoinWithAlias.FindAllStringSubmatch(stripped, -1) {
		rawAlias := fm[2]
		if rawAlias == "" {
			continue
		}
		aliasU := strings.ToUpper(normIdent(rawAlias, ic))
		// Filter out SQL keywords that the regex may capture as aliases.
		if joinStopKW[aliasU] {
			continue
		}
		parts := extractIdentParts(fm[1], ic)
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
		cols, ok := lookupColsForRef(tRef.name, tRef.db, tRef.schema, resolvedRefs, colInfoCache, localColCache, checkEq)
		if !ok {
			continue // Table not cached; cannot validate — skip to avoid false positives.
		}
		aliasColMap[aliasU] = buildKnownColSet(cols, ic)
	}
	if len(aliasColMap) > 0 {
		missing = append(missing, scanAliasedColRefs(selectClause, aliasColMap)...)
	}

	if len(missing) == 0 {
		return nil
	}
	return colRefMarkers(raw, r, missing, "query", ic)
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
