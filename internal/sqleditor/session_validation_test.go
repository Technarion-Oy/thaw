package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_AlterSession(t *testing.T) {
	validCases := []string{
		// Basic SET with string parameter
		"ALTER SESSION SET QUERY_TAG = 'my_tag'",
		// SET with multiple parameters
		"ALTER SESSION SET QUERY_TAG = 'tag' TIMEZONE = 'UTC'",
		// SET with boolean parameter
		"ALTER SESSION SET AUTOCOMMIT = TRUE",
		"ALTER SESSION SET AUTOCOMMIT = FALSE",
		"ALTER SESSION SET USE_CACHED_RESULT = TRUE",
		"ALTER SESSION SET QUOTED_IDENTIFIERS_IGNORE_CASE = FALSE",
		"ALTER SESSION SET STRICT_JSON_OUTPUT = TRUE",
		// SET with integer range parameters
		"ALTER SESSION SET WEEK_START = 0",
		"ALTER SESSION SET WEEK_START = 7",
		"ALTER SESSION SET WEEK_OF_YEAR_POLICY = 0",
		"ALTER SESSION SET WEEK_OF_YEAR_POLICY = 1",
		"ALTER SESSION SET DATE_FIRST_DAY_OF_WEEK = 0",
		"ALTER SESSION SET DATE_FIRST_DAY_OF_WEEK = 6",
		"ALTER SESSION SET JSON_INDENT = 0",
		"ALTER SESSION SET JSON_INDENT = 16",
		// SET with non-negative integer parameters
		"ALTER SESSION SET ROWS_PER_RESULTSET = 0",
		"ALTER SESSION SET ROWS_PER_RESULTSET = 10000",
		"ALTER SESSION SET MULTI_STATEMENT_COUNT = 0",
		"ALTER SESSION SET MULTI_STATEMENT_COUNT = 5",
		// SET with enum parameters
		"ALTER SESSION SET BINARY_OUTPUT_FORMAT = 'HEX'",
		"ALTER SESSION SET BINARY_OUTPUT_FORMAT = 'BASE64'",
		"ALTER SESSION SET BINARY_OUTPUT_FORMAT = 'UTF8'",
		"ALTER SESSION SET TRANSACTION_DEFAULT_ISOLATION_LEVEL = 'READ COMMITTED'",
		// SET with format string parameters
		"ALTER SESSION SET TIMESTAMP_OUTPUT_FORMAT = 'YYYY-MM-DD HH24:MI:SS.FF3'",
		"ALTER SESSION SET DATE_OUTPUT_FORMAT = 'YYYY-MM-DD'",
		"ALTER SESSION SET TIME_OUTPUT_FORMAT = 'HH24:MI:SS'",
		"ALTER SESSION SET TIMESTAMP_INPUT_FORMAT = 'AUTO'",
		"ALTER SESSION SET TIMESTAMP_NTZ_OUTPUT_FORMAT = 'YYYY-MM-DD HH24:MI:SS'",
		"ALTER SESSION SET TIMESTAMP_TZ_OUTPUT_FORMAT = 'YYYY-MM-DD HH24:MI:SS TZH:TZM'",
		"ALTER SESSION SET TIMESTAMP_LTZ_OUTPUT_FORMAT = 'YYYY-MM-DD HH24:MI:SS'",
		// SET with other string parameters
		"ALTER SESSION SET PYTHON_PROFILER_MODULES = 'all'",
		"ALTER SESSION SET PYTHON_PROFILER_TARGET_STAGE = '@my_stage'",
		"ALTER SESSION SET SIMULATED_DATA_SHARING_CONSUMER = 'my_account'",
		// Additional commonly-used parameters
		"ALTER SESSION SET STATEMENT_TIMEOUT_IN_SECONDS = 300",
		"ALTER SESSION SET LOCK_TIMEOUT = 60",
		"ALTER SESSION SET GEOGRAPHY_OUTPUT_FORMAT = 'GEOJSON'",
		"ALTER SESSION SET GEOMETRY_OUTPUT_FORMAT = 'WKT'",
		"ALTER SESSION SET CLIENT_SESSION_KEEP_ALIVE = TRUE",
		"ALTER SESSION SET ABORT_DETACHED_QUERY = FALSE",
		"ALTER SESSION SET ERROR_ON_NONDETERMINISTIC_MERGE = TRUE",
		"ALTER SESSION SET ERROR_ON_NONDETERMINISTIC_UPDATE = TRUE",
		"ALTER SESSION SET CLIENT_RESULT_CHUNK_SIZE = 160",
		"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 1970",
		"ALTER SESSION SET TIMESTAMP_TYPE_MAPPING = 'TIMESTAMP_NTZ'",
		"ALTER SESSION SET NETWORK_POLICY = 'my_policy'",
		"ALTER SESSION SET PERIODIC_DATA_REKEYING = TRUE",
		"ALTER SESSION SET CLIENT_MEMORY_LIMIT = 1536",
		"ALTER SESSION SET CLIENT_PREFETCH_THREADS = 4",
		// Unquoted enum value (matched by [^\s;]+ regex path)
		"ALTER SESSION SET BINARY_OUTPUT_FORMAT = HEX",
		// Case-insensitive enum value
		"ALTER SESSION SET GEOGRAPHY_OUTPUT_FORMAT = 'geojson'",
		// SQL-escaped quotes in string value
		"ALTER SESSION SET QUERY_TAG = 'it''s a tag'",
		// Empty string value
		"ALTER SESSION SET QUERY_TAG = ''",
		// Exact boundary values for TWO_DIGIT_CENTURY_START
		"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 1900",
		"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 2100",
		// Multi-line statement (newline between keywords)
		"ALTER SESSION\nSET QUERY_TAG = 'test'",
		// UNSET with single parameter
		"ALTER SESSION UNSET QUERY_TAG",
		// UNSET with multiple comma-separated parameters
		"ALTER SESSION UNSET QUERY_TAG, TIMEZONE",
		// UNSET with multiple whitespace-separated parameters
		"ALTER SESSION UNSET QUERY_TAG TIMEZONE",
		// Case insensitivity
		"alter session set query_tag = 'test'",
		"ALTER session SET AUTOCOMMIT = true",
		// Comments in statement
		"ALTER SESSION SET /* comment */ QUERY_TAG = 'test'",
		"ALTER SESSION SET QUERY_TAG = 'test' -- trailing comment",
		"ALTER SESSION UNSET /* comment */ QUERY_TAG",
		"ALTER SESSION UNSET QUERY_TAG -- trailing comment",
		// Tab characters between keywords
		"ALTER\tSESSION\tSET\tQUERY_TAG\t=\t'test'",
		// Leading whitespace
		"   ALTER SESSION SET QUERY_TAG = 'test'",
		// String value containing semicolons
		"ALTER SESSION SET QUERY_TAG = 'tag;with;semicolons'",
		// String value containing equals sign
		"ALTER SESSION SET QUERY_TAG = 'key=value'",
		// UNSET with lowercase known param names
		"ALTER SESSION UNSET query_tag",
		"ALTER SESSION UNSET query_tag, timezone",
		// UNSET with spaces around commas
		"ALTER SESSION UNSET QUERY_TAG , TIMEZONE",
		// Trailing whitespace after value
		"ALTER SESSION SET QUERY_TAG = 'test'   ",
		// SET with integer at exact boundary (min)
		"ALTER SESSION SET WEEK_START = 0",
		// Integer with leading zeros (valid value)
		"ALTER SESSION SET WEEK_START = 03",
		// UNSET with mixed case params
		"ALTER SESSION UNSET Query_Tag",
		// Quoted boolean value (exercises unquote path for spBool)
		"ALTER SESSION SET AUTOCOMMIT = 'TRUE'",
		"ALTER SESSION SET AUTOCOMMIT = 'FALSE'",
		// Quoted integer range value (exercises unquote path for spIntRange)
		"ALTER SESSION SET WEEK_START = '3'",
		// Quoted non-negative integer value (exercises unquote path for spNonNeg)
		"ALTER SESSION SET ROWS_PER_RESULTSET = '100'",
		// UNSET with trailing comma (empty token after comma should be skipped)
		"ALTER SESSION UNSET QUERY_TAG,",
		// Unquoted string param value (exercises [^\s;]+ regex path for spString)
		"ALTER SESSION SET QUERY_TAG = my_tag",
		// Multiple params across separate lines
		"ALTER SESSION SET\nAUTOCOMMIT = TRUE\nQUERY_TAG = 'test'",
		// No whitespace around equals sign (compact syntax)
		"ALTER SESSION SET AUTOCOMMIT=TRUE",
		"ALTER SESSION SET QUERY_TAG='test'",
		// Extra whitespace around equals sign
		"ALTER SESSION SET QUERY_TAG   =   'test'",
		// All param type validators in one statement (spBool, spIntRange, spEnum, spString, spNonNeg)
		"ALTER SESSION SET AUTOCOMMIT = TRUE WEEK_START = 3 BINARY_OUTPUT_FORMAT = 'HEX' QUERY_TAG = 'test' ROWS_PER_RESULTSET = 100",
		// Trailing semicolon (common real-world pattern, stripped by parseText pipeline)
		"ALTER SESSION SET AUTOCOMMIT = TRUE;",
		// Duplicate parameter names in SET (each validated independently, no duplicate error)
		"ALTER SESSION SET AUTOCOMMIT = TRUE AUTOCOMMIT = FALSE",
		// String value containing backslashes
		"ALTER SESSION SET QUERY_TAG = 'path\\to\\file'",
		// String value containing line-comment syntax (stripCommentsSQL interaction)
		"ALTER SESSION SET QUERY_TAG = 'tag -- not a comment'",
		// String value containing block-comment syntax (stripCommentsSQL interaction)
		"ALTER SESSION SET QUERY_TAG = 'tag /* not */ end'",
		// Block comment splitting ALTER and SESSION keywords
		"ALTER /* comment */ SESSION SET QUERY_TAG = 'test'",
		// Multi-line SET with trailing comments between param assignments
		"ALTER SESSION SET\nAUTOCOMMIT = TRUE -- first param\nQUERY_TAG = 'test'",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"bare ALTER SESSION without SET or UNSET",
			"ALTER SESSION",
			[]string{"ALTER SESSION requires SET or UNSET"},
		},
		{
			"ALTER SESSION SET without parameters",
			"ALTER SESSION SET",
			[]string{"ALTER SESSION SET requires at least one parameter assignment"},
		},
		{
			"ALTER SESSION UNSET without parameters",
			"ALTER SESSION UNSET",
			[]string{"ALTER SESSION UNSET requires at least one parameter name"},
		},
		{
			"unknown parameter in SET",
			"ALTER SESSION SET UNKNOWN_PARAM = 'value'",
			[]string{"Unknown session parameter 'UNKNOWN_PARAM'"},
		},
		{
			"unknown parameter in UNSET",
			"ALTER SESSION UNSET UNKNOWN_PARAM",
			[]string{"Unknown session parameter 'UNKNOWN_PARAM'"},
		},
		{
			"invalid boolean value",
			"ALTER SESSION SET AUTOCOMMIT = MAYBE",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"WEEK_START out of range high",
			"ALTER SESSION SET WEEK_START = 8",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"WEEK_START out of range negative",
			"ALTER SESSION SET WEEK_START = -1",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"WEEK_OF_YEAR_POLICY out of range",
			"ALTER SESSION SET WEEK_OF_YEAR_POLICY = 2",
			[]string{"WEEK_OF_YEAR_POLICY must be an integer between 0 and 1"},
		},
		{
			"DATE_FIRST_DAY_OF_WEEK out of range",
			"ALTER SESSION SET DATE_FIRST_DAY_OF_WEEK = 7",
			[]string{"DATE_FIRST_DAY_OF_WEEK must be an integer between 0 and 6"},
		},
		{
			"JSON_INDENT out of range",
			"ALTER SESSION SET JSON_INDENT = 17",
			[]string{"JSON_INDENT must be an integer between 0 and 16"},
		},
		{
			"JSON_INDENT not an integer",
			"ALTER SESSION SET JSON_INDENT = abc",
			[]string{"JSON_INDENT must be an integer between 0 and 16"},
		},
		{
			"ROWS_PER_RESULTSET negative",
			"ALTER SESSION SET ROWS_PER_RESULTSET = -1",
			[]string{"ROWS_PER_RESULTSET must be a non-negative integer"},
		},
		{
			"MULTI_STATEMENT_COUNT not an integer",
			"ALTER SESSION SET MULTI_STATEMENT_COUNT = abc",
			[]string{"MULTI_STATEMENT_COUNT must be a non-negative integer"},
		},
		{
			"invalid BINARY_OUTPUT_FORMAT",
			"ALTER SESSION SET BINARY_OUTPUT_FORMAT = 'INVALID'",
			[]string{"BINARY_OUTPUT_FORMAT must be one of: HEX, BASE64, UTF8"},
		},
		{
			"invalid TRANSACTION_DEFAULT_ISOLATION_LEVEL",
			"ALTER SESSION SET TRANSACTION_DEFAULT_ISOLATION_LEVEL = 'SERIALIZABLE'",
			[]string{"TRANSACTION_DEFAULT_ISOLATION_LEVEL must be one of: READ COMMITTED"},
		},
		{
			"multiple errors in one statement",
			"ALTER SESSION SET WEEK_START = 99 AUTOCOMMIT = MAYBE",
			[]string{
				"WEEK_START must be an integer between 0 and 7",
				"AUTOCOMMIT must be TRUE or FALSE",
			},
		},
		{
			"mixed known and unknown in UNSET",
			"ALTER SESSION UNSET QUERY_TAG, FAKE_PARAM",
			[]string{"Unknown session parameter 'FAKE_PARAM'"},
		},
		{
			"stray token without = value",
			"ALTER SESSION SET QUERY_TAG = 'test' TIMEZONE",
			[]string{"missing '= <value>' assignment"},
		},
		{
			"TWO_DIGIT_CENTURY_START out of range",
			"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 1800",
			[]string{"TWO_DIGIT_CENTURY_START must be an integer between 1900 and 2100"},
		},
		{
			"invalid GEOGRAPHY_OUTPUT_FORMAT",
			"ALTER SESSION SET GEOGRAPHY_OUTPUT_FORMAT = 'XML'",
			[]string{"GEOGRAPHY_OUTPUT_FORMAT must be one of: GEOJSON, WKT, WKB, EWKT, EWKB"},
		},
		{
			"invalid TIMESTAMP_TYPE_MAPPING",
			"ALTER SESSION SET TIMESTAMP_TYPE_MAPPING = 'TIMESTAMP_XYZ'",
			[]string{"TIMESTAMP_TYPE_MAPPING must be one of: TIMESTAMP_NTZ, TIMESTAMP_LTZ, TIMESTAMP_TZ"},
		},
		{
			"invalid GEOMETRY_OUTPUT_FORMAT",
			"ALTER SESSION SET GEOMETRY_OUTPUT_FORMAT = 'XML'",
			[]string{"GEOMETRY_OUTPUT_FORMAT must be one of: GEOJSON, WKT, WKB, EWKT, EWKB"},
		},
		{
			"multiple unknown params in SET",
			"ALTER SESSION SET FAKE_A = 'x' FAKE_B = 'y'",
			[]string{
				"Unknown session parameter 'FAKE_A'",
				"Unknown session parameter 'FAKE_B'",
			},
		},
		{
			"multiple unknown params in UNSET",
			"ALTER SESSION UNSET FAKE_ONE, FAKE_TWO",
			[]string{
				"Unknown session parameter 'FAKE_ONE'",
				"Unknown session parameter 'FAKE_TWO'",
			},
		},
		{
			"TWO_DIGIT_CENTURY_START just below min boundary",
			"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 1899",
			[]string{"TWO_DIGIT_CENTURY_START must be an integer between 1900 and 2100"},
		},
		{
			"TWO_DIGIT_CENTURY_START just above max boundary",
			"ALTER SESSION SET TWO_DIGIT_CENTURY_START = 2101",
			[]string{"TWO_DIGIT_CENTURY_START must be an integer between 1900 and 2100"},
		},
		{
			"float value for non-negative integer param",
			"ALTER SESSION SET ROWS_PER_RESULTSET = 3.5",
			[]string{"ROWS_PER_RESULTSET must be a non-negative integer"},
		},
		{
			"float value for integer range param",
			"ALTER SESSION SET WEEK_START = 3.5",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"unquoted invalid enum value",
			"ALTER SESSION SET BINARY_OUTPUT_FORMAT = INVALID",
			[]string{"BINARY_OUTPUT_FORMAT must be one of: HEX, BASE64, UTF8"},
		},
		{
			"ALTER SESSION followed by invalid keyword",
			"ALTER SESSION RESET",
			[]string{"ALTER SESSION requires SET or UNSET"},
		},
		{
			"known param without = value (no other valid pairs)",
			"ALTER SESSION SET QUERY_TAG",
			[]string{"ALTER SESSION SET requires at least one parameter assignment"},
		},
		{
			"lowercase invalid boolean value",
			"ALTER SESSION SET AUTOCOMMIT = maybe",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"integer value with leading zeros out of range",
			"ALTER SESSION SET WEEK_START = 09",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"negative value for non-negative integer param",
			"ALTER SESSION SET STATEMENT_TIMEOUT_IN_SECONDS = -10",
			[]string{"STATEMENT_TIMEOUT_IN_SECONDS must be a non-negative integer"},
		},
		{
			"empty value after equals (no matching value)",
			"ALTER SESSION SET AUTOCOMMIT = TRUE TIMEZONE",
			[]string{"missing '= <value>' assignment"},
		},
		{
			"quoted invalid boolean value",
			"ALTER SESSION SET AUTOCOMMIT = 'MAYBE'",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"quoted out-of-range integer value",
			"ALTER SESSION SET WEEK_START = '99'",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"quoted negative non-negative integer",
			"ALTER SESSION SET ROWS_PER_RESULTSET = '-5'",
			[]string{"ROWS_PER_RESULTSET must be a non-negative integer"},
		},
		{
			"enum with leading/trailing spaces in quotes",
			"ALTER SESSION SET BINARY_OUTPUT_FORMAT = ' HEX '",
			[]string{"BINARY_OUTPUT_FORMAT must be one of: HEX, BASE64, UTF8"},
		},
		{
			"SET body is entirely a line comment",
			"ALTER SESSION SET -- everything",
			[]string{"ALTER SESSION SET requires at least one parameter assignment"},
		},
		{
			"missing equals sign between param and value",
			"ALTER SESSION SET QUERY_TAG 'test'",
			[]string{"ALTER SESSION SET requires at least one parameter assignment"},
		},
		{
			"multiple stray tokens without value assignment",
			"ALTER SESSION SET AUTOCOMMIT = TRUE TIMEZONE QUERY_TAG",
			[]string{
				"missing '= <value>' assignment",
				"missing '= <value>' assignment",
			},
		},
		{
			"integer overflow for spNonNeg param",
			"ALTER SESSION SET ROWS_PER_RESULTSET = 99999999999999999999",
			[]string{"ROWS_PER_RESULTSET must be a non-negative integer"},
		},
		{
			"integer overflow for spIntRange param",
			"ALTER SESSION SET WEEK_START = 99999999999999999999",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"mixed valid and unknown params in SET",
			"ALTER SESSION SET AUTOCOMMIT = TRUE FAKE_PARAM = 'x'",
			[]string{"Unknown session parameter 'FAKE_PARAM'"},
		},
		{
			"known param bad value plus unknown param",
			"ALTER SESSION SET AUTOCOMMIT = MAYBE FAKE_PARAM = 'x'",
			[]string{
				"AUTOCOMMIT must be TRUE or FALSE",
				"Unknown session parameter 'FAKE_PARAM'",
			},
		},
		{
			"numeric 1 for boolean param",
			"ALTER SESSION SET AUTOCOMMIT = 1",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"numeric 0 for boolean param",
			"ALTER SESSION SET AUTOCOMMIT = 0",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"empty quoted value for boolean param",
			"ALTER SESSION SET AUTOCOMMIT = ''",
			[]string{"AUTOCOMMIT must be TRUE or FALSE"},
		},
		{
			"empty quoted value for integer range param",
			"ALTER SESSION SET WEEK_START = ''",
			[]string{"WEEK_START must be an integer between 0 and 7"},
		},
		{
			"empty quoted value for non-negative integer param",
			"ALTER SESSION SET ROWS_PER_RESULTSET = ''",
			[]string{"ROWS_PER_RESULTSET must be a non-negative integer"},
		},
		{
			"unquoted multi-word enum value (only first word captured)",
			"ALTER SESSION SET TRANSACTION_DEFAULT_ISOLATION_LEVEL = READ COMMITTED",
			[]string{
				"TRANSACTION_DEFAULT_ISOLATION_LEVEL must be one of: READ COMMITTED",
				"missing '= <value>' assignment",
			},
		},
		{
			"SET body is entirely a block comment",
			"ALTER SESSION SET /* nothing here */",
			[]string{"ALTER SESSION SET requires at least one parameter assignment"},
		},
		{
			"UNSET body is entirely a block comment",
			"ALTER SESSION UNSET /* nothing here */",
			[]string{"ALTER SESSION UNSET requires at least one parameter name"},
		},
		{
			"UNSET body is entirely a line comment",
			"ALTER SESSION UNSET -- everything",
			[]string{"ALTER SESSION UNSET requires at least one parameter name"},
		},
		{
			"UNSET with assignment syntax rejected",
			"ALTER SESSION UNSET QUERY_TAG = 'test'",
			[]string{
				"Unknown session parameter '='",
				"Unknown session parameter ''TEST''",
			},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) != len(tt.wantMsgs) {
				t.Errorf("Expected %d warning(s) for %q, got %d: %v", len(tt.wantMsgs), tt.sql, len(warns), warns)
			}

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q containing %q, got: %v", tt.sql, wantMsg, warns)
				}
			}
		})
	}

	// Multi-statement test: ALTER SESSION embedded between other statements.
	t.Run("multi-statement with ALTER SESSION", func(t *testing.T) {
		sql := "SELECT 1;\nALTER SESSION SET QUERY_TAG = 'test';\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings for multi-statement SQL, got %d: %v", len(warns), warns)
		}
	})

	t.Run("multi-statement with invalid ALTER SESSION", func(t *testing.T) {
		sql := "SELECT 1;\nALTER SESSION SET AUTOCOMMIT = MAYBE;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning, got %d: %v", len(warns), warns)
		}
		if len(warns) > 0 && !strings.Contains(warns[0].Message, "AUTOCOMMIT must be TRUE or FALSE") {
			t.Errorf("Expected AUTOCOMMIT warning, got: %v", warns[0].Message)
		}
	})
}

// ── SHOW commands ─────────────────────────────────────────────────────────────
