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

// AlterFunction runs an ALTER FUNCTION statement for the given user-defined
// function. clause is everything that follows the function signature, e.g.
// "SET COMMENT = '...'", "UNSET COMMENT", "SET SECURE", "UNSET SECURE", or
// "RENAME TO <new_name>". args is the parameter type list (e.g. "NUMBER, VARCHAR")
// that resolves the overload — it may be empty for a zero-argument function, but
// the parentheses are always required by Snowflake. The caller is responsible for
// correct SQL quoting inside the clause; this method double-quotes the function
// identifier and interpolates args into the signature parentheses.
func (a *App) AlterFunction(database, schema, name, args, clause string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER FUNCTION %s.%s.%s(%s) %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args, clause)
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}
