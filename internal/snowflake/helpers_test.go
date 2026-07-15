// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"strings"
	"testing"
)

func TestIsBoolean(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"BOOLEAN", true},
		{"BOOL", true},
		{"boolean", true},
		{"  BOOLEAN  ", true},
		{"VARCHAR", false},
		{"NUMBER", false},
		{"ARRAY", false},
	}

	for _, tt := range tests {
		if got := IsBoolean(tt.dataType); got != tt.expected {
			t.Errorf("IsBoolean(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"NUMBER", true},
		{"NUMBER(38,0)", true},
		{"DECIMAL(10,2)", true},
		{"INT", true},
		{"INTEGER", true},
		{"BIGINT", true},
		{"SMALLINT", true},
		{"TINYINT", true},
		{"BYTEINT", true},
		{"FLOAT", true},
		{"DOUBLE", true},
		{"REAL", true},
		{"NUMERIC", true},
		{"VARCHAR", false},
		{"BOOLEAN", false},
	}

	for _, tt := range tests {
		if got := IsNumeric(tt.dataType); got != tt.expected {
			t.Errorf("IsNumeric(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestNeedsQuotes(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"VARCHAR", true},
		{"STRING", true},
		{"TEXT", true},
		{"TIMESTAMP", true},
		{"DATE", true},
		{"BOOLEAN", false},
		{"NUMBER", false},
		{"INT", false},
	}

	for _, tt := range tests {
		if got := NeedsQuotes(tt.dataType); got != tt.expected {
			t.Errorf("NeedsQuotes(%q) = %v, want %v", tt.dataType, got, tt.expected)
		}
	}
}

func TestDollarQuoteTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"no dollar quote", "select 1", "$$"},
		{"contains $$", "select '$$'", "$thaw$"},
		{"contains $$ and $thaw$", "a $$ b $thaw$ c", "$thaw_body$"},
		{"contains all named tags", "$$ $thaw$ $thaw_body$", "$thaw_0$"},
	}
	for _, tt := range tests {
		if got := DollarQuoteTag(tt.body); got != tt.want {
			t.Errorf("DollarQuoteTag(%q) = %q, want %q", tt.body, got, tt.want)
		}
		// The chosen tag must never occur in the body.
		if strings.Contains(tt.body, DollarQuoteTag(tt.body)) {
			t.Errorf("DollarQuoteTag(%q) returned a tag present in the body", tt.body)
		}
	}
}
