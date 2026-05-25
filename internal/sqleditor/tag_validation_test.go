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
		"CREATE TAG IF NOT EXISTS my_tag ALLOWED_VALUES 'x', 'y' COMMENT = 'combined'",
		"CREATE TAG IF NOT EXISTS db.schema.my_tag COMMENT = 'with schema'",
		// ── CREATE TAG: case insensitivity ───────────────────────────────
		"create tag my_tag",
		"Create Or Replace Tag my_tag",
		"create tag if not exists my_tag ALLOWED_VALUES 'a', 'b'",
		// ── CREATE TAG: trailing comment should not cause false positives ─
		"CREATE TAG my_tag COMMENT = 'c' -- inline comment",
		// ── CREATE TAG: leading whitespace ───────────────────────────────
		"   CREATE TAG my_tag",
		"\t CREATE OR REPLACE TAG my_tag COMMENT = 'c'",
		// ── CREATE TAG: empty string and escaped-quote-only values ────
		"CREATE TAG my_tag ALLOWED_VALUES ''",
		"CREATE TAG my_tag ALLOWED_VALUES ''''",
		"CREATE TAG my_tag ALLOWED_VALUES '', 'normal'",
		// ── CREATE TAG: multiline statement ──────────────────────────
		"CREATE TAG\n  my_tag\n  ALLOWED_VALUES 'a', 'b'",
		"CREATE TAG\n  my_tag\n  COMMENT = 'c'",
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
		`ALTER TAG "My Tag" RENAME TO "New Tag"`,
		"ALTER TAG db.schema.tag1 ADD ALLOWED_VALUES 'v1', 'v2', 'v3', 'v4'",
		// ── ALTER TAG: fully-qualified with ADD/DROP ALLOWED_VALUES ───────
		"ALTER TAG db.schema.my_tag ADD ALLOWED_VALUES 'v1', 'v2'",
		"ALTER TAG db.schema.my_tag DROP ALLOWED_VALUES 'v1'",
		// ── ALTER TAG: escaped quotes in values ─────────────────────────
		"ALTER TAG my_tag ADD ALLOWED_VALUES 'it''s ok', 'fine'",
		"ALTER TAG my_tag DROP ALLOWED_VALUES 'it''s ok'",
		// ── ALTER TAG: IF EXISTS + DROP ALLOWED_VALUES ───────────────────
		"ALTER TAG IF EXISTS my_tag DROP ALLOWED_VALUES 'v1'",
		// ── ALTER TAG: SET COMMENT with empty string value ───────────────
		"ALTER TAG my_tag SET COMMENT = ''",
		"ALTER TAG IF EXISTS my_tag SET COMMENT = ''",
		// ── ALTER TAG: fully-qualified + IF EXISTS + UNSET COMMENT ───────
		"ALTER TAG IF EXISTS db.schema.my_tag UNSET COMMENT",
		// ── ALTER TAG: case insensitivity ────────────────────────────────
		"alter tag my_tag rename to new_tag",
		"Alter Tag my_tag Set Comment = 'c'",
		"alter tag my_tag add allowed_values 'a'",
		"alter tag my_tag drop allowed_values 'a'",
		"alter tag my_tag unset allowed_values",
		"alter tag my_tag unset comment",
		// ── ALTER TAG: trailing comment should not cause false positives ─
		"ALTER TAG my_tag RENAME TO new_tag -- inline comment",
		"ALTER TAG my_tag SET COMMENT = 'c' -- note",
		// ── DROP TAG ─────────────────────────────────────────────────────
		"DROP TAG my_tag",
		"DROP TAG IF EXISTS my_tag",
		"DROP TAG db.schema.my_tag",
		`DROP TAG "My Tag"`,
		"DROP TAG IF EXISTS db.schema.my_tag",
		// ── DROP TAG: case insensitivity ─────────────────────────────────
		"drop tag my_tag",
		"drop tag if exists my_tag",
		// ── DROP TAG: leading whitespace ─────────────────────────────────
		"   DROP TAG my_tag",
		"\t DROP TAG IF EXISTS my_tag",
		// ── DROP TAG: trailing comment ───────────────────────────────────
		"DROP TAG my_tag -- inline comment",
		"DROP TAG IF EXISTS my_tag -- inline comment",
		// ── ALTER TAG: leading whitespace ────────────────────────────────
		"   ALTER TAG my_tag RENAME TO new_tag",
		"\t ALTER TAG IF EXISTS my_tag SET COMMENT = 'c'",
		// ── ALTER TAG: multiline statement ───────────────────────────────
		"ALTER TAG\n  my_tag\n  RENAME TO new_tag",
		"ALTER TAG\n  my_tag\n  ADD ALLOWED_VALUES 'a', 'b'",
		// ── DROP TAG: multiline statement ────────────────────────────────
		"DROP TAG\n  my_tag",
		"DROP TAG\n  IF EXISTS\n  my_tag",
		// ── CREATE TAG: empty and escaped-quote COMMENT values ───────────
		"CREATE TAG my_tag COMMENT = ''",
		"CREATE TAG my_tag COMMENT = 'it''s a tag'",
		// ── CREATE TAG: COMMENT before ALLOWED_VALUES (reverse order) ────
		"CREATE TAG my_tag COMMENT = 'c' ALLOWED_VALUES 'a', 'b'",
		// ── ALTER TAG: IF EXISTS with quoted identifiers ──────────────────
		`ALTER TAG IF EXISTS "My Tag" RENAME TO "New Tag"`,
		// ── ALTER TAG: escaped quote in SET COMMENT value ─────────────────
		"ALTER TAG my_tag SET COMMENT = 'it''s escaped'",
		// ── ALTER TAG: fully-qualified name + SET COMMENT ─────────────────
		"ALTER TAG db.schema.my_tag SET COMMENT = 'c'",
		// ── ALTER TAG: fully-qualified name + UNSET ALLOWED_VALUES ────────
		"ALTER TAG db.schema.my_tag UNSET ALLOWED_VALUES",
		// ── CREATE TAG: fully-qualified name with ALLOWED_VALUES ─────────
		"CREATE TAG db.schema.my_tag ALLOWED_VALUES 'a', 'b'",
		// ── CREATE TAG: ALLOWED_VALUES value containing comma ────────────
		"CREATE TAG my_tag ALLOWED_VALUES 'a,b', 'c'",
		// ── CREATE TAG: extra whitespace around commas in ALLOWED_VALUES ─
		"CREATE TAG my_tag ALLOWED_VALUES 'a'  ,  'b'  ,  'c'",
		// ── CREATE TAG: ALLOWED_VALUES containing SQL keywords (cleanParseText) ─
		"CREATE TAG my_tag ALLOWED_VALUES 'DROP', 'CASCADE', 'RESTRICT'",
		"CREATE TAG my_tag ALLOWED_VALUES 'ALTER TAG', 'RENAME TO', 'ADD ALLOWED_VALUES'",
		// ── ALTER TAG: COMMENT value containing = sign and keywords ───────
		"ALTER TAG my_tag SET COMMENT = 'key = value RENAME TO something'",
		// ── CREATE TAG: multiple escaped-quote values in a list ───────────
		"CREATE TAG my_tag ALLOWED_VALUES 'it''s', 'won''t', 'they''re'",
		// ── ALTER TAG: ADD/DROP ALLOWED_VALUES with single empty string ──
		"ALTER TAG my_tag ADD ALLOWED_VALUES ''",
		"ALTER TAG my_tag DROP ALLOWED_VALUES ''",
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
			"OR REPLACE + IF NOT EXISTS conflict without name",
			"CREATE OR REPLACE TAG IF NOT EXISTS",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"CREATE TAG with unknown property",
			"CREATE TAG my_tag DATA_RETENTION_TIME_IN_DAYS = 1",
			[]string{"Unexpected property 'DATA_RETENTION_TIME_IN_DAYS'"},
		},
		{
			"CREATE TAG with ALLOWED_VALUES and unknown property",
			"CREATE TAG my_tag ALLOWED_VALUES 'a', 'b' DATA_RETENTION_TIME_IN_DAYS = 1",
			[]string{"Unexpected property 'DATA_RETENTION_TIME_IN_DAYS'"},
		},
		{
			"ALLOWED_VALUES with numeric value",
			"CREATE TAG my_tag ALLOWED_VALUES 123",
			[]string{"ALLOWED_VALUES requires a list of string literals"},
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
		// ── CREATE TAG: triple duplicate produces two warnings ───────────
		{
			"CREATE TAG ALLOWED_VALUES with triple duplicate",
			"CREATE TAG my_tag ALLOWED_VALUES 'a', 'a', 'a'",
			[]string{"Duplicate value 'a'"},
		},
		// ── ALTER TAG ────────────────────────────────────────────────────
		{
			"bare ALTER TAG without name",
			"ALTER TAG",
			[]string{"ALTER TAG requires a tag name"},
		},
		{
			"ALTER TAG IF EXISTS without name parses IF as name",
			"ALTER TAG IF EXISTS",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG name only, no sub-command",
			"ALTER TAG my_tag",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG IF EXISTS with no sub-command",
			"ALTER TAG IF EXISTS my_tag",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG with unknown sub-command",
			"ALTER TAG my_tag RESET",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG ADD without ALLOWED_VALUES keyword",
			"ALTER TAG my_tag ADD",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG DROP without ALLOWED_VALUES keyword",
			"ALTER TAG my_tag DROP",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG SET without COMMENT keyword",
			"ALTER TAG my_tag SET",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG UNSET without target keyword",
			"ALTER TAG my_tag UNSET",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG RENAME TO without new name",
			"ALTER TAG my_tag RENAME TO",
			[]string{"ALTER TAG RENAME TO requires a new tag name"},
		},
		{
			"ALTER TAG IF EXISTS RENAME TO without new name",
			"ALTER TAG IF EXISTS my_tag RENAME TO",
			[]string{"ALTER TAG RENAME TO requires a new tag name"},
		},
		{
			"ALTER TAG SET COMMENT missing equals sign",
			"ALTER TAG my_tag SET COMMENT 'c'",
			[]string{"Unknown ALTER TAG sub-command"},
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
		{
			"ALTER TAG compound ADD + DROP ALLOWED_VALUES",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'a' DROP ALLOWED_VALUES 'b'",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		{
			"ALTER TAG compound SET COMMENT then RENAME",
			"ALTER TAG my_tag SET COMMENT = 'c' RENAME TO new_tag",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		{
			"ALTER TAG compound ADD ALLOWED_VALUES then SET COMMENT",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'a' SET COMMENT = 'c'",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with case-insensitive duplicate",
			"ALTER TAG my_tag DROP ALLOWED_VALUES 'Finance', 'FINANCE'",
			[]string{"case-insensitive match with 'Finance'"},
		},
		// ── ALTER TAG: ADD/DROP ALLOWED_VALUES with numeric value ────────
		{
			"ALTER TAG ADD ALLOWED_VALUES with numeric",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 123",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with numeric",
			"ALTER TAG my_tag DROP ALLOWED_VALUES 123",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
		},
		// ── ALTER TAG: IF EXISTS + empty ADD/DROP ALLOWED_VALUES ─────────
		{
			"ALTER TAG IF EXISTS ADD ALLOWED_VALUES without values",
			"ALTER TAG IF EXISTS my_tag ADD ALLOWED_VALUES",
			[]string{"ADD ALLOWED_VALUES requires at least one string literal"},
		},
		{
			"ALTER TAG IF EXISTS DROP ALLOWED_VALUES without values",
			"ALTER TAG IF EXISTS my_tag DROP ALLOWED_VALUES",
			[]string{"DROP ALLOWED_VALUES requires at least one string literal"},
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
		{
			"DROP TAG IF EXISTS with RESTRICT",
			"DROP TAG IF EXISTS my_tag RESTRICT",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		// ── ALTER TAG: ADD exact-case duplicate ──────────────────────────
		{
			"ALTER TAG ADD ALLOWED_VALUES with exact duplicate",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'v1', 'v2', 'v1'",
			[]string{"Duplicate value 'v1'"},
		},
		// ── ALTER TAG: compound UNSET + UNSET ────────────────────────────
		{
			"ALTER TAG compound UNSET ALLOWED_VALUES and UNSET COMMENT",
			"ALTER TAG my_tag UNSET ALLOWED_VALUES UNSET COMMENT",
			[]string{"ALTER TAG supports only one sub-command per statement"},
		},
		// ── ALLOWED_VALUES: multiple duplicates produce multiple warnings ─
		{
			"CREATE TAG ALLOWED_VALUES with multiple duplicates",
			"CREATE TAG my_tag ALLOWED_VALUES 'a', 'b', 'a', 'b'",
			[]string{"Duplicate value 'a'", "Duplicate value 'b'"},
		},
		// ── ALTER TAG ADD/DROP: multiple distinct duplicate pairs ─────────
		{
			"ALTER TAG ADD ALLOWED_VALUES with multiple duplicates",
			"ALTER TAG my_tag ADD ALLOWED_VALUES 'a', 'b', 'a', 'b'",
			[]string{"Duplicate value 'a'", "Duplicate value 'b'"},
		},
		{
			"ALTER TAG DROP ALLOWED_VALUES with multiple duplicates",
			"ALTER TAG my_tag DROP ALLOWED_VALUES 'a', 'b', 'a', 'b'",
			[]string{"Duplicate value 'a'", "Duplicate value 'b'"},
		},
		// ── ALTER TAG: RENAME without TO ─────────────────────────────────
		{
			"ALTER TAG RENAME without TO keyword",
			"ALTER TAG my_tag RENAME",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		{
			"ALTER TAG IF EXISTS RENAME without TO keyword",
			"ALTER TAG IF EXISTS my_tag RENAME",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		// ── ALTER TAG: sub-command inside comment is not a sub-command ───
		{
			"ALTER TAG with sub-command in comment only",
			"ALTER TAG my_tag -- RENAME TO new_tag",
			[]string{"Unknown ALTER TAG sub-command"},
		},
		// ── ALTER TAG: RENAME TO target name only in comment ─────────────
		{
			"ALTER TAG RENAME TO with target in comment",
			"ALTER TAG my_tag RENAME TO -- new_tag",
			[]string{"ALTER TAG RENAME TO requires a new tag name"},
		},
		// ── DROP TAG: case-insensitive CASCADE / RESTRICT ────────────────
		{
			"DROP TAG with lowercase cascade",
			"DROP TAG my_tag cascade",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		{
			"DROP TAG with lowercase restrict",
			"DROP TAG my_tag restrict",
			[]string{"CASCADE / RESTRICT are not valid for DROP TAG"},
		},
		// ── CREATE TAG: unterminated string literal in ALLOWED_VALUES ─────
		{
			"CREATE TAG ALLOWED_VALUES with unterminated string",
			"CREATE TAG my_tag ALLOWED_VALUES 'unterminated",
			[]string{"ALLOWED_VALUES requires a list of string literals"},
		},
		// ── CREATE TAG: duplicate empty string values ─────────────────────
		{
			"CREATE TAG ALLOWED_VALUES with duplicate empty strings",
			"CREATE TAG my_tag ALLOWED_VALUES '', ''",
			[]string{"Duplicate value ''"},
		},
		// ── ALTER TAG: duplicate empty string values in ADD ───────────────
		{
			"ALTER TAG ADD ALLOWED_VALUES with duplicate empty strings",
			"ALTER TAG my_tag ADD ALLOWED_VALUES '', ''",
			[]string{"Duplicate value ''"},
		},
		// ── ALTER TAG: duplicate empty string values in DROP ──────────────
		{
			"ALTER TAG DROP ALLOWED_VALUES with duplicate empty strings",
			"ALTER TAG my_tag DROP ALLOWED_VALUES '', ''",
			[]string{"Duplicate value ''"},
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

func TestValidateSnowflakePatterns_Tag_MultiStatement(t *testing.T) {
	t.Run("two valid tag statements", func(t *testing.T) {
		sql := "CREATE TAG t1;\nDROP TAG t2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
		}
	})

	t.Run("both statements invalid", func(t *testing.T) {
		sql := "CREATE TAG;\nDROP TAG"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) < 2 {
			t.Errorf("Expected at least 2 warnings (one per statement), got %d: %v", len(warns), warns)
			return
		}
		wantMsgs := []string{"CREATE TAG requires a tag name", "DROP TAG requires a tag name"}
		for _, wantMsg := range wantMsgs {
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning containing %q, got: %v", wantMsg, warns)
			}
		}
	})

	t.Run("one valid one invalid tag statement", func(t *testing.T) {
		sql := "CREATE TAG t1;\nCREATE TAG"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Error("Expected warnings for the second statement, got 0")
			return
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "CREATE TAG requires a tag name") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing tag name, got: %v", warns)
		}
	})

	t.Run("non-tag statement does not interfere with tag validation", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE TAG"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Error("Expected warning for the CREATE TAG statement, got 0")
			return
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "CREATE TAG requires a tag name") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing tag name, got: %v", warns)
		}
	})

	t.Run("ALTER TAG in multi-statement with valid siblings", func(t *testing.T) {
		sql := "CREATE TAG t1;\nALTER TAG t1 ADD ALLOWED_VALUES 'a';\nDROP TAG t1"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
		}
	})

	t.Run("ALTER TAG invalid in multi-statement", func(t *testing.T) {
		sql := "CREATE TAG t1;\nALTER TAG"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Error("Expected warning for the ALTER TAG statement, got 0")
			return
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ALTER TAG requires a tag name") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing tag name, got: %v", warns)
		}
	})
}
