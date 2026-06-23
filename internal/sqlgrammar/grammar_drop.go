package sqlgrammar

// DROP commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDropObj validates the Snowflake `DROP <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop
func (v *Validator) ParseDropObj() bool {
	return true
}

// ParseDropAccount validates the Snowflake `DROP ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-account
func (v *Validator) ParseDropAccount() bool {
	return true
}

// ParseDropAgent validates the Snowflake `DROP AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-agent
func (v *Validator) ParseDropAgent() bool {
	return true
}

// ParseDropAggregationPolicy validates the Snowflake `DROP AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-aggregation-policy
func (v *Validator) ParseDropAggregationPolicy() bool {
	return true
}

// ParseDropAlert validates the Snowflake `DROP ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-alert
func (v *Validator) ParseDropAlert() bool {
	return true
}

// ParseDropApplication validates the Snowflake `DROP APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application
func (v *Validator) ParseDropApplication() bool {
	return true
}

// ParseDropApplicationPackage validates the Snowflake `DROP APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-package
func (v *Validator) ParseDropApplicationPackage() bool {
	return true
}

// ParseDropApplicationRole validates the Snowflake `DROP APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-role
func (v *Validator) ParseDropApplicationRole() bool {
	return true
}

// ParseDropAuthenticationPolicy validates the Snowflake `DROP AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-authentication-policy
func (v *Validator) ParseDropAuthenticationPolicy() bool {
	return true
}

// ParseDropBackupPolicy validates the Snowflake `DROP BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-policy
func (v *Validator) ParseDropBackupPolicy() bool {
	return true
}

// ParseDropBackupSet validates the Snowflake `DROP BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-set
func (v *Validator) ParseDropBackupSet() bool {
	return true
}

// ParseDropCatalogIntegration validates the Snowflake `DROP CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-catalog-integration
func (v *Validator) ParseDropCatalogIntegration() bool {
	return true
}

// ParseDropComputePool validates the Snowflake `DROP COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-compute-pool
func (v *Validator) ParseDropComputePool() bool {
	return true
}

// ParseDropConnection validates the Snowflake `DROP CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-connection
func (v *Validator) ParseDropConnection() bool {
	return true
}

// ParseDropContact validates the Snowflake `DROP CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-contact
func (v *Validator) ParseDropContact() bool {
	return true
}

// ParseDropCortexSearchService validates the Snowflake `DROP CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-cortex-search
func (v *Validator) ParseDropCortexSearchService() bool {
	return true
}

// ParseDropDatabase validates the Snowflake `DROP DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database
func (v *Validator) ParseDropDatabase() bool {
	return true
}

// ParseDropDatabaseRole validates the Snowflake `DROP DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database-role
func (v *Validator) ParseDropDatabaseRole() bool {
	return true
}

// ParseDropDbtProject validates the Snowflake `DROP DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dbt-project
func (v *Validator) ParseDropDbtProject() bool {
	return true
}

// ParseDropDcmProject validates the Snowflake `DROP DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dcm-project
func (v *Validator) ParseDropDcmProject() bool {
	return true
}

// ParseDropDynamicTable validates the Snowflake `DROP DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dynamic-table
func (v *Validator) ParseDropDynamicTable() bool {
	return true
}

// ParseDropExperiment validates the Snowflake `DROP EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-experiment
func (v *Validator) ParseDropExperiment() bool {
	return true
}

// ParseDropExternalAgent validates the Snowflake `DROP EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-agent
func (v *Validator) ParseDropExternalAgent() bool {
	return true
}

// ParseDropExternalTable validates the Snowflake `DROP EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-table
func (v *Validator) ParseDropExternalTable() bool {
	return true
}

// ParseDropExternalVolume validates the Snowflake `DROP EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-volume
func (v *Validator) ParseDropExternalVolume() bool {
	return true
}

// ParseDropFailoverGroup validates the Snowflake `DROP FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-failover-group
func (v *Validator) ParseDropFailoverGroup() bool {
	return true
}

// ParseDropFeaturePolicy validates the Snowflake `DROP FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-feature-policy
func (v *Validator) ParseDropFeaturePolicy() bool {
	return true
}

// ParseDropFileFormat validates the Snowflake `DROP FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-file-format
func (v *Validator) ParseDropFileFormat() bool {
	return true
}

// ParseDropFunction validates the Snowflake `DROP FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function
func (v *Validator) ParseDropFunction() bool {
	return true
}

// ParseDropFunctionDmf validates the Snowflake `DROP FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function-dmf
func (v *Validator) ParseDropFunctionDmf() bool {
	return true
}

// ParseDropFunctionSnowparkContainerServices validates the Snowflake `DROP FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function-spcs
func (v *Validator) ParseDropFunctionSnowparkContainerServices() bool {
	return true
}

// ParseDropGateway validates the Snowflake `DROP GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-gateway
func (v *Validator) ParseDropGateway() bool {
	return true
}

// ParseDropGitRepository validates the Snowflake `DROP GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-git-repository
func (v *Validator) ParseDropGitRepository() bool {
	return true
}

// ParseDropIcebergTable validates the Snowflake `DROP ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-iceberg-table
func (v *Validator) ParseDropIcebergTable() bool {
	return true
}

// ParseDropImageRepository validates the Snowflake `DROP IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-image-repository
func (v *Validator) ParseDropImageRepository() bool {
	return true
}

// ParseDropIndex validates the Snowflake `DROP INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-index
func (v *Validator) ParseDropIndex() bool {
	return true
}

// ParseDropIntegration validates the Snowflake `DROP INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-integration
func (v *Validator) ParseDropIntegration() bool {
	return true
}

// ParseDropJoinPolicy validates the Snowflake `DROP JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-join-policy
func (v *Validator) ParseDropJoinPolicy() bool {
	return true
}

// ParseDropListing validates the Snowflake `DROP LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-listing
func (v *Validator) ParseDropListing() bool {
	return true
}

// ParseDropMaintenancePolicy validates the Snowflake `DROP MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-maintenance-policy
func (v *Validator) ParseDropMaintenancePolicy() bool {
	return true
}

// ParseDropManagedAccount validates the Snowflake `DROP MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-managed-account
func (v *Validator) ParseDropManagedAccount() bool {
	return true
}

// ParseDropMaskingPolicy validates the Snowflake `DROP MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-masking-policy
func (v *Validator) ParseDropMaskingPolicy() bool {
	return true
}

// ParseDropMaterializedView validates the Snowflake `DROP MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-materialized-view
func (v *Validator) ParseDropMaterializedView() bool {
	return true
}

// ParseDropMcpServer validates the Snowflake `DROP MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-mcp-server
func (v *Validator) ParseDropMcpServer() bool {
	return true
}

// ParseDropModel validates the Snowflake `DROP MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model
func (v *Validator) ParseDropModel() bool {
	return true
}

// ParseDropModelMonitor validates the Snowflake `DROP MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model-monitor
func (v *Validator) ParseDropModelMonitor() bool {
	return true
}

// ParseDropNetworkPolicy validates the Snowflake `DROP NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-policy
func (v *Validator) ParseDropNetworkPolicy() bool {
	return true
}

// ParseDropNetworkRule validates the Snowflake `DROP NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-rule
func (v *Validator) ParseDropNetworkRule() bool {
	return true
}

// ParseDropNotebook validates the Snowflake `DROP NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-notebook
func (v *Validator) ParseDropNotebook() bool {
	return true
}

// ParseDropOnlineFeatureTable validates the Snowflake `DROP ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-online-feature-table
func (v *Validator) ParseDropOnlineFeatureTable() bool {
	return true
}

// ParseDropOrganizationProfile validates the Snowflake `DROP ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-profile
func (v *Validator) ParseDropOrganizationProfile() bool {
	return true
}

// ParseDropOrganizationUser validates the Snowflake `DROP ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user
func (v *Validator) ParseDropOrganizationUser() bool {
	return true
}

// ParseDropOrganizationUserGroup validates the Snowflake `DROP ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user-group
func (v *Validator) ParseDropOrganizationUserGroup() bool {
	return true
}

// ParseDropPackagesPolicy validates the Snowflake `DROP PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-packages-policy
func (v *Validator) ParseDropPackagesPolicy() bool {
	return true
}

// ParseDropPasswordPolicy validates the Snowflake `DROP PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-password-policy
func (v *Validator) ParseDropPasswordPolicy() bool {
	return true
}

// ParseDropPipe validates the Snowflake `DROP PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-pipe
func (v *Validator) ParseDropPipe() bool {
	return true
}

// ParseDropPostgresInstance validates the Snowflake `DROP POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-postgres-instance
func (v *Validator) ParseDropPostgresInstance() bool {
	return true
}

// ParseDropPrivacyPolicy validates the Snowflake `DROP PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-privacy-policy
func (v *Validator) ParseDropPrivacyPolicy() bool {
	return true
}

// ParseDropProcedure validates the Snowflake `DROP PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-procedure
func (v *Validator) ParseDropProcedure() bool {
	return true
}

// ParseDropProjectionPolicy validates the Snowflake `DROP PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-projection-policy
func (v *Validator) ParseDropProjectionPolicy() bool {
	return true
}

// ParseDropReplicationGroup validates the Snowflake `DROP REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-replication-group
func (v *Validator) ParseDropReplicationGroup() bool {
	return true
}

// ParseDropResourceMonitor validates the Snowflake `DROP RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-resource-monitor
func (v *Validator) ParseDropResourceMonitor() bool {
	return true
}

// ParseDropRole validates the Snowflake `DROP ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-role
func (v *Validator) ParseDropRole() bool {
	return true
}

// ParseDropRowAccessPolicy validates the Snowflake `DROP ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-row-access-policy
func (v *Validator) ParseDropRowAccessPolicy() bool {
	return true
}

// ParseDropSchema validates the Snowflake `DROP SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-schema
func (v *Validator) ParseDropSchema() bool {
	return true
}

// ParseDropSecret validates the Snowflake `DROP SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-secret
func (v *Validator) ParseDropSecret() bool {
	return true
}

// ParseDropSemanticView validates the Snowflake `DROP SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-semantic-view
func (v *Validator) ParseDropSemanticView() bool {
	return true
}

// ParseDropSequence validates the Snowflake `DROP SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-sequence
func (v *Validator) ParseDropSequence() bool {
	return true
}

// ParseDropService validates the Snowflake `DROP SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-service
func (v *Validator) ParseDropService() bool {
	return true
}

// ParseDropSessionPolicy validates the Snowflake `DROP SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-session-policy
func (v *Validator) ParseDropSessionPolicy() bool {
	return true
}

// ParseDropShare validates the Snowflake `DROP SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-share
func (v *Validator) ParseDropShare() bool {
	return true
}

// ParseDropSnapshot validates the Snowflake `DROP SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot
func (v *Validator) ParseDropSnapshot() bool {
	return true
}

// ParseDropSnapshotPolicy validates the Snowflake `DROP SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-policy
func (v *Validator) ParseDropSnapshotPolicy() bool {
	return true
}

// ParseDropSnapshotSet validates the Snowflake `DROP SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-set
func (v *Validator) ParseDropSnapshotSet() bool {
	return true
}

// ParseDropStage validates the Snowflake `DROP STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stage
func (v *Validator) ParseDropStage() bool {
	return true
}

// ParseDropStorageLifecyclePolicy validates the Snowflake `DROP STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-storage-lifecycle-policy
func (v *Validator) ParseDropStorageLifecyclePolicy() bool {
	return true
}

// ParseDropStream validates the Snowflake `DROP STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stream
func (v *Validator) ParseDropStream() bool {
	return true
}

// ParseDropStreamlit validates the Snowflake `DROP STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-streamlit
func (v *Validator) ParseDropStreamlit() bool {
	return true
}

// ParseDropTable validates the Snowflake `DROP TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-table
func (v *Validator) ParseDropTable() bool {
	return true
}

// ParseDropTag validates the Snowflake `DROP TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-tag
func (v *Validator) ParseDropTag() bool {
	return true
}

// ParseDropTask validates the Snowflake `DROP TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-task
func (v *Validator) ParseDropTask() bool {
	return true
}

// ParseDropType validates the Snowflake `DROP TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-type
func (v *Validator) ParseDropType() bool {
	return true
}

// ParseDropUser validates the Snowflake `DROP USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-user
func (v *Validator) ParseDropUser() bool {
	return true
}

// ParseDropView validates the Snowflake `DROP VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-view
func (v *Validator) ParseDropView() bool {
	return true
}

// ParseDropWarehouse validates the Snowflake `DROP WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-warehouse
func (v *Validator) ParseDropWarehouse() bool {
	return true
}

// ParseDropApplicationService validates the Snowflake `DROP APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-service
func (v *Validator) ParseDropApplicationService() bool {
	return true
}

// ParseDropArtifactRepository validates the Snowflake `DROP ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-artifact-repository
func (v *Validator) ParseDropArtifactRepository() bool {
	return true
}

// ParseDropEventRoutingTable validates the Snowflake `DROP EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-event-routing-table
func (v *Validator) ParseDropEventRoutingTable() bool {
	return true
}
