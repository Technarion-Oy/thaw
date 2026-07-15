// SPDX-License-Identifier: GPL-3.0-or-later

// Package stream builds SQL for Snowflake STREAM objects — CREATE STREAM
// statements over a source TABLE / VIEW / EXTERNAL TABLE / STAGE / DYNAMIC
// TABLE, with the change-tracking options (APPEND_ONLY, SHOW_INITIAL_ROWS,
// INSERT_ONLY). The ALTER clauses (SET / UNSET COMMENT, RENAME TO) are issued
// directly from internal/app as free-form ALTER STREAM statements.
//
// thaw:domain: Object Browser & Administration
package stream
