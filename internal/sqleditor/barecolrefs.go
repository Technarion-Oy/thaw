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

// ── Precompiled regexes ───────────────────────────────────────────────────────

var (
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
// — matching the TS fallback behaviour when node-sql-parser fails to parse.
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
		}
		// SELECT / WITH / UNDROP: skip — SELECT column validation requires an
		// AST to avoid false-positives from table aliases and AS aliases.
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
