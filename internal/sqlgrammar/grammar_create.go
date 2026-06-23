package sqlgrammar

// CREATE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCreateObj validates the Snowflake `CREATE <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateObj() bool {
	return true
}

// ParseCreateAccount validates the Snowflake `CREATE ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-account
//
// Syntax:
//
//	CREATE ACCOUNT <name>
//	      ADMIN_NAME = '<string_literal>'
//	    { ADMIN_PASSWORD = '<string_literal>' | ADMIN_RSA_PUBLIC_KEY = '<string_literal>' }
//	    [ ADMIN_USER_TYPE = { PERSON | SERVICE | LEGACY_SERVICE | NULL } ]
//	    [ FIRST_NAME = '<string_literal>' ]
//	    [ LAST_NAME = '<string_literal>' ]
//	      EMAIL = '<string_literal>'
//	    [ MUST_CHANGE_PASSWORD = { TRUE | FALSE } ]
//	      EDITION = { STANDARD | ENTERPRISE | BUSINESS_CRITICAL }
//	    [ REGION_GROUP = <region_group_id> ]
//	    [ REGION = <snowflake_region_id> ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ POLARIS = { TRUE | FALSE } ]
func (v *Validator) ParseCreateAccount() bool {
	return true
}

// ParseCreateAgent validates the Snowflake `CREATE AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-agent
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AGENT [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<comment>' ]
//	  [ PROFILE = '<profile_object>' ]
//	  FROM SPECIFICATION
//	  $$
//	  <specification_object>
//	  $$;
func (v *Validator) ParseCreateAgent() bool {
	return true
}

// ParseCreateAggregationPolicy validates the Snowflake `CREATE AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-aggregation-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AGGREGATION POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS AGGREGATION_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateAggregationPolicy() bool {
	return true
}

// ParseCreateAlert validates the Snowflake `CREATE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-alert
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ALERT [ IF NOT EXISTS ] <name>
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	  [ WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ CONFIG = '<configuration_string>' ]
//	  [ RUNBOOK = '<string_literal>' ]
//	  [ SUSPEND_ALERT_AFTER_NUM_FAILURES = <number> ]
//	  IF( EXISTS(
//	    <condition>
//	  ))
//	  THEN
//	    <action>
func (v *Validator) ParseCreateAlert() bool {
	return true
}

// ParseCreateApiIntegration validates the Snowflake `CREATE API INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-api-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = { aws_api_gateway | aws_private_api_gateway | aws_gov_api_gateway | aws_gov_private_api_gateway }
//	  API_AWS_ROLE_ARN = '<iam_role>'
//	  [ API_KEY = '<api_key>' ]
//	  API_ALLOWED_PREFIXES = ('<...>')
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = azure_api_management
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  AZURE_AD_APPLICATION_ID = '<azure_application_id>'
//	  [ API_KEY = '<api_key>' ]
//	  API_ALLOWED_PREFIXES = ( '<...>' )
//	  [ API_BLOCKED_PREFIXES = ( '<...>' ) ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = google_api_gateway
//	  GOOGLE_AUDIENCE = '<google_audience_claim>'
//	  API_ALLOWED_PREFIXES = ( '<...>' )
//	  [ API_BLOCKED_PREFIXES = ( '<...>' ) ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = git_https_api
//	  API_ALLOWED_PREFIXES = ('<...>')
//	  [ API_BLOCKED_PREFIXES = ('<...>') ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = ( { <secret_name> [, <secret_name>, ... ] } ) | all | none ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
func (v *Validator) ParseCreateApiIntegration() bool {
	return true
}

// ParseCreateApplication validates the Snowflake `CREATE APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application
//
// Syntax:
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	   [ USING RELEASE CHANNEL { QA | ALPHA | DEFAULT } ]
//	   [ COMMENT = '<string_literal>' ]
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	   [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	   [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	  USING <path_to_version_directory>
//	  [ DEBUG_MODE = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [, ...] ) ]
//	  [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	  [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	  USING VERSION  <version_identifier> [ PATCH <patch_num> ]
//	  [ DEBUG_MODE = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	  [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM LISTING <listing_name>
//	   [ USING RELEASE CHANNEL { QA | ALPHA | DEFAULT } ]
//	   [ COMMENT = '<string_literal>' ]
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	   [ BACKGROUND_INSTALL = { TRUE | FALSE } ]
//	   [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	   [ WITH FEATURE POLICY = <policy_name> ]
func (v *Validator) ParseCreateApplication() bool {
	return true
}

// ParseCreateApplicationPackage validates the Snowflake `CREATE APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-package
//
// Syntax:
//
//	CREATE APPLICATION PACKAGE [ IF NOT EXISTS ] <name>
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ DISTRIBUTION = { INTERNAL | EXTERNAL } ]
//	  [ LISTING_AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ MULTIPLE_INSTANCES = TRUE ]
//	  [ ENABLE_RELEASE_CHANNELS = TRUE ]
func (v *Validator) ParseCreateApplicationPackage() bool {
	return true
}

// ParseCreateApplicationRole validates the Snowflake `CREATE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] APPLICATION ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER APPLICATION ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateApplicationRole() bool {
	return true
}

// ParseCreateAuthenticationPolicy validates the Snowflake `CREATE AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-authentication-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AUTHENTICATION POLICY [ IF NOT EXISTS ] <name>
//	  [ AUTHENTICATION_METHODS = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_TYPES = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_POLICY = ( <client_type> = ( MINIMUM_VERSION = '<version>' ) [ , ... ] ) ]
//	  [ SECURITY_INTEGRATIONS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	  [ MFA_ENROLLMENT = { 'REQUIRED' | 'REQUIRED_PASSWORD_ONLY' | 'OPTIONAL' } ]
//	  [ MFA_POLICY= ( <list_of_properties> ) ]
//	  [ PAT_POLICY = ( <list_of_properties> ) ]
//	  [ WORKLOAD_IDENTITY_POLICY = ( <list_of_properties> ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER AUTHENTICATION POLICY <name>
//	  [ ... same properties as above ... ]
func (v *Validator) ParseCreateAuthenticationPolicy() bool {
	return true
}

// ParseCreateBackupPolicy validates the Snowflake `CREATE BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] BACKUP POLICY [ IF NOT EXISTS ] <name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH RETENTION LOCK ]
//	   [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	   [ EXPIRE_AFTER_DAYS = <days_integer> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateBackupPolicy() bool {
	return true
}

// ParseCreateBackupSet validates the Snowflake `CREATE BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-set
//
// Syntax:
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	   FOR [ DYNAMIC ] TABLE <table_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	  FOR SCHEMA <schema_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	  FOR DATABASE <database_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateBackupSet() bool {
	return true
}

// ParseCreateCatalogIntegration validates the Snowflake `CREATE CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateCatalogIntegration() bool {
	return true
}

// ParseCreateCatalogIntegrationAwsGlue validates the Snowflake `CREATE CATALOG INTEGRATION (AWS Glue)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-glue
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  CATALOG_SOURCE = GLUE
//	  TABLE_FORMAT = ICEBERG
//	  GLUE_AWS_ROLE_ARN = '<arn-for-AWS-role-to-assume>'
//	  GLUE_CATALOG_ID = '<glue-catalog-id>'
//	  [ GLUE_REGION = '<AWS-region-of-the-glue-catalog>' ]
//	  [ CATALOG_NAMESPACE = '<catalog-namespace>' ]
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationAwsGlue() bool {
	return true
}

// ParseCreateCatalogIntegrationObjectStorage validates the Snowflake `CREATE CATALOG INTEGRATION (Object storage)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-object-storage
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  CATALOG_SOURCE = OBJECT_STORE
//	  TABLE_FORMAT = { ICEBERG | DELTA }
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationObjectStorage() bool {
	return true
}

// ParseCreateCatalogIntegrationSnowflakeOpenCatalog validates the Snowflake `CREATE CATALOG INTEGRATION (Snowflake Open Catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-open-catalog
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ]
//	  <name>
//	  CATALOG_SOURCE = POLARIS
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<open_catalog_namespace>' ]
//	  REST_CONFIG = (
//	    CATALOG_URI = '<open_catalog_account_url>'
//	    [ CATALOG_API_TYPE = PUBLIC ]
//	    CATALOG_NAME = '<open_catalog_catalog_name>'
//	    [ ACCESS_DELEGATION_MODE = { VENDED_CREDENTIALS | EXTERNAL_VOLUME_CREDENTIALS } ]
//	  )
//	  REST_AUTHENTICATION = (
//	    TYPE = OAUTH
//	    [ OAUTH_TOKEN_URI = 'https://<token_server_uri>' ]
//	    OAUTH_CLIENT_ID = '<oauth_client_id>'
//	    OAUTH_CLIENT_SECRET = '<oauth_secret>'
//	    OAUTH_ALLOWED_SCOPES = ('<scope 1>', '<scope 2>')
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationSnowflakeOpenCatalog() bool {
	return true
}

// ParseCreateCatalogIntegrationApacheIcebergRest validates the Snowflake `CREATE CATALOG INTEGRATION (Apache Iceberg REST)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-rest
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = ICEBERG_REST
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<namespace>' ]
//	  REST_CONFIG = (
//	    restConfigParams
//	  )
//	  REST_AUTHENTICATION = (
//	    restAuthenticationParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	restConfigParams ::=
//	  CATALOG_URI = '<rest_api_endpoint_url>'
//	  [ PREFIX = '<prefix>' ]
//	  [ CATALOG_NAME = '<catalog_name>' ]
//	  [ CATALOG_API_TYPE = { PUBLIC | PRIVATE | AWS_API_GATEWAY | AWS_PRIVATE_API_GATEWAY | AWS_GLUE | AWS_PRIVATE_GLUE} ]
//	  [ ACCESS_DELEGATION_MODE = { VENDED_CREDENTIALS | EXTERNAL_VOLUME_CREDENTIALS } ]
//
//	restAuthenticationParams (for OAuth) ::=
//	  TYPE = OAUTH
//	  [ OAUTH_TOKEN_URI = 'https://<token_server_uri>' ]
//	  OAUTH_CLIENT_ID = '<oauth_client_id>'
//	  OAUTH_CLIENT_SECRET = '<oauth_client_secret>'
//	  OAUTH_ALLOWED_SCOPES = ('<scope_1>', '<scope_2>')
//
//	restAuthenticationParams (for Bearer token) ::=
//	  TYPE = BEARER
//	  BEARER_TOKEN = '<bearer_token>'
//
//	restAuthenticationParams (for SigV4) ::=
//	  TYPE = SIGV4
//	  SIGV4_IAM_ROLE = '<iam_role_arn>'
//	  [ SIGV4_SIGNING_REGION = '<region>' ]
//	  [ SIGV4_EXTERNAL_ID = '<external_id>' ]
func (v *Validator) ParseCreateCatalogIntegrationApacheIcebergRest() bool {
	return true
}

// ParseCreateObjClone validates the Snowflake `CREATE <object> CLONE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-clone
//
// Syntax:
//
//	CREATE [ OR REPLACE ] { DATABASE | SCHEMA } [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	    [ IGNORE HYBRID TABLES ]
//	    [ INCLUDE INTERNAL STAGES ]
//	  ...
//
//	CREATE [ OR REPLACE ] TABLE [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	  ...
//
//	CREATE [ OR REPLACE ] DYNAMIC TABLE <name>
//	  CLONE <source_dynamic_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	  [
//	    TARGET_LAG = { '<num> { seconds | minutes | hours | days }' | DOWNSTREAM }
//	    WAREHOUSE = <warehouse_name>
//	  ]
//
//	CREATE [ OR REPLACE ] EVENT TABLE <name>
//	  CLONE <source_event_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <name>
//	  CLONE <source_iceberg_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ COPY GRANTS ]
//	    ...
//
//	CREATE [ OR REPLACE ] DATABASE ROLE [ IF NOT EXISTS ] <database_role_name>
//	  CLONE <source_database_role_name>
//
//	CREATE [ OR REPLACE ] { ALERT | FILE FORMAT | SEQUENCE | STAGE | STREAM | TASK }
//	  [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	  ...
func (v *Validator) ParseCreateObjClone() bool {
	return true
}

// ParseCreateComputePool validates the Snowflake `CREATE COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-compute-pool
//
// Syntax:
//
//	CREATE COMPUTE POOL [ IF NOT EXISTS ] <name>
//	  [ FOR APPLICATION <app-name> ]
//	  MIN_NODES = <num>
//	  MAX_NODES = <num>
//	  INSTANCE_FAMILY = <instance_family_name>
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ INITIALLY_SUSPENDED = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND_SECS = <num>  ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ PLACEMENT_GROUP = '<placement_group_name>' ]
func (v *Validator) ParseCreateComputePool() bool {
	return true
}

// ParseCreateConnection validates the Snowflake `CREATE CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-connection
//
// Syntax:
//
//	-- Primary Connection
//	CREATE CONNECTION [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	-- Secondary Connection
//	CREATE CONNECTION [ IF NOT EXISTS ] <name>
//	  AS REPLICA OF <organization_name>.<account_name>.<name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateConnection() bool {
	return true
}

// ParseCreateContact validates the Snowflake `CREATE CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-contact
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CONTACT [ IF NOT EXISTS ] <name>
//	  [ {
//	    USERS = ( '<user_name>' [ , '<user_name>' ... ] )
//	    | EMAIL_DISTRIBUTION_LIST = '<email>'
//	    | URL = '<url>'
//	    } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateContact() bool {
	return true
}

// ParseCreateCortexSearchService validates the Snowflake `CREATE CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-cortex-search
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CORTEX SEARCH SERVICE [ IF NOT EXISTS ] <name>
//	  ON <search_column>
//	  [ PRIMARY KEY ( <col_name> [, ... ] ) ]
//	  ATTRIBUTES <col_name> [ , ... ]
//	  WAREHOUSE = <warehouse_name>
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  [ EMBEDDING_MODEL = <embedding_model_name> ]
//	  [ REFRESH_MODE = { FULL | INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ FULL_INDEX_BUILD_INTERVAL_DAYS = <num> ]
//	  [ REQUEST_LOGGING = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND = <num_seconds> ]
//	  [ COMMENT = '<comment>' ]
//	AS <query>;
//
//	CREATE [ OR REPLACE ] CORTEX SEARCH SERVICE <name>
//	  TEXT INDEXES <text_column_name> [ , ... ]
//	  VECTOR INDEXES <column_specification> [ , ... ]
//	  [ PRIMARY KEY ( <col_name> [, ... ] ) ]
//	  ATTRIBUTES <col_name> [ , ... ]
//	  WAREHOUSE = <warehouse_name>
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  [ REFRESH_MODE = { FULL | INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ FULL_INDEX_BUILD_INTERVAL_DAYS = <num> ]
//	  [ REQUEST_LOGGING = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND = <num_seconds> ]
//	  [ COMMENT = '<comment>' ]
//	AS <query>;
func (v *Validator) ParseCreateCortexSearchService() bool {
	return true
}

// ParseCreateDataMetricFunction validates the Snowflake `CREATE DATA METRIC FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-data-metric-function
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] DATA METRIC FUNCTION [ IF NOT EXISTS ] <name>
//	  ( <table_arg> TABLE( <col_arg> <data_type> [ , ... ] )
//	    [ , <table_arg> TABLE( <col_arg> <data_type> [ , ... ] ) ] )
//	  RETURNS NUMBER [ [ NOT ] NULL ]
//	  [ LANGUAGE SQL ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS
//	  '<expression>'
func (v *Validator) ParseCreateDataMetricFunction() bool {
	return true
}

// ParseCreateDatabase validates the Snowflake `CREATE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] DATABASE [ IF NOT EXISTS ] <name>
//	    [ CLONE <source_schema>
//	        [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	        [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	        [ IGNORE HYBRID TABLES ] ]
//	    [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	    [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	    [ EXTERNAL_VOLUME = <external_volume_name> ]
//	    [ CATALOG = <catalog_integration_name> ]
//	    [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	    [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	    [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	    [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	    [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	    [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	    [ CATALOG_SYNC_NAMESPACE_MODE = { NEST | FLATTEN } ]
//	    [ CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER = '<string_literal>' ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	    [ OBJECT_VISIBILITY = { <object_visibility_spec> | PRIVILEGED } ]
//	    [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//
//	CREATE DATABASE <name> FROM BACKUP SET <backup_set> IDENTIFIER '<backup_id>'
//
//	CREATE DATABASE <name> FROM LISTING '<listing_global_name>'
//
//	CREATE DATABASE <name> FROM SHARE <provider_account>.<share_name>
//
//	CREATE DATABASE <name>
//	    AS REPLICA OF <account_identifier>.<primary_db_name>
//	    [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//
//	CREATE OR ALTER [ TRANSIENT ] DATABASE <name>
//	    [ ... database properties ... ]
func (v *Validator) ParseCreateDatabase() bool {
	return true
}

// ParseCreateDatabaseCatalogLinked validates the Snowflake `CREATE DATABASE (catalog-linked)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-catalog-linked
//
// Syntax:
//
//	CREATE DATABASE <name>
//	  LINKED_CATALOG = ( catalogParams ),
//	  [ EXTERNAL_VOLUME = '<external_vol>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ CATALOG_CASE_SENSITIVITY = { CASE_SENSITIVE | CASE_INSENSITIVE } ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	Where:
//
//	catalogParams ::=
//	  CATALOG = '<catalog_int>',
//	  [ ALLOWED_NAMESPACES = ('<namespace1>', '<namespace2>', ... ) ]
//	  [ BLOCKED_NAMESPACES = ('<namespace1>', '<namespace2>', ... ) ]
//	  [ ALLOWED_WRITE_OPERATIONS = { NONE | ALL } ]
//	  [ NAMESPACE_MODE = { IGNORE_NESTED_NAMESPACE | FLATTEN_NESTED_NAMESPACE } ]
//	  [ NAMESPACE_FLATTEN_DELIMITER = '<string_literal>' ]
//	  [ SYNC_INTERVAL_SECONDS = <value> ]
func (v *Validator) ParseCreateDatabaseCatalogLinked() bool {
	return true
}

// ParseCreateDatabaseRole validates the Snowflake `CREATE DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DATABASE ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER DATABASE ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateDatabaseRole() bool {
	return true
}

// ParseCreateDataset validates the Snowflake `CREATE DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dataset
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ IF NOT EXISTS ] DATASET <name>
func (v *Validator) ParseCreateDataset() bool {
	return true
}

// ParseCreateDbtProject validates the Snowflake `CREATE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dbt-project
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DBT PROJECT [ IF NOT EXISTS ] <name>
//	  [ FROM '<source_location>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ DBT_VERSION = <version_number> ]
//	  [ DEFAULT_TARGET = <default_target> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
func (v *Validator) ParseCreateDbtProject() bool {
	return true
}

// ParseCreateDcmProject validates the Snowflake `CREATE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dcm-project
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DCM PROJECT [ IF NOT EXISTS ] <name>
//	  [LOG_LEVEL = { DEBUG | INFO | WARN | ERROR }]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateDcmProject() bool {
	return true
}

// ParseCreateDynamicTable validates the Snowflake `CREATE DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dynamic-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] DYNAMIC TABLE [ IF NOT EXISTS ] <name> (
//	    -- Column definition
//	    <col_name> <col_type>
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	      [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	    -- Additional column definitions
//	    [ , <col_name> <col_type> [ ... ] ]
//	  )
//	  TARGET_LAG = { '<num> { seconds | minutes | hours | days }' | DOWNSTREAM }
//	  [ SCHEDULER = DISABLE | ENABLE ]
//	  WAREHOUSE = <warehouse_name>
//	  [ INITIALIZATION_WAREHOUSE = <warehouse_name> ]
//	  [ REFRESH_MODE = { AUTO | FULL | INCREMENTAL | ADAPTIVE | CUSTOM_INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ REQUIRE USER ]
//	  [ FROZEN WHERE ( <expr> ) ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ BACKFILL FROM <table_name> ]
//	  [ START AT ({ STREAM => '<stream_name>' | TIMESTAMP => <timestamp> | STATEMENT => <query_id> | OFFSET => -<seconds> }) ]
//	  [ EXECUTE AS USER <user_name>
//	    [ USE SECONDARY ROLES { ALL | NONE | <role> [ , ... ] } ]
//	  ]
//	  [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//	  { AS <query> | REFRESH USING ( <dml_statement> ) }
func (v *Validator) ParseCreateDynamicTable() bool {
	return true
}

// ParseCreateEventTable validates the Snowflake `CREATE EVENT TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-event-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EVENT TABLE [ IF NOT EXISTS ] <name>
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COPY GRANTS ]
//	  [ [ WITH ] COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	CREATE [ OR REPLACE ] EVENT TABLE [ IF NOT EXISTS ] <name>
//	  CLONE <source_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ COPY GRANTS ]
//	    [ ... ]
func (v *Validator) ParseCreateEventTable() bool {
	return true
}

// ParseCreateExperiment validates the Snowflake `CREATE EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-experiment
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXPERIMENT [ IF NOT EXISTS ] <name>
func (v *Validator) ParseCreateExperiment() bool {
	return true
}

// ParseCreateExternalAgent validates the Snowflake `CREATE EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-agent
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL AGENT [ IF NOT EXISTS ] <name>
//	  [ WITH VERSION <version_name> ]
//	  [ COMMENT = '<comment>' ]
func (v *Validator) ParseCreateExternalAgent() bool {
	return true
}

// ParseCreateExternalAccessIntegration validates the Snowflake `CREATE EXTERNAL ACCESS INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-access-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL ACCESS INTEGRATION <name>
//	  ALLOWED_NETWORK_RULES = ( <rule_name_1> [, <rule_name_2>, ... ] )
//	  [ ALLOWED_API_AUTHENTICATION_INTEGRATIONS = { ( <integration_name_1> [, <integration_name_2>, ... ] ) | none } ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = { ( <secret_name_1> [, <secret_name_2>, ... ] ) | all | none } ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateExternalAccessIntegration() bool {
	return true
}

// ParseCreateExternalFunction validates the Snowflake `CREATE EXTERNAL FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-function
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] EXTERNAL FUNCTION <name> ( [ <arg_name> <arg_data_type> ] [ , ... ] )
//	  RETURNS <result_data_type>
//	  [ [ NOT ] NULL ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  API_INTEGRATION = <api_integration_name>
//	  [ HEADERS = ( '<header_1>' = '<value_1>' [ , '<header_2>' = '<value_2>' ... ] ) ]
//	  [ CONTEXT_HEADERS = ( <context_function_1> [ , <context_function_2> ...] ) ]
//	  [ MAX_BATCH_ROWS = <integer> ]
//	  [ COMPRESSION = <compression_type> ]
//	  [ REQUEST_TRANSLATOR = <request_translator_udf_name> ]
//	  [ RESPONSE_TRANSLATOR = <response_translator_udf_name> ]
//	  AS '<url_of_proxy_and_resource>';
//
//	CREATE [ OR ALTER ] EXTERNAL FUNCTION ...
func (v *Validator) ParseCreateExternalFunction() bool {
	return true
}

// ParseCreateExternalTable validates the Snowflake `CREATE EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-table
//
// Syntax:
//
//	-- Partitions computed from expressions
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( [ <col_name> <col_type> AS <expr> | <part_col_name> <col_type> AS <part_expr> ]
//	      [ inlineConstraint ]
//	      [ , ... ] )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  [ REFRESH_ON_CREATE =  { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ PATTERN = '<regex_pattern>' ]
//	  FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET } [ formatTypeOptions ] } )
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON (VALUE) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	-- Partitions added and removed manually
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( ... )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  PARTITION_TYPE = USER_SPECIFIED
//	  FILE_FORMAT = ( ... )
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ... ]
//
//	-- Delta Lake
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( ... )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  PARTITION_TYPE = USER_SPECIFIED
//	  FILE_FORMAT = ( ... )
//	  [ TABLE_FORMAT = DELTA ]
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ... ]
func (v *Validator) ParseCreateExternalTable() bool {
	return true
}

// ParseCreateExternalVolume validates the Snowflake `CREATE EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-volume
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL VOLUME [ IF NOT EXISTS ]
//	  <name>
//	  STORAGE_LOCATIONS =
//	    (
//	      (
//	        NAME = '<storage_location_name>'
//	        { cloudProviderParams | s3CompatibleStorageParams }
//	      )
//	      [, (...), ...]
//	    )
//	  [ ALLOW_WRITES = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	cloudProviderParams (for Amazon S3) ::=
//	  STORAGE_PROVIDER = '{ S3 | S3GOV }'
//	  STORAGE_AWS_ROLE_ARN = '<iam_role>'
//	  STORAGE_BASE_URL = '<protocol>://<bucket>[/<path>/]'
//	  [ STORAGE_AWS_ACCESS_POINT_ARN = '<string>' ]
//	  [ STORAGE_AWS_EXTERNAL_ID = '<external_id>' ]
//	  [ ENCRYPTION = ( [ TYPE = 'AWS_SSE_S3' ] |
//	              [ TYPE = 'AWS_SSE_KMS' [ KMS_KEY_ID = '<string>' ] ] |
//	              [ TYPE = 'NONE' ] ) ]
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	cloudProviderParams (for Google Cloud Storage) ::=
//	  STORAGE_PROVIDER = 'GCS'
//	  STORAGE_BASE_URL = 'gcs://<bucket>[/<path>/]'
//	  [ ENCRYPTION = ( [ TYPE = 'GCS_SSE_KMS' ] [ KMS_KEY_ID = '<string>' ] |
//	              [ TYPE = 'NONE' ] ) ]
//
//	cloudProviderParams (for Microsoft Azure) ::=
//	  STORAGE_PROVIDER = 'AZURE'
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  STORAGE_BASE_URL = 'azure://...'
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	s3CompatibleStorageParams ::=
//	  STORAGE_PROVIDER = 'S3COMPAT'
//	  STORAGE_BASE_URL = 's3compat://<bucket>[/<path>/]'
//	  CREDENTIALS = ( AWS_KEY_ID = '<string>' AWS_SECRET_KEY = '<string>' )
//	  STORAGE_ENDPOINT = '<s3_api_compatible_endpoint>'
func (v *Validator) ParseCreateExternalVolume() bool {
	return true
}

// ParseCreateFailoverGroup validates the Snowflake `CREATE FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-failover-group
//
// Syntax:
//
//	CREATE FAILOVER GROUP [ IF NOT EXISTS ] <name>
//	    OBJECT_TYPES = <object_type> [ , <object_type> , ... ]
//	    [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	    [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	    [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//	    [ ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> , ... ] ]
//	    ALLOWED_ACCOUNTS = <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ]
//	    [ IGNORE EDITION CHECK ]
//	    [ REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	    [ OPTIMIZED_REFRESH = { TRUE | FALSE } ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//
//	CREATE FAILOVER GROUP [ IF NOT EXISTS ] <secondary_name>
//	    AS REPLICA OF <org_name>.<source_account_name>.<name>
func (v *Validator) ParseCreateFailoverGroup() bool {
	return true
}

// ParseCreateFeaturePolicy validates the Snowflake `CREATE FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-feature-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] FEATURE POLICY [ IF NOT EXISTS ] <name>
//	  BLOCKED_OBJECT_TYPES_FOR_CREATION = ( <type> [ , ... ] )
//	  [ COMMENT = '<string-literal>' ]
func (v *Validator) ParseCreateFeaturePolicy() bool {
	return true
}

// ParseCreateFileFormat validates the Snowflake `CREATE FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-file-format
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY | VOLATILE } ] FILE FORMAT [ IF NOT EXISTS ] <name>
//	  [ TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML } [ formatTypeOptions ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	formatTypeOptions ::=
//	-- If TYPE = CSV
//	     COMPRESSION = AUTO | GZIP | BZ2 | BROTLI | ZSTD | DEFLATE | RAW_DEFLATE | NONE
//	     RECORD_DELIMITER = '<string>' | NONE
//	     FIELD_DELIMITER = '<string>' | NONE
//	     MULTI_LINE = TRUE | FALSE
//	     FILE_EXTENSION = '<string>'
//	     PARSE_HEADER = TRUE | FALSE
//	     SKIP_HEADER = <integer>
//	     SKIP_BLANK_LINES = TRUE | FALSE
//	     DATE_FORMAT = '<string>' | AUTO
//	     TIME_FORMAT = '<string>' | AUTO
//	     TIMESTAMP_FORMAT = '<string>' | AUTO
//	     BINARY_FORMAT = HEX | BASE64 | UTF8
//	     ESCAPE = '<character>' | NONE
//	     ESCAPE_UNENCLOSED_FIELD = '<character>' | NONE
//	     TRIM_SPACE = TRUE | FALSE
//	     FIELD_OPTIONALLY_ENCLOSED_BY = '<character>' | NONE
//	     NULL_IF = ( '<string>' [ , '<string>' ... ] )
//	     ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE | FALSE
//	     REPLACE_INVALID_CHARACTERS = TRUE | FALSE
//	     EMPTY_FIELD_AS_NULL = TRUE | FALSE
//	     SKIP_BYTE_ORDER_MARK = TRUE | FALSE
//	     ENCODING = '<string>' | UTF8
//	-- If TYPE = JSON | AVRO | ORC | PARQUET | XML
//	     ... (per-format options; see Reference URL for the full per-type lists)
func (v *Validator) ParseCreateFileFormat() bool {
	return true
}

// ParseCreateFunction validates the Snowflake `CREATE FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function
//
// Syntax:
//
//	-- Java handler (in-line)
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] FUNCTION [ IF NOT EXISTS ] <name> (
//	    [ <arg_name> <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( <col_name> <col_data_type> [ , ... ] ) }
//	  [ [ NOT ] NULL ]
//	  LANGUAGE JAVA
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ RUNTIME_VERSION = <java_jdk_version> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ IMPORTS = ( '<stage_path...>' [ , ... ] ) ]
//	  [ PACKAGES = ( '<package_name_and_version>' [ , ... ] ) ]
//	  HANDLER = '<path_to_method>'
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <name_of_integration> [ , ... ] ) ]
//	  [ SECRETS = ('<secret_variable_name>' = <secret_name> [ , ... ] ) ]
//	  [ TARGET_PATH = '<stage_path_and_file_name_to_write>' ]
//	  AS '<function_definition>'
//
//	-- Other handlers: LANGUAGE JAVASCRIPT, LANGUAGE PYTHON ([ AGGREGATE ],
//	-- RUNTIME_VERSION, ARTIFACT_REPOSITORY), LANGUAGE SCALA, and SQL.
//
//	-- SQL handler
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] FUNCTION <name> ( ... )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( <col_name> <col_data_type> [ , ... ] ) }
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ MEMOIZABLE ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS '<function_definition>'
//
//	-- CREATE OR ALTER FUNCTION ... is also supported.
func (v *Validator) ParseCreateFunction() bool {
	return true
}

// ParseCreateFunctionSnowparkContainerServices validates the Snowflake `CREATE FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function-spcs
//
// Syntax:
//
//	CREATE [ OR REPLACE ] FUNCTION <name> ( [ <arg_name> <arg_data_type> ] [ , ... ] )
//	  RETURNS <result_data_type>
//	  [ [ NOT ] NULL ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  SERVICE = <service_name>
//	  ENDPOINT = <endpoint_name>
//	  [ COMMENT = '<string_literal>' ]
//	  [ CONTEXT_HEADERS = ( <context_function_1> [ , <context_function_2> ...] ) ]
//	  [ MAX_BATCH_ROWS = <integer> ]
//	  [ MAX_BATCH_RETRIES = <integer> ]
//	  [ ON_BATCH_FAILURE = { ABORT | RETURN_NULL } ]
//	  [ BATCH_TIMEOUT_SECS = <integer> ]
//	  AS '<http_path_to_request_handler>'
//
//	CREATE [ OR ALTER ] FUNCTION ...
func (v *Validator) ParseCreateFunctionSnowparkContainerServices() bool {
	return true
}

// ParseCreateGateway validates the Snowflake `CREATE GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-gateway
//
// Syntax:
//
//	CREATE [ OR REPLACE ] GATEWAY [ IF NOT EXISTS ] <name>
//	  FROM SPECIFICATION <specification_text>
func (v *Validator) ParseCreateGateway() bool {
	return true
}

// ParseCreateGitRepository validates the Snowflake `CREATE GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-git-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] GIT REPOSITORY [ IF NOT EXISTS ] <name>
//	  ORIGIN = '<repository_url>'
//	  API_INTEGRATION = <integration_name>
//	  [ GIT_CREDENTIALS = <secret_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateGitRepository() bool {
	return true
}

// ParseCreateHybridTable validates the Snowflake `CREATE HYBRID TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-hybrid-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] HYBRID TABLE [ IF NOT EXISTS ] <table_name>
//	  ( <col_name> <col_type>
//	    [
//	      {
//	        DEFAULT <expr>
//	        | { AUTOINCREMENT | IDENTITY }
//	          [ { ( <start_num> , <step_num> ) | START <num> INCREMENT <num> } ]
//	          [ { ORDER | NOORDER } ]
//	      }
//	    ]
//	    [ NOT NULL ]
//	    [ inlineConstraint ]
//	    [ COLLATE '<collation_specification>' ]
//	    [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ ... ] ]
//	    [ , outoflineConstraint ]
//	    [ , outoflineIndex ]
//	    [ , ... ]
//	  )
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	inlineConstraint ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY | { [ FOREIGN KEY ] REFERENCES <ref_table_name> [ ( <ref_col_name> ) ] } }
//	  [ <constraint_properties> ]
//
//	outoflineConstraint ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE [ ( <col_name> [ , <col_name> , ... ] ) ]
//	    | PRIMARY KEY [ ( <col_name> [ , <col_name> , ... ] ) ]
//	    | [ FOREIGN KEY ] [ ( <col_name> [ , <col_name> , ... ] ) ]
//	      REFERENCES <ref_table_name> [ ( <ref_col_name> [ , <ref_col_name> , ... ] ) ]
//	  }
//	  [ <constraint_properties> ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineIndex ::=
//	  INDEX <index_name> ( <col_name> [ , <col_name> , ... ] )
//	    [ INCLUDE ( <col_name> [ , <col_name> , ... ] ) ]
func (v *Validator) ParseCreateHybridTable() bool {
	return true
}

// ParseCreateIcebergTable validates the Snowflake `CREATE ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name> (
//	    -- Column definition
//	    <col_name> <col_type> [ DEFAULT <col_default> ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ DEFAULT <col_default> ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ PARTITION BY ( partitionExpression [, ...] ) ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = 'SNOWFLAKE' ]
//	  [ BASE_LOCATION = '<directory_for_table_files>' ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ CATALOG_SYNC = '<open_catalog_integration_name>']
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ICEBERG_VERSION = <integer> ]
//	  [ ... ]
//
//	-- Additional forms: CTAS (AS SELECT), LIKE, and create-from-catalog/files
//	-- variants (CATALOG_TABLE_NAME / METADATA_FILE_PATH / BASE_LOCATION).
func (v *Validator) ParseCreateIcebergTable() bool {
	return true
}

// ParseCreateIcebergTableAwsGlueCatalog validates the Snowflake `CREATE ICEBERG TABLE (AWS Glue catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-aws-glue
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  CATALOG_TABLE_NAME = '<catalog_table_name>'
//	  [ CATALOG_NAMESPACE = '<catalog_namespace>' ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableAwsGlueCatalog() bool {
	return true
}

// ParseCreateIcebergTableDeltaFiles validates the Snowflake `CREATE ICEBERG TABLE (Delta files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-delta
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  BASE_LOCATION = '<relative_path_from_external_volume>'
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableDeltaFiles() bool {
	return true
}

// ParseCreateIcebergTableIcebergFiles validates the Snowflake `CREATE ICEBERG TABLE (Iceberg files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-iceberg-files
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  METADATA_FILE_PATH = '<metadata_file_path>'
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableIcebergFiles() bool {
	return true
}

// ParseCreateIcebergTableIcebergRestCatalog validates the Snowflake `CREATE ICEBERG TABLE (Iceberg REST catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-rest
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  CATALOG_TABLE_NAME = '<rest_catalog_table_name>'
//	  [ CATALOG_NAMESPACE = '<catalog_namespace>' ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableIcebergRestCatalog() bool {
	return true
}

// ParseCreateIcebergTableSnowflakeCatalog validates the Snowflake `CREATE ICEBERG TABLE (Snowflake catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-snowflake
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name> (
//	    -- Column definition
//	    <col_name> <col_type> [ DEFAULT <col_default> ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ DEFAULT <col_default> ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ PARTITION BY ( partitionExpression [, ...] ) ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = 'SNOWFLAKE' ]
//	  [ BASE_LOCATION = '<directory_for_table_files>' ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ CATALOG_SYNC = '<open_catalog_integration_name>']
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ICEBERG_VERSION = <integer> ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
func (v *Validator) ParseCreateIcebergTableSnowflakeCatalog() bool {
	return true
}

// ParseCreateImageRepository validates the Snowflake `CREATE IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-image-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] IMAGE REPOSITORY [ IF NOT EXISTS ] <name>
//	  [ ENCRYPTION = ( TYPE = 'SNOWFLAKE_FULL' | TYPE = 'SNOWFLAKE_SSE' ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateImageRepository() bool {
	return true
}

// ParseCreateIndex validates the Snowflake `CREATE INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-index
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INDEX [ IF NOT EXISTS ] <index_name>
//	  ON <table_name>
//	    ( <col_name> [ , <col_name> , ... ] )
//	    [ INCLUDE ( <col_name> [ , <col_name> , ... ] ) ]
func (v *Validator) ParseCreateIndex() bool {
	return true
}

// ParseCreateIntegration validates the Snowflake `CREATE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] <integration_type> INTEGRATION [ IF NOT EXISTS ] <object_name>
//	  [ <integration_type_params> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateIntegration() bool {
	return true
}

// ParseCreateInteractiveTable validates the Snowflake `CREATE INTERACTIVE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INTERACTIVE TABLE [ IF NOT EXISTS ] <table_name>
//	  (
//	    <col_name> <col_type>
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ , <col_name> <col_type> [ ... ] ]
//	  )
//	  CLUSTER BY ( <expr> [ , <expr> , ... ] )
//	  [ TARGET_LAG = '<num> { seconds | minutes | hours | days }' ]
//	  [ WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  AS <query>
func (v *Validator) ParseCreateInteractiveTable() bool {
	return true
}

// ParseCreateInteractiveWarehouse validates the Snowflake `CREATE INTERACTIVE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-warehouse
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INTERACTIVE WAREHOUSE [ IF NOT EXISTS ] <name>
//	       [ TABLES ( <table_name> [ , <table_name> ... ] ) ]
//	       [ [ WITH ] objectProperties ]
//	       [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	       [ objectParams ]
//
//	Where:
//
//	objectProperties ::=
//	  WAREHOUSE_SIZE = { XSMALL | SMALL | MEDIUM | LARGE | XLARGE | XXLARGE | XXXLARGE | X4LARGE | X5LARGE | X6LARGE }
//	  MAX_CLUSTER_COUNT = <num>
//	  MIN_CLUSTER_COUNT = <num>
//	  AUTO_SUSPEND = { <num> | NULL }
//	  AUTO_RESUME = { TRUE | FALSE }
//	  INITIALLY_SUSPENDED = { TRUE | FALSE }
//	  RESOURCE_MONITOR = <monitor_name>
//	  COMMENT = '<string_literal>'
//
//	objectParams ::=
//	  MAX_CONCURRENCY_LEVEL = <num>
//	  STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = <num>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
func (v *Validator) ParseCreateInteractiveWarehouse() bool {
	return true
}

// ParseCreateJoinPolicy validates the Snowflake `CREATE JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-join-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] JOIN POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS JOIN_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateJoinPolicy() bool {
	return true
}

// ParseCreateListing validates the Snowflake `CREATE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-listing
//
// Syntax:
//
//	CREATE EXTERNAL LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  AS '<yaml_manifest_string>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//	  [ REVIEW = { TRUE | FALSE } ]
//	  [ COMMENT = '<string>' ]
//
//	CREATE EXTERNAL LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  FROM '<yaml_manifest_stage_location>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//	  [ REVIEW = { TRUE | FALSE } ]
func (v *Validator) ParseCreateListing() bool {
	return true
}

// ParseCreateMaintenancePolicy validates the Snowflake `CREATE MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-maintenance-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MAINTENANCE POLICY [ IF NOT EXISTS ] <name>
//	  SCHEDULE = 'USING CRON <cron_spec> <timezone>'
//	  [ COMMENT = '<comment>' ]
func (v *Validator) ParseCreateMaintenancePolicy() bool {
	return true
}

// ParseCreateManagedAccount validates the Snowflake `CREATE MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-managed-account
//
// Syntax:
//
//	CREATE MANAGED ACCOUNT <name>
//	    ADMIN_NAME = <username> , ADMIN_PASSWORD = <user_password> ,
//	    TYPE = READER ,
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateManagedAccount() bool {
	return true
}

// ParseCreateMaskingPolicy validates the Snowflake `CREATE MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-masking-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MASKING POLICY [ IF NOT EXISTS ] <name> AS
//	( <arg_name_to_mask> <arg_type_to_mask> [ , <arg_1> <arg_type_1> ... ] )
//	RETURNS <arg_type_to_mask> -> <body>
//	[ COMMENT = '<string_literal>' ]
//	[ EXEMPT_OTHER_POLICIES = { TRUE | FALSE } ]
//
//	CREATE OR ALTER MASKING POLICY <name> AS
//	( <arg_name_to_mask> <arg_type_to_mask> [ , <arg_1> <arg_type_1> ... ] )
//	RETURNS <arg_type_to_mask> -> <body>
//	[ COMMENT = '<string_literal>' ]
//	[ EXEMPT_OTHER_POLICIES = { TRUE | FALSE } ]
func (v *Validator) ParseCreateMaskingPolicy() bool {
	return true
}

// ParseCreateMaterializedView validates the Snowflake `CREATE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-materialized-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] [ INTERACTIVE ] MATERIALIZED VIEW [ IF NOT EXISTS ] <name>
//	  [ COPY GRANTS ]
//	  ( <column_list> )
//	  [ <col1> [ WITH ] MASKING POLICY <policy_name> [ USING ( <col1> , <cond_col1> , ... ) ]
//	           [ WITH ] PROJECTION POLICY <policy_name>
//	           [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ , <col2> [ ... ] ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ CLUSTER BY ( <expr1> [, <expr2> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  AS <select_statement>
func (v *Validator) ParseCreateMaterializedView() bool {
	return true
}

// ParseCreateMcpServer validates the Snowflake `CREATE MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-mcp-server
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MCP SERVER [ IF NOT EXISTS ] <name>
//	  FROM SPECIFICATION $$<specification_yaml>$$
func (v *Validator) ParseCreateMcpServer() bool {
	return true
}

// ParseCreateModel validates the Snowflake `CREATE MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MODEL [ IF NOT EXISTS ] <name> [ WITH VERSION <version_name> ]
//	    FROM MODEL <source_model_name> [ VERSION <source_version_or_alias_name> ]
//
//	CREATE [ OR REPLACE ] MODEL [ IF NOT EXISTS ] <name> [ WITH VERSION <version_name> ]
//	  FROM internalStage
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
func (v *Validator) ParseCreateModel() bool {
	return true
}

// ParseCreateModelMonitor validates the Snowflake `CREATE MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model-monitor
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MODEL MONITOR [ IF NOT EXISTS ] <monitor_name> WITH
//	    MODEL = <model_name>
//	    VERSION = '<version_name>'
//	    FUNCTION = '<function_name>'
//	    SOURCE = <source_name>
//	    WAREHOUSE = <warehouse_name>
//	    REFRESH_INTERVAL = '<num> { seconds | minutes | hours | days }'
//	    AGGREGATION_WINDOW = '<num> days'
//	    TIMESTAMP_COLUMN = <timestamp_name>
//	    [ BASELINE = <baseline_name> ]
//	    [ ID_COLUMNS = <id_column_name_array> ]
//	    [ PREDICTION_CLASS_COLUMNS = <prediction_class_column_name_array> ]
//	    [ PREDICTION_SCORE_COLUMNS = <prediction_column-name_array> ]
//	    [ ACTUAL_CLASS_COLUMNS = <actual_class_column_name_array> ]
//	    [ ACTUAL_SCORE_COLUMNS = <actual_column_name_array> ]
//	    [ SEGMENT_COLUMNS = <segment_column_name_array> ]
//	    [ CUSTOM_METRIC_COLUMNS = <custom_metric_column_name_array> ]
func (v *Validator) ParseCreateModelMonitor() bool {
	return true
}

// ParseCreateNetworkPolicy validates the Snowflake `CREATE NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NETWORK POLICY [ IF NOT EXISTS ] <name>
//	  [ ALLOWED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	  [ BLOCKED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	  [ ALLOWED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' , ... ] ) ]
//	  [ BLOCKED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNetworkPolicy() bool {
	return true
}

// ParseCreateNetworkRule validates the Snowflake `CREATE NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-rule
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NETWORK RULE <name>
//	   TYPE = { IPV4 | IPV6 | AWSVPCEID | AZURELINKID | GCPPSCID | HOST_PORT | PRIVATE_HOST_PORT | COMPUTE_POOL }
//	   VALUE_LIST = ( '<value>' [, '<value>', ... ] )
//	   MODE = { INGRESS | INTERNAL_STAGE | SNOWFLAKE_MANAGED_STORAGE_VOLUME | EGRESS }
//	   [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER NETWORK RULE <name>
//	   TYPE = { IPV4 | IPV6 | AWSVPCEID | AZURELINKID | GCPPSCID | HOST_PORT | PRIVATE_HOST_PORT | COMPUTE_POOL }
//	   VALUE_LIST = ( '<value>' [, '<value>', ... ] )
//	   MODE = { INGRESS | INTERNAL_STAGE | SNOWFLAKE_MANAGED_STORAGE_VOLUME | EGRESS }
//	   [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNetworkRule() bool {
	return true
}

// ParseCreateNotebook validates the Snowflake `CREATE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTEBOOK [ IF NOT EXISTS ] <name>
//	  [ FROM '<source_location>' ]
//	  [ MAIN_FILE = '<main_file_name>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_to_run_nb_and_sql_queries_in> ]
//	  [ IDLE_AUTO_SHUTDOWN_TIME_SECONDS = <number_of_seconds> ]
//	  [ RUNTIME_NAME = '<runtime_name>' ]
//	  [ COMPUTE_POOL = '<compute_pool_name>' ]
//	  [ WAREHOUSE = <warehouse_to_run_notebook_python_runtime> ]
func (v *Validator) ParseCreateNotebook() bool {
	return true
}

// ParseCreateNotebookProject validates the Snowflake `CREATE NOTEBOOK PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook-project
//
// Syntax:
//
//	CREATE NOTEBOOK PROJECT <database_name>.<schema_name>.<project_name>
//	  FROM 'snow://workspace/<workspace_path>'
//	  [ COMMENT = '<string_literal>' ];
//
//	CREATE NOTEBOOK PROJECT [ IF NOT EXISTS ] <database_name>.<schema_name>.<project_name>
//	  FROM '@<database_name>.<schema_name>.<stage_name>'
//	  [ COMMENT = '<string_literal>' ];
func (v *Validator) ParseCreateNotebookProject() bool {
	return true
}

// ParseCreateNotificationIntegration validates the Snowflake `CREATE NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateNotificationIntegration() bool {
	return true
}

// ParseCreateNotificationIntegrationEmail validates the Snowflake `CREATE NOTIFICATION INTEGRATION (email)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-email
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = EMAIL
//	  ENABLED = { TRUE | FALSE }
//	  [ ALLOWED_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_SUBJECT = '<subject_line>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationEmail() bool {
	return true
}

// ParseCreateNotificationIntegrationInboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-azure
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE
//	  AZURE_STORAGE_QUEUE_PRIMARY_URI = '<queue_url>'
//	  AZURE_TENANT_ID = '<ad_directory_id>';
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationInboundAzureEventGrid() bool {
	return true
}

// ParseCreateNotificationIntegrationInboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-gcp
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  NOTIFICATION_PROVIDER = GCP_PUBSUB
//	  GCP_PUBSUB_SUBSCRIPTION_NAME = '<subscription_id>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationInboundGooglePubSub() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundAmazonSns validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Amazon SNS)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-aws
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = AWS_SNS
//	  AWS_SNS_TOPIC_ARN = '<topic_arn>'
//	  AWS_SNS_ROLE_ARN = '<iam_role_arn>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundAmazonSns() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-azure
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = AZURE_EVENT_GRID
//	  AZURE_EVENT_GRID_TOPIC_ENDPOINT = '<event_grid_topic_endpoint>'
//	  AZURE_TENANT_ID = '<ad_directory_id>';
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundAzureEventGrid() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-gcp
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = GCP_PUBSUB
//	  GCP_PUBSUB_TOPIC_NAME = '<topic_id>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundGooglePubSub() bool {
	return true
}

// ParseCreateNotificationIntegrationWebhooks validates the Snowflake `CREATE NOTIFICATION INTEGRATION (webhooks)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-webhooks
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = WEBHOOK
//	  ENABLED = { TRUE | FALSE }
//	  WEBHOOK_URL = '<url>'
//	  [ WEBHOOK_SECRET = <secret_name> ]
//	  [ WEBHOOK_BODY_TEMPLATE = '<template_for_http_request_body>' ]
//	  [ WEBHOOK_HEADERS = ( '<header_1>'='<value_1>' [ , '<header_N>'='<value_N>', ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationWebhooks() bool {
	return true
}

// ParseCreateOnlineFeatureTable validates the Snowflake `CREATE ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-online-feature-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ONLINE FEATURE TABLE <name>
//	  PRIMARY KEY ( <col_name> [ , <col_name> , ... ] )
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  WAREHOUSE = <warehouse_name>
//	  [ REFRESH_MODE = { AUTO | FULL | INCREMENTAL } ]
//	  [ TIMESTAMP_COLUMN = <col_name> ]
//	  [ [ WITH ] COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	FROM <source>
func (v *Validator) ParseCreateOnlineFeatureTable() bool {
	return true
}

// ParseCreateOrAlterObj validates the Snowflake `CREATE OR ALTER <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-or-alter
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateOrAlterObj() bool {
	return true
}

// ParseCreateOrganizationAccount validates the Snowflake `CREATE ORGANIZATION ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-account
//
// Syntax:
//
//	CREATE ORGANIZATION ACCOUNT <name>
//	    ADMIN_NAME = <string>
//	    { ADMIN_PASSWORD = '<string_literal>' | ADMIN_RSA_PUBLIC_KEY = <string> }
//	    [ FIRST_NAME = <string> ]
//	    [ LAST_NAME = <string> ]
//	    EMAIL = '<string>'
//	    [ MUST_CHANGE_PASSWORD = { TRUE | FALSE } ]
//	    EDITION = { ENTERPRISE | BUSINESS_CRITICAL }
//	    [ REGION_GROUP = <region_group_id> ]
//	    [ REGION = <snowflake_region_id> ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateOrganizationAccount() bool {
	return true
}

// ParseCreateOrganizationListing validates the Snowflake `CREATE ORGANIZATION LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-listing
//
// Syntax:
//
//	CREATE ORGANIZATION LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  AS '<yaml_manifest_string>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//
//	CREATE ORGANIZATION LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  FROM '<yaml_manifest_stage_location>'
//	  [ PUBLISH = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationListing() bool {
	return true
}

// ParseCreateOrganizationProfile validates the Snowflake `CREATE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-profile
//
// Syntax:
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//	  AS '<yaml_manifest_string>'
//	  [ VERSION <version_alias_name> ]
//	  [ PUBLISH = { TRUE | FALSE } ]
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//	  FROM @<yaml_manifest_stage_location>
//	  [ VERSION <version_alias_name> ]
//	  [ PUBLISH = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationProfile() bool {
	return true
}

// ParseCreateOrganizationUser validates the Snowflake `CREATE ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user
//
// Syntax:
//
//	CREATE ORGANIZATION USER [ IF NOT EXISTS ] <name>
//	  [ objectProperties ]
//
//	Where:
//
//	objectProperties ::=
//	  EMAIL = '<string>'
//	  LOGIN_NAME = '<string>'
//	  DISPLAY_NAME = '<string>'
//	  FIRST_NAME = '<string>'
//	  MIDDLE_NAME = '<string>'
//	  LAST_NAME = '<string>'
//	  COMMENT = '<string>'
func (v *Validator) ParseCreateOrganizationUser() bool {
	return true
}

// ParseCreateOrganizationUserGroup validates the Snowflake `CREATE ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user-group
//
// Syntax:
//
//	CREATE ORGANIZATION USER GROUP [ IF NOT EXISTS ] <name>
//	  [ IS_GRANTABLE = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationUserGroup() bool {
	return true
}

// ParseCreatePackagesPolicy validates the Snowflake `CREATE PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-packages-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PACKAGES POLICY [ IF NOT EXISTS ] <name>
//	  LANGUAGE PYTHON
//	  [ ALLOWLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ ADDITIONAL_CREATION_BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePackagesPolicy() bool {
	return true
}

// ParseCreatePasswordPolicy validates the Snowflake `CREATE PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-password-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PASSWORD POLICY [ IF NOT EXISTS ] <name>
//	  [ PASSWORD_MIN_LENGTH = <integer> ]
//	  [ PASSWORD_MAX_LENGTH = <integer> ]
//	  [ PASSWORD_MIN_UPPER_CASE_CHARS = <integer> ]
//	  [ PASSWORD_MIN_LOWER_CASE_CHARS = <integer> ]
//	  [ PASSWORD_MIN_NUMERIC_CHARS = <integer> ]
//	  [ PASSWORD_MIN_SPECIAL_CHARS = <integer> ]
//	  [ PASSWORD_MIN_AGE_DAYS = <integer> ]
//	  [ PASSWORD_MAX_AGE_DAYS = <integer> ]
//	  [ PASSWORD_MAX_RETRIES = <integer> ]
//	  [ PASSWORD_LOCKOUT_TIME_MINS = <integer> ]
//	  [ PASSWORD_HISTORY = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePasswordPolicy() bool {
	return true
}

// ParseCreatePipe validates the Snowflake `CREATE PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-pipe
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PIPE [ IF NOT EXISTS ] <name>
//	  [ AUTO_INGEST = [ TRUE | FALSE ] ]
//	  [ ERROR_INTEGRATION = <integration_name> ]
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ INTEGRATION = '<string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS <copy_statement>
//
//	CREATE OR ALTER PIPE <name>
//	  [ AUTO_INGEST = [ TRUE | FALSE ] ]
//	  [ ERROR_INTEGRATION = <integration_name> ]
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ INTEGRATION = '<string>' ]
//	  [ PIPE_EXECUTION_PAUSED = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS <copy_statement>
func (v *Validator) ParseCreatePipe() bool {
	return true
}

// ParseCreatePostgresInstance validates the Snowflake `CREATE POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-postgres-instance
//
// Syntax:
//
//	CREATE POSTGRES INSTANCE <name>
//	  COMPUTE_FAMILY = '<compute_family>'
//	  STORAGE_SIZE_GB = <storage_gb>
//	  AUTHENTICATION_AUTHORITY = { POSTGRES | POSTGRES_OR_SNOWFLAKE }
//	  [ POSTGRES_VERSION = { 16 | 17 | 18 } ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ HIGH_AVAILABILITY = { TRUE | FALSE } ]
//	  [ STORAGE_INTEGRATION = '<storage_integration_name>' ]
//	  [ POSTGRES_SETTINGS = '<json_string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//
//	CREATE POSTGRES INSTANCE <name>
//	  FORK <source_instance>
//	  [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> } ) ]
//	  [ COMPUTE_FAMILY = '<compute_family>' ]
//	  [ STORAGE_SIZE_GB = <storage_gb> ]
//	  [ HIGH_AVAILABILITY = { TRUE | FALSE } ]
//	  [ POSTGRES_SETTINGS = '<json_string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
func (v *Validator) ParseCreatePostgresInstance() bool {
	return true
}

// ParseCreatePrivacyPolicy validates the Snowflake `CREATE PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-privacy-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PRIVACY POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS PRIVACY_BUDGET -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePrivacyPolicy() bool {
	return true
}

// ParseCreateProcedure validates the Snowflake `CREATE PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-procedure
//
// Syntax:
//
//	-- Java handler (in-line)
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] PROCEDURE <name> (
//	    [ <arg_name> <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> [ [ NOT ] NULL ] | TABLE ( [ <col_name> <col_data_type> [ , ... ] ] ) }
//	  LANGUAGE JAVA
//	  RUNTIME_VERSION = '<java_runtime_version>'
//	  PACKAGES = ( 'com.snowflake:snowpark:<version>' [, '<package_name_and_version>' ...] )
//	  [ IMPORTS = ( ... ) ]
//	  HANDLER = '<fully_qualified_method_name>'
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <name_of_integration> [ , ... ] ) ]
//	  [ SECRETS = ('<secret_variable_name>' = <secret_name> [ , ... ] ) ]
//	  [ TARGET_PATH = '<stage_path_and_file_name_to_write>' ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER } ]
//	  AS '<procedure_definition>'
//
//	-- Other languages: JAVASCRIPT, PYTHON, SCALA, and Snowflake Scripting (LANGUAGE SQL).
//
//	-- Snowflake Scripting handler
//	CREATE [ OR REPLACE ] PROCEDURE <name> (
//	    [ <arg_name> [ { IN | INPUT | OUT | OUTPUT } ] <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( [ <col_name> <col_data_type> [ , ... ] ] ) }
//	  [ NOT NULL ]
//	  LANGUAGE SQL
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER } ]
//	  AS <procedure_definition>
func (v *Validator) ParseCreateProcedure() bool {
	return true
}

// ParseCreateProjectionPolicy validates the Snowflake `CREATE PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-projection-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PROJECTION POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS PROJECTION_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateProjectionPolicy() bool {
	return true
}

// ParseCreateProvisionedThroughput validates the Snowflake `CREATE PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-provisioned-throughput
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PROVISIONED THROUGHPUT <name>
//	    CLOUD_PROVIDER = '<cloud_provider>'
//	    MODEL = '<model_name>'
//	    PTUS = <num_ptus>
//	    TERM_START = '<start_date>'
//	    TERM_END = '<end_date>';
func (v *Validator) ParseCreateProvisionedThroughput() bool {
	return true
}

// ParseCreateReplicationGroup validates the Snowflake `CREATE REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-replication-group
//
// Syntax:
//
//	CREATE REPLICATION GROUP [ IF NOT EXISTS ] <name>
//	    OBJECT_TYPES = <object_type> [ , <object_type> , ... ]
//	    [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	    [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	    [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//	    [ ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> , ... ] ]
//	    ALLOWED_ACCOUNTS = <org_name>.<target_account_name> [ , <org_name>.<target_account_name> , ... ]
//	    [ IGNORE EDITION CHECK ]
//	    [ REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//
//	CREATE REPLICATION GROUP [ IF NOT EXISTS ] <secondary_name>
//	    AS REPLICA OF <org_name>.<source_account_name>.<name>
func (v *Validator) ParseCreateReplicationGroup() bool {
	return true
}

// ParseCreateResourceMonitor validates the Snowflake `CREATE RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-resource-monitor
//
// Syntax:
//
//	CREATE [ OR REPLACE ] RESOURCE MONITOR [ IF NOT EXISTS ] <name> WITH
//	                      [ CREDIT_QUOTA = <number> ]
//	                      [ FREQUENCY = { MONTHLY | DAILY | WEEKLY | YEARLY | NEVER } ]
//	                      [ START_TIMESTAMP = { <timestamp> | IMMEDIATELY } ]
//	                      [ END_TIMESTAMP = <timestamp> ]
//	                      [ NOTIFY_USERS = ( <user_name> [ , <user_name> , ... ] ) ]
//	                      [ TRIGGERS triggerDefinition [ triggerDefinition ... ] ]
//
//	Where:
//
//	triggerDefinition ::=
//	    ON <threshold> PERCENT DO { SUSPEND | SUSPEND_IMMEDIATE | NOTIFY }
func (v *Validator) ParseCreateResourceMonitor() bool {
	return true
}

// ParseCreateRole validates the Snowflake `CREATE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//
//	CREATE OR ALTER ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateRole() bool {
	return true
}

// ParseCreateRowAccessPolicy validates the Snowflake `CREATE ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-row-access-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ROW ACCESS POLICY [ IF NOT EXISTS ] <name> AS
//	( <arg_name> <arg_type> [ , ... ] ) RETURNS BOOLEAN -> <body>
//	[ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateRowAccessPolicy() bool {
	return true
}

// ParseCreateSchema validates the Snowflake `CREATE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-schema
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] SCHEMA [ IF NOT EXISTS ] <name>
//	  [ CLONE <source_schema>
//	      [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	      [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	      [ IGNORE HYBRID TABLES ] ]
//	  [ WITH MANAGED ACCESS ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ EXTERNAL_VOLUME = <external_volume_name> ]
//	  [ CATALOG = <catalog_integration_name> ]
//	  [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ CLASSIFICATION_PROFILE = '<classification_profile>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	  [ OBJECT_VISIBILITY = PRIVILEGED ]
//	  [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	CREATE SCHEMA <name> FROM BACKUP SET <backup_set> IDENTIFIER '<backup_id>'
//
//	CREATE OR ALTER [ TRANSIENT ] SCHEMA <name>
//	  [ ... schema properties ... ]
func (v *Validator) ParseCreateSchema() bool {
	return true
}

// ParseCreateSecret validates the Snowflake `CREATE SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-secret
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = OAUTH2
//	  API_AUTHENTICATION = <security_integration_name>
//	  OAUTH_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] )
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = OAUTH2
//	  OAUTH_REFRESH_TOKEN = '<string_literal>'
//	  OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '<string_literal>'
//	  API_AUTHENTICATION = <security_integration_name>;
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = CLOUD_PROVIDER_TOKEN
//	  API_AUTHENTICATION = '<cloud_provider_security_integration>'
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = PASSWORD
//	  USERNAME = '<username>'
//	  PASSWORD = '<password>'
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = GENERIC_STRING
//	  SECRET_STRING = '<string_literal>'
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = SYMMETRIC_KEY
//	  ALGORITHM = GENERIC
func (v *Validator) ParseCreateSecret() bool {
	return true
}

// ParseCreateSecurityIntegration validates the Snowflake `CREATE SECURITY INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	  <name>
//	  TYPE = { API_AUTHENTICATION | EXTERNAL_OAUTH | OAUTH | SAML2 | SCIM }
//	  ...
func (v *Validator) ParseCreateSecurityIntegration() bool {
	return true
}

// ParseCreateSecurityIntegrationExternalApiAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (External API Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-api-auth
//
// Syntax:
//
//	CREATE SECURITY INTEGRATION <name>
//	  TYPE = API_AUTHENTICATION
//	  AUTH_TYPE = OAUTH2
//	  ENABLED = { TRUE | FALSE }
//	  [ OAUTH_TOKEN_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_CLIENT_AUTH_METHOD = { CLIENT_SECRET_BASIC | CLIENT_SECRET_POST } ]
//	  [ OAUTH_CLIENT_ID = '<string_literal>' ]
//	  [ OAUTH_CLIENT_SECRET = '<string_literal>' ]
//	  [ OAUTH_GRANT = 'CLIENT_CREDENTIALS']
//	  [ OAUTH_ACCESS_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_ALLOWED_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- AUTHORIZATION_CODE and JWT_BEARER grant variants are also supported
//	-- (with OAUTH_AUTHORIZATION_ENDPOINT and OAUTH_REFRESH_TOKEN_VALIDITY).
func (v *Validator) ParseCreateSecurityIntegrationExternalApiAuthentication() bool {
	return true
}

// ParseCreateSecurityIntegrationAwsIamAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (AWS IAM Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-aws-iam
//
// Syntax:
//
//	CREATE SECURITY INTEGRATION <name>
//	  TYPE = API_AUTHENTICATION
//	  AUTH_TYPE = AWS_IAM
//	  AWS_ROLE_ARN = '<iam_role_arn>'
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationAwsIamAuthentication() bool {
	return true
}

// ParseCreateSecurityIntegrationExternalOauth validates the Snowflake `CREATE SECURITY INTEGRATION (External OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-external
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = EXTERNAL_OAUTH
//	  ENABLED = { TRUE | FALSE }
//	  EXTERNAL_OAUTH_TYPE = { OKTA | AZURE | PING_FEDERATE | CUSTOM }
//	  EXTERNAL_OAUTH_ISSUER = '<string_literal>'
//	  EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) }
//	  EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = { 'LOGIN_NAME' | 'EMAIL_ADDRESS' }
//	  [ EXTERNAL_OAUTH_JWS_KEYS_URL = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) } ]
//	  [ EXTERNAL_OAUTH_BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY = <public_key1> ]
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY_2 = <public_key2> ]
//	  [ EXTERNAL_OAUTH_AUDIENCE_LIST = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) } ]
//	  [ EXTERNAL_OAUTH_ANY_ROLE_MODE = { DISABLE | ENABLE | ENABLE_FOR_PRIVILEGE } ]
//	  [ EXTERNAL_OAUTH_SCOPE_DELIMITER = '<string_literal>' ]
//	  [ EXTERNAL_OAUTH_SCOPE_MAPPING_ATTRIBUTE = '<string_literal>' ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationExternalOauth() bool {
	return true
}

// ParseCreateSecurityIntegrationSnowflakeOauth validates the Snowflake `CREATE SECURITY INTEGRATION (Snowflake OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-snowflake
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = OAUTH
//	  OAUTH_CLIENT = <partner_application>
//	  OAUTH_REDIRECT_URI = '<uri>'
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_ISSUE_REFRESH_TOKENS = { TRUE | FALSE } ]
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED = { TRUE | FALSE } ]
//	  [ OAUTH_USE_SECONDARY_ROLES = { IMPLICIT | NONE } ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- For custom clients, additionally:
//	--   OAUTH_CLIENT = CUSTOM
//	--   OAUTH_CLIENT_TYPE = 'CONFIDENTIAL' | 'PUBLIC'
func (v *Validator) ParseCreateSecurityIntegrationSnowflakeOauth() bool {
	return true
}

// ParseCreateSecurityIntegrationSaml2 validates the Snowflake `CREATE SECURITY INTEGRATION (SAML2)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-saml2
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	    <name>
//	    TYPE = SAML2
//	    ENABLED = { TRUE | FALSE }
//	    { METADATA_URL = '<string_literal>' | <idp_parameters> }
//	    [ ALLOWED_USER_DOMAINS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	    [ ALLOWED_EMAIL_PATTERNS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	    [ SAML2_SP_INITIATED_LOGIN_PAGE_LABEL = '<string_literal>' ]
//	    [ SAML2_ENABLE_SP_INITIATED = TRUE | FALSE ]
//	    [ SAML2_SNOWFLAKE_X509_CERT = '<string_literal>' ]
//	    [ SAML2_SIGN_REQUEST = TRUE | FALSE ]
//	    [ SAML2_REQUESTED_NAMEID_FORMAT = '<string_literal>' ]
//	    [ SAML2_POST_LOGOUT_REDIRECT_URL = '<string_literal>' ]
//	    [ SAML2_FORCE_AUTHN = TRUE | FALSE ]
//	    [ SAML2_SNOWFLAKE_ISSUER_URL = '<string_literal>' ]
//	    [ SAML2_SNOWFLAKE_ACS_URL = '<string_literal>' ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationSaml2() bool {
	return true
}

// ParseCreateSecurityIntegrationScim validates the Snowflake `CREATE SECURITY INTEGRATION (SCIM)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-scim
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	    <name>
//	    TYPE = SCIM
//	    ENABLED = { TRUE | FALSE }
//	    SCIM_CLIENT = { 'OKTA' | 'AZURE' | 'GENERIC' }
//	    RUN_AS_ROLE = { 'OKTA_PROVISIONER' | 'AAD_PROVISIONER' | 'GENERIC_SCIM_PROVISIONER' | '<custom_role>' }
//	    [ NETWORK_POLICY = '<network_policy>' ]
//	    [ SYNC_PASSWORD = { TRUE | FALSE } ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationScim() bool {
	return true
}

// ParseCreateSemanticView validates the Snowflake `CREATE SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-semantic-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SEMANTIC VIEW [ IF NOT EXISTS ] <name>
//	  TABLES ( logicalTable [ , ... ] )
//	  [ RELATIONSHIPS ( relationshipDef [ , ... ] ) ]
//	  [ FACTS ( factExpression [ , ... ] ) ]
//	  [ DIMENSIONS ( dimensionExpression [ , ... ] ) ]
//	  [ METRICS ( { metricExpression | windowFunctionMetricExpression } [ , ... ] ) ]
//	  [ COMMENT = '<comment_about_semantic_view>' ]
//	  [ AI_SQL_GENERATION '<instructions_for_sql_generation>' ]
//	  [ AI_QUESTION_CATEGORIZATION '<instructions_for_question_categorization>' ]
//	  [ AI_VERIFIED_QUERIES ( verifiedQuery [ , ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COPY GRANTS ]
//
//	-- Sub-definitions: logicalTable, relationshipDef, factExpression,
//	-- dimensionExpression, metricExpression, windowFunctionMetricExpression,
//	-- verifiedQuery (see Reference URL).
func (v *Validator) ParseCreateSemanticView() bool {
	return true
}

// ParseCreateSequence validates the Snowflake `CREATE SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-sequence
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SEQUENCE [ IF NOT EXISTS ] <name>
//	  [ WITH ]
//	  [ START [ WITH ] [ = ] <initial_value> ]
//	  [ INCREMENT [ BY ] [ = ] <sequence_interval> ]
//	  [ { ORDER | NOORDER } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER SEQUENCE <name>
//	  [ WITH ]
//	  [ START [ WITH ] [ = ] <initial_value> ]
//	  [ INCREMENT [ BY ] [ = ] <sequence_interval> ]
//	  [ { ORDER | NOORDER } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSequence() bool {
	return true
}

// ParseCreateService validates the Snowflake `CREATE SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-service
//
// Syntax:
//
//	CREATE SERVICE [ IF NOT EXISTS ] <name>
//	  IN COMPUTE POOL <compute_pool_name>
//	  {
//	     fromSpecification
//	     | fromSpecificationTemplate
//	  }
//	  [ AUTO_SUSPEND_SECS = <num> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <EAI_name> [ , ... ] ) ]
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ MIN_INSTANCES = <num> ]
//	  [ MIN_READY_INSTANCES = <num> ]
//	  [ MAX_INSTANCES = <num> ]
//	  [ LOG_LEVEL = '<log_level>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COMMENT = '{string_literal}']
func (v *Validator) ParseCreateService() bool {
	return true
}

// ParseCreateSessionPolicy validates the Snowflake `CREATE SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-session-policy
//
// Syntax:
//
//	CREATE [OR REPLACE] SESSION POLICY [IF NOT EXISTS] <name>
//	  [ SESSION_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_UI_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_MAX_LIFESPAN_MINS = <integer> ]
//	  [ SESSION_UI_MAX_LIFESPAN_MINS = <integer> ]
//	  [ ALLOWED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ BLOCKED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSessionPolicy() bool {
	return true
}

// ParseCreateShare validates the Snowflake `CREATE SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-share
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SHARE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER SHARE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateShare() bool {
	return true
}

// ParseCreateSnapshot validates the Snowflake `CREATE SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT [ IF NOT EXISTS ] <name>
//	  FROM SERVICE <service_name>
//	  VOLUME "<volume_name>"
//	  INSTANCE <instance_id>
//	  [ COMMENT = '<string_literal>']
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
func (v *Validator) ParseCreateSnapshot() bool {
	return true
}

// ParseCreateSnapshotPolicy validates the Snowflake `CREATE SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT POLICY [ IF NOT EXISTS ] <name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH RETENTION LOCK ]
//	   [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	   [ EXPIRE_AFTER_DAYS = <days_integer> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateSnapshotPolicy() bool {
	return true
}

// ParseCreateSnapshotSet validates the Snowflake `CREATE SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-set
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	   FOR [ DYNAMIC ] TABLE <table_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	  FOR SCHEMA <schema_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	  FOR DATABASE <database_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateSnapshotSet() bool {
	return true
}

// ParseCreateStage validates the Snowflake `CREATE STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stage
//
// Syntax:
//
//	-- Internal stage
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] STAGE [ IF NOT EXISTS ] <internal_stage_name>
//	    internalStageParams
//	    directoryTableParams
//	  [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//
//	-- External stage
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] STAGE [ IF NOT EXISTS ] <external_stage_name>
//	    externalStageParams
//	    directoryTableParams
//	  [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateStage() bool {
	return true
}

// ParseCreateStorageIntegration validates the Snowflake `CREATE STORAGE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = { EXTERNAL_STAGE | POSTGRES_EXTERNAL_STORAGE | POSTGRES_INTERNAL_STORAGE }
//	  cloudProviderParams
//	  ENABLED = { TRUE | FALSE }
//	  STORAGE_ALLOWED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] )
//	  [ STORAGE_BLOCKED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	cloudProviderParams (for Amazon S3) ::=
//	  STORAGE_PROVIDER = 'S3'
//	  STORAGE_AWS_ROLE_ARN = '<iam_role>'
//	  [ STORAGE_AWS_EXTERNAL_ID = '<external_id>' ]
//	  [ STORAGE_AWS_OBJECT_ACL = 'bucket-owner-full-control' ]
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	cloudProviderParams (for Google Cloud Storage) ::=
//	  STORAGE_PROVIDER = 'GCS'
//
//	cloudProviderParams (for Microsoft Azure) ::=
//	  STORAGE_PROVIDER = 'AZURE'
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
func (v *Validator) ParseCreateStorageIntegration() bool {
	return true
}

// ParseCreateStorageLifecyclePolicy validates the Snowflake `CREATE STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-lifecycle-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE LIFECYCLE POLICY [ IF NOT EXISTS ] <name>
//	  AS ( <arg_name> <arg_type> [ , ... ] )
//	  RETURNS BOOLEAN -> <body>
//	  [ ARCHIVE_TIER = { COOL | COLD } ]
//	  [ ARCHIVE_FOR_DAYS = <number_of_days> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateStorageLifecyclePolicy() bool {
	return true
}

// ParseCreateStream validates the Snowflake `CREATE STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stream
//
// Syntax:
//
//	-- Table
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COPY GRANTS ]
//	  ON TABLE <table_name>
//	  [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> | STREAM => '<name>' } ) ]
//	  [ APPEND_ONLY = TRUE | FALSE ]
//	  [ SHOW_INITIAL_ROWS = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- External table
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ COPY GRANTS ]
//	  ON EXTERNAL TABLE <external_table_name>
//	  [ { AT | BEFORE } ( ... ) ]
//	  [ INSERT_ONLY = TRUE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- View
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ COPY GRANTS ]
//	  ON VIEW <view_name>
//	  [ { AT | BEFORE } ( ... ) ]
//	  [ APPEND_ONLY = TRUE | FALSE ]
//	  [ SHOW_INITIAL_ROWS = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- Also supported: ON EVENT TABLE, ON STAGE, ON DYNAMIC TABLE.
func (v *Validator) ParseCreateStream() bool {
	return true
}

// ParseCreateStreamlit validates the Snowflake `CREATE STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-streamlit
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STREAMLIT [ IF NOT EXISTS ] <name>
//	  [ FROM <source_location> ]
//	  [ MAIN_FILE = '<filename>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ RUNTIME_NAME = '<runtime_name>' ]
//	  [ COMPUTE_POOL = <compute_pool_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ TITLE = '<app_title>' ]
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [ , ... ] ) ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ SECRETS = ( '<snowflake_secret_name>' = <snowflake_secret> [ , ... ] ) ]
//
//	CREATE [ OR REPLACE ] STREAMLIT [ IF NOT EXISTS ] <name>
//	  ROOT_LOCATION = '<stage_path_and_root_directory>'
//	  MAIN_FILE = '<path_to_main_file_in_root_directory>'
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ TITLE = '<app_title>' ]
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [ , ... ] ) ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
func (v *Validator) ParseCreateStreamlit() bool {
	return true
}

// ParseCreateTable validates the Snowflake `CREATE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ]
//	    [ { [ { LOCAL | GLOBAL } ] TEMP | TEMPORARY | VOLATILE | TRANSIENT } ]
//	  TABLE [ IF NOT EXISTS ] <table_name>
//	  (
//	    <col_name> <col_type> [ AS ( <expr> ) ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ COLLATE '<collation_specification>' ]
//	      [
//	        {
//	          DEFAULT <expr>
//	          | { AUTOINCREMENT | IDENTITY }
//	            [ { ( <start_num> , <step_num> ) | START <num> INCREMENT <num> } ]
//	            [ { ORDER | NOORDER } ]
//	        }
//	      ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ AS ( <expr> ) ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ ENABLE_SCHEMA_EVOLUTION = { TRUE | FALSE } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COPY GRANTS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COPY TAGS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//
//	-- Variant forms:
//	-- CREATE TABLE ... AS <query>                       (CTAS)
//	-- CREATE TABLE ... USING TEMPLATE <query>
//	-- CREATE TABLE <table_name> LIKE <source_table>
//	-- CREATE TABLE <name> CLONE <source_table> [ { AT | BEFORE } ( ... ) ]
//	-- CREATE [ TRANSIENT ] TABLE <name> FROM ARCHIVE OF <source_table> WHERE <expression>
func (v *Validator) ParseCreateTable() bool {
	return true
}

// ParseCreateAlterTableConstraint validates the Snowflake `CREATE | ALTER TABLE CONSTRAINT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table-constraint
//
// Syntax:
//
//	CREATE TABLE <name> (
//	  <col1_name> <col1_type>  [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH }
//	  [ , <col2_name> <col2_type> [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH } ]
//	  [ , ... ]
//	)
//
//	ALTER TABLE <name> ADD COLUMN
//	  <col_name> <col_type> [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH }
//
//	inlineUniquePK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY }
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//
//	inlineFK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  [ FOREIGN KEY ]
//	  REFERENCES <ref_table_name> [ ( <ref_col_name> ) ]
//	  [ MATCH { FULL | SIMPLE | PARTIAL } ]
//	  [ ON [ UPDATE { CASCADE | SET NULL | SET DEFAULT | RESTRICT | NO ACTION } ]
//	       [ DELETE { CASCADE | SET NULL | SET DEFAULT | RESTRICT | NO ACTION } ] ]
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//
//	inlineCH ::=
//	  [ CONSTRAINT <constraint_name> ] CHECK ( <expr> )
//	  [ ENABLE { VALIDATE | NOVALIDATE } ]
//
//	outoflineUniquePK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY } ( <col_name> [ , <col_name> , ... ] )
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineFK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  FOREIGN KEY ( <col_name> [ , <col_name> , ... ] )
//	  REFERENCES <ref_table_name> [ ( <ref_col_name> [ , <ref_col_name> , ... ] ) ]
//	  [ MATCH { FULL | SIMPLE | PARTIAL } ]
//	  [ ON [ UPDATE { ... } ] [ DELETE { ... } ] ]
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineCH ::=
//	  [ CONSTRAINT <constraint_name> ] CHECK ( <expr> )
//	  [ ENABLE { VALIDATE | NOVALIDATE } ]
func (v *Validator) ParseCreateAlterTableConstraint() bool {
	return true
}

// ParseCreateTag validates the Snowflake `CREATE TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-tag
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TAG [ IF NOT EXISTS ] <name>
//	    [ ALLOWED_VALUES '<val_1>' [ , '<val_2>' [ , ... ] ] ]
//	    [ PROPAGATE = { ON_DEPENDENCY_AND_DATA_MOVEMENT | ON_DEPENDENCY | ON_DATA_MOVEMENT }
//	      [ ON_CONFLICT = { '<string>' | ALLOWED_VALUES_SEQUENCE } ] ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateTag() bool {
	return true
}

// ParseCreateTask validates the Snowflake `CREATE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-task
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TASK [ IF NOT EXISTS ] <name>
//	    [ WITH TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	    [ { WAREHOUSE = <string> }
//	      | { USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = <string> } ]
//	    [ SCHEDULE = { '<num> { HOURS | MINUTES | SECONDS }'
//	      | 'USING CRON <expr> <time_zone>' } ]
//	    [ CONFIG = <configuration_string> ]
//	    [ OVERLAP_POLICY = { NO_OVERLAP | ALLOW_CHILD_OVERLAP | ALLOW_ALL_OVERLAP } ]
//	    [ <session_parameter> = <value> [ , <session_parameter> = <value> ... ] ]
//	    [ USER_TASK_TIMEOUT_MS = <num> ]
//	    [ SUSPEND_TASK_AFTER_NUM_FAILURES = <num> ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//	    [ SUCCESS_INTEGRATION = <integration_name> ]
//	    [ LOG_LEVEL = '<log_level>' ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ FINALIZE = <string> ]
//	    [ TASK_AUTO_RETRY_ATTEMPTS = <num> ]
//	    [ USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = <num> ]
//	    [ TARGET_COMPLETION_INTERVAL = '<num> { HOURS | MINUTES | SECONDS }' ]
//	    [ SERVERLESS_TASK_MIN_STATEMENT_SIZE = '{ XSMALL | ... | XXLARGE }' ]
//	    [ SERVERLESS_TASK_MAX_STATEMENT_SIZE = '{ XSMALL | ... | XXLARGE }' ]
//	  [ AFTER <string> [ , <string> , ... ] ]
//	  [ EXECUTE AS USER <user_name> ]
//	  [ WHEN <boolean_expr> ]
//	  AS
//	    <sql>
func (v *Validator) ParseCreateTask() bool {
	return true
}

// ParseCreateType validates the Snowflake `CREATE TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-type
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TYPE [ IF NOT EXISTS ] <name> AS <type>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateType() bool {
	return true
}

// ParseCreateUser validates the Snowflake `CREATE USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-user
//
// Syntax:
//
//	CREATE [ OR REPLACE ] USER [ IF NOT EXISTS ] <name>
//	  [ objectProperties ]
//	  [ objectParams ]
//	  [ sessionParams ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateUser() bool {
	return true
}

// ParseCreateOrAlterVersionedSchema validates the Snowflake `CREATE OR ALTER VERSIONED SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-versioned-schema
//
// Syntax:
//
//	CREATE OR ALTER VERSIONED SCHEMA <name>
//	  [ WITH MANAGED ACCESS ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateOrAlterVersionedSchema() bool {
	return true
}

// ParseCreateView validates the Snowflake `CREATE VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] [ { [ { LOCAL | GLOBAL } ] TEMP | TEMPORARY | VOLATILE } ] [ RECURSIVE ] VIEW [ IF NOT EXISTS ] <name>
//	  [ ( <column_list> ) ]
//	  [ <col1> [ WITH ] MASKING POLICY <policy_name> [ USING ( <col1> , <cond_col1> , ... ) ]
//	           [ WITH ] PROJECTION POLICY <policy_name>
//	           [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ , <col2> [ ... ] ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  AS <select_statement>
func (v *Validator) ParseCreateView() bool {
	return true
}

// ParseCreateWarehouse validates the Snowflake `CREATE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-warehouse
//
// Syntax:
//
//	CREATE [ OR REPLACE ] WAREHOUSE [ IF NOT EXISTS ] <name>
//	       [ [ WITH ] objectProperties ]
//	       [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	       [ objectParams ]
//
//	Where:
//
//	objectProperties ::=
//	  WAREHOUSE_TYPE = { STANDARD | 'SNOWPARK-OPTIMIZED' | ADAPTIVE }
//	  WAREHOUSE_SIZE = { XSMALL | SMALL | MEDIUM | LARGE | XLARGE | XXLARGE | XXXLARGE | X4LARGE | X5LARGE | X6LARGE }
//	  GENERATION = { '1' | '2' }
//	  RESOURCE_CONSTRAINT = { STANDARD_GEN_1 | STANDARD_GEN_2 | MEMORY_1X | MEMORY_1X_x86 | MEMORY_16X | MEMORY_16X_x86 | MEMORY_64X | MEMORY_64X_x86 }
//	  MAX_CLUSTER_COUNT = <num>
//	  MIN_CLUSTER_COUNT = <num>
//	  SCALING_POLICY = { STANDARD | ECONOMY }
//	  AUTO_SUSPEND = { <num> | NULL }
//	  AUTO_RESUME = { TRUE | FALSE }
//	  INITIALLY_SUSPENDED = { TRUE | FALSE }
//	  RESOURCE_MONITOR = <monitor_name>
//	  COMMENT = '<string_literal>'
//	  ENABLE_QUERY_ACCELERATION = { TRUE | FALSE }
//	  QUERY_ACCELERATION_MAX_SCALE_FACTOR = <num>
//
//	objectParams ::=
//	  MAX_CONCURRENCY_LEVEL = <num>
//	  STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = <num>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
func (v *Validator) ParseCreateWarehouse() bool {
	return true
}

// ParseCreateApplicationService validates the Snowflake `CREATE APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-service
//
// Syntax:
//
//	CREATE APPLICATION SERVICE [ IF NOT EXISTS ] <name>
//	  FROM ARTIFACT REPOSITORY <repository_name> PACKAGE <package_name>
//	  [ VERSION <version_alias> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND_SECS = <num> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateApplicationService() bool {
	return true
}

// ParseCreateArtifactRepository validates the Snowflake `CREATE ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-artifact-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ARTIFACT REPOSITORY [ IF NOT EXISTS ] <name>
//	  TYPE = { APPLICATION | PYPI }
//	  [ API_INTEGRATION = '<integration_name>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateArtifactRepository() bool {
	return true
}

// ParseCreateCatalogIntegrationDeltaSharing validates the Snowflake `CREATE CATALOG INTEGRATION (Delta Sharing)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-delta-sharing
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = DELTA_SHARING
//	  TABLE_FORMAT = DELTA
//	  REST_CONFIG = (
//	    CATALOG_URI = '<delta_sharing_endpoint_url>'
//	    CATALOG_NAME = 'shares/<share_name>'
//	    ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS
//	  )
//	  REST_AUTHENTICATION = (
//	    restAuthenticationParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	Where restAuthenticationParams is one of:
//
//	restAuthenticationParams (for Bearer token) ::=
//	  TYPE = BEARER
//	  BEARER_TOKEN = '<bearer_token>'
//
//	restAuthenticationParams (for OIDC) ::=
//	  TYPE = OIDC
//	  OIDC_AUDIENCE = '<audience>'
//
//	restAuthenticationParams (for OAuth) ::=
//	  TYPE = OAUTH
//	  OAUTH_CLIENT_ID = '<oauth_client_id>'
//	  OAUTH_CLIENT_SECRET = '<oauth_client_secret>'
//	  OAUTH_TOKEN_URI = 'https://<token_server_uri>'
func (v *Validator) ParseCreateCatalogIntegrationDeltaSharing() bool {
	return true
}

// ParseCreateCatalogIntegrationSnowflakePostgres validates the Snowflake `CREATE CATALOG INTEGRATION (Snowflake Postgres)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-snowflake-postgres
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = SNOWFLAKE_POSTGRES
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<namespace>' ]
//	  REST_CONFIG = (
//	    restConfigParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	restConfigParams ::=
//	  POSTGRES_INSTANCE = '<instance_name>'
//	  ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS
//	  [ CATALOG_NAME = '<database_name>' ]
func (v *Validator) ParseCreateCatalogIntegrationSnowflakePostgres() bool {
	return true
}

// ParseCreateEventRoutingTable validates the Snowflake `CREATE EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-event-routing-table
//
// Syntax:
//
//	CREATE EVENT ROUTING TABLE <table_name>
//	   WITH RULES
//	    {rule name} = (REGION_GROUP={region group}, REGIONS=('{region1}', '{region2}', ...), DESTINATION_ACCOUNT = {organization}.{account_name})
//	    ...
func (v *Validator) ParseCreateEventRoutingTable() bool {
	return true
}

// ParseCreateStorageIntegrationPostgresInternal validates the Snowflake `CREATE STORAGE INTEGRATION (Postgres internal)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-integration-postgres-internal
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = POSTGRES_INTERNAL_STORAGE
//	  POSTGRES_INSTANCE = '<instance_name>'
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateStorageIntegrationPostgresInternal() bool {
	return true
}
