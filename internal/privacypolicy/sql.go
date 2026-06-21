// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package privacypolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// PrivacyPolicyConfig holds the parameters for creating a Snowflake PRIVACY
// POLICY object. Like join policies, and unlike masking or row access policies,
// a privacy policy has a fixed signature — it takes no arguments and always
// RETURNS PRIVACY_BUDGET — so the config carries only the name options, the body
// expression, and an optional comment. The body calls either NO_PRIVACY_POLICY()
// for unrestricted access or PRIVACY_BUDGET(BUDGET_NAME => '…', …) to enforce a
// differential-privacy budget on queries that read from objects the policy is
// attached to.
type PrivacyPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Body          string `json:"body"` // PRIVACY_BUDGET(BUDGET_NAME => '…', …) or NO_PRIVACY_POLICY()
	Comment       string `json:"comment"`
}

// BuildCreatePrivacyPolicySql constructs a CREATE PRIVACY POLICY statement from
// the given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL.
//
//	CREATE [OR REPLACE] PRIVACY POLICY [IF NOT EXISTS] <fqn>
//	  AS () RETURNS PRIVACY_BUDGET ->
//	  <body>
//	  [COMMENT = '…'];
func BuildCreatePrivacyPolicySql(db, schema string, cfg PrivacyPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("PRIVACY POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "privacy_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Privacy policies have a fixed, argument-less signature returning the
	// internal PRIVACY_BUDGET type.
	sb.WriteString("\n  AS () RETURNS PRIVACY_BUDGET ->")

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "PRIVACY_BUDGET(BUDGET_NAME => 'privacy_budget')"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
