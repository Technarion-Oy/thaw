package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Tag(t *testing.T) {
	validCases := []string{
		// ── CREATE TAG ───────────────────────────────────────────────────
		"CREATE TAG my_tag",
		"CREATE OR REPLACE TAG my_tag",
		"CREATE TAG IF NOT EXISTS my_tag",
		"CREATE TAG db.schema.my_tag",
		`CREATE TAG "My Tag"`,
		"CREATE TAG my_tag COMMENT = 'cost center tag'",
		"CREATE TAG my_tag ALLOWED_VALUES 'finance', 'engineering', 'marketing'",
		"CREATE TAG my_tag ALLOWED_VALUES 'a'",
		"CREATE OR REPLACE TAG cost_center ALLOWED_VALUES 'finance', 'hr' COMMENT = 'dept tag'",
		"CREATE TAG my_tag ALLOWED_VALUES 'it''s ok'",
		// ── ALTER TAG ────────────────────────────────────────────────────
		"ALTER TAG my_tag RENAME TO new_tag",
		"ALTER TAG db.schema.my_tag RENAME TO db.schema.new_tag",
		"ALTER TAG my_tag ADD ALLOWED_VALUES 'new_val'",
		"ALTER TAG my_tag ADD ALLOWED_VALUES 'v1', 'v2', 'v3'",
		"ALTER TAG my_tag DROP ALLOWED_VALUES 'old_val'",
		"ALTER TAG my_tag DROP ALLOWED_VALUES 'v1', 'v2'",
		"ALTER TAG my_tag UNSET ALLOWED_VALUES",
		"ALTER TAG my_tag SET COMMENT = 'updated tag'",
		"ALTER TAG my_tag UNSET COMMENT",
		"ALTER TAG IF EXISTS my_tag RENAME TO new_tag",
		"ALTER TAG IF EXISTS my_tag ADD ALLOWED_VALUES 'x'",
		"ALTER TAG IF EXISTS my_tag UNSET ALLOWED_VALUES",
		"ALTER TAG IF EXISTS my_tag SET COMMENT = 'c'",
		"ALTER TAG IF EXISTS my_tag UNSET COMMENT",
		// ── DROP TAG ─────────────────────────────────────────────────────
		"DROP TAG my_tag",
		"DROP TAG IF EXISTS my_tag",
		"DROP TAG db.schema.my_tag",
		`DROP TAG "My Tag"`,
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
		// ── CREATE TAG ───────────────────────────────────────────────────
		{
			"bare CREATE TAG without name",
			"CREATE TAG",
			[]string{"CREATE TAG requires a tag name"},
		},
		{
			"CREATE OR REPLACE TAG without name",
			"CREATE OR REPLACE TAG",
			[]string{"CREATE TAG requires a tag name"},
		},
		{
			"OR REPLACE + IF NOT EXISTS conflict",
			"CREATE OR REPLACE TAG IF NOT EXISTS my_tag",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"ALLOWED_VALUES with non-string value",
			"CREATE TAG my_tag ALLOWED_VALUES finance",
			[]string{"ALLOWED_VALUES requires a list of string literals"},
		},
		{
			"ALLOWED_VALUES with duplicate values",
			"CREATE TAG my_tag ALLOWED_VALUES 'finance', 'hr', 'finance'",
			[]string{"Duplicate value 'finance'"},
		},
		{
			"ALLOWED_VALUES with duplicate values case-insensitive",
			"CREATE TAG my_tag ALLOWED_VALUES 'Finance', 'FINANCE'",
			[]string{"Duplicate value"},
		},
		// ── ALTER TAG ────────────────────────────────────────────────────
		{
			"bare ALTER TAG without name",
			"ALTER TAG",
			[]string{"ALTER TAG requires a tag name"},
		},
		{
			"ALTER TAG with unknown sub-command",
			"ALTER TAG my_tag RESET",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG RENAME TO without new name",
			"ALTER TAG my_tag RENAME TO",
			[]string{"ALTER TAG RENAME TO requires a new tag name"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES without values",
			"ALTER TAG my_tag ADD ALLOWED_VALUES",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES with non-string",
			"ALTER TAG my_tag ADD ALLOWED_VALUES finance",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES without values",
			"ALTER TAG my_tag DROP ALLOWED_VALUES",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with non-string",
			"ALTER TAG my_tag DROP ALLOWED_VALUES finance",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with duplicate values",
			"ALTER TAG my_tag DROP ALLOWED_VALUES 'v1', 'v2', 'v1'",
			[]string{"Duplicate value 'v1'"},
		},
		{
			"ALTER TAG ADD ALLOWED_VALUES with case-insensitive duplicate",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'Finance', 'FINANCE'",
			[]string{"case-insensitive match with 'Finance'"},
		},
		{
			"ALTER TAG compound sub-commands",
			"ALTER TAG my_tag RENAME TO new_tag ADD ALLOWED_VALUES 'x'",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		{
			"ALTER TAG compound SET COMMENT and UNSET ALLOWED_VALUES",
			"ALTER TAG my_tag SET COMMENT = 'c' UNSET ALLOWED_VALUES",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		// ── DROP TAG ─────────────────────────────────────────────────────
		{
			"bare DROP TAG without name",
			"DROP TAG",
			[]string{"DROP TAG requires a tag name"},
		},
		{
			"DROP TAG with CASCADE",
			"DROP TAG my_tag CASCADE",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		{
			"DROP TAG with RESTRICT",
			"DROP TAG my_tag RESTRICT",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		{
			"DROP TAG IF EXISTS with CASCADE",
			"DROP TAG IF EXISTS my_tag CASCADE",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
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


