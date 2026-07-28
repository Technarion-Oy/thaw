// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"thaw/internal/sqltok"
)

var reScale = regexp.MustCompile(`\(.*\)$`)

// numericTypeNames is the set of canonical numeric type names, derived from the
// authoritative registry (CategoryNumeric) so adding a numeric type to
// snowflakeDataTypes automatically makes it numeric here too.
var numericTypeNames = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, dt := range snowflakeDataTypes {
		if dt.Category == CategoryNumeric {
			m[dt.Name] = struct{}{}
		}
	}
	return m
}()

// IsBoolean reports whether the given Snowflake data type is a boolean.
func IsBoolean(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	return base == "BOOLEAN" || base == "BOOL"
}

// IsNumeric reports whether the given Snowflake data type is a numeric type.
func IsNumeric(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	_, ok := numericTypeNames[base]
	return ok
}

// NeedsQuotes reports whether a value of the given data type should be quoted in SQL.
// Boolean and numeric values are typically not quoted.
func NeedsQuotes(dataType string) bool {
	return !IsBoolean(dataType) && !IsNumeric(dataType)
}

// DollarQuoteTag returns a dollar-quote delimiter (e.g. "$$" or "$thaw$") that is
// guaranteed not to appear inside body, so wrapping a UDF / procedure body as
// `AS <tag>\n<body>\n<tag>` can never be terminated early by a literal delimiter
// in the body itself. It prefers the bare "$$" when the body doesn't contain it,
// then a small set of named tags, and finally falls back to a numbered tag.
//
// Cost is linear in len(body) regardless of what the body contains.
func DollarQuoteTag(body string) string {
	for _, tag := range []string{"$$", "$thaw$", "$thaw_body$"} {
		if !strings.Contains(body, tag) {
			return tag
		}
	}
	// Probing the numbered tags one at a time ("is $thaw_0$ in the body?
	// $thaw_1$? …") costs a full scan per probe, so a body carrying N decoy
	// tags — trivial to author inside a procedure owned by another account, and
	// then handed to whoever exports that database — would cost
	// O(N · len(body)). Collect the numbered tags the body actually uses in one
	// pass instead, then take the first free index.
	used := numberedTags(body)
	for i := 0; ; i++ {
		if _, taken := used[i]; !taken {
			return fmt.Sprintf("$thaw_%d$", i)
		}
	}
}

// numberedTags returns the set of indexes i for which body contains the exact
// tag "$thaw_<i>$". A non-canonical digit run ("$thaw_007$") names no tag this
// package can ever emit, so it is ignored rather than reserving index 7.
func numberedTags(body string) map[int]struct{} {
	const prefix = "$thaw_"
	used := make(map[int]struct{})
	for i := 0; ; {
		j := strings.Index(body[i:], prefix)
		if j < 0 {
			return used
		}
		start := i + j + len(prefix)
		end := start
		for end < len(body) && body[end] >= '0' && body[end] <= '9' {
			end++
		}
		if end > start && end < len(body) && body[end] == '$' {
			digits := body[start:end]
			// Atoi rejects a run too long to be an int; such a tag is far above
			// any index the search below can reach, so skipping it is safe.
			if n, err := strconv.Atoi(digits); err == nil && strconv.Itoa(n) == digits {
				used[n] = struct{}{}
			}
		}
		// The prefix contains no "$" of its own, so no occurrence can start
		// inside the one just consumed.
		i = start
	}
}

// ─── quoted-string body → dollar-quoted body ─────────────────────────────────

// UnescapeStringLiteral decodes the *inner* text of a Snowflake single-quoted
// string literal (the characters between the quotes) into the value it denotes,
// applying the escaping rules for single-quoted string constants:
//
//	''        →  '
//	\' \" \\  →  ' " \
//	\b \f \n \r \t
//	\ooo      →  octal code point (1–3 digits; \0 is NUL)
//	\xhh      →  hex code point (2 digits)
//	\uhhhh    →  Unicode code point (4 digits)
//	\<other>  →  <other>   (the backslash is dropped)
//
// The second result reports whether the decoded value can be reproduced
// *verbatim* — i.e. embedded in a dollar-quoted body, where no escaping exists.
// It is false when the value carries a control character other than newline,
// carriage return, or tab (a `\0`, `\b`, `\f`, … that only the escaped form can
// express), when a numeric escape names a non-ASCII code point (Snowflake's
// byte-versus-code-point semantics there are ambiguous, so a rewrite could
// silently change the value), or when the input is not valid UTF-8. The decoded
// string is still returned in those cases, but callers that rewrite SQL must
// leave the original literal alone.
//
// The numeric escapes are decoded as written, not range-checked: `\ooo` accepts
// three octal digits, so `\777` decodes to U+01FF rather than being rejected as
// past the byte Snowflake means there. Everything above U+007F already sets the
// second result to false, so a rewriter is unaffected — but a future caller that
// consumes the decoded value itself should not assume the range was validated.
func UnescapeStringLiteral(inner string) (string, bool) {
	if !strings.ContainsAny(inner, `'\`) {
		return inner, isVerbatimSafe(inner)
	}

	var b strings.Builder
	b.Grow(len(inner))
	unambiguous := true

	for i := 0; i < len(inner); {
		c := inner[i]

		// A lone quote cannot occur inside a well-formed literal, so every quote
		// starts the '' escape; a trailing unpaired one is copied through.
		if c == '\'' {
			b.WriteByte('\'')
			i++
			if i < len(inner) && inner[i] == '\'' {
				i++
			}
			continue
		}
		if c != '\\' || i+1 == len(inner) {
			b.WriteByte(c)
			i++
			continue
		}

		i++ // consume the backslash
		switch e := inner[i]; {
		case e == '\'', e == '"', e == '\\':
			b.WriteByte(e)
			i++
		case e == 'b':
			b.WriteByte('\b')
			i++
		case e == 'f':
			b.WriteByte('\f')
			i++
		case e == 'n':
			b.WriteByte('\n')
			i++
		case e == 'r':
			b.WriteByte('\r')
			i++
		case e == 't':
			b.WriteByte('\t')
			i++
		case e >= '0' && e <= '7':
			v, n := readRadix(inner[i:], 8, 3)
			b.WriteRune(rune(v))
			i += n
			unambiguous = unambiguous && v < utf8.RuneSelf
		case e == 'x' || e == 'u':
			digits := 2
			if e == 'u' {
				digits = 4
			}
			v, n := readRadix(inner[i+1:], 16, digits)
			if n < digits {
				// Not a well-formed numeric escape, so it is not one at all:
				// the backslash is dropped and the letter stands for itself.
				b.WriteByte(e)
				i++
				continue
			}
			b.WriteRune(rune(v))
			i += 1 + n
			// \uhhhh always names a code point, so only surrogate halves (which
			// have no UTF-8 encoding — WriteRune substitutes U+FFFD) are
			// unfaithful. \xhh ≥ 0x80 may instead mean a raw byte.
			faithful := v < utf8.RuneSelf || (e == 'u' && (v < 0xD800 || v > 0xDFFF))
			unambiguous = unambiguous && faithful
		default:
			b.WriteByte(e)
			i++
		}
	}

	out := b.String()
	return out, unambiguous && isVerbatimSafe(out)
}

// readRadix reads up to maxDigits digits of the given radix (8 or 16) from the
// front of s and returns their value together with the number of bytes
// consumed. A zero count means s does not start with a digit of that radix.
func readRadix(s string, radix, maxDigits int) (int, int) {
	v, n := 0, 0
	for n < maxDigits && n < len(s) {
		d := digitValue(s[n])
		if d < 0 || d >= radix {
			break
		}
		v = v*radix + d
		n++
	}
	return v, n
}

// digitValue returns the value of an ASCII digit in base 16, or -1 when c is
// not one. Callers bound the result to the radix they want.
func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// isVerbatimSafe reports whether s can be written into a SQL file as-is: valid
// UTF-8 with no control character — C0, DEL, or C1 — other than newline,
// carriage return, and tab.
func isVerbatimSafe(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		switch {
		case r == '\n', r == '\r', r == '\t':
			// The only control characters a SQL file carries verbatim.
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			// C0, DEL, and C1 — invisible in an editor and lost or mangled by
			// anything that normalizes the file.
			return false
		}
	}
	return true
}

// createBodyModifiers are the keywords Snowflake allows between CREATE (or
// CREATE OR REPLACE) and the FUNCTION / PROCEDURE keyword of a definition that
// carries an inline body, e.g. `CREATE OR REPLACE SECURE FUNCTION …`,
// `CREATE TEMPORARY PROCEDURE …`, `CREATE DATA METRIC FUNCTION …`.
//
// EXTERNAL is deliberately absent: an external function's `AS '<url>'` names a
// remote-service endpoint rather than a body, so rewriting it would be noise at
// best.
var createBodyModifiers = map[string]struct{}{
	"SECURE": {}, "TEMPORARY": {}, "TEMP": {},
	"AGGREGATE": {}, "DATA": {}, "METRIC": {},
}

// DollarQuoteBody rewrites the body of a CREATE FUNCTION / CREATE PROCEDURE
// statement from the single-quoted string-literal form GET_DDL returns into the
// dollar-quoted form, so the exported SQL reads (and highlights) as code:
//
//	AS 'begin\n  let x := ''hello'';\nend'   →   AS $$begin
//	                                              let x := 'hello';
//	                                            end$$
//
// The delimiter comes from [DollarQuoteTag], narrowed by closingSafeTag so that
// neither the body's own content nor the seam where the body meets the closing
// delimiter can terminate it early.
//
// stmt is returned unchanged — never partially rewritten — when it is not a
// function or procedure definition, when the body is already dollar-quoted or
// empty, or when the literal does not decode to text that survives being
// embedded verbatim (see [UnescapeStringLiteral]). Callers can therefore apply
// it unconditionally to any statement.
func DollarQuoteBody(stmt string) string {
	sig := sqltok.SignificantTokens(stmt)
	if !isFunctionOrProcedureCreate(sig, stmt) {
		return stmt
	}

	// The body is the string literal introduced by the last top-level AS: an AS
	// inside parentheses belongs to a RETURNS TABLE column list, and an earlier
	// one to a clause the body's AS supersedes.
	depth, body := 0, -1
	for i, t := range sig {
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		case sqltok.Keyword, sqltok.Identifier:
			if depth == 0 && i+1 < len(sig) && sig[i+1].Kind == sqltok.StringLit &&
				strings.EqualFold(t.Text(stmt), "AS") {
				body = i + 1
			}
		}
	}
	if body < 0 {
		return stmt
	}

	lit := sig[body]
	if lit.Unterminated {
		return stmt
	}
	value, ok := UnescapeStringLiteral(stmt[lit.Start+1 : lit.End-1])
	if !ok || value == "" {
		return stmt
	}

	tag, tagged := closingSafeTag(value)
	if !tagged {
		return stmt
	}
	return stmt[:lit.Start] + tag + value + tag + stmt[lit.End:]
}

// closingSafeTag returns a dollar-quote delimiter that survives being appended
// to value with no separator between the two.
//
// [DollarQuoteTag] alone is not enough: it guarantees only that the tag does not
// occur *inside* the body, while the seam between the body's last characters and
// the closing delimiter can form one. Every tag both starts and ends with "$",
// so a body whose tail is a proper prefix of the tag ("x$" for "$$",
// "…$thaw" for "$thaw$") lets the parser — which takes the *first* occurrence of
// the tag after the opening one, as [sqltok] and Snowflake both do — close the
// body early and truncate it.
//
// Rather than enumerating the shapes that can overlap, the seam is checked
// directly: the tag's first occurrence in value+tag must be the appended one.
// Each rejected tag is folded into the string handed to DollarQuoteTag, so the
// next round is guaranteed to propose a different delimiter.
//
// A leading "$" in the body needs no such care — the scan for the closing tag
// starts strictly after the opening one, so the body's head can never be part
// of a match.
//
// The second result is false when no safe tag was found within
// maxTagEscalations rounds. A body has a single tail, so real input settles in
// one or two rounds; the cap only exists so that no input at all can turn the
// search into a long loop over a growing string, and the caller then falls back
// to leaving the literal exactly as Snowflake wrote it.
func closingSafeTag(value string) (string, bool) {
	rejected := value
	for range maxTagEscalations {
		tag := DollarQuoteTag(rejected)
		if strings.Index(value+tag, tag) == len(value) {
			return tag, true
		}
		rejected += tag
	}
	return "", false
}

// maxTagEscalations bounds the delimiter search in closingSafeTag.
const maxTagEscalations = 64

// isFunctionOrProcedureCreate reports whether the significant-token stream is a
// CREATE FUNCTION or CREATE PROCEDURE statement, looking past the OR REPLACE /
// OR ALTER prelude and the modifier keywords Snowflake allows in between.
//
// GET_DDL only ever emits CREATE OR REPLACE, but hand-written SQL — which any
// other consumer of [DollarQuoteBody] would be reading — also uses OR ALTER.
func isFunctionOrProcedureCreate(sig []sqltok.Token, src string) bool {
	if len(sig) == 0 || !strings.EqualFold(sig[0].Text(src), "CREATE") {
		return false
	}
	i := 1
	if i+1 < len(sig) && strings.EqualFold(sig[i].Text(src), "OR") &&
		(strings.EqualFold(sig[i+1].Text(src), "REPLACE") ||
			strings.EqualFold(sig[i+1].Text(src), "ALTER")) {
		i += 2
	}
	for ; i < len(sig); i++ {
		word := strings.ToUpper(sig[i].Text(src))
		if word == "FUNCTION" || word == "PROCEDURE" {
			return true
		}
		if _, isMod := createBodyModifiers[word]; !isMod {
			return false
		}
	}
	return false
}

// IsHandlerLanguage reports whether the given UDF / stored-procedure language is
// one of the handler languages (Python, Java, Scala) that carry their logic in a
// separate handler and therefore accept the RUNTIME_VERSION / PACKAGES / IMPORTS
// / HANDLER clauses. SQL and JavaScript (and the empty default, which is SQL)
// embed their logic inline in the body and do not. The comparison is
// case-insensitive. Shared by the CREATE FUNCTION and CREATE PROCEDURE builders.
func IsHandlerLanguage(language string) bool {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "PYTHON", "JAVA", "SCALA":
		return true
	default:
		return false
	}
}
