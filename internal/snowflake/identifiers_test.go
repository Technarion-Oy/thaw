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
	"testing"
)

func TestTableKey(t *testing.T) {
	tests := []struct {
		schema string
		name   string
		want   string
	}{
		{"PUBLIC", "USERS", "PUBLIC.USERS"},
		{"public", "users", "public.users"},         // preserves lowercase (quoted identifiers)
		{"Public", "Orders", "Public.Orders"},        // preserves mixed case
		{" sales ", " orders ", "sales.orders"},      // trims whitespace, preserves case
		{"", "T", ".T"},
		{"S", "", "S."},
		{"PUBLIC", "my_table", "PUBLIC.my_table"},    // case-sensitive table in PUBLIC schema
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
