// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sessionpolicy

import (
	"strings"
	"testing"
)

func intp(v int) *int { return &v }

func TestBuildCreateSessionPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      SessionPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: SessionPolicyConfig{
				Name:                  "STRICT_SESSION",
				OrReplace:             true,
				IdleTimeoutMins:       intp(30),
				UIIdleTimeoutMins:     intp(15),
				MaxLifespanMins:       intp(480),
				UIMaxLifespanMins:     intp(240),
				AllowedSecondaryRoles: []string{"ALL"},
				Comment:               "corp policy",
			},
			contains: []string{
				"CREATE OR REPLACE SESSION POLICY \"DB\".\"SC\".STRICT_SESSION",
				"SESSION_IDLE_TIMEOUT_MINS = 30",
				"SESSION_UI_IDLE_TIMEOUT_MINS = 15",
				"SESSION_MAX_LIFESPAN_MINS = 480",
				"SESSION_UI_MAX_LIFESPAN_MINS = 240",
				"ALLOWED_SECONDARY_ROLES = ('ALL')",
				"COMMENT = 'corp policy'",
			},
			absent: []string{"IF NOT EXISTS", "BLOCKED_SECONDARY_ROLES"},
		},
		{
			name: "minimal config emits only the name",
			cfg: SessionPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{`CREATE SESSION POLICY "DB"."SC".DEFAULTS;`},
			absent:   []string{"SESSION_", "SECONDARY_ROLES", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: SessionPolicyConfig{
				Name:            "A",
				IfNotExists:     true,
				IdleTimeoutMins: intp(60),
			},
			contains: []string{"CREATE SESSION POLICY IF NOT EXISTS", "SESSION_IDLE_TIMEOUT_MINS = 60"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: SessionPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE SESSION POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "zero lifespan is emitted (distinct from unset)",
			cfg: SessionPolicyConfig{
				Name:            "ZERO",
				MaxLifespanMins: intp(0),
			},
			contains: []string{"SESSION_MAX_LIFESPAN_MINS = 0"},
		},
		{
			name: "blocked roles render bare identifiers",
			cfg: SessionPolicyConfig{
				Name:                  "BLK",
				BlockedSecondaryRoles: []string{"R1", "R2"},
			},
			contains: []string{`BLOCKED_SECONDARY_ROLES = (R1, R2)`},
			absent:   []string{"ALLOWED_SECONDARY_ROLES"},
		},
		{
			name: "comment is escaped",
			cfg: SessionPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name: "blank name yields placeholder",
			cfg:  SessionPolicyConfig{},
			contains: []string{
				"session_policy_name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateSessionPolicySql("DB", "SC", tt.cfg)
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

func TestFormatSecondaryRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  string
	}{
		{"all literal", []string{"ALL"}, "('ALL')"},
		{"all case-insensitive", []string{"all"}, "('ALL')"},
		{"simple roles emitted bare", []string{"R1", "R2"}, `(R1, R2)`},
		{"lowercase emitted bare (Snowflake uppercases)", []string{"analyst"}, `(analyst)`},
		{"role needing quoting is double-quoted", []string{"my role"}, `("my role")`},
		{"blank entries skipped", []string{"", "  ", "R1"}, `(R1)`},
		{"empty list", []string{}, "()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSecondaryRoles(tt.roles); got != tt.want {
				t.Errorf("FormatSecondaryRoles(%v) = %q, want %q", tt.roles, got, tt.want)
			}
		})
	}
}
