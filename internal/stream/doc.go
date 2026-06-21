// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package stream builds SQL for Snowflake STREAM objects — CREATE STREAM
// statements over a source TABLE / VIEW / EXTERNAL TABLE / STAGE / DYNAMIC
// TABLE, with the change-tracking options (APPEND_ONLY, SHOW_INITIAL_ROWS,
// INSERT_ONLY). The ALTER clauses (SET / UNSET COMMENT, RENAME TO) are issued
// directly from internal/app as free-form ALTER STREAM statements.
//
// thaw:domain: Object Browser & Administration
package stream
