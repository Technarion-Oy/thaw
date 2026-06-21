// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package joinpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// JoinPolicyConfig holds the parameters for creating a Snowflake JOIN POLICY
// object. Unlike masking and row access policies, a join policy has a fixed
// signature — it takes no arguments and always RETURNS JOIN_CONSTRAINT — so the
// config carries only the name options, the body expression, and an optional
// comment. The body is a JOIN_CONSTRAINT(...) expression deciding whether joins
// are required for queries that read from objects the policy is attached to.
type JoinPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Body          string `json:"body"` // JOIN_CONSTRAINT(JOIN_REQUIRED => <bool_expr>)
	Comment       string `json:"comment"`
}

// BuildCreateJoinPolicySql constructs a CREATE JOIN POLICY statement from the
// given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL.
//
//	CREATE [OR REPLACE] JOIN POLICY [IF NOT EXISTS] <fqn>
//	  AS () RETURNS JOIN_CONSTRAINT ->
//	  <body>
//	  [COMMENT = '…'];
func BuildCreateJoinPolicySql(db, schema string, cfg JoinPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("JOIN POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "join_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Join policies have a fixed, argument-less signature returning the internal
	// JOIN_CONSTRAINT type.
	sb.WriteString("\n  AS () RETURNS JOIN_CONSTRAINT ->")

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
