// SPDX-License-Identifier: GPL-3.0-or-later

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
