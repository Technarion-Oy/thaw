// SPDX-License-Identifier: GPL-3.0-or-later

// Package notebook builds SQL for Snowflake NOTEBOOK objects — CREATE NOTEBOOK
// statements and the structured config behind them. A Snowflake Notebook is a
// schema-level object that runs cells of Python/SQL/Scala on a warehouse. It can
// be created empty (an editable notebook is provisioned on first open) or from
// files staged in an internal stage or git repository via the FROM + MAIN_FILE
// grammar.
//
// Only the CREATE path is built here. The mutable properties (QUERY_WAREHOUSE,
// COMMENT, …) are issued as free-form ALTER NOTEBOOK statements elsewhere, and
// GET_DDL supports 'NOTEBOOK' directly so DDL export needs no special-casing.
//
// Note: CREATE NOTEBOOK has no clause for the notebook's default cell language
// (Python/SQL/Scala) — that lives in the notebook file's metadata, not the DDL —
// so the builder intentionally emits no language clause.
//
// thaw:domain: Object Browser & Administration
package notebook
