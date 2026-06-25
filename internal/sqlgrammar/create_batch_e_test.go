package sqlgrammar

import "testing"

func TestParseCreateNetworkRule(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNetworkRule,
		`CREATE NETWORK RULE my_rule TYPE = IPV4 VALUE_LIST = ('192.168.1.0/24') MODE = INGRESS`,
		`CREATE OR REPLACE NETWORK RULE db.sch.r TYPE = HOST_PORT VALUE_LIST = ('a.com:443', 'b.com:80') MODE = EGRESS COMMENT = 'x'`,
		`CREATE OR ALTER NETWORK RULE r MODE = INGRESS TYPE = IPV6 VALUE_LIST = ('::1')`,
	)
	assertInvalid(t, (*Validator).ParseCreateNetworkRule,
		`CREATE NETWORK RULE`,
		`CREATE NETWORK my_rule TYPE = IPV4 VALUE_LIST = ('x') MODE = INGRESS`,
		`NETWORK RULE r TYPE = IPV4 VALUE_LIST = ('x') MODE = INGRESS`,
	)
}

func TestParseCreateNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotebook,
		`CREATE NOTEBOOK my_nb`,
		`CREATE OR REPLACE NOTEBOOK IF NOT EXISTS db.sch.nb FROM '@db.sch.stage/path' MAIN_FILE = 'main.ipynb' QUERY_WAREHOUSE = wh`,
		`CREATE NOTEBOOK nb COMMENT = 'hi' IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 600 WAREHOUSE = my_wh`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotebook,
		`CREATE NOTEBOOK`,
		`CREATE OR REPLACE NOTEBOOK nb MAIN_FILE 'main.ipynb'`,
		`NOTEBOOK nb`,
		// Each order-independent option may appear at most once (unorderedOnce).
		`CREATE OR REPLACE NOTEBOOK db.sch.nb COMMENT = '' COMMENT = ''`,
		`CREATE NOTEBOOK nb WAREHOUSE = w1 WAREHOUSE = w2`,
	)
}

func TestParseCreateNotebookProject(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotebookProject,
		`CREATE NOTEBOOK PROJECT db.sch.proj FROM 'snow://workspace/ws_path'`,
		`CREATE NOTEBOOK PROJECT IF NOT EXISTS db.sch.proj FROM '@db.sch.stage' COMMENT = 'desc'`,
		`CREATE NOTEBOOK PROJECT p FROM 'snow://workspace/x'`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotebookProject,
		`CREATE NOTEBOOK PROJECT db.sch.proj`,
		`CREATE NOTEBOOK PROJECT FROM 'x'`,
		`CREATE NOTEBOOK proj FROM 'x'`,
	)
}

func TestParseCreateNotificationIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegration,
		`CREATE NOTIFICATION INTEGRATION ni TYPE = EMAIL ENABLED = TRUE`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE NOTIFICATION_PROVIDER = AWS_SNS AWS_SNS_TOPIC_ARN = 'arn'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegration,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION ni TYPE = EMAIL`,
		`NOTIFICATION INTEGRATION ni`,
	)
}

func TestParseCreateNotificationIntegrationEmail(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationEmail,
		`CREATE NOTIFICATION INTEGRATION ni TYPE = EMAIL ENABLED = TRUE`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = EMAIL ENABLED = FALSE ALLOWED_RECIPIENTS = ('a@x.com', 'b@x.com') DEFAULT_SUBJECT = 'Hi'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = EMAIL DEFAULT_RECIPIENTS = ('c@x.com') COMMENT = 'k'`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationEmail,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION ni TYPE = EMAIL ENABLED = TRUE`,
		`CREATE NOTIFICATION INTEGRATION ni TYPE EMAIL`,
	)
}

func TestParseCreateNotificationIntegrationInboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationInboundAzureEventGrid,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE AZURE_STORAGE_QUEUE_PRIMARY_URI = 'https://q' AZURE_TENANT_ID = 'tid'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE ENABLED = FALSE NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE AZURE_STORAGE_QUEUE_PRIMARY_URI = 'u' AZURE_TENANT_ID = 't' USE_PRIVATELINK_ENDPOINT = TRUE COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationInboundAzureEventGrid,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION INTEGRATION ni TYPE QUEUE`,
		`NOTIFICATION INTEGRATION ni ENABLED = TRUE`,
	)
}

func TestParseCreateNotificationIntegrationInboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationInboundGooglePubSub,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE NOTIFICATION_PROVIDER = GCP_PUBSUB GCP_PUBSUB_SUBSCRIPTION_NAME = 'sub'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE ENABLED = FALSE NOTIFICATION_PROVIDER = GCP_PUBSUB GCP_PUBSUB_SUBSCRIPTION_NAME = 's' COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationInboundGooglePubSub,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION ni ENABLED = TRUE`,
		`CREATE NOTIFICATION INTEGRATION ni GCP_PUBSUB_SUBSCRIPTION_NAME 'sub'`,
	)
}

func TestParseCreateNotificationIntegrationOutboundAmazonSns(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationOutboundAmazonSns,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE DIRECTION = OUTBOUND NOTIFICATION_PROVIDER = AWS_SNS AWS_SNS_TOPIC_ARN = 'arn' AWS_SNS_ROLE_ARN = 'role'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE DIRECTION = OUTBOUND ENABLED = TRUE NOTIFICATION_PROVIDER = AWS_SNS AWS_SNS_TOPIC_ARN = 'a' AWS_SNS_ROLE_ARN = 'r' COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni DIRECTION = OUTBOUND`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationOutboundAmazonSns,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION INTEGRATION ni AWS_SNS_TOPIC_ARN 'arn'`,
		`NOTIFICATION INTEGRATION ni`,
	)
}

func TestParseCreateNotificationIntegrationOutboundAzureEventGrid(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationOutboundAzureEventGrid,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE DIRECTION = OUTBOUND NOTIFICATION_PROVIDER = AZURE_EVENT_GRID AZURE_EVENT_GRID_TOPIC_ENDPOINT = 'ep' AZURE_TENANT_ID = 'tid'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE DIRECTION = OUTBOUND ENABLED = FALSE NOTIFICATION_PROVIDER = AZURE_EVENT_GRID AZURE_EVENT_GRID_TOPIC_ENDPOINT = 'e' AZURE_TENANT_ID = 't' COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationOutboundAzureEventGrid,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION INTEGRATION ni TYPE QUEUE`,
		`NOTIFICATION INTEGRATION ni`,
	)
}

func TestParseCreateNotificationIntegrationOutboundGooglePubSub(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationOutboundGooglePubSub,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = TRUE TYPE = QUEUE DIRECTION = OUTBOUND NOTIFICATION_PROVIDER = GCP_PUBSUB GCP_PUBSUB_TOPIC_NAME = 'topic'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = QUEUE DIRECTION = OUTBOUND ENABLED = TRUE NOTIFICATION_PROVIDER = GCP_PUBSUB GCP_PUBSUB_TOPIC_NAME = 't' COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationOutboundGooglePubSub,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION INTEGRATION ni GCP_PUBSUB_TOPIC_NAME 't'`,
		`NOTIFICATION INTEGRATION ni`,
	)
}

func TestParseCreateNotificationIntegrationWebhooks(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateNotificationIntegrationWebhooks,
		`CREATE NOTIFICATION INTEGRATION ni TYPE = WEBHOOK ENABLED = TRUE WEBHOOK_URL = 'https://x'`,
		`CREATE OR REPLACE NOTIFICATION INTEGRATION IF NOT EXISTS ni TYPE = WEBHOOK ENABLED = TRUE WEBHOOK_URL = 'u' WEBHOOK_SECRET = db.sch.s WEBHOOK_HEADERS = ('Content-Type' = 'application/json') COMMENT = 'c'`,
		`CREATE NOTIFICATION INTEGRATION ni ENABLED = FALSE TYPE = WEBHOOK WEBHOOK_URL = 'u' WEBHOOK_BODY_TEMPLATE = '{}'`,
	)
	assertInvalid(t, (*Validator).ParseCreateNotificationIntegrationWebhooks,
		`CREATE NOTIFICATION INTEGRATION`,
		`CREATE NOTIFICATION INTEGRATION ni WEBHOOK_URL 'u'`,
		`NOTIFICATION INTEGRATION ni TYPE = WEBHOOK`,
	)
}

func TestParseCreateOnlineFeatureTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOnlineFeatureTable,
		`CREATE ONLINE FEATURE TABLE oft PRIMARY KEY (id) TARGET_LAG = '1 minutes' WAREHOUSE = wh FROM src`,
		`CREATE OR REPLACE ONLINE FEATURE TABLE db.sch.oft PRIMARY KEY (a, b) TARGET_LAG = '1 hours' WAREHOUSE = wh REFRESH_MODE = FULL TIMESTAMP_COLUMN = ts WITH COMMENT = 'c' FROM db.sch.feat`,
		`CREATE ONLINE FEATURE TABLE oft WAREHOUSE = wh TARGET_LAG = '5 minutes' PRIMARY KEY (k) FROM source_view`,
	)
	assertInvalid(t, (*Validator).ParseCreateOnlineFeatureTable,
		`CREATE ONLINE FEATURE TABLE oft PRIMARY KEY (id) TARGET_LAG = '1 minutes' WAREHOUSE = wh`,
		`CREATE ONLINE FEATURE TABLE`,
		`CREATE FEATURE TABLE oft PRIMARY KEY (id) FROM src`,
	)
}

func TestParseCreateOrAlterObj(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrAlterObj,
		`CREATE OR ALTER TABLE t (id NUMBER)`,
		`CREATE OR ALTER TASK db.sch.tk WAREHOUSE = wh SCHEDULE = '1 minute'`,
		`CREATE OR ALTER NETWORK RULE r TYPE = IPV4 VALUE_LIST = ('x') MODE = INGRESS`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrAlterObj,
		`CREATE OR ALTER`,
		`CREATE ALTER TABLE t`,
		`CREATE OR ALTER TABLE`,
	)
}

func TestParseCreateOrganizationAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrganizationAccount,
		`CREATE ORGANIZATION ACCOUNT acct ADMIN_NAME = admin ADMIN_PASSWORD = 'pw' EMAIL = 'a@x.com' EDITION = ENTERPRISE`,
		`CREATE ORGANIZATION ACCOUNT acct ADMIN_NAME = admin ADMIN_RSA_PUBLIC_KEY = key FIRST_NAME = first LAST_NAME = last EMAIL = 'a@x.com' MUST_CHANGE_PASSWORD = TRUE EDITION = BUSINESS_CRITICAL REGION = aws_us_west_2 COMMENT = 'c'`,
		`CREATE ORGANIZATION ACCOUNT acct EMAIL = 'a@x.com' EDITION = ENTERPRISE ADMIN_NAME = admin ADMIN_PASSWORD = 'pw'`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrganizationAccount,
		`CREATE ORGANIZATION ACCOUNT`,
		`CREATE ORGANIZATION acct ADMIN_NAME = admin`,
		`CREATE ORGANIZATION ACCOUNT acct ADMIN_NAME admin`,
	)
}

func TestParseCreateOrganizationListing(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrganizationListing,
		`CREATE ORGANIZATION LISTING l AS 'manifest'`,
		`CREATE ORGANIZATION LISTING IF NOT EXISTS l SHARE my_share AS 'yaml' PUBLISH = TRUE`,
		`CREATE ORGANIZATION LISTING l APPLICATION PACKAGE pkg FROM '@stage/manifest.yml' PUBLISH = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrganizationListing,
		`CREATE ORGANIZATION LISTING l`,
		`CREATE ORGANIZATION LISTING`,
		`CREATE ORGANIZATION l AS 'm'`,
	)
}

func TestParseCreateOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateOrganizationProfile,
		`CREATE ORGANIZATION PROFILE p`,
		`CREATE ORGANIZATION PROFILE IF NOT EXISTS p AS 'yaml' VERSION v1 PUBLISH = TRUE`,
		`CREATE ORGANIZATION PROFILE p FROM @db.sch.stage VERSION v2 PUBLISH = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCreateOrganizationProfile,
		`CREATE ORGANIZATION PROFILE`,
		`CREATE ORGANIZATION p`,
		`CREATE ORGANIZATION PROFILE p AS`,
	)
}
