// SPDX-License-Identifier: GPL-3.0-or-later

package sqltok

import (
	"strings"
	"testing"
)

func TestTokenizeEmpty(t *testing.T) {
	tokens := Tokenize("")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token (EOF), got %d", len(tokens))
	}
	if tokens[0].Kind != EOF {
		t.Errorf("expected EOF, got %s", tokens[0].Kind)
	}
}

func TestTokenizeSimpleSelect(t *testing.T) {
	sql := "SELECT 1"
	tokens := Tokenize(sql)

	expected := []struct {
		kind TokenKind
		text string
	}{
		{Keyword, "SELECT"},
		{Whitespace, " "},
		{NumberLit, "1"},
		{EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		tok := tokens[i]
		if tok.Kind != exp.kind {
			t.Errorf("token[%d]: kind=%s, want %s", i, tok.Kind, exp.kind)
		}
		if tok.Kind != EOF {
			if got := tok.Text(sql); got != exp.text {
				t.Errorf("token[%d]: text=%q, want %q", i, got, exp.text)
			}
		}
	}
}

func TestTokenKinds(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []TokenKind
	}{
		{"whitespace", "  \t\r", []TokenKind{Whitespace, EOF}},
		{"newline", "\n", []TokenKind{Newline, EOF}},
		{"mixed ws and newlines", " \n ", []TokenKind{Whitespace, Newline, Whitespace, EOF}},
		{"line comment", "-- comment", []TokenKind{LineComment, EOF}},
		{"line comment then newline", "-- comment\n", []TokenKind{LineComment, Newline, EOF}},
		{"slash line comment", "// comment", []TokenKind{LineComment, EOF}},
		{"slash line comment then newline", "// comment\n", []TokenKind{LineComment, Newline, EOF}},
		{"double slash no space", "a//b", []TokenKind{Identifier, LineComment, EOF}},
		{"file uri triple slash is not comment", "file:///tmp", []TokenKind{Keyword, Colon, Operator, Identifier, EOF}},
		{"s3 uri is not comment", "s3://b", []TokenKind{Identifier, Colon, Operator, Identifier, EOF}},
		{"block comment then line comment", "/* x */" + "//comment", []TokenKind{BlockComment, LineComment, EOF}},
		{"single slashes are division", "a / b / c", []TokenKind{Identifier, Whitespace, Operator, Whitespace, Identifier, Whitespace, Operator, Whitespace, Identifier, EOF}},
		{"block comment", "/* comment */", []TokenKind{BlockComment, EOF}},
		{"block comment multiline", "/* a\nb */", []TokenKind{BlockComment, EOF}},
		{"string literal", "'hello'", []TokenKind{StringLit, EOF}},
		{"string with escape", "'it''s'", []TokenKind{StringLit, EOF}},
		{"string backslash-escaped quote", `'it\'s a test'`, []TokenKind{StringLit, EOF}},
		{"string escaped backslash then close", `'a\\'`, []TokenKind{StringLit, EOF}},
		{"empty string", "''", []TokenKind{StringLit, EOF}},
		{"quoted ident", `"my col"`, []TokenKind{QuotedIdent, EOF}},
		{"quoted ident escape", `"col""name"`, []TokenKind{QuotedIdent, EOF}},
		{"dollar quoted", "$$body$$", []TokenKind{DollarQuoted, EOF}},
		{"dollar quoted tagged", "$tag$body$tag$", []TokenKind{DollarQuoted, EOF}},
		{"number int", "42", []TokenKind{NumberLit, EOF}},
		{"number float", "3.14", []TokenKind{NumberLit, EOF}},
		{"number hex", "0xDEAD", []TokenKind{NumberLit, EOF}},
		{"number exp", "1e10", []TokenKind{NumberLit, EOF}},
		{"number exp neg", "1E-3", []TokenKind{NumberLit, EOF}},
		{"leading dot number", ".5", []TokenKind{NumberLit, EOF}},
		{"dot", ".", []TokenKind{Dot, EOF}},
		{"comma", ",", []TokenKind{Comma, EOF}},
		{"semicolon", ";", []TokenKind{Semicolon, EOF}},
		{"lparen", "(", []TokenKind{LParen, EOF}},
		{"rparen", ")", []TokenKind{RParen, EOF}},
		{"lbracket", "[", []TokenKind{LBracket, EOF}},
		{"rbracket", "]", []TokenKind{RBracket, EOF}},
		{"colon", ":", []TokenKind{Colon, EOF}},
		{"double colon", "::", []TokenKind{Operator, EOF}},
		{"at", "@", []TokenKind{At, EOF}},
		{"pipe pipe", "||", []TokenKind{Operator, EOF}},
		{"arrow", "=>", []TokenKind{Operator, EOF}},
		{"not equal", "!=", []TokenKind{Operator, EOF}},
		{"diamond", "<>", []TokenKind{Operator, EOF}},
		{"less equal", "<=", []TokenKind{Operator, EOF}},
		{"greater equal", ">=", []TokenKind{Operator, EOF}},
		{"less", "<", []TokenKind{Operator, EOF}},
		{"greater", ">", []TokenKind{Operator, EOF}},
		{"equal", "=", []TokenKind{Operator, EOF}},
		{"plus", "+", []TokenKind{Operator, EOF}},
		{"minus (not comment)", "-x", []TokenKind{Operator, Identifier, EOF}},
		{"star", "*", []TokenKind{Operator, EOF}},
		{"slash (not comment)", "/ 1", []TokenKind{Operator, Whitespace, NumberLit, EOF}},
		{"percent", "%", []TokenKind{Operator, EOF}},
		{"caret", "^", []TokenKind{Operator, EOF}},
		{"keyword SELECT", "SELECT", []TokenKind{Keyword, EOF}},
		{"keyword lowercase", "select", []TokenKind{Keyword, EOF}},
		{"identifier", "my_table", []TokenKind{Identifier, EOF}},
		{"identifier with dollar", "SYSTEM$TYPEOF", []TokenKind{Identifier, EOF}},
		{"bare dollar", "$", []TokenKind{Other, EOF}},
		{"dollar number", "$1", []TokenKind{Other, NumberLit, EOF}},
		{"other tilde", "~", []TokenKind{Other, EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := Tokenize(tt.sql)
			if len(tokens) != len(tt.want) {
				kinds := make([]string, len(tokens))
				for i, tk := range tokens {
					kinds[i] = tk.Kind.String()
				}
				t.Fatalf("got %d tokens %v, want %d %v",
					len(tokens), kinds, len(tt.want), tt.want)
			}
			for i, wantKind := range tt.want {
				if tokens[i].Kind != wantKind {
					t.Errorf("token[%d]: kind=%s, want %s", i, tokens[i].Kind, wantKind)
				}
			}
		})
	}
}

func TestTokenizeLineColTracking(t *testing.T) {
	sql := "SELECT\n  1\n  FROM t"
	tokens := Tokenize(sql)

	checks := []struct {
		kind TokenKind
		line int
		col  int
	}{
		{Keyword, 1, 1},     // SELECT
		{Newline, 1, 7},     // \n
		{Whitespace, 2, 1},  // "  "
		{NumberLit, 2, 3},   // 1
		{Newline, 2, 4},     // \n
		{Whitespace, 3, 1},  // "  "
		{Keyword, 3, 3},     // FROM
		{Whitespace, 3, 7},  // " "
		{Identifier, 3, 8},  // t
		{EOF, 3, 9},
	}

	if len(tokens) != len(checks) {
		t.Fatalf("expected %d tokens, got %d", len(checks), len(tokens))
	}
	for i, chk := range checks {
		tok := tokens[i]
		if tok.Kind != chk.kind {
			t.Errorf("token[%d]: kind=%s, want %s", i, tok.Kind, chk.kind)
		}
		if tok.Line != chk.line {
			t.Errorf("token[%d] (%s): line=%d, want %d", i, tok.Kind, tok.Line, chk.line)
		}
		if tok.Col != chk.col {
			t.Errorf("token[%d] (%s): col=%d, want %d", i, tok.Kind, tok.Col, chk.col)
		}
	}
}

func TestTokenizeBlockCommentLineTracking(t *testing.T) {
	sql := "a /* b\nc */ d"
	tokens := Tokenize(sql)

	// a WS /* b\nc */ WS d EOF
	if len(tokens) < 5 {
		t.Fatalf("expected at least 5 tokens, got %d", len(tokens))
	}
	// Block comment starts at line 1
	bc := tokens[2]
	if bc.Kind != BlockComment {
		t.Fatalf("token[2]: kind=%s, want BlockComment", bc.Kind)
	}
	if bc.Line != 1 {
		t.Errorf("block comment: line=%d, want 1", bc.Line)
	}
	// Token after block comment should be on line 2
	ws := tokens[3]
	if ws.Line != 2 {
		t.Errorf("token after block comment: line=%d, want 2", ws.Line)
	}
	d := tokens[4]
	if d.Line != 2 {
		t.Errorf("'d' token: line=%d, want 2", d.Line)
	}
}

func TestTokenizeStringWithNewlines(t *testing.T) {
	sql := "'line1\nline2'"
	tokens := Tokenize(sql)
	if len(tokens) != 2 { // StringLit, EOF
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Kind != StringLit {
		t.Errorf("token[0]: kind=%s, want StringLit", tokens[0].Kind)
	}
	if tokens[0].Line != 1 {
		t.Errorf("string start: line=%d, want 1", tokens[0].Line)
	}
	// EOF should be on line 2
	if tokens[1].Line != 2 {
		t.Errorf("EOF: line=%d, want 2", tokens[1].Line)
	}
}

func TestTokenizeDollarQuotedTag(t *testing.T) {
	sql := "$body$content;here$body$"
	tokens := Tokenize(sql)
	if len(tokens) != 2 { // DollarQuoted, EOF
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Kind != DollarQuoted {
		t.Errorf("token[0]: kind=%s, want DollarQuoted", tokens[0].Kind)
	}
	if tokens[0].Tag != "$body$" {
		t.Errorf("token[0]: tag=%q, want $body$", tokens[0].Tag)
	}
	if tokens[0].Text(sql) != sql {
		t.Errorf("token[0]: text=%q, want %q", tokens[0].Text(sql), sql)
	}
}

func TestTokenizeUnterminatedString(t *testing.T) {
	sql := "SELECT 'unterminated"
	tokens := Tokenize(sql)
	// Should not panic; string should consume to end.
	found := false
	for _, tok := range tokens {
		if tok.Kind == StringLit {
			found = true
			if tok.Text(sql) != "'unterminated" {
				t.Errorf("string text=%q, want %q", tok.Text(sql), "'unterminated")
			}
		}
	}
	if !found {
		t.Error("expected StringLit token for unterminated string")
	}
}

// TestTokenizeBackslashEscapedQuote is the repro from issue #701: a
// backslash-escaped quote inside a Snowflake string constant must not close
// the string, so the whole literal tokenizes as one terminated StringLit.
func TestTokenizeBackslashEscapedQuote(t *testing.T) {
	sql := `SELECT 'it\'s a test';`
	tokens := Tokenize(sql)

	var strs []Token
	for _, tok := range tokens {
		if tok.Kind == StringLit {
			strs = append(strs, tok)
		}
	}
	if len(strs) != 1 {
		t.Fatalf("got %d StringLit tokens, want 1", len(strs))
	}
	if got, want := strs[0].Text(sql), `'it\'s a test'`; got != want {
		t.Errorf("string text=%q, want %q", got, want)
	}
	if strs[0].Unterminated {
		t.Error("string should be terminated")
	}
}

func TestTokenizeUnterminatedBlockComment(t *testing.T) {
	sql := "SELECT /* never closed"
	tokens := Tokenize(sql)
	found := false
	for _, tok := range tokens {
		if tok.Kind == BlockComment {
			found = true
			if tok.Text(sql) != "/* never closed" {
				t.Errorf("block comment text=%q, want %q", tok.Text(sql), "/* never closed")
			}
		}
	}
	if !found {
		t.Error("expected BlockComment token for unterminated block comment")
	}
}

func TestTokenizeUnterminatedDollarQuote(t *testing.T) {
	sql := "SELECT $$ never closed"
	tokens := Tokenize(sql)
	found := false
	for _, tok := range tokens {
		if tok.Kind == DollarQuoted {
			found = true
		}
	}
	if !found {
		t.Error("expected DollarQuoted token for unterminated dollar-quote")
	}
}

func TestTokenizeUnterminatedFlag(t *testing.T) {
	// kind under test → the one token of that kind we check.
	cases := []struct {
		name            string
		sql             string
		kind            TokenKind
		wantUnterminated bool
	}{
		{"closed string", "'abc'", StringLit, false},
		{"unclosed string", "'abc", StringLit, true},
		{"lone quote", "'", StringLit, true},
		{"escaped-then-unclosed string", "'a''", StringLit, true},
		{"closed ident", `"abc"`, QuotedIdent, false},
		{"unclosed ident", `"abc`, QuotedIdent, true},
		{"closed block comment", "/* x */", BlockComment, false},
		{"unclosed block comment", "/* x", BlockComment, true},
		{"closed dollar", "$$x$$", DollarQuoted, false},
		{"unclosed dollar", "$$x", DollarQuoted, true},
		{"closed tagged dollar", "$t$x$t$", DollarQuoted, false},
		{"unclosed tagged dollar", "$t$x", DollarQuoted, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tok Token
			found := false
			for _, tk := range Tokenize(tc.sql) {
				if tk.Kind == tc.kind {
					tok, found = tk, true
					break
				}
			}
			if !found {
				t.Fatalf("no %v token in %q", tc.kind, tc.sql)
			}
			if tok.Unterminated != tc.wantUnterminated {
				t.Errorf("Unterminated = %v; want %v", tok.Unterminated, tc.wantUnterminated)
			}
		})
	}
	// Non-delimited kinds are never flagged.
	for _, tk := range Tokenize("SELECT a + 1") {
		if tk.Unterminated {
			t.Errorf("token %v should not be Unterminated", tk.Kind)
		}
	}
}

func TestTokenizeUnicodeInStrings(t *testing.T) {
	sql := "SELECT 'café résumé'"
	tokens := Tokenize(sql)
	found := false
	for _, tok := range tokens {
		if tok.Kind == StringLit {
			found = true
			if tok.Text(sql) != "'café résumé'" {
				t.Errorf("string text=%q, want %q", tok.Text(sql), "'café résumé'")
			}
		}
	}
	if !found {
		t.Error("expected StringLit token")
	}
}

func TestTokenizeUnicodeInQuotedIdent(t *testing.T) {
	sql := `SELECT "données" FROM t`
	tokens := Tokenize(sql)
	found := false
	for _, tok := range tokens {
		if tok.Kind == QuotedIdent {
			found = true
			if tok.Text(sql) != `"données"` {
				t.Errorf("quoted ident text=%q, want %q", tok.Text(sql), `"données"`)
			}
		}
	}
	if !found {
		t.Error("expected QuotedIdent token")
	}
}

func TestTokenizeMultiCharOperators(t *testing.T) {
	tests := []struct {
		sql  string
		text string
	}{
		{"::", "::"},
		{"||", "||"},
		{"=>", "=>"},
		{"<>", "<>"},
		{"!=", "!="},
		{"<=", "<="},
		{">=", ">="},
	}
	for _, tt := range tests {
		tokens := Tokenize(tt.sql)
		if tokens[0].Kind != Operator {
			t.Errorf("%q: kind=%s, want Operator", tt.sql, tokens[0].Kind)
		}
		if tokens[0].Text(tt.sql) != tt.text {
			t.Errorf("%q: text=%q, want %q", tt.sql, tokens[0].Text(tt.sql), tt.text)
		}
	}
}

func TestTokenizeKeywordCaseInsensitive(t *testing.T) {
	for _, kw := range []string{"select", "SELECT", "Select", "sElEcT"} {
		tokens := Tokenize(kw)
		if tokens[0].Kind != Keyword {
			t.Errorf("%q: kind=%s, want Keyword", kw, tokens[0].Kind)
		}
	}
}

func TestTokenizeIdentifierNotKeyword(t *testing.T) {
	idents := []string{"my_table", "foo", "bar123", "_private"}
	for _, id := range idents {
		tokens := Tokenize(id)
		if tokens[0].Kind != Identifier {
			t.Errorf("%q: kind=%s, want Identifier", id, tokens[0].Kind)
		}
	}
}

func TestTokenizeIter(t *testing.T) {
	sql := "SELECT 1; SELECT 2"
	next := TokenizeIter(sql)

	var kinds []TokenKind
	for {
		tok, ok := next()
		kinds = append(kinds, tok.Kind)
		if !ok {
			break
		}
	}

	// Expect: Keyword, WS, NumberLit, Semicolon, WS, Keyword, WS, NumberLit, EOF
	if len(kinds) < 9 {
		t.Fatalf("expected at least 9 tokens, got %d", len(kinds))
	}
	if kinds[len(kinds)-1] != EOF {
		t.Errorf("last kind=%s, want EOF", kinds[len(kinds)-1])
	}
}

func TestTokenizeIterExhausted(t *testing.T) {
	next := TokenizeIter("")
	tok1, ok1 := next()
	if !ok1 || tok1.Kind != EOF {
		t.Errorf("first call: ok=%v, kind=%s", ok1, tok1.Kind)
	}
	tok2, ok2 := next()
	if ok2 {
		t.Errorf("second call should return ok=false, got ok=%v, kind=%s", ok2, tok2.Kind)
	}
}

func TestTokenizeSystemDollarFunction(t *testing.T) {
	// SYSTEM$TYPEOF should be a single Identifier, not split at $
	sql := "SYSTEM$TYPEOF"
	tokens := Tokenize(sql)
	// Because SYSTEM$TYPEOF is in builtinFunctions but also in keywords as Identifier,
	// it should be classified based on the keywords map. Let's check.
	if len(tokens) != 2 { // word + EOF
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Text(sql) != "SYSTEM$TYPEOF" {
		t.Errorf("text=%q, want SYSTEM$TYPEOF", tokens[0].Text(sql))
	}
}

func TestTokenizeCompleteStatement(t *testing.T) {
	sql := `CREATE TABLE "my_db"."public"."tbl" (id INT, name VARCHAR(100));`
	tokens := Tokenize(sql)

	// Just verify it doesn't panic and produces a reasonable number of tokens
	if len(tokens) < 10 {
		t.Errorf("expected many tokens, got %d", len(tokens))
	}
	// Last token should be EOF
	if tokens[len(tokens)-1].Kind != EOF {
		t.Error("last token should be EOF")
	}
}

// FuzzTokenize verifies that Tokenize never panics and that token spans
// cover the entire input without gaps or overlaps.
func FuzzTokenize(f *testing.F) {
	seeds := []string{
		"",
		"SELECT 1",
		"SELECT 'a;b'",
		`SELECT "col"`,
		"SELECT $$body$$",
		"-- comment\n",
		"/* block */",
		"::||=><>!=<=>=",
		"$tag$content$tag$",
		"0xDEAD 3.14 1e10",
		"SELECT 'unterminated",
		"SELECT /* unclosed",
		"SELECT $$ unclosed",
		"'\\'; DROP TABLE t;",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, sql string) {
		tokens := Tokenize(sql)
		if len(tokens) == 0 {
			t.Fatal("Tokenize returned no tokens")
		}
		if tokens[len(tokens)-1].Kind != EOF {
			t.Fatal("last token is not EOF")
		}

		// Verify coverage: all non-EOF tokens must tile the input.
		prevEnd := 0
		for _, tok := range tokens {
			if tok.Kind == EOF {
				if tok.Start != len(sql) {
					t.Errorf("EOF.Start=%d, want %d", tok.Start, len(sql))
				}
				break
			}
			if tok.Start != prevEnd {
				t.Errorf("gap: token at %d, prev ended at %d", tok.Start, prevEnd)
			}
			if tok.End <= tok.Start {
				t.Errorf("empty token at %d-%d", tok.Start, tok.End)
			}
			prevEnd = tok.End
		}
		if prevEnd != len(sql) {
			t.Errorf("tokens end at %d, input length %d", prevEnd, len(sql))
		}
	})
}

func TestTokenKindString(t *testing.T) {
	if Whitespace.String() != "Whitespace" {
		t.Errorf("Whitespace.String()=%q", Whitespace.String())
	}
	if EOF.String() != "EOF" {
		t.Errorf("EOF.String()=%q", EOF.String())
	}
}

func TestTokenizeComplexDollarQuote(t *testing.T) {
	// Verify that a dollar-quoted block with semicolons and nested comment
	// markers is a single token.
	sql := "CREATE FUNCTION f() AS $$ SELECT 1; -- not comment\n SELECT 2; $$"
	tokens := Tokenize(sql)
	dqCount := 0
	for _, tok := range tokens {
		if tok.Kind == DollarQuoted {
			dqCount++
			text := tok.Text(sql)
			if !strings.Contains(text, "SELECT 1;") {
				t.Errorf("dollar-quoted body missing content: %q", text)
			}
		}
	}
	if dqCount != 1 {
		t.Errorf("expected 1 DollarQuoted token, got %d", dqCount)
	}
}
