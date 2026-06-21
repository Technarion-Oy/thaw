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

// AlterStream runs an ALTER STREAM statement for the given stream. clause is
// everything that follows the stream name, e.g. "SET COMMENT = '...'",
// "UNSET COMMENT", or "RENAME TO ...". The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the stream
// identifier.
func (a *App) AlterStream(database, schema, name, clause string) error {
	return a.alterObject("STREAM", database, schema, name, clause)
}
