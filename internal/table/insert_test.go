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

func TestBuildInsertRowSql_Basic(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "ID", DataType: "NUMBER(38,0)", Mode: "value", Value: "42"},
		{Column: "NAME", DataType: "VARCHAR(256)", Mode: "value", Value: "Alice"},
	}}
	sql, err := BuildInsertRowSql("DB", "SC", "T", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("ID", "NAME") VALUES (42, 'Alice');`)
}

func TestBuildInsertRowSql_StringEscaping(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "NOTE", DataType: "VARCHAR", Mode: "value", Value: "it's a path C:\\tmp"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	// QuoteTextLit doubles both single-quotes and backslashes.
	assertContains(t, sql, `VALUES ('it''s a path C:\\tmp')`)
}

func TestBuildInsertRowSql_NullAndDefault(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "A", DataType: "VARCHAR", Mode: "null"},
		{Column: "B", DataType: "NUMBER", Mode: "default"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("A", "B") VALUES (NULL, DEFAULT);`)
}

func TestBuildInsertRowSql_Expression(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "CREATED_AT", DataType: "TIMESTAMP_NTZ(9)", Mode: "expression", Value: "CURRENT_TIMESTAMP()"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	// Expressions are emitted verbatim, NOT quoted.
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("CREATED_AT") VALUES (CURRENT_TIMESTAMP());`)
}

func TestBuildInsertRowSql_Boolean(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "OK", DataType: "BOOLEAN", Mode: "value", Value: "yes"},
		{Column: "BAD", DataType: "BOOLEAN", Mode: "value", Value: "false"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES (TRUE, FALSE)`)
}

func TestBuildInsertRowSql_NumericEmptyIsNull(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "N", DataType: "NUMBER(10,2)", Mode: "value", Value: ""},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES (NULL)`)
}

func TestBuildInsertRowSql_NonNumericValueIsQuoted(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "N", DataType: "NUMBER(10,2)", Mode: "value", Value: "1); DROP TABLE X;--"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	// An invalid numeric is quoted, so no injection escapes the literal.
	assertContains(t, sql, `VALUES ('1); DROP TABLE X;--')`)
}

func TestBuildInsertRowSql_EmptyStringLiteral(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "S", DataType: "VARCHAR", Mode: "value", Value: ""},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `VALUES ('')`)
}

func TestBuildInsertRowSql_SkipsEmptyColumnName(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: "ID", DataType: "NUMBER", Mode: "value", Value: "1"},
		{Column: "", DataType: "VARCHAR", Mode: "value", Value: "ignored"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertEqual(t, sql, `INSERT INTO "DB"."SC"."T" ("ID") VALUES (1);`)
}

func TestBuildInsertRowSql_QuotesSpecialIdentifiers(t *testing.T) {
	cfg := InsertRowConfig{Values: []InsertRowValue{
		{Column: `My "Col"`, DataType: "VARCHAR", Mode: "value", Value: "x"},
	}}
	sql, _ := BuildInsertRowSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `("My ""Col""")`)
}
