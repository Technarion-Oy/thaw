// SPDX-License-Identifier: GPL-3.0-or-later

package passwordpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// PasswordPolicyConfig holds the parameters for creating a Snowflake PASSWORD
// POLICY object. Every numeric parameter is a pointer so the builder can tell
// "leave at the Snowflake default" (nil) apart from "set to N" (e.g. 0). The
// fields map to the CREATE PASSWORD POLICY options in the order Snowflake
// documents them.
type PasswordPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// Complexity.
	MinLength         *int `json:"minLength"`         // PASSWORD_MIN_LENGTH (8–256, default 14)
	MaxLength         *int `json:"maxLength"`         // PASSWORD_MAX_LENGTH (8–256, default 256)
	MinUpperCaseChars *int `json:"minUpperCaseChars"` // PASSWORD_MIN_UPPER_CASE_CHARS (0–256, default 1)
	MinLowerCaseChars *int `json:"minLowerCaseChars"` // PASSWORD_MIN_LOWER_CASE_CHARS (0–256, default 1)
	MinNumericChars   *int `json:"minNumericChars"`   // PASSWORD_MIN_NUMERIC_CHARS (0–256, default 1)
	MinSpecialChars   *int `json:"minSpecialChars"`   // PASSWORD_MIN_SPECIAL_CHARS (0–256, default 0)

	// Age, retry/lockout, and reuse history.
	MinAgeDays      *int `json:"minAgeDays"`      // PASSWORD_MIN_AGE_DAYS (0–999, default 0)
	MaxAgeDays      *int `json:"maxAgeDays"`      // PASSWORD_MAX_AGE_DAYS (0–999, default 90)
	MaxRetries      *int `json:"maxRetries"`      // PASSWORD_MAX_RETRIES (1–10, default 5)
	LockoutTimeMins *int `json:"lockoutTimeMins"` // PASSWORD_LOCKOUT_TIME_MINS (1–999, default 15)
	History         *int `json:"history"`         // PASSWORD_HISTORY (0–24, default 5)

	Comment string `json:"comment"`
}

// params pairs each CREATE PASSWORD POLICY keyword with the config pointer that
// backs it, so BuildCreatePasswordPolicySql can emit them uniformly in
// Snowflake's documented order.
func (cfg PasswordPolicyConfig) params() []struct {
	keyword string
	value   *int
} {
	return []struct {
		keyword string
		value   *int
	}{
		{"PASSWORD_MIN_LENGTH", cfg.MinLength},
		{"PASSWORD_MAX_LENGTH", cfg.MaxLength},
		{"PASSWORD_MIN_UPPER_CASE_CHARS", cfg.MinUpperCaseChars},
		{"PASSWORD_MIN_LOWER_CASE_CHARS", cfg.MinLowerCaseChars},
		{"PASSWORD_MIN_NUMERIC_CHARS", cfg.MinNumericChars},
		{"PASSWORD_MIN_SPECIAL_CHARS", cfg.MinSpecialChars},
		{"PASSWORD_MIN_AGE_DAYS", cfg.MinAgeDays},
		{"PASSWORD_MAX_AGE_DAYS", cfg.MaxAgeDays},
		{"PASSWORD_MAX_RETRIES", cfg.MaxRetries},
		{"PASSWORD_LOCKOUT_TIME_MINS", cfg.LockoutTimeMins},
		{"PASSWORD_HISTORY", cfg.History},
	}
}

// BuildCreatePasswordPolicySql constructs a CREATE PASSWORD POLICY statement
// from the given config. Only parameters the caller explicitly set (non-nil
// pointers) are emitted; the rest inherit Snowflake's documented defaults. When
// the name is blank the builder substitutes a placeholder so the live preview
// reads as a completable template rather than invalid SQL.
//
//	CREATE [OR REPLACE] PASSWORD POLICY [IF NOT EXISTS] <fqn>
//	  [PASSWORD_MIN_LENGTH = <n>]
//	  …
//	  [PASSWORD_HISTORY = <n>]
//	  [COMMENT = '…'];
func BuildCreatePasswordPolicySql(db, schema string, cfg PasswordPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("PASSWORD POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "password_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	for _, p := range cfg.params() {
		if p.value != nil {
			fmt.Fprintf(&sb, "\n  %s = %d", p.keyword, *p.value)
		}
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
