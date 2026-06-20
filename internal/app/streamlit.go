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

// AlterStreamlit runs an ALTER STREAMLIT statement for the given app. clause is
// everything that follows the streamlit name, e.g. "SET QUERY_WAREHOUSE = MY_WH",
// "UNSET TITLE", or "RENAME TO \"DB\".\"SC\".NEW_NAME". The caller is responsible
// for correct SQL quoting inside the clause; this method only double-quotes the
// streamlit identifier.
func (a *App) AlterStreamlit(database, schema, name, clause string) error {
	return a.alterObject("STREAMLIT", database, schema, name, clause)
}
