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
	"thaw/internal/authenticationpolicy"
	"thaw/internal/snowflake"
)

// AlterAuthenticationPolicy runs an ALTER AUTHENTICATION POLICY statement for
// the given policy. clause is everything that follows the policy name, e.g.
// "RENAME TO <new>", "SET AUTHENTICATION_METHODS = ('PASSWORD')",
// "UNSET CLIENT_TYPES", "SET MFA_ENROLLMENT = REQUIRED",
// "SET COMMENT = '...'", or "UNSET COMMENT". The caller is responsible for
// correct SQL quoting inside the clause; this method only double-quotes the
// policy identifier.
func (a *App) AlterAuthenticationPolicy(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER AUTHENTICATION POLICY %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// DescribeAuthenticationPolicy returns the configured parameter values for the
// given authentication policy by running DESCRIBE AUTHENTICATION POLICY. The
// result has one row per property with the columns property and value —
// SHOW AUTHENTICATION POLICIES does not report the parameter values, so this is
// how the properties panel reads the current settings.
func (a *App) DescribeAuthenticationPolicy(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf("DESCRIBE AUTHENTICATION POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return a.client.QuerySingle(a.ctx, query)
}

// FormatAuthPolicyList renders a token slice into the `('A', 'B')`
// single-quoted-literal list grammar via authenticationpolicy.FormatStringList —
// the same serializer the CREATE builder uses. Exposed so the Authentication
// Policy properties modal builds its ALTER … SET <list> = (…) clause (and
// renders the current value) through one shared implementation. Pure string
// handling — no Snowflake connection required.
func (a *App) FormatAuthPolicyList(tokens []string) string {
	return authenticationpolicy.FormatStringList(tokens)
}

// AuthenticationPolicyListParams returns the metadata (ALTER keyword, label,
// allowed-value enumeration, free-form flag) for the policy's top-level list
// parameters, so the properties modal renders the editors from one source of
// truth instead of duplicating the grammar's allowed values in TypeScript. Pure
// data — no Snowflake connection required.
func (a *App) AuthenticationPolicyListParams() []authenticationpolicy.ListParamMeta {
	return authenticationpolicy.ListParams()
}

// AuthenticationPolicyMFAEnrollmentOptions returns the allowed values for the
// MFA_ENROLLMENT scalar parameter (Snowflake default OPTIONAL). Pure data.
func (a *App) AuthenticationPolicyMFAEnrollmentOptions() []string {
	return authenticationpolicy.MFAEnrollmentOptions()
}

// The Build*Value / Parse* methods below convert the four nested property-bag
// parameters (MFA_POLICY, PAT_POLICY, WORKLOAD_IDENTITY_POLICY, CLIENT_POLICY)
// between their structured form and SQL, so the properties modal keeps no SQL
// serialization or DESCRIBE-parsing logic. Each is a pure delegator to
// internal/authenticationpolicy — no Snowflake connection required. The frontend
// feeds a Build*Value result into AlterAuthenticationPolicy(… "SET <BAG> = " + v).

// BuildMFAPolicyValue serializes an MFA_POLICY bag into its `( … )` SET value.
func (a *App) BuildMFAPolicyValue(p authenticationpolicy.MFAPolicy) string {
	return authenticationpolicy.BuildMFAPolicyValue(p)
}

// ParseMFAPolicy reads a DESCRIBE MFA_POLICY value back into the struct.
func (a *App) ParseMFAPolicy(raw string) authenticationpolicy.MFAPolicy {
	return authenticationpolicy.ParseMFAPolicy(raw)
}

// BuildPATPolicyValue serializes a PAT_POLICY bag into its `( … )` SET value.
func (a *App) BuildPATPolicyValue(p authenticationpolicy.PATPolicy) string {
	return authenticationpolicy.BuildPATPolicyValue(p)
}

// ParsePATPolicy reads a DESCRIBE PAT_POLICY value back into the struct.
func (a *App) ParsePATPolicy(raw string) authenticationpolicy.PATPolicy {
	return authenticationpolicy.ParsePATPolicy(raw)
}

// BuildWorkloadIdentityPolicyValue serializes a WORKLOAD_IDENTITY_POLICY bag.
func (a *App) BuildWorkloadIdentityPolicyValue(p authenticationpolicy.WorkloadIdentityPolicy) string {
	return authenticationpolicy.BuildWorkloadIdentityPolicyValue(p)
}

// ParseWorkloadIdentityPolicy reads a DESCRIBE WORKLOAD_IDENTITY_POLICY value.
func (a *App) ParseWorkloadIdentityPolicy(raw string) authenticationpolicy.WorkloadIdentityPolicy {
	return authenticationpolicy.ParseWorkloadIdentityPolicy(raw)
}

// BuildClientPolicyValue serializes a CLIENT_POLICY bag into its `( … )` value.
func (a *App) BuildClientPolicyValue(p authenticationpolicy.ClientPolicy) string {
	return authenticationpolicy.BuildClientPolicyValue(p)
}

// ParseClientPolicy reads a DESCRIBE CLIENT_POLICY value back into the struct.
func (a *App) ParseClientPolicy(raw string) authenticationpolicy.ClientPolicy {
	return authenticationpolicy.ParseClientPolicy(raw)
}

// GetAuthenticationPolicyReferences returns the users (and/or the account) to
// which the given authentication policy is currently attached, by querying
// SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES filtered to POLICY_KIND =
// 'AUTHENTICATION_POLICY'. The view requires governance privileges (e.g. the
// ACCOUNTADMIN role or a grant on the SNOWFLAKE database) and has propagation
// latency, so a newly-applied policy may not appear immediately.
func (a *App) GetAuthenticationPolicyReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'AUTHENTICATION_POLICY' "+
			"ORDER BY REF_ENTITY_DOMAIN, REF_ENTITY_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return a.client.QuerySingle(a.ctx, query)
}
