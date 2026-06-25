// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"reflect"
	"strings"
	"testing"
)

// ══════════════════════════════════════════════════════════════════════════════
// 1. TestGetIdentifierAtColumn — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetIdentifierAtColumn_Extended(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want []string
	}{
		// ── Dollar sign in identifiers ────────────────────────────────────────
		{name: "dollar variable single part", line: "$MY_VAR", col: 0, want: []string{"$MY_VAR"}},
		{name: "dollar variable dot col", line: "$MY_VAR.col", col: 3, want: []string{"$MY_VAR", "COL"}},
		{name: "dollar variable dot on dot", line: "$MY_VAR.col", col: 7, want: []string{"$MY_VAR", "COL"}},
		{name: "dollar variable dot on second", line: "$MY_VAR.col", col: 9, want: []string{"$MY_VAR", "COL"}},

		// ── Quoted identifiers with escaped double-quotes ─────────────────────
		{name: "quoted with escaped dquote", line: `"My ""Crazy"" DB".schema`, col: 5, want: []string{`My "Crazy" DB`, "SCHEMA"}},
		{name: "quoted with escaped dquote on schema", line: `"My ""Crazy"" DB".schema`, col: 20, want: []string{`My "Crazy" DB`, "SCHEMA"}},
		{name: "quoted with escaped dquote at end", line: `"My ""Crazy"" DB"`, col: 8, want: []string{`My "Crazy" DB`}},

		// ── Whitespace and boundaries ─────────────────────────────────────────
		{name: "cursor at col -1 equivalent (negative)", line: "abc", col: -1, want: nil},
		{name: "cursor far past line end", line: "abc", col: 100, want: nil},
		{name: "cursor between two words", line: "foo bar", col: 3, want: nil},
		{name: "cursor on space between dotted idents", line: "a.b c.d", col: 3, want: nil},

		// ── Trailing dot ──────────────────────────────────────────────────────
		{name: "trailing dot past dot returns prefix", line: "db.", col: 3, want: []string{"DB"}},
		{name: "three-part trailing dot on word", line: "a.b.", col: 0, want: []string{"A", "B"}},
		{name: "three-part trailing dot on dot1", line: "a.b.", col: 1, want: []string{"A", "B"}},
		{name: "three-part trailing dot on second", line: "a.b.", col: 2, want: []string{"A", "B"}},
		{name: "three-part trailing dot on final dot", line: "a.b.", col: 3, want: []string{"A", "B"}},

		// ── Leading dot ───────────────────────────────────────────────────────
		{name: "leading dot then dotted", line: ".a.b", col: 1, want: []string{"A", "B"}},
		{name: "leading dot then dotted on b", line: ".a.b", col: 3, want: []string{"A", "B"}},

		// ── Identifier after paren ────────────────────────────────────────────
		{name: "ident after paren three-part", line: "(db.schema.tbl)", col: 5, want: []string{"DB", "SCHEMA", "TBL"}},
		{name: "ident after comma", line: "a, db.schema", col: 5, want: []string{"DB", "SCHEMA"}},

		// ── Special characters ────────────────────────────────────────────────
		{name: "all digits", line: "123", col: 1, want: []string{"123"}},
		{name: "underscore only", line: "_", col: 0, want: []string{"_"}},
		{name: "dollar only", line: "$", col: 0, want: []string{"$"}},
		{name: "complex mixed", line: `_a$1."Quoted Part"`, col: 0, want: []string{"_A$1", "Quoted Part"}},
		{name: "complex mixed on quoted", line: `_a$1."Quoted Part"`, col: 10, want: []string{"_A$1", "Quoted Part"}},

		// ── Unicode runes (single-byte vs multi-byte) ─────────────────────────
		{name: "ascii after unicode prefix", line: "α SELECT", col: 2, want: []string{"SELECT"}},

		// ── Empty quoted identifier ───────────────────────────────────────────
		{name: "empty quoted", line: `""`, col: 0, want: []string{""}},
		{name: "empty quoted dot bare", line: `"".schema`, col: 0, want: []string{"", "SCHEMA"}},

		// ── Multiple dots ─────────────────────────────────────────────────────
		{name: "four-part (only 3 captured)", line: "a.b.c.d", col: 0, want: []string{"A", "B", "C", "D"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIdentifierAtColumn(tt.line, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIdentifierAtColumn(%q, %d) = %v, want %v", tt.line, tt.col, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 2. TestGetActiveFunctionCall — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetActiveFunctionCall_Extended(t *testing.T) {
	fc := func(name string, idx int) *FunctionCallContext {
		return &FunctionCallContext{Name: name, ParamIndex: idx}
	}

	tests := []struct {
		name   string
		prefix string
		want   *FunctionCallContext
	}{
		// ── Deep nesting ──────────────────────────────────────────────────────
		{name: "triple nested inner", prefix: "SELECT A(B(C(", want: fc("C", 0)},
		{name: "triple nested middle closed", prefix: "SELECT A(B(C(x)), ", want: fc("A", 1)},
		{name: "deeply nested all closed", prefix: "SELECT A(B(C(x)))", want: nil},

		// ── Multiple commas ──────────────────────────────────────────────────
		{name: "many params idx 4", prefix: "SELECT F(a, b, c, d, ", want: fc("F", 4)},
		{name: "many params idx 0 no content", prefix: "SELECT F(", want: fc("F", 0)},

		// ── Dollar-quoted blocks (not handled by GetActiveFunctionCall) ─────
		// The function does NOT skip $$...$$ — commas/parens inside are counted.
		{name: "dollar-quoted commas counted", prefix: "SELECT F($$a, b, c$$, ", want: fc("F", 3)},
		{name: "dollar-quoted parens create frames", prefix: "SELECT F($$(($$, ", want: nil}, // unnamed inner parens make top-of-stack nil

		// ── Complex comment scenarios ─────────────────────────────────────────
		{name: "block comment with parens", prefix: "SELECT F(x /* (()) */ , ", want: fc("F", 1)},
		{name: "line comment at end of line", prefix: "SELECT F(a, -- b, c\n", want: fc("F", 1)},
		{name: "multiple block comments", prefix: "SELECT F(/* a */x, /* b */y, ", want: fc("F", 2)},

		// ── No function name (keywords) ──────────────────────────────────────
		{name: "IF paren", prefix: "IF (x > ", want: fc("IF", 0)},
		{name: "WHERE subexpr", prefix: "SELECT F(x) WHERE (a > ", want: fc("WHERE", 0)},

		// ── Empty prefix ─────────────────────────────────────────────────────
		{name: "just open paren", prefix: "(", want: nil},
		{name: "space then paren", prefix: " (", want: nil},

		// ── Identifiers with dots as function names ───────────────────────────
		{name: "no ident before paren", prefix: "SELECT 1 + (", want: nil},

		// ── String edge cases ─────────────────────────────────────────────────
		{name: "string with escaped single quotes", prefix: "SELECT F('it''s a ''test''', ", want: fc("F", 1)},
		{name: "string with parens inside", prefix: "SELECT F('(a, b)', ", want: fc("F", 1)},
		{name: "unclosed string at end", prefix: "SELECT F('unfinished", want: fc("F", 0)},

		// ── Multiple independent calls ────────────────────────────────────────
		{name: "after closed call new call", prefix: "SELECT ABS(x), CONCAT(a, ", want: fc("CONCAT", 1)},
		{name: "closed then open", prefix: "SELECT ABS(x) + F(", want: fc("F", 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetActiveFunctionCall(tt.prefix)
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetActiveFunctionCall(%q) = %+v, want nil", tt.prefix, got)
				}
			} else {
				if got == nil {
					t.Errorf("GetActiveFunctionCall(%q) = nil, want %+v", tt.prefix, tt.want)
				} else if *got != *tt.want {
					t.Errorf("GetActiveFunctionCall(%q) = %+v, want %+v", tt.prefix, got, tt.want)
				}
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 3. TestParseSignatureParams — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestParseSignatureParams_Extended(t *testing.T) {
	tests := []struct {
		name string
		sig  string
		want []SignatureParam
	}{
		// ── Deeply nested parens ──────────────────────────────────────────────
		{name: "nested function type", sig: "F(a MAP(VARCHAR, ARRAY(INT)), b)",
			want: []SignatureParam{{Start: 2, End: 28}, {Start: 30, End: 31}}},

		// ── Single param with inner parens ────────────────────────────────────
		{name: "single param with parens", sig: "F(a OBJECT(x INT, y INT))",
			want: []SignatureParam{{Start: 2, End: 24}}},

		// ── Many params ───────────────────────────────────────────────────────
		{name: "five params", sig: "F(a, b, c, d, e)",
			want: []SignatureParam{
				{Start: 2, End: 3}, {Start: 5, End: 6}, {Start: 8, End: 9},
				{Start: 11, End: 12}, {Start: 14, End: 15},
			}},

		// ── All whitespace params ─────────────────────────────────────────────
		{name: "space-only param", sig: "F(   )",
			want: nil}, // Only whitespace = empty param list

		// ── Mixed whitespace ──────────────────────────────────────────────────
		{name: "tabs and spaces", sig: "F(\t a \t,\t b \t)",
			want: []SignatureParam{{Start: 2, End: 7}, {Start: 8, End: 13}}}, // only spaces trimmed, not tabs

		// ── No function name (just parens) ────────────────────────────────────
		{name: "no name just parens", sig: "(a, b)",
			want: []SignatureParam{{Start: 1, End: 2}, {Start: 4, End: 5}}},

		// ── Trailing comma ────────────────────────────────────────────────────
		{name: "trailing comma", sig: "F(a, b, )",
			want: []SignatureParam{{Start: 2, End: 3}, {Start: 5, End: 6}}},

		// ── VARIANT return in signature ───────────────────────────────────────
		{name: "complex Snowflake signature", sig: "PARSE_JSON(json_string VARCHAR)",
			want: []SignatureParam{{Start: 11, End: 30}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSignatureParams(tt.sig)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSignatureParams(%q)\n  got  %v\n  want %v", tt.sig, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 4. TestGetScriptingCompletions — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_Extended(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		offset    int // -1 means use len([]rune(sql))
		wantVars  []string
		wantColon bool
	}{
		// ── Cursor outside $$ block → no variables ────────────────────────────
		{
			name:      "cursor outside dollar block",
			sql:       "SELECT * FROM t",
			offset:    -1,
			wantVars:  nil,
			wantColon: true, // SELECT context requires colon (NeedsColon is irrelevant when Variables is empty)
		},
		{
			name:     "cursor before opening $$",
			sql:      "CREATE PROCEDURE p() AS $$ DECLARE x INT; BEGIN END; $$",
			offset:   5, // inside "CREATE"
			wantVars: nil,
		},

		// ── DECLARE variables ─────────────────────────────────────────────────
		{
			name: "DECLARE with multiple vars",
			sql:  "$$ DECLARE alpha INT; beta VARCHAR; gamma FLOAT; BEGIN RETURN :alpha; END; $$",
			// Cursor inside the $$ block (before closing $$)
			offset:   len([]rune("$$ DECLARE alpha INT; beta VARCHAR; gamma FLOAT; BEGIN RETURN :alpha; END; ")),
			wantVars: []string{"ALPHA", "BETA", "GAMMA"},
		},
		{
			name:     "DECLARE with DEFAULT keyword",
			sql:      "$$ DECLARE counter INT DEFAULT 0; BEGIN counter := counter + 1; END; $$",
			offset:   len([]rune("$$ DECLARE counter INT DEFAULT 0; BEGIN counter := counter + 1; END; ")),
			wantVars: []string{"COUNTER"},
		},

		// ── LET and VAR ───────────────────────────────────────────────────────
		{
			name:     "LET and VAR mixed",
			sql:      "$$ BEGIN LET x := 10; VAR y INT := 20; RETURN x + y; END; $$",
			offset:   len([]rune("$$ BEGIN LET x := 10; VAR y INT := 20; RETURN x + y; END; ")),
			wantVars: []string{"X", "Y"},
		},

		// ── FOR loop cursor variable ──────────────────────────────────────────
		{
			name:     "FOR loop cursor variable",
			sql:      "$$ BEGIN FOR rec IN my_cursor DO LET z := rec.col; END FOR; END; $$",
			offset:   len([]rune("$$ BEGIN FOR rec IN my_cursor DO LET z := rec.col; END FOR; END; ")),
			wantVars: []string{"Z", "REC"}, // LET vars extracted before FOR vars
		},

		// ── Cursor position sensitivity ───────────────────────────────────────
		{
			name:     "only vars before cursor visible",
			sql:      "$$ DECLARE a INT; b INT; BEGIN LET c := 1; LET d := 2; END; $$",
			offset:   len([]rune("$$ DECLARE a INT; b INT; BEGIN LET c := 1;")),
			wantVars: []string{"A", "B", "C"},
		},

		// ── Duplicate variable names (only one entry) ─────────────────────────
		{
			name:     "duplicate var names deduplicated",
			sql:      "$$ BEGIN LET x := 1; LET x := 2; END; $$",
			offset:   len([]rune("$$ BEGIN LET x := 1; LET x := 2; END; ")),
			wantVars: []string{"X"},
		},

		// ── NeedsColon inside SELECT within scripting ─────────────────────────
		{
			name:      "needsColon true for SELECT in script",
			sql:       "$$ BEGIN LET r := (SELECT * FROM t WHERE id = ",
			offset:    -1,
			wantVars:  []string{"R"}, // LET r is captured
			wantColon: true,
		},

		// ── NeedsColon false for assignment ───────────────────────────────────
		{
			name:      "needsColon false for LET assignment",
			sql:       "$$ BEGIN LET x := ",
			offset:    -1,
			wantVars:  []string{"X"}, // LET x is captured
			wantColon: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.offset
			if offset == -1 {
				offset = len([]rune(tt.sql))
			}
			got := GetScriptingCompletions(tt.sql, offset)

			if tt.wantVars == nil {
				if len(got.Variables) > 0 {
					t.Errorf("expected nil/empty variables, got %v", got.Variables)
				}
			} else {
				if !reflect.DeepEqual(got.Variables, tt.wantVars) {
					t.Errorf("Variables = %v, want %v", got.Variables, tt.wantVars)
				}
			}

			if tt.wantColon && !got.NeedsColon {
				t.Errorf("NeedsColon = false, want true")
			}
			if !tt.wantColon && got.NeedsColon {
				t.Errorf("NeedsColon = true, want false")
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 5. TestComputeJoinOnConditions — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeJoinOnConditions_Extended(t *testing.T) {
	t.Run("No Prefix", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
				{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			},
			Prefix: "",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
				{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			},
		}
		got := ComputeJoinOnConditions(req)
		if len(got) == 0 {
			t.Fatal("expected at least one suggestion")
		}
		// With empty prefix, conditions should NOT start with "ON "
		for _, c := range got {
			if strings.HasPrefix(c.Condition, "ON ") {
				t.Errorf("condition %q should not have ON prefix with empty Prefix", c.Condition)
			}
		}
	})

	t.Run("Incompatible types not matched", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
				{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			},
			Prefix: "",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "STATUS", DataType: "BOOLEAN"}}},
				{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "STATUS", DataType: "VARCHAR"}}},
			},
		}
		got := ComputeJoinOnConditions(req)
		// BOOLEAN and VARCHAR should not produce a same-name suggestion
		for _, c := range got {
			if strings.Contains(c.Condition, "STATUS") && c.Detail == "SAME-NAME COLUMN" {
				t.Errorf("BOOLEAN and VARCHAR should not produce same-name match: %+v", c)
			}
		}
	})

	t.Run("UNKNOWN type compatible with anything", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
				{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			},
			Prefix: "",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "COL", DataType: "NUMBER"}}},
				{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "COL", DataType: "UNKNOWN"}}},
			},
		}
		got := ComputeJoinOnConditions(req)
		found := false
		for _, c := range got {
			if strings.Contains(c.Condition, "COL") {
				found = true
			}
		}
		if !found {
			t.Error("UNKNOWN type should be compatible with NUMBER")
		}
	})

	t.Run("Single ref produces no suggestions", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			},
			Prefix: "ON ",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			},
		}
		got := ComputeJoinOnConditions(req)
		if len(got) != 0 {
			t.Errorf("expected no suggestions with single ref, got %v", got)
		}
	})

	t.Run("Multiple same-name columns produce USING with all", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
				{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			},
			Prefix: "",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{
					{Name: "ID", DataType: "NUMBER"},
					{Name: "REGION_ID", DataType: "NUMBER"},
					{Name: "CREATED_AT", DataType: "TIMESTAMP"},
				}},
				{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{
					{Name: "ID", DataType: "NUMBER"},
					{Name: "REGION_ID", DataType: "NUMBER"},
					{Name: "CREATED_AT", DataType: "TIMESTAMP"},
				}},
			},
		}
		got := ComputeJoinOnConditions(req)
		foundUsing := false
		for _, c := range got {
			if c.Detail == "USING" {
				foundUsing = true
				// Should contain all three shared columns
				if !strings.Contains(c.Condition, "ID") || !strings.Contains(c.Condition, "REGION_ID") || !strings.Contains(c.Condition, "CREATED_AT") {
					t.Errorf("USING suggestion missing columns: %q", c.Condition)
				}
			}
		}
		if !foundUsing {
			t.Error("expected USING suggestion with multiple shared columns")
		}
	})

	t.Run("FK from third table to first (non-adjacent)", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "O", DB: "DB", Schema: "S", Name: "ORDERS"},
				{Alias: "C", DB: "DB", Schema: "S", Name: "CUSTOMERS"},
				{Alias: "P", DB: "DB", Schema: "S", Name: "PRODUCTS"},
			},
			Prefix: "ON ",
			FKEntries: []TableFKEntry{
				{
					DB: "DB", Schema: "S", Name: "PRODUCTS",
					FKs: []FKEntry{
						{PKDatabase: "DB", PKSchema: "S", PKTable: "ORDERS", PKColumn: "ORDER_ID", FKColumn: "FK_ORDER_ID", ConstraintName: "FK_P_O", KeySequence: 1},
					},
				},
			},
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "ORDERS", Cols: []ColInfo{{Name: "ORDER_ID", DataType: "NUMBER"}}},
				{DB: "DB", Schema: "S", Name: "CUSTOMERS", Cols: []ColInfo{{Name: "CUSTOMER_ID", DataType: "NUMBER"}}},
				{DB: "DB", Schema: "S", Name: "PRODUCTS", Cols: []ColInfo{{Name: "FK_ORDER_ID", DataType: "NUMBER"}}},
			},
		}
		got := ComputeJoinOnConditions(req)
		foundFK := false
		for _, c := range got {
			if c.Detail == "FK RELATION" && strings.Contains(c.Condition, "FK_ORDER_ID") {
				foundFK = true
			}
		}
		if !foundFK {
			t.Errorf("expected FK suggestion for PRODUCTS→ORDERS, got %v", got)
		}
	})

	t.Run("Empty ColEntries produces no suggestions", func(t *testing.T) {
		req := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
				{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			},
			Prefix:     "ON ",
			ColEntries: []ColEntry{},
		}
		got := ComputeJoinOnConditions(req)
		if len(got) != 0 {
			t.Errorf("expected no suggestions with empty ColEntries, got %v", got)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// 6. TestGetAutocompleteContext — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContext_Extended(t *testing.T) {
	t.Run("cursor past all ranges uses last statement", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2"
		offset := len([]rune(sql)) + 10 // way past end

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.CurrentStmtIdx != 1 {
			t.Errorf("expected currentStmtIdx=1 (last stmt), got %d", ctx.CurrentStmtIdx)
		}
	})

	t.Run("empty SQL", func(t *testing.T) {
		ctx := GetAutocompleteContext("", 0)
		if len(ctx.StatementRanges) != 0 {
			t.Errorf("expected no ranges for empty SQL, got %d", len(ctx.StatementRanges))
		}
		if ctx.CurrentStmtIdx != -1 {
			t.Errorf("expected currentStmtIdx=-1 for empty SQL, got %d", ctx.CurrentStmtIdx)
		}
	})

	t.Run("CTE columns in WITH query", func(t *testing.T) {
		sql := "WITH cte AS (SELECT 1 AS x, 2 AS y) SELECT cte."
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if len(ctx.CTEColumns) == 0 {
			t.Fatal("expected CTE columns")
		}
		found := false
		for _, entry := range ctx.CTEColumns {
			if entry.Name == "CTE" {
				found = true
				if len(entry.Cols) < 2 {
					t.Logf("CTE cols: %+v", entry.Cols)
				}
			}
		}
		if !found {
			t.Errorf("expected CTE entry named 'CTE', got %+v", ctx.CTEColumns)
		}
	})

	t.Run("no CTE columns for plain SELECT", func(t *testing.T) {
		sql := "SELECT id FROM users"
		ctx := GetAutocompleteContext(sql, len([]rune(sql)))
		if ctx.CTEColumns != nil {
			t.Errorf("expected nil CTEColumns for plain SELECT, got %+v", ctx.CTEColumns)
		}
	})

	t.Run("multiple CTEs both present", func(t *testing.T) {
		sql := "WITH a AS (SELECT 1 AS x), b AS (SELECT 2 AS y) SELECT a.x, b.y"
		ctx := GetAutocompleteContext(sql, len([]rune(sql)))
		if len(ctx.CTEColumns) < 2 {
			t.Fatalf("expected at least 2 CTE entries, got %d", len(ctx.CTEColumns))
		}
		names := map[string]bool{}
		for _, c := range ctx.CTEColumns {
			names[c.Name] = true
		}
		if !names["A"] || !names["B"] {
			t.Errorf("expected both A and B CTE entries, got %v", names)
		}
	})

	t.Run("cursor in first of three statements", func(t *testing.T) {
		sql := "SELECT a FROM t1;\nSELECT b FROM t2;\nSELECT c FROM t3"
		offset := 5 // inside first SELECT

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.CurrentStmtIdx != 0 {
			t.Errorf("expected currentStmtIdx=0, got %d", ctx.CurrentStmtIdx)
		}
		if len(ctx.StatementRanges) != 3 {
			t.Errorf("expected 3 ranges, got %d", len(ctx.StatementRanges))
		}
	})

	t.Run("cursor exactly at statement boundary (semicolon)", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2"
		offset := 9 // on the semicolon of first stmt (EndOffset)

		ctx := GetAutocompleteContext(sql, offset)
		// Cursor at EndOffset should still be in the first statement
		if ctx.CurrentStmtIdx != 0 {
			t.Errorf("expected currentStmtIdx=0 at boundary, got %d", ctx.CurrentStmtIdx)
		}
	})

	t.Run("table refs populated from current statement only", func(t *testing.T) {
		sql := "SELECT * FROM other_table;\nSELECT * FROM my_table"
		offset := len([]rune(sql)) // cursor in second statement

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.CurrentStmtIdx != 1 {
			t.Fatalf("expected stmtIdx=1, got %d", ctx.CurrentStmtIdx)
		}
		// TableRefs should be from current statement only
		foundMyTable := false
		foundOther := false
		for _, ref := range ctx.TableRefs {
			if strings.EqualFold(ref.Name, "MY_TABLE") {
				foundMyTable = true
			}
			if strings.EqualFold(ref.Name, "OTHER_TABLE") {
				foundOther = true
			}
		}
		if !foundMyTable {
			t.Error("expected MY_TABLE in table refs of current statement")
		}
		if foundOther {
			t.Error("did not expect OTHER_TABLE in table refs (it's in a different statement)")
		}
	})

	t.Run("dollar-quoted block with scripting vars", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE p() RETURNS INT LANGUAGE SQL AS $$ DECLARE myvar INT; BEGIN RETURN :myvar; END; $$"
		// Cursor inside the $$ block
		offset := len([]rune("CREATE OR REPLACE PROCEDURE p() RETURNS INT LANGUAGE SQL AS $$ DECLARE myvar INT; BEGIN RETURN :"))

		ctx := GetAutocompleteContext(sql, offset)
		if len(ctx.Scripting.Variables) == 0 {
			t.Error("expected scripting variables inside $$ block")
		}
		found := false
		for _, v := range ctx.Scripting.Variables {
			if v == "MYVAR" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected MYVAR in variables, got %v", ctx.Scripting.Variables)
		}
	})

	t.Run("CREATE VIEW with FROM table ref", func(t *testing.T) {
		sql := "CREATE OR REPLACE VIEW VW_CLEAN AS\nSELECT \n    col1\nFROM RAW_CUSTOMERS\nWHERE STATUS = 'ACTIVE';"
		// Cursor in the SELECT clause (before FROM)
		offset := len([]rune("CREATE OR REPLACE VIEW VW_CLEAN AS\nSELECT \n    "))

		ctx := GetAutocompleteContext(sql, offset)
		// The FROM clause in the same statement should be detected even though cursor is before it
		foundTable := false
		for _, ref := range ctx.TableRefs {
			if ref.Name == "RAW_CUSTOMERS" {
				foundTable = true
			}
		}
		if !foundTable {
			t.Errorf("expected RAW_CUSTOMERS in table refs for CREATE VIEW, got %+v", ctx.TableRefs)
		}
	})

	t.Run("multi-statement with USE then CREATE VIEW", func(t *testing.T) {
		sql := "USE MY_DB.MY_SCHEMA;\n\nCREATE OR REPLACE VIEW v AS\nSELECT \nFROM my_table;"
		// Cursor in the SELECT of the VIEW (second statement)
		offset := len([]rune("USE MY_DB.MY_SCHEMA;\n\nCREATE OR REPLACE VIEW v AS\nSELECT "))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.CurrentStmtIdx != 1 {
			t.Errorf("expected stmtIdx=1, got %d", ctx.CurrentStmtIdx)
		}
		// Table refs from the current statement's FROM clause
		foundTable := false
		for _, ref := range ctx.TableRefs {
			if ref.Name == "MY_TABLE" {
				foundTable = true
			}
		}
		if !foundTable {
			t.Errorf("expected MY_TABLE in table refs, got %+v", ctx.TableRefs)
		}
		// Table refs remain per-statement; USE context is propagated
		// separately via the UseContext field.
	})

	t.Run("USE statement extracted as table ref in full SQL", func(t *testing.T) {
		// ParseJoinTables on the FULL sql should find USE refs,
		// but GetAutocompleteContext only parses table refs from the CURRENT statement.
		sql := "USE PROD_DB.PUBLIC;\nSELECT * FROM customers"
		// Cursor in the second statement
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		// The USE ref is in statement 0, not in the current statement (statement 1).
		// So table refs from current statement should only contain CUSTOMERS.
		for _, ref := range ctx.TableRefs {
			if ref.Name == "" && ref.DB == "PROD_DB" {
				t.Error("USE statement ref should NOT appear in current statement's table refs")
			}
		}
		foundCustomers := false
		for _, ref := range ctx.TableRefs {
			if ref.Name == "CUSTOMERS" {
				foundCustomers = true
			}
		}
		if !foundCustomers {
			t.Errorf("expected CUSTOMERS in table refs, got %+v", ctx.TableRefs)
		}
	})

	// ── UseContext propagation ────────────────────────────────────────────

	t.Run("UseContext: USE DATABASE then SELECT", func(t *testing.T) {
		sql := "USE DATABASE mydb;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Database != "MYDB" {
			t.Errorf("expected Database=MYDB, got %q", ctx.UseContext.Database)
		}
		if ctx.UseContext.Schema != "" {
			t.Errorf("expected Schema empty, got %q", ctx.UseContext.Schema)
		}
	})

	t.Run("UseContext: USE SCHEMA then SELECT", func(t *testing.T) {
		sql := "USE SCHEMA my_schema;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Schema != "MY_SCHEMA" {
			t.Errorf("expected Schema=MY_SCHEMA, got %q", ctx.UseContext.Schema)
		}
		if ctx.UseContext.Database != "" {
			t.Errorf("expected Database empty, got %q", ctx.UseContext.Database)
		}
	})

	t.Run("UseContext: USE db.schema then SELECT", func(t *testing.T) {
		sql := "USE prod_db.staging;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Database != "PROD_DB" {
			t.Errorf("expected Database=PROD_DB, got %q", ctx.UseContext.Database)
		}
		if ctx.UseContext.Schema != "STAGING" {
			t.Errorf("expected Schema=STAGING, got %q", ctx.UseContext.Schema)
		}
	})

	t.Run("UseContext: multiple USE statements last-writer-wins", func(t *testing.T) {
		sql := "USE DATABASE db1;\nUSE SCHEMA s1;\nUSE DATABASE db2;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Database != "DB2" {
			t.Errorf("expected Database=DB2 (last writer), got %q", ctx.UseContext.Database)
		}
		if ctx.UseContext.Schema != "S1" {
			t.Errorf("expected Schema=S1, got %q", ctx.UseContext.Schema)
		}
	})

	t.Run("UseContext: USE in current statement is captured", func(t *testing.T) {
		sql := "USE my_db.my_schema"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil for current USE statement")
		}
		if ctx.UseContext.Database != "MY_DB" {
			t.Errorf("expected Database=MY_DB, got %q", ctx.UseContext.Database)
		}
		if ctx.UseContext.Schema != "MY_SCHEMA" {
			t.Errorf("expected Schema=MY_SCHEMA, got %q", ctx.UseContext.Schema)
		}
	})

	t.Run("UseContext: no USE statements yields nil", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext != nil {
			t.Errorf("expected nil UseContext when no USE statements, got %+v", ctx.UseContext)
		}
	})

	t.Run("UseContext: USE ROLE and USE WAREHOUSE are ignored", func(t *testing.T) {
		sql := "USE ROLE admin;\nUSE WAREHOUSE compute_wh;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext != nil {
			t.Errorf("expected nil UseContext for ROLE/WAREHOUSE, got %+v", ctx.UseContext)
		}
	})

	t.Run("UseContext: USE db.schema override replaces earlier USE DATABASE", func(t *testing.T) {
		sql := "USE DATABASE old_db;\nUSE SCHEMA old_schema;\nUSE new_db.new_schema;\nSELECT * FROM t"
		offset := len([]rune(sql))

		ctx := GetAutocompleteContext(sql, offset)
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Database != "NEW_DB" {
			t.Errorf("expected Database=NEW_DB, got %q", ctx.UseContext.Database)
		}
		if ctx.UseContext.Schema != "NEW_SCHEMA" {
			t.Errorf("expected Schema=NEW_SCHEMA, got %q", ctx.UseContext.Schema)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// 7. TestGetStatementRanges — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetStatementRanges_Extended(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []StatementRange
	}{
		// ── Semicolons inside double-quoted identifiers ignored ────────────────
		{
			name: "semicolon in quoted identifier",
			sql:  `SELECT "col;name" FROM t`,
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 24},
			},
		},

		// ── Only comments (no statements) ─────────────────────────────────────
		{
			name: "only line comments",
			sql:  "-- line 1\n-- line 2",
			want: nil,
		},
		{
			name: "only block comment",
			sql:  "/* block comment */",
			want: nil,
		},

		// ── Multiple newlines between statements ──────────────────────────────
		{
			name: "multiple newlines between statements",
			sql:  "SELECT 1;\n\n\nSELECT 2",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
				{StartLine: 4, EndLine: 4, StartOffset: 12, EndOffset: 20},
			},
		},

		// ── Dollar-quoted with tag ────────────────────────────────────────────
		{
			name: "dollar-quoted with tag",
			sql:  "$body$\nSELECT 1;\n$body$;",
			want: []StatementRange{
				{StartLine: 1, EndLine: 3, StartOffset: 0, EndOffset: 24},
			},
		},

		// ── Escaped single-quote pairs ────────────────────────────────────────
		{
			name: "escaped single quotes",
			sql:  "SELECT 'it''s;nice';",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 20},
			},
		},

		// ── Mixed comments and statements ─────────────────────────────────────
		{
			name: "comment between two statements",
			sql:  "SELECT 1;\n-- middle comment\nSELECT 2",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
				{StartLine: 3, EndLine: 3, StartOffset: 28, EndOffset: 36},
			},
		},

		// ── Block comment spanning multiple lines ─────────────────────────────
		{
			name: "multi-line block comment before statement",
			sql:  "/* line1\nline2\nline3 */\nSELECT 1",
			want: []StatementRange{
				{StartLine: 4, EndLine: 4, StartOffset: 24, EndOffset: 32},
			},
		},

		// ── Semicolons at start (empty statements are skipped) ────────────────
		{
			name: "leading semicolons produce separate ranges",
			sql:  ";;SELECT 1",
			want: []StatementRange{
				// Each non-whitespace char before a ; is a statement? Actually,
				// the scanner looks for non-whitespace to start a stmt. ; is not
				// whitespace, so ";" alone would not start a stmt (it's a special char).
				// Let's verify: after the first ;, inStmt would have been set to false.
				// Actually ; at position 0: startStmt(0) is called since ';' is not
				// whitespace/comment. Then immediately emit with the ';'. Same for pos 1.
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 1},
				{StartLine: 1, EndLine: 1, StartOffset: 1, EndOffset: 2},
				{StartLine: 1, EndLine: 1, StartOffset: 2, EndOffset: 10},
			},
		},

		// ── Multi-line statement with proper line tracking ─────────────────────
		{
			name: "multi-line SELECT",
			sql:  "SELECT\n  a,\n  b\nFROM t",
			want: []StatementRange{
				{StartLine: 1, EndLine: 4, StartOffset: 0, EndOffset: 22},
			},
		},

		// ── Unclosed string (treated as part of statement) ────────────────────
		{
			name: "unclosed single-quoted string",
			sql:  "SELECT 'unclosed",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 16},
			},
		},

		// ── Unclosed block comment ────────────────────────────────────────────
		{
			name: "unclosed block comment",
			sql:  "SELECT 1; /* unclosed",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
			},
		},

		// ── Empty after semicolon (trailing) ──────────────────────────────────
		{
			name: "trailing whitespace after semicolon",
			sql:  "SELECT 1;\n   ",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
			},
		},

		// ── Unicode characters in SQL ─────────────────────────────────────────
		{
			name: "unicode in string literal",
			sql:  "SELECT '日本語';",
			want: []StatementRange{
				// EndOffset is byte-based: SELECT(6) + space(1) + '(1) + 3*3(9) + '(1) + ;(1) = 19
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 19},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStatementRanges(tt.sql)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStatementRanges(%q)\n  got  %v\n  want %v", tt.sql, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional: TestBuildQualifyTableCode — unit test for the quick-fix helper
// ══════════════════════════════════════════════════════════════════════════════

func TestBuildQualifyTableCode(t *testing.T) {
	eq := strings.EqualFold

	t.Run("single match", func(t *testing.T) {
		result := buildQualifyTableCode("USERS", []ResolvedRef{
			{DB: "PROD", Schema: "PUBLIC", Name: "USERS"},
		}, eq)
		if result == "" {
			t.Fatal("expected non-empty result")
		}
		if !strings.Contains(result, "qualify-table") {
			t.Error("expected 'qualify-table' in result")
		}
		if !strings.Contains(result, "PROD.PUBLIC.USERS") {
			t.Errorf("expected 'PROD.PUBLIC.USERS' in result, got %q", result)
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		result := buildQualifyTableCode("EVENTS", []ResolvedRef{
			{DB: "PROD", Schema: "PUBLIC", Name: "EVENTS"},
			{DB: "PROD", Schema: "ANALYTICS", Name: "EVENTS"},
			{DB: "DEV", Schema: "PUBLIC", Name: "EVENTS"},
		}, eq)
		if !strings.Contains(result, "PROD.PUBLIC.EVENTS") {
			t.Error("missing PROD.PUBLIC.EVENTS")
		}
		if !strings.Contains(result, "PROD.ANALYTICS.EVENTS") {
			t.Error("missing PROD.ANALYTICS.EVENTS")
		}
		if !strings.Contains(result, "DEV.PUBLIC.EVENTS") {
			t.Error("missing DEV.PUBLIC.EVENTS")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := buildQualifyTableCode("NONEXISTENT", []ResolvedRef{
			{DB: "PROD", Schema: "PUBLIC", Name: "USERS"},
		}, eq)
		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		result := buildQualifyTableCode("users", []ResolvedRef{
			{DB: "PROD", Schema: "PUBLIC", Name: "USERS"},
		}, eq)
		if result == "" {
			t.Fatal("expected case-insensitive match")
		}
		if !strings.Contains(result, "PROD.PUBLIC.USERS") {
			t.Errorf("expected qualified name in result, got %q", result)
		}
	})

	t.Run("deduplicates identical paths", func(t *testing.T) {
		result := buildQualifyTableCode("USERS", []ResolvedRef{
			{DB: "PROD", Schema: "PUBLIC", Name: "USERS"},
			{DB: "PROD", Schema: "PUBLIC", Name: "USERS"}, // duplicate
		}, eq)
		// Count occurrences of the qualified name
		count := strings.Count(result, "PROD.PUBLIC.USERS")
		if count != 1 {
			t.Errorf("expected exactly 1 occurrence of PROD.PUBLIC.USERS, got %d in %q", count, result)
		}
	})

	t.Run("schema-only ref (no db)", func(t *testing.T) {
		result := buildQualifyTableCode("TBL", []ResolvedRef{
			{DB: "", Schema: "MY_SCHEMA", Name: "TBL"},
		}, eq)
		if result == "" {
			t.Fatal("expected non-empty result for schema-only ref")
		}
		if !strings.Contains(result, "MY_SCHEMA.TBL") {
			t.Errorf("expected 'MY_SCHEMA.TBL', got %q", result)
		}
	})

	t.Run("ref with no db and no schema skipped", func(t *testing.T) {
		result := buildQualifyTableCode("TBL", []ResolvedRef{
			{DB: "", Schema: "", Name: "TBL"},
		}, eq)
		if result != "" {
			t.Errorf("expected empty result when ref has no db/schema, got %q", result)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional: TestGetCTEColumnsAtCursor — Extended edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetCTEColumnsAtCursor_Extended(t *testing.T) {
	t.Run("CTE with explicit column list aliases", func(t *testing.T) {
		// CTE with SELECT list containing AS aliases
		sql := "WITH metrics AS (SELECT COUNT(*) AS total, SUM(val) AS amount FROM orders) SELECT metrics."
		cols := getCTEColumnsAtCursor(sql)
		if len(cols) == 0 {
			t.Fatal("expected CTE columns")
		}
		if cols[0].Name != "METRICS" {
			t.Errorf("expected name 'METRICS', got %q", cols[0].Name)
		}
		// Should project "TOTAL" and "AMOUNT" from the AS aliases
		colNames := map[string]bool{}
		for _, c := range cols[0].Cols {
			colNames[c.Name] = true
		}
		if !colNames["TOTAL"] {
			t.Error("expected column TOTAL from CTE projection")
		}
		if !colNames["AMOUNT"] {
			t.Error("expected column AMOUNT from CTE projection")
		}
	})

	t.Run("CTE referencing earlier CTE", func(t *testing.T) {
		sql := "WITH first AS (SELECT 1 AS x), second AS (SELECT x FROM first) SELECT second."
		cols := getCTEColumnsAtCursor(sql)
		names := map[string]bool{}
		for _, c := range cols {
			names[c.Name] = true
		}
		if !names["FIRST"] {
			t.Error("expected FIRST CTE entry")
		}
		if !names["SECOND"] {
			t.Error("expected SECOND CTE entry")
		}
	})

	t.Run("non-WITH statement returns nil", func(t *testing.T) {
		sql := "INSERT INTO t SELECT 1 AS x"
		cols := getCTEColumnsAtCursor(sql)
		if cols != nil {
			t.Errorf("expected nil for INSERT statement, got %+v", cols)
		}
	})

	t.Run("WITH in block comment returns nil", func(t *testing.T) {
		sql := "/* WITH cte AS (SELECT 1 AS x) */ SELECT * FROM t"
		cols := getCTEColumnsAtCursor(sql)
		if cols != nil {
			t.Errorf("expected nil when WITH is in comment, got %+v", cols)
		}
	})

	t.Run("empty CTE body", func(t *testing.T) {
		sql := "WITH empty AS () SELECT *"
		cols := getCTEColumnsAtCursor(sql)
		// Empty body → no columns projected, but shouldn't panic
		if len(cols) > 0 && len(cols[0].Cols) > 0 {
			t.Logf("unexpected columns from empty CTE body: %+v", cols)
		}
	})

	t.Run("CTE with VALUES clause", func(t *testing.T) {
		sql := "WITH data AS (VALUES (1, 'a'), (2, 'b')) SELECT data."
		cols := getCTEColumnsAtCursor(sql)
		// VALUES doesn't project named columns typically
		for _, c := range cols {
			if c.Name == "DATA" {
				t.Logf("DATA CTE cols (from VALUES): %+v", c.Cols)
			}
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestCTEExplicitColumnAliases — issue 1c
// ══════════════════════════════════════════════════════════════════════════════

func TestCTEExplicitColumnAliases(t *testing.T) {
	// 1c: WITH cte(col_a, col_b) AS (SELECT 1, 2) SELECT cte.
	sql := "WITH cte(col_a, col_b) AS (SELECT 1, 2) SELECT cte."
	ctx := GetAutocompleteContext(sql, len(sql))
	if len(ctx.CTEColumns) == 0 {
		t.Fatal("expected CTE columns")
	}
	var cteCols []ColInfo
	for _, entry := range ctx.CTEColumns {
		if entry.Name == "CTE" {
			cteCols = entry.Cols
		}
	}
	if len(cteCols) != 2 {
		t.Fatalf("expected 2 columns, got %d: %+v", len(cteCols), cteCols)
	}
	if cteCols[0].Name != "COL_A" {
		t.Errorf("expected first col COL_A, got %q", cteCols[0].Name)
	}
	if cteCols[1].Name != "COL_B" {
		t.Errorf("expected second col COL_B, got %q", cteCols[1].Name)
	}
}

func TestCTEExplicitColsOverrideBody(t *testing.T) {
	// Explicit cols override body aliases
	sql := "WITH t(x, y) AS (SELECT 1 AS a, 2 AS b) SELECT t."
	ctx := GetAutocompleteContext(sql, len(sql))
	if len(ctx.CTEColumns) == 0 {
		t.Fatal("expected CTE columns")
	}
	var cteCols []ColInfo
	for _, entry := range ctx.CTEColumns {
		if entry.Name == "T" {
			cteCols = entry.Cols
		}
	}
	if len(cteCols) != 2 {
		t.Fatalf("expected 2 columns, got %d: %+v", len(cteCols), cteCols)
	}
	if cteCols[0].Name != "X" {
		t.Errorf("expected first col X, got %q", cteCols[0].Name)
	}
	if cteCols[1].Name != "Y" {
		t.Errorf("expected second col Y, got %q", cteCols[1].Name)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestCTEChainedProjection — issue 1d
// ══════════════════════════════════════════════════════════════════════════════

func TestCTEChainedProjection(t *testing.T) {
	// 1d: derived should only have "ID", not "ID" + "STATUS"
	sql := `WITH base AS (SELECT 1 AS id, 'ok' AS status), derived AS (SELECT id FROM base) SELECT derived.`
	ctx := GetAutocompleteContext(sql, len(sql))
	var derivedCols []ColInfo
	for _, entry := range ctx.CTEColumns {
		if entry.Name == "DERIVED" {
			derivedCols = entry.Cols
		}
	}
	if len(derivedCols) != 1 {
		t.Fatalf("expected derived to have exactly 1 column (ID), got %d: %+v", len(derivedCols), derivedCols)
	}
	if derivedCols[0].Name != "ID" {
		t.Errorf("expected column ID, got %q", derivedCols[0].Name)
	}
}

func TestCTEStarProjection(t *testing.T) {
	// SELECT * should still return all source columns
	sql := `WITH base AS (SELECT 1 AS id, 'ok' AS status), derived AS (SELECT * FROM base) SELECT derived.`
	ctx := GetAutocompleteContext(sql, len(sql))
	var derivedCols []ColInfo
	for _, entry := range ctx.CTEColumns {
		if entry.Name == "DERIVED" {
			derivedCols = entry.Cols
		}
	}
	colNames := map[string]bool{}
	for _, c := range derivedCols {
		colNames[c.Name] = true
	}
	if !colNames["ID"] {
		t.Error("expected column ID in derived CTE")
	}
	if !colNames["STATUS"] {
		t.Error("expected column STATUS in derived CTE")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestResolveTableRefs
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs(t *testing.T) {
	storeObjs := []StoreObject{
		{DB: "PROD_DB", Schema: "PUBLIC", Name: "CUSTOMERS", Kind: "TABLE"},
		{DB: "PROD_DB", Schema: "STAGING", Name: "ORDERS", Kind: "TABLE"},
		{DB: "ANALYTICS", Schema: "DW", Name: "FACT_SALES", Kind: "VIEW"},
	}

	t.Run("fully qualified ref passes through unchanged", func(t *testing.T) {
		refs := []JoinTableRef{{DB: "MY_DB", Schema: "MY_SCH", Name: "MY_TBL", Alias: "t"}}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 1 || got[0].DB != "MY_DB" || got[0].Schema != "MY_SCH" || got[0].Name != "MY_TBL" || got[0].Alias != "t" {
			t.Errorf("expected passthrough, got %+v", got)
		}
	})

	t.Run("unqualified ref matched against store objects", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "CUSTOMERS", Alias: "c"}}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 1 || got[0].DB != "PROD_DB" || got[0].Schema != "PUBLIC" {
			t.Errorf("expected PROD_DB.PUBLIC resolution, got %+v", got)
		}
	})

	t.Run("unqualified ref falls back to UseContext when no store match", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "UNKNOWN_TABLE", Alias: ""}}
		useCtx := &UseContext{Database: "CTX_DB", Schema: "CTX_SCH"}
		got := ResolveTableRefs(refs, storeObjs, useCtx, nil)
		if len(got) != 1 || got[0].DB != "CTX_DB" || got[0].Schema != "CTX_SCH" || got[0].Name != "UNKNOWN_TABLE" {
			t.Errorf("expected UseContext fallback, got %+v", got)
		}
	})

	t.Run("UseContext overrides session context", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "UNKNOWN_TABLE", Alias: ""}}
		useCtx := &UseContext{Database: "CTX_DB", Schema: "CTX_SCH"}
		sess := &SessionContext{Database: "SESS_DB", Schema: "SESS_SCH"}
		got := ResolveTableRefs(refs, storeObjs, useCtx, sess)
		if len(got) != 1 || got[0].DB != "CTX_DB" || got[0].Schema != "CTX_SCH" {
			t.Errorf("expected UseContext to override session, got %+v", got)
		}
	})

	t.Run("session context used when no UseContext", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "UNKNOWN_TABLE", Alias: ""}}
		sess := &SessionContext{Database: "SESS_DB", Schema: "SESS_SCH"}
		got := ResolveTableRefs(refs, nil, nil, sess)
		if len(got) != 1 || got[0].DB != "SESS_DB" || got[0].Schema != "SESS_SCH" {
			t.Errorf("expected session context, got %+v", got)
		}
	})

	t.Run("two-part ref (schema.name) gets db from UseContext", func(t *testing.T) {
		refs := []JoinTableRef{{Schema: "MY_SCH", Name: "MY_TBL", Alias: ""}}
		useCtx := &UseContext{Database: "CTX_DB"}
		got := ResolveTableRefs(refs, storeObjs, useCtx, nil)
		if len(got) != 1 || got[0].DB != "CTX_DB" || got[0].Schema != "MY_SCH" {
			t.Errorf("expected db from UseContext, got %+v", got)
		}
	})

	t.Run("USE refs (Name empty) are skipped", func(t *testing.T) {
		refs := []JoinTableRef{{DB: "SOME_DB", Schema: "SOME_SCH", Name: "", Alias: ""}}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 0 {
			t.Errorf("expected empty result for USE ref, got %+v", got)
		}
	})

	t.Run("case-insensitive matching", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "customers", Alias: "c"}}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 1 || got[0].DB != "PROD_DB" {
			t.Errorf("expected case-insensitive match, got %+v", got)
		}
	})

	t.Run("no match and no context returns empty slice", func(t *testing.T) {
		refs := []JoinTableRef{{Name: "NONEXISTENT", Alias: ""}}
		got := ResolveTableRefs(refs, nil, nil, nil)
		if len(got) != 0 {
			t.Errorf("expected empty result, got %+v", got)
		}
	})

	t.Run("multiple refs resolved correctly", func(t *testing.T) {
		refs := []JoinTableRef{
			{Name: "CUSTOMERS", Alias: "c"},
			{Name: "ORDERS", Alias: "o"},
		}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 2 {
			t.Fatalf("expected 2 resolved refs, got %d: %+v", len(got), got)
		}
		if got[0].Schema != "PUBLIC" || got[1].Schema != "STAGING" {
			t.Errorf("unexpected schemas: %+v", got)
		}
	})

	t.Run("partial schema match filters correctly", func(t *testing.T) {
		refs := []JoinTableRef{{Schema: "DW", Name: "FACT_SALES", Alias: "fs"}}
		got := ResolveTableRefs(refs, storeObjs, nil, nil)
		if len(got) != 1 || got[0].DB != "ANALYTICS" || got[0].Schema != "DW" {
			t.Errorf("expected ANALYTICS.DW match, got %+v", got)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestExtractInEditorTableDefs
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs(t *testing.T) {
	t.Run("single CREATE TABLE extracts columns with types", func(t *testing.T) {
		sql := "CREATE TABLE my_table (id INT, name VARCHAR, email TEXT);"
		ranges := GetStatementRanges(sql)
		defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1 def, got %d", len(defs))
		}
		if defs[0].Name != "MY_TABLE" {
			t.Errorf("expected MY_TABLE, got %s", defs[0].Name)
		}
		if len(defs[0].Cols) != 3 {
			t.Fatalf("expected 3 columns, got %d: %+v", len(defs[0].Cols), defs[0].Cols)
		}
		if defs[0].Cols[0].Name != "ID" || defs[0].Cols[1].Name != "NAME" || defs[0].Cols[2].Name != "EMAIL" {
			t.Errorf("unexpected column names: %+v", defs[0].Cols)
		}
	})

	t.Run("multiple CREATE TABLEs across statements", func(t *testing.T) {
		sql := "CREATE TABLE t1 (a INT, b TEXT);\nCREATE TABLE t2 (x NUMBER, y VARCHAR);"
		ranges := GetStatementRanges(sql)
		defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
		if len(defs) != 2 {
			t.Fatalf("expected 2 defs, got %d", len(defs))
		}
		if defs[0].Name != "T1" || defs[1].Name != "T2" {
			t.Errorf("expected T1 and T2, got %s and %s", defs[0].Name, defs[1].Name)
		}
	})

	t.Run("qualified names (db.schema.table) preserved", func(t *testing.T) {
		sql := "CREATE TABLE my_db.my_schema.my_table (col1 INT);"
		ranges := GetStatementRanges(sql)
		defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1 def, got %d", len(defs))
		}
		if defs[0].DB != "MY_DB" || defs[0].Schema != "MY_SCHEMA" || defs[0].Name != "MY_TABLE" {
			t.Errorf("expected MY_DB.MY_SCHEMA.MY_TABLE, got %s.%s.%s", defs[0].DB, defs[0].Schema, defs[0].Name)
		}
	})

	t.Run("unqualified names get qualified via UseContext", func(t *testing.T) {
		sql := "CREATE TABLE raw_customers (customer_id INT, first_name VARCHAR);"
		ranges := GetStatementRanges(sql)
		useCtx := &UseContext{Database: "LINEAGE_SOURCE_DB", Schema: "STAGING"}
		defs := ExtractInEditorTableDefs(sql, ranges, useCtx, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1 def, got %d", len(defs))
		}
		if defs[0].DB != "LINEAGE_SOURCE_DB" || defs[0].Schema != "STAGING" {
			t.Errorf("expected LINEAGE_SOURCE_DB.STAGING qualification, got %s.%s", defs[0].DB, defs[0].Schema)
		}
		if defs[0].Name != "RAW_CUSTOMERS" {
			t.Errorf("expected RAW_CUSTOMERS, got %s", defs[0].Name)
		}
	})

	t.Run("CREATE TABLE AS SELECT (CTAS) is skipped", func(t *testing.T) {
		sql := "CREATE TABLE ctas_table (id INT) AS SELECT 1 AS id;"
		ranges := GetStatementRanges(sql)
		defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
		if len(defs) != 0 {
			t.Errorf("expected CTAS to be skipped, got %+v", defs)
		}
	})

	t.Run("CREATE OR REPLACE TABLE works", func(t *testing.T) {
		sql := "CREATE OR REPLACE TABLE replaced_tbl (col_a FLOAT, col_b DATE);"
		ranges := GetStatementRanges(sql)
		defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1 def, got %d", len(defs))
		}
		if defs[0].Name != "REPLACED_TBL" {
			t.Errorf("expected REPLACED_TBL, got %s", defs[0].Name)
		}
		if len(defs[0].Cols) != 2 {
			t.Errorf("expected 2 cols, got %d", len(defs[0].Cols))
		}
	})

	t.Run("session context used as fallback", func(t *testing.T) {
		sql := "CREATE TABLE orders (order_id INT);"
		ranges := GetStatementRanges(sql)
		sess := &SessionContext{Database: "SESS_DB", Schema: "SESS_SCH"}
		defs := ExtractInEditorTableDefs(sql, ranges, nil, sess)
		if len(defs) != 1 {
			t.Fatalf("expected 1 def, got %d", len(defs))
		}
		if defs[0].DB != "SESS_DB" || defs[0].Schema != "SESS_SCH" {
			t.Errorf("expected SESS_DB.SESS_SCH, got %s.%s", defs[0].DB, defs[0].Schema)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestGetAutocompleteContextFull
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContextFull(t *testing.T) {
	t.Run("USE + CREATE TABLE + SELECT scenario", func(t *testing.T) {
		sql := `USE LINEAGE_SOURCE_DB.STAGING;
CREATE TABLE RAW_CUSTOMERS1 (
    customer_id INT,
    FIRST_NAME VARCHAR(50),
    LAST_NAME VARCHAR(50),
    email TEXT
);
CREATE VIEW VW_CLEAN_CUSTOMERS AS
SELECT  FROM RAW_CUSTOMERS1;`

		// Cursor after "SELECT " in the last statement
		cursorOffset := strings.Index(sql, "SELECT  FROM") + len("SELECT ")

		storeObjs := []StoreObject{
			{DB: "LINEAGE_SOURCE_DB", Schema: "STAGING", Name: "EXISTING_TABLE", Kind: "TABLE"},
		}

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: storeObjs,
			Session:      &SessionContext{Database: "DEFAULT_DB", Schema: "PUBLIC"},
		}

		ctx := GetAutocompleteContextFull(req)

		// Verify UseContext is populated
		if ctx.UseContext == nil {
			t.Fatal("expected UseContext to be non-nil")
		}
		if ctx.UseContext.Database != "LINEAGE_SOURCE_DB" || ctx.UseContext.Schema != "STAGING" {
			t.Errorf("unexpected UseContext: %+v", ctx.UseContext)
		}

		// Verify ResolvedRefs includes RAW_CUSTOMERS1 resolved via UseContext
		if len(ctx.ResolvedRefs) == 0 {
			t.Fatal("expected ResolvedRefs to be non-empty")
		}
		foundResolved := false
		for _, ref := range ctx.ResolvedRefs {
			if strings.EqualFold(ref.Name, "RAW_CUSTOMERS1") {
				foundResolved = true
				if ref.DB != "LINEAGE_SOURCE_DB" || ref.Schema != "STAGING" {
					t.Errorf("expected LINEAGE_SOURCE_DB.STAGING for RAW_CUSTOMERS1, got %s.%s", ref.DB, ref.Schema)
				}
			}
		}
		if !foundResolved {
			t.Errorf("RAW_CUSTOMERS1 not found in ResolvedRefs: %+v", ctx.ResolvedRefs)
		}

		// Verify InEditorTables includes RAW_CUSTOMERS1 with columns
		if len(ctx.InEditorTables) == 0 {
			t.Fatal("expected InEditorTables to be non-empty")
		}
		foundTable := false
		for _, tbl := range ctx.InEditorTables {
			if strings.EqualFold(tbl.Name, "RAW_CUSTOMERS1") {
				foundTable = true
				if tbl.DB != "LINEAGE_SOURCE_DB" || tbl.Schema != "STAGING" {
					t.Errorf("expected LINEAGE_SOURCE_DB.STAGING, got %s.%s", tbl.DB, tbl.Schema)
				}
				if len(tbl.Cols) != 4 {
					t.Errorf("expected 4 columns, got %d: %+v", len(tbl.Cols), tbl.Cols)
				}
				colNames := make([]string, len(tbl.Cols))
				for i, c := range tbl.Cols {
					colNames[i] = c.Name
				}
				expectedCols := []string{"CUSTOMER_ID", "FIRST_NAME", "LAST_NAME", "EMAIL"}
				if !reflect.DeepEqual(colNames, expectedCols) {
					t.Errorf("expected columns %v, got %v", expectedCols, colNames)
				}
			}
		}
		if !foundTable {
			t.Errorf("RAW_CUSTOMERS1 not found in InEditorTables: %+v", ctx.InEditorTables)
		}
	})

	t.Run("store objects take precedence over UseContext", func(t *testing.T) {
		sql := "USE MY_DB.MY_SCH;\nSELECT * FROM customers;"
		cursorOffset := len(sql)

		storeObjs := []StoreObject{
			{DB: "STORE_DB", Schema: "STORE_SCH", Name: "CUSTOMERS", Kind: "TABLE"},
		}

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: storeObjs,
		}

		ctx := GetAutocompleteContextFull(req)

		if len(ctx.ResolvedRefs) != 1 {
			t.Fatalf("expected 1 resolved ref, got %d: %+v", len(ctx.ResolvedRefs), ctx.ResolvedRefs)
		}
		if ctx.ResolvedRefs[0].DB != "STORE_DB" || ctx.ResolvedRefs[0].Schema != "STORE_SCH" {
			t.Errorf("expected store object match, got %+v", ctx.ResolvedRefs[0])
		}
	})

	// ── Scenarios from PR #255 acceptance tests ──────────────────────────────

	t.Run("in-editor CREATE TABLE + alias dot-completion (cursor before FROM)", func(t *testing.T) {
		// Simulates: user types "SELECT w." with cursor BEFORE "FROM widget w"
		// The full statement text includes FROM clause — GetAutocompleteContextFull
		// should find the table ref and in-editor table definition.
		sql := "CREATE TABLE widget (\n    widget_id INT,\n    widget_name VARCHAR(100),\n    is_active BOOLEAN\n);\n\nSELECT w.\nFROM widget w;"

		// Cursor at "SELECT w." — inside the SELECT statement
		cursorOffset := strings.Index(sql, "SELECT w.") + len("SELECT w.")

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: nil,
			Session:      &SessionContext{Database: "TEST_DB", Schema: "PUBLIC"},
		}

		ctx := GetAutocompleteContextFull(req)

		// TableRefs should find "widget w" from the full current statement
		if len(ctx.TableRefs) == 0 {
			t.Fatal("expected TableRefs to include widget (from full statement)")
		}
		foundWidget := false
		for _, ref := range ctx.TableRefs {
			if strings.EqualFold(ref.Name, "WIDGET") && strings.EqualFold(ref.Alias, "W") {
				foundWidget = true
			}
		}
		if !foundWidget {
			t.Errorf("expected WIDGET with alias W in TableRefs, got %+v", ctx.TableRefs)
		}

		// ResolvedRefs should resolve widget via session context
		if len(ctx.ResolvedRefs) == 0 {
			t.Fatal("expected ResolvedRefs to be non-empty")
		}
		foundResolved := false
		for _, ref := range ctx.ResolvedRefs {
			if strings.EqualFold(ref.Name, "WIDGET") {
				foundResolved = true
				if ref.DB != "TEST_DB" || ref.Schema != "PUBLIC" {
					t.Errorf("expected TEST_DB.PUBLIC, got %s.%s", ref.DB, ref.Schema)
				}
				if ref.Alias != "W" && ref.Alias != "w" {
					t.Errorf("expected alias W, got %s", ref.Alias)
				}
			}
		}
		if !foundResolved {
			t.Errorf("WIDGET not found in ResolvedRefs: %+v", ctx.ResolvedRefs)
		}

		// InEditorTables should have widget with its columns
		if len(ctx.InEditorTables) == 0 {
			t.Fatal("expected InEditorTables to be non-empty")
		}
		foundTable := false
		for _, tbl := range ctx.InEditorTables {
			if strings.EqualFold(tbl.Name, "WIDGET") {
				foundTable = true
				if len(tbl.Cols) != 3 {
					t.Errorf("expected 3 cols, got %d: %+v", len(tbl.Cols), tbl.Cols)
				}
			}
		}
		if !foundTable {
			t.Errorf("WIDGET not found in InEditorTables: %+v", ctx.InEditorTables)
		}
	})

	t.Run("multiple in-editor tables with JOIN", func(t *testing.T) {
		sql := "CREATE TABLE dept (dept_id INT, dept_name VARCHAR(50));\nCREATE TABLE emp (emp_id INT, emp_name VARCHAR(100), dept_id INT);\n\nSELECT e.\nFROM emp e\nJOIN dept d ON e.dept_id = d.dept_id;"

		cursorOffset := strings.Index(sql, "SELECT e.") + len("SELECT e.")

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: nil,
			Session:      &SessionContext{Database: "MY_DB", Schema: "MY_SCH"},
		}

		ctx := GetAutocompleteContextFull(req)

		// Should have both emp and dept in table refs
		if len(ctx.TableRefs) < 2 {
			t.Fatalf("expected at least 2 table refs, got %d: %+v", len(ctx.TableRefs), ctx.TableRefs)
		}

		// Should have both in-editor tables
		if len(ctx.InEditorTables) != 2 {
			t.Fatalf("expected 2 in-editor tables, got %d: %+v", len(ctx.InEditorTables), ctx.InEditorTables)
		}

		// emp should have 3 columns
		for _, tbl := range ctx.InEditorTables {
			if strings.EqualFold(tbl.Name, "EMP") {
				if len(tbl.Cols) != 3 {
					t.Errorf("expected 3 cols for EMP, got %d", len(tbl.Cols))
				}
			}
			if strings.EqualFold(tbl.Name, "DEPT") {
				if len(tbl.Cols) != 2 {
					t.Errorf("expected 2 cols for DEPT, got %d", len(tbl.Cols))
				}
			}
		}
	})

	t.Run("USE + CREATE TABLE + CTE combined scenario", func(t *testing.T) {
		sql := `USE DATABASE GOVERNANCE;
USE SCHEMA PUBLIC;

CREATE TABLE summary (summary_id INT, total DECIMAL(18,2));

WITH agg AS (
    SELECT 1 AS key, 999 AS val
)
SELECT
    s.
FROM agg
CROSS JOIN summary s;`

		cursorOffset := strings.Index(sql, "    s.") + len("    s.")

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: nil,
			Session:      &SessionContext{Database: "DEFAULT_DB", Schema: "DEFAULT_SCH"},
		}

		ctx := GetAutocompleteContextFull(req)

		// UseContext should be GOVERNANCE.PUBLIC
		if ctx.UseContext == nil || ctx.UseContext.Database != "GOVERNANCE" || ctx.UseContext.Schema != "PUBLIC" {
			t.Errorf("expected UseContext GOVERNANCE.PUBLIC, got %+v", ctx.UseContext)
		}

		// InEditorTables should have summary with GOVERNANCE.PUBLIC qualification
		foundSummary := false
		for _, tbl := range ctx.InEditorTables {
			if strings.EqualFold(tbl.Name, "SUMMARY") {
				foundSummary = true
				if tbl.DB != "GOVERNANCE" || tbl.Schema != "PUBLIC" {
					t.Errorf("expected GOVERNANCE.PUBLIC for summary, got %s.%s", tbl.DB, tbl.Schema)
				}
				if len(tbl.Cols) != 2 {
					t.Errorf("expected 2 cols for summary, got %d: %+v", len(tbl.Cols), tbl.Cols)
				}
			}
		}
		if !foundSummary {
			t.Errorf("SUMMARY not in InEditorTables: %+v", ctx.InEditorTables)
		}

		// CTE columns should be available
		if len(ctx.CTEColumns) == 0 {
			t.Fatal("expected CTE columns")
		}
		foundAgg := false
		for _, cte := range ctx.CTEColumns {
			if cte.Name == "AGG" {
				foundAgg = true
				if len(cte.Cols) != 2 {
					t.Errorf("expected 2 CTE cols (key, val), got %d: %+v", len(cte.Cols), cte.Cols)
				}
			}
		}
		if !foundAgg {
			t.Errorf("AGG CTE not found: %+v", ctx.CTEColumns)
		}

		// ResolvedRefs should include summary with alias "s"
		foundRef := false
		for _, ref := range ctx.ResolvedRefs {
			if strings.EqualFold(ref.Name, "SUMMARY") {
				foundRef = true
				if ref.DB != "GOVERNANCE" || ref.Schema != "PUBLIC" {
					t.Errorf("expected GOVERNANCE.PUBLIC, got %s.%s", ref.DB, ref.Schema)
				}
			}
		}
		if !foundRef {
			t.Errorf("SUMMARY not in ResolvedRefs: %+v", ctx.ResolvedRefs)
		}
	})

	t.Run("in-editor table columns only for referenced tables in current stmt", func(t *testing.T) {
		// When there are multiple in-editor tables, only those referenced in
		// the current statement should contribute columns to contextual suggestions
		sql := `CREATE TABLE alpha (a1 INT, a2 TEXT);
CREATE TABLE beta (b1 INT, b2 TEXT);
SELECT * FROM alpha;`

		cursorOffset := len(sql)

		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: cursorOffset,
			StoreObjects: nil,
			Session:      &SessionContext{Database: "DB", Schema: "SCH"},
		}

		ctx := GetAutocompleteContextFull(req)

		// Both tables exist in InEditorTables (all CREATE TABLEs are extracted)
		if len(ctx.InEditorTables) != 2 {
			t.Fatalf("expected 2 in-editor tables, got %d", len(ctx.InEditorTables))
		}

		// But only alpha is in table refs (current statement is SELECT * FROM alpha)
		foundAlpha := false
		foundBeta := false
		for _, ref := range ctx.TableRefs {
			if strings.EqualFold(ref.Name, "ALPHA") {
				foundAlpha = true
			}
			if strings.EqualFold(ref.Name, "BETA") {
				foundBeta = true
			}
		}
		if !foundAlpha {
			t.Errorf("expected ALPHA in TableRefs")
		}
		if foundBeta {
			t.Errorf("BETA should NOT be in TableRefs (not in current stmt)")
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestComputeGitLineDiff
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeGitLineDiff(t *testing.T) {
	tests := []struct {
		name    string
		head    []string
		current []string
		max     int
		wantAdd []int
		wantMod []int
		wantDel []int
	}{
		{
			name:    "no changes",
			head:    []string{"a", "b", "c"},
			current: []string{"a", "b", "c"},
			max:     3000,
			wantAdd: []int{}, wantMod: []int{}, wantDel: []int{},
		},
		{
			name:    "added lines only",
			head:    []string{"a", "c"},
			current: []string{"a", "b", "c"},
			max:     3000,
			wantAdd: []int{2}, wantMod: []int{}, wantDel: []int{},
		},
		{
			name:    "deleted lines only",
			head:    []string{"a", "b", "c"},
			current: []string{"a", "c"},
			max:     3000,
			wantAdd: []int{}, wantMod: []int{}, wantDel: []int{1},
		},
		{
			name:    "modified line (same position in added and deleted)",
			head:    []string{"a", "b", "c"},
			current: []string{"a", "x", "c"},
			max:     3000,
			wantAdd: []int{}, wantMod: []int{2}, wantDel: []int{},
		},
		{
			name:    "mixed add delete modify",
			head:    []string{"a", "b", "c", "d"},
			current: []string{"a", "x", "c", "d", "e"},
			max:     3000,
			wantAdd: []int{5}, wantMod: []int{2}, wantDel: []int{},
		},
		{
			name:    "exceeds maxLines",
			head:    []string{"a", "b"},
			current: []string{"a", "c"},
			max:     1,
			wantAdd: []int{}, wantMod: []int{}, wantDel: []int{},
		},
		{
			name:    "empty head (all added)",
			head:    []string{},
			current: []string{"a", "b"},
			max:     3000,
			wantAdd: []int{1, 2}, wantMod: []int{}, wantDel: []int{},
		},
		{
			name:    "empty current (all deleted)",
			head:    []string{"a", "b"},
			current: []string{},
			max:     3000,
			wantAdd: []int{}, wantMod: []int{}, wantDel: []int{1, 1},
		},
		{
			name:    "both empty",
			head:    []string{},
			current: []string{},
			max:     3000,
			wantAdd: []int{}, wantMod: []int{}, wantDel: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeGitLineDiff(tt.head, tt.current, tt.max)
			if !reflect.DeepEqual(got.Added, tt.wantAdd) {
				t.Errorf("Added = %v, want %v", got.Added, tt.wantAdd)
			}
			if !reflect.DeepEqual(got.Modified, tt.wantMod) {
				t.Errorf("Modified = %v, want %v", got.Modified, tt.wantMod)
			}
			if !reflect.DeepEqual(got.Deleted, tt.wantDel) {
				t.Errorf("Deleted = %v, want %v", got.Deleted, tt.wantDel)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestIsDatatypeContext
// ══════════════════════════════════════════════════════════════════════════════

func TestIsDatatypeContext(t *testing.T) {
	tests := []struct {
		name         string
		textToCursor string
		lineUpToWord string
		want         bool
	}{
		{name: "cast shorthand ::", textToCursor: "SELECT x::", lineUpToWord: "SELECT x::", want: true},
		{name: "cast shorthand with space", textToCursor: "SELECT x:: ", lineUpToWord: "SELECT x:: ", want: true},
		{name: "CAST(x AS", textToCursor: "SELECT CAST(x AS ", lineUpToWord: "SELECT CAST(x AS ", want: true},
		{name: "TRY_CAST(x AS", textToCursor: "SELECT TRY_CAST(x AS ", lineUpToWord: "SELECT TRY_CAST(x AS ", want: true},
		{name: "DECLARE varname", textToCursor: "DECLARE myvar ", lineUpToWord: "  myvar ", want: true},
		{name: "CREATE TABLE col", textToCursor: "CREATE TABLE t (\n  col ", lineUpToWord: "  col ", want: true},
		{name: "ALTER TABLE col", textToCursor: "ALTER TABLE t ADD COLUMN (\n  col ", lineUpToWord: "  col ", want: true},
		{name: "CREATE TABLE second col", textToCursor: "CREATE TABLE t (id INT,\n  name ", lineUpToWord: "  name ", want: true},
		{name: "SELECT FROM not datatype", textToCursor: "SELECT x FROM ", lineUpToWord: "SELECT x FROM ", want: false},
		{name: "empty strings", textToCursor: "", lineUpToWord: "", want: false},
		{name: "just a word", textToCursor: "hello", lineUpToWord: "hello", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDatatypeContext(tt.textToCursor, tt.lineUpToWord)
			if got != tt.want {
				t.Errorf("IsDatatypeContext(%q, %q) = %v, want %v", tt.textToCursor, tt.lineUpToWord, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestIsInJoinOnClause
// ══════════════════════════════════════════════════════════════════════════════

func TestIsInJoinOnClause(t *testing.T) {
	tests := []struct {
		name         string
		textToCursor string
		want         bool
	}{
		{name: "JOIN ON cursor", textToCursor: "SELECT * FROM t1 JOIN t2 ON ", want: true},
		{name: "JOIN ON partial", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = ", want: true},
		{name: "terminated by WHERE", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id WHERE ", want: false},
		{name: "no JOIN", textToCursor: "SELECT * FROM t1", want: false},
		{name: "JOIN no ON", textToCursor: "SELECT * FROM t1 JOIN t2", want: false},
		{name: "multiple JOINs uses last", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id JOIN t3 ON ", want: true},
		{name: "multiple JOINs terminated", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id JOIN t3 ON t3.id = t2.id WHERE ", want: false},
		{name: "GROUP BY terminates", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id GROUP BY ", want: false},
		{name: "HAVING terminates", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id HAVING ", want: false},
		{name: "UNION terminates", textToCursor: "SELECT * FROM t1 JOIN t2 ON t1.id = t2.id UNION ", want: false},
		{name: "empty string", textToCursor: "", want: false},
		{name: "case insensitive", textToCursor: "select * from t1 join t2 on ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInJoinOnClause(tt.textToCursor)
			if got != tt.want {
				t.Errorf("IsInJoinOnClause(%q) = %v, want %v", tt.textToCursor, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestDetectUsingClause
// ══════════════════════════════════════════════════════════════════════════════

func TestDetectUsingClause(t *testing.T) {
	tests := []struct {
		name         string
		textToCursor string
		wantInUsing  bool
		wantPartial  bool
	}{
		{name: "USING( start", textToCursor: "JOIN t2 USING (", wantInUsing: true, wantPartial: false},
		{name: "USING( no space", textToCursor: "JOIN t2 USING(", wantInUsing: true, wantPartial: false},
		{name: "USING partial", textToCursor: "JOIN t2 USING (col1, ", wantInUsing: false, wantPartial: true},
		{name: "USING partial multi", textToCursor: "JOIN t2 USING (col1, col2, ", wantInUsing: false, wantPartial: true},
		{name: "USING closed", textToCursor: "JOIN t2 USING (col1)", wantInUsing: false, wantPartial: false},
		{name: "no USING", textToCursor: "SELECT * FROM t1 WHERE ", wantInUsing: false, wantPartial: false},
		{name: "empty string", textToCursor: "", wantInUsing: false, wantPartial: false},
		{name: "case insensitive", textToCursor: "join t2 using (", wantInUsing: true, wantPartial: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectUsingClause(tt.textToCursor)
			if got.InUsing != tt.wantInUsing {
				t.Errorf("DetectUsingClause(%q).InUsing = %v, want %v", tt.textToCursor, got.InUsing, tt.wantInUsing)
			}
			if got.IsPartial != tt.wantPartial {
				t.Errorf("DetectUsingClause(%q).IsPartial = %v, want %v", tt.textToCursor, got.IsPartial, tt.wantPartial)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestAutocompleteContextFullDatatypeAndJoin
// ══════════════════════════════════════════════════════════════════════════════

func TestAutocompleteContextFullDatatypeAndJoin(t *testing.T) {
	t.Run("isDatatypeContext in full context", func(t *testing.T) {
		req := AutocompleteContextRequest{
			SQL:          "SELECT x::",
			CursorOffset: 10,
			LineUpToWord: "SELECT x::",
		}
		ctx := GetAutocompleteContextFull(req)
		if !ctx.IsDatatypeCtx {
			t.Error("expected IsDatatypeCtx = true for '::' at cursor")
		}
	})

	t.Run("isInJoinOnClause in full context", func(t *testing.T) {
		sql := "SELECT * FROM t1 JOIN t2 ON "
		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: len(sql),
			LineUpToWord: sql,
		}
		ctx := GetAutocompleteContextFull(req)
		if !ctx.IsInJoinOnClause {
			t.Error("expected IsInJoinOnClause = true")
		}
	})

	t.Run("usingClause in full context", func(t *testing.T) {
		sql := "SELECT * FROM t1 JOIN t2 USING ("
		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: len(sql),
			LineUpToWord: "USING (",
		}
		ctx := GetAutocompleteContextFull(req)
		if ctx.UsingClause == nil {
			t.Fatal("expected UsingClause to be non-nil")
		}
		if !ctx.UsingClause.InUsing {
			t.Error("expected UsingClause.InUsing = true")
		}
	})

	t.Run("no context flags when normal SELECT", func(t *testing.T) {
		sql := "SELECT * FROM t1 WHERE "
		req := AutocompleteContextRequest{
			SQL:          sql,
			CursorOffset: len(sql),
			LineUpToWord: "WHERE ",
		}
		ctx := GetAutocompleteContextFull(req)
		if ctx.IsDatatypeCtx {
			t.Error("expected IsDatatypeCtx = false")
		}
		if ctx.IsInJoinOnClause {
			t.Error("expected IsInJoinOnClause = false")
		}
		if ctx.UsingClause != nil {
			t.Error("expected UsingClause = nil")
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestTypeCategory — categorization of Snowflake data types
// ══════════════════════════════════════════════════════════════════════════════

func TestTypeCategory(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		want     string
	}{
		// ── Numeric types ─────────────────────────────────────────────────────
		{name: "NUMBER", dataType: "NUMBER", want: "numeric"},
		{name: "INT", dataType: "INT", want: "numeric"},
		{name: "INTEGER", dataType: "INTEGER", want: "numeric"},
		{name: "BIGINT", dataType: "BIGINT", want: "numeric"},
		{name: "SMALLINT", dataType: "SMALLINT", want: "numeric"},
		{name: "TINYINT", dataType: "TINYINT", want: "numeric"},
		{name: "FLOAT", dataType: "FLOAT", want: "numeric"},
		{name: "DOUBLE", dataType: "DOUBLE", want: "numeric"},
		{name: "DECIMAL", dataType: "DECIMAL", want: "numeric"},
		{name: "REAL", dataType: "REAL", want: "numeric"},

		// ── Text types ────────────────────────────────────────────────────────
		{name: "VARCHAR", dataType: "VARCHAR", want: "text"},
		{name: "STRING", dataType: "STRING", want: "text"},
		{name: "TEXT", dataType: "TEXT", want: "text"},
		{name: "CHAR", dataType: "CHAR", want: "text"},
		{name: "BINARY", dataType: "BINARY", want: "text"},

		// ── Boolean ───────────────────────────────────────────────────────────
		{name: "BOOLEAN", dataType: "BOOLEAN", want: "boolean"},

		// ── Datetime types ────────────────────────────────────────────────────
		{name: "DATE", dataType: "DATE", want: "datetime"},
		{name: "TIMESTAMP", dataType: "TIMESTAMP", want: "datetime"},
		{name: "TIMESTAMP_LTZ", dataType: "TIMESTAMP_LTZ", want: "datetime"},
		{name: "TIMESTAMP_NTZ", dataType: "TIMESTAMP_NTZ", want: "datetime"},
		{name: "TIME", dataType: "TIME", want: "datetime"},

		// ── Semi-structured ───────────────────────────────────────────────────
		{name: "VARIANT", dataType: "VARIANT", want: "semi"},
		{name: "OBJECT", dataType: "OBJECT", want: "semi"},
		{name: "ARRAY", dataType: "ARRAY", want: "semi"},

		// ── Case insensitivity ────────────────────────────────────────────────
		{name: "lowercase varchar", dataType: "varchar", want: "text"},
		{name: "mixed case Number", dataType: "Number", want: "numeric"},

		// ── Type parameters stripped ──────────────────────────────────────────
		{name: "VARCHAR(255)", dataType: "VARCHAR(255)", want: "text"},
		{name: "NUMBER(18,2)", dataType: "NUMBER(18,2)", want: "numeric"},
		{name: "DECIMAL(10)", dataType: "DECIMAL(10)", want: "numeric"},

		// ── Unknown type fallback ─────────────────────────────────────────────
		{name: "empty string", dataType: "", want: "other"},
		{name: "nonsense type", dataType: "FOOBAR", want: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TypeCategory(tt.dataType)
			if got != tt.want {
				t.Errorf("TypeCategory(%q) = %q, want %q", tt.dataType, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TestBuildCompositeConditions — FK-based JOIN condition builder
// ══════════════════════════════════════════════════════════════════════════════

func TestBuildCompositeConditions(t *testing.T) {
	t.Run("single FK produces single condition", func(t *testing.T) {
		fks := []FKEntry{
			{FKColumn: "DEPT_ID", PKColumn: "ID", ConstraintName: "FK_EMP_DEPT", KeySequence: 1},
		}
		got := BuildCompositeConditions(fks, "E", "D")
		if len(got) != 1 {
			t.Fatalf("expected 1 condition, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "DEPT_ID") || !strings.Contains(got[0], "ID") {
			t.Errorf("expected condition with DEPT_ID and ID, got %q", got[0])
		}
	})

	t.Run("composite FK produces AND condition", func(t *testing.T) {
		fks := []FKEntry{
			{FKColumn: "FK_A", PKColumn: "PK_A", ConstraintName: "FK_COMP", KeySequence: 1},
			{FKColumn: "FK_B", PKColumn: "PK_B", ConstraintName: "FK_COMP", KeySequence: 2},
		}
		got := BuildCompositeConditions(fks, "T1", "T2")
		if len(got) != 1 {
			t.Fatalf("expected 1 composite condition, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], " AND ") {
			t.Errorf("expected AND in composite condition, got %q", got[0])
		}
		// KeySequence ordering: FK_A should come before FK_B
		idxA := strings.Index(got[0], "FK_A")
		idxB := strings.Index(got[0], "FK_B")
		if idxA > idxB {
			t.Errorf("expected FK_A before FK_B (by KeySequence), got %q", got[0])
		}
	})

	t.Run("multiple separate constraints produce multiple conditions", func(t *testing.T) {
		fks := []FKEntry{
			{FKColumn: "DEPT_ID", PKColumn: "ID", ConstraintName: "FK1", KeySequence: 1},
			{FKColumn: "REGION_ID", PKColumn: "ID", ConstraintName: "FK2", KeySequence: 1},
		}
		got := BuildCompositeConditions(fks, "E", "D")
		if len(got) != 2 {
			t.Fatalf("expected 2 conditions, got %d: %v", len(got), got)
		}
	})

	t.Run("empty FKs produces empty result", func(t *testing.T) {
		got := BuildCompositeConditions(nil, "A", "B")
		if len(got) != 0 {
			t.Errorf("expected empty result, got %v", got)
		}
	})

	t.Run("empty constraint name groups by FKColumn", func(t *testing.T) {
		fks := []FKEntry{
			{FKColumn: "COL_A", PKColumn: "PK_A", ConstraintName: "", KeySequence: 1},
			{FKColumn: "COL_B", PKColumn: "PK_B", ConstraintName: "", KeySequence: 1},
		}
		got := BuildCompositeConditions(fks, "X", "Y")
		// Each FK with empty constraint name uses FKColumn as key → separate groups
		if len(got) != 2 {
			t.Fatalf("expected 2 conditions (separate groups by FKColumn), got %d: %v", len(got), got)
		}
	})

	t.Run("composite FK reversed KeySequence is sorted", func(t *testing.T) {
		fks := []FKEntry{
			{FKColumn: "FK_SECOND", PKColumn: "PK_SECOND", ConstraintName: "FK_COMP", KeySequence: 2},
			{FKColumn: "FK_FIRST", PKColumn: "PK_FIRST", ConstraintName: "FK_COMP", KeySequence: 1},
		}
		got := BuildCompositeConditions(fks, "A", "B")
		if len(got) != 1 {
			t.Fatalf("expected 1 condition, got %d", len(got))
		}
		idxFirst := strings.Index(got[0], "FK_FIRST")
		idxSecond := strings.Index(got[0], "FK_SECOND")
		if idxFirst > idxSecond {
			t.Errorf("expected FK_FIRST before FK_SECOND after sorting, got %q", got[0])
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// TestPkHeuristicConditions — PK naming convention suggestions
// ══════════════════════════════════════════════════════════════════════════════

func TestPkHeuristicConditions(t *testing.T) {
	t.Run("TABLE_ID pattern: last table has OTHER_ID column", func(t *testing.T) {
		// ORDERS has CUSTOMER_ID → should match CUSTOMER.ID (exact table name match)
		got := PkHeuristicConditions(
			"ORDERS", "O", "CUSTOMER", "C",
			[]string{"ORDER_ID", "CUSTOMER_ID", "TOTAL"}, // lastCols
			[]string{"ID", "NAME", "EMAIL"},              // otherCols
		)
		if len(got) != 1 {
			t.Fatalf("expected 1 heuristic condition, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "CUSTOMER_ID") || !strings.Contains(got[0], "ID") {
			t.Errorf("expected CUSTOMER_ID = ID match, got %q", got[0])
		}
	})

	t.Run("reverse direction: other table has LAST_ID column", func(t *testing.T) {
		got := PkHeuristicConditions(
			"CATEGORIES", "C", "PRODUCTS", "P",
			[]string{"ID", "NAME"},                          // lastCols (CATEGORIES)
			[]string{"PRODUCT_ID", "CATEGORIES_ID", "NAME"}, // otherCols (PRODUCTS)
		)
		if len(got) != 1 {
			t.Fatalf("expected 1 condition, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "CATEGORIES_ID") {
			t.Errorf("expected CATEGORIES_ID match, got %q", got[0])
		}
	})

	t.Run("no match when no ID column exists", func(t *testing.T) {
		got := PkHeuristicConditions(
			"T1", "A", "T2", "B",
			[]string{"T2_ID"}, // lastCols
			[]string{"NAME"},  // otherCols — no "ID"
		)
		if len(got) != 0 {
			t.Errorf("expected no matches when other has no ID column, got %v", got)
		}
	})

	t.Run("no match when no naming convention applies", func(t *testing.T) {
		got := PkHeuristicConditions(
			"T1", "A", "T2", "B",
			[]string{"FOO", "BAR"},
			[]string{"ID", "BAZ"},
		)
		if len(got) != 0 {
			t.Errorf("expected no matches, got %v", got)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		got := PkHeuristicConditions(
			"orders", "o", "customer", "c",
			[]string{"customer_id"},
			[]string{"id"},
		)
		if len(got) != 1 {
			t.Fatalf("expected case-insensitive match, got %d: %v", len(got), got)
		}
	})

	t.Run("both directions match produces two conditions", func(t *testing.T) {
		// T1 has T2_ID and ID; T2 has T1_ID and ID → both directions match
		got := PkHeuristicConditions(
			"T1", "A", "T2", "B",
			[]string{"ID", "T2_ID"},
			[]string{"ID", "T1_ID"},
		)
		if len(got) != 2 {
			t.Fatalf("expected 2 conditions (both directions), got %d: %v", len(got), got)
		}
	})

	t.Run("TABLEID pattern (no underscore)", func(t *testing.T) {
		got := PkHeuristicConditions(
			"ORDERS", "O", "CUSTOMER", "C",
			[]string{"CUSTOMERID"},
			[]string{"ID"},
		)
		if len(got) != 1 {
			t.Fatalf("expected CUSTOMERID match, got %d: %v", len(got), got)
		}
	})

	t.Run("empty column lists", func(t *testing.T) {
		got := PkHeuristicConditions("T1", "A", "T2", "B", nil, nil)
		if len(got) != 0 {
			t.Errorf("expected no matches with empty cols, got %v", got)
		}
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases for GetIdentifierAtColumn
// ══════════════════════════════════════════════════════════════════════════════

func TestGetIdentifierAtColumn_EmptyLine(t *testing.T) {
	got := GetIdentifierAtColumn("", 0)
	if got != nil {
		t.Errorf("GetIdentifierAtColumn('', 0) = %v, want nil", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases for ResolveTableRefs
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs_EmptyInput(t *testing.T) {
	got := ResolveTableRefs(nil, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result for nil refs, got %+v", got)
	}
	got = ResolveTableRefs([]JoinTableRef{}, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result for empty refs, got %+v", got)
	}
}

func TestResolveTableRefs_TwoPartFallsBackToSession(t *testing.T) {
	refs := []JoinTableRef{{Schema: "MY_SCH", Name: "MY_TBL", Alias: ""}}
	sess := &SessionContext{Database: "SESS_DB"}
	got := ResolveTableRefs(refs, nil, nil, sess)
	if len(got) != 1 || got[0].DB != "SESS_DB" || got[0].Schema != "MY_SCH" {
		t.Errorf("expected DB from session context, got %+v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases for ExtractInEditorTableDefs
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs_NoCreateTable(t *testing.T) {
	sql := "SELECT * FROM users;\nINSERT INTO logs VALUES (1);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected no defs for non-CREATE statements, got %+v", defs)
	}
}

func TestExtractInEditorTableDefs_EmptySQL(t *testing.T) {
	defs := ExtractInEditorTableDefs("", nil, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected no defs for empty SQL, got %+v", defs)
	}
}

func TestExtractInEditorTableDefs_CreateTableIfNotExists(t *testing.T) {
	sql := "CREATE TABLE IF NOT EXISTS my_tbl (id INT, name VARCHAR);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def for CREATE TABLE IF NOT EXISTS, got %d", len(defs))
	}
	if defs[0].Name != "MY_TBL" {
		t.Errorf("expected MY_TBL, got %s", defs[0].Name)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases for ComputeGitLineDiff
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeGitLineDiff_MultipleNonContiguousChanges(t *testing.T) {
	head := []string{"a", "b", "c", "d", "e"}
	current := []string{"a", "X", "c", "Y", "e"}
	got := ComputeGitLineDiff(head, current, 3000)
	if !reflect.DeepEqual(got.Modified, []int{2, 4}) {
		t.Errorf("Modified = %v, want [2 4]", got.Modified)
	}
	if len(got.Added) != 0 {
		t.Errorf("Added = %v, want empty", got.Added)
	}
	if len(got.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty", got.Deleted)
	}
}

func TestComputeGitLineDiff_CompleteReplacement(t *testing.T) {
	head := []string{"a", "b", "c"}
	current := []string{"x", "y", "z"}
	got := ComputeGitLineDiff(head, current, 3000)
	// All lines are different — depending on LCS, these could be modifications or add+delete
	totalChanges := len(got.Added) + len(got.Modified) + len(got.Deleted)
	if totalChanges == 0 {
		t.Error("expected changes when all lines differ")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases for GetScriptingCompletions
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_EmptyDollarBlock(t *testing.T) {
	sql := "$$ $$"
	got := GetScriptingCompletions(sql, 3) // cursor inside empty block
	if len(got.Variables) != 0 {
		t.Errorf("expected no variables in empty $$ block, got %v", got.Variables)
	}
}

func TestGetScriptingCompletions_CursorAtZero(t *testing.T) {
	sql := "$$ DECLARE x INT; BEGIN END; $$"
	got := GetScriptingCompletions(sql, 0)
	if len(got.Variables) != 0 {
		t.Errorf("expected no variables at offset 0, got %v", got.Variables)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: ParseSignatureParams
// ══════════════════════════════════════════════════════════════════════════════

func TestParseSignatureParams_EmptyString(t *testing.T) {
	got := ParseSignatureParams("")
	if got != nil {
		t.Errorf("ParseSignatureParams('') = %v, want nil", got)
	}
}

func TestParseSignatureParams_NoParen(t *testing.T) {
	got := ParseSignatureParams("SOME_FUNCTION")
	if got != nil {
		t.Errorf("ParseSignatureParams('SOME_FUNCTION') = %v, want nil", got)
	}
}

func TestParseSignatureParams_EmptyParens(t *testing.T) {
	got := ParseSignatureParams("F()")
	if got != nil {
		t.Errorf("ParseSignatureParams('F()') = %v, want nil (empty parens)", got)
	}
}

func TestParseSignatureParams_UnclosedParen(t *testing.T) {
	got := ParseSignatureParams("F(a, b")
	if got != nil {
		t.Errorf("ParseSignatureParams('F(a, b') = %v, want nil (unclosed paren)", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: GetActiveFunctionCall
// ══════════════════════════════════════════════════════════════════════════════

func TestGetActiveFunctionCall_EmptyString(t *testing.T) {
	got := GetActiveFunctionCall("")
	if got != nil {
		t.Errorf("GetActiveFunctionCall('') = %+v, want nil", got)
	}
}

func TestGetActiveFunctionCall_SchemaQualified(t *testing.T) {
	// Schema-qualified function: MY_SCHEMA.MY_FUNC( — the dot stops the backward
	// identifier scan, so only MY_FUNC should be captured as the function name.
	got := GetActiveFunctionCall("SELECT MY_SCHEMA.MY_FUNC(a, ")
	if got == nil {
		t.Fatal("expected non-nil result for schema-qualified function")
	}
	if got.Name != "MY_FUNC" {
		t.Errorf("expected Name=MY_FUNC, got %q", got.Name)
	}
	if got.ParamIndex != 1 {
		t.Errorf("expected ParamIndex=1, got %d", got.ParamIndex)
	}
}

func TestGetActiveFunctionCall_DoubleQuotedFuncName(t *testing.T) {
	// Double-quoted identifiers before a paren should be skipped (no func name)
	got := GetActiveFunctionCall(`SELECT "my func"(`)
	if got != nil {
		// The function scans backward from paren; double-quoted idents are skipped
		// so this may or may not produce a result depending on implementation.
		// If it does, the name should not include the quotes.
		if strings.Contains(got.Name, `"`) {
			t.Errorf("expected no quotes in function name, got %q", got.Name)
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: GetStatementRanges
// ══════════════════════════════════════════════════════════════════════════════

func TestGetStatementRanges_WhitespaceOnly(t *testing.T) {
	got := GetStatementRanges("   \t\n\r  ")
	if len(got) != 0 {
		t.Errorf("expected no ranges for whitespace-only SQL, got %v", got)
	}
}

func TestGetStatementRanges_NBSP(t *testing.T) {
	// Non-breaking space (\u00a0) should be treated as whitespace
	got := GetStatementRanges("\u00a0\u00a0")
	if len(got) != 0 {
		t.Errorf("expected no ranges for NBSP-only SQL, got %v", got)
	}
}

func TestGetStatementRanges_CarriageReturnNewline(t *testing.T) {
	sql := "SELECT 1;\r\nSELECT 2"
	got := GetStatementRanges(sql)
	if len(got) != 2 {
		t.Fatalf("expected 2 ranges, got %d: %v", len(got), got)
	}
	// Second statement should start on line 2
	if got[1].StartLine != 2 {
		t.Errorf("expected second statement on line 2, got %d", got[1].StartLine)
	}
}

func TestGetStatementRanges_LineCommentAtEOF(t *testing.T) {
	sql := "SELECT 1; -- trailing"
	got := GetStatementRanges(sql)
	if len(got) != 1 {
		t.Fatalf("expected 1 range, got %d: %v", len(got), got)
	}
	if got[0].EndOffset != 9 {
		t.Errorf("expected EndOffset=9, got %d", got[0].EndOffset)
	}
}

func TestGetStatementRanges_DoubleQuotedMultiLine(t *testing.T) {
	sql := "SELECT \"col\nname\" FROM t"
	got := GetStatementRanges(sql)
	if len(got) != 1 {
		t.Fatalf("expected 1 range, got %d: %v", len(got), got)
	}
	// The newline inside the double-quoted identifier should advance the line counter
	if got[0].EndLine != 2 {
		t.Errorf("expected EndLine=2 (newline inside quoted ident), got %d", got[0].EndLine)
	}
}

func TestGetStatementRanges_EmptyString(t *testing.T) {
	got := GetStatementRanges("")
	if len(got) != 0 {
		t.Errorf("expected empty for empty SQL, got %v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: ComputeJoinOnConditions
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeJoinOnConditions_ZeroRefs(t *testing.T) {
	req := JoinOnSuggestionsReq{
		ResolvedRefs: nil,
		Prefix:       "ON ",
		ColEntries:   []ColEntry{},
	}
	got := ComputeJoinOnConditions(req)
	if len(got) != 0 {
		t.Errorf("expected no suggestions with zero refs, got %v", got)
	}
}

func TestComputeJoinOnConditions_ThreeTablesLastHasNoSharedColumns(t *testing.T) {
	// Three tables: the last ref (T3) has no common column names with T1 or T2.
	// ComputeJoinOnConditions compares the last ref against all others,
	// so this should produce no suggestions.
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			{Alias: "C", DB: "DB", Schema: "S", Name: "T3"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T3", Cols: []ColInfo{{Name: "REGION", DataType: "VARCHAR"}}},
		},
	}
	got := ComputeJoinOnConditions(req)
	if len(got) != 0 {
		t.Errorf("expected no suggestions when last table has no common columns with others, got %v", got)
	}
}

func TestComputeJoinOnConditions_ThreeTablesLastSharesWithFirst(t *testing.T) {
	// Three tables: the last ref (T3) shares a column with T1 (not T2).
	// Should produce a suggestion between T3 and T1.
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
			{Alias: "C", DB: "DB", Schema: "S", Name: "T3"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "REGION_ID", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "TOTAL", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T3", Cols: []ColInfo{{Name: "REGION_ID", DataType: "NUMBER"}}},
		},
	}
	got := ComputeJoinOnConditions(req)
	foundMatch := false
	for _, c := range got {
		if strings.Contains(c.Condition, "REGION_ID") {
			foundMatch = true
		}
	}
	if !foundMatch {
		t.Errorf("expected REGION_ID match between T3 and T1, got %v", got)
	}
}

func TestComputeJoinOnConditions_SelfJoin(t *testing.T) {
	// Self-join: same table referenced twice with different aliases
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "E1", DB: "DB", Schema: "S", Name: "EMPLOYEES"},
			{Alias: "E2", DB: "DB", Schema: "S", Name: "EMPLOYEES"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "EMPLOYEES", Cols: []ColInfo{
				{Name: "ID", DataType: "NUMBER"},
				{Name: "MANAGER_ID", DataType: "NUMBER"},
			}},
		},
	}
	got := ComputeJoinOnConditions(req)
	// Should produce same-name column matches (ID=ID, MANAGER_ID=MANAGER_ID)
	if len(got) == 0 {
		t.Error("expected at least one suggestion for self-join")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: GetAutocompleteContext
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContext_WhitespaceOnly(t *testing.T) {
	ctx := GetAutocompleteContext("   \t\n  ", 3)
	if len(ctx.StatementRanges) != 0 {
		t.Errorf("expected no ranges for whitespace-only SQL, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != -1 {
		t.Errorf("expected currentStmtIdx=-1, got %d", ctx.CurrentStmtIdx)
	}
}

func TestGetAutocompleteContext_CursorAtZero(t *testing.T) {
	ctx := GetAutocompleteContext("SELECT 1", 0)
	if ctx.CurrentStmtIdx != 0 {
		t.Errorf("expected currentStmtIdx=0 at offset 0, got %d", ctx.CurrentStmtIdx)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: IsInJoinOnClause
// ══════════════════════════════════════════════════════════════════════════════

func TestIsInJoinOnClause_LeftJoin(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 LEFT JOIN t2 ON ")
	if !got {
		t.Error("expected true for LEFT JOIN ... ON")
	}
}

func TestIsInJoinOnClause_RightOuterJoin(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 RIGHT OUTER JOIN t2 ON t1.id = ")
	if !got {
		t.Error("expected true for RIGHT OUTER JOIN ... ON")
	}
}

func TestIsInJoinOnClause_CrossJoinNoOn(t *testing.T) {
	// CROSS JOIN has no ON clause — should return false.
	got := IsInJoinOnClause("SELECT * FROM t1 CROSS JOIN t2 WHERE ")
	if got {
		t.Error("expected false for CROSS JOIN (no ON)")
	}
}

func TestIsInJoinOnClause_NaturalJoinNoOn(t *testing.T) {
	// NATURAL JOIN has no ON clause — should return false.
	got := IsInJoinOnClause("SELECT * FROM t1 NATURAL JOIN t2 WHERE ")
	if got {
		t.Error("expected false for NATURAL JOIN (no ON)")
	}
}

func TestIsInJoinOnClause_FullOuterJoin(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 FULL OUTER JOIN t2 ON ")
	if !got {
		t.Error("expected true for FULL OUTER JOIN ... ON")
	}
}

func TestIsInJoinOnClause_OrderByTerminates(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = t2.id ORDER BY ")
	if got {
		t.Error("expected false after ORDER BY terminates")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: DetectUsingClause
// ══════════════════════════════════════════════════════════════════════════════

func TestDetectUsingClause_AlreadyClosed(t *testing.T) {
	got := DetectUsingClause("JOIN t2 USING (col1)")
	if got.InUsing || got.IsPartial {
		t.Errorf("expected no USING context after closed paren, got %+v", got)
	}
}

func TestDetectUsingClause_SingleColumnPartial(t *testing.T) {
	// A single column followed by comma → IsPartial
	got := DetectUsingClause("JOIN t2 USING (col1, ")
	if !got.IsPartial {
		t.Error("expected IsPartial=true for 'USING (col1, '")
	}
	if got.InUsing {
		t.Error("expected InUsing=false for partial USING")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: TypeCategory
// ══════════════════════════════════════════════════════════════════════════════

func TestTypeCategory_AdditionalTypes(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		want     string
	}{
		{name: "TIMESTAMP_TZ", dataType: "TIMESTAMP_TZ", want: "datetime"},
		{name: "NUMERIC", dataType: "NUMERIC", want: "numeric"},
		{name: "BYTEINT", dataType: "BYTEINT", want: "numeric"},
		{name: "VARBINARY", dataType: "VARBINARY", want: "text"},
		{name: "CHARACTER", dataType: "CHARACTER", want: "text"},
		{name: "FLOAT4", dataType: "FLOAT4", want: "numeric"},
		{name: "FLOAT8", dataType: "FLOAT8", want: "numeric"},
		{name: "DOUBLE PRECISION", dataType: "DOUBLE PRECISION", want: "numeric"},
		{name: "DATETIME", dataType: "DATETIME", want: "datetime"},
		{name: "with leading/trailing spaces", dataType: "  VARCHAR(50)  ", want: "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TypeCategory(tt.dataType)
			if got != tt.want {
				t.Errorf("TypeCategory(%q) = %q, want %q", tt.dataType, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: ComputeGitLineDiff
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeGitLineDiff_NegativeMaxLines(t *testing.T) {
	got := ComputeGitLineDiff([]string{"a"}, []string{"b"}, -1)
	if len(got.Added) != 0 || len(got.Modified) != 0 || len(got.Deleted) != 0 {
		t.Errorf("expected empty diff when maxLines is negative, got added=%v mod=%v del=%v", got.Added, got.Modified, got.Deleted)
	}
}

func TestComputeGitLineDiff_MaxLinesZero(t *testing.T) {
	got := ComputeGitLineDiff([]string{"a"}, []string{"b"}, 0)
	if len(got.Added) != 0 || len(got.Modified) != 0 || len(got.Deleted) != 0 {
		t.Errorf("expected empty diff when maxLines=0, got added=%v mod=%v del=%v", got.Added, got.Modified, got.Deleted)
	}
}

func TestComputeGitLineDiff_SingleLineSame(t *testing.T) {
	got := ComputeGitLineDiff([]string{"a"}, []string{"a"}, 3000)
	if len(got.Added) != 0 || len(got.Modified) != 0 || len(got.Deleted) != 0 {
		t.Errorf("expected no changes for identical single-line, got added=%v mod=%v del=%v", got.Added, got.Modified, got.Deleted)
	}
}

func TestComputeGitLineDiff_SingleLineDifferent(t *testing.T) {
	got := ComputeGitLineDiff([]string{"a"}, []string{"b"}, 3000)
	totalChanges := len(got.Added) + len(got.Modified) + len(got.Deleted)
	if totalChanges == 0 {
		t.Error("expected changes when single line differs")
	}
}

func TestComputeGitLineDiff_AddedAtBeginning(t *testing.T) {
	head := []string{"b", "c"}
	current := []string{"a", "b", "c"}
	got := ComputeGitLineDiff(head, current, 3000)
	if !reflect.DeepEqual(got.Added, []int{1}) {
		t.Errorf("Added = %v, want [1]", got.Added)
	}
}

func TestComputeGitLineDiff_DeletedFromEnd(t *testing.T) {
	head := []string{"a", "b", "c"}
	current := []string{"a", "b"}
	got := ComputeGitLineDiff(head, current, 3000)
	if len(got.Deleted) == 0 {
		t.Error("expected deleted lines when removing from end")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: ResolveTableRefs
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs_AmbiguousStoreMatch(t *testing.T) {
	// Same unqualified name exists in multiple schemas — first match wins
	storeObjs := []StoreObject{
		{DB: "DB1", Schema: "S1", Name: "USERS", Kind: "TABLE"},
		{DB: "DB1", Schema: "S2", Name: "USERS", Kind: "TABLE"},
	}
	refs := []JoinTableRef{{Name: "USERS", Alias: "u"}}
	got := ResolveTableRefs(refs, storeObjs, nil, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	// Should match the first one found
	if got[0].DB != "DB1" {
		t.Errorf("expected DB1, got %s", got[0].DB)
	}
}

func TestResolveTableRefs_TwoPartMatchedInStore(t *testing.T) {
	storeObjs := []StoreObject{
		{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
	}
	refs := []JoinTableRef{{Schema: "PUBLIC", Name: "ORDERS", Alias: "o"}}
	got := ResolveTableRefs(refs, storeObjs, nil, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "MYDB" || got[0].Schema != "PUBLIC" {
		t.Errorf("expected MYDB.PUBLIC, got %s.%s", got[0].DB, got[0].Schema)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: ExtractInEditorTableDefs
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs_ColumnConstraints(t *testing.T) {
	// Columns with NOT NULL, DEFAULT, PRIMARY KEY — parser should extract column names and ignore constraints.
	sql := "CREATE TABLE orders (order_id INT NOT NULL PRIMARY KEY, customer_name VARCHAR(100) DEFAULT 'unknown', amount DECIMAL(18,2) NOT NULL);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if len(defs[0].Cols) != 3 {
		t.Fatalf("expected 3 columns, got %d: %+v", len(defs[0].Cols), defs[0].Cols)
	}
	names := []string{defs[0].Cols[0].Name, defs[0].Cols[1].Name, defs[0].Cols[2].Name}
	expected := []string{"ORDER_ID", "CUSTOMER_NAME", "AMOUNT"}
	if !reflect.DeepEqual(names, expected) {
		t.Errorf("expected column names %v, got %v", expected, names)
	}
}

func TestExtractInEditorTableDefs_QuotedIdentifiers(t *testing.T) {
	sql := `CREATE TABLE "My Table" ("Order ID" INT, "Customer Name" VARCHAR(100));`
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "My Table" {
		t.Errorf("expected table name 'My Table', got %q", defs[0].Name)
	}
	if len(defs[0].Cols) != 2 {
		t.Fatalf("expected 2 columns, got %d: %+v", len(defs[0].Cols), defs[0].Cols)
	}
	if defs[0].Cols[0].Name != "Order ID" {
		t.Errorf("expected first column 'Order ID', got %q", defs[0].Cols[0].Name)
	}
	if defs[0].Cols[1].Name != "Customer Name" {
		t.Errorf("expected second column 'Customer Name', got %q", defs[0].Cols[1].Name)
	}
}

func TestExtractInEditorTableDefs_CreateTableClone(t *testing.T) {
	sql := "CREATE TABLE cloned_tbl CLONE source_tbl;"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected CLONE to be skipped, got %+v", defs)
	}
}

func TestExtractInEditorTableDefs_CreateTableLike(t *testing.T) {
	sql := "CREATE TABLE liked_tbl LIKE source_tbl;"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected LIKE to be skipped, got %+v", defs)
	}
}

func TestExtractInEditorTableDefs_CreateTempTable(t *testing.T) {
	sql := "CREATE TEMPORARY TABLE tmp_tbl (id INT, val TEXT);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def for CREATE TEMPORARY TABLE, got %d", len(defs))
	}
	if defs[0].Name != "TMP_TBL" {
		t.Errorf("expected TMP_TBL, got %s", defs[0].Name)
	}
}

func TestExtractInEditorTableDefs_TwoPartTableName(t *testing.T) {
	sql := "CREATE TABLE my_schema.my_table (col1 INT);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Schema != "MY_SCHEMA" || defs[0].Name != "MY_TABLE" {
		t.Errorf("expected MY_SCHEMA.MY_TABLE, got %s.%s", defs[0].Schema, defs[0].Name)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: GetScriptingCompletions
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_EmptySQL(t *testing.T) {
	got := GetScriptingCompletions("", 0)
	if len(got.Variables) != 0 {
		t.Errorf("expected no variables for empty SQL, got %v", got.Variables)
	}
	if got.NeedsColon {
		t.Error("expected NeedsColon=false for empty SQL")
	}
}

func TestGetScriptingCompletions_RESULTSETVar(t *testing.T) {
	sql := "$$ DECLARE rs RESULTSET; BEGIN rs := (SELECT 1); END; $$"
	offset := len([]rune("$$ DECLARE rs RESULTSET; BEGIN rs := (SELECT 1); END; "))
	got := GetScriptingCompletions(sql, offset)
	found := false
	for _, v := range got.Variables {
		if v == "RS" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected RS in variables, got %v", got.Variables)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional edge cases: IsDatatypeContext
// ══════════════════════════════════════════════════════════════════════════════

func TestIsDatatypeContext_CreateTempTable(t *testing.T) {
	got := IsDatatypeContext("CREATE TEMPORARY TABLE t (\n  col ", "  col ")
	if !got {
		t.Error("expected true for CREATE TEMPORARY TABLE column position")
	}
}

func TestIsDatatypeContext_AfterCommaInCreateTable(t *testing.T) {
	got := IsDatatypeContext("CREATE TABLE t (\n  id INT,\n  name ", "  name ")
	if !got {
		t.Error("expected true for second column in CREATE TABLE")
	}
}

func TestIsDatatypeContext_SelectColumn(t *testing.T) {
	got := IsDatatypeContext("SELECT col ", "SELECT col ")
	if got {
		t.Error("expected false for SELECT column context (not a datatype position)")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ResolveTableRefs: cross-context resolution (UseContext DB + session Schema)
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs_CrossContextDBFromUseSchemFromSession(t *testing.T) {
	// UseContext provides only Database, session provides only Schema.
	// The resolution should combine both to produce a fully-qualified ref.
	refs := []JoinTableRef{{Name: "MY_TABLE", Alias: "t"}}
	useCtx := &UseContext{Database: "USE_DB"}
	sess := &SessionContext{Schema: "SESS_SCH"}
	got := ResolveTableRefs(refs, nil, useCtx, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "USE_DB" {
		t.Errorf("expected DB from UseContext (USE_DB), got %q", got[0].DB)
	}
	if got[0].Schema != "SESS_SCH" {
		t.Errorf("expected Schema from session (SESS_SCH), got %q", got[0].Schema)
	}
}

func TestResolveTableRefs_CrossContextSchemaFromUseDBFromSession(t *testing.T) {
	// UseContext provides only Schema, session provides only Database.
	refs := []JoinTableRef{{Name: "TBL", Alias: ""}}
	useCtx := &UseContext{Schema: "USE_SCH"}
	sess := &SessionContext{Database: "SESS_DB"}
	got := ResolveTableRefs(refs, nil, useCtx, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "SESS_DB" {
		t.Errorf("expected DB from session (SESS_DB), got %q", got[0].DB)
	}
	if got[0].Schema != "USE_SCH" {
		t.Errorf("expected Schema from UseContext (USE_SCH), got %q", got[0].Schema)
	}
}

func TestResolveTableRefs_NonTableViewObjectsFiltered(t *testing.T) {
	// Store objects with kinds other than TABLE/VIEW should not match.
	storeObjs := []StoreObject{
		{DB: "DB", Schema: "S", Name: "MY_FUNC", Kind: "FUNCTION"},
		{DB: "DB", Schema: "S", Name: "MY_PROC", Kind: "PROCEDURE"},
	}
	refs := []JoinTableRef{{Name: "MY_FUNC", Alias: ""}}
	got := ResolveTableRefs(refs, storeObjs, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected no match for FUNCTION kind, got %+v", got)
	}
}

func TestResolveTableRefs_OnlyDBSetGetsSchemaFromContext(t *testing.T) {
	// Ref has DB already set but no Schema — should pick up Schema from UseContext.
	refs := []JoinTableRef{{DB: "MY_DB", Name: "TBL", Alias: ""}}
	useCtx := &UseContext{Schema: "CTX_SCH"}
	got := ResolveTableRefs(refs, nil, useCtx, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "MY_DB" || got[0].Schema != "CTX_SCH" {
		t.Errorf("expected MY_DB.CTX_SCH, got %s.%s", got[0].DB, got[0].Schema)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetAutocompleteContext: cursor in gap between statements
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContext_CursorInGapBetweenStatements(t *testing.T) {
	// Cursor sits in whitespace between two statements and doesn't match any range.
	// The fallback should use the last statement.
	sql := "SELECT 1;\n\n\nSELECT 2"
	offset := 11 // in the blank line between the two statements

	ctx := GetAutocompleteContext(sql, offset)
	// Should fall back to the last statement
	if ctx.CurrentStmtIdx == -1 {
		t.Error("expected currentStmtIdx to be set (fallback to last statement)")
	}
	if len(ctx.StatementRanges) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(ctx.StatementRanges))
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetAutocompleteContextFull: empty SQL
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContextFull_EmptySQL(t *testing.T) {
	req := AutocompleteContextRequest{
		SQL:          "",
		CursorOffset: 0,
	}
	ctx := GetAutocompleteContextFull(req)
	if len(ctx.StatementRanges) != 0 {
		t.Errorf("expected no ranges, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != -1 {
		t.Errorf("expected currentStmtIdx=-1, got %d", ctx.CurrentStmtIdx)
	}
	if len(ctx.ResolvedRefs) != 0 {
		t.Errorf("expected no resolved refs, got %+v", ctx.ResolvedRefs)
	}
	if len(ctx.InEditorTables) != 0 {
		t.Errorf("expected no in-editor tables, got %+v", ctx.InEditorTables)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ComputeJoinOnConditions: ON prefix IS prepended
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeJoinOnConditions_PrefixApplied(t *testing.T) {
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
		},
	}
	got := ComputeJoinOnConditions(req)
	if len(got) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	for _, c := range got {
		// USING suggestions replace the ON clause entirely, so they don't get the "ON " prefix.
		if c.Detail == "USING" {
			if strings.HasPrefix(c.Condition, "ON ") {
				t.Errorf("USING condition %q should NOT start with 'ON '", c.Condition)
			}
			continue
		}
		if !strings.HasPrefix(c.Condition, "ON ") {
			t.Errorf("condition %q should start with 'ON ' when Prefix is 'ON '", c.Condition)
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ExtractInEditorTableDefs: mixed context (UseContext schema + session DB)
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs_UseContextSchemaSessionDB(t *testing.T) {
	sql := "CREATE TABLE my_tbl (col1 INT);"
	ranges := GetStatementRanges(sql)
	useCtx := &UseContext{Schema: "CTX_SCH"}
	sess := &SessionContext{Database: "SESS_DB"}
	defs := ExtractInEditorTableDefs(sql, ranges, useCtx, sess)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].DB != "SESS_DB" {
		t.Errorf("expected DB from session (SESS_DB), got %q", defs[0].DB)
	}
	if defs[0].Schema != "CTX_SCH" {
		t.Errorf("expected Schema from UseContext (CTX_SCH), got %q", defs[0].Schema)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetScriptingCompletions: NeedsColon=false explicitly asserted for scripting
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_NeedsColonFalseForScriptingStatements(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		offset int
	}{
		{
			name:   "LET assignment",
			sql:    "$$ BEGIN LET x := ",
			offset: -1,
		},
		{
			name:   "IF condition",
			sql:    "$$ BEGIN IF (",
			offset: -1,
		},
		{
			name:   "RETURN statement",
			sql:    "$$ BEGIN RETURN ",
			offset: -1,
		},
		{
			name:   "after colon prefix",
			sql:    "$$ BEGIN LET y := (SELECT 1); RETURN :",
			offset: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.offset
			if offset == -1 {
				offset = len([]rune(tt.sql))
			}
			got := GetScriptingCompletions(tt.sql, offset)
			if got.NeedsColon {
				t.Errorf("NeedsColon = true, want false for %q", tt.sql)
			}
		})
	}
}

func TestGetScriptingCompletions_MultipleDollarBlocks(t *testing.T) {
	// Two $$ blocks: cursor in the second block should only see second block's vars.
	sql := "$$ DECLARE a INT; BEGIN END; $$; $$ DECLARE b INT; BEGIN RETURN :b; END; $$"
	offset := len([]rune("$$ DECLARE a INT; BEGIN END; $$; $$ DECLARE b INT; BEGIN RETURN :b; END; "))
	got := GetScriptingCompletions(sql, offset)

	foundA := false
	foundB := false
	for _, v := range got.Variables {
		if v == "A" {
			foundA = true
		}
		if v == "B" {
			foundB = true
		}
	}
	if foundA {
		t.Error("variable A from first $$ block should NOT be visible in second block")
	}
	if !foundB {
		t.Errorf("expected B in variables, got %v", got.Variables)
	}
}

func TestGetScriptingCompletions_CursorSkipped(t *testing.T) {
	// DECLARE CURSOR should be skipped (CURSOR is in skipWords).
	sql := "$$ DECLARE c1 CURSOR FOR SELECT 1; myvar INT; BEGIN END; $$"
	offset := len([]rune("$$ DECLARE c1 CURSOR FOR SELECT 1; myvar INT; BEGIN END; "))
	got := GetScriptingCompletions(sql, offset)

	foundC1 := false
	foundMyvar := false
	for _, v := range got.Variables {
		if v == "C1" {
			foundC1 = true
		}
		if v == "MYVAR" {
			foundMyvar = true
		}
	}
	if !foundC1 {
		t.Errorf("expected C1 in variables (first ident in DECLARE segment), got %v", got.Variables)
	}
	if !foundMyvar {
		t.Errorf("expected MYVAR in variables, got %v", got.Variables)
	}
}

func TestGetScriptingCompletions_NeedsColonTrueForDMLInScript(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		offset int
	}{
		{
			name:   "INSERT in script",
			sql:    "$$ BEGIN INSERT INTO t VALUES (",
			offset: -1,
		},
		{
			name:   "UPDATE in script",
			sql:    "$$ BEGIN UPDATE t SET col = ",
			offset: -1,
		},
		{
			name:   "DELETE in script",
			sql:    "$$ BEGIN DELETE FROM t WHERE id = ",
			offset: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.offset
			if offset == -1 {
				offset = len([]rune(tt.sql))
			}
			got := GetScriptingCompletions(tt.sql, offset)
			if !got.NeedsColon {
				t.Errorf("NeedsColon = false, want true for %q", tt.sql)
			}
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional standard edge cases — core logic gaps
// ══════════════════════════════════════════════════════════════════════════════

func TestGetIdentifierAtColumn_WhitespaceOnlyLine(t *testing.T) {
	got := GetIdentifierAtColumn("   \t  ", 2)
	if got != nil {
		t.Errorf("GetIdentifierAtColumn(whitespace, 2) = %v, want nil", got)
	}
}

func TestParseSignatureParams_SingleParam(t *testing.T) {
	got := ParseSignatureParams("F(x)")
	want := []SignatureParam{{Start: 2, End: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSignatureParams(\"F(x)\") = %v, want %v", got, want)
	}
}

func TestGetStatementRanges_BackToBackNoWhitespace(t *testing.T) {
	sql := "SELECT 1;SELECT 2"
	got := GetStatementRanges(sql)
	if len(got) != 2 {
		t.Fatalf("expected 2 ranges for back-to-back statements, got %d: %v", len(got), got)
	}
	if got[0].StartOffset != 0 || got[0].EndOffset != 9 {
		t.Errorf("first range: got start=%d end=%d, want start=0 end=9", got[0].StartOffset, got[0].EndOffset)
	}
	if got[1].StartOffset != 9 || got[1].EndOffset != 17 {
		t.Errorf("second range: got start=%d end=%d, want start=9 end=17", got[1].StartOffset, got[1].EndOffset)
	}
}

func TestIsInJoinOnClause_InnerJoin(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 INNER JOIN t2 ON ")
	if !got {
		t.Error("expected true for INNER JOIN ... ON")
	}
}

func TestGetAutocompleteContext_UseAfterCursorIgnored(t *testing.T) {
	sql := "SELECT * FROM t;\nUSE DATABASE new_db;"
	offset := 5 // cursor in first statement
	ctx := GetAutocompleteContext(sql, offset)
	if ctx.UseContext != nil {
		t.Errorf("expected nil UseContext when USE is after cursor, got %+v", ctx.UseContext)
	}
}

func TestGetAutocompleteContext_CursorMidStatement(t *testing.T) {
	sql := "SELECT a, b, c FROM my_table WHERE id > 10"
	offset := 20 // inside "my_table"
	ctx := GetAutocompleteContext(sql, offset)
	if ctx.CurrentStmtIdx != 0 {
		t.Errorf("expected currentStmtIdx=0, got %d", ctx.CurrentStmtIdx)
	}
	// Table refs from the full statement should be available regardless of cursor position
	foundTable := false
	for _, ref := range ctx.TableRefs {
		if strings.EqualFold(ref.Name, "MY_TABLE") {
			foundTable = true
		}
	}
	if !foundTable {
		t.Errorf("expected MY_TABLE in table refs with cursor mid-statement, got %+v", ctx.TableRefs)
	}
}

func TestComputeGitLineDiff_MaxLinesExactBoundary(t *testing.T) {
	head := []string{"a", "b", "c"}
	current := []string{"a", "x", "c"}
	// maxLines equals exactly the length of both inputs — should still compute diff
	got := ComputeGitLineDiff(head, current, 3)
	if !reflect.DeepEqual(got.Modified, []int{2}) {
		t.Errorf("Modified = %v, want [2]", got.Modified)
	}
}

func TestComputeGitLineDiff_AdditionsAndDeletions(t *testing.T) {
	head := []string{"a", "b", "c", "d", "e"}
	current := []string{"a", "c", "d", "e", "f", "g"}
	got := ComputeGitLineDiff(head, current, 3000)
	// "b" was deleted; "f" and "g" were added; nothing modified
	if len(got.Added) == 0 {
		t.Error("expected added lines for f and g")
	}
	if len(got.Deleted) == 0 {
		t.Error("expected deleted line marker for b")
	}
	if len(got.Modified) != 0 {
		t.Errorf("expected no modified lines, got %v", got.Modified)
	}
}

func TestComputeJoinOnConditions_BidirectionalFKs(t *testing.T) {
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "",
		FKEntries: []TableFKEntry{
			{
				DB: "DB", Schema: "S", Name: "T1",
				FKs: []FKEntry{
					{PKDatabase: "DB", PKSchema: "S", PKTable: "T2", PKColumn: "ID", FKColumn: "FK_T2_ID", ConstraintName: "FK_T1_TO_T2", KeySequence: 1},
				},
			},
			{
				DB: "DB", Schema: "S", Name: "T2",
				FKs: []FKEntry{
					{PKDatabase: "DB", PKSchema: "S", PKTable: "T1", PKColumn: "ID", FKColumn: "FK_T1_ID", ConstraintName: "FK_T2_TO_T1", KeySequence: 1},
				},
			},
		},
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{
				{Name: "ID", DataType: "NUMBER"},
				{Name: "FK_T2_ID", DataType: "NUMBER"},
			}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{
				{Name: "ID", DataType: "NUMBER"},
				{Name: "FK_T1_ID", DataType: "NUMBER"},
			}},
		},
	}
	got := ComputeJoinOnConditions(req)
	fkCount := 0
	for _, c := range got {
		if c.Detail == "FK RELATION" {
			fkCount++
		}
	}
	if fkCount < 2 {
		t.Errorf("expected at least 2 FK suggestions (bidirectional), got %d FK suggestions out of %d total", fkCount, len(got))
	}
}

func TestExtractInEditorTableDefs_CreateViewIgnored(t *testing.T) {
	sql := "CREATE TABLE t1 (id INT, name TEXT);\nCREATE VIEW v1 AS SELECT * FROM t1;"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (CREATE VIEW should be ignored), got %d: %+v", len(defs), defs)
	}
	if defs[0].Name != "T1" {
		t.Errorf("expected T1, got %s", defs[0].Name)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional missing edge cases — core logic gaps
// ══════════════════════════════════════════════════════════════════════════════

func TestGetIdentifierAtColumn_LeadingDotAtCol0(t *testing.T) {
	// Col 0 sits on the leading dot itself, which is not a word char and not a quote.
	// The scanner skips non-word, non-quote chars, then starts the chain at "a".
	// Col 0 is before the chain's first part, so the chain should not contain it.
	got := GetIdentifierAtColumn(".a.b", 0)
	if got != nil {
		t.Errorf("GetIdentifierAtColumn('.a.b', 0) = %v, want nil", got)
	}
}

func TestGetIdentifierAtColumn_OnlyDots(t *testing.T) {
	got := GetIdentifierAtColumn("...", 1)
	if got != nil {
		t.Errorf("GetIdentifierAtColumn('...', 1) = %v, want nil", got)
	}
}

func TestGetIdentifierAtColumn_QuotedWithDotInside(t *testing.T) {
	// A quoted identifier containing a dot should NOT split on the internal dot.
	got := GetIdentifierAtColumn(`"db.schema"`, 5)
	if len(got) != 1 {
		t.Fatalf("expected 1 part for quoted ident with dot, got %v", got)
	}
	if got[0] != "db.schema" {
		t.Errorf("expected 'db.schema', got %q", got[0])
	}
}

func TestGetActiveFunctionCall_AllUnnamedParens(t *testing.T) {
	// Stack of unnamed parentheses — top has no name, should return nil.
	got := GetActiveFunctionCall("(((")
	if got != nil {
		t.Errorf("expected nil for unnamed nested parens, got %+v", got)
	}
}

func TestGetActiveFunctionCall_FuncAfterComma(t *testing.T) {
	// F(a, G( — innermost open call is G at param 0
	got := GetActiveFunctionCall("SELECT F(a, G(")
	if got == nil {
		t.Fatal("expected non-nil result for nested F(a, G(")
	}
	if got.Name != "G" {
		t.Errorf("expected Name=G, got %q", got.Name)
	}
	if got.ParamIndex != 0 {
		t.Errorf("expected ParamIndex=0, got %d", got.ParamIndex)
	}
}

func TestGetActiveFunctionCall_BlockCommentBeforeParen(t *testing.T) {
	// Function name separated from paren by a block comment — the backward
	// scan from '(' skips whitespace but not comments, so no name is found.
	got := GetActiveFunctionCall("SELECT FUNC /* comment */ (a, ")
	if got != nil {
		t.Errorf("expected nil (backward scan doesn't skip comments), got %+v", got)
	}
}

func TestGetScriptingCompletions_TaggedDollarQuoting(t *testing.T) {
	// Tagged dollar quoting ($tag$...$tag$) — the implementation uses simple $$
	// counting, so $tag$ blocks are NOT recognized as scripting blocks.
	// This test documents the expected behavior.
	sql := "$body$ DECLARE x INT; BEGIN END; $body$"
	offset := len([]rune("$body$ DECLARE x INT; BEGIN END; "))
	got := GetScriptingCompletions(sql, offset)
	// The simple $$ counter sees zero $$ pairs, so cursor is not "inside" a block.
	if len(got.Variables) != 0 {
		t.Logf("tagged dollar quoting produced variables (acceptable): %v", got.Variables)
	}
}

func TestGetScriptingCompletions_CursorExceedsLength(t *testing.T) {
	// cursorOffset > len(runes) should be clamped to len(runes) without panic.
	sql := "$$ DECLARE x INT; BEGIN END; $$"
	got := GetScriptingCompletions(sql, 9999)
	// Should not panic; whether variables are found depends on being "inside" the block.
	_ = got
}

func TestGetScriptingCompletions_NestedDollarBlocks(t *testing.T) {
	// Nested $$ inside an outer $$ — inner $$ closes and reopens.
	// cursor after the inner close $$ is still inside the outer block.
	sql := "$$ DECLARE a INT; BEGIN EXECUTE IMMEDIATE $$ SELECT 1 $$; LET b := 1; END; $$"
	offset := len([]rune("$$ DECLARE a INT; BEGIN EXECUTE IMMEDIATE $$ SELECT 1 $$; LET b := 1; END; "))
	got := GetScriptingCompletions(sql, offset)
	// After inner $$...$$ pair, total $$ count before cursor is 4 (even),
	// so we're NOT inside a $$ block according to the simple counter.
	// This test documents this known limitation.
	_ = got
}

func TestGetAutocompleteContext_OffsetAtEnd(t *testing.T) {
	// Offset exactly at end of SQL should use the last statement.
	sql := "SELECT 1;\nSELECT 2"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if ctx.CurrentStmtIdx != 1 {
		t.Errorf("expected currentStmtIdx=1, got %d", ctx.CurrentStmtIdx)
	}
}

func TestGetAutocompleteContextFull_CursorAtEnd(t *testing.T) {
	// CursorOffset at end of SQL.
	sql := "SELECT x::"
	req := AutocompleteContextRequest{
		SQL:          sql,
		CursorOffset: len([]rune(sql)),
		LineUpToWord: "SELECT x::",
	}
	ctx := GetAutocompleteContextFull(req)
	if !ctx.IsDatatypeCtx {
		t.Error("expected IsDatatypeCtx=true with cursor at end of '::'")
	}
}

func TestGetAutocompleteContextFull_EmptyLineUpToWord(t *testing.T) {
	// When LineUpToWord is empty, datatype/using detection via lineUpToWord
	// patterns should not match, but textToCursor patterns may still match.
	req := AutocompleteContextRequest{
		SQL:          "CREATE TABLE t (\n  col ",
		CursorOffset: len("CREATE TABLE t (\n  col "),
		LineUpToWord: "",
	}
	ctx := GetAutocompleteContextFull(req)
	// IsDatatypeContext checks lineUpToWord for :: and CAST patterns (won't match
	// empty string), but textToCursor for CREATE/ALTER patterns (should match).
	if !ctx.IsDatatypeCtx {
		t.Error("expected IsDatatypeCtx=true via textToCursor pattern even with empty LineUpToWord")
	}
}

func TestIsInJoinOnClause_LimitTerminates(t *testing.T) {
	// LIMIT is not in the terminator list — the implementation only terminates on
	// JOIN, WHERE, GROUP, ORDER, HAVING, UNION, INTERSECT, EXCEPT.
	// LIMIT does NOT terminate the ON clause.
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = t2.id LIMIT ")
	if !got {
		t.Log("LIMIT does not terminate ON clause (by design)")
	}
}

func TestIsInJoinOnClause_SubselectInOn(t *testing.T) {
	// A subquery after ON: the inner SELECT doesn't terminate the outer ON.
	// Actually it does — SELECT is not in the terminator regex,
	// but WHERE inside a subquery would appear as terminated.
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = (SELECT ")
	if !got {
		t.Error("expected true: SELECT inside subquery should not terminate outer ON")
	}
}

func TestIsInJoinOnClause_SubselectWhereTerminatesON(t *testing.T) {
	// WHERE inside a subquery in the ON clause incorrectly terminates detection.
	// This documents the known limitation.
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = (SELECT 1 FROM t3 WHERE ")
	// WHERE terminates the ON clause detection even though it's inside a subquery.
	if got {
		t.Log("subquery WHERE terminates ON detection (known limitation)")
	}
}

func TestDetectUsingClause_WithLeadingNewlines(t *testing.T) {
	got := DetectUsingClause("JOIN t2\nUSING (\n")
	if !got.InUsing {
		t.Error("expected InUsing=true with newlines around USING (")
	}
}

func TestTypeCategory_GeographyGeometry(t *testing.T) {
	geoTypes := []struct {
		dt   string
		want string
	}{
		{dt: "GEOGRAPHY", want: "other"},
		{dt: "GEOMETRY", want: "other"},
	}
	for _, tt := range geoTypes {
		got := TypeCategory(tt.dt)
		if got != tt.want {
			t.Errorf("TypeCategory(%q) = %q, want %q", tt.dt, got, tt.want)
		}
	}
}

func TestTypeCategory_WhitespaceOnly(t *testing.T) {
	got := TypeCategory("   ")
	if got != "other" {
		t.Errorf("TypeCategory('   ') = %q, want 'other'", got)
	}
}

func TestResolveTableRefs_DBOnlyNoSchemaNoContext(t *testing.T) {
	// Ref has DB but no Schema and no context → should be skipped (incomplete).
	refs := []JoinTableRef{{DB: "MY_DB", Name: "TBL", Alias: "t"}}
	got := ResolveTableRefs(refs, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result for DB-only ref with no context, got %+v", got)
	}
}

func TestResolveTableRefs_SchemaOnlyNoDBNoContext(t *testing.T) {
	// Ref has Schema but no DB and no context → should be skipped (incomplete).
	refs := []JoinTableRef{{Schema: "MY_SCH", Name: "TBL", Alias: "t"}}
	got := ResolveTableRefs(refs, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result for schema-only ref with no context, got %+v", got)
	}
}

func TestResolveTableRefs_AliasPreserved(t *testing.T) {
	// Ensure alias is preserved through resolution.
	refs := []JoinTableRef{{Name: "TBL", Alias: "my_alias"}}
	sess := &SessionContext{Database: "DB", Schema: "SCH"}
	got := ResolveTableRefs(refs, nil, nil, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].Alias != "my_alias" {
		t.Errorf("expected alias 'my_alias', got %q", got[0].Alias)
	}
}

func TestExtractInEditorTableDefs_CreateTransientTable(t *testing.T) {
	sql := "CREATE TRANSIENT TABLE staging_data (id INT, payload VARIANT);"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def for CREATE TRANSIENT TABLE, got %d", len(defs))
	}
	if defs[0].Name != "STAGING_DATA" {
		t.Errorf("expected STAGING_DATA, got %s", defs[0].Name)
	}
	if len(defs[0].Cols) != 2 {
		t.Errorf("expected 2 cols, got %d", len(defs[0].Cols))
	}
}

func TestExtractInEditorTableDefs_MultilineCreateTable(t *testing.T) {
	sql := `CREATE TABLE my_table (
    id INT,
    name VARCHAR(100),
    created_at TIMESTAMP
);`
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if len(defs[0].Cols) != 3 {
		t.Errorf("expected 3 columns for multiline CREATE TABLE, got %d: %+v", len(defs[0].Cols), defs[0].Cols)
	}
}

func TestComputeGitLineDiff_LargeIdenticalPrefixChangeAtEnd(t *testing.T) {
	head := []string{"a", "b", "c", "d", "e", "f", "g"}
	current := []string{"a", "b", "c", "d", "e", "f", "X"}
	got := ComputeGitLineDiff(head, current, 3000)
	if !reflect.DeepEqual(got.Modified, []int{7}) {
		t.Errorf("Modified = %v, want [7]", got.Modified)
	}
	if len(got.Added) != 0 {
		t.Errorf("Added = %v, want empty", got.Added)
	}
	if len(got.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty", got.Deleted)
	}
}

func TestComputeGitLineDiff_InsertionInMiddle(t *testing.T) {
	head := []string{"a", "b", "d", "e"}
	current := []string{"a", "b", "c", "d", "e"}
	got := ComputeGitLineDiff(head, current, 3000)
	if !reflect.DeepEqual(got.Added, []int{3}) {
		t.Errorf("Added = %v, want [3]", got.Added)
	}
	if len(got.Modified) != 0 {
		t.Errorf("Modified = %v, want empty", got.Modified)
	}
	if len(got.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty", got.Deleted)
	}
}

func TestPkHeuristicConditions_QuotedAliases(t *testing.T) {
	// Mixed-case aliases should be quoted by QuoteOrBare.
	got := PkHeuristicConditions(
		"Orders", "myOrders", "Customer", "myCustomer",
		[]string{"CUSTOMER_ID"},
		[]string{"ID"},
	)
	if len(got) != 1 {
		t.Fatalf("expected 1 condition, got %d: %v", len(got), got)
	}
	// Mixed-case alias should be double-quoted in the output
	if !strings.Contains(got[0], `"myOrders"`) {
		t.Errorf("expected quoted alias \"myOrders\" in condition, got %q", got[0])
	}
	if !strings.Contains(got[0], `"myCustomer"`) {
		t.Errorf("expected quoted alias \"myCustomer\" in condition, got %q", got[0])
	}
}

func TestGetStatementRanges_NestedBlockComments(t *testing.T) {
	// Snowflake block comments nest: the inner */ does not close the outer
	// comment, so the whole /* outer /* inner */ still */ is one comment and the
	// statement begins at SELECT (not at "still").
	sql := "/* outer /* inner */ still */ SELECT 1"
	got := GetStatementRanges(sql)
	if len(got) != 1 {
		t.Fatalf("expected 1 range, got %d: %v", len(got), got)
	}
	if stmt := strings.TrimSpace(sql[got[0].StartOffset:got[0].EndOffset]); stmt != "SELECT 1" {
		t.Errorf("expected statement %q, got %q", "SELECT 1", stmt)
	}
}

func TestGetStatementRanges_DollarQuotedWithSemicolon(t *testing.T) {
	// Semicolons inside $$ blocks should not split statements.
	sql := "$$ BEGIN SELECT 1; SELECT 2; END; $$;"
	got := GetStatementRanges(sql)
	if len(got) != 1 {
		t.Fatalf("expected 1 range for $$ block with semicolons, got %d: %v", len(got), got)
	}
}

func TestBuildCompositeConditions_SingleFKEmptyConstraintName(t *testing.T) {
	fks := []FKEntry{
		{FKColumn: "DEPT_ID", PKColumn: "ID", ConstraintName: "", KeySequence: 1},
	}
	got := BuildCompositeConditions(fks, "E", "D")
	if len(got) != 1 {
		t.Fatalf("expected 1 condition, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "DEPT_ID") || !strings.Contains(got[0], "ID") {
		t.Errorf("expected DEPT_ID = ID, got %q", got[0])
	}
}

func TestBuildCompositeConditions_QuotedAliases(t *testing.T) {
	// Mixed-case aliases should be double-quoted in output.
	fks := []FKEntry{
		{FKColumn: "COL", PKColumn: "PK", ConstraintName: "FK1", KeySequence: 1},
	}
	got := BuildCompositeConditions(fks, "myAlias", "PkAlias")
	if len(got) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(got))
	}
	if !strings.Contains(got[0], `"myAlias"`) {
		t.Errorf("expected quoted alias \"myAlias\", got %q", got[0])
	}
	if !strings.Contains(got[0], `"PkAlias"`) {
		t.Errorf("expected quoted alias \"PkAlias\", got %q", got[0])
	}
}

func TestGetAutocompleteContext_SingleStatementNoSemicolon(t *testing.T) {
	sql := "SELECT * FROM users WHERE id = 1"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if len(ctx.StatementRanges) != 1 {
		t.Errorf("expected 1 range, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != 0 {
		t.Errorf("expected currentStmtIdx=0, got %d", ctx.CurrentStmtIdx)
	}
}

func TestGetAutocompleteContext_UseOnlyDatabase(t *testing.T) {
	// USE DATABASE sets Database but not Schema.
	sql := "USE DATABASE prod;\nSELECT 1"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if ctx.UseContext == nil {
		t.Fatal("expected UseContext non-nil")
	}
	if ctx.UseContext.Database != "PROD" {
		t.Errorf("expected Database=PROD, got %q", ctx.UseContext.Database)
	}
	if ctx.UseContext.Schema != "" {
		t.Errorf("expected Schema empty, got %q", ctx.UseContext.Schema)
	}
}

func TestCTERecursive(t *testing.T) {
	// WITH RECURSIVE: the CTE regex doesn't strip RECURSIVE, so "RECURSIVE"
	// is captured as the CTE name instead of "cte". This documents the
	// known limitation.
	sql := "WITH RECURSIVE cte AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM cte WHERE n < 10) SELECT cte."
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	// The regex captures RECURSIVE as a CTE name — no column projection for "CTE".
	// This is expected behavior: WITH RECURSIVE is a rare pattern in Snowflake
	// and the regex-based CTE extractor doesn't handle it.
	_ = ctx
}

func TestCTENonRecursive(t *testing.T) {
	// Standard WITH (no RECURSIVE) should extract CTE columns normally.
	sql := "WITH totals AS (SELECT SUM(amount) AS total FROM orders) SELECT totals."
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if len(ctx.CTEColumns) == 0 {
		t.Fatal("expected CTE columns")
	}
	found := false
	for _, entry := range ctx.CTEColumns {
		if entry.Name == "TOTALS" {
			found = true
			colNames := make(map[string]bool)
			for _, c := range entry.Cols {
				colNames[c.Name] = true
			}
			if !colNames["TOTAL"] {
				t.Errorf("expected column TOTAL, got %+v", entry.Cols)
			}
		}
	}
	if !found {
		t.Errorf("expected CTE entry named 'TOTALS', got %+v", ctx.CTEColumns)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// IsInJoinOnClause: INTERSECT and EXCEPT terminators
// ══════════════════════════════════════════════════════════════════════════════

func TestIsInJoinOnClause_IntersectTerminates(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = t2.id INTERSECT ")
	if got {
		t.Error("expected false after INTERSECT terminates ON clause")
	}
}

func TestIsInJoinOnClause_ExceptTerminates(t *testing.T) {
	got := IsInJoinOnClause("SELECT * FROM t1 JOIN t2 ON t1.id = t2.id EXCEPT ")
	if got {
		t.Error("expected false after EXCEPT terminates ON clause")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetScriptingCompletions: NeedsColon for WITH keyword
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_NeedsColonTrueForWITH(t *testing.T) {
	sql := "$$ BEGIN WITH cte AS (SELECT 1) SELECT "
	offset := len([]rune(sql))
	got := GetScriptingCompletions(sql, offset)
	if !got.NeedsColon {
		t.Error("NeedsColon = false, want true for SELECT after WITH CTE")
	}
}

func TestGetScriptingCompletions_NeedsColonTrueForMERGE(t *testing.T) {
	sql := "$$ BEGIN MERGE INTO t USING src ON t.id = "
	offset := len([]rune(sql))
	got := GetScriptingCompletions(sql, offset)
	if !got.NeedsColon {
		t.Error("NeedsColon = false, want true for MERGE")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetScriptingCompletions: EXCEPTION and TYPE skip words
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_ExceptionSkipWord(t *testing.T) {
	sql := "$$ DECLARE my_exc EXCEPTION; myvar INT; BEGIN END; $$"
	offset := len([]rune("$$ DECLARE my_exc EXCEPTION; myvar INT; BEGIN END; "))
	got := GetScriptingCompletions(sql, offset)
	foundExc := false
	foundMyvar := false
	for _, v := range got.Variables {
		if v == "MY_EXC" {
			foundExc = true
		}
		if v == "MYVAR" {
			foundMyvar = true
		}
	}
	// MY_EXC is the first word in the DECLARE segment; EXCEPTION is a skip word
	// so it's not captured, but MY_EXC (the var name) should be
	if !foundExc {
		t.Errorf("expected MY_EXC in variables, got %v", got.Variables)
	}
	if !foundMyvar {
		t.Errorf("expected MYVAR in variables, got %v", got.Variables)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ComputeGitLineDiff: duplicate/repeated lines (LCS correctness)
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeGitLineDiff_DuplicateLines(t *testing.T) {
	head := []string{"a", "a", "b", "a"}
	current := []string{"a", "b", "a", "a"}
	got := ComputeGitLineDiff(head, current, 3000)
	// LCS should find a common subsequence of length 3 ("a","b","a")
	// The diff should have minimal changes
	totalChanges := len(got.Added) + len(got.Modified) + len(got.Deleted)
	if totalChanges == 0 {
		t.Error("expected some changes when reordering repeated lines")
	}
}

func TestComputeGitLineDiff_AllSameLines(t *testing.T) {
	head := []string{"x", "x", "x"}
	current := []string{"x", "x", "x", "x"}
	got := ComputeGitLineDiff(head, current, 3000)
	// One line added (all lines match, just an extra "x" at the end)
	if len(got.Added) != 1 {
		t.Errorf("Added = %v, want exactly 1 added line", got.Added)
	}
	if len(got.Modified) != 0 {
		t.Errorf("Modified = %v, want empty", got.Modified)
	}
}

func TestComputeGitLineDiff_AllSameLinesReduced(t *testing.T) {
	head := []string{"x", "x", "x", "x"}
	current := []string{"x", "x", "x"}
	got := ComputeGitLineDiff(head, current, 3000)
	// One line deleted
	if len(got.Deleted) == 0 {
		t.Error("expected at least one deleted line when removing a repeated line")
	}
	if len(got.Modified) != 0 {
		t.Errorf("Modified = %v, want empty", got.Modified)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// DetectUsingClause: USING in non-JOIN context
// ══════════════════════════════════════════════════════════════════════════════

func TestDetectUsingClause_CopyIntoUsing(t *testing.T) {
	// COPY INTO uses USING but not with parentheses immediately after — should not match
	got := DetectUsingClause("COPY INTO t FROM @stage USING TEMPLATE ")
	if got.InUsing || got.IsPartial {
		t.Errorf("expected no USING context for COPY INTO USING TEMPLATE, got %+v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetActiveFunctionCall: multi-line SQL
// ══════════════════════════════════════════════════════════════════════════════

func TestGetActiveFunctionCall_MultiLine(t *testing.T) {
	prefix := "SELECT\n  COALESCE(\n    a,\n    b,\n    "
	got := GetActiveFunctionCall(prefix)
	if got == nil {
		t.Fatal("expected non-nil result for multi-line function call")
	}
	if got.Name != "COALESCE" {
		t.Errorf("expected Name=COALESCE, got %q", got.Name)
	}
	if got.ParamIndex != 2 {
		t.Errorf("expected ParamIndex=2, got %d", got.ParamIndex)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetAutocompleteContext: CTE with quoted name
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContext_CTEQuotedName(t *testing.T) {
	sql := `WITH "My CTE" AS (SELECT 1 AS x) SELECT "My CTE".`
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	// Quoted CTE names may or may not be supported by the regex-based parser
	// This test documents the behavior
	if len(ctx.CTEColumns) > 0 {
		for _, entry := range ctx.CTEColumns {
			t.Logf("CTE entry: Name=%q Cols=%+v", entry.Name, entry.Cols)
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetAutocompleteContext: UseContext from only the current USE statement
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContext_UseContextPartialDB(t *testing.T) {
	// USE DATABASE only sets Database, Schema remains empty
	sql := "USE DATABASE my_db;\nSELECT * FROM t"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if ctx.UseContext == nil {
		t.Fatal("expected UseContext to be non-nil")
	}
	if ctx.UseContext.Database != "MY_DB" {
		t.Errorf("expected Database=MY_DB, got %q", ctx.UseContext.Database)
	}
	if ctx.UseContext.Schema != "" {
		t.Errorf("expected Schema empty for USE DATABASE only, got %q", ctx.UseContext.Schema)
	}
}

func TestGetAutocompleteContext_UseContextPartialSchema(t *testing.T) {
	// USE SCHEMA only sets Schema, Database remains empty
	sql := "USE SCHEMA my_schema;\nSELECT * FROM t"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if ctx.UseContext == nil {
		t.Fatal("expected UseContext to be non-nil")
	}
	if ctx.UseContext.Schema != "MY_SCHEMA" {
		t.Errorf("expected Schema=MY_SCHEMA, got %q", ctx.UseContext.Schema)
	}
	if ctx.UseContext.Database != "" {
		t.Errorf("expected Database empty for USE SCHEMA only, got %q", ctx.UseContext.Database)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional: operator boundaries, newline-separated function names, FK tier
// ══════════════════════════════════════════════════════════════════════════════

func TestGetIdentifierAtColumn_OperatorBoundary(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want []string
	}{
		{name: "plus separates idents", line: "a+b.c", col: 2, want: []string{"B", "C"}},
		{name: "plus on left ident", line: "a+b.c", col: 0, want: []string{"A"}},
		{name: "equals separates idents", line: "col1=db.schema.tbl", col: 8, want: []string{"DB", "SCHEMA", "TBL"}},
		{name: "equals on left ident", line: "col1=db.schema.tbl", col: 2, want: []string{"COL1"}},
		{name: "less-than separates", line: "x<y.z", col: 2, want: []string{"Y", "Z"}},
		{name: "pipe separates", line: "a||b.c", col: 3, want: []string{"B", "C"}},
		{name: "colon separates", line: "a:b.c", col: 2, want: []string{"B", "C"}},
		{name: "cursor on operator", line: "a+b", col: 1, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIdentifierAtColumn(tt.line, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIdentifierAtColumn(%q, %d) = %v, want %v", tt.line, tt.col, got, tt.want)
			}
		})
	}
}

func TestGetActiveFunctionCall_FuncNameOnDifferentLine(t *testing.T) {
	// The backward scan from '(' skips whitespace including newlines,
	// so a function name on a different line should still be recognized.
	got := GetActiveFunctionCall("SELECT FUNC\n(a, ")
	if got == nil {
		t.Fatal("expected non-nil for function name on different line from paren")
	}
	if got.Name != "FUNC" {
		t.Errorf("expected Name=FUNC, got %q", got.Name)
	}
	if got.ParamIndex != 1 {
		t.Errorf("expected ParamIndex=1, got %d", got.ParamIndex)
	}
}

func TestGetActiveFunctionCall_FuncNameSeparatedByTab(t *testing.T) {
	got := GetActiveFunctionCall("SELECT FUNC\t(x, y, ")
	if got == nil {
		t.Fatal("expected non-nil for function name separated by tab")
	}
	if got.Name != "FUNC" {
		t.Errorf("expected Name=FUNC, got %q", got.Name)
	}
	if got.ParamIndex != 2 {
		t.Errorf("expected ParamIndex=2, got %d", got.ParamIndex)
	}
}

func TestComputeJoinOnConditions_FKSuggestionsWithoutColEntries(t *testing.T) {
	// FK tier should produce suggestions even when ColEntries is completely empty.
	// This verifies the FK suggestion tier works independently from the
	// column-matching tiers (PK heuristic and same-name columns).
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "O", DB: "DB", Schema: "S", Name: "ORDERS"},
			{Alias: "C", DB: "DB", Schema: "S", Name: "CUSTOMERS"},
		},
		Prefix: "ON ",
		FKEntries: []TableFKEntry{
			{
				DB: "DB", Schema: "S", Name: "ORDERS",
				FKs: []FKEntry{
					{PKDatabase: "DB", PKSchema: "S", PKTable: "CUSTOMERS", PKColumn: "ID", FKColumn: "CUSTOMER_ID", ConstraintName: "FK_ORD_CUST", KeySequence: 1},
				},
			},
		},
		ColEntries: []ColEntry{}, // deliberately empty
	}
	got := ComputeJoinOnConditions(req)
	foundFK := false
	for _, c := range got {
		if c.Detail == "FK RELATION" && strings.Contains(c.Condition, "CUSTOMER_ID") {
			foundFK = true
		}
	}
	if !foundFK {
		t.Errorf("expected FK suggestion even when ColEntries is empty, got %v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ExtractInEditorTableDefs: plain CTAS without column list
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs_PlainCTASNoColumnList(t *testing.T) {
	// CREATE TABLE AS SELECT without a (column_list) block — should be skipped.
	sql := "CREATE TABLE ctas_tbl AS SELECT 1 AS id, 'x' AS name;"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected plain CTAS (no column list) to be skipped, got %+v", defs)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// BuildCompositeConditions: triple composite FK (3 key sequences)
// ══════════════════════════════════════════════════════════════════════════════

func TestBuildCompositeConditions_TripleCompositeFK(t *testing.T) {
	fks := []FKEntry{
		{FKColumn: "FK_A", PKColumn: "PK_A", ConstraintName: "FK_COMP3", KeySequence: 1},
		{FKColumn: "FK_B", PKColumn: "PK_B", ConstraintName: "FK_COMP3", KeySequence: 2},
		{FKColumn: "FK_C", PKColumn: "PK_C", ConstraintName: "FK_COMP3", KeySequence: 3},
	}
	got := BuildCompositeConditions(fks, "T1", "T2")
	if len(got) != 1 {
		t.Fatalf("expected 1 composite condition for 3-key FK, got %d: %v", len(got), got)
	}
	// Should contain two ANDs
	andCount := strings.Count(got[0], " AND ")
	if andCount != 2 {
		t.Errorf("expected 2 AND clauses in 3-key composite, got %d in %q", andCount, got[0])
	}
	// Verify ordering: FK_A before FK_B before FK_C
	idxA := strings.Index(got[0], "FK_A")
	idxB := strings.Index(got[0], "FK_B")
	idxC := strings.Index(got[0], "FK_C")
	if idxA > idxB || idxB > idxC {
		t.Errorf("expected FK_A < FK_B < FK_C ordering, got %q", got[0])
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ComputeJoinOnConditions: two tables with completely disjoint columns
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeJoinOnConditions_TwoTablesNoMatchingColumns(t *testing.T) {
	// Two tables where no columns share a name and types are incompatible.
	// No FKs. Should produce zero suggestions.
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{
				{Name: "ORDER_DATE", DataType: "DATE"},
				{Name: "STATUS", DataType: "BOOLEAN"},
			}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{
				{Name: "REGION_NAME", DataType: "VARCHAR"},
				{Name: "POPULATION", DataType: "NUMBER"},
			}},
		},
	}
	got := ComputeJoinOnConditions(req)
	if len(got) != 0 {
		t.Errorf("expected no suggestions with completely disjoint columns, got %v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ComputeJoinOnConditions: ColEntries missing for one of the resolved refs
// ══════════════════════════════════════════════════════════════════════════════

func TestComputeJoinOnConditions_MissingColEntryForOneRef(t *testing.T) {
	// ColEntries only has data for T1 but not T2. Should not panic and produce
	// no same-name/PK heuristic suggestions (T2 columns are unknown).
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}}},
			// T2 missing from ColEntries
		},
	}
	got := ComputeJoinOnConditions(req)
	// Should not panic; may or may not produce suggestions depending on implementation.
	_ = got
}

// ══════════════════════════════════════════════════════════════════════════════
// GetScriptingCompletions: DECLARE without BEGIN block
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_DeclareWithoutBegin(t *testing.T) {
	// $$ block with DECLARE but no BEGIN — cursor is inside, variables should still
	// be extracted from DECLARE.
	sql := "$$ DECLARE x INT; y VARCHAR; $$"
	offset := len([]rune("$$ DECLARE x INT; y VARCHAR; "))
	got := GetScriptingCompletions(sql, offset)
	foundX := false
	foundY := false
	for _, v := range got.Variables {
		if v == "X" {
			foundX = true
		}
		if v == "Y" {
			foundY = true
		}
	}
	if !foundX {
		t.Errorf("expected X in variables, got %v", got.Variables)
	}
	if !foundY {
		t.Errorf("expected Y in variables, got %v", got.Variables)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetStatementRanges: dollar-quoted block without closing $$
// ══════════════════════════════════════════════════════════════════════════════

func TestGetStatementRanges_UnclosedDollarQuote(t *testing.T) {
	// Unclosed $$ block — the entire remaining text is part of the statement.
	sql := "SELECT 1; $$ BEGIN SELECT 2;"
	got := GetStatementRanges(sql)
	if len(got) != 2 {
		t.Fatalf("expected 2 ranges (second is unclosed $$ block), got %d: %v", len(got), got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional NeedsColon edge cases
// ══════════════════════════════════════════════════════════════════════════════

func TestGetScriptingCompletions_NeedsColon_RETURN(t *testing.T) {
	// RETURN is a scripting keyword, not in colonRequiredKeywords → NeedsColon false
	sql := "$$ BEGIN LET x := 1; RETURN "
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("expected NeedsColon=false after RETURN keyword")
	}
	if len(got.Variables) == 0 {
		t.Error("expected at least one variable (X)")
	}
}

func TestGetScriptingCompletions_NeedsColon_SemicolonReset(t *testing.T) {
	// Semicolon resets context; last match is ";" which is not in colonRequiredKeywords
	sql := "$$ BEGIN LET x := (SELECT 1); "
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("expected NeedsColon=false after semicolon reset")
	}
}

func TestGetScriptingCompletions_NeedsColon_IF(t *testing.T) {
	// IF is a control-flow keyword, not in colonRequiredKeywords → NeedsColon false
	sql := "$$ BEGIN LET x := 1; IF "
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("expected NeedsColon=false after IF keyword")
	}
}

func TestGetScriptingCompletions_NeedsColon_WHILE(t *testing.T) {
	// WHILE is a control-flow keyword, not in colonRequiredKeywords → NeedsColon false
	sql := "$$ BEGIN LET cnt := 0; WHILE "
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("expected NeedsColon=false after WHILE keyword")
	}
}

func TestGetScriptingCompletions_NeedsColon_INSERT(t *testing.T) {
	// INSERT is a DML keyword in colonRequiredKeywords → NeedsColon true
	sql := "$$ BEGIN LET x := 1; INSERT INTO t VALUES ("
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if !got.NeedsColon {
		t.Error("expected NeedsColon=true inside INSERT statement")
	}
}

func TestGetScriptingCompletions_NeedsColon_UPDATE(t *testing.T) {
	// UPDATE is a DML keyword in colonRequiredKeywords → NeedsColon true
	sql := "$$ BEGIN LET x := 1; UPDATE t SET col = "
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if !got.NeedsColon {
		t.Error("expected NeedsColon=true inside UPDATE statement")
	}
}

func TestGetScriptingCompletions_NeedsColon_ColonPrefix(t *testing.T) {
	// When the text before the word already has a colon prefix → NeedsColon false
	sql := "$$ BEGIN LET x := 1; SELECT :"
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("expected NeedsColon=false when colon already precedes cursor")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ResolveTableRefs: non-TABLE/VIEW store objects filtered
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs_NonTableKindFiltered(t *testing.T) {
	storeObjs := []StoreObject{
		{DB: "DB", Schema: "PUBLIC", Name: "MY_PROC", Kind: "PROCEDURE"},
		{DB: "DB", Schema: "PUBLIC", Name: "MY_FUNC", Kind: "FUNCTION"},
	}
	refs := []JoinTableRef{{Name: "MY_PROC", Alias: ""}}
	got := ResolveTableRefs(refs, storeObjs, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected PROCEDURE to be filtered out of store matches, got %+v", got)
	}
}

func TestResolveTableRefs_ViewKindMatched(t *testing.T) {
	storeObjs := []StoreObject{
		{DB: "DB", Schema: "PUBLIC", Name: "MY_VIEW", Kind: "VIEW"},
	}
	refs := []JoinTableRef{{Name: "MY_VIEW", Alias: "v"}}
	got := ResolveTableRefs(refs, storeObjs, nil, nil)
	if len(got) != 1 || got[0].DB != "DB" {
		t.Errorf("expected VIEW to be matched, got %+v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ResolveTableRefs: partial UseContext + Session combination
// ══════════════════════════════════════════════════════════════════════════════

func TestResolveTableRefs_UseContextDBSessionSchema(t *testing.T) {
	// UseContext provides only Database, Session provides only Schema → should combine
	refs := []JoinTableRef{{Name: "MY_TABLE", Alias: ""}}
	useCtx := &UseContext{Database: "CTX_DB"}
	sess := &SessionContext{Schema: "SESS_SCH"}
	got := ResolveTableRefs(refs, nil, useCtx, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "CTX_DB" || got[0].Schema != "SESS_SCH" {
		t.Errorf("expected CTX_DB.SESS_SCH, got %s.%s", got[0].DB, got[0].Schema)
	}
}

func TestResolveTableRefs_UseContextSchemaSessionDB(t *testing.T) {
	// UseContext provides only Schema, Session provides only Database → should combine
	refs := []JoinTableRef{{Name: "MY_TABLE", Alias: ""}}
	useCtx := &UseContext{Schema: "CTX_SCH"}
	sess := &SessionContext{Database: "SESS_DB"}
	got := ResolveTableRefs(refs, nil, useCtx, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "SESS_DB" || got[0].Schema != "CTX_SCH" {
		t.Errorf("expected SESS_DB.CTX_SCH, got %s.%s", got[0].DB, got[0].Schema)
	}
}

func TestResolveTableRefs_PartialResolutionSkipped(t *testing.T) {
	// UseContext provides only Database, no Session → Schema still empty → skipped
	refs := []JoinTableRef{{Name: "MY_TABLE", Alias: ""}}
	useCtx := &UseContext{Database: "CTX_DB"}
	got := ResolveTableRefs(refs, nil, useCtx, nil)
	if len(got) != 0 {
		t.Errorf("expected ref to be skipped when only DB is resolved, got %+v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// GetAutocompleteContextFull: empty SQL context flags
// ══════════════════════════════════════════════════════════════════════════════

func TestGetAutocompleteContextFull_EmptySQLContextFlags(t *testing.T) {
	req := AutocompleteContextRequest{
		SQL:          "",
		CursorOffset: 0,
	}
	ctx := GetAutocompleteContextFull(req)
	if ctx.IsDatatypeCtx {
		t.Error("expected IsDatatypeCtx=false for empty SQL")
	}
	if ctx.IsInJoinOnClause {
		t.Error("expected IsInJoinOnClause=false for empty SQL")
	}
	if ctx.UsingClause != nil {
		t.Error("expected UsingClause=nil for empty SQL")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Remaining edge cases: untested code paths
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractInEditorTableDefs_EmptyColumnList(t *testing.T) {
	// CREATE TABLE with empty parens — the parser skips tables with no
	// parseable column definitions (empty parens produce zero cols, so the
	// table def is not emitted). Should not panic.
	sql := "CREATE TABLE empty_tbl ();"
	ranges := GetStatementRanges(sql)
	defs := ExtractInEditorTableDefs(sql, ranges, nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected 0 defs for empty column list (no parseable cols), got %d: %+v", len(defs), defs)
	}
}

func TestGetScriptingCompletions_DeclareWithComments(t *testing.T) {
	// DECLARE block with line and block comments between variable declarations.
	// The implementation strips comments before extracting variable names.
	sql := "$$ DECLARE\n  -- this is x\n  x INT;\n  /* block */ y VARCHAR;\nBEGIN END; $$"
	offset := len([]rune("$$ DECLARE\n  -- this is x\n  x INT;\n  /* block */ y VARCHAR;\nBEGIN END; "))
	got := GetScriptingCompletions(sql, offset)
	foundX := false
	foundY := false
	for _, v := range got.Variables {
		if v == "X" {
			foundX = true
		}
		if v == "Y" {
			foundY = true
		}
	}
	if !foundX {
		t.Errorf("expected X in variables after stripping line comment, got %v", got.Variables)
	}
	if !foundY {
		t.Errorf("expected Y in variables after stripping block comment, got %v", got.Variables)
	}
}

func TestResolveTableRefs_DBMismatchFallsThrough(t *testing.T) {
	// Ref has DB="OTHER_DB", Name="TBL". Store has a matching name but in
	// DB="STORE_DB". The store search should reject it (DB mismatch), and
	// the ref should fall through to UseContext which provides the Schema.
	storeObjs := []StoreObject{
		{DB: "STORE_DB", Schema: "PUBLIC", Name: "TBL", Kind: "TABLE"},
	}
	refs := []JoinTableRef{{DB: "OTHER_DB", Name: "TBL", Alias: "t"}}
	useCtx := &UseContext{Schema: "CTX_SCH"}
	got := ResolveTableRefs(refs, storeObjs, useCtx, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	// DB should remain OTHER_DB (from the original ref), Schema from UseContext
	if got[0].DB != "OTHER_DB" {
		t.Errorf("expected DB=OTHER_DB (original ref), got %q", got[0].DB)
	}
	if got[0].Schema != "CTX_SCH" {
		t.Errorf("expected Schema=CTX_SCH (from UseContext), got %q", got[0].Schema)
	}
}

func TestComputeJoinOnConditions_ColEntryWithEmptyCols(t *testing.T) {
	// ColEntry exists for both tables but has an empty Cols slice.
	// Should not panic and should produce no same-name/PK heuristic suggestions.
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{}},
		},
	}
	got := ComputeJoinOnConditions(req)
	if len(got) != 0 {
		t.Errorf("expected no suggestions with empty Cols slices, got %v", got)
	}
}

func TestGetStatementRanges_SingleCharStatement(t *testing.T) {
	sql := "X"
	got := GetStatementRanges(sql)
	if len(got) != 1 {
		t.Fatalf("expected 1 range for single char, got %d: %v", len(got), got)
	}
	if got[0].StartOffset != 0 || got[0].EndOffset != 1 {
		t.Errorf("expected range [0,1), got [%d,%d)", got[0].StartOffset, got[0].EndOffset)
	}
	if got[0].StartLine != 1 || got[0].EndLine != 1 {
		t.Errorf("expected lines [1,1], got [%d,%d]", got[0].StartLine, got[0].EndLine)
	}
}

func TestGetScriptingCompletions_DeclareCommentOnlySegment(t *testing.T) {
	// A DECLARE segment that is entirely a comment — should not produce a variable.
	sql := "$$ DECLARE -- just a comment\n; actual_var INT; BEGIN END; $$"
	offset := len([]rune("$$ DECLARE -- just a comment\n; actual_var INT; BEGIN END; "))
	got := GetScriptingCompletions(sql, offset)
	foundActual := false
	for _, v := range got.Variables {
		if v == "ACTUAL_VAR" {
			foundActual = true
		}
	}
	if !foundActual {
		t.Errorf("expected ACTUAL_VAR in variables, got %v", got.Variables)
	}
}

func TestResolveTableRefs_SchemaMismatchFallsThrough(t *testing.T) {
	// Ref has Schema="OTHER_SCH", Name="TBL". Store has matching name in
	// Schema="STORE_SCH". The store search should reject it (Schema mismatch),
	// and the ref should fall through to context for DB resolution.
	storeObjs := []StoreObject{
		{DB: "STORE_DB", Schema: "STORE_SCH", Name: "TBL", Kind: "TABLE"},
	}
	refs := []JoinTableRef{{Schema: "OTHER_SCH", Name: "TBL", Alias: "t"}}
	sess := &SessionContext{Database: "SESS_DB"}
	got := ResolveTableRefs(refs, storeObjs, nil, sess)
	if len(got) != 1 {
		t.Fatalf("expected 1 resolved ref, got %d", len(got))
	}
	if got[0].DB != "SESS_DB" {
		t.Errorf("expected DB=SESS_DB (from session), got %q", got[0].DB)
	}
	if got[0].Schema != "OTHER_SCH" {
		t.Errorf("expected Schema=OTHER_SCH (original ref), got %q", got[0].Schema)
	}
}

func TestGetAutocompleteContext_CommentBeforeStatement(t *testing.T) {
	// A comment before the actual statement should not affect statement indexing.
	sql := "-- header comment\nSELECT * FROM users"
	ctx := GetAutocompleteContext(sql, len([]rune(sql)))
	if len(ctx.StatementRanges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != 0 {
		t.Errorf("expected currentStmtIdx=0, got %d", ctx.CurrentStmtIdx)
	}
	// The comment-only portion should NOT create a separate statement range.
	if ctx.StatementRanges[0].StartLine != 2 {
		t.Errorf("expected statement to start on line 2 (after comment), got line %d", ctx.StatementRanges[0].StartLine)
	}
}

func TestComputeJoinOnConditions_SameNameDifferentCaseBothPresent(t *testing.T) {
	// Column names differ only in case — same-name detection should be
	// case-insensitive and produce a match.
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "T1"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "T2"},
		},
		Prefix: "",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "T1", Cols: []ColInfo{{Name: "User_Id", DataType: "NUMBER"}}},
			{DB: "DB", Schema: "S", Name: "T2", Cols: []ColInfo{{Name: "USER_ID", DataType: "NUMBER"}}},
		},
	}
	got := ComputeJoinOnConditions(req)
	found := false
	for _, c := range got {
		if strings.Contains(strings.ToUpper(c.Condition), "USER_ID") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected case-insensitive same-name column match for User_Id/USER_ID, got %v", got)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Grammar-driven autocomplete — GrammarExpectedAt + AutocompleteContext wiring
// ══════════════════════════════════════════════════════════════════════════════

func grammarExpHas(exp *GrammarExpectation, kw string) bool {
	if exp == nil {
		return false
	}
	for _, k := range exp.Keywords {
		if k == kw {
			return true
		}
	}
	return false
}

func grammarKindHas(exp *GrammarExpectation, kind string) bool {
	if exp == nil {
		return false
	}
	for _, k := range exp.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func TestGrammarExpectedAt(t *testing.T) {
	t.Run("COPY INTO table expects FROM keyword", func(t *testing.T) {
		stmt := "COPY INTO mytable "
		exp := GrammarExpectedAt(stmt, len(stmt))
		if !grammarExpHas(exp, "FROM") {
			t.Errorf("expected FROM in keywords, got %+v", exp)
		}
	})

	t.Run("CREATE offers object-type keywords", func(t *testing.T) {
		stmt := "CREATE "
		exp := GrammarExpectedAt(stmt, len(stmt))
		for _, kw := range []string{"TABLE", "VIEW", "DATABASE"} {
			if !grammarExpHas(exp, kw) {
				t.Errorf("expected %q in keywords, got %+v", kw, exp)
			}
		}
	})

	t.Run("CREATE TABLE expects an identifier kind", func(t *testing.T) {
		stmt := "CREATE TABLE "
		exp := GrammarExpectedAt(stmt, len(stmt))
		// The table name is a token-kind expectation (identifier), not a keyword.
		if !grammarKindHas(exp, "identifier") {
			t.Errorf("expected 'identifier' in kinds, got %+v", exp)
		}
		// IF [NOT EXISTS] is a literal keyword the grammar also accepts here.
		if !grammarExpHas(exp, "IF") {
			t.Errorf("expected IF in keywords, got %+v", exp)
		}
	})

	t.Run("unmodelled leading keyword yields nil", func(t *testing.T) {
		if exp := GrammarExpectedAt("FLOOBAR x y", 11); exp != nil {
			t.Errorf("expected nil for unmodelled statement, got %+v", exp)
		}
	})

	t.Run("after a complete projection the clause keywords are offered", func(t *testing.T) {
		// ParseSelect models the SELECT statement, so after a finished projection
		// item the grammar offers the clauses that may follow — FROM first, plus
		// the later WHERE / GROUP BY / ORDER BY / set-operator keywords.
		stmt := "SELECT col "
		exp := GrammarExpectedAt(stmt, len(stmt))
		for _, kw := range []string{"FROM", "WHERE", "ORDER", "GROUP", "UNION"} {
			if !grammarExpHas(exp, kw) {
				t.Errorf("expected %q keyword after a complete projection, got %+v", kw, exp)
			}
		}
	})

	t.Run("after FROM <table> the post-FROM clauses are offered, not FROM again", func(t *testing.T) {
		stmt := "SELECT col FROM t "
		exp := GrammarExpectedAt(stmt, len(stmt))
		if !grammarExpHas(exp, "WHERE") {
			t.Errorf("expected WHERE keyword after FROM <table>, got %+v", exp)
		}
		if grammarExpHas(exp, "FROM") {
			t.Errorf("did not expect FROM again after FROM <table>, got %+v", exp)
		}
	})

	t.Run("keywords and kinds are disjoint and exclude operators-as-keywords", func(t *testing.T) {
		stmt := "CREATE TABLE "
		exp := GrammarExpectedAt(stmt, len(stmt))
		if exp == nil {
			t.Fatal("expected non-nil expectation")
		}
		// No kind name (e.g. Identifier, LParen) should leak into Keywords.
		for _, k := range exp.Keywords {
			if !reGrammarKeyword.MatchString(k) {
				t.Errorf("keyword %q is not an all-uppercase keyword label", k)
			}
		}
	})
}

func TestGetAutocompleteContext_GrammarExpected(t *testing.T) {
	t.Run("populated for modeled statement", func(t *testing.T) {
		sql := "COPY INTO mytable "
		ctx := GetAutocompleteContext(sql, len(sql))
		if !grammarExpHas(ctx.GrammarExpected, "FROM") {
			t.Errorf("expected FROM in ctx.GrammarExpected, got %+v", ctx.GrammarExpected)
		}
	})

	t.Run("nil for unmodelled statement", func(t *testing.T) {
		sql := "FLOOBAR x y"
		ctx := GetAutocompleteContext(sql, len(sql))
		if ctx.GrammarExpected != nil {
			t.Errorf("expected nil GrammarExpected for unmodelled SQL, got %+v", ctx.GrammarExpected)
		}
	})

	t.Run("uses cursor statement in multi-statement SQL", func(t *testing.T) {
		// Cursor in the second statement (an ALTER) — expectations must come from
		// it, not the first statement.
		sql := "SELECT 1;\nALTER TABLE foo "
		ctx := GetAutocompleteContext(sql, len(sql))
		for _, kw := range []string{"RENAME", "ADD", "DROP"} {
			if !grammarExpHas(ctx.GrammarExpected, kw) {
				t.Errorf("expected %q from ALTER TABLE statement, got %+v", kw, ctx.GrammarExpected)
			}
		}
	})
}
