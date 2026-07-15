// SPDX-License-Identifier: GPL-3.0-or-later

// Package streamlit builds SQL for Snowflake STREAMLIT objects — CREATE STREAMLIT
// statements and the structured config behind them. A Streamlit app is a
// schema-level object that renders an interactive Python data app from files in
// a stage (or git repository): it is defined by a source location (the modern
// FROM clause), a relative main file, and an optional query warehouse, title,
// comment, and external-access integrations.
//
// Only the modern FROM <stage location> grammar is emitted; the deprecated
// ROOT_LOCATION form is intentionally not supported. The mutable properties —
// MAIN_FILE, QUERY_WAREHOUSE, TITLE, COMMENT, EXTERNAL_ACCESS_INTEGRATIONS — plus
// RENAME TO are issued as free-form ALTER STREAMLIT statements from
// internal/app/streamlit.go (App.AlterStreamlit). GET_DDL supports 'STREAMLIT'
// directly, so DDL export needs no special-casing.
//
// thaw:domain: Object Browser & Administration
package streamlit
