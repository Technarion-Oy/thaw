package sqlgrammar

import "testing"

func TestParseCreateIcebergTableSnowflakeCatalog(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTableSnowflakeCatalog,
		`CREATE ICEBERG TABLE t (id INT, name STRING) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 'data/'`,
		`CREATE OR REPLACE ICEBERG TABLE db.sch.t (c1 NUMBER) EXTERNAL_VOLUME = 'vol' CHANGE_TRACKING = TRUE`,
		`CREATE TRANSIENT ICEBERG TABLE IF NOT EXISTS t (a INT) CLUSTER BY (a) COMMENT = 'x'`,
		`CREATE ICEBERG TABLE t (a INT) PARTITION BY (a) STORAGE_SERIALIZATION_POLICY = OPTIMIZED`,
		// GET_DDL emits CLUSTER BY before the column list (#776).
		`CREATE ICEBERG TABLE t CLUSTER BY (a) (a INT) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 'data/'`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTableSnowflakeCatalog,
		`CREATE ICEBERG TABLE t`,
		`CREATE ICEBERG t (a INT)`,
		`ICEBERG TABLE t (a INT)`,
	)
}

func TestParseCreateImageRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateImageRepository,
		`CREATE IMAGE REPOSITORY my_repo`,
		`CREATE OR REPLACE IMAGE REPOSITORY IF NOT EXISTS db.sch.repo COMMENT = 'x'`,
		`CREATE IMAGE REPOSITORY r ENCRYPTION = ( TYPE = 'SNOWFLAKE_FULL' )`,
	)
	assertInvalid(t, (*Validator).ParseCreateImageRepository,
		`CREATE IMAGE REPOSITORY`,
		`CREATE REPOSITORY r`,
	)
}

func TestParseCreateIndex(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIndex,
		`CREATE INDEX idx ON tbl (col1)`,
		`CREATE OR REPLACE INDEX IF NOT EXISTS idx ON db.sch.tbl (a, b)`,
		`CREATE INDEX idx ON tbl (a) INCLUDE (b, c)`,
	)
	assertInvalid(t, (*Validator).ParseCreateIndex,
		`CREATE INDEX idx`,
		`CREATE INDEX idx ON tbl`,
	)
}

func TestParseCreateIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIntegration,
		`CREATE API INTEGRATION my_api API_PROVIDER = aws_api_gateway ENABLED = TRUE`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS si TYPE = OAUTH`,
		`CREATE STORAGE INTEGRATION s TYPE = EXTERNAL_STAGE`,
		`CREATE NOTIFICATION INTEGRATION n ENABLED = TRUE COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateIntegration,
		`CREATE INTEGRATION i`,
		`CREATE API INTEGRATION`,
	)
}

func TestParseCreateInteractiveTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateInteractiveTable,
		`CREATE INTERACTIVE TABLE t (a INT) CLUSTER BY (a) AS SELECT 1`,
		// GET_DDL emits CLUSTER BY before the column list (#776).
		`CREATE INTERACTIVE TABLE t CLUSTER BY (a) (a INT) AS SELECT 1`,
		`CREATE OR REPLACE INTERACTIVE TABLE IF NOT EXISTS db.t (a INT, b INT) CLUSTER BY (a, b) WAREHOUSE = wh AS SELECT * FROM x`,
		`CREATE INTERACTIVE TABLE t (a INT) CLUSTER BY (a) TARGET_LAG = '5 minutes' COMMENT = 'c' AS SELECT a FROM y`,
	)
	assertInvalid(t, (*Validator).ParseCreateInteractiveTable,
		`CREATE INTERACTIVE TABLE t (a INT) AS SELECT 1`,
		`CREATE INTERACTIVE TABLE t CLUSTER BY (a) AS SELECT 1`,
	)
}

func TestParseCreateInteractiveWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateInteractiveWarehouse,
		`CREATE INTERACTIVE WAREHOUSE wh`,
		`CREATE OR REPLACE INTERACTIVE WAREHOUSE IF NOT EXISTS wh WAREHOUSE_SIZE = MEDIUM AUTO_RESUME = TRUE`,
		`CREATE INTERACTIVE WAREHOUSE wh TABLES (t1, t2) WITH MAX_CLUSTER_COUNT = 3 COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateInteractiveWarehouse,
		`CREATE INTERACTIVE WAREHOUSE`,
		`CREATE WAREHOUSE wh`,
	)
}

func TestParseCreateJoinPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateJoinPolicy,
		`CREATE JOIN POLICY jp AS () RETURNS JOIN_CONSTRAINT -> ALLOWED`,
		`CREATE OR REPLACE JOIN POLICY IF NOT EXISTS jp AS () RETURNS JOIN_CONSTRAINT -> CASE WHEN TRUE THEN ALLOWED ELSE BLOCKED END COMMENT = 'x'`,
		`CREATE JOIN POLICY db.sch.jp AS () RETURNS JOIN_CONSTRAINT -> BLOCKED`,
	)
	assertInvalid(t, (*Validator).ParseCreateJoinPolicy,
		`CREATE JOIN POLICY jp`,
		`CREATE JOIN POLICY jp AS () RETURNS JOIN_CONSTRAINT`,
	)
}

func TestParseCreateListing(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateListing,
		`CREATE EXTERNAL LISTING l AS 'manifest'`,
		`CREATE EXTERNAL LISTING IF NOT EXISTS l SHARE sh AS 'manifest' PUBLISH = TRUE`,
		`CREATE EXTERNAL LISTING l APPLICATION PACKAGE pkg FROM 'stage/path' REVIEW = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateListing,
		`CREATE EXTERNAL LISTING l`,
		`CREATE LISTING l AS 'manifest'`,
	)
}

func TestParseCreateMaintenancePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateMaintenancePolicy,
		`CREATE MAINTENANCE POLICY mp SCHEDULE = 'USING CRON 0 0 * * * UTC'`,
		`CREATE OR REPLACE MAINTENANCE POLICY IF NOT EXISTS db.mp SCHEDULE = 'USING CRON 0 0 * * * UTC' COMMENT = 'x'`,
		`CREATE MAINTENANCE POLICY mp SCHEDULE = 'spec'`,
	)
	assertInvalid(t, (*Validator).ParseCreateMaintenancePolicy,
		`CREATE MAINTENANCE POLICY mp`,
		`CREATE MAINTENANCE mp SCHEDULE = 'x'`,
	)
}

func TestParseCreateManagedAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateManagedAccount,
		`CREATE MANAGED ACCOUNT acc ADMIN_NAME = 'a' , ADMIN_PASSWORD = 'p' , TYPE = READER`,
		`CREATE MANAGED ACCOUNT acc ADMIN_NAME = 'a' , ADMIN_PASSWORD = 'p' , TYPE = READER , COMMENT = 'x'`,
		`CREATE MANAGED ACCOUNT acc TYPE = READER`,
	)
	assertInvalid(t, (*Validator).ParseCreateManagedAccount,
		`CREATE MANAGED ACCOUNT acc`,
		`CREATE MANAGED ACCOUNT`,
	)
}

func TestParseCreateMaskingPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateMaskingPolicy,
		`CREATE MASKING POLICY mp AS (val STRING) RETURNS STRING -> '***'`,
		`CREATE OR REPLACE MASKING POLICY IF NOT EXISTS db.mp AS (val STRING, role STRING) RETURNS STRING -> CASE WHEN role = 'ADMIN' THEN val ELSE '***' END COMMENT = 'x' EXEMPT_OTHER_POLICIES = TRUE`,
		`CREATE OR ALTER MASKING POLICY mp AS (n NUMBER) RETURNS NUMBER -> 0`,
	)
	assertInvalid(t, (*Validator).ParseCreateMaskingPolicy,
		`CREATE MASKING POLICY mp`,
		`CREATE MASKING POLICY mp AS (val STRING) RETURNS STRING`,
	)
}

func TestParseCreateMaterializedView(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateMaterializedView,
		`CREATE MATERIALIZED VIEW mv AS SELECT 1`,
		`CREATE OR REPLACE SECURE MATERIALIZED VIEW IF NOT EXISTS db.mv (a, b) COMMENT = 'x' AS SELECT a, b FROM t`,
		`CREATE MATERIALIZED VIEW mv CLUSTER BY (a) AS SELECT a FROM t`,
		// GET_DDL emits CLUSTER BY before the column list (#776).
		`CREATE MATERIALIZED VIEW mv CLUSTER BY (a) (a) AS SELECT a FROM t`,
	)
	assertInvalid(t, (*Validator).ParseCreateMaterializedView,
		`CREATE MATERIALIZED VIEW mv`,
		`CREATE VIEW mv AS SELECT 1`,
	)
}

func TestParseCreateMcpServer(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateMcpServer,
		`CREATE MCP SERVER s FROM SPECIFICATION $$ tools: [] $$`,
		`CREATE OR REPLACE MCP SERVER IF NOT EXISTS db.s FROM SPECIFICATION $$spec$$`,
		`CREATE MCP SERVER s FROM SPECIFICATION 'spec'`,
	)
	assertInvalid(t, (*Validator).ParseCreateMcpServer,
		`CREATE MCP SERVER s`,
		`CREATE MCP SERVER s FROM SPECIFICATION`,
	)
}

func TestParseCreateModel(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateModel,
		`CREATE MODEL m FROM MODEL src`,
		`CREATE OR REPLACE MODEL IF NOT EXISTS db.m WITH VERSION v1 FROM MODEL src VERSION v0`,
		`CREATE MODEL m FROM @my_stage/path`,
	)
	assertInvalid(t, (*Validator).ParseCreateModel,
		`CREATE MODEL m`,
		`CREATE MODEL m FROM`,
	)
}

func TestParseCreateModelMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateModelMonitor,
		`CREATE MODEL MONITOR mon WITH MODEL = m VERSION = '1' FUNCTION = 'predict' SOURCE = src WAREHOUSE = wh REFRESH_INTERVAL = '1 hours' AGGREGATION_WINDOW = '1 days' TIMESTAMP_COLUMN = ts`,
		`CREATE OR REPLACE MODEL MONITOR IF NOT EXISTS mon WITH MODEL = m TIMESTAMP_COLUMN = ts ID_COLUMNS = [id] PREDICTION_SCORE_COLUMNS = [score]`,
		`CREATE MODEL MONITOR mon WITH MODEL = m SOURCE = s`,
	)
	assertInvalid(t, (*Validator).ParseCreateModelMonitor,
		`CREATE MODEL MONITOR mon`,
		`CREATE MODEL MONITOR mon WITH`,
	)
}

func TestParseCreateNetworkPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNetworkPolicy,
		`CREATE NETWORK POLICY np`,
		`CREATE OR REPLACE NETWORK POLICY IF NOT EXISTS db.np ALLOWED_IP_LIST = ('1.2.3.4', '5.6.7.8') COMMENT = 'x'`,
		`CREATE NETWORK POLICY np ALLOWED_NETWORK_RULE_LIST = ('r1') BLOCKED_IP_LIST = ()`,
	)
	assertInvalid(t, (*Validator).ParseCreateNetworkPolicy,
		`CREATE NETWORK POLICY`,
		`CREATE POLICY np`,
	)
}
