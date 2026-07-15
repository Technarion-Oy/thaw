// SPDX-License-Identifier: GPL-3.0-or-later

// Package querylog provides a session-scoped, thread-safe log of all SQL
// queries that Thaw sends to Snowflake — both user-initiated (editor) and
// internal (object listing, DDL fetching, session setup). It is used for
// debugging and issue reporting.
//
// thaw:domain: SQL Editor & Diagnostics
package querylog
