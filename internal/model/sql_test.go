// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"strings"
	"testing"
)

func TestBuildCreateModelSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ModelConfig
		contains []string
		absent   []string
	}{
		{
			name: "copy from model, default source version",
			cfg: ModelConfig{
				Name:        "MY_MODEL",
				SourceType:  SourceModel,
				SourceModel: "DB2.SC2.SRC_MODEL",
			},
			contains: []string{
				"CREATE MODEL \"DB\".\"SC\".MY_MODEL",
				"FROM MODEL DB2.SC2.SRC_MODEL",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "WITH VERSION", "VERSION ", "FROM @"},
		},
		{
			name: "copy from model with version and WITH VERSION",
			cfg: ModelConfig{
				Name:          "MY_MODEL",
				OrReplace:     true,
				VersionName:   "V2",
				SourceType:    SourceModel,
				SourceModel:   "SRC_MODEL",
				SourceVersion: "V1",
			},
			contains: []string{
				"CREATE OR REPLACE MODEL \"DB\".\"SC\".MY_MODEL WITH VERSION V2",
				"FROM MODEL SRC_MODEL VERSION V1",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "from internal stage",
			cfg: ModelConfig{
				Name:          "STAGED",
				IfNotExists:   true,
				SourceType:    SourceStage,
				StageLocation: "@models/my_model",
			},
			contains: []string{
				"CREATE MODEL IF NOT EXISTS \"DB\".\"SC\".STAGED",
				"FROM @models/my_model",
			},
			absent: []string{"OR REPLACE", "FROM MODEL"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: ModelConfig{
				Name:        "BOTH",
				OrReplace:   true,
				IfNotExists: true,
				SourceType:  SourceModel,
				SourceModel: "SRC",
			},
			contains: []string{"CREATE OR REPLACE MODEL \"DB\".\"SC\".BOTH"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: ModelConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				SourceType:    SourceModel,
				SourceModel:   "SRC",
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name: "blank name and blank source render placeholders",
			cfg:  ModelConfig{SourceType: SourceModel},
			contains: []string{
				"CREATE MODEL \"DB\".\"SC\".model_name",
				"FROM MODEL source_model_name",
			},
		},
		{
			name: "blank stage location renders placeholder",
			cfg:  ModelConfig{Name: "S", SourceType: SourceStage},
			contains: []string{
				"FROM @my_stage/model_path",
			},
		},
		{
			name: "empty source type defaults to model copy",
			cfg:  ModelConfig{Name: "X"},
			contains: []string{
				"FROM MODEL source_model_name",
			},
			absent: []string{"FROM @"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateModelSql("DB", "SC", tt.cfg)
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
