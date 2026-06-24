package sqlgrammar

import "testing"

// Tests for the second batch of DESCRIBE grammar rules (issue #556).
// Each test exercises >= 3 realistic valid forms and >= 2 malformed forms.

func TestParseDescribeNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeNotebook,
		`DESCRIBE NOTEBOOK my_nb`,
		`DESC NOTEBOOK db.sch.my_nb`,
		`DESCRIBE NOTEBOOK "My Notebook"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeNotebook,
		``,
		`SHOW NOTEBOOK my_nb`,
		`DESCRIBE NOTEBOOK`,
	)
}

func TestParseDescribeNotificationIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeNotificationIntegration,
		`DESCRIBE NOTIFICATION INTEGRATION my_int`,
		`DESC NOTIFICATION INTEGRATION my_int`,
		`DESCRIBE NOTIFICATION INTEGRATION db.sch.int1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeNotificationIntegration,
		``,
		`DESCRIBE NOTIFICATION my_int`,
		`DESCRIBE INTEGRATION my_int`,
	)
}

func TestParseDescribeOpenflowDataPlaneIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeOpenflowDataPlaneIntegration,
		`DESCRIBE OPENFLOW DATA PLANE INTEGRATION my_int`,
		`DESC OPENFLOW DATA PLANE INTEGRATION my_int`,
		`DESCRIBE OPENFLOW DATA PLANE INTEGRATION db.sch.int1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeOpenflowDataPlaneIntegration,
		``,
		`DESCRIBE OPENFLOW DATA INTEGRATION my_int`,
		`DESCRIBE OPENFLOW DATA PLANE INTEGRATION`,
	)
}

func TestParseDescribeOnlineFeatureTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeOnlineFeatureTable,
		`DESCRIBE ONLINE FEATURE TABLE my_oft`,
		`DESC ONLINE FEATURE TABLE my_oft`,
		`DESCRIBE ONLINE FEATURE TABLE db.sch.oft`,
	)
	assertInvalid(t, (*Validator).ParseDescribeOnlineFeatureTable,
		``,
		`DESCRIBE ONLINE TABLE my_oft`,
		`DESCRIBE FEATURE TABLE my_oft`,
	)
}

func TestParseDescribeOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeOrganizationProfile,
		`DESCRIBE ORGANIZATION PROFILE my_prof`,
		`DESC ORGANIZATION PROFILE my_prof`,
		`DESCRIBE ORGANIZATION PROFILE db.prof`,
	)
	assertInvalid(t, (*Validator).ParseDescribeOrganizationProfile,
		``,
		`DESCRIBE ORGANIZATION my_prof`,
		`DESCRIBE PROFILE my_prof`,
	)
}

func TestParseDescribePackagesPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribePackagesPolicy,
		`DESCRIBE PACKAGES POLICY my_pol`,
		`DESC PACKAGES POLICY my_pol`,
		`DESCRIBE PACKAGES POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribePackagesPolicy,
		``,
		`DESCRIBE PACKAGE POLICY my_pol`,
		`DESCRIBE PACKAGES my_pol`,
	)
}

func TestParseDescribePasswordPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribePasswordPolicy,
		`DESCRIBE PASSWORD POLICY my_pol`,
		`DESC PASSWORD POLICY my_pol`,
		`DESCRIBE PASSWORD POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribePasswordPolicy,
		``,
		`DESCRIBE PASSWORD my_pol`,
		`DESCRIBE POLICY my_pol`,
	)
}

func TestParseDescribePipe(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribePipe,
		`DESCRIBE PIPE my_pipe`,
		`DESC PIPE my_pipe`,
		`DESCRIBE PIPE db.sch.pipe1`,
	)
	assertInvalid(t, (*Validator).ParseDescribePipe,
		``,
		`SHOW PIPE my_pipe`,
		`DESCRIBE PIPE`,
	)
}

func TestParseDescribePostgresInstance(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribePostgresInstance,
		`DESCRIBE POSTGRES INSTANCE my_inst`,
		`DESC POSTGRES INSTANCE my_inst`,
		`DESCRIBE POSTGRES INSTANCE db.inst`,
	)
	assertInvalid(t, (*Validator).ParseDescribePostgresInstance,
		``,
		`DESCRIBE POSTGRES my_inst`,
		`DESCRIBE INSTANCE my_inst`,
	)
}

func TestParseDescribePrivacyPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribePrivacyPolicy,
		`DESCRIBE PRIVACY POLICY my_pol`,
		`DESC PRIVACY POLICY my_pol`,
		`DESCRIBE PRIVACY POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribePrivacyPolicy,
		``,
		`DESCRIBE PRIVACY my_pol`,
		`DESCRIBE POLICY my_pol`,
	)
}

func TestParseDescribeProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeProcedure,
		`DESCRIBE PROCEDURE my_proc()`,
		`DESC PROCEDURE my_proc(NUMBER, VARCHAR)`,
		`DESCRIBE PROCEDURE db.sch.my_proc(FLOAT)`,
	)
	assertInvalid(t, (*Validator).ParseDescribeProcedure,
		``,
		`DESCRIBE PROCEDURE my_proc`,
		`DESCRIBE my_proc()`,
	)
}

func TestParseDescribeProjectionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeProjectionPolicy,
		`DESCRIBE PROJECTION POLICY my_pol`,
		`DESC PROJECTION POLICY my_pol`,
		`DESCRIBE PROJECTION POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribeProjectionPolicy,
		``,
		`DESCRIBE PROJECTION my_pol`,
		`DESCRIBE POLICY my_pol`,
	)
}

func TestParseDescribeResult(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeResult,
		`DESCRIBE RESULT '01a2b3c4-0000-1234-0000-000000000001'`,
		`DESC RESULT LAST_QUERY_ID()`,
		`DESCRIBE RESULT '019283'`,
	)
	assertInvalid(t, (*Validator).ParseDescribeResult,
		``,
		`DESCRIBE RESULT`,
		`SHOW RESULT 'abc'`,
	)
}

func TestParseDescribeRowAccessPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeRowAccessPolicy,
		`DESCRIBE ROW ACCESS POLICY my_pol`,
		`DESC ROW ACCESS POLICY my_pol`,
		`DESCRIBE ROW ACCESS POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribeRowAccessPolicy,
		``,
		`DESCRIBE ROW POLICY my_pol`,
		`DESCRIBE ACCESS POLICY my_pol`,
	)
}

func TestParseDescribeSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSchema,
		`DESCRIBE SCHEMA my_schema`,
		`DESC SCHEMA my_db.my_schema`,
		`DESCRIBE SCHEMA "My Schema"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSchema,
		``,
		`SHOW SCHEMA my_schema`,
		`DESCRIBE SCHEMA`,
	)
}

func TestParseDescribeSearchOptimization(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSearchOptimization,
		`DESCRIBE SEARCH OPTIMIZATION ON my_table`,
		`DESC SEARCH OPTIMIZATION ON db.sch.my_table`,
		`DESCRIBE SEARCH OPTIMIZATION ON "My Table"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSearchOptimization,
		``,
		`DESCRIBE SEARCH OPTIMIZATION my_table`,
		`DESCRIBE SEARCH ON my_table`,
	)
}

func TestParseDescribeSecret(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSecret,
		`DESCRIBE SECRET my_secret`,
		`DESC SECRET my_secret`,
		`DESCRIBE SECRET db.sch.sec1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSecret,
		``,
		`SHOW SECRET my_secret`,
		`DESCRIBE SECRET`,
	)
}

func TestParseDescribeSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSemanticView,
		`DESCRIBE SEMANTIC VIEW my_sv`,
		`DESC SEMANTIC VIEW my_sv`,
		`DESCRIBE SEMANTIC VIEW db.sch.sv`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSemanticView,
		``,
		`DESCRIBE SEMANTIC my_sv`,
		`DESCRIBE VIEW my_sv`,
	)
}

func TestParseDescribeSequence(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSequence,
		`DESCRIBE SEQUENCE my_seq`,
		`DESC SEQUENCE my_seq`,
		`DESCRIBE SEQUENCE db.sch.seq1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSequence,
		``,
		`SHOW SEQUENCE my_seq`,
		`DESCRIBE SEQUENCE`,
	)
}

func TestParseDescribeService(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeService,
		`DESCRIBE SERVICE my_svc`,
		`DESC SERVICE my_svc`,
		`DESCRIBE SERVICE db.sch.svc1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeService,
		``,
		`SHOW SERVICE my_svc`,
		`DESCRIBE SERVICE`,
	)
}

func TestParseDescribeSessionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSessionPolicy,
		`DESCRIBE SESSION POLICY my_pol`,
		`DESC SESSION POLICY my_pol`,
		`DESCRIBE SESSION POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSessionPolicy,
		``,
		`DESCRIBE SESSION my_pol`,
		`DESCRIBE POLICY my_pol`,
	)
}

func TestParseDescribeShare(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeShare,
		`DESCRIBE SHARE my_share`,
		`DESC SHARE my_share`,
		`DESCRIBE SHARE provider_acct.share_name`,
	)
	assertInvalid(t, (*Validator).ParseDescribeShare,
		``,
		`SHOW SHARE my_share`,
		`DESCRIBE SHARE`,
	)
}

func TestParseDescribeSnapshot(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSnapshot,
		`DESCRIBE SNAPSHOT my_snap`,
		`DESC SNAPSHOT my_snap`,
		`DESCRIBE SNAPSHOT db.sch.snap`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSnapshot,
		``,
		`SHOW SNAPSHOT my_snap`,
		`DESCRIBE SNAPSHOT`,
	)
}

func TestParseDescribeSnapshotPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSnapshotPolicy,
		`DESCRIBE SNAPSHOT POLICY my_pol`,
		`DESC SNAPSHOT POLICY my_pol`,
		`DESCRIBE SNAPSHOT POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSnapshotPolicy,
		``,
		`DESCRIBE SNAPSHOT my_pol`,
		`DESCRIBE POLICY my_pol`,
	)
}

func TestParseDescribeSnapshotSet(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSnapshotSet,
		`DESCRIBE SNAPSHOT SET my_set`,
		`DESC SNAPSHOT SET my_set`,
		`DESCRIBE SNAPSHOT SET db.sch.set1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSnapshotSet,
		``,
		`DESCRIBE SNAPSHOT my_set`,
		`DESCRIBE SET my_set`,
	)
}

func TestParseDescribeSpecification(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeSpecification,
		`DESCRIBE SPECIFICATION my_spec`,
		`DESC SPECIFICATION my_spec`,
		`DESCRIBE SPECIFICATION my_spec IN APPLICATION my_app`,
	)
	assertInvalid(t, (*Validator).ParseDescribeSpecification,
		``,
		`DESCRIBE my_spec`,
		`DESCRIBE SPECIFICATION`,
	)
}

func TestParseDescribeStage(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeStage,
		`DESCRIBE STAGE my_stage`,
		`DESC STAGE my_stage`,
		`DESCRIBE STAGE db.sch.stg1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeStage,
		``,
		`SHOW STAGE my_stage`,
		`DESCRIBE STAGE`,
	)
}

func TestParseDescribeStorageLifecyclePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeStorageLifecyclePolicy,
		`DESCRIBE STORAGE LIFECYCLE POLICY my_pol`,
		`DESC STORAGE LIFECYCLE POLICY my_pol`,
		`DESCRIBE STORAGE LIFECYCLE POLICY db.sch.pol`,
	)
	assertInvalid(t, (*Validator).ParseDescribeStorageLifecyclePolicy,
		``,
		`DESCRIBE STORAGE POLICY my_pol`,
		`DESCRIBE LIFECYCLE POLICY my_pol`,
	)
}

func TestParseDescribeStream(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeStream,
		`DESCRIBE STREAM my_stream`,
		`DESC STREAM my_stream`,
		`DESCRIBE STREAM db.sch.strm1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeStream,
		``,
		`SHOW STREAM my_stream`,
		`DESCRIBE STREAM`,
	)
}

func TestParseDescribeStreamlit(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeStreamlit,
		`DESCRIBE STREAMLIT my_app`,
		`DESC STREAMLIT my_app`,
		`DESCRIBE STREAMLIT db.sch.app1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeStreamlit,
		``,
		`SHOW STREAMLIT my_app`,
		`DESCRIBE STREAMLIT`,
	)
}

func TestParseDescribeTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeTable,
		`DESCRIBE TABLE my_table`,
		`DESC TABLE db.sch.my_table`,
		`DESCRIBE TABLE my_table TYPE = COLUMNS`,
	)
	assertInvalid(t, (*Validator).ParseDescribeTable,
		``,
		`SHOW TABLE my_table`,
		`DESCRIBE TABLE`,
	)
}

func TestParseDescribeTask(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeTask,
		`DESCRIBE TASK my_task`,
		`DESC TASK my_task`,
		`DESCRIBE TASK db.sch.task1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeTask,
		``,
		`SHOW TASK my_task`,
		`DESCRIBE TASK`,
	)
}

func TestParseDescribeTransaction(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeTransaction,
		`DESCRIBE TRANSACTION 1234567890`,
		`DESC TRANSACTION 42`,
		`DESCRIBE TRANSACTION 1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeTransaction,
		``,
		`DESCRIBE TRANSACTION`,
		`SHOW TRANSACTION 42`,
	)
}

func TestParseDescribeType(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeType,
		`DESCRIBE TYPE my_type`,
		`DESC TYPE my_type`,
		`DESCRIBE TYPE db.sch.type1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeType,
		``,
		`SHOW TYPE my_type`,
		`DESCRIBE TYPE`,
	)
}

func TestParseDescribeUser(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeUser,
		`DESCRIBE USER my_user`,
		`DESC USER my_user`,
		`DESCRIBE USER "My User"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeUser,
		``,
		`SHOW USER my_user`,
		`DESCRIBE USER`,
	)
}

func TestParseDescribeView(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeView,
		`DESCRIBE VIEW my_view`,
		`DESC VIEW my_view`,
		`DESCRIBE VIEW db.sch.v1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeView,
		``,
		`SHOW VIEW my_view`,
		`DESCRIBE VIEW`,
	)
}

func TestParseDescribeWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeWarehouse,
		`DESCRIBE WAREHOUSE my_wh`,
		`DESC WAREHOUSE my_wh`,
		`DESCRIBE WAREHOUSE "My WH"`,
	)
	assertInvalid(t, (*Validator).ParseDescribeWarehouse,
		``,
		`SHOW WAREHOUSE my_wh`,
		`DESCRIBE WAREHOUSE`,
	)
}

func TestParseDescribeApplicationService(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeApplicationService,
		`DESCRIBE APPLICATION SERVICE my_svc`,
		`DESC APPLICATION SERVICE my_svc`,
		`DESCRIBE APPLICATION SERVICE db.sch.svc1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeApplicationService,
		``,
		`DESCRIBE APPLICATION my_svc`,
		`DESCRIBE SERVICE my_svc`,
	)
}

func TestParseDescribeArtifactRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseDescribeArtifactRepository,
		`DESCRIBE ARTIFACT REPOSITORY my_repo`,
		`DESC ARTIFACT REPOSITORY my_repo`,
		`DESCRIBE ARTIFACT REPOSITORY db.sch.repo1`,
	)
	assertInvalid(t, (*Validator).ParseDescribeArtifactRepository,
		``,
		`DESCRIBE ARTIFACT my_repo`,
		`DESCRIBE REPOSITORY my_repo`,
	)
}
