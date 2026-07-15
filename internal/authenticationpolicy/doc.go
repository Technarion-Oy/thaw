// SPDX-License-Identifier: GPL-3.0-or-later

// Package authenticationpolicy builds SQL for Snowflake authentication policy
// objects — CREATE AUTHENTICATION POLICY statements and the structured config
// behind them. Authentication policies are schema-level governance objects that
// restrict how users (or the whole account) may authenticate: which
// authentication methods are permitted (PASSWORD, SAML, OAUTH, KEYPAIR, …),
// which client types may connect (the Snowsight UI, drivers, SnowSQL, …), which
// security integrations are allowed, and whether multi-factor authentication
// enrollment is required. A policy is attached to the account or to individual
// users via ALTER ACCOUNT / ALTER USER … SET AUTHENTICATION POLICY.
//
// Every parameter is optional in the CREATE grammar; an unspecified parameter
// falls back to Snowflake's documented default (most default to ALL), so the
// builder emits only the parameters the caller has explicitly set. The
// list-valued parameters are modeled as []string slices of bare tokens
// (rendered as single-quoted string literals), and MFA_ENROLLMENT is a single
// enumerated keyword. The ALTER clauses (RENAME, SET/UNSET each parameter,
// SET/UNSET COMMENT) are simple enough to be issued as free-form ALTER
// AUTHENTICATION POLICY statements from internal/app/authenticationpolicy.go
// (App.AlterAuthenticationPolicy); the configured values are read back via
// App.DescribeAuthenticationPolicy and the users/account the policy is attached
// to via App.GetAuthenticationPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package authenticationpolicy
