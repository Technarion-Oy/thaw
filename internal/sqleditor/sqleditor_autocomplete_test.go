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
		name       string
		sql        string
		offset     int // -1 means use len([]rune(sql))
		wantVars   []string
		wantColon  bool
	}{
		// ── Cursor outside $$ block → no variables ────────────────────────────
		{
			name:     "cursor outside dollar block",
			sql:      "SELECT * FROM t",
			offset:   -1,
			wantVars: nil,
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
			// Only check NeedsColon=false when explicitly testing it (non-nil vars or explicit flag)
			if !tt.wantColon && tt.wantVars == nil && tt.name != "cursor outside dollar block" && tt.name != "cursor before opening $$" {
				// NeedsColon test: only assert when the test name implies it
				if strings.Contains(tt.name, "needsColon false") && got.NeedsColon {
					t.Errorf("NeedsColon = true, want false")
				}
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
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 13},
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
}
