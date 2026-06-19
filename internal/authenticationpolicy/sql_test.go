// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
