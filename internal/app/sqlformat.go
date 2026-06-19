// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import "thaw/internal/snowflake"

// General SQL string-formatting delegators exposed over IPC so the frontend keeps
// no SQL quoting / DESCRIBE-parsing logic of its own — each is a pure wrapper over
// the shared internal/snowflake implementation and needs no Snowflake connection.

// ParseSqlList parses a DESCRIBE list-cell value (a SQL tuple, a bracketed list,
// or a JSON-style array) into its individual value tokens, with quoting/escapes
// handled by the shared SQL tokenizer. An empty / null cell yields an empty list.
func (a *App) ParseSqlList(raw string) []string {
	return snowflake.ParseSqlList(raw)
}

// NormalizeSqlScalar strips the surrounding brackets/quotes Snowflake may wrap a
// DESCRIBE scalar cell in ("[OPTIONAL]", "'OPTIONAL'", "OPTIONAL" → "OPTIONAL"),
// returning "" for an empty cell.
func (a *App) NormalizeSqlScalar(raw string) string {
	return snowflake.NormalizeScalar(raw)
}

// QuoteSqlText wraps s in single-quotes as a free-text SQL string literal,
// escaping backslashes and single-quotes (the quoting used for human-entered text
// such as object comments).
func (a *App) QuoteSqlText(s string) string {
	return snowflake.QuoteTextLit(s)
}
