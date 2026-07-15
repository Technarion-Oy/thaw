// SPDX-License-Identifier: GPL-3.0-or-later

package notebook

import (
	"strings"
	"testing"
)

func TestBuildCreateNotebookSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      NotebookConfig
		contains []string
		absent   []string
	}{
		{
			name: "full notebook from staged files",
			cfg: NotebookConfig{
				Name:           "MY_NB",
				IfNotExists:    true,
				SourceLocation: "@db.sc.nb_stage/dir",
				MainFile:       "notebook_app.ipynb",
				QueryWarehouse: "MY_WH",
				Comment:        "analysis nb",
			},
			contains: []string{
				"CREATE NOTEBOOK IF NOT EXISTS \"DB\".\"SC\".MY_NB",
				"FROM '@db.sc.nb_stage/dir'",
				"MAIN_FILE = 'notebook_app.ipynb'",
				"QUERY_WAREHOUSE = \"MY_WH\"",
				"COMMENT = 'analysis nb'",
			},
			absent: []string{"OR REPLACE"},
		},
		{
			name: "minimal empty notebook omits optional clauses",
			cfg: NotebookConfig{
				Name:           "NB2",
				QueryWarehouse: "WH2",
			},
			contains: []string{
				"CREATE NOTEBOOK \"DB\".\"SC\".NB2",
				"QUERY_WAREHOUSE = \"WH2\"",
			},
			absent: []string{"IF NOT EXISTS", "FROM ", "MAIN_FILE", "COMMENT ="},
		},
		{
			name: "or replace wins over if not exists",
			cfg: NotebookConfig{
				Name:        "NB3",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE NOTEBOOK \"DB\".\"SC\".NB3"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "case sensitive name and escaped literals",
			cfg: NotebookConfig{
				Name:           "Mixed",
				CaseSensitive:  true,
				SourceLocation: "@stg/it's",
				Comment:        "it's a comment",
			},
			contains: []string{
				"\"Mixed\"",
				"FROM '@stg/it''s'",
				"COMMENT = 'it''s a comment'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateNotebookSql("DB", "SC", tt.cfg)
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
