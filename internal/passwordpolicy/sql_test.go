// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package passwordpolicy

import (
	"strings"
	"testing"
)

func intp(v int) *int { return &v }

func TestBuildCreatePasswordPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      PasswordPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: PasswordPolicyConfig{
				Name:              "STRICT_PWD",
				OrReplace:         true,
				MinLength:         intp(12),
				MaxLength:         intp(64),
				MinUpperCaseChars: intp(2),
				MinLowerCaseChars: intp(2),
				MinNumericChars:   intp(1),
				MinSpecialChars:   intp(1),
				MinAgeDays:        intp(0),
				MaxAgeDays:        intp(30),
				MaxRetries:        intp(3),
				LockoutTimeMins:   intp(20),
				History:           intp(10),
				Comment:           "corp policy",
			},
			contains: []string{
				"CREATE OR REPLACE PASSWORD POLICY \"DB\".\"SC\".STRICT_PWD",
				"PASSWORD_MIN_LENGTH = 12",
				"PASSWORD_MAX_LENGTH = 64",
				"PASSWORD_MIN_UPPER_CASE_CHARS = 2",
				"PASSWORD_MIN_LOWER_CASE_CHARS = 2",
				"PASSWORD_MIN_NUMERIC_CHARS = 1",
				"PASSWORD_MIN_SPECIAL_CHARS = 1",
				"PASSWORD_MIN_AGE_DAYS = 0",
				"PASSWORD_MAX_AGE_DAYS = 30",
				"PASSWORD_MAX_RETRIES = 3",
				"PASSWORD_LOCKOUT_TIME_MINS = 20",
				"PASSWORD_HISTORY = 10",
				"COMMENT = 'corp policy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal config emits only the name",
			cfg: PasswordPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{`CREATE PASSWORD POLICY "DB"."SC".DEFAULTS;`},
			absent:   []string{"PASSWORD_", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: PasswordPolicyConfig{
				Name:        "A",
				IfNotExists: true,
				MinLength:   intp(8),
			},
			contains: []string{"CREATE PASSWORD POLICY IF NOT EXISTS", "PASSWORD_MIN_LENGTH = 8"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: PasswordPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE PASSWORD POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "zero value is emitted (distinct from unset)",
			cfg: PasswordPolicyConfig{
				Name:            "ZERO",
				MinSpecialChars: intp(0),
			},
			contains: []string{"PASSWORD_MIN_SPECIAL_CHARS = 0"},
		},
		{
			name: "comment is escaped",
			cfg: PasswordPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name: "blank name yields placeholder",
			cfg:  PasswordPolicyConfig{},
			contains: []string{
				"password_policy_name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreatePasswordPolicySql("DB", "SC", tt.cfg)
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
