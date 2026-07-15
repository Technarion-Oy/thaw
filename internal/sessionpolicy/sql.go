// SPDX-License-Identifier: GPL-3.0-or-later

package sessionpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// SessionPolicyConfig holds the parameters for creating a Snowflake SESSION
// POLICY object. Each numeric timeout parameter is a pointer so the builder can
// tell "leave at the Snowflake default" (nil) apart from "set to N" (e.g. 0,
// which is a meaningful value for the lifespan parameters — it disables
// enforcement). The fields map to the CREATE SESSION POLICY options in the order
// Snowflake documents them.
type SessionPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// Timeouts (minutes).
	IdleTimeoutMins   *int `json:"idleTimeoutMins"`   // SESSION_IDLE_TIMEOUT_MINS (5–1440, default 240)
	UIIdleTimeoutMins *int `json:"uiIdleTimeoutMins"` // SESSION_UI_IDLE_TIMEOUT_MINS (5–1440, default 240)
	MaxLifespanMins   *int `json:"maxLifespanMins"`   // SESSION_MAX_LIFESPAN_MINS (0–43200, default 0 = no limit)
	UIMaxLifespanMins *int `json:"uiMaxLifespanMins"` // SESSION_UI_MAX_LIFESPAN_MINS (0–43200, default 0 = no limit)

	// Secondary-role controls. Each entry is either the special token "ALL"
	// (rendered as the quoted literal 'ALL') or a role identifier. An empty
	// slice leaves the parameter unset (inheriting the documented default).
	AllowedSecondaryRoles []string `json:"allowedSecondaryRoles"` // ALLOWED_SECONDARY_ROLES (default ('ALL'))
	BlockedSecondaryRoles []string `json:"blockedSecondaryRoles"` // BLOCKED_SECONDARY_ROLES (default ())

	Comment string `json:"comment"`
}

// timeoutParams pairs each CREATE SESSION POLICY timeout keyword with the config
// pointer that backs it, so BuildCreateSessionPolicySql can emit them uniformly
// in Snowflake's documented order.
func (cfg SessionPolicyConfig) timeoutParams() []struct {
	keyword string
	value   *int
} {
	return []struct {
		keyword string
		value   *int
	}{
		{"SESSION_IDLE_TIMEOUT_MINS", cfg.IdleTimeoutMins},
		{"SESSION_UI_IDLE_TIMEOUT_MINS", cfg.UIIdleTimeoutMins},
		{"SESSION_MAX_LIFESPAN_MINS", cfg.MaxLifespanMins},
		{"SESSION_UI_MAX_LIFESPAN_MINS", cfg.UIMaxLifespanMins},
	}
}

// BuildCreateSessionPolicySql constructs a CREATE SESSION POLICY statement from
// the given config. Only parameters the caller explicitly set (non-nil pointers
// / non-empty role lists) are emitted; the rest inherit Snowflake's documented
// defaults. When the name is blank the builder substitutes a placeholder so the
// live preview reads as a completable template rather than invalid SQL.
//
//	CREATE [OR REPLACE] SESSION POLICY [IF NOT EXISTS] <fqn>
//	  [SESSION_IDLE_TIMEOUT_MINS = <n>]
//	  [SESSION_UI_IDLE_TIMEOUT_MINS = <n>]
//	  [SESSION_MAX_LIFESPAN_MINS = <n>]
//	  [SESSION_UI_MAX_LIFESPAN_MINS = <n>]
//	  [ALLOWED_SECONDARY_ROLES = (…)]
//	  [BLOCKED_SECONDARY_ROLES = (…)]
//	  [COMMENT = '…'];
func BuildCreateSessionPolicySql(db, schema string, cfg SessionPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SESSION POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "session_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	for _, p := range cfg.timeoutParams() {
		if p.value != nil {
			fmt.Fprintf(&sb, "\n  %s = %d", p.keyword, *p.value)
		}
	}

	if len(cfg.AllowedSecondaryRoles) > 0 {
		fmt.Fprintf(&sb, "\n  ALLOWED_SECONDARY_ROLES = %s", snowflake.FormatSecondaryRoles(cfg.AllowedSecondaryRoles))
	}
	if len(cfg.BlockedSecondaryRoles) > 0 {
		fmt.Fprintf(&sb, "\n  BLOCKED_SECONDARY_ROLES = %s", snowflake.FormatSecondaryRoles(cfg.BlockedSecondaryRoles))
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
