// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"strings"

	"thaw/internal/sqltok"
)

// StarSelect describes a select-list wildcard (`*` or `alias.*`) located at a
// cursor position. Positions are 1-based Monaco line/column coordinates; the
// [StartLine, StartCol]–[EndLine, EndCol] span is the exact range to replace
// with an expanded column list. Alias is the raw source text of the qualifier
// for `alias.*` (quotes preserved), or "" for a bare `*`.
type StarSelect struct {
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
	Alias     string `json:"alias"`
}

// StarSelectAt reports whether the token at the given 1-based cursor position is
// a select-list wildcard, returning its span (and any `alias.` qualifier) or nil.
//
// Detection is token-based (sqltok), which is why it is correct where the old
// char-scan was not: a `*` inside a quoted identifier ("a*b") is part of a
// QuotedIdent token, never an Operator, so it is never mistaken for a wildcard;
// and a quoted qualifier ("my table".*) is captured whole regardless of spaces.
//
// A `*` is a wildcard only when it introduces a select item, not when it is an
// arithmetic multiplication or a function argument. The preceding significant
// token decides:
//   - Dot                        → `alias.*` (ident-like token before the dot is the alias)
//   - a select-item introducer   → bare wildcard (SELECT / DISTINCT / ALL / `,`, or nothing)
//   - anything else              → not a wildcard: multiplication (`a * b`, `2 * n`,
//                                  `… END * 100`) or a function argument (COUNT(*)) — skip
//
// The introducer test is a whitelist, not a blacklist: a blacklist of "operand"
// token kinds can't tell SELECT/DISTINCT/ALL (which are keywords that legitimately
// precede a wildcard) apart from END/NULL/… (keywords that are the left operand of
// a multiplication), since both are sqltok.Keyword.
func StarSelectAt(sql string, line, col int) *StarSelect {
	sig := sqltok.Significant(sqltok.Tokenize(sql))
	for i, t := range sig {
		if t.Kind != sqltok.Operator || t.Text(sql) != "*" {
			continue
		}
		// A right-click lands the cursor on either edge of the single-char `*`.
		if t.Line != line || (col != t.Col && col != t.Col+1) {
			continue
		}

		var prev sqltok.Token
		if i > 0 {
			prev = sig[i-1]
		}

		switch {
		case i > 0 && prev.Kind == sqltok.Dot:
			// `alias.*` — the ident-like token immediately before the dot.
			if i < 2 || !sig[i-2].Kind.IsIdentLike() {
				return nil
			}
			alias := sig[i-2]
			return &StarSelect{
				StartLine: alias.Line, StartCol: alias.Col,
				EndLine: t.Line, EndCol: t.Col + 1,
				Alias: alias.Text(sql),
			}
		case i == 0 || introducesSelectItem(prev, sql):
			return &StarSelect{
				StartLine: t.Line, StartCol: t.Col,
				EndLine: t.Line, EndCol: t.Col + 1,
			}
		default:
			return nil
		}
	}
	return nil
}

// introducesSelectItem reports whether a select-list item (and thus a bare `*`)
// can legitimately follow this token: a comma, or one of the quantifier keywords
// that open a SELECT list.
func introducesSelectItem(tok sqltok.Token, src string) bool {
	switch tok.Kind {
	case sqltok.Comma:
		return true
	case sqltok.Keyword:
		switch strings.ToUpper(tok.Text(src)) {
		case "SELECT", "DISTINCT", "ALL":
			return true
		}
	}
	return false
}
