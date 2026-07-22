// SPDX-License-Identifier: GPL-3.0-or-later

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
	return a.alterObject("AUTHENTICATION POLICY", database, schema, name, clause)
}

// DescribeAuthenticationPolicy returns the configured parameter values for the
// given authentication policy by running DESCRIBE AUTHENTICATION POLICY, projected
// to property/value pairs (one per row) via snowflake.ResultPropertyValueRows so
// the property/value column indexing stays in Go rather than the modal. SHOW
// AUTHENTICATION POLICIES does not report the parameter values, so this is how the
// properties panel reads the current settings.
func (a *App) DescribeAuthenticationPolicy(database, schema, name string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf("DESCRIBE AUTHENTICATION POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	res, err := client.QuerySingle(a.fctx(FeatureObjectEditor), query)
	if err != nil {
		return nil, err
	}
	return snowflake.ResultPropertyValueRows(res), nil
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

// AuthenticationPolicyBagOptions returns the fixed enumerations the nested
// property-bag editors offer (MFA methods / enforce, PAT network-policy
// evaluation & require-role, workload providers), so the bag editors render their
// allowed-value sets from the Go grammar rather than hardcoding them in
// TypeScript. Pure data.
func (a *App) AuthenticationPolicyBagOptions() authenticationpolicy.BagParamOptions {
	return authenticationpolicy.BagOptions()
}

// AuthenticationPolicyClientDrivers returns the driver/client tokens selectable
// in a CLIENT_POLICY bag — the version-governed subset of the general
// snowflake.ClientDrivers catalog — so the modal's driver picker draws from one
// shared source. Pure data.
func (a *App) AuthenticationPolicyClientDrivers() []string {
	return authenticationpolicy.ClientPolicyDrivers()
}

// AuthenticationPolicyClientDriverVersions runs SYSTEM$CLIENT_VERSION_INFO() and
// returns Snowflake's minimum-supported / recommended versions for the
// CLIENT_POLICY drivers, so the editor can suggest a version instead of the user
// looking it up. Requires a connection; drivers the function doesn't report are
// omitted.
func (a *App) AuthenticationPolicyClientDriverVersions() ([]authenticationpolicy.DriverVersionHint, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	info, err := client.GetClientVersionInfo(a.fctx(FeatureObjectEditor))
	if err != nil {
		return nil, err
	}
	return authenticationpolicy.ClientPolicyDriverVersions(info), nil
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
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	// EscapeTextLit (not EscapeStringLit) for these literal comparisons: Snowflake
	// treats '\' as an escape char inside a string literal, so a name containing a
	// backslash must have it doubled or the WHERE silently matches nothing.
	query := fmt.Sprintf(
		"SELECT REF_ENTITY_NAME, REF_ENTITY_DOMAIN, POLICY_STATUS "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES "+
			"WHERE POLICY_DB = '%s' AND POLICY_SCHEMA = '%s' AND POLICY_NAME = '%s' AND POLICY_KIND = 'AUTHENTICATION_POLICY' "+
			"ORDER BY REF_ENTITY_DOMAIN, REF_ENTITY_NAME",
		snowflake.EscapeTextLit(database), snowflake.EscapeTextLit(schema), snowflake.EscapeTextLit(name))
	return client.QuerySingle(a.fctx(FeatureObjectEditor), query)
}

// ListAccountAuthenticationPolicies returns every authentication policy in the
// account via SHOW AUTHENTICATION POLICIES IN ACCOUNT (name, database_name,
// schema_name, …). It backs the authentication-policy picker in the user
// properties modal, mirroring ListAccountMaskingPolicies. The command requires
// privileges on the policies, so accounts without governance access may see only
// a subset.
func (a *App) ListAccountAuthenticationPolicies() (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.QuerySingle(a.fctx(FeatureUsersRoles), "SHOW AUTHENTICATION POLICIES IN ACCOUNT")
}
