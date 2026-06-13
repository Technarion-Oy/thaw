// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package externaltable

import (
	"strings"
	"testing"
)

func TestBuildCreateExternalTableSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ExternalTableConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config with partitioned columns",
			cfg: ExternalTableConfig{
				Name:      "MY_EXT",
				OrReplace: true,
				Columns: []ExternalTableColumn{
					{Name: "date_part", Type: "DATE", Expression: "to_date(metadata$filename)", Partition: true},
					{Name: "c1", Type: "VARCHAR", Expression: "value:c1::varchar"},
				},
				Location:        "@my_stage/data/",
				RefreshOnCreate: "true",
				AutoRefresh:     "false",
				Pattern:         ".*[.]parquet",
				FileFormatName:  "my_parquet_fmt",
				Comment:         "lake table",
			},
			contains: []string{
				"CREATE OR REPLACE EXTERNAL TABLE \"DB\".\"SC\".MY_EXT (",
				"date_part DATE AS (to_date(metadata$filename))",
				"c1 VARCHAR AS (value:c1::varchar)",
				"PARTITION BY (date_part)",
				"LOCATION = @my_stage/data/",
				"REFRESH_ON_CREATE = TRUE",
				"AUTO_REFRESH = FALSE",
				"PATTERN = '.*[.]parquet'",
				"FILE_FORMAT = (FORMAT_NAME = 'my_parquet_fmt')",
				"COMMENT = 'lake table'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "inline file format type when no named format",
			cfg: ExternalTableConfig{
				Name:           "ET",
				Location:       "@s/p",
				FileFormatType: "json",
			},
			contains: []string{"FILE_FORMAT = (TYPE = JSON)"},
			absent:   []string{"FORMAT_NAME"},
		},
		{
			name: "explicit inline type is emitted",
			cfg: ExternalTableConfig{
				Name:           "ET",
				Location:       "@s/p",
				FileFormatType: "CSV",
			},
			contains: []string{"FILE_FORMAT = (TYPE = CSV)"},
		},
		{
			name: "no name and no type emits a FORMAT_NAME placeholder",
			cfg: ExternalTableConfig{
				Name:     "ET",
				Location: "@s/p",
			},
			contains: []string{"FILE_FORMAT = (FORMAT_NAME = '<file_format>')"},
			absent:   []string{"TYPE ="},
		},
		{
			name: "if not exists wins when not or replace",
			cfg: ExternalTableConfig{
				Name:        "ET",
				IfNotExists: true,
				Location:    "@s/p",
			},
			contains: []string{"CREATE EXTERNAL TABLE IF NOT EXISTS"},
		},
		{
			name: "named format takes precedence over type",
			cfg: ExternalTableConfig{
				Name:           "ET",
				Location:       "@s/p",
				FileFormatName: "fmt",
				FileFormatType: "PARQUET",
			},
			contains: []string{"FILE_FORMAT = (FORMAT_NAME = 'fmt')"},
			absent:   []string{"TYPE = PARQUET"},
		},
		{
			name: "comment, sns topic, copy grants and tags escape quotes",
			cfg: ExternalTableConfig{
				Name:        "ET",
				Location:    "@s/p",
				AwsSnsTopic: "arn:aws:sns:topic",
				CopyGrants:  true,
				Comment:     "it's fine",
				Tags:        []TagPair{{Name: "env", Value: "prod"}, {Name: "team", Value: "data's"}},
			},
			contains: []string{
				"AWS_SNS_TOPIC = 'arn:aws:sns:topic'",
				"COPY GRANTS",
				"COMMENT = 'it''s fine'",
				"TAG (\"env\" = 'prod', \"team\" = 'data''s')",
			},
		},
		{
			name: "no columns omits parens and partition by",
			cfg: ExternalTableConfig{
				Name:     "ET",
				Location: "@s/p",
				Tags:     []TagPair{{Name: "  ", Value: "ignored"}},
			},
			absent: []string{"PARTITION BY", "TAG (", "(\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateExternalTableSql("DB", "SC", tt.cfg)
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

func TestBuildCreateExternalTableSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateExternalTableSql("DB", "SC", ExternalTableConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"external_table_name", "LOCATION = @<stage>/<path>", "FILE_FORMAT = (FORMAT_NAME = '<file_format>')"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}
