// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowflake

import (
	"fmt"
	"strings"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		// ── Basic splitting ─────────────────────────────────────────────
		{
			name: "empty string",
			sql:  "",
			want: nil,
		},
		{
			name: "whitespace only",
			sql:  "   \t\n  ",
			want: nil,
		},
		{
			name: "single statement no semicolon",
			sql:  "SELECT 1",
			want: []string{"SELECT 1"},
		},
		{
			name: "single statement with trailing semicolon",
			sql:  "SELECT 1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "single statement with leading/trailing whitespace",
			sql:  "  \t SELECT 1 \n ",
			want: []string{"SELECT 1"},
		},
		{
			name: "two statements",
			sql:  "SELECT 1; SELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "three statements with trailing semicolon",
			sql:  "SELECT 1; SELECT 2; SELECT 3;",
			want: []string{"SELECT 1", "SELECT 2", "SELECT 3"},
		},
		{
			name: "multiple consecutive semicolons produce no empty statements",
			sql:  "SELECT 1;;; SELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "only semicolons",
			sql:  ";;;",
			want: nil,
		},
		{
			name: "whitespace between semicolons",
			sql:  "SELECT 1;  ;  ; SELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "multiline statement",
			sql:  "SELECT\n  1\nFROM\n  dual",
			want: []string{"SELECT\n  1\nFROM\n  dual"},
		},

		// ── Single-quoted strings ───────────────────────────────────────
		{
			name: "semicolon inside single-quoted string",
			sql:  "SELECT 'a;b'",
			want: []string{"SELECT 'a;b'"},
		},
		{
			name: "escaped single quote with semicolon",
			sql:  "SELECT 'it''s;here'",
			want: []string{"SELECT 'it''s;here'"},
		},
		{
			name: "multiple escaped quotes in string",
			sql:  "SELECT '''';",
			want: []string{"SELECT ''''"},
		},
		{
			name: "empty single-quoted string",
			sql:  "SELECT '';",
			want: []string{"SELECT ''"},
		},
		{
			name: "adjacent single-quoted strings",
			sql:  "SELECT 'a' || 'b;c'; SELECT 1",
			want: []string{"SELECT 'a' || 'b;c'", "SELECT 1"},
		},
		{
			name: "single-quoted string with double quotes inside",
			sql:  `SELECT '"hello";world'`,
			want: []string{`SELECT '"hello";world'`},
		},
		{
			name: "single-quoted string with line comment marker inside",
			sql:  "SELECT '-- not a comment; still string'",
			want: []string{"SELECT '-- not a comment; still string'"},
		},
		{
			name: "single-quoted string with block comment markers inside",
			sql:  "SELECT '/* not a comment; */'",
			want: []string{"SELECT '/* not a comment; */'"},
		},
		{
			name: "single-quoted string at end without closing quote (unterminated)",
			sql:  "SELECT 'unterminated",
			want: []string{"SELECT 'unterminated"},
		},
		{
			name: "unterminated string with semicolons after opening quote",
			sql:  "SELECT 'a;b;c",
			want: []string{"SELECT 'a;b;c"},
		},

		// ── Double-quoted identifiers ───────────────────────────────────
		{
			name: "semicolon inside double-quoted identifier",
			sql:  `SELECT "col;name" FROM t`,
			want: []string{`SELECT "col;name" FROM t`},
		},
		{
			name: "double-quoted identifier with single quote inside",
			sql:  `SELECT "it's" FROM t; SELECT 1`,
			want: []string{`SELECT "it's" FROM t`, "SELECT 1"},
		},
		{
			name: "empty double-quoted identifier",
			sql:  `SELECT "" FROM t`,
			want: []string{`SELECT "" FROM t`},
		},
		{
			name: "unterminated double-quoted identifier with semicolons",
			sql:  `SELECT "unterminated;ident`,
			want: []string{`SELECT "unterminated;ident`},
		},

		// ── Line comments (--) ──────────────────────────────────────────
		{
			name: "semicolon inside line comment",
			sql:  "SELECT 1 -- comment; not a split",
			want: []string{"SELECT 1 -- comment; not a split"},
		},
		{
			name: "line comment before statement",
			sql:  "-- comment\nSELECT 1",
			want: []string{"-- comment\nSELECT 1"},
		},
		{
			name: "line comment between statements",
			sql:  "SELECT 1; -- comment\nSELECT 2",
			want: []string{"SELECT 1", "-- comment\nSELECT 2"},
		},
		{
			name: "multiple line comments",
			sql:  "-- first\n-- second\nSELECT 1",
			want: []string{"-- first\n-- second\nSELECT 1"},
		},
		{
			name: "line comment at end consumes trailing semicolon-like chars",
			sql:  "SELECT 1; -- comment with ; inside",
			want: []string{"SELECT 1", "-- comment with ; inside"},
		},
		{
			name: "only a line comment",
			sql:  "-- just a comment",
			want: []string{"-- just a comment"},
		},

		// ── Block comments (/* */) ──────────────────────────────────────
		{
			name: "semicolon inside block comment",
			sql:  "SELECT /* ; */ 1",
			want: []string{"SELECT /* ; */ 1"},
		},
		{
			name: "block comment spanning multiple lines with semicolons",
			sql:  "SELECT /* line1;\nline2;\n*/ 1",
			want: []string{"SELECT /* line1;\nline2;\n*/ 1"},
		},
		{
			name: "block comment between statements",
			sql:  "SELECT 1; /* comment */ SELECT 2",
			want: []string{"SELECT 1", "/* comment */ SELECT 2"},
		},
		{
			name: "unterminated block comment swallows everything",
			sql:  "SELECT 1; SELECT /* never closed",
			want: []string{"SELECT 1", "SELECT /* never closed"},
		},
		{
			name: "block comment containing single quotes",
			sql:  "SELECT /* 'not a string; ' */ 1",
			want: []string{"SELECT /* 'not a string; ' */ 1"},
		},
		{
			name: "block comment containing double quotes",
			sql:  `SELECT /* "not;ident" */ 1`,
			want: []string{`SELECT /* "not;ident" */ 1`},
		},
		{
			name: "nested block comment markers stop at first close",
			sql:  "SELECT /* outer /* inner */ still_comment */ 1",
			// Parser finds first */, so "still_comment */ 1" is SQL, not comment.
			want: []string{"SELECT /* outer /* inner */ still_comment */ 1"},
		},
		{
			name: "only a block comment",
			sql:  "/* comment only */",
			want: []string{"/* comment only */"},
		},

		// ── Dollar-quoted strings ───────────────────────────────────────
		{
			name: "semicolon inside $$ dollar-quoted string",
			sql:  "SELECT $$foo;bar$$",
			want: []string{"SELECT $$foo;bar$$"},
		},
		{
			name: "semicolon inside tagged dollar-quoted string",
			sql:  "SELECT $tag$foo;bar$tag$",
			want: []string{"SELECT $tag$foo;bar$tag$"},
		},
		{
			name: "dollar-quote with underscore tag",
			sql:  "SELECT $my_tag$content;here$my_tag$",
			want: []string{"SELECT $my_tag$content;here$my_tag$"},
		},
		{
			name: "dollar-quote with numeric tag",
			sql:  "SELECT $123$foo;bar$123$",
			want: []string{"SELECT $123$foo;bar$123$"},
		},
		{
			name: "nested $$ inside tagged dollar-quote",
			sql:  "SELECT $tag$content$$semicol;here$tag$",
			want: []string{"SELECT $tag$content$$semicol;here$tag$"},
		},
		{
			name: "single quotes inside dollar-quoted string",
			sql:  "SELECT $$it's a 'test';$$",
			want: []string{"SELECT $$it's a 'test';$$"},
		},
		{
			name: "double quotes inside dollar-quoted string",
			sql:  `SELECT $$"ident;name"$$`,
			want: []string{`SELECT $$"ident;name"$$`},
		},
		{
			name: "line comment marker inside dollar-quoted string",
			sql:  "SELECT $$-- not a comment;$$",
			want: []string{"SELECT $$-- not a comment;$$"},
		},
		{
			name: "block comment markers inside dollar-quoted string",
			sql:  "SELECT $$/* not; a comment */$$",
			want: []string{"SELECT $$/* not; a comment */$$"},
		},
		{
			name: "unterminated dollar-quoted string",
			sql:  "SELECT $$never closed; oops",
			want: []string{"SELECT $$never closed; oops"},
		},
		{
			name: "unterminated tagged dollar-quoted string",
			sql:  "SELECT $tag$never closed; oops",
			want: []string{"SELECT $tag$never closed; oops"},
		},
		{
			name: "dollar sign not starting a dollar-quote (no closing $)",
			sql:  "SELECT $5; SELECT 1",
			want: []string{"SELECT $5", "SELECT 1"},
		},
		{
			name: "dollar sign alone",
			sql:  "SELECT $ FROM t",
			want: []string{"SELECT $ FROM t"},
		},
		{
			name: "dollar-quote with empty body",
			sql:  "SELECT $$$$; SELECT 1",
			want: []string{"SELECT $$$$", "SELECT 1"},
		},
		{
			name: "dollar-quote in CREATE FUNCTION body",
			sql:  "CREATE FUNCTION f() RETURNS INT LANGUAGE SQL AS $$ SELECT 1; $$; SELECT 2",
			want: []string{"CREATE FUNCTION f() RETURNS INT LANGUAGE SQL AS $$ SELECT 1; $$", "SELECT 2"},
		},
		{
			name: "dollar-quote with mismatched tag does not close",
			sql:  "SELECT $a$content$b$more;here$a$",
			// $a$ opens, $b$ inside does not close it, continues until $a$
			want: []string{"SELECT $a$content$b$more;here$a$"},
		},

		// ── Mixed quoting contexts ──────────────────────────────────────
		{
			name: "single-quoted string followed by double-quoted identifier with semicolons",
			sql:  `SELECT 'val;1', "col;2" FROM t; SELECT 1`,
			want: []string{`SELECT 'val;1', "col;2" FROM t`, "SELECT 1"},
		},
		{
			name: "dollar-quote then single-quote",
			sql:  "SELECT $$a;b$$, 'c;d'; SELECT 1",
			want: []string{"SELECT $$a;b$$, 'c;d'", "SELECT 1"},
		},
		{
			name: "comment inside string is not a comment",
			sql:  "INSERT INTO t VALUES ('-- ;not comment'); SELECT 1",
			want: []string{"INSERT INTO t VALUES ('-- ;not comment')", "SELECT 1"},
		},
		{
			name: "string inside comment is not a string",
			sql:  "SELECT /* 'string;' */ 1; SELECT 2",
			want: []string{"SELECT /* 'string;' */ 1", "SELECT 2"},
		},

		// ── Security / injection edge cases ─────────────────────────────
		{
			name: "classic SQL injection attempt: quote break with semicolon",
			sql:  "SELECT * FROM users WHERE name = ''; DROP TABLE users; --'",
			// The '' is an escaped empty string inside quotes, so the string ends
			// after ''. Then ; is a real split point.
			want: []string{
				"SELECT * FROM users WHERE name = ''",
				"DROP TABLE users",
				"--'",
			},
		},
		{
			name: "backslash is NOT an escape in Snowflake strings",
			sql:  `SELECT '\'; DROP TABLE users; --'`,
			// In Snowflake, backslash has no special meaning.
			// '\' is: open quote, backslash (literal), close quote.
			// Then ; splits, DROP TABLE users is separate.
			want: []string{
				`SELECT '\'`,
				"DROP TABLE users",
				"--'",
			},
		},
		{
			name: "properly escaped string with semicolons is one statement",
			sql:  "SELECT 'O''Brien;test' FROM t",
			want: []string{"SELECT 'O''Brien;test' FROM t"},
		},
		{
			name: "double-quoted identifier injection attempt",
			sql:  `SELECT ""; DROP TABLE users"`,
			// "" is a valid empty identifier, then ; splits
			want: []string{`SELECT ""`, `DROP TABLE users"`},
		},
		{
			name: "dollar-quote injection: wrong tag does not close",
			sql:  "SELECT $a$;DROP TABLE users;$b$;$a$",
			// $a$ opens, the $b$ inside does not close it, semicolons are protected
			want: []string{"SELECT $a$;DROP TABLE users;$b$;$a$"},
		},
		{
			name: "block comment used to hide statement boundary",
			sql:  "SELECT 1 /*;*/ ; SELECT 2",
			// The ; inside the comment is hidden, but the ; after */ is real
			want: []string{"SELECT 1 /*;*/", "SELECT 2"},
		},
		{
			name: "line comment hides trailing semicolons",
			sql:  "SELECT 1 -- ; hidden\n; SELECT 2",
			want: []string{"SELECT 1 -- ; hidden", "SELECT 2"},
		},
		{
			name: "deeply nested quoting attempt",
			sql:  `SELECT ''''; DROP TABLE t`,
			// '''' is: open-quote, escaped-quote(''), close-quote → string is "'"
			// Then ; is a split
			want: []string{"SELECT ''''", "DROP TABLE t"},
		},
		{
			name: "triple single quotes then semicolon",
			sql:  "SELECT '''; SELECT 1",
			// '' is escaped quote (stays in string), then ; is still in string
			// because the string hasn't closed. ' at pos 9 opened, '' at 10-11
			// is escape, then ";" is in string, etc. Actually trace:
			// Open at '. i advances. '' is escape, stays in string.
			// Then rest is consumed as part of the unterminated string.
			want: []string{"SELECT '''; SELECT 1"},
		},

		// ── Whitespace handling ─────────────────────────────────────────
		{
			name: "tabs between statements",
			sql:  "SELECT 1;\tSELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "carriage return and newline",
			sql:  "SELECT 1;\r\nSELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "statement is only whitespace after trimming",
			sql:  "SELECT 1;   ;SELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},

		// ── Realistic Snowflake SQL ─────────────────────────────────────
		{
			name: "CREATE PROCEDURE with dollar-quoted body",
			sql: `CREATE OR REPLACE PROCEDURE my_proc()
RETURNS STRING
LANGUAGE SQL
AS
$$
  BEGIN
    INSERT INTO t VALUES (1);
    RETURN 'done';
  END;
$$;
CALL my_proc();`,
			want: []string{
				"CREATE OR REPLACE PROCEDURE my_proc()\nRETURNS STRING\nLANGUAGE SQL\nAS\n$$\n  BEGIN\n    INSERT INTO t VALUES (1);\n    RETURN 'done';\n  END;\n$$",
				"CALL my_proc()",
			},
		},
		{
			name: "INSERT with quoted values containing semicolons",
			sql:  "INSERT INTO t (a, b) VALUES ('x;y', 'z'); COMMIT",
			want: []string{
				"INSERT INTO t (a, b) VALUES ('x;y', 'z')",
				"COMMIT",
			},
		},
		{
			name: "COPY INTO with file path",
			sql:  "COPY INTO @stage/path FROM t; SELECT 1",
			want: []string{
				"COPY INTO @stage/path FROM t",
				"SELECT 1",
			},
		},
		{
			name: "USE then SELECT",
			sql:  "USE DATABASE mydb; USE SCHEMA public; SELECT * FROM t",
			want: []string{
				"USE DATABASE mydb",
				"USE SCHEMA public",
				"SELECT * FROM t",
			},
		},
		{
			name: "JavaScript UDF with dollar-quoted body",
			sql: `CREATE FUNCTION js_func(x FLOAT)
RETURNS FLOAT
LANGUAGE JAVASCRIPT
AS
$$
  if (x > 0) { return x; }
  return -x;
$$;`,
			want: []string{
				"CREATE FUNCTION js_func(x FLOAT)\nRETURNS FLOAT\nLANGUAGE JAVASCRIPT\nAS\n$$\n  if (x > 0) { return x; }\n  return -x;\n$$",
			},
		},
		{
			name: "comment-only lines between statements",
			sql:  "SELECT 1;\n-- transition\nSELECT 2;",
			want: []string{"SELECT 1", "-- transition\nSELECT 2"},
		},

		// ── PUT/GET normalization (called via normalizePutGet) ───────────
		{
			name: "PUT with newlines gets collapsed",
			sql:  "PUT file:///tmp/data.csv\n@my_stage; SELECT 1",
			want: []string{
				"PUT 'file:///tmp/data.csv' @my_stage",
				"SELECT 1",
			},
		},
		{
			name: "GET with newlines gets collapsed",
			sql:  "GET @my_stage\nfile:///tmp/out; SELECT 1",
			want: []string{
				"GET @my_stage file:///tmp/out",
				"SELECT 1",
			},
		},

		// ── Edge cases for the $ character ──────────────────────────────
		{
			name: "lone dollar sign at end of input",
			sql:  "SELECT $",
			want: []string{"SELECT $"},
		},
		{
			name: "dollar in arithmetic expression",
			sql:  "SELECT $1 + $2 FROM t; SELECT 1",
			want: []string{"SELECT $1 + $2 FROM t", "SELECT 1"},
		},
		{
			name: "dollar followed by special char (not alphanum or underscore)",
			sql:  "SELECT $@var; SELECT 1",
			want: []string{"SELECT $@var", "SELECT 1"},
		},
		{
			name: "dollar at very end before semicolon",
			sql:  "SELECT 1$; SELECT 2",
			want: []string{"SELECT 1$", "SELECT 2"},
		},

		// ── Stress: many quoting styles in one input ────────────────────
		{
			name: "all quoting styles in one input",
			sql:  `SELECT 'a;b', "c;d", $$e;f$$, $t$g;h$t$ /* i;j */ -- k;l`,
			want: []string{`SELECT 'a;b', "c;d", $$e;f$$, $t$g;h$t$ /* i;j */ -- k;l`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitStatements(tt.sql)
			if !slicesEqual(got, tt.want) {
				t.Errorf("splitStatements(%q)\n  got:  %v\n  want: %v", tt.sql, formatStmts(got), formatStmts(tt.want))
			}
		})
	}
}

// slicesEqual compares two string slices, treating nil and empty as equivalent
// for the purpose of "no statements returned".
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// formatStmts formats a slice of statements for readable test output.
func formatStmts(ss []string) string {
	if ss == nil {
		return "nil"
	}
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
