// SPDX-License-Identifier: GPL-3.0-or-later

package warehouse

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildAlterWarehousePropertySQL(t *testing.T) {
	tests := []struct {
		name     string
		property string
		value    string
		want     string
		wantErr  bool
	}{
		{name: "size valid", property: "size", value: "small", want: `ALTER WAREHOUSE "WH" SET WAREHOUSE_SIZE = SMALL`},
		{name: "size invalid", property: "size", value: "HUGE", wantErr: true},
		{name: "autoSuspend zero is null", property: "autoSuspend", value: "0", want: `ALTER WAREHOUSE "WH" SET AUTO_SUSPEND = NULL`},
		{name: "autoSuspend int", property: "autoSuspend", value: "300", want: `ALTER WAREHOUSE "WH" SET AUTO_SUSPEND = 300`},
		{name: "autoSuspend negative", property: "autoSuspend", value: "-5", wantErr: true},
		{name: "autoResume valid", property: "autoResume", value: "true", want: `ALTER WAREHOUSE "WH" SET AUTO_RESUME = TRUE`},
		{name: "autoResume invalid", property: "autoResume", value: "maybe", wantErr: true},
		{name: "comment escapes quotes", property: "comment", value: "it's fine", want: `ALTER WAREHOUSE "WH" SET COMMENT = 'it''s fine'`},
		{name: "comment escapes trailing backslash", property: "comment", value: `path\`, want: `ALTER WAREHOUSE "WH" SET COMMENT = 'path\\'`},
		{name: "scalingPolicy valid", property: "scalingPolicy", value: "economy", want: `ALTER WAREHOUSE "WH" SET SCALING_POLICY = ECONOMY`},
		{name: "resourceMonitor empty is null", property: "resourceMonitor", value: "", want: `ALTER WAREHOUSE "WH" SET RESOURCE_MONITOR = NULL`},
		{name: "resourceMonitor named", property: "resourceMonitor", value: "RM1", want: `ALTER WAREHOUSE "WH" SET RESOURCE_MONITOR = "RM1"`},
		{name: "maxClusterCount int", property: "maxClusterCount", value: "3", want: `ALTER WAREHOUSE "WH" SET MAX_CLUSTER_COUNT = 3`},
		{name: "unknown property", property: "bogus", value: "x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildAlterWarehousePropertySQL("WH", tt.property, tt.value)
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

func TestBuildMeteringHistoryQuery(t *testing.T) {
	sql := BuildMeteringHistoryQuery("WH1", "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z")
	for _, want := range []string{
		"WAREHOUSE_NAME = 'WH1'",
		"START_TIME >= '2026-01-01T00:00:00Z'::TIMESTAMP_LTZ",
		"START_TIME < '2026-02-01T00:00:00Z'::TIMESTAMP_LTZ",
		"WAREHOUSE_METERING_HISTORY",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected %q in SQL:\n%s", want, sql)
		}
	}

	noFilter := BuildMeteringHistoryQuery("", "", "")
	if strings.Contains(noFilter, "WHERE") {
		t.Errorf("expected no WHERE clause when unfiltered:\n%s", noFilter)
	}
}

func TestBuildMeteringHistoryQueryEscapesWarehouse(t *testing.T) {
	sql := BuildMeteringHistoryQuery("O'Brien", "", "")
	if !strings.Contains(sql, "WAREHOUSE_NAME = 'O''Brien'") {
		t.Errorf("expected escaped warehouse name in SQL:\n%s", sql)
	}
}

func TestParseMeteringHistory(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{
			"START_TIME", "END_TIME", "WAREHOUSE_NAME",
			"CREDITS_USED", "CREDITS_USED_COMPUTE", "CREDITS_USED_CLOUD_SERVICES",
		},
		Rows: [][]interface{}{
			{"2026-01-01T00:00:00Z", "2026-01-01T01:00:00Z", "WH1", 1.5, 1.2, 0.3},
		},
	}
	rows := ParseMeteringHistory(res)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.WarehouseName != "WH1" || got.CreditsUsed != 1.5 || got.CreditsUsedCompute != 1.2 || got.CreditsUsedCloudServices != 0.3 {
		t.Errorf("unexpected projection: %+v", got)
	}
}

func TestParseMeteringHistoryNil(t *testing.T) {
	if rows := ParseMeteringHistory(nil); len(rows) != 0 {
		t.Errorf("expected empty slice for nil result, got %d", len(rows))
	}
}
