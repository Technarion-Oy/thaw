// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package view

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateViewSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ViewConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: ViewConfig{
				Name:      "MY_VIEW",
				OrReplace: true,
				Secure:    true,
				Recursive: true,
				Columns:   "c1, c2",
				Comment:   "hello",
				Query:     "SELECT id, total FROM src;",
			},
			contains: []string{
				"CREATE OR REPLACE SECURE RECURSIVE VIEW \"DB\".\"SC\".MY_VIEW",
				"(c1, c2)",
				"COMMENT = 'hello'",
				"AS\nSELECT id, total FROM src;",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal",
			cfg: ViewConfig{
				Name:  "V",
				Query: "SELECT 1",
			},
			contains: []string{"CREATE VIEW \"DB\".\"SC\".V", "AS\nSELECT 1;"},
			absent:   []string{"SECURE", "RECURSIVE", "IF NOT EXISTS", "COPY GRANTS", "COMMENT", "TAG (", "(c1"},
		},
		{
			name: "secure only",
			cfg: ViewConfig{
				Name:   "V",
				Secure: true,
				Query:  "SELECT 1",
			},
			contains: []string{"CREATE SECURE VIEW"},
			absent:   []string{"RECURSIVE"},
		},
		{
			name: "recursive only",
			cfg: ViewConfig{
				Name:      "V",
				Recursive: true,
				Query:     "SELECT 1",
			},
			contains: []string{"CREATE RECURSIVE VIEW"},
			absent:   []string{"SECURE"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: ViewConfig{
				Name:        "V",
				IfNotExists: true,
				Query:       "SELECT 1",
			},
			contains: []string{"CREATE VIEW IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: ViewConfig{
				Name:        "V",
				OrReplace:   true,
				IfNotExists: true,
				Query:       "SELECT 1",
			},
			contains: []string{"CREATE OR REPLACE VIEW"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "columns list",
			cfg: ViewConfig{
				Name:    "V",
				Columns: "  a, b, c  ",
				Query:   "SELECT 1, 2, 3",
			},
			contains: []string{"(a, b, c)"},
		},
		{
			name: "comment with single quote is escaped",
			cfg: ViewConfig{
				Name:    "V",
				Comment: "it's fine",
				Query:   "SELECT 1",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
		{
			name: "copy grants and tags",
			cfg: ViewConfig{
				Name:       "V",
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
			name: "case sensitive name is quoted verbatim",
			cfg: ViewConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Query:         "SELECT 1",
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name: "empty tags emit nothing",
			cfg: ViewConfig{
				Name:  "V",
				Tags:  []snowflake.TagPair{{Name: "  ", Value: "ignored"}},
				Query: "SELECT 1",
			},
			absent: []string{"TAG (", "COPY GRANTS", "SECURE", "COMMENT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateViewSql("DB", "SC", tt.cfg)
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

func TestBuildCreateViewSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateViewSql("DB", "SC", ViewConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"view_name", "SELECT * FROM <source_table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}

// TestBuildCreateViewSqlClauseOrder pins the view-level clause order to the order
// Snowflake documents for CREATE VIEW — (columns) → COPY GRANTS → COMMENT → TAG →
// AS. Snowflake's CREATE parser is order-sensitive, so a config that combines
// several optional clauses must emit them in exactly this sequence.
func TestBuildCreateViewSqlClauseOrder(t *testing.T) {
	got, err := BuildCreateViewSql("DB", "SC", ViewConfig{
		Name:       "V",
		Columns:    "a, b",
		CopyGrants: true,
		Comment:    "c",
		Tags:       []snowflake.TagPair{{Name: "env", Value: "prod"}},
		Query:      "SELECT 1, 2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := []string{"(a, b)", "COPY GRANTS", "COMMENT = ", "TAG (", "AS"}
	prev := -1
	for _, marker := range order {
		idx := strings.Index(got, marker)
		if idx < 0 {
			t.Fatalf("expected clause %q in:\n%s", marker, got)
		}
		if idx <= prev {
			t.Errorf("clause %q is out of order (index %d <= previous %d) in:\n%s", marker, idx, prev, got)
		}
		prev = idx
	}
}
