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

import "testing"

func TestDedupeDataMetricFunctions(t *testing.T) {
	objs := []SnowflakeObject{
		// A regular UDF that is NOT a DMF — must survive.
		{Name: "ADD_ONE", Kind: "FUNCTION", Schema: "PUBLIC", Arguments: "NUMBER"},
		// A DMF that ALSO leaked onto the FUNCTION path (column-absent edition);
		// case-insensitive schema match — must be dropped.
		{Name: "NULL_COUNT", Kind: "FUNCTION", Schema: "public", Arguments: "TABLE(VARCHAR)"},
		// The authoritative DATA METRIC FUNCTION entry, from SHOW DATA METRIC
		// FUNCTIONS.
		{Name: "NULL_COUNT", Kind: "DATA METRIC FUNCTION", Schema: "PUBLIC", Arguments: "TABLE(VARCHAR)"},
		// The same DMF relabeled from the SHOW FUNCTIONS path (is_data_metric=Y) —
		// a duplicate that must collapse into one.
		{Name: "NULL_COUNT", Kind: "DATA METRIC FUNCTION", Schema: "PUBLIC", Arguments: "TABLE(VARCHAR)"},
	}

	got := dedupeFunctionVariant(objs, "DATA METRIC FUNCTION")

	var keys []string
	for _, o := range got {
		keys = append(keys, o.Schema+"."+o.Name+"("+o.Arguments+"):"+o.Kind)
	}

	want := map[string]bool{
		"PUBLIC.ADD_ONE(NUMBER):FUNCTION":                        true,
		"PUBLIC.NULL_COUNT(TABLE(VARCHAR)):DATA METRIC FUNCTION": true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d objects, got %d: %v", len(want), len(got), keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("unexpected object survived dedup: %s", k)
		}
	}
}

func TestDedupeDataMetricFunctionsNone(t *testing.T) {
	objs := []SnowflakeObject{
		{Name: "ADD_ONE", Kind: "FUNCTION", Schema: "PUBLIC", Arguments: "NUMBER"},
	}
	got := dedupeFunctionVariant(objs, "DATA METRIC FUNCTION")
	if len(got) != 1 || got[0].Name != "ADD_ONE" {
		t.Fatalf("expected slice returned unchanged, got %v", got)
	}
}

// TestExtractArgTypesNestedParens guards the data-metric-function case: the
// TABLE argument's type is itself parenthesized, which a naive first-")" scan
// would truncate.
func TestExtractArgTypesNestedParens(t *testing.T) {
	cases := map[string]string{
		"MY_DMF(TABLE(NUMBER)) RETURN NUMBER":                "TABLE(NUMBER)",
		"MY_DMF(TABLE(C1 NUMBER, C2 VARCHAR)) RETURN NUMBER": "TABLE(C1 NUMBER, C2 VARCHAR)",
		"ADD_ONE(NUMBER) RETURN NUMBER":                      "NUMBER",
		"NO_ARGS() RETURN NUMBER":                            "",
		"NOPARENS":                                           "",
	}
	for in, want := range cases {
		if got := extractArgTypes(in); got != want {
			t.Errorf("extractArgTypes(%q) = %q; want %q", in, got, want)
		}
	}
}
