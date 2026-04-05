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
