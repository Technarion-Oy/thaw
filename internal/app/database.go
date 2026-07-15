// SPDX-License-Identifier: GPL-3.0-or-later

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
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER DATABASE %s %s", snowflake.QuoteIdent(database), clause)
	_, err := client.Execute(a.fctx(FeatureObjectBrowser), sql)
	return err
}

// GetDatabaseParameters returns the database-level parameters via SHOW
// PARAMETERS IN DATABASE. SHOW DATABASES reports only comment / options /
// retention_time, so the properties panel reads the current
// MAX_DATA_EXTENSION_TIME_IN_DAYS, DEFAULT_DDL_COLLATION, LOG_LEVEL, etc. from
// here instead. The raw QueryResult is returned (key / value / default / level
// / … columns) so the caller can pick out the parameters it cares about.
func (a *App) GetDatabaseParameters(database string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW PARAMETERS IN DATABASE %s", snowflake.QuoteIdent(database))
	return client.Execute(a.fctx(FeatureObjectBrowser), sql)
}

// ListEventTables returns the fully-qualified names of all event tables visible
// to the current role account-wide, for the EVENT_TABLE picker in the Database
// Properties modal.
func (a *App) ListEventTables() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListEventTables(a.fctx(FeatureObjectBrowser))
}
