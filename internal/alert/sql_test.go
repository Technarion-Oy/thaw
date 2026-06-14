// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package alert

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateAlertSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AlertConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: AlertConfig{
				Name:      "MY_ALERT",
				OrReplace: true,
				Warehouse: "COMPUTE_WH",
				Schedule:  "5 MINUTE",
				Comment:   "hello",
				Condition: "SELECT 1 FROM orders WHERE total > 100;",
				Action:    "INSERT INTO log VALUES (1);",
			},
			contains: []string{
				"CREATE OR REPLACE ALERT \"DB\".\"SC\".MY_ALERT",
				"SCHEDULE = '5 MINUTE'",
				"WAREHOUSE = \"COMPUTE_WH\"",
				"COMMENT = 'hello'",
				"IF (EXISTS (\nSELECT 1 FROM orders WHERE total > 100\n))",
				"THEN\nINSERT INTO log VALUES (1)",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: AlertConfig{
				Name:        "A",
				IfNotExists: true,
				Schedule:    "1 MINUTE",
				Condition:   "SELECT 1",
				Action:      "SELECT 1",
			},
			contains: []string{"CREATE ALERT IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: AlertConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
				Condition:   "SELECT 1",
				Action:      "SELECT 1",
			},
			contains: []string{"CREATE OR REPLACE ALERT"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "serverless alert omits warehouse",
			cfg: AlertConfig{
				Name:      "A",
				Schedule:  "1 MINUTE",
				Condition: "SELECT 1",
				Action:    "SELECT 1",
			},
			absent: []string{"WAREHOUSE ="},
		},
		{
			name: "comment with single quote is escaped",
			cfg: AlertConfig{
				Name:      "A",
				Comment:   "it's fine",
				Condition: "SELECT 1",
				Action:    "SELECT 1",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
		{
			name: "tags emit WITH TAG clause",
			cfg: AlertConfig{
				Name:      "A",
				Tags:      []snowflake.TagPair{{Name: "env", Value: "prod"}, {Name: "team", Value: "data's"}},
				Condition: "SELECT 1",
				Action:    "SELECT 1",
			},
			contains: []string{"WITH TAG (\"env\" = 'prod', \"team\" = 'data''s')"},
		},
		{
			name: "empty tags emit nothing",
			cfg: AlertConfig{
				Name:      "A",
				Tags:      []snowflake.TagPair{{Name: "  ", Value: "ignored"}},
				Condition: "SELECT 1",
				Action:    "SELECT 1",
			},
			absent: []string{"WITH TAG (", "COMMENT", "WAREHOUSE ="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateAlertSql("DB", "SC", tt.cfg)
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

func TestBuildCreateAlertSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateAlertSql("DB", "SC", AlertConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"alert_name",
		"SCHEDULE = '60 MINUTE'",
		"SELECT 1 FROM my_table WHERE <condition>",
		"INSERT INTO my_alert_log SELECT CURRENT_TIMESTAMP()",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}

// TestBuildCreateAlertSqlClauseOrder pins the alert-level clause order to the
// order Snowflake documents for CREATE ALERT — TAG → SCHEDULE → WAREHOUSE →
// COMMENT → IF (EXISTS …) → THEN. Snowflake's CREATE parser is order-sensitive,
// so a config that combines several optional clauses (the case most likely to be
// rejected) must emit them in exactly this sequence.
func TestBuildCreateAlertSqlClauseOrder(t *testing.T) {
	got, err := BuildCreateAlertSql("DB", "SC", AlertConfig{
		Name:      "A",
		Warehouse: "WH",
		Schedule:  "1 MINUTE",
		Comment:   "c",
		Tags:      []snowflake.TagPair{{Name: "env", Value: "prod"}},
		Condition: "SELECT 1",
		Action:    "SELECT 2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := []string{"WITH TAG (", "SCHEDULE = ", "WAREHOUSE = ", "COMMENT = ", "IF (EXISTS (", "THEN"}
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
