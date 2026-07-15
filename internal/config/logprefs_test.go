// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "testing"

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

func TestMergeAdminLogPrefs(t *testing.T) {
	bptr := func(b bool) *bool { return &b }
	sptr := func(s string) *string { return &s }

	tests := []struct {
		name       string
		user       LogPrefs
		cfg        adminLogging
		wantEff    LogPrefs
		wantLocked LogPrefsLocked
	}{
		{
			name:       "empty policy leaves user untouched",
			user:       LogPrefs{LogLevel: "warn", IncludeQuerySQL: true},
			cfg:        adminLogging{},
			wantEff:    LogPrefs{LogLevel: "warn", IncludeQuerySQL: true},
			wantLocked: LogPrefsLocked{},
		},
		{
			name:       "only includeQuerySQL locked off (privacy)",
			user:       LogPrefs{LogLevel: "info", IncludeQuerySQL: true, IncludeInternalQueries: true},
			cfg:        adminLogging{IncludeQuerySQL: bptr(false)},
			wantEff:    LogPrefs{LogLevel: "info", IncludeQuerySQL: false, IncludeInternalQueries: true},
			wantLocked: LogPrefsLocked{IncludeQuerySQL: true},
		},
		{
			name:       "invalid logLevel is ignored (not locked)",
			user:       LogPrefs{LogLevel: "debug"},
			cfg:        adminLogging{LogLevel: sptr("verbose")},
			wantEff:    LogPrefs{LogLevel: "debug"},
			wantLocked: LogPrefsLocked{},
		},
		{
			name:       "valid logLevel is enforced and locked",
			user:       LogPrefs{LogLevel: "debug"},
			cfg:        adminLogging{LogLevel: sptr("error")},
			wantEff:    LogPrefs{LogLevel: "error"},
			wantLocked: LogPrefsLocked{LogLevel: true},
		},
		{
			name:       "internal-on implies and locks includeQuerySQL (audit)",
			user:       LogPrefs{},
			cfg:        adminLogging{IncludeInternalQueries: bptr(true)},
			wantEff:    LogPrefs{IncludeQuerySQL: true, IncludeInternalQueries: true},
			wantLocked: LogPrefsLocked{IncludeQuerySQL: true, IncludeInternalQueries: true},
		},
		{
			name:       "explicit includeQuerySQL=false wins over implied true",
			user:       LogPrefs{},
			cfg:        adminLogging{IncludeQuerySQL: bptr(false), IncludeInternalQueries: bptr(true)},
			wantEff:    LogPrefs{IncludeQuerySQL: false, IncludeInternalQueries: true},
			wantLocked: LogPrefsLocked{IncludeQuerySQL: true, IncludeInternalQueries: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eff, locked := mergeAdminLogPrefs(tt.user, tt.cfg)
			if eff != tt.wantEff {
				t.Errorf("effective = %+v, want %+v", eff, tt.wantEff)
			}
			if locked != tt.wantLocked {
				t.Errorf("locked = %+v, want %+v", locked, tt.wantLocked)
			}
		})
	}
}
