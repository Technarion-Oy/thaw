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
		// Streams
		"CREATE STREAM my_stream ON TABLE my_table",
		"CREATE OR REPLACE STREAM my_stream ON VIEW my_view APPEND_ONLY = TRUE",
		"CREATE STREAM IF NOT EXISTS my_stream ON STAGE my_stage COMMENT = 'my stream'",
		"CREATE STREAM my_stream ON TABLE my_table AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		"CREATE STREAM my_stream ON TABLE my_table BEFORE (STATEMENT => '9e564d60-0000-0000-0000-000000000000')",
		"CREATE STREAM my_stream ON TABLE t SHOW_INITIAL_ROWS = TRUE",
		"CREATE STREAM s ON TABLE t CHANGE_TRACKING = TRUE",
		"CREATE STREAM s COPY GRANTS ON TABLE t",
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
		// Pipes
		"CREATE PIPE my_pipe AS COPY INTO my_table FROM @my_stage",
		"CREATE OR REPLACE PIPE my_pipe AS COPY INTO my_table FROM @my_stage",
		"CREATE PIPE IF NOT EXISTS my_pipe AS COPY INTO my_table FROM @my_stage",
		"CREATE PIPE my_pipe AUTO_INGEST = TRUE AS COPY INTO my_table FROM @my_stage",
		"CREATE PIPE my_pipe AUTO_INGEST = TRUE AWS_SNS_TOPIC = 'arn:aws:sns:us-east-1:123456789012:my-topic' AS COPY INTO my_table FROM @my_stage",
		// Alerts
		"CREATE ALERT my_alert WAREHOUSE = my_wh SCHEDULE = '1 MINUTE' IF (EXISTS (SELECT 1)) THEN CALL my_proc()",
		"CREATE OR REPLACE ALERT my_alert WAREHOUSE = my_wh SCHEDULE = 'USING CRON 0 0 * * * UTC' COMMENT = 'test' IF (EXISTS (SELECT * FROM t WHERE val > 10)) THEN INSERT INTO log SELECT CURRENT_TIMESTAMP()",
		"CREATE ALERT IF NOT EXISTS db.schema.my_alert WAREHOUSE = \"MY WH\" SCHEDULE = '5 MINUTE' IF (EXISTS (SELECT 1)) THEN SYSTEM$SEND_EMAIL('my_int', 'me@example.com', 'Alert!', 'Something happened')",
		"CREATE PIPE my_pipe AUTO_INGEST = TRUE INTEGRATION = 'my_int' AS COPY INTO my_table FROM @my_stage",
		"CREATE PIPE my_pipe COMMENT = 'my pipe' AS COPY INTO my_table FROM @my_stage",
		"CREATE PIPE my_pipe ERROR_INTEGRATION = my_error_int AS COPY INTO my_table FROM @my_stage",
		"ALTER PIPE my_pipe REFRESH",
		"ALTER PIPE my_pipe SET COMMENT = 'updated'",
		"DROP PIPE IF EXISTS my_pipe",
		// CALL statements
		"CALL my_proc()",
		"CALL my_proc(1, 2, 'hello')",
		"CALL my_schema.my_proc()",
		"CALL my_db.my_schema.my_proc()",
		"CALL my_proc() INTO :result_var",
		// Procedures
		"ALTER PROCEDURE my_proc(INT) SET COMMENT = 'updated'",
		"DROP PROCEDURE IF EXISTS my_proc(INT)",
		// Functions
		"ALTER FUNCTION my_func(NUMBER) SET COMMENT = 'updated'",
		"DROP FUNCTION IF EXISTS my_func(NUMBER)",
		// COPY INTO
		"COPY INTO my_table FROM @my_stage",
		"COPY INTO @my_stage FROM my_table",
		"COPY INTO my_table(col1, col2) FROM @my_stage",
		"COPY INTO my_table(col1, col2) FROM @my_stage FILE_FORMAT = (TYPE = CSV)",
		"COPY INTO my_table FROM @my_stage FILES = ('f1.csv', 'f2.csv') ON_ERROR = SKIP_FILE_10",
		"COPY INTO @my_stage FROM (SELECT * FROM t) OVERWRITE = TRUE SINGLE = FALSE MAX_FILE_SIZE = 1048576",
		"COPY INTO my_table FROM @my_stage FILE_FORMAT = (TYPE = CSV FIELD_DELIMITER = '|')",
		"COPY INTO my_table FROM @my_stage FILE_FORMAT = (FORMAT_NAME = my_format)",
		// External Tables
		"CREATE EXTERNAL TABLE et (col1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (FORMAT_NAME = my_format)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = JSON) AUTO_REFRESH = TRUE",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION BY (c1) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = PARQUET)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) TABLE_FORMAT = DELTA",
		"CREATE EXTERNAL TABLE IF NOT EXISTS db.schema.et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) INTEGRATION = 'my_int'",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) AWS_SNS_TOPIC = 'arn:aws:sns:us-east-1:123456789012:my_topic'",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) WITH TAG (t1 = 'v1')",
		"CREATE EXTERNAL TABLE \"MY DB\".\"MY SCHEMA\".et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as(value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION  BY (c1) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int), c2 string as (value:c2::string)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ PATTERN = '.*[.]csv' FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) REFRESH_ON_CREATE = FALSE",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION BY (c1) PARTITION_TYPE = USER_SPECIFIED WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et /* comment */ (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) /* CLUSTER BY x */ WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) COMMENT = 'CLUSTER BY is not applicable here'",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV) COMMENT = 'DATA_RETENTION_TIME_IN_DAYS = 1'",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) -- DATA_RETENTION_TIME_IN_DAYS = 1\n WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) COPY GRANTS WITH LOCATION = @my_stage FILE_FORMAT = (TYPE = CSV)",
		// File Formats
		"CREATE FILE FORMAT my_fmt TYPE = CSV",
		"CREATE FILE FORMAT my_fmt TYPE = CSV FIELD_DELIMITER = '\\x09' COMMENT = 'this is a tab'",
		"CREATE FILE FORMAT my_fmt TYPE = CSV COMMENT = 'FIELD_DELIMITER = |'",
		"CREATE FILE FORMAT my_fmt TYPE = CSV NULL_IF = ('SKIP_HEADER = -1')",
		"CREATE FILE FORMAT my_fmt TYPE = JSON COMPRESSION = GZIP",
		"CREATE FILE FORMAT IF NOT EXISTS my_fmt TYPE = XML",
		"CREATE OR REPLACE FILE FORMAT my_fmt TYPE = CSV",
		"CREATE FILE FORMAT my_fmt TYPE = ORC COMMENT = 'A TRANSIENT format'",
		"CREATE FILE FORMAT my_fmt TYPE = JSON -- FIELD_DELIMITER = ','",
		"CREATE FILE FORMAT my_fmt",
		"CREATE FILE FORMAT my_fmt FIELD_DELIMITER = ','",
		"CREATE FILE FORMAT my_fmt FIELD_DELIMITER = NONE",
		"CREATE FILE FORMAT my_fmt FIELD_DELIMITER = 'NONE'",
		"CREATE FILE FORMAT my_fmt NULL_IF = ('TYPE = CSV')",
		// ALTER / DROP FILE FORMAT
		"ALTER FILE FORMAT my_fmt SET TYPE = CSV",
		"ALTER FILE FORMAT my_fmt SET COMMENT = 'new comment'",
		"DROP FILE FORMAT my_fmt",
		"DROP FILE FORMAT IF EXISTS my_fmt",
		// Network Policies
		"CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('192.168.1.0/24')",
		"CREATE OR REPLACE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.0/8', '192.168.1.1/32')",
		"CREATE NETWORK POLICY my_policy ALLOWED_NETWORK_RULE_LIST = (my_rule)",
		"CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.1/32') BLOCKED_IP_LIST = ('192.168.0.0/16')",
		"CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('0.0.0.0/0')",
		"CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.1')",
		"CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.0/8') ALLOWED_NETWORK_RULE_LIST = (rule1, rule2) COMMENT = 'my policy'",
		"CREATE NETWORK POLICY my_policy ALLOWED_NETWORK_RULE_LIST = (rule1) BLOCKED_NETWORK_RULE_LIST = (rule2)",
		// Quoted policy name whose inner text contains a dot — must not trigger the prefix warning.
		`CREATE NETWORK POLICY "my.policy" ALLOWED_IP_LIST = ('10.0.0.0/8')`,
		// Row Access Policies
		"CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR) RETURNS BOOLEAN -> val = current_user()",
		"CREATE OR REPLACE ROW ACCESS POLICY my_rap AS (n NUMBER) RETURNS BOOLEAN -> n > 0",
		"CREATE ROW ACCESS POLICY IF NOT EXISTS my_rap AS (val VARCHAR) RETURNS BOOLEAN -> TRUE",
		"CREATE ROW ACCESS POLICY my_rap AS (a VARCHAR, b NUMBER) RETURNS BOOLEAN -> a = 'x' AND b > 0",
		"CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR(256)) RETURNS BOOLEAN -> val = 'admin'",
		"CREATE ROW ACCESS POLICY my_rap AS (n NUMBER(10,2)) RETURNS BOOLEAN -> n > 0",
		"CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR) RETURNS BOOLEAN -> CASE WHEN val = 'admin' THEN TRUE ELSE FALSE END",
		"CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR) RETURNS BOOLEAN -> val = current_user() COMMENT = 'my comment'",
		// Session Policies
		"CREATE SESSION POLICY my_session_policy",
		"CREATE OR REPLACE SESSION POLICY my_session_policy",
		"CREATE SESSION POLICY my_session_policy SESSION_IDLE_TIMEOUT_MINS = 60",
		"CREATE SESSION POLICY my_session_policy SESSION_UI_IDLE_TIMEOUT_MINS = 30",
		"CREATE SESSION POLICY my_session_policy SESSION_IDLE_TIMEOUT_MINS = 0",
		"CREATE SESSION POLICY my_session_policy SESSION_IDLE_TIMEOUT_MINS = 56400",
		"CREATE SESSION POLICY my_session_policy SESSION_UI_IDLE_TIMEOUT_MINS = 56400",
		"CREATE SESSION POLICY my_session_policy SESSION_IDLE_TIMEOUT_MINS = 60 SESSION_UI_IDLE_TIMEOUT_MINS = 30 COMMENT = 'my policy'",
		// Quoted session policy name with an inner dot must not trigger the prefix warning.
		`CREATE SESSION POLICY "my.policy" SESSION_IDLE_TIMEOUT_MINS = 60`,
		// Password Policies
		"CREATE PASSWORD POLICY my_password_policy",
		"CREATE OR REPLACE PASSWORD POLICY my_password_policy",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MIN_LENGTH = 8",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MIN_LENGTH = 8 PASSWORD_MAX_LENGTH = 256",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MAX_RETRIES = 5 PASSWORD_LOCKOUT_TIME_MINS = 15",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MIN_UPPER_CASE_CHARS = 1 PASSWORD_MIN_LOWER_CASE_CHARS = 1 PASSWORD_MIN_NUMERIC_CHARS = 1 PASSWORD_MIN_SPECIAL_CHARS = 1",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MIN_AGE_DAYS = 0 PASSWORD_MAX_AGE_DAYS = 90",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_HISTORY = 0",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_HISTORY = 24",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MIN_LENGTH = 10 PASSWORD_MAX_LENGTH = 10",
		"CREATE PASSWORD POLICY my_password_policy PASSWORD_MAX_AGE_DAYS = 0",
		"CREATE PASSWORD POLICY my_password_policy COMMENT = 'corporate policy'",
		// Quoted password policy name with an inner dot must not trigger the prefix warning.
		`CREATE PASSWORD POLICY "my.policy" PASSWORD_MIN_LENGTH = 12`,
		// GRANT statements — valid
		"GRANT SELECT ON TABLE my_table TO ROLE my_role",
		"GRANT INSERT, UPDATE, DELETE ON TABLE my_table TO ROLE my_role",
		"GRANT SELECT ON VIEW my_view TO ROLE my_role",
		"GRANT REFERENCES ON VIEW my_view TO ROLE my_role",
		"GRANT USAGE ON WAREHOUSE my_wh TO ROLE my_role",
		"GRANT USAGE, MODIFY, MONITOR, OPERATE ON WAREHOUSE my_wh TO ROLE my_role",
		"GRANT USAGE, MODIFY ON DATABASE my_db TO ROLE my_role",
		"GRANT CREATE SCHEMA ON DATABASE my_db TO ROLE my_role",
		"GRANT IMPORTED PRIVILEGES ON DATABASE my_db TO ROLE my_role",
		"GRANT CREATE TABLE ON SCHEMA my_schema TO ROLE my_role",
		"GRANT CREATE ROW ACCESS POLICY ON SCHEMA my_schema TO ROLE my_role",
		"GRANT ADD SEARCH OPTIMIZATION ON SCHEMA my_schema TO ROLE my_role",
		"GRANT USAGE ON SCHEMA my_schema TO ROLE my_role WITH GRANT OPTION",
		"GRANT USAGE ON INTEGRATION my_int TO ROLE my_role",
		"GRANT MONITOR, OPERATE ON TASK my_task TO ROLE my_role",
		"GRANT SELECT ON STREAM my_stream TO ROLE my_role",
		"GRANT MONITOR ON USER my_user TO ROLE my_role",
		"GRANT MANAGE GRANTS ON ACCOUNT TO ROLE my_role",
		"GRANT EXECUTE TASK ON ACCOUNT TO ROLE my_role",
		"GRANT ROLE my_role TO ROLE other_role",
		"GRANT ROLE my_role TO USER my_user",
		"GRANT DATABASE ROLE my_db_role TO ROLE my_role",
		"GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA my_schema TO ROLE my_role",
		"GRANT SELECT ON FUTURE TABLES IN DATABASE my_db TO ROLE my_role",
		"GRANT OWNERSHIP ON TABLE my_table TO ROLE my_role",
		"GRANT ALL PRIVILEGES ON TABLE my_table TO ROLE my_role",
		"GRANT ALL ON SCHEMA my_schema TO ROLE my_role",
		// Newer privilege names that were previously missing from the matrix
		"GRANT EVOLVE SCHEMA ON TABLE my_table TO ROLE my_role",
		"GRANT APPLYBUDGET ON WAREHOUSE my_wh TO ROLE my_role",
		"GRANT APPLYBUDGET ON DATABASE my_db TO ROLE my_role",
		"GRANT CREATE STREAMLIT ON SCHEMA my_schema TO ROLE my_role",
		"GRANT CREATE NOTEBOOK ON SCHEMA my_schema TO ROLE my_role",
		"GRANT APPLY SESSION POLICY ON ACCOUNT TO ROLE my_role",
		"GRANT APPLY TAG ON ACCOUNT TO ROLE my_role",
		"GRANT MANAGE WAREHOUSES ON ACCOUNT TO ROLE my_role",
		"GRANT RESOLVE ALL ON ACCOUNT TO ROLE my_role",
		// REVOKE statements — valid
		"REVOKE SELECT ON TABLE my_table FROM ROLE my_role",
		"REVOKE INSERT, UPDATE ON TABLE my_table FROM ROLE my_role",
		"REVOKE USAGE ON WAREHOUSE my_wh FROM ROLE my_role",
		"REVOKE USAGE ON DATABASE my_db FROM ROLE my_role",
		"REVOKE CREATE TABLE ON SCHEMA my_schema FROM ROLE my_role",
		"REVOKE ROLE my_role FROM ROLE other_role",
		"REVOKE ROLE my_role FROM USER my_user",
		"REVOKE DATABASE ROLE my_db_role FROM ROLE my_role",
		"REVOKE SELECT ON TABLE my_table FROM ROLE my_role CASCADE",
		"REVOKE SELECT ON TABLE my_table FROM ROLE my_role RESTRICT",
		"REVOKE GRANT OPTION FOR SELECT ON TABLE my_table FROM ROLE my_role",
		"REVOKE SELECT ON ALL TABLES IN SCHEMA my_schema FROM ROLE my_role",
		"REVOKE SELECT ON FUTURE TABLES IN DATABASE my_db FROM ROLE my_role",
		// Data-sharing grants — TO SHARE / FROM SHARE are valid grantee forms.
		"GRANT USAGE ON DATABASE my_db TO SHARE my_share",
		"GRANT SELECT ON TABLE my_table TO SHARE my_share",
		"REVOKE USAGE ON DATABASE my_db FROM SHARE my_share",
		"REVOKE SELECT ON TABLE my_table FROM SHARE my_share",
		// CREATE SHARE — valid
		"CREATE SHARE my_share",
		"CREATE OR REPLACE SHARE my_share",
		"CREATE SHARE IF NOT EXISTS my_share",
		"CREATE SHARE my_share COMMENT = 'description of the share'",
		"CREATE OR REPLACE SHARE my_share COMMENT = 'updated'",
		// "IF NOT EXISTS" inside a COMMENT value must not trigger the conflict warning.
		"CREATE OR REPLACE SHARE my_share COMMENT = 'IF NOT EXISTS hint'",
		// ALTER SHARE — valid
		"ALTER SHARE my_share ADD ACCOUNTS = account1",
		"ALTER SHARE my_share ADD ACCOUNTS = account1, account2",
		"ALTER SHARE my_share ADD ACCOUNTS = orgname.accountname",
		"ALTER SHARE my_share ADD ACCOUNTS = account1 RESTRICT",
		"ALTER SHARE my_share ADD ACCOUNTS = acct1, acct2 RESTRICT",
		"ALTER SHARE my_share REMOVE ACCOUNTS = account1",
		"ALTER SHARE my_share REMOVE ACCOUNTS = account1, account2",
		"ALTER SHARE my_share SET COMMENT = 'updated comment'",
		"ALTER SHARE my_share RENAME TO new_share_name",
		// Share named "restrict" must not trigger the RESTRICT warning.
		"ALTER SHARE restrict SET COMMENT = 'test'",
		`ALTER SHARE "restrict" ADD OBJECTS = db.schema.tbl`,
		// ADD ACCOUNTS = restrict treated as account named "restrict", not keyword.
		"ALTER SHARE my_share ADD ACCOUNTS = restrict",
		// CREATE DATABASE FROM SHARE — valid
		"CREATE DATABASE my_db FROM SHARE provider_account.share_name",
		"CREATE DATABASE IF NOT EXISTS my_db FROM SHARE provider.my_share",
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

		// Invalid Stream
		{"Stream missing ON", "CREATE STREAM s TABLE t", "Unexpected syntax"},
		{"Stream missing object", "CREATE STREAM s ON TABLE", "Unexpected syntax"},
		{"Stream invalid property", "CREATE STREAM s ON TABLE t AT (OFFSET => -100) INVALID_PROP = TRUE", "Unexpected syntax"},
		{"Stream invalid object type", "CREATE STREAM s ON SEQUENCE seq", "Unexpected syntax"},
		{"Stream COPY GRANTS after ON", "CREATE STREAM s ON TABLE t COPY GRANTS", "Unexpected syntax"},
		{"Stream Replace IF NOT EXISTS", "CREATE OR REPLACE STREAM foo IF NOT EXISTS ON TABLE t", "Conflict between OR REPLACE and IF NOT EXISTS"},

		// Invalid MERGE
		{"MERGE INSERT in MATCHED", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN INSERT (id) VALUES (s.id)", "not allowed in WHEN MATCHED"},
		{"MERGE UPDATE in NOT MATCHED", "MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED THEN UPDATE SET val = s.val", "not allowed in WHEN NOT MATCHED"},
		{"MERGE NOT MATCHED BY SOURCE", "MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED BY SOURCE THEN DELETE", "not supported by Snowflake"},

		// Invalid File Formats
		{"File Format invalid TYPE", "CREATE FILE FORMAT my_fmt TYPE = 'EXCEL'", "Invalid TYPE 'EXCEL' for FILE FORMAT"},
		{"File Format invalid TRANSIENT", "CREATE TRANSIENT FILE FORMAT my_fmt TYPE = CSV", "Unexpected syntax"},
		{"File Format invalid TEMPORARY", "CREATE TEMPORARY FILE FORMAT my_fmt TYPE = CSV", "Unexpected syntax"},
		{"File Format invalid TEMP", "CREATE TEMP FILE FORMAT my_fmt TYPE = CSV", "Unexpected syntax"},
		{"File Format Replace IF NOT EXISTS", "CREATE OR REPLACE FILE FORMAT IF NOT EXISTS my_fmt TYPE = JSON", "Conflict between OR REPLACE and IF NOT EXISTS"}, {"File Format FIELD_DELIMITER on PARQUET", "CREATE FILE FORMAT my_fmt TYPE = PARQUET FIELD_DELIMITER = ','", "Property 'FIELD_DELIMITER' is not applicable for PARQUET"},
		{"File Format FIELD_DELIMITER on AVRO", "CREATE FILE FORMAT my_fmt TYPE = AVRO FIELD_DELIMITER = ','", "Property 'FIELD_DELIMITER' is not applicable for AVRO"},
		{"File Format invalid FIELD_DELIMITER", "CREATE FILE FORMAT my_fmt TYPE = CSV FIELD_DELIMITER = 'abc'", "FIELD_DELIMITER must be a single-character string"},
		{"File Format empty FIELD_DELIMITER", "CREATE FILE FORMAT my_fmt TYPE = CSV FIELD_DELIMITER = ''", "FIELD_DELIMITER cannot be empty"},
		{"File Format negative SKIP_HEADER", "CREATE FILE FORMAT my_fmt TYPE = CSV SKIP_HEADER = -1", "SKIP_HEADER must be a non-negative integer"},
		{"File Format quoted negative SKIP_HEADER", "CREATE FILE FORMAT my_fmt TYPE = CSV SKIP_HEADER = '-1'", "SKIP_HEADER must be a non-negative integer"},

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

		// Invalid GRANT — privilege/object mismatches
		{"Grant invalid priv on table", "GRANT INVALID_PRIV ON TABLE my_table TO ROLE my_role", "not valid for object type TABLE"},
		{"Grant select on warehouse", "GRANT SELECT ON WAREHOUSE my_wh TO ROLE my_role", "not valid for object type WAREHOUSE"},
		{"Grant insert on view", "GRANT INSERT ON VIEW my_view TO ROLE my_role", "not valid for object type VIEW"},
		{"Grant write on table", "GRANT WRITE ON TABLE my_table TO ROLE my_role", "not valid for object type TABLE"},
		{"Grant select on stage", "GRANT SELECT ON STAGE my_stage TO ROLE my_role", "not valid for object type STAGE"},
		{"Grant usage on stream", "GRANT USAGE ON STREAM my_stream TO ROLE my_role", "not valid for object type STREAM"},
		{"Grant select on account", "GRANT SELECT ON ACCOUNT TO ROLE my_role", "not valid for object type ACCOUNT"},
		{"Grant usage on role", "GRANT USAGE ON ROLE my_role TO ROLE other_role", "not valid Snowflake syntax"},
		{"Grant multi priv one invalid", "GRANT SELECT, INVALID_PRIV ON TABLE my_table TO ROLE my_role", "not valid for object type TABLE"},

		// Invalid GRANT — structural issues
		{"Grant role with grant option", "GRANT ROLE my_role TO USER u WITH GRANT OPTION", "WITH GRANT OPTION is not valid"},
		{"Grant role no grantee", "GRANT ROLE my_role", "TO ROLE or TO USER"},
		{"Grant priv missing grantee", "GRANT SELECT ON TABLE my_table", "grantee"},
		{"Grant all tables without in", "GRANT SELECT ON ALL TABLES TO ROLE my_role", "IN SCHEMA or IN DATABASE"},
		{"Grant future tables without in", "GRANT SELECT ON FUTURE TABLES TO ROLE my_role", "IN SCHEMA or IN DATABASE"},

		// Invalid REVOKE — ON ALL/FUTURE without IN qualifier
		{"Revoke all tables without in", "REVOKE SELECT ON ALL TABLES FROM ROLE my_role", "IN SCHEMA or IN DATABASE"},
		{"Revoke future tables without in", "REVOKE SELECT ON FUTURE TABLES FROM ROLE my_role", "IN SCHEMA or IN DATABASE"},

		// Invalid REVOKE — privilege/object mismatches
		{"Revoke insert on view", "REVOKE INSERT ON VIEW my_view FROM ROLE my_role", "not valid for object type VIEW"},
		{"Revoke select on warehouse", "REVOKE SELECT ON WAREHOUSE my_wh FROM ROLE my_role", "not valid for object type WAREHOUSE"},
		{"Revoke select on stage", "REVOKE SELECT ON STAGE my_stage FROM ROLE my_role", "not valid for object type STAGE"},
		{"Revoke write on stream", "REVOKE WRITE ON STREAM my_stream FROM ROLE my_role", "not valid for object type STREAM"},
		{"Revoke usage on role", "REVOKE USAGE ON ROLE my_role FROM ROLE other_role", "not valid Snowflake syntax"},

		// Invalid REVOKE — structural issues
		{"Revoke cascade and restrict", "REVOKE SELECT ON TABLE my_table FROM ROLE my_role CASCADE RESTRICT", "mutually exclusive"},
		{"Revoke missing from", "REVOKE SELECT ON TABLE my_table", "FROM"},
		{"Revoke role no from", "REVOKE ROLE my_role", "FROM ROLE or FROM USER"},

		// Invalid Row Access Policy
		{"RAP OR REPLACE with IF NOT EXISTS", "CREATE OR REPLACE ROW ACCESS POLICY IF NOT EXISTS my_rap AS (val VARCHAR) RETURNS BOOLEAN -> TRUE", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"RAP missing AS param list", "CREATE ROW ACCESS POLICY my_rap RETURNS BOOLEAN -> TRUE", "Missing mandatory AS"},
		{"RAP empty param list", "CREATE ROW ACCESS POLICY my_rap AS () RETURNS BOOLEAN -> TRUE", "at least one argument"},
		{"RAP missing RETURNS BOOLEAN", "CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR) -> TRUE", "Missing mandatory RETURNS BOOLEAN"},
		{"RAP missing arrow", "CREATE ROW ACCESS POLICY my_rap AS (val VARCHAR) RETURNS BOOLEAN CASE WHEN 1=1 THEN TRUE END", "Missing mandatory '->'"},
		{"RAP invalid param data type", "CREATE ROW ACCESS POLICY my_rap AS (val NOTATYPE) RETURNS BOOLEAN -> TRUE", "Unknown data type 'NOTATYPE'"},

		// Invalid Pipe
		{"Pipe missing AS", "CREATE PIPE my_pipe", "Missing mandatory AS COPY INTO"},
		{"Pipe invalid body", "CREATE PIPE my_pipe AS SELECT 1", "Missing mandatory AS COPY INTO"},
		{"Pipe SNS without AUTO_INGEST", "CREATE PIPE my_pipe AWS_SNS_TOPIC = 'arn:...' AS COPY INTO my_table FROM @my_stage", "AWS_SNS_TOPIC is only meaningful when AUTO_INGEST = TRUE"},
		{"Pipe invalid property", "CREATE PIPE my_pipe INVALID_PROP = TRUE AS COPY INTO my_table FROM @my_stage", "Unexpected property 'INVALID_PROP'"},
		{"Pipe Replace IF NOT EXISTS", "CREATE OR REPLACE PIPE IF NOT EXISTS my_pipe AS COPY INTO my_table FROM @my_stage", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Pipe AUTO_INGEST no stage", "CREATE PIPE my_pipe AUTO_INGEST = TRUE AS COPY INTO my_table FROM (SELECT * FROM t)", "typically requires a stage source"},

		// Invalid Alert
		{"Alert missing WAREHOUSE", "CREATE ALERT my_alert SCHEDULE = '1 MINUTE' IF (EXISTS (SELECT 1)) THEN CALL p()", "Missing mandatory WAREHOUSE"},
		{"Alert missing SCHEDULE", "CREATE ALERT my_alert WAREHOUSE = wh IF (EXISTS (SELECT 1)) THEN CALL p()", "Missing mandatory SCHEDULE"},
		{"Alert missing IF", "CREATE ALERT my_alert WAREHOUSE = wh SCHEDULE = '1 MINUTE' THEN CALL p()", "Missing mandatory IF (EXISTS (...))"},
		{"Alert missing THEN", "CREATE ALERT my_alert WAREHOUSE = wh SCHEDULE = '1 MINUTE' IF (EXISTS (SELECT 1)) CALL p()", "Missing mandatory THEN keyword"},
		{"Alert Replace IF NOT EXISTS", "CREATE OR REPLACE ALERT IF NOT EXISTS my_alert WAREHOUSE = wh SCHEDULE = '1 MINUTE' IF (EXISTS (SELECT 1)) THEN CALL p()", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Alert THEN false negative with CASE THEN in subquery", "CREATE ALERT a WAREHOUSE = wh SCHEDULE = '1 MINUTE' IF (EXISTS (SELECT CASE WHEN x > 1 THEN 1 ELSE 0 END FROM t)) CALL p()", "Missing mandatory THEN keyword"},
		{"Alert unknown property", "CREATE ALERT my_alert WAREHOUSE = wh SCHEDULE = '1 MINUTE' FOO = bar IF (EXISTS (SELECT 1)) THEN CALL p()", "Unexpected property 'FOO'"},

		// Invalid COPY INTO
		{"COPY missing FROM", "COPY INTO my_table", "missing the mandatory FROM clause"},
		{"COPY mutually exclusive FILES/PATTERN", "COPY INTO my_table FROM @my_stage FILES = ('f1.csv') PATTERN = '.*\\.csv'", "mutually exclusive"},
		{"COPY invalid ON_ERROR", "COPY INTO my_table FROM @my_stage ON_ERROR = INVALID_VAL", "Invalid ON_ERROR value"},
		{"COPY invalid PURGE", "COPY INTO my_table FROM @my_stage PURGE = YES", "must be TRUE or FALSE"},
		{"COPY invalid MAX_FILE_SIZE", "COPY INTO @my_stage FROM t MAX_FILE_SIZE = -100", "must be a positive integer"},
		{"COPY invalid FILE_FORMAT TYPE", "COPY INTO my_table FROM @my_stage FILE_FORMAT = (TYPE = 'EXCEL')", "Invalid FILE_FORMAT TYPE"},
		{"COPY mutually exclusive FORMAT_NAME/TYPE", "COPY INTO my_table FROM @my_stage FILE_FORMAT = (FORMAT_NAME = my_format TYPE = CSV)", "mutually exclusive"},

		// Invalid External Table
		{"External Table non-virtual col with AS", "CREATE EXTERNAL TABLE et (c1 INT DEFAULT CAST(0 AS INT)) WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)", "must be a virtual column using AS"},
		{"External Table OR REPLACE", "CREATE OR REPLACE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)", "OR REPLACE is not supported"},
		{"External Table CLUSTER BY", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) CLUSTER BY (c1) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)", "CLUSTER BY is not supported"},
		{"External Table Retention", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) DATA_RETENTION_TIME_IN_DAYS = 1", "DATA_RETENTION_TIME_IN_DAYS is not applicable"},
		{"External Table missing Location", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) FILE_FORMAT = (TYPE = CSV)", "WITH LOCATION = @<stage> is mandatory"},
		{"External Table missing File Format", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/", "FILE_FORMAT is mandatory"},
		{"External Table non-virtual column", "CREATE EXTERNAL TABLE et (c1 int) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)", "must be a virtual column using AS"},
		{"External Table invalid prop", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) AUTO_REFRESH = YES", "Unexpected syntax in CREATE EXTERNAL TABLE properties"},
		{"External Table partition missing parens", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION BY c1 WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)", "requires a parenthesised column list"},
		{"External Table partition unclosed parens", "CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION BY (c1 WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV)", "Unclosed parenthesised column list in PARTITION BY clause"},
		{"External Table empty columns", "CREATE EXTERNAL TABLE et () WITH LOCATION = @s/p/ FILE_FORMAT = (TYPE = CSV)", "Column list must not be empty"},

		// Invalid Network Policies
		{"Network Policy with prefix", "CREATE NETWORK POLICY MY_DB.PUBLIC.bad_policy ALLOWED_IP_LIST = ('10.0.0.0/8')", "account-level"},
		{"Network Policy OR REPLACE with prefix", "CREATE OR REPLACE NETWORK POLICY MY_DB.PUBLIC.bad ALLOWED_IP_LIST = ('10.0.0.0/8')", "account-level"},
		{"Network Policy no allowed list (only blocked)", "CREATE NETWORK POLICY my_policy BLOCKED_IP_LIST = ('10.0.0.0/8')", "no effect"},
		{"Network Policy no properties", "CREATE NETWORK POLICY my_policy", "no effect"},
		{"Network Policy only comment", "CREATE NETWORK POLICY my_policy COMMENT = 'test'", "no effect"},
		{"Network Policy empty ALLOWED_IP_LIST", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ()", "no effect"},
		{"Network Policy empty ALLOWED_NETWORK_RULE_LIST", "CREATE NETWORK POLICY my_policy ALLOWED_NETWORK_RULE_LIST = ()", "no effect"},
		{"Network Policy both lists empty", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = () BLOCKED_IP_LIST = ()", "no effect"},
		{"Network Policy whitespace-only quoted entry", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('   ')", "no effect"},
		{"Network Policy invalid IP", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('not_an_ip')", "Invalid IPv4"},
		{"Network Policy invalid CIDR prefix", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('192.168.0.1/33')", "Invalid IPv4"},
		{"Network Policy invalid octet", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('256.0.0.1/24')", "Invalid IPv4"},
		// Snowflake only accepts IPv4 in ALLOWED/BLOCKED_IP_LIST; IPv6 is rejected even if valid.
		{"Network Policy valid IPv6 in IP list (rejected)", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('2001:db8::/32')", "Invalid IPv4"},
		{"Network Policy bare IPv6 in IP list (rejected)", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('::1')", "Invalid IPv4"},
		{"Network Policy IPv6 in blocked list (rejected)", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.0/8') BLOCKED_IP_LIST = ('fe80::/10')", "Invalid IPv4"},
		{"Network Policy malformed IPv6", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('gg::1')", "Invalid IPv4"},
		{"Network Policy IP in both lists", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.1/32') BLOCKED_IP_LIST = ('10.0.0.1/32')", "appears in both"},
		{"Network Policy unknown property", "CREATE NETWORK POLICY my_policy ALLOWED_IP_LIST = ('10.0.0.0/8') INVALID_PROP = TRUE", "Unexpected property 'INVALID_PROP'"},

		// Invalid Session Policies
		{"Session Policy with prefix", "CREATE SESSION POLICY MY_DB.PUBLIC.bad_policy SESSION_IDLE_TIMEOUT_MINS = 60", "account-level"},
		{"Session Policy OR REPLACE with prefix", "CREATE OR REPLACE SESSION POLICY MY_DB.PUBLIC.bad SESSION_IDLE_TIMEOUT_MINS = 60", "account-level"},
		{"Session Policy idle timeout below range", "CREATE SESSION POLICY my_policy SESSION_IDLE_TIMEOUT_MINS = -1", "out of range"},
		{"Session Policy idle timeout above range", "CREATE SESSION POLICY my_policy SESSION_IDLE_TIMEOUT_MINS = 56401", "out of range"},
		{"Session Policy UI idle timeout below range", "CREATE SESSION POLICY my_policy SESSION_UI_IDLE_TIMEOUT_MINS = -1", "out of range"},
		{"Session Policy UI idle timeout above range", "CREATE SESSION POLICY my_policy SESSION_UI_IDLE_TIMEOUT_MINS = 56401", "out of range"},
		{"Session Policy unknown property", "CREATE SESSION POLICY my_policy INVALID_PROP = 60", "Unexpected property 'INVALID_PROP'"},

		// Invalid Password Policies
		{"Password Policy with prefix", "CREATE PASSWORD POLICY MY_DB.PUBLIC.bad_policy PASSWORD_MIN_LENGTH = 12", "account-level"},
		{"Password Policy OR REPLACE with prefix", "CREATE OR REPLACE PASSWORD POLICY MY_DB.PUBLIC.bad PASSWORD_MIN_LENGTH = 12", "account-level"},
		{"Password Policy min length below range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_LENGTH = 7", "below the minimum"},
		{"Password Policy min length above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_LENGTH = 257", "exceeds the maximum"},
		{"Password Policy max length below range", "CREATE PASSWORD POLICY my_policy PASSWORD_MAX_LENGTH = 7", "below the minimum"},
		{"Password Policy max length above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MAX_LENGTH = 257", "exceeds the maximum"},
		{"Password Policy max retries below range", "CREATE PASSWORD POLICY my_policy PASSWORD_MAX_RETRIES = 0", "below the minimum"},
		{"Password Policy max retries above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MAX_RETRIES = 11", "exceeds the maximum"},
		{"Password Policy lockout time below range", "CREATE PASSWORD POLICY my_policy PASSWORD_LOCKOUT_TIME_MINS = 0", "below the minimum"},
		{"Password Policy lockout time above range", "CREATE PASSWORD POLICY my_policy PASSWORD_LOCKOUT_TIME_MINS = 1000", "exceeds the maximum"},
		{"Password Policy max age above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MAX_AGE_DAYS = 1000", "exceeds the maximum"},
		{"Password Policy min age above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_AGE_DAYS = 1000", "exceeds the maximum"},
		{"Password Policy history above range", "CREATE PASSWORD POLICY my_policy PASSWORD_HISTORY = 25", "exceeds the maximum"},
		{"Password Policy history negative", "CREATE PASSWORD POLICY my_policy PASSWORD_HISTORY = -1", "below the minimum"},
		{"Password Policy min uppercase negative", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_UPPER_CASE_CHARS = -1", "below the minimum"},
		{"Password Policy min uppercase above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_UPPER_CASE_CHARS = 257", "exceeds the maximum"},
		{"Password Policy min lowercase above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_LOWER_CASE_CHARS = 257", "exceeds the maximum"},
		{"Password Policy min numeric above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_NUMERIC_CHARS = 257", "exceeds the maximum"},
		{"Password Policy min special above range", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_SPECIAL_CHARS = 257", "exceeds the maximum"},
		{"Password Policy max lt min length", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_LENGTH = 20 PASSWORD_MAX_LENGTH = 10", "greater than or equal to PASSWORD_MIN_LENGTH"},
		{"Password Policy min age gt max age", "CREATE PASSWORD POLICY my_policy PASSWORD_MIN_AGE_DAYS = 90 PASSWORD_MAX_AGE_DAYS = 30", "PASSWORD_MIN_AGE_DAYS"},
		{"Password Policy unknown property", "CREATE PASSWORD POLICY my_policy INVALID_PROP = TRUE", "Unexpected property 'INVALID_PROP'"},

		// Invalid CREATE SHARE
		{"Create Share with prefix", "CREATE SHARE db.schema.my_share", "account-level"},
		{"Create Share OR REPLACE with prefix", "CREATE OR REPLACE SHARE db.schema.my_share", "account-level"},
		{"Create Share invalid property", "CREATE SHARE my_share AUTO_REFRESH = TRUE", "Unexpected property 'AUTO_REFRESH'"},
		{"Create Share OR REPLACE and IF NOT EXISTS", "CREATE OR REPLACE SHARE IF NOT EXISTS my_share", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Create Share missing name", "CREATE SHARE", "Unexpected syntax"},

		// Invalid ALTER SHARE
		{"Alter Share RESTRICT without ADD ACCOUNTS", "ALTER SHARE my_share RESTRICT", "RESTRICT is only valid with ADD ACCOUNTS"},
		{"Alter Share REMOVE ACCOUNTS with RESTRICT", "ALTER SHARE my_share REMOVE ACCOUNTS = account1 RESTRICT", "RESTRICT is only valid with ADD ACCOUNTS"},
		{"Alter Share ADD ACCOUNTS missing account list", "ALTER SHARE my_share ADD ACCOUNTS =", "ADD ACCOUNTS requires at least one"},

		// Invalid CREATE DATABASE FROM SHARE — missing two-part provider name
		{"Create Database FROM SHARE one-part name", "CREATE DATABASE my_db FROM SHARE just_share_name", "Unexpected syntax"},
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
		// Aliased — qualified refs with valid columns must not warn
		"SELECT e.ID, e.FIRST_NAME FROM DB.SCH.EMPLOYEES e",
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

		// String literals containing identifier-like words must not be flagged
		// as unknown column refs (e.g. 'month' in DATE_TRUNC('month', ID)).
		`SELECT DATE_TRUNC('month', ID) AS m FROM DB.SCH.EMPLOYEES`,
		`SELECT TO_CHAR(ID, 'YYYY-MM-DD') AS d, FIRST_NAME FROM DB.SCH.EMPLOYEES`,
		// Regression tests for Issue: Date parts inside date functions triggering false warnings
		"SELECT DATEADD(month, -1, CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT DATE_TRUNC('month', CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT TIMESTAMPDIFF(second, CURRENT_DATE(), CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
		"SELECT EXTRACT(year FROM CURRENT_DATE()) FROM DB.SCH.EMPLOYEES",
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

		// Multi-CTE: all CTE names must be recognized, even those after the first comma
		"WITH cte1 AS (SELECT 1 AS id), cte2 AS (SELECT * FROM LIVE_TABLE) SELECT * FROM cte1 JOIN cte2 ON 1=1",
		"WITH a AS (SELECT 1), b AS (SELECT 2), c AS (SELECT 3) SELECT * FROM a JOIN b ON 1=1 JOIN c ON 1=1",
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

// ── 7. CREATE STAGE / ALTER STAGE Diagnostics (Issue #109) ───────────────────

// TestCreateStage_Valid covers the full CREATE STAGE syntax matrix: all
// modifiers, internal-stage params, external-stage params for S3/GCS/Azure/
// S3-compat/OneLake, FILE_FORMAT options, COPY_OPTIONS, DIRECTORY options.
// Each case must produce zero warnings (no false positives).
func TestCreateStage_Valid(t *testing.T) {
	valid := []string{
		// ── 1. Modifiers ─────────────────────────────────────────────────────
		"CREATE STAGE s",
		"CREATE OR REPLACE STAGE s",
		"CREATE TEMPORARY STAGE s",
		"CREATE OR REPLACE TEMPORARY STAGE s",
		"CREATE STAGE IF NOT EXISTS s",
		// ── 2. Object naming ─────────────────────────────────────────────────
		"CREATE STAGE db.schema.s",
		`CREATE STAGE "my stage"`,
		"CREATE OR REPLACE STAGE db.schema.my_stage",
		// ── 3. COMMENT only ──────────────────────────────────────────────────
		"CREATE STAGE s COMMENT = 'test stage'",
		"CREATE OR REPLACE STAGE s COMMENT = 'production stage'",
		// ── 4. URL variants ──────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://my-bucket/'",
		"CREATE STAGE s URL = 's3://my-bucket/my-prefix/'",
		"CREATE STAGE s URL = 'gcs://my-bucket/'",
		"CREATE STAGE s URL = 'gcs://my-bucket/path/'",
		"CREATE STAGE s URL = 'azure://myaccount.blob.core.windows.net/mycontainer/'",
		"CREATE STAGE s URL = 'azure://myaccount.blob.core.windows.net/mycontainer/path/'",
		"CREATE STAGE s URL = 's3compat://my-bucket/' ENDPOINT = 'storage.example.com'",
		"CREATE STAGE s URL = 'azure://onelake.blob.fabric.microsoft.com/ws-id/item-id/Files/'",
		// ── 5. STORAGE_INTEGRATION ───────────────────────────────────────────
		"CREATE STAGE s STORAGE_INTEGRATION = my_int",
		"CREATE STAGE s URL = 's3://bucket/' STORAGE_INTEGRATION = my_s3_int",
		"CREATE STAGE s URL = 'gcs://bucket/' STORAGE_INTEGRATION = my_gcs_int",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' STORAGE_INTEGRATION = my_az_int",
		// ── 6. CREDENTIALS ───────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'key' AWS_SECRET_KEY = 'secret')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's' AWS_TOKEN = 'tok')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_ROLE = 'arn:aws:iam::123:role/my-role')",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' CREDENTIALS = (AZURE_SAS_TOKEN = 'sas-token')",
		// ── 7. ENCRYPTION – internal stages ──────────────────────────────────
		"CREATE STAGE s ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL')",
		"CREATE STAGE s ENCRYPTION = (TYPE = 'SNOWFLAKE_SSE')",
		// ── 8. ENCRYPTION – S3 external stages ───────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'arn:aws:kms:us-east-1:123:key/id')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_CSE' MASTER_KEY = 'base64key==')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 9. ENCRYPTION – GCS external stages ──────────────────────────────
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS')",
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS' KMS_KEY_ID = 'projects/p/locations/l/keyRings/r/cryptoKeys/k')",
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 10. ENCRYPTION – Azure external stages ────────────────────────────
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' ENCRYPTION = (TYPE = 'AZURE_CSE' MASTER_KEY = 'base64key==')",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 11. FILE_FORMAT – by type (THE BUG CASE and variants) ─────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'AVRO')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'ORC')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML')",
		// The exact failing case from the issue
		"CREATE OR REPLACE STAGE my_internal_test_stage FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1) COMMENT = 'Internal stage for testing Snowpipe'",
		// ── 12. FILE_FORMAT – by name ─────────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (FORMAT_NAME = 'my_csv_format')",
		"CREATE STAGE s FILE_FORMAT = (FORMAT_NAME = 'db.schema.my_format')",
		// ── 13. FILE_FORMAT – CSV options ─────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' COMPRESSION = 'GZIP')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' RECORD_DELIMITER = '\n')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' NULL_IF = ('NULL', 'N/A'))",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_BLANK_LINES = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' TRIM_SPACE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ERROR_ON_COLUMN_COUNT_MISMATCH = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' DATE_FORMAT = 'YYYY-MM-DD')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' TIMESTAMP_FORMAT = 'YYYY-MM-DD HH24:MI:SS')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' BINARY_FORMAT = HEX)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ESCAPE = '\\')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_OPTIONALLY_ENCLOSED_BY = '\"')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' EMPTY_FIELD_AS_NULL = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_BYTE_ORDER_MARK = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ENCODING = 'UTF8')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' PARSE_HEADER = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' MULTI_LINE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FILE_EXTENSION = '.csv')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' REPLACE_INVALID_CHARACTERS = TRUE)",
		// ── 14. FILE_FORMAT – JSON options ────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' STRIP_OUTER_ARRAY = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' ALLOW_DUPLICATE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' STRIP_NULL_VALUES = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' ENABLE_OCTAL = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' IGNORE_UTF8_ERRORS = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' MULTI_LINE = TRUE)",
		// ── 15. FILE_FORMAT – PARQUET / AVRO / ORC / XML options ─────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' BINARY_AS_TEXT = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' COMPRESSION = 'SNAPPY')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' USE_LOGICAL_TYPE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' USE_VECTORIZED_SCANNER = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'AVRO' COMPRESSION = 'GZIP')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' IGNORE_UTF8_ERRORS = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' PRESERVE_SPACE = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' STRIP_OUTER_ELEMENT = TRUE)",
		// ── 16. COPY_OPTIONS ──────────────────────────────────────────────────
		"CREATE STAGE s COPY_OPTIONS = (ON_ERROR = CONTINUE)",
		"CREATE STAGE s COPY_OPTIONS = (ON_ERROR = ABORT_STATEMENT SIZE_LIMIT = 100)",
		"CREATE STAGE s COPY_OPTIONS = (FORCE = TRUE)",
		"CREATE STAGE s COPY_OPTIONS = (PURGE = TRUE)",
		// ── 17. DIRECTORY params – internal ───────────────────────────────────
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = FALSE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE REFRESH_ON_CREATE = FALSE)",
		// ── 18. DIRECTORY params – S3 external ───────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' DIRECTORY = (ENABLE = TRUE REFRESH_ON_CREATE = TRUE AUTO_REFRESH = TRUE)",
		// ── 19. DIRECTORY params – GCS external ──────────────────────────────
		"CREATE STAGE s URL = 'gcs://bucket/' DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE NOTIFICATION_INTEGRATION = 'my_notif_int')",
		// ── 20. DIRECTORY params – Azure external ─────────────────────────────
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE NOTIFICATION_INTEGRATION = 'my_notif_int')",
		// ── 21. AWS optional params ────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' AWS_ACCESS_POINT_ARN = 'arn:aws:s3:::accesspoint/my-ap'",
		"CREATE STAGE s URL = 's3://bucket/' USE_PRIVATELINK_ENDPOINT = TRUE",
		"CREATE STAGE s URL = 's3://bucket/' USE_PRIVATELINK_ENDPOINT = FALSE",
		// ── 22. S3-compatible ─────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3compat://bucket/' ENDPOINT = 'storage.example.com' CREDENTIALS = (AWS_KEY_ID = 'key' AWS_SECRET_KEY = 'secret')",
		// ── 23. Complex combined examples ─────────────────────────────────────
		"CREATE OR REPLACE STAGE my_s3_stage URL = 's3://my-bucket/path/' STORAGE_INTEGRATION = my_int FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1) DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE) COMMENT = 'S3 stage'",
		"CREATE STAGE my_gcs_stage URL = 'gcs://my-bucket/' STORAGE_INTEGRATION = gcs_int FILE_FORMAT = (TYPE = 'JSON') DIRECTORY = (ENABLE = TRUE) COMMENT = 'GCS stage'",
		"CREATE STAGE my_azure_stage URL = 'azure://myacct.blob.core.windows.net/mycontainer/' CREDENTIALS = (AZURE_SAS_TOKEN = 'token') FILE_FORMAT = (TYPE = 'CSV') COMMENT = 'Azure stage'",
		"CREATE STAGE internal_stage ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'internal encrypted'",
		"CREATE OR REPLACE TEMPORARY STAGE temp_stage FILE_FORMAT = (TYPE = 'CSV')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's') ENCRYPTION = (TYPE = 'AWS_SSE_S3') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'stage'",
		"CREATE STAGE s URL = 'azure://onelake.blob.fabric.microsoft.com/ws-id/item-id/Files/' STORAGE_INTEGRATION = my_onelake_int",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1 RECORD_DELIMITER = '\n' TRIM_SPACE = TRUE NULL_IF = ('NULL'))",
		"CREATE OR REPLACE STAGE full_stage URL = 's3://bucket/' STORAGE_INTEGRATION = si ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'kms_key') FILE_FORMAT = (TYPE = 'CSV') COPY_OPTIONS = (ON_ERROR = CONTINUE) DIRECTORY = (ENABLE = TRUE) COMMENT = 'full'",
	}

	for _, sql := range valid {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestCreateStage_Invalid covers CREATE STAGE statements with top-level
// property names that belong inside nested blocks (FILE_FORMAT, ENCRYPTION,
// DIRECTORY, CREDENTIALS, COPY_OPTIONS) or are simply not valid stage params.
// Each case must produce at least one warning.
func TestCreateStage_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		// Wrong top-level URL names
		{"Invalid BUCKET_URL", "CREATE STAGE s BUCKET_URL = 's3://bucket/'", "BUCKET_URL"},
		{"Invalid S3_URL", "CREATE STAGE s S3_URL = 's3://bucket/'", "S3_URL"},
		// Invalid non-stage params at top level
		{"Invalid REGION", "CREATE STAGE s URL = 's3://bucket/' REGION = 'us-east-1'", "REGION"},
		{"Invalid WAREHOUSE", "CREATE STAGE s WAREHOUSE = 'WH'", "WAREHOUSE"},
		{"Invalid MAX_FILE_SIZE", "CREATE STAGE s MAX_FILE_SIZE = 100", "MAX_FILE_SIZE"},
		// FILE_FORMAT sub-options placed at top level
		{"TYPE at top level", "CREATE STAGE s TYPE = 'CSV'", "TYPE"},
		{"FIELD_DELIMITER at top level", "CREATE STAGE s FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"SKIP_HEADER at top level", "CREATE STAGE s SKIP_HEADER = 1", "SKIP_HEADER"},
		{"COMPRESSION at top level", "CREATE STAGE s COMPRESSION = 'GZIP'", "COMPRESSION"},
		{"RECORD_DELIMITER at top level", "CREATE STAGE s RECORD_DELIMITER = '\n'", "RECORD_DELIMITER"},
		{"NULL_IF at top level", "CREATE STAGE s NULL_IF = ('NULL')", "NULL_IF"},
		{"DATE_FORMAT at top level", "CREATE STAGE s DATE_FORMAT = 'YYYY-MM-DD'", "DATE_FORMAT"},
		{"TIMESTAMP_FORMAT at top level", "CREATE STAGE s TIMESTAMP_FORMAT = 'AUTO'", "TIMESTAMP_FORMAT"},
		{"TRIM_SPACE at top level", "CREATE STAGE s TRIM_SPACE = TRUE", "TRIM_SPACE"},
		{"SKIP_BLANK_LINES at top level", "CREATE STAGE s SKIP_BLANK_LINES = TRUE", "SKIP_BLANK_LINES"},
		{"ERROR_ON_COLUMN_COUNT_MISMATCH at top level", "CREATE STAGE s ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE", "ERROR_ON_COLUMN_COUNT_MISMATCH"},
		{"STRIP_OUTER_ARRAY at top level", "CREATE STAGE s STRIP_OUTER_ARRAY = TRUE", "STRIP_OUTER_ARRAY"},
		{"BINARY_AS_TEXT at top level", "CREATE STAGE s BINARY_AS_TEXT = FALSE", "BINARY_AS_TEXT"},
		{"FORMAT_NAME at top level", "CREATE STAGE s FORMAT_NAME = 'my_format'", "FORMAT_NAME"},
		// ENCRYPTION sub-options placed at top level
		{"MASTER_KEY at top level", "CREATE STAGE s MASTER_KEY = 'key'", "MASTER_KEY"},
		{"KMS_KEY_ID at top level", "CREATE STAGE s KMS_KEY_ID = 'key-arn'", "KMS_KEY_ID"},
		// CREDENTIALS sub-options placed at top level
		{"AWS_KEY_ID at top level", "CREATE STAGE s AWS_KEY_ID = 'key'", "AWS_KEY_ID"},
		{"AWS_SECRET_KEY at top level", "CREATE STAGE s AWS_SECRET_KEY = 'secret'", "AWS_SECRET_KEY"},
		// DIRECTORY sub-options placed at top level
		{"ENABLE at top level", "CREATE STAGE s ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "CREATE STAGE s AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "CREATE STAGE s REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		// COPY_OPTIONS sub-options placed at top level
		{"ON_ERROR at top level", "CREATE STAGE s ON_ERROR = CONTINUE", "ON_ERROR"},
		{"SIZE_LIMIT at top level", "CREATE STAGE s SIZE_LIMIT = 100", "SIZE_LIMIT"},
		{"FORCE at top level", "CREATE STAGE s FORCE = TRUE", "FORCE"},
		{"PURGE at top level", "CREATE STAGE s PURGE = TRUE", "PURGE"},
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
				if strings.Contains(strings.ToUpper(w.Message), strings.ToUpper(tt.expectedMatch)) {
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

// TestAlterStage_Valid covers all ALTER STAGE forms: RENAME TO, SET (external
// params, FILE_FORMAT, COMMENT, DIRECTORY), REFRESH, SET TAG, UNSET TAG, and
// UNSET DCM PROJECT.  Each case must produce zero warnings.
func TestAlterStage_Valid(t *testing.T) {
	valid := []string{
		// ── 1. RENAME TO ─────────────────────────────────────────────────────
		"ALTER STAGE s RENAME TO new_s",
		"ALTER STAGE IF EXISTS s RENAME TO new_s",
		"ALTER STAGE my_db.my_schema.s RENAME TO new_s",
		`ALTER STAGE "my stage" RENAME TO "my new stage"`,
		// ── 2. SET COMMENT ───────────────────────────────────────────────────
		"ALTER STAGE s SET COMMENT = 'updated comment'",
		"ALTER STAGE IF EXISTS s SET COMMENT = 'updated'",
		"ALTER STAGE s SET COMMENT = ''",
		// ── 3. SET FILE_FORMAT – by type ─────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'AVRO')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'ORC')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'XML')",
		// ── 4. SET FILE_FORMAT – by name ─────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (FORMAT_NAME = 'my_format')",
		"ALTER STAGE s SET FILE_FORMAT = (FORMAT_NAME = 'db.schema.my_format')",
		// ── 5. SET FILE_FORMAT – CSV options (nested, not top-level) ─────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' COMPRESSION = 'GZIP')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' NULL_IF = ('NULL', 'N/A'))",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' SKIP_BLANK_LINES = TRUE TRIM_SPACE = TRUE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' DATE_FORMAT = 'YYYY-MM-DD' TIMESTAMP_FORMAT = 'YYYY-MM-DD HH24:MI:SS')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' ERROR_ON_COLUMN_COUNT_MISMATCH = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' RECORD_DELIMITER = '\n' FIELD_DELIMITER = ',' SKIP_HEADER = 1 TRIM_SPACE = TRUE)",
		// ── 6. SET FILE_FORMAT – JSON / PARQUET / XML options ────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON' STRIP_OUTER_ARRAY = TRUE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON' ALLOW_DUPLICATE = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET' BINARY_AS_TEXT = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET' COMPRESSION = 'SNAPPY')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'XML' IGNORE_UTF8_ERRORS = TRUE)",
		// ── 7. SET URL ────────────────────────────────────────────────────────
		"ALTER STAGE s SET URL = 's3://new-bucket/'",
		"ALTER STAGE s SET URL = 'gcs://new-bucket/'",
		"ALTER STAGE s SET URL = 'azure://newacct.blob.core.windows.net/newcont/'",
		// ── 8. SET STORAGE_INTEGRATION ────────────────────────────────────────
		"ALTER STAGE s SET STORAGE_INTEGRATION = new_int",
		"ALTER STAGE IF EXISTS s SET STORAGE_INTEGRATION = new_gcs_int",
		// ── 9. SET CREDENTIALS ────────────────────────────────────────────────
		"ALTER STAGE s SET CREDENTIALS = (AWS_KEY_ID = 'new_key' AWS_SECRET_KEY = 'new_secret')",
		"ALTER STAGE s SET CREDENTIALS = (AWS_ROLE = 'arn:aws:iam::123:role/new-role')",
		"ALTER STAGE s SET CREDENTIALS = (AZURE_SAS_TOKEN = 'new-sas-token')",
		// ── 10. SET ENCRYPTION ────────────────────────────────────────────────
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_KMS')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'new-key-arn')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'NONE')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'GCS_SSE_KMS' KMS_KEY_ID = 'gcs-key')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AZURE_CSE' MASTER_KEY = 'az-key==')",
		"ALTER STAGE IF EXISTS s SET ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'SNOWFLAKE_SSE')",
		// ── 11. SET DIRECTORY ─────────────────────────────────────────────────
		"ALTER STAGE s SET DIRECTORY = (ENABLE = TRUE)",
		"ALTER STAGE s SET DIRECTORY = (ENABLE = FALSE)",
		"ALTER STAGE IF EXISTS s SET DIRECTORY = (ENABLE = TRUE)",
		// ── 12. SET multiple properties ───────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV') COMMENT = 'updated stage'",
		"ALTER STAGE s SET URL = 's3://new-bucket/' STORAGE_INTEGRATION = new_int",
		"ALTER STAGE s SET URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's') ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET COMMENT = 'test' FILE_FORMAT = (TYPE = 'CSV')",
		"ALTER STAGE s SET DIRECTORY = (ENABLE = TRUE) COMMENT = 'with directory'",
		"ALTER STAGE IF EXISTS my_s3_stage SET URL = 's3://new-bucket/' STORAGE_INTEGRATION = new_si ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'key') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'updated'",
		// ── 13. SET AWS optional params ───────────────────────────────────────
		"ALTER STAGE s SET AWS_ACCESS_POINT_ARN = 'arn:aws:s3:::accesspoint/my-ap'",
		"ALTER STAGE s SET USE_PRIVATELINK_ENDPOINT = TRUE",
		"ALTER STAGE s SET USE_PRIVATELINK_ENDPOINT = FALSE",
		// ── 14. SET COPY_OPTIONS ──────────────────────────────────────────────
		"ALTER STAGE s SET COPY_OPTIONS = (ON_ERROR = CONTINUE)",
		"ALTER STAGE s SET COPY_OPTIONS = (ON_ERROR = SKIP_FILE SIZE_LIMIT = 100)",
		// ── 15. REFRESH ───────────────────────────────────────────────────────
		"ALTER STAGE s REFRESH",
		"ALTER STAGE IF EXISTS s REFRESH",
		"ALTER STAGE s REFRESH SUBPATH = 'my/subfolder/'",
		"ALTER STAGE IF EXISTS s REFRESH SUBPATH = 'data/'",
		// ── 16. TAG operations – skipped (dynamic tag names) ──────────────────
		"ALTER STAGE s UNSET TAG my_tag",
		"ALTER STAGE s UNSET TAG tag1, tag2",
		"ALTER STAGE s UNSET DCM PROJECT",
		"ALTER STAGE s SET TAG my_tag = 'my_value'",
		"ALTER STAGE s SET TAG tag1 = 'val1', tag2 = 'val2'",
		"ALTER STAGE IF EXISTS s SET TAG dept = 'finance'",
		// ── 17. Quoted stage names ─────────────────────────────────────────────
		`ALTER STAGE "my stage" SET COMMENT = 'quoted name'`,
		`ALTER STAGE IF EXISTS "My Stage" REFRESH`,
	}

	for _, sql := range valid {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestAlterStage_Invalid covers ALTER STAGE SET statements that use sub-option
// property names at the top level instead of inside the correct nested block.
func TestAlterStage_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		// Wrong URL-like names
		{"Invalid BUCKET_URL in SET", "ALTER STAGE s SET BUCKET_URL = 's3://bucket/'", "BUCKET_URL"},
		{"Invalid REGION in SET", "ALTER STAGE s SET REGION = 'us-east-1'", "REGION"},
		// FILE_FORMAT sub-options at top level
		{"TYPE at top level in SET", "ALTER STAGE s SET TYPE = 'CSV'", "TYPE"},
		{"FIELD_DELIMITER at top level", "ALTER STAGE s SET FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"SKIP_HEADER at top level", "ALTER STAGE s SET SKIP_HEADER = 1", "SKIP_HEADER"},
		{"COMPRESSION at top level", "ALTER STAGE s SET COMPRESSION = 'GZIP'", "COMPRESSION"},
		{"RECORD_DELIMITER at top level", "ALTER STAGE s SET RECORD_DELIMITER = '\n'", "RECORD_DELIMITER"},
		{"NULL_IF at top level", "ALTER STAGE s SET NULL_IF = ('NULL')", "NULL_IF"},
		{"DATE_FORMAT at top level", "ALTER STAGE s SET DATE_FORMAT = 'YYYY-MM-DD'", "DATE_FORMAT"},
		{"TRIM_SPACE at top level", "ALTER STAGE s SET TRIM_SPACE = TRUE", "TRIM_SPACE"},
		{"SKIP_BLANK_LINES at top level", "ALTER STAGE s SET SKIP_BLANK_LINES = TRUE", "SKIP_BLANK_LINES"},
		{"STRIP_OUTER_ARRAY at top level", "ALTER STAGE s SET STRIP_OUTER_ARRAY = TRUE", "STRIP_OUTER_ARRAY"},
		{"BINARY_AS_TEXT at top level", "ALTER STAGE s SET BINARY_AS_TEXT = FALSE", "BINARY_AS_TEXT"},
		{"FORMAT_NAME at top level", "ALTER STAGE s SET FORMAT_NAME = 'my_fmt'", "FORMAT_NAME"},
		{"ERROR_ON_COLUMN_COUNT_MISMATCH at top", "ALTER STAGE s SET ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE", "ERROR_ON_COLUMN_COUNT_MISMATCH"},
		{"IGNORE_UTF8_ERRORS at top level", "ALTER STAGE s SET IGNORE_UTF8_ERRORS = TRUE", "IGNORE_UTF8_ERRORS"},
		{"SKIP_BYTE_ORDER_MARK at top level", "ALTER STAGE s SET SKIP_BYTE_ORDER_MARK = TRUE", "SKIP_BYTE_ORDER_MARK"},
		{"ALLOW_DUPLICATE at top level", "ALTER STAGE s SET ALLOW_DUPLICATE = TRUE", "ALLOW_DUPLICATE"},
		{"REPLACE_INVALID_CHARACTERS at top", "ALTER STAGE s SET REPLACE_INVALID_CHARACTERS = TRUE", "REPLACE_INVALID_CHARACTERS"},
		// ENCRYPTION sub-options at top level
		{"MASTER_KEY at top level", "ALTER STAGE s SET MASTER_KEY = 'key'", "MASTER_KEY"},
		{"KMS_KEY_ID at top level", "ALTER STAGE s SET KMS_KEY_ID = 'key-arn'", "KMS_KEY_ID"},
		// CREDENTIALS sub-options at top level
		{"AWS_KEY_ID at top level", "ALTER STAGE s SET AWS_KEY_ID = 'key'", "AWS_KEY_ID"},
		// DIRECTORY sub-options at top level
		{"ENABLE at top level in SET", "ALTER STAGE s SET ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "ALTER STAGE s SET AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "ALTER STAGE s SET REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		// COPY_OPTIONS sub-options at top level
		{"ON_ERROR at top level", "ALTER STAGE s SET ON_ERROR = CONTINUE", "ON_ERROR"},
		{"SIZE_LIMIT at top level", "ALTER STAGE s SET SIZE_LIMIT = 100", "SIZE_LIMIT"},
		// XML sub-options at top level
		{"PRESERVE_SPACE at top level", "ALTER STAGE s SET PRESERVE_SPACE = TRUE", "PRESERVE_SPACE"},
		{"STRIP_OUTER_ELEMENT at top level", "ALTER STAGE s SET STRIP_OUTER_ELEMENT = TRUE", "STRIP_OUTER_ELEMENT"},
		// Invalid REFRESH parameter
		{"Invalid REFRESH param", "ALTER STAGE s REFRESH MAX_SIZE = 100", "MAX_SIZE"},
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
				if strings.Contains(strings.ToUpper(w.Message), strings.ToUpper(tt.expectedMatch)) {
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

func TestValidateSnowflakePatterns_CreateIcebergTable(t *testing.T) {
	validCases := []string{
		// Snowflake-managed
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://my-snowflake-bucket/'",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://another-bucket/' CLUSTER BY (id) DATA_RETENTION_TIME_IN_DAYS = 1",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://comment-bucket/' COMMENT = 'CLUSTER BY is a table property'",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://transient-comment/' COMMENT = 'TRANSIENT tables are not supported'",
		"CREATE ICEBERG TABLE t1 (id int) EXTERNAL_VOLUME = 'my_ev' CATALOG = 'my_cat' BASE_LOCATION = 's3://external-bucket/'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'loc' COMMENT = 'CLUSTER BY is not applicable'",
		"CREATE ICEBERG TABLE transient (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'",
		"CREATE ICEBERG TABLE t (id int) CATALOG = 'snowflake' BASE_LOCATION = 's3://test/'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = my_ev CATALOG = 'my_cat' BASE_LOCATION = 's3://bucket/'",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{"OR REPLACE IF NOT EXISTS Iceberg", "CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'", []string{"Conflict between OR REPLACE and IF NOT EXISTS"}},
		{"Transient keyword used", "CREATE TRANSIENT ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'", []string{"TRANSIENT is not supported for Iceberg tables."}},
		{"Missing BASE_LOCATION for Snowflake-managed", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE'", []string{"BASE_LOCATION is mandatory for all Iceberg tables and cannot be empty."}},
		{"Empty BASE_LOCATION for Snowflake-managed", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = ''", []string{"BASE_LOCATION is mandatory for all Iceberg tables and cannot be empty."}},
		{"Missing EXTERNAL_VOLUME", "CREATE ICEBERG TABLE t (id int) CATALOG = 'c' BASE_LOCATION = 'l'", []string{"EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs."}},
		{"Missing CATALOG", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' BASE_LOCATION = 'l'", []string{"CATALOG is mandatory for Iceberg tables with external catalogs."}},
		{"Empty EXTERNAL_VOLUME", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = '' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs."}},
		{"Empty CATALOG", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = '' BASE_LOCATION = 'l'", []string{"CATALOG is mandatory for Iceberg tables with external catalogs."}},
		{"CATALOG_TABLE_NAME with SNOWFLAKE", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/' CATALOG_TABLE_NAME = 'ctn'", []string{"CATALOG_TABLE_NAME is only valid when CATALOG is not 'SNOWFLAKE'"}},
		{"CATALOG_NAMESPACE with SNOWFLAKE", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/' CATALOG_NAMESPACE = 'cns'", []string{"CATALOG_NAMESPACE is only valid when CATALOG is not 'SNOWFLAKE'"}},
		{"OR REPLACE with external catalog", "CREATE OR REPLACE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
		{"CLUSTER BY with external catalog", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' CLUSTER BY (id)", []string{"CLUSTER BY is supported only for Snowflake-managed Iceberg tables."}},
		{"DATA_RETENTION_TIME_IN_DAYS with external catalog", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' DATA_RETENTION_TIME_IN_DAYS = 1", []string{"DATA_RETENTION_TIME_IN_DAYS applies only to Snowflake-managed Iceberg tables."}},
		{"Invalid REFRESH_MODE", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'INVALID'", []string{"Invalid REFRESH_MODE value. Must be AUTO, FULL, or INCREMENTAL."}},
		{"Invalid INITIALIZE", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = 'INVALID'", []string{"Invalid INITIALIZE value. Must be ON_CREATE or ON_SCHEDULE."}},
		{"Invalid AUTO_REFRESH", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' AUTO_REFRESH = 'INVALID'", []string{"AUTO_REFRESH must be TRUE or FALSE."}},
		{"Quoted AUTO_REFRESH Invalid", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'cat' BASE_LOCATION = 'loc' AUTO_REFRESH = 'BAD'", []string{"AUTO_REFRESH must be TRUE or FALSE."}},
		{"Quoted REPLACE_INVALID_CHARACTERS Invalid", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'cat' BASE_LOCATION = 'loc' REPLACE_INVALID_CHARACTERS = 'BAD'", []string{"REPLACE_INVALID_CHARACTERS must be TRUE or FALSE."}},
		{"OR REPLACE IF NOT EXISTS External Catalog", "CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"Conflict between OR REPLACE and IF NOT EXISTS", "OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
		{"CREATE OR REPLACE TRANSIENT ICEBERG TABLE", "CREATE OR REPLACE TRANSIENT ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"TRANSIENT is not supported for Iceberg tables.", "OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
			if len(warns) < len(tt.wantMsgs) {
				t.Errorf("Expected %d warnings, got %d for %q", len(tt.wantMsgs), len(warns), tt.sql)
			}
		})
	}
}

func TestValidateSnowflakePatterns_CreateHybridTable(t *testing.T) {
	validCases := []string{
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL PRIMARY KEY, val VARCHAR INDEX idx_val (val))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, val VARCHAR) COMMENT = 'test'",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, c2 INT, CONSTRAINT fk_c2 FOREIGN KEY (c2) REFERENCES t2(id))",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, val VARCHAR, PRIMARY KEY (id))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, val VARCHAR NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL) COMMENT = 'no cluster by here'",
		"CREATE TABLE t1 (id INT, val VARCHAR DEFAULT 'INDEX is not supported here')",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, val VARCHAR DEFAULT 'PRIMARY KEY', PRIMARY KEY (id))",
		"CREATE HYBRID TABLE IF NOT EXISTS t1 (id INT PRIMARY KEY NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, CONSTRAINT pk1 PRIMARY KEY (id))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY AUTOINCREMENT)",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY IDENTITY (1, 1))",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{"Missing Primary Key", "CREATE HYBRID TABLE t1 (id INT)", []string{"Hybrid tables must have a PRIMARY KEY"}},
		{"Cluster By not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) CLUSTER BY (id)", []string{"CLUSTER BY is not supported on hybrid tables"}},
		{"Data Retention not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) DATA_RETENTION_TIME_IN_DAYS = 7", []string{"DATA_RETENTION_TIME_IN_DAYS is not applicable to hybrid tables"}},
		{"Change Tracking not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) CHANGE_TRACKING = TRUE", []string{"CHANGE_TRACKING is not supported on hybrid tables"}},
		{"Transient not supported", "CREATE TRANSIENT HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"TRANSIENT is not supported for hybrid tables"}},
		{"TRANSIENT + missing PK", "CREATE TRANSIENT HYBRID TABLE t1 (id INT)", []string{"TRANSIENT is not supported for hybrid tables", "Hybrid tables must have a PRIMARY KEY"}},
		{"OR REPLACE not supported", "CREATE OR REPLACE HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"OR REPLACE is not supported for hybrid tables"}},
		{"COPY GRANTS not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) COPY GRANTS", []string{"COPY GRANTS is not supported on hybrid tables"}},
		{"Index on regular table", "CREATE TABLE t1 (id INT PRIMARY KEY, val VARCHAR INDEX idx_val (val))", []string{"Secondary indexes (INDEX) are only supported on hybrid tables"}},
		{"PK column missing NOT NULL (out of line)", "CREATE HYBRID TABLE t1 (id INT, PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"PK column missing NOT NULL (inline)", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"PK column missing NOT NULL (out of line, extra spaces)", "CREATE HYBRID TABLE t1 (id INT, PRIMARY  KEY  (id))", []string{"Primary key columns in a hybrid table must be NOT NULL (column 'ID' omits it)."}},
		{"Composite PK missing NOT NULL on one column", "CREATE HYBRID TABLE t1 (id INT NOT NULL, name INT, PRIMARY KEY (id, name))", []string{"Primary key columns in a hybrid table must be NOT NULL (column 'NAME' omits it)."}},
		{"Constraint-named PK missing NOT NULL", "CREATE HYBRID TABLE t1 (id INT, CONSTRAINT pk1 PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"string literal containing NOT NULL suppresses false negative", "CREATE HYBRID TABLE t1 (id INT DEFAULT 'NOT NULL here', PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
		})
	}
}

func TestValidateSnowflakePatterns_CreateExternalVolume(t *testing.T) {
	validCases := []string{
		// Minimal valid S3 volume
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'my_s3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://bucket/path' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/r' )) ALLOW_WRITES = TRUE",
		// S3GOV provider
		"CREATE EXTERNAL VOLUME gov_vol STORAGE_LOCATIONS = (( NAME = 'gov' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://gov-bucket/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::123:role/r' ))",
		// S3CHINA provider
		"CREATE EXTERNAL VOLUME cn_vol STORAGE_LOCATIONS = (( NAME = 'cn' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://cn-bucket/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::123:role/r' ))",
		// S3COMPAT provider (S3-compatible storage)
		"CREATE EXTERNAL VOLUME compat_vol STORAGE_LOCATIONS = (( NAME = 'compat' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://endpoint/bucket' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// GCS provider
		"CREATE EXTERNAL VOLUME gcs_vol STORAGE_LOCATIONS = (( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://bucket/path' ))",
		// AZURE provider
		"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/path' AZURE_TENANT_ID = 'tenant-id' ))",
		// OR REPLACE is valid for external volumes
		"CREATE OR REPLACE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// IF NOT EXISTS
		"CREATE EXTERNAL VOLUME IF NOT EXISTS my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// S3 with optional STORAGE_AWS_EXTERNAL_ID
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3COMPAT with STORAGE_AWS_EXTERNAL_ID (same as S3)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3 with ENCRYPTION TYPE = NONE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'NONE') ))",
		// S3 with ENCRYPTION TYPE = AWS_SSE_S3
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
		// S3 with ENCRYPTION TYPE = AWS_SSE_KMS
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
		// GCS with ENCRYPTION TYPE = GCS_SSE_KMS
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
		// ALLOW_WRITES = FALSE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' )) ALLOW_WRITES = FALSE",
		// COMMENT property
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) COMMENT = 'my volume'",
		// Multi-provider: S3 + GCS — both ARN (for S3) and GCS_SSE_KMS (for GCS) valid, no AZURE_TENANT_ID error
		"CREATE EXTERNAL VOLUME multi_vol STORAGE_LOCATIONS = (( NAME = 's3loc' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcsloc' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))",
		// Multi-provider: S3 + AZURE — STORAGE_AWS_EXTERNAL_ID remains valid (hasS3=true)
		"CREATE EXTERNAL VOLUME multi_vol STORAGE_LOCATIONS = (( NAME = 's3loc' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'eid' ) ( NAME = 'azloc' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ))",
		// Lowercase provider string — case-insensitive match should accept 's3'
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 's3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// Quoted volume name containing a dot — should NOT flag account-level prefix
		`CREATE EXTERNAL VOLUME "my.vol" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
		// Quoted reserved-keyword volume name — identPath regex must handle double-quoted identifiers
		`CREATE EXTERNAL VOLUME "S3" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
		// ALLOW_WRITES in a line comment must not trigger a false positive
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) -- ALLOW_WRITES = maybe",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"Missing STORAGE_LOCATIONS",
			"CREATE EXTERNAL VOLUME my_vol ALLOW_WRITES = TRUE",
			[]string{"STORAGE_LOCATIONS is mandatory"},
		},
		{
			"Missing STORAGE_PROVIDER",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"Invalid STORAGE_PROVIDER",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"Invalid STORAGE_PROVIDER 'INVALID'"},
		},
		{
			"Missing STORAGE_BASE_URL",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Each storage location requires STORAGE_BASE_URL"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3GOV",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3CHINA",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3COMPAT",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing AZURE_TENANT_ID for AZURE",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' ))",
			[]string{"AZURE_TENANT_ID is required for AZURE"},
		},
		{
			"STORAGE_AWS_EXTERNAL_ID with non-S3 provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' STORAGE_AWS_EXTERNAL_ID = 'id' ))",
			[]string{"STORAGE_AWS_EXTERNAL_ID is only valid for S3"},
		},
		{
			"Invalid ENCRYPTION TYPE",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'INVALID_ENC') ))",
			[]string{"Invalid ENCRYPTION TYPE 'INVALID_ENC'"},
		},
		{
			"AWS_SSE_S3 encryption with GCS provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
			[]string{"ENCRYPTION TYPE 'AWS_SSE_S3' is only valid for S3"},
		},
		{
			"AWS_SSE_KMS encryption with GCS provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'AWS_SSE_KMS' is only valid for S3"},
		},
		{
			"GCS_SSE_KMS encryption with S3 provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"Invalid ALLOW_WRITES value",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = MAYBE",
			[]string{"ALLOW_WRITES must be TRUE or FALSE"},
		},
		{
			"Account-level prefix not allowed",
			"CREATE EXTERNAL VOLUME mydb.myschema.my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"OR REPLACE and IF NOT EXISTS conflict",
			"CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"Cross-provider: GCS_SSE_KMS on S3 location when GCS also present",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"Cross-provider: STORAGE_AWS_EXTERNAL_ID on GCS location when S3 also present",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' STORAGE_AWS_EXTERNAL_ID = 'id' ))",
			[]string{"STORAGE_AWS_EXTERNAL_ID is only valid for S3"},
		},
		{
			"AZURE location with invalid ENCRYPTION TYPE",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'AZURE_CSE') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"AZURE location with AWS encryption type",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"AZURE location with ENCRYPTION TYPE = NONE",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'NONE') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"Missing NAME in location block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Each storage location requires a NAME attribute"},
		},
		{
			"Empty STORAGE_LOCATIONS block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ()",
			[]string{"STORAGE_LOCATIONS must contain at least one storage location block"},
		},
		{
			"OR REPLACE and IF NOT EXISTS returns early without extra markers",
			"CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol ALLOW_WRITES = TRUE",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
		})
	}

	// Verify that the OR REPLACE + IF NOT EXISTS conflict triggers exactly one
	// warning (proving the early return works and no additional checks run).
	t.Run("OR REPLACE and IF NOT EXISTS emits exactly one marker", func(t *testing.T) {
		sql := "CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol ALLOW_WRITES = TRUE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (early return), got %d: %v", len(warns), warns)
		}
	})
}
