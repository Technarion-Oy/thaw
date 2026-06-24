// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"strings"

	"thaw/internal/sqltok"
)

// ValidateAntiPatterns runs the *semantic* Snowflake anti-pattern checks that
// the grammar engine (internal/sqlgrammar) cannot perform — it consumes clause
// bodies permissively, so it cannot reason about MERGE clause-action validity,
// QUALIFY placement, FLATTEN/LATERAL usage, variant-path traversal, or Cortex
// function names. These were previously embedded in the (now-removed)
// ValidateSnowflakePatterns; they are re-homed here as a small focused validator
// so the structural grammar check and these orthogonal semantic checks stay
// cleanly separated. All markers are Warnings (Severity 4).
func ValidateAntiPatterns(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker
	for _, r := range stmtRanges {
		rawText := sqlStmt(sql, r)
		stripped := strings.TrimSpace(stripCommentsSQL(rawText))
		if stripped == "" {
			continue
		}
		firstTok := getFirstSQLToken(rawText)

		markers = append(markers, checkLateralFlattenTypo(rawText, r)...)
		markers = append(markers, checkFlattenWithoutLateral(rawText, stripped, r)...)
		markers = append(markers, checkVariantPathColon(rawText, r)...)
		markers = append(markers, checkQualifyPlacement(rawText, stripped, r)...)
		if firstTok == "MERGE" {
			markers = append(markers, checkMergeClauses(rawText, r)...)
		}
		if firstTok != "GRANT" && firstTok != "REVOKE" {
			markers = append(markers, checkUnknownCortexFunc(rawText, r)...)
		}
	}
	return markers
}

// knownCortexFunctions lists the recognized SNOWFLAKE.CORTEX.<name> functions.
var knownCortexFunctions = map[string]bool{
	"COMPLETE": true, "EXTRACT_ANSWER": true, "SENTIMENT": true, "SUMMARIZE": true,
	"TRANSLATE": true, "CLASSIFY_TEXT": true, "EMBED_TEXT_768": true, "EMBED_TEXT_1024": true,
	"FINETUNE": true, "SEARCH_PREVIEW": true, "TRY_COMPLETE": true,
}

// apHasKWPair reports whether kw1 is immediately followed by kw2 in the
// significant-token stream (both compared upper-case).
func apHasKWPair(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			return true
		}
	}
	return false
}

// checkLateralFlattenTypo flags `LATERALFLATTEN` (missing space).
func checkLateralFlattenTypo(rawText string, r StatementRange) []DiagMarker {
	var out []DiagMarker
	for _, tok := range sqltok.Tokenize(rawText) {
		if tok.Kind == sqltok.EOF {
			break
		}
		if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
			strings.EqualFold(tok.Text(rawText), "LATERALFLATTEN") {
			line := r.StartLine + tok.Line - 1
			out = append(out, DiagMarker{
				StartLineNumber: line, StartColumn: tok.Col,
				EndLineNumber: line, EndColumn: tok.Col + (tok.End - tok.Start),
				Message:  "Typo detected: Did you mean 'LATERAL FLATTEN'?",
				Severity: SeverityWarning,
			})
		}
	}
	return out
}

// checkFlattenWithoutLateral flags FLATTEN used as a table function (after
// FROM/JOIN/comma) without LATERAL or TABLE(...).
func checkFlattenWithoutLateral(rawText, stripped string, r StatementRange) []DiagMarker {
	sig := sigTokens(stripped)
	hasFlattenFromJoin, hasLateralFlatten, hasTableFlatten := false, false, false
	for i, tok := range sig {
		if tokUpper(tok, stripped) != "FLATTEN" {
			continue
		}
		if i > 0 && tokUpper(sig[i-1], stripped) == "LATERAL" {
			hasLateralFlatten = true
		}
		if i > 0 {
			prevU := tokUpper(sig[i-1], stripped)
			if prevU == "FROM" || prevU == "JOIN" || sig[i-1].Kind == sqltok.Comma {
				hasFlattenFromJoin = true
			}
		}
		if i >= 2 && tokUpper(sig[i-2], stripped) == "TABLE" && sig[i-1].Kind == sqltok.LParen {
			hasTableFlatten = true
		}
	}
	if !hasFlattenFromJoin || hasLateralFlatten || hasTableFlatten {
		return nil
	}
	var out []DiagMarker
	for _, tok := range sqltok.Tokenize(rawText) {
		if tok.Kind == sqltok.EOF {
			break
		}
		if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
			strings.EqualFold(tok.Text(rawText), "FLATTEN") {
			line := r.StartLine + tok.Line - 1
			out = append(out, DiagMarker{
				StartLineNumber: line, StartColumn: tok.Col,
				EndLineNumber: line, EndColumn: tok.Col + (tok.End - tok.Start),
				Message:  "FLATTEN used as a table function requires LATERAL. Use LATERAL FLATTEN(...) or TABLE(FLATTEN(...)).",
				Severity: SeverityWarning,
			})
		}
	}
	return out
}

// checkVariantPathColon flags a dotted `payload.field.sub` traversal that should
// use the `:` variant-path operator. Deliberately narrow (a `payload` root) to
// avoid false positives on ordinary qualified column references.
func checkVariantPathColon(rawText string, r StatementRange) []DiagMarker {
	var out []DiagMarker
	sig := sigTokens(rawText)
	for i := 0; i+4 < len(sig); i++ {
		if sig[i].Kind == sqltok.Identifier && sig[i+1].Kind == sqltok.Dot &&
			sig[i+2].Kind == sqltok.Identifier && sig[i+3].Kind == sqltok.Dot &&
			sig[i+4].Kind == sqltok.Identifier &&
			strings.ToLower(sig[i].Text(rawText)) == "payload" {
			start, end := sig[i], sig[i+4]
			match := rawText[start.Start:end.End]
			parts := strings.SplitN(match, ".", 3)
			suggestion := parts[0] + ":" + strings.Join(parts[1:], ".")
			line := r.StartLine + start.Line - 1
			out = append(out, DiagMarker{
				StartLineNumber: line, StartColumn: start.Col,
				EndLineNumber: line, EndColumn: start.Col + len(match),
				Message:  "Missing colon for variant path. Use ':' for Snowflake JSON traversal (e.g. " + suggestion + ").",
				Severity: SeverityWarning,
			})
		}
	}
	return out
}

// checkQualifyPlacement flags a QUALIFY clause that appears after ORDER BY.
func checkQualifyPlacement(rawText, stripped string, r StatementRange) []DiagMarker {
	sig := sigTokens(stripped)
	orderByIdx := -1
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "ORDER" && tokUpper(sig[i+1], stripped) == "BY" {
			orderByIdx = i
			break
		}
	}
	if orderByIdx < 0 {
		return nil
	}
	hasQualifyAfter := false
	for i := orderByIdx + 2; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "QUALIFY" {
			hasQualifyAfter = true
			break
		}
	}
	if !hasQualifyAfter {
		return nil
	}
	var out []DiagMarker
	for _, tok := range sqltok.Tokenize(rawText) {
		if tok.Kind == sqltok.EOF {
			break
		}
		if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
			strings.EqualFold(tok.Text(rawText), "QUALIFY") {
			line := r.StartLine + tok.Line - 1
			out = append(out, DiagMarker{
				StartLineNumber: line, StartColumn: tok.Col,
				EndLineNumber: line, EndColumn: tok.Col + (tok.End - tok.Start),
				Message:  "Snowflake 'QUALIFY' must come after 'WHERE' or 'HAVING' but before 'ORDER BY'.",
				Severity: SeverityWarning,
			})
		}
	}
	return out
}

// checkMergeClauses flags invalid actions in a MERGE statement's WHEN clauses:
// INSERT in WHEN MATCHED, UPDATE/DELETE in WHEN NOT MATCHED, and the unsupported
// WHEN NOT MATCHED BY SOURCE.
func checkMergeClauses(rawText string, r StatementRange) []DiagMarker {
	var out []DiagMarker
	sig := sigToks(sqltok.Tokenize(rawText))
	// Top-level (depth 0) WHEN boundaries.
	var whenStarts []int
	depth := 0
	for _, t := range sig {
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(t, rawText) == "WHEN" {
				whenStarts = append(whenStarts, t.Start)
			}
		}
	}
	for i, start := range whenStarts {
		end := len(rawText)
		if i+1 < len(whenStarts) {
			end = whenStarts[i+1]
		}
		clause := rawText[start:end]
		clauseSig := sigTokens(stripCommentsSQL(clause))
		lines := strings.Split(rawText[:start], "\n")
		line := r.StartLine + len(lines) - 1
		col := len(lines[len(lines)-1]) + 1

		isMatched := len(clauseSig) >= 2 && kwAt(clauseSig, clause, 0, "WHEN") && kwAt(clauseSig, clause, 1, "MATCHED")
		isNotMatched := len(clauseSig) >= 3 && kwAt(clauseSig, clause, 0, "WHEN") && kwAt(clauseSig, clause, 1, "NOT") && kwAt(clauseSig, clause, 2, "MATCHED")
		hasBySource := apHasKWPair(clauseSig, clause, "BY", "SOURCE")

		switch {
		case isMatched && !isNotMatched:
			if apHasKWPair(clauseSig, clause, "THEN", "INSERT") {
				out = append(out, DiagMarker{
					StartLineNumber: line, StartColumn: col, EndLineNumber: line, EndColumn: col + 12,
					Message:  "INSERT action is not allowed in WHEN MATCHED clause. Use UPDATE or DELETE.",
					Severity: SeverityWarning,
				})
			}
		case isNotMatched && !hasBySource:
			if apHasKWPair(clauseSig, clause, "THEN", "UPDATE") || apHasKWPair(clauseSig, clause, "THEN", "DELETE") {
				out = append(out, DiagMarker{
					StartLineNumber: line, StartColumn: col, EndLineNumber: line, EndColumn: col + 16,
					Message:  "UPDATE or DELETE action is not allowed in WHEN NOT MATCHED clause. Use INSERT.",
					Severity: SeverityWarning,
				})
			}
		case isNotMatched && hasBySource:
			out = append(out, DiagMarker{
				StartLineNumber: line, StartColumn: col, EndLineNumber: line, EndColumn: col + 26,
				Message:  "WHEN NOT MATCHED BY SOURCE is not supported by Snowflake. Use a subquery with a LEFT JOIN as your source to identify missing rows.",
				Severity: SeverityWarning,
			})
		}
	}
	return out
}

// checkUnknownCortexFunc flags a SNOWFLAKE.CORTEX.<name>( call where <name> is
// not a recognized Cortex function.
func checkUnknownCortexFunc(rawText string, r StatementRange) []DiagMarker {
	var out []DiagMarker
	sig := sigTokens(rawText)
	for i := 0; i+5 < len(sig); i++ {
		if tokUpper(sig[i], rawText) == "SNOWFLAKE" && sig[i+1].Kind == sqltok.Dot &&
			tokUpper(sig[i+2], rawText) == "CORTEX" && sig[i+3].Kind == sqltok.Dot &&
			isIdent(sig[i+4]) && sig[i+5].Kind == sqltok.LParen {
			name := strings.ToUpper(sig[i+4].Text(rawText))
			if !knownCortexFunctions[name] {
				start, end := sig[i], sig[i+4]
				match := rawText[start.Start:end.End]
				lines := strings.Split(rawText[:start.Start], "\n")
				line := r.StartLine + len(lines) - 1
				col := len(lines[len(lines)-1]) + 1
				out = append(out, DiagMarker{
					StartLineNumber: line, StartColumn: col,
					EndLineNumber: line, EndColumn: col + len(match),
					Message: "Unknown Cortex function '" + sig[i+4].Text(rawText) + "'. Known functions: COMPLETE, EXTRACT_ANSWER, " +
						"SENTIMENT, SUMMARIZE, TRANSLATE, CLASSIFY_TEXT, EMBED_TEXT_768, EMBED_TEXT_1024, FINETUNE, SEARCH_PREVIEW, TRY_COMPLETE.",
					Severity: SeverityWarning,
				})
			}
			i += 5
		}
	}
	return out
}
