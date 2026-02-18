// Package ddl provides tools for parsing and splitting Snowflake DDL strings
// into individual SQL statements, and for extracting per-object metadata so
// that each statement can be written to an appropriately named file.
package ddl

import "strings"

// psState is the current lexer state.
type psState uint8

const (
	psNormal      psState = iota
	psLineComment         // after --
	psBlockComment        // inside /* … */
	psSingleQuote         // inside '…'
	psDoubleQuote         // inside "…"
	psDollarQuote         // inside $tag$…$tag$
)

// Split tokenises src and returns individual, whitespace-trimmed SQL
// statements.  It correctly handles all quoting and comment styles that
// Snowflake DDL can produce:
//
//   - line comments   --  …  \n
//   - block comments  /* … */   (non-nesting per Snowflake spec)
//   - single-quoted string literals  '…'  with '' escapes
//   - double-quoted identifiers      "…"  with "" escapes
//   - dollar-quoted bodies           $$…$$  or  $tag$…$tag$
//
// Semicolons inside any of the above are never treated as statement
// terminators.
func Split(src string) []string {
	rs := []rune(src)
	n := len(rs)

	out := make([]string, 0, 64)
	var buf strings.Builder
	buf.Grow(min(len(src)/8, 1<<14)) // pre-allocate a reasonable chunk

	st := psNormal
	var dollarTag []rune // the current opening tag, e.g. []rune("$$") or []rune("$body$")

	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			out = append(out, s)
		}
		buf.Reset()
	}

	for i := 0; i < n; {
		r := rs[i]

		switch st {
		// ── normal text ──────────────────────────────────────────────────────
		case psNormal:
			switch {

			// -- line comment
			case r == '-' && i+1 < n && rs[i+1] == '-':
				buf.WriteRune(r)
				buf.WriteRune(rs[i+1])
				i += 2
				st = psLineComment

			// /* block comment */
			case r == '/' && i+1 < n && rs[i+1] == '*':
				buf.WriteRune(r)
				buf.WriteRune(rs[i+1])
				i += 2
				st = psBlockComment

			// 'single-quoted string'
			case r == '\'':
				buf.WriteRune(r)
				i++
				st = psSingleQuote

			// "double-quoted identifier"
			case r == '"':
				buf.WriteRune(r)
				i++
				st = psDoubleQuote

			// $tag$ dollar-quoted body
			// Tag chars: only letters, digits, underscores (Snowflake spec).
			// An immediately repeated $ ($$) is also valid (empty tag).
			case r == '$':
				j := i + 1
				for j < n && (isIdentRune(rs[j])) {
					j++
				}
				if j < n && rs[j] == '$' {
					tag := rs[i : j+1]
					buf.WriteString(string(tag))
					dollarTag = tag
					i = j + 1
					st = psDollarQuote
				} else {
					// Not a valid dollar-quote opener — treat as a literal character.
					buf.WriteRune(r)
					i++
				}

			// ; statement terminator
			case r == ';':
				flush()
				i++

			default:
				buf.WriteRune(r)
				i++
			}

		// ── -- line comment ──────────────────────────────────────────────────
		case psLineComment:
			buf.WriteRune(r)
			i++
			if r == '\n' {
				st = psNormal
			}

		// ── /* block comment */ ──────────────────────────────────────────────
		case psBlockComment:
			buf.WriteRune(r)
			if r == '*' && i+1 < n && rs[i+1] == '/' {
				buf.WriteRune(rs[i+1])
				i += 2
				st = psNormal
			} else {
				i++
			}

		// ── 'single-quoted string' ───────────────────────────────────────────
		case psSingleQuote:
			buf.WriteRune(r)
			i++
			if r == '\'' {
				if i < n && rs[i] == '\'' { // '' escape sequence
					buf.WriteRune(rs[i])
					i++
				} else {
					st = psNormal
				}
			}

		// ── "double-quoted identifier" ───────────────────────────────────────
		case psDoubleQuote:
			buf.WriteRune(r)
			i++
			if r == '"' {
				if i < n && rs[i] == '"' { // "" escape sequence
					buf.WriteRune(rs[i])
					i++
				} else {
					st = psNormal
				}
			}

		// ── $tag$ dollar-quoted body $tag$ ───────────────────────────────────
		case psDollarQuote:
			tl := len(dollarTag)
			// Check whether we are at the start of the matching closing tag.
			if r == '$' && i+tl <= n && runesEqual(rs[i:i+tl], dollarTag) {
				buf.WriteString(string(dollarTag))
				i += tl
				st = psNormal
				dollarTag = nil
			} else {
				buf.WriteRune(r)
				i++
			}
		}
	}

	flush() // capture any trailing content without a closing semicolon
	return out
}

// isIdentRune reports whether r can appear inside a dollar-quote tag
// (letters, digits, underscore).
func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}

// runesEqual reports whether two rune slices have identical contents.
func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
