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

// AlterJoinPolicy runs an ALTER JOIN POLICY statement for the given policy.
// clause is everything that follows the policy name, e.g. "RENAME TO <new>",
// "SET BODY -> <expr>", "SET COMMENT = '...'", "UNSET COMMENT",
// "SET TAG <tag> = '...'", or "UNSET TAG <tag>". The caller is responsible for
// correct SQL quoting inside the clause; this method only double-quotes the
// policy identifier.
func (a *App) AlterJoinPolicy(database, schema, name, clause string) error {
	return a.alterObject("JOIN POLICY", database, schema, name, clause)
}

// GetJoinPolicyTags returns the tags currently applied to the given join policy,
// via the INFORMATION_SCHEMA.TAG_REFERENCES table function (object domain
// JOIN POLICY). Unlike the ACCOUNT_USAGE.TAG_REFERENCES view this reflects
// changes immediately (no propagation latency), which suits an interactive tag
// editor. The raw QueryResult is returned (tag_database / tag_schema / tag_name /
// tag_value columns) so the properties modal can render each tag as a removable
// chip. The caller treats an error as "no tags available" and still allows
// SET/UNSET TAG.
func (a *App) GetJoinPolicyTags(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'JOIN POLICY')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		// EscapeTextLit (not EscapeStringLit): QuoteIdent doubles " but not \, so a
		// backslash in an identifier must be doubled to survive the single-quoted
		// literal rather than being read as a Snowflake escape sequence.
		snowflake.QuoteIdent(database), snowflake.EscapeTextLit(fqn))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// GetJoinPolicyReferences returns the tables and views to which the given join
// policy is currently applied, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'JOIN_POLICY'. The view requires governance privileges (e.g. the ACCOUNTADMIN
// role or a grant on the SNOWFLAKE database) and has propagation latency, so
// newly-applied policies may not appear immediately.
func (a *App) GetJoinPolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'JOIN_POLICY' "+
			"ORDER BY REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.QuerySingle(a.fctx(FeatureObjectEditor), query)
}
