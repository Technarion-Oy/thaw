// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestExplainFormatConstants(t *testing.T) {
	if ExplainJSON != "JSON" {
		t.Errorf("ExplainJSON = %q, want %q", ExplainJSON, "JSON")
	}
	if ExplainTabular != "TABULAR" {
		t.Errorf("ExplainTabular = %q, want %q", ExplainTabular, "TABULAR")
	}
}

func TestExplainSQLFormat(t *testing.T) {
	tests := []struct {
		query  string
		format ExplainFormat
		want   string
	}{
		{
			query:  "SELECT 1",
			format: ExplainJSON,
			want:   "EXPLAIN USING JSON SELECT 1",
		},
		{
			query:  "SELECT * FROM my_table WHERE id = 1",
			format: ExplainTabular,
			want:   "EXPLAIN USING TABULAR SELECT * FROM my_table WHERE id = 1",
		},
	}
	for _, tt := range tests {
		got := "EXPLAIN USING " + string(tt.format) + " " + tt.query
		if got != tt.want {
			t.Errorf("Explain SQL for format %q, query %q:\n  got  %q\n  want %q",
				tt.format, tt.query, got, tt.want)
		}
	}
}

func TestValidateExplainFormat(t *testing.T) {
	// Valid formats.
	for _, f := range []ExplainFormat{ExplainJSON, ExplainTabular} {
		if err := validateExplainFormat(f); err != nil {
			t.Errorf("validateExplainFormat(%q) returned unexpected error: %v", f, err)
		}
	}

	// Invalid formats.
	for _, f := range []ExplainFormat{"", "XML", "TEXT", ExplainFormat("JSON; DROP TABLE users--")} {
		if err := validateExplainFormat(f); err == nil {
			t.Errorf("validateExplainFormat(%q) should return error", f)
		}
	}
}
