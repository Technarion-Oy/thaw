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
	"thaw/internal/hybridtable"
	"thaw/internal/snowflake"
)

// AlterHybridTable runs an ALTER TABLE statement for the given hybrid table.
// Hybrid tables have no dedicated ALTER HYBRID TABLE statement — they are
// altered through the plain TABLE grammar — so clause is everything that
// follows the table name. It accepts any valid ALTER TABLE clause, but the
// properties panel currently uses it only for comment changes
// ("SET COMMENT = 'note'" / "UNSET COMMENT"); RENAME goes through the Sidebar's
// own inline "ALTER TABLE … RENAME TO" path. The caller is responsible for
// correct SQL quoting inside the clause; this method only double-quotes the
// table identifier.
func (a *App) AlterHybridTable(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER TABLE %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// ListHybridTableIndexes returns the indexes defined on the given hybrid table
// via SHOW INDEXES IN TABLE. The raw QueryResult is returned so the properties
// panel can render every column the Snowflake edition reports (typically
// created_on, name, is_unique, columns, included_columns, table) without the
// backend pinning a fixed shape. The primary key surfaces here as an index, so
// this single call covers both the PRIMARY KEY and any secondary indexes.
func (a *App) ListHybridTableIndexes(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW INDEXES IN TABLE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return a.client.Execute(a.ctx, sql)
}

// CreateHybridTableIndex adds a secondary index to an existing hybrid table by
// running CREATE INDEX <name> ON <fqn> (<cols>) [INCLUDE (<cols>)].
func (a *App) CreateHybridTableIndex(database, schema, table string, idx hybridtable.HybridIndex) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := hybridtable.BuildCreateIndexSql(database, schema, table, idx)
	if err != nil {
		return err
	}
	_, err = a.client.Execute(a.ctx, sql)
	return err
}

// DropHybridTableIndex removes a secondary index from a hybrid table by running
// DROP INDEX IF EXISTS <db>.<schema>.<table>.<index>.
func (a *App) DropHybridTableIndex(database, schema, table, index string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := hybridtable.BuildDropIndexSql(database, schema, table, index)
	if err != nil {
		return err
	}
	_, err = a.client.Execute(a.ctx, sql)
	return err
}
