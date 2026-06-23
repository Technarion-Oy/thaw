package sqlgrammar

import "testing"

// Tests for the first 44 ParseDrop* grammar rules (file order: ParseDropObj
// through ParseDropMaskingPolicy).

func TestParseDropObj(t *testing.T) {
	assertValid(t, (*Validator).ParseDropObj,
		`DROP TABLE my_table`,
		`DROP VIEW db.schema.v1 CASCADE`,
		`DROP WAREHOUSE wh`,
	)
	assertInvalid(t, (*Validator).ParseDropObj,
		``,
		`CREATE TABLE t`,
	)
}

func TestParseDropAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseDropAccount,
		`DROP ACCOUNT acct1 GRACE_PERIOD_IN_DAYS = 5`,
		`DROP ACCOUNT IF EXISTS acct1 GRACE_PERIOD_IN_DAYS = 30`,
		`DROP ACCOUNT my_account GRACE_PERIOD_IN_DAYS = 3`,
	)
	assertInvalid(t, (*Validator).ParseDropAccount,
		`DROP ACCOUNT acct1`,
		`DROP acct1 GRACE_PERIOD_IN_DAYS = 5`,
	)
}

func TestParseDropAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseDropAgent,
		`DROP AGENT a1`,
		`DROP AGENT IF EXISTS a1`,
		`DROP AGENT db.schema.a1`,
	)
	assertInvalid(t, (*Validator).ParseDropAgent,
		``,
		`DROP AGENT`,
	)
}

func TestParseDropAggregationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropAggregationPolicy,
		`DROP AGGREGATION POLICY p1`,
		`DROP AGGREGATION POLICY IF EXISTS db.s.p1`,
		`DROP AGGREGATION POLICY schema.p`,
	)
	assertInvalid(t, (*Validator).ParseDropAggregationPolicy,
		`DROP POLICY p1`,
		`DROP AGGREGATION p1`,
	)
}

func TestParseDropAlert(t *testing.T) {
	assertValid(t, (*Validator).ParseDropAlert,
		`DROP ALERT a1`,
		`DROP ALERT IF EXISTS a1`,
		`DROP ALERT db.s.a1`,
	)
	assertInvalid(t, (*Validator).ParseDropAlert,
		``,
		`DROP ALERT`,
	)
}

func TestParseDropApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseDropApplication,
		`DROP APPLICATION app1`,
		`DROP APPLICATION IF EXISTS app1 CASCADE`,
		`DROP APPLICATION app1 CASCADE`,
	)
	assertInvalid(t, (*Validator).ParseDropApplication,
		``,
		`DROP APPLICATION`,
	)
}

func TestParseDropApplicationPackage(t *testing.T) {
	assertValid(t, (*Validator).ParseDropApplicationPackage,
		`DROP APPLICATION PACKAGE pkg1`,
		`DROP APPLICATION PACKAGE IF EXISTS pkg1`,
		`DROP APPLICATION PACKAGE db.pkg1`,
	)
	assertInvalid(t, (*Validator).ParseDropApplicationPackage,
		`DROP APPLICATION pkg1`,
		`DROP PACKAGE pkg1`,
	)
}

func TestParseDropApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseDropApplicationRole,
		`DROP APPLICATION ROLE r1`,
		`DROP APPLICATION ROLE IF EXISTS r1`,
		`DROP APPLICATION ROLE app.r1`,
	)
	assertInvalid(t, (*Validator).ParseDropApplicationRole,
		`DROP APPLICATION r1`,
		`DROP ROLE r1`,
	)
}

func TestParseDropAuthenticationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropAuthenticationPolicy,
		`DROP AUTHENTICATION POLICY p1`,
		`DROP AUTHENTICATION POLICY IF EXISTS db.s.p1`,
		`DROP AUTHENTICATION POLICY p`,
	)
	assertInvalid(t, (*Validator).ParseDropAuthenticationPolicy,
		`DROP AUTHENTICATION p1`,
		`DROP POLICY p1`,
	)
}

func TestParseDropBackupPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropBackupPolicy,
		`DROP BACKUP POLICY p1`,
		`DROP BACKUP POLICY IF EXISTS p1`,
		`DROP BACKUP POLICY db.s.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropBackupPolicy,
		`DROP BACKUP p1`,
		`DROP POLICY p1`,
	)
}

func TestParseDropBackupSet(t *testing.T) {
	assertValid(t, (*Validator).ParseDropBackupSet,
		`DROP BACKUP SET s1`,
		`DROP BACKUP SET IF EXISTS s1`,
		`DROP BACKUP SET db.s1`,
	)
	assertInvalid(t, (*Validator).ParseDropBackupSet,
		`DROP BACKUP s1`,
		`DROP SET s1`,
	)
}

func TestParseDropCatalogIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDropCatalogIntegration,
		`DROP CATALOG INTEGRATION ci1`,
		`DROP CATALOG INTEGRATION IF EXISTS ci1`,
		`DROP CATALOG INTEGRATION db.ci1`,
	)
	assertInvalid(t, (*Validator).ParseDropCatalogIntegration,
		`DROP CATALOG ci1`,
		`DROP INTEGRATION ci1`,
	)
}

func TestParseDropComputePool(t *testing.T) {
	assertValid(t, (*Validator).ParseDropComputePool,
		`DROP COMPUTE POOL cp1`,
		`DROP COMPUTE POOL IF EXISTS cp1`,
		`DROP COMPUTE POOL db.cp1`,
	)
	assertInvalid(t, (*Validator).ParseDropComputePool,
		`DROP COMPUTE cp1`,
		`DROP POOL cp1`,
	)
}

func TestParseDropConnection(t *testing.T) {
	assertValid(t, (*Validator).ParseDropConnection,
		`DROP CONNECTION c1`,
		`DROP CONNECTION IF EXISTS c1`,
		`DROP CONNECTION db.c1`,
	)
	assertInvalid(t, (*Validator).ParseDropConnection,
		``,
		`DROP CONNECTION`,
	)
}

func TestParseDropContact(t *testing.T) {
	assertValid(t, (*Validator).ParseDropContact,
		`DROP CONTACT c1`,
		`DROP CONTACT IF EXISTS c1`,
		`DROP CONTACT db.s.c1`,
	)
	assertInvalid(t, (*Validator).ParseDropContact,
		``,
		`DROP CONTACT`,
	)
}

func TestParseDropCortexSearchService(t *testing.T) {
	assertValid(t, (*Validator).ParseDropCortexSearchService,
		`DROP CORTEX SEARCH SERVICE s1`,
		`DROP CORTEX SEARCH SERVICE IF EXISTS db.s.s1`,
		`DROP CORTEX SEARCH SERVICE svc`,
	)
	assertInvalid(t, (*Validator).ParseDropCortexSearchService,
		`DROP CORTEX SEARCH s1`,
		`DROP SEARCH SERVICE s1`,
	)
}

func TestParseDropDatabase(t *testing.T) {
	assertValid(t, (*Validator).ParseDropDatabase,
		`DROP DATABASE db1`,
		`DROP DATABASE IF EXISTS db1 CASCADE`,
		`DROP DATABASE db1 RESTRICT`,
	)
	assertInvalid(t, (*Validator).ParseDropDatabase,
		``,
		`DROP DATABASE`,
	)
}

func TestParseDropDatabaseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseDropDatabaseRole,
		`DROP DATABASE ROLE r1`,
		`DROP DATABASE ROLE IF EXISTS db.r1`,
		`DROP DATABASE ROLE r`,
	)
	assertInvalid(t, (*Validator).ParseDropDatabaseRole,
		`DROP DATABASE r1`,
		`DROP ROLE r1`,
	)
}

func TestParseDropDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseDropDbtProject,
		`DROP DBT PROJECT p1`,
		`DROP DBT PROJECT IF EXISTS db.s.p1`,
		`DROP DBT PROJECT p`,
	)
	assertInvalid(t, (*Validator).ParseDropDbtProject,
		`DROP DBT p1`,
		`DROP PROJECT p1`,
	)
}

func TestParseDropDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseDropDcmProject,
		`DROP DCM PROJECT p1`,
		`DROP DCM PROJECT IF EXISTS p1`,
		`DROP DCM PROJECT db.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropDcmProject,
		`DROP DCM p1`,
		`DROP PROJECT p1`,
	)
}

func TestParseDropDynamicTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropDynamicTable,
		`DROP DYNAMIC TABLE dt1`,
		`DROP DYNAMIC TABLE IF EXISTS db.s.dt1`,
		`DROP DYNAMIC TABLE dt`,
	)
	assertInvalid(t, (*Validator).ParseDropDynamicTable,
		`DROP DYNAMIC dt1`,
		`DROP TABLE dt1`,
	)
}

func TestParseDropExperiment(t *testing.T) {
	assertValid(t, (*Validator).ParseDropExperiment,
		`DROP EXPERIMENT e1`,
		`DROP EXPERIMENT IF EXISTS e1`,
		`DROP EXPERIMENT db.s.e1`,
	)
	assertInvalid(t, (*Validator).ParseDropExperiment,
		``,
		`DROP EXPERIMENT`,
	)
}

func TestParseDropExternalAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseDropExternalAgent,
		`DROP EXTERNAL AGENT a1`,
		`DROP EXTERNAL AGENT IF EXISTS a1`,
		`DROP EXTERNAL AGENT db.a1`,
	)
	assertInvalid(t, (*Validator).ParseDropExternalAgent,
		`DROP EXTERNAL a1`,
		`DROP AGENT a1`,
	)
}

func TestParseDropExternalTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropExternalTable,
		`DROP EXTERNAL TABLE t1`,
		`DROP EXTERNAL TABLE IF EXISTS t1 CASCADE`,
		`DROP EXTERNAL TABLE db.s.t1 RESTRICT`,
	)
	assertInvalid(t, (*Validator).ParseDropExternalTable,
		`DROP EXTERNAL t1`,
		`DROP TABLE t1`,
	)
}

func TestParseDropExternalVolume(t *testing.T) {
	assertValid(t, (*Validator).ParseDropExternalVolume,
		`DROP EXTERNAL VOLUME v1`,
		`DROP EXTERNAL VOLUME IF EXISTS v1`,
		`DROP EXTERNAL VOLUME db.v1`,
	)
	assertInvalid(t, (*Validator).ParseDropExternalVolume,
		`DROP EXTERNAL v1`,
		`DROP VOLUME v1`,
	)
}

func TestParseDropFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFailoverGroup,
		`DROP FAILOVER GROUP g1`,
		`DROP FAILOVER GROUP IF EXISTS g1`,
		`DROP FAILOVER GROUP db.g1`,
	)
	assertInvalid(t, (*Validator).ParseDropFailoverGroup,
		`DROP FAILOVER g1`,
		`DROP GROUP g1`,
	)
}

func TestParseDropFeaturePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFeaturePolicy,
		`DROP FEATURE POLICY p1`,
		`DROP FEATURE POLICY IF EXISTS p1`,
		`DROP FEATURE POLICY db.s.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropFeaturePolicy,
		`DROP FEATURE p1`,
		`DROP POLICY p1`,
	)
}

func TestParseDropFileFormat(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFileFormat,
		`DROP FILE FORMAT ff1`,
		`DROP FILE FORMAT IF EXISTS ff1`,
		`DROP FILE FORMAT db.s.ff1`,
	)
	assertInvalid(t, (*Validator).ParseDropFileFormat,
		`DROP FILE ff1`,
		`DROP FORMAT ff1`,
	)
}

func TestParseDropFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFunction,
		`DROP FUNCTION my_func()`,
		`DROP FUNCTION IF EXISTS my_func(NUMBER, VARCHAR)`,
		`DROP FUNCTION db.s.my_func(FLOAT)`,
	)
	assertInvalid(t, (*Validator).ParseDropFunction,
		`DROP FUNCTION my_func`,
		`DROP FUNCTION`,
	)
}

func TestParseDropFunctionDmf(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFunctionDmf,
		`DROP FUNCTION my_dmf(TABLE(NUMBER))`,
		`DROP FUNCTION IF EXISTS my_dmf(TABLE(NUMBER), TABLE(VARCHAR))`,
		`DROP FUNCTION db.s.my_dmf(TABLE(FLOAT, NUMBER))`,
	)
	assertInvalid(t, (*Validator).ParseDropFunctionDmf,
		`DROP FUNCTION my_dmf`,
		`DROP FUNCTION`,
	)
}

func TestParseDropFunctionSnowparkContainerServices(t *testing.T) {
	assertValid(t, (*Validator).ParseDropFunctionSnowparkContainerServices,
		`DROP FUNCTION svc_func()`,
		`DROP FUNCTION IF EXISTS svc_func(NUMBER)`,
		`DROP FUNCTION db.s.svc_func(VARCHAR, NUMBER)`,
	)
	assertInvalid(t, (*Validator).ParseDropFunctionSnowparkContainerServices,
		`DROP FUNCTION svc_func`,
		`DROP FUNCTION`,
	)
}

func TestParseDropGateway(t *testing.T) {
	assertValid(t, (*Validator).ParseDropGateway,
		`DROP GATEWAY g1`,
		`DROP GATEWAY IF EXISTS g1`,
		`DROP GATEWAY db.g1`,
	)
	assertInvalid(t, (*Validator).ParseDropGateway,
		``,
		`DROP GATEWAY`,
	)
}

func TestParseDropGitRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseDropGitRepository,
		`DROP GIT REPOSITORY r1`,
		`DROP GIT REPOSITORY IF EXISTS r1`,
		`DROP GIT REPOSITORY db.s.r1`,
	)
	assertInvalid(t, (*Validator).ParseDropGitRepository,
		`DROP GIT r1`,
		`DROP REPOSITORY r1`,
	)
}

func TestParseDropIcebergTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropIcebergTable,
		`DROP ICEBERG TABLE t1`,
		`DROP TABLE IF EXISTS t1 CASCADE`,
		`DROP ICEBERG TABLE db.s.t1 RESTRICT`,
	)
	assertInvalid(t, (*Validator).ParseDropIcebergTable,
		`DROP ICEBERG t1`,
		`DROP TABLE`,
	)
}

func TestParseDropImageRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseDropImageRepository,
		`DROP IMAGE REPOSITORY r1`,
		`DROP IMAGE REPOSITORY IF EXISTS r1`,
		`DROP IMAGE REPOSITORY db.s.r1`,
	)
	assertInvalid(t, (*Validator).ParseDropImageRepository,
		`DROP IMAGE r1`,
		`DROP REPOSITORY r1`,
	)
}

func TestParseDropIndex(t *testing.T) {
	assertValid(t, (*Validator).ParseDropIndex,
		`DROP INDEX my_table.my_index`,
		`DROP INDEX IF EXISTS my_table.my_index`,
		`DROP INDEX db.s.tbl.idx`,
	)
	assertInvalid(t, (*Validator).ParseDropIndex,
		``,
		`DROP INDEX`,
	)
}

func TestParseDropIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDropIntegration,
		`DROP INTEGRATION i1`,
		`DROP API INTEGRATION IF EXISTS i1`,
		`DROP EXTERNAL ACCESS INTEGRATION i1`,
	)
	assertInvalid(t, (*Validator).ParseDropIntegration,
		`DROP API i1`,
		`DROP i1`,
	)
}

func TestParseDropJoinPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropJoinPolicy,
		`DROP JOIN POLICY p1`,
		`DROP JOIN POLICY IF EXISTS p1`,
		`DROP JOIN POLICY db.s.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropJoinPolicy,
		`DROP JOIN p1`,
		`DROP POLICY p1`,
	)
}

func TestParseDropListing(t *testing.T) {
	assertValid(t, (*Validator).ParseDropListing,
		`DROP LISTING l1`,
		`DROP LISTING IF EXISTS l1`,
		`DROP LISTING db.l1`,
	)
	assertInvalid(t, (*Validator).ParseDropListing,
		``,
		`DROP LISTING`,
	)
}

func TestParseDropMaintenancePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropMaintenancePolicy,
		`DROP MAINTENANCE POLICY p1`,
		`DROP MAINTENANCE POLICY IF EXISTS p1`,
		`DROP MAINTENANCE POLICY db.s.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropMaintenancePolicy,
		`DROP MAINTENANCE p1`,
		`DROP POLICY p1`,
	)
}

func TestParseDropManagedAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseDropManagedAccount,
		`DROP MANAGED ACCOUNT a1`,
		`DROP MANAGED ACCOUNT IF EXISTS a1`,
		`DROP MANAGED ACCOUNT db.a1`,
	)
	assertInvalid(t, (*Validator).ParseDropManagedAccount,
		`DROP MANAGED a1`,
		`DROP ACCOUNT a1`,
	)
}

func TestParseDropMaskingPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropMaskingPolicy,
		`DROP MASKING POLICY p1`,
		`DROP MASKING POLICY IF EXISTS p1`,
		`DROP MASKING POLICY db.s.p1`,
	)
	assertInvalid(t, (*Validator).ParseDropMaskingPolicy,
		`DROP MASKING p1`,
		`DROP POLICY p1`,
	)
}
