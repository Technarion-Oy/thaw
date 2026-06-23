package sqlgrammar

import "testing"

// Tests for the first batch of ALTER grammar rules (ParseAlterObj …
// ParseAlterDbtProject) implemented for issue #556.

func TestParseAlterObj(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterObj,
		`ALTER TABLE t1 RENAME TO t2`,
		`ALTER WAREHOUSE wh SET WAREHOUSE_SIZE = 'LARGE'`,
		`ALTER VIEW v1 SET COMMENT = 'hi'`,
	)
	assertInvalid(t, (*Validator).ParseAlterObj,
		``,
		`SELECT 1`,
		`ALTER`,
	)
}

func TestParseAlterAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterAccount,
		`ALTER ACCOUNT SET RESOURCE_MONITOR = mon1`,
		`ALTER ACCOUNT UNSET TIMEZONE`,
		`ALTER ACCOUNT myacct RENAME TO newacct`,
		`ALTER ACCOUNT ADD ORGANIZATION USER GROUP g1`,
	)
	assertInvalid(t, (*Validator).ParseAlterAccount,
		``,
		`DROP ACCOUNT a`,
		`ALTER ACCOUNT`,
	)
}

func TestParseAlterAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterAgent,
		`ALTER AGENT a1 SET COMMENT = 'x'`,
		`ALTER AGENT a1 SET COMMENT = 'x' PROFILE = 'p'`,
		`ALTER AGENT a1 MODIFY LIVE VERSION SET SPECIFICATION = spec`,
	)
	assertInvalid(t, (*Validator).ParseAlterAgent,
		``,
		`ALTER AGENT a1`,
		`ALTER AGENT a1 FOO`,
	)
}

func TestParseAlterAggregationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterAggregationPolicy,
		`ALTER AGGREGATION POLICY p1 RENAME TO p2`,
		`ALTER AGGREGATION POLICY IF EXISTS p1 SET COMMENT = 'x'`,
		`ALTER AGGREGATION POLICY p1 UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterAggregationPolicy,
		``,
		`ALTER AGGREGATION POLICY p1`,
		`ALTER POLICY p1 SET COMMENT = 'x'`,
	)
}

func TestParseAlterAlert(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterAlert,
		`ALTER ALERT a1 RESUME`,
		`ALTER ALERT IF EXISTS a1 SET WAREHOUSE = 'wh'`,
		`ALTER ALERT a1 MODIFY ACTION foo`,
	)
	assertInvalid(t, (*Validator).ParseAlterAlert,
		``,
		`ALTER ALERT a1`,
		`ALTER ALERT a1 FOO`,
	)
}

func TestParseAlterApiIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApiIntegration,
		`ALTER API INTEGRATION i1 SET ENABLED = TRUE`,
		`ALTER INTEGRATION IF EXISTS i1 UNSET COMMENT`,
		`ALTER API INTEGRATION i1 SET TAG t1 = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterApiIntegration,
		``,
		`ALTER API INTEGRATION i1`,
		`ALTER API i1 SET ENABLED = TRUE`,
	)
}

func TestParseAlterApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplication,
		`ALTER APPLICATION app1 RENAME TO app2`,
		`ALTER APPLICATION IF EXISTS app1 SET COMMENT = 'x'`,
		`ALTER APPLICATION app1 UPGRADE`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplication,
		``,
		`ALTER APPLICATION app1`,
		`ALTER APPLICATION app1 FOO`,
	)
}

func TestParseAlterApplicationDropSpecification(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationDropSpecification,
		`ALTER APPLICATION DROP SPECIFICATION spec1`,
		`ALTER APPLICATION DROP SPECIFICATION db.sch.spec1`,
		`ALTER APPLICATION DROP SPECIFICATION "MySpec"`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationDropSpecification,
		``,
		`ALTER APPLICATION DROP SPECIFICATION`,
		`ALTER APPLICATION SPECIFICATION spec1`,
	)
}

func TestParseAlterApplicationDropConfigurationDefinition(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationDropConfigurationDefinition,
		`ALTER APPLICATION DROP CONFIGURATION DEFINITION cfg1`,
		`ALTER APPLICATION DROP CONFIGURATION DEFINITION db.cfg1`,
		`ALTER APPLICATION DROP CONFIGURATION DEFINITION "Cfg"`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationDropConfigurationDefinition,
		``,
		`ALTER APPLICATION DROP CONFIGURATION DEFINITION`,
		`ALTER APPLICATION DROP CONFIGURATION cfg1`,
	)
}

func TestParseAlterApplicationPackage(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationPackage,
		`ALTER APPLICATION PACKAGE pkg1 SET DISTRIBUTION = INTERNAL`,
		`ALTER APPLICATION PACKAGE IF EXISTS pkg1 UNSET COMMENT`,
		`ALTER APPLICATION PACKAGE pkg1 SET TAG t1 = 'v'`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationPackage,
		``,
		`ALTER APPLICATION PACKAGE pkg1`,
		`ALTER PACKAGE pkg1 SET DISTRIBUTION = INTERNAL`,
	)
}

func TestParseAlterApplicationPackageModifyReleaseChannel(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationPackageModifyReleaseChannel,
		`ALTER APPLICATION PACKAGE pkg1 MODIFY RELEASE CHANNEL rc1 SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 2`,
		`ALTER APPLICATION PACKAGE pkg1 MODIFY RELEASE CHANNEL rc1 UNSET RELEASE DIRECTIVE d1`,
		`ALTER APPLICATION PACKAGE pkg1 MODIFY RELEASE CHANNEL rc1 MODIFY RELEASE DIRECTIVE d1 VERSION = v1`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationPackageModifyReleaseChannel,
		``,
		`ALTER APPLICATION PACKAGE pkg1 MODIFY RELEASE CHANNEL rc1`,
		`ALTER APPLICATION PACKAGE pkg1 RENAME TO pkg2`,
	)
}

func TestParseAlterApplicationPackageReleaseDirective(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationPackageReleaseDirective,
		`ALTER APPLICATION PACKAGE pkg1 MODIFY RELEASE DIRECTIVE d1 VERSION = v1 PATCH = 2`,
		`ALTER APPLICATION PACKAGE pkg1 SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 2`,
		`ALTER APPLICATION PACKAGE pkg1 UNSET RELEASE DIRECTIVE d1`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationPackageReleaseDirective,
		``,
		`ALTER APPLICATION PACKAGE pkg1`,
		`ALTER APPLICATION pkg1 SET DEFAULT RELEASE DIRECTIVE VERSION = v1`,
	)
}

func TestParseAlterApplicationPackageVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationPackageVersion,
		`ALTER APPLICATION PACKAGE pkg1 ADD VERSION v1 USING '@stage/path'`,
		`ALTER APPLICATION PACKAGE pkg1 DROP VERSION v1`,
		`ALTER APPLICATION PACKAGE pkg1 ADD PATCH 3 FOR VERSION v1 USING '@stage/path'`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationPackageVersion,
		``,
		`ALTER APPLICATION PACKAGE pkg1`,
		`ALTER APPLICATION PACKAGE pkg1 RENAME TO pkg2`,
	)
}

func TestParseAlterApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationRole,
		`ALTER APPLICATION ROLE r1 RENAME TO r2`,
		`ALTER APPLICATION ROLE IF EXISTS r1 SET COMMENT = 'x'`,
		`ALTER APPLICATION ROLE r1 UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationRole,
		``,
		`ALTER APPLICATION ROLE r1`,
		`ALTER ROLE r1 RENAME TO r2`,
	)
}

func TestParseAlterApplicationApproveDeclineSpecification(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationApproveDeclineSpecification,
		`ALTER APPLICATION app1 APPROVE SPECIFICATION spec1 SEQUENCE_NUMBER = 5`,
		`ALTER APPLICATION app1 DECLINE SPECIFICATION spec1 SEQUENCE_NUMBER = 7`,
		`ALTER APPLICATION app1 APPROVE SPECIFICATION db.spec1 SEQUENCE_NUMBER = 1`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationApproveDeclineSpecification,
		``,
		`ALTER APPLICATION app1 APPROVE SPECIFICATION spec1`,
		`ALTER APPLICATION app1 SPECIFICATION spec1 SEQUENCE_NUMBER = 5`,
	)
}

func TestParseAlterApplicationSetSpecification(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationSetSpecification,
		`ALTER APPLICATION SET SPECIFICATION spec1 TYPE = EXTERNAL_ACCESS LABEL = 'l'`,
		`ALTER APPLICATION SET SPECIFICATION spec1 TYPE = LISTING LABEL = 'l' DESCRIPTION = 'd'`,
		`ALTER APPLICATION SET SPECIFICATION spec1 TYPE = SETTING LABEL = 'l'`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationSetSpecification,
		``,
		`ALTER APPLICATION SET SPECIFICATION spec1`,
		`ALTER APPLICATION SET SPECIFICATION spec1 LABEL = 'l'`,
	)
}

func TestParseAlterApplicationSetConfigurationDefinition(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationSetConfigurationDefinition,
		`ALTER APPLICATION SET CONFIGURATION DEFINITION cfg1 TYPE = STRING LABEL = 'l'`,
		`ALTER APPLICATION SET CONFIGURATION DEFINITION cfg1 TYPE = APPLICATION_NAME LABEL = 'l'`,
		`ALTER APPLICATION SET CONFIGURATION DEFINITION cfg1 TYPE = SECRET_AUTHORIZATION SECRET = s.sec`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationSetConfigurationDefinition,
		``,
		`ALTER APPLICATION SET CONFIGURATION DEFINITION cfg1`,
		`ALTER APPLICATION SET CONFIGURATION cfg1 TYPE = STRING`,
	)
}

func TestParseAlterApplicationSetConfigurationValue(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationSetConfigurationValue,
		`ALTER APPLICATION app1 SET CONFIGURATION cfg1 VALUE = 'v'`,
		`ALTER APPLICATION db.app1 SET CONFIGURATION cfg1 VALUE = 'abc'`,
		`ALTER APPLICATION app1 SET CONFIGURATION db.cfg1 VALUE = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationSetConfigurationValue,
		``,
		`ALTER APPLICATION app1 SET CONFIGURATION cfg1`,
		`ALTER APPLICATION app1 SET CONFIGURATION cfg1 VALUE = 5`,
	)
}

func TestParseAlterApplicationUnsetConfiguration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationUnsetConfiguration,
		`ALTER APPLICATION app1 UNSET CONFIGURATION cfg1`,
		`ALTER APPLICATION db.app1 UNSET CONFIGURATION cfg1`,
		`ALTER APPLICATION app1 UNSET CONFIGURATION db.cfg1`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationUnsetConfiguration,
		``,
		`ALTER APPLICATION app1 UNSET CONFIGURATION`,
		`ALTER APPLICATION app1 SET CONFIGURATION cfg1`,
	)
}

func TestParseAlterAuthenticationPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterAuthenticationPolicy,
		`ALTER AUTHENTICATION POLICY p1 RENAME TO p2`,
		`ALTER AUTHENTICATION POLICY IF EXISTS p1 SET COMMENT = 'x'`,
		`ALTER AUTHENTICATION POLICY p1 UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterAuthenticationPolicy,
		``,
		`ALTER AUTHENTICATION POLICY p1`,
		`ALTER POLICY p1 RENAME TO p2`,
	)
}

func TestParseAlterBackupPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterBackupPolicy,
		`ALTER BACKUP POLICY p1 RENAME TO p2`,
		`ALTER BACKUP POLICY p1 SET EXPIRE_AFTER_DAYS = 30`,
		`ALTER BACKUP POLICY p1 UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterBackupPolicy,
		``,
		`ALTER BACKUP POLICY p1`,
		`ALTER POLICY p1 RENAME TO p2`,
	)
}

func TestParseAlterBackupSet(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterBackupSet,
		`ALTER BACKUP SET bs1 ADD BACKUP`,
		`ALTER BACKUP SET bs1 RENAME TO bs2`,
		`ALTER BACKUP SET bs1 DELETE BACKUP IDENTIFIER 'id'`,
	)
	assertInvalid(t, (*Validator).ParseAlterBackupSet,
		``,
		`ALTER BACKUP SET bs1`,
		`ALTER SET bs1 ADD BACKUP`,
	)
}

func TestParseAlterCatalogIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterCatalogIntegration,
		`ALTER CATALOG INTEGRATION c1 SET REFRESH_INTERVAL_SECONDS = 60`,
		`ALTER CATALOG INTEGRATION IF EXISTS c1 SET COMMENT = 'x'`,
		`ALTER CATALOG INTEGRATION c1 SET REST_AUTHENTICATION = ( BEARER_TOKEN = 'tok' )`,
	)
	assertInvalid(t, (*Validator).ParseAlterCatalogIntegration,
		``,
		`ALTER CATALOG INTEGRATION c1`,
		`ALTER INTEGRATION c1 SET COMMENT = 'x'`,
	)
}

func TestParseAlterComputePool(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterComputePool,
		`ALTER COMPUTE POOL cp1 SUSPEND`,
		`ALTER COMPUTE POOL IF EXISTS cp1 SET MIN_NODES = 1`,
		`ALTER COMPUTE POOL cp1 STOP ALL`,
	)
	assertInvalid(t, (*Validator).ParseAlterComputePool,
		``,
		`ALTER COMPUTE POOL cp1`,
		`ALTER POOL cp1 SUSPEND`,
	)
}

func TestParseAlterConnection(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterConnection,
		`ALTER CONNECTION c1 PRIMARY`,
		`ALTER CONNECTION IF EXISTS c1 SET COMMENT = 'x'`,
		`ALTER CONNECTION c1 ENABLE FAILOVER TO ACCOUNTS org.acc`,
	)
	assertInvalid(t, (*Validator).ParseAlterConnection,
		``,
		`ALTER CONNECTION c1`,
		`ALTER CONNECTION c1 FOO`,
	)
}

func TestParseAlterContact(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterContact,
		`ALTER CONTACT c1 RENAME TO c2`,
		`ALTER CONTACT IF EXISTS c1 SET URL = 'http://x'`,
		`ALTER CONTACT c1 SET COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterContact,
		``,
		`ALTER CONTACT c1`,
		`ALTER CONTACT c1 FOO`,
	)
}

func TestParseAlterCortexSearchService(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterCortexSearchService,
		`ALTER CORTEX SEARCH SERVICE s1 SUSPEND`,
		`ALTER CORTEX SEARCH SERVICE IF EXISTS s1 REFRESH`,
		`ALTER CORTEX SEARCH SERVICE s1 SET WAREHOUSE = wh`,
	)
	assertInvalid(t, (*Validator).ParseAlterCortexSearchService,
		``,
		`ALTER CORTEX SEARCH SERVICE s1`,
		`ALTER SEARCH SERVICE s1 SUSPEND`,
	)
}

func TestParseAlterDatabase(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDatabase,
		`ALTER DATABASE db1 RENAME TO db2`,
		`ALTER DATABASE IF EXISTS db1 SET DATA_RETENTION_TIME_IN_DAYS = 5`,
		`ALTER DATABASE db1 SWAP WITH db2`,
	)
	assertInvalid(t, (*Validator).ParseAlterDatabase,
		``,
		`ALTER DATABASE db1`,
		`ALTER DB db1 RENAME TO db2`,
	)
}

func TestParseAlterDatabaseCatalogLinked(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDatabaseCatalogLinked,
		`ALTER DATABASE db1 RENAME TO db2`,
		`ALTER DATABASE IF EXISTS db1 SUSPEND DISCOVERY`,
		`ALTER DATABASE db1 UPDATE LINKED_CATALOG SET SYNC_INTERVAL_SECONDS = 60`,
	)
	assertInvalid(t, (*Validator).ParseAlterDatabaseCatalogLinked,
		``,
		`ALTER DATABASE db1`,
		`ALTER DATABASE db1 SUSPEND`,
	)
}

func TestParseAlterDatabaseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDatabaseRole,
		`ALTER DATABASE ROLE r1 RENAME TO r2`,
		`ALTER DATABASE ROLE IF EXISTS r1 SET COMMENT = 'x'`,
		`ALTER DATABASE ROLE r1 UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterDatabaseRole,
		``,
		`ALTER DATABASE ROLE r1`,
		`ALTER ROLE r1 RENAME TO r2`,
	)
}

func TestParseAlterDataset(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDataset,
		`ALTER DATASET d1 SET COMMENT = 'x'`,
		`ALTER DATASET IF EXISTS d1 RENAME TO d2`,
		`ALTER DATASET d1 DROP VERSION v1`,
	)
	assertInvalid(t, (*Validator).ParseAlterDataset,
		``,
		`ALTER DATASET d1`,
		`ALTER DATASETS d1 SET COMMENT = 'x'`,
	)
}

func TestParseAlterDatasetAddVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDatasetAddVersion,
		`ALTER DATASET d1 ADD VERSION v1 FROM SELECT 1`,
		`ALTER DATASET d1 ADD VERSION v1 FROM SELECT * FROM t COMMENT = 'x'`,
		`ALTER DATASET db.d1 ADD VERSION v2 FROM SELECT a, b FROM t`,
	)
	assertInvalid(t, (*Validator).ParseAlterDatasetAddVersion,
		``,
		`ALTER DATASET d1 ADD VERSION v1`,
		`ALTER DATASET d1 DROP VERSION v1`,
	)
}

func TestParseAlterDatasetDropVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDatasetDropVersion,
		`ALTER DATASET d1 DROP VERSION v1`,
		`ALTER DATASET IF EXISTS d1 DROP VERSION v1`,
		`ALTER DATASET db.d1 DROP VERSION v2`,
	)
	assertInvalid(t, (*Validator).ParseAlterDatasetDropVersion,
		``,
		`ALTER DATASET d1 DROP VERSION`,
		`ALTER DATASET d1 ADD VERSION v1`,
	)
}

func TestParseAlterDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDbtProject,
		`ALTER DBT PROJECT p1 RENAME TO p2`,
		`ALTER DBT PROJECT IF EXISTS p1 SET DBT_VERSION = '1.7'`,
		`ALTER DBT PROJECT p1 ADD VERSION v1 FROM '@stage/path'`,
	)
	assertInvalid(t, (*Validator).ParseAlterDbtProject,
		``,
		`ALTER DBT PROJECT p1`,
		`ALTER PROJECT p1 RENAME TO p2`,
	)
}
