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
)

// ── Shared diagnostic helpers ─────────────────────────────────────────────────
// These are private-to-package ports of the same-named helpers in
// sqlDiagnostics.ts.

var (
	reLineCommentDH  = regexp.MustCompile(`(?m)--[^\n]*`)
	reBlockCommentDH = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reIdentOrQuoted  = regexp.MustCompile(`[a-zA-Z0-9_$]+|"[^"]+"`)
	reFirstToken     = regexp.MustCompile(`^[a-zA-Z_]\w*`)
)

// stripCommentsSQL removes SQL single-line (--) and block (/* */) comments.
func stripCommentsSQL(sql string) string {
	s := reBlockCommentDH.ReplaceAllString(sql, " ")
	return reLineCommentDH.ReplaceAllString(s, "")
}

// getFirstSQLToken strips comments and returns the first SQL keyword in sql
// (upper-cased), or "" if none.
func getFirstSQLToken(sql string) string {
	s := strings.TrimSpace(stripCommentsSQL(sql))
	m := reFirstToken.FindString(s)
	return strings.ToUpper(m)
}

// tokenPos is a located token found within a statement text.
type tokenPos struct {
	name   string // identifier text (inner content for quoted tokens)
	line   int    // 1-based line number in the overall SQL document
	col    int    // 1-based start column
	endCol int    // 1-based end column (exclusive of last character)
	quoted bool
}

// findTokensLocally scans stmtText line-by-line for occurrences of any
// identifier listed in targets.  baseLine is the 1-based document line of
// stmtText's first line.  Returns one tokenPos per match (in document order).
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

	var result []tokenPos
	for i, lineStr := range strings.Split(stmtText, "\n") {
		for _, loc := range reIdentOrQuoted.FindAllStringIndex(lineStr, -1) {
			raw := lineStr[loc[0]:loc[1]]
			isQuoted := len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"'

			var key, name string
			if isQuoted {
				inner := raw[1 : len(raw)-1]
				if ignoreCase {
					key = strings.ToUpper(inner)
				} else {
					key = inner
				}
				name = inner
			} else {
				key = strings.ToUpper(raw)
				name = raw
			}

			if _, ok := targetSet[key]; ok {
				result = append(result, tokenPos{
					name:   name,
					line:   baseLine + i,
					col:    loc[0] + 1,
					endCol: loc[1] + 1,
					quoted: isQuoted,
				})
			}
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
	ms := reIdentOrQuoted.FindAllString(s, -1)
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		parts = append(parts, normIdent(m, ignoreCase))
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

// diagMarkerSpan constructs a DiagMarker covering the full statement.
func diagMarkerSpan(r StatementRange, msg string, severity int) DiagMarker {
	return DiagMarker{
		StartLineNumber: r.StartLine,
		StartColumn:     1,
		EndLineNumber:   r.EndLine,
		EndColumn:       100,
		Message:         msg,
		Severity:        severity,
	}
}

// sqlStmt returns the raw statement text from sql given a StatementRange.
func sqlStmt(sql string, r StatementRange) string {
	runes := []rune(sql)
	start := r.StartOffset
	end := r.EndOffset
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}
