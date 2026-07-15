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

// AlterMaskingPolicy runs an ALTER MASKING POLICY statement for the given
// policy. clause is everything that follows the policy name, e.g.
// "RENAME TO <new>", "SET BODY -> <expr>", "SET COMMENT = '...'",
// "UNSET COMMENT", "SET TAG <tag> = '...'", or "UNSET TAG <tag>". The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the policy identifier.
func (a *App) AlterMaskingPolicy(database, schema, name, clause string) error {
	return a.alterObject("MASKING POLICY", database, schema, name, clause)
}

// ListAccountMaskingPolicies returns every masking policy in the account via
// SHOW MASKING POLICIES IN ACCOUNT (name, database_name, schema_name, owner,
// comment, …). It backs the masking-policy picker in the column properties
// editor. The command requires privileges on the policies, so accounts without
// governance access may see only a subset.
func (a *App) ListAccountMaskingPolicies() (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.QuerySingle(a.fctx(FeatureObjectEditor), "SHOW MASKING POLICIES IN ACCOUNT")
}

// GetMaskingPolicyReferences returns the columns to which the given masking
// policy is currently applied, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'MASKING_POLICY'. The view requires governance privileges (e.g. the
// ACCOUNTADMIN role or a grant on the SNOWFLAKE database) and has propagation
// latency, so newly-applied policies may not appear immediately.
func (a *App) GetMaskingPolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME, REF_ENTITY_DOMAIN, REF_COLUMN_NAME, REF_ARG_COLUMN_NAMES, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'MASKING_POLICY' "+
			"ORDER BY REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME, REF_COLUMN_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.QuerySingle(a.fctx(FeatureObjectEditor), query)
}
