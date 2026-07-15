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

// AlterDataMetricFunction runs an ALTER FUNCTION statement for the given data
// metric function. DMFs have no dedicated ALTER DATA METRIC FUNCTION statement —
// they are altered through the plain FUNCTION grammar — so clause is everything
// that follows the function signature (e.g. "SET COMMENT = 'note'", "UNSET
// COMMENT", "SET SECURE", "UNSET SECURE", "RENAME TO new_name"). args is the
// TABLE argument signature that resolves the overload (e.g. "TABLE(NUMBER)"); DMFs
// always have at least one argument. The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the function
// identifier and interpolates args into the signature parentheses.
func (a *App) AlterDataMetricFunction(database, schema, name, args, clause string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER FUNCTION %s.%s.%s(%s) %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args, clause)
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}

// DescribeDataMetricFunction returns the DESCRIBE FUNCTION result for a data
// metric function. SHOW DATA METRIC FUNCTIONS reports only a minimal column set
// and omits the body expression, so the properties panel reads the signature,
// return type, language, and body from DESCRIBE instead. args is the TABLE
// argument signature that resolves the overload (e.g. "TABLE(NUMBER)"). The raw
// QueryResult is returned (property / value columns) so the caller can render
// every row without the backend pinning a fixed shape.
func (a *App) DescribeDataMetricFunction(database, schema, name, args string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE FUNCTION %s.%s.%s(%s)",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args)
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// GetDataMetricFunctionReferences returns the tables and views the given data
// metric function is scheduled against, from
// SNOWFLAKE.ACCOUNT_USAGE.DATA_METRIC_FUNCTION_REFERENCES. A DMF stores no
// associations itself — they live on the table side (ALTER TABLE … ADD DATA
// METRIC FUNCTION) — so this account-usage view is the only place to enumerate
// them by DMF. Reads require access to the SNOWFLAKE database (governance
// privilege) and the view carries the usual ACCOUNT_USAGE latency; the caller
// renders the result on demand and treats an error as "no associations
// available". The raw QueryResult is returned so the caller can render every
// column without the backend pinning a fixed shape.
func (a *App) GetDataMetricFunctionReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf(
		"SELECT ref_database_name, ref_schema_name, ref_entity_name, ref_entity_domain, schedule "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.DATA_METRIC_FUNCTION_REFERENCES "+
			"WHERE metric_database_name = '%s' AND metric_schema_name = '%s' AND metric_name = '%s' "+
			"ORDER BY ref_database_name, ref_schema_name, ref_entity_name",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// GetDataMetricFunctionTags returns the tags currently applied to the given data
// metric function, via the INFORMATION_SCHEMA.TAG_REFERENCES table function
// (object domain FUNCTION). Unlike the ACCOUNT_USAGE.TAG_REFERENCES view this
// reflects changes immediately (no propagation latency), which suits an
// interactive tag editor. args is the TABLE argument signature that resolves the
// overload. The raw QueryResult is returned (tag_database / tag_schema / tag_name
// / tag_value columns) so the properties modal can render each tag as a removable
// chip.
func (a *App) GetDataMetricFunctionTags(database, schema, name, args string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s(%s)",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args)
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'FUNCTION')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		snowflake.QuoteIdent(database), snowflake.EscapeStringLit(fqn))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}
