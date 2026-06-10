// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqltok

import "strings"

// Split returns trimmed SQL statements split at top-level semicolons.
// It correctly handles all Snowflake quoting and comment styles.
// This is a direct replacement for sqlutil.Split.
func Split(sql string) []string {
	n := len(sql)
	out := make([]string, 0, 64)
	stmtStart := 0

	tokens := Tokenize(sql)
	for _, tok := range tokens {
		if tok.Kind == Semicolon {
			if s := strings.TrimSpace(sql[stmtStart:tok.Start]); s != "" {
				out = append(out, s)
			}
			stmtStart = tok.End
		}
	}

	// Flush trailing content without a closing semicolon.
	if stmtStart < n {
		if s := strings.TrimSpace(sql[stmtStart:]); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// StatementRange is the position of one SQL statement within a multi-statement
// string.  Offsets are byte-based (not rune-based).
type StatementRange struct {
	StartLine   int `json:"startLine"`   // 1-based line of trimmed statement start
	EndLine     int `json:"endLine"`     // 1-based line of trailing ';' (or last char)
	StartOffset int `json:"startOffset"` // byte offset of trimmed statement start
	EndOffset   int `json:"endOffset"`   // byte offset just past ';' (or end of string)
}

// SplitRanges returns per-statement ranges with line/offset info.
// Semicolons inside comments, strings, and dollar-quoted blocks are ignored.
// Leading comments between statements are attached to the following statement
// (matching the GetStatementRanges behaviour).
func SplitRanges(sql string) []StatementRange {
	var ranges []StatementRange

	tokens := Tokenize(sql)

	inStmt := false
	stmtStartLine := 0
	stmtStartOffset := 0

	for _, tok := range tokens {
		if tok.Kind == EOF {
			break
		}

		// Skip whitespace and newlines between statements.
		if !inStmt && (tok.Kind == Whitespace || tok.Kind == Newline) {
			continue
		}

		// Skip leading comments between statements (they are not statement content).
		if !inStmt && (tok.Kind == LineComment || tok.Kind == BlockComment) {
			continue
		}

		// Start a new statement on the first non-whitespace, non-comment token.
		if !inStmt {
			inStmt = true
			stmtStartLine = tok.Line
			stmtStartOffset = tok.Start
		}

		if tok.Kind == Semicolon {
			ranges = append(ranges, StatementRange{
				StartLine:   stmtStartLine,
				EndLine:     tok.Line,
				StartOffset: stmtStartOffset,
				EndOffset:   tok.End,
			})
			inStmt = false
		}
	}

	// Emit trailing statement with no semicolon.
	if inStmt {
		// Find the last non-whitespace, non-newline token for EndLine.
		endLine := stmtStartLine
		endOffset := stmtStartOffset
		for i := len(tokens) - 1; i >= 0; i-- {
			t := tokens[i]
			if t.Kind != EOF && t.Kind != Whitespace && t.Kind != Newline {
				endLine = t.Line
				// For block comments that span multiple lines, compute the end line.
				if t.Kind == BlockComment {
					endLine += strings.Count(sql[t.Start:t.End], "\n")
				}
				endOffset = t.End
				break
			}
		}
		ranges = append(ranges, StatementRange{
			StartLine:   stmtStartLine,
			EndLine:     endLine,
			StartOffset: stmtStartOffset,
			EndOffset:   endOffset,
		})
	}

	return ranges
}
