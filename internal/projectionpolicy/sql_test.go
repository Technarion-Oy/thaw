// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package projectionpolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateProjectionPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ProjectionPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: ProjectionPolicyConfig{
				Name:      "NO_PROJECT",
				OrReplace: true,
				Body:      "PROJECTION_CONSTRAINT(ALLOW => false)",
				Comment:   "privacy policy",
			},
			contains: []string{
				"CREATE OR REPLACE PROJECTION POLICY \"DB\".\"SC\".NO_PROJECT AS () RETURNS PROJECTION_CONSTRAINT ->",
				"PROJECTION_CONSTRAINT(ALLOW => false)",
				"COMMENT = 'privacy policy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal config emits a default body",
			cfg: ProjectionPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{
				`CREATE PROJECTION POLICY "DB"."SC".DEFAULTS AS () RETURNS PROJECTION_CONSTRAINT ->`,
				"PROJECTION_CONSTRAINT(ALLOW => true)",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: ProjectionPolicyConfig{
				Name:        "A",
				IfNotExists: true,
			},
			contains: []string{"CREATE PROJECTION POLICY IF NOT EXISTS"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: ProjectionPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE PROJECTION POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "conditional body is emitted verbatim",
			cfg: ProjectionPolicyConfig{
				Name: "COND",
				Body: "CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN PROJECTION_CONSTRAINT(ALLOW => true) ELSE PROJECTION_CONSTRAINT(ALLOW => false) END",
			},
			contains: []string{"CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN PROJECTION_CONSTRAINT(ALLOW => true)"},
		},
		{
			name: "comment is escaped",
			cfg: ProjectionPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name:     "blank name yields placeholder",
			cfg:      ProjectionPolicyConfig{},
			contains: []string{"projection_policy_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateProjectionPolicySql("DB", "SC", tt.cfg)
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
