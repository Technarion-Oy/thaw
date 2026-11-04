// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/apperrors"
	"thaw/internal/dynamictable"
)

// AlterDynamicTable runs an ALTER DYNAMIC TABLE statement for the given table.
// clause is everything that follows the table name, e.g. "SUSPEND", "RESUME",
// "REFRESH", "SET TARGET_LAG = '5 minutes'", or "SET WAREHOUSE = my_wh". The
// caller is responsible for correct SQL quoting inside the clause; this method
// only double-quotes the table identifier.
func (a *App) AlterDynamicTable(database, schema, name, clause string) error {
	return a.alterObject("DYNAMIC TABLE", database, schema, name, clause)
}

// GetDynamicTableRefreshHistory returns recent refreshes of a dynamic table from
// INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY() over the last days days (max 7).
func (a *App) GetDynamicTableRefreshHistory(database, schema, name string, days int) ([]dynamictable.RefreshHistoryRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return dynamictable.GetDynamicTableRefreshHistory(a.fctx(FeatureObjectBrowser), client, database, schema, name, days)
}
