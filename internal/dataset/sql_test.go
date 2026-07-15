// SPDX-License-Identifier: GPL-3.0-or-later

package dataset

import (
	"strings"
	"testing"
)

func TestBuildCreateDatasetSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      DatasetConfig
		contains []string
		absent   []string
	}{
		{
			name:     "plain",
			cfg:      DatasetConfig{Name: "MY_DATASET"},
			contains: []string{"CREATE DATASET \"DB\".\"SC\".MY_DATASET;"},
			absent:   []string{"OR REPLACE", "IF NOT EXISTS"},
		},
		{
			name:     "if not exists",
			cfg:      DatasetConfig{Name: "MY_DATASET", IfNotExists: true},
			contains: []string{"CREATE DATASET IF NOT EXISTS \"DB\".\"SC\".MY_DATASET;"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name:     "or replace",
			cfg:      DatasetConfig{Name: "MY_DATASET", OrReplace: true},
			contains: []string{"CREATE OR REPLACE DATASET \"DB\".\"SC\".MY_DATASET;"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name:     "or replace wins over if not exists",
			cfg:      DatasetConfig{Name: "BOTH", OrReplace: true, IfNotExists: true},
			contains: []string{"CREATE OR REPLACE DATASET \"DB\".\"SC\".BOTH;"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name:     "case-sensitive name is quoted",
			cfg:      DatasetConfig{Name: "MixedCase", CaseSensitive: true},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name:     "blank name renders placeholder",
			cfg:      DatasetConfig{},
			contains: []string{"CREATE DATASET \"DB\".\"SC\".dataset_name;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateDatasetSql("DB", "SC", tt.cfg)
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
