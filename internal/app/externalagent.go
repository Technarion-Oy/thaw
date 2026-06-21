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

// AlterExternalAgent runs an ALTER EXTERNAL AGENT statement for the given
// external agent. clause is everything that follows the agent name, e.g.
// "SET COMMENT = '...'" or "ADD VERSION <name>". ALTER EXTERNAL AGENT has no
// RENAME, UNSET, or TAG clause. The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the agent identifier.
func (a *App) AlterExternalAgent(database, schema, name, clause string) error {
	return a.alterObject("EXTERNAL AGENT", database, schema, name, clause)
}
