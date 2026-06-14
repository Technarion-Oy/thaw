// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package maskingpolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateMaskingPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      MaskingPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: MaskingPolicyConfig{
				Name:      "EMAIL_MASK",
				OrReplace: true,
				Args: []MaskingArg{
					{Name: "val", Type: "VARCHAR"},
					{Name: "role", Type: "VARCHAR"},
				},
				ReturnType:          "VARCHAR",
				Body:                "CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN val ELSE '***' END",
				Comment:             "mask emails",
				ExemptOtherPolicies: true,
			},
			contains: []string{
				"CREATE OR REPLACE MASKING POLICY \"DB\".\"SC\".EMAIL_MASK AS",
				"(val VARCHAR, role VARCHAR)",
				"RETURNS VARCHAR ->",
				"CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN val ELSE '***' END",
				"COMMENT = 'mask emails'",
				"EXEMPT_OTHER_POLICIES = TRUE",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: MaskingPolicyConfig{
				Name:        "A",
				IfNotExists: true,
				Args:        []MaskingArg{{Name: "v", Type: "NUMBER"}},
				ReturnType:  "NUMBER",
				Body:        "v",
			},
			contains: []string{"CREATE MASKING POLICY IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: MaskingPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
				Args:        []MaskingArg{{Name: "v", Type: "NUMBER"}},
			},
			contains: []string{"CREATE OR REPLACE MASKING POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "return type defaults to first arg type",
			cfg: MaskingPolicyConfig{
				Name: "A",
				Args: []MaskingArg{{Name: "ssn", Type: "NUMBER(9,0)"}},
				Body: "ssn",
			},
			contains: []string{"(ssn NUMBER(9,0))", "RETURNS NUMBER(9,0) ->"},
		},
		{
			name: "blank arg rows are skipped",
			cfg: MaskingPolicyConfig{
				Name: "A",
				Args: []MaskingArg{
					{Name: " ", Type: "VARCHAR"},
					{Name: "keep", Type: "VARCHAR"},
					{Name: "noType", Type: ""},
				},
				ReturnType: "VARCHAR",
				Body:       "keep",
			},
			contains: []string{"(keep VARCHAR)"},
		},
		{
			name: "single quotes escaped in comment",
			cfg: MaskingPolicyConfig{
				Name:       "A",
				Args:       []MaskingArg{{Name: "v", Type: "VARCHAR"}},
				ReturnType: "VARCHAR",
				Body:       "v",
				Comment:    "o'hare",
			},
			contains: []string{"COMMENT = 'o''hare'"},
		},
		{
			name: "exempt other policies omitted by default",
			cfg: MaskingPolicyConfig{
				Name:       "A",
				Args:       []MaskingArg{{Name: "v", Type: "VARCHAR"}},
				ReturnType: "VARCHAR",
				Body:       "v",
			},
			absent: []string{"EXEMPT_OTHER_POLICIES", "COMMENT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: MaskingPolicyConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Args:          []MaskingArg{{Name: "v", Type: "VARCHAR"}},
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateMaskingPolicySql("DB", "SC", tt.cfg)
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

// TestBuildCreateMaskingPolicySqlPlaceholders verifies that an empty config
// still yields a well-formed, completable template (placeholder name, signature,
// return type, and body) rather than invalid SQL.
func TestBuildCreateMaskingPolicySqlPlaceholders(t *testing.T) {
	got, err := BuildCreateMaskingPolicySql("DB", "SC", MaskingPolicyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"masking_policy_name", "(val VARCHAR)", "RETURNS VARCHAR ->", "'***MASKED***'"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
