// SPDX-License-Identifier: GPL-3.0-or-later

package icebergtable

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateIcebergTableSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      IcebergTableConfig
		contains []string
		absent   []string
	}{
		{
			name: "snowflake-managed with columns, volume, base location",
			cfg: IcebergTableConfig{
				Name:           "SALES",
				IfNotExists:    true,
				TableType:      TableTypeSnowflake,
				Columns:        []IcebergColumn{{Name: "ID", Type: "NUMBER"}, {Name: "TS", Type: "TIMESTAMP_NTZ"}},
				ExternalVolume: "MY_VOL",
				BaseLocation:   "sales/data",
				ClusterBy:      "ID",
				Comment:        "iceberg sales",
				Tags:           []snowflake.TagPair{{Name: "env", Value: "prod"}},
			},
			contains: []string{
				"CREATE ICEBERG TABLE IF NOT EXISTS \"DB\".\"SC\".SALES",
				"ID NUMBER",
				"TS TIMESTAMP_NTZ",
				"EXTERNAL_VOLUME = 'MY_VOL'",
				"CATALOG = 'SNOWFLAKE'",
				"BASE_LOCATION = 'sales/data'",
				"CLUSTER BY (ID)",
				"COMMENT = 'iceberg sales'",
				"TAG (\"env\" = 'prod')",
			},
			absent: []string{"OR REPLACE", "CATALOG_TABLE_NAME", "AUTO_REFRESH", "REPLACE_INVALID_CHARACTERS", "METADATA_FILE_PATH"},
		},
		{
			name: "external catalog (REST / AWS Glue)",
			cfg: IcebergTableConfig{
				Name:                     "GLUE_TBL",
				TableType:                TableTypeExternalCatalog,
				ExternalVolume:           "EXT_VOL",
				Catalog:                  "GLUE_CATALOG",
				CatalogTableName:         "orders",
				CatalogNamespace:         "analytics",
				ReplaceInvalidCharacters: "TRUE",
				AutoRefresh:              "true",
			},
			contains: []string{
				"CREATE ICEBERG TABLE \"DB\".\"SC\".GLUE_TBL",
				"EXTERNAL_VOLUME = 'EXT_VOL'",
				"CATALOG = 'GLUE_CATALOG'",
				"CATALOG_TABLE_NAME = 'orders'",
				"CATALOG_NAMESPACE = 'analytics'",
				"REPLACE_INVALID_CHARACTERS = TRUE",
				"AUTO_REFRESH = TRUE",
			},
			// External catalog tables infer columns; no column list, no
			// BASE_LOCATION / METADATA_FILE_PATH, no Snowflake catalog literal.
			absent: []string{"BASE_LOCATION", "CATALOG = 'SNOWFLAKE'", "<column> <type>", "METADATA_FILE_PATH"},
		},
		{
			name: "delta lake files require base location, no catalog table name",
			cfg: IcebergTableConfig{
				Name:           "DELTA_TBL",
				TableType:      TableTypeDelta,
				ExternalVolume: "EXT_VOL",
				Catalog:        "DELTA_CATALOG",
				BaseLocation:   "delta/orders",
				AutoRefresh:    "TRUE",
			},
			contains: []string{
				"CREATE ICEBERG TABLE \"DB\".\"SC\".DELTA_TBL",
				"CATALOG = 'DELTA_CATALOG'",
				"BASE_LOCATION = 'delta/orders'",
				"AUTO_REFRESH = TRUE",
			},
			absent: []string{"CATALOG_TABLE_NAME", "METADATA_FILE_PATH", "CATALOG = 'SNOWFLAKE'", "<column> <type>"},
		},
		{
			name: "iceberg files require metadata file path, no auto refresh",
			cfg: IcebergTableConfig{
				Name:                     "FILES_TBL",
				TableType:                TableTypeIcebergFiles,
				Catalog:                  "OBJ_CATALOG",
				MetadataFilePath:         "path/to/metadata/v1.metadata.json",
				ReplaceInvalidCharacters: "TRUE",
				AutoRefresh:              "TRUE", // must NOT be emitted for this variant
			},
			contains: []string{
				"CATALOG = 'OBJ_CATALOG'",
				"METADATA_FILE_PATH = 'path/to/metadata/v1.metadata.json'",
				"REPLACE_INVALID_CHARACTERS = TRUE",
			},
			absent: []string{"AUTO_REFRESH", "BASE_LOCATION", "CATALOG_TABLE_NAME", "CATALOG = 'SNOWFLAKE'"},
		},
		{
			name: "external catalog omits CATALOG when not set (uses default)",
			cfg: IcebergTableConfig{
				Name:             "NOCAT",
				TableType:        TableTypeExternalCatalog,
				CatalogTableName: "t",
			},
			contains: []string{"CATALOG_TABLE_NAME = 't'"},
			absent:   []string{"CATALOG ="},
		},
		{
			name: "default table type is snowflake-managed",
			cfg: IcebergTableConfig{
				Name:         "DEF",
				BaseLocation: "d/loc",
				Columns:      []IcebergColumn{{Name: "C1", Type: "VARCHAR"}},
			},
			contains: []string{"CATALOG = 'SNOWFLAKE'", "BASE_LOCATION = 'd/loc'"},
			absent:   []string{"CATALOG_TABLE_NAME"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: IcebergTableConfig{
				Name:         "R",
				OrReplace:    true,
				IfNotExists:  true,
				BaseLocation: "r/loc",
				Columns:      []IcebergColumn{{Name: "C1", Type: "INT"}},
			},
			contains: []string{"CREATE OR REPLACE ICEBERG TABLE \"DB\".\"SC\".R"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "managed without columns emits placeholder",
			cfg: IcebergTableConfig{
				Name: "EMPTY",
			},
			contains: []string{"<column> <type>", "BASE_LOCATION = '<base_location>'"},
		},
		{
			name: "external catalog without table name emits placeholder",
			cfg: IcebergTableConfig{
				Name:      "X",
				TableType: TableTypeExternalCatalog,
			},
			contains: []string{"CATALOG_TABLE_NAME = '<catalog_table_name>'"},
		},
		{
			name: "case sensitive name, escaped literals",
			cfg: IcebergTableConfig{
				Name:          "Mixed",
				CaseSensitive: true,
				BaseLocation:  "a'b",
				Comment:       "it's mine",
				Columns:       []IcebergColumn{{Name: "C", Type: "INT"}},
			},
			contains: []string{
				"\"Mixed\"",
				"BASE_LOCATION = 'a''b'",
				"COMMENT = 'it''s mine'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateIcebergTableSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(got, ";") {
				t.Errorf("statement should end with ';', got:\n%s", got)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, got)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(got, bad) {
					t.Errorf("expected SQL NOT to contain %q, got:\n%s", bad, got)
				}
			}
		})
	}
}
