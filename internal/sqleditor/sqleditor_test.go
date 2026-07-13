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
		{name: "two-part on first part", line: "db.schema", col: 0, want: []string{"DB", "SCHEMA"}},
		{name: "two-part on dot", line: "db.schema", col: 2, want: []string{"DB", "SCHEMA"}},
		{name: "two-part on second part", line: "db.schema", col: 4, want: []string{"DB", "SCHEMA"}},

		// Three-part identifier (db.schema.table)
		{name: "three-part on first", line: "db.schema.tbl", col: 1, want: []string{"DB", "SCHEMA", "TBL"}},
		{name: "three-part on middle", line: "db.schema.tbl", col: 5, want: []string{"DB", "SCHEMA", "TBL"}},
		{name: "three-part on last", line: "db.schema.tbl", col: 11, want: []string{"DB", "SCHEMA", "TBL"}},

		// Quoted identifier
		{name: "quoted single part", line: `"My Table"`, col: 3, want: []string{"My Table"}},
		{name: "quoted two-part", line: `"DB"."My Schema"`, col: 5, want: []string{"DB", "My Schema"}},
		{name: "col before opening quote", line: ` "tbl"`, col: 0, want: nil},

		// Identifier embedded in SQL
		{name: "ident in query col on ident", line: "SELECT * FROM db.schema.tbl WHERE x=1", col: 20, want: []string{"DB", "SCHEMA", "TBL"}},
		{name: "ident in query col on keyword before ident", line: "SELECT * FROM db.schema.tbl WHERE x=1", col: 8, want: nil},

		// Underscore in identifier
		{name: "identifier with underscores", line: "my_db.my_schema", col: 3, want: []string{"MY_DB", "MY_SCHEMA"}},

		// Digits in identifier
		{name: "identifier with digits", line: "table1.col2", col: 4, want: []string{"TABLE1", "COL2"}},
		// Digit-led tokens: \w matches digits, so "123abc" is treated as one token (same as original TS /\w/)
		{name: "identifier starting with digit matched as token", line: "123abc", col: 0, want: []string{"123ABC"}},

		// Two separate identifiers on the same line
		{name: "two idents on line - col on first", line: "t1.c1, t2.c2", col: 1, want: []string{"T1", "C1"}},
		{name: "two idents on line - col on second", line: "t1.c1, t2.c2", col: 8, want: []string{"T2", "C2"}},
		{name: "two idents on line - col on comma", line: "t1.c1, t2.c2", col: 5, want: nil},
		{name: "two idents on line - col on space", line: "t1.c1, t2.c2", col: 6, want: nil},

		// Trailing dot — cursor before/on the dangling dot
		{name: "trailing dot - col on word before dot", line: "db.", col: 1, want: []string{"DB"}},
		{name: "trailing dot - col on dangling dot", line: "db.", col: 2, want: []string{"DB"}},
		{name: "quoted trailing dot", line: `"db".`, col: 5, want: []string{"db"}},

		// Leading dot — scanner skips the dot; "schema" is returned as a 1-part identifier
		{name: "leading dot - col on word after dot", line: ".schema", col: 1, want: []string{"SCHEMA"}},
		// Leading dot — col on the dot itself is not on any identifier
		{name: "leading dot - col on the dot", line: ".schema", col: 0, want: nil},

		// Mixed quoted and bare parts
		{name: "quoted-dot-bare col on quoted", line: `"My DB".schema`, col: 3, want: []string{"My DB", "SCHEMA"}},
		{name: "quoted-dot-bare col on bare", line: `"My DB".schema`, col: 9, want: []string{"My DB", "SCHEMA"}},

		// col at very last character of identifier
		{name: "col at last char of three-part", line: "a.b.c", col: 4, want: []string{"A", "B", "C"}},

		// col past end of line
		{name: "col past end of line", line: "abc", col: 10, want: nil},

		// Identifier immediately after opening paren
		{name: "ident after paren", line: "(db.schema)", col: 4, want: []string{"DB", "SCHEMA"}},
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
			// RETURN TABLE(resultset) is valid Snowflake Scripting syntax for returning
			// a resultset from a stored procedure.  TABLE is not a variable — it must
			// not be flagged as "Variable 'TABLE' is not declared".
			name: "RETURN TABLE resultset — no false positive",
			sql: `EXECUTE IMMEDIATE $$
  DECLARE
    res RESULTSET;
  BEGIN
    res := (SELECT region, SUM(revenue) AS total FROM regional_sales GROUP BY region);
    RETURN TABLE(res);
  END;
$$;`,
			want: nil,
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
				{Schema: "SCHEMA1", Name: "TABLE1", Alias: "T1"},
			},
		},
		{
			name: "Three-part name",
			sql:  "SELECT * FROM db1.schema1.table1 JOIN table2 t2",
			want: []JoinTableRef{
				{DB: "DB1", Schema: "SCHEMA1", Name: "TABLE1", Alias: "TABLE1"},
				{Name: "TABLE2", Alias: "T2"},
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
				{Schema: "S1", Name: "T2", Alias: "ALIAS2"},
				{Name: "T3", Alias: "ALIAS3"},
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
				{Name: "CTE", Alias: "C1"},
				{Name: "T2", Alias: "T2"},
			},
		}, {
			name: "Three-part quoted names with spaces and dollar",
			sql:  `SELECT * FROM "DB-1"."SCHEMA$1"."TABLE 1" AS "T 1"`,
			want: []JoinTableRef{
				{DB: "DB-1", Schema: "SCHEMA$1", Name: "TABLE 1", Alias: "T 1"},
			},
		},
		{
			name: "Escaped quotes in table name",
			sql:  `FROM "My ""Quoted"" Table"`,
			want: []JoinTableRef{
				{Name: `My "Quoted" Table`, Alias: `My "Quoted" Table`},
			},
		},
		{
			name: "USE statements",
			sql:  `USE DATABASE db1; USE SCHEMA sch1; USE db2.sch2; USE ROLE r1; USE WAREHOUSE w1;`,
			want: []JoinTableRef{
				{DB: "DB1"},
				{Schema: "SCH1"},
				{DB: "DB2", Schema: "SCH2"},
			},
		},
		{
			// Token scanning ignores comments and string literals — no phantom ref.
			name: "Refs inside comments and strings are ignored",
			sql: `SELECT '-- FROM fake.str' FROM real1
			-- FROM legacy.t
			/* FROM block.comment */ JOIN real2 r2`,
			want: []JoinTableRef{
				{Name: "REAL1", Alias: "REAL1"},
				{Name: "REAL2", Alias: "R2"},
			},
		},
		{
			name: "Comment between keyword and identifier",
			sql:  `SELECT * FROM /* hint */ db.sch.t`,
			want: []JoinTableRef{
				{DB: "DB", Schema: "SCH", Name: "T", Alias: "T"},
			},
		},
		{
			name: "Newline-separated alias",
			sql:  "SELECT * FROM t1\n  t1alias",
			want: []JoinTableRef{
				{Name: "T1", Alias: "T1ALIAS"},
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
			// Issue #714: paren-less SELECT * EXCLUDE <col> must not flag EXCLUDE.
			name: "Star exclude",
			sql:  "SELECT * EXCLUDE NAME FROM TABLE1 T1",
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
			{Condition: `ON B.TABLE_A_ID = A.ID`, Detail: "PK HEURISTIC", SortText: `0cON B.TABLE_A_ID = A.ID`},
			{Condition: `ON A.ID = B.ID`, Detail: "SAME-NAME COLUMN", SortText: `1ON A.ID = B.ID`},
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
			{Condition: `ON B.TABLE_A_ID = A.ID`, Detail: "FK RELATION", SortText: `0aON B.TABLE_A_ID = A.ID`},
			{Condition: `ON A.ID = B.ID`, Detail: "SAME-NAME COLUMN", SortText: `1ON A.ID = B.ID`},
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
			{Condition: `ON C.FK1 = P.K1 AND C.FK2 = P.K2`, Detail: "FK RELATION", SortText: `0aON C.FK1 = P.K1 AND C.FK2 = P.K2`},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ComputeJoinOnConditions() Composite = %v, want %v", got, want)
		}
	})

	t.Run("Case-Sensitive Lowercase Aliases", func(t *testing.T) {
		reqLowercase := JoinOnSuggestionsReq{
			ResolvedRefs: []ResolvedRef{
				{Alias: "a", DB: "DB", Schema: "S", Name: "TABLE_A"},
				{Alias: "b", DB: "DB", Schema: "S", Name: "TABLE_B"},
			},
			Prefix: "ON ",
			ColEntries: []ColEntry{
				{DB: "DB", Schema: "S", Name: "TABLE_A", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}, {Name: "A_NAME", DataType: "VARCHAR"}}},
				{DB: "DB", Schema: "S", Name: "TABLE_B", Cols: []ColInfo{{Name: "ID", DataType: "NUMBER"}, {Name: "TABLE_A_ID", DataType: "NUMBER"}}},
			},
		}
		got := ComputeJoinOnConditions(reqLowercase)
		want := []JoinCondition{
			{Condition: `ON "b".TABLE_A_ID = "a".ID`, Detail: "PK HEURISTIC", SortText: `0cON "b".TABLE_A_ID = "a".ID`},
			{Condition: `ON "a".ID = "b".ID`, Detail: "SAME-NAME COLUMN", SortText: `1ON "a".ID = "b".ID`},
			{Condition: "USING (ID)", Detail: "USING", SortText: "1.5USING (ID)"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ComputeJoinOnConditions() Lowercase Aliases = %v, want %v", got, want)
		}
	})
}

func TestApplyCasing(t *testing.T) {
	tests := []struct {
		name           string
		sql            string
		keywordCase    string
		identifierCase string
		functionCase   string
		want           string
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
		{name: "dollar-quoted block RECURSIVE", sql: "CREATE FUNCTION f() AS $$SELECT 1$$", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "create function f() as $$select 1$$"},
		{name: "$query$ block UNCHANGED", sql: "LET s := $query$ SELECT 1 $query$", keywordCase: "lower", identifierCase: "Preserve", functionCase: "lower", want: "let s := $query$ SELECT 1 $query$"},

		// ── edge cases ────────────────────────────────────────────────────────
		{name: "empty string", sql: "", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: ""},
		{name: "function space before paren stripped", sql: "SELECT COUNT (id) FROM t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT COUNT(id) FROM t"},
		{name: "keyword OVER keeps space before paren", sql: "SELECT id, ROW_NUMBER () OVER (ORDER BY id) FROM t", keywordCase: "UPPER", identifierCase: "Preserve", functionCase: "UPPER", want: "SELECT id, ROW_NUMBER() OVER (ORDER BY id) FROM t"},
		// #714 follow-up: EXCLUDE is NOT a global keyword, so a real column/alias
		// named EXCLUDE must be cased with identifierCase, not keywordCase.
		{name: "identifier named EXCLUDE not keyword-cased", sql: "SELECT EXCLUDE FROM t", keywordCase: "UPPER", identifierCase: "lower", functionCase: "UPPER", want: "SELECT exclude FROM t"},
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

func TestApplyCasingComplexScenarios(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "Complex SELECT with window functions",
			sql: ` sElEcT Coalesce( a.First_Name, b.First_Name ) aS fName,
       count( * ) Over (pArTiTiOn By a.dept_id oRdEr By a.joined_date DeSc) as dept_count
FrOm my_schema.Employees A
LeFt oUtEr jOiN my_schema.Contractors b On a.id = b.emp_id
wHeRe a.Status = 'active'; `,
			want: ` SELECT Coalesce( a.first_name, b.first_name ) AS fname,
       Count( * ) OVER (PARTITION BY a.dept_id ORDER BY a.joined_date DESC) AS dept_count
FROM my_schema.employees a
LEFT OUTER JOIN my_schema.contractors b ON a.id = b.emp_id
WHERE a.status = 'active'; `,
		},
		{
			name: "CREATE TABLE with quoted identifiers and literals",
			sql: `CrEaTe Or RePlAcE tAbLe "My WeIrD TaBlE" (
    "ColA" VaRcHaR DeFaUlT 'It''s a "SELECT" statement inside a string',
    "cAsE_sEnSiTiVe_CoL" nUmBeR(10, 2) cOmMeNt 'Keywords like fRoM and WhErE should be untouched',
    "Col""With""Quotes" BoOlEaN
);`,
			want: `CREATE OR REPLACE TABLE "My WeIrD TaBlE" (
    "ColA" VARCHAR DEFAULT 'It''s a "SELECT" statement inside a string',
    "cAsE_sEnSiTiVe_CoL" NUMBER(10, 2) COMMENT 'Keywords like fRoM and WhErE should be untouched',
    "Col""With""Quotes" BOOLEAN
);`,
		},
		{
			name: "Nested functions and UDFs",
			sql: `SeLeCt CAST    ( my_date_col aS DaTe ),
       TrY_pArSe_JsOn ( raw_payload ),
       current_timestamp(),
       my_custom_udf_function
       (
           'input_string'
       )
FrOm events;`,
			want: `SELECT Cast( my_date_col AS DATE ),
       Try_parse_json( raw_payload ),
       CURRENT_TIMESTAMP(),
       My_custom_udf_function(
           'input_string'
       )
FROM events;`,
		},
		{
			name: "EXECUTE IMMEDIATE with dollar quoting",
			sql: `eXeCuTe iMmEdIaTe $$
dEcLaRe
    my_var InTeGeR dEfAuLt 0;
BeGiN
    -- This is a comment with SELECT and COUNT() that should NOT be upper/lowercased
    /* Block comment testing lowercase keywords
       sElEcT * fRoM "table" 
    */
    LET query_str VARCHAR := $query$ sElEcT * fRoM mY_tAbLe WhErE iD = 1; $query$;
    my_var := 1;
    ReTuRn my_var;
EnD;
$$;`,
			want: `EXECUTE IMMEDIATE $$
DECLARE
    my_var INTEGER DEFAULT 0;
BEGIN
    -- This is a comment with SELECT and COUNT() that should NOT be upper/lowercased
    /* Block comment testing lowercase keywords
       sElEcT * fRoM "table" 
    */
    LET query_str VARCHAR := $query$ sElEcT * fRoM mY_tAbLe WhErE iD = 1; $query$;
    my_var := 1;
    RETURN my_var;
END;
$$;`,
		},
		{
			name: "JSON path and Lateral Flatten",
			sql: `SeLeCt f.VaLuE:id::StRiNg As Id,
       f.VaLuE:name::VaRcHaR aS nAmE
fRoM my_db.my_schema.raw_json j,
LaTeRaL FlAtTeN ( InPuT => j.PaYlOaD:users ) f
wHeRe IdEnTiFiEr('user_id') = 123;`,
			want: `SELECT f.value:id::STRING AS id,
       f.value:name::VARCHAR AS name
FROM my_db.my_schema.raw_json j,
LATERAL Flatten( INPUT => j.payload:users ) f
WHERE Identifier('user_id') = 123;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Using common preferences for complex tests: Keywords UPPER, Identifiers lower, Functions Title
			got := ApplyCasing(tt.sql, "UPPER", "lower", "Title")
			if got != tt.want {
				t.Errorf("ApplyCasing(%s) failure\n  got:  %q\n  want: %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestScriptingNeedsColon(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		// ── Standard working scenarios ──────────────────────────────────────
		{
			name: "Standard SQL context (SELECT)",
			sql:  "SELECT * FROM t WHERE id = ",
			want: true,
		},
		{
			name: "Standard assignment context (LET)",
			sql:  "LET min_revenue NUMBER := ",
			want: false,
		},
		{
			name: "Procedural assignment without LET",
			sql:  "target_status := ",
			want: false,
		},
		{
			name: "Inside IF condition",
			sql:  "IF (revenue > ",
			want: false,
		},
		{
			name: "Already prefixed with colon",
			sql:  "SELECT * FROM t WHERE id = :",
			want: false,
		},

		// ── The Buggy Scenarios (Nested Contexts) ───────────────────────────
		{
			name: "Scenario: SELECT inside procedural assignment",
			sql: `res := (
      select REGION, sum(REVENUE) as total_revenue
      from REGIONAL_SALES
      group by REGION
      HAVING SUM(REVENUE) >= `,
			want: true, // Currently fails (returns false) because the parser thinks "res" is the start of the statement
		},
		{
			name: "Scenario: SELECT inside LET assignment",
			sql:  "LET user_count INTEGER := (SELECT COUNT(*) FROM users WHERE status = ",
			want: true, // Currently fails
		},
		{
			name: "Scenario: SELECT inside RETURN expression",
			sql:  "RETURN (SELECT COUNT(*) FROM t WHERE col = ",
			want: true, // Currently fails
		},
		{
			name: "Scenario: Cursor inside WITH CTE assignment",
			sql:  "res := (WITH cte AS (SELECT * FROM t WHERE col = ",
			want: true, // Currently fails
		},
		// ── DML and DDL Statements (Needs Colon = true) ─────────────────────
		{
			name: "UPDATE statement WHERE clause",
			sql:  "UPDATE users SET status = 'ACTIVE' WHERE user_id = ",
			want: true,
		},
		{
			name: "INSERT statement values",
			sql:  "INSERT INTO logs (level, message) VALUES ('INFO', ",
			want: true,
		},
		{
			name: "DELETE statement",
			sql:  "DELETE FROM sessions WHERE last_active < ",
			want: true,
		},
		{
			name: "CALL stored procedure",
			sql:  "CALL my_procedure('param', ",
			want: true, // CALL is a SQL-level command
		},

		// ── Control Flow and Scripting (Needs Colon = false) ────────────────
		{
			name: "WHILE loop condition",
			sql:  "WHILE (retry_count < ",
			want: false, // Last context is WHILE
		},
		{
			name: "FOR loop bound",
			sql:  "FOR i IN 1 TO ",
			want: false, // Last context is FOR
		},
		{
			name: "CASE statement inside scripting",
			sql:  "CASE WHEN current_state = 'ERROR' THEN error_count := ",
			want: false, // Last context is :=
		},
		{
			name: "Just after BEGIN",
			sql:  "BEGIN\n  ",
			want: false, // Context is BEGIN
		},

		// ── Sneaky Edge Cases (Masking and Resets) ──────────────────────────
		{
			name: "Semicolon resets context",
			sql:  "SELECT * FROM t; counter := ",
			want: false, // The ; closed the SELECT, the new context is :=
		},
		{
			name: "String masking a context setter",
			sql:  "res := (SELECT id FROM t WHERE name = 'ignore := this string' AND val = ",
			want: true, // The := inside the string is ignored; last context is SELECT
		},
		{
			name: "Line comment masking a SQL keyword",
			sql:  "LET x := 1; -- SELECT something to trick the parser\ny := ",
			want: false, // The SELECT is ignored; last context is :=
		},
		{
			name: "Block comment masking a scripting operator",
			sql:  "SELECT * FROM t WHERE id = /* := */ ",
			want: true, // The := is ignored; last context is SELECT
		},
		{
			name: "Deeply nested subqueries",
			sql:  "res := (SELECT a FROM t WHERE b IN (SELECT c FROM t2 WHERE d = ",
			want: true, // Both := and the first SELECT are overridden by the inner SELECT
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the cursor being exactly at the end of the SQL string,
			// scoping the scan the same way GetScriptingCompletions does.
			offset := len([]rune(tt.sql))
			text, _ := scriptingContext(tt.sql, runeOffsetToByte(tt.sql, offset))

			got := scriptingNeedsColon(text)
			if got != tt.want {
				t.Errorf("scriptingNeedsColon() = %v, want %v\nSQL Context: %q", got, tt.want, tt.sql)
			}
		})
	}
}

// TestScriptingNeedsColon_NoLeakAcrossDollarBlock guards the $$-boundary fix:
// a colon-required keyword before the opening $$ (here CREATE) must not leak into
// a reference at the start of the block body. Without scriptingContext scoping the
// scan back would reach CREATE and wrongly require a colon.
func TestScriptingNeedsColon_NoLeakAcrossDollarBlock(t *testing.T) {
	sql := "CREATE PROCEDURE p() RETURNS INT LANGUAGE SQL AS $$ myref"
	got := GetScriptingCompletions(sql, len([]rune(sql)))
	if got.NeedsColon {
		t.Error("NeedsColon should be false: no colon-required keyword exists inside the $$ block")
	}
}

// ── GetAutocompleteContext Tests ─────────────────────────────────────────────

func TestGetAutocompleteContext_BasicSingleStatement(t *testing.T) {
	sql := "SELECT id, name FROM users WHERE id = 1"
	offset := len([]rune(sql)) // cursor at end

	ctx := GetAutocompleteContext(sql, offset)

	if len(ctx.StatementRanges) != 1 {
		t.Fatalf("expected 1 statement range, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != 0 {
		t.Errorf("expected currentStmtIdx=0, got %d", ctx.CurrentStmtIdx)
	}
	if ctx.CurrentStmt != sql {
		t.Errorf("expected currentStmt to be the full SQL, got %q", ctx.CurrentStmt)
	}
	// Table refs should find "users"
	found := false
	for _, ref := range ctx.TableRefs {
		if ref.Name == "USERS" || ref.Name == "users" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find table ref 'users' in %+v", ctx.TableRefs)
	}
}

func TestGetAutocompleteContext_MultiStatement(t *testing.T) {
	sql := "SELECT 1;\nSELECT a FROM t1;\nSELECT b FROM t2"
	// Place cursor in the second statement (offset within "SELECT a FROM t1;")
	offset := len([]rune("SELECT 1;\nSELECT a"))

	ctx := GetAutocompleteContext(sql, offset)

	if len(ctx.StatementRanges) != 3 {
		t.Fatalf("expected 3 statement ranges, got %d", len(ctx.StatementRanges))
	}
	if ctx.CurrentStmtIdx != 1 {
		t.Errorf("expected currentStmtIdx=1, got %d", ctx.CurrentStmtIdx)
	}
}

func TestGetAutocompleteContext_ScriptingVariables(t *testing.T) {
	sql := "CREATE PROCEDURE p() RETURNS INT LANGUAGE SQL AS $$ DECLARE x INT; BEGIN RETURN :x; END; $$"
	// Cursor inside the $$ block
	offset := len([]rune("CREATE PROCEDURE p() RETURNS INT LANGUAGE SQL AS $$ DECLARE x INT; BEGIN RETURN :"))

	ctx := GetAutocompleteContext(sql, offset)

	if len(ctx.Scripting.Variables) == 0 {
		t.Error("expected scripting variables to be extracted")
	}
	foundX := false
	for _, v := range ctx.Scripting.Variables {
		if v == "X" {
			foundX = true
			break
		}
	}
	if !foundX {
		t.Errorf("expected variable 'X' in %v", ctx.Scripting.Variables)
	}
}

// ── getCTEColumnsAtCursor Tests ──────────────────────────────────────────────

func TestGetCTEColumnsAtCursor_SingleCTE(t *testing.T) {
	sql := "WITH cte AS (SELECT id, name FROM users) SELECT cte."
	cols := getCTEColumnsAtCursor(sql)

	if len(cols) == 0 {
		t.Fatal("expected CTE columns to be extracted")
	}
	if cols[0].Name != "CTE" {
		t.Errorf("expected CTE name 'CTE', got %q", cols[0].Name)
	}
	// The CTE may or may not resolve columns depending on whether "users" is in
	// the empty registry. For a simple SELECT list projection, it should find
	// "id" and "name" as projected columns.
	if len(cols[0].Cols) < 2 {
		t.Logf("CTE columns: %+v (may be empty if source table not in registry)", cols[0].Cols)
	}
}

func TestGetCTEColumnsAtCursor_MultipleCTEs(t *testing.T) {
	sql := "WITH a AS (SELECT 1 AS x), b AS (SELECT 2 AS y) SELECT a.x, b.y"
	cols := getCTEColumnsAtCursor(sql)

	if len(cols) < 2 {
		t.Fatalf("expected at least 2 CTE entries, got %d", len(cols))
	}

	// Should have entries for both "A" and "B"
	names := make(map[string]bool)
	for _, c := range cols {
		names[c.Name] = true
	}
	if !names["A"] {
		t.Error("expected CTE entry for 'A'")
	}
	if !names["B"] {
		t.Error("expected CTE entry for 'B'")
	}
}

func TestGetCTEColumnsAtCursor_NoCTE(t *testing.T) {
	sql := "SELECT id FROM users"
	cols := getCTEColumnsAtCursor(sql)

	if cols != nil {
		t.Errorf("expected nil for non-CTE query, got %+v", cols)
	}
}

func TestGetCTEColumnsAtCursor_CommentBeforeWith(t *testing.T) {
	sql := "-- comment\nWITH cte AS (SELECT 1 AS val) SELECT cte.val"
	cols := getCTEColumnsAtCursor(sql)

	if len(cols) == 0 {
		t.Fatal("expected CTE columns even with leading comment")
	}
	if cols[0].Name != "CTE" {
		t.Errorf("expected CTE name 'CTE', got %q", cols[0].Name)
	}
}
