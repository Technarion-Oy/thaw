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

// AlterTag runs an ALTER TAG statement for the given tag. clause is everything
// that follows the tag name, e.g. "RENAME TO <new>", "SET COMMENT = '...'",
// "UNSET COMMENT", "ADD ALLOWED_VALUES 'a', 'b'", "DROP ALLOWED_VALUES 'a'",
// "UNSET ALLOWED_VALUES", "SET MASKING POLICY <policy>", or "UNSET MASKING
// POLICY <policy>". The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the tag identifier.
func (a *App) AlterTag(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER TAG %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// GetTagReferences returns the objects and columns to which the given tag is
// currently applied, by querying SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES. The
// view requires governance privileges (e.g. the ACCOUNTADMIN role or a grant on
// the SNOWFLAKE database) and has propagation latency, so newly-applied tags may
// not appear immediately. Rows with a non-null OBJECT_DELETED are excluded so
// only live references are returned.
func (a *App) GetTagReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME, DOMAIN, TAG_VALUE "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES "+
			"WHERE TAG_DATABASE = '%s' AND TAG_SCHEMA = '%s' AND TAG_NAME = '%s' AND OBJECT_DELETED IS NULL "+
			"ORDER BY OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return a.client.QuerySingle(a.ctx, query)
}
