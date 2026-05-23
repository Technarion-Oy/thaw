package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_GitRepository(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// CREATE GIT REPOSITORY — basic with mandatory properties
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		"CREATE OR REPLACE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		"CREATE GIT REPOSITORY IF NOT EXISTS my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — three-part name (schema-level object)
		"CREATE GIT REPOSITORY db.schema.my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — with GIT_CREDENTIALS
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' GIT_CREDENTIALS = my_secret",
		// CREATE GIT REPOSITORY — with COMMENT
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' COMMENT = 'main repo'",
		// CREATE GIT REPOSITORY — with all optional properties
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'https://github.com/my-org/my-repo.git' GIT_CREDENTIALS = my_secret COMMENT = 'main repo'",
		// CREATE GIT REPOSITORY — git@ SSH URL
		"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int ORIGIN = 'git@github.com:my-org/my-repo.git'",
		// CREATE GIT REPOSITORY — case insensitive
		"create git repository my_repo api_integration = my_api_int origin = 'https://github.com/my-org/my-repo.git'",
		"Create Git Repository my_repo Api_Integration = my_int Origin = 'https://example.com/repo.git'",

		// ALTER GIT REPOSITORY — FETCH
		"ALTER GIT REPOSITORY my_repo FETCH",
		// ALTER GIT REPOSITORY — SET API_INTEGRATION
		"ALTER GIT REPOSITORY my_repo SET API_INTEGRATION = new_int",
		// ALTER GIT REPOSITORY — SET GIT_CREDENTIALS
		"ALTER GIT REPOSITORY my_repo SET GIT_CREDENTIALS = new_secret",
		// ALTER GIT REPOSITORY — SET COMMENT
		"ALTER GIT REPOSITORY my_repo SET COMMENT = 'updated comment'",
		// ALTER GIT REPOSITORY — UNSET GIT_CREDENTIALS
		"ALTER GIT REPOSITORY my_repo UNSET GIT_CREDENTIALS",
		// ALTER GIT REPOSITORY — UNSET COMMENT
		"ALTER GIT REPOSITORY my_repo UNSET COMMENT",
		// ALTER GIT REPOSITORY — three-part name
		"ALTER GIT REPOSITORY db.schema.my_repo FETCH",

		// DROP GIT REPOSITORY
		"DROP GIT REPOSITORY my_repo",
		"DROP GIT REPOSITORY IF EXISTS my_repo",
		"DROP GIT REPOSITORY db.schema.my_repo",
		"DROP GIT REPOSITORY IF EXISTS db.schema.my_repo",
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
		// CREATE GIT REPOSITORY — missing name
		{
			"CREATE GIT REPOSITORY missing name",
			"CREATE GIT REPOSITORY",
			[]string{"Unexpected syntax in CREATE GIT REPOSITORY"},
		},
		// CREATE GIT REPOSITORY — IF NOT EXISTS without name
		{
			"CREATE GIT REPOSITORY IF NOT EXISTS missing name",
			"CREATE GIT REPOSITORY IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE GIT REPOSITORY"},
		},
		// CREATE GIT REPOSITORY — OR REPLACE and IF NOT EXISTS conflict
		{
			"CREATE GIT REPOSITORY OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE GIT REPOSITORY IF NOT EXISTS my_repo API_INTEGRATION = my_int ORIGIN = 'https://example.com/repo.git'",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// CREATE GIT REPOSITORY — missing API_INTEGRATION
		{
			"CREATE GIT REPOSITORY missing API_INTEGRATION",
			"CREATE GIT REPOSITORY my_repo ORIGIN = 'https://github.com/my-org/my-repo.git'",
			[]string{"requires API_INTEGRATION"},
		},
		// CREATE GIT REPOSITORY — missing ORIGIN
		{
			"CREATE GIT REPOSITORY missing ORIGIN",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_api_int",
			[]string{"requires ORIGIN"},
		},
		// CREATE GIT REPOSITORY — missing both mandatory properties
		{
			"CREATE GIT REPOSITORY missing both mandatory",
			"CREATE GIT REPOSITORY my_repo",
			[]string{"requires API_INTEGRATION", "requires ORIGIN"},
		},
		// CREATE GIT REPOSITORY — ORIGIN not a string literal
		{
			"CREATE GIT REPOSITORY ORIGIN not string literal",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = https://example.com",
			[]string{"ORIGIN value must be a string literal"},
		},
		// CREATE GIT REPOSITORY — ORIGIN with invalid URL scheme
		{
			"CREATE GIT REPOSITORY ORIGIN invalid URL",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'http://example.com/repo.git'",
			[]string{"ORIGIN URL should start with 'https://' or 'git@'"},
		},
		// CREATE GIT REPOSITORY — ORIGIN with ftp URL
		{
			"CREATE GIT REPOSITORY ORIGIN ftp URL",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'ftp://example.com/repo.git'",
			[]string{"ORIGIN URL should start with 'https://' or 'git@'"},
		},
		// CREATE GIT REPOSITORY — unexpected property
		{
			"CREATE GIT REPOSITORY unexpected property",
			"CREATE GIT REPOSITORY my_repo API_INTEGRATION = my_int ORIGIN = 'https://example.com/repo.git' AUTO_REFRESH = TRUE",
			[]string{"Unexpected property 'AUTO_REFRESH'"},
		},

		// ALTER GIT REPOSITORY — missing name
		{
			"ALTER GIT REPOSITORY missing name",
			"ALTER GIT REPOSITORY",
			[]string{"ALTER GIT REPOSITORY requires a repository name"},
		},
		// ALTER GIT REPOSITORY — unknown sub-command
		{
			"ALTER GIT REPOSITORY unknown action",
			"ALTER GIT REPOSITORY my_repo SYNC",
			[]string{"Unknown ALTER GIT REPOSITORY sub-command"},
		},

		// DROP GIT REPOSITORY — missing name
		{
			"DROP GIT REPOSITORY missing name",
			"DROP GIT REPOSITORY",
			[]string{"DROP GIT REPOSITORY requires a repository name"},
		},
		// DROP GIT REPOSITORY IF EXISTS — missing name
		{
			"DROP GIT REPOSITORY IF EXISTS missing name",
			"DROP GIT REPOSITORY IF EXISTS",
			[]string{"DROP GIT REPOSITORY requires a repository name"},
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


