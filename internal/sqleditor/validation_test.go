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
		// ALTER DYNAMIC TABLE — comprehensive tests in TestValidateSnowflakePatterns_AlterDynamicTable
		"ALTER DYNAMIC TABLE my_dt REFRESH",
		"ALTER DYNAMIC TABLE my_dt SUSPEND",
		"ALTER DYNAMIC TABLE my_dt RESUME",
		"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute'",
		"ALTER DYNAMIC TABLE my_dt SET WAREHOUSE = my_wh",
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
		// Tags — comprehensive tests in TestValidateSnowflakePatterns_Tag
		"CREATE TAG my_tag",
		"ALTER TAG my_tag RENAME TO new_tag",
		"DROP TAG my_tag",
		// Notebooks — comprehensive tests in TestValidateSnowflakePatterns_Notebook
		"CREATE NOTEBOOK my_nb",
		"CREATE OR REPLACE NOTEBOOK my_nb",
		"CREATE NOTEBOOK IF NOT EXISTS db.schema.my_nb",
		"ALTER NOTEBOOK my_nb SET COMMENT = 'My notebook'",
		"ALTER NOTEBOOK my_nb RENAME TO new_nb",
		"DROP NOTEBOOK my_nb",
		"DROP NOTEBOOK IF EXISTS my_nb",
		// ALTER TABLE SEARCH OPTIMIZATION — comprehensive tests in TestValidateSnowflakePatterns_AlterTableSearchOptimization
		"ALTER TABLE my_table ADD SEARCH OPTIMIZATION",
		"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2)",
		"ALTER TABLE t DROP SEARCH OPTIMIZATION",
		// ALTER TABLE SWAP WITH — comprehensive tests in TestValidateSnowflakePatterns_AlterTableSwapWith
		"ALTER TABLE orders SWAP WITH orders_backup",
		"ALTER TABLE db1.schema1.t1 SWAP WITH db1.schema1.t2",
		"ALTER TABLE IF EXISTS t1 SWAP WITH t2",
		// SHOW statements — comprehensive tests in TestValidateSnowflakePatterns_Show
		// False Positive Guards (Should be silently ignored, 0 warnings)
		"DELETE FROM t WHERE id = 1",
		"GRANT SELECT ON t TO ROLE r",
		"CREATE STAGE s",
		"ALTER WAREHOUSE wh RESUME",
		"SELECT * FROM t TABLESAMPLE (10 ROWS)",
		// INSERT ALL / INSERT FIRST / INSERT OVERWRITE — comprehensive tests in TestValidateSnowflakePatterns_InsertAllFirstOverwrite
		"INSERT ALL INTO t1 INTO t2 SELECT id FROM source",
		"INSERT ALL WHEN x > 0 THEN INTO t1 ELSE INTO t2 SELECT id, x FROM source",
		"INSERT FIRST WHEN x > 0 THEN INTO t1 ELSE INTO t2 SELECT id, x FROM source",
		"INSERT OVERWRITE INTO t1 SELECT * FROM source",
		// PIVOT / UNPIVOT — comprehensive tests in TestValidateSnowflakePatterns_Pivot / TestValidateSnowflakePatterns_Unpivot
		"SELECT * FROM t PIVOT (SUM(amount) FOR month IN ('Jan', 'Feb', 'Mar')) AS p",
		"SELECT * FROM t UNPIVOT (value FOR metric IN (col_a, col_b, col_c))",
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
		// Aggregation Policies
		"CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)",
		"CREATE OR REPLACE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 1000000)",
		"CREATE AGGREGATION POLICY IF NOT EXISTS my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 1)",
		"CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> NO_AGGREGATION_CONSTRAINT()",
		"CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5) COMMENT = 'minimum 5'",
		"CREATE AGGREGATION POLICY db.sch.my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 10)",
		// Projection Policies
		"CREATE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'none')",
		"CREATE OR REPLACE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'transformation')",
		"CREATE PROJECTION POLICY IF NOT EXISTS my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'NONE')",
		"CREATE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> NO_PROJECTION_CONSTRAINT()",
		"CREATE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'TRANSFORMATION') COMMENT = 'block direct'",
		"CREATE PROJECTION POLICY db.sch.my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> NO_PROJECTION_CONSTRAINT()",
		// ALTER / DROP Aggregation Policy
		"ALTER AGGREGATION POLICY my_agg_policy SET BODY -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 10)",
		"ALTER AGGREGATION POLICY my_agg_policy SET COMMENT = 'updated'",
		"ALTER AGGREGATION POLICY my_agg_policy UNSET COMMENT",
		"ALTER AGGREGATION POLICY my_agg_policy RENAME TO new_agg_policy",
		"DROP AGGREGATION POLICY my_agg_policy",
		"DROP AGGREGATION POLICY IF EXISTS my_agg_policy",
		// ALTER / DROP Projection Policy
		"ALTER PROJECTION POLICY my_proj_policy SET BODY -> PROJECTION_CONSTRAINT(ALLOW => 'none')",
		"ALTER PROJECTION POLICY my_proj_policy SET COMMENT = 'updated'",
		"ALTER PROJECTION POLICY my_proj_policy UNSET COMMENT",
		"ALTER PROJECTION POLICY my_proj_policy RENAME TO new_proj_policy",
		"DROP PROJECTION POLICY my_proj_policy",
		"DROP PROJECTION POLICY IF EXISTS my_proj_policy",
		// Packages Policies
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON",
		"CREATE OR REPLACE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON",
		"CREATE PACKAGES POLICY IF NOT EXISTS my_pkg_policy LANGUAGE PYTHON",
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON ALLOWLIST = ('numpy', 'pandas==1.5.3')",
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON BLOCKLIST = ('os', 'sys')",
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON COMMENT = 'restrict packages'",
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON ALLOWLIST = ('requests==2.28.0') COMMENT = 'allow requests'",
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE python",
		// Neither ALLOWLIST nor BLOCKLIST is valid (uses default Anaconda list)
		"CREATE PACKAGES POLICY default_anaconda_policy LANGUAGE PYTHON",
		// ALLOWLIST and BLOCKLIST can coexist (BLOCKLIST overrides)
		"CREATE PACKAGES POLICY my_pkg_policy LANGUAGE PYTHON ALLOWLIST = ('numpy') BLOCKLIST = ('os')",
		// Schema-level: qualified names are valid
		"CREATE PACKAGES POLICY my_db.my_schema.my_policy LANGUAGE PYTHON",
		"CREATE OR REPLACE PACKAGES POLICY my_db.my_schema.my_policy LANGUAGE PYTHON ALLOWLIST = ('numpy')",
		// ALTER / DROP Packages Policy
		"ALTER PACKAGES POLICY my_pkg_policy SET ALLOWLIST = ('numpy')",
		"ALTER PACKAGES POLICY IF EXISTS my_pkg_policy SET COMMENT = 'test'",
		"ALTER PACKAGES POLICY my_db.my_schema.my_policy SET BLOCKLIST = ('os')",
		"ALTER PACKAGES POLICY my_pkg_policy SET BLOCKLIST = ('os')",
		"ALTER PACKAGES POLICY my_pkg_policy SET ADDITIONAL_CREATION_BLOCKLIST = ('os')",
		"ALTER PACKAGES POLICY my_pkg_policy SET COMMENT = 'updated'",
		"ALTER PACKAGES POLICY my_pkg_policy UNSET ALLOWLIST",
		"ALTER PACKAGES POLICY my_pkg_policy UNSET BLOCKLIST",
		"ALTER PACKAGES POLICY my_pkg_policy UNSET ADDITIONAL_CREATION_BLOCKLIST",
		"ALTER PACKAGES POLICY my_pkg_policy UNSET COMMENT",
		"DROP PACKAGES POLICY my_pkg_policy",
		"DROP PACKAGES POLICY IF EXISTS my_pkg_policy",
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
		// CREATE EVENT TABLE — valid
		"CREATE EVENT TABLE my_events",
		"CREATE OR REPLACE EVENT TABLE my_events",
		"CREATE EVENT TABLE IF NOT EXISTS my_events",
		"CREATE EVENT TABLE my_events COMMENT = 'telemetry data'",
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 30",
		"CREATE EVENT TABLE my_events CHANGE_TRACKING = TRUE",
		// CREATE SHARE — valid
		"CREATE SHARE my_share",
		"CREATE OR REPLACE SHARE my_share",
		"CREATE SHARE IF NOT EXISTS my_share",
		"CREATE SHARE my_share COMMENT = 'description of the share'",
		"CREATE OR REPLACE SHARE my_share COMMENT = 'updated'",
		// "IF NOT EXISTS" inside a COMMENT value must not trigger the conflict warning.
		"CREATE OR REPLACE SHARE my_share COMMENT = 'IF NOT EXISTS hint'",
		// CREATE DATASHARE — valid
		"CREATE DATASHARE my_datashare",
		"CREATE OR REPLACE DATASHARE my_datashare",
		"CREATE DATASHARE IF NOT EXISTS my_datashare",
		"CREATE DATASHARE my_datashare COMMENT = 'provider share'",
		"CREATE DATASHARE my_datashare SHARE_RESTRICTIONS = TRUE",
		"CREATE DATASHARE my_datashare SHARE_RESTRICTIONS = FALSE",
		"CREATE DATASHARE my_datashare SHARE_RESTRICTIONS = TRUE COMMENT = 'desc'",
		"CREATE OR REPLACE DATASHARE my_datashare COMMENT = 'IF NOT EXISTS hint'",
		// ALTER DATASHARE — valid
		"ALTER DATASHARE my_ds ADD ACCOUNTS = org1.acct1",
		"ALTER DATASHARE my_ds ADD ACCOUNTS = org1.acct1, org2.acct2",
		"ALTER DATASHARE my_ds ADD ACCOUNTS = org1.acct1 SHARE_RESTRICTIONS = TRUE",
		"ALTER DATASHARE my_ds REMOVE ACCOUNTS = org1.acct1",
		"ALTER DATASHARE my_ds REMOVE ACCOUNTS = org1.acct1, org2.acct2",
		"ALTER DATASHARE my_ds ADD DATABASES db1",
		"ALTER DATASHARE my_ds ADD DATABASES db1, db2",
		"ALTER DATASHARE my_ds REMOVE DATABASES db1",
		"ALTER DATASHARE my_ds REMOVE DATABASES db1, db2",
		"ALTER DATASHARE my_ds SET COMMENT = 'updated'",
		"ALTER DATASHARE my_ds UNSET COMMENT",
		// DROP DATASHARE — valid
		"DROP DATASHARE my_datashare",
		"DROP DATASHARE IF EXISTS my_datashare",
		// CREATE COMPUTE POOL — valid
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S",
		"CREATE OR REPLACE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = CPU_X64_XS",
		"CREATE COMPUTE POOL IF NOT EXISTS my_pool MIN_NODES = 2 MAX_NODES = 5 INSTANCE_FAMILY = GPU_NV_M",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_RESUME = TRUE",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_RESUME = FALSE",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_SUSPEND_SECS = 300",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_SUSPEND_SECS = 0",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S COMMENT = 'my pool'",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S INITIALLY_SUSPENDED = TRUE",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S INITIALLY_SUSPENDED = FALSE",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 10 INSTANCE_FAMILY = HIGHMEM_X64_S AUTO_RESUME = TRUE AUTO_SUSPEND_SECS = 600 COMMENT = 'high mem' INITIALLY_SUSPENDED = TRUE",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_L",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_XL",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_M",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = HIGHMEM_X64_M",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = HIGHMEM_X64_L",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = HIGHMEM_X64_SL",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = GPU_NV_S",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = GPU_NV_L",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = GPU_NV_XL",
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = GPU_NV_4XL",
		// INSTANCE_FAMILY is case-insensitive
		"CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = cpu_x64_s",
		// COMMENT with IF NOT EXISTS hint inside string should not trigger conflict
		"CREATE OR REPLACE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = CPU_X64_XS COMMENT = 'IF NOT EXISTS hint'",
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
		// Time Travel — valid AT/BEFORE clauses
		"SELECT * FROM orders AT (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders AT (OFFSET => -3600)",
		"SELECT * FROM orders AT (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders BEFORE (OFFSET => -3600)",
		"SELECT * FROM orders AT (STREAM => my_stream)",
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

		// Invalid Aggregation Policies
		{"Agg Policy OR REPLACE with IF NOT EXISTS", "CREATE OR REPLACE AGGREGATION POLICY IF NOT EXISTS my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Agg Policy missing AS", "CREATE AGGREGATION POLICY my_agg_policy RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)", "Missing mandatory AS"},
		{"Agg Policy missing RETURNS", "CREATE AGGREGATION POLICY my_agg_policy AS () -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)", "Missing mandatory RETURNS AGGREGATION_CONSTRAINT"},
		{"Agg Policy missing arrow", "CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)", "Missing mandatory '->'"},
		{"Agg Policy MIN_GROUP_SIZE zero", "CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 0)", "must be between 1 and 1000000"},
		{"Agg Policy MIN_GROUP_SIZE negative", "CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => -1)", "must be between 1 and 1000000"},
		{"Agg Policy MIN_GROUP_SIZE too large", "CREATE AGGREGATION POLICY my_agg_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 1000001)", "must be between 1 and 1000000"},
		// ALTER / DROP Aggregation Policy — invalid
		{"Alter Agg Policy missing action", "ALTER AGGREGATION POLICY my_agg_policy", "requires SET BODY, SET COMMENT, UNSET COMMENT, or RENAME TO"},
		{"Drop Agg Policy missing name", "DROP AGGREGATION POLICY", "requires a policy name"},

		// Invalid Projection Policies
		{"Proj Policy OR REPLACE with IF NOT EXISTS", "CREATE OR REPLACE PROJECTION POLICY IF NOT EXISTS my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'none')", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Proj Policy missing AS", "CREATE PROJECTION POLICY my_proj_policy RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'none')", "Missing mandatory AS"},
		{"Proj Policy missing RETURNS", "CREATE PROJECTION POLICY my_proj_policy AS () -> PROJECTION_CONSTRAINT(ALLOW => 'none')", "Missing mandatory RETURNS PROJECTION_CONSTRAINT"},
		{"Proj Policy missing arrow", "CREATE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT PROJECTION_CONSTRAINT(ALLOW => 'none')", "Missing mandatory '->'"},
		{"Proj Policy invalid ALLOW value", "CREATE PROJECTION POLICY my_proj_policy AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => 'all')", "must be 'none' or 'transformation'"},
		// ALTER / DROP Projection Policy — invalid
		{"Alter Proj Policy missing action", "ALTER PROJECTION POLICY my_proj_policy", "requires SET BODY, SET COMMENT, UNSET COMMENT, or RENAME TO"},
		{"Drop Proj Policy missing name", "DROP PROJECTION POLICY", "requires a policy name"},

		// Invalid Packages Policies
		{"Pkg Policy OR REPLACE with IF NOT EXISTS", "CREATE OR REPLACE PACKAGES POLICY IF NOT EXISTS my_pkg_policy LANGUAGE PYTHON", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Pkg Policy missing LANGUAGE", "CREATE PACKAGES POLICY my_pkg_policy", "Missing mandatory LANGUAGE"},
		{"Pkg Policy unsupported language", "CREATE PACKAGES POLICY my_pkg_policy LANGUAGE JAVA", "only PYTHON is allowed"},
		{"Pkg Policy unsupported language Scala", "CREATE PACKAGES POLICY my_pkg_policy LANGUAGE SCALA", "only PYTHON is allowed"},
		// ALTER / DROP Packages Policy — invalid
		{"Alter Pkg Policy missing action", "ALTER PACKAGES POLICY my_pkg_policy", "requires SET ALLOWLIST"},
		{"Drop Pkg Policy missing name", "DROP PACKAGES POLICY", "requires a policy name"},

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

		// Invalid CREATE EVENT TABLE
		{"Create Event Table OR REPLACE and IF NOT EXISTS", "CREATE OR REPLACE EVENT TABLE IF NOT EXISTS my_events", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Create Event Table column definitions", "CREATE EVENT TABLE my_events (col1 VARCHAR)", "Event tables have a fixed schema"},
		{"Create Event Table CLUSTER BY", "CREATE EVENT TABLE my_events CLUSTER BY (ts)", "CLUSTER BY is not supported for EVENT TABLE"},
		{"Create Event Table invalid property", "CREATE EVENT TABLE my_events AUTO_REFRESH = TRUE", "Unexpected property 'AUTO_REFRESH'"},
		{"Create Event Table missing name", "CREATE EVENT TABLE", "Unexpected syntax"},

		// Invalid CREATE SHARE
		{"Create Share with prefix", "CREATE SHARE db.schema.my_share", "account-level"},
		{"Create Share OR REPLACE with prefix", "CREATE OR REPLACE SHARE db.schema.my_share", "account-level"},
		{"Create Share invalid property", "CREATE SHARE my_share AUTO_REFRESH = TRUE", "Unexpected property 'AUTO_REFRESH'"},
		{"Create Share OR REPLACE and IF NOT EXISTS", "CREATE OR REPLACE SHARE IF NOT EXISTS my_share", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Create Share missing name", "CREATE SHARE", "Unexpected syntax"},

		// Invalid CREATE DATASHARE
		{"Create Datashare with prefix", "CREATE DATASHARE db.schema.my_ds", "account-level"},
		{"Create Datashare OR REPLACE with prefix", "CREATE OR REPLACE DATASHARE db.my_ds", "account-level"},
		{"Create Datashare invalid property", "CREATE DATASHARE my_ds AUTO_REFRESH = TRUE", "Unexpected property 'AUTO_REFRESH'"},
		{"Create Datashare OR REPLACE and IF NOT EXISTS", "CREATE OR REPLACE DATASHARE IF NOT EXISTS my_ds", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Create Datashare missing name", "CREATE DATASHARE", "Unexpected syntax"},
		{"Create Datashare SHARE_RESTRICTIONS invalid value", "CREATE DATASHARE my_ds SHARE_RESTRICTIONS = MAYBE", "SHARE_RESTRICTIONS must be TRUE or FALSE"},

		// Invalid ALTER DATASHARE
		{"Alter Datashare missing name", "ALTER DATASHARE", "requires a datashare name"},
		{"Alter Datashare unknown sub-command", "ALTER DATASHARE my_ds ENABLE CHANGE_TRACKING", "Unknown ALTER DATASHARE sub-command"},
		{"Alter Datashare ADD ACCOUNTS missing list", "ALTER DATASHARE my_ds ADD ACCOUNTS =", "ADD ACCOUNTS requires at least one"},
		{"Alter Datashare REMOVE ACCOUNTS missing list", "ALTER DATASHARE my_ds REMOVE ACCOUNTS =", "REMOVE ACCOUNTS requires at least one"},
		{"Alter Datashare ADD DATABASES missing list", "ALTER DATASHARE my_ds ADD DATABASES", "ADD DATABASES requires at least one"},
		{"Alter Datashare REMOVE DATABASES missing list", "ALTER DATASHARE my_ds REMOVE DATABASES", "REMOVE DATABASES requires at least one"},
		{"Alter Datashare SHARE_RESTRICTIONS invalid", "ALTER DATASHARE my_ds ADD ACCOUNTS = org1.acct1 SHARE_RESTRICTIONS = MAYBE", "SHARE_RESTRICTIONS must be TRUE or FALSE"},
		{"Alter Datashare with prefix", "ALTER DATASHARE db.my_ds ADD ACCOUNTS = org1.acct1", "account-level"},
		{"Alter Datashare SHARE_RESTRICTIONS without ADD ACCOUNTS", "ALTER DATASHARE my_ds REMOVE ACCOUNTS = acct1 SHARE_RESTRICTIONS = TRUE", "SHARE_RESTRICTIONS is only valid with ADD ACCOUNTS"},

		// Invalid DROP DATASHARE
		{"Drop Datashare missing name", "DROP DATASHARE", "requires a datashare name"},
		{"Drop Datashare with prefix", "DROP DATASHARE db.my_ds", "account-level"},

		// Invalid CREATE COMPUTE POOL
		{"Compute Pool missing name", "CREATE COMPUTE POOL", "Unexpected syntax"},
		{"Compute Pool OR REPLACE and IF NOT EXISTS", "CREATE OR REPLACE COMPUTE POOL IF NOT EXISTS my_pool MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = CPU_X64_S", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Compute Pool with db prefix", "CREATE COMPUTE POOL db.my_pool MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = CPU_X64_S", "account-level"},
		{"Compute Pool with db.schema prefix", "CREATE COMPUTE POOL db.schema.my_pool MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = CPU_X64_S", "account-level"},
		{"Compute Pool missing MIN_NODES", "CREATE COMPUTE POOL my_pool MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S", "Missing mandatory property MIN_NODES"},
		{"Compute Pool missing MAX_NODES", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 INSTANCE_FAMILY = CPU_X64_S", "Missing mandatory property MAX_NODES"},
		{"Compute Pool missing INSTANCE_FAMILY", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3", "Missing mandatory property INSTANCE_FAMILY"},
		{"Compute Pool missing all mandatory", "CREATE COMPUTE POOL my_pool", "Missing mandatory property MIN_NODES"},
		{"Compute Pool MIN_NODES zero", "CREATE COMPUTE POOL my_pool MIN_NODES = 0 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S", "MIN_NODES value 0 is below the minimum"},
		{"Compute Pool MIN_NODES negative", "CREATE COMPUTE POOL my_pool MIN_NODES = -1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S", "MIN_NODES value -1 is below the minimum"},
		{"Compute Pool MAX_NODES zero", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 0 INSTANCE_FAMILY = CPU_X64_S", "MAX_NODES value 0 is below the minimum"},
		{"Compute Pool MAX_NODES less than MIN_NODES", "CREATE COMPUTE POOL my_pool MIN_NODES = 5 MAX_NODES = 2 INSTANCE_FAMILY = CPU_X64_S", "MAX_NODES (2) must be >= MIN_NODES (5)"},
		{"Compute Pool invalid INSTANCE_FAMILY", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = INVALID_SKU", "Invalid INSTANCE_FAMILY 'INVALID_SKU'"},
		{"Compute Pool AUTO_RESUME invalid value", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_RESUME = MAYBE", "AUTO_RESUME must be TRUE or FALSE"},
		{"Compute Pool INITIALLY_SUSPENDED invalid value", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S INITIALLY_SUSPENDED = YES", "INITIALLY_SUSPENDED must be TRUE or FALSE"},
		{"Compute Pool AUTO_SUSPEND_SECS negative", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S AUTO_SUSPEND_SECS = -10", "AUTO_SUSPEND_SECS value -10 must be a non-negative integer"},
		{"Compute Pool unexpected property", "CREATE COMPUTE POOL my_pool MIN_NODES = 1 MAX_NODES = 3 INSTANCE_FAMILY = CPU_X64_S WAREHOUSE = wh1", "Unexpected property 'WAREHOUSE'"},

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
		// GCS with ENCRYPTION TYPE = NONE (provider-agnostic, valid for GCS)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'NONE') ))",
		// S3GOV with STORAGE_AWS_EXTERNAL_ID (S3-family, valid)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3CHINA with STORAGE_AWS_EXTERNAL_ID (S3-family, valid)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
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
		// ALLOW_WRITES inside a COMMENT string value must not trigger a false positive
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) COMMENT = 'do not set ALLOW_WRITES = MAYBE here'",
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
			"Missing both STORAGE_PROVIDER and STORAGE_BASE_URL reports both errors",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' ))",
			[]string{"Each storage location requires STORAGE_BASE_URL", "Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"AZURE with ENCRYPTION block but no TYPE key",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (KMS_KEY_ID = 'k') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"S3 with ENCRYPTION block but no TYPE key",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (KMS_KEY_ID = 'k') ))",
			[]string{"ENCRYPTION block must specify a TYPE key"},
		},
		{
			"Empty STORAGE_LOCATIONS block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ()",
			[]string{"STORAGE_LOCATIONS must contain at least one storage location block"},
		},
		{
			"Unmatched paren in STORAGE_LOCATIONS (missing closing paren)",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ((NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/'",
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

func TestValidateSnowflakePatterns_CreateEventTable(t *testing.T) {
	validCases := []string{
		// Minimal valid event table
		"CREATE EVENT TABLE my_events",
		// With OR REPLACE
		"CREATE OR REPLACE EVENT TABLE my_events",
		// With IF NOT EXISTS
		"CREATE EVENT TABLE IF NOT EXISTS my_events",
		// With COMMENT
		"CREATE EVENT TABLE my_events COMMENT = 'telemetry data'",
		// With DATA_RETENTION_TIME_IN_DAYS
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 30",
		// With MAX_DATA_EXTENSION_TIME_IN_DAYS
		"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = 14",
		// With CHANGE_TRACKING = TRUE
		"CREATE EVENT TABLE my_events CHANGE_TRACKING = TRUE",
		// With CHANGE_TRACKING = FALSE
		"CREATE EVENT TABLE my_events CHANGE_TRACKING = FALSE",
		// With DEFAULT_DDL_COLLATION
		"CREATE EVENT TABLE my_events DEFAULT_DDL_COLLATION = 'en-ci'",
		// With COPY GRANTS
		"CREATE OR REPLACE EVENT TABLE my_events COPY GRANTS",
		// Multiple properties
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 7 MAX_DATA_EXTENSION_TIME_IN_DAYS = 14 CHANGE_TRACKING = TRUE COMMENT = 'logs'",
		// Schema-qualified name
		"CREATE EVENT TABLE my_db.my_schema.my_events",
		// Zero retention
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 0",
		// Zero extension
		"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = 0",
		// CLUSTER BY inside COMMENT string must not trigger false positive
		"CREATE EVENT TABLE my_events COMMENT = 'has CLUSTER BY inside'",
		// CLUSTER BY inside a line comment must not trigger false positive
		"CREATE EVENT TABLE my_events\n-- CLUSTER BY (ts)\nCOMMENT = 'test'",
		// Keywords inside a block comment must not trigger false positive
		"CREATE EVENT TABLE my_events /* AUTO_REFRESH = TRUE */ COMMENT = 'test'",
		// TAG property
		"CREATE EVENT TABLE my_events TAG (cost_center = 'finance')",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
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
			"OR REPLACE and IF NOT EXISTS conflict",
			"CREATE OR REPLACE EVENT TABLE IF NOT EXISTS my_events",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"Column definitions not allowed",
			"CREATE EVENT TABLE my_events (col1 VARCHAR, col2 INT)",
			[]string{"Event tables have a fixed schema and do not support column definitions"},
		},
		{
			"CLUSTER BY not supported",
			"CREATE EVENT TABLE my_events CLUSTER BY (timestamp)",
			[]string{"CLUSTER BY is not supported for EVENT TABLE"},
		},
		{
			"Invalid DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = abc",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Negative DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = -1",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Invalid MAX_DATA_EXTENSION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = xyz",
			[]string{"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Negative MAX_DATA_EXTENSION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = -1",
			[]string{"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Invalid CHANGE_TRACKING value",
			"CREATE EVENT TABLE my_events CHANGE_TRACKING = MAYBE",
			[]string{"CHANGE_TRACKING must be TRUE or FALSE"},
		},
		{
			"Unexpected property AUTO_REFRESH",
			"CREATE EVENT TABLE my_events AUTO_REFRESH = TRUE",
			[]string{"Unexpected property 'AUTO_REFRESH'"},
		},
		{
			"Missing name",
			"CREATE EVENT TABLE",
			[]string{"Unexpected syntax in CREATE EVENT TABLE"},
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
		sql := "CREATE OR REPLACE EVENT TABLE IF NOT EXISTS my_events AUTO_REFRESH = TRUE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (early return), got %d: %v", len(warns), warns)
		}
	})
}

// ── ALTER SESSION SET / UNSET ─────────────────────────────────────────────────

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

// TestMatchStringLiteral tests edge cases of the matchStringLiteral helper.
func TestMatchStringLiteral(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"'hello'", 7},
		{"'it''s'", 7},
		{"''", 2},
		{"'abc' rest", 5},
		{"'unclosed", -1},
		{"not a string", -1},
		{"", -1},
		{"'single''escape''pair'", 22},
	}
	for _, tt := range tests {
		got := matchStringLiteral(tt.input)
		if got != tt.want {
			t.Errorf("matchStringLiteral(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ── DESCRIBE / DESC Tests ───────────────────────────────────────────────────

// TestDescribeObjectTypes_OrderingInvariant verifies that describeObjectTypes is sorted
// by word count descending so the longest match is always attempted first.
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
}

// ── Tag Tests ────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Tag(t *testing.T) {
	validCases := []string{
		// ── CREATE TAG ───────────────────────────────────────────────────
		"CREATE TAG my_tag",
		"CREATE OR REPLACE TAG my_tag",
		"CREATE TAG IF NOT EXISTS my_tag",
		"CREATE TAG db.schema.my_tag",
		`CREATE TAG "My Tag"`,
		"CREATE TAG my_tag COMMENT = 'cost center tag'",
		"CREATE TAG my_tag ALLOWED_VALUES 'finance', 'engineering', 'marketing'",
		"CREATE TAG my_tag ALLOWED_VALUES 'a'",
		"CREATE OR REPLACE TAG cost_center ALLOWED_VALUES 'finance', 'hr' COMMENT = 'dept tag'",
		"CREATE TAG my_tag ALLOWED_VALUES 'it''s ok'",
		// ── ALTER TAG ────────────────────────────────────────────────────
		"ALTER TAG my_tag RENAME TO new_tag",
		"ALTER TAG db.schema.my_tag RENAME TO db.schema.new_tag",
		"ALTER TAG my_tag ADD ALLOWED_VALUES 'new_val'",
		"ALTER TAG my_tag ADD ALLOWED_VALUES 'v1', 'v2', 'v3'",
		"ALTER TAG my_tag DROP ALLOWED_VALUES 'old_val'",
		"ALTER TAG my_tag DROP ALLOWED_VALUES 'v1', 'v2'",
		"ALTER TAG my_tag UNSET ALLOWED_VALUES",
		"ALTER TAG my_tag SET COMMENT = 'updated tag'",
		"ALTER TAG my_tag UNSET COMMENT",
		"ALTER TAG IF EXISTS my_tag RENAME TO new_tag",
		"ALTER TAG IF EXISTS my_tag ADD ALLOWED_VALUES 'x'",
		"ALTER TAG IF EXISTS my_tag UNSET ALLOWED_VALUES",
		"ALTER TAG IF EXISTS my_tag SET COMMENT = 'c'",
		"ALTER TAG IF EXISTS my_tag UNSET COMMENT",
		// ── DROP TAG ─────────────────────────────────────────────────────
		"DROP TAG my_tag",
		"DROP TAG IF EXISTS my_tag",
		"DROP TAG db.schema.my_tag",
		`DROP TAG "My Tag"`,
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
		// ── CREATE TAG ───────────────────────────────────────────────────
		{
			"bare CREATE TAG without name",
			"CREATE TAG",
			[]string{"CREATE TAG requires a tag name"},
		},
		{
			"CREATE OR REPLACE TAG without name",
			"CREATE OR REPLACE TAG",
			[]string{"CREATE TAG requires a tag name"},
		},
		{
			"OR REPLACE + IF NOT EXISTS conflict",
			"CREATE OR REPLACE TAG IF NOT EXISTS my_tag",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"ALLOWED_VALUES with non-string value",
			"CREATE TAG my_tag ALLOWED_VALUES finance",
			[]string{"ALLOWED_VALUES requires a list of string literals"},
		},
		{
			"ALLOWED_VALUES with duplicate values",
			"CREATE TAG my_tag ALLOWED_VALUES 'finance', 'hr', 'finance'",
			[]string{"Duplicate value 'finance'"},
		},
		{
			"ALLOWED_VALUES with duplicate values case-insensitive",
			"CREATE TAG my_tag ALLOWED_VALUES 'Finance', 'FINANCE'",
			[]string{"Duplicate value"},
		},
		// ── ALTER TAG ────────────────────────────────────────────────────
		{
			"bare ALTER TAG without name",
			"ALTER TAG",
			[]string{"ALTER TAG requires a tag name"},
		},
		{
			"ALTER TAG with unknown sub-command",
			"ALTER TAG my_tag RESET",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG RENAME TO without new name",
			"ALTER TAG my_tag RENAME TO",
			[]string{"ALTER TAG RENAME TO requires a new tag name"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES without values",
			"ALTER TAG my_tag ADD ALLOWED_VALUES",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES with non-string",
			"ALTER TAG my_tag ADD ALLOWED_VALUES finance",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES without values",
			"ALTER TAG my_tag DROP ALLOWED_VALUES",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with non-string",
			"ALTER TAG my_tag DROP ALLOWED_VALUES finance",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with duplicate values",
			"ALTER TAG my_tag DROP ALLOWED_VALUES 'v1', 'v2', 'v1'",
			[]string{"Duplicate value 'v1'"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES with case-insensitive duplicate",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'Finance', 'FINANCE'",
			[]string{"case-insensitive match with 'Finance'"},
		},
		{
			"ALTER TAG compound sub-commands",
			"ALTER TAG my_tag RENAME TO new_tag ADD ALLOWED_VALUES 'x'",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		{
			"ALTER TAG compound SET COMMENT and UNSET ALLOWED_VALUES",
			"ALTER TAG my_tag SET COMMENT = 'c' UNSET ALLOWED_VALUES",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		// ── DROP TAG ─────────────────────────────────────────────────────
		{
			"bare DROP TAG without name",
			"DROP TAG",
			[]string{"DROP TAG requires a tag name"},
		},
		{
			"DROP TAG with CASCADE",
			"DROP TAG my_tag CASCADE",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		{
			"DROP TAG with RESTRICT",
			"DROP TAG my_tag RESTRICT",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		{
			"DROP TAG IF EXISTS with CASCADE",
			"DROP TAG IF EXISTS my_tag CASCADE",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
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
}

func TestValidateSnowflakePatterns_Task(t *testing.T) {
	validCases := []string{
		// ── CREATE TASK — root tasks (must have SCHEDULE) ────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
		"CREATE OR REPLACE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO t SELECT 1",
		"CREATE TASK IF NOT EXISTS db.schema.my_task WAREHOUSE = wh SCHEDULE = '5 MINUTE' AS CALL my_proc()",
		"CREATE TASK my_task USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = 'XSMALL' SCHEDULE = '1 MINUTE' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'root task' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '60 MINUTE' ALLOW_OVERLAPPING_EXECUTION = TRUE AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' USER_TASK_TIMEOUT_MS = 60000 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' SUSPEND_TASK_AFTER_NUM_FAILURES = 3 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' ERROR_INTEGRATION = my_int AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' CONFIG = $${\"key\": \"val\"}$$ AS SELECT 1",
		// ── CREATE TASK — child tasks (AFTER, no SCHEDULE) ──────────────
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent_task AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER db.schema.parent_task AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER task1, task2, task3 AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent_task WHEN SYSTEM$STREAM_HAS_DATA('my_stream') AS INSERT INTO t SELECT * FROM s",
		// ── CREATE TASK — child task with WHEN condition ─────────────────
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent WHEN SYSTEM$GET_PREDECESSOR_RETURN_VALUE('parent') = 'done' AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent WHEN cond1 AND cond2 AS SELECT 1",
		// ── CREATE TASK — root task with WHEN (valid per Snowflake docs) ─
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' WHEN SYSTEM$STREAM_HAS_DATA('my_stream') AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '5 MINUTE' WHEN SYSTEM$STREAM_HAS_DATA('s1') AND SYSTEM$STREAM_HAS_DATA('s2') AS SELECT 1",
		// ── CREATE TASK — finalizer tasks ────────────────────────────────
		"CREATE TASK finalizer_task FINALIZE = root_task AS SELECT 1",
		"CREATE TASK finalizer_task WAREHOUSE = wh FINALIZE = root_task AS SELECT 1",
		// ── ALTER TASK ──────────────────────────────────────────────────
		"ALTER TASK my_task RESUME",
		"ALTER TASK my_task SUSPEND",
		"ALTER TASK IF EXISTS my_task RESUME",
		"ALTER TASK IF EXISTS my_task SUSPEND",
		"ALTER TASK my_task SET SCHEDULE = '10 MINUTE'",
		"ALTER TASK my_task SET WAREHOUSE = new_wh",
		"ALTER TASK my_task SET USER_TASK_TIMEOUT_MS = 60000",
		"ALTER TASK my_task SET COMMENT = 'updated'",
		"ALTER TASK my_task SET SUSPEND_TASK_AFTER_NUM_FAILURES = 5",
		"ALTER TASK my_task SET ERROR_INTEGRATION = my_int",
		"ALTER TASK my_task UNSET WAREHOUSE",
		"ALTER TASK my_task UNSET COMMENT",
		"ALTER TASK my_task REMOVE AFTER task1",
		"ALTER TASK my_task REMOVE AFTER task1, task2",
		"ALTER TASK my_task ADD AFTER task1",
		"ALTER TASK my_task ADD AFTER task1, task2",
		"ALTER TASK my_task MODIFY AS SELECT 1 FROM t",
		"ALTER TASK my_task MODIFY WHEN SYSTEM$STREAM_HAS_DATA('my_stream')",
		"ALTER TASK my_task SET FINALIZE = root_task",
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
		// ── CREATE TASK — missing name ──────────────────────────────────
		{
			"bare CREATE TASK without name",
			"CREATE TASK",
			[]string{"CREATE TASK requires a task name"},
		},
		{
			"CREATE OR REPLACE TASK without name",
			"CREATE OR REPLACE TASK",
			[]string{"CREATE TASK requires a task name"},
		},
		// ── CREATE TASK — OR REPLACE + IF NOT EXISTS conflict ───────────
		{
			"OR REPLACE + IF NOT EXISTS conflict",
			"CREATE OR REPLACE TASK IF NOT EXISTS my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// ── CREATE TASK — missing AS body ───────────────────────────────
		{
			"missing AS keyword",
			"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE'",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── CREATE TASK — AFTER + SCHEDULE mutual exclusivity ───────────
		{
			"AFTER and SCHEDULE are mutually exclusive",
			"CREATE TASK child WAREHOUSE = wh AFTER parent SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"AFTER and SCHEDULE are mutually exclusive"},
		},
		// ── CREATE TASK — root task without SCHEDULE ────────────────────
		{
			"root task missing SCHEDULE",
			"CREATE TASK my_task WAREHOUSE = wh AS SELECT 1",
			[]string{"Root task (no AFTER or FINALIZE clause) requires a SCHEDULE"},
		},
		// ── CREATE TASK — bare AFTER without names ──────────────────────
		{
			"bare AFTER without predecessor names",
			"CREATE TASK child WAREHOUSE = wh AFTER AS SELECT 1",
			[]string{"AFTER requires at least one predecessor task name"},
		},
		// ── CREATE TASK — FINALIZE + AFTER conflict ─────────────────────
		{
			"FINALIZE with AFTER is invalid",
			"CREATE TASK finalizer WAREHOUSE = wh FINALIZE = root_task AFTER parent AS SELECT 1",
			[]string{"FINALIZE must not be combined with AFTER"},
		},
		// ── CREATE TASK — FINALIZE + SCHEDULE conflict ──────────────────
		{
			"FINALIZE with SCHEDULE is invalid",
			"CREATE TASK finalizer WAREHOUSE = wh FINALIZE = root_task SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"FINALIZE must not be combined with SCHEDULE"},
		},
		// ── CREATE TASK — bare WHEN without expression ──────────────────
		{
			"bare WHEN without expression",
			"CREATE TASK child WAREHOUSE = wh AFTER parent WHEN AS SELECT 1",
			[]string{"WHEN requires a boolean expression"},
		},
		// ── CREATE TASK — FINALIZE without root task name ───────────────
		{
			"bare FINALIZE without root task name",
			"CREATE TASK finalizer FINALIZE AS SELECT 1",
			[]string{"FINALIZE requires a root task name"},
		},
		// ── ALTER TASK — missing name ───────────────────────────────────
		{
			"bare ALTER TASK without name",
			"ALTER TASK",
			[]string{"ALTER TASK requires a task name"},
		},
		// ── ALTER TASK — unknown sub-command ────────────────────────────
		{
			"ALTER TASK unknown sub-command",
			"ALTER TASK my_task RESET",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── ALTER TASK — ADD AFTER without names ────────────────────────
		{
			"ALTER TASK ADD AFTER without names",
			"ALTER TASK my_task ADD AFTER",
			[]string{"ADD AFTER requires at least one predecessor task name"},
		},
		// ── ALTER TASK — MODIFY AS without body ─────────────────────────
		{
			"ALTER TASK MODIFY AS without body",
			"ALTER TASK my_task MODIFY AS",
			[]string{"MODIFY AS requires a SQL statement"},
		},
		// ── ALTER TASK — MODIFY WHEN without expression ─────────────────
		{
			"ALTER TASK MODIFY WHEN without expression",
			"ALTER TASK my_task MODIFY WHEN",
			[]string{"MODIFY WHEN requires a boolean expression"},
		},
		// ── ALTER TASK — SET FINALIZE without root task name ─────────────
		{
			"ALTER TASK SET FINALIZE without name",
			"ALTER TASK my_task SET FINALIZE =",
			[]string{"SET FINALIZE requires a root task name"},
		},
		// ── ALTER TASK — REMOVE AFTER without name ──────────────────────
		{
			"ALTER TASK REMOVE AFTER without task name",
			"ALTER TASK my_task REMOVE AFTER",
			[]string{"REMOVE AFTER requires at least one predecessor task name"},
		},
		// ── ALTER TASK — SET with unknown property ───────────────────────
		{
			"ALTER TASK SET with unknown property",
			"ALTER TASK my_task SET RETRY_LIMIT = 5",
			[]string{"Unexpected property 'RETRY_LIMIT'"},
		},
		// ── ALTER TASK — UNSET with unknown property ─────────────────────
		{
			"ALTER TASK UNSET with unknown property",
			"ALTER TASK my_task UNSET FOOBAR",
			[]string{"Unexpected property 'FOOBAR'"},
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
}

// ── Time Travel AT / BEFORE Tests ──────────────────────────────────────────

func TestValidateSnowflakePatterns_TimeTravel(t *testing.T) {
	// ── Valid Time Travel queries ─────────────────────────────────────────
	validQueries := []string{
		"SELECT * FROM orders AT (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders AT (OFFSET => -3600)",
		"SELECT * FROM orders AT (OFFSET => -60*5)",
		"SELECT * FROM orders AT (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders BEFORE (OFFSET => -3600)",
		"SELECT * FROM orders AT (STREAM => my_stream)",
		// Fully qualified table with Time Travel
		"SELECT * FROM db.schema.orders AT (TIMESTAMP => '2024-01-01')",
		// Time Travel in CLONE context (already supported)
		"CREATE TABLE t CLONE s AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		"CREATE STREAM my_stream ON TABLE my_table AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		// Multiple Time Travel clauses in one query (JOIN)
		"SELECT a.id FROM t1 AT (OFFSET => -60) JOIN t2 BEFORE (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726') ON a.id = b.id",
		// DML with Time Travel
		"INSERT INTO t SELECT * FROM s AT (OFFSET => -3600)",
		// Case variation — lowercase keywords
		"SELECT * FROM orders at (timestamp => '2024-01-01')",
		"SELECT * FROM orders before (statement => 'abc-123')",
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}

	// ── Invalid Time Travel queries ───────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"Missing => operator",
			"SELECT * FROM orders AT (TIMESTAMP '2024-01-01')",
			[]string{"Missing '=>' operator"},
		},
		{
			"Multiple arguments",
			"SELECT * FROM orders AT (TIMESTAMP => '2024-01-01', OFFSET => -60)",
			[]string{"Multiple keyword arguments"},
		},
		{
			"STREAM in BEFORE clause",
			"SELECT * FROM orders BEFORE (STREAM => my_stream)",
			[]string{"STREAM => is not valid in a BEFORE clause"},
		},
		{
			"Missing parentheses",
			"SELECT * FROM orders AT TIMESTAMP '2024-01-01'",
			[]string{"requires parentheses"},
		},
		{
			"Unknown content in AT clause",
			"SELECT * FROM orders AT (123)",
			[]string{"Invalid AT clause. Expected one of"},
		},
		{
			"Unknown content in BEFORE clause",
			"SELECT * FROM orders BEFORE (123)",
			[]string{"Invalid BEFORE clause. Expected one of"},
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
}

// ── Replication Group / Failover Group Tests ────────────────────────────────

func TestValidateSnowflakePatterns_ReplicationFailoverGroup(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE REPLICATION GROUP — minimal
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — DATABASES with ALLOWED_DATABASES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES ALLOWED_DATABASES = db1, db2 ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — INTEGRATIONS with ALLOWED_INTEGRATION_TYPES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = INTEGRATIONS ALLOWED_INTEGRATION_TYPES = SECURITY INTEGRATIONS ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — multiple OBJECT_TYPES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES, ROLES, WAREHOUSES ALLOWED_DATABASES = db1 ALLOWED_ACCOUNTS = org1.acct1, org2.acct2",
		// CREATE REPLICATION GROUP — with IGNORE EDITION CHECK
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 IGNORE EDITION CHECK",
		// CREATE REPLICATION GROUP — with REPLICATION_SCHEDULE
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 REPLICATION_SCHEDULE = '10 MINUTE'",
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 REPLICATION_SCHEDULE = 'USING CRON 0 0 * * * UTC'",
		// CREATE FAILOVER GROUP — minimal
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE FAILOVER GROUP — with ALLOWED_FAILOVER_ACCOUNTS
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_FAILOVER_ACCOUNTS = org1.acct1",
		// CREATE FAILOVER GROUP — DATABASES
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = DATABASES ALLOWED_DATABASES = db1 ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE OR REPLACE variants
		"CREATE OR REPLACE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		"CREATE OR REPLACE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// ALTER REPLICATION GROUP
		"ALTER REPLICATION GROUP my_rg ADD org1.acct2, org2.acct3",
		"ALTER REPLICATION GROUP my_rg REMOVE org1.acct2",
		"ALTER REPLICATION GROUP my_rg MOVE DATABASES db1, db2 TO REPLICATION GROUP other_rg",
		"ALTER REPLICATION GROUP my_rg SET REPLICATION_SCHEDULE = '30 MINUTE'",
		"ALTER REPLICATION GROUP my_rg SET OBJECT_TYPES = ROLES, WAREHOUSES",
		"ALTER REPLICATION GROUP my_rg RENAME TO new_rg_name",
		// ALTER FAILOVER GROUP
		"ALTER FAILOVER GROUP my_fg ADD org1.acct2",
		"ALTER FAILOVER GROUP my_fg REMOVE org1.acct2",
		"ALTER FAILOVER GROUP my_fg PRIMARY",
		"ALTER FAILOVER GROUP my_fg REFRESH",
		"ALTER FAILOVER GROUP my_fg SUSPEND",
		"ALTER FAILOVER GROUP my_fg RESUME",
		"ALTER FAILOVER GROUP my_fg MOVE DATABASES db1 TO REPLICATION GROUP other_rg",
		"ALTER FAILOVER GROUP my_fg SET REPLICATION_SCHEDULE = '10 MINUTE'",
		"ALTER FAILOVER GROUP my_fg RENAME TO new_fg",
		// Group name containing "databases" must not trigger false ALLOWED_DATABASES warning
		"CREATE REPLICATION GROUP databases_backup OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// Group name containing "integrations" must not trigger false ALLOWED_INTEGRATION_TYPES warning
		"CREATE REPLICATION GROUP integrations_sync OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// ALTER FAILOVER GROUP with inline comment after PRIMARY/REFRESH
		"ALTER FAILOVER GROUP my_fg PRIMARY -- promote to primary",
		"ALTER FAILOVER GROUP my_fg REFRESH -- manual refresh",
		// DROP REPLICATION GROUP
		"DROP REPLICATION GROUP my_rg",
		"DROP REPLICATION GROUP IF EXISTS my_rg",
		// DROP FAILOVER GROUP
		"DROP FAILOVER GROUP my_fg",
		"DROP FAILOVER GROUP IF EXISTS my_fg",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// CREATE REPLICATION GROUP — missing mandatory clauses
		{
			"CREATE REPL GROUP missing name",
			"CREATE REPLICATION GROUP",
			[]string{"CREATE REPLICATION GROUP requires a group name"},
		},
		{
			"CREATE REPL GROUP missing OBJECT_TYPES",
			"CREATE REPLICATION GROUP my_rg ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"Missing mandatory OBJECT_TYPES"},
		},
		{
			"CREATE REPL GROUP missing ALLOWED_ACCOUNTS",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES",
			[]string{"Missing mandatory ALLOWED_ACCOUNTS"},
		},
		{
			"CREATE REPL GROUP DATABASES without ALLOWED_DATABASES",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		{
			"CREATE REPL GROUP INTEGRATIONS without ALLOWED_INTEGRATION_TYPES",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = INTEGRATIONS ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes INTEGRATIONS but ALLOWED_INTEGRATION_TYPES is missing"},
		},
		{
			"CREATE REPL GROUP db.schema prefix",
			"CREATE REPLICATION GROUP mydb.my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE FAILOVER GROUP — missing mandatory clauses
		{
			"CREATE FAILOVER GROUP missing name",
			"CREATE FAILOVER GROUP",
			[]string{"CREATE FAILOVER GROUP requires a group name"},
		},
		{
			"CREATE FAILOVER GROUP missing OBJECT_TYPES",
			"CREATE FAILOVER GROUP my_fg ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"Missing mandatory OBJECT_TYPES"},
		},
		{
			"CREATE FAILOVER GROUP missing ALLOWED_ACCOUNTS",
			"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES",
			[]string{"Missing mandatory ALLOWED_ACCOUNTS or ALLOWED_FAILOVER_ACCOUNTS"},
		},
		{
			"CREATE FAILOVER GROUP DATABASES without ALLOWED_DATABASES",
			"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		// ALTER REPLICATION GROUP — missing action
		{
			"ALTER REPL GROUP missing name",
			"ALTER REPLICATION GROUP",
			[]string{"ALTER REPLICATION GROUP requires a group name"},
		},
		{
			"ALTER REPL GROUP missing action",
			"ALTER REPLICATION GROUP my_rg",
			[]string{"ALTER REPLICATION GROUP requires an action"},
		},
		{
			"ALTER REPL GROUP MOVE DATABASES without TO",
			"ALTER REPLICATION GROUP my_rg MOVE DATABASES db1",
			[]string{"MOVE DATABASES in ALTER REPLICATION GROUP requires TO REPLICATION GROUP"},
		},
		// ALTER FAILOVER GROUP — missing action
		{
			"ALTER FAILOVER GROUP missing name",
			"ALTER FAILOVER GROUP",
			[]string{"ALTER FAILOVER GROUP requires a group name"},
		},
		{
			"ALTER FAILOVER GROUP missing action",
			"ALTER FAILOVER GROUP my_fg",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP MOVE DATABASES without TO",
			"ALTER FAILOVER GROUP my_fg MOVE DATABASES db1",
			[]string{"MOVE DATABASES in ALTER FAILOVER GROUP requires TO REPLICATION GROUP"},
		},
		// OBJECT_TYPES at end of statement (no trailing keyword)
		{
			"OBJECT_TYPES = DATABASES at end, missing ALLOWED_DATABASES",
			"CREATE REPLICATION GROUP my_rg ALLOWED_ACCOUNTS = org1.acct1 OBJECT_TYPES = DATABASES",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		// Group named after an action keyword — must still detect missing action
		{
			"ALTER FAILOVER GROUP named primary with no action",
			"ALTER FAILOVER GROUP primary",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named refresh with no action",
			"ALTER FAILOVER GROUP refresh",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named suspend with no action",
			"ALTER FAILOVER GROUP suspend",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named resume with no action",
			"ALTER FAILOVER GROUP resume",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		// ALTER — account-level prefix check
		{
			"ALTER REPL GROUP db.schema prefix",
			"ALTER REPLICATION GROUP mydb.my_rg ADD org1.acct2",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"ALTER FAILOVER GROUP db.schema prefix",
			"ALTER FAILOVER GROUP mydb.my_fg PRIMARY",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// DROP — account-level prefix check
		{
			"DROP REPL GROUP db.schema prefix",
			"DROP REPLICATION GROUP mydb.my_rg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"DROP FAILOVER GROUP db.schema prefix",
			"DROP FAILOVER GROUP mydb.my_fg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// DROP — missing name
		{
			"DROP REPL GROUP missing name",
			"DROP REPLICATION GROUP",
			[]string{"DROP REPLICATION GROUP requires a group name"},
		},
		{
			"DROP FAILOVER GROUP missing name",
			"DROP FAILOVER GROUP",
			[]string{"DROP FAILOVER GROUP requires a group name"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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
}

func TestValidateSnowflakePatterns_Service(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE SERVICE — inline YAML specification
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE — stage-referenced specification file
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// CREATE OR REPLACE SERVICE
		"CREATE OR REPLACE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE IF NOT EXISTS
		"CREATE SERVICE IF NOT EXISTS my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE — with optional properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1 MAX_INSTANCES = 3",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = TRUE",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = FALSE",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ QUERY_WAREHOUSE = wh1",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'my service'",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ EXTERNAL_ACCESS_INTEGRATIONS = (my_eai)",
		// CREATE SERVICE — all optional properties together
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1 MAX_INSTANCES = 5 AUTO_RESUME = TRUE QUERY_WAREHOUSE = wh1 COMMENT = 'full opts'",
		// CREATE SERVICE — MIN_INSTANCES = 0 is valid (non-negative)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 0 MAX_INSTANCES = 3",
		// CREATE SERVICE — with SPECIFICATION_FILE and properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml' MIN_INSTANCES = 1 MAX_INSTANCES = 2",
		// CREATE SERVICE — COMMENT with IF NOT EXISTS inside string should not trigger conflict
		"CREATE OR REPLACE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'IF NOT EXISTS hint'",
		// CREATE SERVICE — FROM @stage SPECIFICATION_FILE (stage-prefix form)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// CREATE SERVICE — FROM SPECIFICATION_TEMPLATE (parameterized spec)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE $$spec with {{ var }}$$",
		// CREATE SERVICE — FROM SPECIFICATION_TEMPLATE_FILE
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE_FILE = '@stage/spec.yaml'",
		// CREATE SERVICE — FROM @stage SPECIFICATION_TEMPLATE_FILE
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_TEMPLATE_FILE = 'spec.yaml'",
		// CREATE SERVICE — additional known properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_SUSPEND_SECS = 300",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_READY_INSTANCES = 1",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ LOG_LEVEL = 'INFO'",

		// EXECUTE SERVICE — inline YAML
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// EXECUTE JOB SERVICE — canonical form
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// EXECUTE SERVICE — stage-referenced specification file
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// EXECUTE JOB SERVICE — stage-prefix form
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// EXECUTE SERVICE — with optional properties
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ QUERY_WAREHOUSE = wh1",
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'batch job'",
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ EXTERNAL_ACCESS_INTEGRATIONS = (eai1)",
		// EXECUTE JOB SERVICE — additional properties
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ NAME = my_named_job",
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ ASYNC = TRUE",
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ REPLICAS = 3",
		// EXECUTE SERVICE — SPECIFICATION_TEMPLATE
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE $$spec$$",

		// ALTER SERVICE — SUSPEND / RESUME
		"ALTER SERVICE my_svc SUSPEND",
		"ALTER SERVICE my_svc RESUME",
		// ALTER SERVICE — SET properties
		"ALTER SERVICE my_svc SET MIN_INSTANCES = 2",
		"ALTER SERVICE my_svc SET MAX_INSTANCES = 5",
		"ALTER SERVICE my_svc SET COMMENT = 'updated'",
		"ALTER SERVICE my_svc SET QUERY_WAREHOUSE = wh2",
		// ALTER SERVICE — UNSET properties
		"ALTER SERVICE my_svc UNSET COMMENT",
		"ALTER SERVICE my_svc UNSET QUERY_WAREHOUSE",
		// ALTER SERVICE — rolling update with FROM SPECIFICATION
		"ALTER SERVICE my_svc FROM SPECIFICATION $$new_spec$$",
		// ALTER SERVICE — rolling update with FROM SPECIFICATION_FILE
		"ALTER SERVICE my_svc FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// ALTER SERVICE — FROM SPECIFICATION_TEMPLATE
		"ALTER SERVICE my_svc FROM SPECIFICATION_TEMPLATE $$new_spec$$",
		// ALTER SERVICE — FROM @stage SPECIFICATION_FILE
		"ALTER SERVICE my_svc FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// ALTER SERVICE — UNSET MIN_INSTANCES / MAX_INSTANCES
		"ALTER SERVICE my_svc UNSET MIN_INSTANCES",
		"ALTER SERVICE my_svc UNSET MAX_INSTANCES",
		// ALTER SERVICE — IF EXISTS
		"ALTER SERVICE IF EXISTS my_svc SUSPEND",

		// DROP SERVICE
		"DROP SERVICE my_svc",
		"DROP SERVICE IF EXISTS my_svc",
		// DROP SERVICE — schema-qualified name
		"DROP SERVICE db.schema.my_svc",

		// CREATE IMAGE REPOSITORY — valid
		"CREATE IMAGE REPOSITORY my_repo",
		"CREATE OR REPLACE IMAGE REPOSITORY my_repo",
		"CREATE IMAGE REPOSITORY IF NOT EXISTS my_repo",
		"CREATE IMAGE REPOSITORY my_repo COMMENT = 'my image repo'",
		"CREATE OR REPLACE IMAGE REPOSITORY my_repo COMMENT = 'replaced repo'",
		// CREATE IMAGE REPOSITORY — schema-qualified names
		"CREATE IMAGE REPOSITORY db.schema.my_repo",
		"CREATE IMAGE REPOSITORY schema.my_repo",
		// CREATE IMAGE REPOSITORY — case insensitive
		"create image repository my_repo",
		"Create Image Repository IF NOT EXISTS my_repo",

		// DROP IMAGE REPOSITORY — valid
		"DROP IMAGE REPOSITORY my_repo",
		"DROP IMAGE REPOSITORY IF EXISTS my_repo",
		"DROP IMAGE REPOSITORY db.schema.my_repo",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// CREATE SERVICE — missing name
		{
			"CREATE SERVICE missing name",
			"CREATE SERVICE",
			[]string{"Unexpected syntax in CREATE SERVICE"},
		},
		// CREATE SERVICE — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE SERVICE OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE SERVICE IF NOT EXISTS my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE SERVICE — missing IN COMPUTE POOL
		{
			"CREATE SERVICE missing COMPUTE POOL",
			"CREATE SERVICE my_svc FROM SPECIFICATION $$spec$$",
			[]string{"Missing mandatory IN COMPUTE POOL"},
		},
		// CREATE SERVICE — missing specification
		{
			"CREATE SERVICE missing spec",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool",
			[]string{"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// CREATE SERVICE — both SPECIFICATION and SPECIFICATION_FILE
		{
			"CREATE SERVICE both spec and spec file",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
			[]string{"requires exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// CREATE SERVICE — MIN_INSTANCES negative
		{
			"CREATE SERVICE MIN_INSTANCES negative",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = -1",
			[]string{"MIN_INSTANCES value -1 must be a non-negative integer"},
		},
		// CREATE SERVICE — MAX_INSTANCES negative
		{
			"CREATE SERVICE MAX_INSTANCES negative",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MAX_INSTANCES = -1",
			[]string{"MAX_INSTANCES value -1 must be a non-negative integer"},
		},
		// CREATE SERVICE — MAX_INSTANCES < MIN_INSTANCES
		{
			"CREATE SERVICE MAX < MIN",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 5 MAX_INSTANCES = 2",
			[]string{"MAX_INSTANCES (2) must be >= MIN_INSTANCES (5)"},
		},
		// CREATE SERVICE — AUTO_RESUME invalid value
		{
			"CREATE SERVICE AUTO_RESUME invalid",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = MAYBE",
			[]string{"AUTO_RESUME must be TRUE or FALSE"},
		},
		// CREATE SERVICE — unexpected property
		{
			"CREATE SERVICE unexpected property",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},

		// EXECUTE SERVICE — missing name
		{
			"EXECUTE SERVICE missing name",
			"EXECUTE SERVICE",
			[]string{"Unexpected syntax in EXECUTE SERVICE"},
		},
		// EXECUTE JOB SERVICE — missing name (canonical form)
		{
			"EXECUTE JOB SERVICE missing name",
			"EXECUTE JOB SERVICE",
			[]string{"Unexpected syntax in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — missing COMPUTE POOL
		{
			"EXECUTE SERVICE missing COMPUTE POOL",
			"EXECUTE SERVICE my_job FROM SPECIFICATION $$spec$$",
			[]string{"Missing mandatory IN COMPUTE POOL"},
		},
		// EXECUTE SERVICE — missing specification
		{
			"EXECUTE SERVICE missing spec",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool",
			[]string{"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// EXECUTE SERVICE — MIN_INSTANCES not supported
		{
			"EXECUTE SERVICE with MIN_INSTANCES",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1",
			[]string{"MIN_INSTANCES is not supported in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — MAX_INSTANCES not supported
		{
			"EXECUTE SERVICE with MAX_INSTANCES",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MAX_INSTANCES = 3",
			[]string{"MAX_INSTANCES is not supported in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — unexpected property
		{
			"EXECUTE SERVICE unexpected property",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = TRUE",
			[]string{"Unexpected property 'AUTO_RESUME'"},
		},

		// ALTER SERVICE — missing name
		{
			"ALTER SERVICE missing name",
			"ALTER SERVICE",
			[]string{"ALTER SERVICE requires a service name"},
		},
		// ALTER SERVICE — unknown sub-command
		{
			"ALTER SERVICE unknown action",
			"ALTER SERVICE my_svc ENABLE",
			[]string{"Unknown ALTER SERVICE sub-command"},
		},
		// ALTER SERVICE — SET with unknown property
		{
			"ALTER SERVICE SET unknown property",
			"ALTER SERVICE my_svc SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER SERVICE SET"},
		},
		// ALTER SERVICE — UNSET with unknown property
		{
			"ALTER SERVICE UNSET unknown property",
			"ALTER SERVICE my_svc UNSET UNKNOWN_PROP",
			[]string{"Unknown property in ALTER SERVICE UNSET"},
		},
		// ALTER SERVICE — MIN_INSTANCES negative
		{
			"ALTER SERVICE MIN_INSTANCES negative",
			"ALTER SERVICE my_svc SET MIN_INSTANCES = -1",
			[]string{"MIN_INSTANCES value -1 must be a non-negative integer"},
		},
		// ALTER SERVICE — MAX_INSTANCES negative
		{
			"ALTER SERVICE MAX_INSTANCES negative",
			"ALTER SERVICE my_svc SET MAX_INSTANCES = -5",
			[]string{"MAX_INSTANCES value -5 must be a non-negative integer"},
		},
		// ALTER SERVICE — MAX < MIN
		{
			"ALTER SERVICE MAX < MIN",
			"ALTER SERVICE my_svc SET MIN_INSTANCES = 10 MAX_INSTANCES = 2",
			[]string{"MAX_INSTANCES (2) must be >= MIN_INSTANCES (10)"},
		},

		// ALTER SERVICE — SET with unknown trailing property
		{
			"ALTER SERVICE SET with unknown trailing property",
			"ALTER SERVICE my_svc SET COMMENT = 'foo' SOME_NONSENSE = bar",
			[]string{"Unexpected property 'SOME_NONSENSE'"},
		},

		// DROP SERVICE — missing name
		{
			"DROP SERVICE missing name",
			"DROP SERVICE",
			[]string{"DROP SERVICE requires a service name"},
		},
		// DROP SERVICE IF EXISTS — missing name
		{
			"DROP SERVICE IF EXISTS missing name",
			"DROP SERVICE IF EXISTS",
			[]string{"DROP SERVICE requires a service name"},
		},

		// CREATE IMAGE REPOSITORY — missing name
		{
			"CREATE IMAGE REPOSITORY missing name",
			"CREATE IMAGE REPOSITORY",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — OR REPLACE without name
		{
			"CREATE OR REPLACE IMAGE REPOSITORY missing name",
			"CREATE OR REPLACE IMAGE REPOSITORY",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — IF NOT EXISTS without name
		{
			"CREATE IMAGE REPOSITORY IF NOT EXISTS missing name",
			"CREATE IMAGE REPOSITORY IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE IMAGE REPOSITORY OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE IMAGE REPOSITORY IF NOT EXISTS my_repo",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE IMAGE REPOSITORY — unexpected property
		{
			"CREATE IMAGE REPOSITORY unexpected property",
			"CREATE IMAGE REPOSITORY my_repo TAG_POLICY = my_policy",
			[]string{"Unexpected property 'TAG_POLICY'"},
		},

		// DROP IMAGE REPOSITORY — missing name
		{
			"DROP IMAGE REPOSITORY missing name",
			"DROP IMAGE REPOSITORY",
			[]string{"DROP IMAGE REPOSITORY requires a repository name"},
		},
		// DROP IMAGE REPOSITORY IF EXISTS — missing name
		{
			"DROP IMAGE REPOSITORY IF EXISTS missing name",
			"DROP IMAGE REPOSITORY IF EXISTS",
			[]string{"DROP IMAGE REPOSITORY requires a repository name"},
		},

		// ALTER IMAGE REPOSITORY — unsupported
		{
			"ALTER IMAGE REPOSITORY unsupported",
			"ALTER IMAGE REPOSITORY my_repo SET COMMENT = 'test'",
			[]string{"ALTER IMAGE REPOSITORY is not supported"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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
}

func TestValidateSnowflakePatterns_ApplicationPackageAndApplication(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE APPLICATION PACKAGE — basic
		"CREATE APPLICATION PACKAGE my_pkg",
		"CREATE OR REPLACE APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION PACKAGE IF NOT EXISTS my_pkg",
		// CREATE APPLICATION PACKAGE — with DISTRIBUTION
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL",
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = EXTERNAL",
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = internal",
		// CREATE APPLICATION PACKAGE — with COMMENT
		"CREATE APPLICATION PACKAGE my_pkg COMMENT = 'my package'",
		// CREATE APPLICATION PACKAGE — with both DISTRIBUTION and COMMENT
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL COMMENT = 'internal pkg'",
		// CREATE APPLICATION PACKAGE — case insensitive
		"create application package my_pkg",
		"Create Application Package IF NOT EXISTS my_pkg",

		// ALTER APPLICATION PACKAGE — SET DEFAULT RELEASE DIRECTIVE
		"ALTER APPLICATION PACKAGE my_pkg SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 0",
		// ALTER APPLICATION PACKAGE — ADD VERSION
		"ALTER APPLICATION PACKAGE my_pkg ADD VERSION v1 USING @stage/path",
		// ALTER APPLICATION PACKAGE — DROP VERSION
		"ALTER APPLICATION PACKAGE my_pkg DROP VERSION v1",
		// ALTER APPLICATION PACKAGE — SET DISTRIBUTION
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = INTERNAL",
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = EXTERNAL",

		// DROP APPLICATION PACKAGE
		"DROP APPLICATION PACKAGE my_pkg",
		"DROP APPLICATION PACKAGE IF EXISTS my_pkg",

		// CREATE APPLICATION — basic with FROM APPLICATION PACKAGE
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg",
		"CREATE OR REPLACE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg",
		// CREATE APPLICATION — with USING VERSION and PATCH
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0",
		// CREATE APPLICATION — with DEBUG_MODE
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = TRUE",
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = FALSE",
		// CREATE APPLICATION — with COMMENT
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg COMMENT = 'my app'",
		// CREATE APPLICATION — with all optional properties
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0 DEBUG_MODE = TRUE COMMENT = 'full opts'",
		// CREATE APPLICATION — case insensitive
		"create application my_app from application package my_pkg",

		// ALTER APPLICATION — UPGRADE
		"ALTER APPLICATION my_app UPGRADE",
		// ALTER APPLICATION — UPGRADE USING VERSION ... PATCH ...
		"ALTER APPLICATION my_app UPGRADE USING VERSION v2 PATCH 1",
		// ALTER APPLICATION — SET DEBUG_MODE
		"ALTER APPLICATION my_app SET DEBUG_MODE = TRUE",
		"ALTER APPLICATION my_app SET DEBUG_MODE = FALSE",
		// ALTER APPLICATION — UNSET DEBUG_MODE
		"ALTER APPLICATION my_app UNSET DEBUG_MODE",

		// DROP APPLICATION
		"DROP APPLICATION my_app",
		"DROP APPLICATION IF EXISTS my_app",
		"DROP APPLICATION my_app CASCADE",
		"DROP APPLICATION IF EXISTS my_app CASCADE",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// CREATE APPLICATION PACKAGE — missing name
		{
			"CREATE APPLICATION PACKAGE missing name",
			"CREATE APPLICATION PACKAGE",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — OR REPLACE without name
		{
			"CREATE OR REPLACE APPLICATION PACKAGE missing name",
			"CREATE OR REPLACE APPLICATION PACKAGE",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — IF NOT EXISTS without name
		{
			"CREATE APPLICATION PACKAGE IF NOT EXISTS missing name",
			"CREATE APPLICATION PACKAGE IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE APPLICATION PACKAGE OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE APPLICATION PACKAGE IF NOT EXISTS my_pkg",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE APPLICATION PACKAGE — account-level prefix
		{
			"CREATE APPLICATION PACKAGE with db prefix",
			"CREATE APPLICATION PACKAGE db.my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION PACKAGE — account-level prefix (three-part name)
		{
			"CREATE APPLICATION PACKAGE with three-part name",
			"CREATE APPLICATION PACKAGE db.schema.my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION PACKAGE — invalid DISTRIBUTION
		{
			"CREATE APPLICATION PACKAGE invalid DISTRIBUTION",
			"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = PUBLIC",
			[]string{"DISTRIBUTION must be INTERNAL or EXTERNAL"},
		},
		// CREATE APPLICATION PACKAGE — unexpected property
		{
			"CREATE APPLICATION PACKAGE unexpected property",
			"CREATE APPLICATION PACKAGE my_pkg DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},

		// ALTER APPLICATION PACKAGE — missing name
		{
			"ALTER APPLICATION PACKAGE missing name",
			"ALTER APPLICATION PACKAGE",
			[]string{"ALTER APPLICATION PACKAGE requires a package name"},
		},
		// ALTER APPLICATION PACKAGE — unknown sub-command
		{
			"ALTER APPLICATION PACKAGE unknown action",
			"ALTER APPLICATION PACKAGE my_pkg ENABLE",
			[]string{"Unknown ALTER APPLICATION PACKAGE sub-command"},
		},
		// ALTER APPLICATION PACKAGE — SET with unknown property
		{
			"ALTER APPLICATION PACKAGE SET unknown property",
			"ALTER APPLICATION PACKAGE my_pkg SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER APPLICATION PACKAGE SET"},
		},
		// ALTER APPLICATION PACKAGE — invalid DISTRIBUTION value
		{
			"ALTER APPLICATION PACKAGE invalid DISTRIBUTION",
			"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = PUBLIC",
			[]string{"DISTRIBUTION must be INTERNAL or EXTERNAL"},
		},

		// DROP APPLICATION PACKAGE — missing name
		{
			"DROP APPLICATION PACKAGE missing name",
			"DROP APPLICATION PACKAGE",
			[]string{"DROP APPLICATION PACKAGE requires a package name"},
		},
		// DROP APPLICATION PACKAGE IF EXISTS — missing name
		{
			"DROP APPLICATION PACKAGE IF EXISTS missing name",
			"DROP APPLICATION PACKAGE IF EXISTS",
			[]string{"DROP APPLICATION PACKAGE requires a package name"},
		},

		// CREATE APPLICATION — missing name
		{
			"CREATE APPLICATION missing name",
			"CREATE APPLICATION",
			[]string{"Unexpected syntax in CREATE APPLICATION"},
		},
		// CREATE APPLICATION — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE APPLICATION OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE APPLICATION — account-level prefix
		{
			"CREATE APPLICATION with db prefix",
			"CREATE APPLICATION db.my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION — missing FROM APPLICATION PACKAGE
		{
			"CREATE APPLICATION missing FROM APPLICATION PACKAGE",
			"CREATE APPLICATION my_app",
			[]string{"Missing mandatory FROM APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION — USING VERSION without PATCH
		{
			"CREATE APPLICATION VERSION without PATCH",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1",
			[]string{"USING VERSION requires a PATCH number"},
		},
		// CREATE APPLICATION — DEBUG_MODE invalid value
		{
			"CREATE APPLICATION DEBUG_MODE invalid",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = MAYBE",
			[]string{"DEBUG_MODE must be TRUE or FALSE"},
		},
		// CREATE APPLICATION — unexpected property
		{
			"CREATE APPLICATION unexpected property",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},
		// CREATE APPLICATION — IF NOT EXISTS without name
		{
			"CREATE APPLICATION IF NOT EXISTS missing name",
			"CREATE APPLICATION IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE APPLICATION"},
		},

		// ALTER APPLICATION — missing name
		{
			"ALTER APPLICATION missing name",
			"ALTER APPLICATION",
			[]string{"ALTER APPLICATION requires an application name"},
		},
		// ALTER APPLICATION — unknown sub-command
		{
			"ALTER APPLICATION unknown action",
			"ALTER APPLICATION my_app ENABLE",
			[]string{"Unknown ALTER APPLICATION sub-command"},
		},
		// ALTER APPLICATION — SET with unknown property
		{
			"ALTER APPLICATION SET unknown property",
			"ALTER APPLICATION my_app SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER APPLICATION SET"},
		},
		// ALTER APPLICATION — UNSET with unknown property
		{
			"ALTER APPLICATION UNSET unknown property",
			"ALTER APPLICATION my_app UNSET UNKNOWN_PROP",
			[]string{"Unknown property in ALTER APPLICATION UNSET"},
		},
		// ALTER APPLICATION — DEBUG_MODE invalid value
		{
			"ALTER APPLICATION DEBUG_MODE invalid",
			"ALTER APPLICATION my_app SET DEBUG_MODE = MAYBE",
			[]string{"DEBUG_MODE must be TRUE or FALSE"},
		},
		// ALTER APPLICATION — UPGRADE USING VERSION without PATCH
		{
			"ALTER APPLICATION UPGRADE VERSION without PATCH",
			"ALTER APPLICATION my_app UPGRADE USING VERSION v2",
			[]string{"USING VERSION requires a PATCH number"},
		},

		// DROP APPLICATION — missing name
		{
			"DROP APPLICATION missing name",
			"DROP APPLICATION",
			[]string{"DROP APPLICATION requires an application name"},
		},
		// DROP APPLICATION IF EXISTS — missing name
		{
			"DROP APPLICATION IF EXISTS missing name",
			"DROP APPLICATION IF EXISTS",
			[]string{"DROP APPLICATION requires an application name"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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
}

func TestValidateSnowflakePatterns_GitRepository(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE GIT REPOSITORY — basic with mandatory properties
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		"CREATE OR REPLACE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		"CREATE GIT REPOSITORY IF NOT EXISTS my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — three-part name (schema-level object)
		"CREATE GIT REPOSITORY db.schema.my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — with GIT_CREDENTIALS
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' GIT_CREDENTIALS = my_secret",
		// CREATE GIT REPOSITORY — with COMMENT
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' COMMENT = 'main repo'",
		// CREATE GIT REPOSITORY — with all optional properties
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' GIT_CREDENTIALS = my_secret COMMENT = 'main repo'",
		// CREATE GIT REPOSITORY — git@ SSH URL
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'git@github.com:my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — case insensitive
		"create git repository my_repo api_integration = my_api_int origin = 'https://github.com/my-org/my-repo.git'",
		"Create Git Repository my_repo Api_Integration = my_int Origin = 'https://example.com/repo.git'",

		// ALTER GIT REPOSITORY — FETCH
		"ALTER GIT REPOSITORY my_repo FETCH",
		// ALTER GIT REPOSITORY — SET API_INTEGRATION
		"ALTER GIT REPOSITORY my_repo SET API_INTEGRATION = new_int",
		// ALTER GIT REPOSITORY — SET GIT_CREDENTIALS
		"ALTER GIT REPOSITORY my_repo SET GIT_CREDENTIALS = new_secret",
		// ALTER GIT REPOSITORY — SET COMMENT
		"ALTER GIT REPOSITORY my_repo SET COMMENT = 'updated comment'",
		// ALTER GIT REPOSITORY — UNSET GIT_CREDENTIALS
		"ALTER GIT REPOSITORY my_repo UNSET GIT_CREDENTIALS",
		// ALTER GIT REPOSITORY — UNSET COMMENT
		"ALTER GIT REPOSITORY my_repo UNSET COMMENT",
		// ALTER GIT REPOSITORY — three-part name
		"ALTER GIT REPOSITORY db.schema.my_repo FETCH",

		// DROP GIT REPOSITORY
		"DROP GIT REPOSITORY my_repo",
		"DROP GIT REPOSITORY IF EXISTS my_repo",
		"DROP GIT REPOSITORY db.schema.my_repo",
		"DROP GIT REPOSITORY IF EXISTS db.schema.my_repo",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// CREATE GIT REPOSITORY — missing name
		{
			"CREATE GIT REPOSITORY missing name",
			"CREATE GIT REPOSITORY",
			[]string{"Unexpected syntax in CREATE GIT REPOSITORY"},
		},
		// CREATE GIT REPOSITORY — IF NOT EXISTS without name
		{
			"CREATE GIT REPOSITORY IF NOT EXISTS missing name",
			"CREATE GIT REPOSITORY IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE GIT REPOSITORY"},
		},
		// CREATE GIT REPOSITORY — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE GIT REPOSITORY OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE GIT REPOSITORY IF NOT EXISTS my_repo API_INTEGRATION = my_int ORIGIN = 'https://example.com/repo.git'",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE GIT REPOSITORY — missing API_INTEGRATION
		{
			"CREATE GIT REPOSITORY missing API_INTEGRATION",
			"CREATE GIT REPOSITORY my_repo ORIGIN = 'https://github.com/my-org/my-repo.git'",
			[]string{"requires API_INTEGRATION"},
		},
		// CREATE GIT REPOSITORY — missing ORIGIN
		{
			"CREATE GIT REPOSITORY missing ORIGIN",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int",
			[]string{"requires ORIGIN"},
		},
		// CREATE GIT REPOSITORY — missing both mandatory properties
		{
			"CREATE GIT REPOSITORY missing both mandatory",
			"CREATE GIT REPOSITORY my_repo",
			[]string{"requires API_INTEGRATION", "requires ORIGIN"},
		},
		// CREATE GIT REPOSITORY — ORIGIN not a string literal
		{
			"CREATE GIT REPOSITORY ORIGIN not string literal",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = https://example.com",
			[]string{"ORIGIN value must be a string literal"},
		},
		// CREATE GIT REPOSITORY — ORIGIN with invalid URL scheme
		{
			"CREATE GIT REPOSITORY ORIGIN invalid URL",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'http://example.com/repo.git'",
			[]string{"ORIGIN URL should start with 'https://' or 'git@'"},
		},
		// CREATE GIT REPOSITORY — ORIGIN with ftp URL
		{
			"CREATE GIT REPOSITORY ORIGIN ftp URL",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'ftp://example.com/repo.git'",
			[]string{"ORIGIN URL should start with 'https://' or 'git@'"},
		},
		// CREATE GIT REPOSITORY — unexpected property
		{
			"CREATE GIT REPOSITORY unexpected property",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'https://example.com/repo.git' AUTO_REFRESH = TRUE",
			[]string{"Unexpected property 'AUTO_REFRESH'"},
		},

		// ALTER GIT REPOSITORY — missing name
		{
			"ALTER GIT REPOSITORY missing name",
			"ALTER GIT REPOSITORY",
			[]string{"ALTER GIT REPOSITORY requires a repository name"},
		},
		// ALTER GIT REPOSITORY — unknown sub-command
		{
			"ALTER GIT REPOSITORY unknown action",
			"ALTER GIT REPOSITORY my_repo SYNC",
			[]string{"Unknown ALTER GIT REPOSITORY sub-command"},
		},

		// DROP GIT REPOSITORY — missing name
		{
			"DROP GIT REPOSITORY missing name",
			"DROP GIT REPOSITORY",
			[]string{"DROP GIT REPOSITORY requires a repository name"},
		},
		// DROP GIT REPOSITORY IF EXISTS — missing name
		{
			"DROP GIT REPOSITORY IF EXISTS missing name",
			"DROP GIT REPOSITORY IF EXISTS",
			[]string{"DROP GIT REPOSITORY requires a repository name"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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
}

func TestValidateSnowflakePatterns_Secret(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// -- PASS: CREATE SECRET TYPE = OAUTH2 with mandatory API_AUTHENTICATION
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_security_integration",
		// -- PASS: CREATE SECRET TYPE = OAUTH2 with all optional properties
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_SCOPES = ('scope1', 'scope2') OAUTH_REFRESH_TOKEN = 'token123' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31 00:00:00' COMMENT = 'my oauth secret'",
		// -- PASS: CREATE SECRET TYPE = PASSWORD with USERNAME and PASSWORD
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'myuser' PASSWORD = 'mypass'",
		// -- PASS: CREATE SECRET TYPE = PASSWORD with COMMENT
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'myuser' PASSWORD = 'mypass' COMMENT = 'basic auth'",
		// -- PASS: CREATE SECRET TYPE = GENERIC_STRING with SECRET_STRING
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'some-api-key-value'",
		// -- PASS: CREATE SECRET TYPE = GENERIC_STRING with COMMENT
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc123' COMMENT = 'api key'",
		// -- PASS: CREATE SECRET TYPE = CLOUD_PROVIDER_TOKEN with API_AUTHENTICATION
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int",
		// -- PASS: CREATE SECRET TYPE = CLOUD_PROVIDER_TOKEN with ENABLED
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int ENABLED = TRUE",
		// -- PASS: CREATE SECRET TYPE = SYMMETRIC_KEY with ALGORITHM
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256'",
		// -- PASS: CREATE SECRET TYPE = SYMMETRIC_KEY with COMMENT
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' COMMENT = 'encryption key'",
		// -- PASS: OR REPLACE variant
		"CREATE OR REPLACE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: IF NOT EXISTS variant
		"CREATE SECRET IF NOT EXISTS my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: Three-part name (schema-level object)
		"CREATE SECRET db.schema.my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: Case insensitivity
		"create secret my_secret type = oauth2 api_authentication = my_int",
		"Create Secret my_secret Type = Password Username = 'u' Password = 'p'",

		// ALTER SECRET — valid cases
		// -- PASS: ALTER SECRET SET for GENERIC_STRING
		"ALTER SECRET my_secret SET SECRET_STRING = 'new_value'",
		// -- PASS: ALTER SECRET SET for PASSWORD
		"ALTER SECRET my_secret SET USERNAME = 'new_user' PASSWORD = 'new_pass'",
		// -- PASS: ALTER SECRET SET for OAUTH2
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN = 'new_token' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2026-01-01 00:00:00'",
		// -- PASS: ALTER SECRET SET OAUTH_SCOPES
		"ALTER SECRET my_secret SET OAUTH_SCOPES = ('scope1', 'scope2')",
		// -- PASS: ALTER SECRET SET API_AUTHENTICATION
		"ALTER SECRET my_secret SET API_AUTHENTICATION = new_int",
		// -- PASS: ALTER SECRET SET COMMENT
		"ALTER SECRET my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET UNSET COMMENT
		"ALTER SECRET my_secret UNSET COMMENT",
		// -- PASS: ALTER SECRET with three-part name
		"ALTER SECRET db.schema.my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET IF EXISTS
		"ALTER SECRET IF EXISTS my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET IF EXISTS with three-part name
		"ALTER SECRET IF EXISTS db.schema.my_secret SET SECRET_STRING = 'new_val'",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// -- FAIL: CREATE SECRET missing name
		{
			"CREATE SECRET missing name",
			"CREATE SECRET",
			[]string{"Unexpected syntax in CREATE SECRET"},
		},
		// -- FAIL: IF NOT EXISTS without name
		{
			"CREATE SECRET IF NOT EXISTS missing name",
			"CREATE SECRET IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE SECRET"},
		},
		// -- FAIL: OR REPLACE + IF NOT EXISTS conflict
		{
			"CREATE SECRET OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE SECRET IF NOT EXISTS my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// -- FAIL: Missing TYPE
		{
			"CREATE SECRET missing TYPE",
			"CREATE SECRET my_secret API_AUTHENTICATION = my_int",
			[]string{"CREATE SECRET requires TYPE"},
		},
		// -- FAIL: Unknown TYPE value
		{
			"CREATE SECRET unknown TYPE",
			"CREATE SECRET my_secret TYPE = BEARER_TOKEN SECRET_STRING = 'abc'",
			[]string{"Unknown TYPE"},
		},
		// -- FAIL: TYPE = OAUTH2 missing API_AUTHENTICATION
		{
			"CREATE SECRET OAUTH2 missing API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = OAUTH2",
			[]string{"TYPE = OAUTH2 requires API_AUTHENTICATION"},
		},
		// -- FAIL: TYPE = PASSWORD missing USERNAME
		{
			"CREATE SECRET PASSWORD missing USERNAME",
			"CREATE SECRET my_secret TYPE = PASSWORD PASSWORD = 'p'",
			[]string{"TYPE = PASSWORD requires USERNAME"},
		},
		// -- FAIL: TYPE = PASSWORD missing PASSWORD
		{
			"CREATE SECRET PASSWORD missing PASSWORD",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u'",
			[]string{"TYPE = PASSWORD requires PASSWORD"},
		},
		// -- FAIL: TYPE = PASSWORD missing both
		{
			"CREATE SECRET PASSWORD missing both",
			"CREATE SECRET my_secret TYPE = PASSWORD",
			[]string{"TYPE = PASSWORD requires USERNAME", "TYPE = PASSWORD requires PASSWORD"},
		},
		// -- FAIL: TYPE = GENERIC_STRING missing SECRET_STRING
		{
			"CREATE SECRET GENERIC_STRING missing SECRET_STRING",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING",
			[]string{"TYPE = GENERIC_STRING requires SECRET_STRING"},
		},
		// -- FAIL: TYPE = CLOUD_PROVIDER_TOKEN missing API_AUTHENTICATION
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN missing API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN",
			[]string{"TYPE = CLOUD_PROVIDER_TOKEN requires API_AUTHENTICATION"},
		},
		// -- FAIL: TYPE = SYMMETRIC_KEY missing ALGORITHM
		{
			"CREATE SECRET SYMMETRIC_KEY missing ALGORITHM",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY",
			[]string{"TYPE = SYMMETRIC_KEY requires ALGORITHM"},
		},
		// -- FAIL: USERNAME on OAUTH2 type (wrong type property)
		{
			"CREATE SECRET OAUTH2 with USERNAME",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int USERNAME = 'u'",
			[]string{"USERNAME is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: SECRET_STRING on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with SECRET_STRING",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' SECRET_STRING = 'abc'",
			[]string{"SECRET_STRING is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: API_AUTHENTICATION on GENERIC_STRING type (wrong type property)
		{
			"CREATE SECRET GENERIC_STRING with API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc' API_AUTHENTICATION = my_int",
			[]string{"API_AUTHENTICATION is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: USERNAME on SYMMETRIC_KEY type (wrong type property)
		{
			"CREATE SECRET SYMMETRIC_KEY with USERNAME",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' USERNAME = 'u'",
			[]string{"USERNAME is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: ALGORITHM on OAUTH2 type (wrong type property)
		{
			"CREATE SECRET OAUTH2 with ALGORITHM",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int ALGORITHM = 'AES-256'",
			[]string{"ALGORITHM is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: ENABLED on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with ENABLED",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' ENABLED = TRUE",
			[]string{"ENABLED is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with OAUTH_REFRESH_TOKEN",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_REFRESH_TOKEN is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: OAUTH_SCOPES on GENERIC_STRING type (wrong type property)
		{
			"CREATE SECRET GENERIC_STRING with OAUTH_SCOPES",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc' OAUTH_SCOPES = ('s1')",
			[]string{"OAUTH_SCOPES is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: Unexpected property
		{
			"CREATE SECRET unexpected property",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' WAREHOUSE = my_wh",
			[]string{"Unexpected property"},
		},

		// ALTER SECRET — invalid cases
		// -- FAIL: ALTER SECRET missing name
		{
			"ALTER SECRET missing name",
			"ALTER SECRET",
			[]string{"ALTER SECRET requires a secret name"},
		},
		// -- FAIL: ALTER SECRET unknown sub-command
		{
			"ALTER SECRET unknown action",
			"ALTER SECRET my_secret REFRESH",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET IF EXISTS unknown sub-command
		{
			"ALTER SECRET IF EXISTS unknown action",
			"ALTER SECRET IF EXISTS my_secret REFRESH",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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
}

// ── Cortex AI function call tests ─────────────────────────────────────────────

func TestCortexAI_ValidateTablesExist_NoFalsePositives(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', user_prompt) FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(review_body) AS score FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(doc_text, 'What is the deadline?') FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', chunk) FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(article_text) FROM LIVE_TABLE",
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:   getLiveRefs(),
		KnownDatabases: []string{"DB", "GOVERNANCE"},
		KnownSchemas:   []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "GOVERNANCE", Name: "PUBLIC"}},
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateTablesExist(req)
			errs := getErrors(markers)
			if len(errs) > 0 {
				t.Errorf("Expected 0 errors for %q, got %d: %v", sql, len(errs), errs[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateBareColumnRefs_NoFalsePositives(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', ID) FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(FIRST_NAME) AS score FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(FIRST_NAME, 'What is the deadline?') FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', FIRST_NAME) FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(LAST_NAME) FROM DB.SCH.EMPLOYEES",
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateSnowflakePatterns_KnownFunctions(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', user_prompt) FROM prompts",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(review_body) AS score FROM reviews",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(doc_text, 'What is the deadline?') FROM contracts",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', chunk) FROM corpus",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(article_text) FROM news",
		"SELECT SNOWFLAKE.CORTEX.TRANSLATE(text_col, 'en', 'fr') FROM docs",
		"SELECT SNOWFLAKE.CORTEX.CLASSIFY_TEXT(review, ARRAY_CONSTRUCT('positive', 'negative')) FROM reviews",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_1024('model', chunk) FROM corpus",
		"SELECT SNOWFLAKE.CORTEX.TRY_COMPLETE('model', prompt) FROM t",
		"SELECT SNOWFLAKE.CORTEX.SEARCH_PREVIEW('service', query) FROM t",
		"SELECT SNOWFLAKE.CORTEX.FINETUNE('model', 'train_data') FROM t",
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			allMarkers := append(getErrors(markers), getWarnings(markers)...)
			if len(allMarkers) > 0 {
				t.Errorf("Expected 0 markers for %q, got %d: %v", sql, len(allMarkers), allMarkers[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateSnowflakePatterns_UnknownFunction(t *testing.T) {
	invalidQueries := []struct {
		sql     string
		wantMsg string
	}{
		{
			sql:     "SELECT SNOWFLAKE.CORTEX.MAGIC_ANSWER(col) FROM t",
			wantMsg: "Unknown Cortex function",
		},
		{
			sql:     "SELECT SNOWFLAKE.CORTEX.DOES_NOT_EXIST(col) FROM t",
			wantMsg: "Unknown Cortex function",
		},
	}

	for _, tc := range invalidQueries {
		t.Run(tc.sql[:min(len(tc.sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatalf("Expected warning for %q, got none", tc.sql)
			}
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, tc.wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns[0].Message)
			}
		})
	}
}

func TestCortexAI_DuplicateUnknownFunction_DistinctMarkers(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.MAGIC(a), SNOWFLAKE.CORTEX.MAGIC(b) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 2 {
		t.Fatalf("Expected 2 warnings for duplicate unknown Cortex function, got %d", len(warns))
	}
	// The two markers must have distinct column positions.
	if warns[0].StartColumn == warns[1].StartColumn {
		t.Errorf("Both markers have the same StartColumn=%d; expected distinct positions", warns[0].StartColumn)
	}
}

func TestCortexAI_NoFalsePositivesInCommentsAndStrings(t *testing.T) {
	noWarnQueries := []string{
		// Line comment
		"-- SELECT SNOWFLAKE.CORTEX.MAGIC_FUNC(x)\nSELECT 1",
		// Block comment
		"/* SNOWFLAKE.CORTEX.FAKE_FUNC(x) */ SELECT 1",
		// String literal
		"SELECT 'SNOWFLAKE.CORTEX.FAKE_FUNC(x)' FROM t",
		// Dollar-quoted string
		"EXECUTE IMMEDIATE $$SELECT SNOWFLAKE.CORTEX.FAKE_FUNC(x) FROM t$$",
		// Dollar-quoted procedure body with tagged delimiter
		"CREATE PROCEDURE p() RETURNS STRING LANGUAGE SQL AS $body$\n  SELECT SNOWFLAKE.CORTEX.NOT_REAL(col) FROM t;\n$body$",
	}

	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			for _, w := range warns {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_CaseInsensitive(t *testing.T) {
	// Known function in lowercase — should produce no warning
	validQueries := []string{
		"SELECT snowflake.cortex.complete('mistral-7b', prompt) FROM t",
		"SELECT Snowflake.Cortex.Sentiment(review) FROM t",
		"SELECT SNOWFLAKE.cortex.SUMMARIZE(text) FROM t",
	}
	for _, sql := range validQueries {
		t.Run("valid/"+sql[:min(len(sql), 40)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for %q, got: %v", sql, w.Message)
				}
			}
		})
	}

	// Unknown function in lowercase — should still produce a warning
	invalidSQL := "SELECT snowflake.cortex.magic_answer(col) FROM t"
	t.Run("invalid/"+invalidSQL[:40], func(t *testing.T) {
		stmtRanges := GetStatementRanges(invalidSQL)
		markers := ValidateSnowflakePatterns(invalidSQL, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected unknown Cortex function warning for %q, got none", invalidSQL)
		}
	})
}

// ── Notebook Tests ──────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Notebook(t *testing.T) {
	t.Run("valid CREATE NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"CREATE NOTEBOOK my_nb",
			"CREATE OR REPLACE NOTEBOOK my_nb",
			"CREATE NOTEBOOK IF NOT EXISTS my_nb",
			"CREATE NOTEBOOK db.schema.my_nb",
			"CREATE OR REPLACE NOTEBOOK db.schema.my_nb",
			"CREATE NOTEBOOK IF NOT EXISTS db.schema.my_nb",
			"CREATE NOTEBOOK my_nb FROM '@my_stage/path' MAIN_FILE = 'notebook.ipynb'",
			"CREATE NOTEBOOK my_nb FROM '@db.schema.stage/dir/file' MAIN_FILE = 'my_nb.ipynb' QUERY_WAREHOUSE = my_wh",
			"CREATE NOTEBOOK my_nb QUERY_WAREHOUSE = my_wh",
			"CREATE NOTEBOOK my_nb COMMENT = 'A test notebook'",
			"CREATE NOTEBOOK my_nb FROM '@stage/path' MAIN_FILE = 'nb.ipynb' COMMENT = 'imported'",
			"CREATE NOTEBOOK my_nb IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 3600",
			// COMMENT containing FROM should not trigger MAIN_FILE requirement
			`CREATE NOTEBOOK my_nb COMMENT = 'imported FROM ''@stage/path'' stuff'`,
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid CREATE NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "CREATE NOTEBOOK",
				wantMsg: "CREATE NOTEBOOK requires a notebook name",
			},
			{
				sql:     "CREATE OR REPLACE NOTEBOOK IF NOT EXISTS my_nb",
				wantMsg: "Conflict between OR REPLACE and IF NOT EXISTS",
			},
			{
				sql:     "CREATE NOTEBOOK my_nb FROM '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("valid ALTER NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"ALTER NOTEBOOK my_nb SET COMMENT = 'updated'",
			"ALTER NOTEBOOK my_nb SET QUERY_WAREHOUSE = my_wh",
			"ALTER NOTEBOOK my_nb UNSET COMMENT",
			"ALTER NOTEBOOK my_nb UNSET QUERY_WAREHOUSE",
			"ALTER NOTEBOOK my_nb RENAME TO new_nb",
			"ALTER NOTEBOOK db.schema.my_nb RENAME TO db.schema.new_nb",
			"ALTER NOTEBOOK IF EXISTS my_nb SET COMMENT = 'safe update'",
			"ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM LAST",
			"ALTER NOTEBOOK my_nb SET IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 7200",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "ALTER NOTEBOOK",
				wantMsg: "ALTER NOTEBOOK requires a notebook name",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb RENAME TO",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM FIRST",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("valid DROP NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"DROP NOTEBOOK my_nb",
			"DROP NOTEBOOK IF EXISTS my_nb",
			"DROP NOTEBOOK db.schema.my_nb",
			"DROP NOTEBOOK IF EXISTS db.schema.my_nb",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid DROP NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "DROP NOTEBOOK",
				wantMsg: "DROP NOTEBOOK requires a notebook name",
			},
			{
				sql:     "DROP NOTEBOOK my_nb CASCADE",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			{
				sql:     "DROP NOTEBOOK my_nb RESTRICT",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

// ── PIVOT / UNPIVOT Tests ────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Pivot(t *testing.T) {
	t.Run("valid PIVOT queries", func(t *testing.T) {
		validQueries := []string{
			// Basic PIVOT with SUM
			"SELECT * FROM monthly_sales PIVOT (SUM(amount) FOR month IN ('Jan', 'Feb', 'Mar')) AS p",
			// PIVOT with AVG
			"SELECT * FROM sales PIVOT (AVG(revenue) FOR quarter IN ('Q1', 'Q2', 'Q3', 'Q4'))",
			// PIVOT with COUNT
			"SELECT * FROM events PIVOT (COUNT(event_id) FOR status IN ('active', 'inactive'))",
			// PIVOT with MAX
			"SELECT * FROM readings PIVOT (MAX(value) FOR sensor IN ('temp', 'humidity')) AS pvt",
			// PIVOT with MIN
			"SELECT * FROM readings PIVOT (MIN(value) FOR sensor IN ('temp', 'humidity'))",
			// PIVOT with ANY_VALUE
			"SELECT * FROM data PIVOT (ANY_VALUE(val) FOR key IN ('a', 'b'))",
			// PIVOT with LISTAGG
			"SELECT * FROM data PIVOT (LISTAGG(name) FOR category IN ('x', 'y'))",
			// PIVOT with MEDIAN
			"SELECT * FROM data PIVOT (MEDIAN(score) FOR subject IN ('math', 'science'))",
			// PIVOT with STDDEV
			"SELECT * FROM data PIVOT (STDDEV(measurement) FOR sensor IN ('s1', 's2'))",
			// PIVOT with VARIANCE
			"SELECT * FROM data PIVOT (VARIANCE(amount) FOR region IN ('east', 'west'))",
			// PIVOT with numeric values in IN list
			"SELECT * FROM data PIVOT (SUM(val) FOR code IN (1, 2, 3))",
			// PIVOT with fully qualified table
			"SELECT * FROM db.schema.monthly_sales PIVOT (SUM(amount) FOR month IN ('Jan', 'Feb'))",
			// PIVOT with alias on the source table
			"SELECT * FROM sales_data s PIVOT (SUM(s.amount) FOR s.month IN ('Jan', 'Feb'))",
			// Mixed-case keywords
			"SELECT * FROM t pivot (sum(amount) for month in ('Jan', 'Feb'))",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid PIVOT queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "SELECT * FROM sales PIVOT (INVALID_FUNC(amount) FOR month IN ('Jan'))",
				wantMsg: "not a valid aggregate function",
			},
			{
				sql:     "SELECT * FROM sales PIVOT (SUM(amount) FOR month IN ())",
				wantMsg: "PIVOT IN list must not be empty",
			},
			{
				sql:     "SELECT * FROM sales PIVOT (SUM(amount) IN ('Jan'))",
				wantMsg: "PIVOT requires FOR <column> IN",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateSnowflakePatterns_Unpivot(t *testing.T) {
	t.Run("valid UNPIVOT queries", func(t *testing.T) {
		validQueries := []string{
			// Basic UNPIVOT
			"SELECT * FROM wide_table UNPIVOT (value FOR metric IN (col_a, col_b, col_c))",
			// UNPIVOT with alias
			"SELECT * FROM wide_table UNPIVOT (val FOR col_name IN (q1, q2, q3, q4)) AS u",
			// UNPIVOT with fully qualified table
			"SELECT * FROM db.schema.wide_table UNPIVOT (value FOR metric IN (col_a, col_b))",
			// UNPIVOT with quoted identifiers
			`SELECT * FROM wide_table UNPIVOT ("value" FOR "metric" IN ("COL_A", "COL_B"))`,
			// UNPIVOT INCLUDE NULLS
			"SELECT * FROM wide_table UNPIVOT INCLUDE NULLS (value FOR metric IN (col_a, col_b))",
			// UNPIVOT EXCLUDE NULLS
			"SELECT * FROM wide_table UNPIVOT EXCLUDE NULLS (value FOR metric IN (col_a, col_b))",
			// No space before opening paren
			"SELECT * FROM wide_table UNPIVOT(value FOR metric IN (col_a, col_b))",
			// Mixed-case keywords
			"SELECT * FROM t unpivot (value FOR metric IN (col_a, col_b))",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid UNPIVOT queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "SELECT * FROM wide_table UNPIVOT (value FOR metric IN ())",
				wantMsg: "UNPIVOT IN list must not be empty",
			},
			{
				sql:     "SELECT * FROM wide_table UNPIVOT (value IN (col_a, col_b))",
				wantMsg: "UNPIVOT requires FOR <name_column> IN",
			},
			{
				sql:     "SELECT * FROM wide_table UNPIVOT INCLUDE NULLS (value FOR metric IN ())",
				wantMsg: "UNPIVOT IN list must not be empty",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateBareColumnRefs_PivotSuppression(t *testing.T) {
	// PIVOT/UNPIVOT queries should not produce false-positive column warnings
	// because the generated columns are dynamic / virtual.
	validQueries := []string{
		// PIVOT — the columns 'Alice','Bob' are generated dynamically; should not flag
		`SELECT * FROM DB.SCH.EMPLOYEES PIVOT (COUNT(ID) FOR FIRST_NAME IN ('Alice', 'Bob'))`,
		// PIVOT with alias — p.Alice, p.Bob should not be flagged
		`SELECT p.ID FROM DB.SCH.EMPLOYEES PIVOT (COUNT(ID) FOR FIRST_NAME IN ('Alice', 'Bob')) AS p`,
		// UNPIVOT — "value" and "metric" are generated columns
		`SELECT * FROM DB.SCH.EMPLOYEES UNPIVOT (value FOR metric IN (FIRST_NAME, LAST_NAME))`,
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for PIVOT/UNPIVOT query %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// ── MATCH_RECOGNIZE Tests ────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_MatchRecognize(t *testing.T) {
	t.Run("valid MATCH_RECOGNIZE queries", func(t *testing.T) {
		validQueries := []string{
			// Basic MATCH_RECOGNIZE with all clauses
			`SELECT * FROM stock_prices
MATCH_RECOGNIZE (
  PARTITION BY symbol
  ORDER BY trade_date
  MEASURES
    FIRST(A.price) AS start_price,
    LAST(B.price)  AS end_price
  ONE ROW PER MATCH
  PATTERN (A B+)
  DEFINE
    A AS price < AVG(price) OVER (ROWS BETWEEN 5 PRECEDING AND CURRENT ROW),
    B AS price > A.price
) AS mr`,
			// Minimal: only mandatory PATTERN
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) DEFINE A AS col > 0, B AS col < 0)`,
			// ALL ROWS PER MATCH
			`SELECT * FROM t MATCH_RECOGNIZE (ALL ROWS PER MATCH PATTERN (X+) DEFINE X AS val > 10)`,
			// AFTER MATCH SKIP TO NEXT ROW
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO NEXT ROW DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP PAST LAST ROW
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP PAST LAST ROW DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP TO FIRST <var>
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO FIRST A DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP TO LAST <var>
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO LAST B DEFINE A AS x > 0, B AS x < 0)`,
			// Mixed-case keywords
			`SELECT * FROM t match_recognize (pattern (a b+) define a AS col > 0, b AS col < 0)`,
			// With alias on result
			`SELECT mr.start_price FROM prices MATCH_RECOGNIZE (ORDER BY ts MEASURES FIRST(A.price) AS start_price PATTERN (A B+) DEFINE A AS price < 100, B AS price > A.price) AS mr`,
			// No space before opening paren
			`SELECT * FROM t MATCH_RECOGNIZE(PATTERN (X) DEFINE X AS val > 0)`,
			// With fully qualified table
			`SELECT * FROM db.schema.events MATCH_RECOGNIZE (ORDER BY ts PATTERN (A B) DEFINE A AS type = 'login', B AS type = 'purchase')`,
			// Complex PATTERN with quantifiers
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B* C+? D{2,5}) DEFINE A AS x=1, B AS x=2, C AS x=3, D AS x=4)`,
			// Keywords inside string literals must not trigger false positives
			`SELECT * FROM t MATCH_RECOGNIZE (
    PATTERN (A B)
    DEFINE A AS col = 'PATTERN', B AS col = 'DEFINE'
)`,
			// AFTER MATCH SKIP TO FIRST with quoted identifier
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO FIRST "myVar" DEFINE A AS x > 0, B AS x < 0)`,
			// Multiple MATCH_RECOGNIZE in a single statement (subquery)
			`SELECT * FROM (SELECT * FROM t1 MATCH_RECOGNIZE (PATTERN (A B) DEFINE A AS x > 0, B AS x < 0)) a JOIN (SELECT * FROM t2 MATCH_RECOGNIZE (PATTERN (X Y+) DEFINE X AS v = 1, Y AS v = 2)) b ON a.id = b.id`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for MATCH_RECOGNIZE query, got: %v", warns)
				}
			})
		}
	})

	t.Run("invalid MATCH_RECOGNIZE queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (DEFINE A AS col > 0)`,
				wantMsg: "MATCH_RECOGNIZE requires a PATTERN clause",
			},
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN () DEFINE A AS col > 0)`,
				wantMsg: "MATCH_RECOGNIZE PATTERN must contain at least one pattern variable",
			},
			{
				sql: `SELECT * FROM t MATCH_RECOGNIZE (
					ONE ROW PER MATCH
					ALL ROWS PER MATCH
					PATTERN (A B)
					DEFINE A AS x > 0, B AS x < 0
				)`,
				wantMsg: "ONE ROW PER MATCH and ALL ROWS PER MATCH are mutually exclusive",
			},
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP NOWHERE DEFINE A AS x > 0, B AS x < 0)`,
				wantMsg: "Invalid AFTER MATCH SKIP target",
			},
			// Multi-line: invalid AFTER MATCH SKIP must be caught even when
			// DEFINE is on a separate line.
			{
				sql: `SELECT * FROM t MATCH_RECOGNIZE (
  PATTERN (A B)
  AFTER MATCH SKIP NOWHERE
  DEFINE A AS x > 0, B AS x < 0
)`,
				wantMsg: "Invalid AFTER MATCH SKIP target",
			},
			// Missing DEFINE clause.
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B))`,
				wantMsg: "MATCH_RECOGNIZE requires a DEFINE clause",
			},
			// DEFINE keyword present but without any variable bindings.
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) DEFINE)`,
				wantMsg: "MATCH_RECOGNIZE requires a DEFINE clause",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})
}

func TestValidateSnowflakePatterns_AsofJoin(t *testing.T) {
	t.Run("valid ASOF JOIN queries", func(t *testing.T) {
		validQueries := []string{
			// Basic ASOF JOIN with MATCH_CONDITION >=
			`SELECT a.ts, a.val, b.price FROM measurements a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with WHERE clause
			`SELECT a.ts, a.val, b.price FROM measurements a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts) WHERE a.sensor = 'X'`,
			// ASOF JOIN with MATCH_CONDITION using >
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts > t2.ts)`,
			// ASOF JOIN with MATCH_CONDITION using <=
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts <= t2.ts)`,
			// ASOF JOIN with MATCH_CONDITION using <
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts < t2.ts)`,
			// Multiline ASOF JOIN
			`SELECT a.ts, b.price
			 FROM measurements a
			 ASOF JOIN prices b
			   MATCH_CONDITION (a.ts >= b.ts)
			 WHERE a.sensor = 'X'`,
			// ASOF JOIN with fully qualified table names
			`SELECT * FROM db.schema.measurements a ASOF JOIN db.schema.prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with quoted identifiers
			`SELECT * FROM "DB"."SCH"."MEASUREMENTS" a ASOF JOIN "DB"."SCH"."PRICES" b MATCH_CONDITION (a."TS" >= b."TS")`,
			// Multiple columns in MATCH_CONDITION expression
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.event_time >= t2.event_time)`,
			// ASOF JOIN with subquery context
			`SELECT * FROM (SELECT ts, val FROM raw_data) a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with CTE
			`WITH cte AS (SELECT ts, val FROM measurements) SELECT * FROM cte a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// USING FUNCTION form (custom matching logic)
			`SELECT * FROM t1 ASOF JOIN t2 USING (my_match_func(t1.ts, t2.ts))`,
			// USING FUNCTION form with qualified function name
			`SELECT * FROM t1 ASOF JOIN t2 USING (db.schema.my_func(t1.ts, t2.ts))`,
			// Multiple ASOF JOINs in one statement
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) ASOF JOIN t3 MATCH_CONDITION (t1.ts >= t3.ts)`,
			// ASOF JOIN with subquery containing a regular JOIN with ON
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y ON x.id = y.id) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
			// ASOF JOIN with subquery containing a regular JOIN with USING
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y USING (id)) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
			// Table name containing "ON" (e.g. options)
			`SELECT * FROM t1 ASOF JOIN options MATCH_CONDITION (t1.ts >= options.ts)`,
			// Nested ASOF JOIN inside subquery — outer scope must not be truncated
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x ASOF JOIN y MATCH_CONDITION (x.ts >= y.ts)) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ASOF JOIN queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing MATCH_CONDITION
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 WHERE t1.ts >= t2.ts`,
				wantMsg: "ASOF JOIN requires a MATCH_CONDITION clause",
			},
			// Bare ASOF JOIN without any condition
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2`,
				wantMsg: "ASOF JOIN requires a MATCH_CONDITION clause",
			},
			// ON clause used instead of MATCH_CONDITION
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 ON t1.ts >= t2.ts`,
				wantMsg: "ON clause is not valid with ASOF JOIN",
			},
			// USING column-list clause (not USING FUNCTION)
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 USING (ts)`,
				wantMsg: "USING clause is not valid with ASOF JOIN",
			},
			// Invalid comparison operator: =
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts = t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
			},
			// Invalid comparison operator: <>
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts <> t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
			},
			// Invalid comparison operator: !=
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts != t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})

	t.Run("ON/USING without MATCH_CONDITION produces single warning", func(t *testing.T) {
		// When ON or USING is used instead of MATCH_CONDITION, only the
		// ON/USING warning should appear — not a redundant "requires
		// MATCH_CONDITION" warning on top of it.
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 ON t1.ts >= t2.ts`,
				wantMsg: "ON clause is not valid with ASOF JOIN",
			},
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 USING (ts)`,
				wantMsg: "USING clause is not valid with ASOF JOIN",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) != 1 {
					t.Errorf("Expected exactly 1 warning, got %d: %v", len(warns), warns)
				}
				if len(warns) > 0 && !strings.Contains(warns[0].Message, tc.wantMsg) {
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})
}

func TestValidateSnowflakePatterns_InsertAllFirstOverwrite(t *testing.T) {
	t.Run("valid INSERT ALL queries", func(t *testing.T) {
		validQueries := []string{
			// Unconditional INSERT ALL (no WHEN)
			`INSERT ALL
			   INTO t1 (id, amount) VALUES (id, amount)
			   INTO t2 (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM source`,
			// Unconditional INSERT ALL without VALUES
			`INSERT ALL
			   INTO t1 (id, amount)
			   INTO t2 (id, amount)
			 SELECT id, amount FROM source`,
			// Unconditional INSERT ALL without column list or VALUES
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id, amount FROM source`,
			// Conditional INSERT ALL with WHEN/THEN
			`INSERT ALL
			   WHEN amount > 1000 THEN INTO large_orders (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount)
			   ELSE INTO small_orders (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// Conditional INSERT ALL with VALUES
			`INSERT ALL
			   WHEN amount > 1000 THEN INTO large_orders (id, amount) VALUES (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount) VALUES (id, amount)
			   ELSE INTO small_orders (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT ALL with multiple INTO per WHEN
			`INSERT ALL
			   WHEN status = 'A' THEN INTO t1 INTO t2
			   ELSE INTO t3
			 SELECT id, status FROM source`,
			// INSERT ALL without ELSE
			`INSERT ALL
			   WHEN amount > 0 THEN INTO positive_amounts (id, amount)
			 SELECT id, amount FROM source`,
			// INSERT ALL with fully qualified table names
			`INSERT ALL
			   WHEN x > 0 THEN INTO db.sch.t1 (id) VALUES (id)
			   ELSE INTO db.sch.t2 (id) VALUES (id)
			 SELECT id, x FROM source`,
			// INSERT ALL with quoted identifiers
			`INSERT ALL
			   WHEN x > 0 THEN INTO "MY_TABLE" (id)
			 SELECT id, x FROM source`,
			// INSERT OVERWRITE ALL (unconditional)
			`INSERT OVERWRITE ALL
			   INTO t1
			   INTO t2
			 SELECT id FROM source`,
			// INSERT OVERWRITE ALL (conditional)
			`INSERT OVERWRITE ALL
			   WHEN x > 0 THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM source`,
			// CASE WHEN/ELSE in trailing SELECT must not trigger false positive
			`INSERT ALL
			   INTO t1 (id, label)
			   INTO t2 (id, label)
			 SELECT id, CASE WHEN status = 1 THEN 'active' ELSE 'inactive' END AS label FROM source`,
			// Subquery with WHEN/ELSE in trailing SELECT
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id, (SELECT CASE WHEN x > 0 THEN 1 ELSE 0 END FROM y) AS flag FROM source`,
			// String literal containing WHEN/ELSE
			`INSERT ALL
			   INTO t1 (id, val)
			 SELECT id, 'WHEN ELSE SELECT INTO' AS val FROM source`,
			// CTE with INSERT ALL (WITH ... SELECT)
			`INSERT ALL
			   INTO t1
			   INTO t2
			 WITH cte AS (SELECT id, amount FROM raw_data) SELECT * FROM cte`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("valid INSERT FIRST queries", func(t *testing.T) {
		validQueries := []string{
			// Basic INSERT FIRST
			`INSERT FIRST
			   WHEN amount > 1000 THEN INTO large_orders (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount)
			   ELSE INTO small_orders (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT FIRST with VALUES
			`INSERT FIRST
			   WHEN amount > 1000 THEN INTO large_orders (id, amount) VALUES (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount) VALUES (id, amount)
			   ELSE INTO small_orders (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT FIRST without ELSE
			`INSERT FIRST
			   WHEN amount > 0 THEN INTO positive_amounts (id)
			 SELECT id, amount FROM source`,
			// INSERT FIRST with multiple WHEN branches
			`INSERT FIRST
			   WHEN status = 'A' THEN INTO t1
			   WHEN status = 'B' THEN INTO t2
			   WHEN status = 'C' THEN INTO t3
			   ELSE INTO t_other
			 SELECT id, status FROM source`,
			// INSERT OVERWRITE FIRST
			`INSERT OVERWRITE FIRST
			   WHEN x > 0 THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("valid INSERT OVERWRITE queries", func(t *testing.T) {
		validQueries := []string{
			// Basic INSERT OVERWRITE INTO
			`INSERT OVERWRITE INTO t1 SELECT * FROM source`,
			// INSERT OVERWRITE INTO with column list
			`INSERT OVERWRITE INTO t1 (id, name) SELECT id, name FROM source`,
			// INSERT OVERWRITE INTO with VALUES
			`INSERT OVERWRITE INTO t1 (id) VALUES (1)`,
			// INSERT OVERWRITE INTO with fully qualified table
			`INSERT OVERWRITE INTO db.sch.t1 SELECT * FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT ALL queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT ALL without any INTO clause
			{
				sql:     `INSERT ALL SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// INSERT ALL without trailing SELECT
			{
				sql:     `INSERT ALL INTO t1 (id) VALUES (1)`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
			// Conditional INSERT ALL without WHEN branches
			{
				sql: `INSERT ALL
				   ELSE INTO t1
				 SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one WHEN branch",
			},
			// INSERT ALL with WHEN but no THEN INTO
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN
				 SELECT id, x FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			// INSERT OVERWRITE ALL without any INTO
			{
				sql:     `INSERT OVERWRITE ALL SELECT * FROM source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// INSERT OVERWRITE ALL without trailing SELECT
			{
				sql:     `INSERT OVERWRITE ALL INTO t1 (id) VALUES (1)`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT FIRST queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT FIRST without WHEN branches
			{
				sql:     `INSERT FIRST INTO t1 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// INSERT FIRST with ELSE only (no WHEN)
			{
				sql: `INSERT FIRST
				   ELSE INTO t1
				 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// INSERT FIRST without trailing SELECT
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN INTO t1`,
				wantMsg: "INSERT FIRST requires a source SELECT",
			},
			// INSERT OVERWRITE FIRST without WHEN branches
			{
				sql:     `INSERT OVERWRITE FIRST INTO t1 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT OVERWRITE queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT OVERWRITE without INTO
			{
				sql:     `INSERT OVERWRITE t1 SELECT * FROM source`,
				wantMsg: "INSERT OVERWRITE requires INTO",
			},
			// INSERT OVERWRITE INTO without source
			{
				sql:     `INSERT OVERWRITE INTO t1`,
				wantMsg: "INSERT OVERWRITE INTO requires a source SELECT or VALUES",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

// ── ALTER TABLE … ADD/DROP SEARCH OPTIMIZATION ──────────────────────────────

func TestValidateSnowflakePatterns_AlterTableSearchOptimization(t *testing.T) {
	t.Run("valid ALTER TABLE SEARCH OPTIMIZATION", func(t *testing.T) {
		validQueries := []string{
			// Bare ADD SEARCH OPTIMIZATION (no ON clause)
			"ALTER TABLE my_table ADD SEARCH OPTIMIZATION",
			"ALTER TABLE db.schema.my_table ADD SEARCH OPTIMIZATION",
			// IF EXISTS form
			"ALTER TABLE IF EXISTS my_table ADD SEARCH OPTIMIZATION",
			"ALTER TABLE IF EXISTS my_table ADD SEARCH OPTIMIZATION ON EQUALITY(c1)",
			// Bare DROP SEARCH OPTIMIZATION
			"ALTER TABLE my_table DROP SEARCH OPTIMIZATION",
			"ALTER TABLE db.schema.my_table DROP SEARCH OPTIMIZATION",
			// ON clause with EQUALITY
			"ALTER TABLE my_table ADD SEARCH OPTIMIZATION ON EQUALITY(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(col1, col2)",
			// ON clause with SUBSTRING
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON SUBSTRING(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON SUBSTRING(col1, col2)",
			// ON clause with GEO
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON GEO(geo_col)",
			// ON clause with FULL_TEXT
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1, col2)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1, LANGUAGE => 'en')",
			// Multiple expression types
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2), GEO(c3), FULL_TEXT(c4)",
			// DROP with ON clause
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON EQUALITY(col1)",
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2)",
			// Case insensitive
			"ALTER TABLE t ADD search optimization ON equality(c1)",
			"alter table t add search optimization on substring(c1), geo(c2)",
			// With trailing semicolons / whitespace
			"ALTER TABLE t ADD SEARCH OPTIMIZATION;",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1);",
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON SUBSTRING(c1), GEO(c2);  ",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER TABLE SEARCH OPTIMIZATION", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Unknown expression type
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON FUZZY(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// Another unknown type
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON HASH(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// Empty ON clause
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON",
				wantMsg: "SEARCH OPTIMIZATION ON requires at least one expression",
			},
			// Mixed valid and invalid
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), FUZZY(c2)",
				wantMsg: "Unknown search optimization type",
			},
			// DROP with unknown expression type
			{
				sql:     "ALTER TABLE t DROP SEARCH OPTIMIZATION ON UNKNOWN(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// IF EXISTS with unknown expression type
			{
				sql:     "ALTER TABLE IF EXISTS t ADD SEARCH OPTIMIZATION ON FUZZY(col1)",
				wantMsg: "Unknown search optimization type",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateBareColumnRefs_MatchRecognizeSuppression(t *testing.T) {
	// MATCH_RECOGNIZE queries should not produce false-positive column warnings
	// because pattern variables (A, B) are local aliases, not table references.
	validQueries := []string{
		// Pattern variables in MEASURES should not be flagged
		`SELECT * FROM DB.SCH.EMPLOYEES MATCH_RECOGNIZE (ORDER BY ID MEASURES FIRST(A.SALARY) AS start_sal PATTERN (A B+) DEFINE A AS SALARY < 50000, B AS SALARY > A.SALARY) AS mr`,
		// Result alias mr.start_sal should not be flagged
		`SELECT mr.start_sal FROM DB.SCH.EMPLOYEES MATCH_RECOGNIZE (ORDER BY ID MEASURES FIRST(A.SALARY) AS start_sal PATTERN (A B+) DEFINE A AS SALARY < 50000, B AS SALARY > A.SALARY) AS mr`,
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for MATCH_RECOGNIZE query %q, got %d: %v", sql[:60], len(warnings), warnings)
			}
		})
	}
}

// ── ALTER DYNAMIC TABLE Tests ────────────────────────────────────────────────

func TestValidateSnowflakePatterns_AlterDynamicTable(t *testing.T) {
	t.Run("valid ALTER DYNAMIC TABLE", func(t *testing.T) {
		validQueries := []string{
			// REFRESH
			"ALTER DYNAMIC TABLE my_dt REFRESH",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt REFRESH",
			"ALTER DYNAMIC TABLE db.schema.my_dt REFRESH",
			// SUSPEND
			"ALTER DYNAMIC TABLE my_dt SUSPEND",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SUSPEND",
			// RESUME
			"ALTER DYNAMIC TABLE my_dt RESUME",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt RESUME",
			// SET TARGET_LAG
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '5 minutes'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 hour'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '2 hours'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '30 seconds'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 day'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '7 days'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = DOWNSTREAM",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SET TARGET_LAG = DOWNSTREAM",
			// SET WAREHOUSE
			"ALTER DYNAMIC TABLE my_dt SET WAREHOUSE = my_wh",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SET WAREHOUSE = my_wh",
			// SET COMMENT (string literal stripped by preprocessing — exercises non-TARGET_LAG SET path)
			"ALTER DYNAMIC TABLE my_dt SET COMMENT = 'hello world'",
			// UNSET
			"ALTER DYNAMIC TABLE my_dt UNSET DATA_RETENTION_TIME_IN_DAYS",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt UNSET DATA_RETENTION_TIME_IN_DAYS",
			// SWAP WITH
			"ALTER DYNAMIC TABLE my_dt SWAP WITH other_dt",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SWAP WITH db.schema.other_dt",
			// RENAME TO
			"ALTER DYNAMIC TABLE my_dt RENAME TO new_name",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt RENAME TO db.schema.new_name",
			// Case insensitive
			"alter dynamic table my_dt refresh",
			"ALTER DYNAMIC TABLE my_dt set target_lag = downstream",
			"Alter Dynamic Table my_dt Suspend",
			// Table name collides with a sub-command keyword — must not false-positive
			"ALTER DYNAMIC TABLE suspend SET TARGET_LAG = DOWNSTREAM",
			"ALTER DYNAMIC TABLE resume SET WAREHOUSE = my_wh",
			"ALTER DYNAMIC TABLE refresh SUSPEND",
			"ALTER DYNAMIC TABLE set RESUME",
			// With trailing semicolons / whitespace
			"ALTER DYNAMIC TABLE my_dt REFRESH;",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute';  ",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER DYNAMIC TABLE", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing table name
			{
				sql:     "ALTER DYNAMIC TABLE",
				wantMsg: "ALTER DYNAMIC TABLE requires a table name",
			},
			// Unknown sub-command
			{
				sql:     "ALTER DYNAMIC TABLE my_dt TRUNCATE",
				wantMsg: "Unknown ALTER DYNAMIC TABLE sub-command",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt DROP",
				wantMsg: "Unknown ALTER DYNAMIC TABLE sub-command",
			},
			// SWAP WITH without target name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			// RENAME TO without new name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt RENAME TO",
				wantMsg: "RENAME TO requires a new table name",
			},
			// Multiple sub-commands in one statement
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SUSPEND RESUME",
				wantMsg: "ALTER DYNAMIC TABLE supports only one sub-command per statement",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt REFRESH SUSPEND",
				wantMsg: "ALTER DYNAMIC TABLE supports only one sub-command per statement",
			},
			// Invalid TARGET_LAG value
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 'invalid'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 fortnight'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 42",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Zero-duration TARGET_LAG (Snowflake requires positive integer)
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '0 seconds'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '0 minutes'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Bare SET / UNSET without a property name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET",
				wantMsg: "SET requires at least one property",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt UNSET",
				wantMsg: "UNSET requires at least one property name",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

// ── ALTER TABLE … SWAP WITH Tests ────────────────────────────────────────────

func TestValidateSnowflakePatterns_AlterTableSwapWith(t *testing.T) {
	t.Run("valid ALTER TABLE SWAP WITH", func(t *testing.T) {
		validQueries := []string{
			// Basic
			"ALTER TABLE orders SWAP WITH orders_backup",
			"ALTER TABLE t1 SWAP WITH t2",
			// IF EXISTS
			"ALTER TABLE IF EXISTS t1 SWAP WITH t2",
			"ALTER TABLE IF EXISTS orders SWAP WITH orders_backup",
			// Three-part names
			"ALTER TABLE db1.schema1.t1 SWAP WITH db1.schema1.t2",
			"ALTER TABLE mydb.public.orders SWAP WITH mydb.public.orders_backup",
			// Two-part names
			"ALTER TABLE schema1.t1 SWAP WITH schema1.t2",
			// Mixed part counts
			"ALTER TABLE db.schema.t1 SWAP WITH t2",
			"ALTER TABLE t1 SWAP WITH db.schema.t2",
			// IF EXISTS with multi-part names
			"ALTER TABLE IF EXISTS db.schema.t1 SWAP WITH db.schema.t2",
			// Quoted identifiers
			`ALTER TABLE "MY_TABLE" SWAP WITH "OTHER_TABLE"`,
			`ALTER TABLE "my table" SWAP WITH "other table"`,
			`ALTER TABLE db."SCHEMA"."TABLE" SWAP WITH db."SCHEMA"."OTHER"`,
			// Case insensitive
			"alter table t1 swap with t2",
			"Alter Table T1 Swap With T2",
			"ALTER TABLE t1 swap WITH t2",
			// With trailing semicolons / whitespace
			"ALTER TABLE t1 SWAP WITH t2;",
			"ALTER TABLE t1 SWAP WITH t2;  ",
			"ALTER TABLE t1 SWAP WITH t2 ;",
			// Table name collides with a keyword
			"ALTER TABLE swap SWAP WITH other_t",
			`ALTER TABLE "select" SWAP WITH "from"`,
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER TABLE SWAP WITH", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing target table name
			{
				sql:     "ALTER TABLE orders SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			{
				sql:     "ALTER TABLE IF EXISTS orders SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			{
				sql:     "ALTER TABLE db.schema.t1 SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			// Same table (no-op)
			{
				sql:     "ALTER TABLE orders SWAP WITH orders",
				wantMsg: "SWAP WITH the same table",
			},
			{
				sql:     "ALTER TABLE t1 SWAP WITH t1",
				wantMsg: "SWAP WITH the same table",
			},
			// Extra clause after target
			{
				sql:     "ALTER TABLE orders SWAP WITH backup CLUSTER BY (id)",
				wantMsg: "Unexpected clause after SWAP WITH target table",
			},
			{
				sql:     "ALTER TABLE orders SWAP WITH backup SET DATA_RETENTION_TIME_IN_DAYS = 1",
				wantMsg: "Unexpected clause after SWAP WITH target table",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

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
