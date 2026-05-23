package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_ApplicationPackageAndApplication(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE APPLICATION PACKAGE — basic
		"CREATE APPLICATION PACKAGE my_pkg",
		"CREATE OR REPLACE APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION PACKAGE IF NOT EXISTS my_pkg",
		// CREATE APPLICATION PACKAGE — with DISTRIBUTION
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL",
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = EXTERNAL",
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = internal",
		// CREATE APPLICATION PACKAGE — with COMMENT
		"CREATE APPLICATION PACKAGE my_pkg COMMENT = 'my package'",
		// CREATE APPLICATION PACKAGE — with both DISTRIBUTION and COMMENT
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL COMMENT = 'internal pkg'",
		// CREATE APPLICATION PACKAGE — case insensitive
		"create application package my_pkg",
		"Create Application Package IF NOT EXISTS my_pkg",

		// ALTER APPLICATION PACKAGE — SET DEFAULT RELEASE DIRECTIVE
		"ALTER APPLICATION PACKAGE my_pkg SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 0",
		// ALTER APPLICATION PACKAGE — ADD VERSION
		"ALTER APPLICATION PACKAGE my_pkg ADD VERSION v1 USING @stage/path",
		// ALTER APPLICATION PACKAGE — DROP VERSION
		"ALTER APPLICATION PACKAGE my_pkg DROP VERSION v1",
		// ALTER APPLICATION PACKAGE — SET DISTRIBUTION
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = INTERNAL",
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = EXTERNAL",

		// DROP APPLICATION PACKAGE
		"DROP APPLICATION PACKAGE my_pkg",
		"DROP APPLICATION PACKAGE IF EXISTS my_pkg",

		// CREATE APPLICATION — basic with FROM APPLICATION PACKAGE
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg",
		"CREATE OR REPLACE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg",
		// CREATE APPLICATION — with USING VERSION and PATCH
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0",
		// CREATE APPLICATION — with DEBUG_MODE
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = TRUE",
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = FALSE",
		// CREATE APPLICATION — with COMMENT
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg COMMENT = 'my app'",
		// CREATE APPLICATION — with all optional properties
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0 DEBUG_MODE = TRUE COMMENT = 'full opts'",
		// CREATE APPLICATION — case insensitive
		"create application my_app from application package my_pkg",

		// ALTER APPLICATION — UPGRADE
		"ALTER APPLICATION my_app UPGRADE",
		// ALTER APPLICATION — UPGRADE USING VERSION ... PATCH ...
		"ALTER APPLICATION my_app UPGRADE USING VERSION v2 PATCH 1",
		// ALTER APPLICATION — SET DEBUG_MODE
		"ALTER APPLICATION my_app SET DEBUG_MODE = TRUE",
		"ALTER APPLICATION my_app SET DEBUG_MODE = FALSE",
		// ALTER APPLICATION — UNSET DEBUG_MODE
		"ALTER APPLICATION my_app UNSET DEBUG_MODE",

		// DROP APPLICATION
		"DROP APPLICATION my_app",
		"DROP APPLICATION IF EXISTS my_app",
		"DROP APPLICATION my_app CASCADE",
		"DROP APPLICATION IF EXISTS my_app CASCADE",
	}

	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 60)], func(t *testing.T) {
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
		// CREATE APPLICATION PACKAGE — missing name
		{
			"CREATE APPLICATION PACKAGE missing name",
			"CREATE APPLICATION PACKAGE",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — OR REPLACE without name
		{
			"CREATE OR REPLACE APPLICATION PACKAGE missing name",
			"CREATE OR REPLACE APPLICATION PACKAGE",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — IF NOT EXISTS without name
		{
			"CREATE APPLICATION PACKAGE IF NOT EXISTS missing name",
			"CREATE APPLICATION PACKAGE IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION PACKAGE — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE APPLICATION PACKAGE OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE APPLICATION PACKAGE IF NOT EXISTS my_pkg",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE APPLICATION PACKAGE — account-level prefix
		{
			"CREATE APPLICATION PACKAGE with db prefix",
			"CREATE APPLICATION PACKAGE db.my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION PACKAGE — account-level prefix (three-part name)
		{
			"CREATE APPLICATION PACKAGE with three-part name",
			"CREATE APPLICATION PACKAGE db.schema.my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION PACKAGE — invalid DISTRIBUTION
		{
			"CREATE APPLICATION PACKAGE invalid DISTRIBUTION",
			"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = PUBLIC",
			[]string{"DISTRIBUTION must be INTERNAL or EXTERNAL"},
		},
		// CREATE APPLICATION PACKAGE — unexpected property
		{
			"CREATE APPLICATION PACKAGE unexpected property",
			"CREATE APPLICATION PACKAGE my_pkg DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},

		// ALTER APPLICATION PACKAGE — missing name
		{
			"ALTER APPLICATION PACKAGE missing name",
			"ALTER APPLICATION PACKAGE",
			[]string{"ALTER APPLICATION PACKAGE requires a package name"},
		},
		// ALTER APPLICATION PACKAGE — unknown sub-command
		{
			"ALTER APPLICATION PACKAGE unknown action",
			"ALTER APPLICATION PACKAGE my_pkg ENABLE",
			[]string{"Unknown ALTER APPLICATION PACKAGE sub-command"},
		},
		// ALTER APPLICATION PACKAGE — SET with unknown property
		{
			"ALTER APPLICATION PACKAGE SET unknown property",
			"ALTER APPLICATION PACKAGE my_pkg SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER APPLICATION PACKAGE SET"},
		},
		// ALTER APPLICATION PACKAGE — invalid DISTRIBUTION value
		{
			"ALTER APPLICATION PACKAGE invalid DISTRIBUTION",
			"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = PUBLIC",
			[]string{"DISTRIBUTION must be INTERNAL or EXTERNAL"},
		},

		// DROP APPLICATION PACKAGE — missing name
		{
			"DROP APPLICATION PACKAGE missing name",
			"DROP APPLICATION PACKAGE",
			[]string{"DROP APPLICATION PACKAGE requires a package name"},
		},
		// DROP APPLICATION PACKAGE IF EXISTS — missing name
		{
			"DROP APPLICATION PACKAGE IF EXISTS missing name",
			"DROP APPLICATION PACKAGE IF EXISTS",
			[]string{"DROP APPLICATION PACKAGE requires a package name"},
		},

		// CREATE APPLICATION — missing name
		{
			"CREATE APPLICATION missing name",
			"CREATE APPLICATION",
			[]string{"Unexpected syntax in CREATE APPLICATION"},
		},
		// CREATE APPLICATION — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE APPLICATION OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE APPLICATION — account-level prefix
		{
			"CREATE APPLICATION with db prefix",
			"CREATE APPLICATION db.my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		// CREATE APPLICATION — missing FROM APPLICATION PACKAGE
		{
			"CREATE APPLICATION missing FROM APPLICATION PACKAGE",
			"CREATE APPLICATION my_app",
			[]string{"Missing mandatory FROM APPLICATION PACKAGE"},
		},
		// CREATE APPLICATION — USING VERSION without PATCH
		{
			"CREATE APPLICATION VERSION without PATCH",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1",
			[]string{"USING VERSION requires a PATCH number"},
		},
		// CREATE APPLICATION — DEBUG_MODE invalid value
		{
			"CREATE APPLICATION DEBUG_MODE invalid",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = MAYBE",
			[]string{"DEBUG_MODE must be TRUE or FALSE"},
		},
		// CREATE APPLICATION — unexpected property
		{
			"CREATE APPLICATION unexpected property",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DATA_RETENTION = 90",
			[]string{"Unexpected property 'DATA_RETENTION'"},
		},
		// CREATE APPLICATION — IF NOT EXISTS without name
		{
			"CREATE APPLICATION IF NOT EXISTS missing name",
			"CREATE APPLICATION IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE APPLICATION"},
		},

		// ALTER APPLICATION — missing name
		{
			"ALTER APPLICATION missing name",
			"ALTER APPLICATION",
			[]string{"ALTER APPLICATION requires an application name"},
		},
		// ALTER APPLICATION — unknown sub-command
		{
			"ALTER APPLICATION unknown action",
			"ALTER APPLICATION my_app ENABLE",
			[]string{"Unknown ALTER APPLICATION sub-command"},
		},
		// ALTER APPLICATION — SET with unknown property
		{
			"ALTER APPLICATION SET unknown property",
			"ALTER APPLICATION my_app SET UNKNOWN_PROP = 1",
			[]string{"Unknown property in ALTER APPLICATION SET"},
		},
		// ALTER APPLICATION — UNSET with unknown property
		{
			"ALTER APPLICATION UNSET unknown property",
			"ALTER APPLICATION my_app UNSET UNKNOWN_PROP",
			[]string{"Unknown property in ALTER APPLICATION UNSET"},
		},
		// ALTER APPLICATION — DEBUG_MODE invalid value
		{
			"ALTER APPLICATION DEBUG_MODE invalid",
			"ALTER APPLICATION my_app SET DEBUG_MODE = MAYBE",
			[]string{"DEBUG_MODE must be TRUE or FALSE"},
		},
		// ALTER APPLICATION — UPGRADE USING VERSION without PATCH
		{
			"ALTER APPLICATION UPGRADE VERSION without PATCH",
			"ALTER APPLICATION my_app UPGRADE USING VERSION v2",
			[]string{"USING VERSION requires a PATCH number"},
		},

		// DROP APPLICATION — missing name
		{
			"DROP APPLICATION missing name",
			"DROP APPLICATION",
			[]string{"DROP APPLICATION requires an application name"},
		},
		// DROP APPLICATION IF EXISTS — missing name
		{
			"DROP APPLICATION IF EXISTS missing name",
			"DROP APPLICATION IF EXISTS",
			[]string{"DROP APPLICATION requires an application name"},
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


