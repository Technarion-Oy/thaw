package sqlgrammar

import "testing"

func TestParseCreateOrganizationUser(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrganizationUser,
		`CREATE ORGANIZATION USER jdoe`,
		`CREATE ORGANIZATION USER IF NOT EXISTS jdoe EMAIL = 'j@x.com' LOGIN_NAME = 'jdoe'`,
		`CREATE ORGANIZATION USER jdoe FIRST_NAME = 'John' LAST_NAME = 'Doe' COMMENT = 'hi'`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrganizationUser,
		`CREATE ORGANIZATION USER`,
		`CREATE ORGANIZATION jdoe`,
		`CREATE ORGANIZATION USER jdoe EMAIL 'j@x.com'`,
	)
}

func TestParseCreateOrganizationUserGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrganizationUserGroup,
		`CREATE ORGANIZATION USER GROUP grp`,
		`CREATE ORGANIZATION USER GROUP IF NOT EXISTS grp`,
		`CREATE ORGANIZATION USER GROUP grp IS_GRANTABLE = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrganizationUserGroup,
		`CREATE ORGANIZATION USER GROUP`,
		`CREATE ORGANIZATION USER grp`,
		`CREATE ORGANIZATION USER GROUP grp IS_GRANTABLE TRUE`,
	)
}

func TestParseCreatePackagesPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreatePackagesPolicy,
		`CREATE PACKAGES POLICY pp LANGUAGE PYTHON`,
		`CREATE OR REPLACE PACKAGES POLICY pp LANGUAGE PYTHON ALLOWLIST = ('numpy', 'pandas')`,
		`CREATE PACKAGES POLICY IF NOT EXISTS pp LANGUAGE PYTHON BLOCKLIST = () COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreatePackagesPolicy,
		`CREATE PACKAGES POLICY pp`,
		`CREATE PACKAGES POLICY LANGUAGE PYTHON`,
		`CREATE PACKAGES POLICY pp LANGUAGE JAVA`,
	)
}

func TestParseCreatePasswordPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreatePasswordPolicy,
		`CREATE PASSWORD POLICY pol`,
		`CREATE OR REPLACE PASSWORD POLICY pol PASSWORD_MIN_LENGTH = 10 PASSWORD_MAX_LENGTH = 256`,
		`CREATE PASSWORD POLICY IF NOT EXISTS pol PASSWORD_MAX_RETRIES = 5 COMMENT = 'strict'`,
	)
	assertInvalid(t, (*Validator).ParseCreatePasswordPolicy,
		`CREATE PASSWORD POLICY`,
		`CREATE PASSWORD pol`,
		`CREATE PASSWORD POLICY pol PASSWORD_MIN_LENGTH 10`,
	)
}

func TestParseCreatePipe(t *testing.T) {
	assertValid(t, (*Validator).ParseCreatePipe,
		`CREATE PIPE mypipe AS COPY INTO t FROM @s`,
		`CREATE OR REPLACE PIPE mypipe AUTO_INGEST = TRUE AS COPY INTO t FROM @s`,
		`CREATE PIPE IF NOT EXISTS mypipe ERROR_INTEGRATION = ei COMMENT = 'c' AS COPY INTO t FROM @s`,
	)
	assertInvalid(t, (*Validator).ParseCreatePipe,
		`CREATE PIPE mypipe`,
		`CREATE PIPE AS COPY INTO t FROM @s`,
		`CREATE mypipe AS COPY INTO t FROM @s`,
	)
}

func TestParseCreatePostgresInstance(t *testing.T) {
	assertValid(t, (*Validator).ParseCreatePostgresInstance,
		`CREATE POSTGRES INSTANCE pg COMPUTE_FAMILY = 'CPU_X64_S' STORAGE_SIZE_GB = 100 AUTHENTICATION_AUTHORITY = POSTGRES`,
		`CREATE POSTGRES INSTANCE pg POSTGRES_VERSION = 17 HIGH_AVAILABILITY = TRUE COMMENT = 'c'`,
		`CREATE POSTGRES INSTANCE pg FORK src AT (TIMESTAMP => '2024-01-01') COMPUTE_FAMILY = 'CPU_X64_S'`,
	)
	assertInvalid(t, (*Validator).ParseCreatePostgresInstance,
		`CREATE POSTGRES INSTANCE`,
		`CREATE POSTGRES pg`,
		`CREATE POSTGRES INSTANCE pg COMPUTE_FAMILY 'x'`,
	)
}

func TestParseCreatePrivacyPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreatePrivacyPolicy,
		`CREATE PRIVACY POLICY pp AS () RETURNS PRIVACY_BUDGET -> 1`,
		`CREATE OR REPLACE PRIVACY POLICY pp AS () RETURNS PRIVACY_BUDGET -> budget(10)`,
		`CREATE PRIVACY POLICY IF NOT EXISTS pp AS () RETURNS PRIVACY_BUDGET -> case when x then 1 else 2 end`,
	)
	assertInvalid(t, (*Validator).ParseCreatePrivacyPolicy,
		`CREATE PRIVACY POLICY pp`,
		`CREATE PRIVACY POLICY pp AS () RETURNS PRIVACY_BUDGET`,
		`CREATE PRIVACY pp AS () RETURNS PRIVACY_BUDGET -> 1`,
	)
}

func TestParseCreateProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateProcedure,
		`CREATE PROCEDURE p() RETURNS STRING LANGUAGE SQL AS 'BEGIN RETURN 1; END'`,
		`CREATE OR REPLACE PROCEDURE p(x INT, y VARCHAR) RETURNS TABLE (a INT) LANGUAGE SQL AS $$ x $$`,
		`CREATE SECURE PROCEDURE p() COPY GRANTS RETURNS INT NOT NULL LANGUAGE SQL AS 'x'`,
		// Full Java handler form: RUNTIME_VERSION / PACKAGES / HANDLER / SECRETS /
		// EXTERNAL_ACCESS_INTEGRATIONS / EXECUTE AS now modeled, not swallowed.
		`CREATE PROCEDURE p(x INT) RETURNS STRING LANGUAGE JAVA RUNTIME_VERSION = '11' ` +
			`PACKAGES = ('com.snowflake:snowpark:1.2') HANDLER = 'C.m' ` +
			`EXTERNAL_ACCESS_INTEGRATIONS = (i1, i2) EXECUTE AS CALLER AS 'body'`,
		`CREATE PROCEDURE p() RETURNS INT LANGUAGE PYTHON STRICT IMMUTABLE COMMENT = 'c' ` +
			`EXECUTE AS RESTRICTED CALLER AS 'x'`,
		// Unmodeled option key tolerated by the generic fallback (no false reject).
		`CREATE PROCEDURE p() RETURNS INT LANGUAGE SQL SOME_FUTURE_OPT = 5 AS 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateProcedure,
		`CREATE PROCEDURE p()`,
		`CREATE PROCEDURE p RETURNS INT`,
		`CREATE PROCEDURE () RETURNS INT`,
		// Newly enforced: RETURNS must have a type, and the AS body is required
		// (the old consumeRest body accepted both of these).
		`CREATE PROCEDURE p() RETURNS LANGUAGE SQL AS 'x'`,
		`CREATE PROCEDURE p() RETURNS INT LANGUAGE SQL`,
	)
}

func TestParseCreateProjectionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateProjectionPolicy,
		`CREATE PROJECTION POLICY pp AS () RETURNS PROJECTION_CONSTRAINT -> PROJECTION_CONSTRAINT(ALLOW => true)`,
		`CREATE OR REPLACE PROJECTION POLICY pp AS () RETURNS PROJECTION_CONSTRAINT -> 1`,
		`CREATE PROJECTION POLICY IF NOT EXISTS db.sc.pp AS () RETURNS PROJECTION_CONSTRAINT -> x`,
	)
	assertInvalid(t, (*Validator).ParseCreateProjectionPolicy,
		`CREATE PROJECTION POLICY pp`,
		`CREATE PROJECTION POLICY pp AS () RETURNS PROJECTION_CONSTRAINT`,
		`CREATE PROJECTION pp AS () RETURNS PROJECTION_CONSTRAINT -> x`,
	)
}

func TestParseCreateProvisionedThroughput(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateProvisionedThroughput,
		`CREATE PROVISIONED THROUGHPUT pt CLOUD_PROVIDER = 'aws' MODEL = 'm' PTUS = 100 TERM_START = '2024-01-01' TERM_END = '2024-12-31'`,
		`CREATE OR REPLACE PROVISIONED THROUGHPUT pt MODEL = 'm' PTUS = 50`,
		`CREATE PROVISIONED THROUGHPUT pt CLOUD_PROVIDER = 'azure'`,
	)
	assertInvalid(t, (*Validator).ParseCreateProvisionedThroughput,
		`CREATE PROVISIONED THROUGHPUT pt`,
		`CREATE PROVISIONED pt`,
		`CREATE PROVISIONED THROUGHPUT pt CLOUD_PROVIDER 'aws'`,
	)
}

func TestParseCreateReplicationGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateReplicationGroup,
		`CREATE REPLICATION GROUP rg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org.acct`,
		`CREATE REPLICATION GROUP IF NOT EXISTS rg OBJECT_TYPES = DATABASES, ROLES ALLOWED_DATABASES = db1, db2 ALLOWED_ACCOUNTS = org.a`,
		`CREATE REPLICATION GROUP rg AS REPLICA OF org.acct.src`,
	)
	assertInvalid(t, (*Validator).ParseCreateReplicationGroup,
		`CREATE REPLICATION GROUP`,
		`CREATE REPLICATION rg OBJECT_TYPES = DATABASES`,
		`CREATE REPLICATION GROUP rg`,
	)
}

func TestParseCreateResourceMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateResourceMonitor,
		`CREATE RESOURCE MONITOR rm WITH CREDIT_QUOTA = 100`,
		`CREATE OR REPLACE RESOURCE MONITOR rm WITH FREQUENCY = MONTHLY START_TIMESTAMP = IMMEDIATELY TRIGGERS ON 80 PERCENT DO SUSPEND ON 100 PERCENT DO SUSPEND_IMMEDIATE`,
		`CREATE RESOURCE MONITOR IF NOT EXISTS rm WITH NOTIFY_USERS = (u1, u2)`,
	)
	assertInvalid(t, (*Validator).ParseCreateResourceMonitor,
		`CREATE RESOURCE MONITOR rm`,
		`CREATE RESOURCE rm WITH CREDIT_QUOTA = 100`,
		`CREATE RESOURCE MONITOR WITH CREDIT_QUOTA = 100`,
	)
}

func TestParseCreateRole(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateRole,
		`CREATE ROLE myrole`,
		`CREATE OR REPLACE ROLE IF NOT EXISTS myrole COMMENT = 'x'`,
		`CREATE ROLE myrole WITH TAG (cost = 'high')`,
	)
	assertInvalid(t, (*Validator).ParseCreateRole,
		`CREATE ROLE`,
		`CREATE myrole`,
		`CREATE ROLE myrole COMMENT 'x'`,
	)
}

func TestParseCreateRowAccessPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateRowAccessPolicy,
		`CREATE ROW ACCESS POLICY rap AS (n VARCHAR) RETURNS BOOLEAN -> n = current_user()`,
		`CREATE OR REPLACE ROW ACCESS POLICY rap AS (a INT, b INT) RETURNS BOOLEAN -> a > b`,
		`CREATE ROW ACCESS POLICY IF NOT EXISTS db.sc.rap AS (x INT) RETURNS BOOLEAN -> true`,
	)
	assertInvalid(t, (*Validator).ParseCreateRowAccessPolicy,
		`CREATE ROW ACCESS POLICY rap`,
		`CREATE ROW ACCESS POLICY rap AS (n VARCHAR) RETURNS BOOLEAN`,
		`CREATE ROW POLICY rap AS (n VARCHAR) RETURNS BOOLEAN -> true`,
	)
}

func TestParseCreateSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSchema,
		`CREATE SCHEMA mysch`,
		`CREATE OR REPLACE TRANSIENT SCHEMA IF NOT EXISTS mysch DATA_RETENTION_TIME_IN_DAYS = 1 WITH MANAGED ACCESS COMMENT = 'c'`,
		`CREATE SCHEMA mysch CLONE src AT (OFFSET => -60)`,
		`CREATE SCHEMA mysch FROM BACKUP SET bs IDENTIFIER 'abc'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSchema,
		`CREATE SCHEMA`,
		`CREATE mysch`,
		`CREATE SCHEMA mysch DATA_RETENTION_TIME_IN_DAYS 1`,
	)
}

func TestParseCreateSecret(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecret,
		`CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'`,
		`CREATE OR REPLACE SECRET IF NOT EXISTS s TYPE = OAUTH2 API_AUTHENTICATION = ai OAUTH_SCOPES = ('a', 'b')`,
		`CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'xyz' COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecret,
		`CREATE SECRET s`,
		`CREATE SECRET TYPE = PASSWORD`,
		`CREATE SECRET s TYPE PASSWORD`,
	)
}
