// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package procedure

import (
	"testing"
)

func TestBuildCallStatement(t *testing.T) {
	tests := []struct {
		name     string
		db       string
		schema   string
		proc     string
		args     []Argument
		expected string
	}{
		{
			name:   "no arguments",
			db:     "DB",
			schema: "PUBLIC",
			proc:   "MY_PROC",
			args:   []Argument{},
			expected: "CALL \"DB\".\"PUBLIC\".\"MY_PROC\"();",
		},
		{
			name:   "mixed arguments",
			db:     "DB",
			schema: "PUBLIC",
			proc:   "MY_PROC",
			args: []Argument{
				{Name: "P1", DataType: "NUMBER", Value: "123"},
				{Name: "P2", DataType: "VARCHAR", Value: "hello"},
				{Name: "P3", DataType: "BOOLEAN", Value: "TRUE"},
				{Name: "P4", DataType: "VARCHAR", Value: ""},
			},
			expected: "CALL \"DB\".\"PUBLIC\".\"MY_PROC\"(123, 'hello', TRUE, NULL);",
		},
		{
			name:   "escaping quotes",
			db:     "MY\"DB",
			schema: "PUBLIC",
			proc:   "MY_PROC",
			args: []Argument{
				{Name: "P1", DataType: "VARCHAR", Value: "O'Reilly"},
			},
			expected: "CALL \"MY\"\"DB\".\"PUBLIC\".\"MY_PROC\"('O''Reilly');",
		},
		{
			name:   "numeric types with scale",
			db:     "DB",
			schema: "PUBLIC",
			proc:   "MY_PROC",
			args: []Argument{
				{Name: "P1", DataType: "NUMBER(38,0)", Value: "456"},
				{Name: "P2", DataType: "DECIMAL(10,2)", Value: "12.34"},
			},
			expected: "CALL \"DB\".\"PUBLIC\".\"MY_PROC\"(456, 12.34);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCallStatement(tt.db, tt.schema, tt.proc, tt.args)
			if got != tt.expected {
				t.Errorf("BuildCallStatement() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildFunctionSelectStatement(t *testing.T) {
	tests := []struct {
		name            string
		db              string
		schema          string
		fn              string
		args            []Argument
		isTableFunction bool
		expected        string
	}{
		{
			name:            "scalar function",
			db:              "DB",
			schema:          "PUBLIC",
			fn:              "MY_FN",
			args:            []Argument{{Name: "P1", DataType: "VARCHAR", Value: "test"}},
			isTableFunction: false,
			expected:        "SELECT \"DB\".\"PUBLIC\".\"MY_FN\"('test') AS result LIMIT 1000;",
		},
		{
			name:            "table function",
			db:              "DB",
			schema:          "PUBLIC",
			fn:              "MY_TF",
			args:            []Argument{{Name: "P1", DataType: "NUMBER", Value: "1"}},
			isTableFunction: true,
			expected:        "SELECT * FROM TABLE(\"DB\".\"PUBLIC\".\"MY_TF\"(1)) LIMIT 1000;",
		},
		{
			name:            "bool type variants",
			db:              "DB",
			schema:          "PUBLIC",
			fn:              "MY_FN",
			args:            []Argument{{Name: "P1", DataType: "BOOL", Value: "FALSE"}},
			isTableFunction: false,
			expected:        "SELECT \"DB\".\"PUBLIC\".\"MY_FN\"(FALSE) AS result LIMIT 1000;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFunctionSelectStatement(tt.db, tt.schema, tt.fn, tt.args, tt.isTableFunction)
			if got != tt.expected {
				t.Errorf("BuildFunctionSelectStatement() = %v, want %v", got, tt.expected)
			}
		})
	}
}
