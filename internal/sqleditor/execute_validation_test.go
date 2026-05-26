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
			name: "Numeric literal as argument — satisfies non-whitespace check",
			sql:  "EXECUTE IMMEDIATE 42",
		},
		{
			name: "USING clause with quoted identifier bind variable",
			sql:  `EXECUTE IMMEDIATE 'SELECT $1' USING ("MyVar")`,
		},
		{
			name: "Multiline string literal argument",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1\nFROM t\nWHERE id = 1'",
		},
		{
			name: "String literal containing USING + real external USING with valid bind vars",
			sql:  "EXECUTE IMMEDIATE 'MERGE INTO t USING (src)' USING (v1)",
		},
		{
			name: "CRLF between keyword and argument",
			sql:  "EXECUTE IMMEDIATE\r\n'SELECT 1'",
		},
		{
			name: "USING clause with newlines inside parens",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING (\n  v1\n)",
		},
		{
			name: "USING clause with tab before paren",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING\t(val)",
		},
		{
			name: "USING clause with trailing semicolon",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING (v1);",
		},
		{
			name: "USING clause with function call expression — function name matches ident",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING (UPPER(x))",
		},
		{
			name: "String literal containing semicolons — statement not split",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1; SELECT 2'",
		},
		{
			name: "Dollar-quoted block containing semicolons — statement not split",
			sql:  "EXECUTE IMMEDIATE $$SELECT 1; SELECT 2; SELECT 3$$",
		},
		{
			name: "Argument on third line — three-line span",
			sql:  "EXECUTE\n  IMMEDIATE\n  'SELECT 1'",
		},
		{
			name: "Newlines between argument and USING clause",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1'\n\nUSING (v1)",
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
		{
			name:          "USING with only newlines inside parens — stripped to empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (\n\n)",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with only tabs inside parens — stripped to empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (\t\t)",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "Dollar-quoted arg with empty external USING",
			sql:           "EXECUTE IMMEDIATE $$SELECT $1$$ USING ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "String literal with internal USING() and empty external USING",
			sql:           "EXECUTE IMMEDIATE 'MERGE INTO t USING ()' USING ()",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with empty quoted identifier — not a valid bind variable",
			sql:           `EXECUTE IMMEDIATE 'SELECT $1' USING ("")`,
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with string literal instead of bind variable — stripped to empty",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING ('val')",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},
		{
			name:          "USING with unclosed paren and no identifier",
			sql:           "EXECUTE IMMEDIATE 'SELECT $1' USING (",
			expectWarning: true,
			expectedMatch: "USING clause in EXECUTE IMMEDIATE must contain at least one bind variable",
		},

		// ── Additional Valid Cases ──────────────────────────────────────────
		{
			name: "Empty dollar-quoted block is a valid argument",
			sql:  "EXECUTE IMMEDIATE $$$$",
		},
		{
			name: "INTO clause after arg — only USING is checked, not other clauses",
			sql:  "EXECUTE IMMEDIATE 'SELECT 1' INTO :result",
		},
		{
			name: "USING with mixed quoted and unquoted bind variables",
			sql:  `EXECUTE IMMEDIATE 'SELECT $1, $2' USING ("MyVar", plain_var)`,
		},
		{
			name: "USING clause commented out via line comment — no USING warning",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' --USING (v1)",
		},
		{
			name: "USING clause commented out via block comment — no USING warning",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' /*USING ()*/",
		},
		{
			name: "Bare USING as variable — no parenthesized USING clause",
			sql:  "EXECUTE IMMEDIATE USING",
		},
		{
			name: "USING clause with unclosed paren containing identifier — regex still matches",
			sql:  "EXECUTE IMMEDIATE 'SELECT $1' USING (val",
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
			name: "Quoted identifier starting with digit — valid when quoted",
			sql:  `EXECUTE TASK "123task"`,
		},
		{
			name: "Quoted identifier containing dots — single identifier, not multi-part",
			sql:  `EXECUTE TASK "db.schema.task"`,
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
		{
			name: "Four-part identifier — regex matches first three parts",
			sql:  "EXECUTE TASK a.b.c.d",
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
		{
			name: "RETRY LAST and USING CONFIG combined",
			sql:  `EXECUTE TASK my_task RETRY LAST USING CONFIG = '{"key": "val"}'`,
		},
		{
			name: "CRLF between keyword and task name",
			sql:  "EXECUTE TASK\r\nmy_task",
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
			name:          "Unclosed quoted task name — regex requires closing quote",
			sql:           `EXECUTE TASK "my_task`,
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Dot-only path — not a valid identifier",
			sql:           "EXECUTE TASK .",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name:          "Leading dot before identifier — not a valid qualified name",
			sql:           "EXECUTE TASK .my_task",
			expectWarning: true,
			expectedMatch: "requires a task name",
		},
		{
			name: "Spaces around dots — db alone matches as single-part name",
			sql:  "EXECUTE TASK db . schema . task",
		},
		{
			name: "Task name that is a SQL keyword — accepted as bare identifier",
			sql:  "EXECUTE TASK SELECT",
		},
		{
			name: "Task name that is USING keyword — accepted as bare identifier",
			sql:  "EXECUTE TASK USING",
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
		{
			name:          "Valid then invalid EXECUTE IMMEDIATE in same multi-statement",
			sql:           "EXECUTE IMMEDIATE 'SELECT 1';\nEXECUTE IMMEDIATE",
			expectWarning: true,
			expectedMatch: "requires a SQL string argument",
		},
		{
			name:          "Valid then invalid EXECUTE TASK in same multi-statement",
			sql:           "EXECUTE TASK my_task;\nEXECUTE TASK",
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
			name: "EXECUTE IMMEDIATE with semicolons inside string in multi-statement",
			sql:  "EXECUTE IMMEDIATE 'INSERT INTO t; SELECT 1';\nSELECT 2",
		},
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

// TestExecuteMultiStatementDuplicateInvalid ensures that two invalid EXECUTE
// statements of the same type each independently produce a warning.
func TestExecuteMultiStatementDuplicateInvalid(t *testing.T) {
	t.Run("two EXECUTE IMMEDIATE without args", func(t *testing.T) {
		sql := "EXECUTE IMMEDIATE;\nEXECUTE IMMEDIATE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 2 {
			t.Fatalf("Expected exactly 2 warnings, got %d: %v", len(warnings), warnings)
		}
		for _, w := range warnings {
			if !strings.Contains(strings.ToLower(w.Message), "requires a sql string argument") {
				t.Errorf("Unexpected warning message: %s", w.Message)
			}
		}
	})

	t.Run("two EXECUTE TASK without names", func(t *testing.T) {
		sql := "EXECUTE TASK;\nEXECUTE TASK"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 2 {
			t.Fatalf("Expected exactly 2 warnings, got %d: %v", len(warnings), warnings)
		}
		for _, w := range warnings {
			if !strings.Contains(strings.ToLower(w.Message), "requires a task name") {
				t.Errorf("Unexpected warning message: %s", w.Message)
			}
		}
	})
}

// TestExecuteMarkerPositionMultiStatement verifies that warning markers point
// to the correct line numbers when EXECUTE statements appear after other
// statements in multi-statement SQL.
func TestExecuteMarkerPositionMultiStatement(t *testing.T) {
	t.Run("EXECUTE IMMEDIATE on line 3", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2;\nEXECUTE IMMEDIATE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].StartLineNumber != 3 {
			t.Errorf("Expected StartLineNumber=3, got %d", warnings[0].StartLineNumber)
		}
	})

	t.Run("multi-line EXECUTE IMMEDIATE starting at line 3", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2;\nEXECUTE\n  IMMEDIATE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].StartLineNumber != 3 {
			t.Errorf("Expected StartLineNumber=3, got %d", warnings[0].StartLineNumber)
		}
		if warnings[0].EndLineNumber != 4 {
			t.Errorf("Expected EndLineNumber=4, got %d", warnings[0].EndLineNumber)
		}
	})

	t.Run("EXECUTE TASK on line 4", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2;\nSELECT 3;\nEXECUTE TASK"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].StartLineNumber != 4 {
			t.Errorf("Expected StartLineNumber=4, got %d", warnings[0].StartLineNumber)
		}
	})
}

// TestValidateSnowflakePatterns_EmptyInput ensures that empty, whitespace-only,
// and semicolons-only input does not produce markers or panics.
func TestValidateSnowflakePatterns_EmptyInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n  \t  "},
		{"semicolons only", ";;;"},
		{"semicolon and whitespace", " ; ; "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			if len(markers) > 0 {
				t.Errorf("Expected no markers for %q, got %d: %v", tc.sql, len(markers), markers)
			}
		})
	}
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

// TestExecuteMarkerEndLine verifies that EndLineNumber is correct for
// multi-line EXECUTE statements and that single-line statements have
// matching start and end lines.
func TestExecuteMarkerEndLine(t *testing.T) {
	t.Run("multi-line EXECUTE IMMEDIATE spans two lines", func(t *testing.T) {
		sql := "EXECUTE\n  IMMEDIATE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].EndLineNumber != 2 {
			t.Errorf("Expected EndLineNumber=2, got %d", warnings[0].EndLineNumber)
		}
	})

	t.Run("three-line EXECUTE IMMEDIATE has EndLineNumber=3", func(t *testing.T) {
		sql := "EXECUTE\n  IMMEDIATE\n  -- no arg"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].EndLineNumber != 3 {
			t.Errorf("Expected EndLineNumber=3, got %d", warnings[0].EndLineNumber)
		}
	})

	t.Run("single-line EXECUTE TASK has matching start and end line", func(t *testing.T) {
		sql := "EXECUTE TASK"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		if len(warnings) != 1 {
			t.Fatalf("Expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].StartLineNumber != warnings[0].EndLineNumber {
			t.Errorf("Expected StartLineNumber=EndLineNumber, got start=%d end=%d",
				warnings[0].StartLineNumber, warnings[0].EndLineNumber)
		}
	})
}

// TestExecuteMultiStatementThreeInvalid verifies that three invalid EXECUTE
// statements each independently produce a warning.
func TestExecuteMultiStatementThreeInvalid(t *testing.T) {
	sql := "EXECUTE IMMEDIATE;\nEXECUTE TASK;\nEXECUTE IMMEDIATE"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warnings := getWarnings(markers)

	if len(warnings) < 3 {
		t.Fatalf("Expected at least 3 warnings, got %d: %v", len(warnings), warnings)
	}

	immCount, taskCount := 0, 0
	for _, w := range warnings {
		msg := strings.ToLower(w.Message)
		if strings.Contains(msg, "requires a sql string argument") {
			immCount++
		}
		if strings.Contains(msg, "requires a task name") {
			taskCount++
		}
	}
	if immCount != 2 {
		t.Errorf("Expected 2 EXECUTE IMMEDIATE warnings, got %d", immCount)
	}
	if taskCount != 1 {
		t.Errorf("Expected 1 EXECUTE TASK warning, got %d", taskCount)
	}
}

// TestExecuteWarningCountSingleStatement verifies that each single-statement
// EXECUTE violation produces exactly 1 warning — no duplicates.
func TestExecuteWarningCountSingleStatement(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"bare EXECUTE IMMEDIATE", "EXECUTE IMMEDIATE"},
		{"EXECUTE IMMEDIATE semicolon only", "EXECUTE IMMEDIATE;"},
		{"EXECUTE IMMEDIATE trailing whitespace", "EXECUTE IMMEDIATE   "},
		{"bare EXECUTE TASK", "EXECUTE TASK"},
		{"EXECUTE TASK semicolon only", "EXECUTE TASK;"},
		{"EXECUTE TASK trailing whitespace", "EXECUTE TASK   "},
		{"EXECUTE IMMEDIATE with empty USING", "EXECUTE IMMEDIATE 'SELECT $1' USING ()"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) != 1 {
				t.Errorf("Expected exactly 1 warning for %q, got %d: %v", tc.sql, len(warnings), warnMsgs(warnings))
			}
		})
	}
}

// TestExecuteMarkerColumns verifies that EXECUTE warning markers use the
// expected column positions (StartColumn=1, EndColumn=100 per diagMarkerSpan).
func TestExecuteMarkerColumns(t *testing.T) {
	sql := "EXECUTE IMMEDIATE"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warnings := getWarnings(markers)

	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].StartColumn != 1 {
		t.Errorf("Expected StartColumn=1, got %d", warnings[0].StartColumn)
	}
	if warnings[0].EndColumn != 100 {
		t.Errorf("Expected EndColumn=100, got %d", warnings[0].EndColumn)
	}
}

// TestExecuteNoFalsePositiveInNonExecuteStatements verifies that EXECUTE
// keywords appearing inside string literals or comments in non-EXECUTE
// statements do not produce EXECUTE-related diagnostics.
func TestExecuteNoFalsePositiveInNonExecuteStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"SELECT with EXECUTE IMMEDIATE in string literal", "SELECT 'EXECUTE IMMEDIATE' AS col"},
		{"Comment containing EXECUTE TASK text", "SELECT 1 -- EXECUTE TASK"},
		{"Block comment containing EXECUTE IMMEDIATE", "SELECT 1 /* EXECUTE IMMEDIATE */"},
		{"Dollar-quoted block containing EXECUTE IMMEDIATE", "SELECT $$EXECUTE IMMEDIATE$$ AS col"},
		{"Dollar-quoted block containing EXECUTE TASK", "SELECT $$EXECUTE TASK$$ AS col"},
		{"Tagged dollar-quoted block containing EXECUTE IMMEDIATE", "SELECT $tag$EXECUTE IMMEDIATE$tag$ AS col"},
		{"CREATE PROCEDURE body containing EXECUTE IMMEDIATE", "CREATE OR REPLACE PROCEDURE sp() RETURNS VARCHAR AS $$ EXECUTE IMMEDIATE 'SELECT 1' $$"},
		{"CREATE PROCEDURE body containing bare EXECUTE TASK", "CREATE OR REPLACE PROCEDURE sp() RETURNS VARCHAR AS $$ EXECUTE TASK $$ "},
		{"Block comment between EXECUTE and IMMEDIATE bypasses guard", "EXECUTE /* comment */ IMMEDIATE 'SELECT 1'"},
		{"Line comment between EXECUTE and IMMEDIATE bypasses guard", "EXECUTE -- comment\nIMMEDIATE 'SELECT 1'"},
		{"Block comment between EXECUTE and TASK bypasses guard", "EXECUTE /* comment */ TASK my_task"},
		{"Line comment between EXECUTE and TASK bypasses guard", "EXECUTE -- comment\nTASK my_task"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			for _, m := range markers {
				msg := strings.ToLower(m.Message)
				if strings.Contains(msg, "execute immediate") || strings.Contains(msg, "execute task") {
					t.Errorf("Unexpected EXECUTE-related marker for %q: %s", tc.sql, m.Message)
				}
			}
		})
	}
}
