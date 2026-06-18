// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"context"
	"regexp"
	"strings"

	"thaw/internal/sqltok"
)

// reUnquotedIdent matches a Snowflake bare (unquoted) identifier: starts with
// a letter or underscore, followed by letters, digits, underscores, or dollar
// signs. The pattern is case-insensitive because Snowflake normalizes unquoted
// identifiers to uppercase regardless of how they were written.
var reUnquotedIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

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
	out := make([]string, 0, len(names))
	for _, n := range names {
		if v := strings.TrimSpace(n); v != "" {
			out = append(out, QuoteOrBare(v, caseSensitive))
		}
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

// splitOnUnquotedCommas splits a comma-separated list while ignoring commas that
// fall inside a single- or double-quoted segment, so a quoted identifier such as
// "a,b" is kept whole. A doubled quote ("" or ”) inside a quoted segment is the
// SQL escape for a literal quote and does not end the segment. (This differs from
// the paren-depth-based splitTopLevelCommas in datatypes.go, which is quote-blind.)
func splitOnUnquotedCommas(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune // 0 when not inside a quoted segment
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case quote != 0:
			if ch == quote {
				if i+1 < len(runes) && runes[i+1] == quote { // doubled → escaped quote
					cur.WriteRune(ch)
					cur.WriteRune(ch)
					i++
					continue
				}
				quote = 0
			}
			cur.WriteRune(ch)
		case ch == '"' || ch == '\'':
			quote = ch
			cur.WriteRune(ch)
		case ch == ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(ch)
		}
	}
	out = append(out, cur.String())
	return out
}

// ParseSecondaryRoles is the inverse of FormatSecondaryRoles: it parses a
// secondary-role list cell — as returned by DESCRIBE SESSION POLICY — into its
// individual role tokens. Snowflake does not document the cell's exact format,
// so two shapes are accepted so a parse → edit → re-serialize round-trip never
// corrupts the list:
//   - a SQL tuple, e.g. ('ALL') or (R1, "my role"); and
//   - a JSON-style array, e.g. ["ALL"] or ["R1","R2"].
//
// The outer (...) / [...] wrapper is stripped, the body is split on top-level
// commas (commas inside quotes are preserved), and each entry's surrounding
// single/double quotes are removed with doubled quotes un-escaped. The "ALL"
// literal is returned verbatim (as the token "ALL"). An empty / null cell yields
// nil.
func ParseSecondaryRoles(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "null") {
		return nil
	}
	if (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		s = s[1 : len(s)-1]
	}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range splitOnUnquotedCommas(s) {
		p := strings.TrimSpace(part)
		switch {
		case len(p) >= 2 && strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'"):
			p = strings.ReplaceAll(p[1:len(p)-1], "''", "'")
		case len(p) >= 2 && strings.HasPrefix(p, `"`) && strings.HasSuffix(p, `"`):
			p = strings.ReplaceAll(p[1:len(p)-1], `""`, `"`)
		}
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
