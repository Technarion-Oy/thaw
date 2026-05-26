package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Service(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE SERVICE — inline YAML specification
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE — stage-referenced specification file
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// CREATE OR REPLACE SERVICE
		"CREATE OR REPLACE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE IF NOT EXISTS
		"CREATE SERVICE IF NOT EXISTS my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE — with optional properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1 MAX_INSTANCES = 3",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = TRUE",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = FALSE",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ QUERY_WAREHOUSE = wh1",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'my service'",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ EXTERNAL_ACCESS_INTEGRATIONS = (my_eai)",
		// CREATE SERVICE — all optional properties together
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1 MAX_INSTANCES = 5 AUTO_RESUME = TRUE QUERY_WAREHOUSE = wh1 COMMENT = 'full opts'",
		// CREATE SERVICE — MIN_INSTANCES = 0 is valid (non-negative)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 0 MAX_INSTANCES = 3",
		// CREATE SERVICE — with SPECIFICATION_FILE and properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml' MIN_INSTANCES = 1 MAX_INSTANCES = 2",
		// CREATE SERVICE — COMMENT with IF NOT EXISTS inside string should not trigger conflict
		"CREATE OR REPLACE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'IF NOT EXISTS hint'",
		// CREATE SERVICE — FROM @stage SPECIFICATION_FILE (stage-prefix form)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// CREATE SERVICE — FROM SPECIFICATION_TEMPLATE (parameterized spec)
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE $$spec with {{ var }}$$",
		// CREATE SERVICE — FROM SPECIFICATION_TEMPLATE_FILE
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE_FILE = '@stage/spec.yaml'",
		// CREATE SERVICE — FROM @stage SPECIFICATION_TEMPLATE_FILE
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_TEMPLATE_FILE = 'spec.yaml'",
		// CREATE SERVICE — additional known properties
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_SUSPEND_SECS = 300",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_READY_INSTANCES = 1",
		"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ LOG_LEVEL = 'INFO'",
		// CREATE SERVICE — schema-qualified name
		"CREATE SERVICE db.schema.my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// CREATE SERVICE — case insensitive
		"create service my_svc in compute pool my_pool from specification $$spec$$",
		"Create Or Replace Service my_svc In Compute Pool my_pool From Specification $$spec$$",

		// EXECUTE SERVICE — inline YAML
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// EXECUTE JOB SERVICE — canonical form
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
		// EXECUTE SERVICE — stage-referenced specification file
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// EXECUTE JOB SERVICE — stage-prefix form
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// EXECUTE SERVICE — with optional properties
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ QUERY_WAREHOUSE = wh1",
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ COMMENT = 'batch job'",
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ EXTERNAL_ACCESS_INTEGRATIONS = (eai1)",
		// EXECUTE JOB SERVICE — additional properties
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ NAME = my_named_job",
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ ASYNC = TRUE",
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ REPLICAS = 3",
		// EXECUTE SERVICE — SPECIFICATION_TEMPLATE
		"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE $$spec$$",
		// EXECUTE JOB SERVICE — SPECIFICATION_TEMPLATE_FILE
		"EXECUTE JOB SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION_TEMPLATE_FILE = '@stage/spec.yaml'",
		// EXECUTE SERVICE — case insensitive
		"execute service my_job in compute pool my_pool from specification $$spec$$",
		"Execute Job Service my_job In Compute Pool my_pool From Specification $$spec$$",

		// ALTER SERVICE — SUSPEND / RESUME
		"ALTER SERVICE my_svc SUSPEND",
		"ALTER SERVICE my_svc RESUME",
		// ALTER SERVICE — SET properties
		"ALTER SERVICE my_svc SET MIN_INSTANCES = 2",
		"ALTER SERVICE my_svc SET MAX_INSTANCES = 5",
		"ALTER SERVICE my_svc SET COMMENT = 'updated'",
		"ALTER SERVICE my_svc SET QUERY_WAREHOUSE = wh2",
		// ALTER SERVICE — UNSET properties
		"ALTER SERVICE my_svc UNSET COMMENT",
		"ALTER SERVICE my_svc UNSET QUERY_WAREHOUSE",
		// ALTER SERVICE — rolling update with FROM SPECIFICATION
		"ALTER SERVICE my_svc FROM SPECIFICATION $$new_spec$$",
		// ALTER SERVICE — rolling update with FROM SPECIFICATION_FILE
		"ALTER SERVICE my_svc FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
		// ALTER SERVICE — FROM SPECIFICATION_TEMPLATE
		"ALTER SERVICE my_svc FROM SPECIFICATION_TEMPLATE $$new_spec$$",
		// ALTER SERVICE — FROM @stage SPECIFICATION_FILE
		"ALTER SERVICE my_svc FROM @my_stage SPECIFICATION_FILE = 'spec.yaml'",
		// ALTER SERVICE — UNSET MIN_INSTANCES / MAX_INSTANCES
		"ALTER SERVICE my_svc UNSET MIN_INSTANCES",
		"ALTER SERVICE my_svc UNSET MAX_INSTANCES",
		// ALTER SERVICE — IF EXISTS with various sub-commands
		"ALTER SERVICE IF EXISTS my_svc SUSPEND",
		"ALTER SERVICE IF EXISTS my_svc RESUME",
		"ALTER SERVICE IF EXISTS my_svc SET MIN_INSTANCES = 2",
		"ALTER SERVICE IF EXISTS my_svc UNSET COMMENT",
		"ALTER SERVICE IF EXISTS my_svc FROM SPECIFICATION $$new_spec$$",
		// ALTER SERVICE — SET with multiple valid properties
		"ALTER SERVICE my_svc SET MIN_INSTANCES = 2 MAX_INSTANCES = 10",
		"ALTER SERVICE my_svc SET COMMENT = 'updated' QUERY_WAREHOUSE = wh2",
		// ALTER SERVICE — case insensitive
		"alter service my_svc suspend",
		"Alter Service my_svc Set Min_Instances = 2",

		// DROP SERVICE
		"DROP SERVICE my_svc",
		"DROP SERVICE IF EXISTS my_svc",
		// DROP SERVICE — schema-qualified name
		"DROP SERVICE db.schema.my_svc",
		// DROP SERVICE — case insensitive
		"drop service my_svc",
		"Drop Service If Exists my_svc",

		// CREATE IMAGE REPOSITORY — valid
		"CREATE IMAGE REPOSITORY my_repo",
		"CREATE OR REPLACE IMAGE REPOSITORY my_repo",
		"CREATE IMAGE REPOSITORY IF NOT EXISTS my_repo",
		"CREATE IMAGE REPOSITORY my_repo COMMENT = 'my image repo'",
		"CREATE OR REPLACE IMAGE REPOSITORY my_repo COMMENT = 'replaced repo'",
		// CREATE IMAGE REPOSITORY — schema-qualified names
		"CREATE IMAGE REPOSITORY db.schema.my_repo",
		"CREATE IMAGE REPOSITORY schema.my_repo",
		// CREATE IMAGE REPOSITORY — case insensitive
		"create image repository my_repo",
		"Create Image Repository IF NOT EXISTS my_repo",

		// DROP IMAGE REPOSITORY — valid
		"DROP IMAGE REPOSITORY my_repo",
		"DROP IMAGE REPOSITORY IF EXISTS my_repo",
		"DROP IMAGE REPOSITORY db.schema.my_repo",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}

	// ── Invalid cases ───────────────────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		// CREATE SERVICE — missing name
		{
			"CREATE SERVICE missing name",
			"CREATE SERVICE",
			[]string{"Unexpected syntax in CREATE SERVICE"},
		},
		// CREATE SERVICE — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE SERVICE OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE SERVICE IF NOT EXISTS my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE SERVICE — missing IN COMPUTE POOL
		{
			"CREATE SERVICE missing COMPUTE POOL",
			"CREATE SERVICE my_svc FROM SPECIFICATION $$spec$$",
			[]string{"Missing mandatory IN COMPUTE POOL"},
		},
		// CREATE SERVICE — missing specification
		{
			"CREATE SERVICE missing spec",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool",
			[]string{"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// CREATE SERVICE — both SPECIFICATION and SPECIFICATION_FILE
		{
			"CREATE SERVICE both spec and spec file",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ FROM SPECIFICATION_FILE = '@stage/spec.yaml'",
			[]string{"requires exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// CREATE SERVICE — MIN_INSTANCES negative
		{
			"CREATE SERVICE MIN_INSTANCES negative",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = -1",
			[]string{"MIN_INSTANCES value -1 must be a non-negative integer"},
		},
		// CREATE SERVICE — MAX_INSTANCES negative
		{
			"CREATE SERVICE MAX_INSTANCES negative",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MAX_INSTANCES = -1",
			[]string{"MAX_INSTANCES value -1 must be a non-negative integer"},
		},
		// CREATE SERVICE — MAX_INSTANCES < MIN_INSTANCES
		{
			"CREATE SERVICE MAX < MIN",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 5 MAX_INSTANCES = 2",
			[]string{"MAX_INSTANCES (2) must be >= MIN_INSTANCES (5)"},
		},
		// CREATE SERVICE — AUTO_RESUME invalid value
		{
			"CREATE SERVICE AUTO_RESUME invalid",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = MAYBE",
			[]string{"AUTO_RESUME must be TRUE or FALSE"},
		},
		// CREATE SERVICE — unexpected property
		{
			"CREATE SERVICE unexpected property",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},

		// CREATE SERVICE — missing both COMPUTE POOL and specification (multiple errors)
		{
			"CREATE SERVICE missing COMPUTE POOL and spec",
			"CREATE SERVICE my_svc",
			[]string{"Missing mandatory IN COMPUTE POOL", "Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// CREATE SERVICE — MAX_INSTANCES negative and less than MIN (dual errors)
		{
			"CREATE SERVICE MAX negative and less than MIN",
			"CREATE SERVICE my_svc IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 3 MAX_INSTANCES = -1",
			[]string{"MAX_INSTANCES value -1 must be a non-negative integer", "MAX_INSTANCES (-1) must be >= MIN_INSTANCES (3)"},
		},
		// CREATE SERVICE — MIN_INSTANCES and MAX_INSTANCES both zero (equal, valid boundary — no warning expected would be a valid case, but MIN=0 MAX=0 is valid)

		// EXECUTE SERVICE — missing name
		{
			"EXECUTE SERVICE missing name",
			"EXECUTE SERVICE",
			[]string{"Unexpected syntax in EXECUTE SERVICE"},
		},
		// EXECUTE JOB SERVICE — missing name (canonical form)
		{
			"EXECUTE JOB SERVICE missing name",
			"EXECUTE JOB SERVICE",
			[]string{"Unexpected syntax in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — missing COMPUTE POOL
		{
			"EXECUTE SERVICE missing COMPUTE POOL",
			"EXECUTE SERVICE my_job FROM SPECIFICATION $$spec$$",
			[]string{"Missing mandatory IN COMPUTE POOL"},
		},
		// EXECUTE SERVICE — missing specification
		{
			"EXECUTE SERVICE missing spec",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool",
			[]string{"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE"},
		},
		// EXECUTE SERVICE — MIN_INSTANCES not supported
		{
			"EXECUTE SERVICE with MIN_INSTANCES",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MIN_INSTANCES = 1",
			[]string{"MIN_INSTANCES is not supported in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — MAX_INSTANCES not supported
		{
			"EXECUTE SERVICE with MAX_INSTANCES",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ MAX_INSTANCES = 3",
			[]string{"MAX_INSTANCES is not supported in EXECUTE SERVICE"},
		},
		// EXECUTE SERVICE — unexpected property
		{
			"EXECUTE SERVICE unexpected property",
			"EXECUTE SERVICE my_job IN COMPUTE POOL my_pool FROM SPECIFICATION $$spec$$ AUTO_RESUME = TRUE",
			[]string{"Unexpected property 'AUTO_RESUME'"},
		},

		// ALTER SERVICE — missing name
		{
			"ALTER SERVICE missing name",
			"ALTER SERVICE",
			[]string{"ALTER SERVICE requires a service name"},
		},
		// ALTER SERVICE — unknown sub-command
		{
			"ALTER SERVICE unknown action",
			"ALTER SERVICE my_svc ENABLE",
			[]string{"Unknown ALTER SERVICE sub-command"},
		},
		// ALTER SERVICE — SET with unknown property
		{
			"ALTER SERVICE SET unknown property",
			"ALTER SERVICE my_svc SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER SERVICE SET"},
		},
		// ALTER SERVICE — UNSET with unknown property
		{
			"ALTER SERVICE UNSET unknown property",
			"ALTER SERVICE my_svc UNSET UNKNOWN_PROP",
			[]string{"Unknown property in ALTER SERVICE UNSET"},
		},
		// ALTER SERVICE — MIN_INSTANCES negative
		{
			"ALTER SERVICE MIN_INSTANCES negative",
			"ALTER SERVICE my_svc SET MIN_INSTANCES = -1",
			[]string{"MIN_INSTANCES value -1 must be a non-negative integer"},
		},
		// ALTER SERVICE — MAX_INSTANCES negative
		{
			"ALTER SERVICE MAX_INSTANCES negative",
			"ALTER SERVICE my_svc SET MAX_INSTANCES = -5",
			[]string{"MAX_INSTANCES value -5 must be a non-negative integer"},
		},
		// ALTER SERVICE — MAX < MIN
		{
			"ALTER SERVICE MAX < MIN",
			"ALTER SERVICE my_svc SET MIN_INSTANCES = 10 MAX_INSTANCES = 2",
			[]string{"MAX_INSTANCES (2) must be >= MIN_INSTANCES (10)"},
		},

		// ALTER SERVICE — SET with unknown trailing property
		{
			"ALTER SERVICE SET with unknown trailing property",
			"ALTER SERVICE my_svc SET COMMENT = 'foo' SOME_NONSENSE = bar",
			[]string{"Unexpected property 'SOME_NONSENSE'"},
		},

		// DROP SERVICE — missing name
		{
			"DROP SERVICE missing name",
			"DROP SERVICE",
			[]string{"DROP SERVICE requires a service name"},
		},
		// DROP SERVICE IF EXISTS — missing name
		{
			"DROP SERVICE IF EXISTS missing name",
			"DROP SERVICE IF EXISTS",
			[]string{"DROP SERVICE requires a service name"},
		},

		// CREATE IMAGE REPOSITORY — missing name
		{
			"CREATE IMAGE REPOSITORY missing name",
			"CREATE IMAGE REPOSITORY",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — OR REPLACE without name
		{
			"CREATE OR REPLACE IMAGE REPOSITORY missing name",
			"CREATE OR REPLACE IMAGE REPOSITORY",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — IF NOT EXISTS without name
		{
			"CREATE IMAGE REPOSITORY IF NOT EXISTS missing name",
			"CREATE IMAGE REPOSITORY IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE IMAGE REPOSITORY"},
		},
		// CREATE IMAGE REPOSITORY — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE IMAGE REPOSITORY OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE IMAGE REPOSITORY IF NOT EXISTS my_repo",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE IMAGE REPOSITORY — unexpected property
		{
			"CREATE IMAGE REPOSITORY unexpected property",
			"CREATE IMAGE REPOSITORY my_repo TAG_POLICY = my_policy",
			[]string{"Unexpected property 'TAG_POLICY'"},
		},

		// DROP IMAGE REPOSITORY — missing name
		{
			"DROP IMAGE REPOSITORY missing name",
			"DROP IMAGE REPOSITORY",
			[]string{"DROP IMAGE REPOSITORY requires a repository name"},
		},
		// DROP IMAGE REPOSITORY IF EXISTS — missing name
		{
			"DROP IMAGE REPOSITORY IF EXISTS missing name",
			"DROP IMAGE REPOSITORY IF EXISTS",
			[]string{"DROP IMAGE REPOSITORY requires a repository name"},
		},

		// ALTER IMAGE REPOSITORY — unsupported
		{
			"ALTER IMAGE REPOSITORY unsupported",
			"ALTER IMAGE REPOSITORY my_repo SET COMMENT = 'test'",
			[]string{"ALTER IMAGE REPOSITORY is not supported"},
		},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
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


