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

	"thaw/internal/sqlgrammar"
	"thaw/internal/sqltok"
)

// ValidateGrammar validates each statement against the recursive-descent
// Snowflake grammar in internal/sqlgrammar. A statement is checked only when its
// leading keyword maps to an implemented grammar (sqlgrammar.Validator.Recognized);
// unmodeled statements are skipped so valid-but-unmodeled SQL is never flagged.
//
// When a recognized statement does not conform, a single marker is emitted at the
// furthest position the grammar reached, carrying the grammar's "expected …"
// message. The grammar is deliberately conservative (generic catch-all rules
// accept any roughly-well-formed statement), so this fires on clearly-broken
// statements — missing names, dangling keywords, unbalanced parens — rather than
// on stylistic or unmodeled-option differences. Markers are Warnings, leaving
// hard Errors to the lexical checks in ValidateSyntax.
func ValidateGrammar(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker
	for _, r := range stmtRanges {
		stmt := sql[r.StartOffset:r.EndOffset]
		v := sqlgrammar.New(stmt)
		if !v.Recognized() || v.ParseTopLevel() {
			continue
		}
		f := v.Failure()
		sl, sc, el, ec := grammarMarkerPos(sql, r, f.Tok)
		markers = append(markers, DiagMarker{
			StartLineNumber: sl, StartColumn: sc,
			EndLineNumber: el, EndColumn: ec,
			Message:  f.Message(),
			Severity: SeverityWarning,
			Code:     "grammar",
		})
	}
	return markers
}

// grammarMarkerPos maps a failure token (in statement-local coordinates) to
// absolute 1-based document coordinates. When the parser ran off the end of the
// statement (EOF sentinel) the marker is anchored just past the last significant
// token — skipping a trailing semicolon, whitespace, and comments, none of which
// the grammar consumes — so it never lands on a trailing comment.
func grammarMarkerPos(sql string, r StatementRange, tok sqltok.Token) (sl, sc, el, ec int) {
	startCol := stmtStartCol(sql, r)

	if tok.Kind == sqltok.EOF || tok.Line == 0 {
		endOff := r.StartOffset + stmtSignificantEnd(sql[r.StartOffset:r.EndOffset])
		el = r.StartLine + strings.Count(sql[r.StartOffset:endOff], "\n")
		elLineStart := strings.LastIndexByte(sql[:endOff], '\n') + 1
		ec = endOff - elLineStart + 1
		return el, max(ec-1, 1), el, ec
	}

	sl = r.StartLine + tok.Line - 1
	if tok.Line == 1 {
		sc = startCol + tok.Col - 1
	} else {
		sc = tok.Col
	}
	// The failure token may span multiple lines ($$…$$, a multi-line string
	// literal), so derive the end from its text rather than assuming the span
	// stays on the start line.
	tokText := sql[r.StartOffset+tok.Start : r.StartOffset+tok.End]
	if nl := strings.Count(tokText, "\n"); nl > 0 {
		return sl, sc, sl + nl, len(tokText) - strings.LastIndexByte(tokText, '\n')
	}
	return sl, sc, sl, sc + (tok.End - tok.Start)
}

// stmtSignificantEnd returns the byte offset within stmt just past its last
// significant token, ignoring a trailing semicolon and any trailing
// whitespace/comment. Returns 0 when the statement has no significant content.
func stmtSignificantEnd(stmt string) int {
	sig := sqltok.SignificantTokens(stmt)
	for i := len(sig) - 1; i >= 0; i-- {
		if sig[i].Kind != sqltok.Semicolon {
			return sig[i].End
		}
	}
	return 0
}
