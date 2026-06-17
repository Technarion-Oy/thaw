// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package eventtable

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateEventTableSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      EventTableConfig
		contains []string
		absent   []string
	}{
		{
			name: "minimal",
			cfg: EventTableConfig{
				Name: "EVENTS",
			},
			contains: []string{`CREATE EVENT TABLE "DB"."SC".EVENTS;`},
			absent: []string{
				"OR REPLACE", "IF NOT EXISTS", "DATA_RETENTION_TIME_IN_DAYS",
				"CHANGE_TRACKING", "COPY GRANTS", "COMMENT", "TAG (",
			},
		},
		{
			name: "all properties",
			cfg: EventTableConfig{
				Name:                       "TELEMETRY",
				IfNotExists:                true,
				DataRetentionTimeInDays:    "30",
				MaxDataExtensionTimeInDays: "14",
				ChangeTracking:             "true",
				DefaultDdlCollation:        "en-ci",
				CopyGrants:                 true,
				Comment:                    "telemetry data",
				Tags:                       []snowflake.TagPair{{Name: "env", Value: "prod"}},
			},
			contains: []string{
				`CREATE EVENT TABLE IF NOT EXISTS "DB"."SC".TELEMETRY`,
				"DATA_RETENTION_TIME_IN_DAYS = 30",
				"MAX_DATA_EXTENSION_TIME_IN_DAYS = 14",
				"CHANGE_TRACKING = TRUE",
				"DEFAULT_DDL_COLLATION = 'en-ci'",
				"COPY GRANTS",
				"COMMENT = 'telemetry data'",
				`TAG ("env" = 'prod')`,
			},
			absent: []string{"OR REPLACE"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: EventTableConfig{
				Name:        "R",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{`CREATE OR REPLACE EVENT TABLE "DB"."SC".R`},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "no column list ever emitted (fixed schema)",
			cfg: EventTableConfig{
				Name:    "FIXED",
				Comment: "fixed schema",
			},
			// No CLUSTER BY here → no parens at all, and never a column list.
			absent: []string{"(", "CLUSTER BY", "<column>"},
		},
		{
			name: "cluster by on predefined columns",
			cfg: EventTableConfig{
				Name:      "CLUSTERED",
				ClusterBy: "timestamp",
			},
			contains: []string{
				`CREATE EVENT TABLE "DB"."SC".CLUSTERED`,
				"CLUSTER BY (timestamp)",
			},
		},
		{
			name:     "empty name emits placeholder",
			cfg:      EventTableConfig{},
			contains: []string{`"DB"."SC".event_table_name`},
		},
		{
			name: "case sensitive name, escaped literals",
			cfg: EventTableConfig{
				Name:          "Mixed",
				CaseSensitive: true,
				Comment:       "it's mine",
			},
			contains: []string{`"Mixed"`, "COMMENT = 'it''s mine'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateEventTableSql("DB", "SC", tt.cfg)
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
