package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_ReplicationFailoverGroup(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE REPLICATION GROUP — minimal
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — DATABASES with ALLOWED_DATABASES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES ALLOWED_DATABASES = db1, db2 ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — INTEGRATIONS with ALLOWED_INTEGRATION_TYPES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = INTEGRATIONS ALLOWED_INTEGRATION_TYPES = SECURITY INTEGRATIONS ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE REPLICATION GROUP — multiple OBJECT_TYPES
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES, ROLES, WAREHOUSES ALLOWED_DATABASES = db1 ALLOWED_ACCOUNTS = org1.acct1, org2.acct2",
		// CREATE REPLICATION GROUP — with IGNORE EDITION CHECK
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 IGNORE EDITION CHECK",
		// CREATE REPLICATION GROUP — with REPLICATION_SCHEDULE
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 REPLICATION_SCHEDULE = '10 MINUTE'",
		"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1 REPLICATION_SCHEDULE = 'USING CRON 0 0 * * * UTC'",
		// CREATE FAILOVER GROUP — minimal
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE FAILOVER GROUP — with ALLOWED_FAILOVER_ACCOUNTS
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_FAILOVER_ACCOUNTS = org1.acct1",
		// CREATE FAILOVER GROUP — DATABASES
		"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = DATABASES ALLOWED_DATABASES = db1 ALLOWED_ACCOUNTS = org1.acct1",
		// CREATE OR REPLACE variants
		"CREATE OR REPLACE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		"CREATE OR REPLACE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// ALTER REPLICATION GROUP
		"ALTER REPLICATION GROUP my_rg ADD org1.acct2, org2.acct3",
		"ALTER REPLICATION GROUP my_rg REMOVE org1.acct2",
		"ALTER REPLICATION GROUP my_rg MOVE DATABASES db1, db2 TO REPLICATION GROUP other_rg",
		"ALTER REPLICATION GROUP my_rg SET REPLICATION_SCHEDULE = '30 MINUTE'",
		"ALTER REPLICATION GROUP my_rg SET OBJECT_TYPES = ROLES, WAREHOUSES",
		"ALTER REPLICATION GROUP my_rg RENAME TO new_rg_name",
		// ALTER FAILOVER GROUP
		"ALTER FAILOVER GROUP my_fg ADD org1.acct2",
		"ALTER FAILOVER GROUP my_fg REMOVE org1.acct2",
		"ALTER FAILOVER GROUP my_fg PRIMARY",
		"ALTER FAILOVER GROUP my_fg REFRESH",
		"ALTER FAILOVER GROUP my_fg SUSPEND",
		"ALTER FAILOVER GROUP my_fg RESUME",
		"ALTER FAILOVER GROUP my_fg MOVE DATABASES db1 TO REPLICATION GROUP other_rg",
		"ALTER FAILOVER GROUP my_fg SET REPLICATION_SCHEDULE = '10 MINUTE'",
		"ALTER FAILOVER GROUP my_fg RENAME TO new_fg",
		// Group name containing "databases" must not trigger false ALLOWED_DATABASES warning
		"CREATE REPLICATION GROUP databases_backup OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// Group name containing "integrations" must not trigger false ALLOWED_INTEGRATION_TYPES warning
		"CREATE REPLICATION GROUP integrations_sync OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
		// ALTER FAILOVER GROUP with inline comment after PRIMARY/REFRESH
		"ALTER FAILOVER GROUP my_fg PRIMARY -- promote to primary",
		"ALTER FAILOVER GROUP my_fg REFRESH -- manual refresh",
		// DROP REPLICATION GROUP
		"DROP REPLICATION GROUP my_rg",
		"DROP REPLICATION GROUP IF EXISTS my_rg",
		// DROP FAILOVER GROUP
		"DROP FAILOVER GROUP my_fg",
		"DROP FAILOVER GROUP IF EXISTS my_fg",
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
		// CREATE REPLICATION GROUP — missing mandatory clauses
		{
			"CREATE REPL GROUP missing name",
			"CREATE REPLICATION GROUP",
			[]string{"CREATE REPLICATION GROUP requires a group name"},
		},
		{
			"CREATE REPL GROUP missing OBJECT_TYPES",
			"CREATE REPLICATION GROUP my_rg ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"Missing mandatory OBJECT_TYPES"},
		},
		{
			"CREATE REPL GROUP missing ALLOWED_ACCOUNTS",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = ROLES",
			[]string{"Missing mandatory ALLOWED_ACCOUNTS"},
		},
		{
			"CREATE REPL GROUP DATABASES without ALLOWED_DATABASES",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		{
			"CREATE REPL GROUP INTEGRATIONS without ALLOWED_INTEGRATION_TYPES",
			"CREATE REPLICATION GROUP my_rg OBJECT_TYPES = INTEGRATIONS ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes INTEGRATIONS but ALLOWED_INTEGRATION_TYPES is missing"},
		},
		{
			"CREATE REPL GROUP db.schema prefix",
			"CREATE REPLICATION GROUP mydb.my_rg OBJECT_TYPES = ROLES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE FAILOVER GROUP — missing mandatory clauses
		{
			"CREATE FAILOVER GROUP missing name",
			"CREATE FAILOVER GROUP",
			[]string{"CREATE FAILOVER GROUP requires a group name"},
		},
		{
			"CREATE FAILOVER GROUP missing OBJECT_TYPES",
			"CREATE FAILOVER GROUP my_fg ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"Missing mandatory OBJECT_TYPES"},
		},
		{
			"CREATE FAILOVER GROUP missing ALLOWED_ACCOUNTS",
			"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = ROLES",
			[]string{"Missing mandatory ALLOWED_ACCOUNTS or ALLOWED_FAILOVER_ACCOUNTS"},
		},
		{
			"CREATE FAILOVER GROUP DATABASES without ALLOWED_DATABASES",
			"CREATE FAILOVER GROUP my_fg OBJECT_TYPES = DATABASES ALLOWED_ACCOUNTS = org1.acct1",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		// ALTER REPLICATION GROUP — missing action
		{
			"ALTER REPL GROUP missing name",
			"ALTER REPLICATION GROUP",
			[]string{"ALTER REPLICATION GROUP requires a group name"},
		},
		{
			"ALTER REPL GROUP missing action",
			"ALTER REPLICATION GROUP my_rg",
			[]string{"ALTER REPLICATION GROUP requires an action"},
		},
		{
			"ALTER REPL GROUP MOVE DATABASES without TO",
			"ALTER REPLICATION GROUP my_rg MOVE DATABASES db1",
			[]string{"MOVE DATABASES in ALTER REPLICATION GROUP requires TO REPLICATION GROUP"},
		},
		// ALTER FAILOVER GROUP — missing action
		{
			"ALTER FAILOVER GROUP missing name",
			"ALTER FAILOVER GROUP",
			[]string{"ALTER FAILOVER GROUP requires a group name"},
		},
		{
			"ALTER FAILOVER GROUP missing action",
			"ALTER FAILOVER GROUP my_fg",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP MOVE DATABASES without TO",
			"ALTER FAILOVER GROUP my_fg MOVE DATABASES db1",
			[]string{"MOVE DATABASES in ALTER FAILOVER GROUP requires TO REPLICATION GROUP"},
		},
		// OBJECT_TYPES at end of statement (no trailing keyword)
		{
			"OBJECT_TYPES = DATABASES at end, missing ALLOWED_DATABASES",
			"CREATE REPLICATION GROUP my_rg ALLOWED_ACCOUNTS = org1.acct1 OBJECT_TYPES = DATABASES",
			[]string{"OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing"},
		},
		// Group named after an action keyword — must still detect missing action
		{
			"ALTER FAILOVER GROUP named primary with no action",
			"ALTER FAILOVER GROUP primary",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named refresh with no action",
			"ALTER FAILOVER GROUP refresh",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named suspend with no action",
			"ALTER FAILOVER GROUP suspend",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		{
			"ALTER FAILOVER GROUP named resume with no action",
			"ALTER FAILOVER GROUP resume",
			[]string{"ALTER FAILOVER GROUP requires an action"},
		},
		// ALTER — account-level prefix check
		{
			"ALTER REPL GROUP db.schema prefix",
			"ALTER REPLICATION GROUP mydb.my_rg ADD org1.acct2",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"ALTER FAILOVER GROUP db.schema prefix",
			"ALTER FAILOVER GROUP mydb.my_fg PRIMARY",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// DROP — account-level prefix check
		{
			"DROP REPL GROUP db.schema prefix",
			"DROP REPLICATION GROUP mydb.my_rg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"DROP FAILOVER GROUP db.schema prefix",
			"DROP FAILOVER GROUP mydb.my_fg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// DROP — missing name
		{
			"DROP REPL GROUP missing name",
			"DROP REPLICATION GROUP",
			[]string{"DROP REPLICATION GROUP requires a group name"},
		},
		{
			"DROP FAILOVER GROUP missing name",
			"DROP FAILOVER GROUP",
			[]string{"DROP FAILOVER GROUP requires a group name"},
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


