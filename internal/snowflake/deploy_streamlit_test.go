// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestBuildDeployStreamlitSQL(t *testing.T) {
	cases := []struct {
		name    string
		stageAt string
		main    string
		params  DeployStreamlitParams
		want    string
	}{
		{
			name:    "minimal omits optional clauses",
			stageAt: "@STG",
			main:    "streamlit_app.py",
			params: DeployStreamlitParams{
				Database: "MY_DB",
				Schema:   "MY_SC",
				Name:     "MY_APP",
			},
			want: `CREATE STREAMLIT "MY_DB"."MY_SC".MY_APP
  FROM @STG
  MAIN_FILE = 'streamlit_app.py'`,
		},
		{
			name:    "full: or replace, case-sensitive name, all optional clauses",
			stageAt: `@"MY_DB"."MY_SC".THAW_STREAMLIT_1`,
			main:    "app/main.py",
			params: DeployStreamlitParams{
				Database:       "MY_DB",
				Schema:         "MY_SC",
				Name:           "My App",
				CaseSensitive:  true,
				OrReplace:      true,
				QueryWarehouse: "MY_WH",
				Title:          "Live App",
				Comment:        "the app",
			},
			want: `CREATE OR REPLACE STREAMLIT "MY_DB"."MY_SC"."My App"
  FROM @"MY_DB"."MY_SC".THAW_STREAMLIT_1
  MAIN_FILE = 'app/main.py'
  QUERY_WAREHOUSE = "MY_WH"
  TITLE = 'Live App'
  COMMENT = 'the app'`,
		},
		{
			name:    "escapes single quotes in main file, title, and comment",
			stageAt: "@STG",
			main:    "it's_app.py",
			params: DeployStreamlitParams{
				Database: "DB",
				Schema:   "SC",
				Name:     "APP",
				Title:    "It's Live",
				Comment:  "a 'quoted' comment",
			},
			want: `CREATE STREAMLIT "DB"."SC".APP
  FROM @STG
  MAIN_FILE = 'it''s_app.py'
  TITLE = 'It''s Live'
  COMMENT = 'a ''quoted'' comment'`,
		},
		{
			name:    "blank optional clauses are trimmed away",
			stageAt: "@STG",
			main:    "streamlit_app.py",
			params: DeployStreamlitParams{
				Database:       "DB",
				Schema:         "SC",
				Name:           "APP",
				QueryWarehouse: "   ",
				Title:          "  ",
				Comment:        "",
			},
			want: `CREATE STREAMLIT "DB"."SC".APP
  FROM @STG
  MAIN_FILE = 'streamlit_app.py'`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildDeployStreamlitSQL(tc.stageAt, tc.main, tc.params)
			if got != tc.want {
				t.Errorf("buildDeployStreamlitSQL mismatch:\n got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
