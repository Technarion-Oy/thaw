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
	"thaw/internal/externalfunction"
	"thaw/internal/snowflake"
)

// AlterExternalFunction runs an ALTER FUNCTION statement for the given external
// function. External functions have no dedicated ALTER EXTERNAL FUNCTION
// statement — they are altered through the plain FUNCTION grammar — so clause is
// everything that follows the function signature (e.g. "SET COMMENT = 'note'",
// "UNSET COMMENT", "SET SECURE", "SET API_INTEGRATION = my_api"). args is the
// parameter type list (e.g. "NUMBER, VARCHAR") that resolves the overload; it may
// be empty for a zero-argument function. The caller is responsible for correct
// SQL quoting inside the clause; this method only double-quotes the function
// identifier and interpolates args into the signature parentheses.
func (a *App) AlterExternalFunction(database, schema, name, args, clause string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER FUNCTION %s.%s.%s(%s) %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args, clause)
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}

// DescribeExternalFunction returns the DESCRIBE FUNCTION result for an external
// function. SHOW EXTERNAL FUNCTIONS reports only a minimal column set and omits
// the transport detail (API integration, URL/body, headers, context headers,
// max batch rows, compression, request/response translators), so the properties
// panel reads those from DESCRIBE instead. args is the parameter type list that
// resolves the overload (empty for a zero-argument function). The raw QueryResult
// is returned (property / value columns) so the caller can render every row
// without the backend pinning a fixed shape.
func (a *App) DescribeExternalFunction(database, schema, name, args string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE FUNCTION %s.%s.%s(%s)",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), args)
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// GetExternalFunctionOptions returns the fixed choice lists (compression, null
// handling, volatility, context-header functions) the create-external-function
// modal renders as dropdowns. It needs no connection — the lists are static
// properties of the CREATE EXTERNAL FUNCTION grammar.
func (a *App) GetExternalFunctionOptions() externalfunction.BuilderOptions {
	return externalfunction.GetBuilderOptions()
}

// ListUserFunctions returns the user-defined functions (SHOW USER FUNCTIONS) in
// the given database, used to populate the request/response translator pickers in
// the external function builder. Pass an empty database to use the session's
// current scope.
func (a *App) ListUserFunctions(database string) ([]snowflake.UserFunction, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListUserFunctions(a.fctx(FeatureObjectEditor), database)
}
