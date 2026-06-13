// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package dynamictable

import (
	"strings"
	"testing"
)

func TestBuildCreateDynamicTableSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      DynamicTableConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: DynamicTableConfig{
				Name:        "MY_DT",
				OrReplace:   true,
				TargetLag:   "1 minute",
				Warehouse:   "COMPUTE_WH",
				RefreshMode: "incremental",
				Initialize:  "on_create",
				ClusterBy:   "c1, c2",
				Comment:     "hello",
				Query:       "SELECT * FROM src;",
			},
			contains: []string{
				"CREATE OR REPLACE DYNAMIC TABLE \"DB\".\"SC\".MY_DT",
				"TARGET_LAG = '1 minute'",
				"WAREHOUSE = \"COMPUTE_WH\"",
				"REFRESH_MODE = INCREMENTAL",
				"INITIALIZE = ON_CREATE",
				"CLUSTER BY (c1, c2)",
				"COMMENT = 'hello'",
				"AS\nSELECT * FROM src;",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "downstream target lag is a keyword",
			cfg: DynamicTableConfig{
				Name:      "DT",
				TargetLag: "downstream",
				Warehouse: "WH",
				Query:     "SELECT 1",
			},
			contains: []string{"TARGET_LAG = DOWNSTREAM"},
			absent:   []string{"TARGET_LAG = 'downstream'"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: DynamicTableConfig{
				Name:        "DT",
				IfNotExists: true,
				TargetLag:   "1 hour",
				Warehouse:   "WH",
				Query:       "SELECT 1",
			},
			contains: []string{"CREATE DYNAMIC TABLE IF NOT EXISTS"},
		},
		{
			name: "transient",
			cfg: DynamicTableConfig{
				Name:      "DT",
				Transient: true,
				TargetLag: "1 hour",
				Warehouse: "WH",
				Query:     "SELECT 1",
			},
			contains: []string{"CREATE TRANSIENT DYNAMIC TABLE"},
		},
		{
			name: "comment with single quote is escaped",
			cfg: DynamicTableConfig{
				Name:      "DT",
				TargetLag: "1 hour",
				Warehouse: "WH",
				Comment:   "it's fine",
				Query:     "SELECT 1",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateDynamicTableSql("DB", "SC", tt.cfg)
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

func TestBuildCreateDynamicTableSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateDynamicTableSql("DB", "SC", DynamicTableConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"dynamic_table_name", "TARGET_LAG = '1 minute'", "WAREHOUSE = <warehouse>", "SELECT * FROM <source_table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}
