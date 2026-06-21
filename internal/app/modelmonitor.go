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
