package sqlgrammar

import "thaw/internal/sqltok"

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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("OBJECTS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowAccounts validates the Snowflake `SHOW ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-accounts
//
// Syntax:
//
//	SHOW ACCOUNTS [ HISTORY ] [ LIKE '<pattern>' ]
func (v *Validator) ParseShowAccounts() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ACCOUNTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("AGENTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("AGGREGATION", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("ALERTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("APPLICATION", "PACKAGES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowApplicationRoles validates the Snowflake `SHOW APPLICATION ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-roles
//
// Syntax:
//
//	SHOW APPLICATION ROLES [ LIKE <pattern> ] IN APPLICATION <name>
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowApplicationRoles() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("APPLICATION", "ROLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("APPLICATIONS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("AUTHENTICATION", "POLICIES") },
		func() bool { return v.Optional(v.likeClause) },
		// [ IN <scope> | ON { ACCOUNT | USER <name> } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					v.inScopeClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("ON") },
							func() bool {
								return v.Choice(
									func() bool { return v.MatchWord("ACCOUNT") },
									func() bool {
										return v.Sequence(
											func() bool { return v.MatchWord("USER") },
											v.parseIdentPath,
										)
									},
								)
							},
						)
					},
				)
			})
		},
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.phrase("AVAILABLE", "LISTINGS") },
		// [ LIMIT <rows> ] and the IS_* = TRUE options, in any order.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.limitFromClause,
					v.option("IS_IMPORTED", v.parseBool),
					v.option("IS_ORGANIZATION", v.parseBool),
					v.option("IS_SHARED_WITH_ME", v.parseBool),
				)
			})
		},
	)
}

// ParseShowAvailableOffers validates the Snowflake `SHOW AVAILABLE OFFERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-offers
//
// Syntax:
//
//	SHOW AVAILABLE OFFERS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowAvailableOffers() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("AVAILABLE", "OFFERS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("LISTING") },
		v.parseIdentPath,
	)
}

// ParseShowAvailableOrganizationProfiles validates the Snowflake `SHOW AVAILABLE ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-organization-profiles
//
// Syntax:
//
//	SHOW AVAILABLE ORGANIZATION PROFILES
func (v *Validator) ParseShowAvailableOrganizationProfiles() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("AVAILABLE", "ORGANIZATION", "PROFILES") },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("BACKUP", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("BACKUP", "SETS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowBackupsInBackupSet validates the Snowflake `SHOW BACKUPS IN BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backups-in-backup-set
//
// Syntax:
//
//	SHOW BACKUPS IN BACKUP SET <name>
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowBackupsInBackupSet() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("BACKUPS") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("BACKUP") },
		func() bool { return v.MatchWord("SET") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.limitFromClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CALLER") },
		func() bool { return v.MatchWord("GRANTS") },
		func() bool {
			return v.Choice(
				// ON ACCOUNT
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("ON") },
						func() bool { return v.MatchWord("ACCOUNT") },
					)
				},
				// ON <object_type> <object_name>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("ON") },
						func() bool { return v.Match(sqltok.Identifier) || v.Match(sqltok.Keyword) },
						v.parseIdentPath,
					)
				},
				// TO { ROLE | DATABASE ROLE } <owner_name>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("TO") },
						func() bool { return v.Optional(func() bool { return v.MatchWord("DATABASE") }) },
						func() bool { return v.MatchWord("ROLE") },
						v.parseIdentPath,
					)
				},
			)
		},
	)
}

// ParseShowCatalogIntegrations validates the Snowflake `SHOW CATALOG INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-catalog-integrations
//
// Syntax:
//
//	SHOW CATALOG INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowCatalogIntegrations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("CATALOG", "INTEGRATIONS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CHANNELS") },
		func() bool { return v.Optional(v.likeClause) },
		// [ IN { ACCOUNT | DATABASE [name] | SCHEMA [name] | <name>
		//        | TABLE [name] | PIPE [name] } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("IN") },
					func() bool {
						return v.Choice(
							func() bool { return v.MatchWord("ACCOUNT") },
							func() bool {
								return v.Sequence(
									v.wordsValue("DATABASE", "SCHEMA", "TABLE", "PIPE"),
									func() bool { return v.Optional(v.parseIdentPath) },
								)
							},
							v.parseIdentPath,
						)
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CLASSES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowColumns validates the Snowflake `SHOW COLUMNS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-columns
//
// Syntax:
//
//	SHOW COLUMNS [ LIKE '<pattern>' ]
//	             [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] | TABLE | [ TABLE ] <table_name> | VIEW | [ VIEW ] <view_name> } | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> ]
func (v *Validator) ParseShowColumns() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("COLUMNS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowComputePoolInstanceFamilies validates the Snowflake `SHOW COMPUTE POOL INSTANCE FAMILIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-compute-pool-instance-families
//
// Syntax:
//
//	SHOW COMPUTE POOL INSTANCE FAMILIES
func (v *Validator) ParseShowComputePoolInstanceFamilies() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("COMPUTE", "POOL", "INSTANCE", "FAMILIES") },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("COMPUTE", "POOLS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowConfigurations validates the Snowflake `SHOW CONFIGURATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-configurations
//
// Syntax:
//
//	SHOW CONFIGURATIONS [ IN APPLICATION <app> ]
func (v *Validator) ParseShowConfigurations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CONFIGURATIONS") },
		func() bool { return v.Optional(v.inScopeClause) },
	)
}

// ParseShowConnections validates the Snowflake `SHOW CONNECTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-connections
//
// Syntax:
//
//	SHOW CONNECTIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowConnections() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CONNECTIONS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("CONTACTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("CORTEX", "SEARCH", "SERVICES") },
		// [ LIKE [ PATTERN ] '<pattern>' ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIKE") },
					func() bool { return v.Optional(func() bool { return v.MatchWord("PATTERN") }) },
					v.parseString,
				)
			})
		},
		func() bool { return v.Optional(v.startsWithClause) },
		func() bool { return v.Optional(v.limitFromClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("DATA", "METRIC", "FUNCTIONS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowDatabaseRoles validates the Snowflake `SHOW DATABASE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-database-roles
//
// Syntax:
//
//	SHOW DATABASE ROLES IN DATABASE <name>
//	  [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowDatabaseRoles() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DATABASE") },
		func() bool { return v.MatchWord("ROLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("DATABASES") },
		func() bool { return v.showTrailers() },
		// [ WITH PRIVILEGES <object_privilege> [ , ... ] ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("PRIVILEGES") },
					func() bool { return v.Match(sqltok.Identifier) || v.Match(sqltok.Keyword) },
					func() bool {
						return v.ZeroOrMore(func() bool {
							return v.Sequence(
								func() bool { return v.Match(sqltok.Comma) },
								func() bool { return v.Match(sqltok.Identifier) || v.Match(sqltok.Keyword) },
							)
						})
					},
				)
			})
		},
	)
}

// ParseShowDatabasesInFailoverGroup validates the Snowflake `SHOW DATABASES IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-failover-group
//
// Syntax:
//
//	SHOW DATABASES IN FAILOVER GROUP <name>
func (v *Validator) ParseShowDatabasesInFailoverGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DATABASES") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("FAILOVER") },
		func() bool { return v.MatchWord("GROUP") },
		v.parseIdentPath,
	)
}

// ParseShowDatabasesInReplicationGroup validates the Snowflake `SHOW DATABASES IN REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-replication-group
//
// Syntax:
//
//	SHOW DATABASES IN REPLICATION GROUP <name>
func (v *Validator) ParseShowDatabasesInReplicationGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DATABASES") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("GROUP") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DATASETS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("DBT", "PROJECTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.phrase("DCM", "PROJECTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DELEGATED") },
		func() bool { return v.MatchWord("AUTHORIZATIONS") },
		// [ BY USER <username> | TO SECURITY INTEGRATION <integration_name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("BY") },
							func() bool { return v.MatchWord("USER") },
							v.parseIdentPath,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("TO") },
							func() bool { return v.MatchWord("SECURITY") },
							func() bool { return v.MatchWord("INTEGRATION") },
							v.parseIdentPath,
						)
					},
				)
			})
		},
	)
}

// ParseShowDeploymentsInDcmProject validates the Snowflake `SHOW DEPLOYMENTS IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-deployments-in-dcm-project
//
// Syntax:
//
//	SHOW DEPLOYMENTS IN DCM PROJECT <name> [ LIMIT <rows> ]
func (v *Validator) ParseShowDeploymentsInDcmProject() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("DEPLOYMENTS") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("DCM") },
		func() bool { return v.MatchWord("PROJECT") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.limitFromClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("DYNAMIC", "TABLES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowEndpoints validates the Snowflake `SHOW ENDPOINTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-endpoints
//
// Syntax:
//
//	SHOW ENDPOINTS IN SERVICE <name>
func (v *Validator) ParseShowEndpoints() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ENDPOINTS") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("SERVICE") },
		v.parseIdentPath,
	)
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
	// LIKE <pattern> here may use a bare identifier/scalar, not just a string.
	likeAny := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("LIKE") },
			v.parseScalar,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ENTITIES") },
		func() bool { return v.Optional(likeAny) },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("DCM") },
		func() bool { return v.MatchWord("PROJECT") },
		v.parseIdentPath,
		// [ STARTS WITH <prefix> ] and [ LIMIT <n> [ FROM <cursor> ] ], any order.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.phrase("STARTS", "WITH") },
							v.parseScalar,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("LIMIT") },
							func() bool { return v.Match(sqltok.NumberLit) },
							func() bool {
								return v.Optional(func() bool {
									return v.Sequence(
										func() bool { return v.MatchWord("FROM") },
										v.parseScalar,
									)
								})
							},
						)
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.phrase("EVENT", "TABLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("EXPERIMENTS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowExternalAgents validates the Snowflake `SHOW EXTERNAL AGENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-agents
//
// Syntax:
//
//	SHOW EXTERNAL AGENTS [ LIKE '<pattern>' ]
//	                     [ IN { ACCOUNT | DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
func (v *Validator) ParseShowExternalAgents() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("EXTERNAL", "AGENTS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowExternalFunctions validates the Snowflake `SHOW EXTERNAL FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-functions
//
// Syntax:
//
//	SHOW EXTERNAL FUNCTIONS [ LIKE '<pattern>' ]
//	           [ IN { APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> }  ]
func (v *Validator) ParseShowExternalFunctions() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("EXTERNAL", "FUNCTIONS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.phrase("EXTERNAL", "TABLES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowExternalVolumes validates the Snowflake `SHOW EXTERNAL VOLUMES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-volumes
//
// Syntax:
//
//	SHOW EXTERNAL VOLUMES [ LIKE '<pattern>' ]
func (v *Validator) ParseShowExternalVolumes() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("EXTERNAL", "VOLUMES") },
		func() bool { return v.Optional(v.likeClause) },
	)
}

// ParseShowFailoverGroups validates the Snowflake `SHOW FAILOVER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-failover-groups
//
// Syntax:
//
//	SHOW FAILOVER GROUPS [ IN ACCOUNT <account> ]
func (v *Validator) ParseShowFailoverGroups() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("FAILOVER", "GROUPS") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.phrase("IN", "ACCOUNT") },
					func() bool { return v.Optional(v.parseIdentPath) },
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("FEATURE", "POLICIES") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("IN", "ON"),
					func() bool {
						return v.Choice(
							func() bool { return v.MatchWord("ACCOUNT") },
							func() bool {
								return v.Sequence(
									v.wordsValue("APPLICATION", "DATABASE", "SCHEMA"),
									func() bool { return v.Optional(func() bool { return v.MatchWord("PACKAGE") }) },
									func() bool { return v.Optional(v.parseIdentPath) },
								)
							},
						)
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("FILE", "FORMATS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("FUNCTIONS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowFunctionsInModel validates the Snowflake `SHOW FUNCTIONS IN MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-functions-in-model
//
// Syntax:
//
//	SHOW FUNCTIONS [ LIKE '<pattern>' ] IN MODEL <model_name>
//	               [ VERSION <version_name> ]
func (v *Validator) ParseShowFunctionsInModel() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("FUNCTIONS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "MODEL") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("VERSION") }, v.parseIdentPath)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("GATEWAYS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowGitBranches validates the Snowflake `SHOW GIT BRANCHES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-branches
//
// Syntax:
//
//	SHOW GIT BRANCHES [ LIKE '<pattern>' ] IN [ GIT REPOSITORY ] <repository_name>
func (v *Validator) ParseShowGitBranches() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("GIT", "BRANCHES") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.Optional(func() bool { return v.phrase("GIT", "REPOSITORY") }) },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("GIT", "REPOSITORIES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowGitTags validates the Snowflake `SHOW GIT TAGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-tags
//
// Syntax:
//
//	SHOW GIT TAGS [ LIKE '<pattern>' ] IN [ GIT REPOSITORY ] <repository_name>
func (v *Validator) ParseShowGitTags() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("GIT", "TAGS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.Optional(func() bool { return v.phrase("GIT", "REPOSITORY") }) },
		v.parseIdentPath,
	)
}

// ParseShowGlobalAccounts validates the Snowflake `SHOW GLOBAL ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-global-accounts
//
// Syntax:
//
//	SHOW GLOBAL ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowGlobalAccounts() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("GLOBAL", "ACCOUNTS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("FUTURE") }) },
		func() bool { return v.MatchWord("GRANTS") },
		func() bool {
			// Optional ON / TO / OF / IN <discriminator> ... — too many object and
			// role shapes (plus an optional trailing LIMIT) to model exactly; require
			// the discriminator keyword then accept the free-form remainder.
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("ON", "TO", "OF", "IN"),
					v.consumeRest,
				)
			})
		},
		func() bool {
			// Bare `SHOW GRANTS [ LIMIT <rows> ]`.
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIMIT") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("FUTURE") }) },
		func() bool { return v.MatchWord("GRANTS") },
		func() bool { return v.phrase("IN", "DCM", "PROJECT") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIMIT") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("HYBRID") }) },
		func() bool { return v.MatchWord("TABLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("ICEBERG") }) },
		func() bool { return v.MatchWord("TABLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("IMAGE", "REPOSITORIES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowImagesInImageRepository validates the Snowflake `SHOW IMAGES IN IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-images-in-image-repository
//
// Syntax:
//
//	SHOW IMAGES IN IMAGE REPOSITORY <name>
func (v *Validator) ParseShowImagesInImageRepository() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("IMAGES") },
		func() bool { return v.phrase("IN", "IMAGE", "REPOSITORY") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("INDEXES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowIntegrations validates the Snowflake `SHOW INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-integrations
//
// Syntax:
//
//	SHOW [ { API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY | STORAGE } ] INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowIntegrations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.phrase("EXTERNAL", "ACCESS") },
					v.wordsValue("API", "CATALOG", "NOTIFICATION", "SECURITY", "STORAGE"),
				)
			})
		},
		func() bool { return v.MatchWord("INTEGRATIONS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("JOIN", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("LISTINGS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowListingsInFailoverGroup validates the Snowflake `SHOW LISTINGS IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-listings-in-failover-group
//
// Syntax:
//
//	SHOW LISTINGS IN FAILOVER GROUP <name>
func (v *Validator) ParseShowListingsInFailoverGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("LISTINGS") },
		func() bool { return v.phrase("IN", "FAILOVER", "GROUP") },
		v.parseIdentPath,
	)
}

// ParseShowLocks validates the Snowflake `SHOW LOCKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-locks
//
// Syntax:
//
//	SHOW LOCKS [ IN ACCOUNT ]
func (v *Validator) ParseShowLocks() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("LOCKS") },
		func() bool { return v.Optional(func() bool { return v.phrase("IN", "ACCOUNT") }) },
	)
}

// ParseShowMaintenancePolicies validates the Snowflake `SHOW MAINTENANCE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-maintenance-policies
//
// Syntax:
//
//	SHOW MAINTENANCE POLICIES { ON | IN } { ACCOUNT | APPLICATION <app_name> | <entity_type> <entity_name> }
func (v *Validator) ParseShowMaintenancePolicies() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MAINTENANCE", "POLICIES") },
		v.wordsValue("ON", "IN"),
		func() bool {
			return v.Choice(
				// ACCOUNT takes no name.
				func() bool { return v.MatchWord("ACCOUNT") },
				// APPLICATION <app_name> or <entity_type> <entity_name>: one
				// entity-type word followed by a name.
				func() bool {
					return v.Sequence(
						func() bool {
							if !v.Peek().Kind.IsIdentLike() {
								return false
							}
							v.advance()
							return true
						},
						v.parseIdentPath,
					)
				},
			)
		},
	)
}

// ParseShowManagedAccounts validates the Snowflake `SHOW MANAGED ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-managed-accounts
//
// Syntax:
//
//	SHOW MANAGED ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowManagedAccounts() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MANAGED", "ACCOUNTS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MASKING", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MATERIALIZED", "VIEWS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MCP", "SERVERS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowMfaMethods validates the Snowflake `SHOW MFA METHODS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-mfa-methods
//
// Syntax:
//
//	SHOW MFA METHODS [ FOR USER <user> ]
func (v *Validator) ParseShowMfaMethods() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MFA", "METHODS") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.phrase("FOR", "USER") }, v.parseIdentPath)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("MODEL", "MONITORS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowModels validates the Snowflake `SHOW MODELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-models
//
// Syntax:
//
//	SHOW MODELS [ LIKE '<pattern>' ]
//	            [ IN { DATABASE [ <db_name> ] | SCHEMA [ <schema_name> ] } ]
func (v *Validator) ParseShowModels() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("MODELS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowNetworkPolicies validates the Snowflake `SHOW NETWORK POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-network-policies
//
// Syntax:
//
//	SHOW NETWORK POLICIES
func (v *Validator) ParseShowNetworkPolicies() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("NETWORK", "POLICIES") },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("NETWORK", "RULES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("NOTEBOOK", "PROJECTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("NOTEBOOKS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowNotificationIntegrations validates the Snowflake `SHOW NOTIFICATION INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notification-integrations
//
// Syntax:
//
//	SHOW NOTIFICATION INTEGRATIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowNotificationIntegrations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("NOTIFICATION", "INTEGRATIONS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("OBJECTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ONLINE", "FEATURE", "TABLES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowOpenListingProviders validates the Snowflake `SHOW OPEN LISTING PROVIDERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-open-listing-providers
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowOpenListingProviders() bool {
	// Syntax unavailable in docs: require the command skeleton, accept any tail.
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("OPEN", "LISTING", "PROVIDERS") },
		v.consumeRest,
	)
}

// ParseShowOrganizationAccounts validates the Snowflake `SHOW ORGANIZATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-accounts
//
// Syntax:
//
//	SHOW ORGANIZATION ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowOrganizationAccounts() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ORGANIZATION", "ACCOUNTS") },
		func() bool { return v.Optional(v.likeClause) },
	)
}

// ParseShowOrganizationProfiles validates the Snowflake `SHOW ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-profiles
//
// Syntax:
//
//	SHOW ORGANIZATION PROFILES
func (v *Validator) ParseShowOrganizationProfiles() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ORGANIZATION", "PROFILES") },
	)
}

// ParseShowOrganizationUsers validates the Snowflake `SHOW ORGANIZATION USERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-users
//
// Syntax:
//
//	SHOW ORGANIZATION USERS [ IN ORGANIZATION USER GROUP <org_user_group> ]
func (v *Validator) ParseShowOrganizationUsers() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ORGANIZATION", "USERS") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.phrase("IN", "ORGANIZATION", "USER", "GROUP") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseShowOrganizationUserGroups validates the Snowflake `SHOW ORGANIZATION USER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-user-groups
//
// Syntax:
//
//	SHOW ORGANIZATION USER GROUPS
func (v *Validator) ParseShowOrganizationUserGroups() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ORGANIZATION", "USER", "GROUPS") },
	)
}

// ParseShowOrganizations validates the Snowflake `SHOW ORGANIZATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organizations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowOrganizations() bool {
	// Syntax unavailable in docs: require the command skeleton, accept any tail.
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ORGANIZATIONS") },
		v.consumeRest,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("PACKAGES", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("PASSWORD", "POLICIES") },
		func() bool { return v.Optional(v.likeClause) },
		// IN <scope> (handled by inScopeClause) or ON { ACCOUNT | USER <name> }.
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					v.inScopeClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("ON") },
							func() bool {
								return v.Choice(
									func() bool { return v.MatchWord("ACCOUNT") },
									func() bool {
										return v.Sequence(func() bool { return v.MatchWord("USER") }, v.parseIdentPath)
									},
								)
							},
						)
					},
				)
			})
		},
		func() bool { return v.Optional(v.startsWithClause) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIMIT") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PARAMETERS") },
		func() bool { return v.Optional(v.likeClause) },
		// [ { IN | FOR } { SESSION | ACCOUNT | <object-words> [ <name> ] } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("IN", "FOR"),
					func() bool {
						return v.Choice(
							func() bool { return v.MatchWord("SESSION") },
							func() bool { return v.MatchWord("ACCOUNT") },
							func() bool {
								return v.Sequence(
									v.wordsValue("USER", "WAREHOUSE", "DATABASE", "SCHEMA", "TASK", "TABLE"),
									func() bool { return v.Optional(v.parseIdentPath) },
								)
							},
						)
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PIPES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("POSTGRES") },
		func() bool { return v.MatchWord("INSTANCES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowPrimaryKeys validates the Snowflake `SHOW PRIMARY KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-primary-keys
//
// Syntax:
//
//	SHOW [ TERSE ] PRIMARY KEYS
//	    [ IN { ACCOUNT | DATABASE [ <database_name> ] | SCHEMA [ <schema_name> ] | TABLE | [ TABLE ] <table_name> } ]
func (v *Validator) ParseShowPrimaryKeys() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("PRIMARY") },
		func() bool { return v.MatchWord("KEYS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowPrivileges validates the Snowflake `SHOW PRIVILEGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-privileges
//
// Syntax:
//
//	SHOW PRIVILEGES IN APPLICATION <name>
func (v *Validator) ParseShowPrivileges() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PRIVILEGES") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PROCEDURES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowProvisionedThroughput validates the Snowflake `SHOW PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-provisioned-throughput
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowProvisionedThroughput() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PROVISIONED") },
		func() bool { return v.MatchWord("THROUGHPUT") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("PROJECTION") },
		func() bool { return v.MatchWord("POLICIES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowQueries validates the Snowflake `SHOW QUERIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-queries
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowQueries() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("QUERIES") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
}

// ParseShowRegions validates the Snowflake `SHOW REGIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-regions
//
// Syntax:
//
//	SHOW REGIONS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowRegions() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REGIONS") },
		func() bool { return v.Optional(v.likeClause) },
	)
}

// ParseShowReplicatedDatabases validates the Snowflake `SHOW REPLICATED DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replicated-databases
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowReplicatedDatabases() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REPLICATED") },
		func() bool { return v.MatchWord("DATABASES") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
}

// ParseShowReplicationAccounts validates the Snowflake `SHOW REPLICATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-accounts
//
// Syntax:
//
//	SHOW REPLICATION ACCOUNTS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowReplicationAccounts() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("ACCOUNTS") },
		func() bool { return v.Optional(v.likeClause) },
	)
}

// ParseShowReplicationDatabases validates the Snowflake `SHOW REPLICATION DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-databases
//
// Syntax:
//
//	SHOW REPLICATION DATABASES [ LIKE '<pattern>' ]
//	                           [ WITH PRIMARY <account_identifier>.<primary_db_name> ]
func (v *Validator) ParseShowReplicationDatabases() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("DATABASES") },
		func() bool { return v.Optional(v.likeClause) },
		// [ WITH PRIMARY <account_identifier>.<primary_db_name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("PRIMARY") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseShowReplicationGroups validates the Snowflake `SHOW REPLICATION GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-groups
//
// Syntax:
//
//	SHOW REPLICATION GROUPS [ IN ACCOUNT <account> ]
func (v *Validator) ParseShowReplicationGroups() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("GROUPS") },
		// [ IN ACCOUNT <account> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("IN") },
					func() bool { return v.MatchWord("ACCOUNT") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseShowResourceMonitors validates the Snowflake `SHOW RESOURCE MONITORS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-resource-monitors
//
// Syntax:
//
//	SHOW RESOURCE MONITORS [ LIKE '<pattern>' ]
func (v *Validator) ParseShowResourceMonitors() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("RESOURCE") },
		func() bool { return v.MatchWord("MONITORS") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("ROLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ROW") },
		func() bool { return v.MatchWord("ACCESS") },
		func() bool { return v.MatchWord("POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("SCHEMAS") },
		func() bool { return v.showTrailers() },
		// [ WITH PRIVILEGES <object_privilege> [ , ... ] ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("PRIVILEGES") },
					v.parseIdentPath,
					func() bool {
						return v.ZeroOrMore(func() bool {
							return v.Sequence(
								func() bool { return v.Match(sqltok.Comma) },
								v.parseIdentPath,
							)
						})
					},
				)
			})
		},
	)
}

// ParseShowSearchIndexes validates the Snowflake `SHOW SEARCH INDEXES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-search-indexes
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSearchIndexes() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SEARCH") },
		func() bool { return v.MatchWord("INDEXES") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
}

// ParseShowSecrets validates the Snowflake `SHOW SECRETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-secrets
//
// Syntax:
//
//	SHOW SECRETS [ LIKE '<pattern>' ]
//	             [ IN { ACCOUNT | [ DATABASE ] <db_name> | [ SCHEMA ] <schema_name> | APPLICATION <application_name> | APPLICATION PACKAGE <application_package_name> } ]
func (v *Validator) ParseShowSecrets() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SECRETS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowSecurityIntegrations validates the Snowflake `SHOW SECURITY INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-security-integrations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSecurityIntegrations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATIONS") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("SEMANTIC") },
		func() bool { return v.MatchWord("VIEWS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SEQUENCES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowServiceRoles validates the Snowflake `SHOW SERVICE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-roles
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowServiceRoles() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SERVICE") },
		func() bool { return v.MatchWord("ROLES") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("JOB") }) },
		func() bool { return v.MatchWord("SERVICES") },
		// [ EXCLUDE JOBS ]
		func() bool {
			return v.Optional(func() bool { return v.phrase("EXCLUDE", "JOBS") })
		},
		func() bool { return v.Optional(v.likeClause) },
		// [ IN { <scope> | COMPUTE POOL <name> } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("IN") },
							func() bool { return v.phrase("COMPUTE", "POOL") },
							v.parseIdentPath,
						)
					},
					v.inScopeClause,
				)
			})
		},
		func() bool { return v.Optional(v.startsWithClause) },
		func() bool { return v.Optional(v.limitFromClause) },
		// [ OF TYPE <workload_type> [ , ... ] ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("OF") },
					func() bool { return v.MatchWord("TYPE") },
					v.parseScalar,
					func() bool {
						return v.ZeroOrMore(func() bool {
							return v.Sequence(
								func() bool { return v.Match(sqltok.Comma) },
								v.parseScalar,
							)
						})
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SESSION") },
		func() bool { return v.MatchWord("POLICIES") },
		func() bool { return v.Optional(v.likeClause) },
		// [ IN <scope> | ON { ACCOUNT | USER <name> } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					v.inScopeClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("ON") },
							func() bool {
								return v.Choice(
									func() bool { return v.MatchWord("ACCOUNT") },
									func() bool {
										return v.Sequence(
											func() bool { return v.MatchWord("USER") },
											v.parseIdentPath,
										)
									},
								)
							},
						)
					},
				)
			})
		},
		func() bool { return v.Optional(v.startsWithClause) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIMIT") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
	)
}

// ParseShowSessions validates the Snowflake `SHOW SESSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-sessions
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowSessions() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SESSIONS") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
}

// ParseShowShares validates the Snowflake `SHOW SHARES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares
//
// Syntax:
//
//	SHOW SHARES [ LIKE '<pattern>' ]
//	            [ LIMIT <rows> [ FROM '<name_string>' ] ]
func (v *Validator) ParseShowShares() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SHARES") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.Optional(v.limitFromClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SNAPSHOTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("SETS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("STAGES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowStorageIntegrations validates the Snowflake `SHOW STORAGE INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-storage-integrations
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowStorageIntegrations() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("INTEGRATIONS") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("LIFECYCLE") },
		func() bool { return v.MatchWord("POLICIES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("STREAMS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("STREAMLITS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowTableFunctions validates the Snowflake `SHOW TABLE FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-table-functions
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowTableFunctions() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.MatchWord("FUNCTIONS") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("TABLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("TAGS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("TASKS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.Optional(v.inScopeClause) },
		func() bool { return v.Optional(v.startsWithClause) },
		// [ ROOT ONLY ]
		func() bool {
			return v.Optional(func() bool { return v.phrase("ROOT", "ONLY") })
		},
		func() bool { return v.Optional(v.limitFromClause) },
	)
}

// ParseShowTransactions validates the Snowflake `SHOW TRANSACTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-transactions
//
// Syntax:
//
//	SHOW TRANSACTIONS [ IN ACCOUNT ]
func (v *Validator) ParseShowTransactions() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("TRANSACTIONS") },
		// [ IN ACCOUNT ]
		func() bool {
			return v.Optional(func() bool { return v.phrase("IN", "ACCOUNT") })
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("TYPES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowUniqueKeys validates the Snowflake `SHOW UNIQUE KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-unique-keys
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseShowUniqueKeys() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("UNIQUE") },
		func() bool { return v.MatchWord("KEYS") },
		func() bool { return v.showTrailers() },
		func() bool { return v.consumeRest() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.MatchWord("FUNCTIONS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("USERS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowVariables validates the Snowflake `SHOW VARIABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-variables
//
// Syntax:
//
//	SHOW VARIABLES [ LIKE '<pattern>' ]
func (v *Validator) ParseShowVariables() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VARIABLES") },
		func() bool { return v.Optional(v.likeClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TERSE") }) },
		func() bool { return v.MatchWord("VIEWS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("WAREHOUSES") },
		func() bool { return v.Optional(v.likeClause) },
		// [ WITH PRIVILEGES <objectPrivilege> [ , ... ] ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("PRIVILEGES") },
					v.parseIdentPath,
					func() bool {
						return v.ZeroOrMore(func() bool {
							return v.Sequence(
								func() bool { return v.Match(sqltok.Comma) },
								v.parseIdentPath,
							)
						})
					},
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.MatchWord("SERVICES") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.Optional(v.inScopeClause) },
		// [ LIMIT <rows> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LIMIT") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("ARTIFACT", "REPOSITORIES") },
		func() bool { return v.showTrailers() },
	)
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
	// SHOW CORTEX BASE MODELS [ LIKE '<pattern>' ] IN [ SCHEMA ] SNOWFLAKE.MODELS
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("CORTEX", "BASE", "MODELS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SCHEMA") }) },
		v.parseIdentPath,
	)
}

// ParseShowEventRoutingTableOnOrganization validates the Snowflake `SHOW EVENT ROUTING TABLE ON ORGANIZATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-routing-table-on-organization
//
// Syntax:
//
//	SHOW EVENT ROUTING TABLE ON ORGANIZATION FOR ALL APPLICATION LISTINGS
func (v *Validator) ParseShowEventRoutingTableOnOrganization() bool {
	// SHOW EVENT ROUTING TABLE ON ORGANIZATION FOR ALL APPLICATION LISTINGS
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("EVENT", "ROUTING", "TABLE") },
		func() bool { return v.phrase("ON", "ORGANIZATION") },
		func() bool { return v.phrase("FOR", "ALL", "APPLICATION", "LISTINGS") },
	)
}

// ParseShowEventRoutingTables validates the Snowflake `SHOW EVENT ROUTING TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-routing-tables
//
// Syntax:
//
//	SHOW EVENT ROUTING TABLES
func (v *Validator) ParseShowEventRoutingTables() bool {
	// SHOW EVENT ROUTING TABLES
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("EVENT", "ROUTING", "TABLES") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("INTERACTIVE", "TABLES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowObjectsOwnedByApplication validates the Snowflake `SHOW OBJECTS OWNED BY APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-objects-owned-by-application
//
// Syntax:
//
//	SHOW OBJECTS OWNED BY APPLICATION <app_name>
func (v *Validator) ParseShowObjectsOwnedByApplication() bool {
	// SHOW OBJECTS OWNED BY APPLICATION <app_name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("OBJECTS") },
		func() bool { return v.phrase("OWNED", "BY", "APPLICATION") },
		v.parseIdentPath,
	)
}

// ParseShowOffers validates the Snowflake `SHOW OFFERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-offers
//
// Syntax:
//
//	SHOW OFFERS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowOffers() bool {
	// SHOW OFFERS [ LIKE '<pattern>' ] IN LISTING <listing>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("OFFERS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "LISTING") },
		v.parseIdentPath,
	)
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
	// SHOW OPENFLOW DATA PLANE INTEGRATIONS [ LIKE ] [ STARTS WITH ] [ LIMIT … ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("OPENFLOW", "DATA", "PLANE", "INTEGRATIONS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowPricingPlans validates the Snowflake `SHOW PRICING PLANS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-pricing-plans
//
// Syntax:
//
//	SHOW PRICING PLANS [ LIKE '<pattern>' ] IN LISTING <listing>
func (v *Validator) ParseShowPricingPlans() bool {
	// SHOW PRICING PLANS [ LIKE '<pattern>' ] IN LISTING <listing>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("PRICING", "PLANS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "LISTING") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("PRIVACY", "POLICIES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowReferences validates the Snowflake `SHOW REFERENCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-references
//
// Syntax:
//
//	SHOW REFERENCES IN APPLICATION <name>
func (v *Validator) ParseShowReferences() bool {
	// SHOW REFERENCES IN APPLICATION <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("REFERENCES") },
		func() bool { return v.phrase("IN", "APPLICATION") },
		v.parseIdentPath,
	)
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
	// SHOW RELEASE CHANNELS IN { APPLICATION PACKAGE <name> | LISTING <name> }
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("RELEASE", "CHANNELS") },
		func() bool { return v.MatchWord("IN") },
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("APPLICATION", "PACKAGE") },
				func() bool { return v.MatchWord("LISTING") },
			)
		},
		v.parseIdentPath,
	)
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
	// SHOW RELEASE DIRECTIVES [ LIKE ] IN APPLICATION PACKAGE <name>
	//   [ FOR RELEASE CHANNEL <release_channel> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("RELEASE", "DIRECTIVES") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "APPLICATION", "PACKAGE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.phrase("FOR", "RELEASE", "CHANNEL") },
					v.parseIdentPath,
				)
			})
		},
	)
}

// ParseShowRolesInService validates the Snowflake `SHOW ROLES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-roles-in-service
//
// Syntax:
//
//	SHOW ROLES IN SERVICE <name>
func (v *Validator) ParseShowRolesInService() bool {
	// SHOW ROLES IN SERVICE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("ROLES") },
		func() bool { return v.phrase("IN", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseShowRulesInEventRoutingTable validates the Snowflake `SHOW RULES IN EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-rules-in-event-routing-table
//
// Syntax:
//
//	SHOW RULES IN EVENT ROUTING TABLE (<event_routing_table_name>)
func (v *Validator) ParseShowRulesInEventRoutingTable() bool {
	// SHOW RULES IN EVENT ROUTING TABLE (<event_routing_table_name>)
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("RULES") },
		func() bool { return v.phrase("IN", "EVENT", "ROUTING", "TABLE") },
		v.consumeBalancedParens,
	)
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
	// SHOW RUN { METRICS | PARAMETERS } [ LIKE ] IN EXPERIMENT <name> [ RUN <name> ]
	//   [ LIMIT <rows> [ FROM <name_string> ] ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("RUN") },
		v.wordsValue("METRICS", "PARAMETERS"),
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "EXPERIMENT") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("RUN") }, v.parseIdentPath)
			})
		},
		func() bool { return v.Optional(v.limitFromClause) },
	)
}

// ParseShowRunsInExperiment validates the Snowflake `SHOW RUNS IN EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-runs-in-experiment
//
// Syntax:
//
//	SHOW RUNS [ LIKE '<pattern>' ] IN EXPERIMENT <name>
func (v *Validator) ParseShowRunsInExperiment() bool {
	// SHOW RUNS [ LIKE '<pattern>' ] IN EXPERIMENT <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("RUNS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "EXPERIMENT") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SEMANTIC", "DIMENSIONS") },
		func() bool { return v.showTrailers() },
	)
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
	// SHOW SEMANTIC DIMENSIONS [ LIKE ] IN <semantic_view_name>
	//   FOR METRIC <metric_name> [ STARTS WITH ] [ LIMIT <rows> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SEMANTIC", "DIMENSIONS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.MatchWord("IN") },
		v.parseIdentPath,
		func() bool { return v.phrase("FOR", "METRIC") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.startsWithClause) },
		func() bool { return v.Optional(v.limitFromClause) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SEMANTIC", "FACTS") },
		func() bool { return v.showTrailers() },
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SEMANTIC", "METRICS") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowServiceContainersInService validates the Snowflake `SHOW SERVICE CONTAINERS IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-containers-in-service
//
// Syntax:
//
//	SHOW SERVICE CONTAINERS IN SERVICE <name>
func (v *Validator) ParseShowServiceContainersInService() bool {
	// SHOW SERVICE CONTAINERS IN SERVICE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SERVICE", "CONTAINERS") },
		func() bool { return v.phrase("IN", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseShowServiceInstancesInService validates the Snowflake `SHOW SERVICE INSTANCES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-instances-in-service
//
// Syntax:
//
//	SHOW SERVICE INSTANCES IN SERVICE <name>
func (v *Validator) ParseShowServiceInstancesInService() bool {
	// SHOW SERVICE INSTANCES IN SERVICE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SERVICE", "INSTANCES") },
		func() bool { return v.phrase("IN", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseShowServiceVolumesInService validates the Snowflake `SHOW SERVICE VOLUMES IN SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-volumes-in-service
//
// Syntax:
//
//	SHOW SERVICE VOLUMES IN SERVICE <name>
func (v *Validator) ParseShowServiceVolumesInService() bool {
	// SHOW SERVICE VOLUMES IN SERVICE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SERVICE", "VOLUMES") },
		func() bool { return v.phrase("IN", "SERVICE") },
		v.parseIdentPath,
	)
}

// ParseShowSharedContent validates the Snowflake `SHOW SHARED CONTENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shared-content
//
// Syntax:
//
//	SHOW SHARED CONTENT IN APPLICATION PACKAGE <pkg_name> FOR VERSION <version_name>
func (v *Validator) ParseShowSharedContent() bool {
	// SHOW SHARED CONTENT IN APPLICATION PACKAGE <pkg_name> FOR VERSION <version_name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("SHARED", "CONTENT") },
		func() bool { return v.phrase("IN", "APPLICATION", "PACKAGE") },
		v.parseIdentPath,
		func() bool { return v.phrase("FOR", "VERSION") },
		v.parseIdentPath,
	)
}

// ParseShowSharesInFailoverGroup validates the Snowflake `SHOW SHARES IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares-in-failover-group
//
// Syntax:
//
//	SHOW SHARES IN FAILOVER GROUP <name>
func (v *Validator) ParseShowSharesInFailoverGroup() bool {
	// SHOW SHARES IN FAILOVER GROUP <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SHARES") },
		func() bool { return v.phrase("IN", "FAILOVER", "GROUP") },
		v.parseIdentPath,
	)
}

// ParseShowSharesInReplicationGroup validates the Snowflake `SHOW SHARES IN REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares-in-replication-group
//
// Syntax:
//
//	SHOW SHARES IN REPLICATION GROUP <name>
func (v *Validator) ParseShowSharesInReplicationGroup() bool {
	// SHOW SHARES IN REPLICATION GROUP <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SHARES") },
		func() bool { return v.phrase("IN", "REPLICATION", "GROUP") },
		v.parseIdentPath,
	)
}

// ParseShowSnapshotsInSnapshotSet validates the Snowflake `SHOW SNAPSHOTS IN SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshots-in-snapshot-set
//
// Syntax:
//
//	SHOW SNAPSHOTS IN SNAPSHOT SET <name>
func (v *Validator) ParseShowSnapshotsInSnapshotSet() bool {
	// SHOW SNAPSHOTS IN SNAPSHOT SET <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("SNAPSHOTS") },
		func() bool { return v.phrase("IN", "SNAPSHOT", "SET") },
		v.parseIdentPath,
	)
}

// ParseShowSpecifications validates the Snowflake `SHOW SPECIFICATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-specifications
//
// Syntax:
//
//	SHOW [ { APPROVED | DECLINED | PENDING } ] SPECIFICATIONS [ IN APPLICATION <app_name> ];
func (v *Validator) ParseShowSpecifications() bool {
	// SHOW [ { APPROVED | DECLINED | PENDING } ] SPECIFICATIONS [ IN APPLICATION <app_name> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.Optional(v.wordsValue("APPROVED", "DECLINED", "PENDING")) },
		func() bool { return v.MatchWord("SPECIFICATIONS") },
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

// ParseShowTelemetryEventDefinitions validates the Snowflake `SHOW TELEMETRY EVENT DEFINITIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-telemetry-event-definitions
//
// Syntax:
//
//	SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION <name>
func (v *Validator) ParseShowTelemetryEventDefinitions() bool {
	// SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("TELEMETRY", "EVENT", "DEFINITIONS") },
		func() bool { return v.phrase("IN", "APPLICATION") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.phrase("USER", "PROCEDURES") },
		func() bool { return v.showTrailers() },
	)
}

// ParseShowUserProgrammaticAccessTokens validates the Snowflake `SHOW USER PROGRAMMATIC ACCESS TOKENS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-programmatic-access-tokens
//
// Syntax:
//
//	SHOW USER { PROGRAMMATIC ACCESS TOKENS | PATS } [ FOR USER <username> ]
func (v *Validator) ParseShowUserProgrammaticAccessTokens() bool {
	// SHOW USER { PROGRAMMATIC ACCESS TOKENS | PATS } [ FOR USER <username> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("USER") },
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("PROGRAMMATIC", "ACCESS", "TOKENS") },
				func() bool { return v.MatchWord("PATS") },
			)
		},
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.phrase("FOR", "USER") }, v.parseIdentPath)
			})
		},
	)
}

// ParseShowUserWorkloadIdentityAuthenticationMethods validates the Snowflake `SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-workload-identity-authentication-methods
//
// Syntax:
//
//	SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS [ FOR USER <username> ]
func (v *Validator) ParseShowUserWorkloadIdentityAuthenticationMethods() bool {
	// SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS [ FOR USER <username> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.phrase("WORKLOAD", "IDENTITY", "AUTHENTICATION", "METHODS") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.phrase("FOR", "USER") }, v.parseIdentPath)
			})
		},
	)
}

// ParseShowVersions validates the Snowflake `SHOW VERSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE <pattern> ]
//	  IN APPLICATION PACKAGE <name>;
func (v *Validator) ParseShowVersions() bool {
	// SHOW VERSIONS [ LIKE <pattern> ] IN APPLICATION PACKAGE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "APPLICATION", "PACKAGE") },
		v.parseIdentPath,
	)
}

// ParseShowVersionsInDataset validates the Snowflake `SHOW VERSIONS IN DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-dataset
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE '<pattern>' ] IN DATASET <dataset_name>
//	  [ LIMIT <rows>]
func (v *Validator) ParseShowVersionsInDataset() bool {
	// SHOW VERSIONS [ LIKE '<pattern>' ] IN DATASET <dataset_name> [ LIMIT <rows> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "DATASET") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.limitFromClause) },
	)
}

// ParseShowVersionsInDbtProject validates the Snowflake `SHOW VERSIONS IN DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-dbt-project
//
// Syntax:
//
//	SHOW VERSIONS IN DBT PROJECT <name>
//	  [ LIMIT <number> ]
func (v *Validator) ParseShowVersionsInDbtProject() bool {
	// SHOW VERSIONS IN DBT PROJECT <name> [ LIMIT <number> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.phrase("IN", "DBT", "PROJECT") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.limitFromClause) },
	)
}

// ParseShowVersionsInListing validates the Snowflake `SHOW VERSIONS IN LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-listing
//
// Syntax:
//
//	SHOW VERSIONS IN LISTING <name>
//	  [ LIMIT <rows> ]
func (v *Validator) ParseShowVersionsInListing() bool {
	// SHOW VERSIONS IN LISTING <name> [ LIMIT <rows> ]
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.phrase("IN", "LISTING") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.limitFromClause) },
	)
}

// ParseShowVersionsInModel validates the Snowflake `SHOW VERSIONS IN MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-model
//
// Syntax:
//
//	SHOW VERSIONS [ LIKE '<pattern>' ] IN MODEL <model_name>
func (v *Validator) ParseShowVersionsInModel() bool {
	// SHOW VERSIONS [ LIKE '<pattern>' ] IN MODEL <model_name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.Optional(v.likeClause) },
		func() bool { return v.phrase("IN", "MODEL") },
		v.parseIdentPath,
	)
}

// ParseShowVersionsInOrganizationProfile validates the Snowflake `SHOW VERSIONS IN ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-versions-in-organization-profile
//
// Syntax:
//
//	SHOW VERSIONS IN ORGANIZATION PROFILE <name>
func (v *Validator) ParseShowVersionsInOrganizationProfile() bool {
	// SHOW VERSIONS IN ORGANIZATION PROFILE <name>
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("VERSIONS") },
		func() bool { return v.phrase("IN", "ORGANIZATION", "PROFILE") },
		v.parseIdentPath,
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("SHOW") },
		func() bool { return v.MatchWord("WORKSPACES") },
		func() bool { return v.showTrailers() },
	)
}
