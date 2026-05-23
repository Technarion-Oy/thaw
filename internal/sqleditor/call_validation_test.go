package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Call(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "Basic call no args",
			sql:           "CALL my_proc()",
			expectWarning: false,
		},
		{
			name:          "Call with arguments",
			sql:           "CALL my_proc(1, 2, 'hello')",
			expectWarning: false,
		},
		{
			name:          "Call with schema prefix",
			sql:           "CALL my_schema.my_proc()",
			expectWarning: false,
		},
		{
			name:          "Call with full database prefix",
			sql:           "CALL my_db.my_schema.my_proc()",
			expectWarning: false,
		},
		{
			name:          "Call with quoted identifier",
			sql:           `CALL "MY PROC"()`,
			expectWarning: false,
		},
		{
			name:          "Call with quoted schema prefix",
			sql:           `CALL "MY SCHEMA"."MY PROC"()`,
			expectWarning: false,
		},
		{
			name:          "Call with INTO colon variable",
			sql:           "CALL my_proc() INTO :result_var",
			expectWarning: false,
		},
		{
			name:          "Call with INTO colon variable and trailing semicolon",
			sql:           "CALL my_proc() INTO :result_var;",
			expectWarning: false,
		},
		{
			name:          "Call with INTO colon variable and args",
			sql:           "CALL my_proc(1, 2) INTO :output",
			expectWarning: false,
		},
		{
			name:          "Call with integer argument",
			sql:           "CALL my_proc(42)",
			expectWarning: false,
		},
		{
			name:          "INTO in comment does not trigger false positive",
			sql:           "CALL my_proc() -- INTO result_var is not supported here",
			expectWarning: false,
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — valid",
			sql:           "WITH p AS PROCEDURE () RETURNS VARIANT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$ CALL p()",
			expectWarning: false,
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE with args — valid",
			sql:           "WITH p AS PROCEDURE (n INT) RETURNS INT LANGUAGE SQL AS $$ BEGIN RETURN n; END; $$ CALL p(42)",
			expectWarning: false,
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE with INTO colon — valid",
			sql:           "WITH p AS PROCEDURE () RETURNS INT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$ CALL p() INTO :output",
			expectWarning: false,
		},
		{
			name:          "Anonymous procedure WITH tagged dollar-quoting — valid",
			sql:           "WITH p AS PROCEDURE () RETURNS INT LANGUAGE SQL AS $proc$ BEGIN RETURN 1; END; $proc$ CALL p()",
			expectWarning: false,
		},
		{
			name:          "Lowercase call keyword",
			sql:           "call my_proc()",
			expectWarning: false,
		},
		{
			name:          "Mixed case Call keyword",
			sql:           "Call My_Proc()",
			expectWarning: false,
		},
		{
			name:          "CALL with trailing semicolon",
			sql:           "CALL my_proc();",
			expectWarning: false,
		},
		{
			name:          "INTO in block comment does not trigger false positive",
			sql:           "CALL my_proc() /* INTO result_var */",
			expectWarning: false,
		},
		{
			name:          "CALL with block comment before valid INTO",
			sql:           "CALL my_proc() /* comment */ INTO :result",
			expectWarning: false,
		},
		{
			name:          "CALL with nested function in arguments",
			sql:           "CALL my_proc(nested_func(1, 2), 'abc')",
			expectWarning: false,
		},
		{
			name:          "CALL with all parts quoted three-part name",
			sql:           `CALL "MY DB"."MY SCHEMA"."MY PROC"()`,
			expectWarning: false,
		},
		{
			name:          "CALL with dollar sign in identifier",
			sql:           "CALL my$proc()",
			expectWarning: false,
		},
		{
			name:          "CALL with leading whitespace",
			sql:           "  CALL my_proc()",
			expectWarning: false,
		},
		{
			name:          "CALL with full path and INTO colon variable",
			sql:           "CALL my_db.my_schema.my_proc() INTO :output",
			expectWarning: false,
		},
		{
			name:          "CALL with CURRENT_TIMESTAMP argument",
			sql:           "CALL my_proc(CURRENT_TIMESTAMP(), 'test')",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare CALL with no procedure name",
			sql:           "CALL",
			expectWarning: true,
			expectedMatch: "Missing procedure name",
		},
		{
			name:          "CALL with semicolon but no procedure name",
			sql:           "CALL ;",
			expectWarning: true,
			expectedMatch: "Missing procedure name",
		},
		{
			name:          "CALL with procedure name but no parens",
			sql:           "CALL my_proc",
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "CALL with arguments but no parens",
			sql:           "CALL my_proc 1, 2",
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "CALL with schema prefix but no parens",
			sql:           "CALL my_schema.my_proc",
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "INTO variable missing colon prefix",
			sql:           "CALL my_proc() INTO result_var",
			expectWarning: true,
			expectedMatch: "prefixed with ':'",
		},
		{
			name:          "INTO variable missing colon — semicolon-terminated",
			sql:           "CALL my_proc() INTO result_var;",
			expectWarning: true,
			expectedMatch: "INTO :result_var instead of INTO result_var",
		},
		{
			name:          "INTO variable missing colon — bare word",
			sql:           "CALL my_proc() INTO output",
			expectWarning: true,
			expectedMatch: "prefixed with ':'",
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — missing CALL",
			sql:           "WITH p AS PROCEDURE () RETURNS VARIANT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "must end with CALL",
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — CALL missing parens",
			sql:           "WITH p AS PROCEDURE () RETURNS VARIANT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$ CALL p",
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — INTO missing colon",
			sql:           "WITH p AS PROCEDURE () RETURNS INT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$ CALL p() INTO output",
			expectWarning: true,
			expectedMatch: "prefixed with ':'",
		},
		{
			name:          "CALL with only trailing whitespace — no procedure name",
			sql:           "CALL   ",
			expectWarning: true,
			expectedMatch: "Missing procedure name",
		},
		{
			name:          "CALL with quoted identifier but no parens",
			sql:           `CALL "MY PROC"`,
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "CALL with full three-part name but no parens",
			sql:           "CALL my_db.my_schema.my_proc",
			expectWarning: true,
			expectedMatch: "parenthesised argument list",
		},
		{
			name:          "INTO variable missing colon — quoted variable",
			sql:           `CALL my_proc() INTO "result_var"`,
			expectWarning: true,
			expectedMatch: "prefixed with ':'",
		},
		{
			name:          "Lowercase call with INTO missing colon",
			sql:           "call my_proc() INTO output",
			expectWarning: true,
			expectedMatch: "prefixed with ':'",
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — no dollar quotes and no CALL",
			sql:           "WITH p AS PROCEDURE () RETURNS INT LANGUAGE SQL",
			expectWarning: true,
			expectedMatch: "must end with CALL",
		},
		{
			name:          "Anonymous procedure WITH AS PROCEDURE — missing CALL after tagged body",
			sql:           "WITH p AS PROCEDURE () RETURNS INT LANGUAGE SQL AS $body$ BEGIN RETURN 1; END; $body$",
			expectWarning: true,
			expectedMatch: "must end with CALL",
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
