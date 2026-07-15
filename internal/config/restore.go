// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "reflect"

// RestoreAdminLockedFields returns a copy of user with every admin-locked bool
// field overwritten by the corresponding effective (admin-controlled) value.
// This prevents a client from bypassing IT policy by submitting flags that
// differ from the enforced admin configuration. Fields that are not
// admin-lockable have locked=false and are therefore left untouched, as are
// any non-bool fields.
func RestoreAdminLockedFields(user, effective, locked FeatureFlags) FeatureFlags {
	result := user
	rv := reflect.ValueOf(&result).Elem()
	ev := reflect.ValueOf(effective)
	lv := reflect.ValueOf(locked)
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).Kind() != reflect.Bool {
			continue
		}
		if lv.Field(i).Bool() {
			rv.Field(i).SetBool(ev.Field(i).Bool())
		}
	}
	return result
}

// ValidateSessionConfig clamps a SessionConfig's fields to their valid ranges
// and normalizes InitMode. The returned value is safe to persist and apply.
func ValidateSessionConfig(sc SessionConfig) SessionConfig {
	if sc.MaxSessions < 1 {
		sc.MaxSessions = 1
	} else if sc.MaxSessions > 32 {
		sc.MaxSessions = 32
	}
	if sc.MaxOpenConnsPerSession < 1 {
		sc.MaxOpenConnsPerSession = 1
	} else if sc.MaxOpenConnsPerSession > 16 {
		sc.MaxOpenConnsPerSession = 16
	}
	if sc.MaxIdleConnsPerSession < 1 {
		sc.MaxIdleConnsPerSession = 1
	} else if sc.MaxIdleConnsPerSession > 16 {
		sc.MaxIdleConnsPerSession = 16
	}
	if sc.MaxIdleConnsPerSession > sc.MaxOpenConnsPerSession {
		sc.MaxIdleConnsPerSession = sc.MaxOpenConnsPerSession
	}
	if sc.InitMode != "lazy" && sc.InitMode != "eager" {
		sc.InitMode = "lazy"
	}
	if sc.IdleTimeoutMinutes < 0 {
		sc.IdleTimeoutMinutes = 0
	} else if sc.IdleTimeoutMinutes > 480 {
		sc.IdleTimeoutMinutes = 480
	}
	return sc
}
