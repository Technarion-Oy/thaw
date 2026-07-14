// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package config

import "testing"

func TestLogPrefsWithDefaults(t *testing.T) {
	got := LogPrefsWithDefaults(LogPrefs{})
	if got.LogLevel != "info" {
		t.Errorf("empty LogLevel should default to info, got %q", got.LogLevel)
	}
	// A set level is preserved.
	got = LogPrefsWithDefaults(LogPrefs{LogLevel: "debug"})
	if got.LogLevel != "debug" {
		t.Errorf("set LogLevel should be preserved, got %q", got.LogLevel)
	}
}

func TestValidLogLevel(t *testing.T) {
	for _, ok := range []string{"debug", "info", "warn", "error"} {
		if !ValidLogLevel(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "trace", "INFO", "verbose"} {
		if ValidLogLevel(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestValidateLogPrefs(t *testing.T) {
	// Unknown level resets to default.
	got := ValidateLogPrefs(LogPrefs{LogLevel: "bogus", IncludeQuerySQL: true, IncludeInternalQueries: true})
	if got.LogLevel != DefaultLogPrefs().LogLevel {
		t.Errorf("bogus level should reset to default, got %q", got.LogLevel)
	}
	// IncludeInternalQueries is cleared when IncludeQuerySQL is off.
	got = ValidateLogPrefs(LogPrefs{LogLevel: "info", IncludeQuerySQL: false, IncludeInternalQueries: true})
	if got.IncludeInternalQueries {
		t.Error("IncludeInternalQueries should be cleared when IncludeQuerySQL is off")
	}
	// Valid, consistent prefs pass through unchanged.
	in := LogPrefs{LogLevel: "warn", IncludeQuerySQL: true, IncludeInternalQueries: true}
	if got := ValidateLogPrefs(in); got != in {
		t.Errorf("valid prefs should pass through unchanged, got %+v", got)
	}

	// An empty LogLevel is the "use the build default" sentinel and must be
	// preserved (not reset to "info"), so the apply path keeps the build
	// default when the user has expressed no preference.
	if got := ValidateLogPrefs(LogPrefs{LogLevel: ""}); got.LogLevel != "" {
		t.Errorf("empty LogLevel should be preserved, got %q", got.LogLevel)
	}
}

func TestValidateLogPrefs_EnforcesInvariantAfterAdminOverride(t *testing.T) {
	// Mirrors the read-path fix: a user who enabled both switches, then an
	// admin force-disables SQL logging only — the effective prefs must not
	// present a checked-but-inert internal-queries switch.
	effective := LogPrefs{LogLevel: "info", IncludeQuerySQL: false, IncludeInternalQueries: true}
	if got := ValidateLogPrefs(effective); got.IncludeInternalQueries {
		t.Error("IncludeInternalQueries must be cleared once IncludeQuerySQL is forced off")
	}
}

func TestRestoreAdminLockedLogPrefs(t *testing.T) {
	user := LogPrefs{LogLevel: "debug", IncludeQuerySQL: true, IncludeInternalQueries: true}
	effective := LogPrefs{LogLevel: "error", IncludeQuerySQL: false, IncludeInternalQueries: false}
	locked := LogPrefsLocked{IncludeQuerySQL: true}

	got := RestoreAdminLockedLogPrefs(user, effective, locked)
	// Locked field takes the enforced value.
	if got.IncludeQuerySQL {
		t.Error("locked IncludeQuerySQL should be forced to the admin value (false)")
	}
	// Unlocked fields keep the user's value.
	if got.LogLevel != "debug" {
		t.Errorf("unlocked LogLevel should keep user value, got %q", got.LogLevel)
	}
	if !got.IncludeInternalQueries {
		t.Error("unlocked IncludeInternalQueries should keep user value")
	}
}

func TestLoadAdminLogPrefs_NoPolicy(t *testing.T) {
	// With no system features.json present, the user's prefs pass through
	// unchanged and nothing is locked.
	user := LogPrefs{LogLevel: "warn", IncludeQuerySQL: true, IncludeInternalQueries: true}
	effective, locked := LoadAdminLogPrefs(user)
	if effective != user {
		t.Errorf("no admin policy should leave prefs unchanged, got %+v", effective)
	}
	if locked != (LogPrefsLocked{}) {
		t.Errorf("no admin policy should lock nothing, got %+v", locked)
	}
}
