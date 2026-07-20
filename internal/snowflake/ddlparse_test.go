// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

// ─── parseProcedureDDL ───────────────────────────────────────────────────────

func TestParseProcedureDDL(t *testing.T) {
	tests := []struct {
		name string
		ddl  string
		want []ProcParam
	}{
		{
			name: "two named params",
			ddl:  `CREATE PROCEDURE "DB"."SCH"."P"(AMOUNT NUMBER, LABEL VARCHAR)`,
			want: []ProcParam{{"AMOUNT", "NUMBER"}, {"LABEL", "VARCHAR"}},
		},
		{
			name: "size qualifier comma is not a separator",
			ddl:  `CREATE PROCEDURE P(AMOUNT NUMBER(38,0))`,
			want: []ProcParam{{"AMOUNT", "NUMBER(38,0)"}},
		},
		{
			name: "no params",
			ddl:  `CREATE PROCEDURE P() RETURNS VARCHAR`,
			want: nil,
		},
		{
			name: "bare type without a name",
			ddl:  `CREATE PROCEDURE P(NUMBER(38,0))`,
			want: []ProcParam{{"param", "NUMBER(38,0)"}},
		},
		{
			name: "DEFAULT clause is dropped",
			ddl:  `CREATE PROCEDURE P(AMOUNT NUMBER DEFAULT 0)`,
			want: []ProcParam{{"AMOUNT", "NUMBER"}},
		},
		{
			// A '(' inside the quoted procedure name used to be mistaken for
			// the start of the parameter list.
			name: "paren inside the quoted procedure name",
			ddl:  `CREATE PROCEDURE "DB"."SCH"."P(legacy)"(AMOUNT NUMBER)`,
			want: []ProcParam{{"AMOUNT", "NUMBER"}},
		},
		{
			// The comma inside the string literal is not a parameter
			// separator, and DEFAULT inside it is not a DEFAULT clause.
			name: "comma inside a string-literal default",
			ddl:  `CREATE PROCEDURE P(TAG VARCHAR DEFAULT 'a,b DEFAULT c', N NUMBER)`,
			want: []ProcParam{{"TAG", "VARCHAR"}, {"N", "NUMBER"}},
		},
		{
			name: "quoted parameter name",
			ddl:  `CREATE PROCEDURE P("my param" NUMBER)`,
			want: []ProcParam{{`"my param"`, "NUMBER"}},
		},
		{
			name: "unbalanced parens yield nothing",
			ddl:  `CREATE PROCEDURE P(AMOUNT NUMBER`,
			want: nil,
		},
		{
			name: "no parens yield nothing",
			ddl:  `CREATE PROCEDURE P`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProcedureDDL(tt.ddl)
			if len(got) != len(tt.want) {
				t.Fatalf("parseProcedureDDL(%q) = %+v, want %+v", tt.ddl, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ─── hasReturnsTable ─────────────────────────────────────────────────────────

func TestHasReturnsTable(t *testing.T) {
	tests := []struct {
		name string
		ddl  string
		want bool
	}{
		{
			name: "UDTF",
			ddl:  `CREATE FUNCTION F(X INT) RETURNS TABLE (Y INT) AS 'SELECT 1'`,
			want: true,
		},
		{
			name: "newline between the keywords",
			ddl:  "CREATE FUNCTION F(X INT) RETURNS\nTABLE (Y INT) AS 'SELECT 1'",
			want: true,
		},
		{
			name: "scalar function",
			ddl:  `CREATE FUNCTION F(X INT) RETURNS INT AS 'SELECT X'`,
			want: false,
		},
		{
			// A docstring in a dollar-quoted Python body used to misclassify
			// the scalar UDF as a table function.
			name: "phrase inside a dollar-quoted body",
			ddl: "CREATE FUNCTION F(X INT) RETURNS INT LANGUAGE PYTHON AS $$\n" +
				"def f(x):\n    \"\"\"This returns table row counts.\"\"\"\n    return x\n$$",
			want: false,
		},
		{
			name: "phrase inside a comment",
			ddl:  "-- returns table of counts\nCREATE FUNCTION F(X INT) RETURNS INT AS 'SELECT X'",
			want: false,
		},
		{
			name: "phrase inside a string literal",
			ddl:  `CREATE FUNCTION F(X INT) RETURNS INT COMMENT='returns table rows' AS 'SELECT X'`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasReturnsTable(tt.ddl); got != tt.want {
				t.Errorf("hasReturnsTable(%q) = %v, want %v", tt.ddl, got, tt.want)
			}
		})
	}
}

// ─── parseFinalizeFromDDLText ────────────────────────────────────────────────

func TestParseFinalizeFromDDLText(t *testing.T) {
	tests := []struct {
		name string
		ddl  string
		want string
	}{
		{
			name: "bare name",
			ddl:  `CREATE TASK T FINALIZE = ROOT_TASK AS SELECT 1`,
			want: "ROOT_TASK",
		},
		{
			name: "qualified name",
			ddl:  `CREATE TASK T FINALIZE = DB.SCH.ROOT_TASK AS SELECT 1`,
			want: "DB.SCH.ROOT_TASK",
		},
		{
			name: "trailing semicolon is not part of the value",
			ddl:  `CREATE TASK T FINALIZE = ROOT_TASK;`,
			want: "ROOT_TASK",
		},
		{
			name: "absent",
			ddl:  `CREATE TASK T AS SELECT 1`,
			want: "",
		},
		{
			// The word inside the task body is not a FINALIZE clause.
			name: "word inside a string literal in the task body",
			ddl:  `CREATE TASK T AS SELECT 'FINALIZE = x' AS c`,
			want: "",
		},
		{
			name: "word inside a comment",
			ddl:  "-- FINALIZE = old_root\nCREATE TASK T AS SELECT 1",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFinalizeFromDDLText(tt.ddl); got != tt.want {
				t.Errorf("parseFinalizeFromDDLText(%q) = %q, want %q", tt.ddl, got, tt.want)
			}
		})
	}
}
