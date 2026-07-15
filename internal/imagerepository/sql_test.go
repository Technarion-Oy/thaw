// SPDX-License-Identifier: GPL-3.0-or-later

package imagerepository

import (
	"strings"
	"testing"
)

func TestBuildCreateImageRepositorySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ImageRepositoryConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config with or replace and comment",
			cfg: ImageRepositoryConfig{
				Name:      "MY_REPO",
				OrReplace: true,
				Comment:   "container images for SPCS",
			},
			contains: []string{
				"CREATE OR REPLACE IMAGE REPOSITORY \"DB\".\"SC\".MY_REPO",
				"COMMENT = 'container images for SPCS'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists, no comment",
			cfg: ImageRepositoryConfig{
				Name:        "TOOLING",
				IfNotExists: true,
			},
			contains: []string{
				"CREATE IMAGE REPOSITORY IF NOT EXISTS \"DB\".\"SC\".TOOLING",
			},
			absent: []string{"OR REPLACE", "COMMENT"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: ImageRepositoryConfig{
				Name:        "BOTH",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE IMAGE REPOSITORY \"DB\".\"SC\".BOTH"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: ImageRepositoryConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name: "blank name renders placeholder",
			cfg:  ImageRepositoryConfig{},
			contains: []string{
				"CREATE IMAGE REPOSITORY \"DB\".\"SC\".image_repository_name",
			},
		},
		{
			name: "comment with single quote is escaped",
			cfg: ImageRepositoryConfig{
				Name:    "R",
				Comment: "it's mine",
			},
			contains: []string{"COMMENT = 'it''s mine'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateImageRepositorySql("DB", "SC", tt.cfg)
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
