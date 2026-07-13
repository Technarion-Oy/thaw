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

// ── Shared diagnostic helpers ─────────────────────────────────────────────────

// isStarExcludeCol reports whether an uppercased identifier is the *-relative
// EXCLUDE column-transform clause keyword — the EXCLUDE in `SELECT * EXCLUDE col`
// (paren-less) — given whether the token immediately before it is a `*`.
//
// EXCLUDE is deliberately kept OUT of the global sqltok keyword set: unlike a
// true keyword it is a valid unquoted column/alias/table name in Snowflake, and
// a global entry would make IsKeyword reclassify every such identifier (e.g.
// ApplyCasing would then key it off keywordCase instead of identifierCase). The
// column-ref validators recognize it here contextually instead. The
// parenthesized `EXCLUDE (col)` form needs no special case: it is already
// skipped by the "identifier followed by ( ⇒ function call" heuristic.
//
// Known limitation: this checks only whether the preceding token is a literal
// `*`, not whether that `*` is a SELECT wildcard vs. the multiplication
// operator. So `SELECT price * EXCLUDE FROM orders` (multiply by a column
// literally named EXCLUDE) would suppress a "column not found" diagnostic for
// that column. This is an accepted trade-off — a column named EXCLUDE is
// unlikely, and it matches the file's existing heuristic style (e.g. the
// "identifier followed by ( ⇒ function call" heuristic has similar blind spots).
func isStarExcludeCol(upper string, prevIsStar bool) bool {
	return prevIsStar && upper == "EXCLUDE"
}

// stripCommentsSQL removes SQL single-line (--) and block (/* */) comments.
// Line comments are removed entirely; block comments are replaced with a
// single space (matching the legacy regex behavior).
func stripCommentsSQL(sql string) string {
	tokens := sqltok.Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))
	prev := 0
	for _, tok := range tokens {
		switch tok.Kind {
		case sqltok.EOF:
			sb.WriteString(sql[prev:])
			return sb.String()
		case sqltok.LineComment:
			sb.WriteString(sql[prev:tok.Start])
			prev = tok.End
		case sqltok.BlockComment:
			sb.WriteString(sql[prev:tok.Start])
			sb.WriteByte(' ')
			prev = tok.End
		}
	}
	sb.WriteString(sql[prev:])
	return sb.String()
}

// stripStringLiterals replaces single-quoted string literals (handling ”
// escape sequences) with a single space, preventing SQL keywords inside
// strings from being mistaken for actual syntax.
func stripStringLiterals(sql string) string {
	return sqltok.StripStrings(sql)
}

// getFirstSQLToken strips comments and returns the first SQL keyword in sql
// (upper-cased), or "" if none.
func getFirstSQLToken(sql string) string {
	return sqltok.FirstToken(sql)
}

// tokenPos is a located token found within a statement text.
type tokenPos struct {
	name   string // identifier text (inner content for quoted tokens)
	line   int    // 1-based line number in the overall SQL document
	col    int    // 1-based start column
	endCol int    // 1-based end column (exclusive of last character)
	quoted bool
}

// stmtStartCol returns the 1-based document column of a statement's first
// character (byte-based, like sqltok columns). A statement that begins mid-line
// (e.g. the second of `SELECT 1; SELECT …`) starts past column 1, so tokens on
// the statement's first line must add this offset to become document-absolute.
func stmtStartCol(sql string, r StatementRange) int {
	lineStart := strings.LastIndexByte(sql[:r.StartOffset], '\n') + 1
	return r.StartOffset - lineStart + 1
}

// findTokensLocally scans stmtText for occurrences of any identifier listed
// in targets.  baseLine is the 1-based document line of stmtText's first line,
// and baseCol the document column of its first character (see stmtStartCol);
// tokens on the statement's first line are rebased by baseCol so a statement
// that starts mid-line reports document-absolute columns.  Returns one tokenPos
// per match (in document order).
//
// If ignoreCase is true the lookup is case-insensitive, otherwise quoted
// identifiers are matched exactly and unquoted ones are uppercased.
func findTokensLocally(stmtText string, targets []string, baseLine, baseCol int, ignoreCase bool) []tokenPos {
	targetSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		if ignoreCase {
			targetSet[strings.ToUpper(t)] = struct{}{}
		} else {
			targetSet[t] = struct{}{}
		}
	}

	tokens := sqltok.Tokenize(stmtText)
	var result []tokenPos
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind != sqltok.Keyword && tok.Kind != sqltok.Identifier && tok.Kind != sqltok.QuotedIdent {
			continue
		}

		text := tok.Text(stmtText)
		isQuoted := tok.Kind == sqltok.QuotedIdent

		var key, name string
		if isQuoted {
			inner := text[1 : len(text)-1]
			if ignoreCase {
				key = strings.ToUpper(inner)
			} else {
				key = inner
			}
			name = inner
		} else {
			key = strings.ToUpper(text)
			name = text
		}

		if _, ok := targetSet[key]; ok {
			col := tok.Col
			if tok.Line == 1 {
				col += baseCol - 1 // rebase first-line columns to document coords
			}
			result = append(result, tokenPos{
				name:   name,
				line:   baseLine + tok.Line - 1,
				col:    col,
				endCol: col + (tok.End - tok.Start),
				quoted: isQuoted,
			})
		}
	}
	return result
}

// normIdent normalises a SQL identifier: strips double-quotes if present and
// upper-cases the result (unless it is a quoted identifier with ignoreCase=false).
func normIdent(s string, ignoreCase bool) string {
	if inner := sqltok.StripQuotePair(s); inner != s {
		// s was a "quoted" identifier; case is significant unless ignoreCase.
		if ignoreCase {
			return strings.ToUpper(inner)
		}
		return inner
	}
	return strings.ToUpper(s)
}

// extractIdentParts splits a dot-separated SQL object path (e.g.
// `"DB"."SCHEMA".TABLE`) into its normalised component strings.
func extractIdentParts(s string, ignoreCase bool) []string {
	tokens := sqltok.Tokenize(s)
	var parts []string
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		switch tok.Kind {
		case sqltok.Identifier, sqltok.Keyword, sqltok.QuotedIdent:
			parts = append(parts, normIdent(tok.Text(s), ignoreCase))
		}
	}
	return parts
}

// diagMarker constructs a DiagMarker at a tokenPos with the given message and
// Monaco severity (8 = Error, 4 = Warning).
func diagMarkerAt(t tokenPos, msg string, severity int) DiagMarker {
	return DiagMarker{
		StartLineNumber: t.line,
		StartColumn:     t.col,
		EndLineNumber:   t.line,
		EndColumn:       t.endCol,
		Message:         msg,
		Severity:        severity,
	}
}

// sqlStmt returns the raw statement text from sql given a StatementRange.
// StatementRange offsets are byte-based (not rune-based).
func sqlStmt(sql string, r StatementRange) string {
	start := r.StartOffset
	end := r.EndOffset
	if start < 0 {
		start = 0
	}
	if end > len(sql) {
		end = len(sql)
	}
	return sql[start:end]
}
