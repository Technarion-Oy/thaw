// SPDX-License-Identifier: GPL-3.0-or-later

// Package projectionpolicy builds SQL for Snowflake projection policy objects —
// CREATE PROJECTION POLICY statements and the structured config behind them. A
// projection policy is a schema-level governance object that controls whether a
// protected column can appear in query output (be projected via SELECT) — unlike
// a masking policy, which transforms values, a projection policy prevents the
// column from being selected at all. Like aggregation policies, a projection
// policy takes no arguments: its signature is always the empty `()` and it always
// RETURNS PROJECTION_CONSTRAINT, so the only authored parts are the body
// expression (an SQL expression returning PROJECTION_CONSTRAINT(ALLOW => true) or
// PROJECTION_CONSTRAINT(ALLOW => false), optionally wrapped in conditional logic)
// and an optional comment. A policy is attached to a column via
// ALTER TABLE/VIEW … MODIFY COLUMN … SET PROJECTION POLICY. The ALTER clauses
// (RENAME, SET BODY, SET/UNSET COMMENT, SET/UNSET TAG) are simple enough to be
// issued as free-form ALTER PROJECTION POLICY statements from
// internal/app/projectionpolicy.go (App.AlterProjectionPolicy); the body is read
// back via the DESCRIBE enrichment in internal/objects, and the columns the
// policy is attached to via App.GetProjectionPolicyReferences (POLICY_REFERENCES).
//
// thaw:domain: Object Browser & Administration
package projectionpolicy
