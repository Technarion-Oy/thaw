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

// AlterPasswordPolicy runs an ALTER PASSWORD POLICY statement for the given
// policy. clause is everything that follows the policy name, e.g.
// "RENAME TO <new>", "SET PASSWORD_MIN_LENGTH = 12", "UNSET PASSWORD_HISTORY",
// "SET COMMENT = '...'", "UNSET COMMENT", "SET TAG <tag> = '...'", or
// "UNSET TAG <tag>". The caller is responsible for correct SQL quoting inside
// the clause; this method only double-quotes the policy identifier.
func (a *App) AlterPasswordPolicy(database, schema, name, clause string) error {
	return a.alterObject("PASSWORD POLICY", database, schema, name, clause)
}

// DescribePasswordPolicy returns the configured parameter values for the given
// password policy by running DESCRIBE PASSWORD POLICY. The result has one row
// per parameter with the columns property, value, default, and description —
// SHOW PASSWORD POLICIES does not report the parameter values, so this is how
// the properties panel reads the current settings alongside their defaults.
func (a *App) DescribePasswordPolicy(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf("DESCRIBE PASSWORD POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.QuerySingle(a.ctx, query)
}

// GetPasswordPolicyReferences returns the users (and/or the account) to which
// the given password policy is currently attached, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'PASSWORD_POLICY'. The view requires governance privileges (e.g. the
// ACCOUNTADMIN role or a grant on the SNOWFLAKE database) and has propagation
// latency, so a newly-applied policy may not appear immediately.
func (a *App) GetPasswordPolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'PASSWORD_POLICY' "+
			"ORDER BY REF_ENTITY_DOMAIN, REF_ENTITY_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.QuerySingle(a.ctx, query)
}
