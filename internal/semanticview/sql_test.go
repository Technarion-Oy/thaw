// SPDX-License-Identifier: GPL-3.0-or-later

package semanticview

import (
	"strings"
	"testing"
)

func TestBuildCreateSemanticViewSql(t *testing.T) {
	body := "TABLES (\n    orders AS DB.SC.ORDERS PRIMARY KEY (order_id)\n  )\n  METRICS (\n    orders.revenue AS SUM(amount)\n  )"

	tests := []struct {
		name     string
		db       string
		schema   string
		cfg      SemanticViewConfig
		contains []string
		absent   []string
	}{
		{
			name:   "basic with body and comment",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:    "sales",
				Body:    body,
				Comment: "sales layer",
			},
			contains: []string{
				`CREATE SEMANTIC VIEW "DB"."SC".sales`,
				"TABLES (",
				"orders.revenue AS SUM(amount)",
				"COMMENT = 'sales layer'",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "COPY GRANTS"},
		},
		{
			name:   "or replace wins over if not exists",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:        "sales",
				OrReplace:   true,
				IfNotExists: true,
				Body:        body,
				CopyGrants:  true,
			},
			contains: []string{
				`CREATE OR REPLACE SEMANTIC VIEW "DB"."SC".sales`,
				"COPY GRANTS",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name:   "if not exists alone",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:        "sales",
				IfNotExists: true,
				Body:        body,
			},
			contains: []string{`CREATE SEMANTIC VIEW IF NOT EXISTS "DB"."SC".sales`},
			absent:   []string{"OR REPLACE"},
		},
		{
			name:   "case sensitive name preserved",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Body:          body,
			},
			contains: []string{`"MixedCase"`},
		},
		{
			name:   "blank body falls back to placeholder",
			db:     "DB",
			schema: "SC",
			cfg:    SemanticViewConfig{Name: "sales"},
			contains: []string{
				"TABLES (",
				"my_table AS",
			},
		},
		{
			name:     "blank name falls back to placeholder",
			db:       "DB",
			schema:   "SC",
			cfg:      SemanticViewConfig{Body: body},
			contains: []string{"semantic_view_name"},
		},
		{
			name:   "comment with single quote escaped",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:    "sales",
				Body:    body,
				Comment: "it's fine",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateSemanticViewSql(tt.db, tt.schema, tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected statement to end with ';', got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(sql, no) {
					t.Errorf("expected SQL NOT to contain %q, got:\n%s", no, sql)
				}
			}
		})
	}
}
