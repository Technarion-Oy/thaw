// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterView runs an ALTER VIEW statement for the given view. clause is everything
// that follows the view name, e.g. "SET SECURE", "UNSET SECURE",
// "SET COMMENT = '...'", "UNSET COMMENT", or "RENAME TO ...". The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the view identifier.
func (a *App) AlterView(database, schema, name, clause string) error {
	return a.alterObject("VIEW", database, schema, name, clause)
}
