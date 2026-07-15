// SPDX-License-Identifier: GPL-3.0-or-later

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
