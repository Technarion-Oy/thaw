// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"thaw/internal/sqltok"
)

// reUnquotedIdent matches a Snowflake bare (unquoted) identifier: starts with
// a letter or underscore, followed by letters, digits, underscores, or dollar
// signs. The pattern is case-insensitive because Snowflake normalizes unquoted
// identifiers to uppercase regardless of how they were written.
var reUnquotedIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// ValidateEnumValue uppercases value and checks it against allowed, returning
// the canonical uppercase form safe for unquoted SQL interpolation. what names
// the target in the error (e.g. `user property "type"`). Shared by the
// per-property ALTER builders in internal/warehouse and internal/users.
func ValidateEnumValue(what, value string, allowed ...string) (string, error) {
	up := strings.ToUpper(strings.TrimSpace(value))
	if slices.Contains(allowed, up) {
		return up, nil
	}
	return "", fmt.Errorf("invalid value %q for %s", value, what)
}

// ValidateNonNegativeInt parses value as a non-negative integer and returns its
// canonical decimal form safe for SQL interpolation. what names the target in
// the error, mirroring ValidateEnumValue.
func ValidateNonNegativeInt(what, value string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return "", fmt.Errorf("invalid integer value %q for %s", value, what)
	}
	return strconv.Itoa(n), nil
}

// TableKey returns the canonical lookup key for a Snowflake table:
// "SCHEMA.TABLE" with both parts trimmed. Names arrive from Snowflake
// metadata already in their canonical stored form (uppercase for unquoted
// identifiers, original case for quoted ones), so no case folding is applied.
func TableKey(schema, name string) string {
	return strings.TrimSpace(schema) + "." + strings.TrimSpace(name)
}

// NeedsQuoting reports whether the given Snowflake object name must be
// double-quoted when used in a SQL statement. A name requires quoting when:
//   - it contains characters outside [A-Za-z0-9_$] or does not start with
//     [A-Za-z_] (i.e. it cannot be expressed as a bare identifier), OR
//   - it is a Snowflake reserved keyword (case-insensitive comparison).
//
// Note: this function does NOT account for QUOTED_IDENTIFIERS_IGNORE_CASE.
// Call GetQuotedIdentifiersIgnoreCase to determine whether the session treats
// quoted and unquoted identifiers as case-equivalent.
func NeedsQuoting(name string) bool {
	if !reUnquotedIdent.MatchString(name) {
		return true
	}
	return sqltok.IsReserved(strings.ToUpper(name))
}

// ReservedKeywords returns the full list of Snowflake reserved keywords.
// The returned slice is sorted alphabetically. Callers must not modify it.
func ReservedKeywords() []string {
	return sqltok.ReservedKeywordList()
}

// QuoteIdent wraps name in double-quotes, escaping any embedded double-quotes.
func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Qualify builds a dotted, fully-qualified Snowflake object reference from its
// name parts, double-quoting each part via QuoteIdent — Qualify("DB", "S", "T")
// yields `"DB"."S"."T"`. It is the shared builder behind the database.schema.name
// and schema.name references that pepper the DDL/SHOW builders. Every part is
// quoted unconditionally; for a reference whose final component is already
// quoted (a generated stage name) or only conditionally quoted (QuoteOrBare),
// build the dotted name inline instead.
func Qualify(parts ...string) string {
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = QuoteIdent(p)
	}
	return strings.Join(quoted, ".")
}

// QualifyOrBare builds a three-part `"db"."schema".name` reference in which db
// and schema are always double-quoted (QuoteIdent) while the trailing name is
// quoted only when required — QuoteOrBare(name, caseSensitive) leaves a valid,
// non-reserved identifier bare so Snowflake applies its uppercasing. It is the
// FQN builder shared by the CREATE/ALTER builders whose final component is an
// authored object name; use Qualify when every part should be quoted
// unconditionally.
func QualifyOrBare(db, schema, name string, caseSensitive bool) string {
	return QuoteIdent(db) + "." + QuoteIdent(schema) + "." + QuoteOrBare(name, caseSensitive)
}

// IdentPart is one dotted segment of a qualified Snowflake name: its logical
// (unquoted, unescaped) text and whether the source wrapped it in double
// quotes. Quoted signals case-sensitive intent, so a caller re-rendering the
// name emits QuoteIdent for a quoted part and QuoteOrBare for a bare one.
type IdentPart struct {
	Text   string
	Quoted bool
}

// SplitQualifiedName splits a qualified Snowflake reference (DATABASE,
// DATABASE.SCHEMA, DB.SCHEMA.OBJECT, …) into its dotted parts using the shared
// sqltok tokenizer. Because the split is quote-aware, a quoted identifier
// containing a literal dot stays one part — `"MY.DB".PUB` yields
// ["MY.DB", "PUB"] — unlike strings.Split(s, "."), which produces three bogus
// segments. Each part's Text is unquoted via sqltok.Unquote (doubled quotes
// collapsed) and guaranteed non-empty.
//
// maxParts caps how many dotted parts are consumed; maxParts <= 0 means
// unbounded. The whole input must be exactly one identifier path: leftover
// tokens (a trailing dot, parts beyond maxParts, any stray token), empty
// segments, and unterminated quotes all return an error.
//
// This is the shared, quote-correct alternative to strings.Split on a qualified
// name; use it wherever a DB/SCHEMA/OBJECT reference is parsed.
func SplitQualifiedName(s string, maxParts int) ([]IdentPart, error) {
	trimmed := strings.TrimSpace(s)
	tokens := sqltok.Tokenize(trimmed)
	raw, next := sqltok.ReadIdentParts(tokens, trimmed, 0, maxParts)
	if raw == nil || next >= len(tokens) || tokens[next].Kind != sqltok.EOF {
		return nil, fmt.Errorf("invalid qualified name %q", s)
	}
	parts := make([]IdentPart, len(raw))
	for i, p := range raw {
		if strings.HasPrefix(p, `"`) {
			// Quoted identifier: must be a terminated pair.
			if len(p) < 2 || !strings.HasSuffix(p, `"`) {
				return nil, fmt.Errorf("unbalanced quotes in %q", s)
			}
			parts[i] = IdentPart{Text: sqltok.Unquote(p), Quoted: true}
		} else {
			parts[i] = IdentPart{Text: p, Quoted: false}
		}
		if parts[i].Text == "" {
			return nil, fmt.Errorf("empty segment in %q", s)
		}
	}
	return parts, nil
}

// IdentEqual reports whether the raw identifier text raw — as it appears in a
// SQL or metadata string, possibly double-quoted — refers to the same object as
// the logical (already unquoted) name. It applies Snowflake's identifier
// folding rules: a quoted raw is unescaped and compared exactly (quoting is how
// case is preserved), while a bare raw is compared case-insensitively (Snowflake
// uppercases unquoted identifiers, so MyTable, MYTABLE and mytable are one name).
//
// Use it instead of comparing strings.ToUpper on both sides, which wrongly folds
// a case-sensitive quoted name and never unescapes doubled quotes.
func IdentEqual(raw, name string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) && len(raw) >= 2 {
		return sqltok.Unquote(raw) == name
	}
	return strings.EqualFold(raw, name)
}

// splitIdent splits a (possibly quoted, possibly multi-part) identifier string
// into its component parts, stripping surrounding double-quotes from each part.
// It is the rough parsing inverse of Qualify: Qualify("DB","S","T") builds
// `"DB"."S"."T"`, splitIdent turns it back into ["DB","S","T"]. The split is on
// every "." regardless of quoting, so a quoted part that itself contains a dot
// is not preserved — callers pass identifiers whose component dots are already
// delimited (e.g. tokens from sqltok.ReadIdentPath). For an arbitrary
// user-supplied qualified name, use the quote-aware SplitQualifiedName instead.
func splitIdent(s string) []string {
	var parts []string
	for _, p := range strings.Split(s, ".") {
		parts = append(parts, stripQuotes(p))
	}
	return parts
}

// stripQuotes trims surrounding whitespace and a single outer double-quote pair
// from an identifier part (e.g. `"My Table"` → `My Table`). Embedded doubled
// quotes are left as-is; for full SQL-token unquoting use unquoteSQLToken.
func stripQuotes(s string) string {
	return sqltok.StripQuotePair(strings.TrimSpace(s))
}

// CreateClause assembles the `CREATE [OR REPLACE] <body> [IF NOT EXISTS]` prefix
// of a DDL statement, enforcing the OR REPLACE / IF NOT EXISTS mutual exclusivity
// Snowflake requires: the two may not appear together, and OR REPLACE wins when
// both are requested. body is the object-type phrase that follows the optional
// OR REPLACE — callers fold any leading SECURE/TRANSIENT modifiers into it (e.g.
// "MASKING POLICY", "SECURE EXTERNAL FUNCTION", "TRANSIENT DYNAMIC TABLE"). The
// result carries no trailing space; the fully-qualified name follows it.
func CreateClause(body string, orReplace, ifNotExists bool) string {
	var sb strings.Builder
	sb.WriteString("CREATE")
	if orReplace {
		sb.WriteString(" OR REPLACE")
	}
	sb.WriteByte(' ')
	sb.WriteString(body)
	if ifNotExists && !orReplace {
		sb.WriteString(" IF NOT EXISTS")
	}
	return sb.String()
}

// EscapeStringLit escapes single-quotes within a SQL string literal value by
// doubling them. It deliberately leaves backslashes untouched so that callers
// emitting delimiter/control values (e.g. a file format's RECORD_DELIMITER =
// '\n') keep Snowflake's backslash escape sequences intact. For free-text
// values such as comments — where a backslash should appear literally — use
// EscapeTextLit instead. It does not add the surrounding single-quote
// delimiters.
func EscapeStringLit(s string) string {
	return strings.ReplaceAll(s, `'`, `''`)
}

// EscapeTextLit escapes a free-text value for use inside a single-quoted SQL
// string literal. Snowflake treats the backslash as an escape character within
// single-quoted literals, so a lone backslash must be doubled or it is
// swallowed (e.g. "C:\temp" would otherwise be read as "C:temp"). Single-quotes
// are doubled as well. Backslashes are escaped first so the doubled quotes are
// not themselves mistaken for an escape sequence. It does not add the
// surrounding single-quote delimiters. Use this for human-entered text
// (comments, descriptions); use EscapeStringLit for delimiter/control values
// where backslash escapes are intentional.
func EscapeTextLit(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `''`)
}

// QuoteStringLit wraps s in single-quotes, doubling any embedded single-quotes.
func QuoteStringLit(s string) string {
	return `'` + EscapeStringLit(s) + `'`
}

// QuoteTextLit wraps s in single-quotes for use as a free-text SQL string literal,
// escaping via EscapeTextLit (backslashes and single-quotes doubled). Use it for
// human-entered text such as comments — it is the quoting counterpart of
// EscapeTextLit. Use QuoteStringLit for delimiter/control values where backslash
// escape sequences are intentional.
func QuoteTextLit(s string) string {
	return `'` + EscapeTextLit(s) + `'`
}

// IsNumericID reports whether s is a non-empty string of decimal digits that
// fits in an int64, with no leading zeros. Use it to validate a value that will
// be embedded unquoted into SQL as a bare integer argument (e.g. a Snowflake
// SESSION_ID, which is an int64): the digit-only requirement blocks argument
// injection, the int64 bound rejects over-long pastes that would otherwise
// surface as a raw numeric-overflow error, and rejecting leading zeros keeps the
// embedded value equal to what the user typed (Snowflake reads "007" as 7).
func IsNumericID(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > 1 && s[0] == '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	// Digit-only above means no sign/whitespace; ParseInt only adds the int64
	// range check (catches > 19 digits and the 19-digit overflow window).
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// CommentClause returns the optional COMMENT clause appended by the CREATE/ALTER
// builders: "\n  COMMENT = '<escaped>'" when comment is non-blank, or "" when it
// is empty. The value is escaped as free text via QuoteTextLit (EscapeTextLit),
// so a backslash in a user-entered comment is preserved rather than swallowed as
// a Snowflake string-literal escape. The leading newline and two-space indent
// match the builders' clause layout; for an ALTER … SET comment assembled into a
// comma-joined SET list, emit `COMMENT = ` + QuoteTextLit(comment) directly.
func CommentClause(comment string) string {
	if comment == "" {
		return ""
	}
	return "\n  COMMENT = " + QuoteTextLit(comment)
}

// CleanList returns items with each entry trimmed of surrounding whitespace and
// every blank entry dropped, preserving order. It is the slice-in / slice-out
// counterpart of SplitValues (which tokenizes a delimited string) and underlies
// the list helpers that quote or join an identifier/column slice.
func CleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// JoinCleanList joins items with sep after trimming each entry and dropping
// blanks (via CleanList). Unlike FormatStringLitList it adds no quoting or
// surrounding parentheses — it is the bare comma-list builder for clauses that
// take an unquoted, unparenthesized column list (e.g. ATTRIBUTES col, …). An
// empty or all-blank slice yields "".
func JoinCleanList(items []string, sep string) string {
	return strings.Join(CleanList(items), sep)
}

// FormatStringLitList renders a token slice into the SQL `('A', 'B')` list
// grammar — each non-blank token (trimmed) becomes a single-quoted string
// literal with embedded backslashes/single-quotes escaped via EscapeTextLit.
// Blank tokens are skipped, so an empty or all-blank slice yields "()". It is the
// serialization counterpart of ParseSqlList and is shared by the policy builders
// (authentication / packages) that emit single-quoted-literal list parameters.
func FormatStringLitList(tokens []string) string {
	cleaned := CleanList(tokens)
	parts := make([]string, 0, len(cleaned))
	for _, t := range cleaned {
		parts = append(parts, "'"+EscapeTextLit(t)+"'")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// HasNonBlankToken reports whether tokens contains at least one non-blank entry,
// so a caller can omit a list parameter whose only entries are empty strings
// (which would otherwise serialize to the invalid empty list "()").
func HasNonBlankToken(tokens []string) bool {
	for _, t := range tokens {
		if strings.TrimSpace(t) != "" {
			return true
		}
	}
	return false
}

// FirstNonEmpty returns the first argument that is non-empty after trimming, or
// "" when every argument is blank. It is the SQL-builder analog of a COALESCE
// over identifier/name parts.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// EscapeLikePattern escapes LIKE-special characters (% and _) in s so that
// the string matches literally when used in a SHOW … LIKE '<pattern>' clause.
// Single-quotes are also doubled (same as EscapeStringLit).
func EscapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `''`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// QuoteOrBare returns a double-quoted identifier when caseSensitive is true or
// when the name requires quoting (invalid bare identifier or reserved keyword);
// otherwise it returns the name unquoted (Snowflake will uppercase it).
func QuoteOrBare(name string, caseSensitive bool) string {
	if caseSensitive || NeedsQuoting(name) {
		return QuoteIdent(name)
	}
	return name
}

// SplitValues splits s into trimmed, non-empty tokens on commas and newlines.
// It performs no quoting or validation — it is the raw tokenizer behind the
// identifier-list helpers and is also handy when a caller needs the bare values
// (e.g. to validate each before emitting).
func SplitValues(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' }) {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// QuoteIdentList trims and drops empty entries from names and quotes each via
// QuoteOrBare(name, caseSensitive). Use it when the values already arrive as a
// slice (e.g. multi-select input); for a delimited string use SplitIdentList.
func QuoteIdentList(names []string, caseSensitive bool) []string {
	cleaned := CleanList(names)
	out := make([]string, 0, len(cleaned))
	for _, n := range cleaned {
		out = append(out, QuoteOrBare(n, caseSensitive))
	}
	return out
}

// SplitIdentList splits a comma/newline-separated string into trimmed, non-empty
// identifiers, each quoted via QuoteOrBare(value, caseSensitive). Pass
// caseSensitive=true to force double-quoting (equivalent to QuoteIdent on every
// entry).
func SplitIdentList(s string, caseSensitive bool) []string {
	return QuoteIdentList(SplitValues(s), caseSensitive)
}

// FormatSecondaryRoles renders a Snowflake secondary-role list value — the
// ( 'ALL' | <role> [, <role> ...] ) grammar shared by ALLOWED_SECONDARY_ROLES /
// BLOCKED_SECONDARY_ROLES (session and authentication policies) and
// DEFAULT_SECONDARY_ROLES (ALTER USER). The special token "ALL"
// (case-insensitive) becomes the quoted string literal 'ALL'; every other entry
// is treated as a role identifier and emitted via QuoteOrBare(role, false) —
// bare when it is a valid unquoted identifier (so "analyst" resolves to role
// ANALYST, matching Snowflake's uppercasing of unquoted names) and double-quoted
// only when it needs quoting (special characters or a reserved keyword). Blank
// entries are skipped. The result is parenthesized, e.g. ('ALL'), (R1, R2),
// ("my role"), or ().
//
// Limitation: a role whose name is a valid bare identifier is emitted unquoted
// even if it was created case-sensitively in lower/mixed case (e.g. a role named
// "analyst"). Snowflake uppercases the bare form, so such a role round-trips to
// ANALYST — a different role. This is inherent to the QuoteOrBare (and Snowflake)
// bare-identifier convention; case-sensitive role names are rare, but for them the
// parse → format round-trip is not strictly lossless.
func FormatSecondaryRoles(roles []string) string {
	parts := make([]string, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.EqualFold(r, "ALL") {
			parts = append(parts, "'ALL'")
		} else {
			parts = append(parts, QuoteOrBare(r, false))
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// ReconcileAllExclusive enforces the `( { 'ALL' | <item> [, ...] } )` grammar's
// mutual exclusivity — the ALL token cannot be mixed with named items. Given a tag
// selection in selection order (as the UI's tag picker reports it), it keeps
// whichever kind was chosen last: if ALL was just added it collapses to ["ALL"];
// if a named item was added while ALL was already present it drops ALL. Lists
// without ALL, or with a single entry, pass through unchanged. It prevents the
// invalid ('ALL', X) clause that Snowflake would otherwise reject only at
// execution time. This is the general reader behind ReconcileSecondaryRoles and
// the authentication-policy list editors; the ALL match is case-insensitive.
func ReconcileAllExclusive(items []string) []string {
	isAll := func(r string) bool { return strings.EqualFold(strings.TrimSpace(r), "ALL") }
	if len(items) <= 1 || !slices.ContainsFunc(items, isAll) {
		return items
	}
	if isAll(items[len(items)-1]) {
		return []string{"ALL"}
	}
	out := make([]string, 0, len(items))
	for _, r := range items {
		if !isAll(r) {
			out = append(out, r)
		}
	}
	return out
}

// ReconcileSecondaryRoles enforces the secondary-role grammar's `( 'ALL' |
// <role>, … )` mutual exclusivity — a thin alias over ReconcileAllExclusive
// (the ALL-vs-named-item rule is identical) kept for the session-policy callers.
func ReconcileSecondaryRoles(roles []string) []string {
	return ReconcileAllExclusive(roles)
}

// unquoteSQLToken returns the value of a quoted token text whose quote character
// is q — a single quote for a StringLit, a double quote for a QuotedIdent. The
// surrounding quotes are stripped and any doubled quote (the SQL escape) is
// collapsed to one. The tokenizer guarantees a leading quote; the trailing quote
// is absent only for an unterminated token, so it is stripped defensively.
func unquoteSQLToken(s string, q byte) string {
	if len(s) > 0 && s[0] == q {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == q {
		s = s[:len(s)-1]
	}
	return strings.ReplaceAll(s, string([]byte{q, q}), string(q))
}

// ParseSecondaryRoles is the inverse of FormatSecondaryRoles: it parses a
// secondary-role list cell — as returned by DESCRIBE SESSION POLICY — into its
// individual role tokens. Snowflake does not document the cell's exact format,
// so two shapes are accepted so a parse → edit → re-serialize round-trip never
// corrupts the list:
//   - a SQL tuple, e.g. ('ALL') or (R1, "my role"); and
//   - a JSON-style array, e.g. ["ALL"] or ["R1","R2"].
//
// It runs the shared SQL tokenizer over the cell and keeps only the value tokens
// — 'single-quoted' literals and "double-quoted" identifiers (unquoted, with
// doubled quotes collapsed) plus bare words/numbers — discarding the (), [], and
// comma punctuation and any whitespace. Quoting and escape handling therefore
// come straight from the tokenizer, so a quoted role containing a comma or an
// embedded quote survives intact. The "ALL" literal is returned verbatim (as the
// token "ALL"). An empty / null cell yields nil.
//
// Note: the JSON-array shape is handled only insofar as JSON double-quotes scan
// as QuotedIdent tokens; a JSON backslash escape (e.g. \") would be un-escaped the
// SQL way (doubled quotes), not the JSON way. Role names never contain such
// characters in practice, so this does not arise for real DESCRIBE output.
func ParseSecondaryRoles(raw string) []string {
	return ParseSqlList(raw)
}

// ParseSqlList parses a DESCRIBE list-cell value into its individual value tokens.
// Snowflake does not document a single uniform rendering for these cells, so the
// SQL/bracket and JSON-array shapes are all accepted so a parse → edit →
// re-serialize round-trip never corrupts the list:
//   - a SQL tuple, e.g. ('PASSWORD', 'SAML') or ('ALL');
//   - a bracketed list, e.g. [PASSWORD, SAML] or [ALL]; and
//   - a JSON-style array, e.g. ["ALL"] or ["R1","R2"].
//
// It runs the shared SQL tokenizer over the cell and keeps only the value tokens
// — 'single-quoted' literals and "double-quoted" identifiers (unquoted, with
// doubled quotes collapsed) plus bare words/numbers — discarding the (), [], and
// comma punctuation and any whitespace. Quoting and escape handling therefore come
// straight from the tokenizer, so a quoted entry containing a comma or an embedded
// quote survives intact. An empty / null cell yields nil. This is the general
// reader behind ParseSecondaryRoles and the authentication-policy list parameters.
func ParseSqlList(raw string) []string {
	if s := strings.TrimSpace(raw); s == "" || strings.EqualFold(s, "null") {
		return nil
	}
	var out []string
	for _, tok := range sqltok.Tokenize(raw) {
		var v string
		switch tok.Kind {
		case sqltok.StringLit:
			v = unquoteSQLToken(tok.Text(raw), '\'')
		case sqltok.QuotedIdent:
			v = unquoteSQLToken(tok.Text(raw), '"')
		case sqltok.Identifier, sqltok.Keyword, sqltok.NumberLit:
			v = tok.Text(raw)
		default:
			continue // (, ), [, ], comma, whitespace, EOF, …
		}
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ParseSqlListVerbatim parses a DESCRIBE list cell — a SQL tuple ('a', 'b'), a
// bracketed list [a, b] or ["a","b"], or a bare comma-separated list — into its
// elements, like ParseSqlList but PRESERVING each element's internal text.
//
// ParseSqlList runs the tokenizer and keeps only value tokens, discarding
// operators — so a bare, unquoted compound element such as a package version
// specifier "numpy==1.26.4" is split into "numpy" and "1.26.4". ParseSqlListVerbatim
// instead groups the tokens between top-level commas and reconstructs each
// element from the source span: a single quoted token (string literal or quoted
// identifier) is unquoted (doubled quotes collapsed), and any other element is
// returned verbatim, so its operators (==, >=, <=, <, >, ::, …) and internal
// spacing survive. Nesting via ()/[] is tracked so a comma inside a nested group
// does not split an element, and a comma inside a quoted literal is part of that
// literal (the tokenizer keeps it). An empty / null cell yields nil.
//
// Use this for lists whose elements may be compound (package specs, expressions);
// use ParseSqlList for lists of plain identifiers/literals where dropping
// punctuation is desirable.
func ParseSqlListVerbatim(raw string) []string {
	if s := strings.TrimSpace(raw); s == "" || strings.EqualFold(s, "null") {
		return nil
	}
	toks := sqltok.Significant(sqltok.Tokenize(raw))
	if len(toks) == 0 {
		return nil
	}
	// A wrapped list — ( … ) or [ … ] — separates its elements with commas one
	// level deep; a bare list separates them at the top level. Pick the depth at
	// which a comma is an element separator accordingly.
	splitDepth := 0
	if k := toks[0].Kind; k == sqltok.LParen || k == sqltok.LBracket {
		splitDepth = 1
	}

	var out []string
	var elem []sqltok.Token
	flush := func() {
		appendListElem(&out, elem, raw)
		elem = nil
	}

	depth := 0
	for _, tok := range toks {
		switch tok.Kind {
		case sqltok.LParen, sqltok.LBracket:
			depth++
			if depth == splitDepth {
				continue // the wrapping bracket itself — not element content
			}
		case sqltok.RParen, sqltok.RBracket:
			if depth == splitDepth {
				depth--
				continue // the closing wrapper bracket
			}
			depth--
		case sqltok.Comma:
			if depth == splitDepth {
				flush()
				continue
			}
		}
		elem = append(elem, tok)
	}
	flush()
	return out
}

// appendListElem reconstructs one element of a ParseSqlListVerbatim list from its
// significant tokens and appends the (non-empty) result to out: a lone quoted
// token is unquoted; anything else is taken as the verbatim source span from its
// first to its last token, so internal operators survive.
func appendListElem(out *[]string, elem []sqltok.Token, raw string) {
	if len(elem) == 0 {
		return
	}
	if len(elem) == 1 {
		switch elem[0].Kind {
		case sqltok.StringLit:
			if v := strings.TrimSpace(unquoteSQLToken(elem[0].Text(raw), '\'')); v != "" {
				*out = append(*out, v)
			}
			return
		case sqltok.QuotedIdent:
			if v := strings.TrimSpace(unquoteSQLToken(elem[0].Text(raw), '"')); v != "" {
				*out = append(*out, v)
			}
			return
		}
	}
	if v := strings.TrimSpace(raw[elem[0].Start:elem[len(elem)-1].End]); v != "" {
		*out = append(*out, v)
	}
}

// NormalizeScalar reduces a DESCRIBE scalar cell to its bare value, stripping any
// surrounding brackets/quotes Snowflake may wrap it in — "[OPTIONAL]", "'OPTIONAL'"
// and "OPTIONAL" all yield "OPTIONAL". It reuses the ParseSqlList tokenizer and
// returns the first value token (scalar cells carry exactly one), or "" when the
// cell is empty / null.
func NormalizeScalar(raw string) string {
	toks := ParseSqlList(raw)
	if len(toks) == 0 {
		return ""
	}
	return toks[0]
}

// GetQuotedIdentifiersIgnoreCase returns the current session value of the
// QUOTED_IDENTIFIERS_IGNORE_CASE parameter. When true, Snowflake treats
// identifiers as case-insensitive regardless of whether they are quoted,
// which affects how double-quoted names are stored and resolved.
func (c *Client) GetQuotedIdentifiersIgnoreCase(ctx context.Context) (bool, error) {
	// SHOW PARAMETERS columns (0-indexed):
	//   0: key, 1: value, 2: default, 3: level, 4: description, 5: type
	vals, err := c.queryStringSlice(ctx,
		"SHOW PARAMETERS LIKE 'QUOTED_IDENTIFIERS_IGNORE_CASE' IN SESSION", 1)
	if err != nil {
		return false, err
	}
	if len(vals) == 0 {
		// Parameter not present in result — treat as the Snowflake default (false).
		return false, nil
	}
	return strings.EqualFold(vals[0], "true"), nil
}
