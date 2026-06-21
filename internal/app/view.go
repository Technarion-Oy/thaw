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

// AlterView runs an ALTER VIEW statement for the given view. clause is everything
// that follows the view name, e.g. "SET SECURE", "UNSET SECURE",
// "SET COMMENT = '...'", "UNSET COMMENT", or "RENAME TO ...". The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the view identifier.
func (a *App) AlterView(database, schema, name, clause string) error {
	return a.alterObject("VIEW", database, schema, name, clause)
}
