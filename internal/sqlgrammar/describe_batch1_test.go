package sqlgrammar

import "testing"

// Tests for the first 40 ParseDescribe* grammar rules (batch 1).

func TestParseDescribeObj(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeObj,
		`DESCRIBE my_obj`,
		`DESC db.schema.obj`,
		`DESCRIBE "Quoted"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeObj,
		``,
		`SELECT 1`,
		`DESCRIBE`,
	)
}

func TestParseDescribeAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAgent,
		`DESCRIBE AGENT a1`,
		`DESC AGENT db.sch.a1`,
		`DESCRIBE AGENT "My Agent"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAgent,
		``,
		`DESCRIBE AGENT`,
		`DESCRIBE a1`,
	)
}

func TestParseDescribeAggregationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAggregationPolicy,
		`DESCRIBE AGGREGATION POLICY p1`,
		`DESC AGGREGATION POLICY db.sch.p1`,
		`DESCRIBE AGGREGATION POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAggregationPolicy,
		``,
		`DESCRIBE AGGREGATION p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeAlert(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAlert,
		`DESCRIBE ALERT a1`,
		`DESC ALERT db.sch.a1`,
		`DESCRIBE ALERT "A"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAlert,
		``,
		`DESCRIBE ALERT`,
		`ALERT a1`,
	)
}

func TestParseDescribeApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeApplication,
		`DESCRIBE APPLICATION app1`,
		`DESC APPLICATION app1`,
		`DESCRIBE APPLICATION "App"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeApplication,
		``,
		`DESCRIBE APPLICATION`,
		`DESCRIBE app1`,
	)
}

func TestParseDescribeApplicationPackage(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeApplicationPackage,
		`DESCRIBE APPLICATION PACKAGE pkg1`,
		`DESC APPLICATION PACKAGE pkg1`,
		`DESCRIBE APPLICATION PACKAGE "Pkg"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeApplicationPackage,
		``,
		`DESCRIBE APPLICATION pkg1`,
		`DESCRIBE PACKAGE pkg1`,
	)
}

func TestParseDescribeAuthenticationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAuthenticationPolicy,
		`DESCRIBE AUTHENTICATION POLICY p1`,
		`DESC AUTHENTICATION POLICY db.sch.p1`,
		`DESCRIBE AUTHENTICATION POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAuthenticationPolicy,
		``,
		`DESCRIBE AUTHENTICATION p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeAvailableListing(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAvailableListing,
		`DESCRIBE AVAILABLE LISTING ORG.NAME`,
		`DESC AVAILABLE LISTING listing1`,
		`DESCRIBE AVAILABLE LISTING "L"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAvailableListing,
		``,
		`DESCRIBE AVAILABLE listing1`,
		`DESCRIBE LISTING listing1`,
	)
}

func TestParseDescribeAvailableOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeAvailableOrganizationProfile,
		`DESCRIBE AVAILABLE ORGANIZATION PROFILE p1`,
		`DESC AVAILABLE ORGANIZATION PROFILE p1`,
		`DESCRIBE AVAILABLE ORGANIZATION PROFILE "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeAvailableOrganizationProfile,
		``,
		`DESCRIBE AVAILABLE ORGANIZATION p1`,
		`DESCRIBE ORGANIZATION PROFILE p1`,
	)
}

func TestParseDescribeBackupPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeBackupPolicy,
		`DESCRIBE BACKUP POLICY p1`,
		`DESC BACKUP POLICY db.sch.p1`,
		`DESCRIBE BACKUP POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeBackupPolicy,
		``,
		`DESCRIBE BACKUP p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeBackupSet(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeBackupSet,
		`DESCRIBE BACKUP SET s1`,
		`DESC BACKUP SET db.sch.s1`,
		`DESCRIBE BACKUP SET "S"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeBackupSet,
		``,
		`DESCRIBE BACKUP s1`,
		`DESCRIBE SET s1`,
	)
}

func TestParseDescribeCatalogIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeCatalogIntegration,
		`DESCRIBE CATALOG INTEGRATION ci1`,
		`DESC CATALOG INTEGRATION ci1`,
		`DESCRIBE CATALOG INTEGRATION "CI"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeCatalogIntegration,
		``,
		`DESCRIBE CATALOG ci1`,
		`DESCRIBE INTEGRATION ci1`,
	)
}

func TestParseDescribeComputePool(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeComputePool,
		`DESCRIBE COMPUTE POOL cp1`,
		`DESC COMPUTE POOL cp1`,
		`DESCRIBE COMPUTE POOL "CP"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeComputePool,
		``,
		`DESCRIBE COMPUTE cp1`,
		`DESCRIBE POOL cp1`,
	)
}

func TestParseDescribeConfiguration(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeConfiguration,
		`DESCRIBE CONFIGURATION cfg1`,
		`DESC CONFIGURATION cfg1 IN APPLICATION app1`,
		`DESCRIBE CONFIGURATION "Cfg" IN APPLICATION app1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeConfiguration,
		``,
		`DESCRIBE cfg1`,
		`DESCRIBE CONFIGURATION`,
	)
}

func TestParseDescribeCortexSearchService(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeCortexSearchService,
		`DESCRIBE CORTEX SEARCH SERVICE css1`,
		`DESC CORTEX SEARCH SERVICE db.sch.css1`,
		`DESCRIBE CORTEX SEARCH SERVICE "CSS"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeCortexSearchService,
		``,
		`DESCRIBE CORTEX SEARCH css1`,
		`DESCRIBE SEARCH SERVICE css1`,
	)
}

func TestParseDescribeDatabase(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeDatabase,
		`DESCRIBE DATABASE db1`,
		`DESC DATABASE db1`,
		`DESCRIBE DATABASE "DB"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeDatabase,
		``,
		`DESCRIBE DATABASE`,
		`DESCRIBE db1`,
	)
}

func TestParseDescribeDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeDbtProject,
		`DESCRIBE DBT PROJECT p1`,
		`DESC DBT PROJECT db.sch.p1`,
		`DESCRIBE DBT PROJECT "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeDbtProject,
		``,
		`DESCRIBE DBT p1`,
		`DESCRIBE PROJECT p1`,
	)
}

func TestParseDescribeDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeDcmProject,
		`DESCRIBE DCM PROJECT p1`,
		`DESC DCM PROJECT db.sch.p1`,
		`DESCRIBE DCM PROJECT "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeDcmProject,
		``,
		`DESCRIBE DCM p1`,
		`DESCRIBE PROJECT p1`,
	)
}

func TestParseDescribeDynamicTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeDynamicTable,
		`DESCRIBE DYNAMIC TABLE dt1`,
		`DESC DYNAMIC TABLE db.sch.dt1`,
		`DESCRIBE DYNAMIC TABLE "DT"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeDynamicTable,
		``,
		`DESCRIBE DYNAMIC dt1`,
		`DESCRIBE TABLE dt1`,
	)
}

func TestParseDescribeEventTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeEventTable,
		`DESCRIBE EVENT TABLE et1`,
		`DESC EVENT TABLE db.sch.et1`,
		`DESCRIBE EVENT TABLE "ET"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeEventTable,
		``,
		`DESCRIBE EVENT et1`,
		`DESCRIBE TABLE et1`,
	)
}

func TestParseDescribeExternalAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeExternalAgent,
		`DESCRIBE EXTERNAL AGENT a1`,
		`DESC EXTERNAL AGENT db.sch.a1`,
		`DESCRIBE EXTERNAL AGENT "A"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeExternalAgent,
		``,
		`DESCRIBE EXTERNAL a1`,
		`DESCRIBE AGENT a1`,
	)
}

func TestParseDescribeExternalTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeExternalTable,
		`DESCRIBE EXTERNAL TABLE t1`,
		`DESC TABLE t1 TYPE = COLUMNS`,
		`DESCRIBE EXTERNAL TABLE db.sch.t1 TYPE = STAGE`,
	)
	assertInvalid(t, (*Validator).ParseDescribeExternalTable,
		``,
		`DESCRIBE EXTERNAL t1`,
		`DESCRIBE TABLE t1 TYPE = FOO`,
	)
}

func TestParseDescribeExternalVolume(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeExternalVolume,
		`DESCRIBE EXTERNAL VOLUME ev1`,
		`DESC EXTERNAL VOLUME ev1`,
		`DESCRIBE EXTERNAL VOLUME "EV"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeExternalVolume,
		``,
		`DESCRIBE EXTERNAL ev1`,
		`DESCRIBE VOLUME ev1`,
	)
}

func TestParseDescribeFeaturePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeFeaturePolicy,
		`DESCRIBE FEATURE POLICY p1`,
		`DESC FEATURE POLICY db.sch.p1`,
		`DESCRIBE FEATURE POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeFeaturePolicy,
		``,
		`DESCRIBE FEATURE p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeFileFormat(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeFileFormat,
		`DESCRIBE FILE FORMAT ff1`,
		`DESC FILE FORMAT db.sch.ff1`,
		`DESCRIBE FILE FORMAT "FF"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeFileFormat,
		``,
		`DESCRIBE FILE ff1`,
		`DESCRIBE FORMAT ff1`,
	)
}

func TestParseDescribeFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeFunction,
		`DESCRIBE FUNCTION f1()`,
		`DESC FUNCTION f1(NUMBER, VARCHAR)`,
		`DESCRIBE FUNCTION db.sch.f1(FLOAT)`,
	)
	assertInvalid(t, (*Validator).ParseDescribeFunction,
		``,
		`DESCRIBE FUNCTION f1`,
		`DESCRIBE f1()`,
	)
}

func TestParseDescribeFunctionDmf(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeFunctionDmf,
		`DESCRIBE FUNCTION f1(TABLE(NUMBER))`,
		`DESC FUNCTION f1(TABLE(NUMBER), TABLE(VARCHAR))`,
		`DESCRIBE FUNCTION IF EXISTS db.sch.f1(TABLE(FLOAT))`,
	)
	assertInvalid(t, (*Validator).ParseDescribeFunctionDmf,
		``,
		`DESCRIBE FUNCTION f1`,
		`DESCRIBE f1(TABLE(NUMBER))`,
	)
}

func TestParseDescribeFunctionSnowparkContainerServices(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeFunctionSnowparkContainerServices,
		`DESCRIBE FUNCTION f1()`,
		`DESC FUNCTION IF EXISTS f1(NUMBER)`,
		`DESCRIBE FUNCTION db.sch.f1(VARCHAR, FLOAT)`,
	)
	assertInvalid(t, (*Validator).ParseDescribeFunctionSnowparkContainerServices,
		``,
		`DESCRIBE FUNCTION f1`,
		`DESCRIBE f1()`,
	)
}

func TestParseDescribeGateway(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeGateway,
		`DESCRIBE GATEWAY g1`,
		`DESC GATEWAY db.sch.g1`,
		`DESCRIBE GATEWAY "G"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeGateway,
		``,
		`DESCRIBE GATEWAY`,
		`DESCRIBE g1`,
	)
}

func TestParseDescribeGitRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeGitRepository,
		`DESCRIBE GIT REPOSITORY r1`,
		`DESC GIT REPOSITORY db.sch.r1`,
		`DESCRIBE GIT REPOSITORY "R"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeGitRepository,
		``,
		`DESCRIBE GIT r1`,
		`DESCRIBE REPOSITORY r1`,
	)
}

func TestParseDescribeIcebergTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeIcebergTable,
		`DESCRIBE ICEBERG TABLE t1`,
		`DESC TABLE t1 TYPE = COLUMNS`,
		`DESCRIBE ICEBERG TABLE db.sch.t1 TYPE = STAGE`,
	)
	assertInvalid(t, (*Validator).ParseDescribeIcebergTable,
		``,
		`DESCRIBE ICEBERG t1`,
		`DESCRIBE TABLE t1 TYPE = FOO`,
	)
}

func TestParseDescribeIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeIntegration,
		`DESCRIBE INTEGRATION i1`,
		`DESC API INTEGRATION i1`,
		`DESCRIBE EXTERNAL ACCESS INTEGRATION i1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeIntegration,
		``,
		`DESCRIBE API i1`,
		`DESCRIBE i1`,
	)
}

func TestParseDescribeJoinPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeJoinPolicy,
		`DESCRIBE JOIN POLICY p1`,
		`DESC JOIN POLICY db.sch.p1`,
		`DESCRIBE JOIN POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeJoinPolicy,
		``,
		`DESCRIBE JOIN p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeListing(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeListing,
		`DESCRIBE LISTING l1`,
		`DESC LISTING l1 REVISION = DRAFT`,
		`DESCRIBE LISTING l1 REVISION = PUBLISHED`,
	)
	assertInvalid(t, (*Validator).ParseDescribeListing,
		``,
		`DESCRIBE l1`,
		`DESCRIBE LISTING l1 REVISION = FOO`,
	)
}

func TestParseDescribeMaintenancePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeMaintenancePolicy,
		`DESCRIBE MAINTENANCE POLICY p1`,
		`DESC MAINTENANCE POLICY db.sch.p1`,
		`DESCRIBE MAINTENANCE POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeMaintenancePolicy,
		``,
		`DESCRIBE MAINTENANCE p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeMaskingPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeMaskingPolicy,
		`DESCRIBE MASKING POLICY p1`,
		`DESC MASKING POLICY db.sch.p1`,
		`DESCRIBE MASKING POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeMaskingPolicy,
		``,
		`DESCRIBE MASKING p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeMaterializedView(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeMaterializedView,
		`DESCRIBE MATERIALIZED VIEW v1`,
		`DESC MATERIALIZED VIEW db.sch.v1`,
		`DESCRIBE MATERIALIZED VIEW "V"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeMaterializedView,
		``,
		`DESCRIBE MATERIALIZED v1`,
		`DESCRIBE VIEW v1`,
	)
}

func TestParseDescribeMcpServer(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeMcpServer,
		`DESCRIBE MCP SERVER s1`,
		`DESC MCP SERVER db.sch.s1`,
		`DESCRIBE MCP SERVER "S"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeMcpServer,
		``,
		`DESCRIBE MCP s1`,
		`DESCRIBE SERVER s1`,
	)
}

func TestParseDescribeModelMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeModelMonitor,
		`DESCRIBE MODEL MONITOR m1`,
		`DESC MODEL MONITOR db.sch.m1`,
		`DESCRIBE MODEL MONITOR "M"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeModelMonitor,
		``,
		`DESCRIBE MODEL m1`,
		`DESCRIBE MONITOR m1`,
	)
}

func TestParseDescribeNetworkPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeNetworkPolicy,
		`DESCRIBE NETWORK POLICY p1`,
		`DESC NETWORK POLICY db.sch.p1`,
		`DESCRIBE NETWORK POLICY "P"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeNetworkPolicy,
		``,
		`DESCRIBE NETWORK p1`,
		`DESCRIBE POLICY p1`,
	)
}

func TestParseDescribeNetworkRule(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeNetworkRule,
		`DESCRIBE NETWORK RULE r1`,
		`DESC NETWORK RULE db.sch.r1`,
		`DESCRIBE NETWORK RULE "R"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeNetworkRule,
		``,
		`DESCRIBE NETWORK r1`,
		`DESCRIBE RULE r1`,
	)
}
