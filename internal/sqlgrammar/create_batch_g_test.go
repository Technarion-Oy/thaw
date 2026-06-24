package sqlgrammar

import "testing"

func TestParseCreateSecurityIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegration,
		`CREATE SECURITY INTEGRATION my_int TYPE = SCIM ENABLED = TRUE`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS my_int TYPE = OAUTH OAUTH_CLIENT = LOOKER`,
		`CREATE SECURITY INTEGRATION my_int TYPE = SAML2 COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegration,
		`CREATE SECURITY INTEGRATION my_int`,
		`CREATE SECURITY my_int TYPE = SCIM`,
		`SECURITY INTEGRATION my_int TYPE = SCIM`,
	)
}

func TestParseCreateSecurityIntegrationExternalApiAuthentication(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationExternalApiAuthentication,
		`CREATE SECURITY INTEGRATION my_api TYPE = API_AUTHENTICATION AUTH_TYPE = OAUTH2 ENABLED = TRUE`,
		`CREATE SECURITY INTEGRATION my_api TYPE = API_AUTHENTICATION AUTH_TYPE = OAUTH2 ENABLED = TRUE OAUTH_TOKEN_ENDPOINT = 'https://x' OAUTH_CLIENT_ID = 'cid' OAUTH_CLIENT_SECRET = 'secret'`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS my_api TYPE = API_AUTHENTICATION AUTH_TYPE = OAUTH2 ENABLED = FALSE OAUTH_ALLOWED_SCOPES = ('a', 'b') COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationExternalApiAuthentication,
		`CREATE SECURITY INTEGRATION my_api`,
		`CREATE SECURITY INTEGRATION TYPE = API_AUTHENTICATION`,
	)
}

func TestParseCreateSecurityIntegrationAwsIamAuthentication(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationAwsIamAuthentication,
		`CREATE SECURITY INTEGRATION my_iam TYPE = API_AUTHENTICATION AUTH_TYPE = AWS_IAM AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENABLED = TRUE`,
		`CREATE OR REPLACE SECURITY INTEGRATION my_iam TYPE = API_AUTHENTICATION AUTH_TYPE = AWS_IAM AWS_ROLE_ARN = 'arn' ENABLED = FALSE COMMENT = 'c'`,
		`CREATE SECURITY INTEGRATION IF NOT EXISTS my_iam AUTH_TYPE = AWS_IAM AWS_ROLE_ARN = 'arn' TYPE = API_AUTHENTICATION ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationAwsIamAuthentication,
		`CREATE SECURITY INTEGRATION my_iam`,
		`CREATE SECURITY INTEGRATION my_iam TYPE API_AUTHENTICATION`,
	)
}

func TestParseCreateSecurityIntegrationExternalOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationExternalOauth,
		`CREATE SECURITY INTEGRATION ext TYPE = EXTERNAL_OAUTH ENABLED = TRUE EXTERNAL_OAUTH_TYPE = OKTA EXTERNAL_OAUTH_ISSUER = 'iss' EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = 'sub' EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = 'LOGIN_NAME'`,
		`CREATE OR REPLACE SECURITY INTEGRATION ext TYPE = EXTERNAL_OAUTH ENABLED = FALSE EXTERNAL_OAUTH_TYPE = CUSTOM EXTERNAL_OAUTH_ISSUER = 'iss' EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = ('a','b') EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = 'EMAIL_ADDRESS' EXTERNAL_OAUTH_ANY_ROLE_MODE = ENABLE`,
		`CREATE SECURITY INTEGRATION ext TYPE = EXTERNAL_OAUTH ENABLED = TRUE EXTERNAL_OAUTH_TYPE = AZURE EXTERNAL_OAUTH_ISSUER = 'iss' EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = 'upn' EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = 'LOGIN_NAME' EXTERNAL_OAUTH_BLOCKED_ROLES_LIST = ('ACCOUNTADMIN') COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationExternalOauth,
		`CREATE SECURITY INTEGRATION ext`,
		`CREATE SECURITY INTEGRATION TYPE = EXTERNAL_OAUTH`,
	)
}

func TestParseCreateSecurityIntegrationSnowflakeOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationSnowflakeOauth,
		`CREATE SECURITY INTEGRATION oa TYPE = OAUTH OAUTH_CLIENT = LOOKER OAUTH_REDIRECT_URI = 'https://x'`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS oa TYPE = OAUTH OAUTH_CLIENT = CUSTOM OAUTH_CLIENT_TYPE = 'CONFIDENTIAL' OAUTH_REDIRECT_URI = 'https://x' ENABLED = TRUE OAUTH_REFRESH_TOKEN_VALIDITY = 86400`,
		`CREATE SECURITY INTEGRATION oa TYPE = OAUTH OAUTH_CLIENT = TABLEAU_DESKTOP OAUTH_REDIRECT_URI = 'https://x' BLOCKED_ROLES_LIST = ('ACCOUNTADMIN') COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationSnowflakeOauth,
		`CREATE SECURITY INTEGRATION oa`,
		`CREATE INTEGRATION oa TYPE = OAUTH`,
	)
}

func TestParseCreateSecurityIntegrationSaml2(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationSaml2,
		`CREATE SECURITY INTEGRATION sa TYPE = SAML2 ENABLED = TRUE METADATA_URL = 'https://idp/meta'`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS sa TYPE = SAML2 ENABLED = TRUE SAML2_ISSUER = 'iss' SAML2_SSO_URL = 'https://sso' SAML2_PROVIDER = 'CUSTOM' SAML2_X509_CERT = 'cert'`,
		`CREATE SECURITY INTEGRATION sa TYPE = SAML2 ENABLED = FALSE METADATA_URL = 'u' ALLOWED_USER_DOMAINS = ('a.com','b.com') COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationSaml2,
		`CREATE SECURITY INTEGRATION sa`,
		`CREATE SECURITY INTEGRATION sa TYPE SAML2`,
	)
}

func TestParseCreateSecurityIntegrationScim(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSecurityIntegrationScim,
		`CREATE SECURITY INTEGRATION sc TYPE = SCIM ENABLED = TRUE SCIM_CLIENT = 'OKTA' RUN_AS_ROLE = 'OKTA_PROVISIONER'`,
		`CREATE OR REPLACE SECURITY INTEGRATION IF NOT EXISTS sc TYPE = SCIM ENABLED = FALSE SCIM_CLIENT = 'AZURE' RUN_AS_ROLE = 'AAD_PROVISIONER' NETWORK_POLICY = mypolicy SYNC_PASSWORD = TRUE COMMENT = 'c'`,
		`CREATE SECURITY INTEGRATION sc SCIM_CLIENT = 'GENERIC' TYPE = SCIM ENABLED = TRUE RUN_AS_ROLE = 'GENERIC_SCIM_PROVISIONER'`,
	)
	assertInvalid(t, (*Validator).ParseCreateSecurityIntegrationScim,
		`CREATE SECURITY INTEGRATION sc`,
		`CREATE SECURITY sc TYPE = SCIM`,
	)
}

func TestParseCreateSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSemanticView,
		`CREATE SEMANTIC VIEW sv TABLES (orders AS o.orders)`,
		`CREATE OR REPLACE SEMANTIC VIEW IF NOT EXISTS sv TABLES (a, b) RELATIONSHIPS (a (id) REFERENCES b (id)) METRICS (total AS SUM(x)) COMMENT = 'c'`,
		`CREATE SEMANTIC VIEW sv TABLES (t) DIMENSIONS (d AS t.col) FACTS (f AS t.val) COPY GRANTS`,
	)
	assertInvalid(t, (*Validator).ParseCreateSemanticView,
		`CREATE SEMANTIC VIEW sv`,
		`CREATE SEMANTIC VIEW TABLES (t)`,
	)
}

func TestParseCreateSequence(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSequence,
		`CREATE SEQUENCE seq`,
		`CREATE OR REPLACE SEQUENCE seq START WITH 1 INCREMENT BY 2 ORDER COMMENT = 'c'`,
		`CREATE SEQUENCE IF NOT EXISTS seq START = 100 INCREMENT = 5 NOORDER`,
		`CREATE OR ALTER SEQUENCE seq WITH START 1 INCREMENT 1`,
	)
	assertInvalid(t, (*Validator).ParseCreateSequence,
		`CREATE SEQUENCE`,
		`CREATE SEQUENCE seq START WITH`,
	)
}

func TestParseCreateService(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateService,
		`CREATE SERVICE svc IN COMPUTE POOL my_pool FROM SPECIFICATION 'spec text'`,
		`CREATE SERVICE IF NOT EXISTS svc IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = 'spec.yaml' MIN_INSTANCES = 1 MAX_INSTANCES = 3 AUTO_RESUME = TRUE`,
		`CREATE OR REPLACE SERVICE svc IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE_FILE = 't.yaml' USING (key => 'val') QUERY_WAREHOUSE = wh COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateService,
		`CREATE SERVICE svc`,
		`CREATE SERVICE svc IN COMPUTE POOL my_pool`,
	)
}

func TestParseCreateSessionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSessionPolicy,
		`CREATE SESSION POLICY sp`,
		`CREATE OR REPLACE SESSION POLICY sp SESSION_IDLE_TIMEOUT_MINS = 30 SESSION_UI_IDLE_TIMEOUT_MINS = 10 COMMENT = 'c'`,
		`CREATE SESSION POLICY IF NOT EXISTS sp ALLOWED_SECONDARY_ROLES = ('ALL')`,
	)
	assertInvalid(t, (*Validator).ParseCreateSessionPolicy,
		`CREATE SESSION sp`,
		`CREATE SESSION POLICY`,
	)
}

func TestParseCreateShare(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateShare,
		`CREATE SHARE sh`,
		`CREATE OR REPLACE SHARE IF NOT EXISTS sh COMMENT = 'c'`,
		`CREATE OR ALTER SHARE sh COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateShare,
		`CREATE SHARE`,
		`CREATE SHARE sh COMMENT 'c'`,
	)
}

func TestParseCreateSnapshot(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSnapshot,
		`CREATE SNAPSHOT snp FROM SERVICE svc VOLUME "vol" INSTANCE 0`,
		`CREATE OR REPLACE SNAPSHOT IF NOT EXISTS snp FROM SERVICE svc VOLUME "vol" INSTANCE 1 COMMENT = 'c'`,
		`CREATE SNAPSHOT snp FROM SERVICE svc VOLUME "vol" INSTANCE 2 TAG (t = 'v')`,
	)
	assertInvalid(t, (*Validator).ParseCreateSnapshot,
		`CREATE SNAPSHOT snp`,
		`CREATE SNAPSHOT snp FROM SERVICE svc VOLUME "vol"`,
	)
}

func TestParseCreateSnapshotPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSnapshotPolicy,
		`CREATE SNAPSHOT POLICY sp`,
		`CREATE OR REPLACE SNAPSHOT POLICY sp SCHEDULE = '60 MINUTE' EXPIRE_AFTER_DAYS = 7 COMMENT = 'c'`,
		`CREATE SNAPSHOT POLICY IF NOT EXISTS sp WITH RETENTION LOCK TAG (t = 'v')`,
	)
	assertInvalid(t, (*Validator).ParseCreateSnapshotPolicy,
		`CREATE SNAPSHOT sp`,
		`CREATE SNAPSHOT POLICY`,
	)
}

func TestParseCreateSnapshotSet(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateSnapshotSet,
		`CREATE SNAPSHOT SET ss FOR TABLE t`,
		`CREATE OR REPLACE SNAPSHOT SET IF NOT EXISTS ss FOR DYNAMIC TABLE t WITH SNAPSHOT POLICY p COMMENT = 'c'`,
		`CREATE SNAPSHOT SET ss FOR SCHEMA s TAG (t = 'v')`,
		`CREATE SNAPSHOT SET ss FOR DATABASE d`,
	)
	assertInvalid(t, (*Validator).ParseCreateSnapshotSet,
		`CREATE SNAPSHOT SET ss`,
		`CREATE SNAPSHOT SET ss FOR t`,
	)
}

func TestParseCreateStage(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateStage,
		`CREATE STAGE st`,
		`CREATE OR REPLACE TEMPORARY STAGE IF NOT EXISTS st URL = 's3://bucket/path' STORAGE_INTEGRATION = my_int`,
		`CREATE STAGE st FILE_FORMAT = (TYPE = CSV FIELD_DELIMITER = ',') COPY_OPTIONS = (ON_ERROR = 'CONTINUE') COMMENT = 'c'`,
		`CREATE STAGE st DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = FALSE) TAG (t = 'v')`,
	)
	assertInvalid(t, (*Validator).ParseCreateStage,
		`CREATE STAGE`,
		`CREATE STAGE st FILE_FORMAT = (TYPE = CSV`,
	)
}
