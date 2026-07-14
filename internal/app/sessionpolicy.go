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

// AlterSessionPolicy runs an ALTER SESSION POLICY statement for the given
// policy. clause is everything that follows the policy name, e.g.
// "RENAME TO <new>", "SET SESSION_IDLE_TIMEOUT_MINS = 30",
// "UNSET SESSION_MAX_LIFESPAN_MINS", "SET COMMENT = '...'", "UNSET COMMENT",
// "SET TAG <tag> = '...'", or "UNSET TAG <tag>". The caller is responsible for
// correct SQL quoting inside the clause; this method only double-quotes the
// policy identifier.
func (a *App) AlterSessionPolicy(database, schema, name, clause string) error {
	return a.alterObject("SESSION POLICY", database, schema, name, clause)
}

// DescribeSessionPolicy returns the configured parameter values for the given
// session policy by running DESCRIBE SESSION POLICY. The result is a single row
// whose columns include session_idle_timeout_mins, session_ui_idle_timeout_mins,
// session_max_lifespan_mins, session_ui_max_lifespan_mins, and
// allowed_secondary_roles — SHOW SESSION POLICIES does not report these values,
// so this is how the properties panel reads the current settings.
func (a *App) DescribeSessionPolicy(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf("DESCRIBE SESSION POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.QuerySingle(a.ctx, query)
}

// FormatSecondaryRoles renders a secondary-role list into the
// `( 'ALL' | <role>, … )` SQL grammar via snowflake.FormatSecondaryRoles — the
// "ALL" token becomes the 'ALL' literal and every other entry is quoted only when
// it needs it (reserved keyword or non-identifier). Exposed so the Session Policy
// properties modal builds its ALTER … SET ALLOWED/BLOCKED_SECONDARY_ROLES clause
// (and renders the current value) through the same serializer the CREATE builder
// uses. Pure string handling — no Snowflake connection required.
func (a *App) FormatSecondaryRoles(roles []string) string {
	return snowflake.FormatSecondaryRoles(roles)
}

// ReconcileSecondaryRoles enforces the secondary-role grammar's ALL-vs-role-list
// mutual exclusivity via snowflake.ReconcileSecondaryRoles, keeping whichever kind
// the user picked last. Exposed so the create / properties modals can clean a tag
// selection as it changes and never emit the invalid ('ALL', R1) shape. Pure
// string handling — no Snowflake connection required.
func (a *App) ReconcileSecondaryRoles(roles []string) []string {
	return snowflake.ReconcileSecondaryRoles(roles)
}

// ParseSecondaryRoles parses a secondary-role list cell as returned by
// DESCRIBE SESSION POLICY (e.g. ('ALL'), (R1, "my role"), or a JSON-style
// ["R1","R2"]) into its individual role tokens. It is the inverse of the
// snowflake.FormatSecondaryRoles serializer used by the CREATE/ALTER builders,
// exposed so the Session Policy properties modal round-trips the allowed/blocked
// lists through one shared, quote-aware implementation rather than re-deriving
// the parse in TypeScript. Pure string handling — no Snowflake connection needed.
func (a *App) ParseSecondaryRoles(raw string) []string {
	return snowflake.ParseSecondaryRoles(raw)
}

// GetSessionPolicyReferences returns the users (and/or the account) to which the
// given session policy is currently attached, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'SESSION_POLICY'. The view requires governance privileges (e.g. the
// ACCOUNTADMIN role or a grant on the SNOWFLAKE database) and has propagation
// latency, so a newly-applied policy may not appear immediately.
func (a *App) GetSessionPolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'SESSION_POLICY' "+
			"ORDER BY REF_ENTITY_DOMAIN, REF_ENTITY_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return client.QuerySingle(a.ctx, query)
}
