package sqlgrammar

import "testing"

func TestParseCreateExternalFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalFunction,
		`CREATE EXTERNAL FUNCTION my_udf (x INTEGER) RETURNS VARIANT API_INTEGRATION = api_int AS 'https://host/resource'`,
		`CREATE OR REPLACE SECURE EXTERNAL FUNCTION db.sch.f () RETURNS VARCHAR NOT NULL STRICT IMMUTABLE COMMENT = 'c' API_INTEGRATION = ai MAX_BATCH_ROWS = 100 AS 'https://x'`,
		`CREATE EXTERNAL FUNCTION f (a INT, b VARCHAR) RETURNS VARIANT API_INTEGRATION = ai HEADERS = ('h1' = 'v1') AS 'https://y'`,
	)
	assertInvalid(t, (*Validator).ParseCreateExternalFunction,
		`CREATE EXTERNAL FUNCTION`,
		`CREATE EXTERNAL FUNCTION f (x INT) RETURNS VARIANT API_INTEGRATION = ai`, // missing AS
		`CREATE EXTERNAL FUNCTION f (x INT) API_INTEGRATION = ai AS 'u'`,          // missing RETURNS
	)
}

func TestParseCreateExternalTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalTable,
		`CREATE EXTERNAL TABLE t (c1 VARCHAR AS (value:c1::varchar)) LOCATION = @my_stage FILE_FORMAT = (TYPE = CSV)`,
		`CREATE OR REPLACE EXTERNAL TABLE IF NOT EXISTS db.sch.t (c1 INT AS (value:c1::int)) WITH LOCATION = @stg PARTITION_TYPE = USER_SPECIFIED FILE_FORMAT = (FORMAT_NAME = 'ff') COMMENT = 'x'`,
		`CREATE EXTERNAL TABLE t (c1 INT AS (value:c1)) PARTITION BY (c1) LOCATION = @s AUTO_REFRESH = TRUE FILE_FORMAT = (TYPE = PARQUET) COPY GRANTS`,
	)
	assertInvalid(t, (*Validator).ParseCreateExternalTable,
		`CREATE EXTERNAL TABLE`,
		`CREATE EXTERNAL t (c1 INT) LOCATION = @s`, // missing TABLE
		`EXTERNAL TABLE t (c1 INT) LOCATION = @s`,  // missing CREATE
	)
}

func TestParseCreateExternalVolume(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalVolume,
		`CREATE EXTERNAL VOLUME vol STORAGE_LOCATIONS = ((NAME = 'l1' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b'))`,
		`CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS vol STORAGE_LOCATIONS = ((NAME = 'l1' STORAGE_PROVIDER = 'GCS')) ALLOW_WRITES = TRUE COMMENT = 'c'`,
		`CREATE EXTERNAL VOLUME vol STORAGE_LOCATIONS = ((NAME = 'a'), (NAME = 'b'))`,
	)
	assertInvalid(t, (*Validator).ParseCreateExternalVolume,
		`CREATE EXTERNAL VOLUME`,
		`CREATE EXTERNAL VOLUME vol STORAGE_LOCATIONS`,         // missing = (...)
		`CREATE VOLUME vol STORAGE_LOCATIONS = ((NAME = 'a'))`, // missing EXTERNAL
	)
}

func TestParseCreateFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateFailoverGroup,
		`CREATE FAILOVER GROUP fg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org.acct`,
		`CREATE FAILOVER GROUP IF NOT EXISTS fg OBJECT_TYPES = DATABASES, ROLES ALLOWED_DATABASES = db1, db2 ALLOWED_ACCOUNTS = org.a1, org.a2 IGNORE EDITION CHECK OPTIMIZED_REFRESH = TRUE`,
		`CREATE FAILOVER GROUP fg AS REPLICA OF org.acct.fg`,
	)
	assertInvalid(t, (*Validator).ParseCreateFailoverGroup,
		`CREATE FAILOVER GROUP`,
		`CREATE FAILOVER fg OBJECT_TYPES = DATABASES`, // missing GROUP
		`CREATE FAILOVER GROUP fg AS REPLICA OF`,      // missing source name
	)
}

func TestParseCreateFeaturePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateFeaturePolicy,
		`CREATE FEATURE POLICY fp BLOCKED_OBJECT_TYPES_FOR_CREATION = (TABLE)`,
		`CREATE OR REPLACE FEATURE POLICY IF NOT EXISTS db.sch.fp BLOCKED_OBJECT_TYPES_FOR_CREATION = (TABLE, VIEW) COMMENT = 'c'`,
		`CREATE FEATURE POLICY fp BLOCKED_OBJECT_TYPES_FOR_CREATION = (VIEW)`,
	)
	assertInvalid(t, (*Validator).ParseCreateFeaturePolicy,
		`CREATE FEATURE POLICY`,
		`CREATE FEATURE POLICY fp`, // missing required option
		`CREATE FEATURE fp BLOCKED_OBJECT_TYPES_FOR_CREATION = (TABLE)`, // missing POLICY
	)
}

func TestParseCreateFileFormat(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateFileFormat,
		`CREATE FILE FORMAT ff TYPE = CSV`,
		`CREATE OR REPLACE TEMPORARY FILE FORMAT IF NOT EXISTS db.sch.ff TYPE = CSV FIELD_DELIMITER = ',' SKIP_HEADER = 1 COMMENT = 'c'`,
		`CREATE FILE FORMAT ff TYPE = JSON COMPRESSION = GZIP`,
	)
	assertInvalid(t, (*Validator).ParseCreateFileFormat,
		`CREATE FILE FORMAT`,
		`CREATE FORMAT ff TYPE = CSV`,           // missing FILE
		`CREATE FILE FORMAT ff TYPE = CSV TYPE`, // dangling TYPE without value
	)
}

func TestParseCreateFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateFunction,
		`CREATE FUNCTION f () RETURNS INTEGER LANGUAGE JAVASCRIPT AS 'return 1;'`,
		`CREATE OR REPLACE SECURE FUNCTION db.sch.f (a INT, b VARCHAR) RETURNS TABLE (x INT, y VARCHAR) LANGUAGE PYTHON RUNTIME_VERSION = '3.10' HANDLER = 'm.run' PACKAGES = ('pandas') AS 'def run(): pass'`,
		`CREATE FUNCTION f (x INT) RETURNS INT IMMUTABLE MEMOIZABLE AS 'select 1'`,
	)
	assertInvalid(t, (*Validator).ParseCreateFunction,
		`CREATE FUNCTION`,
		`CREATE FUNCTION f (x INT) LANGUAGE SQL AS 'x'`, // missing RETURNS
		`CREATE FUNCTION f RETURNS INT AS 'x'`,          // missing argument parens
	)
}

func TestParseCreateFunctionSnowparkContainerServices(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateFunctionSnowparkContainerServices,
		`CREATE FUNCTION f (x INT) RETURNS VARIANT SERVICE = svc ENDPOINT = ep AS '/predict'`,
		`CREATE OR REPLACE FUNCTION db.sch.f () RETURNS VARCHAR NOT NULL STRICT VOLATILE SERVICE = svc ENDPOINT = ep MAX_BATCH_ROWS = 10 ON_BATCH_FAILURE = ABORT AS '/p'`,
		`CREATE FUNCTION f (a VARCHAR, b INT) RETURNS VARIANT SERVICE = s ENDPOINT = e COMMENT = 'c' AS '/x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateFunctionSnowparkContainerServices,
		`CREATE FUNCTION`,
		`CREATE FUNCTION f (x INT) RETURNS VARIANT SERVICE = svc ENDPOINT = ep`, // missing AS
		`CREATE FUNCTION f (x INT) SERVICE = svc ENDPOINT = ep AS '/x'`,         // missing RETURNS
	)
}

func TestParseCreateGateway(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateGateway,
		`CREATE GATEWAY gw FROM SPECIFICATION 'spec text'`,
		`CREATE OR REPLACE GATEWAY IF NOT EXISTS db.sch.gw FROM SPECIFICATION 'yaml: here'`,
		`CREATE GATEWAY gw FROM SPECIFICATION $$ raw body $$`,
	)
	assertInvalid(t, (*Validator).ParseCreateGateway,
		`CREATE GATEWAY`,
		`CREATE GATEWAY gw FROM 'spec'`,          // missing SPECIFICATION
		`CREATE GATEWAY gw SPECIFICATION 'spec'`, // missing FROM
	)
}

func TestParseCreateGitRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateGitRepository,
		`CREATE GIT REPOSITORY repo ORIGIN = 'https://github.com/x/y' API_INTEGRATION = api_int`,
		`CREATE OR REPLACE GIT REPOSITORY IF NOT EXISTS db.sch.repo ORIGIN = 'https://g' API_INTEGRATION = ai GIT_CREDENTIALS = cred COMMENT = 'c'`,
		`CREATE GIT REPOSITORY repo API_INTEGRATION = ai ORIGIN = 'https://g'`,
	)
	assertInvalid(t, (*Validator).ParseCreateGitRepository,
		`CREATE GIT REPOSITORY`,
		`CREATE GIT repo ORIGIN = 'u' API_INTEGRATION = ai`,      // missing REPOSITORY
		`CREATE GIT REPOSITORY repo ORIGIN API_INTEGRATION = ai`, // ORIGIN missing = value
	)
}

func TestParseCreateHybridTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateHybridTable,
		`CREATE HYBRID TABLE t (id INT PRIMARY KEY, name VARCHAR)`,
		`CREATE OR REPLACE HYBRID TABLE IF NOT EXISTS db.sch.t (id INT NOT NULL, INDEX idx (name)) COMMENT = 'c'`,
		`CREATE HYBRID TABLE t (a INT, b VARCHAR, PRIMARY KEY (a))`,
	)
	assertInvalid(t, (*Validator).ParseCreateHybridTable,
		`CREATE HYBRID TABLE`,
		`CREATE HYBRID TABLE t`,    // missing column list
		`CREATE HYBRID t (id INT)`, // missing TABLE
	)
}

func TestParseCreateIcebergTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTable,
		`CREATE ICEBERG TABLE t (id INT, name VARCHAR) EXTERNAL_VOLUME = 'vol' CATALOG = 'SNOWFLAKE' BASE_LOCATION = 'dir'`,
		`CREATE OR REPLACE TRANSIENT ICEBERG TABLE IF NOT EXISTS db.sch.t (id INT) CLUSTER BY (id) STORAGE_SERIALIZATION_POLICY = OPTIMIZED COMMENT = 'c'`,
		`CREATE ICEBERG TABLE t LIKE src EXTERNAL_VOLUME = 'v'`,
		// GET_DDL emits CLUSTER BY before the column list (#776).
		`CREATE OR REPLACE ICEBERG TABLE t CLUSTER BY (id) (id INT) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 'dir'`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTable,
		`CREATE ICEBERG TABLE`,
		`CREATE ICEBERG t (id INT)`, // missing TABLE
		`ICEBERG TABLE t (id INT)`,  // missing CREATE
	)
}

func TestParseCreateIcebergTableAwsGlueCatalog(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTableAwsGlueCatalog,
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME = 'glue_tbl'`,
		`CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS db.sch.t EXTERNAL_VOLUME = 'v' CATALOG = 'ci' CATALOG_TABLE_NAME = 'gt' CATALOG_NAMESPACE = 'ns' AUTO_REFRESH = TRUE COMMENT = 'c'`,
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME = 'gt' REPLACE_INVALID_CHARACTERS = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTableAwsGlueCatalog,
		`CREATE ICEBERG TABLE`,
		`CREATE ICEBERG t CATALOG_TABLE_NAME = 'gt'`, // missing TABLE
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME`,  // missing = value
	)
}

func TestParseCreateIcebergTableDeltaFiles(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTableDeltaFiles,
		`CREATE ICEBERG TABLE t BASE_LOCATION = 'path'`,
		`CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS db.sch.t EXTERNAL_VOLUME = 'v' CATALOG = 'ci' BASE_LOCATION = 'p' AUTO_REFRESH = TRUE COMMENT = 'c'`,
		`CREATE ICEBERG TABLE t BASE_LOCATION = 'p' REPLACE_INVALID_CHARACTERS = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTableDeltaFiles,
		`CREATE ICEBERG TABLE`,
		`CREATE ICEBERG t BASE_LOCATION = 'p'`, // missing TABLE
		`CREATE ICEBERG TABLE t BASE_LOCATION`, // missing = value
	)
}

func TestParseCreateIcebergTableIcebergFiles(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTableIcebergFiles,
		`CREATE ICEBERG TABLE t METADATA_FILE_PATH = 'meta.json'`,
		`CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS db.sch.t EXTERNAL_VOLUME = 'v' CATALOG = 'ci' METADATA_FILE_PATH = 'm' COMMENT = 'c'`,
		`CREATE ICEBERG TABLE t METADATA_FILE_PATH = 'm' REPLACE_INVALID_CHARACTERS = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTableIcebergFiles,
		`CREATE ICEBERG TABLE`,
		`CREATE ICEBERG t METADATA_FILE_PATH = 'm'`, // missing TABLE
		`CREATE ICEBERG TABLE t METADATA_FILE_PATH`, // missing = value
	)
}

func TestParseCreateIcebergTableIcebergRestCatalog(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateIcebergTableIcebergRestCatalog,
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME = 'rest_tbl'`,
		`CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS db.sch.t EXTERNAL_VOLUME = 'v' CATALOG = 'ci' CATALOG_TABLE_NAME = 'rt' PATH_LAYOUT = FLAT STORAGE_SERIALIZATION_POLICY = OPTIMIZED COMMENT = 'c'`,
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME = 'rt' AUTO_REFRESH = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateIcebergTableIcebergRestCatalog,
		`CREATE ICEBERG TABLE`,
		`CREATE ICEBERG t CATALOG_TABLE_NAME = 'rt'`, // missing TABLE
		`CREATE ICEBERG TABLE t CATALOG_TABLE_NAME`,  // missing = value
	)
}
