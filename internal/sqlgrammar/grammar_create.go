package sqlgrammar

// CREATE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCreateObj validates the Snowflake `CREATE <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create
func (v *Validator) ParseCreateObj() bool {
	return true
}

// ParseCreateAccount validates the Snowflake `CREATE ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-account
func (v *Validator) ParseCreateAccount() bool {
	return true
}

// ParseCreateAgent validates the Snowflake `CREATE AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-agent
func (v *Validator) ParseCreateAgent() bool {
	return true
}

// ParseCreateAggregationPolicy validates the Snowflake `CREATE AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-aggregation-policy
func (v *Validator) ParseCreateAggregationPolicy() bool {
	return true
}

// ParseCreateAlert validates the Snowflake `CREATE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-alert
func (v *Validator) ParseCreateAlert() bool {
	return true
}

// ParseCreateApiIntegration validates the Snowflake `CREATE API INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-api-integration
func (v *Validator) ParseCreateApiIntegration() bool {
	return true
}

// ParseCreateApplication validates the Snowflake `CREATE APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application
func (v *Validator) ParseCreateApplication() bool {
	return true
}

// ParseCreateApplicationPackage validates the Snowflake `CREATE APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-package
func (v *Validator) ParseCreateApplicationPackage() bool {
	return true
}

// ParseCreateApplicationRole validates the Snowflake `CREATE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-role
func (v *Validator) ParseCreateApplicationRole() bool {
	return true
}

// ParseCreateAuthenticationPolicy validates the Snowflake `CREATE AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-authentication-policy
func (v *Validator) ParseCreateAuthenticationPolicy() bool {
	return true
}

// ParseCreateBackupPolicy validates the Snowflake `CREATE BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-policy
func (v *Validator) ParseCreateBackupPolicy() bool {
	return true
}

// ParseCreateBackupSet validates the Snowflake `CREATE BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-set
func (v *Validator) ParseCreateBackupSet() bool {
	return true
}

// ParseCreateCatalogIntegration validates the Snowflake `CREATE CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration
func (v *Validator) ParseCreateCatalogIntegration() bool {
	return true
}

// ParseCreateCatalogIntegrationAwsGlue validates the Snowflake `CREATE CATALOG INTEGRATION (AWS Glue)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-glue
func (v *Validator) ParseCreateCatalogIntegrationAwsGlue() bool {
	return true
}

// ParseCreateCatalogIntegrationObjectStorage validates the Snowflake `CREATE CATALOG INTEGRATION (Object storage)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-object-storage
func (v *Validator) ParseCreateCatalogIntegrationObjectStorage() bool {
	return true
}

// ParseCreateCatalogIntegrationSnowflakeOpenCatalog validates the Snowflake `CREATE CATALOG INTEGRATION (Snowflake Open Catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-open-catalog
func (v *Validator) ParseCreateCatalogIntegrationSnowflakeOpenCatalog() bool {
	return true
}

// ParseCreateCatalogIntegrationApacheIcebergRest validates the Snowflake `CREATE CATALOG INTEGRATION (Apache Iceberg REST)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-rest
func (v *Validator) ParseCreateCatalogIntegrationApacheIcebergRest() bool {
	return true
}

// ParseCreateObjClone validates the Snowflake `CREATE <object> CLONE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-clone
func (v *Validator) ParseCreateObjClone() bool {
	return true
}

// ParseCreateComputePool validates the Snowflake `CREATE COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-compute-pool
func (v *Validator) ParseCreateComputePool() bool {
	return true
}

// ParseCreateConnection validates the Snowflake `CREATE CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-connection
func (v *Validator) ParseCreateConnection() bool {
	return true
}

// ParseCreateContact validates the Snowflake `CREATE CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-contact
func (v *Validator) ParseCreateContact() bool {
	return true
}

// ParseCreateCortexSearchService validates the Snowflake `CREATE CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-cortex-search
func (v *Validator) ParseCreateCortexSearchService() bool {
	return true
}

// ParseCreateDataMetricFunction validates the Snowflake `CREATE DATA METRIC FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-data-metric-function
func (v *Validator) ParseCreateDataMetricFunction() bool {
	return true
}

// ParseCreateDatabase validates the Snowflake `CREATE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database
func (v *Validator) ParseCreateDatabase() bool {
	return true
}

// ParseCreateDatabaseCatalogLinked validates the Snowflake `CREATE DATABASE (catalog-linked)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-catalog-linked
func (v *Validator) ParseCreateDatabaseCatalogLinked() bool {
	return true
}

// ParseCreateDatabaseRole validates the Snowflake `CREATE DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-role
func (v *Validator) ParseCreateDatabaseRole() bool {
	return true
}

// ParseCreateDataset validates the Snowflake `CREATE DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dataset
func (v *Validator) ParseCreateDataset() bool {
	return true
}

// ParseCreateDbtProject validates the Snowflake `CREATE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dbt-project
func (v *Validator) ParseCreateDbtProject() bool {
	return true
}

// ParseCreateDcmProject validates the Snowflake `CREATE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dcm-project
func (v *Validator) ParseCreateDcmProject() bool {
	return true
}

// ParseCreateDynamicTable validates the Snowflake `CREATE DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dynamic-table
func (v *Validator) ParseCreateDynamicTable() bool {
	return true
}

// ParseCreateEventTable validates the Snowflake `CREATE EVENT TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-event-table
func (v *Validator) ParseCreateEventTable() bool {
	return true
}

// ParseCreateExperiment validates the Snowflake `CREATE EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-experiment
func (v *Validator) ParseCreateExperiment() bool {
	return true
}

// ParseCreateExternalAgent validates the Snowflake `CREATE EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-agent
func (v *Validator) ParseCreateExternalAgent() bool {
	return true
}

// ParseCreateExternalAccessIntegration validates the Snowflake `CREATE EXTERNAL ACCESS INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-access-integration
func (v *Validator) ParseCreateExternalAccessIntegration() bool {
	return true
}

// ParseCreateExternalFunction validates the Snowflake `CREATE EXTERNAL FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-function
func (v *Validator) ParseCreateExternalFunction() bool {
	return true
}

// ParseCreateExternalTable validates the Snowflake `CREATE EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-table
func (v *Validator) ParseCreateExternalTable() bool {
	return true
}

// ParseCreateExternalVolume validates the Snowflake `CREATE EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-volume
func (v *Validator) ParseCreateExternalVolume() bool {
	return true
}

// ParseCreateFailoverGroup validates the Snowflake `CREATE FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-failover-group
func (v *Validator) ParseCreateFailoverGroup() bool {
	return true
}

// ParseCreateFeaturePolicy validates the Snowflake `CREATE FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-feature-policy
func (v *Validator) ParseCreateFeaturePolicy() bool {
	return true
}

// ParseCreateFileFormat validates the Snowflake `CREATE FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-file-format
func (v *Validator) ParseCreateFileFormat() bool {
	return true
}

// ParseCreateFunction validates the Snowflake `CREATE FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function
func (v *Validator) ParseCreateFunction() bool {
	return true
}

// ParseCreateFunctionSnowparkContainerServices validates the Snowflake `CREATE FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function-spcs
func (v *Validator) ParseCreateFunctionSnowparkContainerServices() bool {
	return true
}

// ParseCreateGateway validates the Snowflake `CREATE GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-gateway
func (v *Validator) ParseCreateGateway() bool {
	return true
}

// ParseCreateGitRepository validates the Snowflake `CREATE GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-git-repository
func (v *Validator) ParseCreateGitRepository() bool {
	return true
}

// ParseCreateHybridTable validates the Snowflake `CREATE HYBRID TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-hybrid-table
func (v *Validator) ParseCreateHybridTable() bool {
	return true
}

// ParseCreateIcebergTable validates the Snowflake `CREATE ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table
func (v *Validator) ParseCreateIcebergTable() bool {
	return true
}

// ParseCreateIcebergTableAwsGlueCatalog validates the Snowflake `CREATE ICEBERG TABLE (AWS Glue catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-aws-glue
func (v *Validator) ParseCreateIcebergTableAwsGlueCatalog() bool {
	return true
}

// ParseCreateIcebergTableDeltaFiles validates the Snowflake `CREATE ICEBERG TABLE (Delta files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-delta
func (v *Validator) ParseCreateIcebergTableDeltaFiles() bool {
	return true
}

// ParseCreateIcebergTableIcebergFiles validates the Snowflake `CREATE ICEBERG TABLE (Iceberg files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-iceberg-files
func (v *Validator) ParseCreateIcebergTableIcebergFiles() bool {
	return true
}

// ParseCreateIcebergTableIcebergRestCatalog validates the Snowflake `CREATE ICEBERG TABLE (Iceberg REST catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-rest
func (v *Validator) ParseCreateIcebergTableIcebergRestCatalog() bool {
	return true
}

// ParseCreateIcebergTableSnowflakeCatalog validates the Snowflake `CREATE ICEBERG TABLE (Snowflake catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-snowflake
func (v *Validator) ParseCreateIcebergTableSnowflakeCatalog() bool {
	return true
}

// ParseCreateImageRepository validates the Snowflake `CREATE IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-image-repository
func (v *Validator) ParseCreateImageRepository() bool {
	return true
}

// ParseCreateIndex validates the Snowflake `CREATE INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-index
func (v *Validator) ParseCreateIndex() bool {
	return true
}

// ParseCreateIntegration validates the Snowflake `CREATE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-integration
func (v *Validator) ParseCreateIntegration() bool {
	return true
}

// ParseCreateInteractiveTable validates the Snowflake `CREATE INTERACTIVE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-table
func (v *Validator) ParseCreateInteractiveTable() bool {
	return true
}

// ParseCreateInteractiveWarehouse validates the Snowflake `CREATE INTERACTIVE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-warehouse
func (v *Validator) ParseCreateInteractiveWarehouse() bool {
	return true
}

// ParseCreateJoinPolicy validates the Snowflake `CREATE JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-join-policy
func (v *Validator) ParseCreateJoinPolicy() bool {
	return true
}

// ParseCreateListing validates the Snowflake `CREATE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-listing
func (v *Validator) ParseCreateListing() bool {
	return true
}

// ParseCreateMaintenancePolicy validates the Snowflake `CREATE MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-maintenance-policy
func (v *Validator) ParseCreateMaintenancePolicy() bool {
	return true
}

// ParseCreateManagedAccount validates the Snowflake `CREATE MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-managed-account
func (v *Validator) ParseCreateManagedAccount() bool {
	return true
}

// ParseCreateMaskingPolicy validates the Snowflake `CREATE MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-masking-policy
func (v *Validator) ParseCreateMaskingPolicy() bool {
	return true
}

// ParseCreateMaterializedView validates the Snowflake `CREATE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-materialized-view
func (v *Validator) ParseCreateMaterializedView() bool {
	return true
}

// ParseCreateMcpServer validates the Snowflake `CREATE MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-mcp-server
func (v *Validator) ParseCreateMcpServer() bool {
	return true
}

// ParseCreateModel validates the Snowflake `CREATE MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model
func (v *Validator) ParseCreateModel() bool {
	return true
}

// ParseCreateModelMonitor validates the Snowflake `CREATE MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model-monitor
func (v *Validator) ParseCreateModelMonitor() bool {
	return true
}

// ParseCreateNetworkPolicy validates the Snowflake `CREATE NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-policy
func (v *Validator) ParseCreateNetworkPolicy() bool {
	return true
}

// ParseCreateNetworkRule validates the Snowflake `CREATE NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-rule
func (v *Validator) ParseCreateNetworkRule() bool {
	return true
}

// ParseCreateNotebook validates the Snowflake `CREATE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook
func (v *Validator) ParseCreateNotebook() bool {
	return true
}

// ParseCreateNotebookProject validates the Snowflake `CREATE NOTEBOOK PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook-project
func (v *Validator) ParseCreateNotebookProject() bool {
	return true
}

// ParseCreateNotificationIntegration validates the Snowflake `CREATE NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration
func (v *Validator) ParseCreateNotificationIntegration() bool {
	return true
}

// ParseCreateNotificationIntegrationEmail validates the Snowflake `CREATE NOTIFICATION INTEGRATION (email)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-email
func (v *Validator) ParseCreateNotificationIntegrationEmail() bool {
	return true
}

// ParseCreateNotificationIntegrationInboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-azure
func (v *Validator) ParseCreateNotificationIntegrationInboundAzureEventGrid() bool {
	return true
}

// ParseCreateNotificationIntegrationInboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-gcp
func (v *Validator) ParseCreateNotificationIntegrationInboundGooglePubSub() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundAmazonSns validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Amazon SNS)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-aws
func (v *Validator) ParseCreateNotificationIntegrationOutboundAmazonSns() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-azure
func (v *Validator) ParseCreateNotificationIntegrationOutboundAzureEventGrid() bool {
	return true
}

// ParseCreateNotificationIntegrationOutboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-gcp
func (v *Validator) ParseCreateNotificationIntegrationOutboundGooglePubSub() bool {
	return true
}

// ParseCreateNotificationIntegrationWebhooks validates the Snowflake `CREATE NOTIFICATION INTEGRATION (webhooks)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-webhooks
func (v *Validator) ParseCreateNotificationIntegrationWebhooks() bool {
	return true
}

// ParseCreateOnlineFeatureTable validates the Snowflake `CREATE ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-online-feature-table
func (v *Validator) ParseCreateOnlineFeatureTable() bool {
	return true
}

// ParseCreateOrAlterObj validates the Snowflake `CREATE OR ALTER <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-or-alter
func (v *Validator) ParseCreateOrAlterObj() bool {
	return true
}

// ParseCreateOrganizationAccount validates the Snowflake `CREATE ORGANIZATION ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-account
func (v *Validator) ParseCreateOrganizationAccount() bool {
	return true
}

// ParseCreateOrganizationListing validates the Snowflake `CREATE ORGANIZATION LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-listing
func (v *Validator) ParseCreateOrganizationListing() bool {
	return true
}

// ParseCreateOrganizationProfile validates the Snowflake `CREATE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-profile
func (v *Validator) ParseCreateOrganizationProfile() bool {
	return true
}

// ParseCreateOrganizationUser validates the Snowflake `CREATE ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user
func (v *Validator) ParseCreateOrganizationUser() bool {
	return true
}

// ParseCreateOrganizationUserGroup validates the Snowflake `CREATE ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user-group
func (v *Validator) ParseCreateOrganizationUserGroup() bool {
	return true
}

// ParseCreatePackagesPolicy validates the Snowflake `CREATE PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-packages-policy
func (v *Validator) ParseCreatePackagesPolicy() bool {
	return true
}

// ParseCreatePasswordPolicy validates the Snowflake `CREATE PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-password-policy
func (v *Validator) ParseCreatePasswordPolicy() bool {
	return true
}

// ParseCreatePipe validates the Snowflake `CREATE PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-pipe
func (v *Validator) ParseCreatePipe() bool {
	return true
}

// ParseCreatePostgresInstance validates the Snowflake `CREATE POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-postgres-instance
func (v *Validator) ParseCreatePostgresInstance() bool {
	return true
}

// ParseCreatePrivacyPolicy validates the Snowflake `CREATE PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-privacy-policy
func (v *Validator) ParseCreatePrivacyPolicy() bool {
	return true
}

// ParseCreateProcedure validates the Snowflake `CREATE PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-procedure
func (v *Validator) ParseCreateProcedure() bool {
	return true
}

// ParseCreateProjectionPolicy validates the Snowflake `CREATE PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-projection-policy
func (v *Validator) ParseCreateProjectionPolicy() bool {
	return true
}

// ParseCreateProvisionedThroughput validates the Snowflake `CREATE PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-provisioned-throughput
func (v *Validator) ParseCreateProvisionedThroughput() bool {
	return true
}

// ParseCreateReplicationGroup validates the Snowflake `CREATE REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-replication-group
func (v *Validator) ParseCreateReplicationGroup() bool {
	return true
}

// ParseCreateResourceMonitor validates the Snowflake `CREATE RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-resource-monitor
func (v *Validator) ParseCreateResourceMonitor() bool {
	return true
}

// ParseCreateRole validates the Snowflake `CREATE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-role
func (v *Validator) ParseCreateRole() bool {
	return true
}

// ParseCreateRowAccessPolicy validates the Snowflake `CREATE ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-row-access-policy
func (v *Validator) ParseCreateRowAccessPolicy() bool {
	return true
}

// ParseCreateSchema validates the Snowflake `CREATE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-schema
func (v *Validator) ParseCreateSchema() bool {
	return true
}

// ParseCreateSecret validates the Snowflake `CREATE SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-secret
func (v *Validator) ParseCreateSecret() bool {
	return true
}

// ParseCreateSecurityIntegration validates the Snowflake `CREATE SECURITY INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration
func (v *Validator) ParseCreateSecurityIntegration() bool {
	return true
}

// ParseCreateSecurityIntegrationExternalApiAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (External API Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-api-auth
func (v *Validator) ParseCreateSecurityIntegrationExternalApiAuthentication() bool {
	return true
}

// ParseCreateSecurityIntegrationAwsIamAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (AWS IAM Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-aws-iam
func (v *Validator) ParseCreateSecurityIntegrationAwsIamAuthentication() bool {
	return true
}

// ParseCreateSecurityIntegrationExternalOauth validates the Snowflake `CREATE SECURITY INTEGRATION (External OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-external
func (v *Validator) ParseCreateSecurityIntegrationExternalOauth() bool {
	return true
}

// ParseCreateSecurityIntegrationSnowflakeOauth validates the Snowflake `CREATE SECURITY INTEGRATION (Snowflake OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-snowflake
func (v *Validator) ParseCreateSecurityIntegrationSnowflakeOauth() bool {
	return true
}

// ParseCreateSecurityIntegrationSaml2 validates the Snowflake `CREATE SECURITY INTEGRATION (SAML2)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-saml2
func (v *Validator) ParseCreateSecurityIntegrationSaml2() bool {
	return true
}

// ParseCreateSecurityIntegrationScim validates the Snowflake `CREATE SECURITY INTEGRATION (SCIM)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-scim
func (v *Validator) ParseCreateSecurityIntegrationScim() bool {
	return true
}

// ParseCreateSemanticView validates the Snowflake `CREATE SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-semantic-view
func (v *Validator) ParseCreateSemanticView() bool {
	return true
}

// ParseCreateSequence validates the Snowflake `CREATE SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-sequence
func (v *Validator) ParseCreateSequence() bool {
	return true
}

// ParseCreateService validates the Snowflake `CREATE SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-service
func (v *Validator) ParseCreateService() bool {
	return true
}

// ParseCreateSessionPolicy validates the Snowflake `CREATE SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-session-policy
func (v *Validator) ParseCreateSessionPolicy() bool {
	return true
}

// ParseCreateShare validates the Snowflake `CREATE SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-share
func (v *Validator) ParseCreateShare() bool {
	return true
}

// ParseCreateSnapshot validates the Snowflake `CREATE SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot
func (v *Validator) ParseCreateSnapshot() bool {
	return true
}

// ParseCreateSnapshotPolicy validates the Snowflake `CREATE SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-policy
func (v *Validator) ParseCreateSnapshotPolicy() bool {
	return true
}

// ParseCreateSnapshotSet validates the Snowflake `CREATE SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-set
func (v *Validator) ParseCreateSnapshotSet() bool {
	return true
}

// ParseCreateStage validates the Snowflake `CREATE STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stage
func (v *Validator) ParseCreateStage() bool {
	return true
}

// ParseCreateStorageIntegration validates the Snowflake `CREATE STORAGE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-integration
func (v *Validator) ParseCreateStorageIntegration() bool {
	return true
}

// ParseCreateStorageLifecyclePolicy validates the Snowflake `CREATE STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-lifecycle-policy
func (v *Validator) ParseCreateStorageLifecyclePolicy() bool {
	return true
}

// ParseCreateStream validates the Snowflake `CREATE STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stream
func (v *Validator) ParseCreateStream() bool {
	return true
}

// ParseCreateStreamlit validates the Snowflake `CREATE STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-streamlit
func (v *Validator) ParseCreateStreamlit() bool {
	return true
}

// ParseCreateTable validates the Snowflake `CREATE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table
func (v *Validator) ParseCreateTable() bool {
	return true
}

// ParseCreateAlterTableConstraint validates the Snowflake `CREATE | ALTER TABLE CONSTRAINT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table-constraint
func (v *Validator) ParseCreateAlterTableConstraint() bool {
	return true
}

// ParseCreateTag validates the Snowflake `CREATE TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-tag
func (v *Validator) ParseCreateTag() bool {
	return true
}

// ParseCreateTask validates the Snowflake `CREATE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-task
func (v *Validator) ParseCreateTask() bool {
	return true
}

// ParseCreateType validates the Snowflake `CREATE TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-type
func (v *Validator) ParseCreateType() bool {
	return true
}

// ParseCreateUser validates the Snowflake `CREATE USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-user
func (v *Validator) ParseCreateUser() bool {
	return true
}

// ParseCreateOrAlterVersionedSchema validates the Snowflake `CREATE OR ALTER VERSIONED SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-versioned-schema
func (v *Validator) ParseCreateOrAlterVersionedSchema() bool {
	return true
}

// ParseCreateView validates the Snowflake `CREATE VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-view
func (v *Validator) ParseCreateView() bool {
	return true
}

// ParseCreateWarehouse validates the Snowflake `CREATE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-warehouse
func (v *Validator) ParseCreateWarehouse() bool {
	return true
}
