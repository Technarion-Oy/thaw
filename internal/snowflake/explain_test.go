// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

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

func TestExplainSQLConstruction(t *testing.T) {
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
