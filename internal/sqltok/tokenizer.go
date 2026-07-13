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

// Tokenize scans sql and returns all tokens. The final token is always EOF.
func Tokenize(sql string) []Token {
	n := len(sql)
	tokens := make([]Token, 0, n/4+1)
	pos := 0
	line := 1
	col := 1

	for pos < n {
		tok := scan(sql, n, &pos, &line, &col)
		tokens = append(tokens, tok)
	}

	tokens = append(tokens, Token{Kind: EOF, Start: n, End: n, Line: line, Col: col})
	return tokens
}

// TokenizeIter returns an iterator function that yields one token per call.
// After the final token (EOF), every subsequent call returns (EOF-token, false).
func TokenizeIter(sql string) func() (Token, bool) {
	n := len(sql)
	pos := 0
	line := 1
	col := 1
	done := false

	return func() (Token, bool) {
		if done {
			return Token{Kind: EOF, Start: n, End: n, Line: line, Col: col}, false
		}
		if pos >= n {
			done = true
			return Token{Kind: EOF, Start: n, End: n, Line: line, Col: col}, true
		}
		tok := scan(sql, n, &pos, &line, &col)
		return tok, true
	}
}

// scan reads the next token starting at *pos and advances *pos, *line, *col.
func scan(src string, n int, pos, line, col *int) Token {
	start := *pos
	startLine := *line
	startCol := *col
	c := src[start]

	switch {

	// ── Newline ──────────────────────────────────────────────────────────
	case c == '\n':
		*pos++
		tok := Token{Kind: Newline, Start: start, End: *pos, Line: startLine, Col: startCol}
		*line++
		*col = 1
		return tok

	// ── Whitespace ───────────────────────────────────────────────────────
	case c == ' ' || c == '\t' || c == '\r' || (c == 0xC2 && start+1 < n && src[start+1] == 0xA0):
		i := start
		if c == 0xC2 {
			i += 2 // skip 2-byte NBSP (U+00A0)
		} else {
			i++
		}
		for i < n {
			b := src[i]
			if b == ' ' || b == '\t' || b == '\r' {
				i++
			} else if b == 0xC2 && i+1 < n && src[i+1] == 0xA0 {
				i += 2 // NBSP
			} else {
				break
			}
		}
		*col += i - start
		*pos = i
		return Token{Kind: Whitespace, Start: start, End: i, Line: startLine, Col: startCol}

	// ── Stage-URI scheme separator `://` ────────────────────────────────
	// A run of `/` right after `:` is a URI authority separator
	// (file:///path, s3://bucket, azure://, gcs://) in PUT/GET, not a `//`
	// line comment. Consume the whole slash run as one operator so the
	// trailing `/path` isn't re-scanned and mistaken for a `//` comment.
	// Only `:` triggers this — a `/` before `//` (e.g. after a block-comment
	// close `*/`) must still start a line comment.
	case c == '/' && start+1 < n && src[start+1] == '/' && start > 0 && src[start-1] == ':':
		i := start + 1
		for i < n && src[i] == '/' {
			i++
		}
		*col += i - start
		*pos = i
		return Token{Kind: Operator, Start: start, End: i, Line: startLine, Col: startCol}

	// ── Line comment -- or // ───────────────────────────────────────────
	// Snowflake treats both `--` and `//` as line comments.
	case start+1 < n && ((c == '-' && src[start+1] == '-') || (c == '/' && src[start+1] == '/')):
		i := start + 2
		if nl := strings.IndexByte(src[i:], '\n'); nl < 0 {
			i = n
		} else {
			i += nl // stop before the \n; it becomes its own Newline token
		}
		*col += i - start
		*pos = i
		return Token{Kind: LineComment, Start: start, End: i, Line: startLine, Col: startCol}

	// ── Block comment /* */ ─────────────────────────────────────────────
	// Snowflake block comments nest, so track depth: a `/*` before the next
	// `*/` opens an inner comment that must be closed first.
	case c == '/' && start+1 < n && src[start+1] == '*':
		i := start + 2
		unterminated := false
		for depth := 1; depth > 0; {
			openIdx := strings.Index(src[i:], "/*")
			closeIdx := strings.Index(src[i:], "*/")
			if closeIdx < 0 {
				i = n // unterminated — consume to end of input
				unterminated = true
				break
			}
			if openIdx >= 0 && openIdx < closeIdx {
				depth++
				i += openIdx + 2
			} else {
				depth--
				i += closeIdx + 2
			}
		}
		// Count newlines in the block comment for line tracking.
		span := src[start:i]
		nlCount := strings.Count(span, "\n")
		if nlCount > 0 {
			*line += nlCount
			lastNL := strings.LastIndexByte(span, '\n')
			*col = len(span) - lastNL // bytes after last newline + 1 for 1-based
		} else {
			*col += i - start
		}
		*pos = i
		return Token{Kind: BlockComment, Start: start, End: i, Line: startLine, Col: startCol, Unterminated: unterminated}

	// ── Single-quoted string '...' ──────────────────────────────────────
	case c == '\'':
		i := start + 1
		terminated := false
		for i < n {
			j := strings.IndexByte(src[i:], '\'')
			if j < 0 {
				i = n
				break
			}
			i += j + 1
			if i < n && src[i] == '\'' {
				i++ // '' escape
			} else {
				terminated = true
				break
			}
		}
		span := src[start:i]
		nlCount := strings.Count(span, "\n")
		if nlCount > 0 {
			*line += nlCount
			lastNL := strings.LastIndexByte(span, '\n')
			*col = len(span) - lastNL
		} else {
			*col += i - start
		}
		*pos = i
		return Token{Kind: StringLit, Start: start, End: i, Line: startLine, Col: startCol, Unterminated: !terminated}

	// ── Double-quoted identifier "..." ───────────────────────────────────
	case c == '"':
		i := start + 1
		terminated := false
		for i < n {
			j := strings.IndexByte(src[i:], '"')
			if j < 0 {
				i = n
				break
			}
			i += j + 1
			if i < n && src[i] == '"' {
				i++ // "" escape
			} else {
				terminated = true
				break
			}
		}
		span := src[start:i]
		nlCount := strings.Count(span, "\n")
		if nlCount > 0 {
			*line += nlCount
			lastNL := strings.LastIndexByte(span, '\n')
			*col = len(span) - lastNL
		} else {
			*col += i - start
		}
		*pos = i
		return Token{Kind: QuotedIdent, Start: start, End: i, Line: startLine, Col: startCol, Unterminated: !terminated}

	// ── Dollar-quoted $$...$$ or $tag$...$tag$ ──────────────────────────
	case c == '$':
		// Try to extract a dollar-quote tag.
		j := start + 1
		for j < n && isIdentByte(src[j]) {
			j++
		}
		if j < n && src[j] == '$' {
			tag := src[start : j+1]
			i := j + 1
			unterminated := false
			if end := strings.Index(src[i:], tag); end < 0 {
				i = n
				unterminated = true
			} else {
				i += end + len(tag)
			}
			span := src[start:i]
			nlCount := strings.Count(span, "\n")
			if nlCount > 0 {
				*line += nlCount
				lastNL := strings.LastIndexByte(span, '\n')
				*col = len(span) - lastNL
			} else {
				*col += i - start
			}
			*pos = i
			return Token{Kind: DollarQuoted, Start: start, End: i, Line: startLine, Col: startCol, Tag: tag, Unterminated: unterminated}
		}
		// Not a dollar-quote — emit as Other.
		*pos++
		*col++
		return Token{Kind: Other, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Semicolon ────────────────────────────────────────────────────────
	case c == ';':
		*pos++
		*col++
		return Token{Kind: Semicolon, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Single-char punctuation ──────────────────────────────────────────
	case c == '(':
		*pos++
		*col++
		return Token{Kind: LParen, Start: start, End: start + 1, Line: startLine, Col: startCol}
	case c == ')':
		*pos++
		*col++
		return Token{Kind: RParen, Start: start, End: start + 1, Line: startLine, Col: startCol}
	case c == '[':
		*pos++
		*col++
		return Token{Kind: LBracket, Start: start, End: start + 1, Line: startLine, Col: startCol}
	case c == ']':
		*pos++
		*col++
		return Token{Kind: RBracket, Start: start, End: start + 1, Line: startLine, Col: startCol}
	case c == ',':
		*pos++
		*col++
		return Token{Kind: Comma, Start: start, End: start + 1, Line: startLine, Col: startCol}
	case c == '.':
		// Check if this starts a numeric literal like .5
		if start+1 < n && src[start+1] >= '0' && src[start+1] <= '9' {
			return scanNumber(src, n, pos, line, col, start, startLine, startCol)
		}
		*pos++
		*col++
		return Token{Kind: Dot, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Colon ────────────────────────────────────────────────────────────
	case c == ':':
		if start+1 < n && src[start+1] == ':' {
			*pos += 2
			*col += 2
			return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
		}
		*pos++
		*col++
		return Token{Kind: Colon, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── At ───────────────────────────────────────────────────────────────
	case c == '@':
		*pos++
		*col++
		return Token{Kind: At, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Numbers ──────────────────────────────────────────────────────────
	case c >= '0' && c <= '9':
		return scanNumber(src, n, pos, line, col, start, startLine, startCol)

	// ── Word (keyword / identifier) ─────────────────────────────────────
	case isWordStart(c):
		i := start + 1
		for i < n && isWordByte(src[i]) {
			i++
		}
		upper := strings.ToUpper(src[start:i])
		kind := Identifier
		if _, ok := keywords[upper]; ok {
			kind = Keyword
		}
		*col += i - start
		*pos = i
		return Token{Kind: kind, Start: start, End: i, Line: startLine, Col: startCol}

	// ── Multi-char operators ─────────────────────────────────────────────
	case c == '|':
		if start+1 < n && src[start+1] == '|' {
			*pos += 2
			*col += 2
			return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
		}
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	case c == '=':
		if start+1 < n && src[start+1] == '>' {
			*pos += 2
			*col += 2
			return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
		}
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	case c == '<':
		if start+1 < n {
			next := src[start+1]
			if next == '>' || next == '=' {
				*pos += 2
				*col += 2
				return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
			}
		}
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	case c == '>':
		if start+1 < n && src[start+1] == '=' {
			*pos += 2
			*col += 2
			return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
		}
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	case c == '!':
		if start+1 < n && src[start+1] == '=' {
			*pos += 2
			*col += 2
			return Token{Kind: Operator, Start: start, End: start + 2, Line: startLine, Col: startCol}
		}
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Single-char operators ────────────────────────────────────────────
	case c == '+' || c == '*' || c == '%' || c == '^' || c == '/' || c == '-':
		*pos++
		*col++
		return Token{Kind: Operator, Start: start, End: start + 1, Line: startLine, Col: startCol}

	// ── Other ────────────────────────────────────────────────────────────
	default:
		*pos++
		*col++
		return Token{Kind: Other, Start: start, End: start + 1, Line: startLine, Col: startCol}
	}
}

// scanNumber reads a numeric literal starting at start.
//
// It is lenient about digit groups: a "0x" with no following hex digits, or an
// "e"/"E" exponent with no following digits, still yields a NumberLit. This is
// intentional — the tokenizer's job is to classify and never drop input; a
// malformed number is still consumed as a NumberLit rather than split, and any
// strict numeric validation is left to the validators.
func scanNumber(src string, n int, pos, _ /* line */, col *int, start, startLine, startCol int) Token {
	i := start

	// Handle 0x hex prefix
	if i < n && src[i] == '0' && i+1 < n && (src[i+1] == 'x' || src[i+1] == 'X') {
		i += 2
		for i < n && isHexDigit(src[i]) {
			i++
		}
		*col += i - start
		*pos = i
		return Token{Kind: NumberLit, Start: start, End: i, Line: startLine, Col: startCol}
	}

	// Integer part (may start with . for fractional-only literals)
	if i < n && src[i] == '.' {
		i++ // leading dot
	} else {
		for i < n && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		// Fractional part
		if i < n && src[i] == '.' {
			i++
		}
	}
	for i < n && src[i] >= '0' && src[i] <= '9' {
		i++
	}

	// Exponent
	if i < n && (src[i] == 'e' || src[i] == 'E') {
		i++
		if i < n && (src[i] == '+' || src[i] == '-') {
			i++
		}
		for i < n && src[i] >= '0' && src[i] <= '9' {
			i++
		}
	}

	*col += i - start
	*pos = i
	return Token{Kind: NumberLit, Start: start, End: i, Line: startLine, Col: startCol}
}

// isIdentByte reports whether b can appear inside a dollar-quote tag.
func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// isWordStart reports whether b can start an unquoted SQL identifier.
func isWordStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// isWordByte reports whether b can continue an unquoted SQL identifier.
// Includes $ for Snowflake identifiers like SYSTEM$TYPEOF.
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_' || b == '$'
}

// isHexDigit reports whether b is a hexadecimal digit.
func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
