// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package privacypolicy builds SQL for Snowflake privacy policy objects — CREATE
// PRIVACY POLICY statements and the structured config behind them. Privacy
// policies enforce differential-privacy guarantees on query results, limiting
// the information that can be extracted about individual records by constraining
// a privacy budget. Like join policies, a privacy policy has a fixed signature:
// it takes no arguments and always RETURNS PRIVACY_BUDGET, with a body that calls
// either NO_PRIVACY_POLICY() (unrestricted access) or
// PRIVACY_BUDGET(BUDGET_NAME => '…', …) (an enforced privacy budget). The ALTER
// clauses (RENAME, SET BODY, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough
// to be issued as free-form ALTER PRIVACY POLICY statements from
// internal/app/privacypolicy.go (App.AlterPrivacyPolicy); the objects a policy is
// applied to are read via App.GetPrivacyPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package privacypolicy
