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

// FormatStringList renders a token slice into the SQL list grammar used by the
// packages-policy list parameters — each token becomes a single-quoted string
// literal, e.g. []string{"numpy", "pandas"} → "('numpy', 'pandas')". Blank
// tokens are skipped. It delegates to the shared snowflake.FormatStringLitList so
// the CREATE builder and the properties modal (which reaches this over IPC via
// App.FormatPackagesPolicyList) serialize lists through one implementation. Pure
// string handling — no connection required.
func FormatStringList(tokens []string) string { return snowflake.FormatStringLitList(tokens) }

// ParseList tokenizes a DESCRIBE PACKAGES POLICY allow/block-list cell into its
// individual package-spec entries. It is deliberately NOT the generic
// snowflake.ParseSqlList: that runs a SQL tokenizer which discards operator
// tokens, so a bare (unquoted) version specifier such as "numpy==1.26.4" would
// be split into "numpy" and "1.26.4" and mangled. Package specs never contain
// commas, so this instead splits on top-level commas/newlines (after stripping
// one optional surrounding [ ] or ( ) layer) and strips one optional matching
// quote layer per element — preserving the "==", ">=", "<=", "<", ">" operators
// whether or not Snowflake quotes the entries. It is the read counterpart of
// FormatStringList and is robust to every shape DESCRIBE might use:
// ('a', 'b'), ["a","b"], [a==1, b], or a bare comma-separated list. An empty /
// null cell yields nil.
func ParseList(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "null") {
		return nil
	}
	// Strip one surrounding bracket/paren layer if present.
	if n := len(s); n >= 2 {
		if (s[0] == '[' && s[n-1] == ']') || (s[0] == '(' && s[n-1] == ')') {
			s = s[1 : n-1]
		}
	}
	var out []string
	for _, part := range snowflake.SplitValues(s) {
		if v := unquoteSpec(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// unquoteSpec strips one optional surrounding matching quote layer ('…' or "…")
// from a single package-spec token, collapsing the SQL-doubled quote escape, and
// trims surrounding whitespace. A bare (unquoted) token is returned trimmed and
// otherwise verbatim, so its version-specifier operators are preserved.
func unquoteSpec(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 {
		switch {
		case s[0] == '\'' && s[n-1] == '\'':
			s = strings.ReplaceAll(s[1:n-1], "''", "'")
		case s[0] == '"' && s[n-1] == '"':
			s = strings.ReplaceAll(s[1:n-1], `""`, `"`)
		}
	}
	return strings.TrimSpace(s)
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

	if snowflake.HasNonBlankToken(cfg.Allowlist) {
		fmt.Fprintf(&sb, "\n  ALLOWLIST = %s", snowflake.FormatStringLitList(cfg.Allowlist))
	}
	if snowflake.HasNonBlankToken(cfg.Blocklist) {
		fmt.Fprintf(&sb, "\n  BLOCKLIST = %s", snowflake.FormatStringLitList(cfg.Blocklist))
	}
	if snowflake.HasNonBlankToken(cfg.AdditionalCreationBlocklist) {
		fmt.Fprintf(&sb, "\n  ADDITIONAL_CREATION_BLOCKLIST = %s", snowflake.FormatStringLitList(cfg.AdditionalCreationBlocklist))
	}

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = %s", snowflake.QuoteTextLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}
