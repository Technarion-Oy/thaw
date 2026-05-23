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

func TestValidateSnowflakePatterns_Application_QuotedIdentifiers(t *testing.T) {
	validCases := []string{
		// CREATE APPLICATION PACKAGE with quoted name
		`CREATE APPLICATION PACKAGE "my-pkg"`,
		`CREATE OR REPLACE APPLICATION PACKAGE "My Pkg"`,
		`CREATE APPLICATION PACKAGE IF NOT EXISTS "123_pkg"`,
		// ALTER APPLICATION PACKAGE with quoted name
		`ALTER APPLICATION PACKAGE "my-pkg" SET DISTRIBUTION = INTERNAL`,
		`ALTER APPLICATION PACKAGE "my-pkg" ADD VERSION v1 USING @stage/path`,
		`ALTER APPLICATION PACKAGE "my-pkg" DROP VERSION v1`,
		// DROP APPLICATION PACKAGE with quoted name
		`DROP APPLICATION PACKAGE "my-pkg"`,
		`DROP APPLICATION PACKAGE IF EXISTS "my-pkg"`,
		// CREATE APPLICATION with quoted names
		`CREATE APPLICATION "my-app" FROM APPLICATION PACKAGE "my-pkg"`,
		`CREATE OR REPLACE APPLICATION "my-app" FROM APPLICATION PACKAGE "my-pkg"`,
		`CREATE APPLICATION IF NOT EXISTS "my-app" FROM APPLICATION PACKAGE "my-pkg"`,
		// ALTER APPLICATION with quoted name
		`ALTER APPLICATION "my-app" UPGRADE`,
		`ALTER APPLICATION "my-app" SET DEBUG_MODE = TRUE`,
		`ALTER APPLICATION "my-app" UNSET DEBUG_MODE`,
		// DROP APPLICATION with quoted name
		`DROP APPLICATION "my-app"`,
		`DROP APPLICATION IF EXISTS "my-app"`,
		`DROP APPLICATION "my-app" CASCADE`,
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_SQLComments(t *testing.T) {
	validCases := []string{
		// Block comment before name
		"CREATE APPLICATION PACKAGE /* comment */ my_pkg",
		// Line comment at end
		"CREATE APPLICATION PACKAGE my_pkg -- this is a pkg",
		// Block comment in ALTER
		"ALTER APPLICATION PACKAGE /* pkg */ my_pkg SET DISTRIBUTION = INTERNAL",
		// Line comment in CREATE APPLICATION
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg -- deploy",
		// Block comment between clauses
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg /* v1 */ USING VERSION v1 PATCH 0",
		// ALTER APPLICATION with comment
		"ALTER APPLICATION my_app /* upgrade */ UPGRADE",
		// DROP with comment
		"DROP APPLICATION PACKAGE /* cleanup */ my_pkg",
		"DROP APPLICATION /* cleanup */ my_app CASCADE",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_LeadingWhitespace(t *testing.T) {
	validCases := []string{
		"  CREATE APPLICATION PACKAGE my_pkg",
		"\tCREATE APPLICATION PACKAGE my_pkg",
		"\n  CREATE APPLICATION PACKAGE my_pkg",
		"  ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = INTERNAL",
		"  DROP APPLICATION PACKAGE my_pkg",
		"  CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg",
		"\tALTER APPLICATION my_app UPGRADE",
		"  DROP APPLICATION my_app",
	}

	for _, sql := range validCases {
		name := strings.TrimSpace(sql)
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_LowercaseBooleans(t *testing.T) {
	validCases := []string{
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = true",
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = false",
		"ALTER APPLICATION my_app SET DEBUG_MODE = true",
		"ALTER APPLICATION my_app SET DEBUG_MODE = false",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_ThreePartPrefix(t *testing.T) {
	// CREATE APPLICATION with three-part name (only two-part was tested before).
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"CREATE APPLICATION with three-part name",
			"CREATE APPLICATION db.schema.my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"CREATE OR REPLACE APPLICATION with three-part name",
			"CREATE OR REPLACE APPLICATION db.schema.my_app FROM APPLICATION PACKAGE my_pkg",
			[]string{"account-level objects and cannot have a database or schema prefix"},
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

func TestValidateSnowflakePatterns_Application_MultipleErrors(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"CREATE APPLICATION prefix + missing FROM",
			"CREATE APPLICATION db.my_app",
			[]string{
				"account-level objects and cannot have a database or schema prefix",
				"Missing mandatory FROM APPLICATION PACKAGE",
			},
		},
		{
			"CREATE APPLICATION invalid DEBUG_MODE + unexpected property",
			"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = MAYBE DATA_RETENTION = 90",
			[]string{
				"DEBUG_MODE must be TRUE or FALSE",
				"Unexpected property 'DATA_RETENTION'",
			},
		},
		{
			"CREATE APPLICATION PACKAGE invalid DISTRIBUTION + unexpected property",
			"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = PUBLIC DATA_RETENTION = 90",
			[]string{
				"DISTRIBUTION must be INTERNAL or EXTERNAL",
				"Unexpected property 'DATA_RETENTION'",
			},
		},
		{
			"CREATE APPLICATION prefix + USING VERSION without PATCH",
			"CREATE APPLICATION db.my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1",
			[]string{
				"account-level objects and cannot have a database or schema prefix",
				"USING VERSION requires a PATCH number",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) < len(tc.wantMsgs) {
				t.Errorf("Expected at least %d warnings for %q, got %d: %v",
					len(tc.wantMsgs), tc.sql, len(warns), warns)
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

func TestValidateSnowflakePatterns_Application_ORReplaceWithProperties(t *testing.T) {
	validCases := []string{
		// OR REPLACE + DISTRIBUTION
		"CREATE OR REPLACE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL",
		"CREATE OR REPLACE APPLICATION PACKAGE my_pkg DISTRIBUTION = EXTERNAL COMMENT = 'pkg'",
		// OR REPLACE + all CREATE APPLICATION properties
		"CREATE OR REPLACE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = TRUE COMMENT = 'app'",
		"CREATE OR REPLACE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_AlterDistributionLowercase(t *testing.T) {
	validCases := []string{
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = internal",
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = external",
		"alter application package my_pkg set distribution = Internal",
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = External",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_IFNotExistsWithProperties(t *testing.T) {
	validCases := []string{
		"CREATE APPLICATION PACKAGE IF NOT EXISTS my_pkg DISTRIBUTION = INTERNAL",
		"CREATE APPLICATION PACKAGE IF NOT EXISTS my_pkg COMMENT = 'pkg'",
		"CREATE APPLICATION PACKAGE IF NOT EXISTS my_pkg DISTRIBUTION = EXTERNAL COMMENT = 'pkg'",
		"CREATE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg USING VERSION v1 PATCH 0",
		"CREATE APPLICATION IF NOT EXISTS my_app FROM APPLICATION PACKAGE my_pkg DEBUG_MODE = TRUE",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_AlterSetDefaultReleaseDirective(t *testing.T) {
	validCases := []string{
		"ALTER APPLICATION PACKAGE my_pkg SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 0",
		"ALTER APPLICATION PACKAGE my_pkg SET DEFAULT RELEASE DIRECTIVE VERSION = v2 PATCH = 3",
		// Quoted package name
		`ALTER APPLICATION PACKAGE "my-pkg" SET DEFAULT RELEASE DIRECTIVE VERSION = v1 PATCH = 0`,
		// Case insensitive
		"alter application package my_pkg set default release directive version = v1 patch = 0",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_AlterAddDropVersion(t *testing.T) {
	validCases := []string{
		// ADD VERSION variations
		"ALTER APPLICATION PACKAGE my_pkg ADD VERSION v1 USING @stage/path",
		"ALTER APPLICATION PACKAGE my_pkg ADD VERSION v2_beta USING @my_db.my_schema.my_stage/manifest.yml",
		// DROP VERSION variations
		"ALTER APPLICATION PACKAGE my_pkg DROP VERSION v1",
		"ALTER APPLICATION PACKAGE my_pkg DROP VERSION v2_beta",
		// Case insensitive
		"alter application package my_pkg add version v1 using @stage/path",
		"alter application package my_pkg drop version v1",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_UpgradeVariants(t *testing.T) {
	validCases := []string{
		"ALTER APPLICATION my_app UPGRADE",
		"ALTER APPLICATION my_app UPGRADE USING VERSION v1 PATCH 0",
		"ALTER APPLICATION my_app UPGRADE USING VERSION v2 PATCH 3",
		// Case insensitive
		"alter application my_app upgrade",
		"alter application my_app upgrade using version v1 patch 0",
		// Quoted name
		`ALTER APPLICATION "my-app" UPGRADE`,
		`ALTER APPLICATION "my-app" UPGRADE USING VERSION v1 PATCH 0`,
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_TrailingSemicolon(t *testing.T) {
	// Statements with trailing semicolons should validate without warnings.
	validCases := []string{
		"CREATE APPLICATION PACKAGE my_pkg;",
		"CREATE APPLICATION PACKAGE my_pkg DISTRIBUTION = INTERNAL;",
		"ALTER APPLICATION PACKAGE my_pkg SET DISTRIBUTION = EXTERNAL;",
		"ALTER APPLICATION PACKAGE my_pkg ADD VERSION v1 USING @stage/path;",
		"DROP APPLICATION PACKAGE my_pkg;",
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg;",
		"ALTER APPLICATION my_app UPGRADE;",
		"ALTER APPLICATION my_app SET DEBUG_MODE = TRUE;",
		"DROP APPLICATION my_app;",
		"DROP APPLICATION my_app CASCADE;",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_MultiLineSQL(t *testing.T) {
	validCases := []string{
		"CREATE APPLICATION PACKAGE\n  my_pkg\n  DISTRIBUTION = INTERNAL",
		"CREATE APPLICATION\n  my_app\n  FROM APPLICATION PACKAGE my_pkg",
		"CREATE APPLICATION my_app\nFROM APPLICATION PACKAGE my_pkg\nUSING VERSION v1 PATCH 0",
		"ALTER APPLICATION PACKAGE\n  my_pkg\n  SET DISTRIBUTION = EXTERNAL",
		"ALTER APPLICATION\n  my_app\n  UPGRADE",
	}

	for _, sql := range validCases {
		name := strings.ReplaceAll(sql, "\n", " ")
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_MultiStatement(t *testing.T) {
	// Two application statements separated by semicolons should each
	// validate independently.
	sql := "CREATE APPLICATION PACKAGE my_pkg;\nCREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg;"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) > 0 {
		t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
	}

	// One valid + one invalid statement: only the invalid one should warn.
	sql2 := "CREATE APPLICATION PACKAGE my_pkg;\nCREATE APPLICATION my_app;"
	ranges2 := GetStatementRanges(sql2)
	markers2 := ValidateSnowflakePatterns(sql2, ranges2)
	warns2 := getWarnings(markers2)
	if len(warns2) == 0 {
		t.Errorf("Expected warnings for the invalid statement, got 0")
	}
	found := false
	for _, w := range warns2 {
		if strings.Contains(w.Message, "Missing mandatory FROM APPLICATION PACKAGE") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'Missing mandatory FROM APPLICATION PACKAGE' warning, got: %v", warns2)
	}
}

func TestValidateSnowflakePatterns_Application_CreateAppMissingFromWithVersion(t *testing.T) {
	// CREATE APPLICATION with USING VERSION but no FROM clause should
	// produce both "missing FROM" and "USING VERSION requires PATCH" errors.
	tests := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"missing FROM + USING VERSION without PATCH",
			"CREATE APPLICATION my_app USING VERSION v1",
			[]string{
				"Missing mandatory FROM APPLICATION PACKAGE",
				"USING VERSION requires a PATCH number",
			},
		},
		{
			"missing FROM + USING VERSION with PATCH",
			"CREATE APPLICATION my_app USING VERSION v1 PATCH 0",
			[]string{
				"Missing mandatory FROM APPLICATION PACKAGE",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) < len(tc.wantMsgs) {
				t.Errorf("Expected at least %d warnings for %q, got %d: %v",
					len(tc.wantMsgs), tc.sql, len(warns), warns)
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

func TestValidateSnowflakePatterns_Application_StringLiteralProperty(t *testing.T) {
	// Property-like text inside a COMMENT string literal should not
	// trigger "Unexpected property" warnings because validateProperties
	// strips string literals before scanning.
	validCases := []string{
		"CREATE APPLICATION PACKAGE my_pkg COMMENT = 'DISTRIBUTION = FOO'",
		"CREATE APPLICATION PACKAGE my_pkg COMMENT = 'DATA_RETENTION = 90'",
		"CREATE APPLICATION my_app FROM APPLICATION PACKAGE my_pkg COMMENT = 'DEBUG_MODE = MAYBE'",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Application_AlterPackageQualifiedName(t *testing.T) {
	// ALTER APPLICATION PACKAGE does not check for account-level prefix
	// in the source, so dotted names should be accepted without warning.
	validCases := []string{
		"ALTER APPLICATION PACKAGE db.my_pkg SET DISTRIBUTION = INTERNAL",
		"ALTER APPLICATION PACKAGE db.schema.my_pkg ADD VERSION v1 USING @stage/path",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}


