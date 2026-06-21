// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package modelmonitor builds SQL for Snowflake MODEL MONITOR objects — CREATE
// MODEL MONITOR statements and the structured config behind them. A model
// monitor is a schema-level object that provides observability for a model
// registered in the Snowpark ML Model Registry: it tracks model performance
// metrics, prediction quality, and data drift by repeatedly aggregating a source
// table/view (and, optionally, a baseline) on a refresh schedule.
//
// CREATE MODEL MONITOR has a WITH clause carrying many parameters. Eight are
// required (MODEL, VERSION, FUNCTION, SOURCE, WAREHOUSE, REFRESH_INTERVAL,
// AGGREGATION_WINDOW, TIMESTAMP_COLUMN); the rest — BASELINE and the column-array
// parameters (ID_COLUMNS, PREDICTION_*_COLUMNS, ACTUAL_*_COLUMNS, SEGMENT_COLUMNS,
// CUSTOM_METRIC_COLUMNS) — are optional. At least one prediction column (score or
// class) is mandatory in practice; the create modal enforces this.
//
// The mutable properties are limited: ALTER MODEL MONITOR only supports
// SUSPEND / RESUME, SET BASELINE / REFRESH_INTERVAL / WAREHOUSE, and
// ADD / DROP segment_column — there is no RENAME, COMMENT, or TAG. Those ALTERs
// are issued as free-form statements from internal/app/modelmonitor.go
// (App.AlterModelMonitor). GET_DDL does not support model monitors, so there is
// no DDL-export path for this type.
//
// thaw:domain: Object Browser & Administration
package modelmonitor
