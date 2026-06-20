// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package maskingpolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// MaskingArg is a single entry in a masking policy's signature. The first
// argument designates the column whose values are masked; any additional
// arguments are conditional columns the body may reference to decide how to
// mask. Name is the parameter name used inside the body; Type is its SQL data
// type (e.g. VARCHAR, NUMBER(38,0)). The first argument's Type must equal the
// policy's ReturnType.
type MaskingArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MaskingPolicyConfig holds the parameters for creating a Snowflake MASKING
// POLICY object. The fields map to the CREATE MASKING POLICY options in the
// order Snowflake documents them: the AS (...) signature, RETURNS, the body
// expression, then COMMENT and EXEMPT_OTHER_POLICIES.
type MaskingPolicyConfig struct {
	Name          string       `json:"name"`
	CaseSensitive bool         `json:"caseSensitive"`
	OrReplace     bool         `json:"orReplace"`
	IfNotExists   bool         `json:"ifNotExists"`
	Args          []MaskingArg `json:"args"`       // signature; the first arg is the masked column
	ReturnType    string       `json:"returnType"` // must match the first arg's type
	Body          string       `json:"body"`       // masking expression (e.g. a CASE expression)
	Comment       string       `json:"comment"`
	// ExemptOtherPolicies, when true, lets a column protected by this policy be
	// used as a conditional column in another masking policy. It is only emitted
	// when set, since FALSE is the Snowflake default.
	ExemptOtherPolicies bool `json:"exemptOtherPolicies"`
}

// BuildCreateMaskingPolicySql constructs a CREATE MASKING POLICY statement from
// the given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL.
//
//	CREATE [OR REPLACE] MASKING POLICY [IF NOT EXISTS] <fqn> AS
//	  (<arg> <type> [, …]) RETURNS <type> -> <body>
//	  [COMMENT = '…']
//	  [EXEMPT_OTHER_POLICIES = TRUE];
func BuildCreateMaskingPolicySql(db, schema string, cfg MaskingPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("MASKING POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "masking_policy_name"
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

	// RETURNS must match the first argument's type. Default to it (or VARCHAR)
	// when the caller leaves it blank so the preview stays valid.
	returnType := strings.TrimSpace(cfg.ReturnType)
	if returnType == "" {
		returnType = firstArgType(cfg.Args)
	}
	fmt.Fprintf(&sb, " RETURNS %s ->", returnType)

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "'***MASKED***'"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	if cfg.ExemptOtherPolicies {
		sb.WriteString("\n  EXEMPT_OTHER_POLICIES = TRUE")
	}

	return sb.String() + ";", nil
}

// firstArgType returns the type of the first signature entry that has both a
// name and a type, falling back to VARCHAR when none is usable. It backs the
// RETURNS default so a masking policy with a single VARCHAR column still
// produces a valid RETURNS clause when the return type is left blank.
func firstArgType(args []MaskingArg) string {
	for _, a := range args {
		if strings.TrimSpace(a.Name) != "" && strings.TrimSpace(a.Type) != "" {
			return strings.TrimSpace(a.Type)
		}
	}
	return "VARCHAR"
}
