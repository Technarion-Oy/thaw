// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package querylog provides a session-scoped, thread-safe log of all SQL
// queries that Thaw sends to Snowflake — both user-initiated (editor) and
// internal (object listing, DDL fetching, session setup). It is used for
// debugging and issue reporting.
//
// thaw:domain: SQL Editor & Diagnostics
package querylog
