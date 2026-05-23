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


