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

// AlterSchema runs an ALTER SCHEMA statement for the given schema. Schemas are
// two-level (<db>.<schema>), so this cannot reuse the three-level alterObject
// helper. clause is everything that follows the schema name, e.g.
// "SET COMMENT = '...'", "UNSET COMMENT", "SET DATA_RETENTION_TIME_IN_DAYS = 7",
// "ENABLE MANAGED ACCESS", or "RENAME TO <db>.<new>". The caller is responsible
// for correct SQL quoting inside the clause; this method only double-quotes the
// schema identifier.
func (a *App) AlterSchema(database, schema, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER SCHEMA %s %s", snowflake.Qualify(database, schema), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// GetSchemaParameters returns the schema-level parameters via SHOW PARAMETERS IN
// SCHEMA. SHOW SCHEMAS reports only comment / options / retention_time, so the
// properties panel reads the current MAX_DATA_EXTENSION_TIME_IN_DAYS and
// DEFAULT_DDL_COLLATION (and retention as a fallback) from here instead. The raw
// QueryResult is returned (key / value / default / level / … columns) so the
// caller can pick out the parameters it cares about without the backend pinning
// a fixed shape.
func (a *App) GetSchemaParameters(database, schema string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW PARAMETERS IN SCHEMA %s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema))
	return a.client.Execute(a.ctx, sql)
}
