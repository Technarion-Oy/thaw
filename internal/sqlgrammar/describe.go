package sqlgrammar

// DESCRIBE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDescribeObj validates the Snowflake `DESCRIBE <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc
//
// Syntax:
//
//	DESCRIBE <object>
func (v *Validator) ParseDescribeObj() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		v.parseIdentPath,
	)
}

// ParseDescribeAgent validates the Snowflake `DESCRIBE AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-agent
//
// Syntax:
//
//	{ DESC | DESCRIBE } AGENT <name>
func (v *Validator) ParseDescribeAgent() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("AGENT") },
		v.parseIdentPath,
	)
}

// ParseDescribeAggregationPolicy validates the Snowflake `DESCRIBE AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-aggregation-policy
//
// Syntax:
//
//	DESC[RIBE] AGGREGATION POLICY <name>
func (v *Validator) ParseDescribeAggregationPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("AGGREGATION", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeAlert validates the Snowflake `DESCRIBE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-alert
//
// Syntax:
//
//	DESC[RIBE] ALERT <name>
func (v *Validator) ParseDescribeAlert() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("ALERT") },
		v.parseIdentPath,
	)
}

// ParseDescribeApplication validates the Snowflake `DESCRIBE APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-application
//
// Syntax:
//
//	DESC[RIBE] APPLICATION <name>
func (v *Validator) ParseDescribeApplication() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
	)
}

// ParseDescribeApplicationPackage validates the Snowflake `DESCRIBE APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-application-package
//
// Syntax:
//
//	DESC[RIBE] APPLICATION PACKAGE <name>
func (v *Validator) ParseDescribeApplicationPackage() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		v.parseIdentPath,
	)
}

// ParseDescribeAuthenticationPolicy validates the Snowflake `DESCRIBE AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-authentication-policy
//
// Syntax:
//
//	{ DESC | DESCRIBE } AUTHENTICATION POLICY <name>
func (v *Validator) ParseDescribeAuthenticationPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("AUTHENTICATION", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeAvailableListing validates the Snowflake `DESCRIBE AVAILABLE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-available-listing
//
// Syntax:
//
//	{ DESC | DESCRIBE } AVAILABLE LISTING <listing_global_name>
func (v *Validator) ParseDescribeAvailableListing() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("AVAILABLE", "LISTING") },
		v.parseIdentPath,
	)
}

// ParseDescribeAvailableOrganizationProfile validates the Snowflake `DESCRIBE AVAILABLE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-available-organization-profile
//
// Syntax:
//
//	{ DESC | DESCRIBE } AVAILABLE ORGANIZATION PROFILE <name>
func (v *Validator) ParseDescribeAvailableOrganizationProfile() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("AVAILABLE", "ORGANIZATION", "PROFILE") },
		v.parseIdentPath,
	)
}

// ParseDescribeBackupPolicy validates the Snowflake `DESCRIBE BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-backup-policy
//
// Syntax:
//
//	DESC[RIBE] BACKUP POLICY <name>
func (v *Validator) ParseDescribeBackupPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("BACKUP", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeBackupSet validates the Snowflake `DESCRIBE BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-backup-set
//
// Syntax:
//
//	DESC[RIBE] BACKUP SET <name>
func (v *Validator) ParseDescribeBackupSet() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("BACKUP", "SET") },
		v.parseIdentPath,
	)
}

// ParseDescribeCatalogIntegration validates the Snowflake `DESCRIBE CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-catalog-integration
//
// Syntax:
//
//	DESC[RIBE] CATALOG INTEGRATION <name>
func (v *Validator) ParseDescribeCatalogIntegration() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("CATALOG", "INTEGRATION") },
		v.parseIdentPath,
	)
}

// ParseDescribeComputePool validates the Snowflake `DESCRIBE COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-compute-pool
//
// Syntax:
//
//	DESC[RIBE] COMPUTE POOL <name>
func (v *Validator) ParseDescribeComputePool() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("COMPUTE", "POOL") },
		v.parseIdentPath,
	)
}

// ParseDescribeConfiguration validates the Snowflake `DESCRIBE CONFIGURATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-configuration
//
// Syntax:
//
//	{ DESC | DESCRIBE } CONFIGURATION <configuration_name> [ IN APPLICATION <app> ]
func (v *Validator) ParseDescribeConfiguration() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("CONFIGURATION") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.phrase("IN", "APPLICATION") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseDescribeCortexSearchService validates the Snowflake `DESCRIBE CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-cortex-search
//
// Syntax:
//
//	{ DESC | DESCRIBE } CORTEX SEARCH SERVICE <name>;
func (v *Validator) ParseDescribeCortexSearchService() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("CORTEX", "SEARCH", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseDescribeDatabase validates the Snowflake `DESCRIBE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-database
//
// Syntax:
//
//	DESC[RIBE] DATABASE <database_name>
func (v *Validator) ParseDescribeDatabase() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("DATABASE") },
		v.parseIdentPath,
	)
}

// ParseDescribeDbtProject validates the Snowflake `DESCRIBE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dbt-project
//
// Syntax:
//
//	{ DESC | DESCRIBE } DBT PROJECT <name>
func (v *Validator) ParseDescribeDbtProject() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("DBT", "PROJECT") },
		v.parseIdentPath,
	)
}

// ParseDescribeDcmProject validates the Snowflake `DESCRIBE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dcm-project
//
// Syntax:
//
//	{ DESCRIBE | DESC } DCM PROJECT <name>
func (v *Validator) ParseDescribeDcmProject() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("DCM", "PROJECT") },
		v.parseIdentPath,
	)
}

// ParseDescribeDynamicTable validates the Snowflake `DESCRIBE DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-dynamic-table
//
// Syntax:
//
//	DESC[RIBE] DYNAMIC TABLE <name>
func (v *Validator) ParseDescribeDynamicTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("DYNAMIC", "TABLE") },
		v.parseIdentPath,
	)
}

// ParseDescribeEventTable validates the Snowflake `DESCRIBE EVENT TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-event-table
//
// Syntax:
//
//	DESC[RIBE] EVENT TABLE <name>
func (v *Validator) ParseDescribeEventTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("EVENT", "TABLE") },
		v.parseIdentPath,
	)
}

// ParseDescribeExternalAgent validates the Snowflake `DESCRIBE EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-agent
//
// Syntax:
//
//	{ DESC | DESCRIBE } EXTERNAL AGENT <name>
func (v *Validator) ParseDescribeExternalAgent() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("EXTERNAL", "AGENT") },
		v.parseIdentPath,
	)
}

// ParseDescribeExternalTable validates the Snowflake `DESCRIBE EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-table
//
// Syntax:
//
//	DESC[RIBE] [ EXTERNAL ] TABLE <name> [ TYPE = { COLUMNS | STAGE } ]
func (v *Validator) ParseDescribeExternalTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.Optional(func() bool { return v.MatchWord("EXTERNAL") }) },
		func() bool { return v.MatchWord("TABLE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.option("TYPE", v.wordsValue("COLUMNS", "STAGE")))
		},
	)
}

// ParseDescribeExternalVolume validates the Snowflake `DESCRIBE EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-external-volume
//
// Syntax:
//
//	DESC[RIBE] EXTERNAL VOLUME <name>
func (v *Validator) ParseDescribeExternalVolume() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("EXTERNAL", "VOLUME") },
		v.parseIdentPath,
	)
}

// ParseDescribeFeaturePolicy validates the Snowflake `DESCRIBE FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-feature-policy
//
// Syntax:
//
//	{ DESC | DESCRIBE } FEATURE POLICY <name>
func (v *Validator) ParseDescribeFeaturePolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("FEATURE", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeFileFormat validates the Snowflake `DESCRIBE FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-file-format
//
// Syntax:
//
//	DESC[RIBE] FILE FORMAT <name>
func (v *Validator) ParseDescribeFileFormat() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("FILE", "FORMAT") },
		v.parseIdentPath,
	)
}

// ParseDescribeFunction validates the Snowflake `DESCRIBE FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function
//
// Syntax:
//
//	DESC[RIBE] FUNCTION <name> ( [ <arg_data_type> ] [ , ... ] )
func (v *Validator) ParseDescribeFunction() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("FUNCTION") },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDescribeFunctionDmf validates the Snowflake `DESCRIBE FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function-dmf
//
// Syntax:
//
//	{ DESC | DESCRIBE } FUNCTION [ IF EXISTS ] <name>(
//	  TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ]
//	  )
func (v *Validator) ParseDescribeFunctionDmf() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDescribeFunctionSnowparkContainerServices validates the Snowflake `DESCRIBE FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-function-spcs
//
// Syntax:
//
//	{ DESC | DESCRIBE } FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> ] [ , ... ] )
func (v *Validator) ParseDescribeFunctionSnowparkContainerServices() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDescribeGateway validates the Snowflake `DESCRIBE GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-gateway
//
// Syntax:
//
//	DESC[RIBE] GATEWAY <name>
func (v *Validator) ParseDescribeGateway() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("GATEWAY") },
		v.parseIdentPath,
	)
}

// ParseDescribeGitRepository validates the Snowflake `DESCRIBE GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-git-repository
//
// Syntax:
//
//	{ DESC | DESCRIBE } GIT REPOSITORY <name>
func (v *Validator) ParseDescribeGitRepository() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("GIT", "REPOSITORY") },
		v.parseIdentPath,
	)
}

// ParseDescribeIcebergTable validates the Snowflake `DESCRIBE ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-iceberg-table
//
// Syntax:
//
//	DESC[RIBE] [ ICEBERG ] TABLE <name> [ TYPE = { COLUMNS | STAGE } ]
func (v *Validator) ParseDescribeIcebergTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.Optional(func() bool { return v.MatchWord("ICEBERG") }) },
		func() bool { return v.MatchWord("TABLE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.option("TYPE", v.wordsValue("COLUMNS", "STAGE")))
		},
	)
}

// ParseDescribeIntegration validates the Snowflake `DESCRIBE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-integration
//
// Syntax:
//
//	{ DESC | DESCRIBE } [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ] INTEGRATION <name>
func (v *Validator) ParseDescribeIntegration() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.phrase("EXTERNAL", "ACCESS") },
					v.wordsValue("API", "CATALOG", "NOTIFICATION", "SECURITY", "STORAGE"),
				)
			})
		},
		func() bool { return v.MatchWord("INTEGRATION") },
		v.parseIdentPath,
	)
}

// ParseDescribeJoinPolicy validates the Snowflake `DESCRIBE JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-join-policy
//
// Syntax:
//
//	{ DESCRIBE | DESC } JOIN POLICY <name>
func (v *Validator) ParseDescribeJoinPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("JOIN", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeListing validates the Snowflake `DESCRIBE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-listing
//
// Syntax:
//
//	{ DESC | DESCRIBE } LISTING <name>  [ REVISION = { DRAFT | PUBLISHED } ]
func (v *Validator) ParseDescribeListing() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("LISTING") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.option("REVISION", v.wordsValue("DRAFT", "PUBLISHED")))
		},
	)
}

// ParseDescribeMaintenancePolicy validates the Snowflake `DESCRIBE MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-maintenance-policy
//
// Syntax:
//
//	DESCRIBE MAINTENANCE POLICY <name>
func (v *Validator) ParseDescribeMaintenancePolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("MAINTENANCE", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeMaskingPolicy validates the Snowflake `DESCRIBE MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-masking-policy
//
// Syntax:
//
//	DESC[RIBE] MASKING POLICY <name>
func (v *Validator) ParseDescribeMaskingPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("MASKING", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeMaterializedView validates the Snowflake `DESCRIBE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-materialized-view
//
// Syntax:
//
//	DESC[RIBE] MATERIALIZED VIEW <name>
func (v *Validator) ParseDescribeMaterializedView() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("MATERIALIZED", "VIEW") },
		v.parseIdentPath,
	)
}

// ParseDescribeMcpServer validates the Snowflake `DESCRIBE MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-mcp-server
//
// Syntax:
//
//	{ DESC | DESCRIBE } MCP SERVER <name>
func (v *Validator) ParseDescribeMcpServer() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("MCP", "SERVER") },
		v.parseIdentPath,
	)
}

// ParseDescribeModelMonitor validates the Snowflake `DESCRIBE MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-model-monitor
//
// Syntax:
//
//	{ DESCRIBE | DESC } MODEL MONITOR <monitor_name>
func (v *Validator) ParseDescribeModelMonitor() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("MODEL", "MONITOR") },
		v.parseIdentPath,
	)
}

// ParseDescribeNetworkPolicy validates the Snowflake `DESCRIBE NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-network-policy
//
// Syntax:
//
//	DESC[RIBE] NETWORK POLICY <name>
func (v *Validator) ParseDescribeNetworkPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("NETWORK", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeNetworkRule validates the Snowflake `DESCRIBE NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-network-rule
//
// Syntax:
//
//	DESC[RIBE] NETWORK RULE <name>
func (v *Validator) ParseDescribeNetworkRule() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("NETWORK", "RULE") },
		v.parseIdentPath,
	)
}

// ParseDescribeNotebook validates the Snowflake `DESCRIBE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-notebook
//
// Syntax:
//
//	{ DESC | DESCRIBE } NOTEBOOK <name>
func (v *Validator) ParseDescribeNotebook() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("NOTEBOOK") },
		v.parseIdentPath,
	)
}

// ParseDescribeNotificationIntegration validates the Snowflake `DESCRIBE NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-notification-integration
//
// Syntax:
//
//	{ DESC | DESCRIBE } NOTIFICATION INTEGRATION <name>
func (v *Validator) ParseDescribeNotificationIntegration() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("NOTIFICATION", "INTEGRATION") },
		v.parseIdentPath,
	)
}

// ParseDescribeOpenflowDataPlaneIntegration validates the Snowflake `DESCRIBE OPENFLOW DATA PLANE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-oflow-data-plane-integration
//
// Syntax:
//
//	{ DESC | DESCRIBE } OPENFLOW DATA PLANE INTEGRATION <name>
func (v *Validator) ParseDescribeOpenflowDataPlaneIntegration() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("OPENFLOW", "DATA", "PLANE", "INTEGRATION") },
		v.parseIdentPath,
	)
}

// ParseDescribeOnlineFeatureTable validates the Snowflake `DESCRIBE ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-online-feature-table
//
// Syntax:
//
//	{ DESC | DESCRIBE } ONLINE FEATURE TABLE <name>
func (v *Validator) ParseDescribeOnlineFeatureTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("ONLINE", "FEATURE", "TABLE") },
		v.parseIdentPath,
	)
}

// ParseDescribeOrganizationProfile validates the Snowflake `DESCRIBE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-organization-profile
//
// Syntax:
//
//	{ DESC | DESCRIBE } ORGANIZATION PROFILE <name>
func (v *Validator) ParseDescribeOrganizationProfile() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("ORGANIZATION", "PROFILE") },
		v.parseIdentPath,
	)
}

// ParseDescribePackagesPolicy validates the Snowflake `DESCRIBE PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-packages-policy
//
// Syntax:
//
//	DESC[RIBE] PACKAGES POLICY <name>
func (v *Validator) ParseDescribePackagesPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("PACKAGES", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribePasswordPolicy validates the Snowflake `DESCRIBE PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-password-policy
//
// Syntax:
//
//	DESC[RIBE] PASSWORD POLICY <name>
func (v *Validator) ParseDescribePasswordPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("PASSWORD", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribePipe validates the Snowflake `DESCRIBE PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-pipe
//
// Syntax:
//
//	DESC[RIBE] PIPE <name>
func (v *Validator) ParseDescribePipe() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("PIPE") },
		v.parseIdentPath,
	)
}

// ParseDescribePostgresInstance validates the Snowflake `DESCRIBE POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-postgres-instance
//
// Syntax:
//
//	{ DESC | DESCRIBE } POSTGRES INSTANCE <name>
func (v *Validator) ParseDescribePostgresInstance() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("POSTGRES", "INSTANCE") },
		v.parseIdentPath,
	)
}

// ParseDescribePrivacyPolicy validates the Snowflake `DESCRIBE PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-privacy-policy
//
// Syntax:
//
//	{ DESC | DESCRIBE } PRIVACY POLICY <name>
func (v *Validator) ParseDescribePrivacyPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("PRIVACY", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeProcedure validates the Snowflake `DESCRIBE PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-procedure
//
// Syntax:
//
//	DESC[RIBE] PROCEDURE <procedure_name> ( [ <arg_data_type> [ , <arg_data_type_2> ... ] ] )
func (v *Validator) ParseDescribeProcedure() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("PROCEDURE") },
		v.parseIdentPath,
		func() bool { return v.consumeBalancedParens() },
	)
}

// ParseDescribeProjectionPolicy validates the Snowflake `DESCRIBE PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-projection-policy
//
// Syntax:
//
//	DESC[RIBE] PROJECTION POLICY <name>
func (v *Validator) ParseDescribeProjectionPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("PROJECTION", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeResult validates the Snowflake `DESCRIBE RESULT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-result
//
// Syntax:
//
//	DESC[RIBE] RESULT { '<query_id>' | LAST_QUERY_ID() }
func (v *Validator) ParseDescribeResult() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("RESULT") },
		func() bool {
			return v.Choice(
				v.parseString,
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.consumeBalancedParens() },
					)
				},
			)
		},
	)
}

// ParseDescribeRowAccessPolicy validates the Snowflake `DESCRIBE ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-row-access-policy
//
// Syntax:
//
//	DESC[RIBE] ROW ACCESS POLICY <name>;
func (v *Validator) ParseDescribeRowAccessPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("ROW", "ACCESS", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeSchema validates the Snowflake `DESCRIBE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-schema
//
// Syntax:
//
//	DESC[RIBE] SCHEMA <schema_name>
func (v *Validator) ParseDescribeSchema() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SCHEMA") },
		v.parseIdentPath,
	)
}

// ParseDescribeSearchOptimization validates the Snowflake `DESCRIBE SEARCH OPTIMIZATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-search-optimization
//
// Syntax:
//
//	DESC[RIBE] SEARCH OPTIMIZATION ON <table_name>;
func (v *Validator) ParseDescribeSearchOptimization() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("SEARCH", "OPTIMIZATION") },
		func() bool { return v.MatchWord("ON") },
		v.parseIdentPath,
	)
}

// ParseDescribeSecret validates the Snowflake `DESCRIBE SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-secret
//
// Syntax:
//
//	{ DESC | DESCRIBE } SECRET <name>
func (v *Validator) ParseDescribeSecret() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SECRET") },
		v.parseIdentPath,
	)
}

// ParseDescribeSemanticView validates the Snowflake `DESCRIBE SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-semantic-view
//
// Syntax:
//
//	{ DESCRIBE | DESC } SEMANTIC VIEW <name>
func (v *Validator) ParseDescribeSemanticView() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("SEMANTIC", "VIEW") },
		v.parseIdentPath,
	)
}

// ParseDescribeSequence validates the Snowflake `DESCRIBE SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-sequence
//
// Syntax:
//
//	DESC[RIBE] SEQUENCE <name>
func (v *Validator) ParseDescribeSequence() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SEQUENCE") },
		v.parseIdentPath,
	)
}

// ParseDescribeService validates the Snowflake `DESCRIBE SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-service
//
// Syntax:
//
//	DESC[RIBE] SERVICE <name>
func (v *Validator) ParseDescribeService() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SERVICE") },
		v.parseIdentPath,
	)
}

// ParseDescribeSessionPolicy validates the Snowflake `DESCRIBE SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-session-policy
//
// Syntax:
//
//	{ DESCRIBE | DESC } SESSION POLICY <name>
func (v *Validator) ParseDescribeSessionPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("SESSION", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeShare validates the Snowflake `DESCRIBE SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-share
//
// Syntax:
//
//	Providers (outbound share):
//
//	DESC[RIBE] SHARE <name>
//
//	Consumers (inbound share):
//
//	DESC[RIBE] SHARE <provider_account>.<share_name>
func (v *Validator) ParseDescribeShare() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SHARE") },
		v.parseIdentPath,
	)
}

// ParseDescribeSnapshot validates the Snowflake `DESCRIBE SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot
//
// Syntax:
//
//	{ DESC | DESCRIBE } SNAPSHOT <name>
func (v *Validator) ParseDescribeSnapshot() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SNAPSHOT") },
		v.parseIdentPath,
	)
}

// ParseDescribeSnapshotPolicy validates the Snowflake `DESCRIBE SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot-policy
//
// Syntax:
//
//	DESC[RIBE] SNAPSHOT POLICY <name>
func (v *Validator) ParseDescribeSnapshotPolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("SNAPSHOT", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeSnapshotSet validates the Snowflake `DESCRIBE SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-snapshot-set
//
// Syntax:
//
//	DESC[RIBE] SNAPSHOT SET <name>
func (v *Validator) ParseDescribeSnapshotSet() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("SNAPSHOT", "SET") },
		v.parseIdentPath,
	)
}

// ParseDescribeSpecification validates the Snowflake `DESCRIBE SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-specification
//
// Syntax:
//
//	{ DESCRIBE | DESC }  SPECIFICATION <name> [ IN APPLICATION <app_name> ];
func (v *Validator) ParseDescribeSpecification() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("SPECIFICATION") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.phrase("IN", "APPLICATION") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseDescribeStage validates the Snowflake `DESCRIBE STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-stage
//
// Syntax:
//
//	DESC[RIBE] STAGE <name>
func (v *Validator) ParseDescribeStage() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("STAGE") },
		v.parseIdentPath,
	)
}

// ParseDescribeStorageLifecyclePolicy validates the Snowflake `DESCRIBE STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-storage-lifecycle-policy
//
// Syntax:
//
//	{ DESC | DESCRIBE } STORAGE LIFECYCLE POLICY <policy_name>
func (v *Validator) ParseDescribeStorageLifecyclePolicy() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("STORAGE", "LIFECYCLE", "POLICY") },
		v.parseIdentPath,
	)
}

// ParseDescribeStream validates the Snowflake `DESCRIBE STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-stream
//
// Syntax:
//
//	DESC[RIBE] STREAM <name>
func (v *Validator) ParseDescribeStream() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("STREAM") },
		v.parseIdentPath,
	)
}

// ParseDescribeStreamlit validates the Snowflake `DESCRIBE STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-streamlit
//
// Syntax:
//
//	DESC[RIBE] STREAMLIT <name>
func (v *Validator) ParseDescribeStreamlit() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("STREAMLIT") },
		v.parseIdentPath,
	)
}

// ParseDescribeTable validates the Snowflake `DESCRIBE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-table
//
// Syntax:
//
//	{ DESCRIBE | DESC } TABLE <name> [ TYPE = { COLUMNS | STAGE } ]
func (v *Validator) ParseDescribeTable() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("TABLE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.option("TYPE", v.wordsValue("COLUMNS", "STAGE")))
		},
	)
}

// ParseDescribeTask validates the Snowflake `DESCRIBE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-task
//
// Syntax:
//
//	DESC[RIBE] TASK <name>
func (v *Validator) ParseDescribeTask() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("TASK") },
		v.parseIdentPath,
	)
}

// ParseDescribeTransaction validates the Snowflake `DESCRIBE TRANSACTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-transaction
//
// Syntax:
//
//	{ DESC | DESCRIBE } TRANSACTION <transaction_id>
func (v *Validator) ParseDescribeTransaction() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("TRANSACTION") },
		v.parseScalar,
	)
}

// ParseDescribeType validates the Snowflake `DESCRIBE TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-type
//
// Syntax:
//
//	{ DESC | DESCRIBE } TYPE <name>
func (v *Validator) ParseDescribeType() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("TYPE") },
		v.parseIdentPath,
	)
}

// ParseDescribeUser validates the Snowflake `DESCRIBE USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-user
//
// Syntax:
//
//	{ DESC | DESCRIBE } USER <name>
func (v *Validator) ParseDescribeUser() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("USER") },
		v.parseIdentPath,
	)
}

// ParseDescribeView validates the Snowflake `DESCRIBE VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-view
//
// Syntax:
//
//	DESC[RIBE] VIEW <name>
func (v *Validator) ParseDescribeView() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("VIEW") },
		v.parseIdentPath,
	)
}

// ParseDescribeWarehouse validates the Snowflake `DESCRIBE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-warehouse
//
// Syntax:
//
//	DESC[RIBE] WAREHOUSE <name>
func (v *Validator) ParseDescribeWarehouse() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.MatchWord("WAREHOUSE") },
		v.parseIdentPath,
	)
}

// ParseDescribeApplicationService validates the Snowflake `DESCRIBE APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-application-service
//
// Syntax:
//
//	{ DESC | DESCRIBE } APPLICATION SERVICE <name>
func (v *Validator) ParseDescribeApplicationService() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("APPLICATION", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseDescribeArtifactRepository validates the Snowflake `DESCRIBE ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc-artifact-repository
//
// Syntax:
//
//	{ DESC | DESCRIBE } ARTIFACT REPOSITORY <name>
func (v *Validator) ParseDescribeArtifactRepository() bool {
	return v.Sequence(
		v.wordsValue("DESC", "DESCRIBE"),
		func() bool { return v.phrase("ARTIFACT", "REPOSITORY") },
		v.parseIdentPath,
	)
}
