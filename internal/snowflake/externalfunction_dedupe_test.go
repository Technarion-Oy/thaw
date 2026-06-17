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

func TestDedupeExternalFunctions(t *testing.T) {
	objs := []SnowflakeObject{
		// A regular UDF that is NOT external — must survive.
		{Name: "ADD_ONE", Kind: "FUNCTION", Schema: "PUBLIC", Arguments: "NUMBER"},
		// Same name as an external function but a different signature — a distinct
		// overload that is NOT external, so it must survive.
		{Name: "CALL_API", Kind: "FUNCTION", Schema: "PUBLIC", Arguments: "VARCHAR"},
		// External function that ALSO leaked onto the FUNCTION path (column-absent
		// edition); case-insensitive schema match — must be dropped.
		{Name: "CALL_API", Kind: "FUNCTION", Schema: "public", Arguments: "NUMBER"},
		// The authoritative EXTERNAL FUNCTION entry (NUMBER signature).
		{Name: "CALL_API", Kind: "EXTERNAL FUNCTION", Schema: "PUBLIC", Arguments: "NUMBER"},
	}

	got := dedupeExternalFunctions(objs)

	var keys []string
	for _, o := range got {
		keys = append(keys, o.Schema+"."+o.Name+"("+o.Arguments+"):"+o.Kind)
	}

	// The FUNCTION CALL_API(NUMBER) collides with EXTERNAL FUNCTION
	// CALL_API(NUMBER) and is dropped. The FUNCTION CALL_API(VARCHAR) overload has
	// no external counterpart, so it survives.
	want := map[string]bool{
		"PUBLIC.ADD_ONE(NUMBER):FUNCTION":           true,
		"PUBLIC.CALL_API(VARCHAR):FUNCTION":         true,
		"PUBLIC.CALL_API(NUMBER):EXTERNAL FUNCTION": true,
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

func TestDedupeExternalFunctionsNone(t *testing.T) {
	objs := []SnowflakeObject{
		{Name: "ADD_ONE", Kind: "FUNCTION", Schema: "PUBLIC", Arguments: "NUMBER"},
	}
	got := dedupeExternalFunctions(objs)
	if len(got) != 1 || got[0].Name != "ADD_ONE" {
		t.Fatalf("expected slice returned unchanged, got %v", got)
	}
}
