// SPDX-License-Identifier: GPL-3.0-or-later

package stream

import (
	"strings"
	"testing"
)

func TestBuildCreateStreamSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      StreamConfig
		contains []string
		absent   []string
	}{
		{
			name: "minimal table stream with bare source",
			cfg: StreamConfig{
				Name:   "MY_STREAM",
				Source: "MY_TABLE",
			},
			contains: []string{
				"CREATE STREAM \"DB\".\"SC\".MY_STREAM",
				"ON TABLE \"DB\".\"SC\".\"MY_TABLE\"",
			},
			absent: []string{
				"OR REPLACE", "IF NOT EXISTS", "COPY GRANTS",
				"APPEND_ONLY", "SHOW_INITIAL_ROWS", "INSERT_ONLY", "COMMENT",
			},
		},
		{
			name: "or replace",
			cfg: StreamConfig{
				Name:      "S",
				OrReplace: true,
				Source:    "T",
			},
			contains: []string{"CREATE OR REPLACE STREAM"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists",
			cfg: StreamConfig{
				Name:        "S",
				IfNotExists: true,
				Source:      "T",
			},
			contains: []string{"CREATE STREAM IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: StreamConfig{
				Name:        "S",
				OrReplace:   true,
				IfNotExists: true,
				Source:      "T",
			},
			contains: []string{"CREATE OR REPLACE STREAM"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "copy grants",
			cfg: StreamConfig{
				Name:       "S",
				CopyGrants: true,
				Source:     "T",
			},
			contains: []string{"COPY GRANTS"},
		},
		{
			name: "view source",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "VIEW",
				Source:     "V",
			},
			contains: []string{"ON VIEW \"DB\".\"SC\".\"V\""},
		},
		{
			name: "external table source",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "EXTERNAL TABLE",
				Source:     "ET",
			},
			contains: []string{"ON EXTERNAL TABLE \"DB\".\"SC\".\"ET\""},
		},
		{
			name: "stage source",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "STAGE",
				Source:     "STG",
			},
			contains: []string{"ON STAGE \"DB\".\"SC\".\"STG\""},
		},
		{
			name: "dynamic table source",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "DYNAMIC TABLE",
				Source:     "DT",
			},
			contains: []string{"ON DYNAMIC TABLE \"DB\".\"SC\".\"DT\""},
		},
		{
			name: "append only",
			cfg: StreamConfig{
				Name:       "S",
				Source:     "T",
				AppendOnly: true,
			},
			contains: []string{"APPEND_ONLY = TRUE"},
		},
		{
			name: "show initial rows",
			cfg: StreamConfig{
				Name:            "S",
				Source:          "T",
				ShowInitialRows: true,
			},
			contains: []string{"SHOW_INITIAL_ROWS = TRUE"},
		},
		{
			name: "insert only (external table source)",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "EXTERNAL TABLE",
				Source:     "T",
				InsertOnly: true,
			},
			contains: []string{"INSERT_ONLY = TRUE"},
		},
		{
			name: "insert only dropped for non-external source",
			cfg: StreamConfig{
				Name:       "S",
				SourceType: "TABLE",
				Source:     "T",
				InsertOnly: true,
			},
			absent: []string{"INSERT_ONLY"},
		},
		{
			name: "append only dropped for external table source",
			cfg: StreamConfig{
				Name:            "S",
				SourceType:      "EXTERNAL TABLE",
				Source:          "T",
				AppendOnly:      true,
				ShowInitialRows: true,
			},
			absent: []string{"APPEND_ONLY", "SHOW_INITIAL_ROWS"},
		},
		{
			// Dynamic table streams support neither the row-change flags nor Time
			// Travel — only COPY GRANTS / COMMENT — so all of these are gated out.
			name: "row-change flags and time travel dropped for dynamic table source",
			cfg: StreamConfig{
				Name:            "S",
				SourceType:      "DYNAMIC TABLE",
				Source:          "DT",
				AppendOnly:      true,
				ShowInitialRows: true,
				TimeTravelMode:  "AT",
				TimeTravelKind:  "OFFSET",
				TimeTravelValue: "-60",
			},
			contains: []string{"ON DYNAMIC TABLE"},
			absent:   []string{"APPEND_ONLY", "SHOW_INITIAL_ROWS", "AT (", "OFFSET"},
		},
		{
			name: "time travel AT offset (table source)",
			cfg: StreamConfig{
				Name:            "S",
				Source:          "T",
				TimeTravelMode:  "AT",
				TimeTravelKind:  "OFFSET",
				TimeTravelValue: "-60",
			},
			contains: []string{"AT ( OFFSET => -60 )"},
		},
		{
			name: "time travel BEFORE statement is quoted (table source)",
			cfg: StreamConfig{
				Name:            "S",
				Source:          "T",
				TimeTravelMode:  "BEFORE",
				TimeTravelKind:  "STATEMENT",
				TimeTravelValue: "8e5d0ca9-005e-44e6-b858-a8f5b37c5726",
			},
			contains: []string{"BEFORE ( STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726' )"},
		},
		{
			name: "time travel timestamp expression emitted verbatim (view source)",
			cfg: StreamConfig{
				Name:            "S",
				SourceType:      "VIEW",
				Source:          "V",
				TimeTravelMode:  "AT",
				TimeTravelKind:  "TIMESTAMP",
				TimeTravelValue: "'2024-01-01 12:00:00'::timestamp",
			},
			contains: []string{"AT ( TIMESTAMP => '2024-01-01 12:00:00'::timestamp )"},
		},
		{
			name: "time travel dropped for stage source",
			cfg: StreamConfig{
				Name:            "S",
				SourceType:      "STAGE",
				Source:          "STG",
				TimeTravelMode:  "AT",
				TimeTravelKind:  "OFFSET",
				TimeTravelValue: "-60",
			},
			contains: []string{"ON STAGE"},
			absent:   []string{"AT (", "OFFSET"},
		},
		{
			name: "time travel omitted when value empty",
			cfg: StreamConfig{
				Name:           "S",
				Source:         "T",
				TimeTravelMode: "AT",
				TimeTravelKind: "OFFSET",
			},
			absent: []string{"AT (", "OFFSET"},
		},
		{
			name: "comment escaped",
			cfg: StreamConfig{
				Name:    "S",
				Source:  "T",
				Comment: "it's a stream",
			},
			contains: []string{"COMMENT = 'it''s a stream'"},
		},
		{
			name: "qualified source passed through verbatim",
			cfg: StreamConfig{
				Name:   "S",
				Source: "OTHER_DB.OTHER_SC.OTHER_T",
			},
			contains: []string{"ON TABLE OTHER_DB.OTHER_SC.OTHER_T"},
		},
		{
			name: "case sensitive name is quoted",
			cfg: StreamConfig{
				Name:          "MyStream",
				CaseSensitive: true,
				Source:        "T",
			},
			contains: []string{"\"DB\".\"SC\".\"MyStream\""},
		},
		{
			// A TABLE source emits APPEND_ONLY / SHOW_INITIAL_ROWS; INSERT_ONLY is
			// gated out (external-table only) even though the config sets it.
			name: "all options together (table source gates out INSERT_ONLY)",
			cfg: StreamConfig{
				Name:            "S",
				OrReplace:       true,
				CopyGrants:      true,
				SourceType:      "TABLE",
				Source:          "T",
				AppendOnly:      true,
				ShowInitialRows: true,
				InsertOnly:      true,
				Comment:         "c",
			},
			contains: []string{
				"COPY GRANTS", "ON TABLE", "APPEND_ONLY = TRUE",
				"SHOW_INITIAL_ROWS = TRUE", "COMMENT = 'c'",
			},
			absent: []string{"INSERT_ONLY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateStreamSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(got, ";") {
				t.Errorf("statement should end with ';', got:\n%s", got)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("expected output to NOT contain %q, got:\n%s", no, got)
				}
			}
		})
	}
}

func TestBuildCreateStreamSqlPlaceholders(t *testing.T) {
	got, err := BuildCreateStreamSql("DB", "SC", StreamConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"stream_name", "<source_object>", "ON TABLE"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder %q in:\n%s", want, got)
		}
	}
}

// TestBuildCreateStreamSqlClauseOrder pins the clause order to the order
// Snowflake documents for CREATE STREAM — COPY GRANTS → ON → AT/BEFORE →
// APPEND_ONLY → SHOW_INITIAL_ROWS → COMMENT. Snowflake's CREATE parser is
// order-sensitive, so a config combining several optional clauses must emit them
// in exactly this sequence. (A TABLE source is used, so INSERT_ONLY —
// external-table only — is not part of this ordering check.)
func TestBuildCreateStreamSqlClauseOrder(t *testing.T) {
	got, err := BuildCreateStreamSql("DB", "SC", StreamConfig{
		Name:            "S",
		CopyGrants:      true,
		Source:          "T",
		TimeTravelMode:  "AT",
		TimeTravelKind:  "OFFSET",
		TimeTravelValue: "-60",
		AppendOnly:      true,
		ShowInitialRows: true,
		Comment:         "c",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := []string{"COPY GRANTS", "ON TABLE", "AT ( OFFSET", "APPEND_ONLY = TRUE", "SHOW_INITIAL_ROWS = TRUE", "COMMENT = "}
	prev := -1
	for _, marker := range order {
		idx := strings.Index(got, marker)
		if idx < 0 {
			t.Fatalf("expected clause %q in:\n%s", marker, got)
		}
		if idx <= prev {
			t.Errorf("clause %q is out of order (index %d <= previous %d) in:\n%s", marker, idx, prev, got)
		}
		prev = idx
	}
}
