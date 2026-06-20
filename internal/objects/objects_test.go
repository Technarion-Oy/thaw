// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package objects

import (
	"reflect"
	"testing"

	"thaw/internal/snowflake"
)

func TestAppendPackagesPolicyDesc(t *testing.T) {
	tests := []struct {
		name string
		desc *snowflake.QueryResult
		want []snowflake.PropertyPair
	}{
		{
			name: "single row with columns",
			desc: &snowflake.QueryResult{
				Columns: []string{"created_on", "name", "language", "allowlist", "blocklist", "additional_creation_blocklist", "comment", "owner"},
				Rows: [][]any{
					{"2026-06-20", "PKG", "PYTHON", "['numpy', 'pandas']", "['requests']", "[]", "hi", "SYSADMIN"},
				},
			},
			// Only the four configuration properties are appended, lowercased.
			want: []snowflake.PropertyPair{
				{Key: "language", Value: "PYTHON"},
				{Key: "allowlist", Value: "['numpy', 'pandas']"},
				{Key: "blocklist", Value: "['requests']"},
				{Key: "additional_creation_blocklist", Value: "[]"},
			},
		},
		{
			name: "uppercase column names are matched case-insensitively",
			desc: &snowflake.QueryResult{
				Columns: []string{"LANGUAGE", "ALLOWLIST"},
				Rows:    [][]any{{"PYTHON", "['*']"}},
			},
			want: []snowflake.PropertyPair{
				{Key: "language", Value: "PYTHON"},
				{Key: "allowlist", Value: "['*']"},
			},
		},
		{
			name: "row-per-property shape",
			desc: &snowflake.QueryResult{
				Columns: []string{"property", "value"},
				Rows: [][]any{
					{"NAME", "PKG"},
					{"LANGUAGE", "PYTHON"},
					{"ALLOWLIST", "['numpy==1.26.4']"},
					{"BLOCKLIST", "[]"},
					{"ADDITIONAL_CREATION_BLOCKLIST", "['scipy']"},
					{"COMMENT", "ignored"},
				},
			},
			want: []snowflake.PropertyPair{
				{Key: "language", Value: "PYTHON"},
				{Key: "allowlist", Value: "['numpy==1.26.4']"},
				{Key: "blocklist", Value: "[]"},
				{Key: "additional_creation_blocklist", Value: "['scipy']"},
			},
		},
		{
			name: "SQL NULL cells render as empty strings",
			desc: &snowflake.QueryResult{
				Columns: []string{"language", "allowlist", "blocklist"},
				Rows:    [][]any{{"PYTHON", nil, nil}},
			},
			want: []snowflake.PropertyPair{
				{Key: "language", Value: "PYTHON"},
				{Key: "allowlist", Value: ""},
				{Key: "blocklist", Value: ""},
			},
		},
		{
			name: "row-per-property with a short row is skipped, not panicked",
			desc: &snowflake.QueryResult{
				Columns: []string{"property", "value"},
				Rows: [][]any{
					{"LANGUAGE"}, // missing value cell
					{"ALLOWLIST", "['numpy']"},
				},
			},
			want: []snowflake.PropertyPair{
				{Key: "allowlist", Value: "['numpy']"},
			},
		},
		{name: "nil result appends nothing", desc: nil, want: nil},
		{
			name: "empty result appends nothing",
			desc: &snowflake.QueryResult{Columns: []string{"language"}, Rows: nil},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendPackagesPolicyDesc(nil, tt.desc)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("appendPackagesPolicyDesc() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestAppendPackagesPolicyDescPreservesExisting verifies the helper appends to
// (rather than replaces) the SHOW pairs it is given.
func TestAppendPackagesPolicyDescPreservesExisting(t *testing.T) {
	existing := []snowflake.PropertyPair{{Key: "name", Value: "PKG"}, {Key: "comment", Value: "hi"}}
	desc := &snowflake.QueryResult{
		Columns: []string{"language"},
		Rows:    [][]any{{"PYTHON"}},
	}
	got := appendPackagesPolicyDesc(existing, desc)
	want := []snowflake.PropertyPair{
		{Key: "name", Value: "PKG"},
		{Key: "comment", Value: "hi"},
		{Key: "language", Value: "PYTHON"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendPackagesPolicyDesc() = %#v, want %#v", got, want)
	}
}
