package sqlgrammar

import "testing"

// Tests for the first 45 ParseShow* grammar rules implemented in show.go.

func TestParseShowObjs(t *testing.T) {
	assertValid(t, (*Validator).ParseShowObjs,
		`SHOW OBJECTS`,
		`SHOW TERSE OBJECTS LIKE '%foo%'`,
		`SHOW OBJECTS IN SCHEMA my_db.my_schema LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowObjs,
		``,
		`SHOW`,
		`SELECT OBJECTS`,
	)
}

func TestParseShowAccounts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAccounts,
		`SHOW ACCOUNTS`,
		`SHOW ACCOUNTS HISTORY`,
		`SHOW ACCOUNTS HISTORY LIKE 'prod%'`,
	)
	assertInvalid(t, (*Validator).ParseShowAccounts,
		``,
		`SHOW`,
		`ACCOUNTS HISTORY`,
	)
}

func TestParseShowAgents(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAgents,
		`SHOW AGENTS`,
		`SHOW AGENTS LIKE 'a%' IN ACCOUNT`,
		`SHOW AGENTS IN SCHEMA db.sch STARTS WITH 'x' LIMIT 5 FROM 'y'`,
	)
	assertInvalid(t, (*Validator).ParseShowAgents,
		``,
		`SHOW`,
		`SHOWS AGENTS`,
	)
}

func TestParseShowAggregationPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAggregationPolicies,
		`SHOW AGGREGATION POLICIES`,
		`SHOW AGGREGATION POLICIES LIKE '%'`,
		`SHOW AGGREGATION POLICIES IN DATABASE mydb`,
	)
	assertInvalid(t, (*Validator).ParseShowAggregationPolicies,
		``,
		`SHOW AGGREGATION`,
		`SHOW POLICIES`,
	)
}

func TestParseShowAlerts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAlerts,
		`SHOW ALERTS`,
		`SHOW TERSE ALERTS LIKE 'a%'`,
		`SHOW ALERTS IN SCHEMA s STARTS WITH 'p' LIMIT 1`,
	)
	assertInvalid(t, (*Validator).ParseShowAlerts,
		``,
		`SHOW`,
		`ALERTS`,
	)
}

func TestParseShowApplicationPackages(t *testing.T) {
	assertValid(t, (*Validator).ParseShowApplicationPackages,
		`SHOW APPLICATION PACKAGES`,
		`SHOW APPLICATION PACKAGES LIKE 'pkg%'`,
		`SHOW APPLICATION PACKAGES STARTS WITH 'a' LIMIT 3 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowApplicationPackages,
		``,
		`SHOW APPLICATION`,
		`SHOW PACKAGES`,
	)
}

func TestParseShowApplicationRoles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowApplicationRoles,
		`SHOW APPLICATION ROLES IN APPLICATION my_app`,
		`SHOW APPLICATION ROLES LIKE 'r%' IN APPLICATION my_app`,
		`SHOW APPLICATION ROLES IN APPLICATION my_app LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowApplicationRoles,
		``,
		`SHOW APPLICATION`,
		`SHOW ROLES IN APPLICATION app`,
	)
}

func TestParseShowApplications(t *testing.T) {
	assertValid(t, (*Validator).ParseShowApplications,
		`SHOW APPLICATIONS`,
		`SHOW APPLICATIONS LIKE 'app%'`,
		`SHOW APPLICATIONS STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowApplications,
		``,
		`SHOW`,
		`APPLICATIONS`,
	)
}

func TestParseShowAuthenticationPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAuthenticationPolicies,
		`SHOW AUTHENTICATION POLICIES`,
		`SHOW AUTHENTICATION POLICIES LIKE '%' IN ACCOUNT`,
		`SHOW AUTHENTICATION POLICIES ON USER bob STARTS WITH 'p' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowAuthenticationPolicies,
		``,
		`SHOW AUTHENTICATION`,
		`SHOW POLICIES`,
	)
}

func TestParseShowAvailableListings(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAvailableListings,
		`SHOW AVAILABLE LISTINGS`,
		`SHOW TERSE AVAILABLE LISTINGS LIMIT 100`,
		`SHOW AVAILABLE LISTINGS IS_IMPORTED = TRUE IS_SHARED_WITH_ME = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseShowAvailableListings,
		``,
		`SHOW AVAILABLE`,
		`SHOW LISTINGS`,
	)
}

func TestParseShowAvailableOffers(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAvailableOffers,
		`SHOW AVAILABLE OFFERS IN LISTING my_listing`,
		`SHOW AVAILABLE OFFERS LIKE 'o%' IN LISTING my_listing`,
		`SHOW AVAILABLE OFFERS IN LISTING db.lst`,
	)
	assertInvalid(t, (*Validator).ParseShowAvailableOffers,
		``,
		`SHOW AVAILABLE OFFERS`,
		`SHOW AVAILABLE OFFERS IN my_listing`,
	)
}

func TestParseShowAvailableOrganizationProfiles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowAvailableOrganizationProfiles,
		`SHOW AVAILABLE ORGANIZATION PROFILES`,
		`show available organization profiles`,
		`SHOW Available Organization Profiles`,
	)
	assertInvalid(t, (*Validator).ParseShowAvailableOrganizationProfiles,
		``,
		`SHOW AVAILABLE ORGANIZATION`,
		`SHOW ORGANIZATION PROFILES`,
	)
}

func TestParseShowBackupPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowBackupPolicies,
		`SHOW BACKUP POLICIES`,
		`SHOW BACKUP POLICIES LIKE '%'`,
		`SHOW BACKUP POLICIES IN DATABASE db STARTS WITH 'p' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowBackupPolicies,
		``,
		`SHOW BACKUP`,
		`SHOW POLICIES`,
	)
}

func TestParseShowBackupSets(t *testing.T) {
	assertValid(t, (*Validator).ParseShowBackupSets,
		`SHOW BACKUP SETS`,
		`SHOW BACKUP SETS LIKE '%'`,
		`SHOW BACKUP SETS IN SCHEMA s LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowBackupSets,
		``,
		`SHOW BACKUP`,
		`SHOW SETS`,
	)
}

func TestParseShowBackupsInBackupSet(t *testing.T) {
	assertValid(t, (*Validator).ParseShowBackupsInBackupSet,
		`SHOW BACKUPS IN BACKUP SET my_set`,
		`SHOW BACKUPS IN BACKUP SET db.sch.my_set`,
		`SHOW BACKUPS IN BACKUP SET my_set LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowBackupsInBackupSet,
		``,
		`SHOW BACKUPS IN BACKUP SET`,
		`SHOW BACKUPS my_set`,
	)
}

func TestParseShowCallerGrants(t *testing.T) {
	assertValid(t, (*Validator).ParseShowCallerGrants,
		`SHOW CALLER GRANTS ON ACCOUNT`,
		`SHOW CALLER GRANTS ON TABLE my_db.my_schema.t1`,
		`SHOW CALLER GRANTS TO ROLE admin`,
	)
	assertInvalid(t, (*Validator).ParseShowCallerGrants,
		``,
		`SHOW CALLER GRANTS`,
		`SHOW GRANTS ON ACCOUNT`,
	)
}

func TestParseShowCatalogIntegrations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowCatalogIntegrations,
		`SHOW CATALOG INTEGRATIONS`,
		`SHOW CATALOG INTEGRATIONS LIKE 'c%'`,
		`SHOW CATALOG INTEGRATIONS LIKE '%' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowCatalogIntegrations,
		``,
		`SHOW CATALOG`,
		`SHOW INTEGRATIONS`,
	)
}

func TestParseShowChannels(t *testing.T) {
	assertValid(t, (*Validator).ParseShowChannels,
		`SHOW CHANNELS`,
		`SHOW CHANNELS LIKE 'c%' IN TABLE my_table`,
		`SHOW CHANNELS IN PIPE my_pipe`,
	)
	assertInvalid(t, (*Validator).ParseShowChannels,
		``,
		`SHOW`,
		`CHANNELS`,
	)
}

func TestParseShowClasses(t *testing.T) {
	assertValid(t, (*Validator).ParseShowClasses,
		`SHOW CLASSES`,
		`SHOW CLASSES LIKE '%'`,
		`SHOW CLASSES IN DATABASE my_db LIMIT 5 FROM 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowClasses,
		``,
		`SHOW`,
		`CLASSES`,
	)
}

func TestParseShowColumns(t *testing.T) {
	assertValid(t, (*Validator).ParseShowColumns,
		`SHOW COLUMNS`,
		`SHOW COLUMNS LIKE '%'`,
		`SHOW COLUMNS IN TABLE my_db.my_schema.t1`,
	)
	assertInvalid(t, (*Validator).ParseShowColumns,
		``,
		`SHOW`,
		`COLUMNS`,
	)
}

func TestParseShowComputePoolInstanceFamilies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowComputePoolInstanceFamilies,
		`SHOW COMPUTE POOL INSTANCE FAMILIES`,
		`show compute pool instance families`,
		`SHOW Compute Pool Instance Families`,
	)
	assertInvalid(t, (*Validator).ParseShowComputePoolInstanceFamilies,
		``,
		`SHOW COMPUTE POOL INSTANCE`,
		`SHOW COMPUTE POOLS`,
	)
}

func TestParseShowComputePools(t *testing.T) {
	assertValid(t, (*Validator).ParseShowComputePools,
		`SHOW COMPUTE POOLS`,
		`SHOW COMPUTE POOLS LIKE 'p%'`,
		`SHOW COMPUTE POOLS STARTS WITH 'a' LIMIT 5 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowComputePools,
		``,
		`SHOW COMPUTE`,
		`SHOW POOLS`,
	)
}

func TestParseShowConfigurations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowConfigurations,
		`SHOW CONFIGURATIONS`,
		`SHOW CONFIGURATIONS IN APPLICATION my_app`,
		`SHOW CONFIGURATIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowConfigurations,
		``,
		`SHOW`,
		`CONFIGURATIONS`,
	)
}

func TestParseShowConnections(t *testing.T) {
	assertValid(t, (*Validator).ParseShowConnections,
		`SHOW CONNECTIONS`,
		`SHOW CONNECTIONS LIKE 'c%'`,
		`SHOW CONNECTIONS LIKE '%'`,
	)
	assertInvalid(t, (*Validator).ParseShowConnections,
		``,
		`SHOW`,
		`CONNECTIONS`,
	)
}

func TestParseShowContacts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowContacts,
		`SHOW CONTACTS`,
		`SHOW CONTACTS LIKE '%'`,
		`SHOW CONTACTS IN SCHEMA s STARTS WITH 'x' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowContacts,
		``,
		`SHOW`,
		`CONTACTS`,
	)
}

func TestParseShowCortexSearchServices(t *testing.T) {
	assertValid(t, (*Validator).ParseShowCortexSearchServices,
		`SHOW CORTEX SEARCH SERVICES`,
		`SHOW CORTEX SEARCH SERVICES LIKE PATTERN 'svc%'`,
		`SHOW CORTEX SEARCH SERVICES STARTS WITH 'a' LIMIT 5 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowCortexSearchServices,
		``,
		`SHOW CORTEX SEARCH`,
		`SHOW SERVICES`,
	)
}

func TestParseShowDataMetricFunctions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDataMetricFunctions,
		`SHOW DATA METRIC FUNCTIONS`,
		`SHOW DATA METRIC FUNCTIONS LIKE '%'`,
		`SHOW DATA METRIC FUNCTIONS IN SCHEMA s STARTS WITH 'p'`,
	)
	assertInvalid(t, (*Validator).ParseShowDataMetricFunctions,
		``,
		`SHOW DATA METRIC`,
		`SHOW FUNCTIONS`,
	)
}

func TestParseShowDatabaseRoles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDatabaseRoles,
		`SHOW DATABASE ROLES IN DATABASE my_db`,
		`SHOW DATABASE ROLES IN DATABASE my_db LIMIT 5`,
		`SHOW DATABASE ROLES IN DATABASE my_db LIMIT 5 FROM 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowDatabaseRoles,
		``,
		`SHOW DATABASE`,
		`SHOW ROLES`,
	)
}

func TestParseShowDatabases(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDatabases,
		`SHOW DATABASES`,
		`SHOW TERSE DATABASES HISTORY LIKE 'db%'`,
		`SHOW DATABASES LIMIT 5 WITH PRIVILEGES USAGE, MONITOR`,
	)
	assertInvalid(t, (*Validator).ParseShowDatabases,
		``,
		`SHOW`,
		`DATABASES`,
	)
}

func TestParseShowDatabasesInFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDatabasesInFailoverGroup,
		`SHOW DATABASES IN FAILOVER GROUP my_fg`,
		`SHOW DATABASES IN FAILOVER GROUP db.fg`,
		`show databases in failover group fg1`,
	)
	assertInvalid(t, (*Validator).ParseShowDatabasesInFailoverGroup,
		``,
		`SHOW DATABASES IN FAILOVER GROUP`,
		`SHOW DATABASES IN GROUP fg`,
	)
}

func TestParseShowDatabasesInReplicationGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDatabasesInReplicationGroup,
		`SHOW DATABASES IN REPLICATION GROUP my_rg`,
		`SHOW DATABASES IN REPLICATION GROUP db.rg`,
		`show databases in replication group rg1`,
	)
	assertInvalid(t, (*Validator).ParseShowDatabasesInReplicationGroup,
		``,
		`SHOW DATABASES IN REPLICATION GROUP`,
		`SHOW DATABASES IN REPLICATION rg`,
	)
}

func TestParseShowDatasets(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDatasets,
		`SHOW DATASETS`,
		`SHOW DATASETS LIKE '%'`,
		`SHOW DATASETS IN SCHEMA my_schema STARTS WITH 'a' LIMIT 5 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowDatasets,
		``,
		`SHOW`,
		`DATASETS`,
	)
}

func TestParseShowDbtProjects(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDbtProjects,
		`SHOW DBT PROJECTS`,
		`SHOW DBT PROJECTS LIKE '%'`,
		`SHOW DBT PROJECTS IN SCHEMA s STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowDbtProjects,
		``,
		`SHOW DBT`,
		`SHOW PROJECTS`,
	)
}

func TestParseShowDcmProjects(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDcmProjects,
		`SHOW DCM PROJECTS`,
		`SHOW TERSE DCM PROJECTS LIKE '%'`,
		`SHOW DCM PROJECTS IN DATABASE db LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowDcmProjects,
		``,
		`SHOW DCM`,
		`SHOW PROJECTS`,
	)
}

func TestParseShowDelegatedAuthorizations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDelegatedAuthorizations,
		`SHOW DELEGATED AUTHORIZATIONS`,
		`SHOW DELEGATED AUTHORIZATIONS BY USER my_user`,
		`SHOW DELEGATED AUTHORIZATIONS TO SECURITY INTEGRATION my_int`,
	)
	assertInvalid(t, (*Validator).ParseShowDelegatedAuthorizations,
		``,
		`SHOW DELEGATED`,
		`SHOW AUTHORIZATIONS`,
	)
}

func TestParseShowDeploymentsInDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDeploymentsInDcmProject,
		`SHOW DEPLOYMENTS IN DCM PROJECT my_proj`,
		`SHOW DEPLOYMENTS IN DCM PROJECT db.proj LIMIT 5`,
		`show deployments in dcm project p1`,
	)
	assertInvalid(t, (*Validator).ParseShowDeploymentsInDcmProject,
		``,
		`SHOW DEPLOYMENTS IN DCM PROJECT`,
		`SHOW DEPLOYMENTS my_proj`,
	)
}

func TestParseShowDynamicTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowDynamicTables,
		`SHOW DYNAMIC TABLES`,
		`SHOW DYNAMIC TABLES LIKE '%'`,
		`SHOW DYNAMIC TABLES IN SCHEMA s STARTS WITH 'a' LIMIT 5 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowDynamicTables,
		``,
		`SHOW DYNAMIC`,
		`SHOW TABLES`,
	)
}

func TestParseShowEndpoints(t *testing.T) {
	assertValid(t, (*Validator).ParseShowEndpoints,
		`SHOW ENDPOINTS IN SERVICE my_svc`,
		`SHOW ENDPOINTS IN SERVICE db.sch.svc`,
		`show endpoints in service s1`,
	)
	assertInvalid(t, (*Validator).ParseShowEndpoints,
		``,
		`SHOW ENDPOINTS`,
		`SHOW ENDPOINTS IN my_svc`,
	)
}

func TestParseShowEntitiesInDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseShowEntitiesInDcmProject,
		`SHOW ENTITIES IN DCM PROJECT my_proj`,
		`SHOW ENTITIES IN DCM PROJECT my_proj LIMIT 10`,
		`SHOW ENTITIES IN DCM PROJECT my_proj STARTS WITH 'pre'`,
	)
	assertInvalid(t, (*Validator).ParseShowEntitiesInDcmProject,
		``,
		`SHOW ENTITIES`,
		`SHOW ENTITIES IN PROJECT my_proj`,
	)
}

func TestParseShowEventTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowEventTables,
		`SHOW EVENT TABLES`,
		`SHOW TERSE EVENT TABLES LIKE '%'`,
		`SHOW EVENT TABLES IN SCHEMA s STARTS WITH 'a' LIMIT 5 FROM 'b'`,
	)
	assertInvalid(t, (*Validator).ParseShowEventTables,
		``,
		`SHOW EVENT`,
		`SHOW TABLES`,
	)
}

func TestParseShowExperiments(t *testing.T) {
	assertValid(t, (*Validator).ParseShowExperiments,
		`SHOW EXPERIMENTS`,
		`SHOW EXPERIMENTS LIKE '%'`,
		`SHOW EXPERIMENTS IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowExperiments,
		``,
		`SHOW`,
		`EXPERIMENTS`,
	)
}

func TestParseShowExternalAgents(t *testing.T) {
	assertValid(t, (*Validator).ParseShowExternalAgents,
		`SHOW EXTERNAL AGENTS`,
		`SHOW EXTERNAL AGENTS LIKE '%'`,
		`SHOW EXTERNAL AGENTS IN DATABASE db`,
	)
	assertInvalid(t, (*Validator).ParseShowExternalAgents,
		``,
		`SHOW EXTERNAL`,
		`SHOW AGENTS`,
	)
}

func TestParseShowExternalFunctions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowExternalFunctions,
		`SHOW EXTERNAL FUNCTIONS`,
		`SHOW EXTERNAL FUNCTIONS LIKE '%'`,
		`SHOW EXTERNAL FUNCTIONS IN APPLICATION my_app`,
	)
	assertInvalid(t, (*Validator).ParseShowExternalFunctions,
		``,
		`SHOW EXTERNAL`,
		`SHOW FUNCTIONS`,
	)
}

func TestParseShowExternalTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowExternalTables,
		`SHOW EXTERNAL TABLES`,
		`SHOW TERSE EXTERNAL TABLES LIKE '%'`,
		`SHOW EXTERNAL TABLES IN SCHEMA s STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowExternalTables,
		``,
		`SHOW EXTERNAL`,
		`SHOW TABLES`,
	)
}

func TestParseShowExternalVolumes(t *testing.T) {
	assertValid(t, (*Validator).ParseShowExternalVolumes,
		`SHOW EXTERNAL VOLUMES`,
		`SHOW EXTERNAL VOLUMES LIKE 'v%'`,
		`SHOW EXTERNAL VOLUMES LIKE '%'`,
	)
	assertInvalid(t, (*Validator).ParseShowExternalVolumes,
		``,
		`SHOW EXTERNAL`,
		`SHOW VOLUMES`,
	)
}
