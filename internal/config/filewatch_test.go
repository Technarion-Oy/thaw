// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"slices"
	"testing"
)

func TestValidateFileWatchConfig(t *testing.T) {
	t.Run("clamps negative cap to zero", func(t *testing.T) {
		got := ValidateFileWatchConfig(FileWatchConfig{MaxWatchedDirs: -5})
		if got.MaxWatchedDirs != 0 {
			t.Errorf("MaxWatchedDirs = %d, want 0", got.MaxWatchedDirs)
		}
	})

	t.Run("trims blank glob patterns", func(t *testing.T) {
		got := ValidateFileWatchConfig(FileWatchConfig{
			ExcludeGlobs: []string{"  node_modules  ", "", "   ", "dist"},
		})
		want := []string{"node_modules", "dist"}
		if !slices.Equal(got.ExcludeGlobs, want) {
			t.Errorf("ExcludeGlobs = %v, want %v", got.ExcludeGlobs, want)
		}
	})

	t.Run("preserves nil (unconfigured) sentinel", func(t *testing.T) {
		got := ValidateFileWatchConfig(FileWatchConfig{ExcludeGlobs: nil})
		if got.ExcludeGlobs != nil {
			t.Errorf("nil ExcludeGlobs should stay nil, got %v", got.ExcludeGlobs)
		}
	})

	t.Run("preserves explicit empty (exclude nothing)", func(t *testing.T) {
		got := ValidateFileWatchConfig(FileWatchConfig{ExcludeGlobs: []string{}})
		if got.ExcludeGlobs == nil {
			t.Error("explicit empty ExcludeGlobs must stay non-nil (exclude nothing)")
		}
		if len(got.ExcludeGlobs) != 0 {
			t.Errorf("explicit empty ExcludeGlobs should have len 0, got %v", got.ExcludeGlobs)
		}
	})
}

func TestFileWatchConfigWithDefaults(t *testing.T) {
	t.Run("nil resolves to defaults", func(t *testing.T) {
		got := FileWatchConfigWithDefaults(FileWatchConfig{ExcludeGlobs: nil})
		if !slices.Equal(got.ExcludeGlobs, DefaultWatchExcludeGlobs()) {
			t.Errorf("nil should resolve to defaults, got %v", got.ExcludeGlobs)
		}
	})

	t.Run("explicit empty stays empty (not backfilled)", func(t *testing.T) {
		got := FileWatchConfigWithDefaults(FileWatchConfig{ExcludeGlobs: []string{}})
		if len(got.ExcludeGlobs) != 0 {
			t.Errorf("explicit empty must not be backfilled with defaults, got %v", got.ExcludeGlobs)
		}
	})

	t.Run("custom list preserved", func(t *testing.T) {
		got := FileWatchConfigWithDefaults(FileWatchConfig{ExcludeGlobs: []string{"foo", "bar/baz"}})
		if !slices.Equal(got.ExcludeGlobs, []string{"foo", "bar/baz"}) {
			t.Errorf("custom list should be preserved, got %v", got.ExcludeGlobs)
		}
	})
}

func TestDefaultWatchExcludeGlobsAreNonHidden(t *testing.T) {
	// Dot-prefixed patterns are unreachable — the watcher's hidden-directory
	// filter drops those events before exclusion runs — so none must ship in the
	// defaults. This guards against reintroducing a dead pattern (finding #1).
	for _, g := range DefaultWatchExcludeGlobs() {
		if len(g) > 0 && g[0] == '.' {
			t.Errorf("default exclude glob %q is dot-prefixed (unreachable via the hidden-dir filter)", g)
		}
	}
}

func TestCollapseDefaultExcludeGlobs(t *testing.T) {
	t.Run("list equal to defaults collapses to nil", func(t *testing.T) {
		// A fresh copy of the default slice, as the modal round-trip would submit.
		in := FileWatchConfig{ExcludeGlobs: slices.Clone(DefaultWatchExcludeGlobs())}
		got := CollapseDefaultExcludeGlobs(in)
		if got.ExcludeGlobs != nil {
			t.Errorf("unchanged defaults should collapse to nil, got %v", got.ExcludeGlobs)
		}
	})

	t.Run("edited list is preserved verbatim", func(t *testing.T) {
		edited := append(slices.Clone(DefaultWatchExcludeGlobs()), ".turbo-but-not-really")
		got := CollapseDefaultExcludeGlobs(FileWatchConfig{ExcludeGlobs: edited})
		if !slices.Equal(got.ExcludeGlobs, edited) {
			t.Errorf("edited list should be preserved, got %v", got.ExcludeGlobs)
		}
	})

	t.Run("reordered list is not treated as default", func(t *testing.T) {
		reordered := slices.Clone(DefaultWatchExcludeGlobs())
		slices.Reverse(reordered)
		got := CollapseDefaultExcludeGlobs(FileWatchConfig{ExcludeGlobs: reordered})
		if got.ExcludeGlobs == nil {
			t.Error("a reordered (deliberately edited) list must not collapse to nil")
		}
	})

	t.Run("explicit empty is preserved (not collapsed)", func(t *testing.T) {
		got := CollapseDefaultExcludeGlobs(FileWatchConfig{ExcludeGlobs: []string{}})
		if got.ExcludeGlobs == nil {
			t.Error("explicit empty (exclude nothing) must not collapse to the track-defaults nil sentinel")
		}
	})

	t.Run("nil stays nil", func(t *testing.T) {
		got := CollapseDefaultExcludeGlobs(FileWatchConfig{ExcludeGlobs: nil})
		if got.ExcludeGlobs != nil {
			t.Errorf("nil should stay nil, got %v", got.ExcludeGlobs)
		}
	})
}

// TestFileWatchDefaultsRoundTripToNil verifies the end-to-end persist behavior
// that keeps a save-unchanged install on the auto-updating defaults track:
// read path (nil -> concrete defaults) then persist path (validate -> collapse)
// returns to the nil sentinel.
func TestFileWatchDefaultsRoundTripToNil(t *testing.T) {
	hydrated := FileWatchConfigWithDefaults(FileWatchConfig{ExcludeGlobs: nil}) // what the modal sees
	persisted := CollapseDefaultExcludeGlobs(ValidateFileWatchConfig(hydrated)) // what Save writes
	if persisted.ExcludeGlobs != nil {
		t.Errorf("save-unchanged should round-trip back to nil, got %v", persisted.ExcludeGlobs)
	}
}
