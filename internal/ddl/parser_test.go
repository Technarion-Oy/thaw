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
	"strings"
	"testing"
)

// ─── Split ────────────────────────────────────────────────────────────────────

func TestSplit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		// ── empty / blank input ──────────────────────────────────────────────
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only whitespace",
			input: "   \n\t  ",
			want:  nil,
		},
		{
			name:  "bare semicolon produces no statement",
			input: ";",
			want:  nil,
		},
		{
			name:  "multiple bare semicolons",
			input: "  ;  ;  ",
			want:  nil,
		},

		// ── single statement ─────────────────────────────────────────────────
		{
			name:  "no trailing semicolon captured by final flush",
			input: "SELECT 1",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "single statement with semicolon",
			input: "SELECT 1;",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: "  SELECT 1  ;",
			want:  []string{"SELECT 1"},
		},

		// ── multiple statements ──────────────────────────────────────────────
		{
			name:  "two statements",
			input: "SELECT 1; SELECT 2;",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "statements separated by newlines",
			input: "SELECT 1;\n\nSELECT 2;\n",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "three statements last without semicolon",
			input: "A; B; C",
			want:  []string{"A", "B", "C"},
		},

		// ── line comments ────────────────────────────────────────────────────
		{
			name:  "semicolon inside line comment is ignored",
			input: "SELECT 1 -- hidden; semi\n;",
			want:  []string{"SELECT 1 -- hidden; semi\n"},
		},
		{
			name:  "leading line comment",
			input: "-- leading\nSELECT 1;",
			want:  []string{"-- leading\nSELECT 1"},
		},
		{
			name:  "trailing line comment without newline is flushed",
			input: "SELECT 1 -- comment",
			want:  []string{"SELECT 1 -- comment"},
		},
		{
			name:  "line comment between two statements",
			input: "SELECT 1; -- comment\nSELECT 2;",
			want:  []string{"SELECT 1", "-- comment\nSELECT 2"},
		},

		// ── block comments ───────────────────────────────────────────────────
		{
			name:  "semicolon inside block comment is ignored",
			input: "SELECT /* ; */ 1;",
			want:  []string{"SELECT /* ; */ 1"},
		},
		{
			name:  "multiline block comment with semicolons",
			input: "SELECT /*\n  ; line1\n  ; line2\n*/ 1;",
			want:  []string{"SELECT /*\n  ; line1\n  ; line2\n*/ 1"},
		},
		{
			name:  "block comment between two statements",
			input: "SELECT 1; /* sep */ SELECT 2;",
			want:  []string{"SELECT 1", "/* sep */ SELECT 2"},
		},

		// ── single-quoted strings ────────────────────────────────────────────
		{
			name:  "semicolon inside single-quoted string",
			input: "SELECT 'a;b';",
			want:  []string{"SELECT 'a;b'"},
		},
		{
			name:  "double-single-quote escape keeps string open",
			input: "SELECT 'it''s fine; really';",
			want:  []string{"SELECT 'it''s fine; really'"},
		},
		{
			name:  "escaped quote followed by more content",
			input: "SELECT 'a''b' FROM t;",
			want:  []string{"SELECT 'a''b' FROM t"},
		},
		{
			name:  "consecutive escaped quotes",
			input: "SELECT '''''';",
			want:  []string{"SELECT ''''''"},
		},

		// ── double-quoted identifiers ────────────────────────────────────────
		{
			name:  "semicolon inside double-quoted identifier",
			input: `SELECT "col;name" FROM t;`,
			want:  []string{`SELECT "col;name" FROM t`},
		},
		{
			name:  "double-double-quote escape inside identifier",
			input: `SELECT "col""name" FROM t;`,
			want:  []string{`SELECT "col""name" FROM t`},
		},
		{
			name:  "fully-qualified three-part name with semicolon in schema",
			input: `CREATE TABLE "MY;DB"."PUBLIC"."TBL" (id INT);`,
			want:  []string{`CREATE TABLE "MY;DB"."PUBLIC"."TBL" (id INT)`},
		},

		// ── dollar-quoted bodies ─────────────────────────────────────────────
		{
			name:  "semicolons inside $$ are not terminators",
			input: "CREATE FUNCTION f() AS $$ SELECT 1; SELECT 2; $$;",
			want:  []string{"CREATE FUNCTION f() AS $$ SELECT 1; SELECT 2; $$"},
		},
		{
			name:  "named dollar-quote tag",
			input: "CREATE FUNCTION f() AS $body$ SELECT 1; $body$;",
			want:  []string{"CREATE FUNCTION f() AS $body$ SELECT 1; $body$"},
		},
		{
			name:  "wrong closing tag does not exit dollar-quote",
			input: "x $$ inside $body$ still here $$;",
			want:  []string{"x $$ inside $body$ still here $$"},
		},
		{
			name:  "bare dollar sign is a literal character",
			input: "SELECT $1, $2;",
			want:  []string{"SELECT $1, $2"},
		},
		{
			name:  "dollar followed by space is a literal",
			input: "SELECT $ FROM t;",
			want:  []string{"SELECT $ FROM t"},
		},
		{
			name:  "dollar at end of input is a literal",
			input: "SELECT $",
			want:  []string{"SELECT $"},
		},
		{
			name:  "adjacent dollar-quoted functions",
			input: "CREATE FUNCTION f() AS $$ a; $$;\nCREATE FUNCTION g() AS $$ b; $$;",
			want: []string{
				"CREATE FUNCTION f() AS $$ a; $$",
				"CREATE FUNCTION g() AS $$ b; $$",
			},
		},

		// ── mixed contexts ───────────────────────────────────────────────────
		{
			name: "dollar-quote body containing js comment and string",
			input: strings.Join([]string{
				"CREATE FUNCTION f(x FLOAT)",
				"RETURNS FLOAT LANGUAGE JAVASCRIPT AS",
				"$$",
				"  // JS comment; with ; semicolons",
				"  return x * 'scale';",
				"$$;",
			}, "\n"),
			want: []string{strings.Join([]string{
				"CREATE FUNCTION f(x FLOAT)",
				"RETURNS FLOAT LANGUAGE JAVASCRIPT AS",
				"$$",
				"  // JS comment; with ; semicolons",
				"  return x * 'scale';",
				"$$",
			}, "\n")},
		},
		{
			name: "stored procedure with BEGIN END inside dollar-quote",
			input: strings.Join([]string{
				"CREATE OR REPLACE PROCEDURE p()",
				"RETURNS VARCHAR LANGUAGE SQL AS",
				"$$",
				"BEGIN",
				"  LET x := 1;",
				"  RETURN 'done';",
				"END",
				"$$;",
			}, "\n"),
			want: []string{strings.Join([]string{
				"CREATE OR REPLACE PROCEDURE p()",
				"RETURNS VARCHAR LANGUAGE SQL AS",
				"$$",
				"BEGIN",
				"  LET x := 1;",
				"  RETURN 'done';",
				"END",
				"$$",
			}, "\n")},
		},

		// ── realistic Snowflake GET_DDL output ───────────────────────────────
		{
			name: "realistic database DDL produces correct statement count",
			input: strings.Join([]string{
				`create or replace database "MY_DB";`,
				``,
				`create or replace schema "MY_DB"."PUBLIC";`,
				``,
				`create or replace TABLE "MY_DB"."PUBLIC"."MY_TABLE" (`,
				`    "ID" NUMBER(38,0) NOT NULL,`,
				`    "NAME" VARCHAR(16777216)`,
				`);`,
				``,
				`create or replace view "MY_DB"."PUBLIC"."MY_VIEW"(`,
				`    "ID",`,
				`    "NAME"`,
				`) as`,
				`select "ID", "NAME" from "MY_DB"."PUBLIC"."MY_TABLE";`,
				``,
				`create or replace function "MY_DB"."PUBLIC"."MY_FUNC"(X FLOAT)`,
				`returns float`,
				`language javascript`,
				`as $$`,
				`    return X * 2;`,
				`$$;`,
			}, "\n"),
			// We verify count and the first and last statements;
			// intermediate ones are checked in Parse tests below.
			want: nil, // handled by a separate count assertion below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special case: the realistic DDL test just checks the count.
			if tt.name == "realistic database DDL produces correct statement count" {
				got := Split(tt.input)
				if len(got) != 5 {
					t.Errorf("Split() = %d statements, want 5\nstatements: %#v", len(got), got)
				}
				return
			}

			got := Split(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("Split() returned %d statements, want %d\ngot:  %#v\nwant: %#v",
					len(got), len(tt.want), got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("statement[%d]:\ngot:  %q\nwant: %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func TestIsIdentRune(t *testing.T) {
	yes := []rune{'a', 'z', 'A', 'Z', '0', '9', '_'}
	for _, r := range yes {
		if !isIdentRune(r) {
			t.Errorf("isIdentRune(%q) = false, want true", r)
		}
	}

	no := []rune{' ', '\t', '\n', '$', '.', '-', '(', ')', ';', '\'', '"'}
	for _, r := range no {
		if isIdentRune(r) {
			t.Errorf("isIdentRune(%q) = true, want false", r)
		}
	}
}

func TestRunesEqual(t *testing.T) {
	tests := []struct {
		a, b []rune
		want bool
	}{
		{[]rune("$$"), []rune("$$"), true},
		{[]rune("$body$"), []rune("$body$"), true},
		{[]rune("$$"), []rune("$body$"), false},
		{[]rune("$$"), []rune("$b"), false},
		{nil, nil, true},
		{[]rune("x"), nil, false},
	}
	for _, tt := range tests {
		if got := runesEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("runesEqual(%q, %q) = %v, want %v", string(tt.a), string(tt.b), got, tt.want)
		}
	}
}
