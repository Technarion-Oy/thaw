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

// AlterAlert runs an ALTER ALERT statement for the given alert. clause is
// everything that follows the alert name, e.g. "RESUME", "SUSPEND",
// "SET WAREHOUSE = ...", "SET SCHEDULE = '...'", "SET COMMENT = '...'",
// "UNSET WAREHOUSE", "UNSET COMMENT", "MODIFY CONDITION EXISTS (...)", or
// "MODIFY ACTION ...". The caller is responsible for correct SQL quoting inside
// the clause; this method only double-quotes the alert identifier. ALTER ALERT
// has no RENAME variant, and EXECUTE is a separate statement (see ExecuteAlert).
func (a *App) AlterAlert(database, schema, name, clause string) error {
	return a.alterObject("ALERT", database, schema, name, clause)
}

// ExecuteAlert manually triggers an immediate evaluation of the given alert via
// the standalone EXECUTE ALERT statement (this is its own SQL command, not an
// ALTER ALERT clause).
func (a *App) ExecuteAlert(database, schema, name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("EXECUTE ALERT %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}
