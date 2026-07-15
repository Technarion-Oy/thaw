// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterStreamlit runs an ALTER STREAMLIT statement for the given app. clause is
// everything that follows the streamlit name, e.g. "SET QUERY_WAREHOUSE = MY_WH",
// "UNSET TITLE", or "RENAME TO \"DB\".\"SC\".NEW_NAME". The caller is responsible
// for correct SQL quoting inside the clause; this method only double-quotes the
// streamlit identifier.
func (a *App) AlterStreamlit(database, schema, name, clause string) error {
	return a.alterObject("STREAMLIT", database, schema, name, clause)
}
