// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqltok

import (
	"strings"
	"testing"
)

func TestStripComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "no comments",
			sql:  "SELECT 1 FROM t",
			want: "SELECT 1 FROM t",
		},
		{
			name: "line comment",
			sql:  "SELECT 1 -- comment\nFROM t",
			want: "SELECT 1           \nFROM t",
		},
		{
			name: "block comment",
			sql:  "SELECT /* comment */ 1",
			want: "SELECT               1",
		},
		{
			name: "multiline block comment",
			sql:  "SELECT /* a\nb */ 1",
			want: "SELECT     \n     1",
		},
		{
			name: "string not stripped",
			sql:  "SELECT '-- not comment'",
			want: "SELECT '-- not comment'",
		},
		{
			name: "both comment types",
			sql:  "SELECT /* block */ 1 -- line\nFROM t",
			want: "SELECT             1        \nFROM t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripComments(tt.sql)
			if got != tt.want {
				t.Errorf("StripComments():\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestStripStrings(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "no strings",
			sql:  "SELECT 1 FROM t",
			want: "SELECT 1 FROM t",
		},
		{
			name: "single string",
			sql:  "SELECT 'hello' FROM t",
			want: "SELECT   FROM t",
		},
		{
			name: "multiple strings",
			sql:  "SELECT 'a', 'b' FROM t",
			want: "SELECT  ,   FROM t",
		},
		{
			name: "escaped quotes",
			sql:  "SELECT 'it''s' FROM t",
			want: "SELECT   FROM t",
		},
		{
			name: "comment not stripped",
			sql:  "SELECT -- 'not a string'\n1",
			want: "SELECT -- 'not a string'\n1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripStrings(tt.sql)
			if got != tt.want {
				t.Errorf("StripStrings():\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestFirstToken(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"SELECT 1", "SELECT"},
		{"  select 1", "SELECT"},
		{"-- comment\nSELECT 1", "SELECT"},
		{"/* block */ INSERT INTO t", "INSERT"},
		{"  \n  ", ""},
		{"", ""},
		{"42", ""},
		{"'string'", ""},
		{"CREATE TABLE t", "CREATE"},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := FirstToken(tt.sql)
			if got != tt.want {
				t.Errorf("FirstToken(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

func TestInertRegions(t *testing.T) {
	sql := "SELECT /* block */ 'string' -- line"
	regions := InertRegions(sql)

	// Expect 3 regions: block comment, string, line comment
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions, got %d: %v", len(regions), regions)
	}

	// Verify regions cover the right spans
	bc := sql[regions[0][0]:regions[0][1]]
	if bc != "/* block */" {
		t.Errorf("region[0] = %q, want /* block */", bc)
	}

	str := sql[regions[1][0]:regions[1][1]]
	if str != "'string'" {
		t.Errorf("region[1] = %q, want 'string'", str)
	}

	lc := sql[regions[2][0]:regions[2][1]]
	if lc != "-- line" {
		t.Errorf("region[2] = %q, want -- line", lc)
	}
}

func TestIsInert(t *testing.T) {
	sql := "SELECT /* block */ 'string' -- line"
	regions := InertRegions(sql)

	tests := []struct {
		offset int
		want   bool
	}{
		{0, false},     // S
		{7, true},      // inside /* block */
		{18, false},    // space between block comment and string
		{19, true},     // inside 'string'
		{27, false},    // space after string
		{28, true},     // inside -- line
		{len(sql), false}, // past end
	}

	for _, tt := range tests {
		got := IsInert(regions, tt.offset)
		if got != tt.want {
			t.Errorf("IsInert(regions, %d) = %v, want %v", tt.offset, got, tt.want)
		}
	}
}

func TestInertRegionsDollarQuoted(t *testing.T) {
	sql := "SELECT $$body$$ FROM t"
	regions := InertRegions(sql)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	body := sql[regions[0][0]:regions[0][1]]
	if body != "$$body$$" {
		t.Errorf("region = %q, want $$body$$", body)
	}
}

func TestStripCommentsPreservesLength(t *testing.T) {
	sql := "SELECT 1 -- comment\nFROM t"
	result := StripComments(sql)
	if len(result) != len(sql) {
		t.Errorf("StripComments changed length: %d → %d", len(sql), len(result))
	}
}

func TestFirstTokenEmpty(t *testing.T) {
	if got := FirstToken(""); got != "" {
		t.Errorf("FirstToken empty: got %q", got)
	}
	if got := FirstToken("   "); got != "" {
		t.Errorf("FirstToken whitespace: got %q", got)
	}
	if got := FirstToken("-- comment only"); got != "" {
		t.Errorf("FirstToken comment only: got %q", got)
	}
}

func TestIsInertEmptyRegions(t *testing.T) {
	if IsInert(nil, 0) {
		t.Error("IsInert(nil, 0) should be false")
	}
	if IsInert([][2]int{}, 5) {
		t.Error("IsInert(empty, 5) should be false")
	}
}

func TestIsTrivia(t *testing.T) {
	trivia := []TokenKind{Whitespace, Newline, LineComment, BlockComment}
	for _, k := range trivia {
		if !k.IsTrivia() {
			t.Errorf("%v should be trivia", k)
		}
	}
	notTrivia := []TokenKind{Keyword, Identifier, QuotedIdent, StringLit, DollarQuoted, NumberLit, Operator, Dot, Comma, Semicolon, LParen, RParen, At, EOF}
	for _, k := range notTrivia {
		if k.IsTrivia() {
			t.Errorf("%v should not be trivia", k)
		}
	}
}

func TestIsIdentLike(t *testing.T) {
	identLike := []TokenKind{Identifier, QuotedIdent, Keyword}
	for _, k := range identLike {
		if !k.IsIdentLike() {
			t.Errorf("%v should be ident-like", k)
		}
	}
	notIdentLike := []TokenKind{Whitespace, Newline, LineComment, BlockComment, StringLit, DollarQuoted, NumberLit, Operator, Dot, Comma, At, EOF}
	for _, k := range notIdentLike {
		if k.IsIdentLike() {
			t.Errorf("%v should not be ident-like", k)
		}
	}
}

func TestSkipTrivia(t *testing.T) {
	sql := "  /* c */\n-- line\nFROM t"
	tokens := Tokenize(sql)
	// From index 0, the first significant token is the FROM keyword.
	i := SkipTrivia(tokens, 0)
	if i >= len(tokens) || tokens[i].Kind != Keyword || tokens[i].Text(sql) != "FROM" {
		t.Fatalf("SkipTrivia did not land on FROM; got index %d", i)
	}
	// Skipping from an already-significant token returns it unchanged.
	if got := SkipTrivia(tokens, i); got != i {
		t.Errorf("SkipTrivia on a significant token = %d; want %d", got, i)
	}
	// All-trivia input lands on the terminating EOF token.
	ws := Tokenize("   \n  ")
	j := SkipTrivia(ws, 0)
	if j >= len(ws) || ws[j].Kind != EOF {
		t.Errorf("SkipTrivia over all-trivia should reach EOF; got index %d", j)
	}
	// Out-of-range / empty inputs are safe.
	if got := SkipTrivia(nil, 0); got != 0 {
		t.Errorf("SkipTrivia(nil,0) = %d; want 0", got)
	}
}

func TestStripCommentsEmpty(t *testing.T) {
	if got := StripComments(""); got != "" {
		t.Errorf("StripComments empty: got %q", got)
	}
}

func TestStripStringsEmpty(t *testing.T) {
	if got := StripStrings(""); got != "" {
		t.Errorf("StripStrings empty: got %q", got)
	}
}

func TestInertRegionsEmpty(t *testing.T) {
	regions := InertRegions("")
	if len(regions) != 0 {
		t.Errorf("expected 0 regions, got %d", len(regions))
	}
}

func TestStripCommentsPreservesNewlines(t *testing.T) {
	sql := "SELECT 1 /* multi\nline\ncomment */ FROM t"
	result := StripComments(sql)

	// Count newlines should be preserved
	origNL := strings.Count(sql, "\n")
	resultNL := strings.Count(result, "\n")
	if origNL != resultNL {
		t.Errorf("newlines changed: %d → %d", origNL, resultNL)
	}
}
