// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"strings"
	"testing"
)

func TestEscapeStringLit(t *testing.T) {
	// EscapeStringLit doubles quotes but leaves backslashes intact so delimiter
	// values (e.g. RECORD_DELIMITER = '\n') keep their escape sequences.
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"it's", "it''s"},
		{`\n`, `\n`}, // backslash escape sequence preserved verbatim
	}
	for _, tt := range tests {
		if got := EscapeStringLit(tt.in); got != tt.want {
			t.Errorf("EscapeStringLit(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEscapeTextLit(t *testing.T) {
	// EscapeTextLit doubles both quotes and backslashes so free-text (comments)
	// containing a literal backslash survives Snowflake's escape processing.
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"it's", "it''s"},
		{`C:\temp`, `C:\\temp`},              // lone backslash is doubled (Snowflake escape char)
		{`a\'b`, `a\\''b`},                   // backslash and quote both escaped
		{`already\\done`, `already\\\\done`}, // each backslash is doubled independently
	}
	for _, tt := range tests {
		if got := EscapeTextLit(tt.in); got != tt.want {
			t.Errorf("EscapeTextLit(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTableKey(t *testing.T) {
	tests := []struct {
		schema string
		name   string
		want   string
	}{
		{"PUBLIC", "USERS", "PUBLIC.USERS"},
		{"public", "users", "public.users"},     // preserves lowercase (quoted identifiers)
		{"Public", "Orders", "Public.Orders"},   // preserves mixed case
		{" sales ", " orders ", "sales.orders"}, // trims whitespace, preserves case
		{"", "T", ".T"},
		{"S", "", "S."},
		{"PUBLIC", "my_table", "PUBLIC.my_table"}, // case-sensitive table in PUBLIC schema
	}
	for _, tc := range tests {
		t.Run(tc.schema+"_"+tc.name, func(t *testing.T) {
			got := TableKey(tc.schema, tc.name)
			if got != tc.want {
				t.Errorf("TableKey(%q, %q) = %q, want %q", tc.schema, tc.name, got, tc.want)
			}
		})
	}
}

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// ── Plain identifiers — no quoting needed ────────────────────────────
		{"MY_TABLE", false},
		{"my_table", false},
		{"MyTable", false},
		{"_leading_underscore", false},
		{"col$dollar", false},
		{"A", false},
		{"_", false},
		{"T1", false},
		{"SCHEMA_NAME", false},

		// ── Snowflake reserved keywords — quoting required ───────────────────
		// Tested in all cases (upper / lower / mixed) to confirm case-insensitivity.
		{"TABLE", true},
		{"table", true},
		{"Table", true},
		{"SELECT", true},
		{"select", true},
		{"FROM", true},
		{"WHERE", true},
		{"JOIN", true},
		{"LEFT", true},
		{"RIGHT", true},
		{"FULL", true},
		{"INNER", true},
		{"CROSS", true},
		{"ON", true},
		{"AS", true},
		{"IN", true},
		{"IS", true},
		{"NOT", true},
		{"NULL", true},
		{"TRUE", true},
		{"FALSE", true},
		{"AND", true},
		{"OR", true},
		{"WITH", true},
		{"VIEW", true},
		{"DATABASE", true},
		{"SCHEMA", false}, // SCHEMA is not in Snowflake's reserved list
		{"ORDER", true},
		{"GROUP", true},
		{"HAVING", true},
		{"UNION", true},
		{"INTERSECT", true},
		{"MINUS", true},
		{"INSERT", true},
		{"UPDATE", true},
		{"DELETE", true},
		{"CREATE", true},
		{"ALTER", true},
		{"DROP", true},
		{"GRANT", true},
		{"REVOKE", true},
		{"SET", true},
		{"VALUES", true},
		{"CASE", true},
		{"WHEN", true},
		{"THEN", true},
		{"ELSE", true},
		{"EXISTS", true},
		{"ALL", true},
		{"ANY", true},
		{"SOME", true},
		{"DISTINCT", true},
		{"BETWEEN", true},
		{"LIKE", true},
		{"ILIKE", true},
		{"RLIKE", true},
		{"REGEXP", true},
		{"QUALIFY", true},
		{"SAMPLE", true},
		{"TABLESAMPLE", true},
		{"LATERAL", true},
		{"FOLLOWING", true},
		{"ROW", true},
		{"ROWS", true},
		{"LIMIT", true},
		{"FOR", true},
		{"BY", true},
		{"TO", true},
		{"OF", true},
		{"CAST", true},
		{"TRY_CAST", true},
		{"ACCOUNT", true},
		{"ORGANIZATION", true},
		{"GSCLUSTER", true},
		{"ISSUE", true},
		{"NATURAL", true},
		{"USING", true},
		{"START", true},
		{"CONNECT", true},
		{"CONNECTION", true},
		{"CONSTRAINT", true},
		{"CHECK", true},
		{"COLUMN", true},
		{"UNIQUE", true},
		{"TRIGGER", true},
		{"INCREMENT", true},
		{"CURRENT", true},
		{"CURRENT_DATE", true},
		{"CURRENT_TIME", true},
		{"CURRENT_TIMESTAMP", true},
		{"CURRENT_USER", true},
		{"LOCALTIME", true},
		{"LOCALTIMESTAMP", true},
		{"WHENEVER", true},

		// ── Invalid bare identifiers — quoting required ──────────────────────
		{"", true},                   // empty
		{"1starts_with_digit", true}, // leading digit
		{"has space", true},          // space
		{"has-hyphen", true},         // hyphen
		{"has.dot", true},            // dot (qualified separator)
		{"has@at", true},             // at-sign
		{"has!bang", true},           // exclamation
		{"has\"quote", true},         // double quote inside
		{"has'apostrophe", true},     // single quote
		{"has/slash", true},          // slash
		{"has\\backslash", true},     // backslash
		{"has(paren", true},          // open paren
		{"has)paren", true},          // close paren
		{"has,comma", true},          // comma
		{"has;semicolon", true},      // semicolon
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsQuoting(tc.name)
			if got != tc.want {
				t.Errorf("NeedsQuoting(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"MY_TABLE", `"MY_TABLE"`},
		{"MixedCase", `"MixedCase"`},
		{"table", `"table"`},
		{`has"quote`, `"has""quote"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := QuoteIdent(tc.name)
			if got != tc.want {
				t.Errorf("QuoteIdent(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestSplitValues(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,, c ", []string{"a", "b", "c"}},
		{"a\nb,c\n", []string{"a", "b", "c"}}, // commas and newlines both split
	}
	for _, tc := range cases {
		got := SplitValues(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("SplitValues(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitValues(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestQuoteIdentList(t *testing.T) {
	// caseSensitive=false: simple names stay bare, mixed/reserved get quoted.
	got := QuoteIdentList([]string{" ID ", "", "MixedCase"}, false)
	want := []string{"ID", "MixedCase"} // "MixedCase" is a valid bare identifier → stays bare
	if len(got) != len(want) {
		t.Fatalf("QuoteIdentList = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("QuoteIdentList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// caseSensitive=true: every entry double-quoted.
	gotCS := QuoteIdentList([]string{"a", "b"}, true)
	if gotCS[0] != `"a"` || gotCS[1] != `"b"` {
		t.Errorf("QuoteIdentList(caseSensitive) = %v, want quoted", gotCS)
	}
}

func TestSplitIdentList(t *testing.T) {
	got := SplitIdentList("a, b\nc", true)
	want := []string{`"a"`, `"b"`, `"c"`}
	if len(got) != len(want) {
		t.Fatalf("SplitIdentList = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SplitIdentList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := SplitIdentList("", false); len(got) != 0 {
		t.Errorf("SplitIdentList(\"\") = %v, want empty", got)
	}
}

func TestFormatSecondaryRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  string
	}{
		{"all literal", []string{"ALL"}, "('ALL')"},
		{"all case-insensitive", []string{"all"}, "('ALL')"},
		{"simple roles emitted bare", []string{"R1", "R2"}, "(R1, R2)"},
		{"lowercase emitted bare (Snowflake uppercases)", []string{"analyst"}, "(analyst)"},
		{"role needing quoting is double-quoted", []string{"my role"}, `("my role")`},
		{"reserved keyword is double-quoted", []string{"ORDER"}, `("ORDER")`},
		{"blank entries skipped", []string{"", "  ", "R1"}, "(R1)"},
		{"empty list", []string{}, "()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSecondaryRoles(tt.roles); got != tt.want {
				t.Errorf("FormatSecondaryRoles(%v) = %q, want %q", tt.roles, got, tt.want)
			}
		})
	}
}

func TestParseSecondaryRoles(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"sql tuple with ALL literal", "('ALL')", []string{"ALL"}},
		{"sql tuple of bare identifiers", "(R1, R2)", []string{"R1", "R2"}},
		{"sql tuple mixing bare and quoted", `(R1, "my role")`, []string{"R1", "my role"}},
		{"json-style array", `["R1","R2"]`, []string{"R1", "R2"}},
		{"comma inside a quoted identifier", `(R1, "a,b")`, []string{"R1", "a,b"}},
		{"comma inside a single-quoted entry", `(R1, 'a,b')`, []string{"R1", "a,b"}},
		{"escaped doubled double-quote", `("we""ird")`, []string{`we"ird`}},
		{"empty cell", "", nil},
		{"null cell", "null", nil},
		{"empty tuple", "()", nil},
		{"empty array", "[]", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSecondaryRoles(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseSecondaryRoles(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseSecondaryRoles(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestSecondaryRolesRoundTrip verifies FormatSecondaryRoles → ParseSecondaryRoles
// recovers the original role tokens (with ALL normalized to its canonical form),
// including names that need quoting or contain a comma / embedded quote.
func TestSecondaryRolesRoundTrip(t *testing.T) {
	cases := [][]string{
		{"ALL"},
		{"R1", "R2"},
		{"analyst"},
		{"my role"},
		{"a,b"},
		{`we"ird`},
		{"ORDER"},
	}
	for _, roles := range cases {
		got := ParseSecondaryRoles(FormatSecondaryRoles(roles))
		if len(got) != len(roles) {
			t.Fatalf("round-trip %v = %v, length mismatch", roles, got)
		}
		for i := range roles {
			want := roles[i]
			if strings.EqualFold(want, "ALL") {
				want = "ALL"
			}
			if got[i] != want {
				t.Errorf("round-trip %v: got[%d] = %q, want %q", roles, i, got[i], want)
			}
		}
	}
}
