package sqleditor

import (
	"strings"
	"testing"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func getWarnings(markers []DiagMarker) []DiagMarker {
	var res []DiagMarker
	for _, m := range markers {
		if m.Severity == 4 {
			res = append(res, m)
		}
	}
	return res
}

func getErrors(markers []DiagMarker) []DiagMarker {
	var res []DiagMarker
	for _, m := range markers {
		if m.Severity == 8 {
			res = append(res, m)
		}
	}
	return res
}

// ── 1. ValidateSnowflakePatterns Tests ────────────────────────────────────────

func TestValidateSnowflakePatterns_ValidQueries(t *testing.T) {
	validQueries := []string{
		// Basic valid statements
		"SELECT 1",
		"SELECT a, b FROM t WHERE c = 1",
		"WITH cte AS (SELECT 1 AS x) SELECT x FROM cte",
		"SELECT * FROM t QUALIFY ROW_NUMBER() OVER (ORDER BY a) = 1",
		"INSERT INTO t SELECT a, b FROM s",
		// Snowflake Databases & Schemas
		"CREATE TRANSIENT DATABASE my_db",
		"CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 90",
		"CREATE TRANSIENT SCHEMA my_sch",
		// Snowflake Views
		"CREATE VIEW v AS SELECT 1 FROM t",
		"CREATE OR REPLACE SECURE VIEW v AS SELECT 1 FROM t",
		"CREATE MATERIALIZED VIEW mv AS SELECT 1 FROM t",
		// Snowflake Dynamic Tables
		"CREATE DYNAMIC TABLE dt TARGET_LAG = '1 minute' WAREHOUSE = wh AS SELECT 1 FROM t",
		// Sequences
		"CREATE SEQUENCE my_seq START WITH 1",
		"ALTER SEQUENCE my_seq INCREMENT = 10",
		"DROP SEQUENCE IF EXISTS my_seq CASCADE",
		// Tables
		"CREATE TABLE IF NOT EXISTS my_database.public.basic_employees (emp_id NUMBER)",
		"CREATE LOCAL TEMP TABLE t (id INT, name VARCHAR)",
		"CREATE TABLE t (id INT) DATA_RETENTION_TIME_IN_DAYS = 7",
		// Drop
		"DROP DATABASE my_db CASCADE",
		"DROP SCHEMA IF EXISTS my_sch RESTRICT",
		// False Positive Guards (Should be silently ignored, 0 warnings)
		"DELETE FROM t WHERE id = 1",
		"GRANT SELECT ON t TO ROLE r",
		"CREATE STAGE s",
		"ALTER WAREHOUSE wh RESUME",
		"SELECT * FROM t TABLESAMPLE (10 ROWS)",
		// Advanced Snowflake CREATE TABLE Syntax
		"CREATE TABLE t1 CLONE t2",
		"CREATE TABLE t1 LIKE t2",
		"CREATE TABLE t1 AS SELECT * FROM t2",
		"CREATE TABLE t1 USING TEMPLATE (SELECT * FROM t2)",
		"CREATE TABLE t1 FROM BACKUP SET 'backup_id'",
		"CREATE TABLE t1 (id INT) CLUSTER BY (id) ENABLE_SCHEMA_EVOLUTION = TRUE ROW_ACCESS_POLICY p1 ON (id)",
		"CREATE TRANSIENT TABLE t (id INT) DATA_RETENTION_TIME_IN_DAYS = 1",
		"CREATE TABLE t CLONE s AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		"CREATE TABLE t (id INT) COMMENT = 'my table' TAG (tag1 = 'val1')",
		"CREATE OR ALTER TABLE t (id INT, val VARCHAR)",
		// Integrations
		"CREATE STORAGE INTEGRATION my_storage_int TYPE=EXTERNAL_STAGE STORAGE_PROVIDER='S3' ENABLED=TRUE STORAGE_AWS_ROLE_ARN='arn:aws:iam::123456789012:role/my_role' STORAGE_ALLOWED_LOCATIONS=('s3://my-bucket/')",
		"CREATE STAGE my_s3_stage URL='s3://bucket/' STORAGE_INTEGRATION=s3_int DIRECTORY=(ENABLE=TRUE)",
		"CREATE WAREHOUSE my_wh WAREHOUSE_SIZE='X-LARGE' WAREHOUSE_TYPE='STANDARD' AUTO_SUSPEND=300",
		// MERGE statements
		"MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.val = s.val WHEN NOT MATCHED THEN INSERT (id, val) VALUES (s.id, s.val)",
		"MERGE INTO t USING (SELECT * FROM s) AS src ON t.id = src.id WHEN MATCHED AND t.v <> src.v THEN UPDATE SET v = src.v WHEN MATCHED THEN DELETE WHEN NOT MATCHED THEN INSERT ALL BY NAME",
		"MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE ALL BY NAME",
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 30)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

func TestValidateSnowflakePatterns_InvalidQueries(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		// Custom Anti-Patterns
		{"Typo LATERALFLATTEN", "SELECT * FROM t, LATERALFLATTEN(input => doc)", "LATERAL FLATTEN"},
		{"FLATTEN missing LATERAL", "SELECT * FROM raw_events, FLATTEN(input => doc)", "requires LATERAL"},
		{"QUALIFY ordering", "SELECT id FROM t ORDER BY id QUALIFY ROW_NUMBER() OVER(ORDER BY id) = 1", "after 'WHERE' or 'HAVING'"},
		{"Variant Path Colon", "SELECT payload.metadata.source FROM t", "Missing colon for variant path"},

		// Invalid Preambles
		{"Invalid DB", "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS 10", "Unexpected syntax"}, // Missing =
		{"Invalid Schema", "CREATE SCHEMA my_sch WITH MANAGED ACCESS = TRUE", "Unexpected syntax"},
		{"Invalid View", "CREATE VIEW v SELECT 1", "Unexpected syntax"}, // Missing AS
		{"Invalid Mat View", "CREATE MATERIALIZED VIEW mv SELECT 1", "Unexpected syntax"},
		{"Invalid Dynamic Table", "CREATE DYNAMIC TABLE dt AS SELECT 1", "Unexpected syntax"}, // Missing TARGET_LAG / WAREHOUSE
		{"Invalid Drop DB", "DROP DATABASE my_db CASCADE RESTRICT", "Unexpected syntax"},      // Conflicting modifiers
		{"Invalid Sequence", "CREATE SEQUENCE my_seq START WITH 'abc'", "Unexpected syntax"},
		{"Invalid Table", "CREATE TRANSIENT OR REPLACE TABLE foo (id INT)", "Unexpected syntax"}, // Wrong modifier order
		{"Table Replace IF NOT EXISTS", "CREATE OR REPLACE TABLE foo IF NOT EXISTS (id INT)", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Table CLUSTER BY no parens", "CREATE TABLE foo (id INT) CLUSTER BY id", "Unexpected syntax"},
		{"Table Retention invalid", "CREATE TABLE foo (id INT) DATA_RETENTION_TIME_IN_DAYS = 'abc'", "Unexpected syntax"},

		// Invalid MERGE
		{"MERGE INSERT in MATCHED", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN INSERT (id) VALUES (s.id)", "not allowed in WHEN MATCHED"},
		{"MERGE UPDATE in NOT MATCHED", "MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED THEN UPDATE SET val = s.val", "not allowed in WHEN NOT MATCHED"},
		{"MERGE NOT MATCHED BY SOURCE", "MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED BY SOURCE THEN DELETE", "not supported by Snowflake"},

		// Invalid Integrations
		{"Integration with prefix", "CREATE STORAGE INTEGRATION MY_DB.PUBLIC.MY_INT TYPE=EXTERNAL_STAGE STORAGE_PROVIDER='S3' ENABLED=TRUE STORAGE_AWS_ROLE_ARN='arn:aws:iam::123456789012:role/bad_role' STORAGE_ALLOWED_LOCATIONS=('s3://bad-bucket/')", "account-level objects"},
		{"API Integration missing provider", "CREATE API INTEGRATION bad_api_int API_AWS_ROLE_ARN='arn:aws:iam::123456789012:role/snowflake_api_role' API_ALLOWED_PREFIXES=('https://xyz.execute-api.us-west-2.amazonaws.com/prod/') ENABLED=TRUE", "Missing required parameter API_PROVIDER"},
		{"Notification Integration invalid type", "CREATE NOTIFICATION INTEGRATION bad_notify_int TYPE=WEBHOOK ENABLED=TRUE WEBHOOK_URL='https://my-slack-webhook.com'", "Invalid TYPE for Notification Integration"},
		{"Security Integration missing type", "CREATE SECURITY INTEGRATION bad_sec_int ENABLED=TRUE OAUTH_CLIENT=CUSTOM OAUTH_CLIENT_TYPE='CONFIDENTIAL'", "Missing required parameter TYPE"},
		{"External Access Integration invalid property", "CREATE EXTERNAL ACCESS INTEGRATION bad_ext_access ALLOWED_NETWORK_RULES=(github_api_network_rule) MAX_RETRIES=5 ENABLED=TRUE", "Unexpected property 'MAX_RETRIES'"},

		// Account-level prefix checks
		{"Warehouse with prefix", "CREATE WAREHOUSE MY_DB.PUBLIC.BAD_WH WITH WAREHOUSE_SIZE = 'SMALL'", "cannot have a database or schema prefix"},
		{"Role with prefix", "CREATE ROLE MY_DB.PUBLIC.BAD_ROLE", "cannot have a database or schema prefix"},
		{"User with prefix", "CREATE USER MY_DB.PUBLIC.BAD_USER PASSWORD='abc'", "cannot have a database or schema prefix"},

		// Property validation
		{"Warehouse invalid param", "CREATE WAREHOUSE broken_wh WITH WAREHOUSE_SIZE='MEDIUM' AUTO_SHUTDOWN=600", "Unexpected property 'AUTO_SHUTDOWN'"},
		{"Resource Monitor invalid param", "CREATE RESOURCE MONITOR bad_rm WITH MAX_CREDITS=500", "Unexpected property 'MAX_CREDITS'"},
		{"Stage invalid param", "CREATE STAGE my_stage BUCKET_URL='s3://bucket/'", "Unexpected property 'BUCKET_URL'"},
		{"Task invalid param", "CREATE TASK my_task WAREHOUSE=WH SCHEDULE='10 MINUTE' RETRY_LIMIT=5 AS SELECT 1", "Unexpected property 'RETRY_LIMIT'"},
		{"User invalid param", "CREATE USER bad_user IS_ACTIVE=TRUE", "Unexpected property 'IS_ACTIVE'"},
		{"User with Warehouse param", "CREATE USER bad_user WAREHOUSE_SIZE='SMALL'", "Unexpected property 'WAREHOUSE_SIZE'"},

		// Other syntax
		{"Grant role to table", "GRANT ROLE my_role TO TABLE my_table", "Unexpected syntax"},
		{"Masking policy missing returns", "CREATE MASKING POLICY bad_mask AS (val string) -> CASE WHEN 1=1 THEN val END", "Missing RETURNS clause"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)
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
		})
	}
}

// ── 2. ValidateBareColumnRefs Tests ───────────────────────────────────────────

func getTestColCaches() []ColEntry {
	return []ColEntry{
		{
			DB: "DB", Schema: "SCH", Name: "EMPLOYEES",
			Cols: []ColInfo{
				{Name: "ID", DataType: "TEXT"},
				{Name: "FIRST_NAME", DataType: "TEXT"},
				{Name: "LAST_NAME", DataType: "TEXT"},
				{Name: "DEPT_ID", DataType: "TEXT"},
				{Name: "SALARY", DataType: "TEXT"},
			},
		},
		{
			DB: "DB", Schema: "SCH", Name: "DEPARTMENTS",
			Cols: []ColInfo{
				{Name: "DEPT_ID", DataType: "TEXT"},
				{Name: "DEPT_NAME", DataType: "TEXT"},
				{Name: "MANAGER_ID", DataType: "TEXT"},
			},
		},
	}
}

func getTestRefs() []ResolvedRef {
	return []ResolvedRef{
		{Alias: "e", DB: "DB", Schema: "SCH", Name: "EMPLOYEES"},
		{Alias: "EMPLOYEES", DB: "DB", Schema: "SCH", Name: "EMPLOYEES"},
		{Alias: "d", DB: "DB", Schema: "SCH", Name: "DEPARTMENTS"},
	}
}

func TestValidateBareColumnRefs_Valid(t *testing.T) {
	validQueries := []string{
		// Standard
		`SELECT "ID", "FIRST_NAME", "LAST_NAME" FROM "DB"."SCH"."EMPLOYEES"`,
		"SELECT ID, FIRST_NAME FROM DB.SCH.EMPLOYEES e",
		`SELECT * FROM "DB"."SCH"."EMPLOYEES"`,
		// Case insensitivity inside quotes
		`SELECT "first_name", salary FROM "DB"."SCH"."EMPLOYEES"`,
		// Aliased
		"SELECT e.ID, e.FIRST_NAME FROM DB.SCH.EMPLOYEES e",
		// Expressions & Functions
		"SELECT COUNT(ID), MAX(SALARY) FROM DB.SCH.EMPLOYEES e",
		"SELECT FIRST_NAME AS fn FROM DB.SCH.EMPLOYEES e",
		// Joins
		"SELECT ID, DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID",
		// Script pre-pass
		"CREATE TABLE local_tab (amount NUMBER);\nSELECT amount FROM local_tab;",
		// Inserts
		"CREATE TABLE my_table (a varchar);\nINSERT INTO my_table (a) SELECT '1';",
		// Views
		`CREATE VIEW my_view AS SELECT FIRST_NAME, LAST_NAME FROM "DB"."SCH"."EMPLOYEES"`,
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 30)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

func TestValidateBareColumnRefs_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		missingCols []string
	}{
		{"Bare unknown", `SELECT wrong_col FROM "DB"."SCH"."EMPLOYEES"`, []string{"wrong_col"}},
		{"Quoted unknown", `SELECT "WRONG_COL" FROM "DB"."SCH"."EMPLOYEES"`, []string{"WRONG_COL"}},
		{"Multiple unknown", `SELECT wrong1, "WRONG2", FIRST_NAME FROM "DB"."SCH"."EMPLOYEES"`, []string{"wrong1", "WRONG2"}},
		{"Inside functions", `SELECT MAX(bad_col) FROM "DB"."SCH"."EMPLOYEES"`, []string{"bad_col"}},
		{"Inside math", `SELECT (ID * bad_col) + (SALARY / 100) FROM "DB"."SCH"."EMPLOYEES"`, []string{"bad_col"}},
		{"JOIN unknown", "SELECT ID, no_such_col FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID", []string{"no_such_col"}},
		{"Script pre-pass unknown", "CREATE TABLE local_tab (amount NUMBER);\nSELECT amountdd FROM local_tab;", []string{"amountdd"}},
		{"INSERT target unknown", `INSERT INTO "DB"."SCH"."EMPLOYEES" (ID, FAKE_COL) SELECT 1, 2;`, []string{"FAKE_COL"}},
		{"CREATE VIEW unknown", `CREATE OR REPLACE VIEW my_view AS SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"`, []string{"bad_col"}},
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req.SQL = tt.sql
			req.StmtRanges = GetStatementRanges(tt.sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)

			if len(warnings) != len(tt.missingCols) {
				t.Fatalf("Expected %d warnings, got %d. Markers: %v", len(tt.missingCols), len(warnings), warnings)
			}

			for _, expectedCol := range tt.missingCols {
				found := false
				for _, w := range warnings {
					if strings.Contains(strings.ToLower(w.Message), strings.ToLower(expectedCol)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find warning about column %q, but didn't. Warnings: %v", expectedCol, warnings)
				}
			}
		})
	}
}

// ── 3. ValidateTablesExist Tests ──────────────────────────────────────────────

func getLiveRefs() []ResolvedRef {
	return []ResolvedRef{
		{Alias: "l", DB: "DB", Schema: "SCH", Name: "LIVE_TABLE"},
	}
}

func TestValidateTablesExist_Valid(t *testing.T) {
	validQueries := []string{
		// Standard
		"SELECT * FROM LIVE_TABLE",
		"SELECT * FROM DB.SCH.LIVE_TABLE",
		// CTEs
		"WITH my_cte AS (SELECT 1 AS id) SELECT * FROM my_cte",
		// Pre-pass tracking
		"CREATE TEMPORARY TABLE local_tab AS SELECT 1;\nSELECT * FROM local_tab;",
		"CREATE OR REPLACE VIEW my_view AS SELECT 1;\nSELECT * FROM my_view;",
		"CREATE DATABASE local_db;\nCREATE SCHEMA local_db.local_sch;\nDROP SCHEMA local_db.local_sch;",
		// Identifiers inside comments
		"SELECT * FROM -- MISSING_TABLE \nLIVE_TABLE",
		// Context tracking
		"USE SCHEMA DB.SCH;\nCREATE TABLE test_1 (id INT);",
		// USE DATABASE & Context tracking
		"USE DATABASE DB;\nCREATE SCHEMA local_sch;\nCREATE TABLE local_sch.test_1 (id INT);",
		"USE DATABASE DB;\nUSE SCHEMA SCH;\nCREATE TABLE test_1 (id INT);",

		// UNDROP State Tracking (Drop then Undrop then Use)
		"CREATE TABLE local_t (id INT);\nDROP TABLE local_t;\nUNDROP TABLE local_t;\nSELECT * FROM local_t;",
		"CREATE DATABASE local_db;\nDROP DATABASE local_db;\nUNDROP DATABASE local_db;\nCREATE SCHEMA local_db.sch;",
		"CREATE DATABASE local_db;\nCREATE SCHEMA local_db.sch;\nDROP SCHEMA local_db.sch;\nUNDROP SCHEMA local_db.sch;\nCREATE TABLE local_db.sch.t1 (id INT);",

		// USE bare two-part: db.schema (no keyword) with known db and schema
		"use GOVERNANCE.public;",
		"use GOVERNANCE.public;\nCREATE TABLE test_1 (id INT);",

		// MERGE statements
		"MERGE INTO LIVE_TABLE USING (SELECT 1) AS s ON 1=1 WHEN MATCHED THEN UPDATE SET a=1",
		"CREATE TABLE local_t (id INT);\nMERGE INTO local_t USING LIVE_TABLE AS s ON local_t.id = s.id WHEN NOT MATCHED THEN INSERT (id) VALUES (s.id)",
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:   getLiveRefs(),
		KnownDatabases: []string{"DB", "GOVERNANCE"},
		KnownSchemas:   []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "GOVERNANCE", Name: "PUBLIC"}},
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 30)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateTablesExist(req)
			errs := getErrors(markers)
			if len(errs) > 0 {
				t.Errorf("Expected 0 errors for %q, got %d: %v", sql, len(errs), errs)
			}
		})
	}
}

func TestValidateTablesExist_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		{"Missing Table", "SELECT * FROM MISSING_TABLE", "MISSING_TABLE"},
		{"Missing Table in Join", "SELECT * FROM LIVE_TABLE JOIN NOPE_TABLE ON a=b", "NOPE_TABLE"},
		{"CTE Leak", "WITH my_cte AS (SELECT 1) SELECT * FROM my_cte;\nSELECT * FROM my_cte;", "my_cte"},
		{"Missing Table in Alter", "ALTER TABLE existing_table ADD COLUMN id INT", "EXISTING_TABLE"},
		{"Wrong DB in path", "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE", "WRONG_DB"},
		{"Wrong Schema in path", "SELECT * FROM DB.WRONG_SCH.LIVE_TABLE", "WRONG_SCH"},
		{"Comment Bypass", "SELECT * FROM MISSING_TABLE -- LIVE_TABLE", "MISSING_TABLE"},
		{"Missing Table in View", "CREATE VIEW my_view AS SELECT * FROM MISSING_TABLE;", "MISSING_TABLE"},
		{"Missing DB in CREATE", "CREATE SCHEMA missing_db.missing_sch;", "MISSING_DB"},
		// Dropped Entity tracking (Using an object after it is dropped)
		{"Query Dropped Table", "CREATE TABLE local_t (id INT);\nDROP TABLE local_t;\nSELECT * FROM local_t;", "local_t"},
		{"Create in Dropped Database", "CREATE DATABASE local_db;\nDROP DATABASE local_db;\nCREATE SCHEMA local_db.sch;", "local_db"},
		{"Create in Dropped Schema", "CREATE DATABASE db1;\nCREATE SCHEMA db1.sch;\nDROP SCHEMA db1.sch;\nCREATE TABLE db1.sch.t1 (id INT);", "db1.sch"},

		// UNDROP Invalid tracking (Undropping objects that were never dropped)
		{"Undrop Non-existent Table", "UNDROP TABLE never_existed;", "never_existed"},
		{"Undrop Non-existent Database", "UNDROP DATABASE never_existed;", "never_existed"},
		{"Undrop Non-existent Schema", "UNDROP SCHEMA never_existed;", "never_existed"},

		// USE statement — unknown database or schema
		{"USE unknown DB two-part bare", "use database_that_not_exists.PUBLIC;", "database_that_not_exists"},
		{"USE unknown DB bare one-part", "use database_that_not_exists", "database_that_not_exists"},
		{"USE known DB unknown schema", "use GOVERNANCE.schema_that_doesnt_exists;", "schema_that_doesnt_exists"},

		// MERGE missing tables
		{"MERGE Missing Target", "MERGE INTO NOPE_TABLE USING (SELECT 1) AS s ON 1=1 WHEN MATCHED THEN UPDATE SET a=1", "NOPE_TABLE"},
		{"MERGE Missing Source", "MERGE INTO LIVE_TABLE USING NOPE_SOURCE ON 1=1 WHEN MATCHED THEN UPDATE SET a=1", "NOPE_SOURCE"},

		// CREATE TABLE missing sources
		{"CREATE TABLE CLONE missing", "CREATE TABLE t CLONE NOPE_TABLE", "NOPE_TABLE"},
		{"CREATE TABLE LIKE missing", "CREATE TABLE t LIKE NOPE_TABLE", "NOPE_TABLE"},
		{"CREATE TABLE AS SELECT missing", "CREATE TABLE t AS SELECT * FROM NOPE_TABLE", "NOPE_TABLE"},
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:   getLiveRefs(),
		KnownDatabases: []string{"DB", "GOVERNANCE"},
		KnownSchemas:   []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "GOVERNANCE", Name: "PUBLIC"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req.SQL = tt.sql
			req.StmtRanges = GetStatementRanges(tt.sql)
			markers := ValidateTablesExist(req)
			errs := getErrors(markers)

			if len(errs) == 0 {
				t.Fatalf("Expected errors for %q, got 0", tt.sql)
			}

			found := false
			for _, e := range errs {
				if strings.Contains(strings.ToLower(e.Message), strings.ToLower(tt.expectedMatch)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected error matching %q, got: %v", tt.expectedMatch, errs[0].Message)
			}
		})
	}
}

// ── 4. ValidateSyntax Tests (Tokenization & Scripting) ────────────────────────

func TestValidateSyntax_Scripting(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedError string // Empty string means we expect 0 errors
	}{
		{
			name: "EXECUTE IMMEDIATE with RETURN TABLE (temp.sql)",
			sql: `
execute immediate $$
  declare
    -- variable and cursor declarations
    target_status varchar default 'ACTIVE';
    min_revenue number default 50000;
    res resultset;
  begin
    -- Snowflake Scripting and sql statements
    res := (
        select region, sum(revenue) as total_revenue
        from regional_sales
        where account_status = :target_status
        group by region
        having sum(revenue) >= :min_revenue
    );
  return table(res);
  end;
$$;
			`,
			expectedError: "", // Should be perfectly valid, no "Variable 'TABLE' is not declared"
		},
		{
			name: "Valid DECLARE with type annotations",
			sql: `
execute immediate $$
  declare
    my_str varchar(100);
    my_num number(10, 2) default 0;
  begin
    my_num := 10;
  end;
$$;
			`,
			expectedError: "",
		},
		{
			name: "Undeclared variable returned",
			sql: `
execute immediate $$
  begin
    return undeclared_var;
  end;
$$;
			`,
			expectedError: "Variable 'undeclared_var' is not declared",
		},
		{
			name: "Undeclared variable assigned",
			sql: `
execute immediate $$
  begin
    undeclared_var := 1;
  end;
$$;
			`,
			expectedError: "Variable 'undeclared_var' is not declared",
		},
		{
			name: "Missing expression after assignment",
			sql: `
execute immediate $$
  declare
    my_var number;
  begin
    my_var := ;
  end;
$$;
			`,
			expectedError: "Missing expression after assignment",
		},
		{
			name: "Incorrect assignment operator",
			sql: `
execute immediate $$
  declare
    my_var number;
  begin
    my_var = 10;
  end;
$$;
			`,
			expectedError: "Expected ':=' for assignment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ValidateSyntax operates directly on the raw SQL string
			markers := ValidateSyntax(tt.sql)
			errs := getErrors(markers)

			if tt.expectedError == "" {
				if len(errs) > 0 {
					t.Errorf("Expected 0 errors for %q, got %d: %v", tt.name, len(errs), errs)
				}
			} else {
				if len(errs) == 0 {
					t.Fatalf("Expected error containing %q, but got 0 errors", tt.expectedError)
				}

				found := false
				for _, e := range errs {
					if strings.Contains(strings.ToLower(e.Message), strings.ToLower(tt.expectedError)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error matching %q, but got: %v", tt.expectedError, errs[0].Message)
				}
			}
		})
	}
}

// ── 5. ValidateDataTypes Tests ────────────────────────────────────────────────

func TestValidateDataTypes(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedError string // Empty string means we expect 0 errors
	}{
		{
			name: "Valid datatypes in CREATE TABLE",
			sql: `CREATE TABLE my_table (
				id NUMBER,
				name VARCHAR(255),
				is_active BOOLEAN,
				created_at TIMESTAMP_LTZ
			);`,
			expectedError: "",
		},
		{
			name: "Invalid datatype in CREATE TABLE",
			sql: `
USE GOVERNANCE.PUBLIC;
create table my_table (
  my_codffsf varchard
);`,
			expectedError: "Unknown data type 'VARCHARD'",
		},
		{
			name: "Invalid datatype after USE, comment in column list, no trailing semicolon",
			sql: `use GOVERNANCE.public;

create table my_table (
  -- Should complain about incorrect datatype
  my_codffsf varchardc
)`,
			expectedError: "Unknown data type 'VARCHARDC'",
		},
		{
			name:          "Invalid datatype in ALTER TABLE",
			sql:           `ALTER TABLE my_table ADD COLUMN new_col NUMBR;`,
			expectedError: "Unknown data type 'NUMBR'",
		},
		{
			name:          "Invalid datatype in CAST function",
			sql:           `SELECT CAST(id AS INTT) FROM my_table;`,
			expectedError: "Unknown data type 'INTT'",
		},
		{
			name:          "Invalid datatype in shorthand cast (::)",
			sql:           `SELECT id::FLOT FROM my_table;`,
			expectedError: "Unknown data type 'FLOT'",
		},
		{
			name:          "Valid parameterized datatype",
			sql:           `CREATE TABLE t (price NUMBER(10, 2));`,
			expectedError: "",
		},
		{
			name:          "Valid array/object types",
			sql:           `CREATE TABLE t (tags ARRAY, metadata OBJECT);`,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)

			// NOTE: You will need to implement ValidateDataTypes in sqleditor.go
			// or patterns.go for these tests to pass!
			markers := ValidateDataTypes(tt.sql, ranges)

			// Assuming we treat unknown data types as warnings (severity 4)
			errs := getWarnings(markers)

			if tt.expectedError == "" {
				if len(errs) > 0 {
					t.Errorf("Expected 0 errors for %q, got %d: %v", tt.name, len(errs), errs)
				}
			} else {
				if len(errs) == 0 {
					t.Fatalf("Expected error containing %q, but got 0 errors", tt.expectedError)
				}

				found := false
				for _, e := range errs {
					if strings.Contains(strings.ToLower(e.Message), strings.ToLower(tt.expectedError)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error matching %q, but got: %v", tt.expectedError, errs[0].Message)
				}
			}
		})
	}
}

// ── 6. ValidateTablesExist — 3-part CREATE TABLE false-alarm regression ───────

// Regression: a CREATE TABLE with a fully-qualified 3-part identifier
// (DB.SCH.TABLE) must never produce false-alarm errors regardless of whether
// the database or schema appears in KnownDatabases / KnownSchemas, because the
// fully-qualified path is self-sufficient and requires no session context.
func TestValidateTablesExist_ThreePartCreateTable_NoFalseAlarms(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		req  ValidateTablesExistRequest
	}{
		{
			// Exact reproduction of the reported bug: long random-looking names
			// that don't appear in the empty known lists.
			name: "long random names, empty known lists",
			sql: `create or replace TABLE RAND_DB_7F42E14F3D1E4268BEA3249D68FCCEC6.RAND_SCH_10.OBJ_0CA0A246E2574193A2E18CF1FB92CE94 (
				ID NUMBER(38,0)
			);`,
			req: ValidateTablesExistRequest{
				KnownDatabases: []string{},
				KnownSchemas:   []SchemaEntry{},
				ResolvedRefs:   []ResolvedRef{},
			},
		},
		{
			// DB is known but no schemas are loaded for it — this was the exact
			// false alarm: "Schema 'RAND_DB_....RAND_SCH_10' does not exist or
			// is not authorized." even though the schema does exist in Snowflake.
			name: "DB known, no schemas loaded for it",
			sql: `create or replace TABLE RAND_DB_7F42E14F3D1E4268BEA3249D68FCCEC6.RAND_SCH_10.OBJ_0CA0A246E2574193A2E18CF1FB92CE94 (
				ID NUMBER(38,0)
			);`,
			req: ValidateTablesExistRequest{
				KnownDatabases: []string{"RAND_DB_7F42E14F3D1E4268BEA3249D68FCCEC6"},
				KnownSchemas:   []SchemaEntry{},
				ResolvedRefs:   []ResolvedRef{},
			},
		},
		{
			// Simple names; same logic should hold.
			name: "simple 3-part name, no session context",
			sql:  `CREATE TABLE mydb.myschema.mytable (id INT);`,
			req: ValidateTablesExistRequest{
				KnownDatabases: []string{},
				KnownSchemas:   []SchemaEntry{},
				ResolvedRefs:   []ResolvedRef{},
			},
		},
		{
			// DB is known, schemas for OTHER databases are loaded, but none for
			// this specific DB — must not produce a false schema error.
			// Note: unquoted identifiers are normalised to uppercase internally.
			name: "DB known, schemas loaded only for other DBs",
			sql:  `CREATE OR REPLACE TABLE MYDB.MYSCHEMA.MYTABLE (id INT);`,
			req: ValidateTablesExistRequest{
				KnownDatabases: []string{"MYDB", "OTHERDB"},
				KnownSchemas:   []SchemaEntry{{DB: "OTHERDB", Name: "PUBLIC"}},
				ResolvedRefs:   []ResolvedRef{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SQL = tt.sql
			tt.req.StmtRanges = GetStatementRanges(tt.sql)
			markers := ValidateTablesExist(tt.req)
			errs := getErrors(markers)
			if len(errs) > 0 {
				t.Errorf("Expected 0 errors for fully-qualified 3-part CREATE TABLE, got %d: %v", len(errs), errs)
			}
		})
	}
}

// ── Helpers for tests ─────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
