// SPDX-License-Identifier: GPL-3.0-or-later

// Package eventtable builds SQL for Snowflake EVENT TABLE objects — CREATE EVENT
// TABLE statements and the structured config behind them. An event table is a
// special table with a predefined, fixed schema that captures telemetry data
// (logs, traces, and metrics) emitted by UDFs, stored procedures, and Snowpark
// Container Services. It is the destination for an account's or session's event
// telemetry and is essential for debugging and observability.
//
// Because event tables have a fixed column layout, CREATE EVENT TABLE accepts no
// column definitions (the visual builder and the SQL editor's
// validateCreateEventTable both enforce this). It does, however, accept a
// CLUSTER BY clause on the predefined columns (e.g. the timestamp column), plus
// the supported table-level properties: DATA_RETENTION_TIME_IN_DAYS,
// MAX_DATA_EXTENSION_TIME_IN_DAYS, CHANGE_TRACKING, DEFAULT_DDL_COLLATION,
// COPY GRANTS, COMMENT, and TAG — all of which the builder emits.
//
// Event tables share the standard TABLE management commands rather than having
// EVENT-specific variants: they are altered, dropped, and renamed through the
// plain TABLE grammar (ALTER TABLE / DROP TABLE / ALTER TABLE … RENAME TO), so
// the mutable properties are issued as free-form ALTER TABLE statements from
// internal/app/eventtable.go (App.AlterEventTable). GET_DDL does expose a
// dedicated EVENT_TABLE object type, so DDL export is normalized in
// internal/snowflake (buildGetDDLQuery), not here.
//
// thaw:domain: Object Browser & Administration
package eventtable
