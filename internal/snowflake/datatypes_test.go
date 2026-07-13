// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"testing"
)

func TestValidateDataType(t *testing.T) {
	tests := []struct {
		input   string
		wantOK  bool
		wantOut string
	}{
		// Numeric — exact
		{"NUMBER", true, "NUMBER"},
		{"NUMBER(10)", true, "NUMBER(10, 0)"},
		{"NUMBER(10,2)", true, "NUMBER(10, 2)"},
		{"DECIMAL(5,3)", true, "DECIMAL(5, 3)"},
		{"NUMERIC", true, "NUMERIC"},
		{"INT", true, "INT"},
		{"INTEGER", true, "INTEGER"},
		{"BIGINT", true, "BIGINT"},
		{"SMALLINT", true, "SMALLINT"},
		{"TINYINT", true, "TINYINT"},
		{"BYTEINT", true, "BYTEINT"},
		// Numeric — approximate
		{"FLOAT", true, "FLOAT"},
		{"FLOAT4", true, "FLOAT4"},
		{"FLOAT8", true, "FLOAT8"},
		{"DOUBLE", true, "DOUBLE"},
		{"DOUBLE PRECISION", true, "DOUBLE PRECISION"},
		{"REAL", true, "REAL"},
		// String & Binary
		{"VARCHAR", true, "VARCHAR"},
		{"VARCHAR(255)", true, "VARCHAR(255)"},
		{"CHAR(1)", true, "CHAR(1)"},
		{"CHARACTER(10)", true, "CHARACTER(10)"},
		{"STRING", true, "STRING"},
		{"TEXT", true, "TEXT"},
		{"BINARY", true, "BINARY"},
		{"BINARY(100)", true, "BINARY(100)"},
		{"VARBINARY", true, "VARBINARY"},
		// Logical
		{"BOOLEAN", true, "BOOLEAN"},
		// Date & Time
		{"DATE", true, "DATE"},
		{"DATETIME", true, "DATETIME"},
		{"TIME", true, "TIME"},
		{"TIME(3)", true, "TIME(3)"},
		{"TIMESTAMP", true, "TIMESTAMP"},
		{"TIMESTAMP(9)", true, "TIMESTAMP(9)"},
		{"TIMESTAMP_LTZ(6)", true, "TIMESTAMP_LTZ(6)"},
		{"TIMESTAMP_NTZ", true, "TIMESTAMP_NTZ"},
		{"TIMESTAMP_TZ(0)", true, "TIMESTAMP_TZ(0)"},
		// No-underscore TIMESTAMP synonyms (issue #711)
		{"TIMESTAMPLTZ", true, "TIMESTAMPLTZ"},
		{"TIMESTAMPNTZ", true, "TIMESTAMPNTZ"},
		{"TIMESTAMPTZ", true, "TIMESTAMPTZ"},
		{"TIMESTAMPTZ(3)", true, "TIMESTAMPTZ(3)"},
		// File (issue #711)
		{"FILE", true, "FILE"},
		// Semi-structured
		{"VARIANT", true, "VARIANT"},
		{"OBJECT", true, "OBJECT"},
		{"ARRAY", true, "ARRAY"},
		// Structured
		{"ARRAY(VARCHAR)", true, "ARRAY(VARCHAR)"},
		{"ARRAY(INT)", true, "ARRAY(INT)"},
		{"OBJECT(name VARCHAR, age INT)", true, "OBJECT(NAME VARCHAR, AGE INT)"},
		{"MAP(VARCHAR, INT)", true, "MAP(VARCHAR, INT)"},
		{"MAP(VARCHAR, ARRAY(INT))", true, "MAP(VARCHAR, ARRAY(INT))"},
		// Geospatial
		{"GEOGRAPHY", true, "GEOGRAPHY"},
		{"GEOMETRY", true, "GEOMETRY"},
		// Vector
		{"VECTOR(FLOAT, 1536)", true, "VECTOR(FLOAT, 1536)"},
		{"VECTOR(INT, 3)", true, "VECTOR(INT, 3)"},
		// Case insensitivity
		{"varchar(100)", true, "VARCHAR(100)"},
		{"boolean", true, "BOOLEAN"},
		// Invalid inputs
		{"NUMBER(0,0)", false, ""},
		{"NUMBER(39,0)", false, ""},
		{"NUMBER(5,6)", false, ""},
		{"VARCHAR(0)", false, ""},
		{"TIME(10)", false, ""},
		{"VECTOR(FLOAT)", false, ""},
		{"MAP(VARCHAR)", false, ""},
		{"FOOBAR", false, ""},
		{"", false, ""},
		{"INT(5)", false, ""},
		{"BOOLEAN(1)", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ValidateDataType(tc.input)
			if tc.wantOK && err != nil {
				t.Errorf("ValidateDataType(%q) unexpected error: %v", tc.input, err)
			}
			if !tc.wantOK && err == nil {
				t.Errorf("ValidateDataType(%q) expected error, got %q", tc.input, got)
			}
			if tc.wantOK && got != tc.wantOut {
				t.Errorf("ValidateDataType(%q) = %q, want %q", tc.input, got, tc.wantOut)
			}
		})
	}
}

func TestBaseType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"VARCHAR(256)", "VARCHAR"},
		{"number(38,0)", "NUMBER"},
		{"TIMESTAMP_TZ(9)", "TIMESTAMP_TZ"},
		{"timestamptz", "TIMESTAMP_TZ"},
		{"TIMESTAMPTZ(3)", "TIMESTAMP_TZ"},
		{"timestampltz", "TIMESTAMP_LTZ"},
		{"TIMESTAMPLTZ(9)", "TIMESTAMP_LTZ"},
		{"timestampntz", "TIMESTAMP_NTZ"},
		{"TIMESTAMPNTZ(9)", "TIMESTAMP_NTZ"},
		{"VECTOR(FLOAT, 256)", "VECTOR"},
		{"  variant  ", "VARIANT"},
		{"TIMESTAMP_NTZ", "TIMESTAMP_NTZ"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := BaseType(tc.in); got != tc.want {
			t.Errorf("BaseType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
