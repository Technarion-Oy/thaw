// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"strings"
	"testing"

	"thaw/internal/sqltok"
)

func TestIsBoolean(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"BOOLEAN", true},
		{"BOOL", true},
		{"boolean", true},
		{"  BOOLEAN  ", true},
		{"VARCHAR", false},
		{"NUMBER", false},
		{"ARRAY", false},
	}

	for _, tt := range tests {
		if got := IsBoolean(tt.dataType); got != tt.expected {
			t.Errorf("IsBoolean(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"NUMBER", true},
		{"NUMBER(38,0)", true},
		{"DECIMAL(10,2)", true},
		{"INT", true},
		{"INTEGER", true},
		{"BIGINT", true},
		{"SMALLINT", true},
		{"TINYINT", true},
		{"BYTEINT", true},
		{"FLOAT", true},
		{"DOUBLE", true},
		{"REAL", true},
		{"NUMERIC", true},
		{"VARCHAR", false},
		{"BOOLEAN", false},
	}

	for _, tt := range tests {
		if got := IsNumeric(tt.dataType); got != tt.expected {
			t.Errorf("IsNumeric(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestNeedsQuotes(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"VARCHAR", true},
		{"STRING", true},
		{"TEXT", true},
		{"TIMESTAMP", true},
		{"DATE", true},
		{"BOOLEAN", false},
		{"NUMBER", false},
		{"INT", false},
	}

	for _, tt := range tests {
		if got := NeedsQuotes(tt.dataType); got != tt.expected {
			t.Errorf("NeedsQuotes(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestDollarQuoteTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"no dollar quote", "select 1", "$$"},
		{"contains $$", "select '$$'", "$thaw$"},
		{"contains $$ and $thaw$", "a $$ b $thaw$ c", "$thaw_body$"},
		{"contains all named tags", "$$ $thaw$ $thaw_body$", "$thaw_0$"},
	}
	for _, tt := range tests {
		if got := DollarQuoteTag(tt.body); got != tt.want {
			t.Errorf("DollarQuoteTag(%q) = %q, want %q", tt.body, got, tt.want)
		}
		// The chosen tag must never occur in the body.
		if strings.Contains(tt.body, DollarQuoteTag(tt.body)) {
			t.Errorf("DollarQuoteTag(%q) returned a tag present in the body", tt.body)
		}
	}
}

func TestUnescapeStringLiteral(t *testing.T) {
	tests := []struct {
		name     string
		inner    string
		want     string
		wantSafe bool
	}{
		{"plain", "select 1", "select 1", true},
		{"doubled quote", "let x := ''hello''", "let x := 'hello'", true},
		{"backslash quote", `it\'s`, "it's", true},
		{"backslash escapes", `a\\b\nc\td`, "a\\b\nc\td", true},
		{"double quote escape", `say \"hi\"`, `say "hi"`, true},
		{"unknown escape drops backslash", `\z`, "z", true},
		{"trailing backslash", `end\`, `end\`, true},
		{"hex escape", `A\x42C`, "ABC", true},
		{"unicode escape", `\u0042`, "B", true},
		{"octal escape", `\101`, "A", true},
		{"malformed hex is a plain x", `\xZZ`, "xZZ", true},
		{"nul is not verbatim safe", `a\0b`, "a\x00b", false},
		{"backspace is not verbatim safe", `a\bb`, "a\bb", false},
		{"non-ascii hex is ambiguous", `caf\xe9`, "café", false},
		{"unicode above ascii is fine", `caf\u00e9`, "café", true},
		{"lone surrogate is not encodable", `\ud83d`, "�", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, safe := UnescapeStringLiteral(tt.inner)
			if got != tt.want {
				t.Errorf("UnescapeStringLiteral(%q) = %q, want %q", tt.inner, got, tt.want)
			}
			if safe != tt.wantSafe {
				t.Errorf("UnescapeStringLiteral(%q) safe = %v, want %v", tt.inner, safe, tt.wantSafe)
			}
		})
	}
}

func TestDollarQuoteBody(t *testing.T) {
	tests := []struct {
		name string
		stmt string
		want string
	}{
		{
			name: "procedure body",
			stmt: "CREATE OR REPLACE PROCEDURE FOO()\nRETURNS VARCHAR\nLANGUAGE SQL\nAS 'begin\n  let x := ''hello'';\n  return x;\nend'",
			want: "CREATE OR REPLACE PROCEDURE FOO()\nRETURNS VARCHAR\nLANGUAGE SQL\nAS $$begin\n  let x := 'hello';\n  return x;\nend$$",
		},
		{
			name: "trailing semicolon and whitespace are preserved",
			stmt: "CREATE FUNCTION F() RETURNS INT AS 'select 1' ;",
			want: "CREATE FUNCTION F() RETURNS INT AS $$select 1$$ ;",
		},
		{
			name: "secure function",
			stmt: "CREATE OR REPLACE SECURE FUNCTION F() RETURNS INT AS 'select ''x'''",
			want: "CREATE OR REPLACE SECURE FUNCTION F() RETURNS INT AS $$select 'x'$$",
		},
		{
			name: "python handler body",
			stmt: "CREATE FUNCTION F() RETURNS INT LANGUAGE PYTHON HANDLER = 'run' AS 'def run():\n\treturn 1'",
			want: "CREATE FUNCTION F() RETURNS INT LANGUAGE PYTHON HANDLER = 'run' AS $$def run():\n\treturn 1$$",
		},
		{
			name: "javascript body with backslash escapes",
			stmt: `CREATE FUNCTION F() RETURNS STRING LANGUAGE JAVASCRIPT AS 'return "a\\nb".replace(/\\s/g, '''')'`,
			want: "CREATE FUNCTION F() RETURNS STRING LANGUAGE JAVASCRIPT AS $$return \"a\\nb\".replace(/\\s/g, '')$$",
		},
		{
			name: "AS inside RETURNS TABLE is not the body",
			stmt: "CREATE FUNCTION F() RETURNS TABLE (A NUMBER AS X) AS 'select 1'",
			want: "CREATE FUNCTION F() RETURNS TABLE (A NUMBER AS X) AS $$select 1$$",
		},
		{
			name: "comment clause before the body",
			stmt: "CREATE FUNCTION F() RETURNS INT COMMENT = 'a comment' AS 'select 1'",
			want: "CREATE FUNCTION F() RETURNS INT COMMENT = 'a comment' AS $$select 1$$",
		},
		{
			name: "body containing $$ gets a named tag",
			stmt: "CREATE FUNCTION F() RETURNS INT AS 'select ''$$'''",
			want: "CREATE FUNCTION F() RETURNS INT AS $thaw$select '$$'$thaw$",
		},
		{
			name: "body ending in a dollar gets a named tag",
			stmt: "CREATE FUNCTION F() RETURNS STRING AS 'select ''x''$'",
			want: "CREATE FUNCTION F() RETURNS STRING AS $thaw$select 'x'$$thaw$",
		},
		{
			// The scan for the closing delimiter starts after the opening one,
			// so a leading "$" cannot collide and needs no escalation.
			name: "body starting with a dollar keeps the bare tag",
			stmt: "CREATE FUNCTION F() RETURNS STRING AS 'select $ x'",
			want: "CREATE FUNCTION F() RETURNS STRING AS $$select $ x$$",
		},
		{
			// "$thaw$" would close inside the body's own "$thaw" tail, even
			// though the body neither contains "$thaw$" nor ends with "$".
			name: "body ending in a tag prefix escalates past that tag",
			stmt: "CREATE FUNCTION F() RETURNS STRING AS 'x$$y$thaw'",
			want: "CREATE FUNCTION F() RETURNS STRING AS $thaw_body$x$$y$thaw$thaw_body$",
		},
		{
			name: "already dollar-quoted is untouched",
			stmt: "CREATE FUNCTION F() RETURNS INT AS $$select 1$$",
			want: "CREATE FUNCTION F() RETURNS INT AS $$select 1$$",
		},
		{
			name: "empty body is untouched",
			stmt: "CREATE FUNCTION F() RETURNS INT AS ''",
			want: "CREATE FUNCTION F() RETURNS INT AS ''",
		},
		{
			name: "control character in the body is untouched",
			stmt: `CREATE FUNCTION F() RETURNS STRING AS 'a\0b'`,
			want: `CREATE FUNCTION F() RETURNS STRING AS 'a\0b'`,
		},
		{
			name: "view is not a function body",
			stmt: "CREATE VIEW V AS SELECT 'a' AS C",
			want: "CREATE VIEW V AS SELECT 'a' AS C",
		},
		{
			name: "table is untouched",
			stmt: "CREATE TABLE T (C VARCHAR DEFAULT 'x')",
			want: "CREATE TABLE T (C VARCHAR DEFAULT 'x')",
		},
		{
			name: "external function endpoint is not a body",
			stmt: "CREATE EXTERNAL FUNCTION F() RETURNS INT API_INTEGRATION = I AS 'https://example.com'",
			want: "CREATE EXTERNAL FUNCTION F() RETURNS INT API_INTEGRATION = I AS 'https://example.com'",
		},
		{
			name: "unterminated literal is untouched",
			stmt: "CREATE FUNCTION F() RETURNS INT AS 'select 1",
			want: "CREATE FUNCTION F() RETURNS INT AS 'select 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DollarQuoteBody(tt.stmt); got != tt.want {
				t.Errorf("DollarQuoteBody(%q)\n got %q\nwant %q", tt.stmt, got, tt.want)
			}
		})
	}
}

// The rewritten statement must tokenize back to the exact body value: neither
// the body's own content nor the seam where it meets the closing delimiter may
// terminate the dollar-quoted span early. The check runs through the same
// first-occurrence-wins scanner Snowflake's parser implements (sqltok), rather
// than trusting the tag this package picked.
func TestDollarQuoteBodyRoundTrip(t *testing.T) {
	bodies := []string{
		"select 1",
		"a $$ b",
		"x$",
		"$x",
		"$thaw$ $$ $thaw_body$",
		"begin\n  let x := 'q';\nend",
		// Bodies whose tail is a proper prefix of the tag that would otherwise
		// be chosen: the closing delimiter starts inside the body unless the
		// seam itself is checked.
		"x$$y$thaw",
		"x$$y$thaw_",
		"$$ $thaw$ ends with $thaw_body",
		"$$ $thaw$ $thaw_body$ $thaw_0",
		"$",
	}
	for _, body := range bodies {
		stmt := "CREATE FUNCTION F() RETURNS STRING AS " + quoteLiteral(body)
		got := DollarQuoteBody(stmt)

		var spans []string
		for _, tok := range sqltok.SignificantTokens(got) {
			if tok.Kind == sqltok.DollarQuoted {
				spans = append(spans, tok.Text(got))
			}
		}
		if len(spans) != 1 {
			t.Errorf("body %q: rewritten as %q, tokenized into %d dollar-quoted spans, want 1",
				body, got, len(spans))
			continue
		}

		// The span carries the delimiter on both ends; what is left must be the
		// original body, byte for byte.
		span := spans[0]
		tag := span[:strings.Index(span[1:], "$")+2]
		inner := strings.TrimSuffix(strings.TrimPrefix(span, tag), tag)
		if inner != body {
			t.Errorf("round trip of %q: rewritten as %q, tag %q, recovered %q", body, got, tag, inner)
		}
	}
}

// quoteLiteral renders body as the single-quoted literal GET_DDL would return.
func quoteLiteral(body string) string {
	return "'" + strings.ReplaceAll(body, "'", "''") + "'"
}
