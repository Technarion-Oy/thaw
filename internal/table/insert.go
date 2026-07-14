// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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

// renderInsertValue renders one InsertRowValue into its VALUES-list token.
func renderInsertValue(v InsertRowValue) string {
	switch strings.ToLower(strings.TrimSpace(v.Mode)) {
	case "null":
		return "NULL"
	case "default":
		return "DEFAULT"
	case "expression":
		if expr := strings.TrimSpace(v.Value); expr != "" {
			return expr
		}
		return "NULL"
	default: // "value" (and any unrecognized mode)
		return renderTypedLiteral(v.DataType, v.Value)
	}
}

// renderTypedLiteral renders a user-entered value as a SQL literal appropriate
// for the column's data type. Numeric and boolean columns emit bare literals
// when the value is valid; every other type (text, date/time, variant, binary,
// …) is emitted as a single-quoted string literal, relying on Snowflake's
// implicit cast from the string form.
func renderTypedLiteral(dataType, value string) string {
	base := baseType(dataType)
	switch {
	case numericTypes[base]:
		t := strings.TrimSpace(value)
		if t == "" {
			return "NULL"
		}
		if reNumericLit.MatchString(t) {
			return t
		}
		// Not a valid number — quote it so the statement stays valid and
		// Snowflake reports the type error rather than the SQL being malformed.
		return snowflake.QuoteStringLit(value)
	case base == "BOOLEAN" || base == "BOOL":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "t", "yes", "y", "1", "on":
			return "TRUE"
		case "false", "f", "no", "n", "0", "off":
			return "FALSE"
		case "":
			return "NULL"
		}
		return snowflake.QuoteStringLit(value)
	default:
		// Text / date / time / timestamp / variant / binary / geo — a
		// single-quoted literal that Snowflake implicitly casts. QuoteTextLit
		// doubles backslashes so a literal backslash survives.
		return snowflake.QuoteTextLit(value)
	}
}

// renderRow renders one row into its double-quoted column list and its rendered
// VALUES tokens. Values with an empty column name are skipped so a
// partially-filled form still yields valid preview SQL.
func renderRow(row InsertRowConfig) (cols, vals []string) {
	cols = make([]string, 0, len(row.Values))
	vals = make([]string, 0, len(row.Values))
	for _, v := range row.Values {
		name := strings.TrimSpace(v.Column)
		if name == "" {
			continue
		}
		cols = append(cols, snowflake.QuoteIdent(name))
		vals = append(vals, renderInsertValue(v))
	}
	return cols, vals
}

// BuildInsertRowsSql constructs an INSERT INTO ... (cols) VALUES (...), (...)
// statement inserting one or more rows in a single statement.
//
// The column list is fully double-quoted (QuoteIdent) and taken from the first
// row; every row must align to those columns in the same order (the frontend
// builds all rows from the same table columns). Each value is rendered by mode:
// a typed literal, NULL, DEFAULT, or a raw expression (function-picker values
// such as CURRENT_TIMESTAMP()).
//
// It always returns a nil error; the error result exists for IPC symmetry with
// the other builders and to leave room for future validation without changing
// the Wails-bound signature.
func BuildInsertRowsSql(db, schema, tableName string, cfg InsertRowsConfig) (string, error) {
	var cols []string
	tuples := make([]string, 0, len(cfg.Rows))
	for i, row := range cfg.Rows {
		c, vals := renderRow(row)
		if i == 0 {
			cols = c
		}
		tuples = append(tuples, "("+strings.Join(vals, ", ")+")")
	}
	if len(tuples) == 0 {
		// No rows yet — emit a degenerate single empty tuple so the live preview
		// stays syntactically shaped; the frontend gates submission on canSubmit.
		tuples = append(tuples, "()")
	}

	return "INSERT INTO " + snowflake.Qualify(db, schema, tableName) +
		" (" + strings.Join(cols, ", ") + ") VALUES " + strings.Join(tuples, ", ") + ";", nil
}
