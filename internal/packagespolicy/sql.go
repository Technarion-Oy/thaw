// SPDX-License-Identifier: GPL-3.0-or-later

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
// individual package-spec entries (exposed over IPC via App.ParsePackagesPolicyList).
// It delegates to snowflake.ParseSqlListVerbatim — the tokenizer-driven list
// parser that preserves each element's verbatim text — rather than the general
// snowflake.ParseSqlList, whose tokenizer discards operator tokens and would split
// a bare version specifier such as "numpy==1.26.4" into "numpy"/"1.26.4". This is
// the read counterpart of FormatStringList and is robust to every shape DESCRIBE
// might use: ('a', 'b'), ["a","b"], [a==1, b], or a bare comma list.
func ParseList(raw string) []string { return snowflake.ParseSqlListVerbatim(raw) }

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

	createClause := snowflake.CreateClause("PACKAGES POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "packages_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

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

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
