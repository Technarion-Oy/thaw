package sqlgrammar

import "testing"

func TestParseDropMaterializedView(t *testing.T) {
	assertValid(t, (*Validator).ParseDropMaterializedView,
		`DROP MATERIALIZED VIEW mv1`,
		`DROP MATERIALIZED VIEW IF EXISTS db.sch.mv1`,
		`DROP MATERIALIZED VIEW "My View"`)
	assertInvalid(t, (*Validator).ParseDropMaterializedView,
		``,
		`DROP VIEW mv1`)
}

func TestParseDropMcpServer(t *testing.T) {
	assertValid(t, (*Validator).ParseDropMcpServer,
		`DROP MCP SERVER s1`,
		`DROP MCP SERVER IF EXISTS db.sch.s1`,
		`DROP MCP SERVER "Srv"`)
	assertInvalid(t, (*Validator).ParseDropMcpServer,
		``,
		`DROP MCP s1`)
}

func TestParseDropModel(t *testing.T) {
	assertValid(t, (*Validator).ParseDropModel,
		`DROP MODEL m1`,
		`DROP MODEL IF EXISTS db.sch.m1`,
		`DROP MODEL "My Model"`)
	assertInvalid(t, (*Validator).ParseDropModel,
		``,
		`DROP m1`)
}

func TestParseDropModelMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseDropModelMonitor,
		`DROP MODEL MONITOR mon1`,
		`DROP MODEL MONITOR IF EXISTS db.sch.mon1`,
		`DROP MODEL MONITOR "Mon"`)
	assertInvalid(t, (*Validator).ParseDropModelMonitor,
		``,
		`DROP MODEL mon1`)
}

func TestParseDropNetworkPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropNetworkPolicy,
		`DROP NETWORK POLICY np1`,
		`DROP NETWORK POLICY IF EXISTS np1`,
		`DROP NETWORK POLICY "My Policy"`)
	assertInvalid(t, (*Validator).ParseDropNetworkPolicy,
		``,
		`DROP NETWORK np1`)
}

func TestParseDropNetworkRule(t *testing.T) {
	assertValid(t, (*Validator).ParseDropNetworkRule,
		`DROP NETWORK RULE nr1`,
		`DROP NETWORK RULE IF EXISTS db.sch.nr1`,
		`DROP NETWORK RULE "Rule"`)
	assertInvalid(t, (*Validator).ParseDropNetworkRule,
		``,
		`DROP RULE nr1`)
}

func TestParseDropNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseDropNotebook,
		`DROP NOTEBOOK nb1`,
		`DROP NOTEBOOK IF EXISTS db.sch.nb1`,
		`DROP NOTEBOOK "My Notebook"`)
	assertInvalid(t, (*Validator).ParseDropNotebook,
		``,
		`DROP nb1`)
}

func TestParseDropOnlineFeatureTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropOnlineFeatureTable,
		`DROP ONLINE FEATURE TABLE oft1`,
		`DROP ONLINE FEATURE TABLE IF EXISTS db.sch.oft1`,
		`DROP ONLINE FEATURE TABLE "Tbl"`)
	assertInvalid(t, (*Validator).ParseDropOnlineFeatureTable,
		``,
		`DROP ONLINE TABLE oft1`)
}

func TestParseDropOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseDropOrganizationProfile,
		`DROP ORGANIZATION PROFILE p1`,
		`DROP ORGANIZATION PROFILE IF EXISTS p1`,
		`DROP ORGANIZATION PROFILE "Prof"`)
	assertInvalid(t, (*Validator).ParseDropOrganizationProfile,
		``,
		`DROP PROFILE p1`)
}

func TestParseDropOrganizationUser(t *testing.T) {
	assertValid(t, (*Validator).ParseDropOrganizationUser,
		`DROP ORGANIZATION USER u1`,
		`DROP ORGANIZATION USER IF EXISTS u1`,
		`DROP ORGANIZATION USER "User"`)
	assertInvalid(t, (*Validator).ParseDropOrganizationUser,
		``,
		`DROP USER u1`)
}

func TestParseDropOrganizationUserGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseDropOrganizationUserGroup,
		`DROP ORGANIZATION USER GROUP g1`,
		`DROP ORGANIZATION USER GROUP IF EXISTS g1`,
		`DROP ORGANIZATION USER GROUP "Grp"`)
	assertInvalid(t, (*Validator).ParseDropOrganizationUserGroup,
		``,
		`DROP ORGANIZATION USER g1`)
}

func TestParseDropPackagesPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropPackagesPolicy,
		`DROP PACKAGES POLICY pp1`,
		`DROP PACKAGES POLICY IF EXISTS db.sch.pp1`,
		`DROP PACKAGES POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropPackagesPolicy,
		``,
		`DROP PACKAGES pp1`)
}

func TestParseDropPasswordPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropPasswordPolicy,
		`DROP PASSWORD POLICY pp1`,
		`DROP PASSWORD POLICY IF EXISTS db.sch.pp1`,
		`DROP PASSWORD POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropPasswordPolicy,
		``,
		`DROP PASSWORD pp1`)
}

func TestParseDropPipe(t *testing.T) {
	assertValid(t, (*Validator).ParseDropPipe,
		`DROP PIPE p1`,
		`DROP PIPE IF EXISTS db.sch.p1`,
		`DROP PIPE "Pipe"`)
	assertInvalid(t, (*Validator).ParseDropPipe,
		``,
		`DROP p1`)
}

func TestParseDropPostgresInstance(t *testing.T) {
	assertValid(t, (*Validator).ParseDropPostgresInstance,
		`DROP POSTGRES INSTANCE pi1`,
		`DROP POSTGRES INSTANCE IF EXISTS pi1`,
		`DROP POSTGRES INSTANCE "Inst"`)
	assertInvalid(t, (*Validator).ParseDropPostgresInstance,
		``,
		`DROP POSTGRES pi1`)
}

func TestParseDropPrivacyPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropPrivacyPolicy,
		`DROP PRIVACY POLICY pp1`,
		`DROP PRIVACY POLICY IF EXISTS db.sch.pp1`,
		`DROP PRIVACY POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropPrivacyPolicy,
		``,
		`DROP PRIVACY pp1`)
}

func TestParseDropProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseDropProcedure,
		`DROP PROCEDURE proc1()`,
		`DROP PROCEDURE IF EXISTS db.sch.proc1(NUMBER, VARCHAR)`,
		`DROP PROCEDURE "Proc"(FLOAT)`)
	assertInvalid(t, (*Validator).ParseDropProcedure,
		``,
		`DROP PROCEDURE proc1`)
}

func TestParseDropProjectionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropProjectionPolicy,
		`DROP PROJECTION POLICY pp1`,
		`DROP PROJECTION POLICY IF EXISTS db.sch.pp1`,
		`DROP PROJECTION POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropProjectionPolicy,
		``,
		`DROP PROJECTION pp1`)
}

func TestParseDropReplicationGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseDropReplicationGroup,
		`DROP REPLICATION GROUP rg1`,
		`DROP REPLICATION GROUP IF EXISTS rg1`,
		`DROP REPLICATION GROUP "Grp"`)
	assertInvalid(t, (*Validator).ParseDropReplicationGroup,
		``,
		`DROP REPLICATION rg1`)
}

func TestParseDropResourceMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseDropResourceMonitor,
		`DROP RESOURCE MONITOR rm1`,
		`DROP RESOURCE MONITOR IF EXISTS rm1`,
		`DROP RESOURCE MONITOR "Mon"`)
	assertInvalid(t, (*Validator).ParseDropResourceMonitor,
		``,
		`DROP RESOURCE rm1`)
}

func TestParseDropRole(t *testing.T) {
	assertValid(t, (*Validator).ParseDropRole,
		`DROP ROLE r1`,
		`DROP ROLE IF EXISTS r1`,
		`DROP ROLE "Role"`)
	assertInvalid(t, (*Validator).ParseDropRole,
		``,
		`DROP r1`)
}

func TestParseDropRowAccessPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropRowAccessPolicy,
		`DROP ROW ACCESS POLICY rap1`,
		`DROP ROW ACCESS POLICY IF EXISTS db.sch.rap1`,
		`DROP ROW ACCESS POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropRowAccessPolicy,
		``,
		`DROP ROW POLICY rap1`)
}

func TestParseDropSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSchema,
		`DROP SCHEMA s1`,
		`DROP SCHEMA IF EXISTS db.s1 CASCADE`,
		`DROP SCHEMA s1 RESTRICT`)
	assertInvalid(t, (*Validator).ParseDropSchema,
		``,
		`DROP s1`)
}

func TestParseDropSecret(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSecret,
		`DROP SECRET sec1`,
		`DROP SECRET IF EXISTS db.sch.sec1`,
		`DROP SECRET "Sec"`)
	assertInvalid(t, (*Validator).ParseDropSecret,
		``,
		`DROP sec1`)
}

func TestParseDropSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSemanticView,
		`DROP SEMANTIC VIEW sv1`,
		`DROP SEMANTIC VIEW IF EXISTS db.sch.sv1`,
		`DROP SEMANTIC VIEW "View"`)
	assertInvalid(t, (*Validator).ParseDropSemanticView,
		``,
		`DROP SEMANTIC sv1`)
}

func TestParseDropSequence(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSequence,
		`DROP SEQUENCE seq1`,
		`DROP SEQUENCE IF EXISTS db.sch.seq1 CASCADE`,
		`DROP SEQUENCE seq1 RESTRICT`)
	assertInvalid(t, (*Validator).ParseDropSequence,
		``,
		`DROP seq1`)
}

func TestParseDropService(t *testing.T) {
	assertValid(t, (*Validator).ParseDropService,
		`DROP SERVICE svc1`,
		`DROP SERVICE IF EXISTS db.sch.svc1 FORCE`,
		`DROP SERVICE "Svc"`)
	assertInvalid(t, (*Validator).ParseDropService,
		``,
		`DROP svc1`)
}

func TestParseDropSessionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSessionPolicy,
		`DROP SESSION POLICY sp1`,
		`DROP SESSION POLICY IF EXISTS db.sch.sp1`,
		`DROP SESSION POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropSessionPolicy,
		``,
		`DROP SESSION sp1`)
}

func TestParseDropShare(t *testing.T) {
	assertValid(t, (*Validator).ParseDropShare,
		`DROP SHARE sh1`,
		`DROP SHARE IF EXISTS sh1`,
		`DROP SHARE "Share"`)
	assertInvalid(t, (*Validator).ParseDropShare,
		``,
		`DROP sh1`)
}

func TestParseDropSnapshot(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSnapshot,
		`DROP SNAPSHOT snap1`,
		`DROP SNAPSHOT IF EXISTS db.sch.snap1`,
		`DROP SNAPSHOT "Snap"`)
	assertInvalid(t, (*Validator).ParseDropSnapshot,
		``,
		`DROP snap1`)
}

func TestParseDropSnapshotPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSnapshotPolicy,
		`DROP SNAPSHOT POLICY sp1`,
		`DROP SNAPSHOT POLICY IF EXISTS db.sch.sp1`,
		`DROP SNAPSHOT POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropSnapshotPolicy,
		``,
		`DROP SNAPSHOT sp1`)
}

func TestParseDropSnapshotSet(t *testing.T) {
	assertValid(t, (*Validator).ParseDropSnapshotSet,
		`DROP SNAPSHOT SET ss1`,
		`DROP SNAPSHOT SET IF EXISTS db.sch.ss1`,
		`DROP SNAPSHOT SET "Set"`)
	assertInvalid(t, (*Validator).ParseDropSnapshotSet,
		``,
		`DROP SNAPSHOT ss1`)
}

func TestParseDropStage(t *testing.T) {
	assertValid(t, (*Validator).ParseDropStage,
		`DROP STAGE st1`,
		`DROP STAGE IF EXISTS db.sch.st1`,
		`DROP STAGE "Stage"`)
	assertInvalid(t, (*Validator).ParseDropStage,
		``,
		`DROP st1`)
}

func TestParseDropStorageLifecyclePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseDropStorageLifecyclePolicy,
		`DROP STORAGE LIFECYCLE POLICY slp1`,
		`DROP STORAGE LIFECYCLE POLICY IF EXISTS db.sch.slp1`,
		`DROP STORAGE LIFECYCLE POLICY "Pol"`)
	assertInvalid(t, (*Validator).ParseDropStorageLifecyclePolicy,
		``,
		`DROP STORAGE POLICY slp1`)
}

func TestParseDropStream(t *testing.T) {
	assertValid(t, (*Validator).ParseDropStream,
		`DROP STREAM strm1`,
		`DROP STREAM IF EXISTS db.sch.strm1`,
		`DROP STREAM "Stream"`)
	assertInvalid(t, (*Validator).ParseDropStream,
		``,
		`DROP strm1`)
}

func TestParseDropStreamlit(t *testing.T) {
	assertValid(t, (*Validator).ParseDropStreamlit,
		`DROP STREAMLIT app1`,
		`DROP STREAMLIT IF EXISTS db.sch.app1`,
		`DROP STREAMLIT "App"`)
	assertInvalid(t, (*Validator).ParseDropStreamlit,
		``,
		`DROP app1`)
}

func TestParseDropTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropTable,
		`DROP TABLE t1`,
		`DROP TABLE IF EXISTS db.sch.t1 CASCADE`,
		`DROP TABLE t1 RESTRICT`)
	assertInvalid(t, (*Validator).ParseDropTable,
		``,
		`DROP t1`)
}

func TestParseDropTag(t *testing.T) {
	assertValid(t, (*Validator).ParseDropTag,
		`DROP TAG tag1`,
		`DROP TAG IF EXISTS db.sch.tag1`,
		`DROP TAG "Tag"`)
	assertInvalid(t, (*Validator).ParseDropTag,
		``,
		`DROP tag1`)
}

func TestParseDropTask(t *testing.T) {
	assertValid(t, (*Validator).ParseDropTask,
		`DROP TASK task1`,
		`DROP TASK IF EXISTS db.sch.task1`,
		`DROP TASK "Task"`)
	assertInvalid(t, (*Validator).ParseDropTask,
		``,
		`DROP task1`)
}

func TestParseDropType(t *testing.T) {
	assertValid(t, (*Validator).ParseDropType,
		`DROP TYPE ty1`,
		`DROP TYPE IF EXISTS db.sch.ty1`,
		`DROP TYPE "Type"`)
	assertInvalid(t, (*Validator).ParseDropType,
		``,
		`DROP ty1`)
}

func TestParseDropUser(t *testing.T) {
	assertValid(t, (*Validator).ParseDropUser,
		`DROP USER u1`,
		`DROP USER IF EXISTS u1`,
		`DROP USER "User"`)
	assertInvalid(t, (*Validator).ParseDropUser,
		``,
		`DROP u1`)
}

func TestParseDropView(t *testing.T) {
	assertValid(t, (*Validator).ParseDropView,
		`DROP VIEW v1`,
		`DROP VIEW IF EXISTS db.sch.v1`,
		`DROP VIEW "View"`)
	assertInvalid(t, (*Validator).ParseDropView,
		``,
		`DROP v1`)
}

func TestParseDropWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseDropWarehouse,
		`DROP WAREHOUSE wh1`,
		`DROP WAREHOUSE IF EXISTS wh1`,
		`DROP WAREHOUSE "WH"`)
	assertInvalid(t, (*Validator).ParseDropWarehouse,
		``,
		`DROP wh1`)
}

func TestParseDropApplicationService(t *testing.T) {
	assertValid(t, (*Validator).ParseDropApplicationService,
		`DROP APPLICATION SERVICE as1`,
		`DROP APPLICATION SERVICE IF EXISTS db.sch.as1`,
		`DROP APPLICATION SERVICE "Svc"`)
	assertInvalid(t, (*Validator).ParseDropApplicationService,
		``,
		`DROP APPLICATION as1`)
}

func TestParseDropArtifactRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseDropArtifactRepository,
		`DROP ARTIFACT REPOSITORY ar1`,
		`DROP ARTIFACT REPOSITORY IF EXISTS db.sch.ar1`,
		`DROP ARTIFACT REPOSITORY "Repo"`)
	assertInvalid(t, (*Validator).ParseDropArtifactRepository,
		``,
		`DROP ARTIFACT ar1`)
}

func TestParseDropEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseDropEventRoutingTable,
		`DROP EVENT ROUTING TABLE ert1`,
		`DROP EVENT ROUTING TABLE IF EXISTS db.sch.ert1`,
		`DROP EVENT ROUTING TABLE "Tbl"`)
	assertInvalid(t, (*Validator).ParseDropEventRoutingTable,
		``,
		`DROP EVENT TABLE ert1`)
}
