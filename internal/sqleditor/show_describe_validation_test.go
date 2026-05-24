package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Show(t *testing.T) {
	validCases := []string{
		// Basic object types (single-word)
		"SHOW TABLES",
		"SHOW VIEWS",
		"SHOW SCHEMAS",
		"SHOW DATABASES",
		"SHOW WAREHOUSES",
		"SHOW ROLES",
		"SHOW USERS",
		"SHOW STAGES",
		"SHOW PIPES",
		"SHOW STREAMS",
		"SHOW TASKS",
		"SHOW FUNCTIONS",
		"SHOW PROCEDURES",
		"SHOW SEQUENCES",
		"SHOW COLUMNS",
		"SHOW INTEGRATIONS",
		"SHOW SHARES",
		"SHOW GRANTS",
		"SHOW PARAMETERS",
		"SHOW LOCKS",
		"SHOW TRANSACTIONS",
		"SHOW CONNECTIONS",
		"SHOW REGIONS",
		"SHOW ALERTS",
		"SHOW TAGS",
		"SHOW SECRETS",
		// Two-word object types
		"SHOW PRIMARY KEYS",
		"SHOW IMPORTED KEYS",
		"SHOW EXPORTED KEYS",
		"SHOW UNIQUE KEYS",
		"SHOW DYNAMIC TABLES",
		"SHOW EXTERNAL TABLES",
		"SHOW EVENT TABLES",
		"SHOW FILE FORMATS",
		"SHOW RESOURCE MONITORS",
		"SHOW MANAGED ACCOUNTS",
		"SHOW NETWORK POLICIES",
		"SHOW MASKING POLICIES",
		"SHOW SESSION POLICIES",
		"SHOW PASSWORD POLICIES",
		"SHOW AGGREGATION POLICIES",
		"SHOW PROJECTION POLICIES",
		"SHOW NETWORK RULES",
		"SHOW PACKAGES POLICIES",
		"SHOW REPLICATION DATABASES",
		"SHOW REPLICATION GROUPS",
		"SHOW FAILOVER GROUPS",
		// Three-word object types
		"SHOW ROW ACCESS POLICIES",
		"SHOW ORGANIZATION ACCOUNTS",
		"SHOW DELEGATED AUTHORIZATIONS",
		// Additional two-word types
		"SHOW HYBRID TABLES",
		"SHOW ICEBERG TABLES",
		"SHOW EXTERNAL FUNCTIONS",
		"SHOW GIT REPOSITORIES",
		"SHOW GIT BRANCHES",
		"SHOW IMAGE REPOSITORIES",
		"SHOW COMPUTE POOLS",
		"SHOW AUTHENTICATION POLICIES",
		// Additional two-word types
		"SHOW MATERIALIZED VIEWS",
		"SHOW CATALOG INTEGRATIONS",
		"SHOW EXTERNAL VOLUMES",
		// Three-word types
		"SHOW CORTEX SEARCH SERVICES",
		"SHOW DATA METRIC FUNCTIONS",
		// Additional single-word types
		"SHOW CHANNELS",
		"SHOW LISTINGS",
		"SHOW MODELS",
		"SHOW OBJECTS",
		"SHOW SNAPSHOTS",
		"SHOW STREAMLITS",
		"SHOW VARIABLES",
		"SHOW SERVICES",
		"SHOW ENDPOINTS",
		"SHOW NOTEBOOKS",
		// FUTURE GRANTS
		"SHOW FUTURE GRANTS IN DATABASE my_db",
		// TERSE modifier (valid types)
		"SHOW TERSE TABLES",
		"SHOW TERSE VIEWS",
		"SHOW TERSE SCHEMAS",
		"SHOW TERSE DATABASES",
		"SHOW TERSE STAGES",
		"SHOW TERSE EXTERNAL TABLES",
		"SHOW TERSE STREAMS",
		"SHOW TERSE USERS",
		// HISTORY modifier (valid for PIPES and REPLICATION DATABASES)
		"SHOW PIPES HISTORY",
		"SHOW REPLICATION DATABASES HISTORY",
		// LIKE clause
		"SHOW TABLES LIKE '%test%'",
		"SHOW TABLES LIKE 'my_table'",
		"SHOW TABLES LIKE 'it''s a test'",
		"SHOW TABLES LIKE ''",
		// IN clause (explicit scope)
		"SHOW TABLES IN ACCOUNT",
		"SHOW TABLES IN DATABASE",
		"SHOW TABLES IN DATABASE my_db",
		"SHOW TABLES IN SCHEMA my_db.my_schema",
		"SHOW TABLES IN TABLE my_db.my_schema.my_table",
		"SHOW VIEWS IN DATABASE",
		"SHOW SCHEMAS IN DATABASE my_db",
		// IN clause (implicit scope — Snowflake allows omitting the scope keyword)
		"SHOW TABLES IN my_schema",
		"SHOW TABLES IN my_db.my_schema",
		"SHOW COLUMNS IN my_db.my_schema.my_table",
		`SHOW TABLES IN "MY DB"."MY SCHEMA"`,
		"SHOW VIEWS IN my_schema",
		// STARTS WITH clause
		"SHOW TABLES STARTS WITH 'test'",
		"SHOW TABLES STARTS WITH 'TEST_'",
		// LIMIT clause
		"SHOW TABLES LIMIT 10",
		"SHOW TABLES LIMIT 1",
		"SHOW TABLES LIMIT 100 FROM 'my_table'",
		"SHOW TABLES LIMIT 50 FROM 'start_name'",
		"SHOW TABLES LIMIT 10 FROM ''",
		// Combined clauses (canonical order: LIKE → IN → STARTS WITH → LIMIT)
		"SHOW TABLES LIKE '%test%' IN DATABASE my_db",
		"SHOW TABLES LIKE '%test%' IN DATABASE my_db STARTS WITH 'test' LIMIT 10",
		"SHOW TERSE TABLES LIKE '%test%' IN SCHEMA",
		"SHOW TABLES IN ACCOUNT LIMIT 5",
		"SHOW VIEWS LIKE '%v%' IN SCHEMA my_db.my_schema LIMIT 20 FROM 'view_name'",
		// Combined clauses (non-canonical order — Snowflake accepts any order)
		"SHOW TABLES IN SCHEMA my_schema LIKE '%test%'",
		"SHOW TABLES LIMIT 10 STARTS WITH 'test_'",
		"SHOW TABLES IN DATABASE my_db LIKE '%foo%' LIMIT 5",
		"SHOW TABLES STARTS WITH 'a' LIKE '%b%' IN ACCOUNT LIMIT 1",
		"SHOW TABLES LIMIT 50 FROM 'x' IN DATABASE LIKE '%y%'",
		// GRANTS with non-standard syntax (clause validation skipped)
		"SHOW GRANTS ON ACCOUNT",
		"SHOW GRANTS TO ROLE admin",
		"SHOW GRANTS OF ROLE admin",
		// PARAMETERS with non-standard FOR syntax (clause validation skipped)
		"SHOW PARAMETERS",
		"SHOW PARAMETERS FOR USER my_user",
		"SHOW PARAMETERS FOR SESSION",
		"SHOW PARAMETERS IN SESSION",
		// Case insensitivity
		"show tables",
		"Show Views",
		"SHOW terse TABLES",
		// Comments (note: a leading block comment like "/* c */ SHOW TABLES"
		// causes reIsShow to not match, so the statement falls through to the
		// generic parser — this matches the behavior of other validators)
		"SHOW /* comment */ TABLES",
		"SHOW TABLES -- trailing comment",
		"SHOW TABLES LIKE '%test%' -- comment",
		// Quoted identifiers in IN clause (including keyword names)
		`SHOW TABLES IN DATABASE "my-db"`,
		`SHOW TABLES IN SCHEMA "MY DB"."MY SCHEMA"`,
		`SHOW TABLES IN DATABASE "LIKE"`,
		`SHOW TABLES IN DATABASE "IN"`,
		`SHOW TABLES IN "LIKE"`,
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
			"bare SHOW without object type",
			"SHOW",
			[]string{"SHOW requires an object type"},
		},
		{
			"unknown object type (typo)",
			"SHOW TABEL",
			[]string{"Unknown object type 'TABEL'"},
		},
		{
			"unknown object type INDEXES",
			"SHOW INDEXES",
			[]string{"Unknown object type 'INDEXES'"},
		},
		{
			"TERSE with invalid type PIPES",
			"SHOW TERSE PIPES",
			[]string{"TERSE is not valid for SHOW PIPES"},
		},
		{
			"TERSE with invalid type FUNCTIONS",
			"SHOW TERSE FUNCTIONS",
			[]string{"TERSE is not valid for SHOW FUNCTIONS"},
		},
		{
			"TERSE with invalid type ALERTS",
			"SHOW TERSE ALERTS",
			[]string{"TERSE is not valid for SHOW ALERTS"},
		},
		{
			"TERSE with unknown object type",
			"SHOW TERSE FOOBAR",
			[]string{"Unknown object type 'FOOBAR'"},
		},
		{
			"HISTORY with non-eligible type",
			"SHOW TABLES HISTORY",
			[]string{"HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES"},
		},
		{
			"HISTORY with VIEWS",
			"SHOW VIEWS HISTORY",
			[]string{"HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES"},
		},
		{
			"TERSE valid but HISTORY invalid for same type",
			"SHOW TERSE TABLES HISTORY",
			[]string{"HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES"},
		},
		{
			"LIKE without string literal",
			"SHOW TABLES LIKE test",
			[]string{"LIKE requires a string literal"},
		},
		{
			"LIKE with bare number",
			"SHOW TABLES LIKE 123",
			[]string{"LIKE requires a string literal"},
		},
		{
			"IN with empty scope",
			"SHOW TABLES IN",
			[]string{"IN clause requires a scope"},
		},
		{
			"IN with non-identifier scope (number)",
			"SHOW TABLES IN 123",
			[]string{"Invalid scope '123'"},
		},
		{
			"STARTS WITH without string literal",
			"SHOW TABLES STARTS WITH test",
			[]string{"STARTS WITH requires a string literal"},
		},
		{
			"LIMIT with zero",
			"SHOW TABLES LIMIT 0",
			[]string{"LIMIT requires a positive integer, got '0'"},
		},
		{
			"LIMIT with negative number",
			"SHOW TABLES LIMIT -1",
			[]string{"LIMIT requires a positive integer, got '-1'"},
		},
		{
			"LIMIT with non-integer",
			"SHOW TABLES LIMIT abc",
			[]string{"LIMIT requires a positive integer, got 'abc'"},
		},
		{
			"LIMIT with decimal number",
			"SHOW TABLES LIMIT 1.5",
			[]string{"LIMIT requires a positive integer, got '1.5'"},
		},
		{
			"LIMIT FROM without string literal",
			"SHOW TABLES LIMIT 10 FROM test",
			[]string{"FROM in LIMIT clause requires a string literal"},
		},
		{
			"bare SHOW TERSE without object type",
			"SHOW TERSE",
			[]string{"SHOW TERSE requires an object type"},
		},
		{
			"TERSE + HISTORY combined (TERSE invalid for PIPES)",
			"SHOW TERSE PIPES HISTORY",
			[]string{"TERSE is not valid for SHOW PIPES"},
		},
		{
			"trailing unrecognized token",
			"SHOW TABLES FOOBAR",
			[]string{"Unexpected token 'FOOBAR'"},
		},
		{
			"typo in clause keyword LIIKE",
			"SHOW TABLES LIIKE '%foo%'",
			[]string{"Unexpected token 'LIIKE'"},
		},
		// ── Duplicate clauses ────────────────────────────────────────
		{
			"duplicate LIKE clause",
			"SHOW TABLES LIKE '%a%' LIKE '%b%'",
			[]string{"Unexpected token 'LIKE'"},
		},
		{
			"duplicate IN clause",
			"SHOW TABLES IN ACCOUNT IN DATABASE",
			[]string{"Unexpected token 'IN'"},
		},
		{
			"duplicate LIMIT clause",
			"SHOW TABLES LIMIT 5 LIMIT 10",
			[]string{"Unexpected token 'LIMIT'"},
		},
		{
			"duplicate STARTS WITH clause",
			"SHOW TABLES STARTS WITH 'a' STARTS WITH 'b'",
			[]string{"Unexpected token 'STARTS'"},
		},
		// ── STARTS WITHOUT WITH ──────────────────────────────────────
		{
			"STARTS without WITH keyword",
			"SHOW TABLES STARTS foo",
			[]string{"Expected WITH after STARTS"},
		},
		{
			"bare STARTS at end of statement",
			"SHOW TABLES STARTS",
			[]string{"Expected WITH after STARTS"},
		},
		{
			"STARTS WITH without string literal",
			"SHOW TABLES STARTS WITH test_prefix",
			[]string{"STARTS WITH requires a string literal"},
		},
		// ── Bare LIMIT (no number) ───────────────────────────────────
		{
			"bare LIMIT at end of statement",
			"SHOW TABLES LIMIT",
			[]string{"LIMIT requires a positive integer"},
		},
		// ── Unterminated string literals ─────────────────────────────
		{
			"LIKE with unterminated string literal",
			"SHOW TABLES LIKE 'unterminated",
			[]string{"Unterminated string literal in LIKE clause"},
		},
		{
			"STARTS WITH unterminated string literal",
			"SHOW TABLES STARTS WITH 'unterminated",
			[]string{"Unterminated string literal in STARTS WITH clause"},
		},
		{
			"LIMIT FROM with unterminated string literal",
			"SHOW TABLES LIMIT 10 FROM 'unterminated",
			[]string{"Unterminated string literal in LIMIT FROM clause"},
		},
		// ── LIMIT FROM without string ────────────────────────────────
		{
			"LIMIT FROM without string literal (bare word)",
			"SHOW TABLES LIMIT 10 FROM",
			[]string{"FROM in LIMIT clause requires a string literal"},
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

	// Multi-statement test: SHOW embedded between other statements.
	t.Run("multi-statement with valid SHOW", func(t *testing.T) {
		sql := "SELECT 1;\nSHOW TABLES LIKE '%test%';\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings for multi-statement SQL, got %d: %v", len(warns), warns)
		}
	})

	t.Run("multi-statement with invalid SHOW", func(t *testing.T) {
		sql := "SELECT 1;\nSHOW TABEL;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning, got %d: %v", len(warns), warns)
		}
		if len(warns) > 0 && !strings.Contains(warns[0].Message, "Unknown object type 'TABEL'") {
			t.Errorf("Expected object type warning, got: %v", warns[0].Message)
		}
	})
}

// TestShowObjectTypes_OrderingInvariant verifies that showObjectTypes is sorted
// by word count descending so the longest match is always attempted first.
func TestShowObjectTypes_OrderingInvariant(t *testing.T) {
	prevWords := 100 // start high
	for i, ot := range showObjectTypes {
		n := len(strings.Fields(ot))
		if n > prevWords {
			t.Errorf("showObjectTypes[%d] = %q has %d words but follows an entry with %d words; entries must be sorted by word count descending",
				i, ot, n, prevWords)
		}
		prevWords = n
	}
}

func TestDescribeObjectTypes_OrderingInvariant(t *testing.T) {
	prevWords := 100 // start high
	for i, ot := range describeObjectTypes {
		n := len(strings.Fields(ot))
		if n > prevWords {
			t.Errorf("describeObjectTypes[%d] = %q has %d words but follows an entry with %d words; entries must be sorted by word count descending",
				i, ot, n, prevWords)
		}
		prevWords = n
	}
}

func TestValidateSnowflakePatterns_Describe(t *testing.T) {
	validCases := []string{
		// ── Basic single-word object types ────────────────────────────────
		"DESCRIBE TABLE my_table",
		"DESCRIBE VIEW my_view",
		"DESCRIBE STAGE my_stage",
		"DESCRIBE STREAM my_stream",
		"DESCRIBE TASK my_task",
		"DESCRIBE PIPE my_pipe",
		"DESCRIBE SEQUENCE my_seq",
		"DESCRIBE DATABASE my_db",
		"DESCRIBE SCHEMA my_schema",
		"DESCRIBE WAREHOUSE my_wh",
		"DESCRIBE USER my_user",
		"DESCRIBE ROLE my_role",
		"DESCRIBE INTEGRATION my_int",
		"DESCRIBE SHARE my_share",
		"DESCRIBE ALERT my_alert",
		"DESCRIBE TAG my_tag",
		"DESCRIBE SECRET my_secret",
		"DESCRIBE SERVICE my_svc",
		// ── Two-word object types ────────────────────────────────────────
		"DESCRIBE NETWORK POLICY my_np",
		"DESCRIBE MASKING POLICY my_mp",
		"DESCRIBE ROW ACCESS POLICY my_rap",
		"DESCRIBE SESSION POLICY my_sp",
		"DESCRIBE PASSWORD POLICY my_pp",
		"DESCRIBE AGGREGATION POLICY my_ap",
		"DESCRIBE PROJECTION POLICY my_pp2",
		"DESCRIBE PACKAGES POLICY my_pkg_pol",
		"DESCRIBE EXTERNAL TABLE my_ext_tbl",
		"DESCRIBE DYNAMIC TABLE my_dyn_tbl",
		"DESCRIBE EVENT TABLE my_evt_tbl",
		"DESCRIBE FILE FORMAT my_ff",
		"DESCRIBE RESOURCE MONITOR my_rm",
		"DESCRIBE REPLICATION GROUP my_rg",
		"DESCRIBE FAILOVER GROUP my_fg",
		// ── DESC alias ───────────────────────────────────────────────────
		"DESC TABLE my_table",
		"DESC VIEW my_view",
		"DESC STAGE my_stage",
		"DESC FUNCTION my_func(NUMBER, VARCHAR)",
		"DESC PROCEDURE my_proc(NUMBER)",
		"DESC NETWORK POLICY my_np",
		"DESC MASKING POLICY my_mp",
		"DESC ROW ACCESS POLICY my_rap",
		// ── Three-part names ─────────────────────────────────────────────
		"DESCRIBE TABLE my_db.my_schema.my_table",
		"DESCRIBE VIEW db.sch.vw",
		"DESC TABLE db.sch.tbl",
		// ── Two-part names ───────────────────────────────────────────────
		"DESCRIBE TABLE my_schema.my_table",
		"DESC VIEW sch.vw",
		// ── Quoted identifiers ───────────────────────────────────────────
		`DESCRIBE TABLE "my-table"`,
		`DESCRIBE TABLE "MY DB"."MY SCHEMA"."MY TABLE"`,
		`DESC VIEW "complex""name"`,
		// ── FUNCTION / PROCEDURE with signatures ─────────────────────────
		"DESCRIBE FUNCTION my_func(NUMBER, VARCHAR)",
		"DESCRIBE FUNCTION my_func()",
		"DESCRIBE FUNCTION db.schema.my_func(INT)",
		"DESCRIBE PROCEDURE my_proc(VARCHAR, NUMBER, BOOLEAN)",
		"DESCRIBE PROCEDURE my_proc()",
		"DESC FUNCTION multiply(NUMBER, NUMBER)",
		"DESC PROCEDURE my_pi()",
		// ── RESULT / TRANSACTION (special: take string-literal IDs) ─────
		"DESCRIBE RESULT '01a4567b-0000-0000-0000-000000000000'",
		"DESC RESULT 'last_query_id'",
		"DESCRIBE TRANSACTION 123456789",
		// ── Case insensitivity ───────────────────────────────────────────
		"describe table my_table",
		"Describe View my_view",
		"desc table my_table",
		// ── With comments ────────────────────────────────────────────────
		"DESCRIBE TABLE my_table -- trailing comment",
		"DESCRIBE /* comment */ TABLE my_table",
		// ── Additional object types ──────────────────────────────────────
		"DESCRIBE APPLICATION my_app",
		"DESCRIBE APPLICATION PACKAGE my_pkg",
		"DESCRIBE CATALOG INTEGRATION my_ci",
		"DESCRIBE COMPUTE POOL my_cp",
		"DESCRIBE EXTERNAL VOLUME my_ev",
		"DESCRIBE NOTIFICATION INTEGRATION my_ni",
		"DESCRIBE GIT REPOSITORY my_repo",
		"DESCRIBE ICEBERG TABLE my_it",
		"DESCRIBE NETWORK RULE my_nr",
		"DESCRIBE CORTEX SEARCH SERVICE my_css",
		"DESCRIBE AUTHENTICATION POLICY my_auth_pol",
		// ── Newly added object types ─────────────────────────────────────
		"DESCRIBE MATERIALIZED VIEW my_mv",
		"DESCRIBE STREAMLIT my_st",
		"DESCRIBE NOTEBOOK my_nb",
		"DESCRIBE SEMANTIC VIEW my_sv",
		"DESCRIBE SNAPSHOT my_snap",
		"DESCRIBE MCP SERVER my_mcp",
		"DESCRIBE ONLINE FEATURE TABLE my_oft",
		"DESCRIBE OPENFLOW DATA PLANE INTEGRATION my_odpi",
		"DESCRIBE STORAGE LIFECYCLE POLICY my_slp",
		"DESCRIBE POSTGRES INSTANCE my_pi",
		"DESCRIBE ORGANIZATION PROFILE my_op",
		"DESCRIBE LISTING my_listing",
		"DESCRIBE SPECIFICATION my_spec",
		// ── Quoted identifiers with embedded dots (no false positive) ────
		`DESCRIBE WAREHOUSE "my.warehouse"`,
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
		// ── Bare DESCRIBE / DESC ─────────────────────────────────────────
		{
			"bare DESCRIBE without anything",
			"DESCRIBE",
			[]string{"DESCRIBE requires an object type and name"},
		},
		{
			"bare DESC without anything",
			"DESC",
			[]string{"DESCRIBE requires an object type and name"},
		},
		// ── Unknown object type ──────────────────────────────────────────
		{
			"unknown object type TABEL (typo)",
			"DESCRIBE TABEL my_table",
			[]string{"Unknown object type 'TABEL'"},
		},
		{
			"unknown object type INDEX",
			"DESC INDEX my_idx",
			[]string{"Unknown object type 'INDEX'"},
		},
		// ── Missing object name ──────────────────────────────────────────
		{
			"DESCRIBE TABLE with no name",
			"DESCRIBE TABLE",
			[]string{"DESCRIBE TABLE requires an object name"},
		},
		{
			"DESC VIEW with no name",
			"DESC VIEW",
			[]string{"DESCRIBE VIEW requires an object name"},
		},
		{
			"DESCRIBE STAGE with no name",
			"DESCRIBE STAGE",
			[]string{"DESCRIBE STAGE requires an object name"},
		},
		{
			"DESCRIBE NETWORK POLICY with no name",
			"DESCRIBE NETWORK POLICY",
			[]string{"DESCRIBE NETWORK POLICY requires an object name"},
		},
		{
			"DESCRIBE ROW ACCESS POLICY with no name (3-word type)",
			"DESCRIBE ROW ACCESS POLICY",
			[]string{"DESCRIBE ROW ACCESS POLICY requires an object name"},
		},
		{
			"DESCRIBE EXTERNAL TABLE with no name (2-word type)",
			"DESCRIBE EXTERNAL TABLE",
			[]string{"DESCRIBE EXTERNAL TABLE requires an object name"},
		},
		// ── Non-identifier object name ──────────────────────────────────
		{
			"DESCRIBE TABLE with non-identifier name (number)",
			"DESCRIBE TABLE 123",
			[]string{"Expected an object name after DESCRIBE TABLE"},
		},
		// ── FUNCTION without signature ───────────────────────────────────
		{
			"DESCRIBE FUNCTION without parens",
			"DESCRIBE FUNCTION my_func",
			[]string{"DESCRIBE FUNCTION requires a parameter signature"},
		},
		{
			"DESC FUNCTION without parens",
			"DESC FUNCTION my_func",
			[]string{"DESCRIBE FUNCTION requires a parameter signature"},
		},
		// ── PROCEDURE without signature ──────────────────────────────────
		{
			"DESCRIBE PROCEDURE without parens",
			"DESCRIBE PROCEDURE my_proc",
			[]string{"DESCRIBE PROCEDURE requires a parameter signature"},
		},
		// ── Account-level object with db/schema prefix ───────────────────
		{
			"DESCRIBE WAREHOUSE with schema prefix",
			"DESCRIBE WAREHOUSE my_db.my_wh",
			[]string{"WAREHOUSE is an account-level object and should not be qualified"},
		},
		{
			"DESC USER with db prefix",
			"DESC USER my_db.my_user",
			[]string{"USER is an account-level object and should not be qualified"},
		},
		{
			"DESCRIBE ROLE with three-part name",
			"DESCRIBE ROLE db.schema.my_role",
			[]string{"ROLE is an account-level object and should not be qualified"},
		},
		{
			"DESCRIBE INTEGRATION with prefix",
			"DESCRIBE INTEGRATION db.my_int",
			[]string{"INTEGRATION is an account-level object and should not be qualified"},
		},
		{
			"DESCRIBE DATABASE with prefix",
			"DESCRIBE DATABASE other_db.my_db",
			[]string{"DATABASE is an account-level object and should not be qualified"},
		},
		// ── Multi-word account-level object with db/schema prefix ─────────
		{
			"DESCRIBE RESOURCE MONITOR with schema prefix",
			"DESCRIBE RESOURCE MONITOR db.my_rm",
			[]string{"RESOURCE MONITOR is an account-level object and should not be qualified"},
		},
		{
			"DESCRIBE SPECIFICATION with schema prefix",
			"DESCRIBE SPECIFICATION db.my_spec",
			[]string{"SPECIFICATION is an account-level object and should not be qualified"},
		},
		// ── RESULT / TRANSACTION without ID ──────────────────────────
		{
			"DESCRIBE RESULT without ID",
			"DESCRIBE RESULT",
			[]string{"DESCRIBE RESULT requires a query/transaction ID"},
		},
		{
			"DESC RESULT without ID",
			"DESC RESULT",
			[]string{"DESCRIBE RESULT requires a query/transaction ID"},
		},
		{
			"DESCRIBE TRANSACTION without ID",
			"DESCRIBE TRANSACTION",
			[]string{"DESCRIBE TRANSACTION requires a query/transaction ID"},
		},
		// ── Trailing unrecognized content ────────────────────────────────
		{
			"DESCRIBE TABLE with trailing garbage",
			"DESCRIBE TABLE my_table SOME_GARBAGE",
			[]string{"Unexpected token 'SOME_GARBAGE' after object name"},
		},
		{
			"DESCRIBE VIEW with extra words",
			"DESCRIBE VIEW my_view EXTRA STUFF",
			[]string{"Unexpected token 'EXTRA' after object name"},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tc.sql)
				return
			}
			for _, wantMsg := range tc.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tc.sql, warns)
				}
			}
		})
	}

	// Multi-statement test: DESCRIBE embedded between other statements.
	t.Run("multi-statement with valid DESCRIBE", func(t *testing.T) {
		sql := "SELECT 1;\nDESCRIBE TABLE my_table;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings for multi-statement SQL, got %d: %v", len(warns), warns)
		}
	})

	t.Run("multi-statement with invalid DESCRIBE", func(t *testing.T) {
		sql := "SELECT 1;\nDESCRIBE TABEL my_table;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning, got %d: %v", len(warns), warns)
		}
		if len(warns) > 0 && !strings.Contains(warns[0].Message, "Unknown object type 'TABEL'") {
			t.Errorf("Expected object type warning, got: %v", warns[0].Message)
		}
	})
}

// TestCountIdentParts tests the countIdentParts helper that counts
// dot-separated parts in an identifier path, skipping dots inside quotes.
func TestCountIdentParts(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"my_table", 1},
		{"schema.table", 2},
		{"db.schema.table", 3},
		// Dots inside quoted identifiers are not separators.
		{`"my.table"`, 1},
		{`"db.schema".table`, 2},
		{`db."my.schema".table`, 3},
		// Empty string (degenerate).
		{"", 1},
		// All quoted parts.
		{`"a"."b"."c"`, 3},
		// Quoted identifier with no dots.
		{`"simple"`, 1},
		// Escaped double-quote inside quoted identifier.
		{`"complex""name"`, 1},
		// Mixed: escaped-quote ident dot regular ident.
		{`"a""b".table`, 2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := countIdentParts(tt.input)
			if got != tt.want {
				t.Errorf("countIdentParts(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ── Tag Tests ────────────────────────────────────────────────────────────────


