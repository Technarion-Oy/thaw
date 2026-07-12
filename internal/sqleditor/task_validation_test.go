package sqleditor

import (
	"testing"
)

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
		ResolvedRefs:    getLiveRefs(),
		KnownDatabases:  []string{"DB", "LINEAGE_SOURCE_DB"},
		KnownSchemas:    []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "LINEAGE_SOURCE_DB", Name: "RAW_DATA"}},
		SessionDatabase: "DB",
		SessionSchema:   "SCH",
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
