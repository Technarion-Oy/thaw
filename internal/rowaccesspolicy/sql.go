// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package rowaccesspolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// RowAccessArg is a single entry in a row access policy's signature. Each
// argument names a column the policy body may reference to decide whether a row
// is visible to the querying role. Name is the parameter name used inside the
// body; Type is its SQL data type (e.g. VARCHAR, NUMBER(38,0)). When the policy
// is attached to a table or view, each argument is mapped to one of that
// object's columns.
type RowAccessArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// RowAccessPolicyConfig holds the parameters for creating a Snowflake ROW ACCESS
// POLICY object. The fields map to the CREATE ROW ACCESS POLICY options in the
// order Snowflake documents them: the AS (...) signature, the (always BOOLEAN)
// RETURNS, the body expression, then COMMENT. Unlike masking policies, row
// access policies always return BOOLEAN and have no EXEMPT_OTHER_POLICIES option.
type RowAccessPolicyConfig struct {
	Name          string         `json:"name"`
	CaseSensitive bool           `json:"caseSensitive"`
	OrReplace     bool           `json:"orReplace"`
	IfNotExists   bool           `json:"ifNotExists"`
	Args          []RowAccessArg `json:"args"` // signature; columns the body evaluates
	Body          string         `json:"body"` // boolean expression deciding row visibility
	Comment       string         `json:"comment"`
}

// BuildCreateRowAccessPolicySql constructs a CREATE ROW ACCESS POLICY statement
// from the given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL.
//
//	CREATE [OR REPLACE] ROW ACCESS POLICY [IF NOT EXISTS] <fqn> AS
//	  (<arg> <type> [, …]) RETURNS BOOLEAN -> <body>
//	  [COMMENT = '…'];
func BuildCreateRowAccessPolicySql(db, schema string, cfg RowAccessPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("ROW ACCESS POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "row_access_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s AS", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Signature: drop entries missing a name or type so a stray empty input row
	// does not emit "( VARCHAR)". If nothing valid remains, emit a placeholder so
	// the statement stays well-formed.
	args := make([]string, 0, len(cfg.Args))
	for _, a := range cfg.Args {
		name := strings.TrimSpace(a.Name)
		typ := strings.TrimSpace(a.Type)
		if name == "" || typ == "" {
			continue
		}
		args = append(args, fmt.Sprintf("%s %s", name, typ))
	}
	if len(args) == 0 {
		args = append(args, "val VARCHAR")
	}
	fmt.Fprintf(&sb, " (%s)", strings.Join(args, ", "))

	// Row access policies always return BOOLEAN.
	sb.WriteString(" RETURNS BOOLEAN ->")

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "TRUE"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
