package sqlgrammar

// SHOW commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseShowObjs validates the Snowflake `SHOW <objects>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowObjs() bool {
	return true
}

// ParseShowAccounts validates the Snowflake `SHOW ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-accounts
//
// Syntax:
//
//	SHOW ACCOUNTS [ HISTORY ] [ LIKE '<pattern>' ]
func (v *Validator) ParseShowAccounts() bool {
	return true
}

// ParseShowAgents validates the Snowflake `SHOW AGENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-agents
//
// Syntax:
//
//	SHOW AGENTS
//	  [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE <db_name> | SCHEMA [<db_name>.]<schema_name> } ]
//	  [ STARTS WITH '<string>' ]
//	  [ LIMIT <rows> [ FROM '<string_from>' ] ]
func (v *Validator) ParseShowAgents() bool {
	return true
}

// ParseShowAggregationPolicies validates the Snowflake `SHOW AGGREGATION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-aggregation-policies
//
// Syntax:
//
//	SHOW AGGREGATION POLICIES  [ LIKE '<pattern>' ]
//	                           [ IN
//	                               {
//	                                 ACCOUNT                  |
//
//	                                 DATABASE [ <database_name> ] |
//
//	                                 SCHEMA [ <schema_name> ]     |
//	                               }
//	                           ]
func (v *Validator) ParseShowAggregationPolicies() bool {
	return true
}

// ParseShowAlerts validates the Snowflake `SHOW ALERTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-alerts
//
// Syntax:
//
//	SHOW [ TERSE ] ALERTS [ LIKE '<pattern>' ]
//	                      [ IN
//	                            {
//	                              ACCOUNT                                         |
//
//	                              DATABASE                                        |
//	                              DATABASE <database_name>                        |
//
//	                              SCHEMA                                          |
//	                              SCHEMA <schema_name>                            |
//	                              <schema_name>
//
//	                              APPLICATION <application_name>                  |
//	                              APPLICATION PACKAGE <application_package_name>  |
//	                            }
//	                      ]
//	                      [ STARTS WITH '<name_string>' ]
//	                      [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowAlerts() bool {
	return true
}

// ParseShowApplicationPackages validates the Snowflake `SHOW APPLICATION PACKAGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-packages
//
// Syntax:
//
//	SHOW APPLICATION PACKAGES [ LIKE '<pattern>' ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ];
func (v *Validator) ParseShowApplicationPackages() bool {
	return true
}

// ParseShowApplicationRoles validates the Snowflake `SHOW APPLICATION ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-roles
//
// Syntax:
//
//	SHOW APPLICATION ROLES [ LIKE <pattern> ] IN APPLICATION <name>
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowApplicationRoles() bool {
	return true
}

// ParseShowApplications validates the Snowflake `SHOW APPLICATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-applications
//
// Syntax:
//
//	SHOW APPLICATIONS [ LIKE '<pattern>' ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ];
func (v *Validator) ParseShowApplications() bool {
	return true
}

// ParseShowAuthenticationPolicies validates the Snowflake `SHOW AUTHENTICATION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-authentication-policies
//
// Syntax:
//
//	SHOW AUTHENTICATION POLICIES
//	  [ LIKE '<pattern>' ]
//	  [ IN
//	       {
//	         ACCOUNT                                         |
//
//	         DATABASE                                        |
//	         DATABASE <database_name>                        |
//
//	         SCHEMA                                          |
//	         SCHEMA <schema_name>                            |
//
//	         APPLICATION <application_name>                  |
//	         APPLICATION PACKAGE <application_package_name>  |
//	       }
//	    |
//	    ON
//	       {
//	         ACCOUNT           |
//	         USER <user_name>  |
//	       }
//	  ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowAuthenticationPolicies() bool {
	return true
}

// ParseShowAvailableListings validates the Snowflake `SHOW AVAILABLE LISTINGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-listings
//
// Syntax:
//
//	SHOW [ TERSE ] AVAILABLE LISTINGS
//	    [ LIMIT <rows> ]
//	    [ IS_IMPORTED = TRUE ]
//	    [ IS_ORGANIZATION = TRUE ]
//	    [ IS_SHARED_WITH_ME = TRUE ]
func (v *Validator) ParseShowAvailableListings() bool {
	return true
}

// ParseShowAvailableOffers validates the Snowflake `SHOW AVAILABLE OFFERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-offers
//
// Syntax:
//
//	SHOW AVAILABLE OFFERS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowAvailableOffers() bool {
	return true
}

// ParseShowAvailableOrganizationProfiles validates the Snowflake `SHOW AVAILABLE ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-organization-profiles
//
// Syntax:
//
//	SHOW AVAILABLE ORGANIZATION PROFILES
func (v *Validator) ParseShowAvailableOrganizationProfiles() bool {
	return true
}

// ParseShowBackupPolicies validates the Snowflake `SHOW BACKUP POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backup-policies
//
// Syntax:
//
//	SHOW BACKUP POLICIES
//	   [ LIKE '<pattern>' ]
//	   [ IN { ACCOUNT | DATABASE | DATABASE <db_name> | SCHEMA | SCHEMA <schema_name> }
//	     [ STARTS WITH '<name_string>' ]
//	     [ LIMIT <rows> ]
//	   ]
func (v *Validator) ParseShowBackupPolicies() bool {
	return true
}

// ParseShowBackupSets validates the Snowflake `SHOW BACKUP SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backup-sets
//
// Syntax:
//
//	SHOW BACKUP SETS
//	   [ LIKE '<pattern>' ]
//	   [ IN { ACCOUNT | DATABASE | DATABASE <db_name> | SCHEMA | SCHEMA <schema_name> }
//	     [ STARTS WITH '<name_string>' ]
//	     [ LIMIT <rows> ]
//	   ]
func (v *Validator) ParseShowBackupSets() bool {
	return true
}

// ParseShowBackupsInBackupSet validates the Snowflake `SHOW BACKUPS IN BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backups-in-backup-set
//
// Syntax:
//
//	SHOW BACKUPS IN BACKUP SET <name>
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowBackupsInBackupSet() bool {
	return true
}

// ParseShowCallerGrants validates the Snowflake `SHOW CALLER GRANTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-caller-grants
//
// Syntax:
//
//	SHOW CALLER GRANTS
//	{
//	{ ON <object_type> <object_name> | ON ACCOUNT }
//	| TO { ROLE | DATABASE ROLE }  <owner_name>
//	}
func (v *Validator) ParseShowCallerGrants() bool {
	return true
}

// ParseShowCatalogIntegrations validates the Snowflake `SHOW CATALOG INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-catalog-integrations
//
// Syntax:
//
//	SHOW CATALOG INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowCatalogIntegrations() bool {
	return true
}

// ParseShowChannels validates the Snowflake `SHOW CHANNELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-channels
//
// Syntax:
//
//	SHOW CHANNELS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>            |
//
//	                  TABLE                    |
//	                  TABLE <table_name>       |
//
//	                  PIPE                     |
//	                  PIPE <pipe_name>
//	                }
//	           ]
func (v *Validator) ParseShowChannels() bool {
	return true
}

// ParseShowClasses validates the Snowflake `SHOW CLASSES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-classes
//
// Syntax:
//
//	SHOW CLASSES [ LIKE '<pattern>' ]
//	             [ IN DATABASE [ <db_name> ] ]
//	             [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowClasses() bool {
	return true
}

// ParseShowColumns validates the Snowflake `SHOW COLUMNS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-columns
//
// Syntax:
//
//	SHOW COLUMNS [ LIKE '<pattern>' ]
//	             [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] | TABLE | [ TABLE ] <table_name> | VIEW | [ VIEW ] <view_name> } | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> ]
func (v *Validator) ParseShowColumns() bool {
	return true
}

// ParseShowComputePoolInstanceFamilies validates the Snowflake `SHOW COMPUTE POOL INSTANCE FAMILIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-compute-pool-instance-families
//
// Syntax:
//
//	SHOW COMPUTE POOL INSTANCE FAMILIES
func (v *Validator) ParseShowComputePoolInstanceFamilies() bool {
	return true
}

// ParseShowComputePools validates the Snowflake `SHOW COMPUTE POOLS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-compute-pools
//
// Syntax:
//
//	SHOW COMPUTE POOLS [ LIKE '<pattern>' ]
//	                   [ STARTS WITH '<name_string>' ]
//	                   [ LIMIT <ROWS> [ FROM '<name-string>' ] ]
func (v *Validator) ParseShowComputePools() bool {
	return true
}

// ParseShowConfigurations validates the Snowflake `SHOW CONFIGURATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-configurations
//
// Syntax:
//
//	SHOW CONFIGURATIONS [ IN APPLICATION <app> ]
func (v *Validator) ParseShowConfigurations() bool {
	return true
}

// ParseShowConnections validates the Snowflake `SHOW CONNECTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-connections
//
// Syntax:
//
//	SHOW CONNECTIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowConnections() bool {
	return true
}

// ParseShowContacts validates the Snowflake `SHOW CONTACTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-contacts
//
// Syntax:
//
//	SHOW CONTACTS [ LIKE '<pattern>' ]
//	          [ IN
//	              {
//	                ACCOUNT                  |
//
//	                DATABASE                 |
//	                DATABASE <database_name> |
//
//	                SCHEMA                   |
//	                SCHEMA <schema_name>     |
//	                <schema_name>
//	              }
//	          ]
//	          [ STARTS WITH '<name_string>' ]
//	          [ LIMIT <rows> ]
//	          [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowContacts() bool {
	return true
}

// ParseShowCortexSearchServices validates the Snowflake `SHOW CORTEX SEARCH SERVICES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-cortex-search
//
// Syntax:
//
//	SHOW CORTEX SEARCH SERVICES
//	  [ LIKE PATTERN '<pattern>' ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowCortexSearchServices() bool {
	return true
}

// ParseShowDataMetricFunctions validates the Snowflake `SHOW DATA METRIC FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-data-metric-functions
//
// Syntax:
//
//	SHOW DATA METRIC FUNCTIONS
//	  [ LIKE '<pattern>' ]
//	  [ IN
//	      {
//	        ACCOUNT                  |
//
//	        DATABASE                 |
//	        DATABASE <database_name> |
//
//	        SCHEMA                   |
//	        SCHEMA <schema_name>     |
//	        <schema_name>
//	      }
//	  ]
//	  [ STARTS WITH '<name_string>' ]
func (v *Validator) ParseShowDataMetricFunctions() bool {
	return true
}

// ParseShowDatabaseRoles validates the Snowflake `SHOW DATABASE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-database-roles
//
// Syntax:
//
//	SHOW DATABASE ROLES IN DATABASE <name>
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowDatabaseRoles() bool {
	return true
}

// ParseShowDatabases validates the Snowflake `SHOW DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases
//
// Syntax:
//
//	SHOW [ TERSE ] DATABASES [ HISTORY ] [ LIKE '<pattern>' ]
//	                                     [ STARTS WITH '<name_string>' ]
//	                                     [ LIMIT <rows> [ FROM '<name_string>' ] ]
//	                                     [ WITH PRIVILEGES <object_privilege> [ , <object_privilege> [ , ... ] ] ]
func (v *Validator) ParseShowDatabases() bool {
	return true
}

// ParseShowDatabasesInFailoverGroup validates the Snowflake `SHOW DATABASES IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-failover-group
//
// Syntax:
//
//	SHOW DATABASES IN FAILOVER GROUP <name>
func (v *Validator) ParseShowDatabasesInFailoverGroup() bool {
	return true
}

// ParseShowDatabasesInReplicationGroup validates the Snowflake `SHOW DATABASES IN REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-replication-group
//
// Syntax:
//
//	SHOW DATABASES IN REPLICATION GROUP <name>
func (v *Validator) ParseShowDatabasesInReplicationGroup() bool {
	return true
}

// ParseShowDatasets validates the Snowflake `SHOW DATASETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-datasets
//
// Syntax:
//
//	SHOW DATASETS
//	  [ LIKE '<pattern>' ]
//	  [ IN { SCHEMA <schema_name> | DATABASE <db_name> | ACCOUNT } ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowDatasets() bool {
	return true
}

// ParseShowDbtProjects validates the Snowflake `SHOW DBT PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dbt-projects
//
// Syntax:
//
//	SHOW DBT PROJECTS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>
//	                }
//	           ]
//	           [ STARTS WITH '<name_string>' ]
//	           [ LIMIT <rows> ]
//	           [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowDbtProjects() bool {
	return true
}

// ParseShowDcmProjects validates the Snowflake `SHOW DCM PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dcm-projects
//
// Syntax:
//
//	SHOW [ TERSE ] DCM PROJECTS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>
//	                }
//	           ]
//	           [ LIMIT <rows> ]
func (v *Validator) ParseShowDcmProjects() bool {
	return true
}

// ParseShowDelegatedAuthorizations validates the Snowflake `SHOW DELEGATED AUTHORIZATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-delegated-authorizations
//
// Syntax:
//
//	SHOW DELEGATED AUTHORIZATIONS
//
//	SHOW DELEGATED AUTHORIZATIONS BY USER <username>
//
//	SHOW DELEGATED AUTHORIZATIONS TO SECURITY INTEGRATION <integration_name>
func (v *Validator) ParseShowDelegatedAuthorizations() bool {
	return true
}

// ParseShowDeploymentsInDcmProject validates the Snowflake `SHOW DEPLOYMENTS IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-deployments-in-dcm-project
//
// Syntax:
//
//	SHOW DEPLOYMENTS IN DCM PROJECT <name> [ LIMIT <rows> ]
func (v *Validator) ParseShowDeploymentsInDcmProject() bool {
	return true
}

// ParseShowDynamicTables validates the Snowflake `SHOW DYNAMIC TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dynamic-tables
//
// Syntax:
//
//	SHOW DYNAMIC TABLES [ LIKE '<pattern>' ]
//	                    [ IN
//	                      {
//	                           ACCOUNT              |
//
//	                           DATABASE             |
//	                           DATABASE <db_name>   |
//
//	                           SCHEMA               |
//	                           SCHEMA <schema_name> |
//	                           <schema_name>
//	                      }
//	                    ]
//	                    [ STARTS WITH '<name_string>' ]
//	                    [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowDynamicTables() bool {
	return true
}

// ParseShowEndpoints validates the Snowflake `SHOW ENDPOINTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-endpoints
//
// Syntax:
//
//	SHOW ENDPOINTS IN SERVICE <name>
func (v *Validator) ParseShowEndpoints() bool {
	return true
}

// ParseShowEntitiesInDcmProject validates the Snowflake `SHOW ENTITIES IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-entities-in-dcm-project
//
// Syntax:
//
//	SHOW ENTITIES IN DCM PROJECT <name> [ LIMIT <rows> ]
//
//	SHOW ENTITIES LIKE <pattern> IN DCM PROJECT <name>;
//
//	SHOW ENTITIES IN DCM PROJECT <name> STARTS WITH <prefix>;
//
//	SHOW ENTITIES IN DCM PROJECT <name> LIMIT <n> FROM <cursor>;
func (v *Validator) ParseShowEntitiesInDcmProject() bool {
	return true
}

// ParseShowEventTables validates the Snowflake `SHOW EVENT TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-tables
//
// Syntax:
//
//	SHOW [ TERSE ] EVENT TABLES [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowEventTables() bool {
	return true
}

// ParseShowExperiments validates the Snowflake `SHOW EXPERIMENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-experiments
//
// Syntax:
//
//	SHOW EXPERIMENTS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                      |
//	                  DATABASE [ <database_name> ] |
//	                  SCHEMA [ <schema_name> ]
//	                }
//	           ]
func (v *Validator) ParseShowExperiments() bool {
	return true
}

// ParseShowExternalAgents validates the Snowflake `SHOW EXTERNAL AGENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-agents
//
// Syntax:
//
//	SHOW EXTERNAL AGENTS [ LIKE '<pattern>' ]
//	                     [ IN { ACCOUNT | DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
func (v *Validator) ParseShowExternalAgents() bool {
	return true
}

// ParseShowExternalFunctions validates the Snowflake `SHOW EXTERNAL FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-functions
//
// Syntax:
//
//	SHOW EXTERNAL FUNCTIONS [ LIKE '<pattern>' ]
//	           [ IN { APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> }  ]
func (v *Validator) ParseShowExternalFunctions() bool {
	return true
}

// ParseShowExternalTables validates the Snowflake `SHOW EXTERNAL TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-tables
//
// Syntax:
//
//	SHOW [ TERSE ] EXTERNAL TABLES [ LIKE '<pattern>' ]
//	                               [ IN
//	                                        {
//	                                          ACCOUNT                                         |
//
//	                                          DATABASE                                        |
//	                                          DATABASE <database_name>                        |
//
//	                                          SCHEMA                                          |
//	                                          SCHEMA <schema_name>                            |
//	                                          <schema_name>
//
//	                                          APPLICATION <application_name>                  |
//	                                          APPLICATION PACKAGE <application_package_name>  |
//	                                        }
//	                               ]
//	                               [ STARTS WITH '<name_string>' ]
//	                               [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowExternalTables() bool {
	return true
}

// ParseShowExternalVolumes validates the Snowflake `SHOW EXTERNAL VOLUMES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-volumes
//
// Syntax:
//
//	SHOW EXTERNAL VOLUMES [ LIKE '<pattern>' ]
func (v *Validator) ParseShowExternalVolumes() bool {
	return true
}

// ParseShowFailoverGroups validates the Snowflake `SHOW FAILOVER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-failover-groups
//
// Syntax:
//
//	SHOW FAILOVER GROUPS [ IN ACCOUNT <account> ]
func (v *Validator) ParseShowFailoverGroups() bool {
	return true
}

// ParseShowFeaturePolicies validates the Snowflake `SHOW FEATURE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-feature-policies
//
// Syntax:
//
//	SHOW FEATURE POLICIES
//	  [ IN
//	    {
//	      ACCOUNT                                        |
//	      APPLICATION {app_name}                         |
//	      APPLICATION PACKAGE {app_package_name}         |
//	      DATABASE {database_name}                       |
//	      SCHEMA {schema_name}                           |
//	    }
//	  ]
//
//	SHOW FEATURE POLICIES ON ACCOUNT
//
//	SHOW FEATURE POLICIES ON APPLICATION <application_name>
func (v *Validator) ParseShowFeaturePolicies() bool {
	return true
}

// ParseShowFileFormats validates the Snowflake `SHOW FILE FORMATS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-file-formats
//
// Syntax:
//
//	SHOW FILE FORMATS [ LIKE '<pattern>' ]
//	                  [ IN
//	                       {
//	                          ACCOUNT                                         |
//
//	                          DATABASE                                        |
//	                          DATABASE <database_name>                        |
//
//	                          SCHEMA                                          |
//	                          SCHEMA <schema_name>                            |
//	                          <schema_name>
//
//	                          APPLICATION <application_name>                  |
//	                          APPLICATION PACKAGE <application_package_name>  |
//	                       }
//	                  ]
func (v *Validator) ParseShowFileFormats() bool {
	return true
}

// ParseShowFunctions validates the Snowflake `SHOW FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-functions
//
// Syntax:
//
//	SHOW FUNCTIONS [ LIKE '<pattern>' ]
//	  [ IN
//	    {
//	      ACCOUNT                       |
//
//	      CLASS <class_name>            |
//
//	      DATABASE                      |
//	      DATABASE <database_name>      |
//
//	      SCHEMA                        |
//	      SCHEMA <schema_name>          |
//	      <schema_name>
//	    }
//	  ]
func (v *Validator) ParseShowFunctions() bool {
	return true
}

// ParseShowFunctionsInModel validates the Snowflake `SHOW FUNCTIONS IN MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-functions-in-model
//
// Syntax:
//
//	SHOW FUNCTIONS [ LIKE '<pattern>' ] IN MODEL <model_name>
//	               [ VERSION <version_name> ]
func (v *Validator) ParseShowFunctionsInModel() bool {
	return true
}

// ParseShowGateways validates the Snowflake `SHOW GATEWAYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-gateways
//
// Syntax:
//
//	SHOW GATEWAYS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>
//	                }
//	           ]
//	           [ STARTS WITH '<name_string>' ]
//	           [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowGateways() bool {
	return true
}

// ParseShowGitBranches validates the Snowflake `SHOW GIT BRANCHES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-branches
//
// Syntax:
//
//	SHOW GIT BRANCHES [ LIKE '<pattern>' ] IN [ GIT REPOSITORY ] <repository_name>
func (v *Validator) ParseShowGitBranches() bool {
	return true
}

// ParseShowGitRepositories validates the Snowflake `SHOW GIT REPOSITORIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-repositories
//
// Syntax:
//
//	SHOW GIT REPOSITORIES [ LIKE '<pattern>' ]
//	  [ IN
//	      {
//	        ACCOUNT                  |
//
//	        DATABASE                 |
//	        DATABASE <database_name> |
//
//	        SCHEMA                   |
//	        SCHEMA <schema_name>     |
//	        <schema_name>
//	      }
//	  ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowGitRepositories() bool {
	return true
}

// ParseShowGitTags validates the Snowflake `SHOW GIT TAGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-tags
//
// Syntax:
//
//	SHOW GIT TAGS [ LIKE '<pattern>' ] IN [ GIT REPOSITORY ] <repository_name>
func (v *Validator) ParseShowGitTags() bool {
	return true
}

// ParseShowGlobalAccounts validates the Snowflake `SHOW GLOBAL ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-global-accounts
//
// Syntax:
//
//	SHOW GLOBAL ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowGlobalAccounts() bool {
	return true
}

// ParseShowGrants validates the Snowflake `SHOW GRANTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-grants
//
// Syntax:
//
//	SHOW GRANTS [ LIMIT <rows> ]
//
//	SHOW GRANTS ON ACCOUNT [ LIMIT <rows> ]
//
//	SHOW GRANTS ON <object_type> <object_name> [ LIMIT <rows> ]
//
//	SHOW GRANTS TO {
//	  APPLICATION <app_name>
//	  | APPLICATION ROLE [ <app_name>. ]<app_role_name>
//	  | SERVICE ROLE <service_name>!<service_role_name>
//	  | <class_name> ROLE <instance_name>!<instance_role_name>
//	  | ROLE <role_name>
//	  | SHARE <share_name> [ IN APPLICATION PACKAGE <app_package_name> ]
//	  | USER <user_name>
//	} [ LIMIT <rows> ]
//
//	SHOW GRANTS OF {
//	  APPLICATION ROLE <app_role_name>
//	  | SERVICE ROLE <service_name>!<service_role_name>
//	  | ROLE <role_name>
//	} [ LIMIT <rows> ]
//
//	SHOW GRANTS OF SHARE <share_name> [ LIMIT <rows> ]
//
//	SHOW FUTURE GRANTS IN SCHEMA { <schema_name> } [ LIMIT <rows> ]
//
//	SHOW FUTURE GRANTS IN DATABASE { <database_name> } [ LIMIT <rows> ]
//
//	SHOW FUTURE GRANTS TO ROLE <role_name> [ LIMIT <rows> ]
//
//	SHOW FUTURE GRANTS TO DATABASE ROLE <database_role_name>
func (v *Validator) ParseShowGrants() bool {
	return true
}

// ParseShowGrantsInDcmProject validates the Snowflake `SHOW GRANTS IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-grants-in-dcm-project
//
// Syntax:
//
//	SHOW GRANTS IN DCM PROJECT <name> [ LIMIT <rows> ]
//
//	SHOW FUTURE GRANTS IN DCM PROJECT <name> [ LIMIT <rows> ]
func (v *Validator) ParseShowGrantsInDcmProject() bool {
	return true
}

// ParseShowHybridTables validates the Snowflake `SHOW HYBRID TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-hybrid-tables
//
// Syntax:
//
//	SHOW [ TERSE ] [ HYBRID ] TABLES [ LIKE '<pattern>' ]
//	                                 [ IN { ACCOUNT | DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
//	                                 [ STARTS WITH '<name_string>' ]
//	                                 [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowHybridTables() bool {
	return true
}

// ParseShowIcebergTables validates the Snowflake `SHOW ICEBERG TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-iceberg-tables
//
// Syntax:
//
//	SHOW [ TERSE ] [ ICEBERG ] TABLES [ LIKE '<pattern>' ]
//	                                  [ IN
//	                                        {
//	                                          ACCOUNT                  |
//
//	                                          DATABASE                 |
//	                                          DATABASE <database_name> |
//
//	                                          SCHEMA                   |
//	                                          SCHEMA <schema_name>     |
//	                                          <schema_name>
//	                                        }
//	                                  ]
//	                                  [ STARTS WITH '<name_string>' ]
//	                                  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowIcebergTables() bool {
	return true
}

// ParseShowImageRepositories validates the Snowflake `SHOW IMAGE REPOSITORIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-image-repositories
//
// Syntax:
//
//	SHOW IMAGE REPOSITORIES [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>
//	                }
//	           ]
func (v *Validator) ParseShowImageRepositories() bool {
	return true
}

// ParseShowImagesInImageRepository validates the Snowflake `SHOW IMAGES IN IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-images-in-image-repository
//
// Syntax:
//
//	SHOW IMAGES IN IMAGE REPOSITORY <name>
func (v *Validator) ParseShowImagesInImageRepository() bool {
	return true
}

// ParseShowIndexes validates the Snowflake `SHOW INDEXES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-indexes
//
// Syntax:
//
//	SHOW [ TERSE ] INDEXES
//	  [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] | TABLE | TABLE <table_name> } ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowIndexes() bool {
	return true
}

// ParseShowIntegrations validates the Snowflake `SHOW INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-integrations
//
// Syntax:
//
//	SHOW [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ] INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowIntegrations() bool {
	return true
}

// ParseShowJoinPolicies validates the Snowflake `SHOW JOIN POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-join-policies
//
// Syntax:
//
//	SHOW JOIN POLICIES  [ LIKE '<pattern>' ]
//	                           [ IN
//	                               {
//	                                 ACCOUNT |
//	                                 DATABASE [ <database_name> ] |
//	                                 SCHEMA [ <schema_name> ] |
//	                               }
//	                           ]
func (v *Validator) ParseShowJoinPolicies() bool {
	return true
}

// ParseShowListings validates the Snowflake `SHOW LISTINGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-listings
//
// Syntax:
//
//	SHOW LISTINGS [ LIKE '<pattern>' ]
//	              [ STARTS WITH '<name_string>' ]
//	              [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowListings() bool {
	return true
}

// ParseShowListingsInFailoverGroup validates the Snowflake `SHOW LISTINGS IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-listings-in-failover-group
//
// Syntax:
//
//	SHOW LISTINGS IN FAILOVER GROUP <name>
func (v *Validator) ParseShowListingsInFailoverGroup() bool {
	return true
}

// ParseShowLocks validates the Snowflake `SHOW LOCKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-locks
//
// Syntax:
//
//	SHOW LOCKS [ IN ACCOUNT ]
func (v *Validator) ParseShowLocks() bool {
	return true
}

// ParseShowMaintenancePolicies validates the Snowflake `SHOW MAINTENANCE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-maintenance-policies
//
// Syntax:
//
//	SHOW MAINTENANCE POLICIES { ON | IN } { ACCOUNT | APPLICATION <app_name> | <entity_type> <entity_name> }
func (v *Validator) ParseShowMaintenancePolicies() bool {
	return true
}

// ParseShowManagedAccounts validates the Snowflake `SHOW MANAGED ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-managed-accounts
//
// Syntax:
//
//	SHOW MANAGED ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowManagedAccounts() bool {
	return true
}

// ParseShowMaskingPolicies validates the Snowflake `SHOW MASKING POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-masking-policies
//
// Syntax:
//
//	SHOW MASKING POLICIES  [ LIKE '<pattern>' ]
//	                       [ IN
//	                            {
//	                              ACCOUNT                                         |
//
//	                              DATABASE                                        |
//	                              DATABASE <database_name>                        |
//
//	                              SCHEMA                                          |
//	                              SCHEMA <schema_name>                            |
//	                              <schema_name>
//
//	                              APPLICATION <application_name>                  |
//	                              APPLICATION PACKAGE <application_package_name>  |
//	                            }
//	                       ]
func (v *Validator) ParseShowMaskingPolicies() bool {
	return true
}

// ParseShowMaterializedViews validates the Snowflake `SHOW MATERIALIZED VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-materialized-views
//
// Syntax:
//
//	SHOW MATERIALIZED VIEWS [ LIKE '<pattern>' ]
//	                        [ IN
//	                             {
//	                               ACCOUNT                                         |
//
//	                               DATABASE                                        |
//	                               DATABASE <database_name>                        |
//
//	                               SCHEMA                                          |
//	                               SCHEMA <schema_name>                            |
//	                               <schema_name>
//
//	                               APPLICATION <application_name>                  |
//	                               APPLICATION PACKAGE <application_package_name>  |
//	                             }
//	                        ]
func (v *Validator) ParseShowMaterializedViews() bool {
	return true
}

// ParseShowMcpServers validates the Snowflake `SHOW MCP SERVERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-mcp-servers
//
// Syntax:
//
//	SHOW MCP SERVERS [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>
//	                }
//	           ]
func (v *Validator) ParseShowMcpServers() bool {
	return true
}

// ParseShowMfaMethods validates the Snowflake `SHOW MFA METHODS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-mfa-methods
//
// Syntax:
//
//	SHOW MFA METHODS [ FOR USER <user> ]
func (v *Validator) ParseShowMfaMethods() bool {
	return true
}

// ParseShowModelMonitors validates the Snowflake `SHOW MODEL MONITORS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-model-monitors
//
// Syntax:
//
//	SHOW MODEL MONITORS
//	[ LIKE <pattern> ]
//	[ IN
//	    {
//	      ACCOUNT                  |
//
//	      DATABASE                 |
//	      DATABASE <database_name> |
//
//	      SCHEMA                   |
//	      SCHEMA <schema_name>     |
//	      <schema_name>
//	    }
//	 ]
func (v *Validator) ParseShowModelMonitors() bool {
	return true
}

// ParseShowModels validates the Snowflake `SHOW MODELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-models
//
// Syntax:
//
//	SHOW MODELS [ LIKE '<pattern>' ]
//	            [ IN { DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
func (v *Validator) ParseShowModels() bool {
	return true
}

// ParseShowNetworkPolicies validates the Snowflake `SHOW NETWORK POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-network-policies
//
// Syntax:
//
//	SHOW NETWORK POLICIES
func (v *Validator) ParseShowNetworkPolicies() bool {
	return true
}

// ParseShowNetworkRules validates the Snowflake `SHOW NETWORK RULES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-network-rules
//
// Syntax:
//
//	SHOW NETWORK RULES [ LIKE '<pattern>' ]
//	                   [ IN { ACCOUNT | DATABASE [ <db_name> ] | [ SCHEMA ] [ <schema_name> ] } ]
//	                   [ STARTS WITH '<name_string>' ]
//	                   [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowNetworkRules() bool {
	return true
}

// ParseShowNotebookProjects validates the Snowflake `SHOW NOTEBOOK PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notebook-projects
//
// Syntax:
//
//	SHOW NOTEBOOK PROJECTS;
//
//	SHOW NOTEBOOK PROJECTS IN SCHEMA <database_name>.<schema_name>;
//
//	SHOW NOTEBOOK PROJECTS IN DATABASE <database_name>;
//
//	SHOW NOTEBOOK PROJECTS IN ACCOUNT;
func (v *Validator) ParseShowNotebookProjects() bool {
	return true
}

// ParseShowNotebooks validates the Snowflake `SHOW NOTEBOOKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notebooks
//
// Syntax:
//
//	SHOW NOTEBOOKS [ LIKE '<pattern>' ]
//	               [ IN
//	                     {
//	                       ACCOUNT                  |
//
//	                       DATABASE                 |
//	                       DATABASE <database_name> |
//
//	                       SCHEMA                   |
//	                       SCHEMA <schema_name>     |
//	                       <schema_name>
//	                     }
//	               ]
//	               [ STARTS WITH '<name_string>' ]
//	               [ LIMIT <rows> ]
//	               [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowNotebooks() bool {
	return true
}

// ParseShowNotificationIntegrations validates the Snowflake `SHOW NOTIFICATION INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notification-integrations
//
// Syntax:
//
//	SHOW NOTIFICATION INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowNotificationIntegrations() bool {
	return true
}

// ParseShowObjects validates the Snowflake `SHOW OBJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-objects
//
// Syntax:
//
//	SHOW [ TERSE ] OBJECTS [ LIKE '<pattern>' ]
//	                       [ IN
//	                             {
//	                               ACCOUNT                                         |
//
//	                               DATABASE                                        |
//	                               DATABASE <database_name>                        |
//
//	                               SCHEMA                                          |
//	                               SCHEMA <schema_name>                            |
//	                               <schema_name>
//
//	                               APPLICATION <application_name>                  |
//	                               APPLICATION PACKAGE <application_package_name>  |
//	                             }
//	                       ]
//	                       [ STARTS WITH '<name_string>' ]
//	                       [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowObjects() bool {
	return true
}

// ParseShowOnlineFeatureTables validates the Snowflake `SHOW ONLINE FEATURE TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-online-feature-tables
//
// Syntax:
//
//	SHOW ONLINE FEATURE TABLES [ LIKE '<pattern>' ]
//	                            [ IN
//	                               {
//	                                 ACCOUNT                  |
//	                                 DATABASE                 |
//	                                 DATABASE <database_name> |
//	                                 SCHEMA                   |
//	                                 SCHEMA <schema_name>     |
//	                                 <schema_name>
//	                               }
//	                            ]
//	                            [ STARTS WITH '<name_string>' ]
//	                            [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowOnlineFeatureTables() bool {
	return true
}

// ParseShowOpenListingProviders validates the Snowflake `SHOW OPEN LISTING PROVIDERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-open-listing-providers
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowOpenListingProviders() bool {
	return true
}

// ParseShowOrganizationAccounts validates the Snowflake `SHOW ORGANIZATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-accounts
//
// Syntax:
//
//	SHOW ORGANIZATION ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowOrganizationAccounts() bool {
	return true
}

// ParseShowOrganizationProfiles validates the Snowflake `SHOW ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-profiles
//
// Syntax:
//
//	SHOW ORGANIZATION PROFILES
func (v *Validator) ParseShowOrganizationProfiles() bool {
	return true
}

// ParseShowOrganizationUsers validates the Snowflake `SHOW ORGANIZATION USERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-users
//
// Syntax:
//
//	SHOW ORGANIZATION USERS [ IN ORGANIZATION USER GROUP <org_user_group> ]
func (v *Validator) ParseShowOrganizationUsers() bool {
	return true
}

// ParseShowOrganizationUserGroups validates the Snowflake `SHOW ORGANIZATION USER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-user-groups
//
// Syntax:
//
//	SHOW ORGANIZATION USER GROUPS
func (v *Validator) ParseShowOrganizationUserGroups() bool {
	return true
}

// ParseShowOrganizations validates the Snowflake `SHOW ORGANIZATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organizations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowOrganizations() bool {
	return true
}

// ParseShowPackagesPolicies validates the Snowflake `SHOW PACKAGES POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-packages-policies
//
// Syntax:
//
//	SHOW PACKAGES POLICIES [ IN
//	                            {
//	                              SCHEMA                   |
//	                              SCHEMA <schema_name>     |
//	                              <schema_name>
//	                            }
//	                       ]
func (v *Validator) ParseShowPackagesPolicies() bool {
	return true
}

// ParseShowPasswordPolicies validates the Snowflake `SHOW PASSWORD POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-password-policies
//
// Syntax:
//
//	SHOW PASSWORD POLICIES [ LIKE '<pattern>' ]
//	                       [ IN
//	                            {
//	                              ACCOUNT                                         |
//
//	                              DATABASE                                        |
//	                              DATABASE <database_name>                        |
//
//	                              SCHEMA                                          |
//	                              SCHEMA <schema_name>                            |
//
//	                              APPLICATION <application_name>                  |
//	                              APPLICATION PACKAGE <application_package_name>  |
//	                            }
//	                         |
//	                         ON
//	                            {
//	                              ACCOUNT           |
//	                              USER <user_name>  |
//	                            }
//	                       ]
//	                       [ STARTS WITH '<name_string>' ]
//	                       [ LIMIT <rows> ]
func (v *Validator) ParseShowPasswordPolicies() bool {
	return true
}

// ParseShowParameters validates the Snowflake `SHOW PARAMETERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-parameters
//
// Syntax:
//
//	SHOW PARAMETERS
//	  [ LIKE '<pattern>' ]
//	  [ { IN | FOR } {
//	        { SESSION | ACCOUNT }
//	      | { USER | WAREHOUSE | DATABASE | SCHEMA | TASK } [ <name> ]
//	      | TABLE [ <table_or_view_name> ]
//	    } ]
func (v *Validator) ParseShowParameters() bool {
	return true
}

// ParseShowPipes validates the Snowflake `SHOW PIPES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-pipes
//
// Syntax:
//
//	SHOW PIPES [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                                         |
//
//	                  DATABASE                                        |
//	                  DATABASE <database_name>                        |
//
//	                  SCHEMA                                          |
//	                  SCHEMA <schema_name>                            |
//	                  <schema_name>
//
//	                  APPLICATION <application_name>                  |
//	                  APPLICATION PACKAGE <application_package_name>  |
//	                }
//	           ]
func (v *Validator) ParseShowPipes() bool {
	return true
}

// ParseShowPostgresInstances validates the Snowflake `SHOW POSTGRES INSTANCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-postgres-instances
//
// Syntax:
//
//	SHOW POSTGRES INSTANCES [ LIKE '<pattern>' ]
//	                        [ STARTS WITH '<name_string>' ]
//	                        [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowPostgresInstances() bool {
	return true
}

// ParseShowPrimaryKeys validates the Snowflake `SHOW PRIMARY KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-primary-keys
//
// Syntax:
//
//	SHOW [ TERSE ] PRIMARY KEYS
//	    [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] | TABLE | [ TABLE ] <table_name> } ]
func (v *Validator) ParseShowPrimaryKeys() bool {
	return true
}

// ParseShowPrivileges validates the Snowflake `SHOW PRIVILEGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-privileges
//
// Syntax:
//
//	SHOW PRIVILEGES IN APPLICATION <name>
func (v *Validator) ParseShowPrivileges() bool {
	return true
}

// ParseShowProcedures validates the Snowflake `SHOW PROCEDURES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-procedures
//
// Syntax:
//
//	SHOW PROCEDURES [ LIKE '<pattern>' ]
//	  [ IN
//	    {
//	      ACCOUNT                                         |
//
//	      CLASS <class_name>                              |
//
//	      DATABASE                                        |
//	      DATABASE <database_name>                        |
//
//	      SCHEMA                                          |
//	      SCHEMA <schema_name>                            |
//	      <schema_name>
//
//	      APPLICATION <application_name>                  |
//	      APPLICATION PACKAGE <application_package_name>  |
//	    }
//	  ]
func (v *Validator) ParseShowProcedures() bool {
	return true
}

// ParseShowProvisionedThroughput validates the Snowflake `SHOW PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-provisioned-throughput
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowProvisionedThroughput() bool {
	return true
}

// ParseShowProjectionPolicies validates the Snowflake `SHOW PROJECTION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-projection-policies
//
// Syntax:
//
//	SHOW PROJECTION POLICIES [ LIKE '<pattern>' ]
//	                         [ IN
//	                              {
//	                                ACCOUNT                  |
//
//	                                DATABASE [ <database_name> ] |
//
//	                                SCHEMA [ <schema_name> ]     |
//	                              }
//	                         ]
func (v *Validator) ParseShowProjectionPolicies() bool {
	return true
}

// ParseShowQueries validates the Snowflake `SHOW QUERIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-queries
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowQueries() bool {
	return true
}

// ParseShowRegions validates the Snowflake `SHOW REGIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-regions
//
// Syntax:
//
//	SHOW REGIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowRegions() bool {
	return true
}

// ParseShowReplicatedDatabases validates the Snowflake `SHOW REPLICATED DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replicated-databases
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowReplicatedDatabases() bool {
	return true
}

// ParseShowReplicationAccounts validates the Snowflake `SHOW REPLICATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-accounts
//
// Syntax:
//
//	SHOW REPLICATION ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowReplicationAccounts() bool {
	return true
}

// ParseShowReplicationDatabases validates the Snowflake `SHOW REPLICATION DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-databases
//
// Syntax:
//
//	SHOW REPLICATION DATABASES [ LIKE '<pattern>' ]
//	                           [ WITH PRIMARY <account_identifier>.<primary_db_name> ]
func (v *Validator) ParseShowReplicationDatabases() bool {
	return true
}

// ParseShowReplicationGroups validates the Snowflake `SHOW REPLICATION GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-groups
//
// Syntax:
//
//	SHOW REPLICATION GROUPS [ IN ACCOUNT <account> ]
func (v *Validator) ParseShowReplicationGroups() bool {
	return true
}

// ParseShowResourceMonitors validates the Snowflake `SHOW RESOURCE MONITORS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-resource-monitors
//
// Syntax:
//
//	SHOW RESOURCE MONITORS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowResourceMonitors() bool {
	return true
}

// ParseShowRoles validates the Snowflake `SHOW ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-roles
//
// Syntax:
//
//	SHOW [ TERSE ] ROLES
//	  [ LIKE '<pattern>' ]
//	  [ IN CLASS <class_name> ]
//	  [ STARTS WITH '<name_string>']
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowRoles() bool {
	return true
}

// ParseShowRowAccessPolicies validates the Snowflake `SHOW ROW ACCESS POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-row-access-policies
//
// Syntax:
//
//	SHOW ROW ACCESS POLICIES [ LIKE '<pattern>' ]
//	                         [ LIMIT <rows> [ FROM '<name_string>' ] ]
//	                         [ IN
//	                              {
//	                                ACCOUNT                                         |
//
//	                                DATABASE                                        |
//	                                DATABASE <database_name>                        |
//
//	                                SCHEMA                                          |
//	                                SCHEMA <schema_name>                            |
//	                                <schema_name>
//
//	                                APPLICATION <application_name>                  |
//	                                APPLICATION PACKAGE <application_package_name>  |
//	                              }
//	                         ]
func (v *Validator) ParseShowRowAccessPolicies() bool {
	return true
}

// ParseShowSchemas validates the Snowflake `SHOW SCHEMAS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-schemas
//
// Syntax:
//
//	SHOW [ TERSE ] SCHEMAS
//	  [ HISTORY ]
//	  [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE [ <db_name> ] | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
//	  [ WITH PRIVILEGES <object_privilege> [ , <object_privilege> [ , ... ] ] ]
func (v *Validator) ParseShowSchemas() bool {
	return true
}

// ParseShowSearchIndexes validates the Snowflake `SHOW SEARCH INDEXES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-search-indexes
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSearchIndexes() bool {
	return true
}

// ParseShowSecrets validates the Snowflake `SHOW SECRETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-secrets
//
// Syntax:
//
//	SHOW SECRETS [ LIKE '<pattern>' ]
//	             [ IN { ACCOUNT | [ DATABASE ] <db_name> | [ SCHEMA ] <schema_name> | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
func (v *Validator) ParseShowSecrets() bool {
	return true
}

// ParseShowSecurityIntegrations validates the Snowflake `SHOW SECURITY INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-security-integrations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSecurityIntegrations() bool {
	return true
}

// ParseShowSemanticViews validates the Snowflake `SHOW SEMANTIC VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-views
//
// Syntax:
//
//	SHOW [ TERSE ] SEMANTIC VIEWS [ LIKE '<pattern>' ]
//	  [ IN
//	       {
//	         ACCOUNT                                         |
//
//	         DATABASE                                        |
//	         DATABASE <database_name>                        |
//
//	         SCHEMA                                          |
//	         SCHEMA <schema_name>                            |
//	         <schema_name>
//	       }
//	  ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowSemanticViews() bool {
	return true
}

// ParseShowSequences validates the Snowflake `SHOW SEQUENCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-sequences
//
// Syntax:
//
//	SHOW SEQUENCES [ LIKE '<pattern>' ]
//	               [ IN
//	                    {
//	                      ACCOUNT                                         |
//
//	                      DATABASE                                        |
//	                      DATABASE <database_name>                        |
//
//	                      SCHEMA                                          |
//	                      SCHEMA <schema_name>                            |
//	                      <schema_name>
//
//	                      APPLICATION <application_name>                  |
//	                      APPLICATION PACKAGE <application_package_name>  |
//	                    }
//	               ]
func (v *Validator) ParseShowSequences() bool {
	return true
}

// ParseShowServiceRoles validates the Snowflake `SHOW SERVICE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-roles
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowServiceRoles() bool {
	return true
}

// ParseShowServices validates the Snowflake `SHOW SERVICES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-services
//
// Syntax:
//
//	SHOW [ JOB ] SERVICES [ EXCLUDE JOBS ] [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT                  |
//
//	                  DATABASE                 |
//	                  DATABASE <database_name> |
//
//	                  SCHEMA                   |
//	                  SCHEMA <schema_name>     |
//	                  <schema_name>            |
//
//	                  COMPUTE POOL <compute_pool_name>
//	                }
//	           ]
//	           [ STARTS WITH '<name_string>' ]
//	           [ LIMIT <rows> [ FROM '<name_string>' ] ]
//	           [ OF TYPE <workload_type> [ , ... ] ]
func (v *Validator) ParseShowServices() bool {
	return true
}

// ParseShowSessionPolicies validates the Snowflake `SHOW SESSION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-session-policies
//
// Syntax:
//
//	SHOW SESSION POLICIES
//	  [ LIKE '<pattern>' ]
//	  [ IN
//	       {
//	         ACCOUNT                                         |
//
//	         DATABASE                                        |
//	         DATABASE <database_name>                        |
//
//	         SCHEMA                                          |
//	         SCHEMA <schema_name>                            |
//
//	         APPLICATION <application_name>                  |
//	         APPLICATION PACKAGE <application_package_name>  |
//	       }
//	    |
//	    ON
//	       {
//	         ACCOUNT           |
//	         USER <user_name>  |
//	       }
//	  ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowSessionPolicies() bool {
	return true
}

// ParseShowSessions validates the Snowflake `SHOW SESSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-sessions
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSessions() bool {
	return true
}

// ParseShowShares validates the Snowflake `SHOW SHARES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares
//
// Syntax:
//
//	SHOW SHARES [ LIKE '<pattern>' ]
//	            [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowShares() bool {
	return true
}

// ParseShowSnapshots validates the Snowflake `SHOW SNAPSHOTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshots
//
// Syntax:
//
//	SHOW SNAPSHOTS [ LIKE '<pattern>' ]
//	               [ IN
//	                   {
//	                       ACCOUNT                  |
//
//	                       DATABASE                 |
//	                       DATABASE <database_name> |
//
//	                       SCHEMA                   |
//	                       SCHEMA <schema_name>     |
//	                       <schema_name>            |
//	                   }
//	               ]
//	               [ STARTS WITH '<name_string>' ]
//	               [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowSnapshots() bool {
	return true
}

// ParseShowSnapshotPolicies validates the Snowflake `SHOW SNAPSHOT POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshot-policies
//
// Syntax:
//
//	SHOW SNAPSHOT POLICIES
//	   [ LIKE '<pattern>' ]
//	   [ IN { ACCOUNT | DATABASE | DATABASE <db_name> | SCHEMA | SCHEMA <schema_name> }
//	     [ STARTS WITH '<name_string>' ]
//	     [ LIMIT <rows> [ FROM '<name_string>' ]
//	   ]
func (v *Validator) ParseShowSnapshotPolicies() bool {
	return true
}

// ParseShowSnapshotSets validates the Snowflake `SHOW SNAPSHOT SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshot-sets
//
// Syntax:
//
//	SHOW SNAPSHOT SETS
//	   [ LIKE '<pattern>' ]
//	   [ IN { ACCOUNT | DATABASE | DATABASE <db_name> | SCHEMA | SCHEMA <schema_name> }
//	     [ STARTS WITH '<name_string>' ]
//	     [ LIMIT <rows> [ FROM '<name_string>' ]
//	   ]
func (v *Validator) ParseShowSnapshotSets() bool {
	return true
}

// ParseShowStages validates the Snowflake `SHOW STAGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-stages
//
// Syntax:
//
//	SHOW STAGES [ LIKE '<pattern>' ]
//	            [ IN
//	                 {
//	                   ACCOUNT                                         |
//
//	                   DATABASE                                        |
//	                   DATABASE <database_name>                        |
//
//	                   SCHEMA                                          |
//	                   SCHEMA <schema_name>                            |
//	                   <schema_name>
//
//	                   APPLICATION <application_name>                  |
//	                   APPLICATION PACKAGE <application_package_name>  |
//	                 }
//	            ]
func (v *Validator) ParseShowStages() bool {
	return true
}

// ParseShowStorageIntegrations validates the Snowflake `SHOW STORAGE INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-storage-integrations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowStorageIntegrations() bool {
	return true
}

// ParseShowStorageLifecyclePolicies validates the Snowflake `SHOW STORAGE LIFECYCLE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-storage-lifecycle-policies
//
// Syntax:
//
//	SHOW STORAGE LIFECYCLE POLICIES
//	  [ LIKE '<pattern>' ]
//	  [ IN
//	        {
//	          ACCOUNT                  |
//
//	          DATABASE                 |
//	          DATABASE <database_name> |
//
//	          SCHEMA                   |
//	          SCHEMA <schema_name>     |
//	          <schema_name>
//	        }
//	  ]
func (v *Validator) ParseShowStorageLifecyclePolicies() bool {
	return true
}

// ParseShowStreams validates the Snowflake `SHOW STREAMS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-streams
//
// Syntax:
//
//	SHOW [ TERSE ] STREAMS [ LIKE '<pattern>' ]
//	                       [ IN { ACCOUNT | DATABASE [ <db_name> ] | [ SCHEMA ] [ <schema_name> ] | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
//	                       [ STARTS WITH '<name_string>' ]
//	                       [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowStreams() bool {
	return true
}

// ParseShowStreamlits validates the Snowflake `SHOW STREAMLITS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-streamlits
//
// Syntax:
//
//	SHOW [ TERSE ] STREAMLITS [ LIKE '<pattern>' ]
//	                          [ IN
//	                                {
//	                                  ACCOUNT                   |
//
//	                                  DATABASE                  |
//	                                  DATABASE <db_name>        |
//
//	                                  SCHEMA
//	                                  SCHEMA <schema_name>      |
//	                                  <schema_name>             |
//	                                }
//	                          ]
//	                          [ LIMIT <rows> [ FROM '<name_string>' ]
func (v *Validator) ParseShowStreamlits() bool {
	return true
}

// ParseShowTableFunctions validates the Snowflake `SHOW TABLE FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-table-functions
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowTableFunctions() bool {
	return true
}

// ParseShowTables validates the Snowflake `SHOW TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tables
//
// Syntax:
//
//	SHOW [ TERSE ] TABLES [ HISTORY ] [ LIKE '<pattern>' ]
//	                                  [ IN
//	                                        {
//	                                          ACCOUNT                                         |
//
//	                                          DATABASE                                        |
//	                                          DATABASE <database_name>                        |
//
//	                                          SCHEMA                                          |
//	                                          SCHEMA <schema_name>                            |
//	                                          <schema_name>
//
//	                                          APPLICATION <application_name>                  |
//	                                          APPLICATION PACKAGE <application_package_name>  |
//	                                        }
//	                                  ]
//	                                  [ STARTS WITH '<name_string>' ]
//	                                  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowTables() bool {
	return true
}

// ParseShowTags validates the Snowflake `SHOW TAGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tags
//
// Syntax:
//
//	SHOW TAGS [ LIKE '<pattern>' ]
//	          [ IN
//	               {
//	                 ACCOUNT                                         |
//
//	                 DATABASE                                        |
//	                 DATABASE <database_name>                        |
//
//	                 SCHEMA                                          |
//	                 SCHEMA <schema_name>                            |
//	                 <schema_name>
//
//	                 APPLICATION <application_name>                  |
//	                 APPLICATION PACKAGE <application_package_name>  |
//	               }
//	          ]
func (v *Validator) ParseShowTags() bool {
	return true
}

// ParseShowTasks validates the Snowflake `SHOW TASKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tasks
//
// Syntax:
//
//	SHOW [ TERSE ] TASKS [ LIKE '<pattern>' ]
//	                     [ IN { ACCOUNT | DATABASE [ <db_name> ] | [ SCHEMA ] [ <schema_name> ] | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
//	                     [ STARTS WITH '<name_string>' ]
//	                     [ ROOT ONLY ]
//	                     [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowTasks() bool {
	return true
}

// ParseShowTransactions validates the Snowflake `SHOW TRANSACTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-transactions
//
// Syntax:
//
//	SHOW TRANSACTIONS [ IN ACCOUNT ]
func (v *Validator) ParseShowTransactions() bool {
	return true
}

// ParseShowTypes validates the Snowflake `SHOW TYPES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-types
//
// Syntax:
//
//	SHOW TYPES [ LIKE '<pattern>' ]
//	               [ IN
//	                    {
//	                      ACCOUNT                                         |
//
//	                      DATABASE                                        |
//	                      DATABASE <database_name>                        |
//
//	                      SCHEMA                                          |
//	                      SCHEMA <schema_name>                            |
//	                      <schema_name>
//
//	                      APPLICATION <application_name>                  |
//	                      APPLICATION PACKAGE <application_package_name>  |
//	                    }
//	               ]
//	           [ STARTS WITH '<name_string>' ]
func (v *Validator) ParseShowTypes() bool {
	return true
}

// ParseShowUniqueKeys validates the Snowflake `SHOW UNIQUE KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-unique-keys
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowUniqueKeys() bool {
	return true
}

// ParseShowUserFunctions validates the Snowflake `SHOW USER FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-functions
//
// Syntax:
//
//	SHOW USER FUNCTIONS [ LIKE '<pattern>' ]
//	  [ IN
//	    {
//	      ACCOUNT                                         |
//
//	      DATABASE                                        |
//	      DATABASE <database_name>                        |
//
//	      SCHEMA                                          |
//	      SCHEMA <schema_name>                            |
//	      <schema_name>
//
//	      APPLICATION <application_name>                  |
//	      APPLICATION PACKAGE <application_package_name>  |
//	    }
//	  ]
func (v *Validator) ParseShowUserFunctions() bool {
	return true
}

// ParseShowUsers validates the Snowflake `SHOW USERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-users
//
// Syntax:
//
//	SHOW [ TERSE ] USERS
//	  [ LIKE '<pattern>' ]
//	  [ STARTS WITH '<name_string>' ]
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowUsers() bool {
	return true
}

// ParseShowVariables validates the Snowflake `SHOW VARIABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-variables
//
// Syntax:
//
//	SHOW VARIABLES [ LIKE '<pattern>' ]
func (v *Validator) ParseShowVariables() bool {
	return true
}

// ParseShowViews validates the Snowflake `SHOW VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-views
//
// Syntax:
//
//	SHOW [ TERSE ] VIEWS [ LIKE '<pattern>' ]
//	                     [ IN { ACCOUNT | DATABASE [ <db_name> ] | [ SCHEMA ] [ <schema_name> ] | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
//	                     [ STARTS WITH '<name_string>' ]
//	                     [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowViews() bool {
	return true
}

// ParseShowWarehouses validates the Snowflake `SHOW WAREHOUSES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-warehouses
//
// Syntax:
//
//	SHOW WAREHOUSES
//	  [ LIKE '<pattern>' ]
//	  [ WITH PRIVILEGES <objectPrivilege> [ , <objectPrivilege> [ , ... ] ] ]
func (v *Validator) ParseShowWarehouses() bool {
	return true
}

// ParseShowApplicationServices validates the Snowflake `SHOW APPLICATION SERVICES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-services
//
// Syntax:
//
//	SHOW APPLICATION SERVICES
//	  [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] } ]
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowApplicationServices() bool {
	return true
}

// ParseShowArtifactRepositories validates the Snowflake `SHOW ARTIFACT REPOSITORIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-artifact-repositories
//
// Syntax:
//
//	SHOW ARTIFACT REPOSITORIES
//	  [ LIKE '<pattern>' ]
//	  [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] } ]
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowArtifactRepositories() bool {
	return true
}

// ParseShowCortexBaseModels validates the Snowflake `SHOW CORTEX BASE MODELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-cortex-base-models
//
// Syntax:
//
//	SHOW CORTEX BASE MODELS
//	  [ LIKE '<pattern>' ]
//	  IN [ SCHEMA ] SNOWFLAKE.MODELS
func (v *Validator) ParseShowCortexBaseModels() bool {
	return true
}

// ParseShowEventRoutingTableOnOrganization validates the Snowflake `SHOW EVENT ROUTING TABLE ON ORGANIZATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-routing-table-on-organization
//
// Syntax:
//
//	SHOW EVENT ROUTING TABLE ON ORGANIZATION FOR ALL APPLICATION LISTINGS
func (v *Validator) ParseShowEventRoutingTableOnOrganization() bool {
	return true
}

// ParseShowEventRoutingTables validates the Snowflake `SHOW EVENT ROUTING TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-routing-tables
//
// Syntax:
//
//	SHOW EVENT ROUTING TABLES
func (v *Validator) ParseShowEventRoutingTables() bool {
	return true
}

// ParseShowInteractiveTables validates the Snowflake `SHOW INTERACTIVE TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-interactive-tables
//
// Syntax:
//
//	SHOW INTERACTIVE TABLES [ LIKE '<pattern>' ]
//	                        [ IN
//	                          {
//	                               ACCOUNT              |
//
//	                               DATABASE             |
//	                               DATABASE <db_name>   |
//
//	                               SCHEMA               |
//	                               SCHEMA <schema_name> |
//	                               <schema_name>
//	                          }
//	                        ]
//	                        [ STARTS WITH '<name_string>' ]
//	                        [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowInteractiveTables() bool {
	return true
}

// ParseShowObjectsOwnedByApplication validates the Snowflake `SHOW OBJECTS OWNED BY APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-objects-owned-by-application
//
// Syntax:
//
//	SHOW OBJECTS OWNED BY APPLICATION <app_name>
func (v *Validator) ParseShowObjectsOwnedByApplication() bool {
	return true
}

// ParseShowOffers validates the Snowflake `SHOW OFFERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-offers
//
// Syntax:
//
//	SHOW OFFERS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowOffers() bool {
	return true
}

// ParseShowOpenflowDataPlaneIntegration validates the Snowflake `SHOW OPENFLOW DATA PLANE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-oflow-data-plane-integration
//
// Syntax:
//
//	SHOW OPENFLOW DATA PLANE INTEGRATIONS [ LIKE '<pattern>' ]
//	              [ STARTS WITH '<name_string>' ]
//	              [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowOpenflowDataPlaneIntegration() bool {
	return true
}

// ParseShowPricingPlans validates the Snowflake `SHOW PRICING PLANS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-pricing-plans
//
// Syntax:
//
//	SHOW PRICING PLANS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowPricingPlans() bool {
	return true
}

// ParseShowPrivacyPolicies validates the Snowflake `SHOW PRIVACY POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-privacy-policies
//
// Syntax:
//
//	SHOW PRIVACY POLICIES [ LIKE '<pattern>' ]
//	           [ IN
//	                {
//	                  ACCOUNT
//	                  | DATABASE [ <database_name> ]
//	                  | SCHEMA [ <schema_name> ]
//	                }
//	           ]
func (v *Validator) ParseShowPrivacyPolicies() bool {
	return true
}

// ParseShowReferences validates the Snowflake `SHOW REFERENCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-references
//
// Syntax:
//
//	SHOW REFERENCES IN APPLICATION <name>
func (v *Validator) ParseShowReferences() bool {
	return true
}

// ParseShowReleaseChannels validates the Snowflake `SHOW RELEASE CHANNELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-release-channels
//
// Syntax:
//
//	SHOW RELEASE CHANNELS IN APPLICATION PACKAGE <application_package_name>
//
//	SHOW RELEASE CHANNELS IN LISTING <listing_name>
func (v *Validator) ParseShowReleaseChannels() bool {
	return true
}

// ParseShowReleaseDirectives validates the Snowflake `SHOW RELEASE DIRECTIVES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-release-directives
//
// Syntax:
//
//	SHOW RELEASE DIRECTIVES [ LIKE '<pattern>' ]
//	  IN APPLICATION PACKAGE <name>
//	  [ FOR RELEASE CHANNEL <release_channel> ]
func (v *Validator) ParseShowReleaseDirectives() bool {
	return true
}

// ParseShowRolesInService validates the Snowflake `SHOW ROLES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-roles-in-service
//
// Syntax:
//
//	SHOW ROLES IN SERVICE <name>
func (v *Validator) ParseShowRolesInService() bool {
	return true
}

// ParseShowRulesInEventRoutingTable validates the Snowflake `SHOW RULES IN EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-rules-in-event-routing-table
//
// Syntax:
//
//	SHOW RULES IN EVENT ROUTING TABLE (<event_routing_table_name>)
func (v *Validator) ParseShowRulesInEventRoutingTable() bool {
	return true
}

// ParseShowRunInExperiment validates the Snowflake `SHOW RUN IN EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-run-in-experiment
//
// Syntax:
//
//	SHOW RUN METRICS [ LIKE '<pattern>' ]
//	  IN EXPERIMENT <experiment_name> [ RUN <run_name> ]
//	  [ LIMIT <rows> [ FROM <name_string> ] ]
//
//	SHOW RUN PARAMETERS [ LIKE '<pattern>' ]
//	  IN EXPERIMENT <experiment_name> [ RUN <run_name> ]
//	  [ LIMIT <rows> [ FROM <name_string> ] ]
func (v *Validator) ParseShowRunInExperiment() bool {
	return true
}

// ParseShowRunsInExperiment validates the Snowflake `SHOW RUNS IN EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-runs-in-experiment
//
// Syntax:
//
//	SHOW RUNS [ LIKE '<pattern>' ] IN EXPERIMENT <name>
func (v *Validator) ParseShowRunsInExperiment() bool {
	return true
}

// ParseShowSemanticDimensions validates the Snowflake `SHOW SEMANTIC DIMENSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-dimensions
//
// Syntax:
//
//	SHOW SEMANTIC DIMENSIONS [ LIKE '<pattern>' ]
//	                         [ IN
//	                              {
//	                                <semantic_view_name>           |
//
//	                                ACCOUNT                        |
//
//	                                DATABASE                       |
//	                                DATABASE <db_name>             |
//
//	                                SCHEMA                         |
//	                                SCHEMA <db_name>.<schema_name>
//	                              }
//	                         ]
//	                         [ STARTS WITH '<name_string>' ]
//	                         [ LIMIT <rows> ]
func (v *Validator) ParseShowSemanticDimensions() bool {
	return true
}

// ParseShowSemanticDimensionsForMetric validates the Snowflake `SHOW SEMANTIC DIMENSIONS FOR METRIC` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-dimensions-for-metric
//
// Syntax:
//
//	SHOW SEMANTIC DIMENSIONS [ LIKE '<pattern>' ]
//	                         IN <semantic_view_name>
//	                         FOR METRIC <metric_name>
//	                         [ STARTS WITH '<name_string>' ]
//	                         [ LIMIT <rows> ]
func (v *Validator) ParseShowSemanticDimensionsForMetric() bool {
	return true
}

// ParseShowSemanticFacts validates the Snowflake `SHOW SEMANTIC FACTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-facts
//
// Syntax:
//
//	SHOW SEMANTIC FACTS [ LIKE '<pattern>' ]
//	                    [ IN
//	                         {
//	                           <semantic_view_name>           |
//
//	                           ACCOUNT                        |
//
//	                           DATABASE                       |
//	                           DATABASE <db_name>             |
//
//	                           SCHEMA                         |
//	                           SCHEMA <db_name>.<schema_name>
//	                         }
//	                    ]
//	                    [ STARTS WITH '<name_string>' ]
//	                    [ LIMIT <rows> ]
func (v *Validator) ParseShowSemanticFacts() bool {
	return true
}

// ParseShowSemanticMetrics validates the Snowflake `SHOW SEMANTIC METRICS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-metrics
//
// Syntax:
//
//	SHOW SEMANTIC METRICS [ LIKE '<pattern>' ]
//	                      [ IN
//	                           {
//	                             <semantic_view_name>           |
//
//	                             ACCOUNT                        |
//
//	                             DATABASE                       |
//	                             DATABASE <db_name>             |
//
//	                             SCHEMA                         |
//	                             SCHEMA <db_name>.<schema_name>
//	                           }
//	                      ]
//	                      [ STARTS WITH '<name_string>' ]
//	                      [ LIMIT <rows> ]
func (v *Validator) ParseShowSemanticMetrics() bool {
	return true
}

// ParseShowServiceContainersInService validates the Snowflake `SHOW SERVICE CONTAINERS IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-containers-in-service
//
// Syntax:
//
//	SHOW SERVICE CONTAINERS IN SERVICE <name>
func (v *Validator) ParseShowServiceContainersInService() bool {
	return true
}

// ParseShowServiceInstancesInService validates the Snowflake `SHOW SERVICE INSTANCES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-instances-in-service
//
// Syntax:
//
//	SHOW SERVICE INSTANCES IN SERVICE <name>
func (v *Validator) ParseShowServiceInstancesInService() bool {
	return true
}

// ParseShowServiceVolumesInService validates the Snowflake `SHOW SERVICE VOLUMES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-volumes-in-service
//
// Syntax:
//
//	SHOW SERVICE VOLUMES IN SERVICE <name>
func (v *Validator) ParseShowServiceVolumesInService() bool {
	return true
}

// ParseShowSharedContent validates the Snowflake `SHOW SHARED CONTENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shared-content
//
// Syntax:
//
//	SHOW SHARED CONTENT IN APPLICATION PACKAGE <pkg_name> FOR VERSION <version_name>
func (v *Validator) ParseShowSharedContent() bool {
	return true
}

// ParseShowSharesInFailoverGroup validates the Snowflake `SHOW SHARES IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares-in-failover-group
//
// Syntax:
//
//	SHOW SHARES IN FAILOVER GROUP <name>
func (v *Validator) ParseShowSharesInFailoverGroup() bool {
	return true
}

// ParseShowSharesInReplicationGroup validates the Snowflake `SHOW SHARES IN REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares-in-replication-group
//
// Syntax:
//
//	SHOW SHARES IN REPLICATION GROUP <name>
func (v *Validator) ParseShowSharesInReplicationGroup() bool {
	return true
}

// ParseShowSnapshotsInSnapshotSet validates the Snowflake `SHOW SNAPSHOTS IN SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshots-in-snapshot-set
//
// Syntax:
//
//	SHOW SNAPSHOTS IN SNAPSHOT SET <name>
func (v *Validator) ParseShowSnapshotsInSnapshotSet() bool {
	return true
}

// ParseShowSpecifications validates the Snowflake `SHOW SPECIFICATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-specifications
//
// Syntax:
//
//	SHOW [ { APPROVED | DECLINED | PENDING } ] SPECIFICATIONS [ IN APPLICATION <app_name> ];
func (v *Validator) ParseShowSpecifications() bool {
	return true
}

// ParseShowTelemetryEventDefinitions validates the Snowflake `SHOW TELEMETRY EVENT DEFINITIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-telemetry-event-definitions
//
// Syntax:
//
//	SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION <name>
func (v *Validator) ParseShowTelemetryEventDefinitions() bool {
	return true
}

// ParseShowUserProcedures validates the Snowflake `SHOW USER PROCEDURES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-procedures
//
// Syntax:
//
//	SHOW USER PROCEDURES [ LIKE '<pattern>' ]
//	  [ IN
//	    {
//	      ACCOUNT                                         |
//
//	      DATABASE                                        |
//	      DATABASE <database_name>                        |
//
//	      SCHEMA                                          |
//	      SCHEMA <schema_name>                            |
//	      <schema_name>
//
//	      APPLICATION <application_name>                  |
//	      APPLICATION PACKAGE <application_package_name>  |
//	    }
//	  ]
func (v *Validator) ParseShowUserProcedures() bool {
	return true
}

// ParseShowUserProgrammaticAccessTokens validates the Snowflake `SHOW USER PROGRAMMATIC ACCESS TOKENS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-programmatic-access-tokens
//
// Syntax:
//
//	SHOW USER { PROGRAMMATIC ACCESS TOKENS | PATS } [ FOR USER <username> ]
func (v *Validator) ParseShowUserProgrammaticAccessTokens() bool {
	return true
}

// ParseShowUserWorkloadIdentityAuthenticationMethods validates the Snowflake `SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-workload-identity-authentication-methods
//
// Syntax:
//
//	SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS [ FOR USER <username> ]
func (v *Validator) ParseShowUserWorkloadIdentityAuthenticationMethods() bool {
	return true
}

// ParseShowVersions validates the Snowflake `SHOW VERSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE <pattern> ]
//	  IN APPLICATION PACKAGE <name>;
func (v *Validator) ParseShowVersions() bool {
	return true
}

// ParseShowVersionsInDataset validates the Snowflake `SHOW VERSIONS IN DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-dataset
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE '<pattern>' ] IN DATASET <dataset_name>
//	  [ LIMIT <rows>]
func (v *Validator) ParseShowVersionsInDataset() bool {
	return true
}

// ParseShowVersionsInDbtProject validates the Snowflake `SHOW VERSIONS IN DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-dbt-project
//
// Syntax:
//
//	SHOW VERSIONS IN DBT PROJECT <name>
//	  [ LIMIT <number> ]
func (v *Validator) ParseShowVersionsInDbtProject() bool {
	return true
}

// ParseShowVersionsInListing validates the Snowflake `SHOW VERSIONS IN LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-listing
//
// Syntax:
//
//	SHOW VERSIONS IN LISTING <name>
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowVersionsInListing() bool {
	return true
}

// ParseShowVersionsInModel validates the Snowflake `SHOW VERSIONS IN MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-model
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE '<pattern>' ] IN MODEL <model_name>
func (v *Validator) ParseShowVersionsInModel() bool {
	return true
}

// ParseShowVersionsInOrganizationProfile validates the Snowflake `SHOW VERSIONS IN ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-organization-profile
//
// Syntax:
//
//	SHOW VERSIONS IN ORGANIZATION PROFILE <name>
func (v *Validator) ParseShowVersionsInOrganizationProfile() bool {
	return true
}

// ParseShowWorkspaces validates the Snowflake `SHOW WORKSPACES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-workspaces
//
// Syntax:
//
//	SHOW WORKSPACES [ LIKE '<pattern>' ]
//	                [ IN
//	                     {
//	                       ACCOUNT                  |
//
//	                       DATABASE                 |
//	                       DATABASE <database_name> |
//
//	                       SCHEMA                   |
//	                       SCHEMA <schema_name>     |
//	                       <schema_name>
//	                     }
func (v *Validator) ParseShowWorkspaces() bool {
	return true
}
