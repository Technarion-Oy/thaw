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
	// sqltok reports byte columns; Monaco counts UTF-16 units. Convert token
	// columns through the source lines so a non-ASCII character earlier on the
	// line (e.g. 'café') doesn't shift the match/replace range (cf. the []rune
	// handling in GetIdentifierAtColumn).
	lines := strings.Split(sql, "\n")
	sig := sqltok.Significant(sqltok.Tokenize(sql))
	for i, t := range sig {
		if t.Kind != sqltok.Operator || t.Text(sql) != "*" {
			continue
		}
		starCol := utf16Col(lines, t.Line, t.Col)
		// A right-click lands the cursor on either edge of the single-char `*`.
		if t.Line != line || (col != starCol && col != starCol+1) {
			continue
		}

		var prev sqltok.Token
		if i > 0 {
			prev = sig[i-1]
		}

		switch {
		case i > 0 && prev.Kind == sqltok.Dot:
			// `alias.*` — the ident-like token immediately before the dot is the
			// qualifier used as the column prefix and match key.
			if i < 2 || !sig[i-2].Kind.IsIdentLike() {
				return nil
			}
			alias := sig[i-2]
			// Walk back the full dotted chain (db.schema.tbl.*) so the replace
			// range starts at the first segment — otherwise a multi-part qualifier
			// leaves `db.schema.` dangling before the expanded list.
			start := alias
			for j := i - 2; j >= 2 && sig[j-1].Kind == sqltok.Dot && sig[j-2].Kind.IsIdentLike(); j -= 2 {
				start = sig[j-2]
			}
			return &StarSelect{
				StartLine: start.Line, StartCol: utf16Col(lines, start.Line, start.Col),
				EndLine: t.Line, EndCol: starCol + 1,
				Alias: alias.Text(sql),
			}
		case i == 0 || introducesSelectItem(prev, sql):
			return &StarSelect{
				StartLine: t.Line, StartCol: starCol,
				EndLine: t.Line, EndCol: starCol + 1,
			}
		default:
			return nil
		}
	}
	return nil
}

// utf16Col converts a 1-based byte column on lines[lineNum-1] to a 1-based
// UTF-16 column (Monaco's coordinate system). Out-of-range inputs pass through.
func utf16Col(lines []string, lineNum, byteCol int) int {
	if lineNum < 1 || lineNum > len(lines) {
		return byteCol
	}
	line := lines[lineNum-1]
	b := min(max(byteCol-1, 0), len(line))
	n := 0
	for _, r := range line[:b] {
		if r > 0xFFFF { // astral plane → surrogate pair (2 UTF-16 units)
			n += 2
		} else {
			n++
		}
	}
	return n + 1
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

// fromTerminatorKW ends the FROM/JOIN clause when seen at paren depth 0.
var fromTerminatorKW = map[string]bool{
	"WHERE": true, "GROUP": true, "HAVING": true, "ORDER": true, "QUALIFY": true,
	"LIMIT": true, "OFFSET": true, "WINDOW": true, "UNION": true, "INTERSECT": true,
	"EXCEPT": true, "MINUS": true, "FETCH": true,
}

// FromSourceCount reports how many table sources the statement's top-level FROM
// clause has (JOIN- and comma-separated), so a bare `*` expansion can be checked
// for completeness against the resolved-ref count. It returns -1 when the ref
// parser (ParseJoinTables, which is not depth-aware) can't be trusted to have
// enumerated exactly those sources — i.e. the statement has a subquery or CTE
// (more than one SELECT), or there is no top-level FROM. The caller should treat
// a bare `*` as expandable only when this equals the resolved-ref count; -1 or a
// mismatch means "refuse rather than write an incomplete/wrong list".
func FromSourceCount(sql string) int {
	sig := sqltok.Significant(sqltok.Tokenize(sql))

	// A nested SELECT (subquery or CTE body) makes ParseJoinTables over- or
	// under-count, so refuse anything but a single, flat SELECT.
	selects := 0
	for _, t := range sig {
		if t.Kind == sqltok.Keyword && strings.EqualFold(t.Text(sql), "SELECT") {
			selects++
		}
	}
	if selects != 1 {
		return -1
	}

	// Find the top-level FROM.
	depth, fromIdx := 0, -1
	for i, t := range sig {
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		case sqltok.Keyword:
			if depth == 0 && strings.EqualFold(t.Text(sql), "FROM") {
				fromIdx = i
			}
		}
		if fromIdx >= 0 {
			break
		}
	}
	if fromIdx < 0 {
		return -1
	}

	// One source, plus one per top-level JOIN keyword or comma, until the clause
	// ends. (No nested SELECT exists here, so any `(` is a function-call arg whose
	// inner commas are at depth > 0 and don't count.)
	sources := 1
	depth = 0
	for i := fromIdx + 1; i < len(sig); i++ {
		t := sig[i]
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		case sqltok.Semicolon:
			return sources
		case sqltok.Comma:
			if depth == 0 {
				sources++
			}
		case sqltok.Keyword:
			if depth == 0 {
				up := strings.ToUpper(t.Text(sql))
				if fromTerminatorKW[up] {
					return sources
				}
				if up == "JOIN" {
					sources++
				}
			}
		}
	}
	return sources
}
