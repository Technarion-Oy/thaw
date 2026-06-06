// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqlutil

import "strings"

// Split tokenises src and returns individual, whitespace-trimmed SQL
// statements.  It correctly handles all quoting and comment styles that
// Snowflake DDL can produce:
//
//   - line comments   --  ...  \n
//   - block comments  /* ... */   (non-nesting per Snowflake spec)
//   - single-quoted string literals  '...'  with '' escapes
//   - double-quoted identifiers      "..."  with "" escapes
//   - dollar-quoted bodies           $$...$$ or $tag$...$tag$
//
// Semicolons inside any of the above are never treated as statement
// terminators.
//
// Implementation notes:
//   - Works entirely at the byte level — no []rune allocation.
//   - Uses strings.Index / strings.IndexByte for comment and quoted-body
//     boundaries; these compile to SIMD-accelerated memchr/memmem on
//     amd64 and arm64, making large procedure bodies very fast to skip.
//   - Plain SQL text is bulk-copied with WriteString rather than emitted
//     one rune at a time.
func Split(src string) []string {
	n := len(src)

	out := make([]string, 0, 64)
	var buf strings.Builder
	buf.Grow(min(n/8, 1<<14)) // pre-allocate a reasonable chunk

	flush := func() {
		if s := strings.TrimSpace(buf.String()); s != "" {
			out = append(out, s)
		}
		buf.Reset()
	}

	i := 0
	for i < n {
		switch src[i] {

		// -- line comment
		case '-':
			if i+1 < n && src[i+1] == '-' {
				start := i
				i += 2
				if nl := strings.IndexByte(src[i:], '\n'); nl < 0 {
					i = n
				} else {
					i += nl + 1
				}
				buf.WriteString(src[start:i])
			} else {
				buf.WriteByte('-')
				i++
			}

		// /* block comment */
		case '/':
			if i+1 < n && src[i+1] == '*' {
				start := i
				i += 2
				if end := strings.Index(src[i:], "*/"); end < 0 {
					i = n
				} else {
					i += end + 2
				}
				buf.WriteString(src[start:i])
			} else {
				buf.WriteByte('/')
				i++
			}

		// 'single-quoted string'
		case '\'':
			start := i
			i++ // skip opening quote
			for i < n {
				j := strings.IndexByte(src[i:], '\'')
				if j < 0 {
					i = n
					break
				}
				i += j + 1 // position after the quote
				if i < n && src[i] == '\'' {
					i++ // '' escape — continue scanning
				} else {
					break // closing quote found
				}
			}
			buf.WriteString(src[start:i])

		// "double-quoted identifier"
		case '"':
			start := i
			i++ // skip opening quote
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
					break
				}
			}
			buf.WriteString(src[start:i])

		// $tag$ dollar-quoted body $tag$
		case '$':
			j := i + 1
			for j < n && isIdentByte(src[j]) {
				j++
			}
			if j < n && src[j] == '$' {
				tag := src[i : j+1] // e.g. "$$" or "$body$" — ASCII only per spec
				start := i
				i = j + 1
				if end := strings.Index(src[i:], tag); end < 0 {
					i = n
				} else {
					i += end + len(tag)
				}
				buf.WriteString(src[start:i])
			} else {
				// Not a valid dollar-quote opener — treat as a literal character.
				buf.WriteByte('$')
				i++
			}

		// ; statement terminator
		case ';':
			flush()
			i++

		// plain SQL text (bulk copy)
		default:
			// Scan forward past all bytes that cannot trigger a state change,
			// then copy the entire span with a single WriteString call.
			start := i
			i++
			for i < n {
				c := src[i]
				if c == '-' || c == '/' || c == '\'' || c == '"' || c == '$' || c == ';' {
					break
				}
				i++
			}
			buf.WriteString(src[start:i])
		}
	}

	flush() // capture any trailing content without a closing semicolon
	return out
}

// isIdentByte reports whether b can appear inside a dollar-quote tag
// (ASCII letters, digits, underscore — Snowflake spec).
// Used by Split; operates at the byte level because tag chars are always ASCII.
func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
