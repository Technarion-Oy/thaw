package sqlgrammar

import "testing"

func TestParseAlterNotificationIntegrationEmail(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationEmail,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int SET DEFAULT_SUBJECT = 'hi'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ALLOWED_RECIPIENTS = ('a@b.com') COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET TAG t1 = 'v1'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET TAG t1, t2`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationEmail,
		``,
		`DROP NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET BOGUS`,
	)
}

func TestParseAlterNotificationIntegrationInboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationInboundAzureEventGrid,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = FALSE`,
		`ALTER INTEGRATION my_int SET COMMENT = 'x'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET TAG t1 = 'v1'`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationInboundAzureEventGrid,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int SET BOGUS = 'x'`,
	)
}

func TestParseAlterNotificationIntegrationInboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationInboundGooglePubSub,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION my_int SET COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = FALSE COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationInboundGooglePubSub,
		``,
		`SHOW INTEGRATIONS`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET BOGUS`,
	)
}

func TestParseAlterNotificationIntegrationOutboundAmazonSns(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAmazonSns,
		`ALTER NOTIFICATION INTEGRATION my_int SET AWS_SNS_TOPIC_ARN = 'arn'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE AWS_SNS_ROLE_ARN = 'r'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET AWS_SNS_TOPIC_ARN = 'arn' AWS_SNS_ROLE_ARN = 'r' COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET TAG t1 = 'v1'`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAmazonSns,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int SET AWS_SNS_TOPIC_ARN`,
	)
}

func TestParseAlterNotificationIntegrationOutboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAzureEventGrid,
		`ALTER NOTIFICATION INTEGRATION my_int SET AZURE_TENANT_ID = 'id'`,
		`ALTER INTEGRATION my_int SET COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET AZURE_STORAGE_QUEUE_PRIMARY_URI = 'u' AZURE_TENANT_ID = 'id'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAzureEventGrid,
		``,
		`CREATE NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int SET BOGUS = 'x'`,
	)
}

func TestParseAlterNotificationIntegrationOutboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundGooglePubSub,
		`ALTER NOTIFICATION INTEGRATION my_int SET GCP_PUBSUB_SUBSCRIPTION_NAME = 'sub'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE GCP_PUBSUB_SUBSCRIPTION_NAME = 'sub' COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET TAG t1 = 'v1'`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundGooglePubSub,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int SET BOGUS = 'x'`,
	)
}

func TestParseAlterNotificationIntegrationWebhooks(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationWebhooks,
		`ALTER NOTIFICATION INTEGRATION my_int SET WEBHOOK_URL = 'https://x'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET WEBHOOK_SECRET`,
		`ALTER NOTIFICATION INTEGRATION my_int SET WEBHOOK_SECRET = my_secret WEBHOOK_HEADERS = ('h1'='v1') COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationWebhooks,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int SET`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET WEBHOOK_URL`,
	)
}

func TestParseAlterOpenflowDataPlane(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOpenflowDataPlane,
		`ALTER OPENFLOW DATA PLANE INTEGRATION my_dp SET EVENT_TABLE = 'db.sch.tbl'`,
		`ALTER OPENFLOW DATA PLANE INTEGRATION db.sch.my_dp SET EVENT_TABLE = my_tbl`,
		`ALTER OPENFLOW DATA PLANE INTEGRATION my_dp SET EVENT_TABLE = 'evt'`,
	)
	assertInvalid(t, (*Validator).ParseAlterOpenflowDataPlane,
		``,
		`ALTER OPENFLOW DATA PLANE INTEGRATION my_dp`,
		`ALTER OPENFLOW INTEGRATION my_dp SET EVENT_TABLE = 'x'`,
	)
}

func TestParseAlterOnlineFeatureTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOnlineFeatureTable,
		`ALTER ONLINE FEATURE TABLE my_t SUSPEND`,
		`ALTER ONLINE FEATURE TABLE IF EXISTS my_t RENAME TO new_t`,
		`ALTER ONLINE FEATURE TABLE my_t SET TARGET_LAG = '1 hours'`,
		`ALTER ONLINE FEATURE TABLE my_t REFRESH`,
		`ALTER ONLINE FEATURE TABLE my_t SET WAREHOUSE = wh COMMENT = 'c'`,
		`ALTER ONLINE FEATURE TABLE my_t SET TAG t = 'v'`,
		`ALTER ONLINE FEATURE TABLE my_t UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterOnlineFeatureTable,
		``,
		`ALTER ONLINE FEATURE TABLE my_t`,
		// Newly enforced: ungated catch-all removed, so a garbage action is flagged.
		`ALTER ONLINE FEATURE TABLE my_t FOOBAR x`,
		`ALTER ONLINE FEATURE TABLE my_t SET BOGUS = 'x'`,
		`ALTER ONLINE FEATURE TABLE my_t SET`,
	)
}

func TestParseAlterOrganizationAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationAccount,
		`ALTER ORGANIZATION ACCOUNT SET RESOURCE_MONITOR = mon`,
		`ALTER ORGANIZATION ACCOUNT UNSET TIMEZONE`,
		`ALTER ORGANIZATION ACCOUNT SET TAG t = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationAccount,
		``,
		`ALTER ORGANIZATION ACCOUNT`,
	)
}

func TestParseAlterOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationProfile,
		`ALTER ORGANIZATION PROFILE my_p AS 'yaml'`,
		`ALTER ORGANIZATION PROFILE IF EXISTS my_p RENAME TO new_p`,
		`ALTER ORGANIZATION PROFILE my_p PUBLISH`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationProfile,
		``,
		`ALTER ORGANIZATION PROFILE my_p`,
	)
}

func TestParseAlterOrganizationUser(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationUser,
		`ALTER ORGANIZATION USER bob SET EMAIL = 'a@b.com'`,
		`ALTER ORGANIZATION USER IF EXISTS bob SET FIRST_NAME = 'Bob'`,
		`ALTER ORGANIZATION USER bob UNSET COMMENT`,
		`ALTER ORGANIZATION USER bob SET DISPLAY_NAME = 'B' LAST_NAME = 'Z'`,
		`ALTER ORGANIZATION USER bob UNSET EMAIL`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationUser,
		``,
		`ALTER ORGANIZATION USER bob`,
		`ALTER ORGANIZATION USER bob SET BOGUS = 'x'`,
		`ALTER ORGANIZATION USER bob SET EMAIL = 5`,
	)
}

func TestParseAlterOrganizationUserGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationUserGroup,
		`ALTER ORGANIZATION USER GROUP g ADD ORGANIZATION USERS bob`,
		`ALTER ORGANIZATION USER GROUP IF EXISTS g REMOVE ORGANIZATION USERS bob, alice`,
		`ALTER ORGANIZATION USER GROUP g SET IS_GRANTABLE = TRUE`,
		`ALTER ORGANIZATION USER GROUP g SET VISIBILITY = ALL`,
		`ALTER ORGANIZATION USER GROUP g SET VISIBILITY = ACCOUNTS a1, a2`,
		`ALTER ORGANIZATION USER GROUP g SET VISIBILITY = REGION GROUPS 'r1', 'r2'`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationUserGroup,
		``,
		`ALTER ORGANIZATION USER GROUP g`,
		`ALTER ORGANIZATION USER GROUP g SET IS_GRANTABLE = 5`,
		`ALTER ORGANIZATION USER GROUP g ADD USERS bob`,
	)
}

func TestParseAlterPackagesPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPackagesPolicy,
		`ALTER PACKAGES POLICY p SET COMMENT = 'c'`,
		`ALTER PACKAGES POLICY IF EXISTS p SET ALLOWLIST = ('numpy')`,
		`ALTER PACKAGES POLICY p UNSET COMMENT`,
		`ALTER PACKAGES POLICY p SET BLOCKLIST = ('a','b') COMMENT = 'c'`,
		`ALTER PACKAGES POLICY p UNSET ALLOWLIST, BLOCKLIST`,
	)
	assertInvalid(t, (*Validator).ParseAlterPackagesPolicy,
		``,
		`ALTER PACKAGES POLICY p`,
		`ALTER PACKAGES POLICY p SET BOGUS = ('x')`,
		`ALTER PACKAGES POLICY p UNSET BOGUS`,
	)
}

func TestParseAlterPasswordPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPasswordPolicy,
		`ALTER PASSWORD POLICY p RENAME TO p2`,
		`ALTER PASSWORD POLICY IF EXISTS p SET PASSWORD_MIN_LENGTH = 10`,
		`ALTER PASSWORD POLICY p UNSET COMMENT`,
		`ALTER PASSWORD POLICY p SET PASSWORD_MIN_LENGTH = 8 PASSWORD_MAX_AGE_DAYS = 30 COMMENT = 'c'`,
		`ALTER PASSWORD POLICY p SET TAG t = 'v'`,
		`ALTER PASSWORD POLICY p UNSET PASSWORD_HISTORY, COMMENT`,
		`ALTER PASSWORD POLICY p UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterPasswordPolicy,
		``,
		`ALTER PASSWORD POLICY p`,
		`ALTER PASSWORD POLICY p SET PASSWORD_MIN_LENGTH = 'x'`,
		`ALTER PASSWORD POLICY p SET BOGUS = 5`,
	)
}

func TestParseAlterPipe(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPipe,
		`ALTER PIPE p SET PIPE_EXECUTION_PAUSED = TRUE`,
		`ALTER PIPE IF EXISTS p REFRESH`,
		`ALTER PIPE p REFRESH PREFIX = 'x/'`,
		`ALTER PIPE p REFRESH PREFIX = 'x/' MODIFIED_AFTER = '2020-01-01'`,
		`ALTER PIPE p SET TAG t = 'v'`,
		`ALTER PIPE p UNSET PIPE_EXECUTION_PAUSED`,
		`ALTER PIPE p SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterPipe,
		``,
		`ALTER PIPE p`,
		`ALTER PIPE p SET BOGUS = TRUE`,
		`ALTER PIPE p UNSET BOGUS`,
	)
}

func TestParseAlterPostgresInstance(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPostgresInstance,
		`ALTER POSTGRES INSTANCE pg RENAME TO pg2`,
		`ALTER POSTGRES INSTANCE IF EXISTS pg SET STORAGE_SIZE_GB = 100`,
		`ALTER POSTGRES INSTANCE pg SUSPEND`,
		`ALTER POSTGRES INSTANCE pg SET AUTHENTICATION_AUTHORITY = POSTGRES HIGH_AVAILABILITY = TRUE`,
		`ALTER POSTGRES INSTANCE pg SET APPLY IMMEDIATELY`,
		`ALTER POSTGRES INSTANCE pg RESET ACCESS FOR 'application'`,
		`ALTER POSTGRES INSTANCE pg UNSET COMMENT, NETWORK_POLICY`,
		`ALTER POSTGRES INSTANCE pg SET TAG t = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterPostgresInstance,
		``,
		`ALTER POSTGRES INSTANCE pg`,
		`ALTER POSTGRES INSTANCE pg SET BOGUS = 1`,
		`ALTER POSTGRES INSTANCE pg UNSET BOGUS`,
	)
}

func TestParseAlterPrivacyPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPrivacyPolicy,
		`ALTER PRIVACY POLICY p RENAME TO p2`,
		`ALTER PRIVACY POLICY p SET BODY -> TRUE`,
		`ALTER PRIVACY POLICY IF EXISTS p SET COMMENT = 'c'`,
		`ALTER PRIVACY POLICY p SET TAG t = 'v'`,
		`ALTER PRIVACY POLICY p UNSET COMMENT`,
		`ALTER PRIVACY POLICY p UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterPrivacyPolicy,
		``,
		`ALTER PRIVACY POLICY p`,
		`ALTER PRIVACY POLICY p SET BOGUS = 'c'`,
		`ALTER PRIVACY POLICY p UNSET BOGUS`,
	)
}

func TestParseAlterProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterProcedure,
		`ALTER PROCEDURE p(NUMBER) RENAME TO p2`,
		`ALTER PROCEDURE IF EXISTS p(VARCHAR, NUMBER) SET LOG_LEVEL = 'INFO'`,
		`ALTER PROCEDURE p() EXECUTE AS CALLER`,
		`ALTER PROCEDURE p() EXECUTE AS RESTRICTED CALLER`,
		`ALTER PROCEDURE p() SET COMMENT = 'c'`,
		`ALTER PROCEDURE p() SET TAG t = 'v'`,
		`ALTER PROCEDURE p() UNSET COMMENT`,
		`ALTER PROCEDURE p() SET SECRETS = 'cred' = my_secret`,
	)
	assertInvalid(t, (*Validator).ParseAlterProcedure,
		``,
		`ALTER PROCEDURE p RENAME TO p2`,
		`ALTER PROCEDURE p() SET BOGUS = 'x'`,
		`ALTER PROCEDURE p() EXECUTE AS NOBODY`,
	)
}

func TestParseAlterProjectionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterProjectionPolicy,
		`ALTER PROJECTION POLICY p RENAME TO p2`,
		`ALTER PROJECTION POLICY p SET BODY -> PROJECTION_CONSTRAINT(ALLOW = true)`,
		`ALTER PROJECTION POLICY IF EXISTS p SET COMMENT = 'c'`,
		`ALTER PROJECTION POLICY p SET TAG t = 'v'`,
		`ALTER PROJECTION POLICY p UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterProjectionPolicy,
		``,
		`ALTER PROJECTION POLICY p`,
		`ALTER PROJECTION POLICY p SET BOGUS = 'c'`,
		`ALTER PROJECTION POLICY p UNSET BOGUS`,
	)
}

func TestParseAlterReplicationGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterReplicationGroup,
		`ALTER REPLICATION GROUP rg RENAME TO rg2`,
		`ALTER REPLICATION GROUP IF EXISTS rg SUSPEND IMMEDIATE`,
		`ALTER REPLICATION GROUP rg ADD db1 TO ALLOWED_DATABASES`,
	)
	assertInvalid(t, (*Validator).ParseAlterReplicationGroup,
		``,
		`ALTER REPLICATION GROUP rg`,
	)
}

func TestParseAlterResourceMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterResourceMonitor,
		`ALTER RESOURCE MONITOR rm SET CREDIT_QUOTA = 100`,
		`ALTER RESOURCE MONITOR IF EXISTS rm TRIGGERS ON 80 PERCENT DO SUSPEND`,
		`ALTER RESOURCE MONITOR rm`,
		`ALTER RESOURCE MONITOR rm SET FREQUENCY = DAILY START_TIMESTAMP = IMMEDIATELY`,
		`ALTER RESOURCE MONITOR rm SET NOTIFY_USERS = (u1, u2)`,
		`ALTER RESOURCE MONITOR rm SET CREDIT_QUOTA = 50 TRIGGERS ON 90 PERCENT DO NOTIFY ON 100 PERCENT DO SUSPEND_IMMEDIATE`,
	)
	assertInvalid(t, (*Validator).ParseAlterResourceMonitor,
		``,
		`ALTER MONITOR rm`,
		`ALTER RESOURCE MONITOR rm SET BOGUS = 1`,
		`ALTER RESOURCE MONITOR rm TRIGGERS ON 80 PERCENT DO EXPLODE`,
	)
}

func TestParseAlterRole(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterRole,
		`ALTER ROLE r RENAME TO r2`,
		`ALTER ROLE IF EXISTS r SET COMMENT = 'c'`,
		`ALTER ROLE r UNSET COMMENT`,
		`ALTER ROLE r SET TAG t = 'v'`,
		`ALTER ROLE r UNSET TAG t`,
		`ALTER ROLE r UNSET DCM PROJECT`,
	)
	assertInvalid(t, (*Validator).ParseAlterRole,
		``,
		`ALTER ROLE r`,
		`ALTER ROLE r SET BOGUS = 'c'`,
		`ALTER ROLE r UNSET BOGUS`,
	)
}

func TestParseAlterRowAccessPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterRowAccessPolicy,
		`ALTER ROW ACCESS POLICY p RENAME TO p2`,
		`ALTER ROW ACCESS POLICY p SET BODY -> TRUE`,
		`ALTER ROW ACCESS POLICY IF EXISTS p SET COMMENT = 'c'`,
		`ALTER ROW ACCESS POLICY p SET TAG t1 = 'v1', t2 = 'v2'`,
		`ALTER ROW ACCESS POLICY p UNSET TAG t1, t2`,
		`ALTER ROW ACCESS POLICY p UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterRowAccessPolicy,
		``,
		`ALTER ROW ACCESS POLICY p`,
		`ALTER ROW ACCESS POLICY p SET NONSENSE = 'x'`,
		`ALTER ROW ACCESS POLICY p UNSET ENABLED`,
	)
}

func TestParseAlterSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSchema,
		`ALTER SCHEMA s RENAME TO s2`,
		`ALTER SCHEMA IF EXISTS s SWAP WITH s3`,
		`ALTER SCHEMA s ENABLE MANAGED ACCESS`,
		`ALTER SCHEMA s SET DATA_RETENTION_TIME_IN_DAYS = 7 COMMENT = 'c'`,
		`ALTER SCHEMA s SET STORAGE_SERIALIZATION_POLICY = OPTIMIZED`,
		`ALTER SCHEMA s SET TAG t1 = 'v1', t2 = 'v2'`,
		`ALTER SCHEMA s UNSET DATA_RETENTION_TIME_IN_DAYS, COMMENT`,
		`ALTER SCHEMA s UNSET DCM PROJECT`,
		`ALTER SCHEMA s UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterSchema,
		``,
		`ALTER SCHEMA s`,
		`ALTER SCHEMA s SET BOGUS_OPTION = 1`,
		`ALTER SCHEMA s UNSET BOGUS_OPTION`,
	)
}

func TestParseAlterSecret(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecret,
		`ALTER SECRET s SET COMMENT = 'c'`,
		`ALTER SECRET IF EXISTS s SET USERNAME = 'u' PASSWORD = 'p'`,
		`ALTER SECRET s UNSET COMMENT`,
		`ALTER SECRET s SET OAUTH_SCOPES = ('a', 'b') COMMENT = 'c'`,
		`ALTER SECRET s SET SECRET_STRING = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecret,
		``,
		`ALTER SECRET s`,
		`ALTER SECRET s SET BOGUS = 'x'`,
		`ALTER SECRET s UNSET USERNAME`,
	)
}

func TestParseAlterSecurityIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegration,
		`ALTER SECURITY INTEGRATION si SET ENABLED = TRUE`,
		`ALTER INTEGRATION IF EXISTS si UNSET ENABLED`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegration,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationExternalApiAuthentication(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationExternalApiAuthentication,
		`ALTER SECURITY INTEGRATION si SET OAUTH_CLIENT_ID = 'id'`,
		`ALTER SECURITY INTEGRATION IF EXISTS si UNSET ENABLED`,
		`ALTER INTEGRATION si SET ENABLED = TRUE`,
		`ALTER SECURITY INTEGRATION si SET OAUTH_CLIENT_AUTH_METHOD = CLIENT_SECRET_POST OAUTH_ACCESS_TOKEN_VALIDITY = 3600`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
		`ALTER INTEGRATION si UNSET TAG t1, t2`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationExternalApiAuthentication,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si UNSET OAUTH_CLIENT_ID`,
	)
}

func TestParseAlterSecurityIntegrationAwsIamAuthentication(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationAwsIamAuthentication,
		`ALTER SECURITY INTEGRATION si SET AWS_ROLE_ARN = 'arn'`,
		`ALTER SECURITY INTEGRATION IF EXISTS si SET ENABLED = TRUE`,
		`ALTER INTEGRATION si UNSET TAG t`,
		`ALTER SECURITY INTEGRATION si SET TYPE = AWS_IAM AWS_ROLE_ARN = 'arn' ENABLED = TRUE`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
		`ALTER SECURITY INTEGRATION si UNSET ENABLED, COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationAwsIamAuthentication,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si UNSET AWS_ROLE_ARN`,
	)
}

func TestParseAlterSecurityIntegrationExternalOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationExternalOauth,
		`ALTER SECURITY INTEGRATION si SET EXTERNAL_OAUTH_ISSUER = 'iss'`,
		`ALTER SECURITY INTEGRATION IF EXISTS si UNSET ENABLED`,
		`ALTER INTEGRATION si SET ENABLED = TRUE`,
		`ALTER SECURITY INTEGRATION si SET EXTERNAL_OAUTH_TYPE = OKTA EXTERNAL_OAUTH_ANY_ROLE_MODE = ENABLE`,
		`ALTER SECURITY INTEGRATION si SET EXTERNAL_OAUTH_BLOCKED_ROLES_LIST = ('a', 'b')`,
		`ALTER SECURITY INTEGRATION si UNSET ENABLED, EXTERNAL_OAUTH_AUDIENCE_LIST`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationExternalOauth,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si UNSET EXTERNAL_OAUTH_ISSUER`,
	)
}

func TestParseAlterSecurityIntegrationSnowflakeOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationSnowflakeOauth,
		`ALTER SECURITY INTEGRATION si SET ENABLED = TRUE`,
		`ALTER SECURITY INTEGRATION IF EXISTS si REFRESH OAUTH_CLIENT_SECRET`,
		`ALTER INTEGRATION si REFRESH OAUTH_CLIENT_SECRET_2`,
		`ALTER SECURITY INTEGRATION si SET OAUTH_USE_SECONDARY_ROLES = IMPLICIT BLOCKED_ROLES_LIST = ('r1')`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
		`ALTER SECURITY INTEGRATION si UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationSnowflakeOauth,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si REFRESH BOGUS`,
	)
}

func TestParseAlterSecurityIntegrationSaml2(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationSaml2,
		`ALTER SECURITY INTEGRATION si SET SAML2_ISSUER = 'iss'`,
		`ALTER SECURITY INTEGRATION si REFRESH SAML2_SNOWFLAKE_PRIVATE_KEY`,
		`ALTER INTEGRATION si REFRESH METADATA_URL`,
		`ALTER SECURITY INTEGRATION si SET TYPE = SAML2 ENABLED = TRUE SAML2_SIGN_REQUEST = FALSE`,
		`ALTER SECURITY INTEGRATION si SET ALLOWED_USER_DOMAINS = ('a.com', 'b.com')`,
		`ALTER SECURITY INTEGRATION si UNSET ENABLED`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationSaml2,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si UNSET SAML2_ISSUER`,
	)
}

func TestParseAlterSecurityIntegrationScim(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationScim,
		`ALTER SECURITY INTEGRATION si SET SYNC_PASSWORD = TRUE`,
		`ALTER SECURITY INTEGRATION IF EXISTS si UNSET NETWORK_POLICY`,
		`ALTER INTEGRATION si SET ENABLED = FALSE`,
		`ALTER SECURITY INTEGRATION si SET REJECT_TOKENS_ISSUED_BEFORE = '2020-01-01' COMMENT = 'c'`,
		`ALTER SECURITY INTEGRATION si SET TAG t = 'v'`,
		`ALTER SECURITY INTEGRATION si UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationScim,
		``,
		`ALTER SECURITY INTEGRATION si`,
		`ALTER SECURITY INTEGRATION si SET BOGUS = 'x'`,
		`ALTER SECURITY INTEGRATION si UNSET ENABLED`,
	)
}

func TestParseAlterSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSemanticView,
		`ALTER SEMANTIC VIEW sv RENAME TO sv2`,
		`ALTER SEMANTIC VIEW IF EXISTS sv SET COMMENT = 'c'`,
		`ALTER SEMANTIC VIEW sv UNSET COMMENT`,
		`ALTER SEMANTIC VIEW sv SET TAG t1 = 'v1', t2 = 'v2'`,
		`ALTER SEMANTIC VIEW sv UNSET TAG t1, t2`,
	)
	assertInvalid(t, (*Validator).ParseAlterSemanticView,
		``,
		`ALTER SEMANTIC VIEW sv`,
		`ALTER SEMANTIC VIEW sv SET BOGUS = 'x'`,
		`ALTER SEMANTIC VIEW sv UNSET ENABLED`,
	)
}
