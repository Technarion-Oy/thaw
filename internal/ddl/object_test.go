// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ddl

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ─── Parse ────────────────────────────────────────────────────────────────────

func TestParse_Kinds(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantKind  Kind
		wantDB    string
		wantSch   string
		wantName  string
		wantArgSig string
	}{
		// ── DATABASE ────────────────────────────────────────────────────────
		{
			name:     "database quoted",
			sql:      `create or replace database "MY_DB"`,
			wantKind: KindDatabase, wantName: "MY_DB",
		},
		{
			name:     "database unquoted",
			sql:      `CREATE OR REPLACE DATABASE MY_DB`,
			wantKind: KindDatabase, wantName: "MY_DB",
		},

		// ── SCHEMA ──────────────────────────────────────────────────────────
		{
			name:     "schema two-part",
			sql:      `create or replace schema "MY_DB"."PUBLIC"`,
			wantKind: KindSchema, wantSch: "MY_DB", wantName: "PUBLIC",
		},
		{
			name:     "schema one-part unquoted",
			sql:      `create schema PUBLIC`,
			wantKind: KindSchema, wantName: "PUBLIC",
		},

		// ── TABLE ───────────────────────────────────────────────────────────
		{
			name:     "table three-part fully-qualified",
			sql:      `CREATE OR REPLACE TABLE "DB"."SCH"."TBL" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "TBL",
		},
		{
			name:     "table two-part",
			sql:      `CREATE TABLE "PUBLIC"."MY_TABLE" (id INT)`,
			wantKind: KindTable, wantSch: "PUBLIC", wantName: "MY_TABLE",
		},
		{
			name:     "table one-part unquoted",
			sql:      `CREATE TABLE orders (id INT)`,
			wantKind: KindTable, wantName: "orders",
		},
		{
			name:     "transient table modifier",
			sql:      `CREATE OR REPLACE TRANSIENT TABLE t (id INT)`,
			wantKind: KindTable, wantName: "t",
		},
		{
			name:     "temporary table modifier",
			sql:      `CREATE OR REPLACE TEMPORARY TABLE t (id INT)`,
			wantKind: KindTable, wantName: "t",
		},
		{
			name:     "external table modifier",
			sql:      `CREATE OR REPLACE EXTERNAL TABLE "DB"."SCH"."EXT" LOCATION=@s`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "EXT",
		},
		{
			name:     "dynamic table modifier",
			sql:      `CREATE OR REPLACE DYNAMIC TABLE "DB"."SCH"."DYN" TARGET_LAG='1 minute' AS SELECT 1`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "DYN",
		},

		// ── VIEW ────────────────────────────────────────────────────────────
		{
			name:     "view three-part",
			sql:      `CREATE OR REPLACE VIEW "DB"."SCH"."VW" AS SELECT 1`,
			wantKind: KindView, wantDB: "DB", wantSch: "SCH", wantName: "VW",
		},
		{
			name:     "secure view modifier",
			sql:      `CREATE OR REPLACE SECURE VIEW v AS SELECT 1`,
			wantKind: KindView, wantName: "v",
		},
		{
			name:     "secure recursive view modifiers",
			sql:      `CREATE OR REPLACE SECURE RECURSIVE VIEW v AS SELECT 1`,
			wantKind: KindView, wantName: "v",
		},
		{
			name:     "materialized view modifier",
			sql:      `CREATE OR REPLACE MATERIALIZED VIEW mv AS SELECT 1`,
			wantKind: KindView, wantName: "mv",
		},

		// ── FUNCTION ────────────────────────────────────────────────────────
		{
			name:      "function three-part with args",
			sql:       `CREATE OR REPLACE FUNCTION "DB"."SCH"."F"(X FLOAT) RETURNS FLOAT AS $$ return X; $$`,
			wantKind:  KindFunction, wantDB: "DB", wantSch: "SCH", wantName: "F",
			wantArgSig: "FLOAT",
		},
		{
			name:      "function no arguments",
			sql:       `CREATE FUNCTION f() RETURNS INT AS $$ return 1; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "noargs",
		},
		{
			name:      "function two args",
			sql:       `CREATE FUNCTION f(A FLOAT, B VARCHAR) RETURNS FLOAT AS $$ return 0; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "FLOAT_VARCHAR",
		},
		{
			name:      "function arg with size qualifier stripped",
			sql:       `CREATE FUNCTION f(X VARCHAR(256)) RETURNS VARCHAR AS $$ return X; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "VARCHAR",
		},
		{
			name:      "function secure modifier",
			sql:       `CREATE OR REPLACE SECURE FUNCTION "DB"."SCH"."F"(X NUMBER) RETURNS NUMBER AS $$ return X; $$`,
			wantKind:  KindFunction, wantDB: "DB", wantSch: "SCH", wantName: "F",
			wantArgSig: "NUMBER",
		},

		// ── PROCEDURE ───────────────────────────────────────────────────────
		{
			name:      "procedure three-part",
			sql:       `CREATE OR REPLACE PROCEDURE "DB"."SCH"."P"(N NUMBER) RETURNS STRING AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantDB: "DB", wantSch: "SCH", wantName: "P",
			wantArgSig: "NUMBER",
		},
		{
			name:      "procedure no args",
			sql:       `CREATE PROCEDURE do_thing() RETURNS VARCHAR AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantName: "do_thing",
			wantArgSig: "noargs",
		},

		// ── SEQUENCE ────────────────────────────────────────────────────────
		{
			name:     "sequence three-part",
			sql:      `CREATE OR REPLACE SEQUENCE "DB"."SCH"."SEQ" START 1 INCREMENT 1`,
			wantKind: KindSequence, wantDB: "DB", wantSch: "SCH", wantName: "SEQ",
		},

		// ── STAGE ───────────────────────────────────────────────────────────
		{
			name:     "stage three-part",
			sql:      `CREATE OR REPLACE STAGE "DB"."SCH"."STG" URL='s3://bucket/'`,
			wantKind: KindStage, wantDB: "DB", wantSch: "SCH", wantName: "STG",
		},

		// ── STREAM ──────────────────────────────────────────────────────────
		{
			name:     "stream three-part",
			sql:      `CREATE OR REPLACE STREAM "DB"."SCH"."STR" ON TABLE t`,
			wantKind: KindStream, wantDB: "DB", wantSch: "SCH", wantName: "STR",
		},

		// ── TASK ────────────────────────────────────────────────────────────
		{
			name:     "task three-part",
			sql:      `CREATE OR REPLACE TASK "DB"."SCH"."TSK" WAREHOUSE=wh AS SELECT 1`,
			wantKind: KindTask, wantDB: "DB", wantSch: "SCH", wantName: "TSK",
		},

		// ── FILE FORMAT ─────────────────────────────────────────────────────
		{
			name:     "file format three-part",
			sql:      `CREATE OR REPLACE FILE FORMAT "DB"."SCH"."FF" TYPE='CSV'`,
			wantKind: KindFileFormat, wantDB: "DB", wantSch: "SCH", wantName: "FF",
		},

		// ── PIPE ────────────────────────────────────────────────────────────
		{
			name:     "pipe three-part",
			sql:      `CREATE OR REPLACE PIPE "DB"."SCH"."PP" AS COPY INTO t FROM @s`,
			wantKind: KindPipe, wantDB: "DB", wantSch: "SCH", wantName: "PP",
		},

		// ── IF NOT EXISTS ────────────────────────────────────────────────────
		{
			name:     "IF NOT EXISTS is skipped",
			sql:      `CREATE TABLE IF NOT EXISTS "DB"."SCH"."T" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "T",
		},

		// ── case insensitivity ───────────────────────────────────────────────
		{
			name:     "lowercase create table",
			sql:      `create or replace table "PUBLIC"."my_table" (id int)`,
			wantKind: KindTable, wantSch: "PUBLIC", wantName: "my_table",
		},

		// ── quoted identifier with embedded double-quote ─────────────────────
		{
			name:     `double-quote escape in table name`,
			sql:      `CREATE TABLE "MY""TABLE" (id INT)`,
			wantKind: KindTable, wantName: `MY"TABLE`,
		},

		// ── quoted names with special characters ─────────────────────────────
		{
			name:     "table name with embedded space",
			sql:      `CREATE TABLE "MY DB"."MY SCHEMA"."MY TABLE" (id INT)`,
			wantKind: KindTable, wantDB: "MY DB", wantSch: "MY SCHEMA", wantName: "MY TABLE",
		},
		{
			name:     "object name with dot inside quotes",
			sql:      `CREATE TABLE "DB"."SCH.1"."TBL.2" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH.1", wantName: "TBL.2",
		},
		{
			name:     "object name with semicolon inside quotes",
			sql:      `CREATE TABLE "DB"."SCH"."tbl;1" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "tbl;1",
		},
		{
			name:     "object name with hyphen inside quotes",
			sql:      `CREATE TABLE "DB"."SCH"."my-table" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "my-table",
		},
		{
			name:     "object name with dollar sign inside quotes",
			sql:      `CREATE TABLE "DB"."SCH"."$TEMP" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "$TEMP",
		},
		{
			name:     "object name with open paren inside quotes",
			sql:      `CREATE TABLE "DB"."SCH"."tbl(1)" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "tbl(1)",
		},
		{
			name:     "multiple double-quote escapes in name",
			sql:      `CREATE TABLE "A""B""C" (id INT)`,
			wantKind: KindTable, wantName: `A"B"C`,
		},

		// ── unicode object names ──────────────────────────────────────────────
		{
			name:     "unicode table name",
			sql:      `CREATE TABLE "DB"."SCH"."données" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "données",
		},
		{
			name:     "japanese table name",
			sql:      `CREATE TABLE "DB"."SCH"."注文" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "注文",
		},

		// ── leading / trailing whitespace in input SQL ────────────────────────
		{
			name:     "leading and trailing whitespace in SQL",
			sql:      "  \n  CREATE TABLE t (id INT)  \n  ",
			wantKind: KindTable, wantName: "t",
		},

		// ── very long names ───────────────────────────────────────────────────
		{
			name: "very long unquoted identifier",
			sql:  `CREATE TABLE ` + strings.Repeat("A", 128) + ` (id INT)`,
			wantKind: KindTable, wantName: strings.Repeat("A", 128),
		},

		// ── additional modifiers ──────────────────────────────────────────────
		{
			name:     "iceberg table modifier",
			sql:      `CREATE OR REPLACE ICEBERG TABLE "DB"."SCH"."ICE" EXTERNAL_VOLUME='vol'`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "ICE",
		},
		{
			name:     "event table modifier",
			sql:      `CREATE OR REPLACE EVENT TABLE "DB"."SCH"."EVT"`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "EVT",
		},

		// ── functions / procedures with precision args ────────────────────────
		{
			name:      "function with NUMBER precision args",
			sql:       `CREATE FUNCTION f(A NUMBER(18,2), B NUMBER(38,0)) RETURNS NUMBER AS $$ return 0; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "NUMBER_NUMBER",
		},
		{
			name:      "function with mixed precision and plain args",
			sql:       `CREATE FUNCTION f(A VARCHAR(256), B FLOAT, C NUMBER(10,4)) RETURNS FLOAT AS $$ return 0; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "VARCHAR_FLOAT_NUMBER",
		},
		{
			name:      "python procedure with multiple args",
			sql:       `CREATE OR REPLACE PROCEDURE "DB"."SCH"."PY_PROC"(N NUMBER, S VARCHAR(256)) RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION='3.11' AS $$pass$$`,
			wantKind:  KindProcedure, wantDB: "DB", wantSch: "SCH", wantName: "PY_PROC",
			wantArgSig: "NUMBER_VARCHAR",
		},

		// ── pipe / stream with quoted names containing spaces ─────────────────
		{
			name:     "pipe with space in quoted name",
			sql:      `CREATE OR REPLACE PIPE "DB"."SCH"."my pipe" AS COPY INTO t`,
			wantKind: KindPipe, wantDB: "DB", wantSch: "SCH", wantName: "my pipe",
		},
		{
			name:     "stream with hyphen in quoted name",
			sql:      `CREATE OR REPLACE STREAM "DB"."SCH"."orders-stream" ON TABLE t`,
			wantKind: KindStream, wantDB: "DB", wantSch: "SCH", wantName: "orders-stream",
		},

		// ── VOLATILE modifier ────────────────────────────────────────────────
		{
			name:     "volatile table modifier",
			sql:      `CREATE OR REPLACE VOLATILE TABLE "DB"."SCH"."VOL" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "VOL",
		},

		// ── SQL reserved words as object names ───────────────────────────────
		{
			name:     "SQL keyword SELECT as table name",
			sql:      `CREATE TABLE "DB"."SCH"."SELECT" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "SELECT",
		},
		{
			name:     "SQL keyword CREATE as view name",
			sql:      `CREATE OR REPLACE VIEW "DB"."SCH"."CREATE" AS SELECT 1`,
			wantKind: KindView, wantDB: "DB", wantSch: "SCH", wantName: "CREATE",
		},
		{
			name:     "SQL keyword FROM as schema name",
			sql:      `CREATE TABLE "DB"."FROM"."TBL" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "FROM", wantName: "TBL",
		},

		// ── object names with path separator characters ───────────────────────
		{
			name:     "object name with forward slash",
			sql:      `CREATE TABLE "DB"."SCH"."path/to/table" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "path/to/table",
		},
		{
			name:     "object name with backslash",
			sql:      `CREATE TABLE "DB"."SCH"."path\table" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: `path\table`,
		},
		{
			name:     "database name with forward slash",
			sql:      `CREATE TABLE "db/1"."SCH"."TBL" (id INT)`,
			wantKind: KindTable, wantDB: "db/1", wantSch: "SCH", wantName: "TBL",
		},

		// ── object names with single quotes ───────────────────────────────────
		{
			name:     "table name with single quote inside double-quoted name",
			sql:      `CREATE TABLE "DB"."SCH"."it's a table" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "it's a table",
		},

		// ── object names with newline / tab ───────────────────────────────────
		{
			name:     "table name with embedded tab",
			sql:      "CREATE TABLE \"DB\".\"SCH\".\"col\tname\" (id INT)",
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "col\tname",
		},

		// ── whitespace normalization in CREATE keyword ────────────────────────
		{
			name:     "tabs between keywords instead of spaces",
			sql:      "CREATE\t\tOR\t\tREPLACE\t\tTABLE\t\t\"DB\".\"SCH\".\"T\" (id INT)",
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "T",
		},
		{
			name:     "mixed whitespace between keywords",
			sql:      "CREATE  \t  OR  \t  REPLACE  \t  VIEW v AS SELECT 1",
			wantKind: KindView, wantName: "v",
		},

		// ── very long object names ────────────────────────────────────────────
		{
			name: "table name at Snowflake maximum length (255 chars)",
			sql:  `CREATE TABLE "DB"."SCH"."` + strings.Repeat("A", 255) + `" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH",
			wantName: strings.Repeat("A", 255),
		},
		{
			name: "fully-qualified name all parts at 64 chars",
			sql: `CREATE VIEW "` + strings.Repeat("D", 64) + `"."` +
				strings.Repeat("S", 64) + `"."` + strings.Repeat("V", 64) + `" AS SELECT 1`,
			wantKind: KindView,
			wantDB:   strings.Repeat("D", 64),
			wantSch:  strings.Repeat("S", 64),
			wantName: strings.Repeat("V", 64),
		},

		// ── IF NOT EXISTS on all major object kinds ───────────────────────────
		{
			name:     "CREATE DATABASE IF NOT EXISTS",
			sql:      `CREATE DATABASE IF NOT EXISTS "MY_DB"`,
			wantKind: KindDatabase, wantName: "MY_DB",
		},
		{
			name:     "CREATE SCHEMA IF NOT EXISTS three-part",
			sql:      `CREATE SCHEMA IF NOT EXISTS "DB"."PUBLIC"`,
			wantKind: KindSchema, wantSch: "DB", wantName: "PUBLIC",
		},
		{
			name:     "CREATE VIEW IF NOT EXISTS",
			sql:      `CREATE VIEW IF NOT EXISTS "DB"."SCH"."V" AS SELECT 1`,
			wantKind: KindView, wantDB: "DB", wantSch: "SCH", wantName: "V",
		},

		// ── function / procedure extreme name and arg cases ──────────────────
		{
			name:      "function name with no space before open paren (unquoted)",
			sql:       `CREATE FUNCTION f(X FLOAT) RETURNS FLOAT AS $$ return X; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "FLOAT",
		},
		{
			name:      "function with ARRAY type parameter",
			sql:       `CREATE FUNCTION arr(A ARRAY) RETURNS VARIANT AS $$ return A; $$`,
			wantKind:  KindFunction, wantName: "arr",
			wantArgSig: "ARRAY",
		},
		{
			name:      "function with MAP type parameter",
			sql:       `CREATE FUNCTION mp(M MAP(VARCHAR, NUMBER)) RETURNS VARIANT AS $$ return M; $$`,
			wantKind:  KindFunction, wantName: "mp",
			wantArgSig: "MAP",
		},
		{
			name:      "function with VECTOR type parameter",
			sql:       `CREATE FUNCTION vec(V VECTOR(FLOAT, 64)) RETURNS FLOAT AS $$ return V[0]; $$`,
			wantKind:  KindFunction, wantName: "vec",
			wantArgSig: "VECTOR",
		},
		{
			name:      "procedure with ten parameters",
			sql: `CREATE PROCEDURE "DB"."SCH"."P"(A VARCHAR, B NUMBER, C FLOAT, D DATE,` +
				` E BOOLEAN, F TIMESTAMP_NTZ, G VARIANT, H ARRAY, I OBJECT, J BINARY)` +
				` RETURNS VARCHAR AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantDB: "DB", wantSch: "SCH", wantName: "P",
			wantArgSig: "VARCHAR_NUMBER_FLOAT_DATE_BOOLEAN_TIMESTAMP_NTZ_VARIANT_ARRAY_OBJECT_BINARY",
		},
		{
			name:      "function name is a SQL keyword",
			sql:       `CREATE FUNCTION "DB"."SCH"."SELECT"(X FLOAT) RETURNS FLOAT AS $$ return X; $$`,
			wantKind:  KindFunction, wantDB: "DB", wantSch: "SCH", wantName: "SELECT",
			wantArgSig: "FLOAT",
		},
		{
			name:      "procedure with all-special-char quoted name",
			sql:       `CREATE PROCEDURE "DB"."SCH"."!@#$%"() RETURNS VARCHAR AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantDB: "DB", wantSch: "SCH", wantName: "!@#$%",
			wantArgSig: "noargs",
		},

		// ── non-CREATE / unknown statements ──────────────────────────────────
		{
			name:     "SELECT is unknown",
			sql:      "SELECT 1",
			wantKind: KindUnknown,
		},
		{
			name:     "DROP TABLE is unknown",
			sql:      "DROP TABLE t",
			wantKind: KindUnknown,
		},
		{
			name:     "ALTER TABLE is unknown",
			sql:      "ALTER TABLE t ADD COLUMN x INT",
			wantKind: KindUnknown,
		},
		{
			name:     "USE DATABASE is unknown",
			sql:      "USE DATABASE MY_DB",
			wantKind: KindUnknown,
		},
		{
			name:     "empty string is unknown",
			sql:      "",
			wantKind: KindUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.sql)

			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if got.Database != tt.wantDB {
				t.Errorf("Database = %q, want %q", got.Database, tt.wantDB)
			}
			if got.Schema != tt.wantSch {
				t.Errorf("Schema = %q, want %q", got.Schema, tt.wantSch)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.ArgSig != tt.wantArgSig {
				t.Errorf("ArgSig = %q, want %q", got.ArgSig, tt.wantArgSig)
			}
			if got.SQL != tt.sql {
				t.Errorf("SQL not preserved:\ngot:  %q\nwant: %q", got.SQL, tt.sql)
			}
		})
	}
}

// ─── parseArgSig ─────────────────────────────────────────────────────────────

func TestParseArgSig(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"()", "noargs"},
		{"(  )", "noargs"},
		{"(X FLOAT)", "FLOAT"},
		{"(X FLOAT, Y VARCHAR)", "FLOAT_VARCHAR"},
		{"(X FLOAT, Y VARCHAR, Z NUMBER)", "FLOAT_VARCHAR_NUMBER"},
		// Size qualifiers stripped.
		{"(X VARCHAR(256))", "VARCHAR"},
		{"(X NUMBER(38,0))", "NUMBER"},
		// Positional params (type only, no name).
		{"(FLOAT)", "FLOAT"},
		// No opening paren → empty.
		{"FLOAT", ""},
		{"", ""},
		// Unclosed paren → empty.
		{"(FLOAT", ""},

		// Multiple params each with precision qualifiers.
		{"(A NUMBER(18,2), B VARCHAR(256))", "NUMBER_VARCHAR"},
		{"(A NUMBER(18,2), B NUMBER(38,0), C FLOAT)", "NUMBER_NUMBER_FLOAT"},
		// Five args — no precision.
		{"(A FLOAT, B INT, C DATE, D BOOLEAN, E VARIANT)", "FLOAT_INT_DATE_BOOLEAN_VARIANT"},
		// TABLE type with inline column definition list.
		{"(T TABLE(X FLOAT, Y NUMBER))", "TABLE"},
		// DEFAULT keyword after type — only the type is captured.
		{"(X FLOAT DEFAULT 1.0)", "FLOAT"},
		{"(X VARCHAR(256) DEFAULT 'hello')", "VARCHAR"},
		// OBJECT type with nested parens.
		{"(O OBJECT(k VARCHAR, v VARIANT))", "OBJECT"},
		// Deeply nested precision: not real Snowflake but tests depth tracking.
		{"(A NESTED((1,2),3))", "NESTED"},
		// Whitespace-heavy formatting.
		{"(  X   FLOAT  ,  Y   VARCHAR  )", "FLOAT_VARCHAR"},
		// All-caps complex type names preserved.
		{"(X TIMESTAMP_NTZ)", "TIMESTAMP_NTZ"},
		{"(X TIMESTAMP_LTZ)", "TIMESTAMP_LTZ"},
		// Trailing content after closing paren is ignored.
		{"(X FLOAT) RETURNS FLOAT AS $$ x $$", "FLOAT"},

		// All positional (no names) — type is the only field.
		{"(FLOAT, VARCHAR, NUMBER)", "FLOAT_VARCHAR_NUMBER"},
		{"(FLOAT)", "FLOAT"},
		// ARRAY, MAP, VECTOR complex types (named params — name precedes type).
		{"(A ARRAY)", "ARRAY"},
		{"(A MAP(VARCHAR, NUMBER))", "MAP"},
		{"(A VECTOR(FLOAT, 64))", "VECTOR"},
		// Positional TABLE type with inline column list: fields[1] (not fields[0])
		// is used when len(fields) >= 2, so the first word after TABLE( is picked up
		// as a "field" and the actual type extraction gives "FLOAT_" not "TABLE".
		// This is the documented (if surprising) behaviour for positional TABLE params.
		{"(TABLE(X FLOAT, Y NUMBER))", "FLOAT_"},
		// Ten parameters.
		{"(A VARCHAR, B NUMBER, C FLOAT, D DATE, E BOOLEAN, F TIMESTAMP_NTZ, G VARIANT, H ARRAY, I OBJECT, J BINARY)",
			"VARCHAR_NUMBER_FLOAT_DATE_BOOLEAN_TIMESTAMP_NTZ_VARIANT_ARRAY_OBJECT_BINARY"},
		// Comma inside single-quoted DEFAULT value splits at depth 0 —
		// this is expected (and documented) behaviour: the parser does not track
		// quote context inside parameter lists.
		{"(X VARCHAR DEFAULT 'hello, world')", "VARCHAR_WORLD_"},
		// Only-whitespace inner content.
		{"(   )", "noargs"},
		// Unclosed inner paren: depth never returns to 0, so end=-1 → "".
		{"(X NESTED(1,2)", ""},
		// Single param with very long type name.
		{"(A " + strings.Repeat("X", 60) + ")", sanitize(strings.Repeat("X", 60))},
		// Param where type name contains digits (e.g. FLOAT4).
		{"(X FLOAT4)", "FLOAT4"},
		// Param with GEOGRAPHY / GEOMETRY types.
		{"(G GEOGRAPHY)", "GEOGRAPHY"},
		{"(G GEOMETRY)", "GEOMETRY"},
		// Very deeply nested brackets.
		{"(A FUNC(FUNC2(FUNC3(x,y),z),w))", "FUNC"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseArgSig(tt.input); got != tt.want {
				t.Errorf("parseArgSig(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── sanitize ────────────────────────────────────────────────────────────────

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MY_TABLE", "MY_TABLE"},
		{"my-table", "my-table"},
		{"MY TABLE", "MY_TABLE"},    // space → underscore
		{"MY.TABLE", "MY_TABLE"},    // dot → underscore
		{"MY\"TABLE", "MY_TABLE"},   // quote → underscore
		{"schema;name", "schema_name"},
		{"", ""},
		{"abc123", "abc123"},
		{"MY__TABLE", "MY__TABLE"},  // double underscore preserved
		// Path separator characters.
		{"path/to/file", "path_to_file"},
		{`path\to\file`, "path_to_file"},
		// Tab and other whitespace.
		{"col\tname", "col_name"},
		{"\n\r\t", "___"},
		// Single quote, brackets, operators.
		{"it's a table", "it_s_a_table"},
		{"[bracket]", "_bracket_"},
		{"col+1", "col_1"},
		{"col*2", "col_2"},
		{"a=b", "a_b"},
		// All allowed characters together.
		{"aZ0_-", "aZ0_-"},
		// Alternating valid / invalid characters.
		{"a!b@c#d", "a_b_c_d"},
		// Only path separators.
		{"///", "___"},
		// Digits only.
		{"0123456789", "0123456789"},
		// Single characters.
		{"a", "a"},
		{"!", "_"},
		{"-", "-"},
		{"_", "_"},
		// Very long valid string (performance check).
		{strings.Repeat("aA0_-", 50), strings.Repeat("aA0_-", 50)},
		// Very long string with every-other char invalid.
		{strings.Repeat("a!", 50), strings.Repeat("a_", 50)},
		// Unicode: each non-ASCII rune becomes an underscore.
		{"café", "caf_"},
		{"注文テーブル", "______"},
		{"naïve", "na_ve"},
		{"emoji🔥table", "emoji_table"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitize(tt.input); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── FilePath ────────────────────────────────────────────────────────────────

func TestFilePath(t *testing.T) {
	fp := func(parts ...string) string { return filepath.Join(parts...) }

	tests := []struct {
		name string
		obj  Object
		want string
	}{
		// DATABASE always maps to the root sentinel file.
		{
			name: "database",
			obj:  Object{Kind: KindDatabase, Name: "MY_DB"},
			want: "_database.sql",
		},
		// SCHEMA goes into the schemas/ directory.
		{
			name: "schema",
			obj:  Object{Kind: KindSchema, Name: "PUBLIC"},
			want: fp("schemas", "PUBLIC.sql"),
		},
		{
			name: "schema with special chars sanitized",
			obj:  Object{Kind: KindSchema, Name: "MY SCHEMA"},
			want: fp("schemas", "MY_SCHEMA.sql"),
		},
		// Regular objects: <schema>/<dir>/<name>.sql
		{
			name: "table",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "MY_TABLE"},
			want: fp("PUBLIC", "tables", "MY_TABLE.sql"),
		},
		{
			name: "view",
			obj:  Object{Kind: KindView, Schema: "PUBLIC", Name: "MY_VIEW"},
			want: fp("PUBLIC", "views", "MY_VIEW.sql"),
		},
		{
			name: "sequence",
			obj:  Object{Kind: KindSequence, Schema: "PUBLIC", Name: "MY_SEQ"},
			want: fp("PUBLIC", "sequences", "MY_SEQ.sql"),
		},
		{
			name: "stage",
			obj:  Object{Kind: KindStage, Schema: "PUBLIC", Name: "MY_STG"},
			want: fp("PUBLIC", "stages", "MY_STG.sql"),
		},
		{
			name: "stream",
			obj:  Object{Kind: KindStream, Schema: "PUBLIC", Name: "MY_STR"},
			want: fp("PUBLIC", "streams", "MY_STR.sql"),
		},
		{
			name: "task",
			obj:  Object{Kind: KindTask, Schema: "PUBLIC", Name: "MY_TSK"},
			want: fp("PUBLIC", "tasks", "MY_TSK.sql"),
		},
		{
			name: "file format",
			obj:  Object{Kind: KindFileFormat, Schema: "PUBLIC", Name: "MY_FF"},
			want: fp("PUBLIC", "file_formats", "MY_FF.sql"),
		},
		{
			name: "pipe",
			obj:  Object{Kind: KindPipe, Schema: "PUBLIC", Name: "MY_PP"},
			want: fp("PUBLIC", "pipes", "MY_PP.sql"),
		},
		// Functions/procedures include the arg signature.
		{
			name: "function with args",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: "FLOAT"},
			want: fp("PUBLIC", "functions", "F__FLOAT.sql"),
		},
		{
			name: "function no args",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: "noargs"},
			want: fp("PUBLIC", "functions", "F__noargs.sql"),
		},
		{
			name: "function empty argsig (no parens found)",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: ""},
			want: fp("PUBLIC", "functions", "F.sql"),
		},
		{
			name: "procedure with args",
			obj:  Object{Kind: KindProcedure, Schema: "SCH", Name: "P", ArgSig: "NUMBER_VARCHAR"},
			want: fp("SCH", "procedures", "P__NUMBER_VARCHAR.sql"),
		},
		// Empty schema falls back to _root.
		{
			name: "table without schema",
			obj:  Object{Kind: KindTable, Schema: "", Name: "T"},
			want: fp("_root", "tables", "T.sql"),
		},
		{
			name: "function without schema",
			obj:  Object{Kind: KindFunction, Schema: "", Name: "F", ArgSig: "noargs"},
			want: fp("_root", "functions", "F__noargs.sql"),
		},
		// Name sanitization.
		{
			name: "table name with space",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "MY TABLE"},
			want: fp("PUBLIC", "tables", "MY_TABLE.sql"),
		},
		{
			name: "schema name with dot",
			obj:  Object{Kind: KindTable, Schema: "MY.SCHEMA", Name: "T"},
			want: fp("MY_SCHEMA", "tables", "T.sql"),
		},
		// Unicode names: non-ASCII characters become underscores.
		{
			name: "unicode table name sanitized to underscores",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "données"},
			want: fp("PUBLIC", "tables", "donn_es.sql"),
		},
		{
			name: "japanese table name fully sanitized",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "注文"},
			want: fp("PUBLIC", "tables", "__.sql"),
		},
		// Name consisting entirely of special characters.
		{
			name: "name with only special chars becomes underscores",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: ";:@#!"},
			want: fp("PUBLIC", "tables", "_____.sql"),
		},
		// KindUnknown uses the "other" fallback directory.
		{
			name: "unknown kind uses other directory",
			obj:  Object{Kind: KindUnknown, Schema: "PUBLIC", Name: "X"},
			want: fp("PUBLIC", "other", "X.sql"),
		},
		// Hyphen in name is preserved (it's in sanitize's allowed set).
		{
			name: "hyphen in name is preserved",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "my-table"},
			want: fp("SCH", "tables", "my-table.sql"),
		},
		// Dollar sign in name becomes underscore.
		{
			name: "dollar sign in name sanitized",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "$TEMP"},
			want: fp("SCH", "tables", "_TEMP.sql"),
		},
		// Very long name passes through unchanged (only chars matter, not length).
		{
			name: "very long name",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: strings.Repeat("A", 128)},
			want: fp("SCH", "tables", strings.Repeat("A", 128)+".sql"),
		},
		// Path separator in name is sanitized.
		{
			name: "forward slash in name becomes underscore",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "path/to/table"},
			want: fp("SCH", "tables", "path_to_table.sql"),
		},
		{
			name: "backslash in name becomes underscore",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: `path\table`},
			want: fp("SCH", "tables", "path_table.sql"),
		},
		// Path separator in schema is sanitized.
		{
			name: "forward slash in schema becomes underscore",
			obj:  Object{Kind: KindView, Schema: "MY/SCH", Name: "V"},
			want: fp("MY_SCH", "views", "V.sql"),
		},
		// Single quote in name sanitized.
		{
			name: "single quote in name becomes underscore",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "it's a table"},
			want: fp("SCH", "tables", "it_s_a_table.sql"),
		},
		// Tab in name sanitized.
		{
			name: "tab in name becomes underscore",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "col\tname"},
			want: fp("SCH", "tables", "col_name.sql"),
		},
		// Emoji in schema → underscores.
		{
			name: "emoji in schema sanitized",
			obj:  Object{Kind: KindTable, Schema: "SCH🔥1", Name: "T"},
			want: fp("SCH_1", "tables", "T.sql"),
		},
		// Leading/trailing hyphens preserved.
		{
			name: "leading and trailing hyphens preserved",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "-my-table-"},
			want: fp("SCH", "tables", "-my-table-.sql"),
		},
		// Function with very long arg sig.
		{
			name: "function with very long arg sig",
			obj:  Object{Kind: KindFunction, Schema: "SCH", Name: "F",
				ArgSig: strings.Repeat("VARCHAR_", 10) + "NUMBER"},
			want: fp("SCH", "functions", "F__"+strings.Repeat("VARCHAR_", 10)+"NUMBER.sql"),
		},
		// All object kinds with no schema fall back to _root.
		{
			name: "stage without schema uses _root",
			obj:  Object{Kind: KindStage, Schema: "", Name: "STG"},
			want: fp("_root", "stages", "STG.sql"),
		},
		{
			name: "pipe without schema uses _root",
			obj:  Object{Kind: KindPipe, Schema: "", Name: "PP"},
			want: fp("_root", "pipes", "PP.sql"),
		},
		{
			name: "file_format without schema uses _root",
			obj:  Object{Kind: KindFileFormat, Schema: "", Name: "FF"},
			want: fp("_root", "file_formats", "FF.sql"),
		},
		// Name that would produce an empty sanitized string (all special chars + digits).
		{
			name: "name with all numeric digits preserved",
			obj:  Object{Kind: KindTable, Schema: "SCH", Name: "0123"},
			want: fp("SCH", "tables", "0123.sql"),
		},
		// Schema that itself would be _root if used as fallback.
		{
			name: "schema literally named _root",
			obj:  Object{Kind: KindTable, Schema: "_root", Name: "T"},
			want: fp("_root", "tables", "T.sql"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obj.FilePath(); got != tt.want {
				t.Errorf("FilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── FilePathFor ──────────────────────────────────────────────────────────────

func TestFilePathFor(t *testing.T) {
	fp := func(parts ...string) string { return filepath.Join(parts...) }

	tests := []struct {
		name     string
		obj      Object
		template string
		database string
		want     string
	}{
		// DATABASE and SCHEMA always use fixed paths regardless of template.
		{
			name:     "database ignores custom template",
			obj:      Object{Kind: KindDatabase},
			template: "custom/{object_name}.sql",
			database: "MY_DB",
			want:     fp("MY_DB", "_database.sql"),
		},
		{
			name:     "database with special chars in db name",
			obj:      Object{Kind: KindDatabase},
			template: "",
			database: "MY DB",
			want:     fp("MY_DB", "_database.sql"),
		},
		{
			name:     "schema ignores custom template",
			obj:      Object{Kind: KindSchema, Name: "PUBLIC"},
			template: "flat/{object_name}.sql",
			database: "MY_DB",
			want:     fp("MY_DB", "schemas", "PUBLIC.sql"),
		},
		{
			name:     "schema with special chars in schema name",
			obj:      Object{Kind: KindSchema, Name: "MY SCHEMA"},
			template: "",
			database: "DB",
			want:     fp("DB", "schemas", "MY_SCHEMA.sql"),
		},

		// Empty template falls back to DefaultExportPathTemplate.
		{
			name:     "empty template uses default layout",
			obj:      Object{Kind: KindTable, Schema: "PUBLIC", Name: "T"},
			template: "",
			database: "MY_DB",
			want:     fp("MY_DB", "PUBLIC", "tables", "T.sql"),
		},

		// Custom template with all four placeholders.
		{
			name:     "custom template with all placeholders",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "objects/{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("objects", "DB", "SCH", "tables", "T.sql"),
		},

		// Custom template omitting schema placeholder.
		{
			name:     "custom template omitting schema",
			obj:      Object{Kind: KindView, Schema: "PUBLIC", Name: "V"},
			template: "{database}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "views", "V.sql"),
		},

		// Custom template omitting database placeholder.
		{
			name:     "custom template omitting database",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("SCH", "tables", "T.sql"),
		},

		// Function: {object_name} includes the __argsig suffix.
		{
			name:     "function argsig included in object_name placeholder",
			obj:      Object{Kind: KindFunction, Schema: "SCH", Name: "F", ArgSig: "FLOAT"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "SCH", "functions", "F__FLOAT.sql"),
		},
		{
			name:     "procedure argsig included in object_name placeholder",
			obj:      Object{Kind: KindProcedure, Schema: "SCH", Name: "P", ArgSig: "NUMBER_VARCHAR"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "SCH", "procedures", "P__NUMBER_VARCHAR.sql"),
		},

		// Special characters in various components are sanitized.
		{
			name:     "special chars in database name sanitized",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "MY DB",
			want:     fp("MY_DB", "SCH", "tables", "T.sql"),
		},
		{
			name:     "special chars in schema name sanitized",
			obj:      Object{Kind: KindTable, Schema: "MY.SCHEMA", Name: "T"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "MY_SCHEMA", "tables", "T.sql"),
		},
		{
			name:     "special chars in object name sanitized",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "MY TABLE"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "SCH", "tables", "MY_TABLE.sql"),
		},

		// Empty schema falls back to _root.
		{
			name:     "empty schema uses _root",
			obj:      Object{Kind: KindTable, Schema: "", Name: "T"},
			template: "",
			database: "DB",
			want:     fp("DB", "_root", "tables", "T.sql"),
		},

		// Flat template — single file per object regardless of type.
		{
			name:     "flat template produces single level",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{object_name}.sql",
			database: "DB",
			want:     "T.sql",
		},
		// Template with no placeholders at all: every object maps to the same path.
		{
			name:     "template with no placeholders",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "output/all.sql",
			database: "DB",
			want:     fp("output", "all.sql"),
		},
		// Template with a repeated placeholder.
		{
			name:     "template with repeated database placeholder",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{database}/{database}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "DB", "T.sql"),
		},
		// Template with an unknown placeholder passes through literally.
		{
			name:     "unknown placeholder passes through unchanged",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{database}/{version}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "{version}", "T.sql"),
		},
		// Template with only the object_type placeholder.
		{
			name:     "template with only object_type and object_name",
			obj:      Object{Kind: KindView, Schema: "SCH", Name: "V"},
			template: "{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("views", "V.sql"),
		},
		// Special chars in database name sanitized in FilePathFor.
		{
			name:     "path separator in database name sanitized",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{database}/{object_name}.sql",
			database: "db/path",
			want:     fp("db_path", "T.sql"),
		},
		{
			name:     "unicode in database name sanitized",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "T"},
			template: "{database}/{object_name}.sql",
			database: "データ", // 3 katakana runes → 3 underscores
			want:     fp("___", "T.sql"),
		},
		// Special chars in object name sanitized.
		{
			name:     "slash in object name sanitized",
			obj:      Object{Kind: KindTable, Schema: "SCH", Name: "a/b"},
			template: "{database}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "a_b.sql"),
		},
		// DATABASE and SCHEMA special-case: unicode in database arg sanitized.
		// "my データ db": m,y,<space>,デ,ー,タ,<space>,d,b → invalid: space+3 kana+space = 5 → "my_____db"
		{
			name:     "DATABASE with unicode db arg sanitized",
			obj:      Object{Kind: KindDatabase},
			template: "",
			database: "my データ db",
			want:     fp("my_____db", "_database.sql"),
		},
		// Function with empty schema and argsig in FilePathFor.
		{
			name:     "function no schema uses _root with argsig",
			obj:      Object{Kind: KindFunction, Schema: "", Name: "F", ArgSig: "FLOAT_VARCHAR"},
			template: "{database}/{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("DB", "_root", "functions", "F__FLOAT_VARCHAR.sql"),
		},
		// Procedure with no args.
		{
			name:     "procedure noargs in custom template",
			obj:      Object{Kind: KindProcedure, Schema: "SCH", Name: "P", ArgSig: "noargs"},
			template: "{schema}/{object_type}/{object_name}.sql",
			database: "DB",
			want:     fp("SCH", "procedures", "P__noargs.sql"),
		},
		// Template that is just a filename with no directory component.
		{
			name:     "template with just object_name and extension",
			obj:      Object{Kind: KindSequence, Schema: "SCH", Name: "SEQ"},
			template: "{object_name}.sql",
			database: "DB",
			want:     "SEQ.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.obj.FilePathFor(tt.template, tt.database)
			if got != tt.want {
				t.Errorf("FilePathFor(%q, %q) = %q, want %q", tt.template, tt.database, got, tt.want)
			}
		})
	}
}

// ─── nameTracker ─────────────────────────────────────────────────────────────

func TestNameTracker_FirstCallReturnsOriginal(t *testing.T) {
	nt := newNameTracker()
	got := nt.resolve("foo.sql")
	if got != "foo.sql" {
		t.Errorf("resolve() = %q, want %q", got, "foo.sql")
	}
}

func TestNameTracker_SecondCallGetsSuffix(t *testing.T) {
	nt := newNameTracker()
	nt.resolve("foo.sql")
	got := nt.resolve("foo.sql")
	if got != "foo_2.sql" {
		t.Errorf("resolve() second = %q, want %q", got, "foo_2.sql")
	}
}

func TestNameTracker_ThirdCallGetsNextSuffix(t *testing.T) {
	nt := newNameTracker()
	nt.resolve("foo.sql")
	nt.resolve("foo.sql")
	got := nt.resolve("foo.sql")
	if got != "foo_3.sql" {
		t.Errorf("resolve() third = %q, want %q", got, "foo_3.sql")
	}
}

func TestNameTracker_CollisionWithGeneratedSuffix(t *testing.T) {
	nt := newNameTracker()
	// Register foo.sql and foo_2.sql independently before any collision.
	nt.resolve("foo.sql")
	nt.resolve("foo_2.sql") // this is legitimately named foo_2

	// Now a collision on foo.sql must skip foo_2 (already taken) and use foo_3.
	got := nt.resolve("foo.sql")
	if got != "foo_3.sql" {
		t.Errorf("resolve() after pre-registered suffix = %q, want %q", got, "foo_3.sql")
	}
}

func TestNameTracker_DifferentPathsDontInterfere(t *testing.T) {
	nt := newNameTracker()
	got1 := nt.resolve("a.sql")
	got2 := nt.resolve("b.sql")
	if got1 != "a.sql" {
		t.Errorf("first path = %q, want %q", got1, "a.sql")
	}
	if got2 != "b.sql" {
		t.Errorf("second path = %q, want %q", got2, "b.sql")
	}
}

func TestNameTracker_PathWithSubdirectory(t *testing.T) {
	nt := newNameTracker()
	path := filepath.Join("PUBLIC", "tables", "T.sql")
	got := nt.resolve(path)
	if got != path {
		t.Errorf("resolve() = %q, want %q", got, path)
	}
	// Collision keeps the directory prefix.
	want := filepath.Join("PUBLIC", "tables", "T_2.sql")
	got = nt.resolve(path)
	if got != want {
		t.Errorf("resolve() collision = %q, want %q", got, want)
	}
}

func TestNameTracker_AllResultsUnique(t *testing.T) {
	// Resolve the same path many times and verify no duplicates.
	nt := newNameTracker()
	seen := make(map[string]bool)
	const n = 20
	for i := 0; i < n; i++ {
		p := nt.resolve("obj.sql")
		if seen[p] {
			t.Fatalf("duplicate path returned at iteration %d: %q", i, p)
		}
		seen[p] = true
	}
}

func TestNameTracker_NoExtension(t *testing.T) {
	// filepath.Ext("foo") == "" so base="foo", suffix candidate is "foo_2".
	nt := newNameTracker()
	nt.resolve("foo")
	got := nt.resolve("foo")
	if got != "foo_2" {
		t.Errorf("resolve() = %q, want %q", got, "foo_2")
	}
}

func TestNameTracker_MultipleDots(t *testing.T) {
	// filepath.Ext("my.table.sql") == ".sql", base = "my.table".
	nt := newNameTracker()
	nt.resolve("my.table.sql")
	got := nt.resolve("my.table.sql")
	if got != "my.table_2.sql" {
		t.Errorf("resolve() = %q, want %q", got, "my.table_2.sql")
	}
}

func TestNameTracker_DotfileStylePath(t *testing.T) {
	// filepath.Ext(".gitignore") returns ".gitignore" (the leading dot makes the
	// entire string the extension; base is "").  Second call gets "_2.gitignore".
	nt := newNameTracker()
	nt.resolve(".gitignore")
	got := nt.resolve(".gitignore")
	if got != "_2.gitignore" {
		t.Errorf("resolve() = %q, want %q", got, "_2.gitignore")
	}
}

func TestNameTracker_EmptyStringPath(t *testing.T) {
	// An empty string has no extension; second call gets "_2".
	nt := newNameTracker()
	nt.resolve("")
	got := nt.resolve("")
	if got != "_2" {
		t.Errorf("resolve() = %q, want %q", got, "_2")
	}
}

func TestNameTracker_100Collisions(t *testing.T) {
	// Resolve the same path 100 times; all results must be unique and the
	// last must be the _100 variant.
	nt := newNameTracker()
	seen := make(map[string]bool, 100)
	var last string
	for i := 0; i < 100; i++ {
		last = nt.resolve("obj.sql")
		if seen[last] {
			t.Fatalf("duplicate at iteration %d: %q", i, last)
		}
		seen[last] = true
	}
	if last != "obj_100.sql" {
		t.Errorf("100th result = %q, want %q", last, "obj_100.sql")
	}
}

func TestNameTracker_OnlyExtension(t *testing.T) {
	// filepath.Ext(".sql") returns ".sql" (leading dot, no other dot → whole
	// string is the extension, base is "").  Second call gets "_2.sql".
	nt := newNameTracker()
	nt.resolve(".sql")
	got := nt.resolve(".sql")
	if got != "_2.sql" {
		t.Errorf("resolve() = %q, want %q", got, "_2.sql")
	}
}

func TestNameTracker_UnicodeInPath(t *testing.T) {
	// Unicode characters are valid path components; tracker must handle them.
	nt := newNameTracker()
	p := filepath.Join("SCH", "tables", "données.sql")
	got1 := nt.resolve(p)
	got2 := nt.resolve(p)
	if got1 != p {
		t.Errorf("first = %q, want %q", got1, p)
	}
	want2 := filepath.Join("SCH", "tables", "données_2.sql")
	if got2 != want2 {
		t.Errorf("second = %q, want %q", got2, want2)
	}
}

func TestNameTracker_ConcurrentMixedPaths(t *testing.T) {
	// Concurrent access with two hot paths; each must return unique results.
	nt := newNameTracker()
	const goroutines = 30

	type result struct {
		path string
		val  string
	}
	ch := make(chan result, goroutines*2)
	for i := 0; i < goroutines; i++ {
		go func() { ch <- result{"a.sql", nt.resolve("a.sql")} }()
		go func() { ch <- result{"b.sql", nt.resolve("b.sql")} }()
	}

	seenA := make(map[string]bool)
	seenB := make(map[string]bool)
	for i := 0; i < goroutines*2; i++ {
		r := <-ch
		m := seenA
		if r.path == "b.sql" {
			m = seenB
		}
		if m[r.val] {
			t.Errorf("duplicate for %s: %q", r.path, r.val)
		}
		m[r.val] = true
	}
}

func TestNameTracker_ConcurrentSafety(t *testing.T) {
	// Many goroutines resolving the same path must all receive unique results.
	nt := newNameTracker()
	const goroutines = 50

	results := make([]string, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = nt.resolve("concurrent.sql")
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, goroutines)
	for _, r := range results {
		if seen[r] {
			t.Errorf("duplicate path in concurrent results: %q", r)
		}
		seen[r] = true
	}
}

// ─── Parse + Split integration ────────────────────────────────────────────────

// TestParseAfterSplit verifies that the full pipeline — splitting a realistic
// DDL blob produced by GET_DDL and then parsing each statement — yields the
// correct metadata for every object type.
func TestParseAfterSplit(t *testing.T) {
	ddl := `create or replace database "ACME";

create or replace schema "ACME"."SALES";

create or replace TABLE "ACME"."SALES"."ORDERS" (
	"ORDER_ID" NUMBER(38,0) NOT NULL,
	"CUSTOMER" VARCHAR(256)
);

create or replace view "ACME"."SALES"."RECENT_ORDERS"(
	"ORDER_ID",
	"CUSTOMER"
) as
select "ORDER_ID", "CUSTOMER" from "ACME"."SALES"."ORDERS" where 1=1;

create or replace function "ACME"."SALES"."DISCOUNT"(PRICE FLOAT, PCT FLOAT)
returns float
language javascript
as $$
    return PRICE * (1 - PCT / 100);
$$;

create or replace procedure "ACME"."SALES"."REFRESH"()
returns varchar
language sql
as $$
BEGIN
    RETURN 'ok';
END
$$;

create or replace sequence "ACME"."SALES"."ORDER_SEQ" start 1 increment 1;

create or replace stage "ACME"."SALES"."LOAD_STAGE" url='s3://acme/load/';

create or replace stream "ACME"."SALES"."ORDERS_STREAM" on table "ACME"."SALES"."ORDERS";

create or replace task "ACME"."SALES"."NIGHTLY_REFRESH"
    warehouse = COMPUTE_WH
    schedule = 'USING CRON 0 2 * * * UTC'
AS
CALL "ACME"."SALES"."REFRESH"();

create or replace file format "ACME"."SALES"."CSV_FORMAT"
    type = 'CSV'
    field_delimiter = ','
    skip_header = 1;

create or replace pipe "ACME"."SALES"."ORDERS_PIPE"
    as copy into "ACME"."SALES"."ORDERS" from @"ACME"."SALES"."LOAD_STAGE";`

	stmts := Split(ddl)

	type wantRow struct {
		kind   Kind
		db     string
		schema string
		name   string
	}

	want := []wantRow{
		{KindDatabase, "", "", "ACME"},
		{KindSchema, "", "ACME", "SALES"},
		{KindTable, "ACME", "SALES", "ORDERS"},
		{KindView, "ACME", "SALES", "RECENT_ORDERS"},
		{KindFunction, "ACME", "SALES", "DISCOUNT"},
		{KindProcedure, "ACME", "SALES", "REFRESH"},
		{KindSequence, "ACME", "SALES", "ORDER_SEQ"},
		{KindStage, "ACME", "SALES", "LOAD_STAGE"},
		{KindStream, "ACME", "SALES", "ORDERS_STREAM"},
		{KindTask, "ACME", "SALES", "NIGHTLY_REFRESH"},
		{KindFileFormat, "ACME", "SALES", "CSV_FORMAT"},
		{KindPipe, "ACME", "SALES", "ORDERS_PIPE"},
	}

	if len(stmts) != len(want) {
		t.Fatalf("Split produced %d statements, want %d\nstmts: %#v", len(stmts), len(want), stmts)
	}

	for i, stmt := range stmts {
		obj := Parse(stmt)
		w := want[i]

		if obj.Kind != w.kind {
			t.Errorf("[%d] Kind = %q, want %q (sql: %q)", i, obj.Kind, w.kind, stmt[:min(60, len(stmt))])
		}
		if obj.Database != w.db {
			t.Errorf("[%d] Database = %q, want %q", i, obj.Database, w.db)
		}
		if obj.Schema != w.schema {
			t.Errorf("[%d] Schema = %q, want %q", i, obj.Schema, w.schema)
		}
		if obj.Name != w.name {
			t.Errorf("[%d] Name = %q, want %q", i, obj.Name, w.name)
		}
	}

	// Spot-check the function overload argument signature.
	funcObj := Parse(stmts[4])
	if funcObj.ArgSig != "FLOAT_FLOAT" {
		t.Errorf("DISCOUNT ArgSig = %q, want %q", funcObj.ArgSig, "FLOAT_FLOAT")
	}

	// Spot-check the procedure no-args signature.
	procObj := Parse(stmts[5])
	if procObj.ArgSig != "noargs" {
		t.Errorf("REFRESH ArgSig = %q, want %q", procObj.ArgSig, "noargs")
	}
}
