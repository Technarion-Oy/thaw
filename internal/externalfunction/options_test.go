// SPDX-License-Identifier: GPL-3.0-or-later

package externalfunction

import "testing"

func TestGetBuilderOptions(t *testing.T) {
	opts := GetBuilderOptions()
	if len(opts.Compression) == 0 || len(opts.NullHandling) == 0 ||
		len(opts.Volatility) == 0 || len(opts.ContextHeaders) == 0 {
		t.Fatalf("expected all option lists to be populated: %+v", opts)
	}
	// The 21 documented context functions must all be present.
	if len(opts.ContextHeaders) != 21 {
		t.Errorf("expected 21 context-header functions, got %d", len(opts.ContextHeaders))
	}

	// The returned slices must be copies — mutating them must not affect the
	// package-level lists handed to the next caller.
	opts.Compression[0] = "MUTATED"
	if GetBuilderOptions().Compression[0] == "MUTATED" {
		t.Error("GetBuilderOptions returned a slice aliasing the package-level list")
	}
}
