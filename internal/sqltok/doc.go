// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package sqltok implements a single-pass O(N) tokenizer for Snowflake SQL.
// It replaces regex-based parsing with a byte-level state machine that
// correctly handles all quoting and comment styles, classifies tokens by
// kind (keyword, identifier, literal, operator, etc.), and tracks line/column
// positions for diagnostic integration.
//
// thaw:domain: SQL Editor & Diagnostics
package sqltok
