// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterDatabase runs an ALTER DATABASE statement for the given database.
// Databases are one-level, so this is simpler than AlterSchema — no schema
// part. clause is everything that follows the database name, e.g.
// "SET COMMENT = '...'", "UNSET COMMENT", "SET DATA_RETENTION_TIME_IN_DAYS = 7",
// "RENAME TO <new>", "SWAP WITH <target>", or "ENABLE REPLICATION TO ACCOUNTS
// <acct>". The caller is responsible for correct SQL quoting inside the clause;
// this method only double-quotes the database identifier.
func (a *App) AlterDatabase(database, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER DATABASE %s %s", snowflake.QuoteIdent(database), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// GetDatabaseParameters returns the database-level parameters via SHOW
// PARAMETERS IN DATABASE. SHOW DATABASES reports only comment / options /
// retention_time, so the properties panel reads the current
// MAX_DATA_EXTENSION_TIME_IN_DAYS, DEFAULT_DDL_COLLATION, LOG_LEVEL, etc. from
// here instead. The raw QueryResult is returned (key / value / default / level
// / … columns) so the caller can pick out the parameters it cares about.
func (a *App) GetDatabaseParameters(database string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW PARAMETERS IN DATABASE %s", snowflake.QuoteIdent(database))
	return a.client.Execute(a.ctx, sql)
}
