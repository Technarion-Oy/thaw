package sqlgrammar

import "testing"

func TestParseShowParameters(t *testing.T) {
	assertValid(t, (*Validator).ParseShowParameters,
		`SHOW PARAMETERS`,
		`SHOW PARAMETERS LIKE 'MAX%'`,
		`SHOW PARAMETERS IN SESSION`,
		`SHOW PARAMETERS LIKE 'TIME%' FOR WAREHOUSE my_wh`,
		`SHOW PARAMETERS IN TABLE my_db.my_schema.my_table`,
	)
	assertInvalid(t, (*Validator).ParseShowParameters,
		``,
		`SELECT PARAMETERS`,
		`SHOW`,
	)
}

func TestParseShowPipes(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPipes,
		`SHOW PIPES`,
		`SHOW PIPES LIKE 'p%'`,
		`SHOW PIPES IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowPipes,
		``,
		`SHOW`,
		`DESCRIBE PIPES`,
	)
}

func TestParseShowPostgresInstances(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPostgresInstances,
		`SHOW POSTGRES INSTANCES`,
		`SHOW POSTGRES INSTANCES LIKE 'pg%'`,
		`SHOW POSTGRES INSTANCES STARTS WITH 'pg' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowPostgresInstances,
		``,
		`SHOW POSTGRES`,
		`SHOW INSTANCES`,
	)
}

func TestParseShowPrimaryKeys(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPrimaryKeys,
		`SHOW PRIMARY KEYS`,
		`SHOW TERSE PRIMARY KEYS`,
		`SHOW PRIMARY KEYS IN TABLE my_db.my_schema.t1`,
	)
	assertInvalid(t, (*Validator).ParseShowPrimaryKeys,
		``,
		`SHOW PRIMARY`,
		`SHOW KEYS`,
	)
}

func TestParseShowPrivileges(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPrivileges,
		`SHOW PRIVILEGES IN APPLICATION my_app`,
		`SHOW PRIVILEGES IN APPLICATION db.app`,
		`SHOW PRIVILEGES IN APPLICATION "MyApp"`,
	)
	assertInvalid(t, (*Validator).ParseShowPrivileges,
		``,
		`SHOW PRIVILEGES`,
		`SHOW PRIVILEGES IN APPLICATION`,
	)
}

func TestParseShowProcedures(t *testing.T) {
	assertValid(t, (*Validator).ParseShowProcedures,
		`SHOW PROCEDURES`,
		`SHOW PROCEDURES LIKE 'p%'`,
		`SHOW PROCEDURES IN CLASS my_db.my_class`,
	)
	assertInvalid(t, (*Validator).ParseShowProcedures,
		``,
		`SHOW`,
		`DROP PROCEDURES`,
	)
}

func TestParseShowProvisionedThroughput(t *testing.T) {
	assertValid(t, (*Validator).ParseShowProvisionedThroughput,
		`SHOW PROVISIONED THROUGHPUT`,
		`SHOW PROVISIONED THROUGHPUT LIKE 'x%'`,
		`SHOW PROVISIONED THROUGHPUT IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowProvisionedThroughput,
		``,
		`SHOW PROVISIONED`,
	)
}

func TestParseShowProjectionPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowProjectionPolicies,
		`SHOW PROJECTION POLICIES`,
		`SHOW PROJECTION POLICIES LIKE 'p%'`,
		`SHOW PROJECTION POLICIES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowProjectionPolicies,
		``,
		`SHOW PROJECTION`,
		`SHOW POLICIES`,
	)
}

func TestParseShowQueries(t *testing.T) {
	assertValid(t, (*Validator).ParseShowQueries,
		`SHOW QUERIES`,
		`SHOW QUERIES LIKE 'q%'`,
		`SHOW QUERIES IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowQueries,
		``,
		`SHOW`,
	)
}

func TestParseShowRegions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRegions,
		`SHOW REGIONS`,
		`SHOW REGIONS LIKE 'aws%'`,
		`SHOW REGIONS LIKE 'AZURE%'`,
	)
	assertInvalid(t, (*Validator).ParseShowRegions,
		``,
		`SHOW`,
		`SHOW REGIONS IN ACCOUNT`,
	)
}

func TestParseShowReplicatedDatabases(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReplicatedDatabases,
		`SHOW REPLICATED DATABASES`,
		`SHOW REPLICATED DATABASES LIKE 'd%'`,
		`SHOW REPLICATED DATABASES IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowReplicatedDatabases,
		``,
		`SHOW REPLICATED`,
	)
}

func TestParseShowReplicationAccounts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReplicationAccounts,
		`SHOW REPLICATION ACCOUNTS`,
		`SHOW REPLICATION ACCOUNTS LIKE 'a%'`,
		`SHOW REPLICATION ACCOUNTS LIKE 'org1%'`,
	)
	assertInvalid(t, (*Validator).ParseShowReplicationAccounts,
		``,
		`SHOW REPLICATION`,
		`SHOW REPLICATION ACCOUNTS IN ACCOUNT`,
	)
}

func TestParseShowReplicationDatabases(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReplicationDatabases,
		`SHOW REPLICATION DATABASES`,
		`SHOW REPLICATION DATABASES LIKE 'd%'`,
		`SHOW REPLICATION DATABASES WITH PRIMARY org1.account1.db1`,
	)
	assertInvalid(t, (*Validator).ParseShowReplicationDatabases,
		``,
		`SHOW REPLICATION`,
		`SHOW REPLICATION DATABASES WITH PRIMARY`,
	)
}

func TestParseShowReplicationGroups(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReplicationGroups,
		`SHOW REPLICATION GROUPS`,
		`SHOW REPLICATION GROUPS IN ACCOUNT my_account`,
		`SHOW REPLICATION GROUPS IN ACCOUNT org1.acct1`,
	)
	assertInvalid(t, (*Validator).ParseShowReplicationGroups,
		``,
		`SHOW REPLICATION`,
		`SHOW REPLICATION GROUPS IN ACCOUNT`,
	)
}

func TestParseShowResourceMonitors(t *testing.T) {
	assertValid(t, (*Validator).ParseShowResourceMonitors,
		`SHOW RESOURCE MONITORS`,
		`SHOW RESOURCE MONITORS LIKE 'rm%'`,
		`SHOW RESOURCE MONITORS LIKE 'prod%'`,
	)
	assertInvalid(t, (*Validator).ParseShowResourceMonitors,
		``,
		`SHOW RESOURCE`,
		`SHOW MONITORS`,
	)
}

func TestParseShowRoles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRoles,
		`SHOW ROLES`,
		`SHOW TERSE ROLES`,
		`SHOW ROLES LIKE 'admin%' LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowRoles,
		``,
		`SHOW`,
		`DROP ROLES`,
	)
}

func TestParseShowRowAccessPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRowAccessPolicies,
		`SHOW ROW ACCESS POLICIES`,
		`SHOW ROW ACCESS POLICIES LIKE 'p%'`,
		`SHOW ROW ACCESS POLICIES IN SCHEMA my_db.my_schema LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowRowAccessPolicies,
		``,
		`SHOW ROW ACCESS`,
		`SHOW ROW POLICIES`,
	)
}

func TestParseShowSchemas(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSchemas,
		`SHOW SCHEMAS`,
		`SHOW TERSE SCHEMAS HISTORY LIKE 's%'`,
		`SHOW SCHEMAS IN DATABASE my_db WITH PRIVILEGES USAGE, MONITOR`,
	)
	assertInvalid(t, (*Validator).ParseShowSchemas,
		``,
		`SHOW`,
		`DROP SCHEMAS`,
	)
}

func TestParseShowSearchIndexes(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSearchIndexes,
		`SHOW SEARCH INDEXES`,
		`SHOW SEARCH INDEXES LIKE 'i%'`,
		`SHOW SEARCH INDEXES IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowSearchIndexes,
		``,
		`SHOW SEARCH`,
	)
}

func TestParseShowSecrets(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSecrets,
		`SHOW SECRETS`,
		`SHOW SECRETS LIKE 's%'`,
		`SHOW SECRETS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowSecrets,
		``,
		`SHOW`,
		`DROP SECRETS`,
	)
}

func TestParseShowSecurityIntegrations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSecurityIntegrations,
		`SHOW SECURITY INTEGRATIONS`,
		`SHOW SECURITY INTEGRATIONS LIKE 'oauth%'`,
		`SHOW SECURITY INTEGRATIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowSecurityIntegrations,
		``,
		`SHOW SECURITY`,
	)
}

func TestParseShowSemanticViews(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSemanticViews,
		`SHOW SEMANTIC VIEWS`,
		`SHOW TERSE SEMANTIC VIEWS LIKE 'v%'`,
		`SHOW SEMANTIC VIEWS IN SCHEMA my_db.my_schema STARTS WITH 'sv'`,
	)
	assertInvalid(t, (*Validator).ParseShowSemanticViews,
		``,
		`SHOW SEMANTIC`,
		`SHOW VIEWS SEMANTIC`,
	)
}

func TestParseShowSequences(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSequences,
		`SHOW SEQUENCES`,
		`SHOW SEQUENCES LIKE 'seq%'`,
		`SHOW SEQUENCES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowSequences,
		``,
		`SHOW`,
		`DROP SEQUENCES`,
	)
}

func TestParseShowServiceRoles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowServiceRoles,
		`SHOW SERVICE ROLES`,
		`SHOW SERVICE ROLES IN ACCOUNT`,
		`SHOW SERVICE ROLES LIKE 'r%'`,
	)
	assertInvalid(t, (*Validator).ParseShowServiceRoles,
		``,
		`SHOW SERVICE`,
	)
}

func TestParseShowServices(t *testing.T) {
	assertValid(t, (*Validator).ParseShowServices,
		`SHOW SERVICES`,
		`SHOW JOB SERVICES EXCLUDE JOBS LIKE 's%'`,
		`SHOW SERVICES IN COMPUTE POOL my_pool OF TYPE inference, training`,
	)
	assertInvalid(t, (*Validator).ParseShowServices,
		``,
		`SHOW`,
		`SHOW JOB`,
	)
}

func TestParseShowSessionPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSessionPolicies,
		`SHOW SESSION POLICIES`,
		`SHOW SESSION POLICIES LIKE 'p%' IN ACCOUNT`,
		`SHOW SESSION POLICIES ON USER my_user LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowSessionPolicies,
		``,
		`SHOW SESSION`,
		`SHOW POLICIES`,
	)
}

func TestParseShowSessions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSessions,
		`SHOW SESSIONS`,
		`SHOW SESSIONS LIKE 's%'`,
		`SHOW SESSIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowSessions,
		``,
		`SHOW`,
	)
}

func TestParseShowShares(t *testing.T) {
	assertValid(t, (*Validator).ParseShowShares,
		`SHOW SHARES`,
		`SHOW SHARES LIKE 'sh%'`,
		`SHOW SHARES LIMIT 10 FROM 'last_share'`,
	)
	assertInvalid(t, (*Validator).ParseShowShares,
		``,
		`SHOW`,
		`SHOW SHARES IN ACCOUNT`,
	)
}

func TestParseShowSnapshots(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSnapshots,
		`SHOW SNAPSHOTS`,
		`SHOW SNAPSHOTS LIKE 's%'`,
		`SHOW SNAPSHOTS IN SCHEMA my_db.my_schema STARTS WITH 'snap'`,
	)
	assertInvalid(t, (*Validator).ParseShowSnapshots,
		``,
		`SHOW`,
		`DROP SNAPSHOTS`,
	)
}

func TestParseShowSnapshotPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSnapshotPolicies,
		`SHOW SNAPSHOT POLICIES`,
		`SHOW SNAPSHOT POLICIES LIKE 'p%'`,
		`SHOW SNAPSHOT POLICIES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowSnapshotPolicies,
		``,
		`SHOW SNAPSHOT`,
		`SHOW POLICIES`,
	)
}

func TestParseShowSnapshotSets(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSnapshotSets,
		`SHOW SNAPSHOT SETS`,
		`SHOW SNAPSHOT SETS LIKE 's%'`,
		`SHOW SNAPSHOT SETS IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowSnapshotSets,
		``,
		`SHOW SNAPSHOT`,
		`SHOW SETS`,
	)
}

func TestParseShowStages(t *testing.T) {
	assertValid(t, (*Validator).ParseShowStages,
		`SHOW STAGES`,
		`SHOW STAGES LIKE 'st%'`,
		`SHOW STAGES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowStages,
		``,
		`SHOW`,
		`DROP STAGES`,
	)
}

func TestParseShowStorageIntegrations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowStorageIntegrations,
		`SHOW STORAGE INTEGRATIONS`,
		`SHOW STORAGE INTEGRATIONS LIKE 's3%'`,
		`SHOW STORAGE INTEGRATIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowStorageIntegrations,
		``,
		`SHOW STORAGE`,
	)
}

func TestParseShowStorageLifecyclePolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowStorageLifecyclePolicies,
		`SHOW STORAGE LIFECYCLE POLICIES`,
		`SHOW STORAGE LIFECYCLE POLICIES LIKE 'p%'`,
		`SHOW STORAGE LIFECYCLE POLICIES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowStorageLifecyclePolicies,
		``,
		`SHOW STORAGE LIFECYCLE`,
		`SHOW LIFECYCLE POLICIES`,
	)
}

func TestParseShowStreams(t *testing.T) {
	assertValid(t, (*Validator).ParseShowStreams,
		`SHOW STREAMS`,
		`SHOW TERSE STREAMS LIKE 's%'`,
		`SHOW STREAMS IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowStreams,
		``,
		`SHOW`,
		`DROP STREAMS`,
	)
}

func TestParseShowStreamlits(t *testing.T) {
	assertValid(t, (*Validator).ParseShowStreamlits,
		`SHOW STREAMLITS`,
		`SHOW TERSE STREAMLITS LIKE 's%'`,
		`SHOW STREAMLITS IN DATABASE my_db LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowStreamlits,
		``,
		`SHOW`,
		`DROP STREAMLITS`,
	)
}

func TestParseShowTableFunctions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTableFunctions,
		`SHOW TABLE FUNCTIONS`,
		`SHOW TABLE FUNCTIONS LIKE 'f%'`,
		`SHOW TABLE FUNCTIONS IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowTableFunctions,
		``,
		`SHOW TABLE`,
	)
}

func TestParseShowTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTables,
		`SHOW TABLES`,
		`SHOW TERSE TABLES HISTORY LIKE 't%'`,
		`SHOW TABLES IN SCHEMA my_db.my_schema STARTS WITH 'tbl'`,
	)
	assertInvalid(t, (*Validator).ParseShowTables,
		``,
		`SHOW`,
		`DROP TABLES`,
	)
}

func TestParseShowTags(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTags,
		`SHOW TAGS`,
		`SHOW TAGS LIKE 'env%'`,
		`SHOW TAGS IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowTags,
		``,
		`SHOW`,
		`DROP TAGS`,
	)
}

func TestParseShowTasks(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTasks,
		`SHOW TASKS`,
		`SHOW TERSE TASKS LIKE 't%' ROOT ONLY`,
		`SHOW TASKS IN SCHEMA my_db.my_schema LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowTasks,
		``,
		`SHOW`,
		`DROP TASKS`,
	)
}

func TestParseShowTransactions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTransactions,
		`SHOW TRANSACTIONS`,
		`SHOW TRANSACTIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowTransactions,
		``,
		`SHOW`,
		`SHOW TRANSACTIONS IN DATABASE`,
	)
}

func TestParseShowTypes(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTypes,
		`SHOW TYPES`,
		`SHOW TYPES LIKE 't%'`,
		`SHOW TYPES IN SCHEMA my_db.my_schema STARTS WITH 'ty'`,
	)
	assertInvalid(t, (*Validator).ParseShowTypes,
		``,
		`SHOW`,
		`DROP TYPES`,
	)
}

func TestParseShowUniqueKeys(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUniqueKeys,
		`SHOW UNIQUE KEYS`,
		`SHOW TERSE UNIQUE KEYS`,
		`SHOW UNIQUE KEYS IN TABLE my_db.my_schema.t1`,
	)
	assertInvalid(t, (*Validator).ParseShowUniqueKeys,
		``,
		`SHOW UNIQUE`,
		`SHOW KEYS`,
	)
}

func TestParseShowUserFunctions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUserFunctions,
		`SHOW USER FUNCTIONS`,
		`SHOW USER FUNCTIONS LIKE 'f%'`,
		`SHOW USER FUNCTIONS IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowUserFunctions,
		``,
		`SHOW USER`,
		`SHOW FUNCTIONS`,
	)
}

func TestParseShowUsers(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUsers,
		`SHOW USERS`,
		`SHOW TERSE USERS LIKE 'u%'`,
		`SHOW USERS STARTS WITH 'admin' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowUsers,
		``,
		`SHOW`,
		`DROP USERS`,
	)
}

func TestParseShowVariables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVariables,
		`SHOW VARIABLES`,
		`SHOW VARIABLES LIKE 'v%'`,
		`SHOW VARIABLES LIKE 'session%'`,
	)
	assertInvalid(t, (*Validator).ParseShowVariables,
		``,
		`SHOW`,
		`SHOW VARIABLES IN ACCOUNT`,
	)
}

func TestParseShowViews(t *testing.T) {
	assertValid(t, (*Validator).ParseShowViews,
		`SHOW VIEWS`,
		`SHOW TERSE VIEWS LIKE 'v%'`,
		`SHOW VIEWS IN SCHEMA my_db.my_schema STARTS WITH 'vw'`,
	)
	assertInvalid(t, (*Validator).ParseShowViews,
		``,
		`SHOW`,
		`DROP VIEWS`,
	)
}

func TestParseShowWarehouses(t *testing.T) {
	assertValid(t, (*Validator).ParseShowWarehouses,
		`SHOW WAREHOUSES`,
		`SHOW WAREHOUSES LIKE 'wh%'`,
		`SHOW WAREHOUSES WITH PRIVILEGES USAGE, OPERATE`,
	)
	assertInvalid(t, (*Validator).ParseShowWarehouses,
		``,
		`SHOW`,
		`DROP WAREHOUSES`,
	)
}

func TestParseShowApplicationServices(t *testing.T) {
	assertValid(t, (*Validator).ParseShowApplicationServices,
		`SHOW APPLICATION SERVICES`,
		`SHOW APPLICATION SERVICES LIKE 's%'`,
		`SHOW APPLICATION SERVICES IN DATABASE my_db LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowApplicationServices,
		``,
		`SHOW APPLICATION`,
		`SHOW SERVICES`,
	)
}
