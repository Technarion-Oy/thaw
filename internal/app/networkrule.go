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

// AlterNetworkRule runs an ALTER NETWORK RULE statement for the given rule.
// clause is everything that follows the rule name, e.g.
// "SET VALUE_LIST = ('a', 'b')", "UNSET VALUE_LIST", "SET COMMENT = '...'", or
// "UNSET COMMENT". TYPE and MODE cannot be altered and network rules cannot be
// renamed, so those operations require recreating the rule. The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the rule identifier.
func (a *App) AlterNetworkRule(database, schema, name, clause string) error {
	return a.alterObject("NETWORK RULE", database, schema, name, clause)
}
