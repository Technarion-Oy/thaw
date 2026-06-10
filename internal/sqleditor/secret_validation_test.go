package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_Secret_EmptyInput(t *testing.T) {
	// Empty string and whitespace-only input should produce no markers.
	for _, sql := range []string{"", "   ", "\t\n"} {
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if len(markers) > 0 {
			t.Errorf("Expected 0 markers for empty/whitespace input %q, got %d", sql, len(markers))
		}
	}
}

func TestValidateSnowflakePatterns_Secret_CommentedOut(t *testing.T) {
	// CREATE/ALTER SECRET entirely inside comments should not trigger validation.
	cases := []string{
		"/* CREATE SECRET my_secret TYPE = PASSWORD */",
		"-- CREATE SECRET my_secret TYPE = PASSWORD",
		"/* ALTER SECRET my_secret SET COMMENT = 'x' */",
		"-- ALTER SECRET my_secret REFRESH",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for commented-out SQL %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_InsideStringLiteral(t *testing.T) {
	// CREATE SECRET embedded in a string literal of another statement should not fire.
	sql := "SELECT 'CREATE SECRET my_secret TYPE = PASSWORD' AS col"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) > 0 {
		t.Errorf("Expected 0 warnings for SQL inside string literal, got %d: %v", len(warns), warns)
	}
}

func TestValidateSnowflakePatterns_Secret_MultiStatement(t *testing.T) {
	// Only the CREATE SECRET statement should produce warnings, not unrelated statements.
	t.Run("valid secret in batch", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for valid CREATE SECRET in batch, got %d: %v", len(warns), warns)
		}
	})
	t.Run("invalid secret in batch", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE SECRET my_secret TYPE = PASSWORD;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 2 {
			t.Errorf("Expected 2 warnings (missing USERNAME + PASSWORD), got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("multiple secret statements mixed valid and invalid", func(t *testing.T) {
		sql := "CREATE SECRET s1 TYPE = GENERIC_STRING SECRET_STRING = 'val';\nCREATE SECRET s2 TYPE = OAUTH2;\nALTER SECRET s3 SET COMMENT = 'ok';"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Only s2 should produce a warning (missing API_AUTHENTICATION).
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning for mixed batch, got %d: %v", len(warns), warnMsgs(warns))
		}
		if len(warns) == 1 && !strings.Contains(warns[0].Message, "API_AUTHENTICATION") {
			t.Errorf("Expected API_AUTHENTICATION warning, got: %s", warns[0].Message)
		}
	})
}

func TestValidateSnowflakePatterns_Secret_MarkerSeverity(t *testing.T) {
	// All secret-related diagnostics should use severity 4 (Warning).
	sqls := []string{
		"CREATE SECRET my_secret TYPE = PASSWORD",
		"CREATE SECRET my_secret TYPE = OAUTH2 USERNAME = 'u'",
		"ALTER SECRET my_secret REFRESH",
		"CREATE SECRET",
	}
	for _, sql := range sqls {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			for _, m := range markers {
				if m.Severity != 4 {
					t.Errorf("Expected severity 4 for marker %q, got %d", m.Message, m.Severity)
				}
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MarkerLinePositions(t *testing.T) {
	// Multi-line CREATE SECRET: marker should span the correct line range.
	sql := "SELECT 1;\nCREATE SECRET my_secret\n  TYPE = PASSWORD\n  USERNAME = 'u';"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	// Should warn about missing PASSWORD.
	if len(warns) == 0 {
		t.Fatal("Expected at least 1 warning for multi-line CREATE SECRET missing PASSWORD")
	}
	for _, w := range warns {
		if w.StartLineNumber < 2 {
			t.Errorf("Expected StartLineNumber >= 2 (CREATE SECRET starts on line 2), got %d", w.StartLineNumber)
		}
		if w.EndLineNumber < w.StartLineNumber {
			t.Errorf("EndLineNumber (%d) should be >= StartLineNumber (%d)", w.EndLineNumber, w.StartLineNumber)
		}
	}
}

func TestValidateSnowflakePatterns_Secret_QuotedTypeValue(t *testing.T) {
	// TYPE = 'OAUTH2' (quoted) should fail because cleanParseText strips string
	// literals before the TYPE regex runs.
	sql := "CREATE SECRET my_secret TYPE = 'OAUTH2' API_AUTHENTICATION = my_int"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Error("Expected a warning when TYPE value is a string literal (quoted)")
	}
}

func TestValidateSnowflakePatterns_Secret_MixedCaseTypeValues(t *testing.T) {
	// TYPE values should be case-insensitive — mixed case must be accepted.
	cases := []struct {
		sql string
	}{
		{"CREATE SECRET my_secret TYPE = Oauth2 API_AUTHENTICATION = my_int"},
		{"CREATE SECRET my_secret TYPE = oAuth2 API_AUTHENTICATION = my_int"},
		{"CREATE SECRET my_secret TYPE = password USERNAME = 'u' PASSWORD = 'p'"},
		{"CREATE SECRET my_secret TYPE = Password USERNAME = 'u' PASSWORD = 'p'"},
		{"CREATE SECRET my_secret TYPE = generic_string SECRET_STRING = 'val'"},
		{"CREATE SECRET my_secret TYPE = Generic_String SECRET_STRING = 'val'"},
		{"CREATE SECRET my_secret TYPE = cloud_provider_token API_AUTHENTICATION = my_int"},
		{"CREATE SECRET my_secret TYPE = Cloud_Provider_Token API_AUTHENTICATION = my_int"},
		{"CREATE SECRET my_secret TYPE = symmetric_key ALGORITHM = 'AES-256'"},
		{"CREATE SECRET my_secret TYPE = Symmetric_Key ALGORITHM = 'AES-256'"},
	}
	for _, tc := range cases {
		t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for mixed-case TYPE value %q, got %d: %v", tc.sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_NameIsKeyword(t *testing.T) {
	// Secret names that happen to be SQL keywords should still work (the name
	// regex uses _identPath which matches bare identifiers).
	validCases := []string{
		"CREATE SECRET TYPE TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET SET TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET PASSWORD TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"ALTER SECRET SET SET COMMENT = 'x'",
		"ALTER SECRET TYPE SET COMMENT = 'x'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for keyword-as-name %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterNameStartsWithIF(t *testing.T) {
	// Names starting with "IF" should not be confused with "IF EXISTS" prefix.
	validCases := []string{
		"ALTER SECRET IF_SECRET SET COMMENT = 'x'",
		"ALTER SECRET IFOO SET SECRET_STRING = 'val'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CommentValueContainsKeywords(t *testing.T) {
	// COMMENT value containing property keywords should not trigger false positives
	// because cleanParseText strips string literals.
	validCases := []string{
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int COMMENT = 'USERNAME = admin PASSWORD = secret'",
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' COMMENT = 'TYPE = OAUTH2 API_AUTHENTICATION = x'",
		"ALTER SECRET my_secret SET COMMENT = 'ALGORITHM = AES SECRET_STRING = dangerous'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for COMMENT containing keywords %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_PropertyOrderIndependent(t *testing.T) {
	// Properties can appear in any order — COMMENT before mandatory, mandatory
	// after optional, etc.
	validCases := []string{
		"CREATE SECRET my_secret TYPE = OAUTH2 COMMENT = 'note' API_AUTHENTICATION = my_int",
		"CREATE SECRET my_secret TYPE = PASSWORD COMMENT = 'note' USERNAME = 'u' PASSWORD = 'p'",
		"CREATE SECRET my_secret TYPE = OAUTH2 OAUTH_SCOPES = ('s1') API_AUTHENTICATION = my_int",
		"CREATE SECRET my_secret TYPE = PASSWORD PASSWORD = 'p' USERNAME = 'u'",
		"CREATE SECRET my_secret COMMENT = 'first' TYPE = GENERIC_STRING SECRET_STRING = 'val'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for reordered properties %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_InlineCommentHidesProperty(t *testing.T) {
	// A property keyword inside a block comment should be stripped by cleanParseText
	// and should not satisfy mandatory checks.
	sql := "CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' /* PASSWORD = 'p' */"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Error("Expected warning for PASSWORD hidden inside block comment")
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "PASSWORD") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing PASSWORD, got: %v", warnMsgs(warns))
	}
}

func TestValidateSnowflakePatterns_Secret_InvalidWarningCountExact(t *testing.T) {
	// Verify exact warning counts for cases with multiple violations to ensure
	// no spurious extra warnings are produced.
	cases := []struct {
		name      string
		sql       string
		wantCount int
	}{
		{
			"OAUTH2 missing mandatory + two cross-type",
			"CREATE SECRET my_secret TYPE = OAUTH2 USERNAME = 'u' PASSWORD = 'p'",
			3, // missing API_AUTHENTICATION + USERNAME invalid + PASSWORD invalid
		},
		{
			"SYMMETRIC_KEY missing mandatory + three cross-type",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY USERNAME = 'u' PASSWORD = 'p' SECRET_STRING = 'val'",
			4, // missing ALGORITHM + 3 cross-type violations
		},
		{
			"GENERIC_STRING all mandatory present + one cross-type",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' ENABLED = TRUE",
			1, // ENABLED cross-type only
		},
		{
			"PASSWORD both mandatory present + two cross-type",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' API_AUTHENTICATION = x ALGORITHM = 'AES'",
			2, // API_AUTHENTICATION + ALGORITHM cross-type
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != tc.wantCount {
				t.Errorf("Expected exactly %d warning(s), got %d: %v", tc.wantCount, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_NoFalsePositive(t *testing.T) {
	// Statements that should NOT trigger the secret validator at all.
	unrelatedCases := []string{
		"CREATE TABLE secrets (id INT, name VARCHAR)",
		"ALTER TABLE my_secret ADD COLUMN val TEXT",
		"DROP SECRET my_secret",
		"DESCRIBE SECRET my_secret",
		"SHOW SECRETS",
		"SELECT * FROM information_schema.secrets",
		"CREATE SEQUENCE my_secret_seq START = 1",
	}
	for _, sql := range unrelatedCases {
		t.Run("no_fire/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for unrelated SQL %q, got %d: %v", sql, len(warns), warns)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_WarningCount(t *testing.T) {
	// Verify exact warning counts for specific cases to catch spurious diagnostics.
	cases := []struct {
		name      string
		sql       string
		wantCount int
	}{
		{
			"single missing mandatory",
			"CREATE SECRET my_secret TYPE = OAUTH2",
			1,
		},
		{
			"two missing mandatory PASSWORD",
			"CREATE SECRET my_secret TYPE = PASSWORD",
			2,
		},
		{
			"one cross-type violation",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int USERNAME = 'u'",
			1,
		},
		{
			"missing mandatory + cross-type violation",
			"CREATE SECRET my_secret TYPE = OAUTH2 USERNAME = 'u'",
			2, // missing API_AUTHENTICATION + USERNAME is not valid for OAUTH2
		},
		{
			"OR REPLACE + IF NOT EXISTS returns only one",
			"CREATE OR REPLACE SECRET IF NOT EXISTS my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
			1,
		},
		{
			"ALTER SECRET unknown sub-command single warning",
			"ALTER SECRET my_secret REFRESH",
			1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != tc.wantCount {
				msgs := make([]string, len(warns))
				for i, w := range warns {
					msgs[i] = w.Message
				}
				t.Errorf("Expected exactly %d warning(s), got %d: %v", tc.wantCount, len(warns), msgs)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CreateOrReplaceNoName(t *testing.T) {
	// CREATE OR REPLACE SECRET without a name should produce "Unexpected syntax".
	cases := []string{
		"CREATE OR REPLACE SECRET",
		"CREATE OR REPLACE SECRET   ",
		"CREATE OR REPLACE SECRET\t\n",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unexpected syntax") {
				t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_EscapedQuotesInValues(t *testing.T) {
	// Doubled single-quote escapes inside property values should not interfere
	// with validation — cleanParseText strips string literals via
	// reStripStringLiterals which handles '' escapes.
	validCases := []string{
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'it''s me' PASSWORD = 'p@ss''word'",
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'value with ''quotes'' inside'",
		"ALTER SECRET my_secret SET COMMENT = 'updated ''again'''",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for escaped-quote values, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_OAuthTokenRegexNoOverlap(t *testing.T) {
	// When only OAUTH_REFRESH_TOKEN_EXPIRY_TIME is present (no OAUTH_REFRESH_TOKEN),
	// the OAUTH_REFRESH_TOKEN regex must NOT falsely match. Verify exact warning
	// counts to ensure no spurious cross-type violations.
	t.Run("only EXPIRY_TIME on PASSWORD type", func(t *testing.T) {
		sql := "CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Exactly 1 warning: OAUTH_REFRESH_TOKEN_EXPIRY_TIME cross-type only.
		// OAUTH_REFRESH_TOKEN must NOT fire.
		if len(warns) != 1 {
			t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
		}
		if !strings.Contains(warns[0].Message, "OAUTH_REFRESH_TOKEN_EXPIRY_TIME") {
			t.Errorf("Expected OAUTH_REFRESH_TOKEN_EXPIRY_TIME warning, got: %s", warns[0].Message)
		}
	})
	t.Run("only EXPIRY_TIME on GENERIC_STRING type", func(t *testing.T) {
		sql := "CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'v' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
		}
		if !strings.Contains(warns[0].Message, "OAUTH_REFRESH_TOKEN_EXPIRY_TIME") {
			t.Errorf("Expected OAUTH_REFRESH_TOKEN_EXPIRY_TIME warning, got: %s", warns[0].Message)
		}
	})
	t.Run("both REFRESH_TOKEN and EXPIRY_TIME on PASSWORD type", func(t *testing.T) {
		sql := "CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_REFRESH_TOKEN = 'tok' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Exactly 2 warnings: one for each cross-type violation.
		if len(warns) != 2 {
			t.Fatalf("Expected exactly 2 warnings, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_DuplicateProperty(t *testing.T) {
	// Duplicate properties (same property appearing twice) should still pass
	// validation because checks are presence-based, not cardinality-based.
	validCases := []string{
		"CREATE SECRET s TYPE = PASSWORD USERNAME = 'u1' USERNAME = 'u2' PASSWORD = 'p'",
		"CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'a' SECRET_STRING = 'b'",
		"CREATE SECRET s TYPE = OAUTH2 API_AUTHENTICATION = int1 API_AUTHENTICATION = int2",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for duplicate properties, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_NewlinesInClauses(t *testing.T) {
	// Newlines within OR REPLACE and IF NOT EXISTS clauses should be accepted.
	validCases := []string{
		"CREATE OR\nREPLACE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE OR\r\nREPLACE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET IF\nNOT\nEXISTS my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		"ALTER SECRET IF\nEXISTS my_secret SET COMMENT = 'x'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for newlines in clauses, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterQuotedMultiPartName(t *testing.T) {
	// ALTER SECRET with quoted multi-part identifiers should be accepted.
	validCases := []string{
		`ALTER SECRET "my db"."my schema"."my-secret" SET COMMENT = 'x'`,
		`ALTER SECRET IF EXISTS "my db"."my schema"."my-secret" SET USERNAME = 'u' PASSWORD = 'p'`,
		`ALTER SECRET "special-schema"."secret_name" SET SECRET_STRING = 'val'`,
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for quoted multi-part ALTER SECRET, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MissingTypeWithModifiers(t *testing.T) {
	// CREATE SECRET with OR REPLACE or IF NOT EXISTS modifier and valid name but
	// no TYPE clause should produce "requires TYPE" — not "Unexpected syntax".
	cases := []struct {
		name string
		sql  string
	}{
		{"OR REPLACE no TYPE", "CREATE OR REPLACE SECRET my_secret"},
		{"OR REPLACE no TYPE with trailing whitespace", "CREATE OR REPLACE SECRET my_secret   "},
		{"IF NOT EXISTS no TYPE", "CREATE SECRET IF NOT EXISTS my_secret"},
		{"IF NOT EXISTS three-part name no TYPE", "CREATE SECRET IF NOT EXISTS db.schema.my_secret"},
		{"OR REPLACE three-part name no TYPE", "CREATE OR REPLACE SECRET db.schema.my_secret"},
		{"OR REPLACE quoted name no TYPE", `CREATE OR REPLACE SECRET "my-secret"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "CREATE SECRET requires TYPE") {
				t.Errorf("Expected 'CREATE SECRET requires TYPE' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_InvalidNameFormat(t *testing.T) {
	// A name starting with a digit does not match _ident, so the validator
	// should treat it as "Unexpected syntax" because no name was extracted.
	cases := []string{
		"CREATE SECRET 123abc TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET 9secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for digit-prefixed name")
			}
			if !strings.Contains(warns[0].Message, "Unexpected syntax") {
				t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterMultiPartNameNoAction(t *testing.T) {
	// ALTER SECRET with a two- or three-part name but no valid sub-command
	// should produce an "Unknown ALTER SECRET sub-command" warning.
	cases := []string{
		"ALTER SECRET db.schema.my_secret",
		"ALTER SECRET schema.my_secret",
		"ALTER SECRET IF EXISTS db.schema.my_secret",
		"ALTER SECRET IF EXISTS schema.my_secret",
		"ALTER SECRET db.schema.my_secret REFRESH",
		"ALTER SECRET IF EXISTS db.schema.my_secret DROP",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_TypeWithoutEquals(t *testing.T) {
	// TYPE keyword present but without "=" should produce "requires TYPE"
	// because the TYPE regex requires the equals sign.
	cases := []string{
		"CREATE SECRET my_secret TYPE OAUTH2",
		"CREATE SECRET my_secret TYPE PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for TYPE without =")
			}
			if !strings.Contains(warns[0].Message, "CREATE SECRET requires TYPE") {
				t.Errorf("Expected 'CREATE SECRET requires TYPE' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_SemicolonSplitsStatement(t *testing.T) {
	// A semicolon in the middle of a CREATE SECRET splits it into two
	// statements — the first is incomplete and should produce warnings.
	sql := "CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u'; PASSWORD = 'p'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	// First statement is CREATE SECRET ... USERNAME = 'u' which is missing PASSWORD.
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "PASSWORD") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing PASSWORD when semicolon splits statement, got: %v", warnMsgs(warns))
	}
}

func TestValidateSnowflakePatterns_Secret_LineCommentHidesProperty(t *testing.T) {
	// A mandatory property keyword after a line comment should be stripped by
	// cleanParseText and should not satisfy mandatory checks — complementing
	// the block-comment test (TestValidateSnowflakePatterns_Secret_InlineCommentHidesProperty).
	t.Run("end-of-line comment hides PASSWORD", func(t *testing.T) {
		sql := "CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' -- PASSWORD = 'p'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Fatal("Expected warning for PASSWORD hidden behind line comment")
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "PASSWORD") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing PASSWORD, got: %v", warnMsgs(warns))
		}
	})
	t.Run("multi-line with commented-out property", func(t *testing.T) {
		sql := "CREATE SECRET my_secret TYPE = PASSWORD\n  USERNAME = 'u'\n  -- PASSWORD = 'p'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Fatal("Expected warning for PASSWORD on commented-out line")
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "PASSWORD") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing PASSWORD, got: %v", warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_AlterIfExistsUnsetComment(t *testing.T) {
	// ALTER SECRET IF EXISTS ... UNSET COMMENT should be valid.
	cases := []string{
		"ALTER SECRET IF EXISTS my_secret UNSET COMMENT",
		"ALTER SECRET IF EXISTS db.schema.my_secret UNSET COMMENT",
		`ALTER SECRET IF EXISTS "my-secret" UNSET COMMENT`,
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for ALTER SECRET IF EXISTS UNSET COMMENT, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MultiStatementAlter(t *testing.T) {
	// Mixed valid and invalid ALTER SECRET statements in a batch.
	t.Run("valid alter in batch", func(t *testing.T) {
		sql := "SELECT 1;\nALTER SECRET my_secret SET COMMENT = 'ok';\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for valid ALTER SECRET in batch, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("invalid alter in batch", func(t *testing.T) {
		sql := "SELECT 1;\nALTER SECRET my_secret REFRESH;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning for invalid ALTER SECRET in batch, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("mixed valid and invalid alters", func(t *testing.T) {
		sql := "ALTER SECRET s1 SET COMMENT = 'ok';\nALTER SECRET s2 REFRESH;\nALTER SECRET s3 UNSET COMMENT;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning (only s2 invalid), got %d: %v", len(warns), warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_AlterTrailingWhitespace(t *testing.T) {
	// ALTER SECRET with only trailing whitespace (no name) should produce
	// "requires a secret name" — same as bare "ALTER SECRET".
	cases := []string{
		"ALTER SECRET   ",
		"ALTER SECRET\t\n",
		"ALTER SECRET \t ",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "ALTER SECRET requires a secret name") {
				t.Errorf("Expected 'ALTER SECRET requires a secret name' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterBareUnset(t *testing.T) {
	// ALTER SECRET ... UNSET without specifying a property should produce
	// "Unknown ALTER SECRET sub-command" because only UNSET COMMENT is valid.
	cases := []string{
		"ALTER SECRET my_secret UNSET",
		"ALTER SECRET IF EXISTS my_secret UNSET",
		"ALTER SECRET my_secret UNSET   ",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CreateQuotedThreePartName(t *testing.T) {
	// CREATE SECRET with quoted three-part identifiers should be accepted
	// (complementing TestValidateSnowflakePatterns_Secret_AlterQuotedMultiPartName).
	validCases := []string{
		`CREATE SECRET "my db"."my schema"."my-secret" TYPE = GENERIC_STRING SECRET_STRING = 'val'`,
		`CREATE OR REPLACE SECRET "db"."schema"."name" TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'`,
		`CREATE SECRET IF NOT EXISTS "db"."sch"."s" TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256'`,
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for quoted three-part CREATE SECRET, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_TypeValueSpecialChars(t *testing.T) {
	// TYPE values with special characters: the regex ([\w]+) stops at
	// non-word characters, producing different errors depending on whether
	// a partial word is captured.
	cases := []struct {
		name    string
		sql     string
		wantMsg string
	}{
		{
			"hyphenated captures prefix only",
			"CREATE SECRET my_secret TYPE = OAUTH-2 API_AUTHENTICATION = my_int",
			"Unknown TYPE",
		},
		{
			"non-word prefix means TYPE not found",
			"CREATE SECRET my_secret TYPE = @OAUTH2 API_AUTHENTICATION = my_int",
			"CREATE SECRET requires TYPE",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for TYPE with special characters")
			}
			if !strings.Contains(warns[0].Message, tc.wantMsg) {
				t.Errorf("Expected warning containing %q, got: %s", tc.wantMsg, warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CommentBetweenSetAndProperty(t *testing.T) {
	// A comment between SET and the property keyword should be stripped by
	// cleanParseText, and the ALTER should still be recognized as valid.
	validCases := []string{
		"ALTER SECRET my_secret SET /* note */ COMMENT = 'x'",
		"ALTER SECRET my_secret SET -- setting comment\nCOMMENT = 'new'",
		"ALTER SECRET my_secret SET /* hidden */ SECRET_STRING = 'val'",
		"ALTER SECRET my_secret SET -- update\nUSERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for comment between SET and property, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CaseInsensitivePropertyNames(t *testing.T) {
	// Property keywords should be case-insensitive (regexes use (?i)).
	// MixedCaseTypeValues tests TYPE value casing; this tests property KEYWORD casing.
	validCases := []string{
		"CREATE SECRET my_secret TYPE = OAUTH2 api_authentication = my_int",
		"CREATE SECRET my_secret TYPE = PASSWORD username = 'u' password = 'p'",
		"CREATE SECRET my_secret TYPE = GENERIC_STRING secret_string = 'val'",
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN Api_Authentication = my_int enabled = TRUE",
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY algorithm = 'AES-256'",
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int oauth_scopes = ('s1') comment = 'note'",
		"ALTER SECRET my_secret SET username = 'u' password = 'p'",
		"ALTER SECRET my_secret SET secret_string = 'val'",
		"ALTER SECRET my_secret SET comment = 'note'",
		"ALTER SECRET my_secret unset comment",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for lowercase property names %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_OrReplaceIfNotExistsNoName(t *testing.T) {
	// Both OR REPLACE and IF NOT EXISTS present but no name — conflict check
	// should fire before the name check.
	cases := []string{
		"CREATE OR REPLACE SECRET IF NOT EXISTS",
		"CREATE OR REPLACE SECRET IF NOT EXISTS   ",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for OR REPLACE + IF NOT EXISTS without name")
			}
			// The conflict check runs first; we expect the conflict message.
			if !strings.Contains(warns[0].Message, "Conflict between OR REPLACE and IF NOT EXISTS") {
				t.Errorf("Expected conflict warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_UnexpectedPropertyNonPasswordTypes(t *testing.T) {
	// validateProperties should flag unexpected properties on all TYPE values,
	// not just PASSWORD (which is tested in the main table).
	cases := []struct {
		name string
		sql  string
	}{
		{"OAUTH2 with WAREHOUSE", "CREATE SECRET s TYPE = OAUTH2 API_AUTHENTICATION = my_int WAREHOUSE = my_wh"},
		{"GENERIC_STRING with SCHEDULE", "CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'val' SCHEDULE = 'USING CRON 0 * * * *'"},
		{"CLOUD_PROVIDER_TOKEN with CATALOG", "CREATE SECRET s TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int CATALOG = my_cat"},
		{"SYMMETRIC_KEY with WAREHOUSE", "CREATE SECRET s TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' WAREHOUSE = my_wh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "Unexpected property") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected 'Unexpected property' warning for %s, got: %v", tc.name, warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterSetEmptyStringValue(t *testing.T) {
	// ALTER SECRET SET with an empty string value ('') — cleanParseText strips
	// string literals but the SET keyword regex should still match.
	validCases := []string{
		"ALTER SECRET my_secret SET COMMENT = ''",
		"ALTER SECRET my_secret SET SECRET_STRING = ''",
		"ALTER SECRET my_secret SET USERNAME = '' PASSWORD = ''",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for ALTER SET with empty string, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CommentPropertyWithoutType(t *testing.T) {
	// CREATE SECRET with only COMMENT (no TYPE) should still produce a
	// "requires TYPE" warning — the presence of optional properties must not
	// suppress the mandatory TYPE check.
	sql := "CREATE SECRET my_secret COMMENT = 'some note'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected warning for CREATE SECRET with COMMENT but no TYPE")
	}
	if !strings.Contains(warns[0].Message, "CREATE SECRET requires TYPE") {
		t.Errorf("Expected 'CREATE SECRET requires TYPE' warning, got: %s", warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_CreateNameStartsWithIF(t *testing.T) {
	// Names starting with "IF" should not be confused with "IF NOT EXISTS" prefix.
	// Parity with TestValidateSnowflakePatterns_Secret_AlterNameStartsWithIF.
	validCases := []string{
		"CREATE SECRET IF_SECRET TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET IFOO TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterNewlinesInIfExists(t *testing.T) {
	// Newlines within ALTER SECRET IF EXISTS should be accepted.
	// Parity with TestValidateSnowflakePatterns_Secret_NewlinesInClauses (CREATE side).
	validCases := []string{
		"ALTER SECRET IF\nEXISTS my_secret SET COMMENT = 'x'",
		"ALTER SECRET IF\r\nEXISTS my_secret SET SECRET_STRING = 'val'",
		"ALTER SECRET IF\nEXISTS my_secret UNSET COMMENT",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for newlines in ALTER IF EXISTS, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MultipleTYPEDeclarations(t *testing.T) {
	// When TYPE appears twice, reSecretType captures the first occurrence.
	// The second TYPE value's properties trigger cross-type warnings.
	sql := "CREATE SECRET s TYPE = OAUTH2 TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	// First TYPE = OAUTH2 wins → missing API_AUTHENTICATION + USERNAME/PASSWORD cross-type.
	if len(warns) < 1 {
		t.Fatal("Expected at least 1 warning for duplicate TYPE declarations")
	}
	foundAPIA := false
	for _, w := range warns {
		if strings.Contains(w.Message, "API_AUTHENTICATION") {
			foundAPIA = true
		}
	}
	if !foundAPIA {
		t.Errorf("Expected API_AUTHENTICATION warning (first TYPE=OAUTH2 wins), got: %v", warnMsgs(warns))
	}
}

func TestValidateSnowflakePatterns_Secret_CreateIfExistsWrongModifier(t *testing.T) {
	// CREATE SECRET IF EXISTS (should be IF NOT EXISTS) — the validator
	// treats "IF" as the secret name because the optional group only matches
	// IF NOT EXISTS. The statement validates without warnings since TYPE and
	// mandatory properties are present.
	sql := "CREATE SECRET IF EXISTS my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	// "IF" is captured as the name; TYPE + SECRET_STRING satisfy GENERIC_STRING.
	if len(warns) != 0 {
		t.Errorf("Expected 0 warnings (wrong modifier not detected, name='IF'), got %d: %v", len(warns), warnMsgs(warns))
	}
}

func TestValidateSnowflakePatterns_Secret_AlterIfNotExistsWrongModifier(t *testing.T) {
	// ALTER SECRET IF NOT EXISTS (should be IF EXISTS) — the validator
	// treats "IF" as the secret name because the optional group only matches
	// IF EXISTS. The statement validates without warnings since SET COMMENT
	// is found in the text.
	sql := "ALTER SECRET IF NOT EXISTS my_secret SET COMMENT = 'x'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	// "IF" is captured as the name and "SET COMMENT" matches the action regex.
	if len(warns) != 0 {
		t.Errorf("Expected 0 warnings (wrong modifier not detected, name='IF'), got %d: %v", len(warns), warnMsgs(warns))
	}
}

func TestValidateSnowflakePatterns_Secret_FourPartName(t *testing.T) {
	// A four-part name (a.b.c.d) exceeds the three-part _identPath limit.
	// The regex captures only the first three parts; the fourth part is
	// ignored and validation proceeds normally.
	t.Run("CREATE four-part name", func(t *testing.T) {
		sql := "CREATE SECRET a.b.c.d TYPE = GENERIC_STRING SECRET_STRING = 'val'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Name captures "a.b.c"; ".d" is leftover but doesn't interfere.
		if len(warns) != 0 {
			t.Errorf("Expected 0 warnings for four-part name, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("ALTER four-part name", func(t *testing.T) {
		sql := "ALTER SECRET a.b.c.d SET COMMENT = 'x'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 0 {
			t.Errorf("Expected 0 warnings for four-part name, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_TypeValueOnNewline(t *testing.T) {
	// TYPE = <value> split across lines — \s* in reSecretType should match newlines.
	validCases := []string{
		"CREATE SECRET my_secret TYPE =\nOAUTH2 API_AUTHENTICATION = my_int",
		"CREATE SECRET my_secret TYPE =\n  PASSWORD\n  USERNAME = 'u'\n  PASSWORD = 'p'",
		"CREATE SECRET my_secret TYPE\n=\nGENERIC_STRING\nSECRET_STRING = 'val'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for TYPE value on newline, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_TypeValueAfterBlockComment(t *testing.T) {
	// A block comment between TYPE = and the value should be stripped by
	// cleanParseText, leaving TYPE = <value> for the regex to match.
	validCases := []string{
		"CREATE SECRET my_secret TYPE = /* note */ OAUTH2 API_AUTHENTICATION = my_int",
		"CREATE SECRET my_secret TYPE = /* type */ PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for TYPE value after block comment, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterActionHiddenInComment(t *testing.T) {
	// When the ALTER sub-command is inside a comment, cleanParseText strips it,
	// leaving no valid action — should produce "Unknown ALTER SECRET sub-command".
	cases := []struct {
		name string
		sql  string
	}{
		{"line comment hides SET COMMENT", "ALTER SECRET my_secret -- SET COMMENT = 'x'"},
		{"block comment hides SET", "ALTER SECRET my_secret /* SET USERNAME = 'u' PASSWORD = 'p' */"},
		{"block comment hides UNSET", "ALTER SECRET my_secret /* UNSET COMMENT */"},
		{"multi-line with action on commented line", "ALTER SECRET my_secret\n-- SET COMMENT = 'x'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_ConflictWithNewlines(t *testing.T) {
	// OR REPLACE + IF NOT EXISTS conflict should be detected even when the
	// modifier keywords are split across lines.
	cases := []string{
		"CREATE OR\nREPLACE SECRET IF\nNOT\nEXISTS my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		"CREATE OR\r\nREPLACE SECRET IF NOT EXISTS my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected conflict warning for OR REPLACE + IF NOT EXISTS with newlines")
			}
			if !strings.Contains(warns[0].Message, "Conflict between OR REPLACE and IF NOT EXISTS") {
				t.Errorf("Expected conflict warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_CreateTwoPartQuotedName(t *testing.T) {
	// CREATE SECRET with two-part quoted identifier should be accepted.
	// Parity with TestValidateSnowflakePatterns_Secret_AlterQuotedMultiPartName.
	validCases := []string{
		`CREATE SECRET "my schema"."my-secret" TYPE = GENERIC_STRING SECRET_STRING = 'val'`,
		`CREATE OR REPLACE SECRET "schema"."name" TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'`,
		`CREATE SECRET IF NOT EXISTS "sch"."s" TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256'`,
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for two-part quoted CREATE SECRET, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterExtraWhitespace(t *testing.T) {
	// ALTER SECRET with extra whitespace between tokens should be accepted.
	// Parity with the CREATE extra-whitespace valid case.
	validCases := []string{
		"ALTER   SECRET   my_secret   SET   COMMENT = 'x'",
		"ALTER\tSECRET\tmy_secret\tSET\tUSERNAME = 'u'\tPASSWORD = 'p'",
		"ALTER   SECRET   IF   EXISTS   my_secret   UNSET   COMMENT",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for extra whitespace ALTER SECRET, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterSetNewlineBeforeProperty(t *testing.T) {
	// ALTER SECRET SET with a newline directly between SET and the property
	// name (no intervening comment) — verifies \s+ in reAlterSecretAction
	// spans newlines.
	validCases := []string{
		"ALTER SECRET my_secret SET\nCOMMENT = 'x'",
		"ALTER SECRET my_secret SET\nUSERNAME = 'u' PASSWORD = 'p'",
		"ALTER SECRET my_secret SET\n  SECRET_STRING = 'val'",
		"ALTER SECRET my_secret UNSET\nCOMMENT",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for SET with newline before property, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_TypeEqualsNoSpaces(t *testing.T) {
	// TYPE=VALUE with zero whitespace around the equals sign should be
	// accepted — reSecretType uses \s* on both sides of =.
	validCases := []string{
		"CREATE SECRET my_secret TYPE=OAUTH2 API_AUTHENTICATION=my_int",
		"CREATE SECRET my_secret TYPE=PASSWORD USERNAME='u' PASSWORD='p'",
		"CREATE SECRET my_secret TYPE=GENERIC_STRING SECRET_STRING='val'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for TYPE= with no spaces, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterCRLFLineEndings(t *testing.T) {
	// ALTER SECRET with CRLF (\r\n) line endings should be accepted.
	// Parity with the CREATE CRLF valid case.
	validCases := []string{
		"ALTER SECRET my_secret\r\n  SET USERNAME = 'u'\r\n  PASSWORD = 'p'",
		"ALTER SECRET IF EXISTS my_secret\r\n  SET COMMENT = 'x'",
		"ALTER SECRET my_secret\r\n  UNSET COMMENT",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for ALTER SECRET with CRLF, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_TrailingCommentHidesType(t *testing.T) {
	// A line comment at the end that hides the TYPE clause should cause the
	// mandatory TYPE check to fire — cleanParseText strips the comment.
	sql := "CREATE SECRET my_secret -- TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected warning for TYPE hidden behind trailing line comment")
	}
	if !strings.Contains(warns[0].Message, "CREATE SECRET requires TYPE") {
		t.Errorf("Expected 'CREATE SECRET requires TYPE' warning, got: %s", warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_NameLiterallyIF(t *testing.T) {
	// Name literally "IF" (not "IF_" or "IFOO") should be accepted when the
	// full modifier clause (IF NOT EXISTS / IF EXISTS) does not match.
	// Complements TestValidateSnowflakePatterns_Secret_AlterNameStartsWithIF and
	// TestValidateSnowflakePatterns_Secret_CreateNameStartsWithIF.
	t.Run("CREATE SECRET IF TYPE", func(t *testing.T) {
		sql := "CREATE SECRET IF TYPE = GENERIC_STRING SECRET_STRING = 'val'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for name='IF', got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("ALTER SECRET IF SET COMMENT", func(t *testing.T) {
		sql := "ALTER SECRET IF SET COMMENT = 'x'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for name='IF', got %d: %v", len(warns), warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_AlterIfExistsQuotedNameNoAction(t *testing.T) {
	// ALTER SECRET IF EXISTS with a quoted identifier but no valid sub-command
	// should produce "Unknown ALTER SECRET sub-command".
	cases := []string{
		`ALTER SECRET IF EXISTS "my-secret"`,
		`ALTER SECRET IF EXISTS "db"."schema"."secret"`,
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterUnsetRemainingInvalidProperties(t *testing.T) {
	// Only UNSET COMMENT is valid for ALTER SECRET. Verify that UNSET with
	// other property names produces "Unknown ALTER SECRET sub-command".
	// Complements the existing tests for UNSET SECRET_STRING, USERNAME,
	// PASSWORD, OAUTH_REFRESH_TOKEN, and API_AUTHENTICATION.
	cases := []struct {
		name string
		sql  string
	}{
		{"UNSET OAUTH_SCOPES", "ALTER SECRET my_secret UNSET OAUTH_SCOPES"},
		{"UNSET ALGORITHM", "ALTER SECRET my_secret UNSET ALGORITHM"},
		{"UNSET ENABLED", "ALTER SECRET my_secret UNSET ENABLED"},
		{"UNSET OAUTH_REFRESH_TOKEN_EXPIRY_TIME", "ALTER SECRET my_secret UNSET OAUTH_REFRESH_TOKEN_EXPIRY_TIME"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_EmptyQuotedIdentifier(t *testing.T) {
	// An empty quoted identifier ("") does not match _ident's "[^"]+" pattern
	// (requires at least one character between quotes), so the name regex
	// fails — producing "Unexpected syntax" for CREATE and "requires a
	// secret name" for ALTER.
	t.Run("CREATE with empty quoted name", func(t *testing.T) {
		sql := `CREATE SECRET "" TYPE = GENERIC_STRING SECRET_STRING = 'val'`
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Fatal("Expected at least 1 warning for empty quoted identifier")
		}
		if !strings.Contains(warns[0].Message, "Unexpected syntax") {
			t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
		}
	})
	t.Run("ALTER with empty quoted name", func(t *testing.T) {
		sql := `ALTER SECRET "" SET COMMENT = 'x'`
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) == 0 {
			t.Fatal("Expected at least 1 warning for empty quoted identifier")
		}
		if !strings.Contains(warns[0].Message, "ALTER SECRET requires a secret name") {
			t.Errorf("Expected 'ALTER SECRET requires a secret name' warning, got: %s", warns[0].Message)
		}
	})
}

func TestValidateSnowflakePatterns_Secret_AlterSetValidPlusExtraToken(t *testing.T) {
	// ALTER SECRET SET validates only that at least one known sub-command is
	// present; additional unrecognized tokens after the valid property are not
	// flagged. This test documents that permissive behavior.
	validCases := []string{
		"ALTER SECRET my_secret SET COMMENT = 'x' FOOBAR = 'y'",
		"ALTER SECRET my_secret SET SECRET_STRING = 'v' EXTRA = 'e'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings (permissive ALTER validation), got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

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

		// -- PASS: Two-part name (schema.name)
		"CREATE SECRET schema.my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: Quoted identifier name
		"CREATE SECRET \"my-secret\" TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: Extra whitespace between tokens
		"CREATE   SECRET   my_secret   TYPE  =  OAUTH2   API_AUTHENTICATION  =  my_int",
		// -- PASS: CLOUD_PROVIDER_TOKEN with all optional properties
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int ENABLED = FALSE COMMENT = 'note'",
		// -- PASS: OAUTH2 with OAUTH_REFRESH_TOKEN_EXPIRY_TIME
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_REFRESH_TOKEN = 'tok' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31 00:00:00'",
		// -- PASS: Trailing semicolons handled by GetStatementRanges
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val';",

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
		// -- PASS: ALTER SECRET case insensitivity
		"alter secret my_secret set comment = 'updated'",
		// -- PASS: ALTER SECRET UNSET COMMENT (case-insensitive)
		"alter secret my_secret unset comment",
		// -- PASS: ALTER SECRET SET OAUTH_REFRESH_TOKEN_EXPIRY_TIME
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2026-06-01 00:00:00'",
		// -- PASS: ALTER SECRET with quoted identifier
		"ALTER SECRET \"my-secret\" SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET with two-part name (schema.name)
		"ALTER SECRET schema.my_secret SET SECRET_STRING = 'new_val'",
		// -- PASS: CREATE SECRET OAUTH2 with only OAUTH_SCOPES (subset of optional props)
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_SCOPES = ('scope1')",
		// -- PASS: ALTER SECRET IF EXISTS with quoted identifier
		"ALTER SECRET IF EXISTS \"my-secret\" SET USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: ALTER SECRET with trailing semicolon
		"ALTER SECRET my_secret SET COMMENT = 'updated';",
		// -- PASS: CREATE OR REPLACE with three-part name
		"CREATE OR REPLACE SECRET db.schema.my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",

		// -- PASS: OAUTH2 with only OAUTH_REFRESH_TOKEN (no EXPIRY_TIME)
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_REFRESH_TOKEN = 'tok'",
		// -- PASS: OAUTH2 with only OAUTH_REFRESH_TOKEN_EXPIRY_TIME (no REFRESH_TOKEN)
		"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31 00:00:00'",
		// -- PASS: Multi-line CREATE SECRET (newlines between tokens)
		"CREATE SECRET my_secret\n  TYPE = PASSWORD\n  USERNAME = 'u'\n  PASSWORD = 'p'",
		// -- PASS: SQL comment embedded in CREATE SECRET
		"CREATE SECRET my_secret /* oauth secret */ TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: Line comment in CREATE SECRET
		"CREATE SECRET my_secret -- my secret\nTYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: CREATE SECRET IF NOT EXISTS with two-part name
		"CREATE SECRET IF NOT EXISTS schema.my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: ALTER SECRET IF EXISTS with two-part name
		"ALTER SECRET IF EXISTS schema.my_secret SET COMMENT = 'updated'",
		// -- PASS: ALTER SECRET with multi-line
		"ALTER SECRET my_secret\n  SET USERNAME = 'u'\n  PASSWORD = 'p'",
		// -- PASS: ALTER SECRET SET PASSWORD alone (partial update)
		"ALTER SECRET my_secret SET PASSWORD = 'new_pass'",
		// -- PASS: ALTER SECRET SET USERNAME alone (partial update)
		"ALTER SECRET my_secret SET USERNAME = 'new_user'",
		// -- PASS: ALTER SECRET SET OAUTH_REFRESH_TOKEN alone (partial update)
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN = 'new_token'",
		// -- PASS: CREATE SECRET IF NOT EXISTS with three-part name
		"CREATE SECRET IF NOT EXISTS db.schema.my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: CREATE OR REPLACE SECRET with quoted identifier
		"CREATE OR REPLACE SECRET \"my-secret\" TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",

		// -- PASS: Tab characters between tokens
		"CREATE SECRET\tmy_secret\tTYPE = OAUTH2\tAPI_AUTHENTICATION = my_int",
		// -- PASS: Mixed tabs and spaces
		"CREATE\t SECRET \tmy_secret\t TYPE\t=\tPASSWORD\tUSERNAME = 'u'\tPASSWORD = 'p'",
		// -- PASS: ALTER SECRET with block comment
		"ALTER SECRET my_secret /* update comment */ SET COMMENT = 'new'",
		// -- PASS: ALTER SECRET with line comment
		"ALTER SECRET my_secret -- updating secret\nSET SECRET_STRING = 'new_val'",
		// -- PASS: ALTER SECRET IF EXISTS with block comment
		"ALTER SECRET IF EXISTS /* checking */ my_secret SET USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: CREATE SECRET with Windows-style line endings (CRLF)
		"CREATE SECRET my_secret\r\n  TYPE = GENERIC_STRING\r\n  SECRET_STRING = 'val'",
		// -- PASS: CREATE SECRET name starting with underscore
		"CREATE SECRET _private_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: CREATE SECRET name with dollar sign
		"CREATE SECRET my$secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		// -- PASS: CREATE SECRET with numeric suffix in name
		"CREATE SECRET secret_123 TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		// -- PASS: CLOUD_PROVIDER_TOKEN with all allowed properties
		"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int ENABLED = TRUE COMMENT = 'full'",
		// -- PASS: SYMMETRIC_KEY with COMMENT (all allowed properties)
		"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' COMMENT = 'full'",
		// -- PASS: GENERIC_STRING with COMMENT (all allowed properties)
		"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' COMMENT = 'full'",
		// -- PASS: PASSWORD with COMMENT (all allowed properties)
		"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' COMMENT = 'full'",
		// -- PASS: ALTER SECRET SET multiple OAUTH2 properties
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN = 'tok' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2026-12-31 00:00:00' OAUTH_SCOPES = ('a', 'b')",
		// -- PASS: CREATE SECRET quoted identifier with spaces
		"CREATE SECRET \"My Secret Name\" TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		// -- PASS: CREATE SECRET quoted identifier with dots
		"CREATE SECRET \"db.name\" TYPE = GENERIC_STRING SECRET_STRING = 'val'",
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

		// -- FAIL: Multiple cross-type violations
		{
			"CREATE SECRET GENERIC_STRING with USERNAME and PASSWORD",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' USERNAME = 'u' PASSWORD = 'p'",
			[]string{"USERNAME is not valid for TYPE = GENERIC_STRING", "PASSWORD is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN on SYMMETRIC_KEY
		{
			"CREATE SECRET SYMMETRIC_KEY with OAUTH_REFRESH_TOKEN",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_REFRESH_TOKEN is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN_EXPIRY_TIME on PASSWORD type
		{
			"CREATE SECRET PASSWORD with OAUTH_REFRESH_TOKEN_EXPIRY_TIME",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'",
			[]string{"OAUTH_REFRESH_TOKEN_EXPIRY_TIME is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: SECRET_STRING on CLOUD_PROVIDER_TOKEN
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with SECRET_STRING",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int SECRET_STRING = 'val'",
			[]string{"SECRET_STRING is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: ENABLED on GENERIC_STRING type
		{
			"CREATE SECRET GENERIC_STRING with ENABLED",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' ENABLED = TRUE",
			[]string{"ENABLED is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: ALGORITHM on PASSWORD type
		{
			"CREATE SECRET PASSWORD with ALGORITHM",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' ALGORITHM = 'AES'",
			[]string{"ALGORITHM is not valid for TYPE = PASSWORD"},
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
		// -- FAIL: ALTER SECRET with no sub-command (just name)
		{
			"ALTER SECRET no sub-command",
			"ALTER SECRET my_secret",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET UNSET non-COMMENT property
		{
			"ALTER SECRET UNSET SECRET_STRING",
			"ALTER SECRET my_secret UNSET SECRET_STRING",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET IF EXISTS missing name (parser treats "IF" as name, "EXISTS" as unknown action)
		{
			"ALTER SECRET IF EXISTS missing name",
			"ALTER SECRET IF EXISTS",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET bare SET (no property after SET)
		{
			"ALTER SECRET bare SET",
			"ALTER SECRET my_secret SET",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET SET ENABLED (not a valid ALTER SET target)
		{
			"ALTER SECRET SET ENABLED",
			"ALTER SECRET my_secret SET ENABLED = TRUE",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET SET ALGORITHM (not a valid ALTER SET target)
		{
			"ALTER SECRET SET ALGORITHM",
			"ALTER SECRET my_secret SET ALGORITHM = 'AES-256'",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET UNSET USERNAME (only COMMENT can be UNSET)
		{
			"ALTER SECRET UNSET USERNAME",
			"ALTER SECRET my_secret UNSET USERNAME",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET UNSET PASSWORD (only COMMENT can be UNSET)
		{
			"ALTER SECRET UNSET PASSWORD",
			"ALTER SECRET my_secret UNSET PASSWORD",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ENABLED on OAUTH2 type
		{
			"CREATE SECRET OAUTH2 with ENABLED",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int ENABLED = TRUE",
			[]string{"ENABLED is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: OAUTH_SCOPES on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with OAUTH_SCOPES",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int OAUTH_SCOPES = ('s1')",
			[]string{"OAUTH_SCOPES is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: SECRET_STRING on OAUTH2 type
		{
			"CREATE SECRET OAUTH2 with SECRET_STRING",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int SECRET_STRING = 'val'",
			[]string{"SECRET_STRING is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: PASSWORD property on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with PASSWORD",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int PASSWORD = 'p'",
			[]string{"PASSWORD is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: ALTER SECRET IF EXISTS with name but no sub-command
		{
			"ALTER SECRET IF EXISTS no sub-command",
			"ALTER SECRET IF EXISTS my_secret",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: API_AUTHENTICATION on PASSWORD type (shared prop, valid for OAUTH2+CLOUD_PROVIDER_TOKEN only)
		{
			"CREATE SECRET PASSWORD with API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' API_AUTHENTICATION = my_int",
			[]string{"API_AUTHENTICATION is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: API_AUTHENTICATION on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' API_AUTHENTICATION = my_int",
			[]string{"API_AUTHENTICATION is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: SECRET_STRING on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with SECRET_STRING",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' SECRET_STRING = 'val'",
			[]string{"SECRET_STRING is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: PASSWORD property on OAUTH2 type (prop name collides with TYPE name)
		{
			"CREATE SECRET OAUTH2 with PASSWORD prop",
			"CREATE SECRET my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int PASSWORD = 'p'",
			[]string{"PASSWORD is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: OAUTH_SCOPES on PASSWORD type
		{
			"CREATE SECRET PASSWORD with OAUTH_SCOPES",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' OAUTH_SCOPES = ('s1')",
			[]string{"OAUTH_SCOPES is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN_EXPIRY_TIME on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with OAUTH_REFRESH_TOKEN_EXPIRY_TIME",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'",
			[]string{"OAUTH_REFRESH_TOKEN_EXPIRY_TIME is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},

		// -- FAIL: CREATE SECRET bare name only (no TYPE, no properties)
		{
			"CREATE SECRET bare name only",
			"CREATE SECRET my_secret",
			[]string{"CREATE SECRET requires TYPE"},
		},
		// -- FAIL: ALGORITHM on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with ALGORITHM",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int ALGORITHM = 'AES-256'",
			[]string{"ALGORITHM is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: USERNAME on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with USERNAME",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int USERNAME = 'u'",
			[]string{"USERNAME is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: ENABLED on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with ENABLED",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' ENABLED = TRUE",
			[]string{"ENABLED is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: OAUTH_SCOPES on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with OAUTH_SCOPES",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' OAUTH_SCOPES = ('s1')",
			[]string{"OAUTH_SCOPES is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN on GENERIC_STRING type
		{
			"CREATE SECRET GENERIC_STRING with OAUTH_REFRESH_TOKEN",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_REFRESH_TOKEN is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN_EXPIRY_TIME on GENERIC_STRING type
		{
			"CREATE SECRET GENERIC_STRING with OAUTH_REFRESH_TOKEN_EXPIRY_TIME",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'",
			[]string{"OAUTH_REFRESH_TOKEN_EXPIRY_TIME is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: ALGORITHM on GENERIC_STRING type
		{
			"CREATE SECRET GENERIC_STRING with ALGORITHM",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val' ALGORITHM = 'AES-256'",
			[]string{"ALGORITHM is not valid for TYPE = GENERIC_STRING"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN on CLOUD_PROVIDER_TOKEN type
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN with OAUTH_REFRESH_TOKEN",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN API_AUTHENTICATION = my_int OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_REFRESH_TOKEN is not valid for TYPE = CLOUD_PROVIDER_TOKEN"},
		},
		// -- FAIL: PASSWORD on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with PASSWORD",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' PASSWORD = 'p'",
			[]string{"PASSWORD is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: OAUTH_REFRESH_TOKEN_EXPIRY_TIME on SYMMETRIC_KEY type
		{
			"CREATE SECRET SYMMETRIC_KEY with OAUTH_REFRESH_TOKEN_EXPIRY_TIME",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '2025-12-31'",
			[]string{"OAUTH_REFRESH_TOKEN_EXPIRY_TIME is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: Multiple unexpected properties
		{
			"CREATE SECRET multiple unexpected properties",
			"CREATE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' WAREHOUSE = my_wh SCHEDULE = 'USING CRON 0 * * * *'",
			[]string{"Unexpected property", "Unexpected property"},
		},
		// -- FAIL: ALTER SECRET UNSET OAUTH_REFRESH_TOKEN (only COMMENT can be UNSET)
		{
			"ALTER SECRET UNSET OAUTH_REFRESH_TOKEN",
			"ALTER SECRET my_secret UNSET OAUTH_REFRESH_TOKEN",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET UNSET API_AUTHENTICATION (only COMMENT can be UNSET)
		{
			"ALTER SECRET UNSET API_AUTHENTICATION",
			"ALTER SECRET my_secret UNSET API_AUTHENTICATION",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET SET TYPE (TYPE is not a valid ALTER SET target)
		{
			"ALTER SECRET SET TYPE",
			"ALTER SECRET my_secret SET TYPE = OAUTH2",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: ALTER SECRET SET unknown property (completely unrecognized)
		{
			"ALTER SECRET SET unknown property",
			"ALTER SECRET my_secret SET WAREHOUSE = my_wh",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: CREATE SECRET OAUTH2 with COMMENT but missing mandatory API_AUTHENTICATION
		{
			"CREATE SECRET OAUTH2 COMMENT but no API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = OAUTH2 COMMENT = 'note'",
			[]string{"TYPE = OAUTH2 requires API_AUTHENTICATION"},
		},
		// -- FAIL: CREATE SECRET CLOUD_PROVIDER_TOKEN with optional ENABLED but missing mandatory API_AUTHENTICATION
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN ENABLED but no API_AUTHENTICATION",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN ENABLED = TRUE",
			[]string{"TYPE = CLOUD_PROVIDER_TOKEN requires API_AUTHENTICATION"},
		},
		// -- FAIL: CREATE SECRET SYMMETRIC_KEY with COMMENT but missing mandatory ALGORITHM
		{
			"CREATE SECRET SYMMETRIC_KEY COMMENT but no ALGORITHM",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY COMMENT = 'note'",
			[]string{"TYPE = SYMMETRIC_KEY requires ALGORITHM"},
		},
		// -- FAIL: CREATE SECRET with TYPE keyword but no value
		{
			"CREATE SECRET TYPE keyword no value",
			"CREATE SECRET my_secret TYPE =",
			[]string{"CREATE SECRET requires TYPE"},
		},
		// -- FAIL: TYPE value is a near-miss (typo)
		{
			"CREATE SECRET TYPE near-miss OAUTH",
			"CREATE SECRET my_secret TYPE = OAUTH",
			[]string{"Unknown TYPE"},
		},
		{
			"CREATE SECRET TYPE near-miss PASS_WORD",
			"CREATE SECRET my_secret TYPE = PASS_WORD",
			[]string{"Unknown TYPE"},
		},
		{
			"CREATE SECRET TYPE near-miss GENERIC",
			"CREATE SECRET my_secret TYPE = GENERIC",
			[]string{"Unknown TYPE"},
		},
		// -- FAIL: Mixed mandatory-missing + cross-type violation (OAUTH2 missing API_AUTH + wrong USERNAME)
		{
			"CREATE SECRET OAUTH2 missing mandatory + cross-type",
			"CREATE SECRET my_secret TYPE = OAUTH2 USERNAME = 'u'",
			[]string{"TYPE = OAUTH2 requires API_AUTHENTICATION", "USERNAME is not valid for TYPE = OAUTH2"},
		},
		// -- FAIL: GENERIC_STRING missing SECRET_STRING but has COMMENT
		{
			"CREATE SECRET GENERIC_STRING missing SECRET_STRING with COMMENT",
			"CREATE SECRET my_secret TYPE = GENERIC_STRING COMMENT = 'note'",
			[]string{"TYPE = GENERIC_STRING requires SECRET_STRING"},
		},
		// -- FAIL: CLOUD_PROVIDER_TOKEN missing API_AUTHENTICATION but has ENABLED + COMMENT
		{
			"CREATE SECRET CLOUD_PROVIDER_TOKEN missing mandatory with optionals",
			"CREATE SECRET my_secret TYPE = CLOUD_PROVIDER_TOKEN ENABLED = TRUE COMMENT = 'note'",
			[]string{"TYPE = CLOUD_PROVIDER_TOKEN requires API_AUTHENTICATION"},
		},
		// -- FAIL: Multiple cross-type violations on same statement
		{
			"CREATE SECRET SYMMETRIC_KEY with multiple OAUTH2 props",
			"CREATE SECRET my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256' OAUTH_SCOPES = ('s1') OAUTH_REFRESH_TOKEN = 'tok'",
			[]string{"OAUTH_SCOPES is not valid for TYPE = SYMMETRIC_KEY", "OAUTH_REFRESH_TOKEN is not valid for TYPE = SYMMETRIC_KEY"},
		},
		// -- FAIL: ALTER SECRET with tab-separated tokens and unknown sub-command
		{
			"ALTER SECRET tab-separated unknown sub-command",
			"ALTER SECRET\tmy_secret\tDROP",
			[]string{"Unknown ALTER SECRET sub-command"},
		},
		// -- FAIL: CREATE SECRET with only whitespace after keyword
		{
			"CREATE SECRET trailing whitespace only",
			"CREATE SECRET   ",
			[]string{"Unexpected syntax in CREATE SECRET"},
		},
		// -- FAIL: PASSWORD missing both + cross-type violation (ALGORITHM)
		{
			"CREATE SECRET PASSWORD missing mandatory + cross-type ALGORITHM",
			"CREATE SECRET my_secret TYPE = PASSWORD ALGORITHM = 'AES-256'",
			[]string{"TYPE = PASSWORD requires USERNAME", "TYPE = PASSWORD requires PASSWORD", "ALGORITHM is not valid for TYPE = PASSWORD"},
		},
		// -- FAIL: CREATE SECRET with numeric-only TYPE value
		{
			"CREATE SECRET TYPE numeric value",
			"CREATE SECRET my_secret TYPE = 123",
			[]string{"Unknown TYPE"},
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

func TestValidateSnowflakePatterns_Secret_NameStartsWithDollar(t *testing.T) {
	// Names starting with '$' do not match _ident ([a-zA-Z_] start), so the
	// validator should produce "Unexpected syntax".
	cases := []string{
		"CREATE SECRET $secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET $123 TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for $-prefixed name")
			}
			if !strings.Contains(warns[0].Message, "Unexpected syntax") {
				t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MultipleWhitespaceInModifiers(t *testing.T) {
	// Extra whitespace (multiple spaces/tabs) inside OR REPLACE and IF NOT EXISTS
	// modifiers should be accepted — the regexes use \s+ which matches any amount.
	validCases := []string{
		"CREATE OR  REPLACE SECRET my_secret TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE OR\t\tREPLACE SECRET my_secret TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
		"CREATE SECRET IF  NOT  EXISTS my_secret TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		"CREATE SECRET IF\tNOT\tEXISTS my_secret TYPE = SYMMETRIC_KEY ALGORITHM = 'AES-256'",
		"ALTER SECRET IF  EXISTS my_secret SET COMMENT = 'x'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for multi-whitespace modifiers, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterNameIsUNSET(t *testing.T) {
	// When the secret is literally named "UNSET", the parser should extract
	// "UNSET" as the name and then check the remaining text for a valid action.
	t.Run("ALTER SECRET UNSET UNSET COMMENT valid", func(t *testing.T) {
		sql := "ALTER SECRET UNSET UNSET COMMENT"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for name='UNSET' with UNSET COMMENT action, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
	t.Run("ALTER SECRET UNSET SET COMMENT valid", func(t *testing.T) {
		sql := "ALTER SECRET UNSET SET COMMENT = 'x'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected 0 warnings for name='UNSET' with SET COMMENT action, got %d: %v", len(warns), warnMsgs(warns))
		}
	})
}

func TestValidateSnowflakePatterns_Secret_CreateNameIsTYPEWithoutClause(t *testing.T) {
	// When the secret is literally named "TYPE" but there is no TYPE = ... clause
	// after it, the validator should produce "requires TYPE".
	sql := "CREATE SECRET TYPE API_AUTHENTICATION = my_int"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected warning for name='TYPE' but no actual TYPE = ... clause")
	}
	// reSecretType is \bTYPE\b\s*=\s*([\w]+) — "TYPE API_AUTHENTICATION" has no "="
	// immediately after "TYPE", so it should fire "requires TYPE".
	if !strings.Contains(warns[0].Message, "CREATE SECRET requires TYPE") {
		t.Errorf("Expected 'CREATE SECRET requires TYPE' warning, got: %s", warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_BacktickQuotedIdentifier(t *testing.T) {
	// Backtick-quoted identifiers are not valid in Snowflake SQL. The _ident
	// pattern only recognizes double-quoted identifiers, so backtick names
	// should produce "Unexpected syntax".
	cases := []string{
		"CREATE SECRET `my-secret` TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		"CREATE SECRET `special name` TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for backtick-quoted identifier")
			}
			if !strings.Contains(warns[0].Message, "Unexpected syntax") {
				t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_NameSwallowedByIF(t *testing.T) {
	// When the name regex captures "IF" as the secret name AND the text also
	// contains "IF NOT EXISTS", checkNameSwallowedByIF fires and returns
	// "Unexpected syntax" — this exercises the otherwise-untested branch.
	cases := []struct {
		name string
		sql  string
	}{
		{
			"IF NOT EXISTS + literal IF name",
			"CREATE SECRET IF NOT EXISTS IF TYPE = GENERIC_STRING SECRET_STRING = 'val'",
		},
		{
			"IF NOT EXISTS + literal IF name with OR REPLACE absent",
			"CREATE SECRET IF NOT EXISTS IF TYPE = OAUTH2 API_AUTHENTICATION = my_int",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unexpected syntax") {
				t.Errorf("Expected 'Unexpected syntax' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_InvalidCaseExactWarningCount(t *testing.T) {
	// Spot-check that invalid cases produce EXACTLY the expected number of
	// warnings — no spurious extra diagnostics beyond those explicitly expected.
	cases := []struct {
		name      string
		sql       string
		wantCount int
		wantMsgs  []string
	}{
		{
			"OAUTH2 missing mandatory + cross-type USERNAME",
			"CREATE SECRET s TYPE = OAUTH2 USERNAME = 'u'",
			2,
			[]string{"TYPE = OAUTH2 requires API_AUTHENTICATION", "USERNAME is not valid for TYPE = OAUTH2"},
		},
		{
			"PASSWORD missing both mandatory",
			"CREATE SECRET s TYPE = PASSWORD",
			2,
			[]string{"TYPE = PASSWORD requires USERNAME", "TYPE = PASSWORD requires PASSWORD"},
		},
		{
			"PASSWORD missing both + two cross-type",
			"CREATE SECRET s TYPE = PASSWORD ALGORITHM = 'AES' API_AUTHENTICATION = x",
			4,
			[]string{"TYPE = PASSWORD requires USERNAME", "TYPE = PASSWORD requires PASSWORD",
				"ALGORITHM is not valid for TYPE = PASSWORD", "API_AUTHENTICATION is not valid for TYPE = PASSWORD"},
		},
		{
			"unknown TYPE produces exactly 1",
			"CREATE SECRET s TYPE = FOOBAR",
			1,
			[]string{"Unknown TYPE"},
		},
		{
			"missing name produces exactly 1",
			"CREATE SECRET",
			1,
			[]string{"Unexpected syntax"},
		},
		{
			"ALTER missing name produces exactly 1",
			"ALTER SECRET",
			1,
			[]string{"ALTER SECRET requires a secret name"},
		},
		{
			"ALTER unknown sub-command produces exactly 1",
			"ALTER SECRET s DROP",
			1,
			[]string{"Unknown ALTER SECRET sub-command"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != tc.wantCount {
				t.Fatalf("Expected exactly %d warning(s), got %d: %v", tc.wantCount, len(warns), warnMsgs(warns))
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
					t.Errorf("Expected warning containing %q, got: %v", wantMsg, warnMsgs(warns))
				}
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_UnexpectedPropertyIncludesName(t *testing.T) {
	// The "Unexpected property" warning should include the actual property name
	// so the user knows which property to remove.
	cases := []struct {
		sql      string
		wantProp string
	}{
		{"CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' WAREHOUSE = my_wh", "WAREHOUSE"},
		{"CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'val' SCHEDULE = 'USING CRON 0 * * * *'", "SCHEDULE"},
		{"CREATE SECRET s TYPE = OAUTH2 API_AUTHENTICATION = i CATALOG = cat", "CATALOG"},
	}
	for _, tc := range cases {
		t.Run(tc.wantProp, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "Unexpected property") && strings.Contains(w.Message, tc.wantProp) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected 'Unexpected property' warning mentioning %q, got: %v", tc.wantProp, warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_MandatoryMissingIncludesHint(t *testing.T) {
	// Mandatory-missing warnings include the full hint with the property syntax
	// (e.g., "USERNAME = '<username>'"), not just the keyword.
	cases := []struct {
		name     string
		sql      string
		wantHint string
	}{
		{"OAUTH2 API_AUTHENTICATION hint", "CREATE SECRET s TYPE = OAUTH2", "API_AUTHENTICATION = <security_integration_name>"},
		{"PASSWORD USERNAME hint", "CREATE SECRET s TYPE = PASSWORD PASSWORD = 'p'", "USERNAME = '<username>'"},
		{"PASSWORD PASSWORD hint", "CREATE SECRET s TYPE = PASSWORD USERNAME = 'u'", "PASSWORD = '<password>'"},
		{"GENERIC_STRING SECRET_STRING hint", "CREATE SECRET s TYPE = GENERIC_STRING", "SECRET_STRING = '<value>'"},
		{"CLOUD_PROVIDER_TOKEN API_AUTHENTICATION hint", "CREATE SECRET s TYPE = CLOUD_PROVIDER_TOKEN", "API_AUTHENTICATION = <security_integration_name>"},
		{"SYMMETRIC_KEY ALGORITHM hint", "CREATE SECRET s TYPE = SYMMETRIC_KEY", "ALGORITHM = '<algorithm>'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, tc.wantHint) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning containing hint %q, got: %v", tc.wantHint, warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterUnknownSubCmdFullMessage(t *testing.T) {
	// The ALTER SECRET unknown sub-command warning should list all valid
	// SET targets and UNSET COMMENT in its message for discoverability.
	sql := "ALTER SECRET my_secret DROP"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
	}
	msg := warns[0].Message
	for _, want := range []string{
		"SECRET_STRING", "USERNAME", "PASSWORD",
		"OAUTH_REFRESH_TOKEN", "OAUTH_REFRESH_TOKEN_EXPIRY_TIME",
		"OAUTH_SCOPES", "API_AUTHENTICATION", "COMMENT",
		"UNSET COMMENT",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("ALTER unknown sub-command message should mention %q, got: %s", want, msg)
		}
	}
}

func TestValidateSnowflakePatterns_Secret_ConflictWarningCountExact(t *testing.T) {
	// When OR REPLACE + IF NOT EXISTS conflict is detected, exactly 1 warning
	// should be produced and no further validation runs (early return).
	sql := "CREATE OR REPLACE SECRET IF NOT EXISTS my_secret TYPE = PASSWORD"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected exactly 1 warning (conflict only), got %d: %v", len(warns), warnMsgs(warns))
	}
	if !strings.Contains(warns[0].Message, "Conflict between OR REPLACE and IF NOT EXISTS") {
		t.Errorf("Expected conflict warning, got: %s", warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_AlterSetMultipleValidTargets(t *testing.T) {
	// ALTER SECRET SET with multiple valid property targets in a single SET —
	// only one match of reAlterSecretAction is needed for the statement to pass.
	validCases := []string{
		"ALTER SECRET my_secret SET USERNAME = 'u' PASSWORD = 'p' COMMENT = 'note'",
		"ALTER SECRET my_secret SET OAUTH_REFRESH_TOKEN = 'tok' OAUTH_SCOPES = ('s1') API_AUTHENTICATION = my_int",
		"ALTER SECRET my_secret SET SECRET_STRING = 'new' COMMENT = 'updated'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for ALTER SET with multiple valid targets, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_UnexpectedPropertyCaseInsensitive(t *testing.T) {
	// Unexpected properties should be detected regardless of case.
	cases := []struct {
		name string
		sql  string
	}{
		{"lowercase unexpected", "CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' warehouse = my_wh"},
		{"mixed case unexpected", "CREATE SECRET s TYPE = GENERIC_STRING SECRET_STRING = 'val' Schedule = 'USING CRON 0 * * * *'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, "Unexpected property") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected 'Unexpected property' warning for %s, got: %v", tc.name, warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_UnknownTypeWarningContent(t *testing.T) {
	// The "Unknown TYPE" warning should include the actual type value and list
	// all valid types for discoverability.
	sql := "CREATE SECRET my_secret TYPE = BEARER_TOKEN"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
	}
	msg := warns[0].Message
	if !strings.Contains(msg, "BEARER_TOKEN") {
		t.Errorf("Warning should include the invalid type value 'BEARER_TOKEN', got: %s", msg)
	}
	for _, validType := range []string{"OAUTH2", "PASSWORD", "GENERIC_STRING", "CLOUD_PROVIDER_TOKEN", "SYMMETRIC_KEY"} {
		if !strings.Contains(msg, validType) {
			t.Errorf("Warning should list valid type %s, got: %s", validType, msg)
		}
	}
}

func TestValidateSnowflakePatterns_Secret_AlterNoNameProducesExactMessage(t *testing.T) {
	// Bare "ALTER SECRET" (no trailing tokens) should produce the exact
	// "requires a secret name" message with no extra warnings.
	sql := "ALTER SECRET"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
	}
	expected := "ALTER SECRET requires a secret name."
	if warns[0].Message != expected {
		t.Errorf("Expected exact message %q, got: %q", expected, warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_CreateMissingNameProducesExactMessage(t *testing.T) {
	// Bare "CREATE SECRET" (no trailing tokens) should produce the exact
	// "Unexpected syntax" message.
	sql := "CREATE SECRET"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected exactly 1 warning, got %d: %v", len(warns), warnMsgs(warns))
	}
	expected := "Unexpected syntax in CREATE SECRET statement."
	if warns[0].Message != expected {
		t.Errorf("Expected exact message %q, got: %q", expected, warns[0].Message)
	}
}

func TestValidateSnowflakePatterns_Secret_AlterInvalidNameFormat(t *testing.T) {
	// Parity with TestValidateSnowflakePatterns_Secret_InvalidNameFormat (CREATE side).
	// Names starting with a digit do not match _ident, so the validator should
	// produce "requires a secret name".
	cases := []string{
		"ALTER SECRET 123abc SET COMMENT = 'x'",
		"ALTER SECRET 9secret SET SECRET_STRING = 'val'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for digit-prefixed name")
			}
			if !strings.Contains(warns[0].Message, "ALTER SECRET requires a secret name") {
				t.Errorf("Expected 'ALTER SECRET requires a secret name' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterNameStartsWithDollar(t *testing.T) {
	// Parity with TestValidateSnowflakePatterns_Secret_NameStartsWithDollar (CREATE side).
	// Names starting with '$' do not match _ident ([a-zA-Z_] start), so the
	// validator should produce "requires a secret name".
	cases := []string{
		"ALTER SECRET $secret SET COMMENT = 'x'",
		"ALTER SECRET $123 SET SECRET_STRING = 'val'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for $-prefixed name")
			}
			if !strings.Contains(warns[0].Message, "ALTER SECRET requires a secret name") {
				t.Errorf("Expected 'ALTER SECRET requires a secret name' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterBacktickQuotedIdentifier(t *testing.T) {
	// Parity with TestValidateSnowflakePatterns_Secret_BacktickQuotedIdentifier (CREATE side).
	// Backtick-quoted identifiers are not valid in Snowflake SQL; the _ident
	// pattern only recognizes double-quoted identifiers.
	cases := []string{
		"ALTER SECRET `my-secret` SET COMMENT = 'x'",
		"ALTER SECRET `special name` SET SECRET_STRING = 'val'",
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatal("Expected at least 1 warning for backtick-quoted identifier")
			}
			if !strings.Contains(warns[0].Message, "ALTER SECRET requires a secret name") {
				t.Errorf("Expected 'ALTER SECRET requires a secret name' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterSpecialNameFormats(t *testing.T) {
	// Parity with CREATE valid cases for underscore-prefixed names (line 1557),
	// dollar-containing names (line 1559), and numeric-suffixed names (line 1561).
	validCases := []string{
		"ALTER SECRET _private_secret SET COMMENT = 'x'",
		"ALTER SECRET my$secret SET SECRET_STRING = 'val'",
		"ALTER SECRET secret_123 SET USERNAME = 'u' PASSWORD = 'p'",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for special name format, got %d: %v", len(warns), warnMsgs(warns))
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_AlterQuotedNameNoIfExistsNoAction(t *testing.T) {
	// ALTER SECRET with a quoted identifier (no IF EXISTS) but no valid
	// sub-command should produce "Unknown ALTER SECRET sub-command".
	// Complements TestValidateSnowflakePatterns_Secret_AlterIfExistsQuotedNameNoAction
	// (which uses IF EXISTS) and the bare-name test at line 1781.
	cases := []string{
		`ALTER SECRET "my-secret"`,
		`ALTER SECRET "db"."schema"."secret"`,
		`ALTER SECRET "special name"`,
	}
	for _, sql := range cases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warns := getWarnings(markers)
			if len(warns) != 1 {
				t.Fatalf("Expected 1 warning, got %d: %v", len(warns), warnMsgs(warns))
			}
			if !strings.Contains(warns[0].Message, "Unknown ALTER SECRET sub-command") {
				t.Errorf("Expected 'Unknown ALTER SECRET sub-command' warning, got: %s", warns[0].Message)
			}
		})
	}
}

func TestValidateSnowflakePatterns_Secret_UnexpectedPropertyInBlockComment(t *testing.T) {
	// Token-based validateProperties correctly ignores KEY = patterns inside
	// block comments. Previously this was a known false positive with regex.
	sql := "CREATE SECRET s TYPE = PASSWORD USERNAME = 'u' PASSWORD = 'p' /* WAREHOUSE = my_wh */"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warns := getWarnings(markers)
	for _, w := range warns {
		if strings.Contains(w.Message, "Unexpected property") && strings.Contains(w.Message, "WAREHOUSE") {
			t.Errorf("Token-based validateProperties should not flag WAREHOUSE inside a block comment, got: %s", w.Message)
		}
	}
}
