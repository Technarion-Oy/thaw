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
			// ── mixed case ──────────────────────────────────────────────────
			"create notebook my_nb",
			"Create Or Replace Notebook my_nb",
			"create notebook if not exists my_nb",
			// ── quoted identifiers ──────────────────────────────────────────
			`CREATE NOTEBOOK "My-Notebook"`,
			`CREATE NOTEBOOK "db"."schema"."My Notebook"`,
			`CREATE OR REPLACE NOTEBOOK "my_nb" FROM '@stage/path' MAIN_FILE = 'nb.ipynb'`,
			// ── IF NOT EXISTS without name (known false negative) ──────────
			// The regex backtracks and parses "IF" as the notebook name;
			// no secondary check catches this, so no warning is produced.
			"CREATE NOTEBOOK IF NOT EXISTS",
			// ── multiline formatting ────────────────────────────────────────
			"CREATE NOTEBOOK my_nb\n\tFROM '@stage/path'\n\tMAIN_FILE = 'nb.ipynb'\n\tQUERY_WAREHOUSE = my_wh",
			"CREATE OR REPLACE NOTEBOOK db.schema.my_nb\n  COMMENT = 'multiline'\n  IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 3600",
			// ── MAIN_FILE without FROM (no false positive) ─────────────────
			"CREATE NOTEBOOK my_nb MAIN_FILE = 'nb.ipynb'",
			// ── combined properties ─────────────────────────────────────────
			"CREATE NOTEBOOK my_nb FROM '@stage/path' MAIN_FILE = 'nb.ipynb' QUERY_WAREHOUSE = my_wh COMMENT = 'all props' IDLE_AUTO_SHUTDOWN_TIME_SECONDS = 7200",
			// ── trailing semicolon ──────────────────────────────────────────
			"CREATE NOTEBOOK my_nb;",
			"CREATE NOTEBOOK my_nb FROM '@stage/path' MAIN_FILE = 'nb.ipynb';",
			// ── FROM in a line comment should not trigger MAIN_FILE requirement ──
			"CREATE NOTEBOOK my_nb -- FROM '@stage/path'",
			// ── FROM in a block comment should not trigger MAIN_FILE requirement ──
			"CREATE NOTEBOOK my_nb /* FROM '@stage/path' */",
			// ── reversed FROM/MAIN_FILE order (order-independent checks) ──
			"CREATE NOTEBOOK my_nb MAIN_FILE = 'nb.ipynb' FROM '@stage/path'",
			// ── leading whitespace ────────────────────────────────────────
			"  CREATE NOTEBOOK my_nb",
			"  CREATE OR REPLACE NOTEBOOK my_nb FROM '@stage/path' MAIN_FILE = 'nb.ipynb'",
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
				sql:     "CREATE OR REPLACE NOTEBOOK",
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
			// mixed case FROM without MAIN_FILE
			{
				sql:     "create notebook my_nb from '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
			// multiline FROM without MAIN_FILE
			{
				sql:     "CREATE NOTEBOOK my_nb\n  FROM '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
			// bare CREATE NOTEBOOK with semicolon only
			{
				sql:     "CREATE NOTEBOOK;",
				wantMsg: "CREATE NOTEBOOK requires a notebook name",
			},
			// mixed case OR REPLACE + IF NOT EXISTS conflict
			{
				sql:     "create or replace notebook if not exists my_nb",
				wantMsg: "Conflict between OR REPLACE and IF NOT EXISTS",
			},
			// OR REPLACE with semicolon but no name
			{
				sql:     "CREATE OR REPLACE NOTEBOOK;",
				wantMsg: "CREATE NOTEBOOK requires a notebook name",
			},
			// OR REPLACE with FROM but missing MAIN_FILE
			{
				sql:     "CREATE OR REPLACE NOTEBOOK my_nb FROM '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
			// IF NOT EXISTS with FROM but missing MAIN_FILE
			{
				sql:     "CREATE NOTEBOOK IF NOT EXISTS my_nb FROM '@stage/path'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
			// MAIN_FILE hidden in a line comment — comment stripping removes it,
			// so the FROM-without-MAIN_FILE warning must still fire.
			{
				sql:     "CREATE NOTEBOOK my_nb FROM '@stage/path' -- MAIN_FILE = 'nb.ipynb'",
				wantMsg: "MAIN_FILE is required when FROM is specified",
			},
			// MAIN_FILE hidden in a block comment
			{
				sql:     "CREATE NOTEBOOK my_nb FROM '@stage/path' /* MAIN_FILE = 'nb.ipynb' */",
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

	t.Run("CREATE NOTEBOOK conflict takes precedence over FROM/MAIN_FILE check", func(t *testing.T) {
		sql := "CREATE OR REPLACE NOTEBOOK IF NOT EXISTS my_nb FROM '@stage/path'"
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warns)
		}
		if !strings.Contains(warns[0].Message, "Conflict between OR REPLACE and IF NOT EXISTS") {
			t.Errorf("Expected conflict warning, got: %v", warns[0].Message)
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
			// ── mixed case ──────────────────────────────────────────────────
			"alter notebook my_nb set comment = 'lower'",
			"Alter Notebook my_nb Rename To new_nb",
			"alter notebook my_nb add live version from last",
			// ── quoted identifiers ──────────────────────────────────────────
			`ALTER NOTEBOOK "My-Notebook" SET COMMENT = 'quoted'`,
			`ALTER NOTEBOOK "db"."schema"."My Notebook" RENAME TO "db"."schema"."New Notebook"`,
			// ── trailing semicolon ──────────────────────────────────────────
			"ALTER NOTEBOOK my_nb SET COMMENT = 'semi';",
			"ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM LAST;",
			// ── multiple SET properties ─────────────────────────────────────
			"ALTER NOTEBOOK my_nb SET QUERY_WAREHOUSE = my_wh COMMENT = 'updated'",
			// ── IF EXISTS with each sub-command ─────────────────────────────
			"ALTER NOTEBOOK IF EXISTS my_nb UNSET QUERY_WAREHOUSE",
			"ALTER NOTEBOOK IF EXISTS my_nb RENAME TO new_nb",
			"ALTER NOTEBOOK IF EXISTS my_nb ADD LIVE VERSION FROM LAST",
			// ── multiline formatting ────────────────────────────────────────
			"ALTER NOTEBOOK my_nb\n  SET COMMENT = 'multiline'",
			"ALTER NOTEBOOK my_nb\n  RENAME TO new_nb",
			// ── leading whitespace ──────────────────────────────────────────
			"  ALTER NOTEBOOK my_nb SET COMMENT = 'test'",
			// ── SET COMMENT containing RENAME TO keyword (false positive) ───
			`ALTER NOTEBOOK my_nb SET COMMENT = 'RENAME TO bad_name'`,
			// ── SET COMMENT containing ADD LIVE VERSION (false positive) ─────
			`ALTER NOTEBOOK my_nb SET COMMENT = 'ADD LIVE VERSION FROM LAST'`,
			// ── UNSET multiple properties ───────────────────────────────────
			"ALTER NOTEBOOK my_nb UNSET COMMENT QUERY_WAREHOUSE",
			// ── extra whitespace between ADD LIVE VERSION keywords ──────────
			"ALTER NOTEBOOK my_nb ADD  LIVE  VERSION  FROM  LAST",
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
			// mixed case invalid
			{
				sql:     "alter notebook",
				wantMsg: "ALTER NOTEBOOK requires a notebook name",
			},
			{
				sql:     "alter notebook my_nb",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// IF EXISTS without a name — validator parses "IF" as the name,
			// then "EXISTS" fails sub-command check
			{
				sql:     "ALTER NOTEBOOK IF EXISTS",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// mixed case RENAME TO without target
			{
				sql:     "alter notebook my_nb rename to",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			// mixed case ADD LIVE VERSION incomplete
			{
				sql:     "alter notebook my_nb add live version",
				wantMsg: "ADD LIVE VERSION requires FROM LAST",
			},
			// partial ADD sub-command (missing LIVE VERSION)
			{
				sql:     "ALTER NOTEBOOK my_nb ADD",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// partial ADD LIVE sub-command (missing VERSION)
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// IF EXISTS still requires a sub-command after the name
			{
				sql:     "ALTER NOTEBOOK IF EXISTS my_nb",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// sub-command hidden in a line comment — stripped, so no sub-command remains
			{
				sql:     "ALTER NOTEBOOK my_nb -- SET COMMENT = 'x'",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// sub-command hidden in a block comment — stripped, so no sub-command remains
			{
				sql:     "ALTER NOTEBOOK my_nb /* SET COMMENT = 'x' */",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// bare ALTER NOTEBOOK with semicolon only
			{
				sql:     "ALTER NOTEBOOK;",
				wantMsg: "ALTER NOTEBOOK requires a notebook name",
			},
			// qualified name with no sub-command
			{
				sql:     "ALTER NOTEBOOK db.schema.my_nb",
				wantMsg: "Unknown ALTER NOTEBOOK sub-command",
			},
			// RENAME TO with trailing semicolon but no target name
			{
				sql:     "ALTER NOTEBOOK my_nb RENAME TO;",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			// RENAME TO target hidden in a line comment
			{
				sql:     "ALTER NOTEBOOK my_nb RENAME TO -- new_nb",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			// RENAME TO target hidden in a block comment
			{
				sql:     "ALTER NOTEBOOK my_nb RENAME TO /* new_nb */",
				wantMsg: "RENAME TO requires a new notebook name",
			},
			// ADD LIVE VERSION with LAST hidden in a line comment
			{
				sql:     "ALTER NOTEBOOK my_nb ADD LIVE VERSION FROM -- LAST",
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
			// ── mixed case ──────────────────────────────────────────────────
			"drop notebook my_nb",
			"Drop Notebook If Exists my_nb",
			// ── quoted identifiers ──────────────────────────────────────────
			`DROP NOTEBOOK "My-Notebook"`,
			`DROP NOTEBOOK IF EXISTS "db"."schema"."My Notebook"`,
			// ── IF EXISTS without name (known false negative) ──────────────
			// The regex backtracks and parses "IF" as the notebook name;
			// no secondary check catches this, so no warning is produced.
			"DROP NOTEBOOK IF EXISTS",
			// ── trailing semicolon ──────────────────────────────────────────
			"DROP NOTEBOOK my_nb;",
			// ── leading whitespace ──────────────────────────────────────────
			"  DROP NOTEBOOK my_nb",
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

	t.Run("multi-statement notebook", func(t *testing.T) {
		t.Run("second statement missing name", func(t *testing.T) {
			sql := "SELECT 1;\nCREATE NOTEBOOK"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "CREATE NOTEBOOK requires a notebook name") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected warning for second statement, got: %v", warns)
			}
		})

		t.Run("valid notebook among other statements", func(t *testing.T) {
			sql := "USE DATABASE my_db;\nCREATE NOTEBOOK my_nb;\nSELECT 1"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected no warnings, got: %v", warns)
			}
		})

		t.Run("multiple notebook errors in same script", func(t *testing.T) {
			sql := "CREATE NOTEBOOK;\nDROP NOTEBOOK"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			createFound := false
			dropFound := false
			for _, w := range warns {
				if strings.Contains(w.Message, "CREATE NOTEBOOK requires a notebook name") {
					createFound = true
				}
				if strings.Contains(w.Message, "DROP NOTEBOOK requires a notebook name") {
					dropFound = true
				}
			}
			if !createFound {
				t.Errorf("Expected CREATE NOTEBOOK warning, got: %v", warns)
			}
			if !dropFound {
				t.Errorf("Expected DROP NOTEBOOK warning, got: %v", warns)
			}
		})

		t.Run("ALTER NOTEBOOK error in multi-statement script", func(t *testing.T) {
			sql := "SELECT 1;\nALTER NOTEBOOK my_nb;\nSELECT 2"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "Unknown ALTER NOTEBOOK sub-command") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected ALTER NOTEBOOK warning in multi-statement script, got: %v", warns)
			}
		})

		t.Run("DROP NOTEBOOK error in multi-statement script", func(t *testing.T) {
			sql := "SELECT 1;\nDROP NOTEBOOK;\nSELECT 2"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "DROP NOTEBOOK requires a notebook name") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected DROP NOTEBOOK warning in multi-statement script, got: %v", warns)
			}
		})

		t.Run("CREATE NOTEBOOK FROM without MAIN_FILE in multi-statement script", func(t *testing.T) {
			sql := "SELECT 1;\nCREATE NOTEBOOK my_nb FROM '@stage/path';\nSELECT 2"
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "MAIN_FILE is required when FROM is specified") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected MAIN_FILE warning in multi-statement script, got: %v", warns)
			}
		})
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
			{
				sql:     "DROP NOTEBOOK IF EXISTS my_nb CASCADE",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			// mixed case invalid
			{
				sql:     "drop notebook",
				wantMsg: "DROP NOTEBOOK requires a notebook name",
			},
			// IF EXISTS with RESTRICT
			{
				sql:     "DROP NOTEBOOK IF EXISTS my_nb RESTRICT",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			// mixed case CASCADE / RESTRICT
			{
				sql:     "drop notebook my_nb cascade",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			{
				sql:     "drop notebook my_nb restrict",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			// bare DROP NOTEBOOK with semicolon only
			{
				sql:     "DROP NOTEBOOK;",
				wantMsg: "DROP NOTEBOOK requires a notebook name",
			},
			// qualified name with CASCADE
			{
				sql:     "DROP NOTEBOOK db.schema.my_nb CASCADE",
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			// quoted identifier with CASCADE
			{
				sql:     `DROP NOTEBOOK "my_nb" CASCADE`,
				wantMsg: "CASCADE / RESTRICT are not valid for DROP NOTEBOOK",
			},
			// quoted identifier with RESTRICT
			{
				sql:     `DROP NOTEBOOK "my_nb" RESTRICT`,
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
