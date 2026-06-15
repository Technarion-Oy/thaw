// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package streamlit

import (
	"strings"
	"testing"
)

func TestBuildCreateStreamlitSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      StreamlitConfig
		contains []string
		absent   []string
	}{
		{
			name: "full app with all properties",
			cfg: StreamlitConfig{
				Name:                       "MY_APP",
				IfNotExists:                true,
				StageLocation:              "@db.sc.app_stage/dashboard",
				MainFile:                   "main.py",
				QueryWarehouse:             "MY_WH",
				ExternalAccessIntegrations: "EAI_ONE, EAI_TWO",
				Title:                      "My Dashboard",
				Comment:                    "sales app",
			},
			contains: []string{
				"CREATE STREAMLIT IF NOT EXISTS \"DB\".\"SC\".MY_APP",
				"FROM @db.sc.app_stage/dashboard",
				"MAIN_FILE = 'main.py'",
				"QUERY_WAREHOUSE = \"MY_WH\"",
				"EXTERNAL_ACCESS_INTEGRATIONS = (\"EAI_ONE\", \"EAI_TWO\")",
				"TITLE = 'My Dashboard'",
				"COMMENT = 'sales app'",
			},
			absent: []string{"ROOT_LOCATION", "OR REPLACE"},
		},
		{
			name: "minimal app supplies main-file default and leading @",
			cfg: StreamlitConfig{
				Name:          "APP2",
				StageLocation: "db.sc.stg",
			},
			contains: []string{
				"CREATE STREAMLIT \"DB\".\"SC\".APP2",
				"FROM @db.sc.stg",
				"MAIN_FILE = 'streamlit_app.py'",
			},
			absent: []string{
				"IF NOT EXISTS", "QUERY_WAREHOUSE", "TITLE =", "COMMENT =",
				"EXTERNAL_ACCESS_INTEGRATIONS",
			},
		},
		{
			name: "or replace wins over if not exists",
			cfg: StreamlitConfig{
				Name:          "APP3",
				OrReplace:     true,
				IfNotExists:   true,
				StageLocation: "@stg",
				MainFile:      "app.py",
			},
			contains: []string{"CREATE OR REPLACE STREAMLIT \"DB\".\"SC\".APP3"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "case sensitive name and escaped literals",
			cfg: StreamlitConfig{
				Name:          "Mixed",
				CaseSensitive: true,
				StageLocation: "@stg",
				MainFile:      "a'b.py",
				Title:         "It's mine",
				Comment:       "it's a comment",
			},
			contains: []string{
				"\"Mixed\"",
				"MAIN_FILE = 'a''b.py'",
				"TITLE = 'It''s mine'",
				"COMMENT = 'it''s a comment'",
			},
		},
		{
			name: "empty stage yields placeholder",
			cfg: StreamlitConfig{
				Name: "APP4",
			},
			contains: []string{"FROM @<stage>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateStreamlitSql("DB", "SC", tt.cfg)
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
