package sqlgrammar

import "testing"

func TestParseAlterSequence(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSequence,
		`ALTER SEQUENCE my_seq RENAME TO my_seq2`,
		`ALTER SEQUENCE IF EXISTS db.sch.s SET INCREMENT BY 5`,
		`ALTER SEQUENCE s SET ORDER COMMENT = 'hi'`,
		`ALTER SEQUENCE s UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSequence,
		``,
		`DROP SEQUENCE s`,
		`ALTER SEQUENCE`,
	)
}

func TestParseAlterService(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterService,
		`ALTER SERVICE svc SUSPEND`,
		`ALTER SERVICE IF EXISTS svc RESUME`,
		`ALTER SERVICE svc SET MIN_INSTANCES = 1 MAX_INSTANCES = 4`,
		`ALTER SERVICE svc FROM SPECIFICATION_FILE = 'spec.yaml'`,
	)
	assertInvalid(t, (*Validator).ParseAlterService,
		``,
		`ALTER SERVICE`,
		`CREATE SERVICE svc SUSPEND`,
	)
}

func TestParseAlterSession(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSession,
		`ALTER SESSION SET AUTOCOMMIT = TRUE`,
		`ALTER SESSION SET QUERY_TAG = 'tag' LOCK_TIMEOUT = 60`,
		`ALTER SESSION UNSET AUTOCOMMIT`,
		`ALTER SESSION UNSET QUERY_TAG, LOCK_TIMEOUT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSession,
		``,
		`ALTER SESSION`,
		`ALTER SESSION FOO`,
	)
}

func TestParseAlterSessionPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSessionPolicy,
		`ALTER SESSION POLICY sp RENAME TO sp2`,
		`ALTER SESSION POLICY IF EXISTS sp SET SESSION_IDLE_TIMEOUT_MINS = 30`,
		`ALTER SESSION POLICY sp UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterSessionPolicy,
		``,
		`ALTER SESSION sp RENAME TO sp2`,
	)
}

func TestParseAlterShare(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterShare,
		`ALTER SHARE s ADD ACCOUNTS = acct1`,
		`ALTER SHARE IF EXISTS s SET COMMENT = 'x'`,
		`ALTER SHARE s UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterShare,
		``,
		`ALTER SHARE`,
	)
}

func TestParseAlterSnapshot(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSnapshot,
		`ALTER SNAPSHOT s SET COMMENT = 'hi'`,
		`ALTER SNAPSHOT IF EXISTS db.sch.s SET COMMENT = 'desc'`,
		`ALTER SNAPSHOT snap1 SET COMMENT = ''`,
	)
	assertInvalid(t, (*Validator).ParseAlterSnapshot,
		``,
		`ALTER SNAPSHOT s SET COMMENT`,
		`ALTER SNAPSHOT s UNSET COMMENT`,
	)
}

func TestParseAlterSnapshotPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSnapshotPolicy,
		`ALTER SNAPSHOT POLICY p RENAME TO p2`,
		`ALTER SNAPSHOT POLICY p SET COMMENT = 'x'`,
		`ALTER SNAPSHOT POLICY p UNSET SCHEDULE`,
	)
	assertInvalid(t, (*Validator).ParseAlterSnapshotPolicy,
		``,
		`ALTER SNAPSHOT p RENAME TO p2`,
	)
}

func TestParseAlterSnapshotSet(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterSnapshotSet,
		`ALTER SNAPSHOT SET ss ADD SNAPSHOT`,
		`ALTER SNAPSHOT SET ss APPLY SNAPSHOT POLICY p FORCE`,
		`ALTER SNAPSHOT SET ss DELETE SNAPSHOT IDENTIFIER 'id1'`,
		`ALTER SNAPSHOT SET ss SET COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterSnapshotSet,
		``,
		`ALTER SNAPSHOT ss ADD SNAPSHOT`,
	)
}

func TestParseAlterStage(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterStage,
		`ALTER STAGE st RENAME TO st2`,
		`ALTER STAGE IF EXISTS st REFRESH`,
		`ALTER STAGE st REFRESH SUBPATH = 'a/b'`,
		`ALTER STAGE st SET COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterStage,
		``,
		`ALTER STAGE`,
	)
}

func TestParseAlterStorageIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterStorageIntegration,
		`ALTER STORAGE INTEGRATION si SET ENABLED = TRUE`,
		`ALTER INTEGRATION IF EXISTS si SET COMMENT = 'x'`,
		`ALTER STORAGE INTEGRATION si UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterStorageIntegration,
		``,
		`ALTER STORAGE si SET ENABLED = TRUE`,
	)
}

func TestParseAlterStorageLifecyclePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterStorageLifecyclePolicy,
		`ALTER STORAGE LIFECYCLE POLICY p RENAME TO p2`,
		`ALTER STORAGE LIFECYCLE POLICY IF EXISTS p SET ARCHIVE_FOR_DAYS = 30`,
		`ALTER STORAGE LIFECYCLE POLICY p UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterStorageLifecyclePolicy,
		``,
		`ALTER STORAGE POLICY p RENAME TO p2`,
	)
}

func TestParseAlterStream(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterStream,
		`ALTER STREAM s SET COMMENT = 'x'`,
		`ALTER STREAM IF EXISTS s UNSET COMMENT`,
		`ALTER STREAM s UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterStream,
		``,
		`ALTER STREAM`,
	)
}

func TestParseAlterStreamlit(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterStreamlit,
		`ALTER STREAMLIT app RENAME TO app2`,
		`ALTER STREAMLIT IF EXISTS app SET MAIN_FILE = 'main.py'`,
		`ALTER STREAMLIT app COMMIT`,
		`ALTER STREAMLIT app ADD LIVE VERSION FROM LAST`,
	)
	assertInvalid(t, (*Validator).ParseAlterStreamlit,
		``,
		`ALTER STREAMLIT`,
	)
}

func TestParseAlterTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterTable,
		`ALTER TABLE t RENAME TO t2`,
		`ALTER TABLE IF EXISTS t SWAP WITH t2`,
		`ALTER TABLE t SET DATA_RETENTION_TIME_IN_DAYS = 1`,
		`ALTER TABLE t ADD COLUMN c INT`,
	)
	assertInvalid(t, (*Validator).ParseAlterTable,
		``,
		`ALTER TABLE`,
		`ALTER TABLE t`,
	)
}

func TestParseAlterTableAlterColumn(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterTableAlterColumn,
		`ALTER TABLE t ALTER COLUMN c DROP DEFAULT`,
		`ALTER TABLE t MODIFY COLUMN c SET NOT NULL`,
		`ALTER TABLE IF EXISTS t ALTER c COMMENT 'note'`,
	)
	assertInvalid(t, (*Validator).ParseAlterTableAlterColumn,
		``,
		`ALTER TABLE t ALTER`,
	)
}

func TestParseAlterTableEventTables(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterTableEventTables,
		`ALTER TABLE t RENAME TO t2`,
		`ALTER TABLE IF EXISTS t SET CHANGE_TRACKING = TRUE`,
		`ALTER TABLE t CLUSTER BY ( a, b )`,
	)
	assertInvalid(t, (*Validator).ParseAlterTableEventTables,
		``,
		`ALTER TABLE t`,
	)
}

func TestParseAlterTag(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterTag,
		`ALTER TAG tg RENAME TO tg2`,
		`ALTER TAG IF EXISTS tg ADD ALLOWED_VALUES 'a', 'b'`,
		`ALTER TAG tg SET COMMENT = 'x'`,
		`ALTER TAG tg UNSET ALLOWED_VALUES`,
	)
	assertInvalid(t, (*Validator).ParseAlterTag,
		``,
		`ALTER TAG`,
	)
}

func TestParseAlterTask(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterTask,
		`ALTER TASK tk RESUME`,
		`ALTER TASK IF EXISTS tk SET WAREHOUSE = 'wh'`,
		`ALTER TASK tk MODIFY WHEN TRUE`,
		`ALTER TASK tk REMOVE WHEN`,
	)
	assertInvalid(t, (*Validator).ParseAlterTask,
		``,
		`ALTER TASK`,
	)
}

func TestParseAlterType(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterType,
		`ALTER TYPE ty SET COMMENT = 'x'`,
		`ALTER TYPE IF EXISTS ty UNSET COMMENT`,
		`ALTER TYPE db.sch.ty SET COMMENT = 'desc'`,
	)
	assertInvalid(t, (*Validator).ParseAlterType,
		``,
		`ALTER TYPE ty DROP COMMENT`,
	)
}

func TestParseAlterUser(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterUser,
		`ALTER USER u RENAME TO u2`,
		`ALTER USER IF EXISTS u SET DEFAULT_ROLE = r1`,
		`ALTER USER u RESET PASSWORD`,
		`ALTER USER u ABORT ALL QUERIES`,
	)
	assertInvalid(t, (*Validator).ParseAlterUser,
		``,
		`ALTER USER`,
		`CREATE USER u RENAME TO u2`,
	)
}

func TestParseAlterUserAddProgrammaticAccessToken(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterUserAddProgrammaticAccessToken,
		`ALTER USER u ADD PROGRAMMATIC ACCESS TOKEN tok`,
		`ALTER USER u ADD PAT tok DAYS_TO_EXPIRY = 30`,
		`ALTER USER IF EXISTS ADD PAT tok`,
	)
	assertInvalid(t, (*Validator).ParseAlterUserAddProgrammaticAccessToken,
		``,
		`ALTER USER u ADD TOKEN tok`,
	)
}

func TestParseAlterUserModifyProgrammaticAccessToken(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterUserModifyProgrammaticAccessToken,
		`ALTER USER u MODIFY PROGRAMMATIC ACCESS TOKEN tok RENAME TO tok2`,
		`ALTER USER u MODIFY PAT tok SET DISABLED = TRUE`,
		`ALTER USER u MODIFY PAT tok UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterUserModifyProgrammaticAccessToken,
		``,
		`ALTER USER u MODIFY PAT tok`,
	)
}

func TestParseAlterUserRemoveProgrammaticAccessToken(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterUserRemoveProgrammaticAccessToken,
		`ALTER USER u REMOVE PROGRAMMATIC ACCESS TOKEN tok`,
		`ALTER USER u REMOVE PAT tok`,
		`ALTER USER IF EXISTS REMOVE PAT tok`,
	)
	assertInvalid(t, (*Validator).ParseAlterUserRemoveProgrammaticAccessToken,
		``,
		`ALTER USER u REMOVE PAT`,
	)
}

func TestParseAlterUserRotateProgrammaticAccessToken(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterUserRotateProgrammaticAccessToken,
		`ALTER USER u ROTATE PROGRAMMATIC ACCESS TOKEN tok`,
		`ALTER USER u ROTATE PAT tok EXPIRE_ROTATED_TOKEN_AFTER_HOURS = 24`,
		`ALTER USER IF EXISTS ROTATE PAT tok`,
	)
	assertInvalid(t, (*Validator).ParseAlterUserRotateProgrammaticAccessToken,
		``,
		`ALTER USER u ROTATE PAT`,
	)
}

func TestParseAlterView(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterView,
		`ALTER VIEW vw RENAME TO vw2`,
		`ALTER VIEW IF EXISTS vw SET SECURE`,
		`ALTER VIEW vw UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterView,
		``,
		`ALTER VIEW`,
	)
}

func TestParseAlterWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterWarehouse,
		`ALTER WAREHOUSE wh SUSPEND`,
		`ALTER WAREHOUSE IF EXISTS wh RESUME IF SUSPENDED`,
		`ALTER WAREHOUSE wh SET WAREHOUSE_SIZE = LARGE`,
		`ALTER WAREHOUSE wh ADD TABLES ( t1, t2 )`,
	)
	assertInvalid(t, (*Validator).ParseAlterWarehouse,
		``,
		`CREATE WAREHOUSE wh SUSPEND`,
	)
}

func TestParseAlterApplicationService(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterApplicationService,
		`ALTER APPLICATION SERVICE svc SUSPEND`,
		`ALTER APPLICATION SERVICE IF EXISTS svc UPGRADE TO VERSION v1`,
		`ALTER APPLICATION SERVICE svc SET AUTO_RESUME = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseAlterApplicationService,
		``,
		`ALTER SERVICE svc SUSPEND`,
	)
}

func TestParseAlterArtifactRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterArtifactRepository,
		`ALTER ARTIFACT REPOSITORY ar SET COMMENT = 'x'`,
		`ALTER ARTIFACT REPOSITORY IF EXISTS ar UNSET COMMENT`,
		`ALTER ARTIFACT REPOSITORY ar UNSET TAG t1`,
	)
	assertInvalid(t, (*Validator).ParseAlterArtifactRepository,
		``,
		`ALTER REPOSITORY ar SET COMMENT = 'x'`,
	)
}

func TestParseAlterEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterEventRoutingTable,
		`ALTER EVENT ROUTING TABLE ert RENAME TO ert2`,
		`ALTER EVENT ROUTING TABLE ert FORCE SET RULE r1 REGION_GROUP = pubic`,
		`ALTER EVENT ROUTING TABLE ert MODIFY RULE r1 RENAME TO r2`,
	)
	assertInvalid(t, (*Validator).ParseAlterEventRoutingTable,
		``,
		`ALTER ROUTING TABLE ert RENAME TO ert2`,
	)
}

func TestParseAlterOrganizationSetEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationSetEventRoutingTable,
		`ALTER ORGANIZATION SET EVENT ROUTING TABLE ert FOR ALL APPLICATION LISTINGS`,
		`ALTER ORGANIZATION SET EVENT ROUTING TABLE db.sch.ert FOR ALL APPLICATION LISTINGS`,
		`ALTER ORGANIZATION SET EVENT ROUTING TABLE "MyTable" FOR ALL APPLICATION LISTINGS`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationSetEventRoutingTable,
		``,
		`ALTER ORGANIZATION SET EVENT ROUTING TABLE ert`,
	)
}

func TestParseAlterOrganizationUnsetEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterOrganizationUnsetEventRoutingTable,
		`ALTER ORGANIZATION UNSET EVENT ROUTING TABLE FOR ALL APPLICATION LISTINGS`,
	)
	assertInvalid(t, (*Validator).ParseAlterOrganizationUnsetEventRoutingTable,
		``,
		`ALTER ORGANIZATION SET EVENT ROUTING TABLE FOR ALL APPLICATION LISTINGS`,
		`ALTER ORGANIZATION UNSET EVENT ROUTING TABLE`,
	)
}
