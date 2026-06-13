// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package materializedview

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateMaterializedViewSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      MaterializedViewConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: MaterializedViewConfig{
				Name:      "MY_MV",
				OrReplace: true,
				Secure:    true,
				ClusterBy: "c1, c2",
				Comment:   "hello",
				Query:     "SELECT id, total FROM src;",
			},
			contains: []string{
				"CREATE OR REPLACE SECURE MATERIALIZED VIEW \"DB\".\"SC\".MY_MV",
				"CLUSTER BY (c1, c2)",
				"COMMENT = 'hello'",
				"AS\nSELECT id, total FROM src;",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: MaterializedViewConfig{
				Name:        "MV",
				IfNotExists: true,
				Query:       "SELECT 1",
			},
			contains: []string{"CREATE MATERIALIZED VIEW IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: MaterializedViewConfig{
				Name:        "MV",
				OrReplace:   true,
				IfNotExists: true,
				Query:       "SELECT 1",
			},
			contains: []string{"CREATE OR REPLACE MATERIALIZED VIEW"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "comment with single quote is escaped",
			cfg: MaterializedViewConfig{
				Name:    "MV",
				Comment: "it's fine",
				Query:   "SELECT 1",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
		{
			name: "copy grants and tags",
			cfg: MaterializedViewConfig{
				Name:       "MV",
				CopyGrants: true,
				Tags:       []snowflake.TagPair{{Name: "env", Value: "prod"}, {Name: "team", Value: "data's"}},
				Query:      "SELECT 1",
			},
			contains: []string{
				"COPY GRANTS",
				"TAG (\"env\" = 'prod', \"team\" = 'data''s')",
			},
		},
		{
			name: "empty tags emit nothing",
			cfg: MaterializedViewConfig{
				Name:  "MV",
				Tags:  []snowflake.TagPair{{Name: "  ", Value: "ignored"}},
				Query: "SELECT 1",
			},
			absent: []string{"TAG (", "CLUSTER BY", "COPY GRANTS", "SECURE", "COMMENT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateMaterializedViewSql("DB", "SC", tt.cfg)
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

func TestBuildCreateMaterializedViewSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateMaterializedViewSql("DB", "SC", MaterializedViewConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"materialized_view_name", "SELECT * FROM <source_table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}

// TestBuildCreateMaterializedViewSqlClauseOrder pins the view-level clause order
// to the order Snowflake documents for CREATE MATERIALIZED VIEW — COPY GRANTS →
// COMMENT → CLUSTER BY → TAG → AS. Snowflake's CREATE parser is order-sensitive,
// so a config that combines several optional clauses (the case most likely to be
// rejected) must emit them in exactly this sequence.
func TestBuildCreateMaterializedViewSqlClauseOrder(t *testing.T) {
	got, err := BuildCreateMaterializedViewSql("DB", "SC", MaterializedViewConfig{
		Name:       "MV",
		CopyGrants: true,
		Comment:    "c",
		ClusterBy:  "c1",
		Tags:       []snowflake.TagPair{{Name: "env", Value: "prod"}},
		Query:      "SELECT 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each clause must appear, and in the documented relative order.
	order := []string{"COPY GRANTS", "COMMENT = ", "CLUSTER BY (", "TAG (", "AS"}
	prev := -1
	for _, marker := range order {
		idx := strings.Index(got, marker)
		if idx < 0 {
			t.Fatalf("expected clause %q in:\n%s", marker, got)
		}
		if idx <= prev {
			t.Errorf("clause %q is out of order (index %d ≤ previous %d) in:\n%s", marker, idx, prev, got)
		}
		prev = idx
	}
}
