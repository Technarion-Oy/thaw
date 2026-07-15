// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
