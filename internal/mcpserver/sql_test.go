// SPDX-License-Identifier: GPL-3.0-or-later

package mcpserver

import "testing"

func TestBuildCreateMCPServerSql(t *testing.T) {
	const spec = "tools:\n  - name: \"product-search\"\n    type: \"CORTEX_SEARCH_SERVICE_QUERY\""

	tests := []struct {
		name    string
		cfg     MCPServerConfig
		want    string
		wantErr bool
	}{
		{
			name: "minimal: blank spec falls back to placeholder",
			cfg:  MCPServerConfig{Name: "MY_MCP"},
			want: "CREATE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".MY_MCP\n" +
				"  FROM SPECIFICATION\n  $THAW$\ntools: []\n  $THAW$;",
		},
		{
			name: "with specification",
			cfg:  MCPServerConfig{Name: "MY_MCP", Specification: spec},
			want: "CREATE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".MY_MCP\n" +
				"  FROM SPECIFICATION\n  $THAW$\n" + spec + "\n  $THAW$;",
		},
		{
			name: "or replace",
			cfg:  MCPServerConfig{Name: "MY_MCP", OrReplace: true, Specification: spec},
			want: "CREATE OR REPLACE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".MY_MCP\n" +
				"  FROM SPECIFICATION\n  $THAW$\n" + spec + "\n  $THAW$;",
		},
		{
			name: "if not exists",
			cfg:  MCPServerConfig{Name: "A", IfNotExists: true},
			want: "CREATE MCP SERVER IF NOT EXISTS \"MY_DB\".\"MY_SCHEMA\".A\n" +
				"  FROM SPECIFICATION\n  $THAW$\ntools: []\n  $THAW$;",
		},
		{
			name: "or replace wins over if not exists",
			cfg:  MCPServerConfig{Name: "A", OrReplace: true, IfNotExists: true},
			want: "CREATE OR REPLACE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".A\n" +
				"  FROM SPECIFICATION\n  $THAW$\ntools: []\n  $THAW$;",
		},
		{
			name: "case sensitive name is quoted",
			cfg:  MCPServerConfig{Name: "MixedCase", CaseSensitive: true},
			want: "CREATE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".\"MixedCase\"\n" +
				"  FROM SPECIFICATION\n  $THAW$\ntools: []\n  $THAW$;",
		},
		{
			name: "blank name falls back to placeholder",
			cfg:  MCPServerConfig{},
			want: "CREATE MCP SERVER \"MY_DB\".\"MY_SCHEMA\".mcp_server_name\n" +
				"  FROM SPECIFICATION\n  $THAW$\ntools: []\n  $THAW$;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateMCPServerSql("MY_DB", "MY_SCHEMA", tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
