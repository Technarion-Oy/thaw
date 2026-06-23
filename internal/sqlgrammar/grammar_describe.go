package sqlgrammar

// DESCRIBE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDescribeObj validates the Snowflake `DESCRIBE <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc
func (v *Validator) ParseDescribeObj() bool {
	return true
}

// ParseDescribeAgent validates the Snowflake `DESCRIBE AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-agent
func (v *Validator) ParseDescribeAgent() bool {
	return true
}

// ParseDescribeAggregationPolicy validates the Snowflake `DESCRIBE AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-aggregation-policy
func (v *Validator) ParseDescribeAggregationPolicy() bool {
	return true
}

// ParseDescribeAlert validates the Snowflake `DESCRIBE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-alert
func (v *Validator) ParseDescribeAlert() bool {
	return true
}

// ParseDescribeApplication validates the Snowflake `DESCRIBE APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-application
func (v *Validator) ParseDescribeApplication() bool {
	return true
}

// ParseDescribeApplicationPackage validates the Snowflake `DESCRIBE APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-application-package
func (v *Validator) ParseDescribeApplicationPackage() bool {
	return true
}

// ParseDescribeAuthenticationPolicy validates the Snowflake `DESCRIBE AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-authentication-policy
func (v *Validator) ParseDescribeAuthenticationPolicy() bool {
	return true
}

// ParseDescribeAvailableListing validates the Snowflake `DESCRIBE AVAILABLE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-available-listing
func (v *Validator) ParseDescribeAvailableListing() bool {
	return true
}

// ParseDescribeAvailableOrganizationProfile validates the Snowflake `DESCRIBE AVAILABLE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-available-organization-profile
func (v *Validator) ParseDescribeAvailableOrganizationProfile() bool {
	return true
}

// ParseDescribeBackupPolicy validates the Snowflake `DESCRIBE BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-backup-policy
func (v *Validator) ParseDescribeBackupPolicy() bool {
	return true
}

// ParseDescribeBackupSet validates the Snowflake `DESCRIBE BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-backup-set
func (v *Validator) ParseDescribeBackupSet() bool {
	return true
}

// ParseDescribeCatalogIntegration validates the Snowflake `DESCRIBE CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-catalog-integration
func (v *Validator) ParseDescribeCatalogIntegration() bool {
	return true
}

// ParseDescribeComputePool validates the Snowflake `DESCRIBE COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-compute-pool
func (v *Validator) ParseDescribeComputePool() bool {
	return true
}

// ParseDescribeConfiguration validates the Snowflake `DESCRIBE CONFIGURATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-configuration
func (v *Validator) ParseDescribeConfiguration() bool {
	return true
}

// ParseDescribeCortexSearchService validates the Snowflake `DESCRIBE CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-cortex-search
func (v *Validator) ParseDescribeCortexSearchService() bool {
	return true
}

// ParseDescribeDatabase validates the Snowflake `DESCRIBE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-database
func (v *Validator) ParseDescribeDatabase() bool {
	return true
}

// ParseDescribeDbtProject validates the Snowflake `DESCRIBE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dbt-project
func (v *Validator) ParseDescribeDbtProject() bool {
	return true
}

// ParseDescribeDcmProject validates the Snowflake `DESCRIBE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dcm-project
func (v *Validator) ParseDescribeDcmProject() bool {
	return true
}

// ParseDescribeDynamicTable validates the Snowflake `DESCRIBE DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dynamic-table
func (v *Validator) ParseDescribeDynamicTable() bool {
	return true
}

// ParseDescribeEventTable validates the Snowflake `DESCRIBE EVENT TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-event-table
func (v *Validator) ParseDescribeEventTable() bool {
	return true
}

// ParseDescribeExternalAgent validates the Snowflake `DESCRIBE EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-agent
func (v *Validator) ParseDescribeExternalAgent() bool {
	return true
}

// ParseDescribeExternalTable validates the Snowflake `DESCRIBE EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-table
func (v *Validator) ParseDescribeExternalTable() bool {
	return true
}

// ParseDescribeExternalVolume validates the Snowflake `DESCRIBE EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-volume
func (v *Validator) ParseDescribeExternalVolume() bool {
	return true
}

// ParseDescribeFeaturePolicy validates the Snowflake `DESCRIBE FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-feature-policy
func (v *Validator) ParseDescribeFeaturePolicy() bool {
	return true
}

// ParseDescribeFileFormat validates the Snowflake `DESCRIBE FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-file-format
func (v *Validator) ParseDescribeFileFormat() bool {
	return true
}

// ParseDescribeFunction validates the Snowflake `DESCRIBE FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function
func (v *Validator) ParseDescribeFunction() bool {
	return true
}

// ParseDescribeFunctionDmf validates the Snowflake `DESCRIBE FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function-dmf
func (v *Validator) ParseDescribeFunctionDmf() bool {
	return true
}

// ParseDescribeFunctionSnowparkContainerServices validates the Snowflake `DESCRIBE FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function-spcs
func (v *Validator) ParseDescribeFunctionSnowparkContainerServices() bool {
	return true
}

// ParseDescribeGateway validates the Snowflake `DESCRIBE GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-gateway
func (v *Validator) ParseDescribeGateway() bool {
	return true
}

// ParseDescribeGitRepository validates the Snowflake `DESCRIBE GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-git-repository
func (v *Validator) ParseDescribeGitRepository() bool {
	return true
}

// ParseDescribeIcebergTable validates the Snowflake `DESCRIBE ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-iceberg-table
func (v *Validator) ParseDescribeIcebergTable() bool {
	return true
}

// ParseDescribeIntegration validates the Snowflake `DESCRIBE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-integration
func (v *Validator) ParseDescribeIntegration() bool {
	return true
}

// ParseDescribeJoinPolicy validates the Snowflake `DESCRIBE JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-join-policy
func (v *Validator) ParseDescribeJoinPolicy() bool {
	return true
}

// ParseDescribeListing validates the Snowflake `DESCRIBE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-listing
func (v *Validator) ParseDescribeListing() bool {
	return true
}

// ParseDescribeMaintenancePolicy validates the Snowflake `DESCRIBE MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-maintenance-policy
func (v *Validator) ParseDescribeMaintenancePolicy() bool {
	return true
}

// ParseDescribeMaskingPolicy validates the Snowflake `DESCRIBE MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-masking-policy
func (v *Validator) ParseDescribeMaskingPolicy() bool {
	return true
}

// ParseDescribeMaterializedView validates the Snowflake `DESCRIBE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-materialized-view
func (v *Validator) ParseDescribeMaterializedView() bool {
	return true
}

// ParseDescribeMcpServer validates the Snowflake `DESCRIBE MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-mcp-server
func (v *Validator) ParseDescribeMcpServer() bool {
	return true
}

// ParseDescribeModelMonitor validates the Snowflake `DESCRIBE MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-model-monitor
func (v *Validator) ParseDescribeModelMonitor() bool {
	return true
}

// ParseDescribeNetworkPolicy validates the Snowflake `DESCRIBE NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-network-policy
func (v *Validator) ParseDescribeNetworkPolicy() bool {
	return true
}

// ParseDescribeNetworkRule validates the Snowflake `DESCRIBE NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-network-rule
func (v *Validator) ParseDescribeNetworkRule() bool {
	return true
}

// ParseDescribeNotebook validates the Snowflake `DESCRIBE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-notebook
func (v *Validator) ParseDescribeNotebook() bool {
	return true
}

// ParseDescribeNotificationIntegration validates the Snowflake `DESCRIBE NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-notification-integration
func (v *Validator) ParseDescribeNotificationIntegration() bool {
	return true
}

// ParseDescribeOpenflowDataPlaneIntegration validates the Snowflake `DESCRIBE OPENFLOW DATA PLANE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-oflow-data-plane-integration
func (v *Validator) ParseDescribeOpenflowDataPlaneIntegration() bool {
	return true
}

// ParseDescribeOnlineFeatureTable validates the Snowflake `DESCRIBE ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-online-feature-table
func (v *Validator) ParseDescribeOnlineFeatureTable() bool {
	return true
}

// ParseDescribeOrganizationProfile validates the Snowflake `DESCRIBE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-organization-profile
func (v *Validator) ParseDescribeOrganizationProfile() bool {
	return true
}

// ParseDescribePackagesPolicy validates the Snowflake `DESCRIBE PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-packages-policy
func (v *Validator) ParseDescribePackagesPolicy() bool {
	return true
}

// ParseDescribePasswordPolicy validates the Snowflake `DESCRIBE PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-password-policy
func (v *Validator) ParseDescribePasswordPolicy() bool {
	return true
}

// ParseDescribePipe validates the Snowflake `DESCRIBE PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-pipe
func (v *Validator) ParseDescribePipe() bool {
	return true
}

// ParseDescribePostgresInstance validates the Snowflake `DESCRIBE POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-postgres-instance
func (v *Validator) ParseDescribePostgresInstance() bool {
	return true
}

// ParseDescribePrivacyPolicy validates the Snowflake `DESCRIBE PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-privacy-policy
func (v *Validator) ParseDescribePrivacyPolicy() bool {
	return true
}

// ParseDescribeProcedure validates the Snowflake `DESCRIBE PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-procedure
func (v *Validator) ParseDescribeProcedure() bool {
	return true
}

// ParseDescribeProjectionPolicy validates the Snowflake `DESCRIBE PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-projection-policy
func (v *Validator) ParseDescribeProjectionPolicy() bool {
	return true
}

// ParseDescribeResult validates the Snowflake `DESCRIBE RESULT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-result
func (v *Validator) ParseDescribeResult() bool {
	return true
}

// ParseDescribeRowAccessPolicy validates the Snowflake `DESCRIBE ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-row-access-policy
func (v *Validator) ParseDescribeRowAccessPolicy() bool {
	return true
}

// ParseDescribeSchema validates the Snowflake `DESCRIBE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-schema
func (v *Validator) ParseDescribeSchema() bool {
	return true
}

// ParseDescribeSearchOptimization validates the Snowflake `DESCRIBE SEARCH OPTIMIZATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-search-optimization
func (v *Validator) ParseDescribeSearchOptimization() bool {
	return true
}

// ParseDescribeSecret validates the Snowflake `DESCRIBE SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-secret
func (v *Validator) ParseDescribeSecret() bool {
	return true
}

// ParseDescribeSemanticView validates the Snowflake `DESCRIBE SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-semantic-view
func (v *Validator) ParseDescribeSemanticView() bool {
	return true
}

// ParseDescribeSequence validates the Snowflake `DESCRIBE SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-sequence
func (v *Validator) ParseDescribeSequence() bool {
	return true
}

// ParseDescribeService validates the Snowflake `DESCRIBE SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-service
func (v *Validator) ParseDescribeService() bool {
	return true
}

// ParseDescribeSessionPolicy validates the Snowflake `DESCRIBE SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-session-policy
func (v *Validator) ParseDescribeSessionPolicy() bool {
	return true
}

// ParseDescribeShare validates the Snowflake `DESCRIBE SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-share
func (v *Validator) ParseDescribeShare() bool {
	return true
}

// ParseDescribeSnapshot validates the Snowflake `DESCRIBE SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot
func (v *Validator) ParseDescribeSnapshot() bool {
	return true
}

// ParseDescribeSnapshotPolicy validates the Snowflake `DESCRIBE SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot-policy
func (v *Validator) ParseDescribeSnapshotPolicy() bool {
	return true
}

// ParseDescribeSnapshotSet validates the Snowflake `DESCRIBE SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot-set
func (v *Validator) ParseDescribeSnapshotSet() bool {
	return true
}

// ParseDescribeSpecification validates the Snowflake `DESCRIBE SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-specification
func (v *Validator) ParseDescribeSpecification() bool {
	return true
}

// ParseDescribeStage validates the Snowflake `DESCRIBE STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-stage
func (v *Validator) ParseDescribeStage() bool {
	return true
}

// ParseDescribeStorageLifecyclePolicy validates the Snowflake `DESCRIBE STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-storage-lifecycle-policy
func (v *Validator) ParseDescribeStorageLifecyclePolicy() bool {
	return true
}

// ParseDescribeStream validates the Snowflake `DESCRIBE STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-stream
func (v *Validator) ParseDescribeStream() bool {
	return true
}

// ParseDescribeStreamlit validates the Snowflake `DESCRIBE STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-streamlit
func (v *Validator) ParseDescribeStreamlit() bool {
	return true
}

// ParseDescribeTable validates the Snowflake `DESCRIBE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-table
func (v *Validator) ParseDescribeTable() bool {
	return true
}

// ParseDescribeTask validates the Snowflake `DESCRIBE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-task
func (v *Validator) ParseDescribeTask() bool {
	return true
}

// ParseDescribeTransaction validates the Snowflake `DESCRIBE TRANSACTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-transaction
func (v *Validator) ParseDescribeTransaction() bool {
	return true
}

// ParseDescribeType validates the Snowflake `DESCRIBE TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-type
func (v *Validator) ParseDescribeType() bool {
	return true
}

// ParseDescribeUser validates the Snowflake `DESCRIBE USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-user
func (v *Validator) ParseDescribeUser() bool {
	return true
}

// ParseDescribeView validates the Snowflake `DESCRIBE VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-view
func (v *Validator) ParseDescribeView() bool {
	return true
}

// ParseDescribeWarehouse validates the Snowflake `DESCRIBE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-warehouse
func (v *Validator) ParseDescribeWarehouse() bool {
	return true
}
