// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package sessionpolicy builds SQL for Snowflake session policy objects — CREATE
// SESSION POLICY statements and the structured config behind them. Session
// policies are schema-level governance objects that control session behavior:
// the idle timeout (for programmatic clients and for the Snowsight UI), the
// maximum session lifespan (likewise split into a client and a UI value), and
// which secondary roles may be activated in a session. A policy is attached to
// the account or to individual users via ALTER ACCOUNT / ALTER USER … SET
// SESSION POLICY. Every parameter is optional in the CREATE grammar; an
// unspecified parameter falls back to Snowflake's documented default, so the
// builder emits only the parameters the caller has explicitly set (the four
// timeout parameters are modeled as *int fields). The ALTER clauses (RENAME,
// SET/UNSET each parameter, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough
// to be issued as free-form ALTER SESSION POLICY statements from
// internal/app/sessionpolicy.go (App.AlterSessionPolicy); the configured values
// are read back via App.DescribeSessionPolicy and the users/account the policy
// is attached to via App.GetSessionPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package sessionpolicy
