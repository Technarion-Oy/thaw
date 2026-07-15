// SPDX-License-Identifier: GPL-3.0-or-later

package authenticationpolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateAuthenticationPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AuthenticationPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: AuthenticationPolicyConfig{
				Name:                  "STRICT_AUTH",
				OrReplace:             true,
				AuthenticationMethods: []string{"PASSWORD", "SAML"},
				ClientTypes:           []string{"SNOWFLAKE_UI", "DRIVERS"},
				SecurityIntegrations:  []string{"MY_OKTA"},
				MFAEnrollment:         "REQUIRED",
				Comment:               "corp policy",
			},
			contains: []string{
				"CREATE OR REPLACE AUTHENTICATION POLICY \"DB\".\"SC\".STRICT_AUTH",
				"AUTHENTICATION_METHODS = ('PASSWORD', 'SAML')",
				"CLIENT_TYPES = ('SNOWFLAKE_UI', 'DRIVERS')",
				"SECURITY_INTEGRATIONS = ('MY_OKTA')",
				"MFA_ENROLLMENT = REQUIRED",
				"COMMENT = 'corp policy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal config emits only the name",
			cfg: AuthenticationPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{`CREATE AUTHENTICATION POLICY "DB"."SC".DEFAULTS;`},
			absent:   []string{"AUTHENTICATION_METHODS", "CLIENT_TYPES", "SECURITY_INTEGRATIONS", "MFA_ENROLLMENT", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: AuthenticationPolicyConfig{
				Name:                  "A",
				IfNotExists:           true,
				AuthenticationMethods: []string{"ALL"},
			},
			contains: []string{"CREATE AUTHENTICATION POLICY IF NOT EXISTS", "AUTHENTICATION_METHODS = ('ALL')"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: AuthenticationPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE AUTHENTICATION POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "mfa enrollment is uppercased",
			cfg: AuthenticationPolicyConfig{
				Name:          "M",
				MFAEnrollment: "optional",
			},
			contains: []string{"MFA_ENROLLMENT = OPTIONAL"},
		},
		{
			name: "blank tokens are skipped (no empty list)",
			cfg: AuthenticationPolicyConfig{
				Name:        "B",
				ClientTypes: []string{"", "  "},
			},
			contains: []string{`CREATE AUTHENTICATION POLICY "DB"."SC".B;`},
			absent:   []string{"CLIENT_TYPES"},
		},
		{
			name: "mfa enrollment with breakout chars is dropped (bare-token guard)",
			cfg: AuthenticationPolicyConfig{
				Name:          "INJ",
				MFAEnrollment: "OPTIONAL) ; DROP",
			},
			contains: []string{`CREATE AUTHENTICATION POLICY "DB"."SC".INJ;`},
			absent:   []string{"MFA_ENROLLMENT"},
		},
		{
			name: "comment is escaped",
			cfg: AuthenticationPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name:     "blank name yields placeholder",
			cfg:      AuthenticationPolicyConfig{},
			contains: []string{"authentication_policy_name"},
		},
		{
			name: "nested bags are emitted when set",
			cfg: AuthenticationPolicyConfig{
				Name:                   "BAGS",
				MFAPolicy:              MFAPolicy{AllowedMethods: []string{"TOTP", "DUO"}},
				PATPolicy:              PATPolicy{DefaultExpiryInDays: intp(30)},
				WorkloadIdentityPolicy: WorkloadIdentityPolicy{AllowedProviders: []string{"AWS"}},
				ClientPolicy:           ClientPolicy{Entries: []ClientPolicyEntry{{Driver: "JDBC_DRIVER", MinimumVersion: "3.13.0"}}},
			},
			contains: []string{
				"MFA_POLICY = ( ALLOWED_METHODS = ('TOTP', 'DUO') )",
				"PAT_POLICY = ( DEFAULT_EXPIRY_IN_DAYS = 30 )",
				"WORKLOAD_IDENTITY_POLICY = ( ALLOWED_PROVIDERS = (AWS) )",
				"CLIENT_POLICY = ( JDBC_DRIVER = ( MINIMUM_VERSION = '3.13.0' ) )",
			},
		},
		{
			name:   "empty nested bags are omitted",
			cfg:    AuthenticationPolicyConfig{Name: "EMPTYBAGS"},
			absent: []string{"MFA_POLICY", "PAT_POLICY", "WORKLOAD_IDENTITY_POLICY", "CLIENT_POLICY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateAuthenticationPolicySql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected SQL to contain %q\ngot:\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("expected SQL to NOT contain %q\ngot:\n%s", no, got)
				}
			}
		})
	}
}

func TestFormatStringList(t *testing.T) {
	if got := FormatStringList([]string{"PASSWORD", "SAML"}); got != "('PASSWORD', 'SAML')" {
		t.Errorf("got %q", got)
	}
	if got := FormatStringList([]string{"ALL"}); got != "('ALL')" {
		t.Errorf("got %q", got)
	}
	if got := FormatStringList([]string{"", " "}); got != "()" {
		t.Errorf("got %q", got)
	}
}
