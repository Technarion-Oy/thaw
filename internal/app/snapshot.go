// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "thaw/internal/snowflake"

// DescribeSnapshot runs DESCRIBE SNAPSHOT and returns the raw QueryResult for
// the properties panel.
func (a *App) DescribeSnapshot(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.execObjectSQL("DESCRIBE SNAPSHOT " + snowflake.Qualify(database, schema, name))
}

// AlterSnapshot runs ALTER SNAPSHOT <fqn> <clause>. SET COMMENT = '…' is the
// entire ALTER SNAPSHOT surface (no RENAME, no UNSET); the caller quotes the
// comment literal.
func (a *App) AlterSnapshot(database, schema, name, clause string) error {
	return a.alterObject("SNAPSHOT", database, schema, name, clause)
}

// UndropSnapshot restores the most recently dropped snapshot of that name.
func (a *App) UndropSnapshot(database, schema, name string) error {
	_, err := a.execObjectSQL("UNDROP SNAPSHOT " + snowflake.Qualify(database, schema, name))
	return err
}

// ListSnapshots runs SHOW SNAPSHOTS IN ACCOUNT for the Container Services
// dialog's Snapshots tab.
func (a *App) ListSnapshots() (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW SNAPSHOTS IN ACCOUNT")
}

// DropSnapshot runs DROP SNAPSHOT <fqn>.
func (a *App) DropSnapshot(database, schema, name string) error {
	_, err := a.execPoolSQL("DROP SNAPSHOT " + snowflake.Qualify(database, schema, name))
	return err
}
