// SPDX-License-Identifier: GPL-3.0-or-later

package ddl

import (
	"strings"
	"testing"
)

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
