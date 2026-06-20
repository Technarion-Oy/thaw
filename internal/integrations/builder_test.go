// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package integrations

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

// ── Assertion helpers ─────────────────────────────────────────────────────────

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertNotContains(t *testing.T, sql, substr string) {
	t.Helper()
	if strings.Contains(sql, substr) {
		t.Errorf("expected SQL NOT to contain %q\nSQL:\n%s", substr, sql)
	}
}

// requireSQL fails the test if err != nil, otherwise returns sql.
// Call as: sql := requireSQL(t)(BuildXxxSQL(p))
// The inner func signature matches the multi-return of builder functions so Go
// allows passing the builder call directly as the sole argument.
func requireSQL(t *testing.T) func(string, error) string {
	t.Helper()
	return func(sql string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return sql
	}
}

// ── string-literal / identifier quoting helpers ───────────────────────────────

func TestSq(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "'hello'"},
		{"it's", "'it''s'"},
		{"a''b", "'a''''b'"},
		{"", "''"},
		{"'; DROP TABLE foo; --", "'''; DROP TABLE foo; --'"},
	}
	for _, tc := range cases {
		got := snowflake.QuoteStringLit(tc.in)
		if got != tc.want {
			t.Errorf("QuoteStringLit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestQident(t *testing.T) {
	cases := []struct{ in, want string }{
		{"MY_TABLE", `"MY_TABLE"`},
		{`bad"name`, `"bad""name"`},
		{"", `""`},
	}
	for _, tc := range cases {
		got := snowflake.QuoteIdent(tc.in)
		if got != tc.want {
			t.Errorf("QuoteIdent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── mustBeOneOf ───────────────────────────────────────────────────────────────

func TestMustBeOneOf(t *testing.T) {
	v, err := mustBeOneOf("f", "s3", "S3", "GCS", "AZURE")
	if err != nil || v != "S3" {
		t.Fatalf("mustBeOneOf case-insensitive: got %q, %v", v, err)
	}
	_, err = mustBeOneOf("f", "INVALID", "S3", "GCS")
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
}

// ── secretsTuple ─────────────────────────────────────────────────────────────

func TestSecretsTuple(t *testing.T) {
	cases := []struct {
		in   []string
		want string
		ok   bool
	}{
		{nil, "", false},
		{[]string{}, "", false},
		{[]string{"all"}, "ALL", true},
		{[]string{"NONE"}, "NONE", true},
		{[]string{"MY_SECRET"}, "(MY_SECRET)", true},
		{[]string{"SEC_A", "SEC_B"}, "(SEC_A, SEC_B)", true},
	}
	for _, tc := range cases {
		got, ok, err := secretsTuple(tc.in)
		if err != nil {
			t.Errorf("secretsTuple(%v) unexpected error: %v", tc.in, err)
			continue
		}
		if ok != tc.ok || got != tc.want {
			t.Errorf("secretsTuple(%v) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}

	// Injection attempt via identifier
	_, _, err := secretsTuple([]string{"'; DROP TABLE secrets; --"})
	if err == nil {
		t.Error("expected error for injection attempt in secretsTuple")
	}
}

// ── validateIdentRef ──────────────────────────────────────────────────────────

func TestValidateIdentRef(t *testing.T) {
	valid := []string{"MY_DB", "MY_DB.MY_SCHEMA.MY_TABLE", `"My DB"."My Table"`, "A$1"}
	for _, v := range valid {
		if _, err := validateIdentRef(v); err != nil {
			t.Errorf("validateIdentRef(%q) unexpected error: %v", v, err)
		}
	}
	invalid := []string{"'; DROP TABLE foo; --", "123bad", "a b c"}
	for _, v := range invalid {
		if _, err := validateIdentRef(v); err == nil {
			t.Errorf("validateIdentRef(%q) expected error", v)
		}
	}
}

// ── BuildStorageIntegrationSQL ────────────────────────────────────────────────

func TestBuildStorageIntegrationSQL_S3(t *testing.T) {
	must := requireSQL(t)
	p := StorageIntegrationParams{
		Name:             "MY_S3_INT",
		Enabled:          true,
		Provider:         "S3",
		AwsRoleArn:       "arn:aws:iam::123:role/my-role",
		AllowedLocations: "s3://bucket/path/",
		UsePrivatelink:   true,
	}
	sql := must(BuildStorageIntegrationSQL(p))
	assertContains(t, sql, `CREATE STORAGE INTEGRATION "MY_S3_INT"`)
	assertContains(t, sql, `STORAGE_PROVIDER = 'S3'`)
	assertContains(t, sql, `STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/my-role'`)
	assertContains(t, sql, `STORAGE_ALLOWED_LOCATIONS = ('s3://bucket/path/')`)
	assertContains(t, sql, `USE_PRIVATELINK_ENDPOINT = TRUE`)
	assertContains(t, sql, `ENABLED = TRUE`)
}

func TestBuildStorageIntegrationSQL_InvalidProvider(t *testing.T) {
	p := StorageIntegrationParams{Name: "X", Provider: "INVALID_PROVIDER"}
	_, err := BuildStorageIntegrationSQL(p)
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
}

func TestBuildStorageIntegrationSQL_SQLInjectionComment(t *testing.T) {
	must := requireSQL(t)
	p := StorageIntegrationParams{
		Name:     "MY_INT",
		Provider: "S3",
		Comment:  "'; DROP TABLE foo; --",
	}
	sql := must(BuildStorageIntegrationSQL(p))
	// The single-quote doubling neutralizes the injection: the payload is safely
	// embedded as a string literal and cannot escape the quotes.
	assertContains(t, sql, `COMMENT = '''; DROP TABLE foo; --'`)
}

// ── BuildApiIntegrationSQL — non-git providers ────────────────────────────────

func TestBuildApiIntegrationSQL_AWS(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:            "MY_API",
		Enabled:         true,
		Provider:        "aws_api_gateway",
		AllowedPrefixes: "https://api.example.com/",
		AwsRoleArn:      "arn:aws:iam::123:role/api-role",
		ApiKey:          "my-key",
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `CREATE API INTEGRATION "MY_API"`)
	assertContains(t, sql, `API_PROVIDER = aws_api_gateway`)
	assertContains(t, sql, `API_ALLOWED_PREFIXES = ('https://api.example.com/')`)
	assertContains(t, sql, `API_AWS_ROLE_ARN = 'arn:aws:iam::123:role/api-role'`)
	assertContains(t, sql, `API_KEY = 'my-key'`)
}

// ── BuildApiIntegrationSQL — git_https_api modes ──────────────────────────────

func TestBuildApiIntegrationSQL_GitToken(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:            "MY_GIT",
		Enabled:         true,
		Provider:        "git_https_api",
		GitAuthMode:     "TOKEN",
		AllowedPrefixes: "https://github.com/my-org/",
		AllowedSecrets:  []string{"MY_TOKEN_SECRET"},
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `CREATE API INTEGRATION "MY_GIT"`)
	assertContains(t, sql, `API_PROVIDER = git_https_api`)
	assertContains(t, sql, `API_ALLOWED_PREFIXES = ('https://github.com/my-org/')`)
	assertContains(t, sql, `ALLOWED_AUTHENTICATION_SECRETS = (MY_TOKEN_SECRET)`)
	assertNotContains(t, sql, `API_USER_AUTHENTICATION`)
}

func TestBuildApiIntegrationSQL_GitGithubApp(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:          "MY_GIT_APP",
		Enabled:       true,
		Provider:      "git_https_api",
		GitAuthMode:   "GITHUB_APP",
		GithubAppPath: "my-org/my-repo",
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `API_ALLOWED_PREFIXES = ('https://github.com/my-org/my-repo')`)
	assertContains(t, sql, `API_USER_AUTHENTICATION = (`)
	assertContains(t, sql, `TYPE = SNOWFLAKE_GITHUB_APP`)
}

func TestBuildApiIntegrationSQL_GitGithubApp_EmptyPath(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:        "GIT_APP",
		Enabled:     true,
		Provider:    "git_https_api",
		GitAuthMode: "GITHUB_APP",
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `API_ALLOWED_PREFIXES = ('https://github.com/')`)
}

func TestBuildApiIntegrationSQL_GitGithubApp_LeadingSlashStripped(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:          "GIT_APP",
		Enabled:       true,
		Provider:      "git_https_api",
		GitAuthMode:   "GITHUB_APP",
		GithubAppPath: "///my-org/my-repo",
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `API_ALLOWED_PREFIXES = ('https://github.com/my-org/my-repo')`)
}

func TestBuildApiIntegrationSQL_GitOAuth2(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:               "GIT_OAUTH",
		Enabled:            true,
		Provider:           "git_https_api",
		GitAuthMode:        "OAUTH2",
		AllowedPrefixes:    "https://gitlab.com/my-group/",
		OauthClientId:      "CLIENT_ID",
		OauthClientSecret:  "CLIENT_SECRET",
		OauthTokenEndpoint: "https://gitlab.com/oauth/token",
		OauthScopes:        "read_api, write_repository",
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `TYPE = OAUTH2`)
	assertContains(t, sql, `OAUTH_CLIENT_ID = 'CLIENT_ID'`)
	assertContains(t, sql, `OAUTH_CLIENT_SECRET = 'CLIENT_SECRET'`)
	assertContains(t, sql, `OAUTH_TOKEN_ENDPOINT = 'https://gitlab.com/oauth/token'`)
	assertContains(t, sql, `OAUTH_ALLOWED_SCOPES = ('read_api', 'write_repository')`)
}

func TestBuildApiIntegrationSQL_GitPrivatelink(t *testing.T) {
	must := requireSQL(t)
	p := ApiIntegrationParams{
		Name:            "GIT_PL",
		Enabled:         true,
		Provider:        "git_https_api",
		GitAuthMode:     "PRIVATELINK",
		AllowedPrefixes: "https://private.example.com/",
		UsePrivateLink:  true,
		TlsCertificates: []string{"TLS_CERT_1", "TLS_CERT_2"},
		AllowedSecrets:  []string{"ALL"},
	}
	sql := must(BuildApiIntegrationSQL(p))
	assertContains(t, sql, `USE_PRIVATELINK_ENDPOINT = TRUE`)
	assertContains(t, sql, `TLS_TRUSTED_CERTIFICATES = (TLS_CERT_1, TLS_CERT_2)`)
	assertContains(t, sql, `ALLOWED_AUTHENTICATION_SECRETS = ALL`)
}

func TestBuildApiIntegrationSQL_OrReplaceIfNotExists(t *testing.T) {
	must := requireSQL(t)

	p1 := ApiIntegrationParams{Name: "X", Provider: "git_https_api", GitAuthMode: "TOKEN", OrReplace: true}
	sql1 := must(BuildApiIntegrationSQL(p1))
	assertContains(t, sql1, "CREATE OR REPLACE API INTEGRATION")
	assertNotContains(t, sql1, "IF NOT EXISTS")

	p2 := ApiIntegrationParams{Name: "X", Provider: "git_https_api", GitAuthMode: "TOKEN", IfNotExists: true}
	sql2 := must(BuildApiIntegrationSQL(p2))
	assertContains(t, sql2, "CREATE API INTEGRATION IF NOT EXISTS")
	assertNotContains(t, sql2, "OR REPLACE")

	// OrReplace wins — IfNotExists is ignored when both are set
	p3 := ApiIntegrationParams{Name: "X", Provider: "git_https_api", GitAuthMode: "TOKEN", OrReplace: true, IfNotExists: true}
	sql3 := must(BuildApiIntegrationSQL(p3))
	assertContains(t, sql3, "CREATE OR REPLACE API INTEGRATION")
	assertNotContains(t, sql3, "IF NOT EXISTS")
}

func TestBuildApiIntegrationSQL_InvalidAuthMode(t *testing.T) {
	p := ApiIntegrationParams{Name: "X", Provider: "git_https_api", GitAuthMode: "INVALID"}
	_, err := BuildApiIntegrationSQL(p)
	if err == nil {
		t.Fatal("expected error for invalid gitAuthMode")
	}
}

func TestBuildApiIntegrationSQL_SQLInjectionInSecret(t *testing.T) {
	p := ApiIntegrationParams{
		Name:           "GIT",
		Provider:       "git_https_api",
		GitAuthMode:    "TOKEN",
		AllowedSecrets: []string{"'; DROP TABLE secrets; --"},
	}
	_, err := BuildApiIntegrationSQL(p)
	if err == nil {
		t.Error("expected error for injection attempt in secret name")
	}
}

// ── BuildExternalAccessIntegrationSQL ────────────────────────────────────────

func TestBuildExternalAccessIntegrationSQL(t *testing.T) {
	must := requireSQL(t)
	p := ExternalAccessIntegrationParams{
		Name:                "MY_EAI",
		Enabled:             true,
		AllowedNetworkRules: "MY_RULE_1, MY_RULE_2",
		AllowedAuthSecrets:  "ALL",
	}
	sql := must(BuildExternalAccessIntegrationSQL(p))
	assertContains(t, sql, `CREATE EXTERNAL ACCESS INTEGRATION "MY_EAI"`)
	assertContains(t, sql, `ALLOWED_NETWORK_RULES = (MY_RULE_1, MY_RULE_2)`)
	assertContains(t, sql, `ALLOWED_AUTHENTICATION_SECRETS = ALL`)
}

func TestBuildExternalAccessIntegrationSQL_InjectionInNetworkRule(t *testing.T) {
	p := ExternalAccessIntegrationParams{
		Name:                "EAI",
		AllowedNetworkRules: "'; DROP TABLE rules; --",
	}
	_, err := BuildExternalAccessIntegrationSQL(p)
	if err == nil {
		t.Error("expected error for injection attempt in network rule name")
	}
}

// ── BuildNotificationIntegrationSQL ──────────────────────────────────────────

func TestBuildNotificationIntegrationSQL_Email(t *testing.T) {
	must := requireSQL(t)
	p := NotificationIntegrationParams{
		Name:              "MY_EMAIL_NOTIF",
		Enabled:           true,
		Subtype:           "EMAIL",
		AllowedRecipients: "alice@example.com, bob@example.com",
		DefaultSubject:    "Alert: it's important",
	}
	sql := must(BuildNotificationIntegrationSQL(p))
	assertContains(t, sql, `CREATE NOTIFICATION INTEGRATION "MY_EMAIL_NOTIF"`)
	assertContains(t, sql, `TYPE = EMAIL`)
	assertContains(t, sql, `ALLOWED_RECIPIENTS = ('alice@example.com', 'bob@example.com')`)
	assertContains(t, sql, `DEFAULT_SUBJECT = 'Alert: it''s important'`)
}

func TestBuildNotificationIntegrationSQL_InvalidSubtype(t *testing.T) {
	p := NotificationIntegrationParams{Name: "N", Subtype: "BOGUS"}
	_, err := BuildNotificationIntegrationSQL(p)
	if err == nil {
		t.Fatal("expected error for invalid subtype")
	}
}

// ── BuildSecurityIntegrationSQL ───────────────────────────────────────────────

func TestBuildSecurityIntegrationSQL_ApiAuthentication(t *testing.T) {
	must := requireSQL(t)
	p := SecurityIntegrationParams{
		Name:               "MY_SEC",
		Enabled:            true,
		SecType:            "API_AUTHENTICATION",
		AuthType:           "OAUTH2",
		OauthGrant:         "CLIENT_CREDENTIALS",
		OauthClientId:      "cid",
		OauthClientSecret:  "csec",
		OauthTokenEndpoint: "https://auth.example.com/token",
	}
	sql := must(BuildSecurityIntegrationSQL(p))
	assertContains(t, sql, `TYPE = API_AUTHENTICATION`)
	assertContains(t, sql, `AUTH_TYPE = OAUTH2`)
	assertContains(t, sql, `OAUTH_GRANT = CLIENT_CREDENTIALS`)
	assertContains(t, sql, `OAUTH_CLIENT_ID = 'cid'`)
	assertContains(t, sql, `OAUTH_TOKEN_ENDPOINT = 'https://auth.example.com/token'`)
}

func TestBuildSecurityIntegrationSQL_Scim(t *testing.T) {
	must := requireSQL(t)
	p := SecurityIntegrationParams{
		Name:          "MY_SCIM",
		Enabled:       true,
		SecType:       "SCIM",
		ScimClient:    "OKTA",
		RunAsRole:     "SCIM_PROVISIONER",
		NetworkPolicy: "MY_NETWORK_POLICY",
		SyncPassword:  true,
	}
	sql := must(BuildSecurityIntegrationSQL(p))
	assertContains(t, sql, `TYPE = SCIM`)
	assertContains(t, sql, `SCIM_CLIENT = 'OKTA'`)
	assertContains(t, sql, `RUN_AS_SERVICE_USER = "SCIM_PROVISIONER"`)
	assertContains(t, sql, `NETWORK_POLICY = "MY_NETWORK_POLICY"`)
	assertContains(t, sql, `SYNC_PASSWORD = TRUE`)
}

func TestBuildSecurityIntegrationSQL_InvalidSecType(t *testing.T) {
	p := SecurityIntegrationParams{Name: "S", SecType: "BOGUS"}
	_, err := BuildSecurityIntegrationSQL(p)
	if err == nil {
		t.Fatal("expected error for invalid secType")
	}
}

func TestBuildSecurityIntegrationSQL_NetworkPolicyInjection(t *testing.T) {
	must := requireSQL(t)
	p := SecurityIntegrationParams{
		Name:          "S",
		SecType:       "SCIM",
		ScimClient:    "GENERIC",
		RunAsRole:     "R",
		NetworkPolicy: `"; DROP TABLE policies; --`,
	}
	sql := must(BuildSecurityIntegrationSQL(p))
	// The double-quote doubling neutralizes the injection: the payload is safely
	// embedded as a double-quoted identifier and cannot escape the quotes.
	assertContains(t, sql, `"""`)
}

// ── identToken ────────────────────────────────────────────────────────────────

func TestIdentToken_CaseSensitive(t *testing.T) {
	got := identToken("myTable", true)
	if got != `"myTable"` {
		t.Errorf("got %q", got)
	}
}

func TestIdentToken_CaseInsensitive(t *testing.T) {
	got := identToken("myTable", false)
	if got != `"MYTABLE"` {
		t.Errorf("got %q", got)
	}
}
