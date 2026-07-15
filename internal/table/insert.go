// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"regexp"
	"strings"

	"thaw/internal/snowflake"
)

// InsertRowValue is one column's contribution to a single-row INSERT. Mode
// selects how Value is rendered into the VALUES list:
//
//   - "value"      — Value is a user literal rendered per DataType (a numeric or
//     boolean literal is emitted bare when it is valid; everything else is a
//     single-quoted string literal). An empty Value yields '' for text columns
//     and NULL for numeric/boolean columns.
//   - "null"       — the SQL keyword NULL (Value is ignored).
//   - "default"    — the SQL keyword DEFAULT, applying the column's default
//     (Value is ignored).
//   - "expression" — Value is emitted verbatim as a raw SQL expression, e.g.
//     CURRENT_TIMESTAMP() from the built-in function picker. The caller is
//     responsible for its correctness.
//
// DataType is the column's Snowflake type string (e.g. "VARCHAR(256)",
// "NUMBER(38,0)"), used only to choose literal rendering in "value" mode.
type InsertRowValue struct {
	Column   string `json:"column"`
	DataType string `json:"dataType"`
	Mode     string `json:"mode"`
	Value    string `json:"value"`
}

// InsertRowConfig holds the per-column values for one row of an INSERT. Field
// names mirror the frontend so the Wails-generated model maps cleanly.
type InsertRowConfig struct {
	Values []InsertRowValue `json:"values"`
}

// InsertRowsConfig holds one or more rows to insert in a single statement. Every
// row's Values align to the same columns (in the same order); the column list of
// the emitted statement is taken from the first row.
type InsertRowsConfig struct {
	Rows []InsertRowConfig `json:"rows"`
}

// reNumericLit matches a decimal or scientific numeric literal (optionally
// signed). It gates whether a "value"-mode entry on a numeric column is emitted
// bare; a non-match is quoted as a string literal so the statement stays
// syntactically valid (and injection-safe) and Snowflake surfaces the type
// error itself.
var reNumericLit = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// reVectorType extracts a VECTOR column's element type and dimension, e.g.
// "VECTOR(FLOAT, 256)" → ("FLOAT", "256"). Used to render a validated array
// literal cast to the exact declared vector type.
var reVectorType = regexp.MustCompile(`(?i)^VECTOR\s*\(\s*(INT|FLOAT)\s*,\s*(\d+)\s*\)$`)

// baseType extracts the leading type word from a Snowflake data-type string,
// uppercased — "number(38,0)" → "NUMBER", "TIMESTAMP_NTZ(9)" → "TIMESTAMP_NTZ",
// "double precision" → "DOUBLE". It stops at the first character that cannot be
// part of a type identifier (a space, '(', etc.).
func baseType(dataType string) string {
	s := strings.TrimSpace(dataType)
	end := len(s)
	for i, r := range s {
		if r == ' ' || r == '(' || r == '\t' {
			end = i
			break
		}
	}
	return strings.ToUpper(s[:end])
}

// numericTypes is the set of base type words rendered as bare numeric literals.
var numericTypes = map[string]bool{
	"NUMBER": true, "DECIMAL": true, "NUMERIC": true,
	"INT": true, "INTEGER": true, "BIGINT": true, "SMALLINT": true,
	"TINYINT": true, "BYTEINT": true,
	"FLOAT": true, "FLOAT4": true, "FLOAT8": true,
	"DOUBLE": true, "REAL": true,
}

// semiStructuredTypes is the set of base type words whose values are rendered
// via a JSON constructor (PARSE_JSON / TO_OBJECT / TO_ARRAY). Snowflake rejects
// such constructors inside a VALUES clause, so any of these on a "value"-mode
// cell forces the whole statement onto the INSERT … SELECT path.
var semiStructuredTypes = map[string]bool{
	"VARIANT": true, "OBJECT": true, "ARRAY": true,
}

// renderInsertValue renders one InsertRowValue into its token and reports
// whether that token requires the INSERT … SELECT form. Constructor
// expressions such as PARSE_JSON (semi-structured columns) and array-literal
// vector casts are illegal inside a VALUES clause, so when needsSelect is true
// the whole statement is emitted as INSERT … SELECT … UNION ALL … instead.
func renderInsertValue(v InsertRowValue) (token string, needsSelect bool) {
	switch strings.ToLower(strings.TrimSpace(v.Mode)) {
	case "null":
		return "NULL", false
	case "default":
		return "DEFAULT", false
	case "expression":
		if expr := strings.TrimSpace(v.Value); expr != "" {
			return expr, false
		}
		return "NULL", false
	default: // "value" (and any unrecognized mode)
		return renderTypedLiteral(v.DataType, v.Value)
	}
}

// renderTypedLiteral renders a user-entered value as a SQL literal appropriate
// for the column's data type, reporting whether the rendered token requires the
// INSERT … SELECT form (see renderInsertValue). Numeric and boolean columns emit
// bare literals when valid; semi-structured columns emit a JSON constructor and
// vector columns a validated array-literal cast (both need SELECT); every other
// type (text, date/time, binary, geo, uuid, …) is emitted as a single-quoted
// literal, relying on Snowflake's implicit cast from the string form.
func renderTypedLiteral(dataType, value string) (token string, needsSelect bool) {
	base := baseType(dataType)
	switch {
	case numericTypes[base]:
		t := strings.TrimSpace(value)
		if t == "" {
			return "NULL", false
		}
		if reNumericLit.MatchString(t) {
			return t, false
		}
		// Not a valid number — quote it so the statement stays valid and
		// Snowflake reports the type error rather than the SQL being malformed.
		// QuoteTextLit (not QuoteStringLit) so a stray backslash is treated as a
		// literal character, matching the default text path below.
		return snowflake.QuoteTextLit(value), false
	case base == "BOOLEAN" || base == "BOOL":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "t", "yes", "y", "1", "on":
			return "TRUE", false
		case "false", "f", "no", "n", "0", "off":
			return "FALSE", false
		case "":
			return "NULL", false
		}
		return snowflake.QuoteTextLit(value), false
	case semiStructuredTypes[base]:
		return renderSemiStructuredLiteral(base, value)
	case base == "VECTOR":
		return renderVectorLiteral(dataType, value)
	default:
		// Text / date / time / timestamp / binary / geo / uuid — a
		// single-quoted literal that Snowflake implicitly casts. QuoteTextLit
		// doubles backslashes so a literal backslash survives.
		return snowflake.QuoteTextLit(value), false
	}
}

// renderSemiStructuredLiteral renders a VARIANT/OBJECT/ARRAY value as a JSON
// constructor over a single-quoted string literal: PARSE_JSON for VARIANT, and
// TO_OBJECT/TO_ARRAY(PARSE_JSON(…)) for OBJECT/ARRAY so the result matches the
// column's declared semi-structured type rather than a bare VARIANT. The JSON
// text is quoted (never interpolated raw), so the expression stays injection-safe
// and Snowflake validates the JSON at execution time. An empty value is NULL and
// needs no SELECT form.
func renderSemiStructuredLiteral(base, value string) (token string, needsSelect bool) {
	if strings.TrimSpace(value) == "" {
		return "NULL", false
	}
	lit := "PARSE_JSON(" + snowflake.QuoteTextLit(value) + ")"
	switch base {
	case "OBJECT":
		return "TO_OBJECT(" + lit + ")", true
	case "ARRAY":
		return "TO_ARRAY(" + lit + ")", true
	default: // VARIANT
		return lit, true
	}
}

// renderVectorLiteral renders a VECTOR value as an array literal cast to the
// declared vector type, e.g. "[1, 2, 3, 4]::VECTOR(FLOAT, 4)". The elements are
// validated as numeric literals and re-emitted (never interpolated raw), so the
// cast stays injection-safe. If the declared type is not a recognized
// VECTOR(elem, dim) or the value is not a bracketed list of numbers, the raw
// value is single-quoted instead so Snowflake reports the type error; an empty
// value is NULL. Only the successfully-built array cast needs the SELECT form.
func renderVectorLiteral(dataType, value string) (token string, needsSelect bool) {
	if strings.TrimSpace(value) == "" {
		return "NULL", false
	}
	m := reVectorType.FindStringSubmatch(strings.TrimSpace(dataType))
	nums, ok := parseNumberList(value)
	if m == nil || !ok {
		return snowflake.QuoteTextLit(value), false
	}
	elem := strings.ToUpper(m[1])
	dim := m[2]
	return "[" + strings.Join(nums, ", ") + "]::VECTOR(" + elem + ", " + dim + ")", true
}

// parseNumberList parses a bracketed, comma-separated list of numeric literals
// ("[1.0, 2.0, 3]") into the trimmed element strings, validating each against
// reNumericLit. It returns ok=false if the value is not bracketed or any element
// is not a plain number, so the caller can fall back to a safe quoted string.
func parseNumberList(value string) (nums []string, ok bool) {
	s := strings.TrimSpace(value)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, false
	}
	for _, part := range strings.Split(inner, ",") {
		t := strings.TrimSpace(part)
		if !reNumericLit.MatchString(t) {
			return nil, false
		}
		nums = append(nums, t)
	}
	return nums, true
}

// includedColumns derives the emitted column list from the FIRST row: the
// double-quoted names of every value whose column name is non-empty, together
// with their positions within the row. Every row is then rendered against these
// same positions (see BuildInsertRowsSql), so all VALUES tuples share the column
// list's arity even if a later row carries an empty column name that per-row
// skipping would otherwise drop. Values with an empty column name are skipped so
// a partially-filled form still yields valid preview SQL.
func includedColumns(rows []InsertRowConfig) (cols []string, positions []int) {
	if len(rows) == 0 {
		return nil, nil
	}
	for i, v := range rows[0].Values {
		name := strings.TrimSpace(v.Column)
		if name == "" {
			continue
		}
		cols = append(cols, snowflake.QuoteIdent(name))
		positions = append(positions, i)
	}
	return cols, positions
}

// BuildInsertRowsSql constructs an INSERT INTO ... (cols) VALUES (...), (...)
// statement inserting one or more rows in a single statement.
//
// The column list is fully double-quoted (QuoteIdent) and taken from the first
// row; every row is rendered against those same column positions (the frontend
// builds all rows from the same table columns), guaranteeing that each VALUES
// tuple has exactly as many elements as the column list regardless of how any
// individual row was filled in. Each value is rendered by mode: a typed literal,
// NULL, DEFAULT, or a raw expression (function-picker values such as
// CURRENT_TIMESTAMP()).
//
// It always returns a nil error; the error result exists for IPC symmetry with
// the other builders and to leave room for future validation without changing
// the Wails-bound signature.
// The statement uses the VALUES form unless any rendered value requires a
// constructor expression that Snowflake rejects in a VALUES clause (a
// semi-structured PARSE_JSON or a vector array cast — see renderInsertValue). In
// that case the whole statement is emitted as INSERT … SELECT … UNION ALL …
// instead, which does accept those expressions.
func BuildInsertRowsSql(db, schema, tableName string, cfg InsertRowsConfig) (string, error) {
	cols, positions := includedColumns(cfg.Rows)

	rowsVals := make([][]string, 0, len(cfg.Rows))
	needsSelect := false
	for _, row := range cfg.Rows {
		vals := make([]string, 0, len(positions))
		for _, pos := range positions {
			if pos < len(row.Values) {
				tok, sel := renderInsertValue(row.Values[pos])
				if sel {
					needsSelect = true
				}
				vals = append(vals, tok)
			} else {
				// Ragged row (fewer values than the first) — keep arity by
				// filling the missing cell with NULL rather than a short tuple.
				vals = append(vals, "NULL")
			}
		}
		rowsVals = append(rowsVals, vals)
	}

	into := "INSERT INTO " + snowflake.Qualify(db, schema, tableName) +
		" (" + strings.Join(cols, ", ") + ")"

	if needsSelect {
		// SELECT form: one SELECT per row, joined by UNION ALL. Required because
		// Snowflake disallows PARSE_JSON and array-cast constructors in VALUES.
		selects := make([]string, 0, len(rowsVals))
		for _, vals := range rowsVals {
			selects = append(selects, "SELECT "+strings.Join(vals, ", "))
		}
		return into + " " + strings.Join(selects, " UNION ALL ") + ";", nil
	}

	tuples := make([]string, 0, len(rowsVals))
	for _, vals := range rowsVals {
		tuples = append(tuples, "("+strings.Join(vals, ", ")+")")
	}
	if len(tuples) == 0 {
		// No rows yet — emit a degenerate single empty tuple so the live preview
		// stays syntactically shaped; the frontend gates submission on canSubmit.
		tuples = append(tuples, "()")
	}

	return into + " VALUES " + strings.Join(tuples, ", ") + ";", nil
}
