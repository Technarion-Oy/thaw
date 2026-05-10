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
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_ExecuteImmediate(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "String literal argument",
			sql:           "EXECUTE IMMEDIATE 'SELECT 1'",
			expectWarning: false,
		},
		{
			name:          "Colon-prefixed variable",
			sql:           "EXECUTE IMMEDIATE :my_sql_var",
			expectWarning: false,
		},
		{
			name:          "Bare identifier variable",
			sql:           "EXECUTE IMMEDIATE my_sql_var",
			expectWarning: false,
		},
		{
			name:          "Dollar-quoted block",
			sql:           "EXECUTE IMMEDIATE $$SELECT 1$$",
			expectWarning: false,
		},
		{
			name:          "String literal with USING clause",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (val)",
			expectWarning: false,
		},
		{
			name:          "Variable with USING clause — multiple bind vars",
			sql:           "EXECUTE IMMEDIATE :sql_str USING (a, b, c)",
			expectWarning: false,
		},
		{
			name:          "String literal with multi-var USING",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1, $2' USING (v1, v2)",
			expectWarning: false,
		},
		{
			name:          "Quoted identifier variable",
			sql:           `EXECUTE IMMEDIATE "my_sql_var"`,
			expectWarning: false,
		},
		{
			name:          "String literal with semicolon terminator",
			sql:           "EXECUTE IMMEDIATE 'SELECT 1';",
			expectWarning: false,
		},
		{
			name:          "USING keyword inside string literal does not trigger false positive",
			sql:           "EXECUTE IMMEDIATE 'INSERT INTO t USING (src)'",
			expectWarning: false,
		},
		{
			name:          "USING inside dollar-quoted block does not trigger false positive",
			sql:           "EXECUTE IMMEDIATE $$MERGE INTO t USING (SELECT 1 AS id) AS src ON t.id = src.id WHEN MATCHED THEN DELETE$$",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare EXECUTE IMMEDIATE — no argument",
			sql:           "EXECUTE IMMEDIATE",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "EXECUTE IMMEDIATE with only a semicolon",
			sql:           "EXECUTE IMMEDIATE;",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "EXECUTE IMMEDIATE with space then semicolon",
			sql:           "EXECUTE IMMEDIATE ;",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "EXECUTE IMMEDIATE with empty USING clause",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "Variable with empty USING clause",
			sql:           "EXECUTE IMMEDIATE :sql_str USING ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)

			if tt.expectWarning {
				if len(warnings) == 0 {
					t.Fatalf("Expected warnings for %q, got 0", tt.sql)
				}
				found := false
				for _, w := range warnings {
					if strings.Contains(strings.ToLower(w.Message), strings.ToLower(tt.expectedMatch)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning matching %q, got: %v", tt.expectedMatch, warnings[0].Message)
				}
			} else {
				if len(warnings) > 0 {
					t.Errorf("Expected 0 warnings for %q, got %d: %v", tt.sql, len(warnings), warnings)
				}
			}
		})
	}
}

func TestValidateSnowflakePatterns_ExecuteTask(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "Simple task name",
			sql:           "EXECUTE TASK my_task",
			expectWarning: false,
		},
		{
			name:          "Schema-qualified task name",
			sql:           "EXECUTE TASK my_schema.my_task",
			expectWarning: false,
		},
		{
			name:          "Fully qualified task name",
			sql:           "EXECUTE TASK my_db.my_schema.my_task",
			expectWarning: false,
		},
		{
			name:          "Quoted task name",
			sql:           `EXECUTE TASK "My Task"`,
			expectWarning: false,
		},
		{
			name:          "Task name with trailing semicolon",
			sql:           "EXECUTE TASK my_task;",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare EXECUTE TASK — no task name",
			sql:           "EXECUTE TASK",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "EXECUTE TASK with only a semicolon",
			sql:           "EXECUTE TASK;",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)

			if tt.expectWarning {
				if len(warnings) == 0 {
					t.Fatalf("Expected warnings for %q, got 0", tt.sql)
				}
				found := false
				for _, w := range warnings {
					if strings.Contains(strings.ToLower(w.Message), strings.ToLower(tt.expectedMatch)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning matching %q, got: %v", tt.expectedMatch, warnings[0].Message)
				}
			} else {
				if len(warnings) > 0 {
					t.Errorf("Expected 0 warnings for %q, got %d: %v", tt.sql, len(warnings), warnings)
				}
			}
		})
	}
}

// TestExecuteOtherForms ensures that other EXECUTE variants (ALERT, MANAGED TASK)
// do not produce spurious warnings.
func TestExecuteOtherForms(t *testing.T) {
	validCases := []string{
		"EXECUTE ALERT my_alert",
		"EXECUTE MANAGED TASK my_task",
	}

	for _, sql := range validCases {
		t.Run(sql, func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}
