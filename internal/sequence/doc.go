// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package sequence builds SQL for Snowflake sequence objects — CREATE SEQUENCE
// statements. The ALTER clauses used to edit a sequence (SET INCREMENT, SET /
// UNSET COMMENT, RENAME TO) are simple enough to be issued as free-form
// ALTER SEQUENCE statements from internal/app/sequence.go.
//
// thaw:domain: Object Browser & Administration
package sequence
