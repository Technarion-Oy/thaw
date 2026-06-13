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

// AlterMaterializedView runs an ALTER MATERIALIZED VIEW statement for the given
// view. clause is everything that follows the view name, e.g. "SUSPEND",
// "RESUME", "SUSPEND RECLUSTER", "RESUME RECLUSTER", "CLUSTER BY (c1)",
// "DROP CLUSTERING KEY", "SET COMMENT = '...'", "SET SECURE", or
// "UNSET COMMENT". The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the view identifier. Materialized views
// have no manual REFRESH command — Snowflake maintains them automatically.
func (a *App) AlterMaterializedView(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER MATERIALIZED VIEW %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}
