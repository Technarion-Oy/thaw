package sqleditor

import (
	"strings"
	"testing"
)

// ── Notebook Tests ──────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Notebook(t *testing.T) {
	t.Run("valid CREATE NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"CREATE NOTEBOOK my_nb",
			"CREATE OR REPLACE NOTEBOOK my_nb",
			"CREATE NOTEBOOK IF NOT EXISTS my_nb",
			"CREATE NOTEBOOK db.schema.my_nb",
			"CREATE OR REPLACE NOTEBOOK db.schema.my_nb",
			"CREATE NOTEBOOK IF NOT EXISTS db.schema.my_nb",
			"CREATE NOTEBOOK my_nb FROM '@my_stage/path' MAIN_FILE = 'notebook.ipynb'",
			"CREATE NOTEBOOK my_nb FROM '@db.schema.stage/dir/file' MAIN_FILE = 'my_nb.ipynb' QUERY_WAREHOUSE = my_wh",
			"CREATE NOTEBOOK my_nb QUERY_WAREHOUSE = my_wh",
			"CREATE NOTEBOOK my_nb COMMENT = 'A test notebook'",
			"CREATE NOTEBOOK my_nb FROM '@stage/path' MAIN_FILE = 'nb.ipynb' COMMENT = 'imported'",
			"CREATE NOTEBOOK my_nb IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 3600",
			// COMMENT containing FROM should not trigger MAIN_FILE requirement
			`CREATE NOTEBOOK my_nb COMMENT = 'imported FROM ''@stage/path'' stuff'`,
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid CREATE NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "CREATE NOTEBOOK",
				wantMsg: "CREATE NOTEBOOK requires a notebook name",
			},
			{
				sql:     "CREATE OR REPLACE NOTEBOOK IF NOT EXISTS my_nb",
				wantMsg: "Conflict between OR REPLACE and IF NOT EXISTS",
			},
			{
				sql:     "CREATE NOTEBOOK my_nb FROM '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("valid ALTER NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"ALTER NOTEBOOK my_nb SET COMMENT = 'updated'",
			"ALTER NOTEBOOK my_nb SET QUERY_WAREHOUSE = my_wh",
			"ALTER NOTEBOOK my_nb UNSET COMMENT",
			"ALTER NOTEBOOK my_nb UNSET QUERY_WAREHOUSE",
			"ALTER NOTEBOOK my_nb RENAME TO new_nb",
			"ALTER NOTEBOOK db.schema.my_nb RENAME TO db.schema.new_nb",
			"ALTER NOTEBOOK IF EXISTS my_nb SET COMMENT = 'safe update'",
			"ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM LAST",
			"ALTER NOTEBOOK my_nb SET IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 7200",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "ALTER NOTEBOOK",
				wantMsg: "ALTER NOTEBOOK requires a notebook name",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb RENAME TO",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM FIRST",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("valid DROP NOTEBOOK", func(t *testing.T) {
		validQueries := []string{
			"DROP NOTEBOOK my_nb",
			"DROP NOTEBOOK IF EXISTS my_nb",
			"DROP NOTEBOOK db.schema.my_nb",
			"DROP NOTEBOOK IF EXISTS db.schema.my_nb",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid DROP NOTEBOOK", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "DROP NOTEBOOK",
				wantMsg: "DROP NOTEBOOK requires a notebook name",
			},
			{
				sql:     "DROP NOTEBOOK my_nb CASCADE",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			{
				sql:     "DROP NOTEBOOK my_nb RESTRICT",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

