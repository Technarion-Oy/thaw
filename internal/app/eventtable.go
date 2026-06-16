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

// AlterEventTable runs an ALTER TABLE statement for the given event table. Event
// tables have no dedicated ALTER EVENT TABLE statement — they are altered through
// the plain TABLE grammar — so clause is everything that follows the table name.
// It accepts any valid ALTER TABLE clause (e.g. "SET COMMENT = 'note'",
// "UNSET COMMENT", "SET CHANGE_TRACKING = TRUE", "RENAME TO <new>"). The caller
// is responsible for correct SQL quoting inside the clause; this method only
// double-quotes the table identifier.
func (a *App) AlterEventTable(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER TABLE %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}
