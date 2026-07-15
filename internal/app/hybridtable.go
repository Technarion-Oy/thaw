// SPDX-License-Identifier: GPL-3.0-or-later

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
	return a.alterObject("TABLE", database, schema, name, clause)
}

// ListHybridTableIndexes returns the indexes defined on the given hybrid table
// via SHOW INDEXES IN TABLE. The raw QueryResult is returned so the properties
// panel can render every column the Snowflake edition reports (typically
// created_on, name, is_unique, columns, included_columns, table) without the
// backend pinning a fixed shape. The primary key surfaces here as an index, so
// this single call covers both the PRIMARY KEY and any secondary indexes.
func (a *App) ListHybridTableIndexes(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW INDEXES IN TABLE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// CreateHybridTableIndex adds a secondary index to an existing hybrid table by
// running CREATE INDEX <name> ON <fqn> (<cols>) [INCLUDE (<cols>)]. caseSensitive
// controls how the index name is quoted (the columns are always double-quoted as
// catalog-canonical names), mirroring the inline-index path so the same typed
// name produces the same stored identifier at create time and afterwards.
func (a *App) CreateHybridTableIndex(database, schema, table string, idx hybridtable.HybridIndex, caseSensitive bool) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := hybridtable.BuildCreateIndexSql(database, schema, table, idx, caseSensitive)
	if err != nil {
		return err
	}
	_, err = client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}

// HybridIndexColumnOptions partitions the given columns into those eligible as
// hybrid-table index key columns vs. INCLUDE columns, applying Snowflake's
// per-role datatype restrictions (semi-structured / geospatial barred from both;
// VECTOR / TIMESTAMP_TZ additionally barred from keys). It is a pure helper (no
// connection required) that the index editors call to populate their dropdowns.
func (a *App) HybridIndexColumnOptions(columns []hybridtable.IndexColumn) hybridtable.IndexColumnOptions {
	return hybridtable.EligibleIndexColumns(columns)
}

// DropHybridTableIndex removes a secondary index from a hybrid table by running
// DROP INDEX IF EXISTS <db>.<schema>.<table>.<index>.
func (a *App) DropHybridTableIndex(database, schema, table, index string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := hybridtable.BuildDropIndexSql(database, schema, table, index)
	if err != nil {
		return err
	}
	_, err = client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}
