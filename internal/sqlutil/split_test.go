// SPDX-License-Identifier: GPL-3.0-or-later

package sqlutil

import (
	"strings"
	"testing"
)

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
		{
			name:  "only semicolons",
			input: ";;;",
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
		{
			name:  "single statement with leading/trailing whitespace",
			input: "  \t SELECT 1 \n ",
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
		{
			name:  "three statements with trailing semicolon",
			input: "SELECT 1; SELECT 2; SELECT 3;",
			want:  []string{"SELECT 1", "SELECT 2", "SELECT 3"},
		},
		{
			name:  "multiple consecutive semicolons produce no empty statements",
			input: "SELECT 1;;; SELECT 2",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "multiline statement",
			input: "SELECT\n  1\nFROM\n  dual",
			want:  []string{"SELECT\n  1\nFROM\n  dual"},
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
		{
			name:  "multiple line comments",
			input: "-- first\n-- second\nSELECT 1",
			want:  []string{"-- first\n-- second\nSELECT 1"},
		},
		{
			name:  "line comment at end consumes trailing semicolon-like chars",
			input: "SELECT 1; -- comment with ; inside",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "only a line comment",
			input: "-- just a comment",
			want:  nil,
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
		{
			name:  "unterminated block comment swallows everything",
			input: "SELECT 1; SELECT /* never closed",
			want:  []string{"SELECT 1", "SELECT /* never closed"},
		},
		{
			name:  "block comment containing single quotes",
			input: "SELECT /* 'not a string; ' */ 1",
			want:  []string{"SELECT /* 'not a string; ' */ 1"},
		},
		{
			name:  "block comment containing double quotes",
			input: `SELECT /* "not;ident" */ 1`,
			want:  []string{`SELECT /* "not;ident" */ 1`},
		},
		{
			name:  "nested block comment closes at the matching outer */",
			input: "SELECT /* outer /* inner */ still_comment */ 1",
			want:  []string{"SELECT /* outer /* inner */ still_comment */ 1"},
		},
		{
			name:  "only a block comment",
			input: "/* comment only */",
			want:  nil,
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
		{
			name:  "empty single-quoted string",
			input: "SELECT '';",
			want:  []string{"SELECT ''"},
		},
		{
			name:  "adjacent single-quoted strings",
			input: "SELECT 'a' || 'b;c'; SELECT 1",
			want:  []string{"SELECT 'a' || 'b;c'", "SELECT 1"},
		},
		{
			name:  "single-quoted string with double quotes inside",
			input: `SELECT '"hello";world'`,
			want:  []string{`SELECT '"hello";world'`},
		},
		{
			name:  "single-quoted string with line comment marker inside",
			input: "SELECT '-- not a comment; still string'",
			want:  []string{"SELECT '-- not a comment; still string'"},
		},
		{
			name:  "single-quoted string with block comment markers inside",
			input: "SELECT '/* not a comment; */'",
			want:  []string{"SELECT '/* not a comment; */'"},
		},
		{
			name:  "unterminated single-quoted string",
			input: "SELECT 'unterminated",
			want:  []string{"SELECT 'unterminated"},
		},
		{
			name:  "unterminated string with semicolons after opening quote",
			input: "SELECT 'a;b;c",
			want:  []string{"SELECT 'a;b;c"},
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
		{
			name:  "double-quoted identifier with single quote inside",
			input: `SELECT "it's" FROM t; SELECT 1`,
			want:  []string{`SELECT "it's" FROM t`, "SELECT 1"},
		},
		{
			name:  "empty double-quoted identifier",
			input: `SELECT "" FROM t`,
			want:  []string{`SELECT "" FROM t`},
		},
		{
			name:  "escaped double-quote inside identifier with semicolon",
			input: `SELECT "semi;""colon" FROM t; SELECT 1`,
			want:  []string{`SELECT "semi;""colon" FROM t`, "SELECT 1"},
		},
		{
			name:  "unterminated double-quoted identifier",
			input: `SELECT "never closed`,
			want:  []string{`SELECT "never closed`},
		},
		{
			name:  "unterminated double-quoted identifier with semicolons",
			input: `SELECT "unterminated;ident`,
			want:  []string{`SELECT "unterminated;ident`},
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
		{
			name:  "dollar-quote with underscore tag",
			input: "SELECT $my_tag$content;here$my_tag$",
			want:  []string{"SELECT $my_tag$content;here$my_tag$"},
		},
		{
			name:  "dollar-quote with numeric tag",
			input: "SELECT $123$foo;bar$123$",
			want:  []string{"SELECT $123$foo;bar$123$"},
		},
		{
			name:  "nested $$ inside tagged dollar-quote",
			input: "SELECT $tag$content$$semicol;here$tag$",
			want:  []string{"SELECT $tag$content$$semicol;here$tag$"},
		},
		{
			name:  "single quotes inside dollar-quoted string",
			input: "SELECT $$it's a 'test';$$",
			want:  []string{"SELECT $$it's a 'test';$$"},
		},
		{
			name:  "double quotes inside dollar-quoted string",
			input: `SELECT $$"ident;name"$$`,
			want:  []string{`SELECT $$"ident;name"$$`},
		},
		{
			name:  "line comment marker inside dollar-quoted string",
			input: "SELECT $$-- not a comment;$$",
			want:  []string{"SELECT $$-- not a comment;$$"},
		},
		{
			name:  "block comment markers inside dollar-quoted string",
			input: "SELECT $$/* not; a comment */$$",
			want:  []string{"SELECT $$/* not; a comment */$$"},
		},
		{
			name:  "unterminated dollar-quote captured by final flush",
			input: "CREATE FUNCTION f() AS $$ never closed",
			want:  []string{"CREATE FUNCTION f() AS $$ never closed"},
		},
		{
			name:  "unterminated tagged dollar-quoted string",
			input: "SELECT $tag$never closed; oops",
			want:  []string{"SELECT $tag$never closed; oops"},
		},
		{
			name:  "dollar sign not starting a dollar-quote (no closing $)",
			input: "SELECT $5; SELECT 1",
			want:  []string{"SELECT $5", "SELECT 1"},
		},
		{
			name:  "dollar-quote with empty body",
			input: "SELECT $$$$; SELECT 1",
			want:  []string{"SELECT $$$$", "SELECT 1"},
		},
		{
			name:  "dollar-quote in CREATE FUNCTION body",
			input: "CREATE FUNCTION f() RETURNS INT LANGUAGE SQL AS $$ SELECT 1; $$; SELECT 2",
			want:  []string{"CREATE FUNCTION f() RETURNS INT LANGUAGE SQL AS $$ SELECT 1; $$", "SELECT 2"},
		},
		{
			name:  "dollar-quote with mismatched tag does not close",
			input: "SELECT $a$content$b$more;here$a$",
			want:  []string{"SELECT $a$content$b$more;here$a$"},
		},
		{
			name:  "dollar at very end before semicolon",
			input: "SELECT 1$; SELECT 2",
			want:  []string{"SELECT 1$", "SELECT 2"},
		},
		{
			name:  "dollar followed by special char (not alphanum or underscore)",
			input: "SELECT $@var; SELECT 1",
			want:  []string{"SELECT $@var", "SELECT 1"},
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
		{
			name:  "single-quoted string followed by double-quoted identifier with semicolons",
			input: `SELECT 'val;1', "col;2" FROM t; SELECT 1`,
			want:  []string{`SELECT 'val;1', "col;2" FROM t`, "SELECT 1"},
		},
		{
			name:  "dollar-quote then single-quote",
			input: "SELECT $$a;b$$, 'c;d'; SELECT 1",
			want:  []string{"SELECT $$a;b$$, 'c;d'", "SELECT 1"},
		},
		{
			name:  "comment inside string is not a comment",
			input: "INSERT INTO t VALUES ('-- ;not comment'); SELECT 1",
			want:  []string{"INSERT INTO t VALUES ('-- ;not comment')", "SELECT 1"},
		},
		{
			name:  "string inside comment is not a string",
			input: "SELECT /* 'string;' */ 1; SELECT 2",
			want:  []string{"SELECT /* 'string;' */ 1", "SELECT 2"},
		},
		{
			name:  "all quoting styles in one input",
			input: `SELECT 'a;b', "c;d", $$e;f$$, $t$g;h$t$ /* i;j */ -- k;l`,
			want:  []string{`SELECT 'a;b', "c;d", $$e;f$$, $t$g;h$t$ /* i;j */ -- k;l`},
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
			input: "SELECT 'caf\u00e9 r\u00e9sum\u00e9 na\u00efve';",
			want:  []string{"SELECT 'caf\u00e9 r\u00e9sum\u00e9 na\u00efve'"},
		},
		{
			name:  "unicode in double-quoted identifier",
			input: `SELECT "donn` + "\u00e9" + `es" FROM t;`,
			want:  []string{`SELECT "donn` + "\u00e9" + `es" FROM t`},
		},
		{
			name:  "japanese characters in quoted identifier",
			input: "SELECT \"\u30c6\u30fc\u30d6\u30eb\" FROM t;",
			want:  []string{"SELECT \"\u30c6\u30fc\u30d6\u30eb\" FROM t"},
		},
		{
			name:  "multibyte rune adjacent to semicolon terminator",
			input: "SELECT '\u65e5\u672c\u8a9e';",
			want:  []string{"SELECT '\u65e5\u672c\u8a9e'"},
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
			// Nesting: a ';' between the inner and outer close stays in the comment.
			name:  "nested block comment: ; before the outer close stays in the comment",
			input: "SELECT /* a /* b */ ; c */ 1",
			want:  []string{"SELECT /* a /* b */ ; c */ 1"},
		},
		{
			// A fully-closed nested comment, then a real top-level ';' splits.
			name:  "nested block comment then real statement split",
			input: "SELECT /* a /* b */ c */ 1; SELECT 2",
			want:  []string{"SELECT /* a /* b */ c */ 1", "SELECT 2"},
		},
		{
			// Unbalanced nesting runs to end of input; the trailing ';' is consumed.
			name:  "unbalanced nested block comment runs to end of input",
			input: "SELECT /* outer /* inner */ rest;",
			want:  []string{"SELECT /* outer /* inner */ rest;"},
		},
		{
			name:  "unterminated block comment captured by final flush",
			input: "SELECT /* never closed",
			want:  []string{"SELECT /* never closed"},
		},
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
			input: "-- comment\rSELECT 2;",
			want:  nil, // whole input is one line comment (CR doesn't end it) → comment-only, dropped
		},

		// ── backslash IS the escape character in Snowflake strings (#701) ────
		{
			name:  "backslash-escaped quote does not close the string",
			input: "SELECT 'a\\';b';",
			want:  []string{"SELECT 'a\\';b'"},
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

		// ── comment-only statements are dropped (Snowflake rejects them) ──────
		{
			name:  "block comment only statement",
			input: "/* comment */;",
			want:  nil,
		},
		{
			name:  "line comment only with no terminating newline",
			input: "-- only a comment",
			want:  nil,
		},
		{
			name:  "line comment only followed by semicolon on next line",
			input: "-- comment\n;",
			want:  nil,
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
			name:  "tabs between statements",
			input: "SELECT 1;\tSELECT 2",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "statement is only whitespace after trimming",
			input: "SELECT 1;   ;SELECT 2",
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

		// ── single-quoted string edge cases ───────────────────────────────────
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
			input: func() string {
				var b strings.Builder
				for i := 0; i < 50; i++ {
					b.WriteString("SELECT 1;")
				}
				return b.String()
			}(),
			want: nil, // handled separately in the loop below
		},

		// ── realistic Snowflake DDL ─────────────────────────────────────────
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
			want: nil, // handled by a separate count assertion below
		},

		// ── realistic Snowflake SQL ─────────────────────────────────────────
		{
			name: "CREATE PROCEDURE with dollar-quoted body",
			input: `CREATE OR REPLACE PROCEDURE my_proc()
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
			name:  "INSERT with quoted values containing semicolons",
			input: "INSERT INTO t (a, b) VALUES ('x;y', 'z'); COMMIT",
			want: []string{
				"INSERT INTO t (a, b) VALUES ('x;y', 'z')",
				"COMMIT",
			},
		},
		{
			name:  "COPY INTO with file path",
			input: "COPY INTO @stage/path FROM t; SELECT 1",
			want: []string{
				"COPY INTO @stage/path FROM t",
				"SELECT 1",
			},
		},
		{
			name:  "USE then SELECT",
			input: "USE DATABASE mydb; USE SCHEMA public; SELECT * FROM t",
			want: []string{
				"USE DATABASE mydb",
				"USE SCHEMA public",
				"SELECT * FROM t",
			},
		},
		{
			name: "JavaScript UDF with dollar-quoted body",
			input: `CREATE FUNCTION js_func(x FLOAT)
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
			name:  "comment-only lines between statements",
			input: "SELECT 1;\n-- transition\nSELECT 2;",
			want:  []string{"SELECT 1", "-- transition\nSELECT 2"},
		},
		{
			name:  "EXECUTE IMMEDIATE with dollar-quoted body",
			input: "EXECUTE IMMEDIATE $$INSERT INTO t VALUES (1);$$; SELECT 1",
			want:  []string{"EXECUTE IMMEDIATE $$INSERT INTO t VALUES (1);$$", "SELECT 1"},
		},

		// ── Security / injection edge cases ─────────────────────────────────
		{
			name: "classic SQL injection attempt: quote break with semicolon",
			input: "SELECT * FROM users WHERE name = ''; DROP TABLE users; --'",
			want: []string{
				"SELECT * FROM users WHERE name = ''",
				"DROP TABLE users",
			},
		},
		{
			name:  "backslash escapes the quote: whole tail is one string literal (#701)",
			input: `SELECT '\'; DROP TABLE users; --'`,
			want: []string{
				`SELECT '\'; DROP TABLE users; --'`,
			},
		},
		{
			name:  "properly escaped string with semicolons is one statement",
			input: "SELECT 'O''Brien;test' FROM t",
			want:  []string{"SELECT 'O''Brien;test' FROM t"},
		},
		{
			name:  "double-quoted identifier injection attempt",
			input: `SELECT ""; DROP TABLE users"`,
			want:  []string{`SELECT ""`, `DROP TABLE users"`},
		},
		{
			name:  "dollar-quote injection: wrong tag does not close",
			input: "SELECT $a$;DROP TABLE users;$b$;$a$",
			want:  []string{"SELECT $a$;DROP TABLE users;$b$;$a$"},
		},
		{
			name:  "block comment used to hide statement boundary",
			input: "SELECT 1 /*;*/ ; SELECT 2",
			want:  []string{"SELECT 1 /*;*/", "SELECT 2"},
		},
		{
			name:  "line comment hides trailing semicolons",
			input: "SELECT 1 -- ; hidden\n; SELECT 2",
			want:  []string{"SELECT 1 -- ; hidden", "SELECT 2"},
		},
		{
			name:  "deeply nested quoting attempt",
			input: `SELECT ''''; DROP TABLE t`,
			want:  []string{"SELECT ''''", "DROP TABLE t"},
		},
		{
			name:  "triple single quotes then semicolon",
			input: "SELECT '''; SELECT 1",
			want:  []string{"SELECT '''; SELECT 1"},
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

// FuzzSplit verifies that Split never panics on arbitrary input and that
// every returned statement is non-empty after trimming.
func FuzzSplit(f *testing.F) {
	seeds := []string{
		"",
		"SELECT 1",
		"SELECT 1; SELECT 2",
		"SELECT 'a;b'",
		`SELECT "a;b"`,
		"SELECT $$a;b$$",
		"SELECT $tag$a;b$tag$",
		"SELECT /* ; */ 1",
		"SELECT 1 -- ;comment",
		"SELECT '''';",
		"SELECT '\\'; DROP TABLE t; --'",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, sql string) {
		stmts := Split(sql)
		for i, s := range stmts {
			if strings.TrimSpace(s) == "" {
				t.Errorf("Split(%q): statement[%d] is empty or whitespace-only", sql, i)
			}
		}
	})
}
