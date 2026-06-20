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

func TestCell(t *testing.T) {
	row := []any{"alpha", nil, []byte("beta"), 42}
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"in range string", 0, "alpha"},
		{"in range nil → empty", 1, ""},
		{"in range bytes", 2, "beta"},
		{"in range int", 3, "42"},
		{"negative (ColIdx miss)", -1, ""},
		{"past end", 4, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Cell(row, tc.idx); got != tc.want {
				t.Errorf("Cell(row, %d) = %q, want %q", tc.idx, got, tc.want)
			}
		})
	}
}
