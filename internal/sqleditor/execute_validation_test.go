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
			name: "Tagged dollar-quoted block",
			sql:  "EXECUTE IMMEDIATE $tag$SELECT 1$tag$",
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
			name: "USING without space before paren",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING(val)",
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
			name: "Empty string literal is still a valid argument",
			sql:  "EXECUTE IMMEDIATE ''",
		},
		{
			name: "USING keyword inside string literal does not trigger false positive",
			sql:  "EXECUTE IMMEDIATE 'INSERT INTO t USING (src)'",
		},
		{
			name: "String literal with doubled quotes — stripping handles escaped quotes",
			sql:  "EXECUTE IMMEDIATE 'SELECT ''hello'' FROM t'",
		},
		{
			name: "Bare colon as argument — satisfies has-arg regex",
			sql:  "EXECUTE IMMEDIATE :",
		},
		{
			name: "Function call as argument",
			sql:  "EXECUTE IMMEDIATE CONCAT('SELECT ', '1')",
		},
		{
			name: "Leading whitespace before EXECUTE",
			sql:  "   EXECUTE IMMEDIATE 'SELECT 1'",
		},
		{
			name: "Multiline dollar-quoted block",
			sql:  "EXECUTE IMMEDIATE $$\nSELECT 1\nFROM t\n$$",
		},
		{
			name: "USING inside dollar-quoted block does not trigger false positive",
			sql:  "EXECUTE IMMEDIATE $$MERGE INTO t USING (SELECT 1 AS id) AS src ON t.id = src.id WHEN MATCHED THEN DELETE$$",
		},
		{
			name: "Dollar-quoted block with internal USING plus real external USING",
			sql:  "EXECUTE IMMEDIATE $$MERGE INTO t USING src$$ USING (v1)",
		},
		{
			name: "Block comment between keyword and argument",
			sql:  "EXECUTE IMMEDIATE /* pick the sql */ 'SELECT 1'",
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
		{
			name: "Tab between keywords",
			sql:  "EXECUTE\tIMMEDIATE 'SELECT 1'",
		},
		{
			name: "Line comment between keyword and argument on next line",
			sql:  "EXECUTE IMMEDIATE -- pick sql\n'SELECT 1'",
		},
		{
			name: "Multiple spaces between EXECUTE and IMMEDIATE",
			sql:  "EXECUTE   IMMEDIATE 'SELECT 1'",
		},
		{
			name: "USING clause with quoted identifier bind variable",
			sql:  `EXECUTE IMMEDIATE 'SELECT $1' USING ("MyVar")`,
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
			name:          "EXECUTE IMMEDIATE with trailing whitespace only",
			sql:           "EXECUTE IMMEDIATE   ",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "Line comment as only argument — stripped to nothing",
			sql:           "EXECUTE IMMEDIATE -- this is not an argument",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "Block comment as only argument — stripped to nothing",
			sql:           "EXECUTE IMMEDIATE /* not an arg */",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "Newline then semicolon",
			sql:           "EXECUTE IMMEDIATE\n;",
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
		{
			name:          "USING with whitespace inside empty parens",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING ( )",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING without space before paren — empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name: "USING keyword treated as bare identifier argument — no missing-arg warning",
			sql:  "EXECUTE IMMEDIATE USING (val)",
		},
		{
			name: "Lowercase USING keyword — case insensitive",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' using (v1)",
		},
		{
			name: "USING without parentheses — no false positive",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING val",
		},

		// ── Additional Invalid Cases ────────────────────────────────────────
		{
			name:          "USING clause with only commas — no valid identifier",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (,)",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING clause with commas and spaces — no valid identifier",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING ( , , )",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING treated as arg but empty USING clause still warns",
			sql:           "EXECUTE IMMEDIATE USING ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "Mixed case Using with empty parens",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' Using ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with block comment inside parens — stripped to empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (/* val */)",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with line comment inside parens — stripped to empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (-- val\n)",
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
		{
			name: "Block comment between keyword and task name",
			sql:  "EXECUTE TASK /* pick task */ my_task",
		},
		{
			name: "Tab between keywords",
			sql:  "EXECUTE\tTASK my_task",
		},
		{
			name: "Line comment between keyword and task name on next line",
			sql:  "EXECUTE TASK -- pick task\nmy_task",
		},
		{
			name: "Task name starting with underscore",
			sql:  "EXECUTE TASK _my_task",
		},
		{
			name: "Task name with dollar sign in identifier",
			sql:  "EXECUTE TASK my$task",
		},
		{
			name: "Mixed quoted and unquoted parts in qualified name",
			sql:  `EXECUTE TASK db."my schema".my_task`,
		},
		{
			name: "Leading whitespace before EXECUTE TASK",
			sql:  "   EXECUTE TASK my_task",
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
		{
			name:          "Line comment as only task name — stripped to nothing",
			sql:           "EXECUTE TASK -- not a task name",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Block comment as only task name — stripped to nothing",
			sql:           "EXECUTE TASK /* not a name */",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Newline then semicolon — no task name",
			sql:           "EXECUTE TASK\n;",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Task name starting with digit — not a valid identifier",
			sql:           "EXECUTE TASK 123task",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Empty quoted identifier — not a valid task name",
			sql:           `EXECUTE TASK ""`,
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name: "Spaces around dots — db alone matches as single-part name",
			sql:  "EXECUTE TASK db . schema . task",
		},
	})
}

// TestExecuteMultiStatement ensures EXECUTE validation works correctly when
// EXECUTE statements appear inside multi-statement SQL.
func TestExecuteMultiStatement(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		{
			name: "Valid EXECUTE IMMEDIATE as second statement",
			sql:  "SELECT 1;\nEXECUTE IMMEDIATE 'SELECT 2'",
		},
		{
			name: "Valid EXECUTE TASK as second statement",
			sql:  "SELECT 1;\nEXECUTE TASK my_task",
		},
		{
			name:          "Invalid EXECUTE IMMEDIATE in multi-statement",
			sql:           "SELECT 1;\nEXECUTE IMMEDIATE",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "Invalid EXECUTE TASK in multi-statement",
			sql:           "SELECT 1;\nEXECUTE TASK",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
	})
}

// TestExecuteMultiStatementBothInvalid ensures that two invalid EXECUTE
// statements in the same SQL each independently produce a warning.
func TestExecuteMultiStatementBothInvalid(t *testing.T) {
	sql := "EXECUTE IMMEDIATE;\nEXECUTE TASK"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warnings := getWarnings(markers)

	if len(warnings) < 2 {
		t.Fatalf("Expected at least 2 warnings, got %d: %v", len(warnings), warnings)
	}
	foundImm, foundTask := false, false
	for _, w := range warnings {
		msg := strings.ToLower(w.Message)
		if strings.Contains(msg, "requires a sql string argument") {
			foundImm = true
		}
		if strings.Contains(msg, "requires a task name") {
			foundTask = true
		}
	}
	if !foundImm {
		t.Error("Missing EXECUTE IMMEDIATE warning")
	}
	if !foundTask {
		t.Error("Missing EXECUTE TASK warning")
	}
}

// TestExecuteMultiStatementMixedVariants tests EXECUTE IMMEDIATE and EXECUTE
// TASK together in multi-statement SQL with both valid.
func TestExecuteMultiStatementMixedVariants(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		{
			name: "Valid EXECUTE IMMEDIATE then valid EXECUTE TASK",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1';\nEXECUTE TASK my_task",
		},
		{
			name: "Valid EXECUTE TASK then valid EXECUTE IMMEDIATE",
			sql:  "EXECUTE TASK my_task;\nEXECUTE IMMEDIATE :sql_var",
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
		{name: "Bare EXECUTE — no subcommand", sql: "EXECUTE"},
	})
}
