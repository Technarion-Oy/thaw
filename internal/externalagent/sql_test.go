// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package externalagent

import "testing"

func TestBuildCreateExternalAgentSql(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ExternalAgentConfig
		want    string
		wantErr bool
	}{
		{
			name: "minimal",
			cfg:  ExternalAgentConfig{Name: "MY_EXT_AGENT"},
			want: "CREATE EXTERNAL AGENT \"MY_DB\".\"MY_SCHEMA\".MY_EXT_AGENT;",
		},
		{
			name: "full: or replace, version, comment",
			cfg: ExternalAgentConfig{
				Name:        "RAG_APP",
				OrReplace:   true,
				VersionName: "V1",
				Comment:     "external RAG application",
			},
			want: "CREATE OR REPLACE EXTERNAL AGENT \"MY_DB\".\"MY_SCHEMA\".RAG_APP\n" +
				"  WITH VERSION V1\n" +
				"  COMMENT = 'external RAG application';",
		},
		{
			name: "if not exists",
			cfg:  ExternalAgentConfig{Name: "A", IfNotExists: true},
			want: "CREATE EXTERNAL AGENT IF NOT EXISTS \"MY_DB\".\"MY_SCHEMA\".A;",
		},
		{
			name: "or replace wins over if not exists",
			cfg:  ExternalAgentConfig{Name: "A", OrReplace: true, IfNotExists: true},
			want: "CREATE OR REPLACE EXTERNAL AGENT \"MY_DB\".\"MY_SCHEMA\".A;",
		},
		{
			name: "case sensitive name is quoted",
			cfg:  ExternalAgentConfig{Name: "MixedCase", CaseSensitive: true},
			want: "CREATE EXTERNAL AGENT \"MY_DB\".\"MY_SCHEMA\".\"MixedCase\";",
		},
		{
			name: "blank name falls back to placeholder",
			cfg:  ExternalAgentConfig{},
			want: "CREATE EXTERNAL AGENT \"MY_DB\".\"MY_SCHEMA\".external_agent_name;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateExternalAgentSql("MY_DB", "MY_SCHEMA", tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
