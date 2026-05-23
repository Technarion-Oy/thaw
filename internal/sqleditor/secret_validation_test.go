package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Secret(t *testing.T) {
	// ── Valid cases ──────────────────────────────────────────────────────
	validCases := []string{
		// -- PASS: CREATE SECRET TYPE = OAUTH2 with mandatory API_AUTHENTICATION
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_security_integration",
		// -- PASS: CREATE SECRET TYPE = OAUTH2 with all optional properties
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_SCOPES = ('scope1', 'scope2') OAUTH_REFRESH_TOKEN = 'token123' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31 00:00:00' COMMENT = 'my oauth secret'",
		// -- PASS: CREATE SECRET TYPE = PASSWORD with USERNAME and PASSWORD
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'myuser' PASSWORD = 'mypass'",
		// -- PASS: CREATE SECRET TYPE = PASSWORD with COMMENT
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'myuser' PASSWORD = 'mypass' COMMENT = 'basic auth'",
		// -- PASS: CREATE SECRET TYPE = GENERIC_STRING with SECRET_STRING
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'some-api-key-value'",
		// -- PASS: CREATE SECRET TYPE = GENERIC_STRING with COMMENT
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc123' COMMENT = 'api key'",
		// -- PASS: CREATE SECRET TYPE = CLOUD_PROVIDER_TOKEN with API_AUTHENTICATION
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int",
		// -- PASS: CREATE SECRET TYPE = CLOUD_PROVIDER_TOKEN with ENABLED
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int ENABLED = TRUE",
		// -- PASS: CREATE SECRET TYPE = SYMMETRIC_KEY with ALGORITHM
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256'",
		// -- PASS: CREATE SECRET TYPE = SYMMETRIC_KEY with COMMENT
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' COMMENT = 'encryption key'",
		// -- PASS: OR REPLACE variant
		"CREATE OR REPLACE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: IF NOT EXISTS variant
		"CREATE SECRET IF NOT EXISTS my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: Three-part name (schema-level object)
		"CREATE SECRET db.schema.my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: Case insensitivity
		"create secret my_secret type = oauth2 api_authentication = my_int",
		"Create Secret my_secret Type = Password Username = 'u' Password = 'p'",

		// ALTER SECRET — valid cases
		// -- PASS: ALTER SECRET SET for GENERIC_STRING
		"ALTER SECRET my_secret SET SECRET_STRING = 'new_value'",
		// -- PASS: ALTER SECRET SET for PASSWORD
		"ALTER SECRET my_secret SET USERNAME = 'new_user' PASSWORD = 'new_pass'",
		// -- PASS: ALTER SECRET SET for OAUTH2
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN = 'new_token' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2026-01-01 00:00:00'",
		// -- PASS: ALTER SECRET SET OAUTH_SCOPES
		"ALTER SECRET my_secret SET OAUTH_SCOPES = ('scope1', 'scope2')",
		// -- PASS: ALTER SECRET SET API_AUTHENTICATION
		"ALTER SECRET my_secret SET API_AUTHENTICATION = new_int",
		// -- PASS: ALTER SECRET SET COMMENT
		"ALTER SECRET my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET UNSET COMMENT
		"ALTER SECRET my_secret UNSET COMMENT",
		// -- PASS: ALTER SECRET with three-part name
		"ALTER SECRET db.schema.my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET IF EXISTS
		"ALTER SECRET IF EXISTS my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET IF EXISTS with three-part name
		"ALTER SECRET IF EXISTS db.schema.my_secret SET SECRET_STRING = 'new_val'",
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
		// -- FAIL: CREATE SECRET missing name
		{
			"CREATE SECRET missing name",
			"CREATE SECRET",
			[]string{"Unexpected syntax in CREATE SECRET"},
		},
		// -- FAIL: IF NOT EXISTS without name
		{
			"CREATE SECRET IF NOT EXISTS missing name",
			"CREATE SECRET IF NOT EXISTS",
			[]string{"Unexpected syntax in CREATE SECRET"},
		},
		// -- FAIL: OR REPLACE + IF NOT EXISTS conflict
		{
			"CREATE SECRET OR REPLACE + IF NOT EXISTS",
			"CREATE OR REPLACE SECRET IF NOT EXISTS my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		// -- FAIL: Missing TYPE
		{
			"CREATE SECRET missing TYPE",
			"CREATE SECRET my_secret API_AUTHENTICATION = my_int",
			[]string{"CREATE SECRET requires TYPE"},
		},
		// -- FAIL: Unknown TYPE value
		{
			"CREATE SECRET unknown TYPE",
			"CREATE SECRET my_secret TYPE = BEARER_TOKEN SECRET_STRING = 'abc'",
			[]string{"Unknown TYPE"},
		},
		// -- FAIL: TYPE = OAUTH2 missing API_AUTHENTICATION
		{
			"CREATE SECRET OAUTH2 missing API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = OAUTH2",
			[]string{"TYPE = OAUTH2 requires API_AUTHENTICATION"},
		},
		// -- FAIL: TYPE = PASSWORD missing USERNAME
		{
			"CREATE SECRET PASSWORD missing USERNAME",
			"CREATE SECRET my_secret TYPE = PASSWORD PASSWORD = 'p'",
			[]string{"TYPE = PASSWORD requires USERNAME"},
		},
		// -- FAIL: TYPE = PASSWORD missing PASSWORD
		{
			"CREATE SECRET PASSWORD missing PASSWORD",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u'",
			[]string{"TYPE = PASSWORD requires PASSWORD"},
		},
		// -- FAIL: TYPE = PASSWORD missing both
		{
			"CREATE SECRET PASSWORD missing both",
			"CREATE SECRET my_secret TYPE = PASSWORD",
			[]string{"TYPE = PASSWORD requires USERNAME", "TYPE = PASSWORD requires PASSWORD"},
		},
		// -- FAIL: TYPE = GENERIC_STRING missing SECRET_STRING
		{
			"CREATE SECRET GENERIC_STRING missing SECRET_STRING",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING",
			[]string{"TYPE = GENERIC_STRING requires SECRET_STRING"},
		},
		// -- FAIL: TYPE = CLOUD_PROVIDER_TOKEN missing API_AUTHENTICATION
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN missing API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN",
			[]string{"TYPE = CLOUD_PROVIDER_TOKEN requires API_AUTHENTICATION"},
		},
		// -- FAIL: TYPE = SYMMETRIC_KEY missing ALGORITHM
		{
			"CREATE SECRET SYMMETRIC_KEY missing ALGORITHM",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY",
			[]string{"TYPE = SYMMETRIC_KEY requires ALGORITHM"},
		},
		// -- FAIL: USERNAME on OAUTH2 type (wrong type property)
		{
			"CREATE SECRET OAUTH2 with USERNAME",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int USERNAME = 'u'",
			[]string{"USERNAME is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: SECRET_STRING on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with SECRET_STRING",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' SECRET_STRING = 'abc'",
			[]string{"SECRET_STRING is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: API_AUTHENTICATION on GENERIC_STRING type (wrong type property)
		{
			"CREATE SECRET GENERIC_STRING with API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc' API_AUTHENTICATION = my_int",
			[]string{"API_AUTHENTICATION is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: USERNAME on SYMMETRIC_KEY type (wrong type property)
		{
			"CREATE SECRET SYMMETRIC_KEY with USERNAME",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' USERNAME = 'u'",
			[]string{"USERNAME is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: ALGORITHM on OAUTH2 type (wrong type property)
		{
			"CREATE SECRET OAUTH2 with ALGORITHM",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int ALGORITHM = 'AES-256'",
			[]string{"ALGORITHM is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: ENABLED on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with ENABLED",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' ENABLED = TRUE",
			[]string{"ENABLED is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN on PASSWORD type (wrong type property)
		{
			"CREATE SECRET PASSWORD with OAUTH_REFRESH_TOKEN",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_REFRESH_TOKEN is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: OAUTH_SCOPES on GENERIC_STRING type (wrong type property)
		{
			"CREATE SECRET GENERIC_STRING with OAUTH_SCOPES",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'abc' OAUTH_SCOPES = ('s1')",
			[]string{"OAUTH_SCOPES is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: Unexpected property
		{
			"CREATE SECRET unexpected property",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' WAREHOUSE = my_wh",
			[]string{"Unexpected property"},
		},

		// ALTER SECRET — invalid cases
		// -- FAIL: ALTER SECRET missing name
		{
			"ALTER SECRET missing name",
			"ALTER SECRET",
			[]string{"ALTER SECRET requires a secret name"},
		},
		// -- FAIL: ALTER SECRET unknown sub-command
		{
			"ALTER SECRET unknown action",
			"ALTER SECRET my_secret REFRESH",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET IF EXISTS unknown sub-command
		{
			"ALTER SECRET IF EXISTS unknown action",
			"ALTER SECRET IF EXISTS my_secret REFRESH",
			[]string{"Unknown ALTER SECRET sub-command"},
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

