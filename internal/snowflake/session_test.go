// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

// TestParseParameters covers the shared SHOW PARAMETERS row parser used by both
// GetSessionParameters and GetAccountParameters: column resolution, skipping
// blank keys, and the guaranteed non-nil empty slice.
func TestParseParameters(t *testing.T) {
	res := &QueryResult{
		// SHOW PARAMETERS column order: key, value, default, level, description, type
		Columns: []string{"key", "value", "default", "level", "description", "type"},
		Rows: [][]any{
			{"AUTOCOMMIT", "true", "true", "", "Controls autocommit", "BOOLEAN"},
			{"TIMEZONE", "America/Los_Angeles", "America/Los_Angeles", "ACCOUNT", "Session timezone", "STRING"},
			{"", "ignored", "", "", "blank key is dropped", "STRING"},
		},
	}

	got := parseParameters(res)
	if len(got) != 2 {
		t.Fatalf("expected 2 params (blank key dropped), got %d: %+v", len(got), got)
	}
	if got[0] != (SessionParam{Key: "AUTOCOMMIT", Value: "true", Type: "BOOLEAN", Level: "", Description: "Controls autocommit"}) {
		t.Errorf("unexpected first param: %+v", got[0])
	}
	if got[1].Key != "TIMEZONE" || got[1].Value != "America/Los_Angeles" || got[1].Type != "STRING" || got[1].Level != "ACCOUNT" {
		t.Errorf("unexpected second param: %+v", got[1])
	}
}

// TestParseParametersEmpty verifies an empty result yields a non-nil slice so
// the value marshals to a JSON array (not null) — the frontend renders a
// graceful "no parameters" state for unprivileged roles.
func TestParseParametersEmpty(t *testing.T) {
	res := &QueryResult{Columns: []string{"key", "value", "type", "description"}}
	got := parseParameters(res)
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}
