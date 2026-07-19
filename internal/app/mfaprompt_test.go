// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"testing"

	"thaw/internal/snowflake"
)

// TestUsesSingleUseMFACredential locks the classification that decides whether a
// session must run on the single shared connection (tab sessions reuse it,
// MCP is rejected) rather than open per-connection logins that would re-send a
// spent one-time credential. See issue #804.
func TestUsesSingleUseMFACredential(t *testing.T) {
	tests := []struct {
		name string
		auth string
		pass string
		want bool
	}{
		{"mfa push", "username_password_mfa", "", true},
		{"mfa push, mixed case", "Username_Password_MFA", "", true},
		{"password with TOTP", "snowflake", "123456", true},
		{"empty authenticator with TOTP", "", "123456", true},
		{"password without TOTP", "snowflake", "", false},
		{"empty authenticator without TOTP", "", "", false},
		{"key-pair", "snowflake_jwt", "", false},
		{"external browser", "externalbrowser", "", false},
		{"oauth with stray passcode", "oauth", "123456", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &snowflake.ConnectParams{Authenticator: tt.auth, Passcode: tt.pass}
			if got := usesSingleUseMFACredential(p); got != tt.want {
				t.Errorf("usesSingleUseMFACredential(auth=%q pass=%q) = %v, want %v", tt.auth, tt.pass, got, tt.want)
			}
		})
	}
}
