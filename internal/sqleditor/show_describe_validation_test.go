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
		"SHOW WORKSPACES",
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

// TestValidateShow_ClauseKeywordBoundary tests that clause keywords (LIKE,
// IN, STARTS, LIMIT, FROM) are not consumed as identifiers in the IN clause.
func TestValidateShow_ClauseKeywordBoundary(t *testing.T) {
	// Valid: LIKE follows an IN scope without being consumed as the scope name.
	validCases := []string{
		"SHOW TABLES IN SCHEMA LIKE '%test%'",
		"SHOW TABLES IN DATABASE LIKE '%test%'",
		"SHOW TABLES IN DATABASE my_db LIMIT 5",
		"SHOW TABLES IN SCHEMA my_schema STARTS WITH 'a'",
		// Quoted clause keywords as identifiers should be consumed.
		`SHOW TABLES IN "LIKE"`,
		`SHOW TABLES IN DATABASE "LIMIT"`,
		`SHOW TABLES IN SCHEMA "FROM"."STARTS"`,
		// Escaped quotes in STARTS WITH.
		"SHOW TABLES STARTS WITH 'it''s a prefix'",
		// Escaped quotes in LIMIT FROM.
		"SHOW TABLES LIMIT 10 FROM 'it''s a name'",
		// Very large LIMIT.
		"SHOW TABLES LIMIT 999999",
		// Tab and newline as whitespace.
		"SHOW\tTABLES",
		"SHOW TABLES\tLIKE\t'%x%'",
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

	// Invalid: clause keyword as implicit scope is rejected.
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"IN with LIKE as implicit scope (clause keyword not consumed)",
			"SHOW TABLES IN LIKE '%x%'",
			[]string{"Invalid scope 'LIKE'"},
		},
		{
			"IN with STARTS as implicit scope",
			"SHOW TABLES IN STARTS WITH 'x'",
			[]string{"Invalid scope 'STARTS'"},
		},
		{
			"IN with LIMIT as implicit scope",
			"SHOW TABLES IN LIMIT",
			[]string{"Invalid scope 'LIMIT'"},
		},
		{
			"IN with FROM as implicit scope",
			"SHOW TABLES IN FROM",
			[]string{"Invalid scope 'FROM'"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateDescribe_EdgeCases tests additional edge cases for DESCRIBE.
func TestValidateDescribe_EdgeCases(t *testing.T) {
	validCases := []string{
		// RESULT/TRANSACTION with any non-empty content (no format validation).
		"DESCRIBE RESULT 'anything-here'",
		"DESCRIBE TRANSACTION 999",
		"DESC RESULT some_id",
		"DESC TRANSACTION 0",
		// Qualified FUNCTION with signature.
		"DESCRIBE FUNCTION db.schema.my_func(INT, VARCHAR)",
		"DESC PROCEDURE db.sch.proc()",
		// Whitespace variations.
		"DESCRIBE\tTABLE\tmy_table",
		"DESC  VIEW  my_view",
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
			"DESC FUNCTION with qualified name but no parens",
			"DESC FUNCTION db.schema.my_func",
			[]string{"DESCRIBE FUNCTION requires a parameter signature"},
		},
		{
			"DESC PROCEDURE with qualified name but no parens",
			"DESCRIBE PROCEDURE db.schema.my_proc",
			[]string{"DESCRIBE PROCEDURE requires a parameter signature"},
		},
		{
			"DESCRIBE SHARE (account-level) with prefix",
			"DESCRIBE SHARE db.my_share",
			[]string{"SHARE is an account-level object and should not be qualified"},
		},
		{
			"DESCRIBE with only whitespace after object type",
			"DESCRIBE TABLE   ",
			[]string{"DESCRIBE TABLE requires an object name"},
		},
		{
			"DESC with only whitespace after object type",
			"DESC VIEW   ",
			[]string{"DESCRIBE VIEW requires an object name"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestCountIdentParts_AdditionalEdgeCases extends countIdentParts coverage
// with multi-part and degenerate inputs.
func TestCountIdentParts_AdditionalEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		// Four-part identifier (rare but valid input to the function).
		{"a.b.c.d", 4},
		// Unterminated quoted identifier — dot after opening quote is inside.
		{`"abc`, 1},
		{`"a.b`, 1},
		// Trailing dot.
		{"table.", 2},
		// Leading dot.
		{".table", 2},
		// Just a dot.
		{".", 2},
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

// TestValidateShow_AdditionalEdgeCases tests additional SHOW edge cases not
// covered by the main test table.
func TestValidateShow_AdditionalEdgeCases(t *testing.T) {
	// Valid cases that exercise boundary conditions.
	validCases := []string{
		// SHOW with trailing whitespace only (should not differ from bare SHOW
		// keyword followed by object type, but whitespace-only remainder should
		// still be caught at the "requires an object type" check).
		// ── HISTORY modifier followed by additional clauses ──
		"SHOW PIPES HISTORY LIKE '%test%'",
		"SHOW PIPES HISTORY LIMIT 10",
		"SHOW REPLICATION DATABASES HISTORY LIKE '%prod%' LIMIT 5",
		// ── FUTURE GRANTS variants ──
		"SHOW FUTURE GRANTS IN SCHEMA my_db.my_schema",
		// ── Bare explicit scopes (without an identifier after the scope keyword) ──
		"SHOW TABLES IN TABLE",
		"SHOW TABLES IN SCHEMA",
		// ── Dollar sign in identifiers ──
		"SHOW TABLES IN my$schema",
		"SHOW TABLES IN DATABASE my$db",
		"SHOW TABLES IN SCHEMA db.my$schema",
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

	// Invalid: SHOW with only trailing whitespace is still bare SHOW.
	t.Run("SHOW with trailing whitespace", func(t *testing.T) {
		sql := "SHOW   "
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning for %q, got %d: %v", sql, len(warns), warns)
			return
		}
		if !strings.Contains(warns[0].Message, "SHOW requires an object type") {
			t.Errorf("Expected 'SHOW requires an object type' warning, got: %v", warns[0].Message)
		}
	})

	// Invalid: SHOW TERSE with trailing whitespace.
	t.Run("SHOW TERSE with trailing whitespace", func(t *testing.T) {
		sql := "SHOW TERSE   "
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning for %q, got %d: %v", sql, len(warns), warns)
			return
		}
		if !strings.Contains(warns[0].Message, "SHOW TERSE requires an object type") {
			t.Errorf("Expected 'SHOW TERSE requires an object type' warning, got: %v", warns[0].Message)
		}
	})
}

// TestValidateDescribe_AdditionalEdgeCases tests additional DESCRIBE edge cases.
func TestValidateDescribe_AdditionalEdgeCases(t *testing.T) {
	validCases := []string{
		// Quoted identifier that looks like a keyword as object name.
		`DESCRIBE TABLE "LIKE"`,
		`DESCRIBE TABLE "IN"`,
		`DESCRIBE TABLE "SHOW"`,
		// FUNCTION with unclosed paren (implementation only checks Contains("(")).
		"DESCRIBE FUNCTION my_func(INT",
		// Four-word object type.
		"DESCRIBE OPENFLOW DATA PLANE INTEGRATION my_odpi",
		// Dollar sign in identifier (valid Snowflake identifier character).
		"DESCRIBE TABLE my$table",
		"DESCRIBE TABLE db.sch.my$table",
		"DESC VIEW my$view",
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
			"multi-word unknown type (first word flagged)",
			"DESCRIBE FOOBAR BAZ my_name",
			[]string{"Unknown object type 'FOOBAR'"},
		},
		{
			"DESC with trailing whitespace only (bare DESC)",
			"DESC   ",
			[]string{"DESCRIBE requires an object type and name"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}

	// Multi-statement with DESC alias (main tests only use DESCRIBE).
	t.Run("multi-statement with valid DESC", func(t *testing.T) {
		sql := "SELECT 1;\nDESC TABLE my_table;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings for multi-statement with DESC, got %d: %v", len(warns), warns)
		}
	})

	t.Run("multi-statement with invalid DESC", func(t *testing.T) {
		sql := "SELECT 1;\nDESC TABEL my_table;\nSELECT 2"
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

// TestValidateShow_WhitespaceVariations tests that SHOW validation handles
// various whitespace characters (newlines, carriage returns) between tokens.
func TestValidateShow_WhitespaceVariations(t *testing.T) {
	validCases := []string{
		"SHOW\nTABLES",
		"SHOW\r\nTABLES",
		"SHOW TABLES\nLIKE\n'%x%'",
		"SHOW\n\tTABLES\n\tLIKE '%x%'\n\tIN DATABASE",
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
}

// TestValidateShow_ThreeWordTypeWithClauses tests that three-word object types
// work correctly when combined with optional clauses.
func TestValidateShow_ThreeWordTypeWithClauses(t *testing.T) {
	validCases := []string{
		"SHOW CORTEX SEARCH SERVICES LIKE '%test%'",
		"SHOW DATA METRIC FUNCTIONS IN DATABASE my_db",
		"SHOW ROW ACCESS POLICIES LIKE '%prod%' IN SCHEMA my_schema LIMIT 10",
		"SHOW CORTEX SEARCH SERVICES STARTS WITH 'my_'",
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
}

// TestValidateShow_AllClausesCombined tests SHOW with all four optional
// clauses (LIKE, IN, STARTS WITH, LIMIT) present simultaneously.
func TestValidateShow_AllClausesCombined(t *testing.T) {
	validCases := []string{
		"SHOW TABLES LIKE '%test%' IN DATABASE my_db STARTS WITH 'test' LIMIT 10",
		"SHOW TABLES LIKE '%test%' IN SCHEMA my_schema STARTS WITH 'a' LIMIT 5 FROM 'x'",
		// Reverse order.
		"SHOW TABLES LIMIT 10 STARTS WITH 'a' IN ACCOUNT LIKE '%x%'",
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
}

// TestValidateShow_MisplacedModifier tests that TERSE after the object type
// (instead of before it) is flagged as unexpected.
func TestValidateShow_MisplacedModifier(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"TERSE after object type",
			"SHOW TABLES TERSE",
			[]string{"Unexpected token 'TERSE'"},
		},
		{
			"HISTORY before object type (misplaced)",
			"SHOW HISTORY PIPES",
			[]string{"Unknown object type 'HISTORY'"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_LikeWithDoubleQuotes tests that LIKE with a double-quoted
// string (instead of single-quoted) is flagged.
func TestValidateShow_LikeWithDoubleQuotes(t *testing.T) {
	sql := `SHOW TABLES LIKE "test"`
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)

	if len(warns) == 0 {
		t.Errorf("Expected warning for LIKE with double-quoted string, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "LIKE requires a string literal") {
		t.Errorf("Expected 'LIKE requires a string literal' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_InWithNonKeywordScope tests that IN with a non-keyword
// identifier is treated as valid implicit scope (e.g. schema named HISTORY).
func TestValidateShow_InWithNonKeywordScope(t *testing.T) {
	validCases := []string{
		"SHOW TABLES IN HISTORY",
		"SHOW TABLES IN TERSE",
		"SHOW TABLES IN MY_SCHEMA",
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
}

// TestValidateDescribe_CommentBetweenTypeAndName tests that block comments
// between object type and name are correctly stripped.
func TestValidateDescribe_CommentBetweenTypeAndName(t *testing.T) {
	validCases := []string{
		"DESCRIBE TABLE /* comment */ my_table",
		"DESC VIEW /* multi\nline\ncomment */ my_view",
		"DESCRIBE TABLE -- comment\nmy_table", // line comment before name on next line
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
}

// TestValidateDescribe_QualifiedNonAccountLevel tests that non-account-level
// object types with schema/database qualification are accepted.
func TestValidateDescribe_QualifiedNonAccountLevel(t *testing.T) {
	validCases := []string{
		"DESCRIBE FILE FORMAT my_db.my_ff",
		"DESCRIBE DYNAMIC TABLE my_db.my_schema.my_dt",
		"DESCRIBE MASKING POLICY my_db.my_schema.my_mp",
		"DESCRIBE EVENT TABLE my_db.my_schema.my_et",
		"DESC STAGE my_db.my_schema.my_stage",
		"DESCRIBE TABLE my_db.my_schema.my_table",
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
}

// TestValidateDescribe_AccountLevelMultiWord tests that multi-word account-level
// object types are flagged when qualified with a db/schema prefix.
func TestValidateDescribe_AccountLevelMultiWord(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"NOTIFICATION INTEGRATION with prefix",
			"DESCRIBE NOTIFICATION INTEGRATION db.my_ni",
			[]string{"NOTIFICATION INTEGRATION is an account-level object and should not be qualified"},
		},
		{
			"CATALOG INTEGRATION with prefix",
			"DESCRIBE CATALOG INTEGRATION db.my_ci",
			[]string{"CATALOG INTEGRATION is an account-level object and should not be qualified"},
		},
		{
			"COMPUTE POOL with prefix",
			"DESC COMPUTE POOL db.my_cp",
			[]string{"COMPUTE POOL is an account-level object and should not be qualified"},
		},
		{
			"EXTERNAL VOLUME with prefix",
			"DESCRIBE EXTERNAL VOLUME db.my_ev",
			[]string{"EXTERNAL VOLUME is an account-level object and should not be qualified"},
		},
		{
			"NETWORK POLICY with three-part name",
			"DESCRIBE NETWORK POLICY db.schema.my_np",
			[]string{"NETWORK POLICY is an account-level object and should not be qualified"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestCountIdentParts_QuotedWithDotsAndEscapes extends countIdentParts coverage
// for quoted identifiers with dots inside escaped quotes.
func TestCountIdentParts_QuotedWithDotsAndEscapes(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		// Quoted identifier with escaped quote and dot.
		{`"a""b.c"`, 1},
		// Multiple consecutive dots (degenerate but valid to count).
		{"a..b", 3},
		// Quoted then unquoted with dots.
		{`"a.b".c.d`, 3},
		// All dots (degenerate).
		{"..", 3},
		// Quoted empty string (degenerate).
		{`""`, 1},
		// Dollar sign in identifier (valid Snowflake identifier character).
		{"my$table", 1},
		{"db.my$schema.table", 3},
		{"$seq.nextval", 2},
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

// TestValidateShow_MultipleStatementsMultipleErrors tests that multiple invalid
// SHOW/DESCRIBE statements in a single SQL string each produce their own warnings.
func TestValidateShow_MultipleStatementsMultipleErrors(t *testing.T) {
	sql := "SHOW TABEL;\nSHOW TERSE;\nDESCRIBE TABEL my_table"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)

	if len(warns) < 3 {
		t.Errorf("Expected at least 3 warnings for three invalid statements, got %d: %v", len(warns), warns)
	}
}

// TestValidateShow_BareLikeAndStartsWith tests that bare LIKE and STARTS WITH
// (with nothing after the keyword) produce appropriate errors.
func TestValidateShow_BareLikeAndStartsWith(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"bare LIKE at end of statement",
			"SHOW TABLES LIKE",
			[]string{"LIKE requires a string literal"},
		},
		{
			"bare STARTS WITH at end of statement (nothing after WITH)",
			"SHOW TABLES STARTS WITH",
			[]string{"STARTS WITH requires a string literal"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}

	// Valid: STARTS WITH with empty string literal (analogous to LIKE '').
	t.Run("STARTS WITH empty string literal", func(t *testing.T) {
		sql := "SHOW TABLES STARTS WITH ''"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
		}
	})
}

// TestValidateShow_InScopeWithSpecialChars tests the IN clause with various
// quoted identifier patterns that do not contain escaped (doubled) quotes.
func TestValidateShow_InScopeWithSpecialChars(t *testing.T) {
	validCases := []string{
		`SHOW TABLES IN "MY-SCHEMA"`,
		`SHOW TABLES IN DATABASE "MY DB"`,
		`SHOW TABLES IN SCHEMA "UPPER"."LOWER"`,
		`SHOW TABLES IN "123starts_with_digit"`,
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
}

// TestValidateDescribe_FunctionBareParens tests that DESCRIBE FUNCTION with
// only empty parens (no name preceding them) still passes the signature check
// since the validator only verifies the presence of '(' in the remainder.
func TestValidateDescribe_FunctionBareParens(t *testing.T) {
	validCases := []string{
		"DESCRIBE FUNCTION ()",
		"DESC PROCEDURE ()",
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
}

// TestValidateShow_TerseWithMultiWordType tests that TERSE validation works
// correctly for multi-word object types that are not TERSE-eligible.
func TestValidateShow_TerseWithMultiWordType(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"TERSE with DYNAMIC TABLES (two-word, not eligible)",
			"SHOW TERSE DYNAMIC TABLES",
			[]string{"TERSE is not valid for SHOW DYNAMIC TABLES"},
		},
		{
			"TERSE with ROW ACCESS POLICIES (three-word, not eligible)",
			"SHOW TERSE ROW ACCESS POLICIES",
			[]string{"TERSE is not valid for SHOW ROW ACCESS POLICIES"},
		},
		{
			"TERSE with NETWORK POLICIES (two-word, not eligible)",
			"SHOW TERSE NETWORK POLICIES",
			[]string{"TERSE is not valid for SHOW NETWORK POLICIES"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateDescribe_TrailingCommentNoName tests that DESCRIBE with a
// trailing comment but no object name is correctly flagged after comment stripping.
func TestValidateDescribe_TrailingCommentNoName(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"DESCRIBE TABLE with only a line comment (no name)",
			"DESCRIBE TABLE -- no name here",
			[]string{"DESCRIBE TABLE requires an object name"},
		},
		{
			"DESC VIEW with only a block comment (no name)",
			"DESC VIEW /* no name */",
			[]string{"DESCRIBE VIEW requires an object name"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_CommentBetweenTerseAndType tests that block comments
// between the TERSE modifier and the object type are correctly stripped.
func TestValidateShow_CommentBetweenTerseAndType(t *testing.T) {
	validCases := []string{
		"SHOW TERSE /* comment */ TABLES",
		"SHOW TERSE /* multi\nline */ VIEWS",
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
}

// TestValidateShow_LimitWithLeadingZeros tests that LIMIT accepts numbers
// with leading zeros (strconv.Atoi handles them as valid integers).
func TestValidateShow_LimitWithLeadingZeros(t *testing.T) {
	sql := "SHOW TABLES LIMIT 007"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	if warns := getWarnings(markers); len(warns) > 0 {
		t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
	}
}

// TestValidateShow_NoClauseValidationBare tests that object types in
// showNoClauseValidation (GRANTS, FUTURE GRANTS, PARAMETERS) are accepted
// with no trailing content at all.
func TestValidateShow_NoClauseValidationBare(t *testing.T) {
	validCases := []string{
		"SHOW GRANTS",
		"SHOW FUTURE GRANTS",
		// SHOW PARAMETERS is already tested in the main table but included
		// here for completeness alongside the other noClauseValidation types.
		"SHOW PARAMETERS",
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

// TestValidateDescribe_PartialMultiWordType tests that only the first word of
// a multi-word type is flagged as unknown when the remaining words don't
// complete a known multi-word type.
func TestValidateDescribe_PartialMultiWordType(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"CORTEX alone (valid only as CORTEX SEARCH SERVICE)",
			"DESCRIBE CORTEX my_svc",
			[]string{"Unknown object type 'CORTEX'"},
		},
		{
			"MCP alone (valid only as MCP SERVER)",
			"DESCRIBE MCP my_server",
			[]string{"Unknown object type 'MCP'"},
		},
		{
			"ONLINE alone (valid only as ONLINE FEATURE TABLE)",
			"DESC ONLINE my_table",
			[]string{"Unknown object type 'ONLINE'"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateDescribe_MultiWordTypeMissingName_LongestMatch tests that when
// the first word of a multi-word type is itself a valid single-word type,
// the longest match wins and the missing-name error references the full
// multi-word type (not just the first word).
func TestValidateDescribe_MultiWordTypeMissingName_LongestMatch(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"APPLICATION PACKAGE (APPLICATION is also a valid single-word type)",
			"DESCRIBE APPLICATION PACKAGE",
			[]string{"DESCRIBE APPLICATION PACKAGE requires an object name"},
		},
		{
			"CATALOG INTEGRATION (no single-word CATALOG type)",
			"DESCRIBE CATALOG INTEGRATION",
			[]string{"DESCRIBE CATALOG INTEGRATION requires an object name"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_CommentStripsObjectType tests that when a block comment
// replaces the intended object type, the validator correctly reports bare SHOW.
func TestValidateShow_CommentStripsObjectType(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"block comment replaces object type",
			"SHOW /* TABLES */",
			[]string{"SHOW requires an object type"},
		},
		{
			"block comment replaces object type after TERSE",
			"SHOW TERSE /* TABLES */",
			[]string{"SHOW TERSE requires an object type"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_LikeEscapedQuoteOnlyPattern tests that a LIKE pattern
// consisting solely of an escaped single quote (””) is accepted.
func TestValidateShow_LikeEscapedQuoteOnlyPattern(t *testing.T) {
	sql := "SHOW TABLES LIKE ''''"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	if warns := getWarnings(markers); len(warns) > 0 {
		t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
	}
}

// TestValidateShow_LineCommentStripping tests that line comments (--) are
// correctly stripped, including cases where the line comment removes the
// object type entirely vs. only preceding it on the prior line.
func TestValidateShow_LineCommentStripping(t *testing.T) {
	validCases := []string{
		// Line comment between SHOW and object type (type on next line).
		"SHOW -- comment\nTABLES",
		"SHOW -- some note\nTERSE TABLES",
		"SHOW TERSE -- pick tables\nTABLES",
		// Line comment between clauses.
		"SHOW TABLES -- listing\nLIKE '%test%'",
		"SHOW TABLES IN DATABASE -- in my db\nmy_db",
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

	// Invalid: line comment removes the object type entirely (single line).
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"line comment removes object type",
			"SHOW -- TABLES",
			[]string{"SHOW requires an object type"},
		},
		{
			"line comment removes object type after TERSE",
			"SHOW TERSE -- TABLES",
			[]string{"SHOW TERSE requires an object type"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateDescribe_UnquotedKeywordNames tests that DESCRIBE TABLE accepts
// unquoted identifiers that happen to match SQL keywords (LIKE, IN, LIMIT, etc.)
// since the DESCRIBE validator does not have clause-keyword awareness.
func TestValidateDescribe_UnquotedKeywordNames(t *testing.T) {
	validCases := []string{
		"DESCRIBE TABLE LIKE",
		"DESCRIBE TABLE IN",
		"DESCRIBE TABLE LIMIT",
		"DESC TABLE STARTS",
		"DESC TABLE FROM",
		"DESC VIEW WITH",
		"DESCRIBE STAGE SHOW",
		"DESCRIBE TABLE DESCRIBE",
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
}

// TestValidateShow_InClauseDotsInsideQuotes tests that the IN clause correctly
// handles quoted identifiers containing embedded dots (not treated as path separators).
func TestValidateShow_InClauseDotsInsideQuotes(t *testing.T) {
	validCases := []string{
		// Single quoted identifier with dot — one-part path.
		`SHOW TABLES IN "db.schema"`,
		// Explicit scope + quoted identifier with dot.
		`SHOW TABLES IN DATABASE "my.db"`,
		`SHOW TABLES IN SCHEMA "my.schema"`,
		// Multi-part path where each part contains a dot inside quotes.
		`SHOW TABLES IN "my.db"."my.schema"`,
		// Explicit scope + multi-part path with dots inside quotes.
		`SHOW TABLES IN DATABASE "prod.v2"`,
		`SHOW VIEWS IN SCHEMA "db.1"."schema.2"`,
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
}

// TestValidateDescribe_LineCommentStripping tests line comment stripping
// for DESCRIBE/DESC statements.
func TestValidateDescribe_LineCommentStripping(t *testing.T) {
	validCases := []string{
		// Line comment between DESC and object type.
		"DESC -- comment\nTABLE my_table",
		"DESCRIBE -- note\nVIEW my_view",
		// Line comment between type and name.
		"DESCRIBE TABLE -- which one?\nmy_table",
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

	// Invalid: line comment removes everything after DESC (single line).
	t.Run("line comment removes type and name", func(t *testing.T) {
		sql := "DESC -- TABLE my_table"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Errorf("Expected warnings for %q, got 0", sql)
			return
		}
		if !strings.Contains(warns[0].Message, "DESCRIBE requires an object type and name") {
			t.Errorf("Expected 'DESCRIBE requires an object type and name' warning, got: %v", warns[0].Message)
		}
	})
}

// TestValidateDescribe_IdentifierStartingWithUnderscore tests that identifiers
// beginning with an underscore (valid in Snowflake) are accepted.
func TestValidateDescribe_IdentifierStartingWithUnderscore(t *testing.T) {
	validCases := []string{
		"DESCRIBE TABLE _my_table",
		"DESC VIEW _internal._private._view",
		"DESCRIBE STAGE _stage",
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
}

// TestValidateShow_LimitIntegerOverflow tests that LIMIT with an integer
// value exceeding int64 range is correctly flagged by strconv.Atoi failure.
func TestValidateShow_LimitIntegerOverflow(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"LIMIT with integer overflow",
			"SHOW TABLES LIMIT 99999999999999999999",
			[]string{"LIMIT requires a positive integer, got '99999999999999999999'"},
		},
		{
			"LIMIT with max int64+1",
			"SHOW TABLES LIMIT 9223372036854775808",
			[]string{"LIMIT requires a positive integer, got '9223372036854775808'"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_FromWithoutLimit tests that FROM keyword appearing without
// a preceding LIMIT clause is flagged as an unexpected token.
func TestValidateShow_FromWithoutLimit(t *testing.T) {
	sql := "SHOW TABLES FROM 'something'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for FROM without LIMIT, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "Unexpected token 'FROM'") {
		t.Errorf("Expected 'Unexpected token' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_InAccountTrailingContent tests that non-clause content
// after IN ACCOUNT is flagged as unexpected.
func TestValidateShow_InAccountTrailingContent(t *testing.T) {
	sql := "SHOW TABLES IN ACCOUNT FOOBAR"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for trailing content after IN ACCOUNT, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "Unexpected token 'FOOBAR'") {
		t.Errorf("Expected 'Unexpected token' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_LikeWithSemicolonInString tests that semicolons inside
// LIKE string literals are not treated as statement terminators.
func TestValidateShow_LikeWithSemicolonInString(t *testing.T) {
	validCases := []string{
		"SHOW TABLES LIKE 'test;value'",
		"SHOW TABLES LIKE 'a;b;c'",
		"SHOW TABLES STARTS WITH 'prefix;x'",
		"SHOW TABLES LIMIT 10 FROM 'name;1'",
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
}

// TestValidateShow_TerseEligibleHistoryIneligible tests that when an object
// type is TERSE-eligible but not HISTORY-eligible, only the HISTORY error fires.
func TestValidateShow_TerseEligibleHistoryIneligible(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"TERSE STREAMS HISTORY (STREAMS is TERSE-eligible, not HISTORY-eligible)",
			"SHOW TERSE STREAMS HISTORY",
			[]string{"HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES"},
		},
		{
			"TERSE TABLES HISTORY (TABLES is TERSE-eligible, not HISTORY-eligible)",
			"SHOW TERSE TABLES HISTORY",
			[]string{"HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Errorf("Expected exactly 1 warning for %q, got %d: %v", tt.sql, len(warns), warns)
				return
			}
			if !strings.Contains(warns[0].Message, tt.wantMsgs[0]) {
				t.Errorf("Expected warning containing %q, got: %v", tt.wantMsgs[0], warns[0].Message)
			}
		})
	}
}

// TestValidateDescribe_FunctionTrailingContentIgnored tests that trailing
// content after a FUNCTION/PROCEDURE signature is NOT flagged (the validator
// returns early once a signature is detected).
func TestValidateDescribe_FunctionTrailingContentIgnored(t *testing.T) {
	validCases := []string{
		"DESCRIBE FUNCTION my_func(INT) EXTRA",
		"DESCRIBE PROCEDURE my_proc(VARCHAR) STUFF HERE",
		"DESC FUNCTION db.schema.fn(NUMBER, NUMBER) TRAILING",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q (FUNCTION trailing content is not validated), got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

// TestValidateDescribe_FourPartIdentifier tests that a four-part identifier
// produces a trailing content warning, because the object-name path is consumed
// at most three dot-separated parts (mirroring _identPath).
func TestValidateDescribe_FourPartIdentifier(t *testing.T) {
	sql := "DESCRIBE TABLE a.b.c.d"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for 4-part identifier (regex only matches 3 parts), got 0")
		return
	}
	// The regex matches "a.b.c", leaving ".d" as trailing content. Since "."
	// is not a valid start of a Fields word (it becomes ".D" after ToUpper),
	// verify the trailing check fires.
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "Unexpected token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'Unexpected token' warning for trailing '.d', got: %v", warns)
	}
}

// TestValidateShow_MultipleConsecutiveBlockComments tests that multiple
// consecutive block comments are correctly stripped.
func TestValidateShow_MultipleConsecutiveBlockComments(t *testing.T) {
	validCases := []string{
		"SHOW /* c1 */ /* c2 */ TABLES",
		"SHOW /* a */ /* b */ /* c */ VIEWS",
		"SHOW TABLES /* x */ /* y */ LIKE /* z */ '%test%'",
		"DESCRIBE /* c1 */ /* c2 */ TABLE my_table",
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
}

// TestValidateShow_LikeFollowedByExtraStringLiteral tests that a string
// literal appearing after a valid LIKE clause (but not part of any clause)
// is flagged as unexpected trailing content.
func TestValidateShow_LikeFollowedByExtraStringLiteral(t *testing.T) {
	sql := "SHOW TABLES LIKE '%a%' 'extra'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for trailing string literal after LIKE, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "Unexpected token") {
		t.Errorf("Expected 'Unexpected token' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_HistoryFollowedByTerse tests that HISTORY followed by
// TERSE (wrong position) is flagged as unexpected trailing content.
func TestValidateShow_HistoryFollowedByTerse(t *testing.T) {
	sql := "SHOW PIPES HISTORY TERSE"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for TERSE after HISTORY, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "Unexpected token 'TERSE'") {
		t.Errorf("Expected 'Unexpected token TERSE' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_InDatabaseTrailingNonClause tests that non-clause keywords
// after IN DATABASE <name> are flagged as unexpected.
func TestValidateShow_InDatabaseTrailingNonClause(t *testing.T) {
	sql := "SHOW TABLES IN DATABASE my_db FOOBAR"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Errorf("Expected warning for trailing 'FOOBAR' after IN DATABASE, got 0")
		return
	}
	if !strings.Contains(warns[0].Message, "Unexpected token 'FOOBAR'") {
		t.Errorf("Expected 'Unexpected token FOOBAR' warning, got: %v", warns[0].Message)
	}
}

// TestValidateShow_TerseAndHistoryBothEligible tests the combination of
// TERSE + HISTORY on a type that is eligible for both modifiers.
func TestValidateShow_TerseAndHistoryBothEligible(t *testing.T) {
	// REPLICATION DATABASES is neither TERSE-eligible nor HISTORY-eligible
	// at the same time in the current implementation. Check all TERSE-eligible
	// types against HISTORY-eligible types to see if any overlap.
	// Currently TERSE-eligible: TABLES, EXTERNAL TABLES, VIEWS, SCHEMAS,
	//   DATABASES, STAGES, STREAMS, USERS
	// HISTORY-eligible: PIPES, REPLICATION DATABASES
	// No overlap — so TERSE + HISTORY should always produce at least one error.

	// PIPES is HISTORY-eligible but not TERSE-eligible.
	t.Run("TERSE PIPES HISTORY (TERSE invalid)", func(t *testing.T) {
		sql := "SHOW TERSE PIPES HISTORY"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Errorf("Expected warning for %q, got 0", sql)
			return
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "TERSE is not valid for SHOW PIPES") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected TERSE invalid warning, got: %v", warns)
		}
	})

	// TABLES is TERSE-eligible but not HISTORY-eligible.
	// Already tested in main table, but confirm the exact warning count.
	t.Run("TERSE TABLES HISTORY (HISTORY invalid, exactly 1 warning)", func(t *testing.T) {
		sql := "SHOW TERSE TABLES HISTORY"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning for %q, got %d: %v", sql, len(warns), warns)
			return
		}
		if !strings.Contains(warns[0].Message, "HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES") {
			t.Errorf("Expected HISTORY invalid warning, got: %v", warns[0].Message)
		}
	})
}

// TestValidateShow_CarriageReturnWhitespace tests that carriage returns are
// handled as valid whitespace in full SHOW/DESCRIBE validation.
func TestValidateShow_CarriageReturnWhitespace(t *testing.T) {
	validCases := []string{
		"SHOW\rTABLES",
		"SHOW\r\nTABLES",
		"SHOW TABLES\rLIKE\r'%x%'",
		"DESCRIBE\rTABLE\rmy_table",
		"DESCRIBE\r\nTABLE\r\nmy_table",
		"DESC\rVIEW\rmy_view",
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
}

// TestValidateShow_NoClauseValidationAcceptsArbitraryContent tests that
// object types in showNoClauseValidation (GRANTS, FUTURE GRANTS, PARAMETERS)
// accept arbitrary trailing content without producing warnings.
func TestValidateShow_NoClauseValidationAcceptsArbitraryContent(t *testing.T) {
	validCases := []string{
		"SHOW GRANTS LIKE '%test%'",
		"SHOW GRANTS ON USER admin",
		"SHOW GRANTS TO USER admin",
		"SHOW GRANTS OF SHARE share_name",
		"SHOW FUTURE GRANTS IN DATABASE my_db LIKE '%test%'",
		"SHOW FUTURE GRANTS ON SCHEMA my_db.my_schema",
		"SHOW PARAMETERS LIKE '%timeout%'",
		"SHOW PARAMETERS IN ACCOUNT",
		"SHOW PARAMETERS FOR WAREHOUSE my_wh",
		"SHOW PARAMETERS FOOBAR BARBAZ", // arbitrary words accepted
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q (noClauseValidation type), got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

// TestValidateShow_MultipleEscapedQuotesInStringLiteral tests that string
// literals with multiple consecutive escaped single quotes are accepted.
func TestValidateShow_MultipleEscapedQuotesInStringLiteral(t *testing.T) {
	validCases := []string{
		"SHOW TABLES LIKE 'a''''b'",                // a''b
		"SHOW TABLES LIKE '''''test'''''",          // ''test''
		"SHOW TABLES STARTS WITH 'it''s a''thing'", // it's a'thing
		"SHOW TABLES LIMIT 10 FROM 'x''''y'",       // x''y
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
}

// TestValidateShow_EmptyLikeFollowedByClause tests that an empty LIKE pattern
// followed by another valid clause works correctly.
func TestValidateShow_EmptyLikeFollowedByClause(t *testing.T) {
	validCases := []string{
		"SHOW TABLES LIKE '' IN ACCOUNT",
		"SHOW TABLES LIKE '' STARTS WITH 'a'",
		"SHOW TABLES LIKE '' LIMIT 10",
		"SHOW TABLES LIKE '' IN DATABASE my_db STARTS WITH 'x' LIMIT 5",
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
}

// TestValidateDescribe_TrailingQuotedIdentifier tests the code path where
// trailing content after the matched identifier starts with a double quote.
// The validator skips the trailing-content error in this case (to handle
// escaped quotes within the identifier that the regex can't fully parse).
func TestValidateDescribe_TrailingQuotedIdentifier(t *testing.T) {
	// These should NOT produce a trailing-content warning because the
	// remaining text starts with '"' which the validator intentionally skips.
	validCases := []string{
		// Escaped double-quote inside a quoted identifier that the regex
		// matched partially — trailing starts with '"'.
		`DESCRIBE TABLE "my""table"`,
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
}

// TestValidateShow_TabInClausePositions tests that tab characters work as
// whitespace in various clause-parsing positions.
func TestValidateShow_TabInClausePositions(t *testing.T) {
	validCases := []string{
		"SHOW TABLES IN\tDATABASE",
		"SHOW TABLES IN\tDATABASE\tmy_db",
		"SHOW TABLES IN\tSCHEMA\tmy_db.my_schema",
		"SHOW TABLES STARTS\tWITH\t'test'",
		"SHOW TABLES LIMIT\t10",
		"SHOW TABLES LIMIT\t10\tFROM\t'name'",
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
}

// TestValidateDescribe_ResultTransactionVariousIDs tests DESCRIBE RESULT and
// DESCRIBE TRANSACTION with various ID formats to confirm the validator
// accepts any non-empty content after these special types.
func TestValidateDescribe_ResultTransactionVariousIDs(t *testing.T) {
	validCases := []string{
		"DESCRIBE RESULT ''",         // empty string literal
		"DESCRIBE RESULT 'a b c'",    // spaces in string
		"DESC RESULT 12345",          // numeric ID
		"DESCRIBE TRANSACTION 0",     // zero
		"DESC TRANSACTION 99999999",  // large number
		"DESCRIBE RESULT some_var",   // bare identifier
		"DESC RESULT 'it''s a test'", // escaped quotes in string
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
}

// TestValidateDescribe_AccountLevelThreePartName tests that account-level
// objects with three-part identifiers (db.schema.name) are correctly flagged.
func TestValidateDescribe_AccountLevelThreePartName(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"RESOURCE MONITOR with three-part name",
			"DESCRIBE RESOURCE MONITOR db.schema.my_rm",
			[]string{"RESOURCE MONITOR is an account-level object and should not be qualified"},
		},
		{
			"WAREHOUSE with three-part name",
			"DESCRIBE WAREHOUSE db.schema.my_wh",
			[]string{"WAREHOUSE is an account-level object and should not be qualified"},
		},
		{
			"USER with three-part name",
			"DESC USER db.schema.my_user",
			[]string{"USER is an account-level object and should not be qualified"},
		},
		{
			"SHARE with three-part name",
			"DESCRIBE SHARE a.b.c",
			[]string{"SHARE is an account-level object and should not be qualified"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_HistoryWithClauses tests that HISTORY-eligible types
// correctly combine with optional clauses (LIKE, IN, STARTS WITH, LIMIT).
func TestValidateShow_HistoryWithClauses(t *testing.T) {
	validCases := []string{
		"SHOW PIPES HISTORY LIKE '%prod%' IN DATABASE my_db",
		"SHOW PIPES HISTORY STARTS WITH 'pipe_' LIMIT 10",
		"SHOW REPLICATION DATABASES HISTORY LIKE '%test%' LIMIT 5 FROM 'x'",
		"SHOW PIPES HISTORY IN ACCOUNT",
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
}

// TestValidateDescribe_NonIdentifierAfterMultiWordType tests that
// non-identifier tokens (e.g. numbers, special chars) after multi-word
// object types produce the correct "Expected an object name" error.
func TestValidateDescribe_NonIdentifierAfterMultiWordType(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"EXTERNAL TABLE with number as name",
			"DESCRIBE EXTERNAL TABLE 123",
			[]string{"Expected an object name after DESCRIBE EXTERNAL TABLE"},
		},
		{
			"DYNAMIC TABLE with number as name",
			"DESC DYNAMIC TABLE 456",
			[]string{"Expected an object name after DESCRIBE DYNAMIC TABLE"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateDescribe_BareSignatureTypes tests that DESCRIBE FUNCTION and
// DESCRIBE PROCEDURE with no content at all (neither name nor signature)
// produce the "requires an object name" error rather than the signature error.
func TestValidateDescribe_BareSignatureTypes(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"DESCRIBE FUNCTION bare",
			"DESCRIBE FUNCTION",
			[]string{"DESCRIBE FUNCTION requires an object name"},
		},
		{
			"DESC FUNCTION bare",
			"DESC FUNCTION",
			[]string{"DESCRIBE FUNCTION requires an object name"},
		},
		{
			"DESCRIBE PROCEDURE bare",
			"DESCRIBE PROCEDURE",
			[]string{"DESCRIBE PROCEDURE requires an object name"},
		},
		{
			"DESC PROCEDURE bare",
			"DESC PROCEDURE",
			[]string{"DESCRIBE PROCEDURE requires an object name"},
		},
		{
			"DESCRIBE FUNCTION with trailing whitespace only",
			"DESCRIBE FUNCTION   ",
			[]string{"DESCRIBE FUNCTION requires an object name"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_ClauseKeywordsAsObjectType tests that clause keywords
// (LIKE, LIMIT, IN) used as the object type are flagged as unknown.
func TestValidateShow_ClauseKeywordsAsObjectType(t *testing.T) {
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"SHOW LIKE (clause keyword as object type)",
			"SHOW LIKE",
			[]string{"Unknown object type 'LIKE'"},
		},
		{
			"SHOW LIMIT (clause keyword as object type)",
			"SHOW LIMIT",
			[]string{"Unknown object type 'LIMIT'"},
		},
		{
			"SHOW IN (clause keyword as object type)",
			"SHOW IN",
			[]string{"Unknown object type 'IN'"},
		},
		{
			"SHOW FROM (clause keyword as object type)",
			"SHOW FROM",
			[]string{"Unknown object type 'FROM'"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tt.sql)
				return
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
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tt.sql, warns)
				}
			}
		})
	}
}

// TestValidateShow_TerseMultiWordWithClauses tests that TERSE-eligible
// multi-word object types correctly combine with optional clauses.
func TestValidateShow_TerseMultiWordWithClauses(t *testing.T) {
	validCases := []string{
		"SHOW TERSE EXTERNAL TABLES LIKE '%prod%'",
		"SHOW TERSE EXTERNAL TABLES IN DATABASE my_db",
		"SHOW TERSE EXTERNAL TABLES LIKE '%x%' IN SCHEMA my_schema LIMIT 10",
		"SHOW TERSE VIEWS STARTS WITH 'v_' LIMIT 5 FROM 'view_abc'",
		"SHOW TERSE SCHEMAS IN DATABASE my_db LIKE '%test%'",
		"SHOW TERSE DATABASES LIMIT 100",
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
}

// TestValidateShow_InQuotedIdentFollowedByClause tests that a quoted identifier
// in the IN scope is consumed correctly and subsequent clauses are still parsed.
func TestValidateShow_InQuotedIdentFollowedByClause(t *testing.T) {
	validCases := []string{
		`SHOW TABLES IN DATABASE "my_db" LIKE '%x%'`,
		`SHOW TABLES IN SCHEMA "my.schema" STARTS WITH 'a'`,
		`SHOW TABLES IN "my_schema" LIMIT 10`,
		`SHOW TABLES IN DATABASE "special-db" LIKE '%test%' LIMIT 5`,
		`SHOW VIEWS IN SCHEMA "db.1"."schema.2" LIKE '%v%' LIMIT 20`,
		`SHOW TABLES IN "MY SCHEMA" STARTS WITH 'prefix' LIMIT 10 FROM 'x'`,
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
}

// ── Tag Tests ────────────────────────────────────────────────────────────────
