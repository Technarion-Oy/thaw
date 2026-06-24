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
	// "(unavailable)" generic syntax: require a leading DROP, then accept the
	// remaining object-words/name/options permissively.
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		v.parseIdentPath,
		func() bool { return v.consumeRest() },
	)
}

// ParseDropAccount validates the Snowflake `DROP ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-account
//
// Syntax:
//
//	DROP ACCOUNT [ IF EXISTS ] <name> GRACE_PERIOD_IN_DAYS = <integer>
func (v *Validator) ParseDropAccount() bool {
	// DROP ACCOUNT [ IF EXISTS ] <name> GRACE_PERIOD_IN_DAYS = <integer>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ACCOUNT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		v.option("GRACE_PERIOD_IN_DAYS", v.parseNumber),
	)
}

// ParseDropAgent validates the Snowflake `DROP AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-agent
//
// Syntax:
//
//	DROP AGENT [ IF EXISTS ] <name>
func (v *Validator) ParseDropAgent() bool {
	// DROP AGENT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("AGENT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropAggregationPolicy validates the Snowflake `DROP AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-aggregation-policy
//
// Syntax:
//
//	DROP AGGREGATION POLICY <name>
func (v *Validator) ParseDropAggregationPolicy() bool {
	// DROP AGGREGATION POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("AGGREGATION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropAlert validates the Snowflake `DROP ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-alert
//
// Syntax:
//
//	DROP ALERT [ IF EXISTS ] <name>
func (v *Validator) ParseDropAlert() bool {
	// DROP ALERT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ALERT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropApplication validates the Snowflake `DROP APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application
//
// Syntax:
//
//	DROP APPLICATION [ IF EXISTS ] <name> [ CASCADE ]
func (v *Validator) ParseDropApplication() bool {
	// DROP APPLICATION [ IF EXISTS ] <name> [ CASCADE ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE")) },
	)
}

// ParseDropApplicationPackage validates the Snowflake `DROP APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-package
//
// Syntax:
//
//	DROP APPLICATION PACKAGE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationPackage() bool {
	// DROP APPLICATION PACKAGE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropApplicationRole validates the Snowflake `DROP APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-role
//
// Syntax:
//
//	DROP APPLICATION ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationRole() bool {
	// DROP APPLICATION ROLE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropAuthenticationPolicy validates the Snowflake `DROP AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-authentication-policy
//
// Syntax:
//
//	DROP AUTHENTICATION POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropAuthenticationPolicy() bool {
	// DROP AUTHENTICATION POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("AUTHENTICATION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropBackupPolicy validates the Snowflake `DROP BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-policy
//
// Syntax:
//
//	DROP BACKUP POLICY <name>
func (v *Validator) ParseDropBackupPolicy() bool {
	// DROP BACKUP POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("BACKUP", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropBackupSet validates the Snowflake `DROP BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-backup-set
//
// Syntax:
//
//	DROP BACKUP SET <name>
func (v *Validator) ParseDropBackupSet() bool {
	// DROP BACKUP SET <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("BACKUP", "SET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropCatalogIntegration validates the Snowflake `DROP CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-catalog-integration
//
// Syntax:
//
//	DROP CATALOG INTEGRATION [ IF EXISTS ] <name>
func (v *Validator) ParseDropCatalogIntegration() bool {
	// DROP CATALOG INTEGRATION [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("CATALOG", "INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropComputePool validates the Snowflake `DROP COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-compute-pool
//
// Syntax:
//
//	DROP COMPUTE POOL [ IF EXISTS ] <name>
func (v *Validator) ParseDropComputePool() bool {
	// DROP COMPUTE POOL [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("COMPUTE", "POOL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropConnection validates the Snowflake `DROP CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-connection
//
// Syntax:
//
//	DROP CONNECTION [ IF EXISTS ] <name>
func (v *Validator) ParseDropConnection() bool {
	// DROP CONNECTION [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("CONNECTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropContact validates the Snowflake `DROP CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-contact
//
// Syntax:
//
//	DROP CONTACT <name>
func (v *Validator) ParseDropContact() bool {
	// DROP CONTACT <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("CONTACT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropCortexSearchService validates the Snowflake `DROP CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-cortex-search
//
// Syntax:
//
//	DROP CORTEX SEARCH SERVICE <name>;
func (v *Validator) ParseDropCortexSearchService() bool {
	// DROP CORTEX SEARCH SERVICE <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("CORTEX", "SEARCH", "SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropDatabase validates the Snowflake `DROP DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database
//
// Syntax:
//
//	DROP DATABASE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropDatabase() bool {
	// DROP DATABASE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchKeyword("DATABASE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropDatabaseRole validates the Snowflake `DROP DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-database-role
//
// Syntax:
//
//	DROP DATABASE ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropDatabaseRole() bool {
	// DROP DATABASE ROLE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropDbtProject validates the Snowflake `DROP DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dbt-project
//
// Syntax:
//
//	DROP DBT PROJECT [ IF EXISTS ] <name>
func (v *Validator) ParseDropDbtProject() bool {
	// DROP DBT PROJECT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("DBT", "PROJECT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropDcmProject validates the Snowflake `DROP DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dcm-project
//
// Syntax:
//
//	DROP DCM PROJECT [ IF EXISTS ] <name>
func (v *Validator) ParseDropDcmProject() bool {
	// DROP DCM PROJECT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("DCM", "PROJECT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropDynamicTable validates the Snowflake `DROP DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-dynamic-table
//
// Syntax:
//
//	DROP DYNAMIC TABLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropDynamicTable() bool {
	// DROP DYNAMIC TABLE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("DYNAMIC", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropExperiment validates the Snowflake `DROP EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-experiment
//
// Syntax:
//
//	DROP EXPERIMENT <name>;
func (v *Validator) ParseDropExperiment() bool {
	// DROP EXPERIMENT <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("EXPERIMENT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropExternalAgent validates the Snowflake `DROP EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-agent
//
// Syntax:
//
//	DROP EXTERNAL AGENT [ IF EXISTS ] <name>
func (v *Validator) ParseDropExternalAgent() bool {
	// DROP EXTERNAL AGENT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("EXTERNAL", "AGENT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropExternalTable validates the Snowflake `DROP EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-table
//
// Syntax:
//
//	DROP EXTERNAL TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropExternalTable() bool {
	// DROP EXTERNAL TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("EXTERNAL", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropExternalVolume validates the Snowflake `DROP EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-external-volume
//
// Syntax:
//
//	DROP EXTERNAL VOLUME [ IF EXISTS ] <name>
func (v *Validator) ParseDropExternalVolume() bool {
	// DROP EXTERNAL VOLUME [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("EXTERNAL", "VOLUME") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropFailoverGroup validates the Snowflake `DROP FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-failover-group
//
// Syntax:
//
//	DROP FAILOVER GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropFailoverGroup() bool {
	// DROP FAILOVER GROUP [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("FAILOVER", "GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropFeaturePolicy validates the Snowflake `DROP FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-feature-policy
//
// Syntax:
//
//	DROP FEATURE POLICY <name>
func (v *Validator) ParseDropFeaturePolicy() bool {
	// DROP FEATURE POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("FEATURE", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropFileFormat validates the Snowflake `DROP FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-file-format
//
// Syntax:
//
//	DROP FILE FORMAT [ IF EXISTS ] <name>
func (v *Validator) ParseDropFileFormat() bool {
	// DROP FILE FORMAT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("FILE", "FORMAT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropFunction validates the Snowflake `DROP FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function
//
// Syntax:
//
//	DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropFunction() bool {
	// DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// ( [ <arg_data_type> , ... ] ) — model the signature permissively.
		func() bool { return v.consumeBalancedParens() },
	)
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
	// DROP FUNCTION [ IF EXISTS ] <name>(
	//   TABLE( <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ]
	// )
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// The whole TABLE(...)-list signature is a balanced-parens group.
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDropFunctionSnowparkContainerServices validates the Snowflake `DROP FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-function-spcs
//
// Syntax:
//
//	DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropFunctionSnowparkContainerServices() bool {
	// DROP FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDropGateway validates the Snowflake `DROP GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-gateway
//
// Syntax:
//
//	DROP GATEWAY [ IF EXISTS ] <name>
func (v *Validator) ParseDropGateway() bool {
	// DROP GATEWAY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("GATEWAY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropGitRepository validates the Snowflake `DROP GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-git-repository
//
// Syntax:
//
//	DROP GIT REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropGitRepository() bool {
	// DROP GIT REPOSITORY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("GIT", "REPOSITORY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropIcebergTable validates the Snowflake `DROP ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-iceberg-table
//
// Syntax:
//
//	DROP [ ICEBERG ] TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropIcebergTable() bool {
	// DROP [ ICEBERG ] TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("ICEBERG") }) },
		func() bool { return v.MatchKeyword("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropImageRepository validates the Snowflake `DROP IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-image-repository
//
// Syntax:
//
//	DROP IMAGE REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropImageRepository() bool {
	// DROP IMAGE REPOSITORY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("IMAGE", "REPOSITORY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropIndex validates the Snowflake `DROP INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-index
//
// Syntax:
//
//	DROP INDEX [ IF EXISTS ] <table_name>.<index_name>
func (v *Validator) ParseDropIndex() bool {
	// DROP INDEX [ IF EXISTS ] <table_name>.<index_name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("INDEX") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropIntegration validates the Snowflake `DROP INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-integration
//
// Syntax:
//
//	DROP [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ] INTEGRATION [ IF EXISTS ] <name>
func (v *Validator) ParseDropIntegration() bool {
	// DROP [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ]
	//   INTEGRATION [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.phrase("EXTERNAL", "ACCESS") },
					v.wordsValue("API", "CATALOG", "NOTIFICATION", "SECURITY", "STORAGE"),
				)
			})
		},
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropJoinPolicy validates the Snowflake `DROP JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-join-policy
//
// Syntax:
//
//	DROP JOIN POLICY <name>
func (v *Validator) ParseDropJoinPolicy() bool {
	// DROP JOIN POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("JOIN", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropListing validates the Snowflake `DROP LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-listing
//
// Syntax:
//
//	DROP LISTING <name>
func (v *Validator) ParseDropListing() bool {
	// DROP LISTING <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("LISTING") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropMaintenancePolicy validates the Snowflake `DROP MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-maintenance-policy
//
// Syntax:
//
//	DROP MAINTENANCE POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropMaintenancePolicy() bool {
	// DROP MAINTENANCE POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("MAINTENANCE", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropManagedAccount validates the Snowflake `DROP MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-managed-account
//
// Syntax:
//
//	DROP MANAGED ACCOUNT <name>
func (v *Validator) ParseDropManagedAccount() bool {
	// DROP MANAGED ACCOUNT <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("MANAGED", "ACCOUNT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropMaskingPolicy validates the Snowflake `DROP MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-masking-policy
//
// Syntax:
//
//	DROP MASKING POLICY <name>
func (v *Validator) ParseDropMaskingPolicy() bool {
	// DROP MASKING POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.phrase("MASKING", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropMaterializedView validates the Snowflake `DROP MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-materialized-view
//
// Syntax:
//
//	DROP MATERIALIZED VIEW [ IF EXISTS ] <view_name>
func (v *Validator) ParseDropMaterializedView() bool {
	// DROP MATERIALIZED VIEW [ IF EXISTS ] <view_name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("MATERIALIZED") },
		func() bool { return v.MatchWord("VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropMcpServer validates the Snowflake `DROP MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-mcp-server
//
// Syntax:
//
//	DROP MCP SERVER [ IF EXISTS ] <name>
func (v *Validator) ParseDropMcpServer() bool {
	// DROP MCP SERVER [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("MCP") },
		func() bool { return v.MatchWord("SERVER") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropModel validates the Snowflake `DROP MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model
//
// Syntax:
//
//	DROP MODEL <name>
func (v *Validator) ParseDropModel() bool {
	// DROP MODEL <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropModelMonitor validates the Snowflake `DROP MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-model-monitor
//
// Syntax:
//
//	DROP MODEL MONITOR [ IF EXISTS ] <monitor_name>;
func (v *Validator) ParseDropModelMonitor() bool {
	// DROP MODEL MONITOR [ IF EXISTS ] <monitor_name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.MatchWord("MONITOR") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropNetworkPolicy validates the Snowflake `DROP NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-policy
//
// Syntax:
//
//	DROP NETWORK POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropNetworkPolicy() bool {
	// DROP NETWORK POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("NETWORK") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropNetworkRule validates the Snowflake `DROP NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-network-rule
//
// Syntax:
//
//	DROP NETWORK RULE [ IF EXISTS ] <name>
func (v *Validator) ParseDropNetworkRule() bool {
	// DROP NETWORK RULE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("NETWORK") },
		func() bool { return v.MatchWord("RULE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropNotebook validates the Snowflake `DROP NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-notebook
//
// Syntax:
//
//	DROP NOTEBOOK <name>
func (v *Validator) ParseDropNotebook() bool {
	// DROP NOTEBOOK <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("NOTEBOOK") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropOnlineFeatureTable validates the Snowflake `DROP ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-online-feature-table
//
// Syntax:
//
//	DROP ONLINE FEATURE TABLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropOnlineFeatureTable() bool {
	// DROP ONLINE FEATURE TABLE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ONLINE") },
		func() bool { return v.MatchWord("FEATURE") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropOrganizationProfile validates the Snowflake `DROP ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-profile
//
// Syntax:
//
//	DROP ORGANIZATION PROFILE <name>
func (v *Validator) ParseDropOrganizationProfile() bool {
	// DROP ORGANIZATION PROFILE <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("PROFILE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropOrganizationUser validates the Snowflake `DROP ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user
//
// Syntax:
//
//	DROP ORGANIZATION USER [ IF EXISTS ] <name>
func (v *Validator) ParseDropOrganizationUser() bool {
	// DROP ORGANIZATION USER [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropOrganizationUserGroup validates the Snowflake `DROP ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-organization-user-group
//
// Syntax:
//
//	DROP ORGANIZATION USER GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropOrganizationUserGroup() bool {
	// DROP ORGANIZATION USER GROUP [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.MatchWord("GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropPackagesPolicy validates the Snowflake `DROP PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-packages-policy
//
// Syntax:
//
//	DROP PACKAGES POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropPackagesPolicy() bool {
	// DROP PACKAGES POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PACKAGES") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropPasswordPolicy validates the Snowflake `DROP PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-password-policy
//
// Syntax:
//
//	DROP PASSWORD POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropPasswordPolicy() bool {
	// DROP PASSWORD POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PASSWORD") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropPipe validates the Snowflake `DROP PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-pipe
//
// Syntax:
//
//	DROP PIPE [ IF EXISTS ] <name>
func (v *Validator) ParseDropPipe() bool {
	// DROP PIPE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PIPE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropPostgresInstance validates the Snowflake `DROP POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-postgres-instance
//
// Syntax:
//
//	DROP POSTGRES INSTANCE [ IF EXISTS ] <name>
func (v *Validator) ParseDropPostgresInstance() bool {
	// DROP POSTGRES INSTANCE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("POSTGRES") },
		func() bool { return v.MatchWord("INSTANCE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropPrivacyPolicy validates the Snowflake `DROP PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-privacy-policy
//
// Syntax:
//
//	DROP PRIVACY POLICY <name>
func (v *Validator) ParseDropPrivacyPolicy() bool {
	// DROP PRIVACY POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PRIVACY") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropProcedure validates the Snowflake `DROP PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-procedure
//
// Syntax:
//
//	DROP PROCEDURE [ IF EXISTS ] <procedure_name> ( [ <arg_data_type> , ... ] )
func (v *Validator) ParseDropProcedure() bool {
	// DROP PROCEDURE [ IF EXISTS ] <procedure_name> ( [ <arg_data_type> , ... ] )
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PROCEDURE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDropProjectionPolicy validates the Snowflake `DROP PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-projection-policy
//
// Syntax:
//
//	DROP PROJECTION POLICY <name>
func (v *Validator) ParseDropProjectionPolicy() bool {
	// DROP PROJECTION POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("PROJECTION") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropReplicationGroup validates the Snowflake `DROP REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-replication-group
//
// Syntax:
//
//	DROP REPLICATION GROUP [ IF EXISTS ] <name>
func (v *Validator) ParseDropReplicationGroup() bool {
	// DROP REPLICATION GROUP [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropResourceMonitor validates the Snowflake `DROP RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-resource-monitor
//
// Syntax:
//
//	DROP RESOURCE MONITOR [ IF EXISTS ] <name>
func (v *Validator) ParseDropResourceMonitor() bool {
	// DROP RESOURCE MONITOR [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("RESOURCE") },
		func() bool { return v.MatchWord("MONITOR") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropRole validates the Snowflake `DROP ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-role
//
// Syntax:
//
//	DROP ROLE [ IF EXISTS ] <name>
func (v *Validator) ParseDropRole() bool {
	// DROP ROLE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropRowAccessPolicy validates the Snowflake `DROP ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-row-access-policy
//
// Syntax:
//
//	DROP ROW ACCESS POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropRowAccessPolicy() bool {
	// DROP ROW ACCESS POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ROW") },
		func() bool { return v.MatchWord("ACCESS") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSchema validates the Snowflake `DROP SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-schema
//
// Syntax:
//
//	DROP SCHEMA [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropSchema() bool {
	// DROP SCHEMA [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SCHEMA") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropSecret validates the Snowflake `DROP SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-secret
//
// Syntax:
//
//	DROP SECRET [ IF EXISTS ] <name>
func (v *Validator) ParseDropSecret() bool {
	// DROP SECRET [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SECRET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSemanticView validates the Snowflake `DROP SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-semantic-view
//
// Syntax:
//
//	DROP SEMANTIC VIEW [ IF EXISTS ] <name>
func (v *Validator) ParseDropSemanticView() bool {
	// DROP SEMANTIC VIEW [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SEMANTIC") },
		func() bool { return v.MatchWord("VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSequence validates the Snowflake `DROP SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-sequence
//
// Syntax:
//
//	DROP SEQUENCE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropSequence() bool {
	// DROP SEQUENCE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SEQUENCE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropService validates the Snowflake `DROP SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-service
//
// Syntax:
//
//	DROP SERVICE [ IF EXISTS ] <name> [ FORCE ]
func (v *Validator) ParseDropService() bool {
	// DROP SERVICE [ IF EXISTS ] <name> [ FORCE ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(func() bool { return v.MatchWord("FORCE") }) },
	)
}

// ParseDropSessionPolicy validates the Snowflake `DROP SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-session-policy
//
// Syntax:
//
//	DROP SESSION POLICY [ IF EXISTS ] <name>
func (v *Validator) ParseDropSessionPolicy() bool {
	// DROP SESSION POLICY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SESSION") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropShare validates the Snowflake `DROP SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-share
//
// Syntax:
//
//	DROP SHARE <name>
func (v *Validator) ParseDropShare() bool {
	// DROP SHARE <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SHARE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSnapshot validates the Snowflake `DROP SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot
//
// Syntax:
//
//	DROP SNAPSHOT [ IF EXISTS ] <name>;
func (v *Validator) ParseDropSnapshot() bool {
	// DROP SNAPSHOT [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSnapshotPolicy validates the Snowflake `DROP SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-policy
//
// Syntax:
//
//	DROP SNAPSHOT POLICY <name>
func (v *Validator) ParseDropSnapshotPolicy() bool {
	// DROP SNAPSHOT POLICY <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropSnapshotSet validates the Snowflake `DROP SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-snapshot-set
//
// Syntax:
//
//	DROP SNAPSHOT SET <name>
func (v *Validator) ParseDropSnapshotSet() bool {
	// DROP SNAPSHOT SET <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("SET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropStage validates the Snowflake `DROP STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stage
//
// Syntax:
//
//	DROP STAGE [ IF EXISTS ] <name>
func (v *Validator) ParseDropStage() bool {
	// DROP STAGE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("STAGE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropStorageLifecyclePolicy validates the Snowflake `DROP STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-storage-lifecycle-policy
//
// Syntax:
//
//	DROP STORAGE LIFECYCLE POLICY [ IF EXISTS ] <policy_name>
func (v *Validator) ParseDropStorageLifecyclePolicy() bool {
	// DROP STORAGE LIFECYCLE POLICY [ IF EXISTS ] <policy_name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("LIFECYCLE") },
		func() bool { return v.MatchWord("POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropStream validates the Snowflake `DROP STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-stream
//
// Syntax:
//
//	DROP STREAM [ IF EXISTS ] <name>
func (v *Validator) ParseDropStream() bool {
	// DROP STREAM [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("STREAM") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropStreamlit validates the Snowflake `DROP STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-streamlit
//
// Syntax:
//
//	DROP STREAMLIT [IF EXISTS] <name>
func (v *Validator) ParseDropStreamlit() bool {
	// DROP STREAMLIT [IF EXISTS] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("STREAMLIT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropTable validates the Snowflake `DROP TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-table
//
// Syntax:
//
//	DROP TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
func (v *Validator) ParseDropTable() bool {
	// DROP TABLE [ IF EXISTS ] <name> [ CASCADE | RESTRICT ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Optional(v.wordsValue("CASCADE", "RESTRICT")) },
	)
}

// ParseDropTag validates the Snowflake `DROP TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-tag
//
// Syntax:
//
//	DROP TAG [ IF EXISTS ] <name>
func (v *Validator) ParseDropTag() bool {
	// DROP TAG [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("TAG") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropTask validates the Snowflake `DROP TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-task
//
// Syntax:
//
//	DROP TASK [ IF EXISTS ] <name>
func (v *Validator) ParseDropTask() bool {
	// DROP TASK [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("TASK") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropType validates the Snowflake `DROP TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-type
//
// Syntax:
//
//	DROP TYPE <name>
func (v *Validator) ParseDropType() bool {
	// DROP TYPE <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("TYPE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropUser validates the Snowflake `DROP USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-user
//
// Syntax:
//
//	DROP USER [ IF EXISTS ] <name>
func (v *Validator) ParseDropUser() bool {
	// DROP USER [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropView validates the Snowflake `DROP VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-view
//
// Syntax:
//
//	DROP VIEW [ IF EXISTS ] <name>
func (v *Validator) ParseDropView() bool {
	// DROP VIEW [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropWarehouse validates the Snowflake `DROP WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-warehouse
//
// Syntax:
//
//	DROP WAREHOUSE [ IF EXISTS ] <name>
func (v *Validator) ParseDropWarehouse() bool {
	// DROP WAREHOUSE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("WAREHOUSE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropApplicationService validates the Snowflake `DROP APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-application-service
//
// Syntax:
//
//	DROP APPLICATION SERVICE [ IF EXISTS ] <name>
func (v *Validator) ParseDropApplicationService() bool {
	// DROP APPLICATION SERVICE [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.MatchWord("SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropArtifactRepository validates the Snowflake `DROP ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-artifact-repository
//
// Syntax:
//
//	DROP ARTIFACT REPOSITORY [ IF EXISTS ] <name>
func (v *Validator) ParseDropArtifactRepository() bool {
	// DROP ARTIFACT REPOSITORY [ IF EXISTS ] <name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("ARTIFACT") },
		func() bool { return v.MatchWord("REPOSITORY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}

// ParseDropEventRoutingTable validates the Snowflake `DROP EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/drop-event-routing-table
//
// Syntax:
//
//	DROP EVENT ROUTING TABLE <table_name>
func (v *Validator) ParseDropEventRoutingTable() bool {
	// DROP EVENT ROUTING TABLE <table_name>
	return v.Sequence(
		func() bool { return v.MatchKeyword("DROP") },
		func() bool { return v.MatchWord("EVENT") },
		func() bool { return v.MatchWord("ROUTING") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
	)
}
