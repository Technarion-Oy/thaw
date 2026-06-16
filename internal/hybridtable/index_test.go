// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package hybridtable

import (
	"strings"
	"testing"
)

func TestIsIndexableType(t *testing.T) {
	// Eligible as an index key column.
	for _, typ := range []string{"NUMBER(38,0)", "VARCHAR(256)", "BOOLEAN", "TIMESTAMP_NTZ", "DATE", "timestamp_ltz"} {
		if !IsIndexableType(typ) {
			t.Errorf("IsIndexableType(%q) = false, want true", typ)
		}
	}
	// Barred from index keys.
	for _, typ := range []string{"VARIANT", "OBJECT", "ARRAY", "GEOGRAPHY", "GEOMETRY", "VECTOR(FLOAT, 8)", "TIMESTAMP_TZ", "timestamptz"} {
		if IsIndexableType(typ) {
			t.Errorf("IsIndexableType(%q) = true, want false", typ)
		}
	}
}

func TestIsIncludableType(t *testing.T) {
	// INCLUDE bars only semi-structured + geospatial; VECTOR / TIMESTAMP_TZ are OK.
	for _, typ := range []string{"NUMBER", "VARCHAR(10)", "VECTOR(FLOAT, 8)", "TIMESTAMP_TZ", "BOOLEAN"} {
		if !IsIncludableType(typ) {
			t.Errorf("IsIncludableType(%q) = false, want true", typ)
		}
	}
	for _, typ := range []string{"VARIANT", "OBJECT", "ARRAY", "GEOGRAPHY", "GEOMETRY"} {
		if IsIncludableType(typ) {
			t.Errorf("IsIncludableType(%q) = true, want false", typ)
		}
	}
}

func TestEligibleIndexColumns(t *testing.T) {
	cols := []IndexColumn{
		{Name: "ID", Type: "NUMBER(38,0)"},
		{Name: "TS", Type: "TIMESTAMP_TZ"},      // INCLUDE-only
		{Name: "DATA", Type: "VARIANT"},         // neither
		{Name: "EMB", Type: "VECTOR(FLOAT, 8)"}, // INCLUDE-only
	}
	opts := EligibleIndexColumns(cols)

	if strings.Join(opts.KeyColumns, ",") != "ID" {
		t.Errorf("KeyColumns = %v, want [ID]", opts.KeyColumns)
	}
	if strings.Join(opts.IncludeColumns, ",") != "ID,TS,EMB" {
		t.Errorf("IncludeColumns = %v, want [ID TS EMB]", opts.IncludeColumns)
	}

	// Empty input yields non-nil empty slices (so they marshal as []).
	empty := EligibleIndexColumns(nil)
	if empty.KeyColumns == nil || empty.IncludeColumns == nil {
		t.Errorf("expected non-nil empty slices, got %+v", empty)
	}
}
