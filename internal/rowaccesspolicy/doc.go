// SPDX-License-Identifier: GPL-3.0-or-later

// Package rowaccesspolicy builds SQL for Snowflake row access policy objects —
// CREATE ROW ACCESS POLICY statements and the structured config behind them.
// Row access policies are part of Snowflake's row-level security framework: a
// policy defines a signature (the columns it evaluates), always returns a
// BOOLEAN, and a body expression that decides — typically from the querying
// role — whether a given row is visible. The ALTER clauses (RENAME, SET BODY,
// SET/UNSET COMMENT, SET/UNSET TAG) are simple enough to be issued as free-form
// ALTER ROW ACCESS POLICY statements from internal/app/rowaccesspolicy.go
// (App.AlterRowAccessPolicy); the objects a policy is applied to are read via
// App.GetRowAccessPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package rowaccesspolicy
