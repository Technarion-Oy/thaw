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
	"thaw/internal/cortexsearchservice"
	"thaw/internal/snowflake"
)

// AlterCortexSearchService runs an ALTER CORTEX SEARCH SERVICE statement for the
// given service. clause is everything that follows the service name, e.g.
// "SUSPEND", "RESUME", "REFRESH", "SET TARGET_LAG = '1 hour'",
// "SET WAREHOUSE = \"WH\"", "SET ATTRIBUTES ( COL1, COL2 )", "UNSET ATTRIBUTES",
// "SET COMMENT = '...'", "UNSET COMMENT", "SET TAG ...", or "UNSET TAG ...". The
// caller is responsible for correct SQL quoting inside the clause; this method
// only double-quotes the service identifier. ALTER CORTEX SEARCH SERVICE has no
// RENAME clause.
func (a *App) AlterCortexSearchService(database, schema, name, clause string) error {
	return a.alterObject("CORTEX SEARCH SERVICE", database, schema, name, clause)
}

// FormatCortexSearchAttributes joins the given column names into a comma-separated
// ATTRIBUTES list (without the surrounding parentheses) for the properties
// modal's "SET ATTRIBUTES ( … )" clause, dropping blank entries. Exposed over IPC
// so the frontend doesn't duplicate the trim/skip-blank logic.
func (a *App) FormatCortexSearchAttributes(columns []string) string {
	return cortexsearchservice.FormatAttributes(columns)
}

// GetCortexSearchServiceTags returns the tags currently applied to the given
// cortex search service, via the INFORMATION_SCHEMA.TAG_REFERENCES table function
// (object domain CORTEX SEARCH SERVICE). Unlike the ACCOUNT_USAGE.TAG_REFERENCES
// view this reflects changes immediately (no propagation latency), which suits an
// interactive tag editor. The raw QueryResult is returned (tag_database /
// tag_schema / tag_name / tag_value columns) so the properties modal can render
// each tag as a removable chip. The caller treats an error as "no tags available"
// and still allows SET/UNSET TAG.
func (a *App) GetCortexSearchServiceTags(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'CORTEX SEARCH SERVICE')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		// EscapeTextLit (not EscapeStringLit): QuoteIdent doubles " but not \, so a
		// backslash in an identifier must be doubled to survive the single-quoted
		// literal rather than being read as a Snowflake escape sequence.
		snowflake.QuoteIdent(database), snowflake.EscapeTextLit(fqn))
	return a.client.Execute(a.ctx, sql)
}
