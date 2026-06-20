// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"thaw/internal/packagespolicy"
)

// AlterPackagesPolicy runs an ALTER PACKAGES POLICY statement for the given
// policy. clause is everything that follows the policy name, e.g.
// "SET ALLOWLIST = ('numpy')", "UNSET BLOCKLIST", "SET COMMENT = '...'", or
// "UNSET COMMENT". Packages policies have no RENAME or TAG support. The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the policy identifier.
func (a *App) AlterPackagesPolicy(database, schema, name, clause string) error {
	return a.alterObject("PACKAGES POLICY", database, schema, name, clause)
}

// FormatPackagesPolicyList renders a token slice into the `('A', 'B')`
// single-quoted-literal list grammar via packagespolicy.FormatStringList — the
// same serializer the CREATE builder uses. Exposed so the Packages Policy
// properties modal builds its ALTER … SET <list> = (…) clause (and renders the
// current value) through one shared implementation. Pure string handling — no
// Snowflake connection required.
func (a *App) FormatPackagesPolicyList(tokens []string) string {
	return packagespolicy.FormatStringList(tokens)
}

// ParsePackagesPolicyList tokenizes a DESCRIBE PACKAGES POLICY allow/block-list
// cell into its individual package-spec entries via packagespolicy.ParseList.
// Unlike the general App.ParseSqlList, it preserves version-specifier operators
// (e.g. "numpy==1.26.4") whether or not Snowflake quotes the list entries, so the
// properties modal can't mangle a spec when reading the current value back. Pure
// string handling — no Snowflake connection required.
func (a *App) ParsePackagesPolicyList(raw string) []string {
	return packagespolicy.ParseList(raw)
}
