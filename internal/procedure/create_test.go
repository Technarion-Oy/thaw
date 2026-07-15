// SPDX-License-Identifier: GPL-3.0-or-later

package procedure

import (
	"strings"
	"testing"
)

func TestBuildCreateProcedureSql(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ProcedureConfig
		want    []string // substrings that must appear
		notWant []string // substrings that must NOT appear
	}{
		{
			name: "minimal SQL procedure",
			cfg: ProcedureConfig{
				Name:       "MY_PROC",
				ReturnType: "VARCHAR",
				Body:       "BEGIN\n  RETURN 'hi';\nEND",
			},
			want: []string{
				`CREATE PROCEDURE "DB"."SC".MY_PROC()`,
				"\n  RETURNS VARCHAR",
				"\n  LANGUAGE SQL", // required for CREATE PROCEDURE (no implicit default)
				"\n  AS $$\nBEGIN\n  RETURN 'hi';\nEND\n$$;",
			},
			notWant: []string{"EXECUTE AS", "COMMENT"},
		},
		{
			name: "default body and return when empty",
			cfg:  ProcedureConfig{Name: "P"},
			want: []string{
				"RETURNS VARIANT",
				"AS $$\nBEGIN\n  RETURN 1;\nEND\n$$;",
			},
		},
		{
			name: "placeholder name when empty",
			cfg:  ProcedureConfig{},
			want: []string{`procedure_name()`},
		},
		{
			name: "OR REPLACE",
			cfg:  ProcedureConfig{Name: "P", OrReplace: true},
			want: []string{"CREATE OR REPLACE PROCEDURE"},
		},
		{
			name: "SECURE",
			cfg:  ProcedureConfig{Name: "P", Secure: true},
			want: []string{"CREATE SECURE PROCEDURE"},
		},
		{
			name: "OR REPLACE SECURE together",
			cfg:  ProcedureConfig{Name: "P", OrReplace: true, Secure: true},
			want: []string{"CREATE OR REPLACE SECURE PROCEDURE"},
		},
		{
			name: "IF NOT EXISTS",
			cfg:  ProcedureConfig{Name: "P", IfNotExists: true},
			want: []string{"PROCEDURE IF NOT EXISTS"},
		},
		{
			name: "args",
			cfg: ProcedureConfig{
				Name: "ADD",
				Args: []ProcArg{
					{Name: "x", DataType: "NUMBER"},
					{Name: "y", DataType: "VARCHAR"},
					{Name: "  ", DataType: "INT"}, // skipped (blank name)
					{Name: "z", DataType: ""},     // blank type -> VARIANT
				},
			},
			want: []string{"(x NUMBER, y VARCHAR, z VARIANT)"},
		},
		{
			name: "RETURNS TABLE",
			cfg: ProcedureConfig{
				Name:         "T",
				ReturnsTable: true,
				TableColumns: []ProcArg{
					{Name: "id", DataType: "NUMBER"},
					{Name: "label", DataType: "VARCHAR"},
				},
			},
			want:    []string{"RETURNS TABLE (id NUMBER, label VARCHAR)"},
			notWant: []string{"RETURNS VARIANT"},
		},
		{
			name: "Python procedure",
			cfg: ProcedureConfig{
				Name:           "PY_PROC",
				ReturnType:     "STRING",
				Language:       "python",
				RuntimeVersion: "3.10",
				Packages:       []string{"snowflake-snowpark-python", "pandas"},
				Imports:        []string{"@stage/handler.py"},
				Handler:        "main.run",
				Body:           "def run(session):\n    return 'ok'",
			},
			want: []string{
				"\n  LANGUAGE PYTHON",
				"\n  RUNTIME_VERSION = '3.10'",
				"\n  PACKAGES = ('snowflake-snowpark-python', 'pandas')",
				"\n  IMPORTS = ('@stage/handler.py')",
				"\n  HANDLER = 'main.run'",
				"def run(session):",
			},
		},
		{
			name: "SQL language always emitted (required for procedures)",
			cfg:  ProcedureConfig{Name: "P", Language: "SQL"},
			want: []string{"\n  LANGUAGE SQL"},
		},
		{
			name: "empty language defaults to LANGUAGE SQL",
			cfg:  ProcedureConfig{Name: "P"},
			want: []string{"\n  LANGUAGE SQL"},
		},
		{
			name: "SQL procedure drops handler-only clauses even when set",
			cfg: ProcedureConfig{
				Name:           "P",
				Language:       "SQL",
				RuntimeVersion: "3.10",
				Packages:       []string{"snowflake-snowpark-python"},
				Imports:        []string{"@stage/h.py"},
				Handler:        "main.run",
				Body:           "BEGIN\n  RETURN 1;\nEND",
			},
			want:    []string{"\n  LANGUAGE SQL"},
			notWant: []string{"RUNTIME_VERSION", "PACKAGES", "IMPORTS", "HANDLER"},
		},
		{
			name: "JavaScript procedure drops handler-only clauses",
			cfg: ProcedureConfig{
				Name:           "P",
				Language:       "javascript",
				RuntimeVersion: "ignored",
				Packages:       []string{"ignored"},
				Handler:        "ignored",
				Body:           "return 1;",
			},
			want:    []string{"\n  LANGUAGE JAVASCRIPT"},
			notWant: []string{"RUNTIME_VERSION", "PACKAGES", "IMPORTS", "HANDLER"},
		},
		{
			name: "EXECUTE AS CALLER",
			cfg:  ProcedureConfig{Name: "P", ExecuteAs: "caller"},
			want: []string{"\n  EXECUTE AS CALLER"},
		},
		{
			name: "EXECUTE AS OWNER",
			cfg:  ProcedureConfig{Name: "P", ExecuteAs: "OWNER"},
			want: []string{"\n  EXECUTE AS OWNER"},
		},
		{
			name: "null handling",
			cfg:  ProcedureConfig{Name: "P", NullHandling: "RETURNS NULL ON NULL INPUT"},
			want: []string{"\n  RETURNS NULL ON NULL INPUT"},
		},
		{
			name: "volatility",
			cfg:  ProcedureConfig{Name: "P", Volatility: "immutable"},
			want: []string{"\n  IMMUTABLE"},
		},
		{
			name: "comment escaped",
			cfg:  ProcedureConfig{Name: "P", Comment: "it's a proc"},
			want: []string{"COMMENT = 'it''s a proc'"},
		},
		{
			name: "case-sensitive name",
			cfg:  ProcedureConfig{Name: "myProc", CaseSensitive: true},
			want: []string{`"myProc"`},
		},
		{
			name: "body containing $$ uses an alternate dollar-quote tag",
			cfg: ProcedureConfig{
				Name: "P",
				Body: "BEGIN\n  LET x := '$$';\n  RETURN x;\nEND",
			},
			want:    []string{"AS $thaw$\n", "\n$thaw$;"},
			notWant: []string{"AS $$"},
		},
	}

	// COMMENT must precede EXECUTE AS per the CREATE PROCEDURE grammar.
	t.Run("COMMENT precedes EXECUTE AS", func(t *testing.T) {
		sql, err := BuildCreateProcedureSql("DB", "SC", ProcedureConfig{
			Name: "P", Comment: "note", ExecuteAs: "OWNER",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ci := strings.Index(sql, "COMMENT =")
		ei := strings.Index(sql, "EXECUTE AS")
		if ci < 0 || ei < 0 {
			t.Fatalf("expected both COMMENT and EXECUTE AS in:\n%s", sql)
		}
		if ci > ei {
			t.Errorf("COMMENT must precede EXECUTE AS\nfull SQL:\n%s", sql)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateProcedureSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(sql, w) {
					t.Errorf("expected SQL to contain %q\nfull SQL:\n%s", w, sql)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(sql, nw) {
					t.Errorf("expected SQL NOT to contain %q\nfull SQL:\n%s", nw, sql)
				}
			}
		})
	}
}
