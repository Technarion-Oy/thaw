// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import "testing"

func TestBuildCreateAgentSql(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AgentConfig
		want    string
		wantErr bool
	}{
		{
			name: "minimal with default spec placeholder",
			cfg:  AgentConfig{Name: "MY_AGENT"},
			want: "CREATE AGENT \"MY_DB\".\"MY_SCHEMA\".MY_AGENT\n" +
				"  FROM SPECIFICATION\n  $THAW$\nmodels:\n  orchestration: auto\n  $THAW$;",
		},
		{
			name: "full: or replace, comment, profile, spec",
			cfg: AgentConfig{
				Name:          "SUPPORT_BOT",
				OrReplace:     true,
				Comment:       "customer support",
				Profile:       `{"display_name": "Support", "color": "#FF0000"}`,
				Specification: "models:\n  orchestration: claude-3-5-sonnet",
			},
			want: "CREATE OR REPLACE AGENT \"MY_DB\".\"MY_SCHEMA\".SUPPORT_BOT\n" +
				"  COMMENT = 'customer support'\n" +
				"  PROFILE = '{\"display_name\": \"Support\", \"color\": \"#FF0000\"}'\n" +
				"  FROM SPECIFICATION\n  $THAW$\nmodels:\n  orchestration: claude-3-5-sonnet\n  $THAW$;",
		},
		{
			// A profile value containing a backslash and a single quote must
			// survive the single-quoted SQL literal: QuoteTextLit doubles the
			// backslash (\\ -> \\\\) and the single quote (' -> ''), so Snowflake
			// parses the literal back to the original JSON `{"avatar":"a\\b'c"}`.
			name: "profile with backslash and quote is escaped",
			cfg: AgentConfig{
				Name:          "A",
				Profile:       `{"avatar":"a\\b'c"}`,
				Specification: "x: 1",
			},
			want: "CREATE AGENT \"MY_DB\".\"MY_SCHEMA\".A\n" +
				"  PROFILE = '{\"avatar\":\"a\\\\\\\\b''c\"}'\n" +
				"  FROM SPECIFICATION\n  $THAW$\nx: 1\n  $THAW$;",
		},
		{
			name: "if not exists",
			cfg:  AgentConfig{Name: "A", IfNotExists: true, Specification: "x: 1"},
			want: "CREATE AGENT IF NOT EXISTS \"MY_DB\".\"MY_SCHEMA\".A\n" +
				"  FROM SPECIFICATION\n  $THAW$\nx: 1\n  $THAW$;",
		},
		{
			name: "or replace wins over if not exists",
			cfg:  AgentConfig{Name: "A", OrReplace: true, IfNotExists: true, Specification: "x: 1"},
			want: "CREATE OR REPLACE AGENT \"MY_DB\".\"MY_SCHEMA\".A\n" +
				"  FROM SPECIFICATION\n  $THAW$\nx: 1\n  $THAW$;",
		},
		{
			name: "case sensitive name is quoted",
			cfg:  AgentConfig{Name: "MixedCase", CaseSensitive: true, Specification: "x: 1"},
			want: "CREATE AGENT \"MY_DB\".\"MY_SCHEMA\".\"MixedCase\"\n" +
				"  FROM SPECIFICATION\n  $THAW$\nx: 1\n  $THAW$;",
		},
		{
			name: "blank name falls back to placeholder",
			cfg:  AgentConfig{Specification: "x: 1"},
			want: "CREATE AGENT \"MY_DB\".\"MY_SCHEMA\".agent_name\n" +
				"  FROM SPECIFICATION\n  $THAW$\nx: 1\n  $THAW$;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateAgentSql("MY_DB", "MY_SCHEMA", tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
