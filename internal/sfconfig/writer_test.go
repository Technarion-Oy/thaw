// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sfconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"myprofile", false},
		{"my-profile", false},
		{"my_profile_2", false},
		{"UPPER", false},
		{"a", false},
		{"", true},
		{"has space", true},
		{"has.dot", true},
		{"has/slash", true},
		{"special@char", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveProfile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := SaveProfile(path, Connection{
		Name:    "dev",
		Account: "myorg-dev",
		User:    "admin",
		Role:    "SYSADMIN",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, "[connections.dev]") {
		t.Error("missing section header")
	}
	if !strings.Contains(content, `account = "myorg-dev"`) {
		t.Error("missing account key")
	}
	if !strings.Contains(content, `user = "admin"`) {
		t.Error("missing user key")
	}
	if !strings.Contains(content, `role = "SYSADMIN"`) {
		t.Error("missing role key")
	}
	// Empty fields should not be present.
	if strings.Contains(content, "password") {
		t.Error("empty password field should not be written")
	}
}

func TestSaveProfile_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = "dev"

[connections.dev]
account = "old-account"
user = "old-user"

[connections.prod]
account = "prod-account"
user = "prod-user"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SaveProfile(path, Connection{
		Name:    "dev",
		Account: "new-account",
		User:    "new-user",
		Role:    "SYSADMIN",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `account = "new-account"`) {
		t.Error("account not updated")
	}
	if !strings.Contains(content, `role = "SYSADMIN"`) {
		t.Error("new role not added")
	}
	// prod section must survive.
	if !strings.Contains(content, "[connections.prod]") {
		t.Error("prod section was lost")
	}
	if !strings.Contains(content, `account = "prod-account"`) {
		t.Error("prod account was lost")
	}
	// default_connection_name must survive.
	if !strings.Contains(content, `default_connection_name = "dev"`) {
		t.Error("default_connection_name was lost")
	}
}

func TestSaveProfile_CommentPreservation(t *testing.T) {
	dir := t.TempDir()
	initial := `# This is a top-level comment
default_connection_name = "dev"

# Dev environment
[connections.dev]
account = "old"
# inline note
user = "old"

# Prod
[connections.prod]
account = "prod"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SaveProfile(path, Connection{
		Name:    "dev",
		Account: "updated",
		User:    "updated",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, "# This is a top-level comment") {
		t.Error("top-level comment was lost")
	}
	if !strings.Contains(content, "# inline note") {
		t.Error("intra-section comment was lost")
	}
	if !strings.Contains(content, "# Prod") {
		t.Error("prod comment was lost")
	}
}

func TestSaveProfile_PreservesUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "myorg"
user = "admin"
custom_timeout = "30"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SaveProfile(path, Connection{
		Name:    "dev",
		Account: "myorg-updated",
		User:    "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `custom_timeout = "30"`) {
		t.Error("unknown key custom_timeout was lost")
	}
	if !strings.Contains(content, `account = "myorg-updated"`) {
		t.Error("account not updated")
	}
}

func TestDeleteProfile(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := DeleteProfile(path, "dev")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if strings.Contains(content, "[connections.dev]") {
		t.Error("dev section still present")
	}
	if !strings.Contains(content, "[connections.prod]") {
		t.Error("prod section was lost")
	}
}

func TestDeleteProfile_ClearsDefault(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = "dev"

[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := DeleteProfile(path, "dev")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = ""`) {
		t.Error("default_connection_name was not cleared")
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := DeleteProfile(path, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestCloneProfile(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
user = "admin"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := CloneProfile(path, "dev", "dev-copy")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, "[connections.dev]") {
		t.Error("original section was lost")
	}
	if !strings.Contains(content, "[connections.dev-copy]") {
		t.Error("cloned section not created")
	}
	// Count occurrences of account line to verify content was cloned.
	if strings.Count(content, `account = "dev-account"`) != 2 {
		t.Error("cloned section should have same account value")
	}
}

func TestCloneProfile_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := CloneProfile(path, "dev", "dev")
	if err == nil {
		t.Error("expected error when cloning to same name")
	}
}

func TestCloneProfile_SourceNotFound(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := CloneProfile(path, "nonexistent", "new")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestSetDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = "dev"

[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SetDefaultProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = "prod"`) {
		t.Error("default not updated to prod")
	}
	if strings.Contains(content, `default_connection_name = "dev"`) {
		t.Error("old default still present")
	}
}

func TestSetDefaultProfile_InsertNew(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SetDefaultProfile(path, "dev")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = "dev"`) {
		t.Error("default not inserted")
	}
}

func TestSetDefaultProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SetDefaultProfile(path, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestClearDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = "dev"

[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := ClearDefaultProfile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = ""`) {
		t.Error("default_connection_name was not cleared")
	}
	// Profile should still exist.
	if !strings.Contains(content, "[connections.dev]") {
		t.Error("profile section was lost")
	}
}

func TestClearDefaultProfile_NoDefault(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := ClearDefaultProfile(path)
	if err != nil {
		t.Errorf("clearing when no default exists should be a no-op, got: %v", err)
	}
}

func TestRenameProfile(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = "dev"

[connections.dev]
account = "dev-account"
user = "admin"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := RenameProfile(path, "dev", "staging")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if strings.Contains(content, "[connections.dev]") {
		t.Error("old section header still present")
	}
	if !strings.Contains(content, "[connections.staging]") {
		t.Error("new section header not found")
	}
	if !strings.Contains(content, `account = "dev-account"`) {
		t.Error("section body was lost")
	}
	// default_connection_name should be updated.
	if !strings.Contains(content, `default_connection_name = "staging"`) {
		t.Error("default_connection_name was not updated to new name")
	}
	// prod should be untouched.
	if !strings.Contains(content, "[connections.prod]") {
		t.Error("prod section was lost")
	}
}

func TestRenameProfile_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := RenameProfile(path, "dev", "prod")
	if err == nil {
		t.Error("expected error when renaming to existing name")
	}
}

func TestRenameProfile_SameName(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := RenameProfile(path, "dev", "dev")
	if err != nil {
		t.Errorf("renaming to same name should be a no-op, got: %v", err)
	}
}

func TestRenameProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "dev-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := RenameProfile(path, "nonexistent", "new")
	if err == nil {
		t.Error("expected error for nonexistent source profile")
	}
}

func TestSaveProfile_PreservesCommentsAndUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	initial := `[connections.dev]
account = "myorg"
# This is the service user
user = "admin"
custom_timeout = "30"

# Keep this note
custom_flag = "true"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SaveProfile(path, Connection{
		Name:    "dev",
		Account: "myorg-updated",
		User:    "admin",
		Role:    "SYSADMIN",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `account = "myorg-updated"`) {
		t.Error("account not updated")
	}
	if !strings.Contains(content, `role = "SYSADMIN"`) {
		t.Error("new role not added")
	}
	if !strings.Contains(content, `custom_timeout = "30"`) {
		t.Error("unknown key custom_timeout was lost")
	}
	if !strings.Contains(content, `custom_flag = "true"`) {
		t.Error("unknown key custom_flag was lost")
	}
	if !strings.Contains(content, "# This is the service user") {
		t.Error("intra-section comment was lost")
	}
	if !strings.Contains(content, "# Keep this note") {
		t.Error("second intra-section comment was lost")
	}
}

func TestDeleteProfile_ClearsDefault_SingleQuoted(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = 'dev'

[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := DeleteProfile(path, "dev")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = ""`) {
		t.Error("default_connection_name was not cleared for single-quoted value")
	}
}

func TestDeleteProfile_ClearsDefault_BareValue(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = dev

[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := DeleteProfile(path, "dev")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = ""`) {
		t.Error("default_connection_name was not cleared for bare value")
	}
}

func TestSetDefaultProfile_SingleQuoted(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = 'dev'

[connections.dev]
account = "dev-account"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := SetDefaultProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = "prod"`) {
		t.Error("default not updated from single-quoted value")
	}
}

func TestRenameProfile_BareDefault(t *testing.T) {
	dir := t.TempDir()
	initial := `default_connection_name = dev

[connections.dev]
account = "dev-account"
user = "admin"

[connections.prod]
account = "prod-account"
`
	path := writeTestFile(t, dir, "config.toml", initial)

	err := RenameProfile(path, "dev", "staging")
	if err != nil {
		t.Fatal(err)
	}

	content := readTestFile(t, path)
	if !strings.Contains(content, `default_connection_name = "staging"`) {
		t.Error("default_connection_name was not updated from bare value")
	}
	if !strings.Contains(content, "[connections.staging]") {
		t.Error("section header not renamed")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	profile := Connection{
		Name:          "roundtrip",
		Account:       "myorg-test",
		User:          "testuser",
		Password:      "secret",
		Role:          "SYSADMIN",
		Warehouse:     "COMPUTE_WH",
		Database:      "MYDB",
		Schema:        "PUBLIC",
		Authenticator: "snowflake",
	}

	// Save.
	if err := SaveProfile(path, profile); err != nil {
		t.Fatal(err)
	}

	// Load.
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	c := cfg.Connections[0]
	if c.Name != "roundtrip" || c.Account != "myorg-test" || c.User != "testuser" ||
		c.Password != "secret" || c.Role != "SYSADMIN" || c.Warehouse != "COMPUTE_WH" ||
		c.Database != "MYDB" || c.Schema != "PUBLIC" || c.Authenticator != "snowflake" {
		t.Errorf("round-trip mismatch: got %+v", c)
	}

	// Save again — should produce identical Load result.
	if err := SaveProfile(path, profile); err != nil {
		t.Fatal(err)
	}
	cfg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Connections) != 1 {
		t.Fatalf("expected 1 connection after re-save, got %d", len(cfg2.Connections))
	}
	c2 := cfg2.Connections[0]
	if c2.Account != c.Account || c2.User != c.User || c2.Password != c.Password {
		t.Error("re-save changed values")
	}
}

// TestRoundTrip_AuthFields verifies the token/OAuth2/WIF authenticator fields
// survive a SaveProfile → Load cycle with their snake_case TOML keys.
func TestRoundTrip_AuthFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	profile := Connection{
		Name:                              "oauthcc",
		Account:                           "myorg-test",
		Authenticator:                     "oauth_client_credentials",
		Token:                             "tok-123",
		TokenFilePath:                     "/path/to/token",
		OAuthClientID:                     "client-abc",
		OAuthClientSecret:                 "secret-xyz",
		OAuthTokenRequestURL:              "https://idp.example.com/oauth/token",
		OAuthAuthorizationURL:             "https://idp.example.com/oauth/authorize",
		OAuthRedirectURI:                  "http://127.0.0.1:8080",
		OAuthScope:                        "session:role:ANALYST",
		EnableSingleUseRefreshTokens:      true,
		WorkloadIdentityProvider:          "AWS",
		WorkloadIdentityEntraResource:     "api://snowflake",
		WorkloadIdentityImpersonationPath: "arn:aws:iam::111:role/a,arn:aws:iam::222:role/b",
	}

	if err := SaveProfile(path, profile); err != nil {
		t.Fatal(err)
	}

	// The serialized TOML must use snake_case keys.
	out := readTestFile(t, path)
	for _, key := range []string{
		"token =", "token_file_path =", "oauth_client_id =", "oauth_client_secret =",
		"oauth_token_request_url =", "oauth_authorization_url =", "oauth_redirect_uri =",
		"oauth_scope =", "workload_identity_provider =",
		"workload_identity_entra_resource =", "workload_identity_impersonation_path =",
	} {
		if !strings.Contains(out, key) {
			t.Errorf("expected TOML key %q in output:\n%s", key, out)
		}
	}
	// The bool field must be written as an unquoted TOML boolean.
	if !strings.Contains(out, "enable_single_use_refresh_tokens = true") {
		t.Errorf("expected unquoted bool key in output:\n%s", out)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	c := cfg.Connections[0]
	if c.Token != profile.Token || c.TokenFilePath != profile.TokenFilePath ||
		c.OAuthClientID != profile.OAuthClientID || c.OAuthClientSecret != profile.OAuthClientSecret ||
		c.OAuthTokenRequestURL != profile.OAuthTokenRequestURL ||
		c.OAuthAuthorizationURL != profile.OAuthAuthorizationURL ||
		c.OAuthRedirectURI != profile.OAuthRedirectURI || c.OAuthScope != profile.OAuthScope ||
		c.EnableSingleUseRefreshTokens != profile.EnableSingleUseRefreshTokens ||
		c.WorkloadIdentityProvider != profile.WorkloadIdentityProvider ||
		c.WorkloadIdentityEntraResource != profile.WorkloadIdentityEntraResource ||
		c.WorkloadIdentityImpersonationPath != profile.WorkloadIdentityImpersonationPath {
		t.Errorf("auth-field round-trip mismatch: got %+v", c)
	}
}

func TestRoundTrip_ProxyFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	profile := Connection{
		Name:          "proxied",
		Account:       "myorg-test",
		User:          "alice",
		ProxyHost:     "proxy.example.com",
		ProxyPort:     8080,
		ProxyUser:     "puser",
		ProxyPassword: "ppass",
		ProxyProtocol: "https",
		NoProxy:       "localhost,*.internal",
	}

	if err := SaveProfile(path, profile); err != nil {
		t.Fatal(err)
	}

	out := readTestFile(t, path)
	for _, key := range []string{
		"proxy_host =", "proxy_user =", "proxy_password =", "proxy_protocol =", "no_proxy =",
	} {
		if !strings.Contains(out, key) {
			t.Errorf("expected TOML key %q in output:\n%s", key, out)
		}
	}
	// The port must be written as an unquoted TOML integer.
	if !strings.Contains(out, "proxy_port = 8080") {
		t.Errorf("expected unquoted int port in output:\n%s", out)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	c := cfg.Connections[0]
	if c.ProxyHost != profile.ProxyHost || c.ProxyPort != profile.ProxyPort ||
		c.ProxyUser != profile.ProxyUser || c.ProxyPassword != profile.ProxyPassword ||
		c.ProxyProtocol != profile.ProxyProtocol || c.NoProxy != profile.NoProxy {
		t.Errorf("proxy-field round-trip mismatch: got %+v", c)
	}
}

// A zero proxy port must not be emitted, so non-proxied profiles stay clean.
func TestRoundTrip_ProxyPortOmittedWhenZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := SaveProfile(path, Connection{Name: "plain", Account: "a"}); err != nil {
		t.Fatal(err)
	}
	out := readTestFile(t, path)
	if strings.Contains(out, "proxy_port") {
		t.Errorf("expected no proxy_port key for zero port:\n%s", out)
	}
}

func TestAtomicWrite_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := SaveProfile(path, Connection{Name: "test", Account: "a"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected 0600 permissions, got %o", perm)
	}
}

func TestTomlEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`normal`, `normal`},
		{`has "quotes"`, `has \"quotes\"`},
		{`back\slash`, `back\\slash`},
		{`both "and" \`, `both \"and\" \\`},
		{"tab\there", `tab\there`},
		{"new\nline", `new\nline`},
		{"cr\rreturn", `cr\rreturn`},
	}
	for _, tt := range tests {
		got := tomlEscape(tt.input)
		if got != tt.want {
			t.Errorf("tomlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
