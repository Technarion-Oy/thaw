package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Task(t *testing.T) {
	validCases := []string{
		// ── CREATE TASK — root tasks (must have SCHEDULE) ────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
		"CREATE OR REPLACE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO t SELECT 1",
		"CREATE TASK IF NOT EXISTS db.schema.my_task WAREHOUSE = wh SCHEDULE = '5 MINUTE' AS CALL my_proc()",
		"CREATE TASK my_task USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = 'XSMALL' SCHEDULE = '1 MINUTE' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'root task' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '60 MINUTE' ALLOW_OVERLAPPING_EXECUTION = TRUE AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' USER_TASK_TIMEOUT_MS = 60000 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' SUSPEND_TASK_AFTER_NUM_FAILURES = 3 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' ERROR_INTEGRATION = my_int AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' CONFIG = $${\"key\": \"val\"}$$ AS SELECT 1",
		// ── CREATE TASK — multiline formatting ──────────────────────────
		"CREATE TASK my_task\n\tWAREHOUSE = wh\n\tSCHEDULE = '10 MINUTE'\n\tAS SELECT 1",
		"CREATE TASK my_task\n  WAREHOUSE = wh\n  SCHEDULE = '10 MINUTE'\n  AS\n  SELECT 1",
		"CREATE OR REPLACE TASK db.schema.my_task\n\tWAREHOUSE=COMPUTE_WH\n\tSCHEDULE='USING CRON 0 0 * * * UTC'\n\tAS SELECT SYSTEM$WAIT(5)",
		// ── CREATE TASK — quoted identifiers ────────────────────────────
		`CREATE TASK "My Task" WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1`,
		`CREATE TASK "db"."schema"."My Task" WAREHOUSE = wh SCHEDULE = '5 MINUTE' AS SELECT 1`,
		// ── CREATE TASK — mixed case ────────────────────────────────────
		"create task my_task warehouse = wh schedule = '10 MINUTE' as select 1",
		"Create Or Replace Task my_task Warehouse = wh Schedule = '5 MINUTE' As Select 1",
		// ── CREATE TASK — serverless (no WAREHOUSE, uses managed size) ──
		"CREATE TASK my_task USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = 'MEDIUM' SCHEDULE = '10 MINUTE' AS SELECT 1",
		// ── CREATE TASK — CRON schedule variants ────────────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 6 * * MON-FRI America/Los_Angeles' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON */5 * * * * UTC' AS SELECT 1",
		// ── CREATE TASK — multiple properties combined ──────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' ALLOW_OVERLAPPING_EXECUTION = TRUE USER_TASK_TIMEOUT_MS = 60000 SUSPEND_TASK_AFTER_NUM_FAILURES = 3 COMMENT = 'all props' AS SELECT 1",
		// ── CREATE TASK — AS body with complex SQL ──────────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS INSERT INTO t SELECT a, b FROM s WHERE x > 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS MERGE INTO target USING source ON target.id = source.id WHEN MATCHED THEN UPDATE SET val = source.val",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS CALL my_schema.my_proc('arg1', 42)",
		// ── CREATE TASK — child tasks (AFTER, no SCHEDULE) ──────────────
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent_task AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER db.schema.parent_task AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER task1, task2, task3 AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent_task WHEN SYSTEM$STREAM_HAS_DATA('my_stream') AS INSERT INTO t SELECT * FROM s",
		// ── CREATE TASK — child task with WHEN condition ─────────────────
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent WHEN SYSTEM$GET_PREDECESSOR_RETURN_VALUE('parent') = 'done' AS SELECT 1",
		"CREATE TASK child_task WAREHOUSE = wh AFTER parent WHEN cond1 AND cond2 AS SELECT 1",
		// ── CREATE TASK — multiple predecessors + WHEN combined ──────────
		"CREATE TASK child_task WAREHOUSE = wh AFTER task1, task2, task3 WHEN SYSTEM$STREAM_HAS_DATA('s') AS SELECT 1",
		// ── CREATE TASK — root task with WHEN (valid per Snowflake docs) ─
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' WHEN SYSTEM$STREAM_HAS_DATA('my_stream') AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '5 MINUTE' WHEN SYSTEM$STREAM_HAS_DATA('s1') AND SYSTEM$STREAM_HAS_DATA('s2') AS SELECT 1",
		// ── CREATE TASK — finalizer tasks ────────────────────────────────
		"CREATE TASK finalizer_task FINALIZE = root_task AS SELECT 1",
		"CREATE TASK finalizer_task WAREHOUSE = wh FINALIZE = root_task AS SELECT 1",
		// ── CREATE TASK — new properties (Section A) ─────────────────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' SUCCESS_INTEGRATION = my_int AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' OVERLAP_POLICY = 'NO_OVERLAP' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' TASK_AUTO_RETRY_ATTEMPTS = 3 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = 30 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' TARGET_COMPLETION_INTERVAL = '5 MINUTE' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' SERVERLESS_TASK_MIN_STATEMENT_SIZE = 'SMALL' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' SERVERLESS_TASK_MAX_STATEMENT_SIZE = 'XLARGE' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' LOG_LEVEL = 'INFO' AS SELECT 1",
		// ── CREATE OR ALTER TASK (Section B) ──────────────────────────────
		"CREATE OR ALTER TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
		"CREATE OR ALTER TASK db.schema.my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO t SELECT 1",
		// ── CREATE TASK — CLONE variant (Section B) ──────────────────────
		"CREATE TASK my_task CLONE other_task",
		"CREATE TASK my_task CLONE db.schema.other_task",
		"CREATE OR REPLACE TASK my_task CLONE other_task",
		// ── CREATE TASK — HOURS/SECONDS schedule units (Section C.5) ─────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '5 HOURS' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '30 SECONDS' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '1 HOUR' AS SELECT 1",
		// ── CREATE TASK — trailing semicolon with CRON (Section D) ────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS SELECT 1;",
		// ── CREATE TASK — empty AS body (Section D) — validator does not
		// check AS body content, only structure. Passes validation.
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS",
		// ── ALTER TASK ──────────────────────────────────────────────────
		"ALTER TASK my_task RESUME",
		"ALTER TASK my_task SUSPEND",
		"ALTER TASK IF EXISTS my_task RESUME",
		"ALTER TASK IF EXISTS my_task SUSPEND",
		"ALTER TASK my_task SET SCHEDULE = '10 MINUTE'",
		"ALTER TASK my_task SET WAREHOUSE = new_wh",
		"ALTER TASK my_task SET USER_TASK_TIMEOUT_MS = 60000",
		"ALTER TASK my_task SET COMMENT = 'updated'",
		"ALTER TASK my_task SET SUSPEND_TASK_AFTER_NUM_FAILURES = 5",
		"ALTER TASK my_task SET ERROR_INTEGRATION = my_int",
		"ALTER TASK my_task UNSET WAREHOUSE",
		"ALTER TASK my_task UNSET COMMENT",
		"ALTER TASK my_task REMOVE AFTER task1",
		"ALTER TASK my_task REMOVE AFTER task1, task2",
		"ALTER TASK my_task ADD AFTER task1",
		"ALTER TASK my_task ADD AFTER task1, task2",
		"ALTER TASK my_task MODIFY AS SELECT 1 FROM t",
		"ALTER TASK my_task MODIFY WHEN SYSTEM$STREAM_HAS_DATA('my_stream')",
		"ALTER TASK my_task SET FINALIZE = root_task",
		// ── ALTER TASK — RESUME/SUSPEND with trailing semicolons ────────
		"ALTER TASK my_task RESUME;",
		"ALTER TASK my_task SUSPEND;",
		// ── ALTER TASK — quoted identifiers ──────────────────────────────
		`ALTER TASK "My Task" RESUME`,
		`ALTER TASK "db"."schema"."My Task" SUSPEND`,
		// ── ALTER TASK — mixed case ─────────────────────────────────────
		"alter task my_task resume",
		"Alter Task my_task Set Warehouse = new_wh",
		// ── ALTER TASK — IF EXISTS with various sub-commands ─────────────
		"ALTER TASK IF EXISTS my_task SUSPEND",
		"ALTER TASK IF EXISTS my_task SET WAREHOUSE = wh",
		"ALTER TASK IF EXISTS my_task MODIFY AS SELECT 1",
		"ALTER TASK IF EXISTS db.schema.my_task RESUME",
		// ── ALTER TASK — SET with CRON schedule ─────────────────────────
		"ALTER TASK my_task SET SCHEDULE = 'USING CRON 0 6 * * MON-FRI UTC'",
		// ── ALTER TASK — qualified task names ───────────────────────────
		"ALTER TASK db.schema.my_task RESUME",
		"ALTER TASK db.schema.my_task SET WAREHOUSE = new_wh",
		"ALTER TASK db.schema.my_task ADD AFTER db.schema.parent_task",
		// ── ALTER TASK — REMOVE/ADD AFTER with qualified names ──────────
		"ALTER TASK my_task REMOVE AFTER db.schema.task1, db.schema.task2",
		"ALTER TASK my_task ADD AFTER schema.task1, schema.task2, schema.task3",
		// ── ALTER TASK — MODIFY AS with complex body ────────────────────
		"ALTER TASK my_task MODIFY AS INSERT INTO t SELECT * FROM s WHERE x > 1",
		"ALTER TASK my_task MODIFY WHEN SYSTEM$STREAM_HAS_DATA('s1') AND SYSTEM$STREAM_HAS_DATA('s2')",
		// ── ALTER TASK — SET FINALIZE with qualified name ────────────────
		"ALTER TASK my_task SET FINALIZE = db.schema.root_task",
		// ── ALTER TASK — UNSET multiple known properties ─────────────────
		"ALTER TASK my_task UNSET SCHEDULE",
		"ALTER TASK my_task UNSET ERROR_INTEGRATION",
		"ALTER TASK my_task UNSET USER_TASK_TIMEOUT_MS",
		"ALTER TASK my_task UNSET SUSPEND_TASK_AFTER_NUM_FAILURES",
		"ALTER TASK my_task UNSET ALLOW_OVERLAPPING_EXECUTION",
		// ── ALTER TASK — UNSET new properties (Section A) ────────────────
		"ALTER TASK my_task UNSET SUCCESS_INTEGRATION",
		"ALTER TASK my_task UNSET OVERLAP_POLICY",
		"ALTER TASK my_task UNSET TASK_AUTO_RETRY_ATTEMPTS",
		"ALTER TASK my_task UNSET USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS",
		"ALTER TASK my_task UNSET TARGET_COMPLETION_INTERVAL",
		"ALTER TASK my_task UNSET SERVERLESS_TASK_MIN_STATEMENT_SIZE",
		"ALTER TASK my_task UNSET SERVERLESS_TASK_MAX_STATEMENT_SIZE",
		"ALTER TASK my_task UNSET LOG_LEVEL",
		// ── ALTER TASK — SET new properties (Section A) ──────────────────
		"ALTER TASK my_task SET SUCCESS_INTEGRATION = my_int",
		"ALTER TASK my_task SET OVERLAP_POLICY = 'NO_OVERLAP'",
		"ALTER TASK my_task SET TASK_AUTO_RETRY_ATTEMPTS = 3",
		"ALTER TASK my_task SET USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = 30",
		"ALTER TASK my_task SET TARGET_COMPLETION_INTERVAL = '5 MINUTE'",
		"ALTER TASK my_task SET SERVERLESS_TASK_MIN_STATEMENT_SIZE = 'SMALL'",
		"ALTER TASK my_task SET SERVERLESS_TASK_MAX_STATEMENT_SIZE = 'XLARGE'",
		"ALTER TASK my_task SET LOG_LEVEL = 'INFO'",
		// ── ALTER TASK — REMOVE WHEN (Section B) ─────────────────────────
		"ALTER TASK my_task REMOVE WHEN",
		// ── ALTER TASK — UNSET FINALIZE (Section B) ──────────────────────
		"ALTER TASK my_task UNSET FINALIZE",
		// ── ALTER TASK — SET TAG / UNSET TAG (Section B) ─────────────────
		"ALTER TASK my_task SET TAG cost_center = 'finance'",
		"ALTER TASK my_task UNSET TAG cost_center",
		// ── ALTER TASK — multiple SET properties (Section D) ─────────────
		"ALTER TASK my_task SET WAREHOUSE = new_wh COMMENT = 'updated'",
		// ── CREATE TASK — WITH TAG (no false positive on tag keys) ────────
		"CREATE TASK my_task\n  WITH TAG (cost_center = 'finance')\n  WAREHOUSE = wh SCHEDULE = '10 MINUTE'\n  AS SELECT 1",
		// ── CREATE TASK — WITH CONTACT ───────────────────────────────────
		"CREATE TASK my_task\n  WITH CONTACT (purpose = contact_name)\n  WAREHOUSE = wh SCHEDULE = '10 MINUTE'\n  AS SELECT 1",
		// ── CREATE TASK — session parameters (no false positive) ─────────
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' STATEMENT_TIMEOUT_IN_SECONDS = 3600 AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' TIMEZONE = 'America/New_York' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' QUERY_TAG = 'my_tag' AS SELECT 1",
		// ── CREATE TASK — EXECUTE AS USER / CALLER / OWNER ───────────────
		"CREATE TASK my_task\n  WAREHOUSE = wh SCHEDULE = '10 MINUTE'\n  EXECUTE AS USER my_user\n  AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' EXECUTE AS CALLER AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' EXECUTE AS OWNER AS SELECT 1",
		// ── CREATE TASK — BEGIN...END scripting body ─────────────────────
		"CREATE TASK my_task\n  WAREHOUSE = wh SCHEDULE = '10 MINUTE'\n  AS\n  BEGIN\n    INSERT INTO t1 SELECT * FROM s1;\n    INSERT INTO t2 SELECT * FROM s2;\n  END;",
		// ── ALTER TASK — SET/UNSET session parameters ────────────────────
		"ALTER TASK my_task SET STATEMENT_TIMEOUT_IN_SECONDS = 3600",
		"ALTER TASK my_task SET TIMEZONE = 'UTC'",
		"ALTER TASK my_task SET QUERY_TAG = 'my_tag'",
		// ── ALTER TASK — SET/UNSET CONTACT ────────────────────────────────
		"ALTER TASK my_task SET CONTACT purpose = contact_name",
		"ALTER TASK my_task UNSET CONTACT purpose",
		// ── ALTER TASK — SET/UNSET EXECUTE AS ─────────────────────────────
		"ALTER TASK my_task SET EXECUTE AS USER my_user",
		"ALTER TASK my_task UNSET EXECUTE AS USER",
		// ── ALTER TASK — UNSET DCM PROJECT ────────────────────────────────
		"ALTER TASK my_task UNSET DCM PROJECT",
		// ── CREATE TASK — CLONE + IF NOT EXISTS ──────────────────────────
		"CREATE TASK IF NOT EXISTS my_task CLONE other_task",
		// ── DROP TASK ────────────────────────────────────────────────────
		"DROP TASK my_task",
		"DROP TASK IF EXISTS my_task",
		"DROP TASK db.schema.my_task",
		"DROP TASK IF EXISTS db.schema.my_task",
		`DROP TASK "My Task"`,
		"drop task my_task",
		// ── DROP TASK — trailing semicolons ──────────────────────────────
		"DROP TASK my_task;",
		"DROP TASK IF EXISTS my_task;",
		// ── DROP TASK — IF EXISTS + quoted identifiers ───────────────────
		`DROP TASK IF EXISTS "My Task"`,
		// ── DROP TASK — IF EXISTS lowercase ──────────────────────────────
		"drop task if exists my_task",
		// ── CREATE TASK — no explicit warehouse (serverless, inherits defaults)
		"CREATE TASK my_task SCHEDULE = '10 MINUTE' AS SELECT 1",
		// ── CREATE TASK — string-literal false-positive prevention ───────
		// Keywords inside COMMENT (or other string-valued properties) must
		// not trigger structural validation after reStripStringLiterals.
		"CREATE TASK child WAREHOUSE = wh AFTER parent COMMENT = 'SCHEDULE = 10 MINUTE' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'AFTER parent_task' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'FINALIZE = root' AS SELECT 1",
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'WHEN true' AS SELECT 1",
		// ── String-literal false-positive: AS keyword inside COMMENT must
		// not be mistaken for the body delimiter (tests string-stripping +
		// EXECUTE_AS neutralisation path).
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'AS SELECT 1' AS SELECT 2",
		// ── String-literal false-positive: CLONE keyword inside COMMENT must
		// not trigger the CLONE early-return branch.
		"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' COMMENT = 'CLONE other_task' AS SELECT 1",
		// ── CREATE TASK — AFTER with quoted predecessor identifiers ──────
		`CREATE TASK child WAREHOUSE = wh AFTER "My Parent Task" AS SELECT 1`,
		`CREATE TASK child WAREHOUSE = wh AFTER "db"."schema"."parent" AS SELECT 1`,
		`CREATE TASK child WAREHOUSE = wh AFTER "task1", "task2" AS SELECT 1`,
		// ── CREATE TASK — FINALIZE with fully-qualified name ─────────────
		"CREATE TASK f FINALIZE = db.schema.root_task AS SELECT 1",
		`CREATE TASK f FINALIZE = "db"."schema"."root_task" AS SELECT 1`,
		// ── CREATE OR ALTER TASK — child task variant ────────────────────
		"CREATE OR ALTER TASK child WAREHOUSE = wh AFTER parent_task AS SELECT 1",
		"CREATE OR ALTER TASK child WAREHOUSE = wh AFTER task1, task2 AS SELECT 1",
		// ── ALTER TASK — ADD/REMOVE AFTER with quoted identifiers ────────
		`ALTER TASK my_task ADD AFTER "task1", "task2"`,
		`ALTER TASK my_task REMOVE AFTER "task1"`,
		`ALTER TASK my_task ADD AFTER "db"."schema"."task1"`,
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// ── CREATE TASK — missing name ──────────────────────────────────
		{
			"bare CREATE TASK without name",
			"CREATE TASK",
			[]string{"CREATE TASK requires a task name"},
		},
		{
			"CREATE OR REPLACE TASK without name",
			"CREATE OR REPLACE TASK",
			[]string{"CREATE TASK requires a task name"},
		},
		// ── CREATE TASK — OR REPLACE + IF NOT EXISTS conflict ───────────
		{
			"OR REPLACE + IF NOT EXISTS conflict",
			"CREATE OR REPLACE TASK IF NOT EXISTS my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// ── CREATE TASK — missing AS body ───────────────────────────────
		{
			"missing AS keyword",
			"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE'",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── CREATE TASK — AFTER + SCHEDULE mutual exclusivity ───────────
		{
			"AFTER and SCHEDULE are mutually exclusive",
			"CREATE TASK child WAREHOUSE = wh AFTER parent SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"AFTER and SCHEDULE are mutually exclusive"},
		},
		// ── CREATE TASK — root task without SCHEDULE ────────────────────
		{
			"root task missing SCHEDULE",
			"CREATE TASK my_task WAREHOUSE = wh AS SELECT 1",
			[]string{"Root task (no AFTER or FINALIZE clause) requires a SCHEDULE"},
		},
		// ── CREATE TASK — bare AFTER without names ──────────────────────
		{
			"bare AFTER without predecessor names",
			"CREATE TASK child WAREHOUSE = wh AFTER AS SELECT 1",
			[]string{"AFTER requires at least one predecessor task name"},
		},
		// ── CREATE TASK — FINALIZE + AFTER conflict ─────────────────────
		{
			"FINALIZE with AFTER is invalid",
			"CREATE TASK finalizer WAREHOUSE = wh FINALIZE = root_task AFTER parent AS SELECT 1",
			[]string{"FINALIZE must not be combined with AFTER"},
		},
		// ── CREATE TASK — FINALIZE + SCHEDULE conflict ──────────────────
		{
			"FINALIZE with SCHEDULE is invalid",
			"CREATE TASK finalizer WAREHOUSE = wh FINALIZE = root_task SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"FINALIZE must not be combined with SCHEDULE"},
		},
		// ── CREATE TASK — bare WHEN without expression ──────────────────
		{
			"bare WHEN without expression",
			"CREATE TASK child WAREHOUSE = wh AFTER parent WHEN AS SELECT 1",
			[]string{"WHEN requires a boolean expression"},
		},
		// ── CREATE TASK — FINALIZE without root task name ───────────────
		{
			"bare FINALIZE without root task name",
			"CREATE TASK finalizer FINALIZE AS SELECT 1",
			[]string{"FINALIZE requires a root task name"},
		},
		// ── ALTER TASK — missing name ───────────────────────────────────
		{
			"bare ALTER TASK without name",
			"ALTER TASK",
			[]string{"ALTER TASK requires a task name"},
		},
		// ── ALTER TASK — unknown sub-command ────────────────────────────
		{
			"ALTER TASK unknown sub-command",
			"ALTER TASK my_task RESET",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── ALTER TASK — ADD AFTER without names ────────────────────────
		{
			"ALTER TASK ADD AFTER without names",
			"ALTER TASK my_task ADD AFTER",
			[]string{"ADD AFTER requires at least one predecessor task name"},
		},
		// ── ALTER TASK — MODIFY AS without body ─────────────────────────
		{
			"ALTER TASK MODIFY AS without body",
			"ALTER TASK my_task MODIFY AS",
			[]string{"MODIFY AS requires a SQL statement"},
		},
		// ── ALTER TASK — MODIFY WHEN without expression ─────────────────
		{
			"ALTER TASK MODIFY WHEN without expression",
			"ALTER TASK my_task MODIFY WHEN",
			[]string{"MODIFY WHEN requires a boolean expression"},
		},
		// ── ALTER TASK — SET FINALIZE without root task name ─────────────
		{
			"ALTER TASK SET FINALIZE without name",
			"ALTER TASK my_task SET FINALIZE =",
			[]string{"SET FINALIZE requires a root task name"},
		},
		// ── ALTER TASK — REMOVE AFTER without name ──────────────────────
		{
			"ALTER TASK REMOVE AFTER without task name",
			"ALTER TASK my_task REMOVE AFTER",
			[]string{"REMOVE AFTER requires at least one predecessor task name"},
		},
		// Property validation removed for tasks — they accept arbitrary session
		// parameters. SET/UNSET with unknown properties are tested as valid cases.
		// ── CREATE TASK — IF NOT EXISTS without name ─────────────────────
		// Note: the regex parses "IF" as the task name, so the error is
		// "missing AS" rather than "missing name" — a known limitation.
		{
			"CREATE TASK IF NOT EXISTS without name",
			"CREATE TASK IF NOT EXISTS",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── CREATE TASK — multiline missing AS ──────────────────────────
		{
			"multiline CREATE TASK missing AS",
			"CREATE TASK my_task\n\tWAREHOUSE = wh\n\tSCHEDULE = '10 MINUTE'",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── CREATE TASK — FINALIZE + AFTER + SCHEDULE triple conflict ───
		{
			"FINALIZE with AFTER and SCHEDULE",
			"CREATE TASK finalizer WAREHOUSE = wh FINALIZE = root_task AFTER parent SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"FINALIZE must not be combined with AFTER", "FINALIZE must not be combined with SCHEDULE"},
		},
		// ── CREATE TASK — FINALIZE = (equals but no name) ───────────────
		{
			"FINALIZE with equals but no name",
			"CREATE TASK finalizer FINALIZE = AS SELECT 1",
			[]string{"FINALIZE requires a root task name"},
		},
		// Property validation removed for tasks — arbitrary session parameters
		// are valid. Unknown properties are tested as valid cases instead.
		// ── ALTER TASK — MODIFY AS bare (trailing whitespace only) ──────
		{
			"ALTER TASK MODIFY AS with trailing whitespace",
			"ALTER TASK my_task MODIFY AS   ",
			[]string{"MODIFY AS requires a SQL statement"},
		},
		// ── ALTER TASK — MODIFY WHEN bare (trailing whitespace only) ────
		{
			"ALTER TASK MODIFY WHEN with trailing whitespace",
			"ALTER TASK my_task MODIFY WHEN   ",
			[]string{"MODIFY WHEN requires a boolean expression"},
		},
		// ── ALTER TASK — SET FINALIZE = (equals but no name) ────────────
		{
			"ALTER TASK SET FINALIZE equals no name",
			"ALTER TASK my_task SET FINALIZE = ;",
			[]string{"SET FINALIZE requires a root task name"},
		},
		// Property validation removed — MAX_RETRIES treated as session parameter.
		// ── ALTER TASK — bare ALTER TASK with no sub-command (Section C.2) ─
		{
			"ALTER TASK with no sub-command",
			"ALTER TASK my_task",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── CREATE TASK — FINALIZE + SCHEDULE without AFTER (Section D) ──
		{
			"FINALIZE with SCHEDULE but no AFTER",
			"CREATE TASK finalizer FINALIZE = root_task SCHEDULE = '10 MINUTE' AS SELECT 1",
			[]string{"FINALIZE must not be combined with SCHEDULE"},
		},
		// ── DROP TASK — missing name ─────────────────────────────────────
		{
			"bare DROP TASK without name",
			"DROP TASK",
			[]string{"DROP TASK requires a task name"},
		},
		{
			"DROP TASK with semicolon only",
			"DROP TASK;",
			[]string{"DROP TASK requires a task name"},
		},
		// ── ALTER TASK — IF EXISTS without name ─────────────────────────
		// Note: the regex parses "IF" as the task name (known limitation,
		// mirrors CREATE TASK IF NOT EXISTS). "EXISTS" becomes an unknown
		// sub-command, so we get a different error than "missing name".
		{
			"ALTER TASK IF EXISTS without name",
			"ALTER TASK IF EXISTS",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── CREATE OR ALTER TASK — missing name ─────────────────────────
		{
			"CREATE OR ALTER TASK without name",
			"CREATE OR ALTER TASK",
			[]string{"CREATE TASK requires a task name"},
		},
		// ── CREATE TASK — FINALIZE without = sign ───────────────────────
		{
			"FINALIZE without equals sign",
			"CREATE TASK finalizer FINALIZE root_task AS SELECT 1",
			[]string{"FINALIZE requires a root task name"},
		},
		// ── CREATE TASK — root task with WHEN but no SCHEDULE ────────────
		{
			"root task with WHEN but missing SCHEDULE",
			"CREATE TASK my_task WAREHOUSE = wh WHEN SYSTEM$STREAM_HAS_DATA('s') AS SELECT 1",
			[]string{"Root task (no AFTER or FINALIZE clause) requires a SCHEDULE"},
		},
		// ── ALTER TASK — bare MODIFY without AS or WHEN ─────────────────
		{
			"ALTER TASK bare MODIFY",
			"ALTER TASK my_task MODIFY",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── CREATE TASK — EXECUTE AS CALLER but no body AS ──────────────
		{
			"EXECUTE AS CALLER without body AS",
			"CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' EXECUTE AS CALLER",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── CREATE TASK — CLONE without source name ─────────────────────
		// CLONE regex requires a source identifier; without one the validator
		// falls through to the AS-required check.
		{
			"CREATE TASK CLONE without source name",
			"CREATE TASK my_task CLONE",
			[]string{"CREATE TASK requires an AS clause"},
		},
		// ── ALTER TASK — bare ADD without AFTER keyword ─────────────────
		{
			"ALTER TASK bare ADD without AFTER",
			"ALTER TASK my_task ADD",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── ALTER TASK — bare REMOVE without AFTER or WHEN ──────────────
		{
			"ALTER TASK bare REMOVE without target",
			"ALTER TASK my_task REMOVE",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		// ── ALTER TASK — RESUME/SUSPEND with trailing content ────────────
		// RESUME and SUSPEND must be standalone at end of statement (regex
		// anchors to $). Trailing tokens make the sub-command unrecognised.
		{
			"ALTER TASK RESUME with trailing content",
			"ALTER TASK my_task RESUME NOW",
			[]string{"Unknown ALTER TASK sub-command"},
		},
		{
			"ALTER TASK SUSPEND with trailing content",
			"ALTER TASK my_task SUSPEND IMMEDIATELY",
			[]string{"Unknown ALTER TASK sub-command"},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tc.sql)
				return
			}
			for _, wantMsg := range tc.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tc.sql, warns)
				}
			}
		})
	}
}

// ── Time Travel AT / BEFORE Tests ──────────────────────────────────────────


func TestValidateTablesExist_CreateTask_UsingCron(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			name: "USING CRON in SCHEDULE",
			sql:  "CREATE OR REPLACE TASK LINEAGE_SOURCE_DB.RAW_DATA.TASK_1\n\tWAREHOUSE=COMPUTE_WH\n\tSCHEDULE='USING CRON 0 0 * * * UTC'\n\tAS SELECT SYSTEM$WAIT(5)",
		},
		{
			name: "USING CRON with INSERT INTO",
			sql:  "CREATE OR REPLACE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 0 * * * UTC' AS INSERT INTO LIVE_TABLE SELECT 1",
		},
		{
			name: "USING CRON weekday schedule",
			sql:  "CREATE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON 0 6 * * MON-FRI America/Los_Angeles' AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "USING CRON every-5-minutes schedule",
			sql:  "CREATE TASK my_task WAREHOUSE = wh SCHEDULE = 'USING CRON */5 * * * * UTC' AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "child task with AFTER does not flag preamble tokens",
			sql:  "CREATE TASK child_task WAREHOUSE = wh AFTER parent_task AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "task with WHEN condition referencing SYSTEM function",
			sql:  "CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' WHEN SYSTEM$STREAM_HAS_DATA('my_stream') AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "task with FINALIZE does not flag root task name",
			sql:  "CREATE TASK finalizer FINALIZE = root_task AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "string containing FROM keyword",
			sql:  "SELECT * FROM LIVE_TABLE WHERE name = 'FROM FAKE_TABLE'",
		},
		{
			name: "string containing JOIN keyword",
			sql:  "SELECT * FROM LIVE_TABLE WHERE note = 'JOIN SOMETHING ON x'",
		},
		{
			name: "task with multiple AFTER predecessors does not flag predecessor names",
			sql:  "CREATE TASK child WAREHOUSE = wh AFTER task1, task2, task3 AS SELECT * FROM LIVE_TABLE",
		},
		{
			name: "task with EXECUTE AS USER does not flag user name as table ref",
			sql:  "CREATE TASK my_task WAREHOUSE = wh SCHEDULE = '10 MINUTE' EXECUTE AS USER my_user AS SELECT * FROM LIVE_TABLE",
		},
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:   getLiveRefs(),
		KnownDatabases: []string{"DB", "LINEAGE_SOURCE_DB"},
		KnownSchemas:   []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "LINEAGE_SOURCE_DB", Name: "RAW_DATA"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req.SQL = tc.sql
			req.StmtRanges = GetStatementRanges(tc.sql)
			markers := ValidateTablesExist(req)
			errs := getErrors(markers)
			if len(errs) > 0 {
				t.Errorf("Expected 0 errors for %q, got %d:", tc.sql, len(errs))
				for _, e := range errs {
					t.Errorf("  - %s (line %d, col %d)", e.Message, e.StartLineNumber, e.StartColumn)
				}
			}
		})
	}
}

