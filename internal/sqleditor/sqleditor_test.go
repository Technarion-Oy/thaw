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
	"testing"
)

func TestGetIdentifierAtColumn(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want []string
	}{
		// No identifier at all
		{name: "empty line", line: "", col: 0, want: nil},
		{name: "only spaces", line: "   ", col: 1, want: nil},
		{name: "col on operator", line: "a + b", col: 2, want: nil},

		// Bare single-part identifier
		{name: "bare word col at start", line: "SELECT", col: 0, want: []string{"SELECT"}},
		{name: "bare word col in middle", line: "SELECT", col: 3, want: []string{"SELECT"}},
		{name: "bare word col at end", line: "SELECT", col: 5, want: []string{"SELECT"}},
		{name: "bare word col one past end", line: "SELECT", col: 6, want: nil},

		// Two-part identifier
		{name: "two-part on first part", line: "db.schema", col: 0, want: []string{"db", "schema"}},
		{name: "two-part on dot", line: "db.schema", col: 2, want: []string{"db", "schema"}},
		{name: "two-part on second part", line: "db.schema", col: 4, want: []string{"db", "schema"}},

		// Three-part identifier (db.schema.table)
		{name: "three-part on first", line: "db.schema.tbl", col: 1, want: []string{"db", "schema", "tbl"}},
		{name: "three-part on middle", line: "db.schema.tbl", col: 5, want: []string{"db", "schema", "tbl"}},
		{name: "three-part on last", line: "db.schema.tbl", col: 11, want: []string{"db", "schema", "tbl"}},

		// Quoted identifier
		{name: "quoted single part", line: `"My Table"`, col: 3, want: []string{"My Table"}},
		{name: "quoted two-part", line: `"DB"."My Schema"`, col: 5, want: []string{"DB", "My Schema"}},
		{name: "col before opening quote", line: ` "tbl"`, col: 0, want: nil},

		// Identifier embedded in SQL
		{name: "ident in query col on ident", line: "SELECT * FROM db.schema.tbl WHERE x=1", col: 20, want: []string{"db", "schema", "tbl"}},
		{name: "ident in query col on keyword before ident", line: "SELECT * FROM db.schema.tbl WHERE x=1", col: 8, want: nil},

		// Underscore in identifier
		{name: "identifier with underscores", line: "my_db.my_schema", col: 3, want: []string{"my_db", "my_schema"}},

		// Digits in identifier
		{name: "identifier with digits", line: "table1.col2", col: 4, want: []string{"table1", "col2"}},
		// Digit-led tokens: \w matches digits, so "123abc" is treated as one token (same as original TS /\w/)
		{name: "identifier starting with digit matched as token", line: "123abc", col: 0, want: []string{"123abc"}},

		// Two separate identifiers on the same line
		{name: "two idents on line - col on first", line: "t1.c1, t2.c2", col: 1, want: []string{"t1", "c1"}},
		{name: "two idents on line - col on second", line: "t1.c1, t2.c2", col: 8, want: []string{"t2", "c2"}},
		{name: "two idents on line - col on comma", line: "t1.c1, t2.c2", col: 5, want: nil},
		{name: "two idents on line - col on space", line: "t1.c1, t2.c2", col: 6, want: nil},

		// Trailing dot — cursor before/on the dangling dot
		{name: "trailing dot - col on word before dot", line: "db.", col: 1, want: []string{"db"}},
		{name: "trailing dot - col on dangling dot", line: "db.", col: 2, want: nil},

		// Leading dot — scanner skips the dot; "schema" is returned as a 1-part identifier
		{name: "leading dot - col on word after dot", line: ".schema", col: 1, want: []string{"schema"}},
		// Leading dot — col on the dot itself is not on any identifier
		{name: "leading dot - col on the dot", line: ".schema", col: 0, want: nil},

		// Mixed quoted and bare parts
		{name: "quoted-dot-bare col on quoted", line: `"My DB".schema`, col: 3, want: []string{"My DB", "schema"}},
		{name: "quoted-dot-bare col on bare", line: `"My DB".schema`, col: 9, want: []string{"My DB", "schema"}},

		// col at very last character of identifier
		{name: "col at last char of three-part", line: "a.b.c", col: 4, want: []string{"a", "b", "c"}},

		// col past end of line
		{name: "col past end of line", line: "abc", col: 10, want: nil},

		// Identifier immediately after opening paren
		{name: "ident after paren", line: "(db.schema)", col: 4, want: []string{"db", "schema"}},
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

func TestGetActiveFunctionCall(t *testing.T) {
	type want = *FunctionCallContext // nil means "expect nil"
	fc := func(name string, idx int) *FunctionCallContext {
		return &FunctionCallContext{Name: name, ParamIndex: idx}
	}

	tests := []struct {
		name   string
		prefix string
		want   want
	}{
		// ── not inside any call ──────────────────────────────────────────────
		{name: "empty prefix", prefix: "", want: nil},
		{name: "no parens", prefix: "SELECT a, b FROM t", want: nil},
		{name: "closed call", prefix: "SELECT ABS(x)", want: nil},

		// ── basic single-argument calls ───────────────────────────────────
		{name: "first param", prefix: "SELECT ABS(", want: fc("ABS", 0)},
		{name: "second param via comma", prefix: "SELECT CONCAT('hi', ", want: fc("CONCAT", 1)},
		{name: "third param", prefix: "SELECT DATEADD(year, 1, ", want: fc("DATEADD", 2)},

		// ── nested calls — innermost wins ────────────────────────────────
		{name: "nested: cursor in inner call", prefix: "SELECT OUTER(INNER(", want: fc("INNER", 0)},
		{name: "nested: cursor back in outer", prefix: "SELECT OUTER(INNER(x), ", want: fc("OUTER", 1)},

		// ── string handling ──────────────────────────────────────────────
		{name: "comma inside single-quoted string not counted", prefix: "SELECT F('a,b', ", want: fc("F", 1)},
		{name: "single-quote '' escape handled", prefix: "SELECT F('it''s', ", want: fc("F", 1)},
		{name: "paren inside string not counted", prefix: "SELECT F('(not a call)', ", want: fc("F", 1)},

		// ── comment handling ─────────────────────────────────────────────
		{name: "comma in line comment not counted", prefix: "SELECT F(x -- , extra\n, ", want: fc("F", 1)},
		{name: "comma in block comment not counted", prefix: "SELECT F(x /* , */ , ", want: fc("F", 1)},

		// ── double-quoted identifiers ────────────────────────────────────
		{name: "double-quoted ident in arg does not break stack", prefix: `SELECT F("col,name", `, want: fc("F", 1)},

		// ── nameless paren (subexpression) ───────────────────────────────
		// "SELECT (" — "SELECT" is the word before '(' so it is captured as the
		// function name.  GetFunctionTooltip("SELECT") returns nothing, so
		// signature help is still nil end-to-end.
		{name: "keyword before paren treated as fn name", prefix: "SELECT (1 + ", want: fc("SELECT", 0)},
		// Inner nameless '(' captures the ',' — outer F() frame doesn't see it.
		{name: "comma inside nameless subexpr not propagated to outer", prefix: "SELECT F(x + (1, ", want: nil},
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

func TestParseSignatureParams(t *testing.T) {
	tests := []struct {
		name string
		sig  string
		want []SignatureParam
	}{
		// ── no / empty params ────────────────────────────────────────────
		{name: "no opening paren", sig: "GETDATE", want: nil},
		{name: "empty params", sig: "GETDATE()", want: nil},
		{name: "no closing paren", sig: "F(a, b", want: nil},

		// ── single param ─────────────────────────────────────────────────
		{name: "single param", sig: "ABS(numeric_expr)",
			want: []SignatureParam{{Start: 4, End: 16}}},

		// ── multiple params ───────────────────────────────────────────────
		{name: "two params", sig: "CONCAT(str1, str2)",
			want: []SignatureParam{{Start: 7, End: 11}, {Start: 13, End: 17}}},
		{name: "three params", sig: "DATEADD(date_part, value, date_expr)",
			want: []SignatureParam{{Start: 8, End: 17}, {Start: 19, End: 24}, {Start: 26, End: 35}}},

		// ── nested parens inside signature (type annotation) ─────────────
		{name: "nested parens in type", sig: "F(a ARRAY(INT), b VARCHAR)",
			want: []SignatureParam{{Start: 2, End: 14}, {Start: 16, End: 25}}},

		// ── whitespace trimming ───────────────────────────────────────────
		{name: "extra spaces trimmed", sig: "F(  param1  ,  param2  )",
			want: []SignatureParam{{Start: 4, End: 10}, {Start: 15, End: 21}}},
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

func TestFindTokenPositions(t *testing.T) {
	type tm = TokenMatch // shorthand

	tests := []struct {
		name         string
		sql          string
		bareTargets  []string
		quotedTargets []string
		want         []TokenMatch
	}{
		// ── nil / empty inputs ──────────────────────────────────────────────
		{name: "empty sql", sql: "", bareTargets: []string{"X"}, want: nil},
		{name: "no targets", sql: "SELECT col FROM t", want: nil},
		{name: "target not present", sql: "SELECT a FROM t", bareTargets: []string{"MISSING"}, want: nil},

		// ── bare word matches ───────────────────────────────────────────────
		{
			name:        "single bare word match",
			sql:         "SELECT bad_col FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        []tm{{Name: "bad_col", Line: 1, Col: 8, EndCol: 15, Quoted: false}},
		},
		{
			name:        "case-insensitive bare match",
			sql:         "SELECT Bad_Col FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        []tm{{Name: "Bad_Col", Line: 1, Col: 8, EndCol: 15, Quoted: false}},
		},
		{
			name:        "bare word on line 3",
			sql:         "SELECT\n  x,\n  bad_col\nFROM t",
			bareTargets: []string{"BAD_COL"},
			want:        []tm{{Name: "bad_col", Line: 3, Col: 3, EndCol: 10, Quoted: false}},
		},
		{
			name:        "bare word after dot is not matched (qualified ref)",
			sql:         "SELECT t.bad_col FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        nil,
		},
		{
			name:        "bare word before '(' is not matched (function call)",
			sql:         "SELECT bad_col() FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        nil,
		},
		{
			name:        "multiple bare word matches",
			sql:         "SELECT w1, w2, x FROM t",
			bareTargets: []string{"W1", "W2"},
			want: []tm{
				{Name: "w1", Line: 1, Col: 8, EndCol: 10, Quoted: false},
				{Name: "w2", Line: 1, Col: 12, EndCol: 14, Quoted: false},
			},
		},

		// ── quoted identifier matches ────────────────────────────────────────
		{
			name:          "single quoted match",
			sql:           `SELECT "WRONG_COL" FROM t`,
			quotedTargets: []string{"WRONG_COL"},
			want:          []tm{{Name: "WRONG_COL", Line: 1, Col: 8, EndCol: 19, Quoted: true}},
		},
		{
			name:          "quoted match case-insensitive",
			sql:           `SELECT "wrong_col" FROM t`,
			quotedTargets: []string{"WRONG_COL"},
			want:          []tm{{Name: "wrong_col", Line: 1, Col: 8, EndCol: 19, Quoted: true}},
		},
		{
			name:          "quoted not in targets → not matched",
			sql:           `SELECT "other_col" FROM t`,
			quotedTargets: []string{"WRONG_COL"},
			want:          nil,
		},

		// ── tokens inside skipped regions are not reported ──────────────────
		{
			name:        "token inside line comment skipped",
			sql:         "SELECT x -- bad_col\nFROM t",
			bareTargets: []string{"BAD_COL"},
			want:        nil,
		},
		{
			name:        "token inside block comment skipped",
			sql:         "SELECT x /* bad_col */ FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        nil,
		},
		{
			name:        "token inside single-quoted string skipped",
			sql:         "SELECT 'bad_col' FROM t",
			bareTargets: []string{"BAD_COL"},
			want:        nil,
		},
		{
			name:          "quoted ident inside single-quoted string not collected",
			sql:           `SELECT '"WRONG_COL"' FROM t`,
			quotedTargets: []string{"WRONG_COL"},
			want:          nil,
		},

		// ── both bare and quoted in same call ────────────────────────────────
		{
			name:          "bare and quoted targets together",
			sql:           `SELECT w1, "WRONG2", x FROM t`,
			bareTargets:   []string{"W1"},
			quotedTargets: []string{"WRONG2"},
			want: []tm{
				{Name: "w1",     Line: 1, Col: 8,  EndCol: 10, Quoted: false},
				{Name: "WRONG2", Line: 1, Col: 12, EndCol: 20, Quoted: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindTokenPositions(tt.sql, tt.bareTargets, tt.quotedTargets)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindTokenPositions(%q, %v, %v)\n  got  %+v\n  want %+v",
					tt.sql, tt.bareTargets, tt.quotedTargets, got, tt.want)
			}
		})
	}
}

func TestGetStatementRanges(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []StatementRange
	}{
		{
			name: "Empty string",
			sql:  "",
			want: nil,
		},
		{
			name: "Only whitespace",
			sql:  "  \n  \t  ",
			want: nil,
		},
		{
			name: "Single statement no semicolon",
			sql:  "SELECT 1",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 8},
			},
		},
		{
			name: "Single statement with semicolon",
			sql:  "SELECT 1;",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
			},
		},
		{
			name: "Two statements separated by semicolon",
			sql:  "SELECT 1;\nSELECT 2",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
				{StartLine: 2, EndLine: 2, StartOffset: 10, EndOffset: 18},
			},
		},
		{
			name: "Leading whitespace skipped in StartOffset",
			sql:  "  SELECT 1",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 2, EndOffset: 10},
			},
		},
		{
			name: "Dollar-quoted block treated as single statement",
			sql:  "$$\nBEGIN\n  LET x := 1;\nEND;\n$$;",
			want: []StatementRange{
				{StartLine: 1, EndLine: 5, StartOffset: 0, EndOffset: 31},
			},
		},
		{
			name: "Line comment before statement does not affect StartLine",
			sql:  "-- comment\nSELECT 1",
			want: []StatementRange{
				{StartLine: 2, EndLine: 2, StartOffset: 11, EndOffset: 19},
			},
		},
		{
			name: "Semicolons inside line comments ignored",
			sql:  "SELECT 1 -- this; is a comment\nFROM t",
			want: []StatementRange{
				{StartLine: 1, EndLine: 2, StartOffset: 0, EndOffset: 37},
			},
		},
		{
			name: "Semicolons inside block comments ignored",
			sql:  "SELECT /* ; */ 1",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 16},
			},
		},
		{
			name: "Semicolons inside single-quoted strings ignored",
			sql:  "SELECT 'a;b'",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 12},
			},
		},
		{
			name: "Three statements",
			sql:  "SELECT 1;\nSELECT 2;\nSELECT 3",
			want: []StatementRange{
				{StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 9},
				{StartLine: 2, EndLine: 2, StartOffset: 10, EndOffset: 19},
				{StartLine: 3, EndLine: 3, StartOffset: 20, EndOffset: 28},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStatementRanges(tt.sql)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStatementRanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSyntax(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []DiagMarker
	}{
		{
			name: "Valid simple select",
			sql:  "SELECT * FROM table;",
			want: nil,
		},
		{
			name: "Unbalanced parentheses",
			sql:  "SELECT (col1 FROM table;",
			want: []DiagMarker{
				{StartLineNumber: 1, StartColumn: 8, EndLineNumber: 1, EndColumn: 9, Message: "Unclosed '('", Severity: 8},
			},
		},
		{
			name: "Unexpected token",
			sql:  "INVALID STATEMENT;",
			want: []DiagMarker{
				{StartLineNumber: 1, StartColumn: 1, EndLineNumber: 1, EndColumn: 8, Message: "Unexpected token 'INVALID'", Severity: 8},
			},
		},
		{
			name: "Snowflake scripting valid",
			sql:  "$$\nBEGIN\n  LET x := 1;\nEND;\n$$",
			want: nil,
		},
		{
			name: "Snowflake scripting invalid assignment",
			sql:  "$$\nBEGIN\n  LET x = 1;\nEND;\n$$",
			want: []DiagMarker{
				{StartLineNumber: 3, StartColumn: 9, EndLineNumber: 3, EndColumn: 10, Message: "Expected ':=' for assignment", Severity: 8},
			},
		},
		{
			name: "Complex scripting with nested blocks and loops",
			sql: `$$
DECLARE
  x INT;
  y INT;
  c1 CURSOR FOR SELECT * FROM t1;
BEGIN
  x := 0;
  FOR r IN c1 DO
    IF (r.id > 10) THEN
      y := r.id * 2;
      CASE
        WHEN y < 100 THEN
          INSERT INTO t2 VALUES (:y);
        ELSE
          RETURN 'Too big';
      END CASE;
    END IF;
  END FOR;
  RETURN 'Done';
END;
$$`,
			want: nil,
		},
		{
			name: "Nested dollar quotes",
			sql: `EXECUTE IMMEDIATE $$
BEGIN
  EXECUTE IMMEDIATE $inner$
    BEGIN
      LET x := 1;
    END;
  $inner$;
END;
$$;`,
			want: nil,
		},
		{
			name: "LET with type annotation and missing expression",
			sql: `$$
BEGIN
  LET temp_calc FLOAT := ;
  LET typed_varchar VARCHAR(100) := ;
  LET no_type := ;
END;
$$`,
			want: []DiagMarker{
				{StartLineNumber: 3, StartColumn: 23, EndLineNumber: 3, EndColumn: 25, Message: "Missing expression after assignment", Severity: 8},
				{StartLineNumber: 4, StartColumn: 34, EndLineNumber: 4, EndColumn: 36, Message: "Missing expression after assignment", Severity: 8},
				{StartLineNumber: 5, StartColumn: 15, EndLineNumber: 5, EndColumn: 17, Message: "Missing expression after assignment", Severity: 8},
			},
		},
		{
			name: "LET with type annotation valid",
			sql: `$$
BEGIN
  LET x FLOAT := 1.5;
  LET s VARCHAR(100) := 'hello';
END;
$$`,
			want: nil,
		},
		{
			name: "Undeclared variable in RETURN and FOR",
			sql: `$$
BEGIN
  RETURN missing_var;
  FOR r IN missing_cursor DO
    NULL;
  END FOR;
END;
$$`,
			want: []DiagMarker{
				{StartLineNumber: 3, StartColumn: 10, EndLineNumber: 3, EndColumn: 21, Message: "Variable 'missing_var' is not declared", Severity: 8},
				{StartLineNumber: 4, StartColumn: 12, EndLineNumber: 4, EndColumn: 26, Message: "Variable 'missing_cursor' is not declared", Severity: 8},
			},
		},
	{
		name: "Named dollar tag with block comment and escaped quote",
		sql: `EXECUTE IMMEDIATE $body$
DECLARE
    base_val FLOAT DEFAULT 10.0;
    str_val  VARCHAR;
BEGIN
    str_val := /* inline comment */ 'It''s a valid string';
    RETURN str_val;
END;
$body$;`,
		want: nil,
	},
	{
		// Tokenizer limitation: comment before keyword masks missing expression
		name: "Named dollar tag comment-masked missing expression",
		sql: `EXECUTE IMMEDIATE $body$
DECLARE
    base_val FLOAT DEFAULT 10.0;
BEGIN
    base_val :=

    -- comment
    IF (base_val > 5.0) THEN
        RETURN base_val;
    END IF;
END;
$body$;`,
		want: nil,
	},
	{
		name: "FOR loop with declared cursor",
		sql: `EXECUTE IMMEDIATE $$
DECLARE
    my_cursor CURSOR FOR SELECT id FROM t;
    total INTEGER DEFAULT 0;
BEGIN
    FOR rec IN my_cursor DO
        total := total + 1;
    END FOR;
    RETURN total;
END;
$$;`,
		want: nil,
	},
	{
		name: "FOR loop with undeclared cursor",
		sql: `EXECUTE IMMEDIATE $$
DECLARE
    total INTEGER DEFAULT 0;
BEGIN
    FOR rec IN ghost_cursor DO
        total := total + 1;
    END FOR;
    RETURN total;
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 5, StartColumn: 16, EndLineNumber: 5, EndColumn: 28, Message: "Variable 'ghost_cursor' is not declared", Severity: 8},
		},
	},
	{
		name: "Unmatched bracket with unclosed bracket and paren",
		sql: `EXECUTE IMMEDIATE $$
DECLARE
    json_data VARIANT;
BEGIN
    LET my_array ARRAY := [1, 2, (3 + 4];
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 5, StartColumn: 40, EndLineNumber: 5, EndColumn: 41, Message: "Unmatched ']'", Severity: 8},
			{StartLineNumber: 5, StartColumn: 27, EndLineNumber: 5, EndColumn: 28, Message: "Unclosed '['", Severity: 8},
			{StartLineNumber: 5, StartColumn: 34, EndLineNumber: 5, EndColumn: 35, Message: "Unclosed '('", Severity: 8},
		},
	},
	{
		name: "Unclosed string literal",
		sql: `EXECUTE IMMEDIATE $$
BEGIN
    LET bad_string VARCHAR := 'This string has no end;
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 3, StartColumn: 31, EndLineNumber: 3, EndColumn: 32, Message: "Unclosed string literal", Severity: 8},
		},
	},
	{
		name: "Unclosed block comment",
		sql: `EXECUTE IMMEDIATE $$
BEGIN
    LET x INTEGER := 1;
    /* This block comment never closes
    RETURN x;
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 4, StartColumn: 5, EndLineNumber: 4, EndColumn: 7, Message: "Unclosed block comment", Severity: 8},
		},
	},
	{
		name: "LET with subquery in parens",
		sql: `EXECUTE IMMEDIATE $$
BEGIN
    LET user_count INTEGER := (
        SELECT COUNT(*)
        FROM users
        WHERE status = 'ACTIVE'
    );
    RETURN user_count;
END;
$$;`,
		want: nil,
	},
	{
		name: "Bare equals assignment error",
		sql: `EXECUTE IMMEDIATE $$
BEGIN
    LET n INTEGER := 0;
    n = n + 1;
    RETURN n;
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 4, StartColumn: 7, EndLineNumber: 4, EndColumn: 8, Message: "Expected ':=' for assignment", Severity: 8},
		},
	},
	{
		name: "Template injection brace at statement start",
		sql: `EXECUTE IMMEDIATE $$
BEGIN
    {TEMPLATE_INJECTION_ERROR}
    RETURN 1;
END;
$$;`,
		want: []DiagMarker{
			{StartLineNumber: 3, StartColumn: 5, EndLineNumber: 3, EndColumn: 6, Message: "Unexpected token '{'", Severity: 8},
		},
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateSyntax(tt.sql); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseJoinTables(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []JoinTableRef
	}{
		{
			name: "Simple FROM",
			sql:  "SELECT * FROM table1",
			want: []JoinTableRef{
				{Name: "TABLE1", Alias: "TABLE1"},
			},
		},
		{
			name: "Two-part name with alias",
			sql:  "SELECT * FROM schema1.table1 AS t1",
			want: []JoinTableRef{
				{Schema: "SCHEMA1", Name: "TABLE1", Alias: "t1"},
			},
		},
		{
			name: "Three-part name",
			sql:  "SELECT * FROM db1.schema1.table1 JOIN table2 t2",
			want: []JoinTableRef{
				{DB: "DB1", Schema: "SCHEMA1", Name: "TABLE1", Alias: "TABLE1"},
				{Name: "TABLE2", Alias: "t2"},
			},
		},
		{
			name: "Multi-JOIN mix",
			sql: `SELECT *
                  FROM db1.s1.t1
                  JOIN s1.t2 AS alias2 ON t1.id = alias2.t1_id
                  LEFT JOIN t3 alias3 ON alias2.id = alias3.t2_id
                  FULL OUTER JOIN db2.s2.t4 ON t4.id = alias3.t4_id`,
			want: []JoinTableRef{
				{DB: "DB1", Schema: "S1", Name: "T1", Alias: "T1"},
				{Schema: "S1", Name: "T2", Alias: "alias2"},
				{Name: "T3", Alias: "alias3"},
				{DB: "DB2", Schema: "S2", Name: "T4", Alias: "T4"},
			},
		},
		{
			name: "Subquery and CTE",
			sql: `WITH cte AS (SELECT * FROM t1)
                  SELECT * FROM cte c1
                  JOIN (SELECT * FROM t2) sub ON c1.id = sub.id`,
			want: []JoinTableRef{
				{Name: "T1", Alias: "T1"},
				{Name: "CTE", Alias: "c1"},
				{Name: "T2", Alias: "T2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseJoinTables(tt.sql); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseJoinTables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSemantics(t *testing.T) {
	resolvedRefs := []ResolvedRef{
		{Alias: "T1", DB: "DB1", Schema: "S1", Name: "TABLE1"},
	}
	colEntries := []ColEntry{
		{
			DB: "DB1", Schema: "S1", Name: "TABLE1",
			Cols: []ColInfo{
				{Name: "ID", DataType: "NUMBER"},
				{Name: "NAME", DataType: "VARCHAR"},
			},
		},
	}

	tests := []struct {
		name string
		sql  string
		want []DiagMarker
	}{
		{
			name: "Valid reference",
			sql:  "SELECT T1.ID FROM TABLE1 T1",
			want: nil,
		},
		{
			name: "Invalid column",
			sql:  "SELECT T1.MISSING FROM TABLE1 T1",
			want: []DiagMarker{
				{StartLineNumber: 1, StartColumn: 11, EndLineNumber: 1, EndColumn: 18, Message: "Column 'MISSING' does not exist in TABLE1", Severity: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateSemantics(tt.sql, resolvedRefs, colEntries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateSemantics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeJoinOnConditions(t *testing.T) {
	req := JoinOnSuggestionsReq{
		ResolvedRefs: []ResolvedRef{
			{Alias: "A", DB: "DB", Schema: "S", Name: "TABLE_A"},
			{Alias: "B", DB: "DB", Schema: "S", Name: "TABLE_B"},
		},
		Prefix: "ON ",
		ColEntries: []ColEntry{
			{DB: "DB", Schema: "S", Name: "TABLE_A", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}, {Name: "A_NAME", DataType: "VARCHAR"}}},
			{DB: "DB", Schema: "S", Name: "TABLE_B", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}, {Name: "TABLE_A_ID", DataType: "NUMBER"}}},
		},
	}

	t.Run("PK Heuristic Tier", func(t *testing.T) {
		got := ComputeJoinOnConditions(req)
		want := []JoinCondition{
			{Condition: "ON B.TABLE_A_ID = A.ID", Detail: "PK HEURISTIC", SortText: "0cON B.TABLE_A_ID = A.ID"},
			{Condition: "ON A.ID = B.ID", Detail: "SAME-NAME COLUMN", SortText: "1ON A.ID = B.ID"},
			{Condition: "USING (ID)", Detail: "USING", SortText: "1.5USING (ID)"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ComputeJoinOnConditions() Tier 1b = %v, want %v", got, want)
		}
	})

	t.Run("FK Tier", func(t *testing.T) {
		reqWithFK := req
		reqWithFK.FKEntries = []TableFKEntry{
			{
				DB: "DB", Schema: "S", Name: "TABLE_B",
				FKs: []FKEntry{
					{PKDatabase: "DB", PKSchema: "S", PKTable: "TABLE_A", PKColumn: "ID", FKColumn: "TABLE_A_ID", ConstraintName: "FK_B_A", KeySequence: 1},
				},
			},
		}
		got := ComputeJoinOnConditions(reqWithFK)
		want := []JoinCondition{
			{Condition: "ON B.TABLE_A_ID = A.ID", Detail: "FK RELATION", SortText: "0aON B.TABLE_A_ID = A.ID"},
			{Condition: "ON A.ID = B.ID", Detail: "SAME-NAME COLUMN", SortText: "1ON A.ID = B.ID"},
			{Condition: "USING (ID)", Detail: "USING", SortText: "1.5USING (ID)"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ComputeJoinOnConditions() Tier 1a = %v, want %v", got, want)
		}
	})

	t.Run("Composite FK", func(t *testing.T) {
		reqComp := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "P", DB: "DB", Schema: "S", Name: "PARENT"},
				{Alias: "C", DB: "DB", Schema: "S", Name: "CHILD"},
			},
			Prefix: "ON ",
			FKEntries: []TableFKEntry{
				{
					DB: "DB", Schema: "S", Name: "CHILD",
					FKs: []FKEntry{
						{PKDatabase: "DB", PKSchema: "S", PKTable: "PARENT", PKColumn: "K1", FKColumn: "FK1", ConstraintName: "FK_COMP", KeySequence: 1},
						{PKDatabase: "DB", PKSchema: "S", PKTable: "PARENT", PKColumn: "K2", FKColumn: "FK2", ConstraintName: "FK_COMP", KeySequence: 2},
					},
				},
			},
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "PARENT", Cols: []ColInfo{{Name: "K1", DataType: "NUMBER"}, {Name: "K2", DataType: "NUMBER"}}},
				{DB: "DB", Schema: "S", Name: "CHILD", Cols: []ColInfo{{Name: "FK1", DataType: "NUMBER"}, {Name: "FK2", DataType: "NUMBER"}}},
			},
		}
		got := ComputeJoinOnConditions(reqComp)
		want := []JoinCondition{
			{Condition: "ON C.FK1 = P.K1 AND C.FK2 = P.K2", Detail: "FK RELATION", SortText: "0aON C.FK1 = P.K1 AND C.FK2 = P.K2"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ComputeJoinOnConditions() Composite = %v, want %v", got, want)
		}
	})
}

func TestApplyCasing(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		keywordCase   string
		identifierCase string
		functionCase  string
		want          string
	}{
		// ── keywordCase ───────────────────────────────────────────────────────
		{name: "UPPER keyword", sql: "select id from t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT id FROM t"},
		{name: "lower keyword", sql: "SELECT id FROM t", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "select id from t"},
		{name: "Title keyword", sql: "SELECT id FROM t", keywordCase: "Title", identifierCase: "Preserve", functionCase: "UPPER", want: "Select id From t"},
		{name: "Preserve keyword", sql: "SeLeCt id FrOm t", keywordCase: "Preserve", identifierCase: "Preserve", functionCase: "UPPER", want: "SeLeCt id FrOm t"},

		// ── identifierCase ────────────────────────────────────────────────────
		{name: "identifier UPPER", sql: "SELECT MyCol FROM MyTable", keywordCase: "UPPER", identifierCase: "UPPER", functionCase: "UPPER", want: "SELECT MYCOL FROM MYTABLE"},
		{name: "identifier lower", sql: "SELECT MyCol FROM MyTable", keywordCase: "UPPER", identifierCase: "lower", functionCase: "UPPER", want: "SELECT mycol FROM mytable"},
		{name: "identifier Preserve", sql: "SELECT MyCol FROM MyTable", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT MyCol FROM MyTable"},

		// ── functionCase ──────────────────────────────────────────────────────
		{name: "function UPPER", sql: "select count(id) from t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT COUNT(id) FROM t"},
		{name: "function lower", sql: "select COUNT(id) from t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "lower", want: "SELECT count(id) FROM t"},
		{name: "UDF gets functionCase", sql: "select my_udf(x) from t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT MY_UDF(x) FROM t"},

		// ── pass-through sections ─────────────────────────────────────────────
		{name: "single-quoted string unchanged", sql: "select 'SELECT from' from t", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "select 'SELECT from' from t"},
		{name: "double-quoted ident unchanged", sql: `select "MyCol" from t`, keywordCase: "UPPER", identifierCase: "lower", functionCase: "UPPER", want: `SELECT "MyCol" FROM t`},
		{name: "line comment unchanged", sql: "-- SELECT\nselect 1", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "-- SELECT\nselect 1"},
		{name: "block comment unchanged", sql: "/* SELECT */ select 1", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "/* SELECT */ select 1"},
		{name: "dollar-quoted block unchanged", sql: "CREATE FUNCTION f() AS $$SELECT 1$$", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "create function f() as $$SELECT 1$$"},

		// ── edge cases ────────────────────────────────────────────────────────
		{name: "empty string", sql: "", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: ""},
		{name: "function space before paren stripped", sql: "SELECT COUNT (id) FROM t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT COUNT(id) FROM t"},
		{name: "keyword OVER keeps space before paren", sql: "SELECT id, ROW_NUMBER () OVER (ORDER BY id) FROM t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT id, ROW_NUMBER() OVER (ORDER BY id) FROM t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyCasing(tt.sql, tt.keywordCase, tt.identifierCase, tt.functionCase)
			if got != tt.want {
				t.Errorf("ApplyCasing(%q, %q, %q, %q)\n  got  %q\n  want %q",
					tt.sql, tt.keywordCase, tt.identifierCase, tt.functionCase, got, tt.want)
			}
		})
	}
}
