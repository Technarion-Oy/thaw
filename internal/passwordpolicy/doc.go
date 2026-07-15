// SPDX-License-Identifier: GPL-3.0-or-later

// Package passwordpolicy builds SQL for Snowflake password policy objects —
// CREATE PASSWORD POLICY statements and the structured config behind them.
// Password policies are schema-level governance objects that define the
// complexity (minimum length, required character classes), age, retry/lockout,
// and reuse-history rules enforced when users set or change their Snowflake
// password. Every parameter is optional in the CREATE grammar; an unspecified
// parameter falls back to Snowflake's documented default, so the builder emits
// only the parameters the caller has explicitly set (modeled as *int fields).
// The ALTER clauses (RENAME, SET/UNSET each parameter, SET/UNSET COMMENT,
// SET/UNSET TAG) are simple enough to be issued as free-form ALTER PASSWORD
// POLICY statements from internal/app/passwordpolicy.go (App.AlterPasswordPolicy);
// the configured values are read back via App.DescribePasswordPolicy and the
// users/account the policy is attached to via App.GetPasswordPolicyReferences
// (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package passwordpolicy
