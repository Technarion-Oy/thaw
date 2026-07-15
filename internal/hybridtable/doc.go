// SPDX-License-Identifier: GPL-3.0-or-later

// Package hybridtable builds SQL for Snowflake HYBRID TABLE objects — CREATE
// HYBRID TABLE statements and the structured config behind them, plus the
// CREATE INDEX / DROP INDEX statements used to manage a hybrid table's
// secondary indexes.
//
// Hybrid tables back Snowflake's Unistore / HTAP workloads: they serve
// low-latency single-row operations (point lookups, DML) alongside analytical
// queries, enforce a PRIMARY KEY, and support secondary indexes. Every hybrid
// table must define a PRIMARY KEY constraint on one or more columns, so the
// builder always emits a PRIMARY KEY clause (a placeholder when none is
// selected, so the live preview reads as a template rather than invalid SQL)
// and forces every primary-key column to NOT NULL, which Snowflake requires.
// Hybrid tables do NOT support OR REPLACE, TRANSIENT, CLUSTER BY,
// DATA_RETENTION_TIME_IN_DAYS, CHANGE_TRACKING, or COPY GRANTS, so none of those
// are modeled.
//
// CREATE HYBRID TABLE skeleton (the subset the visual builder covers):
//
//	CREATE HYBRID TABLE [IF NOT EXISTS] <fqn> (
//	    <col> <type> [NOT NULL] [DEFAULT <expr>],
//	    ...,
//	    PRIMARY KEY (<col> [, ...]),
//	    [INDEX <name> (<col> [, ...]) [INCLUDE (<col> [, ...])], ...]
//	  )
//	  [COMMENT = '<string>'];
//
// Indexes can also be added or removed after creation:
//
//	CREATE INDEX <name> ON <fqn> (<col> [, ...]) [INCLUDE (<col> [, ...])]
//	DROP INDEX [IF EXISTS] <fqn>.<name>
//
// Hybrid tables are altered, renamed, and dropped through the plain TABLE
// grammar — there is no ALTER/DROP HYBRID TABLE statement. The mutable
// properties (COMMENT) and RENAME TO are issued as free-form ALTER TABLE
// statements from internal/app/hybridtable.go (App.AlterHybridTable), and
// GET_DDL has no HYBRID_TABLE object type, so DDL export is normalized to the
// 'TABLE' type in internal/snowflake (buildGetDDLQuery), not here.
//
// thaw:domain: Object Browser & Administration
package hybridtable
