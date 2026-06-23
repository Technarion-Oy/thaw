package sqlgrammar

import "testing"

func TestParseCreateStorageIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStorageIntegration,
		`CREATE STORAGE INTEGRATION my_int TYPE = EXTERNAL_STAGE STORAGE_PROVIDER = 'S3' ENABLED = TRUE STORAGE_ALLOWED_LOCATIONS = ('s3://bucket/path/')`,
		`CREATE OR REPLACE STORAGE INTEGRATION IF NOT EXISTS s TYPE = EXTERNAL_STAGE STORAGE_PROVIDER = 'AZURE' AZURE_TENANT_ID = 'abc' ENABLED = FALSE STORAGE_ALLOWED_LOCATIONS = ('azure://x/') STORAGE_BLOCKED_LOCATIONS = ('azure://y/') COMMENT = 'c'`,
		`CREATE STORAGE INTEGRATION g TYPE = EXTERNAL_STAGE STORAGE_PROVIDER = 'GCS' ENABLED = TRUE STORAGE_ALLOWED_LOCATIONS = ('gcs://b/p/')`,
	)
	assertInvalid(t, (*Validator).ParseCreateStorageIntegration,
		`CREATE STORAGE INTEGRATION`,
		`CREATE STORAGE my_int TYPE = EXTERNAL_STAGE`,
		`STORAGE INTEGRATION my_int TYPE = EXTERNAL_STAGE`,
	)
}

func TestParseCreateStorageLifecyclePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStorageLifecyclePolicy,
		`CREATE STORAGE LIFECYCLE POLICY p AS (age INT) RETURNS BOOLEAN -> age > 30`,
		`CREATE OR REPLACE STORAGE LIFECYCLE POLICY IF NOT EXISTS p AS (a INT, b VARCHAR) RETURNS BOOLEAN -> a > 1 ARCHIVE_TIER = COLD ARCHIVE_FOR_DAYS = 90 COMMENT = 'x'`,
		`CREATE STORAGE LIFECYCLE POLICY p AS (x INT) RETURNS BOOLEAN -> x = 5 ARCHIVE_TIER = COOL`,
	)
	assertInvalid(t, (*Validator).ParseCreateStorageLifecyclePolicy,
		`CREATE STORAGE LIFECYCLE POLICY p`,
		`CREATE STORAGE LIFECYCLE POLICY p AS (a INT) RETURNS BOOLEAN age > 30`,
		`CREATE STORAGE LIFECYCLE POLICY p AS (a INT) BOOLEAN -> a > 1`,
	)
}

func TestParseCreateStream(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStream,
		`CREATE STREAM s ON TABLE t`,
		`CREATE OR REPLACE STREAM IF NOT EXISTS s COPY GRANTS ON EXTERNAL TABLE t INSERT_ONLY = TRUE COMMENT = 'c'`,
		`CREATE STREAM s ON VIEW v AT (TIMESTAMP => '2020-01-01') APPEND_ONLY = FALSE SHOW_INITIAL_ROWS = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateStream,
		`CREATE STREAM s`,
		`CREATE STREAM ON TABLE t`,
		`CREATE STREAM s TABLE t`,
	)
}

func TestParseCreateStreamlit(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStreamlit,
		`CREATE STREAMLIT app ROOT_LOCATION = '@stage/dir' MAIN_FILE = 'app.py'`,
		`CREATE OR REPLACE STREAMLIT IF NOT EXISTS app FROM '@stage' MAIN_FILE = 'main.py' QUERY_WAREHOUSE = wh TITLE = 't' COMMENT = 'c'`,
		`CREATE STREAMLIT app MAIN_FILE = 'm.py' IMPORTS = ('@s/a.py', '@s/b.py') EXTERNAL_ACCESS_INTEGRATIONS = (i1, i2)`,
	)
	assertInvalid(t, (*Validator).ParseCreateStreamlit,
		`CREATE STREAMLIT`,
		`CREATE app MAIN_FILE = 'm.py'`,
		`STREAMLIT app MAIN_FILE = 'm.py'`,
	)
}

func TestParseCreateTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateTable,
		`CREATE TABLE t (id INT, name VARCHAR(20))`,
		`CREATE OR REPLACE TRANSIENT TABLE IF NOT EXISTS t (id INT NOT NULL, v VARIANT) CLUSTER BY (id) DATA_RETENTION_TIME_IN_DAYS = 5 COMMENT = 'c'`,
		`CREATE TABLE t AS SELECT * FROM src`,
		`CREATE TABLE t LIKE src`,
		`CREATE TABLE t CLONE src AT (OFFSET => -60)`,
	)
	assertInvalid(t, (*Validator).ParseCreateTable,
		`CREATE TABLE`,
		`CREATE TABLE t (id INT`,
		`TABLE t (id INT)`,
	)
}

func TestParseCreateAlterTableConstraint(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAlterTableConstraint,
		`CREATE TABLE t (id INT PRIMARY KEY)`,
		`CREATE TABLE t (a INT, b INT, CONSTRAINT fk FOREIGN KEY (b) REFERENCES o (id))`,
		`ALTER TABLE t ADD COLUMN c INT NOT NULL UNIQUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateAlterTableConstraint,
		`CREATE TABLE t`,
		`ALTER TABLE t ADD c INT`,
		`CREATE t (id INT)`,
	)
}

func TestParseCreateTag(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateTag,
		`CREATE TAG t`,
		`CREATE OR REPLACE TAG IF NOT EXISTS t ALLOWED_VALUES 'a', 'b', 'c' COMMENT = 'x'`,
		`CREATE TAG t PROPAGATE = ON_DEPENDENCY ON_CONFLICT = ALLOWED_VALUES_SEQUENCE`,
	)
	assertInvalid(t, (*Validator).ParseCreateTag,
		`CREATE TAG`,
		`CREATE t ALLOWED_VALUES 'a'`,
		`CREATE TAG t PROPAGATE ON_DEPENDENCY`,
	)
}

func TestParseCreateTask(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateTask,
		`CREATE TASK t WAREHOUSE = 'wh' AS SELECT 1`,
		`CREATE OR REPLACE TASK IF NOT EXISTS t SCHEDULE = '5 MINUTES' COMMENT = 'c' AFTER 'parent' WHEN 1 = 1 AS INSERT INTO log VALUES (1)`,
		`CREATE TASK t USER_TASK_TIMEOUT_MS = 1000 EXECUTE AS USER bob AS CALL my_proc()`,
	)
	assertInvalid(t, (*Validator).ParseCreateTask,
		`CREATE TASK t`,
		`CREATE TASK t AS`,
		`CREATE TASK AS SELECT 1`,
	)
}

func TestParseCreateType(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateType,
		`CREATE TYPE t AS GEOMETRY`,
		`CREATE OR REPLACE TYPE IF NOT EXISTS t AS NUMBER COMMENT = 'c'`,
		`CREATE TYPE t AS VARCHAR (100)`,
	)
	assertInvalid(t, (*Validator).ParseCreateType,
		`CREATE TYPE t`,
		`CREATE TYPE AS GEOMETRY`,
		`CREATE TYPE t GEOMETRY`,
	)
}

func TestParseCreateUser(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateUser,
		`CREATE USER u`,
		`CREATE OR REPLACE USER IF NOT EXISTS u PASSWORD = 'abc' DEFAULT_ROLE = sysadmin MUST_CHANGE_PASSWORD = TRUE`,
		`CREATE USER u LOGIN_NAME = 'u1' TAG (env = 'prod')`,
	)
	assertInvalid(t, (*Validator).ParseCreateUser,
		`CREATE USER`,
		`CREATE u PASSWORD = 'abc'`,
		`USER u`,
	)
}

func TestParseCreateOrAlterVersionedSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrAlterVersionedSchema,
		`CREATE OR ALTER VERSIONED SCHEMA s`,
		`CREATE OR ALTER VERSIONED SCHEMA s WITH MANAGED ACCESS DATA_RETENTION_TIME_IN_DAYS = 3 COMMENT = 'c'`,
		`CREATE OR ALTER VERSIONED SCHEMA s DEFAULT_DDL_COLLATION = 'en'`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrAlterVersionedSchema,
		`CREATE VERSIONED SCHEMA s`,
		`CREATE OR ALTER SCHEMA s`,
		`CREATE OR ALTER VERSIONED SCHEMA`,
	)
}

func TestParseCreateView(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateView,
		`CREATE VIEW v AS SELECT 1`,
		`CREATE OR REPLACE SECURE VIEW IF NOT EXISTS v (a, b) COMMENT = 'c' AS SELECT a, b FROM t`,
		`CREATE RECURSIVE VIEW v COPY GRANTS AS SELECT * FROM t`,
	)
	assertInvalid(t, (*Validator).ParseCreateView,
		`CREATE VIEW v`,
		`CREATE VIEW AS SELECT 1`,
		`CREATE VIEW v SELECT 1`,
	)
}

func TestParseCreateWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateWarehouse,
		`CREATE WAREHOUSE wh`,
		`CREATE OR REPLACE WAREHOUSE IF NOT EXISTS wh WITH WAREHOUSE_SIZE = MEDIUM AUTO_SUSPEND = 60 AUTO_RESUME = TRUE COMMENT = 'c'`,
		`CREATE WAREHOUSE wh MAX_CLUSTER_COUNT = 3 MIN_CLUSTER_COUNT = 1 SCALING_POLICY = STANDARD`,
	)
	assertInvalid(t, (*Validator).ParseCreateWarehouse,
		`CREATE WAREHOUSE`,
		`CREATE wh WAREHOUSE_SIZE = MEDIUM`,
		`WAREHOUSE wh`,
	)
}

func TestParseCreateApplicationService(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateApplicationService,
		`CREATE APPLICATION SERVICE svc FROM ARTIFACT REPOSITORY repo PACKAGE pkg`,
		`CREATE APPLICATION SERVICE IF NOT EXISTS svc FROM ARTIFACT REPOSITORY repo PACKAGE pkg VERSION v1 QUERY_WAREHOUSE = wh AUTO_RESUME = TRUE COMMENT = 'c'`,
		`CREATE APPLICATION SERVICE svc FROM ARTIFACT REPOSITORY repo PACKAGE pkg EXTERNAL_ACCESS_INTEGRATIONS = (i1, i2) AUTO_SUSPEND_SECS = 300`,
	)
	assertInvalid(t, (*Validator).ParseCreateApplicationService,
		`CREATE APPLICATION SERVICE svc`,
		`CREATE APPLICATION SERVICE svc FROM ARTIFACT REPOSITORY repo`,
		`CREATE APPLICATION SERVICE FROM ARTIFACT REPOSITORY repo PACKAGE pkg`,
	)
}

func TestParseCreateArtifactRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateArtifactRepository,
		`CREATE ARTIFACT REPOSITORY r TYPE = PYPI`,
		`CREATE OR REPLACE ARTIFACT REPOSITORY IF NOT EXISTS r TYPE = APPLICATION API_INTEGRATION = 'i' COMMENT = 'c'`,
		`CREATE ARTIFACT REPOSITORY r TYPE = PYPI TAG (env = 'prod')`,
	)
	assertInvalid(t, (*Validator).ParseCreateArtifactRepository,
		`CREATE ARTIFACT REPOSITORY r`,
		`CREATE ARTIFACT r TYPE = PYPI`,
		`CREATE ARTIFACT REPOSITORY TYPE = PYPI`,
	)
}

func TestParseCreateCatalogIntegrationDeltaSharing(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationDeltaSharing,
		`CREATE CATALOG INTEGRATION c CATALOG_SOURCE = DELTA_SHARING TABLE_FORMAT = DELTA REST_CONFIG = (CATALOG_URI = 'u') REST_AUTHENTICATION = (TYPE = BEARER BEARER_TOKEN = 't') ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS c CATALOG_SOURCE = DELTA_SHARING TABLE_FORMAT = DELTA REST_CONFIG = (CATALOG_URI = 'u' CATALOG_NAME = 'shares/s' ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS) REST_AUTHENTICATION = (TYPE = OIDC OIDC_AUDIENCE = 'a') ENABLED = FALSE COMMENT = 'x'`,
		`CREATE CATALOG INTEGRATION c CATALOG_SOURCE = DELTA_SHARING TABLE_FORMAT = DELTA REST_CONFIG = (x = 'y') REST_AUTHENTICATION = (TYPE = OAUTH) ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationDeltaSharing,
		`CREATE CATALOG INTEGRATION c`,
		`CREATE CATALOG c CATALOG_SOURCE = DELTA_SHARING`,
		`CREATE INTEGRATION c CATALOG_SOURCE = DELTA_SHARING`,
	)
}

func TestParseCreateCatalogIntegrationSnowflakePostgres(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationSnowflakePostgres,
		`CREATE CATALOG INTEGRATION c CATALOG_SOURCE = SNOWFLAKE_POSTGRES TABLE_FORMAT = ICEBERG REST_CONFIG = (POSTGRES_INSTANCE = 'i') ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS c CATALOG_SOURCE = SNOWFLAKE_POSTGRES TABLE_FORMAT = ICEBERG CATALOG_NAMESPACE = 'ns' REST_CONFIG = (POSTGRES_INSTANCE = 'i' ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS) ENABLED = FALSE COMMENT = 'c'`,
		`CREATE CATALOG INTEGRATION c CATALOG_SOURCE = SNOWFLAKE_POSTGRES TABLE_FORMAT = ICEBERG REST_CONFIG = (POSTGRES_INSTANCE = 'i' CATALOG_NAME = 'db') ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationSnowflakePostgres,
		`CREATE CATALOG INTEGRATION c`,
		`CREATE CATALOG c CATALOG_SOURCE = SNOWFLAKE_POSTGRES`,
		`CREATE CATALOG INTEGRATION`,
	)
}

func TestParseCreateEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateEventRoutingTable,
		`CREATE EVENT ROUTING TABLE t WITH RULES r1 = (REGION_GROUP = g)`,
		`CREATE EVENT ROUTING TABLE t WITH RULES r1 = (REGION_GROUP = g, REGIONS = ('a', 'b'), DESTINATION_ACCOUNT = org.acct)`,
		`CREATE EVENT ROUTING TABLE t WITH RULES r1 = (x = 1) r2 = (y = 2)`,
	)
	assertInvalid(t, (*Validator).ParseCreateEventRoutingTable,
		`CREATE EVENT ROUTING TABLE t`,
		`CREATE EVENT ROUTING TABLE t WITH RULES`,
		`CREATE EVENT TABLE t WITH RULES r1 = (x = 1)`,
	)
}

func TestParseCreateStorageIntegrationPostgresInternal(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStorageIntegrationPostgresInternal,
		`CREATE STORAGE INTEGRATION s TYPE = POSTGRES_INTERNAL_STORAGE POSTGRES_INSTANCE = 'i'`,
		`CREATE OR REPLACE STORAGE INTEGRATION IF NOT EXISTS s TYPE = POSTGRES_INTERNAL_STORAGE POSTGRES_INSTANCE = 'i' ENABLED = TRUE COMMENT = 'c'`,
		`CREATE STORAGE INTEGRATION s TYPE = POSTGRES_INTERNAL_STORAGE POSTGRES_INSTANCE = 'i' ENABLED = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateStorageIntegrationPostgresInternal,
		`CREATE STORAGE INTEGRATION s`,
		`CREATE STORAGE s TYPE = POSTGRES_INTERNAL_STORAGE`,
		`CREATE STORAGE INTEGRATION`,
	)
}
