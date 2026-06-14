// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package networkrule

import (
	"strings"
	"testing"
)

func TestBuildCreateNetworkRuleSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      NetworkRuleConfig
		contains []string
		absent   []string
	}{
		{
			name: "full egress host_port config",
			cfg: NetworkRuleConfig{
				Name:      "EXT_ACCESS",
				OrReplace: true,
				Type:      "HOST_PORT",
				Mode:      "EGRESS",
				ValueList: []string{"example.com:443", "api.example.com:443"},
				Comment:   "outbound API access",
			},
			contains: []string{
				"CREATE OR REPLACE NETWORK RULE \"DB\".\"SC\".EXT_ACCESS",
				"TYPE = HOST_PORT",
				"VALUE_LIST = ('example.com:443', 'api.example.com:443')",
				"MODE = EGRESS",
				"COMMENT = 'outbound API access'",
			},
		},
		{
			name: "ipv4 ingress",
			cfg: NetworkRuleConfig{
				Name:      "ALLOW_OFFICE",
				Type:      "IPV4",
				Mode:      "INGRESS",
				ValueList: []string{"192.168.1.0/24"},
			},
			contains: []string{
				"CREATE NETWORK RULE",
				"TYPE = IPV4",
				"VALUE_LIST = ('192.168.1.0/24')",
				"MODE = INGRESS",
			},
			absent: []string{"OR REPLACE", "COMMENT"},
		},
		{
			name: "blank value entries are skipped",
			cfg: NetworkRuleConfig{
				Name:      "A",
				Type:      "IPV4",
				Mode:      "INGRESS",
				ValueList: []string{" ", "10.0.0.0/8", ""},
			},
			contains: []string{"VALUE_LIST = ('10.0.0.0/8')"},
		},
		{
			name: "empty value list renders as ()",
			cfg: NetworkRuleConfig{
				Name: "A",
				Type: "IPV4",
				Mode: "INGRESS",
			},
			contains: []string{"VALUE_LIST = ()"},
		},
		{
			name: "single quotes escaped in value and comment",
			cfg: NetworkRuleConfig{
				Name:      "A",
				Type:      "HOST_PORT",
				Mode:      "EGRESS",
				ValueList: []string{"o'hare.example.com:443"},
				Comment:   "o'hare",
			},
			contains: []string{
				"VALUE_LIST = ('o''hare.example.com:443')",
				"COMMENT = 'o''hare'",
			},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: NetworkRuleConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Type:          "IPV4",
				Mode:          "INGRESS",
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateNetworkRuleSql("DB", "SC", tt.cfg)
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

// TestBuildCreateNetworkRuleSqlPlaceholders verifies that an empty config still
// yields a well-formed, completable template (placeholder name, default TYPE and
// MODE, empty VALUE_LIST) rather than invalid SQL.
func TestBuildCreateNetworkRuleSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateNetworkRuleSql("DB", "SC", NetworkRuleConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"network_rule_name", "TYPE = IPV4", "VALUE_LIST = ()", "MODE = INGRESS"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
