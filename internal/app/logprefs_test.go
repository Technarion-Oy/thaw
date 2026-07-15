// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"path/filepath"
	"testing"

	"thaw/internal/config"
)

// isolateConfig points os.UserConfigDir at a fresh temp directory so config
// reads/writes in a test can't touch the developer's real ~/.config/thaw.
// Covers both the macOS ($HOME/Library/…) and Linux ($XDG_CONFIG_HOME) paths.
func isolateConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// TestUpdateLogPrefs_PersistsNormalizedAndApplies exercises the app-level
// composition that is the actual security/consistency boundary: UpdateLogPrefs
// must normalize the invariant before it hits disk (not defer it to the next
// read) and apply the very same value to the runtime cache.
func TestUpdateLogPrefs_PersistsNormalizedAndApplies(t *testing.T) {
	isolateConfig(t)
	a := &App{}

	// Inconsistent input: internal-queries on while SQL logging is off. With no
	// admin policy present, this must be normalized to internal-queries off and
	// persisted that way, so a later read never surfaces the inert combination.
	err := a.UpdateLogPrefs(config.LogPrefs{
		LogLevel:               "warn",
		IncludeQuerySQL:        false,
		IncludeInternalQueries: true,
	})
	if err != nil {
		t.Fatalf("UpdateLogPrefs: %v", err)
	}

	// Read path (config on disk + effective normalization).
	got := a.GetLogPrefs()
	if got.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", got.LogLevel)
	}
	if got.IncludeInternalQueries {
		t.Error("IncludeInternalQueries should have been normalized off on disk")
	}

	// Runtime cache consulted by maybeFileLogQuery must match what was persisted.
	if rt := a.currentLogPrefs(); rt.IncludeInternalQueries {
		t.Error("runtime-applied prefs should be normalized too")
	}

	// Sanity: the raw persisted value (not just the effective read) is normalized.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.LogPrefs.IncludeInternalQueries {
		t.Error("on-disk LogPrefs.IncludeInternalQueries should be false, not merely reconciled on read")
	}
}

// TestUpdateLogPrefs_RejectsInvalidLevel confirms a bogus level is coerced to
// the default before persisting.
func TestUpdateLogPrefs_RejectsInvalidLevel(t *testing.T) {
	isolateConfig(t)
	a := &App{}

	if err := a.UpdateLogPrefs(config.LogPrefs{LogLevel: "bogus", IncludeQuerySQL: true}); err != nil {
		t.Fatalf("UpdateLogPrefs: %v", err)
	}
	if got := a.GetLogPrefs(); got.LogLevel != config.DefaultLogPrefs().LogLevel {
		t.Errorf("invalid level should coerce to default, got %q", got.LogLevel)
	}
}
