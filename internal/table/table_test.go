// SPDX-License-Identifier: GPL-3.0-or-later

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
		"IS_TRANSIENT",
		"IS_DYNAMIC",
		"IS_ICEBERG",
		"IS_HYBRID",
		"ORDER BY TABLE_SCHEMA, TABLE_NAME",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected %q in SQL:\n%s", want, sql)
		}
	}
	// Views and the non-base table types must not be filtered out (#908).
	if strings.Contains(sql, "WHERE") {
		t.Errorf("expected no TABLE_TYPE restriction:\n%s", sql)
	}
}

func TestParseDatabaseTableSummary(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	altered := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	res := &snowflake.QueryResult{
		Columns: []string{
			"TABLE_NAME", "TABLE_SCHEMA", "TABLE_TYPE", "ROW_COUNT", "BYTES",
			"TABLE_OWNER", "RETENTION_TIME", "CREATED", "LAST_ALTERED", "COMMENT",
			"IS_TRANSIENT", "IS_DYNAMIC", "IS_ICEBERG", "IS_HYBRID",
		},
		Rows: [][]interface{}{
			// BYTES in exponential notation, as the driver can return it for large values.
			{"T1", "PUBLIC", "BASE TABLE", int64(100), "4.096e+03", "SYSADMIN", 1, created, altered, "hi", "NO", "NO", "NO", "NO"},
			{"T2", "PUBLIC", "BASE TABLE", int64(1), int64(8), "SYSADMIN", 1, created, altered, "", "YES", "NO", "NO", "NO"},
			{"V1", "PUBLIC", "VIEW", nil, nil, nil, nil, created, altered, nil, "NO", "NO", "NO", "NO"},
			{"T3", "PUBLIC", "TEMPORARY TABLE", int64(2), int64(16), "SYSADMIN", 1, created, altered, "", "YES", "NO", "NO", "NO"},
			// A dynamic table reports TABLE_TYPE = BASE TABLE; its flag wins over IS_TRANSIENT.
			{"D1", "PUBLIC", "BASE TABLE", int64(3), int64(24), "SYSADMIN", 1, created, altered, "", "YES", "YES", "NO", "NO"},
			{"I1", "PUBLIC", "BASE TABLE", int64(4), int64(32), "SYSADMIN", 1, created, altered, "", "NO", "NO", "YES", "NO"},
			{"H1", "PUBLIC", "BASE TABLE", int64(5), int64(40), "SYSADMIN", 1, created, altered, "", "NO", "NO", "NO", "YES"},
			{"too", "short"}, // skipped: fewer than the query's 14 columns
		},
	}
	tables := ParseDatabaseTableSummary(res)
	if len(tables) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(tables))
	}
	// IS_TRANSIENT = YES is folded into Kind.
	if tables[1].Kind != "TRANSIENT" {
		t.Errorf("expected transient table kind TRANSIENT, got %q", tables[1].Kind)
	}
	// A view keeps its kind, has no counts to report, and must not render NULL
	// TABLE_OWNER / COMMENT as the literal "<nil>".
	if tables[2].Kind != "VIEW" || tables[2].Rows != nil || tables[2].Bytes != nil {
		t.Errorf("unexpected view projection: %+v", tables[2])
	}
	if tables[2].RetentionTime != nil {
		t.Errorf("expected NULL RETENTION_TIME to project as nil, got %v", *tables[2].RetentionTime)
	}
	if tables[2].Owner != "" || tables[2].Comment != "" {
		t.Errorf("expected NULL owner/comment to project as empty, got %+v", tables[2])
	}
	// A non-BASE TABLE type keeps its own kind even when IS_TRANSIENT = YES.
	if tables[3].Kind != "TEMPORARY TABLE" {
		t.Errorf("expected TEMPORARY TABLE to survive the transient fold, got %q", tables[3].Kind)
	}
	// The IS_DYNAMIC / IS_ICEBERG / IS_HYBRID flags are folded in the same way.
	for i, want := range map[int]string{4: "DYNAMIC TABLE", 5: "ICEBERG TABLE", 6: "HYBRID TABLE"} {
		if tables[i].Kind != want {
			t.Errorf("expected %s for row %d, got %q", want, i, tables[i].Kind)
		}
	}
	got := tables[0]
	if got.Name != "T1" || got.Schema != "PUBLIC" || got.Kind != "BASE TABLE" || got.Owner != "SYSADMIN" {
		t.Errorf("unexpected string projection: %+v", got)
	}
	if got.Rows == nil || *got.Rows != 100 || got.Bytes == nil || *got.Bytes != 4096 ||
		got.RetentionTime == nil || *got.RetentionTime != 1 || got.Comment != "hi" {
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
