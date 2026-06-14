// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package maskingpolicy builds SQL for Snowflake masking policy objects —
// CREATE MASKING POLICY statements and the structured config behind them.
// Masking policies are part of Snowflake's column-level governance framework:
// a policy defines a signature (the column type to mask plus any conditional
// columns), a return type that must match the first argument, and a body
// expression that decides — typically from the querying role — whether to
// return the value unchanged or a masked substitute. The ALTER clauses (RENAME,
// SET BODY, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough to be issued as
// free-form ALTER MASKING POLICY statements from internal/app/maskingpolicy.go
// (App.AlterMaskingPolicy); the objects a policy is applied to are read via
// App.GetMaskingPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package maskingpolicy
