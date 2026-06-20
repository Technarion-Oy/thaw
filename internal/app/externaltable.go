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

// AlterExternalTable runs an ALTER EXTERNAL TABLE statement for the given table.
// clause is everything that follows the table name, e.g. "REFRESH",
// "REFRESH '2024/01/'", or "SET AUTO_REFRESH = TRUE". The caller is responsible
// for correct SQL quoting inside the clause; this method only double-quotes the
// table identifier. Note: the ALTER EXTERNAL TABLE grammar does not accept
// SET/UNSET COMMENT or RENAME TO — comments are changed via COMMENT ON TABLE.
func (a *App) AlterExternalTable(database, schema, name, clause string) error {
	return a.alterObject("EXTERNAL TABLE", database, schema, name, clause)
}
