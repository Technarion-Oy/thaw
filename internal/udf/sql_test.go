// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package udf

import (
	"strings"
	"testing"
)

func TestBuildCreateFunctionSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      FunctionConfig
		contains []string
		absent   []string
	}{
		{
			name: "minimal SQL scalar UDF",
			cfg: FunctionConfig{
				Name:       "ADD_ONE",
				ReturnType: "NUMBER",
				Body:       "x + 1",
			},
			contains: []string{
				`CREATE FUNCTION "DB"."SC".ADD_ONE()`,
				"\n  RETURNS NUMBER",
				"\n  AS $$\nx + 1\n$$;",
			},
			absent: []string{"LANGUAGE", "OR REPLACE", "SECURE", "IF NOT EXISTS"},
		},
		{
			name: "OR REPLACE",
			cfg: FunctionConfig{
				Name:       "FN",
				OrReplace:  true,
				ReturnType: "NUMBER",
				Body:       "1",
			},
			contains: []string{`CREATE OR REPLACE FUNCTION "DB"."SC".FN()`},
		},
		{
			name: "SECURE",
			cfg: FunctionConfig{
				Name:       "FN",
				Secure:     true,
				ReturnType: "NUMBER",
				Body:       "1",
			},
			contains: []string{`CREATE SECURE FUNCTION "DB"."SC".FN()`},
		},
		{
			name: "IF NOT EXISTS",
			cfg: FunctionConfig{
				Name:        "FN",
				IfNotExists: true,
				ReturnType:  "NUMBER",
				Body:        "1",
			},
			contains: []string{`CREATE FUNCTION IF NOT EXISTS "DB"."SC".FN()`},
		},
		{
			name: "args",
			cfg: FunctionConfig{
				Name:       "ADD",
				Args:       []FuncArg{{Name: "x", DataType: "NUMBER"}, {Name: "y", DataType: "NUMBER"}},
				ReturnType: "NUMBER",
				Body:       "x + y",
			},
			contains: []string{`ADD(x NUMBER, y NUMBER)`},
		},
		{
			name: "RETURNS TABLE",
			cfg: FunctionConfig{
				Name:         "PETS",
				ReturnsTable: true,
				TableColumns: []FuncArg{{Name: "id", DataType: "NUMBER"}, {Name: "nm", DataType: "VARCHAR"}},
				Body:         "SELECT id, nm FROM pets",
			},
			contains: []string{"\n  RETURNS TABLE (id NUMBER, nm VARCHAR)"},
		},
		{
			name: "Python UDF",
			cfg: FunctionConfig{
				Name:           "PY_FN",
				ReturnType:     "STRING",
				Language:       "python",
				RuntimeVersion: "3.10",
				Packages:       []string{"numpy", "pandas"},
				Handler:        "compute",
				Body:           "def compute():\n    return 'x'",
			},
			contains: []string{
				"\n  LANGUAGE PYTHON",
				"\n  RUNTIME_VERSION = '3.10'",
				"\n  PACKAGES = ('numpy', 'pandas')",
				"\n  HANDLER = 'compute'",
				"def compute():",
			},
		},
		{
			name: "null handling and volatility",
			cfg: FunctionConfig{
				Name:         "FN",
				ReturnType:   "NUMBER",
				NullHandling: "returns null on null input",
				Volatility:   "immutable",
				Body:         "1",
			},
			contains: []string{
				"\n  RETURNS NULL ON NULL INPUT",
				"\n  IMMUTABLE",
			},
		},
		{
			name: "imports",
			cfg: FunctionConfig{
				Name:       "FN",
				ReturnType: "STRING",
				Language:   "java",
				Imports:    []string{"@stage/a.jar", "@stage/b.jar"},
				Handler:    "Cls.run",
				Body:       "// code",
			},
			contains: []string{
				"\n  IMPORTS = ('@stage/a.jar', '@stage/b.jar')",
				"\n  LANGUAGE JAVA",
			},
		},
		{
			name: "comment escaped",
			cfg: FunctionConfig{
				Name:       "FN",
				ReturnType: "NUMBER",
				Comment:    "it's mine",
				Body:       "1",
			},
			contains: []string{"COMMENT = 'it''s mine'"},
		},
		{
			name: "case-sensitive name",
			cfg: FunctionConfig{
				Name:          "myFn",
				CaseSensitive: true,
				ReturnType:    "NUMBER",
				Body:          "1",
			},
			contains: []string{`"myFn"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateFunctionSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, c := range tt.contains {
				if !strings.Contains(sql, c) {
					t.Errorf("expected SQL to contain %q\nfull SQL:\n%s", c, sql)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(sql, a) {
					t.Errorf("expected SQL NOT to contain %q\nfull SQL:\n%s", a, sql)
				}
			}
		})
	}
}

func TestBuildCreateFunctionSql_Placeholders(t *testing.T) {
	sql, err := BuildCreateFunctionSql("DB", "SC", FunctionConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range []string{"function_name()", "RETURNS VARIANT", "<function_body>"} {
		if !strings.Contains(sql, c) {
			t.Errorf("expected placeholder %q in:\n%s", c, sql)
		}
	}
}

func TestBuildArgList_SkipsBlankNames(t *testing.T) {
	got := buildArgList([]FuncArg{
		{Name: "a", DataType: "INT"},
		{Name: "  ", DataType: "INT"},
		{Name: "b", DataType: ""},
	})
	if got != "a INT, b VARIANT" {
		t.Errorf("buildArgList = %q", got)
	}
}
