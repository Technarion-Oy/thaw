// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterModelMonitor runs an ALTER MODEL MONITOR statement for the given monitor.
// clause is everything that follows the monitor name, e.g. "SUSPEND", "RESUME",
// "SET BASELINE = 'base_tbl'", "SET REFRESH_INTERVAL = '1 hour'",
// "SET WAREHOUSE = my_wh", "ADD segment_column = 'region'", or
// "DROP segment_column = 'region'". ALTER MODEL MONITOR has no RENAME, COMMENT,
// or TAG support. The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the monitor identifier.
func (a *App) AlterModelMonitor(database, schema, name, clause string) error {
	return a.alterObject("MODEL MONITOR", database, schema, name, clause)
}
