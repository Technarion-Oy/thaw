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

// AlterContact runs an ALTER CONTACT statement for the given contact. clause is
// everything that follows the contact name, e.g. "RENAME TO <new>",
// "SET USERS = ('alice', 'bob')", "SET EMAIL_DISTRIBUTION_LIST = '…'",
// "SET URL = '…'", or "SET COMMENT = '…'". Snowflake's ALTER CONTACT supports
// renaming and a SET of the (mutually exclusive) contact method plus the
// comment — there is no UNSET. The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the contact
// identifier.
func (a *App) AlterContact(database, schema, name, clause string) error {
	return a.alterObject("CONTACT", database, schema, name, clause)
}
