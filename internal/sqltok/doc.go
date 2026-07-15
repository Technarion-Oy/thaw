// SPDX-License-Identifier: GPL-3.0-or-later

// Package sqltok implements a single-pass O(N) tokenizer for Snowflake SQL.
// It replaces regex-based parsing with a byte-level state machine that
// correctly handles all quoting and comment styles, classifies tokens by
// kind (keyword, identifier, literal, operator, etc.), and tracks line/column
// positions for diagnostic integration.
//
// thaw:domain: SQL Editor & Diagnostics
package sqltok
