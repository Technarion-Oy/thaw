// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package table

import (
	"strings"
	"testing"
	"time"

	"thaw/internal/snowflake"
)

func TestBuildAlterTablePropertySQL(t *testing.T) {
	tests := []struct {
		name     string
		property string
		value    string
		want     string
		wantErr  bool
	}{
		{name: "clusterBy set", property: "clusterBy", value: "C1, C2", want: `ALTER TABLE "DB"."SC"."T" CLUSTER BY (C1, C2)`},
		{name: "clusterBy empty drops", property: "clusterBy", value: "", want: `ALTER TABLE "DB"."SC"."T" DROP CLUSTERING KEY`},
		{name: "enableSchemaEvolution", property: "enableSchemaEvolution", value: "true", want: `ALTER TABLE "DB"."SC"."T" SET ENABLE_SCHEMA_EVOLUTION = TRUE`},
		{name: "dataRetentionDays", property: "dataRetentionDays", value: "7", want: `ALTER TABLE "DB"."SC"."T" SET DATA_RETENTION_TIME_IN_DAYS = 7`},
		{name: "maxDataExtensionDays", property: "maxDataExtensionDays", value: "14", want: `ALTER TABLE "DB"."SC"."T" SET MAX_DATA_EXTENSION_TIME_IN_DAYS = 14`},
		{name: "changeTracking", property: "changeTracking", value: "false", want: `ALTER TABLE "DB"."SC"."T" SET CHANGE_TRACKING = FALSE`},
		{name: "defaultDDLCollation escapes", property: "defaultDDLCollation", value: "en's", want: `ALTER TABLE "DB"."SC"."T" SET DEFAULT_DDL_COLLATION = 'en''s'`},
		{name: "comment escapes", property: "comment", value: "a'b", want: `ALTER TABLE "DB"."SC"."T" SET COMMENT = 'a''b'`},
		{name: "unknown property", property: "bogus", value: "x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildAlterTablePropertySQL("DB", "SC", "T", tt.property, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got SQL %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDatabaseTableSummaryQuery(t *testing.T) {
	sql := BuildDatabaseTableSummaryQuery("MYDB")
	for _, want := range []string{
		`"MYDB".INFORMATION_SCHEMA.TABLES`,
		"TABLE_TYPE IN ('BASE TABLE', 'TRANSIENT', 'TEMPORARY')",
		"ORDER BY TABLE_SCHEMA, TABLE_NAME",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected %q in SQL:\n%s", want, sql)
		}
	}
}

func TestParseDatabaseTableSummary(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	altered := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	res := &snowflake.QueryResult{
		Columns: []string{
			"TABLE_NAME", "TABLE_SCHEMA", "TABLE_TYPE", "ROW_COUNT", "BYTES",
			"TABLE_OWNER", "RETENTION_TIME", "CREATED", "LAST_ALTERED", "COMMENT",
		},
		Rows: [][]interface{}{
			{"T1", "PUBLIC", "BASE TABLE", int64(100), int64(4096), "SYSADMIN", 1, created, altered, "hi"},
			{"too", "short"}, // skipped: < 10 columns
		},
	}
	tables := ParseDatabaseTableSummary(res)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	got := tables[0]
	if got.Name != "T1" || got.Schema != "PUBLIC" || got.Kind != "BASE TABLE" || got.Owner != "SYSADMIN" {
		t.Errorf("unexpected string projection: %+v", got)
	}
	if got.Rows != 100 || got.Bytes != 4096 || got.RetentionTime != 1 || got.Comment != "hi" {
		t.Errorf("unexpected numeric projection: %+v", got)
	}
	if got.Created != created.Format(time.RFC3339) || got.LastAltered != altered.Format(time.RFC3339) {
		t.Errorf("unexpected time projection: %+v", got)
	}
}

func TestParseDatabaseTableSummaryNil(t *testing.T) {
	if tables := ParseDatabaseTableSummary(nil); tables != nil {
		t.Errorf("expected nil for nil result, got %v", tables)
	}
}
