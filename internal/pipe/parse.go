// SPDX-License-Identifier: GPL-3.0-or-later

package pipe

import (
	"fmt"
	"strings"

	"thaw/internal/sqltok"
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
// The DDL is tokenized with [sqltok], so a COPY INTO occurring inside a comment
// or a string literal — e.g. the COMMENT clause that precedes AS COPY INTO in
// CREATE PIPE DDL — is not mistaken for the real target, and any amount of
// whitespace (including newlines) may separate COPY from INTO.
//
// For quoted identifiers the inner "" escape sequences are resolved to a
// single ". Unquoted identifiers are returned as-is (GET_DDL may return them
// in any case).
func ParseCopyIntoTargetParts(ddl string) ([]FQNPart, error) {
	tokens := sqltok.SignificantTokens(ddl)

	for i := 0; i+1 < len(tokens); i++ {
		if !isWord(tokens[i], ddl, "COPY") || !isWord(tokens[i+1], ddl, "INTO") {
			continue
		}
		raw, _ := sqltok.ReadIdentParts(tokens, ddl, i+2, 3)
		if len(raw) == 0 {
			return nil, fmt.Errorf("no table name found after COPY INTO")
		}
		parts := make([]FQNPart, 0, len(raw))
		for _, p := range raw {
			parts = append(parts, FQNPart{
				Value:  sqltok.Unquote(p),
				Quoted: strings.HasPrefix(p, `"`),
			})
		}
		return parts, nil
	}

	return nil, fmt.Errorf("COPY INTO not found in pipe DDL")
}

// isWord reports whether tok is an identifier-like token whose text equals
// word, case-insensitively. String literals, comments, and dollar-quoted
// bodies are single tokens of a different kind, so they never match.
func isWord(tok sqltok.Token, src, word string) bool {
	return tok.Kind.IsIdentLike() && strings.EqualFold(tok.Text(src), word)
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
