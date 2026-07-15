// SPDX-License-Identifier: GPL-3.0-or-later

package pipe

import (
	"fmt"
	"strings"
	"unicode"
)

// FQNPart is one component of a dot-separated Snowflake fully qualified name,
// together with whether it was double-quoted in the source DDL.
//
// Snowflake resolves unquoted identifiers case-insensitively and stores them
// in uppercase. GET_DDL may return unquoted identifiers in any case.
// Callers that need canonical resolution should uppercase unquoted parts.
type FQNPart struct {
	Value  string
	Quoted bool // true if originally surrounded by double-quotes in the DDL
}

// ParseCopyIntoTargetParts parses the COPY INTO target table from a Snowflake
// pipe DDL and returns up to three identifier parts (db, schema, table) with
// their quoting status.
//
// The search for COPY INTO is case-insensitive. For quoted identifiers the
// inner "" escape sequences are resolved to a single ". Unquoted identifiers
// are returned as-is (GET_DDL may return them in any case).
func ParseCopyIntoTargetParts(ddl string) ([]FQNPart, error) {
	// Locate COPY INTO (case-insensitive).
	upper := strings.ToUpper(ddl)
	idx := strings.Index(upper, "COPY INTO")
	if idx < 0 {
		return nil, fmt.Errorf("COPY INTO not found in pipe DDL")
	}
	rest := ddl[idx+len("COPY INTO"):]

	// Skip leading whitespace.
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	if rest == "" {
		return nil, fmt.Errorf("no table name found after COPY INTO")
	}

	// Parse up to 3 dot-separated identifier parts.
	var parts []FQNPart
	for {
		val, remaining, quoted, err := parseIdentPartWithQuoting(rest)
		if err != nil {
			return nil, fmt.Errorf("parsing COPY INTO target: %w", err)
		}
		parts = append(parts, FQNPart{Value: val, Quoted: quoted})
		rest = remaining

		// Stop unless the very next char is a dot.
		trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
		if len(trimmed) == 0 || trimmed[0] != '.' {
			break
		}
		rest = trimmed[1:] // consume the dot
		if len(parts) == 3 {
			break // safety guard
		}
	}

	if len(parts) < 1 || len(parts) > 3 {
		return nil, fmt.Errorf("unexpected identifier part count %d", len(parts))
	}
	return parts, nil
}

// ParseCopyIntoTarget is a convenience wrapper around ParseCopyIntoTargetParts
// that returns the identifier values without quoting metadata.
//
// db and schema are empty strings for 1-part and 2-part names respectively.
// Unquoted identifiers are returned as-is; callers should not uppercase them
// again because Snowflake may return uppercase unquoted identifiers from
// GET_DDL. Use ParseCopyIntoTargetParts when quoting metadata is needed.
func ParseCopyIntoTarget(ddl string) (db, schema, table string, err error) {
	parts, err := ParseCopyIntoTargetParts(ddl)
	if err != nil {
		return "", "", "", err
	}
	switch len(parts) {
	case 1:
		return "", "", parts[0].Value, nil
	case 2:
		return "", parts[0].Value, parts[1].Value, nil
	default: // 3
		return parts[0].Value, parts[1].Value, parts[2].Value, nil
	}
}

// parseIdentPartWithQuoting consumes one identifier (quoted or unquoted) from s
// and returns the value, the remaining string, whether it was quoted, and any error.
func parseIdentPartWithQuoting(s string) (ident, rest string, quoted bool, err error) {
	if s == "" {
		return "", "", false, fmt.Errorf("empty input while parsing identifier")
	}
	if s[0] == '"' {
		ident, rest, err = parseQuotedIdent(s)
		return ident, rest, true, err
	}
	ident, rest, err = parseUnquotedIdent(s)
	return ident, rest, false, err
}

// parseQuotedIdent consumes a double-quoted identifier, treating "" as an
// escaped double-quote inside the value.
func parseQuotedIdent(s string) (ident, rest string, err error) {
	if len(s) < 2 || s[0] != '"' {
		return "", "", fmt.Errorf("expected quoted identifier, got: %.20q", s)
	}
	var sb strings.Builder
	i := 1 // skip opening "
	for i < len(s) {
		if s[i] == '"' {
			if i+1 < len(s) && s[i+1] == '"' {
				// Escaped double-quote inside a quoted identifier.
				sb.WriteByte('"')
				i += 2
			} else {
				// Closing quote.
				i++
				break
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String(), s[i:], nil
}

// parseUnquotedIdent consumes an unquoted identifier, stopping at '.', '(',
// ';', or any whitespace character.
func parseUnquotedIdent(s string) (ident, rest string, err error) {
	i := 0
	for i < len(s) {
		c := rune(s[i])
		if c == '.' || c == '(' || c == ';' || unicode.IsSpace(c) {
			break
		}
		i++
	}
	if i == 0 {
		return "", "", fmt.Errorf("empty unquoted identifier near: %.20q", s)
	}
	return s[:i], s[i:], nil
}
