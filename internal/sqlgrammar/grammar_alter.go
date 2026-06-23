package sqlgrammar

// ALTER commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseAlterObj validates the Snowflake `ALTER <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter
func (v *Validator) ParseAlterObj() bool {
	return true
}

// ParseAlterAccount validates the Snowflake `ALTER ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-account
func (v *Validator) ParseAlterAccount() bool {
	return true
}

// ParseAlterAgent validates the Snowflake `ALTER AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-agent
func (v *Validator) ParseAlterAgent() bool {
	return true
}

// ParseAlterAggregationPolicy validates the Snowflake `ALTER AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-aggregation-policy
func (v *Validator) ParseAlterAggregationPolicy() bool {
	return true
}

// ParseAlterAlert validates the Snowflake `ALTER ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-alert
func (v *Validator) ParseAlterAlert() bool {
	return true
}

// ParseAlterApiIntegration validates the Snowflake `ALTER API INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-api-integration
func (v *Validator) ParseAlterApiIntegration() bool {
	return true
}

// ParseAlterApplication validates the Snowflake `ALTER APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application
func (v *Validator) ParseAlterApplication() bool {
	return true
}

// ParseAlterApplicationDropSpecification validates the Snowflake `ALTER APPLICATION DROP SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-drop-app-spec
func (v *Validator) ParseAlterApplicationDropSpecification() bool {
	return true
}

// ParseAlterApplicationDropConfigurationDefinition validates the Snowflake `ALTER APPLICATION DROP CONFIGURATION DEFINITION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-drop-configuration-definition
func (v *Validator) ParseAlterApplicationDropConfigurationDefinition() bool {
	return true
}

// ParseAlterApplicationPackage validates the Snowflake `ALTER APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package
func (v *Validator) ParseAlterApplicationPackage() bool {
	return true
}

// ParseAlterApplicationPackageModifyReleaseChannel validates the Snowflake `ALTER APPLICATION PACKAGE MODIFY RELEASE CHANNEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-release-channel
func (v *Validator) ParseAlterApplicationPackageModifyReleaseChannel() bool {
	return true
}

// ParseAlterApplicationPackageReleaseDirective validates the Snowflake `ALTER APPLICATION PACKAGE RELEASE DIRECTIVE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-release-directive
func (v *Validator) ParseAlterApplicationPackageReleaseDirective() bool {
	return true
}

// ParseAlterApplicationPackageVersion validates the Snowflake `ALTER APPLICATION PACKAGE VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-version
func (v *Validator) ParseAlterApplicationPackageVersion() bool {
	return true
}

// ParseAlterApplicationRole validates the Snowflake `ALTER APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-role
func (v *Validator) ParseAlterApplicationRole() bool {
	return true
}

// ParseAlterApplicationApproveDeclineSpecification validates the Snowflake `ALTER APPLICATION APPROVE DECLINE SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-sequence-number
func (v *Validator) ParseAlterApplicationApproveDeclineSpecification() bool {
	return true
}

// ParseAlterApplicationSetSpecification validates the Snowflake `ALTER APPLICATION SET SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-app-spec
func (v *Validator) ParseAlterApplicationSetSpecification() bool {
	return true
}

// ParseAlterApplicationSetConfigurationDefinition validates the Snowflake `ALTER APPLICATION SET CONFIGURATION DEFINITION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-configuration-definition
func (v *Validator) ParseAlterApplicationSetConfigurationDefinition() bool {
	return true
}

// ParseAlterApplicationSetConfigurationValue validates the Snowflake `ALTER APPLICATION SET CONFIGURATION VALUE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-configuration-value
func (v *Validator) ParseAlterApplicationSetConfigurationValue() bool {
	return true
}

// ParseAlterApplicationUnsetConfiguration validates the Snowflake `ALTER APPLICATION UNSET CONFIGURATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-unset-configuration
func (v *Validator) ParseAlterApplicationUnsetConfiguration() bool {
	return true
}

// ParseAlterAuthenticationPolicy validates the Snowflake `ALTER AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-authentication-policy
func (v *Validator) ParseAlterAuthenticationPolicy() bool {
	return true
}

// ParseAlterBackupPolicy validates the Snowflake `ALTER BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-backup-policy
func (v *Validator) ParseAlterBackupPolicy() bool {
	return true
}

// ParseAlterBackupSet validates the Snowflake `ALTER BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-backup-set
func (v *Validator) ParseAlterBackupSet() bool {
	return true
}

// ParseAlterCatalogIntegration validates the Snowflake `ALTER CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-catalog-integration
func (v *Validator) ParseAlterCatalogIntegration() bool {
	return true
}

// ParseAlterComputePool validates the Snowflake `ALTER COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-compute-pool
func (v *Validator) ParseAlterComputePool() bool {
	return true
}

// ParseAlterConnection validates the Snowflake `ALTER CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-connection
func (v *Validator) ParseAlterConnection() bool {
	return true
}

// ParseAlterContact validates the Snowflake `ALTER CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-contact
func (v *Validator) ParseAlterContact() bool {
	return true
}

// ParseAlterCortexSearchService validates the Snowflake `ALTER CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-cortex-search
func (v *Validator) ParseAlterCortexSearchService() bool {
	return true
}

// ParseAlterDatabase validates the Snowflake `ALTER DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database
func (v *Validator) ParseAlterDatabase() bool {
	return true
}

// ParseAlterDatabaseCatalogLinked validates the Snowflake `ALTER DATABASE (catalog-linked)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database-catalog-linked
func (v *Validator) ParseAlterDatabaseCatalogLinked() bool {
	return true
}

// ParseAlterDatabaseRole validates the Snowflake `ALTER DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database-role
func (v *Validator) ParseAlterDatabaseRole() bool {
	return true
}

// ParseAlterDataset validates the Snowflake `ALTER DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset
func (v *Validator) ParseAlterDataset() bool {
	return true
}

// ParseAlterDatasetAddVersion validates the Snowflake `ALTER DATASET ADD VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset-add-version
func (v *Validator) ParseAlterDatasetAddVersion() bool {
	return true
}

// ParseAlterDatasetDropVersion validates the Snowflake `ALTER DATASET DROP VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset-drop-version
func (v *Validator) ParseAlterDatasetDropVersion() bool {
	return true
}

// ParseAlterDbtProject validates the Snowflake `ALTER DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dbt-project
func (v *Validator) ParseAlterDbtProject() bool {
	return true
}

// ParseAlterDcmProject validates the Snowflake `ALTER DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dcm-project
func (v *Validator) ParseAlterDcmProject() bool {
	return true
}

// ParseAlterDynamicTable validates the Snowflake `ALTER DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dynamic-table
func (v *Validator) ParseAlterDynamicTable() bool {
	return true
}

// ParseAlterExperiment validates the Snowflake `ALTER EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-experiment
func (v *Validator) ParseAlterExperiment() bool {
	return true
}

// ParseAlterExternalAgent validates the Snowflake `ALTER EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-agent
func (v *Validator) ParseAlterExternalAgent() bool {
	return true
}

// ParseAlterExternalAccessIntegration validates the Snowflake `ALTER EXTERNAL ACCESS INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-access-integration
func (v *Validator) ParseAlterExternalAccessIntegration() bool {
	return true
}

// ParseAlterExternalTable validates the Snowflake `ALTER EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-table
func (v *Validator) ParseAlterExternalTable() bool {
	return true
}

// ParseAlterExternalVolume validates the Snowflake `ALTER EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-volume
func (v *Validator) ParseAlterExternalVolume() bool {
	return true
}

// ParseAlterFailoverGroup validates the Snowflake `ALTER FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-failover-group
func (v *Validator) ParseAlterFailoverGroup() bool {
	return true
}

// ParseAlterFeaturePolicy validates the Snowflake `ALTER FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-feature-policy
func (v *Validator) ParseAlterFeaturePolicy() bool {
	return true
}

// ParseAlterFileFormat validates the Snowflake `ALTER FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-file-format
func (v *Validator) ParseAlterFileFormat() bool {
	return true
}

// ParseAlterFunction validates the Snowflake `ALTER FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function
func (v *Validator) ParseAlterFunction() bool {
	return true
}

// ParseAlterFunctionDmf validates the Snowflake `ALTER FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function-dmf
func (v *Validator) ParseAlterFunctionDmf() bool {
	return true
}

// ParseAlterFunctionSnowparkContainerServices validates the Snowflake `ALTER FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function-spcs
func (v *Validator) ParseAlterFunctionSnowparkContainerServices() bool {
	return true
}

// ParseAlterGateway validates the Snowflake `ALTER GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-gateway
func (v *Validator) ParseAlterGateway() bool {
	return true
}

// ParseAlterGitRepository validates the Snowflake `ALTER GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-git-repository
func (v *Validator) ParseAlterGitRepository() bool {
	return true
}

// ParseAlterIcebergTable validates the Snowflake `ALTER ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table
func (v *Validator) ParseAlterIcebergTable() bool {
	return true
}

// ParseAlterIcebergTableAlterColumnSetDataType validates the Snowflake `ALTER ICEBERG TABLE ALTER COLUMN SET DATA TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-alter-column-set-data-type
func (v *Validator) ParseAlterIcebergTableAlterColumnSetDataType() bool {
	return true
}

// ParseAlterIcebergTableConvertToManaged validates the Snowflake `ALTER ICEBERG TABLE CONVERT TO MANAGED` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-convert-to-managed
func (v *Validator) ParseAlterIcebergTableConvertToManaged() bool {
	return true
}

// ParseAlterIcebergTableRefresh validates the Snowflake `ALTER ICEBERG TABLE REFRESH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-refresh
func (v *Validator) ParseAlterIcebergTableRefresh() bool {
	return true
}

// ParseAlterIntegration validates the Snowflake `ALTER INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-integration
func (v *Validator) ParseAlterIntegration() bool {
	return true
}

// ParseAlterJoinPolicy validates the Snowflake `ALTER JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-join-policy
func (v *Validator) ParseAlterJoinPolicy() bool {
	return true
}

// ParseAlterListing validates the Snowflake `ALTER LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-listing
func (v *Validator) ParseAlterListing() bool {
	return true
}

// ParseAlterMaintenancePolicy validates the Snowflake `ALTER MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-maintenance-policy
func (v *Validator) ParseAlterMaintenancePolicy() bool {
	return true
}

// ParseAlterMaskingPolicy validates the Snowflake `ALTER MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-masking-policy
func (v *Validator) ParseAlterMaskingPolicy() bool {
	return true
}

// ParseAlterMaterializedView validates the Snowflake `ALTER MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-materialized-view
func (v *Validator) ParseAlterMaterializedView() bool {
	return true
}

// ParseAlterModel validates the Snowflake `ALTER MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model
func (v *Validator) ParseAlterModel() bool {
	return true
}

// ParseAlterModelAddVersion validates the Snowflake `ALTER MODEL ADD VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-add-version
func (v *Validator) ParseAlterModelAddVersion() bool {
	return true
}

// ParseAlterModelDropVersion validates the Snowflake `ALTER MODEL DROP VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-drop-version
func (v *Validator) ParseAlterModelDropVersion() bool {
	return true
}

// ParseAlterModelModifyVersion validates the Snowflake `ALTER MODEL MODIFY VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-modify-version
func (v *Validator) ParseAlterModelModifyVersion() bool {
	return true
}

// ParseAlterModelMonitor validates the Snowflake `ALTER MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-monitor
func (v *Validator) ParseAlterModelMonitor() bool {
	return true
}

// ParseAlterNetworkPolicy validates the Snowflake `ALTER NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-network-policy
func (v *Validator) ParseAlterNetworkPolicy() bool {
	return true
}

// ParseAlterNetworkRule validates the Snowflake `ALTER NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-network-rule
func (v *Validator) ParseAlterNetworkRule() bool {
	return true
}

// ParseAlterNotebook validates the Snowflake `ALTER NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notebook
func (v *Validator) ParseAlterNotebook() bool {
	return true
}

// ParseAlterNotificationIntegration validates the Snowflake `ALTER NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration
func (v *Validator) ParseAlterNotificationIntegration() bool {
	return true
}

// ParseAlterNotificationIntegrationEmail validates the Snowflake `ALTER NOTIFICATION INTEGRATION (email)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-email
func (v *Validator) ParseAlterNotificationIntegrationEmail() bool {
	return true
}

// ParseAlterNotificationIntegrationInboundAzureEventGrid validates the Snowflake `ALTER NOTIFICATION INTEGRATION (inbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-inbound-azure
func (v *Validator) ParseAlterNotificationIntegrationInboundAzureEventGrid() bool {
	return true
}

// ParseAlterNotificationIntegrationInboundGooglePubSub validates the Snowflake `ALTER NOTIFICATION INTEGRATION (inbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-inbound-gcp
func (v *Validator) ParseAlterNotificationIntegrationInboundGooglePubSub() bool {
	return true
}

// ParseAlterNotificationIntegrationOutboundAmazonSns validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Amazon SNS)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-aws
func (v *Validator) ParseAlterNotificationIntegrationOutboundAmazonSns() bool {
	return true
}

// ParseAlterNotificationIntegrationOutboundAzureEventGrid validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-azure
func (v *Validator) ParseAlterNotificationIntegrationOutboundAzureEventGrid() bool {
	return true
}

// ParseAlterNotificationIntegrationOutboundGooglePubSub validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-gcp
func (v *Validator) ParseAlterNotificationIntegrationOutboundGooglePubSub() bool {
	return true
}

// ParseAlterNotificationIntegrationWebhooks validates the Snowflake `ALTER NOTIFICATION INTEGRATION (webhooks)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-webhooks
func (v *Validator) ParseAlterNotificationIntegrationWebhooks() bool {
	return true
}

// ParseAlterOpenflowDataPlane validates the Snowflake `ALTER OPENFLOW DATA PLANE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-oflow-data-plane
func (v *Validator) ParseAlterOpenflowDataPlane() bool {
	return true
}

// ParseAlterOnlineFeatureTable validates the Snowflake `ALTER ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-online-feature-table
func (v *Validator) ParseAlterOnlineFeatureTable() bool {
	return true
}

// ParseAlterOrganizationAccount validates the Snowflake `ALTER ORGANIZATION ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-account
func (v *Validator) ParseAlterOrganizationAccount() bool {
	return true
}

// ParseAlterOrganizationProfile validates the Snowflake `ALTER ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-profile
func (v *Validator) ParseAlterOrganizationProfile() bool {
	return true
}

// ParseAlterOrganizationUser validates the Snowflake `ALTER ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-user
func (v *Validator) ParseAlterOrganizationUser() bool {
	return true
}

// ParseAlterOrganizationUserGroup validates the Snowflake `ALTER ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-user-group
func (v *Validator) ParseAlterOrganizationUserGroup() bool {
	return true
}

// ParseAlterPackagesPolicy validates the Snowflake `ALTER PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-packages-policy
func (v *Validator) ParseAlterPackagesPolicy() bool {
	return true
}

// ParseAlterPasswordPolicy validates the Snowflake `ALTER PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-password-policy
func (v *Validator) ParseAlterPasswordPolicy() bool {
	return true
}

// ParseAlterPipe validates the Snowflake `ALTER PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-pipe
func (v *Validator) ParseAlterPipe() bool {
	return true
}

// ParseAlterPostgresInstance validates the Snowflake `ALTER POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-postgres-instance
func (v *Validator) ParseAlterPostgresInstance() bool {
	return true
}

// ParseAlterPrivacyPolicy validates the Snowflake `ALTER PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-privacy-policy
func (v *Validator) ParseAlterPrivacyPolicy() bool {
	return true
}

// ParseAlterProcedure validates the Snowflake `ALTER PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-procedure
func (v *Validator) ParseAlterProcedure() bool {
	return true
}

// ParseAlterProjectionPolicy validates the Snowflake `ALTER PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-projection-policy
func (v *Validator) ParseAlterProjectionPolicy() bool {
	return true
}

// ParseAlterReplicationGroup validates the Snowflake `ALTER REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-replication-group
func (v *Validator) ParseAlterReplicationGroup() bool {
	return true
}

// ParseAlterResourceMonitor validates the Snowflake `ALTER RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-resource-monitor
func (v *Validator) ParseAlterResourceMonitor() bool {
	return true
}

// ParseAlterRole validates the Snowflake `ALTER ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-role
func (v *Validator) ParseAlterRole() bool {
	return true
}

// ParseAlterRowAccessPolicy validates the Snowflake `ALTER ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-row-access-policy
func (v *Validator) ParseAlterRowAccessPolicy() bool {
	return true
}

// ParseAlterSchema validates the Snowflake `ALTER SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-schema
func (v *Validator) ParseAlterSchema() bool {
	return true
}

// ParseAlterSecret validates the Snowflake `ALTER SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-secret
func (v *Validator) ParseAlterSecret() bool {
	return true
}

// ParseAlterSecurityIntegration validates the Snowflake `ALTER SECURITY INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration
func (v *Validator) ParseAlterSecurityIntegration() bool {
	return true
}

// ParseAlterSecurityIntegrationExternalApiAuthentication validates the Snowflake `ALTER SECURITY INTEGRATION (External API Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-api-auth
func (v *Validator) ParseAlterSecurityIntegrationExternalApiAuthentication() bool {
	return true
}

// ParseAlterSecurityIntegrationAwsIamAuthentication validates the Snowflake `ALTER SECURITY INTEGRATION (AWS IAM Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-aws-iam
func (v *Validator) ParseAlterSecurityIntegrationAwsIamAuthentication() bool {
	return true
}

// ParseAlterSecurityIntegrationExternalOauth validates the Snowflake `ALTER SECURITY INTEGRATION (External OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-oauth-external
func (v *Validator) ParseAlterSecurityIntegrationExternalOauth() bool {
	return true
}

// ParseAlterSecurityIntegrationSnowflakeOauth validates the Snowflake `ALTER SECURITY INTEGRATION (Snowflake OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-oauth-snowflake
func (v *Validator) ParseAlterSecurityIntegrationSnowflakeOauth() bool {
	return true
}

// ParseAlterSecurityIntegrationSaml2 validates the Snowflake `ALTER SECURITY INTEGRATION (SAML2)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-saml2
func (v *Validator) ParseAlterSecurityIntegrationSaml2() bool {
	return true
}

// ParseAlterSecurityIntegrationScim validates the Snowflake `ALTER SECURITY INTEGRATION (SCIM)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-scim
func (v *Validator) ParseAlterSecurityIntegrationScim() bool {
	return true
}

// ParseAlterSemanticView validates the Snowflake `ALTER SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-semantic-view
func (v *Validator) ParseAlterSemanticView() bool {
	return true
}

// ParseAlterSequence validates the Snowflake `ALTER SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-sequence
func (v *Validator) ParseAlterSequence() bool {
	return true
}

// ParseAlterService validates the Snowflake `ALTER SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-service
func (v *Validator) ParseAlterService() bool {
	return true
}

// ParseAlterSession validates the Snowflake `ALTER SESSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-session
func (v *Validator) ParseAlterSession() bool {
	return true
}

// ParseAlterSessionPolicy validates the Snowflake `ALTER SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-session-policy
func (v *Validator) ParseAlterSessionPolicy() bool {
	return true
}

// ParseAlterShare validates the Snowflake `ALTER SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-share
func (v *Validator) ParseAlterShare() bool {
	return true
}

// ParseAlterSnapshot validates the Snowflake `ALTER SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot
func (v *Validator) ParseAlterSnapshot() bool {
	return true
}

// ParseAlterSnapshotPolicy validates the Snowflake `ALTER SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot-policy
func (v *Validator) ParseAlterSnapshotPolicy() bool {
	return true
}

// ParseAlterSnapshotSet validates the Snowflake `ALTER SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot-set
func (v *Validator) ParseAlterSnapshotSet() bool {
	return true
}

// ParseAlterStage validates the Snowflake `ALTER STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-stage
func (v *Validator) ParseAlterStage() bool {
	return true
}

// ParseAlterStorageIntegration validates the Snowflake `ALTER STORAGE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-storage-integration
func (v *Validator) ParseAlterStorageIntegration() bool {
	return true
}

// ParseAlterStorageLifecyclePolicy validates the Snowflake `ALTER STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-storage-lifecycle-policy
func (v *Validator) ParseAlterStorageLifecyclePolicy() bool {
	return true
}

// ParseAlterStream validates the Snowflake `ALTER STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-stream
func (v *Validator) ParseAlterStream() bool {
	return true
}

// ParseAlterStreamlit validates the Snowflake `ALTER STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-streamlit
func (v *Validator) ParseAlterStreamlit() bool {
	return true
}

// ParseAlterTable validates the Snowflake `ALTER TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table
func (v *Validator) ParseAlterTable() bool {
	return true
}

// ParseAlterTableAlterColumn validates the Snowflake `ALTER TABLE ALTER COLUMN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table-column
func (v *Validator) ParseAlterTableAlterColumn() bool {
	return true
}

// ParseAlterTableEventTables validates the Snowflake `ALTER TABLE (event tables)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table-event-table
func (v *Validator) ParseAlterTableEventTables() bool {
	return true
}

// ParseAlterTag validates the Snowflake `ALTER TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-tag
func (v *Validator) ParseAlterTag() bool {
	return true
}

// ParseAlterTask validates the Snowflake `ALTER TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-task
func (v *Validator) ParseAlterTask() bool {
	return true
}

// ParseAlterType validates the Snowflake `ALTER TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-type
func (v *Validator) ParseAlterType() bool {
	return true
}

// ParseAlterUser validates the Snowflake `ALTER USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user
func (v *Validator) ParseAlterUser() bool {
	return true
}

// ParseAlterUserAddProgrammaticAccessToken validates the Snowflake `ALTER USER ADD PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-add-programmatic-access-token
func (v *Validator) ParseAlterUserAddProgrammaticAccessToken() bool {
	return true
}

// ParseAlterUserModifyProgrammaticAccessToken validates the Snowflake `ALTER USER MODIFY PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-modify-programmatic-access-token
func (v *Validator) ParseAlterUserModifyProgrammaticAccessToken() bool {
	return true
}

// ParseAlterUserRemoveProgrammaticAccessToken validates the Snowflake `ALTER USER REMOVE PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-remove-programmatic-access-token
func (v *Validator) ParseAlterUserRemoveProgrammaticAccessToken() bool {
	return true
}

// ParseAlterUserRotateProgrammaticAccessToken validates the Snowflake `ALTER USER ROTATE PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-rotate-programmatic-access-token
func (v *Validator) ParseAlterUserRotateProgrammaticAccessToken() bool {
	return true
}

// ParseAlterView validates the Snowflake `ALTER VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-view
func (v *Validator) ParseAlterView() bool {
	return true
}

// ParseAlterWarehouse validates the Snowflake `ALTER WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-warehouse
func (v *Validator) ParseAlterWarehouse() bool {
	return true
}
