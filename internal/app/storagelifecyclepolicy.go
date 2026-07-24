// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterStorageLifecyclePolicy runs an ALTER STORAGE LIFECYCLE POLICY statement
// for the given policy. clause is everything that follows the policy name, e.g.
// "RENAME TO <new>", "SET BODY -> <expr>", "SET ARCHIVE_TIER = COLD",
// "SET ARCHIVE_FOR_DAYS = 180", "UNSET ARCHIVE_FOR_DAYS", "SET COMMENT = '...'",
// "UNSET COMMENT", "SET TAG <tag> = '...'", or "UNSET TAG <tag>". The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the policy identifier.
func (a *App) AlterStorageLifecyclePolicy(database, schema, name, clause string) error {
	return a.alterObject("STORAGE LIFECYCLE POLICY", database, schema, name, clause)
}

// GetStorageLifecyclePolicyReferences returns the tables to which the given
// storage lifecycle policy is currently applied, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'STORAGE_LIFECYCLE_POLICY'. The view requires governance privileges (e.g. the
// ACCOUNTADMIN role or a grant on the SNOWFLAKE database) and has propagation
// latency, so newly-applied policies may not appear immediately.
func (a *App) GetStorageLifecyclePolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'STORAGE_LIFECYCLE_POLICY' "+
			"ORDER BY REF_DATABASE_NAME, REF_SCHEMA_NAME, REF_ENTITY_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.QuerySingle(a.fctx(FeatureObjectEditor), query)
}
