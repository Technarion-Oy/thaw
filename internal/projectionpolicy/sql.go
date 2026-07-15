// SPDX-License-Identifier: GPL-3.0-or-later

package projectionpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ProjectionPolicyConfig holds the parameters for creating a Snowflake
// PROJECTION POLICY object. A projection policy takes no arguments — the
// signature is always the empty `()` and the return type is always
// PROJECTION_CONSTRAINT — so the only authored parts are the Body (an SQL
// expression returning a projection constraint) and an optional Comment. The
// fields map to the CREATE PROJECTION POLICY options in the order Snowflake
// documents them.
type ProjectionPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	// Body is the SQL expression that follows `RETURNS PROJECTION_CONSTRAINT ->`.
	// Typical values are PROJECTION_CONSTRAINT(ALLOW => true) or
	// PROJECTION_CONSTRAINT(ALLOW => false), optionally wrapped in conditional
	// logic (e.g. a CASE on CURRENT_ROLE()).
	Body    string `json:"body"`
	Comment string `json:"comment"`
}

// BuildCreateProjectionPolicySql constructs a CREATE PROJECTION POLICY
// statement from the given config. When the name or body is blank the builder
// substitutes a placeholder so the live preview reads as a completable template
// rather than invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive;
// OR REPLACE wins.
//
//	CREATE [OR REPLACE] PROJECTION POLICY [IF NOT EXISTS] <fqn>
//	  AS () RETURNS PROJECTION_CONSTRAINT -> <body>
//	  [COMMENT = '…'];
func BuildCreateProjectionPolicySql(db, schema string, cfg ProjectionPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("PROJECTION POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "projection_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s AS () RETURNS PROJECTION_CONSTRAINT ->", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// The body is raw SQL (not a string literal), so it is emitted verbatim. A
	// blank body becomes a sensible allow-projection constraint so the preview
	// stays a valid template.
	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "PROJECTION_CONSTRAINT(ALLOW => true)"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
