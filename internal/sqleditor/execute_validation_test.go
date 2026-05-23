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

// patternTestCase is a single PASS/FAIL case for ValidateSnowflakePatterns.
type patternTestCase struct {
	name          string
	sql           string
	expectWarning bool
	expectedMatch string // substring that must appear in a warning message (when expectWarning is true)
}

// runPatternTests runs a slice of patternTestCase entries against
// ValidateSnowflakePatterns and reports failures via t.
func runPatternTests(t *testing.T, cases []patternTestCase) {
	t.Helper()
	for _, tt := range cases {
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

func TestValidateSnowflakePatterns_ExecuteImmediate(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "String literal argument",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1'",
		},
		{
			name: "Colon-prefixed variable",
			sql:  "EXECUTE IMMEDIATE :my_sql_var",
		},
		{
			name: "Bare identifier variable",
			sql:  "EXECUTE IMMEDIATE my_sql_var",
		},
		{
			name: "Dollar-quoted block",
			sql:  "EXECUTE IMMEDIATE $$SELECT 1$$",
		},
		{
			name: "String literal with USING clause",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING (val)",
		},
		{
			name: "Variable with USING clause — multiple bind vars",
			sql:  "EXECUTE IMMEDIATE :sql_str USING (a, b, c)",
		},
		{
			name: "String literal with multi-var USING",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1, $2' USING (v1, v2)",
		},
		{
			name: "Quoted identifier variable",
			sql:  `EXECUTE IMMEDIATE "my_sql_var"`,
		},
		{
			name: "String literal with semicolon terminator",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1';",
		},
		{
			name: "USING keyword inside string literal does not trigger false positive",
			sql:  "EXECUTE IMMEDIATE 'INSERT INTO t USING (src)'",
		},
		{
			name: "USING inside dollar-quoted block does not trigger false positive",
			sql:  "EXECUTE IMMEDIATE $$MERGE INTO t USING (SELECT 1 AS id) AS src ON t.id = src.id WHEN MATCHED THEN DELETE$$",
		},
		{
			name: "Lowercase execute immediate — case insensitive",
			sql:  "execute immediate 'SELECT 1'",
		},
		{
			name: "Mixed case execute immediate — case insensitive",
			sql:  "Execute Immediate 'SELECT 1'",
		},
		{
			name: "Argument on next line — multiline input",
			sql:  "EXECUTE IMMEDIATE\n  'SELECT 1'",
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
	})
}

func TestValidateSnowflakePatterns_ExecuteTask(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "Simple task name",
			sql:  "EXECUTE TASK my_task",
		},
		{
			name: "Schema-qualified task name",
			sql:  "EXECUTE TASK my_schema.my_task",
		},
		{
			name: "Fully qualified task name",
			sql:  "EXECUTE TASK my_db.my_schema.my_task",
		},
		{
			name: "Quoted task name",
			sql:  `EXECUTE TASK "My Task"`,
		},
		{
			name: "Task name with trailing semicolon",
			sql:  "EXECUTE TASK my_task;",
		},
		{
			name: "Lowercase execute task — case insensitive",
			sql:  "execute task my_task",
		},
		{
			name: "Mixed case execute task",
			sql:  "Execute Task my_task",
		},
		{
			name: "Quoted fully qualified task name",
			sql:  `EXECUTE TASK "my_db"."my_schema"."My Task"`,
		},
		{
			name: "Task name with extra whitespace",
			sql:  "EXECUTE TASK   my_task",
		},
		{
			name: "Task name on next line",
			sql:  "EXECUTE TASK\n  my_task",
		},
		// ── EXECUTE TASK — RETRY LAST (Section B) ────────────────────────────
		{
			name: "EXECUTE TASK with RETRY LAST",
			sql:  "EXECUTE TASK my_task RETRY LAST",
		},
		{
			name: "EXECUTE TASK RETRY LAST fully qualified",
			sql:  "EXECUTE TASK db.schema.my_task RETRY LAST",
		},
		// ── EXECUTE TASK — USING CONFIG (Section B) ──────────────────────────
		{
			name: "EXECUTE TASK with USING CONFIG",
			sql:  `EXECUTE TASK my_task USING CONFIG = '{"key": "val"}'`,
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
		{
			name:          "Lowercase execute task — no task name",
			sql:           "execute task",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "EXECUTE TASK with space then semicolon",
			sql:           "EXECUTE TASK ;",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "EXECUTE TASK with trailing whitespace only",
			sql:           "EXECUTE TASK   ",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
	})
}

// TestExecuteOtherForms ensures that other EXECUTE variants (ALERT, MANAGED TASK)
// do not produce spurious warnings.
func TestExecuteOtherForms(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		{name: "EXECUTE ALERT", sql: "EXECUTE ALERT my_alert"},
		{name: "EXECUTE MANAGED TASK", sql: "EXECUTE MANAGED TASK my_task"},
		{name: "EXECUTE ALERT qualified", sql: "EXECUTE ALERT db.schema.my_alert"},
		{name: "lowercase EXECUTE ALERT", sql: "execute alert my_alert"},
	})
}
