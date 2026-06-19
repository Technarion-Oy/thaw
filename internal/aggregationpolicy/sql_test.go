// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package aggregationpolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateAggregationPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AggregationPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: AggregationPolicyConfig{
				Name:      "MIN_GROUP",
				OrReplace: true,
				Body:      "AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)",
				Comment:   "privacy policy",
			},
			contains: []string{
				"CREATE OR REPLACE AGGREGATION POLICY \"DB\".\"SC\".MIN_GROUP AS () RETURNS AGGREGATION_CONSTRAINT ->",
				"AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)",
				"COMMENT = 'privacy policy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal config emits a default body",
			cfg: AggregationPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{
				`CREATE AGGREGATION POLICY "DB"."SC".DEFAULTS AS () RETURNS AGGREGATION_CONSTRAINT ->`,
				"AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: AggregationPolicyConfig{
				Name:        "A",
				IfNotExists: true,
			},
			contains: []string{"CREATE AGGREGATION POLICY IF NOT EXISTS"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: AggregationPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE AGGREGATION POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "conditional body is emitted verbatim",
			cfg: AggregationPolicyConfig{
				Name: "COND",
				Body: "CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN NO_AGGREGATION_CONSTRAINT() ELSE AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 3) END",
			},
			contains: []string{"CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN NO_AGGREGATION_CONSTRAINT()"},
		},
		{
			name: "comment is escaped",
			cfg: AggregationPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name:     "blank name yields placeholder",
			cfg:      AggregationPolicyConfig{},
			contains: []string{"aggregation_policy_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateAggregationPolicySql("DB", "SC", tt.cfg)
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
