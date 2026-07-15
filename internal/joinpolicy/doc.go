// SPDX-License-Identifier: GPL-3.0-or-later

// Package joinpolicy builds SQL for Snowflake join policy objects — CREATE JOIN
// POLICY statements and the structured config behind them. Join policies are a
// governance primitive that restrict which tables and views may be joined
// together, preventing unauthorized correlation across datasets. Unlike masking
// or row access policies, a join policy has a fixed signature: it takes no
// arguments and always RETURNS JOIN_CONSTRAINT, with a body of the form
// JOIN_CONSTRAINT(JOIN_REQUIRED => <boolean_expression>). The ALTER clauses
// (RENAME, SET BODY, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough to be
// issued as free-form ALTER JOIN POLICY statements from
// internal/app/joinpolicy.go (App.AlterJoinPolicy); the objects a policy is
// applied to are read via App.GetJoinPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package joinpolicy
