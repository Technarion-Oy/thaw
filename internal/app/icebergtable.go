// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterIcebergTable runs an ALTER ICEBERG TABLE statement for the given table.
// clause is everything that follows the table name, e.g. "REFRESH",
// "SET COMMENT = 'note'", "UNSET COMMENT", or
// "RENAME TO \"DB\".\"SC\".NEW_NAME". The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the table
// identifier.
func (a *App) AlterIcebergTable(database, schema, name, clause string) error {
	return a.alterObject("ICEBERG TABLE", database, schema, name, clause)
}
