// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package storagelifecyclepolicy builds SQL for Snowflake storage lifecycle
// policy objects — CREATE STORAGE LIFECYCLE POLICY statements and the structured
// config behind them. Storage lifecycle policies automate data retention,
// archival, and deletion: rows of a table the policy is attached to become
// eligible for the lifecycle action (archive to a COOL/COLD tier, then expire)
// once the policy body evaluates to TRUE for them. Like masking and row access
// policies, a storage lifecycle policy has a real signature — AS (<arg> <type>,
// …) RETURNS BOOLEAN -> <body> — that maps each argument to a column of the
// attached table (Snowflake requires at least one argument); unlike them it also
// carries the archival options ARCHIVE_TIER and ARCHIVE_FOR_DAYS. The ALTER
// clauses (RENAME, SET BODY, SET ARCHIVE_TIER,
// SET/UNSET ARCHIVE_FOR_DAYS, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough
// to be issued as free-form ALTER STORAGE LIFECYCLE POLICY statements from
// internal/app/storagelifecyclepolicy.go (App.AlterStorageLifecyclePolicy); the
// tables a policy is applied to are read via
// App.GetStorageLifecyclePolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package storagelifecyclepolicy
