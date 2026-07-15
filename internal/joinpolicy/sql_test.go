// SPDX-License-Identifier: GPL-3.0-or-later

package joinpolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateJoinPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      JoinPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: JoinPolicyConfig{
				Name:      "REQUIRE_JOIN",
				OrReplace: true,
				Body:      "JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)",
				Comment:   "force joins",
			},
			contains: []string{
				"CREATE OR REPLACE JOIN POLICY \"DB\".\"SC\".REQUIRE_JOIN",
				"AS () RETURNS JOIN_CONSTRAINT ->",
				"JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)",
				"COMMENT = 'force joins'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists when not or replace",
			cfg: JoinPolicyConfig{
				Name:        "A",
				IfNotExists: true,
				Body:        "JOIN_CONSTRAINT(JOIN_REQUIRED => FALSE)",
			},
			contains: []string{"CREATE JOIN POLICY IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: JoinPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE JOIN POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "single quotes escaped in comment",
			cfg: JoinPolicyConfig{
				Name:    "A",
				Comment: "o'hare",
			},
			contains: []string{"COMMENT = 'o''hare'"},
		},
		{
			name: "comment omitted by default",
			cfg: JoinPolicyConfig{
				Name: "A",
			},
			absent: []string{"COMMENT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: JoinPolicyConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateJoinPolicySql("DB", "SC", tt.cfg)
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

// TestBuildCreateJoinPolicySqlPlaceholders verifies that an empty config still
// yields a well-formed, completable template (placeholder name and body) rather
// than invalid SQL.
func TestBuildCreateJoinPolicySqlPlaceholders(t *testing.T) {
	got, err := BuildCreateJoinPolicySql("DB", "SC", JoinPolicyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"join_policy_name", "AS () RETURNS JOIN_CONSTRAINT ->", "JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
