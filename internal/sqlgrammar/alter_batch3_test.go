package sqlgrammar

import "testing"

func TestParseAlterNotificationIntegrationEmail(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationEmail,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int SET DEFAULT_SUBJECT = 'hi'`,
		`ALTER NOTIFICATION INTEGRATION my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationEmail,
		``,
		`DROP NOTIFICATION INTEGRATION my_int`,
		`ALTER NOTIFICATION INTEGRATION my_int`,
	)
}

func TestParseAlterNotificationIntegrationInboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationInboundAzureEventGrid,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = FALSE`,
		`ALTER INTEGRATION my_int SET COMMENT = 'x'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationInboundAzureEventGrid,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
	)
}

func TestParseAlterNotificationIntegrationInboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationInboundGooglePubSub,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION my_int SET COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationInboundGooglePubSub,
		``,
		`SHOW INTEGRATIONS`,
	)
}

func TestParseAlterNotificationIntegrationOutboundAmazonSns(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAmazonSns,
		`ALTER NOTIFICATION INTEGRATION my_int SET AWS_SNS_TOPIC_ARN = 'arn'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE AWS_SNS_ROLE_ARN = 'r'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAmazonSns,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
	)
}

func TestParseAlterNotificationIntegrationOutboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAzureEventGrid,
		`ALTER NOTIFICATION INTEGRATION my_int SET AZURE_TENANT_ID = 'id'`,
		`ALTER INTEGRATION my_int SET COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundAzureEventGrid,
		``,
		`CREATE NOTIFICATION INTEGRATION my_int`,
	)
}

func TestParseAlterNotificationIntegrationOutboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationOutboundGooglePubSub,
		`ALTER NOTIFICATION INTEGRATION my_int SET GCP_PUBSUB_SUBSCRIPTION_NAME = 'sub'`,
		`ALTER NOTIFICATION INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationOutboundGooglePubSub,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
	)
}

func TestParseAlterNotificationIntegrationWebhooks(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegrationWebhooks,
		`ALTER NOTIFICATION INTEGRATION my_int SET WEBHOOK_URL = 'https://x'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int SET ENABLED = TRUE`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_int UNSET WEBHOOK_SECRET`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegrationWebhooks,
		``,
		`ALTER NOTIFICATION INTEGRATION my_int`,
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
	)
	assertInvalid(t, (*Validator).ParseAlterOnlineFeatureTable,
		``,
		`ALTER ONLINE FEATURE TABLE my_t`,
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
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationUser,
		``,
		`ALTER ORGANIZATION USER bob`,
	)
}

func TestParseAlterOrganizationUserGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationUserGroup,
		`ALTER ORGANIZATION USER GROUP g ADD ORGANIZATION USERS bob`,
		`ALTER ORGANIZATION USER GROUP IF EXISTS g REMOVE ORGANIZATION USERS bob, alice`,
		`ALTER ORGANIZATION USER GROUP g SET IS_GRANTABLE = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationUserGroup,
		``,
		`ALTER ORGANIZATION USER GROUP g`,
	)
}

func TestParseAlterPackagesPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPackagesPolicy,
		`ALTER PACKAGES POLICY p SET COMMENT = 'c'`,
		`ALTER PACKAGES POLICY IF EXISTS p SET ALLOWLIST = ('numpy')`,
		`ALTER PACKAGES POLICY p UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterPackagesPolicy,
		``,
		`ALTER PACKAGES POLICY p`,
	)
}

func TestParseAlterPasswordPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPasswordPolicy,
		`ALTER PASSWORD POLICY p RENAME TO p2`,
		`ALTER PASSWORD POLICY IF EXISTS p SET PASSWORD_MIN_LENGTH = 10`,
		`ALTER PASSWORD POLICY p UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterPasswordPolicy,
		``,
		`ALTER PASSWORD POLICY p`,
	)
}

func TestParseAlterPipe(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPipe,
		`ALTER PIPE p SET PIPE_EXECUTION_PAUSED = TRUE`,
		`ALTER PIPE IF EXISTS p REFRESH`,
		`ALTER PIPE p REFRESH PREFIX = 'x/'`,
	)
	assertInvalid(t, (*Validator).ParseAlterPipe,
		``,
		`ALTER PIPE p`,
	)
}

func TestParseAlterPostgresInstance(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPostgresInstance,
		`ALTER POSTGRES INSTANCE pg RENAME TO pg2`,
		`ALTER POSTGRES INSTANCE IF EXISTS pg SET STORAGE_SIZE_GB = 100`,
		`ALTER POSTGRES INSTANCE pg SUSPEND`,
	)
	assertInvalid(t, (*Validator).ParseAlterPostgresInstance,
		``,
		`ALTER POSTGRES INSTANCE pg`,
	)
}

func TestParseAlterPrivacyPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterPrivacyPolicy,
		`ALTER PRIVACY POLICY p RENAME TO p2`,
		`ALTER PRIVACY POLICY p SET BODY -> TRUE`,
		`ALTER PRIVACY POLICY IF EXISTS p SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterPrivacyPolicy,
		``,
		`ALTER PRIVACY POLICY p`,
	)
}

func TestParseAlterProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterProcedure,
		`ALTER PROCEDURE p(NUMBER) RENAME TO p2`,
		`ALTER PROCEDURE IF EXISTS p(VARCHAR, NUMBER) SET LOG_LEVEL = 'INFO'`,
		`ALTER PROCEDURE p() EXECUTE AS CALLER`,
	)
	assertInvalid(t, (*Validator).ParseAlterProcedure,
		``,
		`ALTER PROCEDURE p RENAME TO p2`,
	)
}

func TestParseAlterProjectionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterProjectionPolicy,
		`ALTER PROJECTION POLICY p RENAME TO p2`,
		`ALTER PROJECTION POLICY p SET BODY -> PROJECTION_CONSTRAINT(ALLOW = true)`,
		`ALTER PROJECTION POLICY IF EXISTS p SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterProjectionPolicy,
		``,
		`ALTER PROJECTION POLICY p`,
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
	)
	assertInvalid(t, (*Validator).ParseAlterResourceMonitor,
		``,
		`ALTER MONITOR rm`,
	)
}

func TestParseAlterRole(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterRole,
		`ALTER ROLE r RENAME TO r2`,
		`ALTER ROLE IF EXISTS r SET COMMENT = 'c'`,
		`ALTER ROLE r UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterRole,
		``,
		`ALTER ROLE r`,
	)
}

func TestParseAlterRowAccessPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterRowAccessPolicy,
		`ALTER ROW ACCESS POLICY p RENAME TO p2`,
		`ALTER ROW ACCESS POLICY p SET BODY -> TRUE`,
		`ALTER ROW ACCESS POLICY IF EXISTS p SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterRowAccessPolicy,
		``,
		`ALTER ROW ACCESS POLICY p`,
	)
}

func TestParseAlterSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSchema,
		`ALTER SCHEMA s RENAME TO s2`,
		`ALTER SCHEMA IF EXISTS s SWAP WITH s3`,
		`ALTER SCHEMA s ENABLE MANAGED ACCESS`,
	)
	assertInvalid(t, (*Validator).ParseAlterSchema,
		``,
		`ALTER SCHEMA s`,
	)
}

func TestParseAlterSecret(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecret,
		`ALTER SECRET s SET COMMENT = 'c'`,
		`ALTER SECRET IF EXISTS s SET USERNAME = 'u' PASSWORD = 'p'`,
		`ALTER SECRET s UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecret,
		``,
		`ALTER SECRET s`,
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
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationExternalApiAuthentication,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationAwsIamAuthentication(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationAwsIamAuthentication,
		`ALTER SECURITY INTEGRATION si SET AWS_ROLE_ARN = 'arn'`,
		`ALTER SECURITY INTEGRATION IF EXISTS si SET ENABLED = TRUE`,
		`ALTER INTEGRATION si UNSET TAG t`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationAwsIamAuthentication,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationExternalOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationExternalOauth,
		`ALTER SECURITY INTEGRATION si SET EXTERNAL_OAUTH_ISSUER = 'iss'`,
		`ALTER SECURITY INTEGRATION IF EXISTS si UNSET ENABLED`,
		`ALTER INTEGRATION si SET ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationExternalOauth,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationSnowflakeOauth(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationSnowflakeOauth,
		`ALTER SECURITY INTEGRATION si SET ENABLED = TRUE`,
		`ALTER SECURITY INTEGRATION IF EXISTS si REFRESH OAUTH_CLIENT_SECRET`,
		`ALTER INTEGRATION si REFRESH OAUTH_CLIENT_SECRET_2`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationSnowflakeOauth,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationSaml2(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationSaml2,
		`ALTER SECURITY INTEGRATION si SET SAML2_ISSUER = 'iss'`,
		`ALTER SECURITY INTEGRATION si REFRESH SAML2_SNOWFLAKE_PRIVATE_KEY`,
		`ALTER INTEGRATION si REFRESH METADATA_URL`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationSaml2,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSecurityIntegrationScim(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSecurityIntegrationScim,
		`ALTER SECURITY INTEGRATION si SET SYNC_PASSWORD = TRUE`,
		`ALTER SECURITY INTEGRATION IF EXISTS si UNSET NETWORK_POLICY`,
		`ALTER INTEGRATION si SET ENABLED = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseAlterSecurityIntegrationScim,
		``,
		`ALTER SECURITY INTEGRATION si`,
	)
}

func TestParseAlterSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSemanticView,
		`ALTER SEMANTIC VIEW sv RENAME TO sv2`,
		`ALTER SEMANTIC VIEW IF EXISTS sv SET COMMENT = 'c'`,
		`ALTER SEMANTIC VIEW sv UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSemanticView,
		``,
		`ALTER SEMANTIC VIEW sv`,
	)
}
