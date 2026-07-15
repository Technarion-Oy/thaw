// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterStream runs an ALTER STREAM statement for the given stream. clause is
// everything that follows the stream name, e.g. "SET COMMENT = '...'",
// "UNSET COMMENT", or "RENAME TO ...". The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the stream
// identifier.
func (a *App) AlterStream(database, schema, name, clause string) error {
	return a.alterObject("STREAM", database, schema, name, clause)
}
