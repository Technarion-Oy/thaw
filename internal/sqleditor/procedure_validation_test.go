package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_CreateProcedure(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "Valid Javascript procedure",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc(param1 VARCHAR) RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ return 'hello'; $$",
			expectWarning: false,
		},
		{
			name:          "Valid SQL procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python procedure with IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' IMPORTS = ('@stage/file.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid with EXECUTE AS OWNER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS OWNER AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with STRICT",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS NUMBER LANGUAGE SQL STRICT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with CALLED ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS NUMBER LANGUAGE SQL CALLED ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid table-valued procedure",
			sql:           "CREATE PROCEDURE get_data() RETURNS TABLE(name VARCHAR, age INT) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT name, age FROM t); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid procedure with EXECUTE AS in body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ var s = 'EXECUTE AS INVOKER'; $$",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Missing RETURNS",
			sql:           "CREATE PROCEDURE my_proc() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		{
			name:          "Missing LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		{
			name:          "Invalid LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE RUBY AS $$ puts 'hello' $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		{
			name:          "Missing AS body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		{
			name:          "Conflict CALLED ON NULL and RETURNS NULL ON NULL",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT RETURNS NULL ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Conflict CALLED ON NULL and STRICT",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Duplicate STRICT and RETURNS NULL ON NULL",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "redundant",
		},
		{
			name:          "Invalid EXECUTE AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS USER AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		{
			name:          "Python missing RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION is required",
		},
		{
			name:          "Python missing PACKAGES and IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS is required",
		},
		{
			name:          "Invalid parameter type",
			sql:           "CREATE PROCEDURE my_proc(param1 UNKNOWNTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'hello'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)

			// ValidateDataTypes is a separate validator; combine its output to test parameter and return-type checking.
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
