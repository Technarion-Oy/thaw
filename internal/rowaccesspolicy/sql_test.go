// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package rowaccesspolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateRowAccessPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RowAccessPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: RowAccessPolicyConfig{
				Name:      "SALES_RAP",
				OrReplace: true,
				Args: []RowAccessArg{
					{Name: "region", Type: "VARCHAR"},
					{Name: "dept", Type: "VARCHAR"},
				},
				Body:    "CURRENT_ROLE() = 'ADMIN' OR region = CURRENT_REGION()",
				Comment: "limit by region",
			},
			contains: []string{
				"CREATE OR REPLACE ROW ACCESS POLICY \"DB\".\"SC\".SALES_RAP AS",
				"(region VARCHAR, dept VARCHAR)",
				"RETURNS BOOLEAN ->",
				"CURRENT_ROLE() = 'ADMIN' OR region = CURRENT_REGION()",
				"COMMENT = 'limit by region'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: RowAccessPolicyConfig{
				Name:        "A",
				IfNotExists: true,
				Args:        []RowAccessArg{{Name: "v", Type: "NUMBER"}},
				Body:        "TRUE",
			},
			contains: []string{"CREATE ROW ACCESS POLICY IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: RowAccessPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
				Args:        []RowAccessArg{{Name: "v", Type: "NUMBER"}},
			},
			contains: []string{"CREATE OR REPLACE ROW ACCESS POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "always returns boolean",
			cfg: RowAccessPolicyConfig{
				Name: "A",
				Args: []RowAccessArg{{Name: "n", Type: "NUMBER(9,0)"}},
				Body: "n > 0",
			},
			contains: []string{"(n NUMBER(9,0))", "RETURNS BOOLEAN ->"},
		},
		{
			name: "blank arg rows are skipped",
			cfg: RowAccessPolicyConfig{
				Name: "A",
				Args: []RowAccessArg{
					{Name: " ", Type: "VARCHAR"},
					{Name: "keep", Type: "VARCHAR"},
					{Name: "noType", Type: ""},
				},
				Body: "TRUE",
			},
			contains: []string{"(keep VARCHAR)"},
		},
		{
			name: "single quotes escaped in comment",
			cfg: RowAccessPolicyConfig{
				Name:    "A",
				Args:    []RowAccessArg{{Name: "v", Type: "VARCHAR"}},
				Body:    "TRUE",
				Comment: "o'hare",
			},
			contains: []string{"COMMENT = 'o''hare'"},
		},
		{
			name: "comment omitted by default",
			cfg: RowAccessPolicyConfig{
				Name: "A",
				Args: []RowAccessArg{{Name: "v", Type: "VARCHAR"}},
				Body: "TRUE",
			},
			absent: []string{"COMMENT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: RowAccessPolicyConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Args:          []RowAccessArg{{Name: "v", Type: "VARCHAR"}},
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateRowAccessPolicySql("DB", "SC", tt.cfg)
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

// TestBuildCreateRowAccessPolicySqlPlaceholders verifies that an empty config
// still yields a well-formed, completable template (placeholder name, signature,
// and body) rather than invalid SQL.
func TestBuildCreateRowAccessPolicySqlPlaceholders(t *testing.T) {
	got, err := BuildCreateRowAccessPolicySql("DB", "SC", RowAccessPolicyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"row_access_policy_name", "(val VARCHAR)", "RETURNS BOOLEAN ->", "TRUE"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
