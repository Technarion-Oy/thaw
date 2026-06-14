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

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterNetworkRule runs an ALTER NETWORK RULE statement for the given rule.
// clause is everything that follows the rule name, e.g.
// "SET VALUE_LIST = ('a', 'b')", "UNSET VALUE_LIST", "SET COMMENT = '...'", or
// "UNSET COMMENT". TYPE and MODE cannot be altered and network rules cannot be
// renamed, so those operations require recreating the rule. The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the rule identifier.
func (a *App) AlterNetworkRule(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER NETWORK RULE %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}
