// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package aggregationpolicy builds SQL for Snowflake aggregation policy objects —
// CREATE AGGREGATION POLICY statements and the structured config behind them. An
// aggregation policy is a schema-level governance object that enforces query
// results on a protected table or view to be aggregated to a minimum group size,
// so individual records cannot be identified. Unlike masking and row-access
// policies, an aggregation policy takes no arguments: its signature is always the
// empty `()` and it always RETURNS AGGREGATION_CONSTRAINT, so the only authored
// parts are the body expression (an SQL expression returning
// AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => n) or NO_AGGREGATION_CONSTRAINT(),
// optionally wrapped in conditional logic) and an optional comment. A policy is
// attached to a table or view via ALTER TABLE/VIEW … SET AGGREGATION POLICY. The
// ALTER clauses (RENAME, SET BODY, SET/UNSET COMMENT, SET/UNSET TAG) are simple
// enough to be issued as free-form ALTER AGGREGATION POLICY statements from
// internal/app/aggregationpolicy.go (App.AlterAggregationPolicy); the body is read
// back via the DESCRIBE enrichment in internal/objects, and the tables/views the
// policy is attached to via App.GetAggregationPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package aggregationpolicy
