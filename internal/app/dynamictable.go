// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterDynamicTable runs an ALTER DYNAMIC TABLE statement for the given table.
// clause is everything that follows the table name, e.g. "SUSPEND", "RESUME",
// "REFRESH", "SET TARGET_LAG = '5 minutes'", or "SET WAREHOUSE = my_wh". The
// caller is responsible for correct SQL quoting inside the clause; this method
// only double-quotes the table identifier.
func (a *App) AlterDynamicTable(database, schema, name, clause string) error {
	return a.alterObject("DYNAMIC TABLE", database, schema, name, clause)
}
