package sqleditor

import (
	"strings"
	"testing"
)

// ── 2. ValidateBareColumnRefs Tests ───────────────────────────────────────────

func TestValidateBareColumnRefs_Valid(t *testing.T) {
	validQueries := []string{
		// Standard
		`SELECT "ID", "FIRST_NAME", "LAST_NAME" FROM "DB"."SCH"."EMPLOYEES"`,
		"SELECT ID, FIRST_NAME FROM DB.SCH.EMPLOYEES e",
		`SELECT * FROM "DB"."SCH"."EMPLOYEES"`,
		// Case insensitivity inside quotes
		`SELECT "first_name", salary FROM "DB"."SCH"."EMPLOYEES"`,
		// Aliased — qualified refs with valid columns must not warn
		"SELECT e.ID, e.FIRST_NAME FROM DB.SCH.EMPLOYEES e",
		// Whitespace around the dot must still read as a qualified ref (token
		// adjacency, not byte adjacency) — issue #675. Old byte-based scan
		// treated `e` here as a bare unknown column.
		"SELECT e . FIRST_NAME FROM DB.SCH.EMPLOYEES e",
		"SELECT e.ID, d.DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID",
		// Local table via alias — valid qualified refs
		"CREATE TABLE local_t (id NUMBER, name VARCHAR);\nSELECT t.id, t.name FROM local_t t;",
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

		// Implicit (AS-less) column aliases must not be flagged (issue #713).
		"SELECT ID employee_id FROM DB.SCH.EMPLOYEES",
		"SELECT ID employee_id, FIRST_NAME fn FROM DB.SCH.EMPLOYEES",
		"SELECT COUNT(ID) cnt FROM DB.SCH.EMPLOYEES",

		// String literals containing identifier-like words must not be flagged
		// as unknown column refs (e.g. 'month' in DATE_TRUNC('month', ID)).
		`SELECT DATE_TRUNC('month', ID) AS m FROM DB.SCH.EMPLOYEES`,
		`SELECT TO_CHAR(ID, 'YYYY-MM-DD') AS d, FIRST_NAME FROM DB.SCH.EMPLOYEES`,
		// Regression tests for Issue: Date parts inside date functions triggering false warnings
		"SELECT DATEADD(month, -1, CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT DATE_TRUNC('month', CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT TIMESTAMPDIFF(second, CURRENT_DATE(), CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT EXTRACT(year FROM CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",

		// Regression: double-quoted column names + comments in CREATE TABLE must
		// not break the in-script column cache for subsequent INSERT/SELECT.
		"CREATE TABLE t1 (\n  \"CUSTOMER_ID\" INT,\n  FIRST_NAME VARCHAR\n);\nINSERT INTO t1 (\"CUSTOMER_ID\", FIRST_NAME) SELECT 1, 'a';",
		// Column after a line comment must still be cached.
		"CREATE TABLE t2 (\n  -- primary key\n  id INT,\n  name VARCHAR\n);\nINSERT INTO t2 (id, name) SELECT 1, 'a';",
		// Column after a block comment must still be cached.
		"CREATE TABLE t3 (\n  /* pk */ id INT,\n  name VARCHAR\n);\nINSERT INTO t3 (id, name) SELECT 1, 'a';",
		// Double-quoted column containing comma must be handled correctly.
		"CREATE TABLE t4 (\n  \"A,B\" INT,\n  COL2 INT\n);\nINSERT INTO t4 (\"A,B\", COL2) SELECT 1, 2;",
		// SELECT from a table whose columns are defined after comments.
		"CREATE TABLE t5 (\n  -- the id\n  customer_id INT,\n  full_name VARCHAR\n);\nSELECT customer_id, full_name FROM t5;",
		// Column with escaped double-quote in name (Snowflake uses "" to embed a literal ").
		"CREATE TABLE t6 (\n  \"col\"\"name\" INT,\n  other INT\n);\nINSERT INTO t6 (\"col\"\"name\", other) SELECT 1, 2;",
		// Column after a DEFAULT with escaped single-quote must still be cached.
		"CREATE TABLE t7 (\n  greeting VARCHAR DEFAULT 'it''s',\n  id INT\n);\nINSERT INTO t7 (greeting, id) SELECT 'hi', 1;",
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
		// Qualified refs (alias.column) — unknown column via Snowflake metadata
		{"Qualified alias unknown", `SELECT e.bad_col FROM DB.SCH.EMPLOYEES e`, []string{"bad_col"}},
		// Qualified refs (alias.column) — unknown column via local CREATE TABLE pre-scan
		{"Local table qualified unknown", "CREATE TABLE local_t (id NUMBER, name VARCHAR);\nSELECT t.wrong_col FROM local_t t;", []string{"wrong_col"}},
		// Regression: Ensure date parts are still validated if used as normal columns outside date functions
		{"Bare date part outside function", `SELECT month FROM "DB"."SCH"."EMPLOYEES"`, []string{"month"}},
		{"Date part as 2nd param", `SELECT DATEADD(day, month, CURRENT_DATE()) FROM "DB"."SCH"."EMPLOYEES"`, []string{"month"}},

		// Regression: columns after comments must be correctly cached; wrong columns must still be flagged.
		{"INSERT wrong col after comment in CREATE TABLE",
			"CREATE TABLE t1 (\n  -- primary key\n  id INT,\n  name VARCHAR\n);\nINSERT INTO t1 (id, WRONG_COL) SELECT 1, 'a';",
			[]string{"WRONG_COL"}},
		{"INSERT wrong col with quoted col in CREATE TABLE",
			"CREATE TABLE t1 (\n  \"CUSTOMER_ID\" INT,\n  FIRST_NAME VARCHAR\n);\nINSERT INTO t1 (\"CUSTOMER_ID\", FAKE_COL) SELECT 1, 'a';",
			[]string{"FAKE_COL"}},

		// Regression: bare reference to a case-sensitive quoted column must be flagged.
		// "customer_id" (quoted, lowercase) cannot be referenced as bare customer_id
		// because Snowflake normalizes bare identifiers to CUSTOMER_ID which ≠ customer_id.
		{"SELECT bare ref to quoted lowercase col",
			"CREATE OR REPLACE TABLE RAW_CUSTOMERS1 (\n  \"customer_id\" INT,\n  FIRST_NAME VARCHAR\n);\nCREATE OR REPLACE VIEW VW AS\nSELECT\n  customer_id,\n  FIRST_NAME\nFROM RAW_CUSTOMERS1;",
			[]string{"CUSTOMER_ID"}},
		{"SELECT bare ref to quoted lowercase col simple",
			"CREATE TABLE t1 (\n  \"customer_id\" INT,\n  name VARCHAR\n);\nSELECT customer_id, name FROM t1;",
			[]string{"CUSTOMER_ID"}},
		// User-reported scenario: CREATE TABLE + CREATE VIEW referencing quoted col with bare ident.
		{"CREATE VIEW bare ref to quoted lowercase col",
			"CREATE OR REPLACE TABLE RAW_CUSTOMERS1 (\n  \"customer_id\" INT,\n  FIRST_NAME VARCHAR,\n  LAST_NAME VARCHAR,\n  REGISTRATION_DATE DATE,\n  STATUS VARCHAR\n);\n\nCREATE OR REPLACE VIEW VW_CLEAN_CUSTOMERS AS\nSELECT\n  customer_id,\n  UPPER(FIRST_NAME || ' ' || LAST_NAME) AS FULL_NAME,\n  REGISTRATION_DATE\nFROM RAW_CUSTOMERS1\nWHERE STATUS = 'ACTIVE';",
			[]string{"CUSTOMER_ID"}},
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
		// CREATE OR ALTER must also register the table (PR #472 review finding 1).
		"CREATE OR ALTER TABLE local_oa (id INT);\nSELECT * FROM local_oa;",
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

		// Multi-CTE: all CTE names must be recognized, even those after the first comma
		"WITH cte1 AS (SELECT 1 AS id), cte2 AS (SELECT * FROM LIVE_TABLE) SELECT * FROM cte1 JOIN cte2 ON 1=1",
		"WITH a AS (SELECT 1), b AS (SELECT 2), c AS (SELECT 3) SELECT * FROM a JOIN b ON 1=1 JOIN c ON 1=1",

		// CREATE TASK — SCHEDULE with USING CRON must not flag CRON as a table (Issue #306).
		// Fully qualified with a cataloged DB.SCH so the schema-scoped name check passes.
		"CREATE OR REPLACE TASK DB.SCH.TASK_1\n\tWAREHOUSE=COMPUTE_WH\n\tSCHEDULE='USING CRON 0 0 * * * UTC'\n\tAS SELECT SYSTEM$WAIT(5)",
		"CREATE OR REPLACE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO LIVE_TABLE SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:    getLiveRefs(),
		KnownDatabases:  []string{"DB", "GOVERNANCE"},
		KnownSchemas:    []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "GOVERNANCE", Name: "PUBLIC"}},
		SessionDatabase: "DB",
		SessionSchema:   "SCH",
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

// Regression test for Issue: USE statements containing underscores failing to set context
func TestValidateTablesExist_UseWithUnderscores(t *testing.T) {
	sql := `use LINEAGE_SOURCE_DB.RAW_DATA;
SELECT * FROM GLOBAL_SHIPMENTS;`

	req := ValidateTablesExistRequest{
		ResolvedRefs: []ResolvedRef{
			// The frontend/parser will resolve this as a known table reference
			{Alias: "GLOBAL_SHIPMENTS", DB: "LINEAGE_SOURCE_DB", Schema: "RAW_DATA", Name: "GLOBAL_SHIPMENTS"},
		},
		KnownDatabases: []string{"LINEAGE_SOURCE_DB"},
		KnownSchemas:   []SchemaEntry{{DB: "LINEAGE_SOURCE_DB", Name: "RAW_DATA"}},
	}

	req.SQL = sql
	req.StmtRanges = GetStatementRanges(sql)

	markers := ValidateTablesExist(req)
	errs := getErrors(markers)

	if len(errs) > 0 {
		t.Errorf("Expected 0 errors for USE statement with underscores, got %d: %v", len(errs), errs)
	}
}

func TestValidateTablesExist_CaseSensitive(t *testing.T) {
	sql := `USE DATABASE DB; USE SCHEMA SCH;
SELECT * FROM "MixedCaseTable";
SELECT * FROM DB.SCH."MixedCaseTable";`

	req := ValidateTablesExistRequest{
		ResolvedRefs: []ResolvedRef{
			{DB: "DB", Schema: "SCH", Name: "MixedCaseTable"},
		},
		KnownDatabases:              []string{"DB"},
		KnownSchemas:                []SchemaEntry{{DB: "DB", Name: "SCH"}},
		QuotedIdentifiersIgnoreCase: false,
	}

	req.SQL = sql
	req.StmtRanges = GetStatementRanges(sql)

	markers := ValidateTablesExist(req)
	errs := getErrors(markers)
	if len(errs) > 0 {
		t.Errorf("Expected 0 errors for case-sensitive tables, got %d: %+v", len(errs), errs)
	}

	// Negative test: genuinely missing table still produces an error
	req2 := req
	req2.SQL = `SELECT * FROM DB.SCH."NonexistentTable";`
	req2.StmtRanges = GetStatementRanges(req2.SQL)
	markers2 := ValidateTablesExist(req2)
	if len(getErrors(markers2)) == 0 {
		t.Error("Expected an error for a table that does not exist, got none")
	}

	// Test QuotedIdentifiersIgnoreCase: true
	req3 := req
	req3.SQL = `SELECT * FROM "mixedcasetable";` // lowercase in SQL
	req3.StmtRanges = GetStatementRanges(req3.SQL)
	req3.QuotedIdentifiersIgnoreCase = true
	markers3 := ValidateTablesExist(req3)
	if len(getErrors(markers3)) > 0 {
		t.Errorf("Expected 0 errors with QuotedIdentifiersIgnoreCase=true, got %v", getErrors(markers3))
	}
}

func TestValidateTablesExist_MissingTables(t *testing.T) {
	sql := `SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA".this_table_does_not_exists;

SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA"."this_table_does_not_exists";`

	req := ValidateTablesExistRequest{
		SQL:        sql,
		StmtRanges: GetStatementRanges(sql),
		// Empty ResolvedRefs simulates the frontend correctly dropping missing tables
		// once the schema has been fetched.
		ResolvedRefs:   []ResolvedRef{},
		KnownDatabases: []string{"LINEAGE_SOURCE_DB"},
		KnownSchemas:   []SchemaEntry{{DB: "LINEAGE_SOURCE_DB", Name: "RAW_DATA"}},
	}
	markers := ValidateTablesExist(req)
	errs := getErrors(markers)
	if len(errs) != 2 {
		t.Errorf("Expected 2 errors for missing tables, got %d: %v", len(errs), errs)
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
		// Pipes
		{"Pipe missing target table", "CREATE PIPE p AS COPY INTO NOPE_TABLE FROM @s", "NOPE_TABLE"},
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

// ── 5. ValidateSemantics Tests ────────────────────────────────────────────────

// TestValidateSemantics_CTEAliasColumns verifies that column references via
// CTE aliases are validated against the CTE's projected columns even though
// CTEs are absent from resolvedRefs (the frontend drops them because they are
// not in the global Snowflake object store).
func TestValidateSemantics_CTEAliasColumns(t *testing.T) {
	// ── Valid cases: no warnings expected ─────────────────────────────────
	validCases := []struct {
		name string
		sql  string
	}{
		{
			name: "CTE alias valid columns",
			sql:  `WITH vip AS (SELECT customer_id, customer_name FROM t) SELECT vc.customer_id, vc.customer_name FROM vip vc`,
		},
		{
			name: "CTE used directly - valid columns",
			sql:  `WITH vip AS (SELECT id, name FROM t) SELECT vip.id, vip.name FROM vip`,
		},
		{
			name: "Multiple CTEs - all valid columns",
			sql:  `WITH a AS (SELECT x, y FROM t), b AS (SELECT p, q FROM s) SELECT a.x, b.p FROM a JOIN b ON a.x = b.p`,
		},
		{
			name: "CTE with AS-aliased expressions",
			sql:  `WITH summary AS (SELECT COUNT(*) AS cnt, SUM(amount) AS total FROM t) SELECT s.cnt, s.total FROM summary s`,
		},
		{
			// Issue #713: implicit (AS-less) projection alias in a CTE must be
			// resolvable in the outer query — no "cnt not found" / "does not
			// exist in C" markers.
			name: "CTE with implicit (AS-less) alias",
			sql:  `WITH c AS (SELECT ID, COUNT(*) cnt FROM t GROUP BY ID) SELECT c.cnt FROM c`,
		},
	}
	for _, tt := range validCases {
		t.Run(tt.name, func(t *testing.T) {
			markers := ValidateSemantics(tt.sql, nil, nil)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected no warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	// ── Invalid cases: warning for unknown column expected ─────────────────
	invalidCases := []struct {
		name    string
		sql     string
		wantCol string
	}{
		{
			name:    "CTE alias unknown column",
			sql:     `WITH vip AS (SELECT customer_id, customer_name FROM t) SELECT vc.customer_id, vc.bad_col FROM vip vc`,
			wantCol: "bad_col",
		},
		{
			name:    "CTE used directly - unknown column",
			sql:     `WITH vip AS (SELECT id, name FROM t) SELECT vip.bad_col FROM vip`,
			wantCol: "bad_col",
		},
		{
			name:    "Second CTE alias unknown column",
			sql:     `WITH a AS (SELECT x, y FROM t), b AS (SELECT p, q FROM s) SELECT a.x, b.wrong FROM a JOIN b ON a.x = b.p`,
			wantCol: "wrong",
		},
		{
			name: "Complex CTE projections",
			sql: `WITH Monthly_Sales_Summary AS (
				SELECT
					DATE_TRUNC('month', sale_date) AS sales_month,
					SUM(amount) AS total_revenue,
					COUNT(sale_id) AS total_transactions
				FROM BIG_SALES_DATA
				GROUP BY 1
			)
			SELECT mss.sales_month, mss.missing_col
			FROM Monthly_Sales_Summary mss`,
			wantCol: "missing_col",
		},
		{
			name: "Local table created in script",
			sql: `CREATE TABLE local_tab (col1 INT);
			SELECT t.col1, t.col2 FROM local_tab t;`,
			wantCol: "col2",
		},
		{
			name:    "Quoted CTE alias",
			sql:     `WITH "my_cte" AS (SELECT 1 AS x) SELECT "my_cte".y FROM "my_cte"`,
			wantCol: "y",
		},
		{
			name: "User reported failing query (Issue #73) - bare column",
			sql: `
use LINEAGE_SOURCE_DB.RAW_DATA;

CREATE OR REPLACE TABLE BIG_SALES_DATA (
    sale_id NUMBER,
    customer_id NUMBER,
    sale_date DATE,
    amount NUMBER(10,2),
    notes VARCHAR
) CLUSTER BY (sale_date);

CREATE OR REPLACE TABLE CUSTOMERS (
    customer_id NUMBER,
    customer_name VARCHAR,
    region VARCHAR
);

WITH Monthly_Sales_Summary AS (
    SELECT 
        DATE_TRUNC('month', sale_date) AS sales_month,
        SUM(amount) AS total_revenue,
        COUNT(sale_id) AS total_transactions
    FROM BIG_SALES_DATA
    GROUP BY DATE_TRUNC('month', sale_date)
),
VIP_Customers AS (
    SELECT 
        customer_id,
        -- In the next line it should complain about incorrect customer_name1
        customer_name1,
        -- In the next line validation works correctly
        c.region1
    FROM CUSTOMERS c
    WHERE region = 'NORTH'
)
SELECT 
    vc.customer_name,
    vc.region,
    mss.sales_month,
    mss.total_revenue
FROM VIP_Customers vc
CROSS JOIN Monthly_Sales_Summary mss
ORDER BY mss.total_revenue DESC
LIMIT 100;`,
			wantCol: "customer_name1",
		},
	}
	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			markers := ValidateSemantics(tt.sql, nil, nil)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(strings.ToLower(w.Message), strings.ToLower(tt.wantCol)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning for column %q but got markers: %v", tt.wantCol, warns)
			}
		})
	}
}

// TestValidateSemantics_ImplicitColumnAlias verifies that an implicit (AS-less)
// output alias is treated as an alias, not a bare column reference against the
// FROM tables (issue #713).
func TestValidateSemantics_ImplicitColumnAlias(t *testing.T) {
	sql := "SELECT ID employee_id FROM DB.SCH.EMPLOYEES"
	markers := ValidateSemantics(sql, getTestRefs(), getTestColCaches())
	if warns := getWarnings(markers); len(warns) > 0 {
		t.Errorf("Expected no warnings for %q, got %d: %v", sql, len(warns), warns)
	}
}

// TestValidateSemantics_ImplicitAliasScopedToProjection verifies the
// implicit-alias heuristic only applies inside the SELECT projection list.
// A bogus trailing identifier in GROUP BY / ORDER BY is two adjacent bare
// idents but must still be flagged as a missing column — it is NOT an implicit
// output alias (PR #739 regression guard).
func TestValidateSemantics_ImplicitAliasScopedToProjection(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want string // substring of the expected warning message
	}{
		{
			name: "GROUP BY trailing bogus ident",
			sql:  "SELECT DEPT_ID, COUNT(*) c FROM DB.SCH.EMPLOYEES e GROUP BY DEPT_ID bogus_col",
			want: "bogus_col",
		},
		{
			name: "ORDER BY trailing bogus ident",
			sql:  "SELECT ID FROM DB.SCH.EMPLOYEES e ORDER BY ID bogus_col2",
			want: "bogus_col2",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			warns := getWarnings(ValidateSemantics(tt.sql, getTestRefs(), getTestColCaches()))
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, tt.want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected a warning mentioning %q for %q, got %d: %v",
					tt.want, tt.sql, len(warns), warns)
			}
		})
	}
}

// TestValidateSemantics_LocalTableAliasColumns verifies that alias.column
// references against script-local CREATE TABLE tables are validated.
func TestValidateSemantics_LocalTableAliasColumns(t *testing.T) {
	validCases := []struct {
		name string
		sql  string
	}{
		{
			name: "local table alias valid columns",
			sql: "CREATE OR REPLACE TABLE TEST_USERS (user_id NUMBER, user_name VARCHAR, country VARCHAR);\n" +
				"CREATE OR REPLACE TABLE TEST_ORDERS (order_id NUMBER, product_name VARCHAR, user_id NUMBER, country VARCHAR);\n" +
				"SELECT u.user_id, o.product_name, u.country FROM TEST_USERS u JOIN TEST_ORDERS o ON u.user_id = o.user_id",
		},
		{
			name: "local table alias from a single-table query",
			sql:  "CREATE TABLE foo (a NUMBER, b VARCHAR);\nSELECT f.a, f.b FROM foo f",
		},
	}
	for _, tt := range validCases {
		t.Run(tt.name, func(t *testing.T) {
			if warns := getWarnings(ValidateSemantics(tt.sql, nil, nil)); len(warns) > 0 {
				t.Errorf("Expected no warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name    string
		sql     string
		wantCol string
	}{
		{
			name: "local table alias unknown column",
			sql: "CREATE OR REPLACE TABLE TEST_USERS (user_id NUMBER, user_name VARCHAR, country VARCHAR);\n" +
				"CREATE OR REPLACE TABLE TEST_ORDERS (order_id NUMBER, product_name VARCHAR, user_id NUMBER, country VARCHAR);\n" +
				"SELECT u.this_should_complain, o.product_name, u.country FROM TEST_USERS u JOIN TEST_ORDERS o ON u.user_id = o.user_id",
			wantCol: "this_should_complain",
		},
		{
			name:    "single-table alias unknown column",
			sql:     "CREATE TABLE foo (a NUMBER, b VARCHAR);\nSELECT f.bad_col FROM foo f",
			wantCol: "bad_col",
		},
	}
	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			warns := getWarnings(ValidateSemantics(tt.sql, nil, nil))
			found := false
			for _, w := range warns {
				if strings.Contains(strings.ToLower(w.Message), strings.ToLower(tt.wantCol)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning for column %q but got: %v", tt.wantCol, warns)
			}
		})
	}
}

// TestValidateSemantics_FullCommentScript tests the exact SQL patterns from
// PR #73 comment 4318612732, which reported two false-negative cases:
//   - vc.incorrect_column_name (CTE alias with unknown column)
//   - u.this_should_complain   (local table alias with unknown column)
func TestValidateSemantics_FullCommentScript(t *testing.T) {
	// SQL approximating the full script from the PR comment.
	fullSQL := `
CREATE OR REPLACE TABLE BIG_SALES_DATA (
    sale_id NUMBER,
    customer_id NUMBER,
    sale_date DATE,
    amount NUMBER(10,2),
    notes VARCHAR
) CLUSTER BY (sale_date);

CREATE OR REPLACE TABLE CUSTOMERS (
    customer_id NUMBER,
    customer_name VARCHAR,
    region VARCHAR
);

SELECT sale_id, amount, notes FROM BIG_SALES_DATA;

SELECT sale_id, amount FROM BIG_SALES_DATA WHERE sale_date = '2024-01-01';

SELECT s.sale_id, c.customer_name FROM BIG_SALES_DATA s JOIN CUSTOMERS c ON s.customer_id = c.customer_id;

WITH Monthly_Sales_Summary AS (
    SELECT
        DATE_TRUNC('month', sale_date) AS sales_month,
        SUM(amount) AS total_revenue,
        COUNT(sale_id) AS total_transactions
    FROM BIG_SALES_DATA
    GROUP BY DATE_TRUNC('month', sale_date)
),
VIP_Customers AS (
    SELECT
        customer_id,
        customer_name,
        region
    FROM CUSTOMERS
    WHERE region = 'NORTH'
)
SELECT
    vc.incorrect_column_name,
    vc.region,
    mss.sales_month,
    mss.total_revenue
FROM VIP_Customers vc
CROSS JOIN Monthly_Sales_Summary mss
ORDER BY mss.total_revenue DESC
LIMIT 100;

CREATE OR REPLACE TABLE TEST_USERS (
    user_id NUMBER,
    user_name VARCHAR,
    country VARCHAR
);

CREATE OR REPLACE TABLE TEST_ORDERS (
    order_id NUMBER,
    product_name VARCHAR,
    user_id NUMBER,
    country VARCHAR
);

SELECT
    u.this_should_complain,
    o.product_name,
    u.country
FROM TEST_USERS u
JOIN TEST_ORDERS o ON u.user_id = o.user_id`

	markers := ValidateSemantics(fullSQL, nil, nil)
	warns := getWarnings(markers)

	mustWarn := []string{"incorrect_column_name", "this_should_complain"}
	mustNotWarn := []string{"sale_id", "amount", "notes", "customer_name", "region",
		"sales_month", "total_revenue", "product_name", "country"}

	for _, col := range mustWarn {
		found := false
		for _, w := range warns {
			if strings.Contains(strings.ToLower(w.Message), col) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning for %q but did not get it. Warnings: %v", col, warns)
		}
	}

	for _, col := range mustNotWarn {
		for _, w := range warns {
			if strings.Contains(strings.ToLower(w.Message), col) {
				t.Errorf("Got unexpected warning mentioning %q: %v", col, w)
			}
		}
	}
}

// TestValidateSemantics_UserEditorSQL tests the exact multi-statement script the
// user pasted from their editor, which begins with USE and includes INSERT INTO
// statements using TABLE(GENERATOR(...)).
func TestValidateSemantics_UserEditorSQL(t *testing.T) {
	sql := `USE LINEAGE_SOURCE_DB.RAW_DATA;

CREATE OR REPLACE TABLE BIG_SALES_DATA (
    sale_id NUMBER,
    customer_id NUMBER,
    sale_date DATE,
    amount NUMBER(10,2),
    notes VARCHAR
) CLUSTER BY (sale_date);


INSERT INTO BIG_SALES_DATA
SELECT
    SEQ4(),
    UNIFORM(1, 100000, RANDOM()),
    DATEADD(day, UNIFORM(1, 3650, RANDOM()), '2015-01-01'),
    UNIFORM(100, 100000, RANDOM()) / 100.0,
    RANDSTR(500, RANDOM())
FROM TABLE(GENERATOR(ROWCOUNT => 5000000));

CREATE OR REPLACE TABLE CUSTOMERS (
    customer_id NUMBER,
    customer_name VARCHAR,
    region VARCHAR
);

INSERT INTO CUSTOMERS
SELECT
    SEQ4(),
    'Customer ' || TO_VARCHAR(SEQ4()),
    DECODE(MOD(SEQ4(), 4), 0, 'NORTH', 1, 'SOUTH', 2, 'EAST', 3, 'WEST')
FROM TABLE(GENERATOR(ROWCOUNT => 100000));

SELECT
    sale_id,
    amount,
    notes
FROM BIG_SALES_DATA;

SELECT
    sale_id,
    amount
FROM BIG_SALES_DATA
WHERE sale_date = '2024-01-01';

SELECT
    s.sale_id,
    c.customer_name
FROM BIG_SALES_DATA s
JOIN CUSTOMERS c;


WITH Monthly_Sales_Summary AS (
    SELECT
        DATE_TRUNC('month', sale_date) AS sales_month,
        SUM(amount) AS total_revenue,
        COUNT(sale_id) AS total_transactions
    FROM BIG_SALES_DATA
    GROUP BY DATE_TRUNC('month', sale_date)
),
VIP_Customers AS (
    SELECT
        customer_id,
        customer_name,
        region
    FROM CUSTOMERS
    WHERE region = 'NORTH'
)
SELECT
-- The next row should complain about incorrect column name
    vc.incorrect_column_name,
    vc.region,
    mss.sales_month,
    mss.total_revenue
FROM VIP_Customers vc
CROSS JOIN Monthly_Sales_Summary mss
ORDER BY mss.total_revenue DESC
LIMIT 100;`

	markers := ValidateSemantics(sql, nil, nil)
	warns := getWarnings(markers)

	t.Logf("All warnings (%d):", len(warns))
	for _, w := range warns {
		t.Logf("  Line %d col %d-%d: %q", w.StartLineNumber, w.StartColumn, w.EndColumn, w.Message)
	}

	found := false
	for _, w := range warns {
		if strings.Contains(strings.ToLower(w.Message), "incorrect_column_name") {
			found = true
		}
	}
	if !found {
		t.Error("Expected warning for 'incorrect_column_name' but got none")
	}

	for _, col := range []string{"sale_id", "amount", "notes", "customer_name", "region",
		"sales_month", "total_revenue"} {
		for _, w := range warns {
			if strings.Contains(strings.ToLower(w.Message), col) {
				t.Errorf("Got unexpected warning mentioning %q: %q", col, w.Message)
			}
		}
	}
}

// TestFromValues_NoFalsePositives covers issue #692: `SELECT ... FROM VALUES
// (...), (...)` is a valid Snowflake table literal. Its implicit columns
// (column1..columnN) must not be flagged as out-of-scope, and VALUES itself
// must not be treated as a missing table.
func TestFromValues_NoFalsePositives(t *testing.T) {
	sql := `CREATE OR REPLACE TABLE spatial_test_data (
    id INT,
    feature_type VARCHAR,
    geo_data GEOGRAPHY
);

INSERT INTO spatial_test_data (id, feature_type, geo_data)
SELECT
    column1 AS id,
    column2 AS feature_type,
    TO_GEOGRAPHY(column3) AS geo_data
FROM VALUES
    (1, 'Point', 'POINT(-122.35 37.55)'),
    (2, 'Line', 'LINESTRING(-124.20 42.00, -120.01 41.99)');`

	// Bug 2: implicit columnN names must not be flagged out-of-scope.
	warns := getWarnings(ValidateSemantics(sql, nil, nil))
	for _, w := range warns {
		if strings.Contains(strings.ToLower(w.Message), "column1") ||
			strings.Contains(strings.ToLower(w.Message), "column2") ||
			strings.Contains(strings.ToLower(w.Message), "column3") {
			t.Errorf("unexpected column-in-scope warning for FROM VALUES: %q", w.Message)
		}
	}

	// Bug 1: VALUES must not be reported as a missing table.
	markers := ValidateTablesExist(ValidateTablesExistRequest{
		SQL:        sql,
		StmtRanges: GetStatementRanges(sql),
	})
	for _, m := range markers {
		if strings.Contains(strings.ToUpper(m.Message), "'VALUES'") {
			t.Errorf("VALUES wrongly reported as a table: %q", m.Message)
		}
	}
}

// TestValidateSemantics_MultiByteCharacters ensures that multi-byte Unicode
// characters (like em-dashes or emojis) do not corrupt the string slicing
// used to look backward for function contexts.
func TestValidateSemantics_MultiByteCharacters(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "Em-dash in comment before DATEADD",
			sql: `
			CREATE TABLE my_table (id INT);
			-- Incorrect warning "WARNING — Column 'month' not found in any of the tables in scope."
			SELECT DATEADD(month, -1, CURRENT_DATE()) FROM my_table;
			`,
		},
		{
			name: "Emoji in comment before EXTRACT",
			sql: `
			CREATE TABLE my_table (id INT);
			/* Checking for year 📅🚀 */
			SELECT EXTRACT(year FROM CURRENT_DATE()) FROM my_table;
			`,
		},
		{
			name: "Multi-byte string literal before function",
			sql: `
			CREATE TABLE my_table (id INT);
			SELECT 'こんにちは' AS greeting, DATE_TRUNC('month', CURRENT_DATE()) FROM my_table;
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := ValidateSemantics(tt.sql, nil, nil)
			warns := getWarnings(markers)

			for _, w := range warns {
				// If the slicing bug is present, the parser won't see the date function,
				// and it will flag 'month' or 'year' as missing columns.
				if strings.Contains(strings.ToLower(w.Message), "month") ||
					strings.Contains(strings.ToLower(w.Message), "year") {
					t.Errorf("Multi-byte slicing bug detected! Got false warning: %q", w.Message)
				}
			}
		})
	}
}

// TestValidateSemantics_MergeUsingSource ensures the MERGE source table —
// introduced by USING (not FROM/JOIN) — is recognized as a table in scope, so
// its bare name is not wrongly flagged as a missing column.
func TestValidateSemantics_MergeUsingSource(t *testing.T) {
	sql := `
CREATE OR REPLACE TABLE t (id INT, a INT, b INT, name VARCHAR(100));
CREATE OR REPLACE TABLE s (id INT, a INT);
MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.a = s.a;
`
	for _, w := range getWarnings(ValidateSemantics(sql, nil, nil)) {
		if strings.Contains(strings.ToLower(w.Message), "not found in any of the tables") {
			t.Errorf("false positive on MERGE USING source: %q", w.Message)
		}
	}
}

// ── 7. CREATE STAGE / ALTER STAGE Diagnostics (Issue #109) ───────────────────

// TestCreateStage_Valid covers the full CREATE STAGE syntax matrix: all
// modifiers, internal-stage params, external-stage params for S3/GCS/Azure/
// S3-compat/OneLake, FILE_FORMAT options, COPY_OPTIONS, DIRECTORY options.
// Each case must produce zero warnings (no false positives).

func TestIssue129_FalsePositives(t *testing.T) {
	sql := `
CREATE OR REPLACE VIEW VW_CLEAN_CUSTOMERS AS
SELECT 
    CUSTOMER_ID,
    UPPER(FIRST_NAME || ' ' || LAST_NAME) AS FULL_NAME,
    REGISTRATION_DATE
FROM RAW_CUSTOMERS
WHERE STATUS = 'ACTIVE';

CREATE OR REPLACE PROCEDURE SP_REFRESH_EXECUTIVE_MART()
RETURNS VARCHAR
LANGUAGE SQL
AS
$$
BEGIN
    TRUNCATE TABLE MART_EXECUTIVE_SUMMARY;

    INSERT INTO MART_EXECUTIVE_SUMMARY (
        CUSTOMER_ID, 
        FULL_NAME, 
        TOTAL_LIFETIME_SPEND, 
        TOTAL_LIFETIME_PROFIT, 
        LAST_REFRESH_DATE
    )
    SELECT 
        clv.CUSTOMER_ID,
        clv.FULL_NAME,
        SUM(op.NET_REVENUE) AS TOTAL_LIFETIME_SPEND,
        SUM(op.ORDER_PROFIT) AS TOTAL_LIFETIME_PROFIT,
        CURRENT_TIMESTAMP() AS LAST_REFRESH_DATE
    FROM VW_CUSTOMER_LIFETIME_VALUE clv
    JOIN VW_ORDER_PROFITABILITY op 
      ON clv.CUSTOMER_ID = op.CUSTOMER_ID
    GROUP BY 
        clv.CUSTOMER_ID,
        clv.FULL_NAME;
    RETURN 'Executive Mart successfully refreshed. Lineage trace complete.';
END;
$$;
`
	// Setup: Mock column data for the referenced tables
	colEntries := []ColEntry{
		{DB: "", Schema: "", Name: "RAW_CUSTOMERS", Cols: []ColInfo{
			{Name: "CUSTOMER_ID", DataType: "INT"},
			{Name: "FIRST_NAME", DataType: "VARCHAR"},
			{Name: "LAST_NAME", DataType: "VARCHAR"},
			{Name: "REGISTRATION_DATE", DataType: "DATE"},
			{Name: "STATUS", DataType: "VARCHAR"},
		}},
		{DB: "", Schema: "", Name: "VW_CUSTOMER_LIFETIME_VALUE", Cols: []ColInfo{
			{Name: "CUSTOMER_ID", DataType: "INT"},
			{Name: "FULL_NAME", DataType: "VARCHAR"},
		}},
		{DB: "", Schema: "", Name: "VW_ORDER_PROFITABILITY", Cols: []ColInfo{
			{Name: "ORDER_ID", DataType: "INT"},
			{Name: "CUSTOMER_ID", DataType: "INT"},
			{Name: "NET_REVENUE", DataType: "NUMBER"},
			{Name: "ORDER_PROFIT", DataType: "NUMBER"},
		}},
		{DB: "", Schema: "", Name: "MART_EXECUTIVE_SUMMARY", Cols: []ColInfo{
			{Name: "CUSTOMER_ID", DataType: "INT"},
			{Name: "FULL_NAME", DataType: "VARCHAR"},
			{Name: "TOTAL_LIFETIME_SPEND", DataType: "NUMBER"},
			{Name: "TOTAL_LIFETIME_PROFIT", DataType: "NUMBER"},
			{Name: "LAST_REFRESH_DATE", DataType: "TIMESTAMP_NTZ"},
		}},
	}

	resolvedRefs := []ResolvedRef{
		{Alias: "RAW_CUSTOMERS", Name: "RAW_CUSTOMERS"},
		{Alias: "VW_CUSTOMER_LIFETIME_VALUE", Name: "VW_CUSTOMER_LIFETIME_VALUE"},
		{Alias: "clv", Name: "VW_CUSTOMER_LIFETIME_VALUE"},
		{Alias: "VW_ORDER_PROFITABILITY", Name: "VW_ORDER_PROFITABILITY"},
		{Alias: "op", Name: "VW_ORDER_PROFITABILITY"},
		{Alias: "MART_EXECUTIVE_SUMMARY", Name: "MART_EXECUTIVE_SUMMARY"},
	}

	markers := ValidateSemantics(sql, resolvedRefs, colEntries)

	for _, m := range markers {
		t.Errorf("Unexpected diagnostic marker: %s at line %d, col %d", m.Message, m.StartLineNumber, m.StartColumn)
	}

	// Also test bare column validation
	req := ValidateBareColsRequest{
		SQL:                         sql,
		StmtRanges:                  GetStatementRanges(sql),
		ResolvedRefs:                resolvedRefs,
		ColEntries:                  colEntries,
		QuotedIdentifiersIgnoreCase: true,
	}
	bareMarkers := ValidateBareColumnRefs(req)
	for _, m := range bareMarkers {
		t.Errorf("Unexpected bare column diagnostic marker: %s at line %d, col %d", m.Message, m.StartLineNumber, m.StartColumn)
	}
}

// ── DESCRIBE / DESC Tests ───────────────────────────────────────────────────

// TestDescribeObjectTypes_OrderingInvariant verifies that describeObjectTypes is sorted
// by word count descending so the longest match is always attempted first.

// ── Quick-Fix Code Field Tests ───────────────────────────────────────────────

func TestValidateTablesExist_QuickFixCode(t *testing.T) {
	t.Run("populates Code when alternative qualifications exist", func(t *testing.T) {
		sql := "SELECT * FROM my_table"
		ranges := GetStatementRanges(sql)
		markers := ValidateTablesExist(ValidateTablesExistRequest{
			SQL:            sql,
			StmtRanges:     ranges,
			ResolvedRefs:   []ResolvedRef{},
			KnownDatabases: []string{"PROD_DB"},
			KnownSchemas:   []SchemaEntry{{DB: "PROD_DB", Name: "PUBLIC"}},
			AllKnownTables: []ResolvedRef{
				{DB: "PROD_DB", Schema: "PUBLIC", Name: "MY_TABLE", Alias: ""},
				{DB: "PROD_DB", Schema: "ANALYTICS", Name: "MY_TABLE", Alias: ""},
			},
		})

		errors := getErrors(markers)
		if len(errors) == 0 {
			t.Fatal("expected at least one error marker")
		}

		foundCode := false
		for _, m := range errors {
			if m.Code != "" {
				foundCode = true
				if !strings.Contains(m.Code, "qualify-table") {
					t.Errorf("expected Code to contain 'qualify-table', got %q", m.Code)
				}
				if !strings.Contains(m.Code, "PROD_DB.PUBLIC.MY_TABLE") {
					t.Errorf("expected Code to contain qualified suggestion, got %q", m.Code)
				}
			}
		}
		if !foundCode {
			t.Error("expected at least one marker to have non-empty Code field")
		}
	})

	t.Run("Code is empty when no alternative qualifications exist", func(t *testing.T) {
		sql := "SELECT * FROM nonexistent_table"
		ranges := GetStatementRanges(sql)
		markers := ValidateTablesExist(ValidateTablesExistRequest{
			SQL:            sql,
			StmtRanges:     ranges,
			ResolvedRefs:   []ResolvedRef{},
			KnownDatabases: []string{"PROD_DB"},
			KnownSchemas:   []SchemaEntry{{DB: "PROD_DB", Name: "PUBLIC"}},
			AllKnownTables: []ResolvedRef{
				{DB: "PROD_DB", Schema: "PUBLIC", Name: "OTHER_TABLE", Alias: ""},
			},
		})

		errors := getErrors(markers)
		if len(errors) == 0 {
			t.Fatal("expected at least one error marker")
		}

		for _, m := range errors {
			if m.Code != "" {
				t.Errorf("expected empty Code field when no matches, got %q", m.Code)
			}
		}
	})
}

// TestValidateBareColumnRefs_NoFromClause_Valid verifies that SELECT statements
// without a FROM clause do NOT produce warnings for literals, keywords, and
// built-in function calls.
func TestValidateBareColumnRefs_NoFromClause_Valid(t *testing.T) {
	validQueries := []string{
		"SELECT 1",
		"SELECT 'hello'",
		"SELECT CURRENT_DATE",
		"SELECT TRUE",
		"SELECT FALSE",
		"SELECT NULL",
		"SELECT 1 + 2",
		"SELECT CURRENT_TIMESTAMP()",
		"SELECT IFF(TRUE, 1, 2)",
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: []ResolvedRef{},
		ColEntries:   []ColEntry{},
	}

	for _, sql := range validQueries {
		t.Run(sql, func(t *testing.T) {
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

// TestValidateBareColumnRefs_NoFromClause_Invalid verifies that bare identifiers
// in a SELECT without a FROM clause produce warnings — they can never resolve.
func TestValidateBareColumnRefs_NoFromClause_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		missingCols []string
	}{
		{"Single bare ident", "SELECT abcd", []string{"ABCD"}},
		{"Literal + bare ident", "SELECT 1, rrrf", []string{"RRRF"}},
		{"Multiple bare idents", "SELECT foo, bar", []string{"FOO", "BAR"}},
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: []ResolvedRef{},
		ColEntries:   []ColEntry{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req.SQL = tt.sql
			req.StmtRanges = GetStatementRanges(tt.sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) != len(tt.missingCols) {
				t.Fatalf("Expected %d warnings for %q, got %d: %v", len(tt.missingCols), tt.sql, len(warnings), warnings)
			}
			for _, col := range tt.missingCols {
				found := false
				for _, w := range warnings {
					if strings.Contains(w.Message, col) || strings.Contains(w.Message, strings.ToLower(col)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for column %q in %q, but not found in %v", col, tt.sql, warnings)
				}
			}
			// Verify the "no FROM clause" label appears in the message.
			for _, w := range warnings {
				if !strings.Contains(w.Message, "no FROM clause") {
					t.Errorf("Expected 'no FROM clause' in message, got %q", w.Message)
				}
			}
		})
	}
}
