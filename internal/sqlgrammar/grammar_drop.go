package sqlgrammar

// DROP commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDropObj validates the Snowflake `DROP <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseDropObj() bool {
	return true
}

// ParseDropAccount validates the Snowflake `DROP ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-account
//
// Syntax:
//
//	DROP ACCOUNT [ IF EXISTS ] <name> GRACE_PERIOD_IN_DAYS = <integer>
func (v *Validator) ParseDropAccount() bool {
	return true
}

// ParseDropAgent validates the Snowflake `DROP AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-agent
//
// Syntax:
//
//	DROP AGENT [ IF EXISTS ] <name>
func (v *Validator) ParseDropAgent() bool {
	return true
}

// ParseDropAggregationPolicy validates the Snowflake `DROP AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-aggregation-policy
//
// Syntax:
//
//	DROP AGGREGATION POLICY <name>
func (v *Validator) ParseDropAggregationPolicy() bool {
	return true
}

// ParseDropAlert validates the Snowflake `DROP ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-alert
//
// Syntax:
//
//	DROP ALERT [ IF EXISTS ] <name>
func (v *Validator) ParseDropAlert() bool {
	return true
}

// ParseDropApplication validates the Snowflake `DROP APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application
//
// Syntax:
//
//	DROP APPLICATION [ IF EXISTS ] <name> [ CASCADE ]
func (v *Validator) ParseDropApplication() bool {
	return true
}

// ParseDropApplicationPackage validates the Snowflake `DROP APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-package
//
// Syntax:
//
//	DROP APPLICATION PACKAGE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationPackage() bool {
	return true
}

// ParseDropApplicationRole validates the Snowflake `DROP APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-role
//
// Syntax:
//
//	DROP APPLICATION ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationRole() bool {
	return true
}

// ParseDropAuthenticationPolicy validates the Snowflake `DROP AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-authentication-policy
//
// Syntax:
//
//	DROP AUTHENTICATION POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropAuthenticationPolicy() bool {
	return true
}

// ParseDropBackupPolicy validates the Snowflake `DROP BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-policy
//
// Syntax:
//
//	DROP BACKUP POLICY <name>
func (v *Validator) ParseDropBackupPolicy() bool {
	return true
}

// ParseDropBackupSet validates the Snowflake `DROP BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-set
//
// Syntax:
//
//	DROP BACKUP SET <name>
func (v *Validator) ParseDropBackupSet() bool {
	return true
}

// ParseDropCatalogIntegration validates the Snowflake `DROP CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-catalog-integration
//
// Syntax:
//
//	DROP CATALOG INTEGRATION [ IF EXISTS ] <name>
func (v *Validator) ParseDropCatalogIntegration() bool {
	return true
}

// ParseDropComputePool validates the Snowflake `DROP COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-compute-pool
//
// Syntax:
//
//	DROP COMPUTE POOL [ IF EXISTS ] <name>
func (v *Validator) ParseDropComputePool() bool {
	return true
}

// ParseDropConnection validates the Snowflake `DROP CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-connection
//
// Syntax:
//
//	DROP CONNECTION [ IF EXISTS ] <name>
func (v *Validator) ParseDropConnection() bool {
	return true
}

// ParseDropContact validates the Snowflake `DROP CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-contact
//
// Syntax:
//
//	DROP CONTACT <name>
func (v *Validator) ParseDropContact() bool {
	return true
}

// ParseDropCortexSearchService validates the Snowflake `DROP CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-cortex-search
//
// Syntax:
//
//	DROP CORTEX SEARCH SERVICE <name>;
func (v *Validator) ParseDropCortexSearchService() bool {
	return true
}

// ParseDropDatabase validates the Snowflake `DROP DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database
//
// Syntax:
//
//	DROP DATABASE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropDatabase() bool {
	return true
}

// ParseDropDatabaseRole validates the Snowflake `DROP DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database-role
//
// Syntax:
//
//	DROP DATABASE ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropDatabaseRole() bool {
	return true
}

// ParseDropDbtProject validates the Snowflake `DROP DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dbt-project
//
// Syntax:
//
//	DROP DBT PROJECT [ IF EXISTS ] <name>
func (v *Validator) ParseDropDbtProject() bool {
	return true
}

// ParseDropDcmProject validates the Snowflake `DROP DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dcm-project
//
// Syntax:
//
//	DROP DCM PROJECT [ IF EXISTS ] <name>
func (v *Validator) ParseDropDcmProject() bool {
	return true
}

// ParseDropDynamicTable validates the Snowflake `DROP DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dynamic-table
//
// Syntax:
//
//	DROP DYNAMIC TABLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropDynamicTable() bool {
	return true
}

// ParseDropExperiment validates the Snowflake `DROP EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-experiment
//
// Syntax:
//
//	DROP EXPERIMENT <name>;
func (v *Validator) ParseDropExperiment() bool {
	return true
}

// ParseDropExternalAgent validates the Snowflake `DROP EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-agent
//
// Syntax:
//
//	DROP EXTERNAL AGENT [ IF EXISTS ] <name>
func (v *Validator) ParseDropExternalAgent() bool {
	return true
}

// ParseDropExternalTable validates the Snowflake `DROP EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-table
//
// Syntax:
//
//	DROP EXTERNAL TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropExternalTable() bool {
	return true
}

// ParseDropExternalVolume validates the Snowflake `DROP EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-volume
//
// Syntax:
//
//	DROP EXTERNAL VOLUME [ IF EXISTS ] <name>
func (v *Validator) ParseDropExternalVolume() bool {
	return true
}

// ParseDropFailoverGroup validates the Snowflake `DROP FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-failover-group
//
// Syntax:
//
//	DROP FAILOVER GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropFailoverGroup() bool {
	return true
}

// ParseDropFeaturePolicy validates the Snowflake `DROP FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-feature-policy
//
// Syntax:
//
//	DROP FEATURE POLICY <name>
func (v *Validator) ParseDropFeaturePolicy() bool {
	return true
}

// ParseDropFileFormat validates the Snowflake `DROP FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-file-format
//
// Syntax:
//
//	DROP FILE FORMAT [ IF EXISTS ] <name>
func (v *Validator) ParseDropFileFormat() bool {
	return true
}

// ParseDropFunction validates the Snowflake `DROP FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function
//
// Syntax:
//
//	DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropFunction() bool {
	return true
}

// ParseDropFunctionDmf validates the Snowflake `DROP FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function-dmf
//
// Syntax:
//
//	DROP FUNCTION [ IF EXISTS ] <name>(
//	TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ]
//	)
func (v *Validator) ParseDropFunctionDmf() bool {
	return true
}

// ParseDropFunctionSnowparkContainerServices validates the Snowflake `DROP FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function-spcs
//
// Syntax:
//
//	DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropFunctionSnowparkContainerServices() bool {
	return true
}

// ParseDropGateway validates the Snowflake `DROP GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-gateway
//
// Syntax:
//
//	DROP GATEWAY [ IF EXISTS ] <name>
func (v *Validator) ParseDropGateway() bool {
	return true
}

// ParseDropGitRepository validates the Snowflake `DROP GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-git-repository
//
// Syntax:
//
//	DROP GIT REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropGitRepository() bool {
	return true
}

// ParseDropIcebergTable validates the Snowflake `DROP ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-iceberg-table
//
// Syntax:
//
//	DROP [ ICEBERG ] TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropIcebergTable() bool {
	return true
}

// ParseDropImageRepository validates the Snowflake `DROP IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-image-repository
//
// Syntax:
//
//	DROP IMAGE REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropImageRepository() bool {
	return true
}

// ParseDropIndex validates the Snowflake `DROP INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-index
//
// Syntax:
//
//	DROP INDEX [ IF EXISTS ] <table_name>.<index_name>
func (v *Validator) ParseDropIndex() bool {
	return true
}

// ParseDropIntegration validates the Snowflake `DROP INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-integration
//
// Syntax:
//
//	DROP [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ] INTEGRATION [ IF EXISTS ] <name>
func (v *Validator) ParseDropIntegration() bool {
	return true
}

// ParseDropJoinPolicy validates the Snowflake `DROP JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-join-policy
//
// Syntax:
//
//	DROP JOIN POLICY <name>
func (v *Validator) ParseDropJoinPolicy() bool {
	return true
}

// ParseDropListing validates the Snowflake `DROP LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-listing
//
// Syntax:
//
//	DROP LISTING <name>
func (v *Validator) ParseDropListing() bool {
	return true
}

// ParseDropMaintenancePolicy validates the Snowflake `DROP MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-maintenance-policy
//
// Syntax:
//
//	DROP MAINTENANCE POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropMaintenancePolicy() bool {
	return true
}

// ParseDropManagedAccount validates the Snowflake `DROP MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-managed-account
//
// Syntax:
//
//	DROP MANAGED ACCOUNT <name>
func (v *Validator) ParseDropManagedAccount() bool {
	return true
}

// ParseDropMaskingPolicy validates the Snowflake `DROP MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-masking-policy
//
// Syntax:
//
//	DROP MASKING POLICY <name>
func (v *Validator) ParseDropMaskingPolicy() bool {
	return true
}

// ParseDropMaterializedView validates the Snowflake `DROP MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-materialized-view
//
// Syntax:
//
//	DROP MATERIALIZED VIEW [ IF EXISTS ] <view_name>
func (v *Validator) ParseDropMaterializedView() bool {
	return true
}

// ParseDropMcpServer validates the Snowflake `DROP MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-mcp-server
//
// Syntax:
//
//	DROP MCP SERVER [ IF EXISTS ] <name>
func (v *Validator) ParseDropMcpServer() bool {
	return true
}

// ParseDropModel validates the Snowflake `DROP MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model
//
// Syntax:
//
//	DROP MODEL <name>
func (v *Validator) ParseDropModel() bool {
	return true
}

// ParseDropModelMonitor validates the Snowflake `DROP MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model-monitor
//
// Syntax:
//
//	DROP MODEL MONITOR [ IF EXISTS ] <monitor_name>;
func (v *Validator) ParseDropModelMonitor() bool {
	return true
}

// ParseDropNetworkPolicy validates the Snowflake `DROP NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-policy
//
// Syntax:
//
//	DROP NETWORK POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropNetworkPolicy() bool {
	return true
}

// ParseDropNetworkRule validates the Snowflake `DROP NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-rule
//
// Syntax:
//
//	DROP NETWORK RULE [ IF EXISTS ] <name>
func (v *Validator) ParseDropNetworkRule() bool {
	return true
}

// ParseDropNotebook validates the Snowflake `DROP NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-notebook
//
// Syntax:
//
//	DROP NOTEBOOK <name>
func (v *Validator) ParseDropNotebook() bool {
	return true
}

// ParseDropOnlineFeatureTable validates the Snowflake `DROP ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-online-feature-table
//
// Syntax:
//
//	DROP ONLINE FEATURE TABLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropOnlineFeatureTable() bool {
	return true
}

// ParseDropOrganizationProfile validates the Snowflake `DROP ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-profile
//
// Syntax:
//
//	DROP ORGANIZATION PROFILE <name>
func (v *Validator) ParseDropOrganizationProfile() bool {
	return true
}

// ParseDropOrganizationUser validates the Snowflake `DROP ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user
//
// Syntax:
//
//	DROP ORGANIZATION USER [ IF EXISTS ] <name>
func (v *Validator) ParseDropOrganizationUser() bool {
	return true
}

// ParseDropOrganizationUserGroup validates the Snowflake `DROP ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user-group
//
// Syntax:
//
//	DROP ORGANIZATION USER GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropOrganizationUserGroup() bool {
	return true
}

// ParseDropPackagesPolicy validates the Snowflake `DROP PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-packages-policy
//
// Syntax:
//
//	DROP PACKAGES POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropPackagesPolicy() bool {
	return true
}

// ParseDropPasswordPolicy validates the Snowflake `DROP PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-password-policy
//
// Syntax:
//
//	DROP PASSWORD POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropPasswordPolicy() bool {
	return true
}

// ParseDropPipe validates the Snowflake `DROP PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-pipe
//
// Syntax:
//
//	DROP PIPE [ IF EXISTS ] <name>
func (v *Validator) ParseDropPipe() bool {
	return true
}

// ParseDropPostgresInstance validates the Snowflake `DROP POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-postgres-instance
//
// Syntax:
//
//	DROP POSTGRES INSTANCE [ IF EXISTS ] <name>
func (v *Validator) ParseDropPostgresInstance() bool {
	return true
}

// ParseDropPrivacyPolicy validates the Snowflake `DROP PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-privacy-policy
//
// Syntax:
//
//	DROP PRIVACY POLICY <name>
func (v *Validator) ParseDropPrivacyPolicy() bool {
	return true
}

// ParseDropProcedure validates the Snowflake `DROP PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-procedure
//
// Syntax:
//
//	DROP PROCEDURE [ IF EXISTS ] <procedure_name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropProcedure() bool {
	return true
}

// ParseDropProjectionPolicy validates the Snowflake `DROP PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-projection-policy
//
// Syntax:
//
//	DROP PROJECTION POLICY <name>
func (v *Validator) ParseDropProjectionPolicy() bool {
	return true
}

// ParseDropReplicationGroup validates the Snowflake `DROP REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-replication-group
//
// Syntax:
//
//	DROP REPLICATION GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropReplicationGroup() bool {
	return true
}

// ParseDropResourceMonitor validates the Snowflake `DROP RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-resource-monitor
//
// Syntax:
//
//	DROP RESOURCE MONITOR [ IF EXISTS ] <name>
func (v *Validator) ParseDropResourceMonitor() bool {
	return true
}

// ParseDropRole validates the Snowflake `DROP ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-role
//
// Syntax:
//
//	DROP ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropRole() bool {
	return true
}

// ParseDropRowAccessPolicy validates the Snowflake `DROP ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-row-access-policy
//
// Syntax:
//
//	DROP ROW ACCESS POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropRowAccessPolicy() bool {
	return true
}

// ParseDropSchema validates the Snowflake `DROP SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-schema
//
// Syntax:
//
//	DROP SCHEMA [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropSchema() bool {
	return true
}

// ParseDropSecret validates the Snowflake `DROP SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-secret
//
// Syntax:
//
//	DROP SECRET [ IF EXISTS ] <name>
func (v *Validator) ParseDropSecret() bool {
	return true
}

// ParseDropSemanticView validates the Snowflake `DROP SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-semantic-view
//
// Syntax:
//
//	DROP SEMANTIC VIEW [ IF EXISTS ] <name>
func (v *Validator) ParseDropSemanticView() bool {
	return true
}

// ParseDropSequence validates the Snowflake `DROP SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-sequence
//
// Syntax:
//
//	DROP SEQUENCE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropSequence() bool {
	return true
}

// ParseDropService validates the Snowflake `DROP SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-service
//
// Syntax:
//
//	DROP SERVICE [ IF EXISTS ] <name> [ FORCE ]
func (v *Validator) ParseDropService() bool {
	return true
}

// ParseDropSessionPolicy validates the Snowflake `DROP SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-session-policy
//
// Syntax:
//
//	DROP SESSION POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropSessionPolicy() bool {
	return true
}

// ParseDropShare validates the Snowflake `DROP SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-share
//
// Syntax:
//
//	DROP SHARE <name>
func (v *Validator) ParseDropShare() bool {
	return true
}

// ParseDropSnapshot validates the Snowflake `DROP SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot
//
// Syntax:
//
//	DROP SNAPSHOT [ IF EXISTS ] <name>;
func (v *Validator) ParseDropSnapshot() bool {
	return true
}

// ParseDropSnapshotPolicy validates the Snowflake `DROP SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-policy
//
// Syntax:
//
//	DROP SNAPSHOT POLICY <name>
func (v *Validator) ParseDropSnapshotPolicy() bool {
	return true
}

// ParseDropSnapshotSet validates the Snowflake `DROP SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-set
//
// Syntax:
//
//	DROP SNAPSHOT SET <name>
func (v *Validator) ParseDropSnapshotSet() bool {
	return true
}

// ParseDropStage validates the Snowflake `DROP STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stage
//
// Syntax:
//
//	DROP STAGE [ IF EXISTS ] <name>
func (v *Validator) ParseDropStage() bool {
	return true
}

// ParseDropStorageLifecyclePolicy validates the Snowflake `DROP STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-storage-lifecycle-policy
//
// Syntax:
//
//	DROP STORAGE LIFECYCLE POLICY [ IF EXISTS ] <policy_name>
func (v *Validator) ParseDropStorageLifecyclePolicy() bool {
	return true
}

// ParseDropStream validates the Snowflake `DROP STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stream
//
// Syntax:
//
//	DROP STREAM [ IF EXISTS ] <name>
func (v *Validator) ParseDropStream() bool {
	return true
}

// ParseDropStreamlit validates the Snowflake `DROP STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-streamlit
//
// Syntax:
//
//	DROP STREAMLIT [IF EXISTS] <name>
func (v *Validator) ParseDropStreamlit() bool {
	return true
}

// ParseDropTable validates the Snowflake `DROP TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-table
//
// Syntax:
//
//	DROP TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropTable() bool {
	return true
}

// ParseDropTag validates the Snowflake `DROP TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-tag
//
// Syntax:
//
//	DROP TAG [ IF EXISTS ] <name>
func (v *Validator) ParseDropTag() bool {
	return true
}

// ParseDropTask validates the Snowflake `DROP TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-task
//
// Syntax:
//
//	DROP TASK [ IF EXISTS ] <name>
func (v *Validator) ParseDropTask() bool {
	return true
}

// ParseDropType validates the Snowflake `DROP TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-type
//
// Syntax:
//
//	DROP TYPE <name>
func (v *Validator) ParseDropType() bool {
	return true
}

// ParseDropUser validates the Snowflake `DROP USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-user
//
// Syntax:
//
//	DROP USER [ IF EXISTS ] <name>
func (v *Validator) ParseDropUser() bool {
	return true
}

// ParseDropView validates the Snowflake `DROP VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-view
//
// Syntax:
//
//	DROP VIEW [ IF EXISTS ] <name>
func (v *Validator) ParseDropView() bool {
	return true
}

// ParseDropWarehouse validates the Snowflake `DROP WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-warehouse
//
// Syntax:
//
//	DROP WAREHOUSE [ IF EXISTS ] <name>
func (v *Validator) ParseDropWarehouse() bool {
	return true
}

// ParseDropApplicationService validates the Snowflake `DROP APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-service
//
// Syntax:
//
//	DROP APPLICATION SERVICE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationService() bool {
	return true
}

// ParseDropArtifactRepository validates the Snowflake `DROP ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-artifact-repository
//
// Syntax:
//
//	DROP ARTIFACT REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropArtifactRepository() bool {
	return true
}

// ParseDropEventRoutingTable validates the Snowflake `DROP EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-event-routing-table
//
// Syntax:
//
//	DROP EVENT ROUTING TABLE <table_name>
func (v *Validator) ParseDropEventRoutingTable() bool {
	return true
}
