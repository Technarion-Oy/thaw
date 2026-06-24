package sqlgrammar

import "testing"

func TestParseCreateObj(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateObj,
		`CREATE WIDGET my_widget ( a, b )`,
		`CREATE OR REPLACE GADGET db.schema.thing OPTION = 1`,
		`CREATE THING IF NOT EXISTS foo BAR = 'baz'`,
	)
	assertInvalid(t, (*Validator).ParseCreateObj,
		`CREATE`,
		`WIDGET my_widget`,
	)
}

func TestParseCreateAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAccount,
		`CREATE ACCOUNT acct ADMIN_NAME = 'admin' ADMIN_PASSWORD = 'pw' EMAIL = 'a@b.com' EDITION = STANDARD`,
		`CREATE ACCOUNT acct ADMIN_NAME = 'admin' ADMIN_RSA_PUBLIC_KEY = 'key' EMAIL = 'a@b.com' EDITION = ENTERPRISE MUST_CHANGE_PASSWORD = TRUE`,
		`CREATE ACCOUNT acct EDITION = BUSINESS_CRITICAL ADMIN_USER_TYPE = SERVICE COMMENT = 'x' POLARIS = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateAccount,
		`CREATE ACCOUNT acct`,
		`CREATE ACCOUNT ADMIN_NAME = 'admin'`,
	)
}

func TestParseCreateAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAgent,
		`CREATE AGENT my_agent FROM SPECIFICATION $$ spec $$`,
		`CREATE OR REPLACE AGENT IF NOT EXISTS db.sc.agent COMMENT = 'c' PROFILE = 'p' FROM SPECIFICATION $$body$$`,
		`CREATE AGENT a FROM SPECIFICATION 'spec text'`,
	)
	assertInvalid(t, (*Validator).ParseCreateAgent,
		`CREATE AGENT my_agent`,
		`CREATE AGENT my_agent FROM SPECIFICATION`,
	)
}

func TestParseCreateAggregationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAggregationPolicy,
		`CREATE AGGREGATION POLICY my_policy AS () RETURNS AGGREGATION_CONSTRAINT -> AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)`,
		`CREATE OR REPLACE AGGREGATION POLICY IF NOT EXISTS p AS () RETURNS AGGREGATION_CONSTRAINT -> NO_AGGREGATION_CONSTRAINT() COMMENT = 'c'`,
		`CREATE AGGREGATION POLICY p AS () RETURNS AGGREGATION_CONSTRAINT -> CASE WHEN x THEN y ELSE z END`,
	)
	assertInvalid(t, (*Validator).ParseCreateAggregationPolicy,
		`CREATE AGGREGATION POLICY p`,
		`CREATE AGGREGATION POLICY p AS () RETURNS AGGREGATION_CONSTRAINT`,
	)
}

func TestParseCreateAlert(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAlert,
		`CREATE ALERT my_alert WAREHOUSE = wh SCHEDULE = '1 MINUTE' IF( EXISTS( SELECT 1 )) THEN INSERT INTO t VALUES (1)`,
		`CREATE OR REPLACE ALERT IF NOT EXISTS a COMMENT = 'c' IF (EXISTS (SELECT col FROM tbl WHERE x > 1)) THEN CALL p()`,
		`CREATE ALERT a SUSPEND_ALERT_AFTER_NUM_FAILURES = 3 IF(EXISTS(SELECT 1)) THEN SELECT 1`,
	)
	assertInvalid(t, (*Validator).ParseCreateAlert,
		`CREATE ALERT a`,
		`CREATE ALERT a IF (EXISTS (SELECT 1))`,
	)
}

func TestParseCreateApiIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateApiIntegration,
		`CREATE API INTEGRATION api API_PROVIDER = aws_api_gateway API_AWS_ROLE_ARN = 'arn' API_ALLOWED_PREFIXES = ('https://x') ENABLED = TRUE`,
		`CREATE OR REPLACE API INTEGRATION IF NOT EXISTS api API_PROVIDER = azure_api_management AZURE_TENANT_ID = 't' AZURE_AD_APPLICATION_ID = 'a' API_ALLOWED_PREFIXES = ('x') ENABLED = FALSE COMMENT = 'c'`,
		`CREATE API INTEGRATION api API_PROVIDER = google_api_gateway GOOGLE_AUDIENCE = 'aud' API_ALLOWED_PREFIXES = ('x', 'y') ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateApiIntegration,
		`CREATE API INTEGRATION api`,
		`CREATE API api API_PROVIDER = aws_api_gateway`,
	)
}

func TestParseCreateApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateApplication,
		`CREATE APPLICATION app FROM APPLICATION PACKAGE pkg`,
		`CREATE APPLICATION app FROM APPLICATION PACKAGE pkg USING VERSION v1 PATCH 3 DEBUG_MODE = TRUE COMMENT = 'c'`,
		`CREATE APPLICATION app FROM LISTING my_listing USING RELEASE CHANNEL DEFAULT BACKGROUND_INSTALL = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateApplication,
		`CREATE APPLICATION app`,
		`CREATE APPLICATION app FROM APPLICATION pkg`,
	)
}

func TestParseCreateApplicationPackage(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateApplicationPackage,
		`CREATE APPLICATION PACKAGE pkg`,
		`CREATE APPLICATION PACKAGE IF NOT EXISTS pkg DATA_RETENTION_TIME_IN_DAYS = 7 DISTRIBUTION = INTERNAL COMMENT = 'c'`,
		`CREATE APPLICATION PACKAGE pkg MULTIPLE_INSTANCES = TRUE ENABLE_RELEASE_CHANNELS = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateApplicationPackage,
		`CREATE APPLICATION PACKAGE`,
		`CREATE PACKAGE pkg`,
	)
}

func TestParseCreateApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateApplicationRole,
		`CREATE APPLICATION ROLE r`,
		`CREATE OR REPLACE APPLICATION ROLE IF NOT EXISTS r COMMENT = 'c'`,
		`CREATE OR ALTER APPLICATION ROLE r COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateApplicationRole,
		`CREATE APPLICATION ROLE`,
		`CREATE ROLE r`,
	)
}

func TestParseCreateAuthenticationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateAuthenticationPolicy,
		`CREATE AUTHENTICATION POLICY p`,
		`CREATE OR REPLACE AUTHENTICATION POLICY IF NOT EXISTS p AUTHENTICATION_METHODS = ('PASSWORD', 'SAML') MFA_ENROLLMENT = 'REQUIRED' COMMENT = 'c'`,
		`CREATE AUTHENTICATION POLICY p CLIENT_POLICY = ( DRIVERS = ( MINIMUM_VERSION = '1.0' ) ) MFA_POLICY = ( ALLOWED_METHODS = ('PASSKEY') )`,
	)
	assertInvalid(t, (*Validator).ParseCreateAuthenticationPolicy,
		`CREATE AUTHENTICATION POLICY`,
		`CREATE POLICY p`,
	)
}

func TestParseCreateBackupPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateBackupPolicy,
		`CREATE BACKUP POLICY p`,
		`CREATE OR REPLACE BACKUP POLICY IF NOT EXISTS p WITH RETENTION LOCK SCHEDULE = '1 HOUR' EXPIRE_AFTER_DAYS = 30 COMMENT = 'c'`,
		`CREATE BACKUP POLICY p TAG ( t = 'v' ) EXPIRE_AFTER_DAYS = 10`,
	)
	assertInvalid(t, (*Validator).ParseCreateBackupPolicy,
		`CREATE BACKUP POLICY`,
		`CREATE POLICY p`,
	)
}

func TestParseCreateBackupSet(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateBackupSet,
		`CREATE BACKUP SET bs FOR TABLE t`,
		`CREATE OR REPLACE BACKUP SET IF NOT EXISTS bs FOR DYNAMIC TABLE t WITH BACKUP POLICY pol COMMENT = 'c'`,
		`CREATE BACKUP SET bs FOR DATABASE db TAG ( a = 'b' )`,
	)
	assertInvalid(t, (*Validator).ParseCreateBackupSet,
		`CREATE BACKUP SET bs`,
		`CREATE BACKUP SET bs FOR t`,
	)
}

func TestParseCreateCatalogIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegration,
		`CREATE CATALOG INTEGRATION ci CATALOG_SOURCE = GLUE ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS ci CATALOG_SOURCE = OBJECT_STORE TABLE_FORMAT = ICEBERG ENABLED = FALSE`,
		`CREATE CATALOG INTEGRATION ci REST_CONFIG = ( CATALOG_URI = 'x' ) ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegration,
		`CREATE CATALOG INTEGRATION ci`,
		`CREATE INTEGRATION ci CATALOG_SOURCE = GLUE`,
	)
}

func TestParseCreateCatalogIntegrationAwsGlue(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationAwsGlue,
		`CREATE CATALOG INTEGRATION ci CATALOG_SOURCE = GLUE TABLE_FORMAT = ICEBERG GLUE_AWS_ROLE_ARN = 'arn' GLUE_CATALOG_ID = 'id' ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS ci CATALOG_SOURCE = GLUE GLUE_REGION = 'us-east-1' CATALOG_NAMESPACE = 'ns' REFRESH_INTERVAL_SECONDS = 60 ENABLED = FALSE COMMENT = 'c'`,
		`CREATE CATALOG INTEGRATION ci ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationAwsGlue,
		`CREATE CATALOG INTEGRATION ci`,
		`CREATE CATALOG ci CATALOG_SOURCE = GLUE`,
	)
}

func TestParseCreateCatalogIntegrationObjectStorage(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationObjectStorage,
		`CREATE CATALOG INTEGRATION ci CATALOG_SOURCE = OBJECT_STORE TABLE_FORMAT = ICEBERG ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS ci CATALOG_SOURCE = OBJECT_STORE TABLE_FORMAT = DELTA REFRESH_INTERVAL_SECONDS = 30 ENABLED = FALSE COMMENT = 'c'`,
		`CREATE CATALOG INTEGRATION ci ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationObjectStorage,
		`CREATE CATALOG INTEGRATION ci`,
		`CREATE INTEGRATION ci ENABLED = TRUE`,
	)
}

func TestParseCreateCatalogIntegrationSnowflakeOpenCatalog(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationSnowflakeOpenCatalog,
		`CREATE CATALOG INTEGRATION ci CATALOG_SOURCE = POLARIS TABLE_FORMAT = ICEBERG REST_CONFIG = ( CATALOG_URI = 'u' CATALOG_NAME = 'n' ) REST_AUTHENTICATION = ( TYPE = OAUTH OAUTH_CLIENT_ID = 'id' OAUTH_CLIENT_SECRET = 's' OAUTH_ALLOWED_SCOPES = ('s1', 's2') ) ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS ci CATALOG_SOURCE = POLARIS TABLE_FORMAT = ICEBERG CATALOG_NAMESPACE = 'ns' ENABLED = FALSE COMMENT = 'c'`,
		`CREATE CATALOG INTEGRATION ci ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationSnowflakeOpenCatalog,
		`CREATE CATALOG INTEGRATION ci`,
		`CREATE CATALOG ci CATALOG_SOURCE = POLARIS`,
	)
}

func TestParseCreateCatalogIntegrationApacheIcebergRest(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCatalogIntegrationApacheIcebergRest,
		`CREATE CATALOG INTEGRATION ci CATALOG_SOURCE = ICEBERG_REST TABLE_FORMAT = ICEBERG REST_CONFIG = ( CATALOG_URI = 'u' ) REST_AUTHENTICATION = ( TYPE = BEARER BEARER_TOKEN = 'tok' ) ENABLED = TRUE`,
		`CREATE OR REPLACE CATALOG INTEGRATION IF NOT EXISTS ci CATALOG_SOURCE = ICEBERG_REST REST_AUTHENTICATION = ( TYPE = SIGV4 SIGV4_IAM_ROLE = 'arn' SIGV4_SIGNING_REGION = 'r' ) ENABLED = FALSE COMMENT = 'c'`,
		`CREATE CATALOG INTEGRATION ci ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateCatalogIntegrationApacheIcebergRest,
		`CREATE CATALOG INTEGRATION ci`,
		`CREATE INTEGRATION ci CATALOG_SOURCE = ICEBERG_REST`,
	)
}
