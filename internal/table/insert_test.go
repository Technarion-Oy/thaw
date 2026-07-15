// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"strings"
	"testing"
)

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("SQL mismatch\n got: %s\nwant: %s", got, want)
	}
}

// oneRow wraps a single row's values into an InsertRowsConfig for the
// single-row test cases.
func oneRow(vals ...InsertRowValue) InsertRowsConfig {
	return InsertRowsConfig{Rows: []InsertRowConfig{{Values: vals}}}
}

func TestBuildInsertRowsSql_Basic(t *testing.T) {
	cfg := oneRow(
		InsertRowValue{Column: "ID", DataType: "NUMBER(38,0)", Mode: "value", Value: "42"},
		InsertRowValue{Column: "NAME", DataType: "VARCHAR(256)", Mode: "value", Value: "Alice"},
	)
	sql, err := BuildInsertRowsSql("DB", "SC", "T", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("ID", "NAME") VALUES (42, 'Alice');`)
}

func TestBuildInsertRowsSql_StringEscaping(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "NOTE", DataType: "VARCHAR", Mode: "value", Value: "it's a path C:\\tmp"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	// QuoteTextLit doubles both single-quotes and backslashes.
	assertContains(t, sql, `VALUES ('it''s a path C:\\tmp')`)
}

func TestBuildInsertRowsSql_NullAndDefault(t *testing.T) {
	cfg := oneRow(
		InsertRowValue{Column: "A", DataType: "VARCHAR", Mode: "null"},
		InsertRowValue{Column: "B", DataType: "NUMBER", Mode: "default"},
	)
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("A", "B") VALUES (NULL, DEFAULT);`)
}

func TestBuildInsertRowsSql_Expression(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "CREATED_AT", DataType: "TIMESTAMP_NTZ(9)", Mode: "expression", Value: "CURRENT_TIMESTAMP()"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	// Expressions are emitted verbatim, NOT quoted.
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("CREATED_AT") VALUES (CURRENT_TIMESTAMP());`)
}

func TestBuildInsertRowsSql_Boolean(t *testing.T) {
	cfg := oneRow(
		InsertRowValue{Column: "OK", DataType: "BOOLEAN", Mode: "value", Value: "yes"},
		InsertRowValue{Column: "BAD", DataType: "BOOLEAN", Mode: "value", Value: "false"},
	)
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES (TRUE, FALSE)`)
}

func TestBuildInsertRowsSql_NumericEmptyIsNull(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "N", DataType: "NUMBER(10,2)", Mode: "value", Value: ""})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES (NULL)`)
}

func TestBuildInsertRowsSql_NonNumericValueIsQuoted(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "N", DataType: "NUMBER(10,2)", Mode: "value", Value: "1); DROP TABLE X;--"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	// An invalid numeric is quoted, so no injection escapes the literal.
	assertContains(t, sql, `VALUES ('1); DROP TABLE X;--')`)
}

func TestBuildInsertRowsSql_EmptyStringLiteral(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "S", DataType: "VARCHAR", Mode: "value", Value: ""})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES ('')`)
}

func TestBuildInsertRowsSql_SkipsEmptyColumnName(t *testing.T) {
	cfg := oneRow(
		InsertRowValue{Column: "ID", DataType: "NUMBER", Mode: "value", Value: "1"},
		InsertRowValue{Column: "", DataType: "VARCHAR", Mode: "value", Value: "ignored"},
	)
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("ID") VALUES (1);`)
}

func TestBuildInsertRowsSql_QuotesSpecialIdentifiers(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: `My "Col"`, DataType: "VARCHAR", Mode: "value", Value: "x"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `("My ""Col""")`)
}

func TestBuildInsertRowsSql_MultipleRows(t *testing.T) {
	cfg := InsertRowsConfig{Rows: []InsertRowConfig{
		{Values: []InsertRowValue{
			{Column: "ID", DataType: "NUMBER(38,0)", Mode: "value", Value: "1"},
			{Column: "NAME", DataType: "VARCHAR", Mode: "value", Value: "Alice"},
		}},
		{Values: []InsertRowValue{
			{Column: "ID", DataType: "NUMBER(38,0)", Mode: "value", Value: "2"},
			{Column: "NAME", DataType: "VARCHAR", Mode: "null"},
		}},
		{Values: []InsertRowValue{
			{Column: "ID", DataType: "NUMBER(38,0)", Mode: "default"},
			{Column: "NAME", DataType: "VARCHAR", Mode: "expression", Value: "UPPER('bob')"},
		}},
	}}
	sql, err := BuildInsertRowsSql("DB", "SC", "T", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Column list appears once; one parenthesized tuple per row, comma-separated.
	assertEqual(t, sql,
		`INSERT INTO "DB"."SC"."T" ("ID", "NAME") VALUES (1, 'Alice'), (2, NULL), (DEFAULT, UPPER('bob'));`)
}

func TestBuildInsertRowsSql_NoRows(t *testing.T) {
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", InsertRowsConfig{})
	// Degenerate but syntactically shaped preview before any row is added.
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" () VALUES ();`)
}

func TestBuildInsertRowsSql_MultiRowArityStaysConsistent(t *testing.T) {
	// A later row carrying an empty column name (while row 0 does not) must NOT
	// drop a tuple element: the first row's columns govern arity for every row,
	// so each tuple has exactly as many values as the column list.
	cfg := InsertRowsConfig{Rows: []InsertRowConfig{
		{Values: []InsertRowValue{
			{Column: "A", DataType: "VARCHAR", Mode: "value", Value: "x"},
			{Column: "B", DataType: "VARCHAR", Mode: "value", Value: "y"},
		}},
		{Values: []InsertRowValue{
			{Column: "", DataType: "VARCHAR", Mode: "value", Value: "z"},
			{Column: "B", DataType: "VARCHAR", Mode: "value", Value: "w"},
		}},
	}}
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("A", "B") VALUES ('x', 'y'), ('z', 'w');`)
}

func TestBuildInsertRowsSql_RaggedRowPaddedWithNull(t *testing.T) {
	// A row with fewer values than the first keeps the column list's arity by
	// padding the missing trailing cell with NULL rather than emitting a short tuple.
	cfg := InsertRowsConfig{Rows: []InsertRowConfig{
		{Values: []InsertRowValue{
			{Column: "A", DataType: "VARCHAR", Mode: "value", Value: "x"},
			{Column: "B", DataType: "VARCHAR", Mode: "value", Value: "y"},
		}},
		{Values: []InsertRowValue{
			{Column: "A", DataType: "VARCHAR", Mode: "value", Value: "z"},
		}},
	}}
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("A", "B") VALUES ('x', 'y'), ('z', NULL);`)
}

func TestBuildInsertRowsSql_VariantUsesSelectForm(t *testing.T) {
	// A VARIANT value in "value" mode is rendered via PARSE_JSON, which Snowflake
	// rejects in a VALUES clause — so the whole statement switches to INSERT … SELECT.
	cfg := oneRow(
		InsertRowValue{Column: "ID", DataType: "NUMBER(38,0)", Mode: "value", Value: "1"},
		InsertRowValue{Column: "V", DataType: "VARIANT", Mode: "value", Value: `{"a": 1}`},
	)
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql,
		`INSERT INTO "DB"."SC"."T" ("ID", "V") SELECT 1, PARSE_JSON('{"a": 1}');`)
}

func TestBuildInsertRowsSql_ObjectAndArrayConstructors(t *testing.T) {
	cfg := oneRow(
		InsertRowValue{Column: "OBJ", DataType: "OBJECT", Mode: "value", Value: `{"k": "v"}`},
		InsertRowValue{Column: "ARR", DataType: "ARRAY", Mode: "value", Value: `[1, 2, 3]`},
	)
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql,
		`INSERT INTO "DB"."SC"."T" ("OBJ", "ARR") SELECT TO_OBJECT(PARSE_JSON('{"k": "v"}')), TO_ARRAY(PARSE_JSON('[1, 2, 3]'));`)
}

func TestBuildInsertRowsSql_EmptyVariantIsNullNoSelect(t *testing.T) {
	// An empty semi-structured value is NULL and does not force the SELECT form.
	cfg := oneRow(InsertRowValue{Column: "V", DataType: "VARIANT", Mode: "value", Value: ""})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("V") VALUES (NULL);`)
}

func TestBuildInsertRowsSql_VariantQuotingIsInjectionSafe(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "V", DataType: "VARIANT", Mode: "value", Value: `{"a": "x')--"}`})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	// The single-quote inside the JSON is doubled, so nothing escapes the literal.
	assertContains(t, sql, `PARSE_JSON('{"a": "x'')--"}')`)
}

func TestBuildInsertRowsSql_VectorArrayCast(t *testing.T) {
	cfg := oneRow(InsertRowValue{Column: "VEC", DataType: "VECTOR(FLOAT, 4)", Mode: "value", Value: "[1.0, 2.0, 3.0, 4.0]"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql,
		`INSERT INTO "DB"."SC"."T" ("VEC") SELECT [1.0, 2.0, 3.0, 4.0]::VECTOR(FLOAT, 4);`)
}

func TestBuildInsertRowsSql_InvalidVectorIsQuoted(t *testing.T) {
	// A non-numeric vector element can't be built safely, so the whole value is
	// quoted as a string and stays on the VALUES path for Snowflake to reject.
	cfg := oneRow(InsertRowValue{Column: "VEC", DataType: "VECTOR(FLOAT, 4)", Mode: "value", Value: "[1, oops, 3, 4]"})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("VEC") VALUES ('[1, oops, 3, 4]');`)
}

func TestBuildInsertRowsSql_MultiRowVariantSelectForm(t *testing.T) {
	// Multi-row with a semi-structured column emits SELECT … UNION ALL SELECT ….
	cfg := InsertRowsConfig{Rows: []InsertRowConfig{
		{Values: []InsertRowValue{
			{Column: "ID", DataType: "NUMBER", Mode: "value", Value: "1"},
			{Column: "V", DataType: "VARIANT", Mode: "value", Value: `{"a": 1}`},
		}},
		{Values: []InsertRowValue{
			{Column: "ID", DataType: "NUMBER", Mode: "value", Value: "2"},
			{Column: "V", DataType: "VARIANT", Mode: "null"},
		}},
	}}
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertEqual(t, sql,
		`INSERT INTO "DB"."SC"."T" ("ID", "V") SELECT 1, PARSE_JSON('{"a": 1}') UNION ALL SELECT 2, NULL;`)
}

func TestBuildInsertRowsSql_InvalidNumericPreservesBackslash(t *testing.T) {
	// The invalid-numeric fallback uses QuoteTextLit like the text path, so a
	// backslash survives as a literal character (doubled) rather than being read
	// as a Snowflake string-literal escape.
	cfg := oneRow(InsertRowValue{Column: "N", DataType: "NUMBER(10,2)", Mode: "value", Value: `1\2`})
	sql, _ := BuildInsertRowsSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES ('1\\2')`)
}
