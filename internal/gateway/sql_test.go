// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"strings"
	"testing"
)

const sampleSpec = `spec:
  type: traffic_split
  split_type: custom
  targets:
  - type: endpoint
    value: db.sc.svc!api
    weight: 100`

func TestBuildCreateGatewaySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      GatewayConfig
		contains []string
		absent   []string
	}{
		{
			name:     "plain with spec",
			cfg:      GatewayConfig{Name: "MY_GATEWAY", Specification: sampleSpec},
			contains: []string{"CREATE GATEWAY \"DB\".\"SC\".MY_GATEWAY", "FROM SPECIFICATION", "$THAW$", "traffic_split", "db.sc.svc!api"},
			absent:   []string{"OR REPLACE", "IF NOT EXISTS"},
		},
		{
			name:     "if not exists",
			cfg:      GatewayConfig{Name: "MY_GATEWAY", IfNotExists: true, Specification: sampleSpec},
			contains: []string{"CREATE GATEWAY IF NOT EXISTS \"DB\".\"SC\".MY_GATEWAY"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name:     "or replace",
			cfg:      GatewayConfig{Name: "MY_GATEWAY", OrReplace: true, Specification: sampleSpec},
			contains: []string{"CREATE OR REPLACE GATEWAY \"DB\".\"SC\".MY_GATEWAY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name:     "or replace wins over if not exists",
			cfg:      GatewayConfig{Name: "BOTH", OrReplace: true, IfNotExists: true, Specification: sampleSpec},
			contains: []string{"CREATE OR REPLACE GATEWAY \"DB\".\"SC\".BOTH"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name:     "case-sensitive name is quoted",
			cfg:      GatewayConfig{Name: "MixedCase", CaseSensitive: true, Specification: sampleSpec},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name:     "blank name and spec render placeholders",
			cfg:      GatewayConfig{},
			contains: []string{"CREATE GATEWAY \"DB\".\"SC\".gateway_name", "traffic_split"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateGatewaySql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected trailing semicolon, got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(sql, bad) {
					t.Errorf("expected SQL to NOT contain %q, got:\n%s", bad, sql)
				}
			}
		})
	}
}

func TestBuildAlterGatewaySpecSql(t *testing.T) {
	sql := BuildAlterGatewaySpecSql("DB", "SC", "MY_GATEWAY", sampleSpec)

	for _, want := range []string{
		"ALTER GATEWAY \"DB\".\"SC\".\"MY_GATEWAY\"",
		"FROM SPECIFICATION",
		"$THAW$",
		"traffic_split",
		"db.sc.svc!api",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
	if !strings.HasSuffix(sql, ";") {
		t.Errorf("expected trailing semicolon, got:\n%s", sql)
	}

	// A blank spec still produces a valid, completable statement.
	blank := BuildAlterGatewaySpecSql("DB", "SC", "G", "")
	if !strings.Contains(blank, "traffic_split") {
		t.Errorf("expected placeholder spec in blank ALTER, got:\n%s", blank)
	}
}
