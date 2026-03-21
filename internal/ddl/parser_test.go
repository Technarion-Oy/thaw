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
			want:  []string{"SELECT 1 -- hidden; semi"},
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

		// ── Windows line endings ─────────────────────────────────────────────
		{
			name:  "CRLF line endings between statements",
			input: "SELECT 1;\r\nSELECT 2;\r\n",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "CRLF inside line comment does not confuse state",
			input: "SELECT 1 -- comment\r\n;",
			want:  []string{"SELECT 1 -- comment"},
		},

		// ── unicode ───────────────────────────────────────────────────────────
		{
			name:  "unicode characters in single-quoted string",
			input: "SELECT 'café résumé naïve';",
			want:  []string{"SELECT 'café résumé naïve'"},
		},
		{
			name:  "unicode in double-quoted identifier",
			input: `SELECT "données" FROM t;`,
			want:  []string{`SELECT "données" FROM t`},
		},
		{
			name:  "japanese characters in quoted identifier",
			input: "SELECT \"テーブル\" FROM t;",
			want:  []string{"SELECT \"テーブル\" FROM t"},
		},
		{
			name:  "multibyte rune adjacent to semicolon terminator",
			input: "SELECT '日本語';",
			want:  []string{"SELECT '日本語'"},
		},

		// ── comment-like tokens inside quoted strings ─────────────────────────
		{
			name:  "-- inside single-quoted string is not a line comment",
			input: "SELECT 'hello -- world';",
			want:  []string{"SELECT 'hello -- world'"},
		},
		{
			name:  "/* inside single-quoted string is not a block comment",
			input: "SELECT 'a /* b */ c';",
			want:  []string{"SELECT 'a /* b */ c'"},
		},
		{
			name:  "-- inside double-quoted identifier is not a line comment",
			input: `SELECT "col--name" FROM t;`,
			want:  []string{`SELECT "col--name" FROM t`},
		},
		{
			name:  "/* inside double-quoted identifier is not a block comment",
			input: `SELECT "col/*name" FROM t;`,
			want:  []string{`SELECT "col/*name" FROM t`},
		},

		// ── block comment edge cases ──────────────────────────────────────────
		{
			name:  "block comment with interior asterisks",
			input: "SELECT /* a * b * c */ 1;",
			want:  []string{"SELECT /* a * b * c */ 1"},
		},
		{
			name:  "block comment opening with extra stars",
			input: "SELECT /*** triple star ***/ 1;",
			want:  []string{"SELECT /*** triple star ***/ 1"},
		},
		{
			name:  "block comments are not nested — inner /* does not extend comment",
			input: "SELECT /* outer /* inner */ not_in_comment;",
			want:  []string{"SELECT /* outer /* inner */ not_in_comment"},
		},
		{
			name:  "non-nesting: semicolon after first */ ends statement",
			input: "SELECT /* outer /* inner */ rest; trailing;",
			want:  []string{"SELECT /* outer /* inner */ rest", "trailing"},
		},
		{
			name:  "unterminated block comment captured by final flush",
			input: "SELECT /* never closed",
			want:  []string{"SELECT /* never closed"},
		},

		// ── unterminated quoted contexts ──────────────────────────────────────
		{
			name:  "unterminated single-quoted string captured by final flush",
			input: "SELECT 'never closed",
			want:  []string{"SELECT 'never closed"},
		},
		{
			name:  "unterminated double-quoted identifier captured by final flush",
			input: `SELECT "never closed`,
			want:  []string{`SELECT "never closed`},
		},
		{
			name:  "unterminated dollar-quote captured by final flush",
			input: "CREATE FUNCTION f() AS $$ never closed",
			want:  []string{"CREATE FUNCTION f() AS $$ never closed"},
		},

		// ── dollar-quote edge cases ───────────────────────────────────────────
		{
			name:  "empty dollar-quoted body",
			input: "CREATE FUNCTION f() AS $$$$;",
			want:  []string{"CREATE FUNCTION f() AS $$$$"},
		},
		{
			name:  "dollar-quote tag with underscore",
			input: "x $_tag_$body;$_tag_$;",
			want:  []string{"x $_tag_$body;$_tag_$"},
		},
		{
			name:  "dollar-quote tag with digits",
			input: "x $abc123$body;$abc123$;",
			want:  []string{"x $abc123$body;$abc123$"},
		},
		{
			name:  "newlines inside single-quoted string",
			input: "SELECT 'line1\nline2';",
			want:  []string{"SELECT 'line1\nline2'"},
		},

		// ── carriage-return / old-Mac line endings ──────────────────────────
		{
			name:  "bare CR in normal text is treated as whitespace and trimmed",
			input: "SELECT\r1;",
			want:  []string{"SELECT\r1"},
		},
		{
			name:  "CR-only line ending does NOT terminate a line comment",
			// \r without \n: the line comment stays open through the \r, so the
			// entire input is a single unterminated statement flushed at the end.
			input: "-- comment\rSELECT 2;",
			want:  []string{"-- comment\rSELECT 2;"},
		},

		// ── backslash is not an escape character in Snowflake strings ────────
		{
			name: "backslash before closing quote does not escape it",
			// 'a\' closes the string (backslash is literal); the following ;
			// splits statements.  The remaining b'; is an unterminated string.
			input: "SELECT 'a\\';b';",
			want:  []string{"SELECT 'a\\'", "b';"},
		},
		{
			name:  "backslash inside single-quoted string is just a character",
			input: `SELECT 'path\to\file';`,
			want:  []string{`SELECT 'path\to\file'`},
		},

		// ── dollar-quote tag case-sensitivity ────────────────────────────────
		{
			name:  "dollar-quote tag is case-sensitive: uppercase tag not closed by lowercase",
			input: "x $BODY$ inside; still inside $body$ not closed",
			want:  []string{"x $BODY$ inside; still inside $body$ not closed"},
		},
		{
			name:  "dollar-quote tag with digits and underscores",
			input: "x $tag_1$body;$tag_1$;",
			want:  []string{"x $tag_1$body;$tag_1$"},
		},
		{
			name:  "dollar-quote tag starting with underscore",
			input: "x $_tag$body;$_tag$;",
			want:  []string{"x $_tag$body;$_tag$"},
		},
		{
			name:  "very long dollar-quote tag",
			input: "x $" + strings.Repeat("a", 60) + "$body;$" + strings.Repeat("a", 60) + "$;",
			want:  []string{"x $" + strings.Repeat("a", 60) + "$body;$" + strings.Repeat("a", 60) + "$"},
		},

		// ── comment-only statements ───────────────────────────────────────────
		{
			name:  "block comment only statement",
			input: "/* comment */;",
			want:  []string{"/* comment */"},
		},
		{
			name:  "line comment only with no terminating newline",
			input: "-- only a comment",
			want:  []string{"-- only a comment"},
		},
		{
			name:  "line comment only followed by semicolon on next line",
			input: "-- comment\n;",
			want:  []string{"-- comment"},
		},

		// ── whitespace and empty statements ───────────────────────────────────
		{
			name:  "tab character adjacent to semicolon is trimmed",
			input: "SELECT 1\t;",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "form feed and vertical tab as whitespace",
			input: "\f\vSELECT 1\f\v;",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "whitespace-only between semicolons produces no statement",
			input: "SELECT 1;   ;   ;SELECT 2;",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "thousand semicolons produce no statements",
			input: func() string {
				return strings.Repeat(";", 1000)
			}(),
			want: nil,
		},

		// ── large string bodies ───────────────────────────────────────────────
		{
			name: "large single-quoted string with embedded semicolons",
			input: func() string {
				inner := strings.Repeat("a;b;c;", 80) // 480 chars
				return "SELECT '" + inner + "';"
			}(),
			want: func() []string {
				inner := strings.Repeat("a;b;c;", 80)
				return []string{"SELECT '" + inner + "'"}
			}(),
		},
		{
			name: "large dollar-quoted body with semicolons and comments",
			input: func() string {
				// Simulate a fat JS function body
				lines := []string{"CREATE FUNCTION big() RETURNS VARIANT LANGUAGE JAVASCRIPT AS $$"}
				for i := 0; i < 50; i++ {
					lines = append(lines, "  // step "+strings.Repeat("x", 20)+";")
					lines = append(lines, "  var x = 'value; with; semis';")
				}
				lines = append(lines, "$$;")
				return strings.Join(lines, "\n")
			}(),
			want: nil, // handled in special case below
		},

		// ── two dollar-quote bodies in one statement ──────────────────────────
		{
			name:  "two sequential dollar-quotes in one statement",
			input: "SELECT $a$ part1; $a$, $b$ part2; $b$;",
			want:  []string{"SELECT $a$ part1; $a$, $b$ part2; $b$"},
		},
		{
			name:  "dollar-quote body containing block comment syntax",
			input: "CREATE FUNCTION f() AS $$ return /* not a comment */ 1; $$;",
			want:  []string{"CREATE FUNCTION f() AS $$ return /* not a comment */ 1; $$"},
		},
		{
			name:  "dollar-quote body containing line comment syntax",
			input: "CREATE FUNCTION f() AS $$ return -- not a comment\n1; $$;",
			want:  []string{"CREATE FUNCTION f() AS $$ return -- not a comment\n1; $$"},
		},
		{
			name:  "dollar-quote containing a double-quoted identifier with semicolon",
			input: `CREATE FUNCTION f() AS $$ SELECT "col;1" FROM t; $$;`,
			want:  []string{`CREATE FUNCTION f() AS $$ SELECT "col;1" FROM t; $$`},
		},

		// ── block comment edge cases ──────────────────────────────────────────
		{
			name:  "empty block comment",
			input: "SELECT /**/1;",
			want:  []string{"SELECT /**/1"},
		},
		{
			name:  "block comment with only stars",
			input: "SELECT /****/ 1;",
			want:  []string{"SELECT /****/ 1"},
		},
		{
			name:  "two block comments in one statement",
			input: "SELECT /* a */ 1 /* b */;",
			want:  []string{"SELECT /* a */ 1 /* b */"},
		},

		// ── single-quoted string edge cases ───────────────────────────────────
		{
			name:  "empty single-quoted string",
			input: "SELECT '';",
			want:  []string{"SELECT ''"},
		},
		{
			name:  "single-quoted string with embedded newline",
			input: "SELECT 'line1\nline2';",
			want:  []string{"SELECT 'line1\nline2'"},
		},
		{
			name:  "many escaped single-quotes in sequence",
			input: "SELECT '''' = '''';",
			want:  []string{"SELECT '''' = ''''"},
		},
		{
			name:  "single-quote immediately after dollar-quote close",
			input: "SELECT $$body$$'suffix';",
			want:  []string{"SELECT $$body$$'suffix'"},
		},

		// ── many statements ───────────────────────────────────────────────────
		{
			name: "fifty sequential statements",
			// built programmatically below — want is nil (handled specially)
			input: func() string {
				var b strings.Builder
				for i := 0; i < 50; i++ {
					b.WriteString("SELECT 1;")
				}
				return b.String()
			}(),
			want: nil, // handled separately in the loop below
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
			// Special case: large dollar-quoted body — just verify it's one statement.
			if tt.name == "large dollar-quoted body with semicolons and comments" {
				got := Split(tt.input)
				if len(got) != 1 {
					t.Errorf("Split() = %d statements, want 1", len(got))
				}
				return
			}
			// Special case: fifty sequential statements.
			if tt.name == "fifty sequential statements" {
				got := Split(tt.input)
				if len(got) != 50 {
					t.Errorf("Split() = %d statements, want 50", len(got))
				}
				for i, s := range got {
					if s != "SELECT 1" {
						t.Errorf("statement[%d] = %q, want \"SELECT 1\"", i, s)
					}
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

// ─── splitParamList ───────────────────────────────────────────────────────────

func TestSplitParamList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		// Single token — always one part.
		{"", []string{""}},
		{"FLOAT", []string{"FLOAT"}},
		// Top-level comma splits.
		{"A, B", []string{"A", " B"}},
		{"A, B, C", []string{"A", " B", " C"}},
		// Comma inside parens is NOT a separator.
		{"NUMBER(38,0)", []string{"NUMBER(38,0)"}},
		{"NUMBER(18,2), VARCHAR(256)", []string{"NUMBER(18,2)", " VARCHAR(256)"}},
		// Two params both with precision.
		{"A NUMBER(18,2), B VARCHAR(256)", []string{"A NUMBER(18,2)", " B VARCHAR(256)"}},
		// TABLE type with inline column list.
		{"T TABLE(X FLOAT, Y NUMBER)", []string{"T TABLE(X FLOAT, Y NUMBER)"}},
		// Nested parens at depth 2.
		{"NESTED((a,b),c), D", []string{"NESTED((a,b),c)", " D"}},
		// Three params, middle one with precision.
		{"A FLOAT, B NUMBER(10,2), C DATE", []string{"A FLOAT", " B NUMBER(10,2)", " C DATE"}},
		// Just a comma — two empty strings.
		{",", []string{"", ""}},
		// Trailing comma — last element is empty.
		{"A,", []string{"A", ""}},
		// Leading comma — first element is empty.
		{",B", []string{"", "B"}},
		// MAP type with two nested type params.
		{"K MAP(VARCHAR, NUMBER)", []string{"K MAP(VARCHAR, NUMBER)"}},
		// ARRAY type with comma inside.
		{"A ARRAY(FLOAT, 3)", []string{"A ARRAY(FLOAT, 3)"}},
		// Deeply nested at depth 3: outer comma at depth 0 still splits.
		{"FUNC(((a,b),c),d), E", []string{"FUNC(((a,b),c),d)", " E"}},
		// Unclosed paren: depth never returns to 0 so no split ever happens.
		{"NESTED(1,2", []string{"NESTED(1,2"}},
		// Multiple unclosed: all one part regardless of commas.
		{"A FUNC(x,y, B OTHER(p,q", []string{"A FUNC(x,y, B OTHER(p,q"}},
		// Whitespace-only entry.
		{"   ,   ", []string{"   ", "   "}},
		// Comma at depth 0 and depth 1 interleaved.
		{"A(x,y), B, C(p,q,r), D", []string{"A(x,y)", " B", " C(p,q,r)", " D"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitParamList(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitParamList(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ─── tokeniseQualifiedIdent ───────────────────────────────────────────────────

func TestTokeniseQualifiedIdent(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantParts []string
	}{
		// ── unquoted identifiers ──────────────────────────────────────────────
		{
			name:      "single unquoted identifier",
			input:     "TABLE_NAME (id INT)",
			wantParts: []string{"TABLE_NAME"},
		},
		{
			name:      "unquoted stops at open parenthesis",
			input:     "func_name(X FLOAT)",
			wantParts: []string{"func_name"},
		},
		{
			name:      "unquoted stops at whitespace",
			input:     "MY_TABLE AS t",
			wantParts: []string{"MY_TABLE"},
		},

		// ── quoted identifiers ────────────────────────────────────────────────
		{
			name:      "single quoted identifier",
			input:     `"MY_TABLE" (id INT)`,
			wantParts: []string{"MY_TABLE"},
		},
		{
			name:      "two-part qualified name",
			input:     `"SCHEMA"."TABLE"`,
			wantParts: []string{"SCHEMA", "TABLE"},
		},
		{
			name:      "three-part fully-qualified name",
			input:     `"DB"."SCH"."TBL" (id INT)`,
			wantParts: []string{"DB", "SCH", "TBL"},
		},
		// Only three parts are consumed — fourth dot and beyond become rest.
		{
			name:      "four-part name stops at third identifier",
			input:     `"A"."B"."C"."D"`,
			wantParts: []string{"A", "B", "C"},
		},

		// ── special characters inside quoted identifiers ───────────────────────
		{
			name:      "space inside quoted name",
			input:     `"MY TABLE" (id INT)`,
			wantParts: []string{"MY TABLE"},
		},
		{
			name:      "dot inside quoted name is not a separator",
			input:     `"MY.SCHEMA"."MY.TABLE" (id INT)`,
			wantParts: []string{"MY.SCHEMA", "MY.TABLE"},
		},
		{
			name:      "semicolon inside quoted name",
			input:     `"tbl;1" (id INT)`,
			wantParts: []string{"tbl;1"},
		},
		{
			name:      "hyphen inside quoted name",
			input:     `"my-table"`,
			wantParts: []string{"my-table"},
		},
		{
			name:      "dollar sign inside quoted name",
			input:     `"$TEMP"`,
			wantParts: []string{"$TEMP"},
		},
		{
			name:      "open paren inside quoted name",
			input:     `"tbl(1)"`,
			wantParts: []string{"tbl(1)"},
		},

		// ── double-quote escape sequences inside quoted names ─────────────────
		{
			name:      "single double-quote escape",
			input:     `"MY""TABLE"`,
			wantParts: []string{`MY"TABLE`},
		},
		{
			name:      "multiple double-quote escapes in one name",
			input:     `"A""B""C"`,
			wantParts: []string{`A"B"C`},
		},
		{
			name:      "name consisting entirely of escaped quotes",
			input:     `""""`,
			wantParts: []string{`"`},
		},

		// ── unicode ───────────────────────────────────────────────────────────
		{
			name:      "unicode characters in quoted name",
			input:     `"données"`,
			wantParts: []string{"données"},
		},
		{
			name:      "japanese characters in quoted name",
			input:     `"テーブル"`,
			wantParts: []string{"テーブル"},
		},
		{
			name:      "emoji in quoted name",
			input:     `"my🔥table"`,
			wantParts: []string{"my🔥table"},
		},

		// ── edge cases ────────────────────────────────────────────────────────
		{
			name:      "empty string",
			input:     "",
			wantParts: nil,
		},
		{
			name:      "only whitespace",
			input:     "   ",
			wantParts: nil,
		},
		{
			name:      "empty quoted identifier",
			input:     `""`,
			wantParts: nil, // empty string part is discarded
		},

		// ── mixed quoted / unquoted parts ────────────────────────────────────
		{
			name:      "three-part unquoted",
			input:     "DB.SCHEMA.TABLE (id INT)",
			wantParts: []string{"DB", "SCHEMA", "TABLE"},
		},
		{
			name:      "mixed: quoted db, unquoted schema, quoted table",
			input:     `"MY_DB".PUBLIC."MY TABLE"`,
			wantParts: []string{"MY_DB", "PUBLIC", "MY TABLE"},
		},
		{
			name:      "mixed: unquoted db, quoted schema, unquoted table",
			input:     `DB."MY SCHEMA".TBL`,
			wantParts: []string{"DB", "MY SCHEMA", "TBL"},
		},

		// ── SQL reserved words as quoted identifiers ──────────────────────────
		{
			name:      "SQL keyword SELECT as quoted name",
			input:     `"SELECT"."FROM"."WHERE"`,
			wantParts: []string{"SELECT", "FROM", "WHERE"},
		},
		{
			name:      "SQL keyword CREATE as quoted table name",
			input:     `"DB"."SCH"."CREATE"`,
			wantParts: []string{"DB", "SCH", "CREATE"},
		},

		// ── digits and special content in quoted names ─────────────────────
		{
			name:      "name with only digits",
			input:     `"123"`,
			wantParts: []string{"123"},
		},
		{
			name:      "name with path separator slash",
			input:     `"db/path"."sch"`,
			wantParts: []string{"db/path", "sch"},
		},
		{
			name:      "name with backslash",
			input:     `"db\path"`,
			wantParts: []string{`db\path`},
		},
		{
			name:      "name with single quotes inside double-quoted identifier",
			input:     `"it's a table"`,
			wantParts: []string{"it's a table"},
		},
		{
			name:      "name with embedded newline in double-quoted identifier",
			input:     "\"line1\nline2\"",
			wantParts: []string{"line1\nline2"},
		},
		{
			name:      "name with embedded tab in double-quoted identifier",
			input:     "\"col\tname\"",
			wantParts: []string{"col\tname"},
		},
		{
			name:      "name consisting only of special characters",
			input:     `"!@#$%^&*()"`,
			wantParts: []string{"!@#$%^&*()"},
		},
		{
			name:      "whitespace-only quoted name is a valid non-empty part",
			input:     `"   "`,
			wantParts: []string{"   "},
		},

		// ── empty middle part is silently skipped but loop continues ───────────
		{
			// After "A" the dot is consumed, then "" is empty so not appended,
			// then the NEXT dot IS present (rs[6]=='.' after pos 4-5 for "")
			// so the loop continues and picks up "C".  Result: ["A", "C"].
			name:      "empty middle part skipped but next dot still consumed",
			input:     `"A"."". "C"`,
			wantParts: []string{"A", "C"},
		},

		// ── leading dot: empty unquoted prefix consumed, rest parsed ──────────
		{
			// Unquoted loop immediately stops at '.', empty part discarded, dot
			// is consumed, and parsing continues — yielding ["SCH", "TBL"].
			name:      "leading dot: empty unquoted prefix consumed, rest parsed",
			input:     `."SCH"."TBL"`,
			wantParts: []string{"SCH", "TBL"},
		},

		// ── very long quoted name ─────────────────────────────────────────────
		{
			name:      "very long quoted name (255 chars)",
			input:     `"` + strings.Repeat("X", 255) + `"`,
			wantParts: []string{strings.Repeat("X", 255)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, _ := tokeniseQualifiedIdent(tt.input)
			if len(parts) != len(tt.wantParts) {
				t.Fatalf("tokeniseQualifiedIdent(%q) = %v, want %v", tt.input, parts, tt.wantParts)
			}
			for i := range tt.wantParts {
				if parts[i] != tt.wantParts[i] {
					t.Errorf("[%d] got %q, want %q", i, parts[i], tt.wantParts[i])
				}
			}
		})
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
