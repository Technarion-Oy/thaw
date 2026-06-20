// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package packagespolicy

import (
	"strings"
	"testing"
)

func TestBuildCreatePackagesPolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      PackagesPolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config",
			cfg: PackagesPolicyConfig{
				Name:                        "PKG_GOVERNANCE",
				OrReplace:                   true,
				Allowlist:                   []string{"numpy", "pandas==2.*"},
				Blocklist:                   []string{"requests"},
				AdditionalCreationBlocklist: []string{"scipy"},
				Comment:                     "governance policy",
			},
			contains: []string{
				`CREATE OR REPLACE PACKAGES POLICY "DB"."SC".PKG_GOVERNANCE`,
				"LANGUAGE PYTHON",
				"ALLOWLIST = ('numpy', 'pandas==2.*')",
				"BLOCKLIST = ('requests')",
				"ADDITIONAL_CREATION_BLOCKLIST = ('scipy')",
				"COMMENT = 'governance policy'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "minimal config emits only LANGUAGE PYTHON",
			cfg: PackagesPolicyConfig{
				Name: "DEFAULTS",
			},
			contains: []string{
				`CREATE PACKAGES POLICY "DB"."SC".DEFAULTS`,
				"LANGUAGE PYTHON",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "ALLOWLIST", "BLOCKLIST", "ADDITIONAL_CREATION_BLOCKLIST", "COMMENT ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: PackagesPolicyConfig{
				Name:        "A",
				IfNotExists: true,
			},
			contains: []string{"CREATE PACKAGES POLICY IF NOT EXISTS"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "or replace and if not exists are mutually exclusive",
			cfg: PackagesPolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE PACKAGES POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "blank tokens are skipped and a wildcard allowlist is emitted",
			cfg: PackagesPolicyConfig{
				Name:      "WILD",
				Allowlist: []string{"*", "  ", ""},
			},
			contains: []string{"ALLOWLIST = ('*')"},
		},
		{
			name: "all-blank list is omitted entirely",
			cfg: PackagesPolicyConfig{
				Name:      "EMPTY",
				Blocklist: []string{"", "   "},
			},
			absent: []string{"BLOCKLIST"},
		},
		{
			name: "comment is escaped",
			cfg: PackagesPolicyConfig{
				Name:    "Q",
				Comment: "it's strict",
			},
			contains: []string{"COMMENT = 'it''s strict'"},
		},
		{
			name:     "blank name yields placeholder",
			cfg:      PackagesPolicyConfig{},
			contains: []string{"packages_policy_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreatePackagesPolicySql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected SQL to contain %q\ngot:\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("expected SQL to NOT contain %q\ngot:\n%s", no, got)
				}
			}
		})
	}
}

func TestFormatStringList(t *testing.T) {
	got := FormatStringList([]string{"numpy", "", "pandas"})
	want := "('numpy', 'pandas')"
	if got != want {
		t.Errorf("FormatStringList = %q, want %q", got, want)
	}
}
