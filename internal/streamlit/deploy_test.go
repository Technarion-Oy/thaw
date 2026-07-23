// SPDX-License-Identifier: GPL-3.0-or-later

package streamlit

import (
	"strings"
	"testing"
)

// TestDeployConfigBuildsCreateSQL verifies the deploy path's param → StreamlitConfig
// mapping produces the CREATE STREAMLIT statement BuildCreateStreamlitSql emits —
// i.e. the temp-stage deploy uses the same grammar as the Create modal.
func TestDeployConfigBuildsCreateSQL(t *testing.T) {
	params := DeployStreamlitParams{
		Database:       "MY_DB",
		Schema:         "MY_SC",
		Name:           "My App",
		CaseSensitive:  true,
		OrReplace:      true,
		QueryWarehouse: "MY_WH",
		Title:          "Live App",
		Comment:        "the app",
	}
	cfg := deployConfig(`@"MY_DB"."MY_SC".THAW_STREAMLIT_1`, "app/main.py", params)

	sql, err := BuildCreateStreamlitSql(params.Database, params.Schema, cfg)
	if err != nil {
		t.Fatalf("BuildCreateStreamlitSql: %v", err)
	}

	for _, want := range []string{
		`CREATE OR REPLACE STREAMLIT "MY_DB"."MY_SC"."My App"`,
		`FROM @"MY_DB"."MY_SC".THAW_STREAMLIT_1`,
		`MAIN_FILE = 'app/main.py'`,
		`QUERY_WAREHOUSE = "MY_WH"`,
		`TITLE = 'Live App'`,
		`COMMENT = 'the app'`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("CREATE STREAMLIT missing %q in:\n%s", want, sql)
		}
	}
}

func TestDeployConfigMinimalOmitsOptionalClauses(t *testing.T) {
	cfg := deployConfig("@STG", "streamlit_app.py", DeployStreamlitParams{
		Database: "DB", Schema: "SC", Name: "APP",
	})
	sql, err := BuildCreateStreamlitSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("BuildCreateStreamlitSql: %v", err)
	}
	if strings.Contains(sql, "OR REPLACE") || strings.Contains(sql, "QUERY_WAREHOUSE") ||
		strings.Contains(sql, "TITLE") || strings.Contains(sql, "COMMENT") {
		t.Errorf("minimal deploy emitted an optional clause:\n%s", sql)
	}
	if !strings.Contains(sql, `FROM @STG`) || !strings.Contains(sql, `MAIN_FILE = 'streamlit_app.py'`) {
		t.Errorf("minimal deploy missing FROM/MAIN_FILE:\n%s", sql)
	}
}
