// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package privacypolicy

import (
	"strings"
	"testing"
)

func TestBuildCreatePrivacyPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      PrivacyPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: PrivacyPolicyConfig{
				Name:      "BUDGET_POLICY",
				OrReplace: true,
				Body:      "PRIVACY_BUDGET(BUDGET_NAME => 'analytics', BUDGET_LIMIT => 233.0)",
				Comment:   "differential privacy",
			},
			contains: []string{
				"CREATE OR REPLACE PRIVACY POLICY \"DB\".\"SC\".BUDGET_POLICY",
				"AS () RETURNS PRIVACY_BUDGET ->",
				"PRIVACY_BUDGET(BUDGET_NAME => 'analytics', BUDGET_LIMIT => 233.0)",
				"COMMENT = 'differential privacy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "no privacy policy body when not or replace",
			cfg: PrivacyPolicyConfig{
				Name:        "A",
				IfNotExists: true,
				Body:        "NO_PRIVACY_POLICY()",
			},
			contains: []string{"CREATE PRIVACY POLICY IF NOT EXISTS", "NO_PRIVACY_POLICY()"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: PrivacyPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE PRIVACY POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "single quotes escaped in comment",
			cfg: PrivacyPolicyConfig{
				Name:    "A",
				Comment: "o'hare",
			},
			contains: []string{"COMMENT = 'o''hare'"},
		},
		{
			name: "comment omitted by default",
			cfg: PrivacyPolicyConfig{
				Name: "A",
			},
			absent: []string{"COMMENT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: PrivacyPolicyConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreatePrivacyPolicySql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(got, ";") {
				t.Errorf("statement should end with ';', got:\n%s", got)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("expected output to NOT contain %q, got:\n%s", no, got)
				}
			}
		})
	}
}

// TestBuildCreatePrivacyPolicySqlPlaceholders verifies that an empty config still
// yields a well-formed, completable template (placeholder name and body) rather
// than invalid SQL.
func TestBuildCreatePrivacyPolicySqlPlaceholders(t *testing.T) {
	got, err := BuildCreatePrivacyPolicySql("DB", "SC", PrivacyPolicyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"privacy_policy_name", "AS () RETURNS PRIVACY_BUDGET ->", "PRIVACY_BUDGET(BUDGET_NAME => 'privacy_budget')"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
