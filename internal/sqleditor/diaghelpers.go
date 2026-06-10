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
	"regexp"
	"strings"

	"thaw/internal/sqltok"
)

// ── Shared diagnostic helpers ─────────────────────────────────────────────────

// reIdentOrQuoted matches bare SQL identifiers or double-quoted identifiers.
// Used directly by tableexist.go, barecolrefs.go, and patterns.go.
var reIdentOrQuoted = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_$]*|"(?:[^"]|"")*"`)

// stripCommentsSQL removes SQL single-line (--) and block (/* */) comments.
// Line comments are removed entirely; block comments are replaced with a
// single space (matching the legacy regex behaviour).
func stripCommentsSQL(sql string) string {
	tokens := sqltok.Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))
	prev := 0
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind == sqltok.LineComment {
			sb.WriteString(sql[prev:tok.Start])
			prev = tok.End
		} else if tok.Kind == sqltok.BlockComment {
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

// findTokensLocally scans stmtText for occurrences of any identifier listed
// in targets.  baseLine is the 1-based document line of stmtText's first
// line.  Returns one tokenPos per match (in document order).
//
// If ignoreCase is true the lookup is case-insensitive, otherwise quoted
// identifiers are matched exactly and unquoted ones are uppercased.
func findTokensLocally(stmtText string, targets []string, baseLine int, ignoreCase bool) []tokenPos {
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
			result = append(result, tokenPos{
				name:   name,
				line:   baseLine + tok.Line - 1,
				col:    tok.Col,
				endCol: tok.Col + (tok.End - tok.Start),
				quoted: isQuoted,
			})
		}
	}
	return result
}

// normIdent normalises a SQL identifier: strips double-quotes if present and
// upper-cases the result (unless it is a quoted identifier with ignoreCase=false).
func normIdent(s string, ignoreCase bool) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
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

// diagMarkerSpan constructs a Warning DiagMarker covering the full statement.
// Every pattern diagnostic is a warning (severity 4), so the severity is fixed.
func diagMarkerSpan(r StatementRange, msg string) DiagMarker {
	return DiagMarker{
		StartLineNumber: r.StartLine,
		StartColumn:     1,
		EndLineNumber:   r.EndLine,
		EndColumn:       100,
		Message:         msg,
		Severity:        4, // Warning
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
