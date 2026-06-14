// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package tag

import (
	"strings"
	"testing"
)

func TestBuildCreateTagSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      TagConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: TagConfig{
				Name:          "COST_CENTER",
				OrReplace:     true,
				AllowedValues: []string{"finance", "eng", "ops"},
				Comment:       "department classification",
			},
			contains: []string{
				"CREATE OR REPLACE TAG \"DB\".\"SC\".COST_CENTER",
				"ALLOWED_VALUES 'finance', 'eng', 'ops'",
				"COMMENT = 'department classification'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: TagConfig{
				Name:        "A",
				IfNotExists: true,
			},
			contains: []string{"CREATE TAG IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: TagConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE TAG"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "no allowed values omits clause",
			cfg: TagConfig{
				Name: "A",
			},
			absent: []string{"ALLOWED_VALUES", "COMMENT"},
		},
		{
			name: "blank allowed values are skipped",
			cfg: TagConfig{
				Name:          "A",
				AllowedValues: []string{"  ", "keep", ""},
			},
			contains: []string{"ALLOWED_VALUES 'keep'"},
			absent:   []string{"''"},
		},
		{
			name: "single quotes are escaped in values and comment",
			cfg: TagConfig{
				Name:          "A",
				AllowedValues: []string{"it's"},
				Comment:       "o'hare",
			},
			contains: []string{"ALLOWED_VALUES 'it''s'", "COMMENT = 'o''hare'"},
		},
		{
			name: "propagate with allowed-values-sequence conflict",
			cfg: TagConfig{
				Name:          "A",
				AllowedValues: []string{"hi", "lo"},
				Propagate:     "ON_DEPENDENCY_AND_DATA_MOVEMENT",
				OnConflict:    AllowedValuesSequence,
			},
			contains: []string{"PROPAGATE = ON_DEPENDENCY_AND_DATA_MOVEMENT ON_CONFLICT = ALLOWED_VALUES_SEQUENCE"},
			absent:   []string{"'ALLOWED_VALUES_SEQUENCE'"},
		},
		{
			name: "propagate with fixed string conflict value",
			cfg: TagConfig{
				Name:       "A",
				Propagate:  "ON_DEPENDENCY",
				OnConflict: "it's",
			},
			contains: []string{"PROPAGATE = ON_DEPENDENCY ON_CONFLICT = 'it''s'"},
		},
		{
			name: "propagate lowercased is normalized",
			cfg: TagConfig{
				Name:      "A",
				Propagate: "on_data_movement",
			},
			contains: []string{"PROPAGATE = ON_DATA_MOVEMENT"},
		},
		{
			name: "invalid propagate mode is dropped",
			cfg: TagConfig{
				Name:       "A",
				Propagate:  "BOGUS",
				OnConflict: "x",
			},
			absent: []string{"PROPAGATE", "ON_CONFLICT"},
		},
		{
			name: "on_conflict without propagate is omitted",
			cfg: TagConfig{
				Name:       "A",
				OnConflict: "x",
			},
			absent: []string{"PROPAGATE", "ON_CONFLICT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: TagConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateTagSql("DB", "SC", tt.cfg)
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

func TestBuildCreateTagSqlPlaceholder(t *testing.T) {
	got, err := BuildCreateTagSql("DB", "SC", TagConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "tag_name") {
		t.Errorf("expected placeholder name, got:\n%s", got)
	}
}
