// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowflake

import (
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
