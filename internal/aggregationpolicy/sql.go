// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package aggregationpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AggregationPolicyConfig holds the parameters for creating a Snowflake
// AGGREGATION POLICY object. An aggregation policy takes no arguments — the
// signature is always the empty `()` and the return type is always
// AGGREGATION_CONSTRAINT — so the only authored parts are the Body (an SQL
// expression returning an aggregation constraint) and an optional Comment. The
// fields map to the CREATE AGGREGATION POLICY options in the order Snowflake
// documents them.
type AggregationPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	// Body is the SQL expression that follows `RETURNS AGGREGATION_CONSTRAINT ->`.
	// Typical values are AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5) or
	// NO_AGGREGATION_CONSTRAINT(), optionally wrapped in conditional logic (e.g. a
	// CASE on CURRENT_ROLE()).
	Body    string `json:"body"`
	Comment string `json:"comment"`
}

// BuildCreateAggregationPolicySql constructs a CREATE AGGREGATION POLICY
// statement from the given config. When the name or body is blank the builder
// substitutes a placeholder so the live preview reads as a completable template
// rather than invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive;
// OR REPLACE wins.
//
//	CREATE [OR REPLACE] AGGREGATION POLICY [IF NOT EXISTS] <fqn>
//	  AS () RETURNS AGGREGATION_CONSTRAINT -> <body>
//	  [COMMENT = '…'];
func BuildCreateAggregationPolicySql(db, schema string, cfg AggregationPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("AGGREGATION POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "aggregation_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s AS () RETURNS AGGREGATION_CONSTRAINT ->", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// The body is raw SQL (not a string literal), so it is emitted verbatim. A
	// blank body becomes a sensible minimum-group-size constraint so the preview
	// stays a valid template.
	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
