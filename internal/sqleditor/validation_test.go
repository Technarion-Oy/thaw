package sqleditor

import (
	"strings"
	"testing"
)

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
		"CREATE OR REPLACE DATABASE db CLONE src",
		"CREATE DATABASE db CLONE src AT (TIMESTAMP => 1) IGNORE TABLES WITH INSUFFICIENT DATA RETENTION IGNORE HYBRID TABLES",
		"CREATE SCHEMA s WITH MANAGED ACCESS COMMENT = 'sch'",
		"CREATE DATABASE db FROM SHARE provider_acct.my_share",
		"CREATE DATABASE db EXTERNAL_VOLUME = vol CATALOG = cat STORAGE_SERIALIZATION_POLICY = OPTIMIZED",
		"CREATE DATABASE db CATALOG_SYNC = 'open' CATALOG_SYNC_NAMESPACE_MODE = FLATTEN OBJECT_VISIBILITY = PRIVILEGED",
		"CREATE DATABASE db REPLACE_INVALID_CHARACTERS = TRUE MAX_DATA_EXTENSION_TIME_IN_DAYS = 14 WITH TAG (t = 'v') CONTACT (support = c)",
		// Snowflake Views
		"CREATE VIEW v AS SELECT 1 FROM t",
		"CREATE OR REPLACE SECURE VIEW v AS SELECT 1 FROM t",
		"CREATE MATERIALIZED VIEW mv AS SELECT 1 FROM t",
		"CREATE VIEW IF NOT EXISTS db.sch.v AS SELECT 1 FROM t",
		"CREATE VIEW v (a, b) AS SELECT 1, 2 FROM t",
		"CREATE VIEW v COPY GRANTS COMMENT = 'my view' AS SELECT 1 FROM t",
		"CREATE VIEW v CHANGE_TRACKING = TRUE AS SELECT 1 FROM t",
		"CREATE VIEW v CLUSTER BY (a, b) AS SELECT a, b FROM t",
		"CREATE VIEW v WITH ROW ACCESS POLICY p ON (a) AS SELECT a FROM t",
		"CREATE VIEW v WITH AGGREGATION POLICY p ENTITY KEY (a) AS SELECT a FROM t",
		"CREATE VIEW v WITH TAG (cost_center = 'sales') AS SELECT 1 FROM t",
		"CREATE VIEW v WITH CONTACT (support = c) AS SELECT 1 FROM t",
		"CREATE LOCAL TEMP RECURSIVE VIEW v (a) AS SELECT 1 FROM t",
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
		"CREATE OR REPLACE SEQUENCE IF NOT EXISTS s START = 100 INCREMENT BY -2 ORDER COMMENT = 'seq'",
		"CREATE SEQUENCE s WITH START 5 INCREMENT 1 NOORDER",
		"CREATE SEQUENCE s START -10 INCREMENT = 3",
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
		// CREATE TABLE property tail — one per supported property
		"CREATE TABLE t (id INT) MAX_DATA_EXTENSION_TIME_IN_DAYS = 14 DEFAULT_DDL_COLLATION = 'en_US'",
		"CREATE TABLE t (id INT) COPY GRANTS COPY TAGS",
		"CREATE TABLE t (id INT) ERROR_LOGGING = TRUE ROW_TIMESTAMP = FALSE CHANGE_TRACKING = TRUE",
		"CREATE TABLE t (id INT) WITH AGGREGATION POLICY ap ENTITY KEY (id)",
		"CREATE TABLE t (id INT) WITH JOIN POLICY jp ALLOWED JOIN KEYS (id)",
		"CREATE TABLE t (id INT) STORAGE LIFECYCLE POLICY slp ON (id)",
		"CREATE TABLE t (id INT) WITH TAG (cost = 'x') WITH CONTACT (support = c)",
		"CREATE TABLE t (id INT) ROW ACCESS POLICY rap ON (id)",
		// Integrations
		"CREATE STORAGE INTEGRATION my_storage_int TYPE=EXTERNAL_STAGE STORAGE_PROVIDER='S3' ENABLED=TRUE STORAGE_AWS_ROLE_ARN='arn:aws:iam::123456789012:role/my_role' STORAGE_ALLOWED_LOCATIONS=('s3://my-bucket/')",
		"CREATE STAGE my_s3_stage URL='s3://bucket/' STORAGE_INTEGRATION=s3_int DIRECTORY=(ENABLE=TRUE)",
		"CREATE WAREHOUSE my_wh WAREHOUSE_SIZE='X-LARGE' WAREHOUSE_TYPE='STANDARD' AUTO_SUSPEND=300",
		"CREATE OR REPLACE WAREHOUSE COMPUTE_WH WITH WAREHOUSE_TYPE='STANDARD' RESOURCE_CONSTRAINT='STANDARD_GEN_1' WAREHOUSE_SIZE='X-Small' MAX_CLUSTER_COUNT=1 MIN_CLUSTER_COUNT=1 SCALING_POLICY=STANDARD AUTO_SUSPEND=600 AUTO_RESUME=TRUE INITIALLY_SUSPENDED=TRUE ENABLE_QUERY_ACCELERATION=FALSE QUERY_ACCELERATION_MAX_SCALE_FACTOR=8 MAX_CONCURRENCY_LEVEL=8 STATEMENT_QUEUED_TIMEOUT_IN_SECONDS=0 STATEMENT_TIMEOUT_IN_SECONDS=172800",
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
		// Grants on ROLE objects — OWNERSHIP is valid (ownership transfer)
		"GRANT OWNERSHIP ON ROLE my_role TO ROLE other_role",
		"GRANT OWNERSHIP ON ROLE my_role TO ROLE other_role WITH GRANT OPTION",
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
		{"DB unknown property", "CREATE DATABASE db BOGUS_PROP = 1", "Unexpected syntax"},
		{"DB enum bad value", "CREATE DATABASE db STORAGE_SERIALIZATION_POLICY = BOGUS", "Unexpected syntax"},
		{"DB bool prop bad value", "CREATE DATABASE db ENABLE_DATA_COMPACTION = MAYBE", "Unexpected syntax"},
		{"Invalid View", "CREATE VIEW v SELECT 1", "Unexpected syntax"}, // Missing AS
		{"Invalid Mat View", "CREATE MATERIALIZED VIEW mv SELECT 1", "Unexpected syntax"},
		{"View bad clause", "CREATE VIEW v BOGUS_PROP = 1 AS SELECT 1", "Unexpected syntax"},
		{"View CHANGE_TRACKING bad value", "CREATE VIEW v CHANGE_TRACKING = MAYBE AS SELECT 1", "Unexpected syntax"},
		{"View CLUSTER BY no parens", "CREATE VIEW v CLUSTER BY a AS SELECT a FROM t", "Unexpected syntax"},
		{"View CONTACT without WITH", "CREATE VIEW v CONTACT (x = c) AS SELECT 1", "Unexpected syntax"},
		{"Invalid Dynamic Table", "CREATE DYNAMIC TABLE dt AS SELECT 1", "Unexpected syntax"}, // Missing TARGET_LAG / WAREHOUSE
		{"Invalid Drop DB", "DROP DATABASE my_db CASCADE RESTRICT", "Unexpected syntax"},      // Conflicting modifiers
		{"Invalid Sequence", "CREATE SEQUENCE my_seq START WITH 'abc'", "Unexpected syntax"},
		{"Sequence ORDER+NOORDER", "CREATE SEQUENCE s ORDER NOORDER", "Unexpected syntax"},
		{"Sequence unknown clause", "CREATE SEQUENCE s BOGUS = 1", "Unexpected syntax"},
		{"Sequence START no value", "CREATE SEQUENCE s START WITH", "Unexpected syntax"},
		{"Invalid Table", "CREATE TRANSIENT OR REPLACE TABLE foo (id INT)", "Unexpected syntax"}, // Wrong modifier order
		{"Table Replace IF NOT EXISTS", "CREATE OR REPLACE TABLE foo IF NOT EXISTS (id INT)", "Conflict between OR REPLACE and IF NOT EXISTS"},
		{"Table CLUSTER BY no parens", "CREATE TABLE foo (id INT) CLUSTER BY id", "Unexpected syntax"},
		{"Table Retention invalid", "CREATE TABLE foo (id INT) DATA_RETENTION_TIME_IN_DAYS = 'abc'", "Unexpected syntax"},
		{"Table unknown property", "CREATE TABLE foo (id INT) BOGUS_PROP = 1", "Unexpected syntax"},
		{"Table bool prop bad value", "CREATE TABLE foo (id INT) CHANGE_TRACKING = MAYBE", "Unexpected syntax"},
		{"Table CONTACT without WITH-only on contact", "CREATE TABLE foo (id INT) CONTACT (c = x)", "Unexpected syntax"},

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
		// Task property validation removed — tasks accept arbitrary session parameters.
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
		{"Grant select on role", "GRANT SELECT ON ROLE my_role TO ROLE other_role", "not valid for object type ROLE"},
		{"Grant usage on role", "GRANT USAGE ON ROLE my_role TO ROLE other_role", "Use 'GRANT ROLE <name> TO ROLE/USER' to assign a role"},
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
		{"Revoke select on role", "REVOKE SELECT ON ROLE my_role FROM ROLE other_role", "not valid for object type ROLE"},
		{"Revoke usage on role", "REVOKE USAGE ON ROLE my_role FROM ROLE other_role", "Use 'REVOKE ROLE <name> FROM ROLE/USER' to revoke a role"},

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

		// CREATE TASK — SCHEDULE with USING CRON must not flag CRON as a table (Issue #306)
		"CREATE OR REPLACE TASK LINEAGE_SOURCE_DB.RAW_DATA.TASK_1\n\tWAREHOUSE=COMPUTE_WH\n\tSCHEDULE='USING CRON 0 0 * * * UTC'\n\tAS SELECT SYSTEM$WAIT(5)",
		"CREATE OR REPLACE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO LIVE_TABLE SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
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
