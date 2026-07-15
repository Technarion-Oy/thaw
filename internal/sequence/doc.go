// SPDX-License-Identifier: GPL-3.0-or-later

// Package sequence builds SQL for Snowflake sequence objects — CREATE SEQUENCE
// statements. The ALTER clauses used to edit a sequence (SET INCREMENT, SET /
// UNSET COMMENT, RENAME TO) are simple enough to be issued as free-form
// ALTER SEQUENCE statements from internal/app/sequence.go.
//
// thaw:domain: Object Browser & Administration
package sequence
