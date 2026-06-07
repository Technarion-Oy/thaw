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
	"sort"
	"strings"
)

// reUnquotedIdent matches a Snowflake bare (unquoted) identifier: starts with
// a letter or underscore, followed by letters, digits, underscores, or dollar
// signs. The pattern is case-insensitive because Snowflake normalises unquoted
// identifiers to uppercase regardless of how they were written.
var reUnquotedIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// snowflakeReservedKeywords is the set of Snowflake SQL reserved words.
// An unquoted identifier that matches one of these (case-insensitively) is
// interpreted as a keyword and must be double-quoted to be used as a name.
// Source: https://docs.snowflake.com/en/sql-reference/reserved-keywords
var snowflakeReservedKeywords = map[string]struct{}{
	"ACCOUNT": {}, "ALL": {}, "ALTER": {}, "AND": {}, "ANY": {}, "AS": {},
	"BETWEEN": {}, "BY": {},
	"CASE": {}, "CAST": {}, "CHECK": {}, "COLUMN": {}, "CONNECT": {},
	"CONNECTION": {}, "CONSTRAINT": {}, "CREATE": {}, "CROSS": {},
	"CURRENT": {}, "CURRENT_DATE": {}, "CURRENT_TIME": {},
	"CURRENT_TIMESTAMP": {}, "CURRENT_USER": {},
	"DATABASE": {}, "DELETE": {}, "DISTINCT": {}, "DROP": {},
	"ELSE": {}, "EXISTS": {},
	"FALSE": {}, "FOLLOWING": {}, "FOR": {}, "FROM": {}, "FULL": {},
	"GRANT": {}, "GROUP": {}, "GSCLUSTER": {},
	"HAVING": {},
	"ILIKE": {}, "IN": {}, "INCREMENT": {}, "INNER": {}, "INSERT": {},
	"INTERSECT": {}, "INTO": {}, "IS": {}, "ISSUE": {},
	"JOIN": {},
	"LATERAL": {}, "LEFT": {}, "LIKE": {}, "LIMIT": {},
	"LOCALTIME": {}, "LOCALTIMESTAMP": {},
	"MINUS": {},
	"NATURAL": {}, "NOT": {}, "NULL": {},
	"OF": {}, "ON": {}, "OR": {}, "ORDER": {}, "ORGANIZATION": {},
	"QUALIFY": {},
	"REGEXP": {}, "REVOKE": {}, "RIGHT": {}, "RLIKE": {}, "ROW": {}, "ROWS": {},
	"SAMPLE": {}, "SELECT": {}, "SET": {}, "SOME": {}, "START": {},
	"TABLE": {}, "TABLESAMPLE": {}, "THEN": {}, "TO": {}, "TRIGGER": {},
	"TRUE": {}, "TRY_CAST": {},
	"UNION": {}, "UNIQUE": {}, "UPDATE": {}, "USING": {},
	"VALUES": {}, "VIEW": {},
	"WHEN": {}, "WHENEVER": {}, "WHERE": {}, "WITH": {},
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
	_, reserved := snowflakeReservedKeywords[strings.ToUpper(name)]
	return reserved
}

// ReservedKeywords returns the full list of Snowflake reserved keywords.
// The returned slice is sorted alphabetically. Callers must not modify it.
func ReservedKeywords() []string {
	out := make([]string, 0, len(snowflakeReservedKeywords))
	for kw := range snowflakeReservedKeywords {
		out = append(out, kw)
	}
	// Sort for deterministic ordering (map iteration is random in Go).
	sort.Strings(out)
	return out
}

// QuoteIdent wraps name in double-quotes, escaping any embedded double-quotes.
func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// EscapeStringLit escapes single-quotes within a SQL string literal value by
// doubling them. It does not add surrounding single-quote delimiters.
func EscapeStringLit(s string) string {
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
