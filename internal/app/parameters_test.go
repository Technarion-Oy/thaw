// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "testing"

// TestObjectTarget covers the shared "<TYPE> <qualified-name>" clause builder:
// object-type allowlisting, per-part quoting, blank-part skipping, and the
// require-a-name guard.
func TestObjectTarget(t *testing.T) {
	cases := []struct {
		name       string
		objectType string
		parts      []string
		want       string
		wantErr    bool
	}{
		{"database", "DATABASE", []string{"MY_DB"}, `DATABASE "MY_DB"`, false},
		{"schema two-level", "SCHEMA", []string{"MY_DB", "MY_SCH"}, `SCHEMA "MY_DB"."MY_SCH"`, false},
		{"warehouse", "WAREHOUSE", []string{"WH"}, `WAREHOUSE "WH"`, false},
		{"lowercase type normalized", "schema", []string{"D", "S"}, `SCHEMA "D"."S"`, false},
		{"blank parts skipped", "DATABASE", []string{"", "MY_DB", ""}, `DATABASE "MY_DB"`, false},
		{"embedded quote escaped", "DATABASE", []string{`we"ird`}, `DATABASE "we""ird"`, false},
		{"unsupported type", "PROCEDURE", []string{"P"}, "", true},
		{"empty type", "", []string{"X"}, "", true},
		{"no name parts", "DATABASE", []string{"", "  "}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := objectTarget(c.objectType, c.parts)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("objectTarget(%q, %v) = %q, want %q", c.objectType, c.parts, got, c.want)
			}
		})
	}
}

// TestParamNamePattern guards the parameter-name validation that protects the
// unquoted interpolation into ALTER … SET/UNSET.
func TestParamNamePattern(t *testing.T) {
	valid := []string{"LOG_LEVEL", "DATA_RETENTION_TIME_IN_DAYS", "_hidden", "ABC$123"}
	for _, s := range valid {
		if !paramNamePattern.MatchString(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	invalid := []string{"", "1BAD", "HAS SPACE", "HAS-DASH", "DROP;TABLE", "a.b"}
	for _, s := range invalid {
		if paramNamePattern.MatchString(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}
