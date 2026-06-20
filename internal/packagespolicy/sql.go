// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package packagespolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// PackagesPolicyConfig holds the parameters for creating a Snowflake PACKAGES
// POLICY object. LANGUAGE PYTHON is required and always emitted (PYTHON is the
// only language Snowflake currently supports). The three list parameters are
// optional: an unspecified list inherits Snowflake's documented default
// (ALLOWLIST defaults to ('*') = all allowed, BLOCKLIST and
// ADDITIONAL_CREATION_BLOCKLIST default to () = none blocked), so the builder
// emits only the lists the caller has explicitly set. Each list is a []string of
// bare package-spec tokens (e.g. "numpy", "numpy==1.26.4", "*") which the builder
// renders as single-quoted string literals.
type PackagesPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// Allowlist sets ALLOWLIST — the package specifications a UDF/procedure may
	// import. Default: ('*') (all packages allowed).
	Allowlist []string `json:"allowlist"`
	// Blocklist sets BLOCKLIST — the package specifications that are forbidden
	// (takes precedence over the allowlist). Default: () (none blocked).
	Blocklist []string `json:"blocklist"`
	// AdditionalCreationBlocklist sets ADDITIONAL_CREATION_BLOCKLIST — package
	// specifications blocked only when an object is created (not when it runs).
	// Default: () (none blocked).
	AdditionalCreationBlocklist []string `json:"additionalCreationBlocklist"`

	Comment string `json:"comment"`
}

// formatStringList renders a token slice into the SQL list grammar used by the
// packages-policy list parameters — each token becomes a single-quoted string
// literal, e.g. []string{"numpy", "pandas"} → "('numpy', 'pandas')". Blank
// tokens are skipped. Unexported; the CREATE builder and FormatStringList (the
// exported IPC-facing wrapper below) both serialize lists through it.
func formatStringList(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts = append(parts, "'"+snowflake.EscapeTextLit(t)+"'")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// FormatStringList is the exported wrapper around formatStringList so the
// packages-policy properties modal renders / builds list values through the same
// quote-aware serializer the CREATE builder uses (reached over IPC via
// App.FormatPackagesPolicyList). Pure string handling — no connection required.
func FormatStringList(tokens []string) string { return formatStringList(tokens) }

// hasToken reports whether the slice contains at least one non-blank token, so
// the builder omits a parameter whose only entries are empty strings (which
// would serialize to the invalid empty list `()`).
func hasToken(tokens []string) bool {
	for _, t := range tokens {
		if strings.TrimSpace(t) != "" {
			return true
		}
	}
	return false
}

// BuildCreatePackagesPolicySql constructs a CREATE PACKAGES POLICY statement
// from the given config. LANGUAGE PYTHON is always emitted; only the list
// parameters the caller explicitly set (non-empty lists) and a non-empty comment
// are written, the rest inherit Snowflake's documented defaults. When the name is
// blank the builder substitutes a placeholder so the live preview reads as a
// completable template rather than invalid SQL. OR REPLACE and IF NOT EXISTS are
// mutually exclusive; OR REPLACE wins.
//
//	CREATE [OR REPLACE] PACKAGES POLICY [IF NOT EXISTS] <fqn>
//	  LANGUAGE PYTHON
//	  [ALLOWLIST = (…)]
//	  [BLOCKLIST = (…)]
//	  [ADDITIONAL_CREATION_BLOCKLIST = (…)]
//	  [COMMENT = '…'];
func BuildCreatePackagesPolicySql(db, schema string, cfg PackagesPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " PACKAGES POLICY"
	// OR REPLACE and IF NOT EXISTS are mutually exclusive; OR REPLACE wins.
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "packages_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	// LANGUAGE PYTHON is required by the grammar and is the only supported value.
	fmt.Fprint(&sb, "\n  LANGUAGE PYTHON")

	if hasToken(cfg.Allowlist) {
		fmt.Fprintf(&sb, "\n  ALLOWLIST = %s", formatStringList(cfg.Allowlist))
	}
	if hasToken(cfg.Blocklist) {
		fmt.Fprintf(&sb, "\n  BLOCKLIST = %s", formatStringList(cfg.Blocklist))
	}
	if hasToken(cfg.AdditionalCreationBlocklist) {
		fmt.Fprintf(&sb, "\n  ADDITIONAL_CREATION_BLOCKLIST = %s", formatStringList(cfg.AdditionalCreationBlocklist))
	}

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeTextLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}
