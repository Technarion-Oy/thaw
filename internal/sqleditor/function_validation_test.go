package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_CreateFunction(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "Valid Javascript UDF",
			sql:           "CREATE OR REPLACE FUNCTION my_func(a NUMBER) RETURNS NUMBER LANGUAGE JAVASCRIPT AS $$ return a + 1; $$",
			expectWarning: false,
		},
		{
			name:          "Valid SQL UDF",
			sql:           "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE SQL AS $$ SELECT 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python UDF",
			sql:           "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' HANDLER = 'my_handler' PACKAGES = ('snowflake-snowpark-python') AS $$ def my_handler(): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid Java UDF",
			sql:           "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE JAVA HANDLER = 'TestClass.myMethod' AS $$ class TestClass { public static String myMethod() { return \"hello\"; } } $$",
			expectWarning: false,
		},
		{
			name:          "Valid UDTF (Table Function)",
			sql:           "CREATE FUNCTION my_udtf() RETURNS TABLE(x NUMBER) LANGUAGE SQL AS $$ SELECT 1 $$",
			expectWarning: false,
		},
		{
			name:          "Valid UDAF (Aggregate Function)",
			sql:           "CREATE AGGREGATE FUNCTION my_udaf() RETURNS NUMBER LANGUAGE JAVASCRIPT AS $$ /* ... */ $$",
			expectWarning: false,
		},
		{
			name:          "Valid SECURE UDF",
			sql:           "CREATE SECURE FUNCTION my_secure_func() RETURNS NUMBER LANGUAGE SQL AS $$ SELECT 1 $$",
			expectWarning: false,
		},
		{
			name:          "Valid MEMOIZABLE UDF",
			sql:           "CREATE FUNCTION my_mem_func() RETURNS NUMBER LANGUAGE SQL MEMOIZABLE AS $$ SELECT 1 $$",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Missing RETURNS",
			sql:           "CREATE FUNCTION my_func() LANGUAGE SQL AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		{
			name:          "Missing LANGUAGE",
			sql:           "CREATE FUNCTION my_func() RETURNS NUMBER AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		{
			name:          "Invalid LANGUAGE",
			sql:           "CREATE FUNCTION my_func() RETURNS NUMBER LANGUAGE RUBY AS $$ puts 'hello' $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		{
			name:          "Missing AS body",
			sql:           "CREATE FUNCTION my_func() RETURNS NUMBER LANGUAGE SQL",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		{
			name:          "Python missing HANDLER",
			sql:           "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') AS $$ def my_handler(): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "HANDLER is required for PYTHON",
		},
		{
			name:          "Java missing HANDLER",
			sql:           "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE JAVA AS $$ class TestClass {} $$",
			expectWarning: true,
			expectedMatch: "HANDLER is required for JAVA",
		},
		{
			name:          "Conflict CALLED ON NULL and RETURNS NULL ON NULL",
			sql:           "CREATE FUNCTION my_func(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT RETURNS NULL ON NULL INPUT AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Duplicate STRICT and RETURNS NULL ON NULL",
			sql:           "CREATE FUNCTION my_func(a int) RETURNS int LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "redundant",
		},
		{
			name:          "AGGREGATE + RETURNS TABLE",
			sql:           "CREATE AGGREGATE FUNCTION my_udaf() RETURNS TABLE(x NUMBER) LANGUAGE SQL AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "AGGREGATE functions cannot return a TABLE",
		},
		{
			name:          "MEMOIZABLE on AGGREGATE",
			sql:           "CREATE AGGREGATE FUNCTION my_func() RETURNS NUMBER LANGUAGE SQL MEMOIZABLE AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "MEMOIZABLE is only valid for scalar functions",
		},
		{
			name:          "MEMOIZABLE on UDTF",
			sql:           "CREATE FUNCTION my_func() RETURNS TABLE(x NUMBER) LANGUAGE SQL MEMOIZABLE AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "MEMOIZABLE is only valid for scalar functions",
		},
		{
			name:          "SECURE on AGGREGATE",
			sql:           "CREATE SECURE AGGREGATE FUNCTION my_udaf() RETURNS NUMBER LANGUAGE SQL AS $$ SELECT 1 $$",
			expectWarning: true,
			expectedMatch: "SECURE is not supported for AGGREGATE functions",
		},
		{
			name:          "Invalid parameter type",
			sql:           "CREATE FUNCTION my_func(param1 UNKNOWNTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ SELECT 'hello' $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			markers = append(markers, ValidateDataTypes(tt.sql, ranges)...)

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
