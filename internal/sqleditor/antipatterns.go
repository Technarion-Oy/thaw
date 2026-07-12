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

	"thaw/internal/sqlgrammar"
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

	// Block-level transaction tracking spans the whole statement list, so it
	// lives outside the per-statement loop. It only tracks BEGIN/COMMIT/ROLLBACK
	// nesting depth — the per-statement syntax of those commands is validated by
	// the grammar engine.
	txnDepth := 0
	var txnBeginRange StatementRange // range of the opening BEGIN (for end-of-script warning)

	for _, r := range stmtRanges {
		rawText := sqlStmt(sql, r)
		stripped := strings.TrimSpace(stripCommentsSQL(rawText))
		if stripped == "" {
			continue
		}
		firstTok := getFirstSQLToken(rawText)
		// stmtKind is the statement's effective top-level verb, derived by the
		// grammar engine (which looks past a leading WITH/CTE prefix) rather than
		// from fragile substring matching. firstTok stays the *literal* leader,
		// used only for the lexical guards below (GRANT/REVOKE, BEGIN/COMMIT/…).
		stmtKind := sqlgrammar.New(rawText).IdentifyStatement()
		// present is the set of significant keyword/identifier tokens in the
		// statement. The clause-level validators gate on it instead of
		// strings.Contains, which mis-fires on substrings — "AT" inside CREATE/DATE,
		// "PIVOT" inside UNPIVOT — running validators on statements that have no
		// such clause at all.
		present := sigWordSet(stripped)

		markers = append(markers, checkLateralFlattenTypo(rawText, r)...)
		markers = append(markers, checkFlattenWithoutLateral(rawText, stripped, r)...)
		markers = append(markers, checkVariantPathColon(rawText, r)...)
		markers = append(markers, checkQualifyPlacement(rawText, stripped, r)...)
		if stmtKind == sqlgrammar.StmtMerge {
			markers = append(markers, checkMergeClauses(rawText, r)...)
		}
		if firstTok != "GRANT" && firstTok != "REVOKE" {
			markers = append(markers, checkUnknownCortexFunc(rawText, r)...)
		}
		// Stray token / dangling AS after a FROM/JOIN table reference. The grammar
		// consumes the FROM body permissively, so it does not catch these typos.
		if present["FROM"] || present["JOIN"] {
			markers = append(markers, checkStrayAfterTableRef(sigTokens(stripped), stripped, r)...)
		}

		// ── Clause-level semantic anti-patterns ───────────────────────────
		if present["PIVOT"] {
			markers = append(markers, validatePivotClauses(stripped, r)...)
		}
		if present["UNPIVOT"] {
			markers = append(markers, validateUnpivotClauses(stripped, r)...)
		}
		if present["MATCH_RECOGNIZE"] {
			markers = append(markers, validateMatchRecognizeClauses(stripped, r)...)
		}
		if present["ASOF"] {
			markers = append(markers, validateAsofJoinClauses(stripped, r)...)
		}
		if present["AT"] || present["BEFORE"] {
			markers = append(markers, validateTimeTravelClauses(stripped, r)...)
		}

		// ── INSERT ALL / FIRST / OVERWRITE ────────────────────────────────
		// Gated on the literal leading INSERT: the multi-table validators index
		// from sig[0]=INSERT, so the (non-standard) WITH-prefixed form is out of
		// scope. stmtKind confirms the classification; firstTok pins sig[0].
		if stmtKind == sqlgrammar.StmtInsert && firstTok == "INSERT" {
			sig := sigTokens(stripped)
			secondTok := ""
			if len(sig) >= 2 {
				secondTok = tokUpper(sig[1], stripped)
			}
			switch secondTok {
			case "ALL":
				markers = append(markers, validateInsertAll(stripped, r)...)
			case "FIRST":
				markers = append(markers, validateInsertFirst(stripped, r)...)
			case "OVERWRITE":
				markers = append(markers, validateInsertOverwrite(stripped, r)...)
			}
		}

		// ── Block-level transaction tracking ──────────────────────────────
		switch firstTok {
		case "BEGIN":
			// A transaction BEGIN is bare `BEGIN` or `BEGIN {WORK|TRANSACTION|NAME …}`.
			// Anything else is an anonymous scripting block whose body opens with a
			// statement (LET/IF/FOR/…, or DML/DDL such as INSERT/SELECT). SplitRanges
			// glues the block's first inner statement onto the BEGIN (e.g.
			// `BEGIN INSERT …`), so a scripting-verb blacklist missed DML/DDL-opening
			// blocks and mis-counted them as open transactions. Whitelist the
			// transaction forms instead.
			sig := sigTokens(stripped)
			if len(sig) >= 2 {
				u := tokUpper(sig[1], stripped)
				// u == "" is a non-word token (e.g. the trailing `;` of a bare
				// `BEGIN;` transaction) — those stay transactions. Only a real
				// statement verb after BEGIN marks a scripting block.
				if u != "" && u != "WORK" && u != "TRANSACTION" && u != "NAME" {
					continue // scripting block, not a transaction
				}
			}
			if txnDepth > 0 {
				markers = append(markers, diagMarkerSpan(r,
					"Snowflake does not support nested BEGIN. A transaction is already open."))
				// Don't increment txnDepth — Snowflake rejects nested BEGIN,
				// so we keep tracking only the original transaction.
			} else {
				txnBeginRange = r
				txnDepth++
			}
		case "COMMIT":
			if txnDepth == 0 {
				markers = append(markers, diagMarkerSpan(r,
					"COMMIT with no open transaction. Add BEGIN before COMMIT."))
			} else {
				txnDepth--
			}
		case "ROLLBACK":
			// ROLLBACK TO SAVEPOINT does NOT end the transaction — only bare
			// ROLLBACK / ROLLBACK WORK closes it.
			rbSig := sigTokens(stripped)
			isToSavepoint := false
			for j := 0; j+1 < len(rbSig); j++ {
				if tokUpper(rbSig[j], stripped) == "TO" && tokUpper(rbSig[j+1], stripped) == "SAVEPOINT" {
					isToSavepoint = true
					break
				}
			}
			if !isToSavepoint {
				if txnDepth == 0 {
					markers = append(markers, diagMarkerSpan(r,
						"ROLLBACK with no open transaction. Add BEGIN before ROLLBACK."))
				} else {
					txnDepth--
				}
			}
		}
	}

	if txnDepth > 0 {
		markers = append(markers, diagMarkerSpan(txnBeginRange,
			"Transaction not committed or rolled back. Add COMMIT or ROLLBACK before the end of the script."))
	}

	return markers
}

// knownCortexFunctions lists the recognized SNOWFLAKE.CORTEX.<name> functions.
var knownCortexFunctions = map[string]bool{
	"COMPLETE": true, "EXTRACT_ANSWER": true, "SENTIMENT": true, "SUMMARIZE": true,
	"TRANSLATE": true, "CLASSIFY_TEXT": true, "EMBED_TEXT_768": true, "EMBED_TEXT_1024": true,
	"FINETUNE": true, "SEARCH_PREVIEW": true, "TRY_COMPLETE": true,
	"PARSE_DOCUMENT": true, "COUNT_TOKENS": true, "ENTITY_SENTIMENT": true,
	"SPLIT_TEXT_RECURSIVE_CHARACTER": true, "SPLIT_TEXT_MARKDOWN_HEADER": true,
	"AGENT_RUN": true, "DATA_AGENT_RUN": true,
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
		// Only a FLATTEN *call* (FLATTEN followed by `(`) is a table function; a bare
		// `flatten` after a comma/FROM is just a column/table named flatten.
		if i > 0 && i+1 < len(sig) && sig[i+1].Kind == sqltok.LParen {
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
		// A `payload.x.y` right after FROM/JOIN is a qualified object name
		// (db.schema.table), not a variant-path traversal — don't flag it.
		if i > 0 {
			if prev := tokUpper(sig[i-1], rawText); prev == "FROM" || prev == "JOIN" {
				continue
			}
		}
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
	// Only a *top-level* ORDER BY out-orders QUALIFY. An ORDER BY nested inside a
	// window's OVER (…) or a subquery is unrelated to statement clause order, so
	// track paren depth and consider depth-0 matches only — otherwise the canonical
	// `… ROW_NUMBER() OVER (ORDER BY b) … QUALIFY rn = 1` false-positives.
	orderByIdx := -1
	depth := 0
	for i := 0; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
			continue
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && i+1 < len(sig) &&
			tokUpper(sig[i], stripped) == "ORDER" && tokUpper(sig[i+1], stripped) == "BY" {
			orderByIdx = i
			break
		}
	}
	if orderByIdx < 0 {
		return nil
	}
	hasQualifyAfter := false
	depth = 0
	for i := orderByIdx + 2; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
			continue
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && tokUpper(sig[i], stripped) == "QUALIFY" {
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
		hasBySource := hasKWPair(clauseSig, clause, "BY", "SOURCE")

		switch {
		case isMatched && !isNotMatched:
			if hasKWPair(clauseSig, clause, "THEN", "INSERT") {
				out = append(out, DiagMarker{
					StartLineNumber: line, StartColumn: col, EndLineNumber: line, EndColumn: col + 12,
					Message:  "INSERT action is not allowed in WHEN MATCHED clause. Use UPDATE or DELETE.",
					Severity: SeverityWarning,
				})
			}
		case isNotMatched && !hasBySource:
			if hasKWPair(clauseSig, clause, "THEN", "UPDATE") || hasKWPair(clauseSig, clause, "THEN", "DELETE") {
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

// checkStrayAfterTableRef flags a stray token or dangling AS immediately after a
// FROM/JOIN table reference — typo patterns the grammar does not catch because
// ParseSelect consumes the FROM body permissively:
//   - `FROM t 1000`        → a bare literal where a comma/JOIN/WHERE was expected
//   - `FROM t AS`          → AS with no alias following it
//   - `FROM t myalias AS`  → a second AS after the alias is already given
func checkStrayAfterTableRef(sig []sqltok.Token, sql string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	for i := 0; i+1 < len(sig); i++ {
		if u := tokUpper(sig[i], sql); u != "FROM" && u != "JOIN" {
			continue
		}
		j := i + 1
		if j >= len(sig) || !isIdent(sig[j]) {
			continue // subquery "(", @stage, or nothing — not a bare table path
		}
		_, j = readIdentPath(sig, sql, j)

		// Optional [AS] <alias>. An implicit alias must be an Identifier/QuotedIdent
		// (not a keyword), mirroring findFromJoinWithAlias.
		hadAS, aliasConsumed := false, false
		if j < len(sig) && tokUpper(sig[j], sql) == "AS" {
			hadAS = true
			j++
			// After an explicit AS the next token is unambiguously the alias, so a
			// non-reserved keyword (KEY, FIRST, TYPE, …) is legal here even though it
			// tokenizes as Keyword. isAliasTok stays strict for the implicit-alias
			// branch below (it must not swallow PIVOT/AT/ASOF clause keywords).
			if j < len(sig) && isAliasWord(sig[j], sql) {
				j++
				aliasConsumed = true
			}
		} else if j < len(sig) && isAliasTok(sig[j]) {
			j++
			aliasConsumed = true
		}

		switch {
		case hadAS && !aliasConsumed:
			// `FROM t AS` with no (valid) alias after AS.
			markers = append(markers, diagMarkerSpan(r,
				"Expected an alias after AS in the FROM clause."))
		case j < len(sig) && (sig[j].Kind == sqltok.NumberLit ||
			sig[j].Kind == sqltok.StringLit || sig[j].Kind == sqltok.DollarQuoted):
			markers = append(markers, diagMarkerSpan(r,
				"Unexpected token '"+sig[j].Text(sql)+"' after table reference in FROM clause. "+
					"Add a comma, JOIN, or WHERE clause."))
		case j < len(sig) && aliasConsumed && tokUpper(sig[j], sql) == "AS":
			// `FROM t aaa AS …` — a second AS after the alias is already given.
			markers = append(markers, diagMarkerSpan(r,
				"Unexpected 'AS' after the table alias in the FROM clause."))
		}
		i = j - 1 // resume scanning after this table reference
	}
	return markers
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
			// normIdent strips a quoted-identifier's double quotes, so
			// SNOWFLAKE.CORTEX."COMPLETE"(…) resolves to COMPLETE, not "COMPLETE".
			name := normIdent(sig[i+4].Text(rawText), true)
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
						"SENTIMENT, SUMMARIZE, TRANSLATE, CLASSIFY_TEXT, EMBED_TEXT_768, EMBED_TEXT_1024, FINETUNE, SEARCH_PREVIEW, TRY_COMPLETE, " +
						"PARSE_DOCUMENT, COUNT_TOKENS, ENTITY_SENTIMENT, SPLIT_TEXT_RECURSIVE_CHARACTER, SPLIT_TEXT_MARKDOWN_HEADER, AGENT_RUN, DATA_AGENT_RUN.",
					Severity: SeverityWarning,
				})
			}
			i += 5
		}
	}
	return out
}

// ── Recovered semantic anti-pattern validators ──────────────────────────────
//
// The validators and helpers below were originally part of the (now-removed)
// ValidateSnowflakePatterns. They reason about clause-body structure that the
// grammar engine consumes permissively, so they are re-homed here verbatim and
// wired into ValidateAntiPatterns.

// pivotValidAggs lists the aggregate functions Snowflake accepts in a PIVOT.
var pivotValidAggs = map[string]bool{
	"SUM": true, "AVG": true, "COUNT": true, "MAX": true, "MIN": true,
	"ANY_VALUE": true, "LISTAGG": true, "MEDIAN": true,
	"STDDEV": true, "VARIANCE": true,
}

// ── token helpers ────────────────────────────────────────────────────────────

// hasKWPair checks if keyword kw1 is immediately followed by kw2 in the
// significant token stream.
func hasKWPair(sig []sqltok.Token, sql, kw1, kw2 string) bool {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 {
			return true
		}
	}
	return false
}

// hasKWSeq4 checks for four consecutive keywords kw1 kw2 kw3 kw4 in the
// significant token stream.
func hasKWSeq4(sig []sqltok.Token, sql, kw1, kw2, kw3, kw4 string) bool {
	for i := 0; i+3 < len(sig); i++ {
		if tokUpper(sig[i], sql) == kw1 && tokUpper(sig[i+1], sql) == kw2 &&
			tokUpper(sig[i+2], sql) == kw3 && tokUpper(sig[i+3], sql) == kw4 {
			return true
		}
	}
	return false
}

// findKWLParen returns the index of the keyword followed immediately by an
// opening "(", or -1 if no such keyword/"(" pair exists.
func findKWLParen(sig []sqltok.Token, sql, keyword string) int {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == keyword && sig[i+1].Kind == sqltok.LParen {
			return i
		}
	}
	return -1
}

// extractParenContentTok extracts the raw SQL text inside the parentheses
// following the keyword at index kwIdx in sig. It handles nested parens.
// For example, given "AS ( a VARCHAR, b NUMBER(10,2) )", it returns
// " a VARCHAR, b NUMBER(10,2) ".
func extractParenContentTok(sig []sqltok.Token, sql string, kwIdx int) string {
	if kwIdx+1 >= len(sig) || sig[kwIdx+1].Kind != sqltok.LParen {
		return ""
	}
	openPos := sig[kwIdx+1].End // byte position after the opening (
	depth := 1
	for i := kwIdx + 2; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return sql[openPos:sig[i].Start]
			}
		}
	}
	// Unterminated — return everything after the opening paren.
	return sql[openPos:]
}

// ── diagnostic helpers ───────────────────────────────────────────────────────

// diagMarkerSpan constructs a Warning DiagMarker covering the full statement.
// Every pattern diagnostic is a warning (severity 4), so the severity is fixed.
func diagMarkerSpan(r StatementRange, msg string) DiagMarker {
	return DiagMarker{
		StartLineNumber: r.StartLine,
		StartColumn:     1,
		EndLineNumber:   r.EndLine,
		EndColumn:       100,
		Message:         msg,
		Severity:        4, // Warning
	}
}

// cleanParseText strips comments and string literals and trims the result.
func cleanParseText(s string) string {
	return strings.TrimSpace(sqltok.StripStrings(sqltok.StripComments(s)))
}

// ── PIVOT / UNPIVOT validation ───────────────────────────────────────────────

// validatePivotClauses checks all PIVOT(...) occurrences in the statement for
// structural correctness: valid aggregate function, FOR ... IN ..., non-empty
// IN list.
func validatePivotClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigTokens(stripped)

	// Find all PIVOT ( occurrences in the token stream.
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], stripped) != "PIVOT" || sig[i+1].Kind != sqltok.LParen {
			continue
		}
		// Make sure this is not UNPIVOT (the U and N are separate tokens? No —
		// UNPIVOT is a single keyword token, so PIVOT here is standalone).

		// Extract balanced paren content.
		pivotBody := extractParenContentTok(sig, stripped, i)
		if pivotBody == "" {
			continue
		}

		// 1. Validate aggregate function — first ident followed by ( inside the body.
		bodySig := sigTokens(pivotBody)
		if len(bodySig) >= 2 && isIdent(bodySig[0]) && bodySig[1].Kind == sqltok.LParen {
			funcName := strings.ToUpper(bodySig[0].Text(pivotBody))
			if !pivotValidAggs[funcName] {
				markers = append(markers, diagMarkerSpan(r,
					"'"+funcName+"' is not a valid aggregate function for PIVOT. Use SUM, AVG, COUNT, MAX, MIN, ANY_VALUE, LISTAGG, MEDIAN, STDDEV, or VARIANCE."))
			}
		}

		// 2. Check FOR ... IN ( is present in body.
		hasForIn := false
		for j := 0; j+2 < len(bodySig); j++ {
			if tokUpper(bodySig[j], pivotBody) == "FOR" {
				for k := j + 1; k+1 < len(bodySig); k++ {
					if tokUpper(bodySig[k], pivotBody) == "IN" && bodySig[k+1].Kind == sqltok.LParen {
						hasForIn = true
						// 3. Check IN list is not empty — LParen immediately followed by RParen.
						if k+2 < len(bodySig) && bodySig[k+2].Kind == sqltok.RParen {
							markers = append(markers, diagMarkerSpan(r,
								"PIVOT IN list must not be empty. Provide at least one literal value."))
						}
						break
					}
				}
				break
			}
		}
		if !hasForIn {
			markers = append(markers, diagMarkerSpan(r,
				"PIVOT requires FOR <column> IN (<values>)."))
		}
	}
	return markers
}

// validateUnpivotClauses checks all UNPIVOT(...) occurrences in the statement
// for structural correctness: FOR ... IN ..., non-empty IN list.
func validateUnpivotClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigTokens(stripped)

	// Find all UNPIVOT [INCLUDE|EXCLUDE NULLS] ( occurrences.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) != "UNPIVOT" {
			continue
		}
		// Skip optional INCLUDE/EXCLUDE NULLS.
		j := i + 1
		if j < len(sig) {
			u := tokUpper(sig[j], stripped)
			if u == "INCLUDE" || u == "EXCLUDE" {
				j++
				if j < len(sig) && tokUpper(sig[j], stripped) == "NULLS" {
					j++
				}
			}
		}
		if j >= len(sig) || sig[j].Kind != sqltok.LParen {
			continue
		}

		// Extract balanced paren content.
		unpivotBody := extractParenContentTok(sig, stripped, j-1)
		if unpivotBody == "" {
			continue
		}

		// Tokenize the body for structural checks.
		bodySig := sigTokens(unpivotBody)

		// 1. Check FOR ... IN ( is present.
		hasForIn := false
		for k := 0; k+2 < len(bodySig); k++ {
			if tokUpper(bodySig[k], unpivotBody) == "FOR" {
				for m := k + 1; m+1 < len(bodySig); m++ {
					if tokUpper(bodySig[m], unpivotBody) == "IN" && bodySig[m+1].Kind == sqltok.LParen {
						hasForIn = true
						// 2. Check IN list is not empty.
						if m+2 < len(bodySig) && bodySig[m+2].Kind == sqltok.RParen {
							markers = append(markers, diagMarkerSpan(r,
								"UNPIVOT IN list must not be empty. Provide at least one column name."))
						}
						break
					}
				}
				break
			}
		}
		if !hasForIn {
			markers = append(markers, diagMarkerSpan(r,
				"UNPIVOT requires FOR <name_column> IN (<columns>)."))
		}
	}
	return markers
}

// ── MATCH_RECOGNIZE validation ────────────────────────────────────────────────

// validateMatchRecognizeClauses checks all MATCH_RECOGNIZE(...) occurrences in
// the statement for structural correctness:
//   - mandatory PATTERN clause with at least one variable
//   - mandatory DEFINE clause
//   - ONE ROW PER MATCH / ALL ROWS PER MATCH mutual exclusion
//   - AFTER MATCH SKIP target validity
func validateMatchRecognizeClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	clean := cleanParseText(stripped)

	sig := sigTokens(clean)

	// Find all MATCH_RECOGNIZE ( occurrences at the top level.
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], clean) != "MATCH_RECOGNIZE" || sig[i+1].Kind != sqltok.LParen {
			continue
		}
		// Extract balanced paren body as tokens.
		bodyStart, bodyEnd, ok := parenInnerRange(sig, i+1)
		if !ok {
			continue
		}
		body := sig[bodyStart:bodyEnd]

		// 1. Validate mandatory PATTERN clause.
		patternIdx := findKWLParen(body, clean, "PATTERN")
		if patternIdx < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"MATCH_RECOGNIZE requires a PATTERN clause."))
		} else {
			// Check for empty PATTERN — LParen immediately followed by RParen.
			lpIdx := patternIdx + 1 // the LParen after PATTERN
			if lpIdx < len(body) && lpIdx+1 < len(body) && body[lpIdx+1].Kind == sqltok.RParen {
				// Verify the RParen matches this LParen (depth=1).
				markers = append(markers, diagMarkerSpan(r,
					"MATCH_RECOGNIZE PATTERN must contain at least one pattern variable."))
			}
		}

		// 2. Validate mandatory DEFINE clause (DEFINE must be followed by at least
		//    one binding — bare "DEFINE)" is treated as missing).
		hasDefine := false
		for j := 0; j < len(body); j++ {
			if tokUpper(body[j], clean) == "DEFINE" && j+1 < len(body) {
				hasDefine = true
				break
			}
		}
		if !hasDefine {
			markers = append(markers, diagMarkerSpan(r,
				"MATCH_RECOGNIZE requires a DEFINE clause to bind pattern variables."))
		}

		// 3. ONE ROW PER MATCH / ALL ROWS PER MATCH mutual exclusion.
		hasOneRow := hasKWSeq4(body, clean, "ONE", "ROW", "PER", "MATCH")
		hasAllRows := hasKWSeq4(body, clean, "ALL", "ROWS", "PER", "MATCH")
		if hasOneRow && hasAllRows {
			markers = append(markers, diagMarkerSpan(r,
				"ONE ROW PER MATCH and ALL ROWS PER MATCH are mutually exclusive. Use one or the other."))
		}

		// 4. AFTER MATCH SKIP target validation.
		for j := 0; j+2 < len(body); j++ {
			if tokUpper(body[j], clean) == "AFTER" &&
				tokUpper(body[j+1], clean) == "MATCH" &&
				tokUpper(body[j+2], clean) == "SKIP" {
				// Collect target tokens until a boundary keyword or end.
				target := j + 3
				end := len(body)
				for k := target; k < len(body); k++ {
					u := tokUpper(body[k], clean)
					if u == "PATTERN" || u == "DEFINE" || u == "MEASURES" ||
						u == "ONE" || u == "ALL" || u == "ORDER" || u == "PARTITION" {
						end = k
						break
					}
				}
				if target < end {
					targetToks := body[target:end]
					if !isValidAfterMatchSkipTarget(targetToks, clean) {
						markers = append(markers, diagMarkerSpan(r,
							"Invalid AFTER MATCH SKIP target. Use TO NEXT ROW, PAST LAST ROW, TO FIRST <variable>, or TO LAST <variable>."))
					}
				}
				break
			}
		}
	}
	return markers
}

// isValidAfterMatchSkipTarget checks if the token sequence represents a valid
// AFTER MATCH SKIP target: TO NEXT ROW, PAST LAST ROW, TO FIRST <ident>, TO LAST <ident>.
func isValidAfterMatchSkipTarget(toks []sqltok.Token, sql string) bool {
	if len(toks) < 2 {
		return false
	}
	first := tokUpper(toks[0], sql)
	switch first {
	case "TO":
		if len(toks) >= 3 && tokUpper(toks[1], sql) == "NEXT" && tokUpper(toks[2], sql) == "ROW" {
			return true
		}
		if len(toks) >= 3 && (tokUpper(toks[1], sql) == "FIRST" || tokUpper(toks[1], sql) == "LAST") && isIdent(toks[2]) {
			return true
		}
	case "PAST":
		if len(toks) >= 3 && tokUpper(toks[1], sql) == "LAST" && tokUpper(toks[2], sql) == "ROW" {
			return true
		}
	}
	return false
}

// ── ASOF JOIN validation ───────────────────────────────────────────────────────

// validateAsofJoinClauses checks all ASOF JOIN occurrences in the statement for
// structural correctness:
//   - mandatory MATCH_CONDITION clause (unless USING FUNCTION form is used)
//   - comparison operator inside MATCH_CONDITION must be >=, >, <=, or <
//   - ON and USING clauses are not valid with ASOF JOIN
func validateAsofJoinClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	clean := cleanParseText(stripped)
	sig := sigTokens(clean)

	// Find all top-level ASOF JOIN positions (skip matches inside parens).
	type asofPos struct{ afterIdx int } // index into sig after "JOIN"
	var asofPositions []asofPos
	depth := 0
	for i := 0; i+1 < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(sig[i], clean) == "ASOF" && tokUpper(sig[i+1], clean) == "JOIN" {
				asofPositions = append(asofPositions, asofPos{afterIdx: i + 2})
			}
		}
	}

	for idx, ap := range asofPositions {
		// Scope: tokens after this ASOF JOIN up to the next top-level ASOF JOIN.
		scopeEnd := len(sig)
		if idx+1 < len(asofPositions) {
			scopeEnd = asofPositions[idx+1].afterIdx - 2
		}
		scope := sig[ap.afterIdx:scopeEnd]

		hasMatchCondition := findKWLParen(scope, clean, "MATCH_CONDITION") >= 0
		hasUsingFunction := hasUsingFunctionTok(scope, clean)

		// 1. Check for invalid ON clause.
		flaggedOnOrUsing := false
		if hasOnClauseTok(scope, clean, hasMatchCondition) {
			markers = append(markers, diagMarkerSpan(r,
				"ON clause is not valid with ASOF JOIN. Use MATCH_CONDITION instead."))
			flaggedOnOrUsing = true
		}

		// 2. Check for invalid USING clause (plain USING, not USING FUNCTION).
		//    Docs allow `MATCH_CONDITION (…) [ ON … | USING (…) ]`, so a USING that
		//    follows MATCH_CONDITION is legal — only a USING standing in *for* the
		//    match condition is flagged.
		if hasUsingClauseTok(scope, clean, hasUsingFunction, hasMatchCondition) {
			markers = append(markers, diagMarkerSpan(r,
				"USING clause is not valid with ASOF JOIN. Use MATCH_CONDITION instead."))
			flaggedOnOrUsing = true
		}

		// 3. Validate MATCH_CONDITION or USING FUNCTION is present.
		if !hasMatchCondition && !hasUsingFunction && !flaggedOnOrUsing {
			markers = append(markers, diagMarkerSpan(r,
				"ASOF JOIN requires a MATCH_CONDITION clause. Use ASOF JOIN <table> MATCH_CONDITION (<left_expr> >= <right_expr>)."))
			continue
		}

		// 4. If MATCH_CONDITION is present, validate the comparison operator.
		if hasMatchCondition {
			mcIdx := findKWLParen(scope, clean, "MATCH_CONDITION")
			if mcIdx >= 0 {
				// Check if the MATCH_CONDITION paren is properly closed.
				_, _, matched := parenInnerRange(scope, mcIdx+1)
				if matched {
					mcBody := extractParenContentTok(scope, clean, mcIdx)
					// Empty body or body without valid comparison.
					if !containsAsofValidComparison(mcBody) {
						markers = append(markers, diagMarkerSpan(r,
							"MATCH_CONDITION comparison must use one of: >=, >, <=, <. Operators =, <>, != are not supported."))
					}
				}
			}
		}
	}
	return markers
}

// hasOnClauseTok checks if a top-level ON keyword appears in the token scope,
// excluding ON that appears after MATCH_CONDITION.
func hasOnClauseTok(scope []sqltok.Token, sql string, hasMatchCondition bool) bool {
	mcIdx := -1
	if hasMatchCondition {
		mcIdx = findKWLParen(scope, sql, "MATCH_CONDITION")
	}
	depth := 0
	for i := 0; i < len(scope); i++ {
		switch scope[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(scope[i], sql) == "ON" {
				if mcIdx >= 0 && i > mcIdx {
					continue
				}
				return true
			}
		}
	}
	return false
}

// hasUsingClauseTok checks if USING ( appears at the top level, and it's not
// the USING (func(...)) function form, nor a USING that follows MATCH_CONDITION
// (where it is a valid equi-join key list). Mirrors hasOnClauseTok.
func hasUsingClauseTok(scope []sqltok.Token, sql string, hasUsingFunction, hasMatchCondition bool) bool {
	if hasUsingFunction {
		return false
	}
	mcIdx := -1
	if hasMatchCondition {
		mcIdx = findKWLParen(scope, sql, "MATCH_CONDITION")
	}
	depth := 0
	for i := 0; i < len(scope); i++ {
		switch scope[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(scope[i], sql) == "USING" &&
				i+1 < len(scope) && scope[i+1].Kind == sqltok.LParen {
				if mcIdx >= 0 && i > mcIdx {
					continue
				}
				return true
			}
		}
	}
	return false
}

// hasUsingFunctionTok checks for the USING (func_name(...)) pattern in scope.
// This detects: USING ( ident [. ident]* (
func hasUsingFunctionTok(scope []sqltok.Token, sql string) bool {
	for i := 0; i+2 < len(scope); i++ {
		if tokUpper(scope[i], sql) != "USING" || scope[i+1].Kind != sqltok.LParen {
			continue
		}
		// After '(' look for ident [.ident]* (
		j := i + 2
		if j >= len(scope) || !isIdent(scope[j]) {
			continue
		}
		j++
		for j+1 < len(scope) && scope[j].Kind == sqltok.Dot && isIdent(scope[j+1]) {
			j += 2
		}
		if j < len(scope) && scope[j].Kind == sqltok.LParen {
			return true
		}
	}
	return false
}

// containsAsofValidComparison checks whether the MATCH_CONDITION body contains
// one of the valid comparison operators (>=, >, <=, <) and does NOT contain only
// invalid operators (=, <>, !=).
func containsAsofValidComparison(body string) bool {
	for i := 0; i < len(body); i++ {
		ch := body[i]
		switch ch {
		case '>':
			if i+1 < len(body) && body[i+1] == '=' {
				return true // >=
			}
			return true // >
		case '<':
			if i+1 < len(body) && body[i+1] == '>' {
				i++ // <> — invalid, skip past '>'
				continue
			}
			if i+1 < len(body) && body[i+1] == '=' {
				return true // <=
			}
			return true // <
		case '!':
			if i+1 < len(body) && body[i+1] == '=' {
				i++ // != — invalid, skip past '='
				continue
			}
		case '=':
			// Bare = — invalid, skip
			continue
		}
	}
	return false
}

// ── INSERT ALL / INSERT FIRST / INSERT OVERWRITE validation ───────────────────

// validateInsertAll validates INSERT [OVERWRITE] ALL statements.
// Rules:
//   - At least one INTO clause is required
//   - If WHEN branches are present, at least one is required (ELSE alone is invalid)
//   - Each WHEN branch must contain a THEN INTO
//   - A trailing SELECT is mandatory
func validateInsertAll(stripped string, r StatementRange) []DiagMarker {
	return validateInsertMultiTable("ALL", stripped, r)
}

// validateInsertFirst validates INSERT [OVERWRITE] FIRST statements.
// Rules:
//   - At least one WHEN branch is required (INSERT FIRST always requires conditions)
//   - Each WHEN branch must contain a THEN INTO
//   - A trailing SELECT is mandatory
func validateInsertFirst(stripped string, r StatementRange) []DiagMarker {
	return validateInsertMultiTable("FIRST", stripped, r)
}

// validateInsertMultiTable is the shared implementation for INSERT ALL and
// INSERT FIRST validation. It is fully token-based: the tokenizer classifies
// keywords inside string literals as non-keyword tokens, so they are ignored
// without a separate string-stripping pass, and word boundaries are intrinsic.
func validateInsertMultiTable(keyword string, stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigTokens(stripped)

	// Find the last top-level (depth 0) SELECT. Keywords after it belong to the
	// source query (e.g. CASE WHEN/ELSE inside the SELECT) and must be ignored.
	trailingSelectIdx := -1
	depth := 0
	for i, t := range sig {
		switch t.Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(t, stripped) == "SELECT" {
				trailingSelectIdx = i
			}
		}
	}

	// Scan for top-level WHEN/ELSE/INTO keywords before the trailing SELECT.
	scanEnd := len(sig)
	if trailingSelectIdx >= 0 {
		scanEnd = trailingSelectIdx
	}
	var whenIdxs []int
	hasElse, hasInto := false, false
	depth = 0
	for i := 0; i < scanEnd; i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth != 0 {
				continue
			}
			switch tokUpper(sig[i], stripped) {
			case "WHEN":
				whenIdxs = append(whenIdxs, i)
			case "ELSE":
				hasElse = true
			case "INTO":
				hasInto = true
			}
		}
	}
	hasWhen := len(whenIdxs) > 0

	// INSERT FIRST always requires WHEN branches.
	if keyword == "FIRST" && !hasWhen {
		return append(markers, diagMarkerSpan(r,
			"INSERT FIRST requires at least one WHEN branch. Use WHEN <condition> THEN INTO <table>."))
	}

	if hasWhen || hasElse {
		// Conditional form: require at least one WHEN (ELSE alone is invalid).
		if !hasWhen {
			return append(markers, diagMarkerSpan(r,
				"INSERT "+keyword+" requires at least one WHEN branch when using conditional insert. Use WHEN <condition> THEN INTO <table>."))
		}
		// Each WHEN must contain a THEN INTO before the next WHEN.
		for k, w := range whenIdxs {
			end := scanEnd
			if k+1 < len(whenIdxs) {
				end = whenIdxs[k+1]
			}
			if !hasKWPair(sig[w:end], stripped, "THEN", "INTO") {
				markers = append(markers, diagMarkerSpan(r,
					"WHEN branch must contain INTO clause. Use WHEN <condition> THEN INTO <table>."))
			}
		}
	} else if !hasInto {
		// Unconditional form: require at least one INTO.
		return append(markers, diagMarkerSpan(r,
			"INSERT "+keyword+" requires at least one INTO clause."))
	}

	// Trailing SELECT is mandatory for all multi-table inserts.
	if trailingSelectIdx < 0 {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT "+keyword+" requires a source SELECT at the end of the statement."))
	}

	return markers
}

// validateInsertOverwrite validates INSERT OVERWRITE INTO statements.
// Rules:
//   - INTO is required after OVERWRITE (bare INSERT OVERWRITE <table> is invalid)
//   - A source SELECT or VALUES is required
func validateInsertOverwrite(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigTokens(stripped)
	if len(sig) < 2 {
		return nil
	}

	// If this is actually INSERT OVERWRITE ALL or INSERT OVERWRITE FIRST,
	// those are handled by validateInsertAll/validateInsertFirst.
	// Check: INSERT [OVERWRITE] ALL/FIRST
	idx := 1 // start after INSERT
	if tokUpper(sig[idx], stripped) == "OVERWRITE" {
		idx++
	}
	if idx < len(sig) {
		kw := tokUpper(sig[idx], stripped)
		if kw == "ALL" || kw == "FIRST" {
			return nil
		}
	}

	// INSERT OVERWRITE must be followed by INTO.
	// sig[0]=INSERT, sig[1]=OVERWRITE — check sig[2] for INTO.
	if len(sig) < 3 || tokUpper(sig[2], stripped) != "INTO" {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT OVERWRITE requires INTO. Use INSERT OVERWRITE INTO <table>."))
		return markers
	}

	// Check for a source: SELECT or VALUES must appear after the table name
	// and optional column list.
	i := 3 // after INTO
	if i < len(sig) && isIdent(sig[i]) {
		_, i = readIdentPath(sig, stripped, i)
	}
	// Skip optional column list (parenthesized).
	if i < len(sig) && sig[i].Kind == sqltok.LParen {
		if _, closeIdx, ok := parenInnerRange(sig, i); ok {
			i = closeIdx + 1
		} else {
			i = len(sig)
		}
	}

	hasSource := false
	for ; i < len(sig); i++ {
		u := tokUpper(sig[i], stripped)
		if u == "SELECT" || u == "VALUES" {
			hasSource = true
			break
		}
	}
	if !hasSource {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT OVERWRITE INTO requires a source SELECT or VALUES clause."))
	}

	return markers
}

// ── Time Travel validation ────────────────────────────────────────────────────

// validateTimeTravelClauses checks AT(...) / BEFORE(...) Time Travel clauses:
//   - parentheses are required (bare AT TIMESTAMP ... is invalid)
//   - exactly one keyword argument (TIMESTAMP/OFFSET/STATEMENT, plus STREAM for AT)
//   - the => operator is required
//   - STREAM => is not valid inside BEFORE
func validateTimeTravelClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigTokens(stripped)

	// Check for bare AT/BEFORE without parentheses after a table reference.
	// Pattern: FROM/JOIN <ident> AT/BEFORE TIMESTAMP/OFFSET/STATEMENT/STREAM
	ttKWs := []string{"TIMESTAMP", "OFFSET", "STATEMENT", "STREAM"}
	for i := 0; i+3 < len(sig); i++ {
		u := tokUpper(sig[i], stripped)
		if u != "FROM" && u != "JOIN" {
			continue
		}
		// Skip ident path after FROM/JOIN
		j := i + 1
		if !isIdent(sig[j]) {
			continue
		}
		_, j = readIdentPath(sig, stripped, j)
		if j >= len(sig) {
			continue
		}
		ab := tokUpper(sig[j], stripped)
		if ab != "AT" && ab != "BEFORE" {
			continue
		}
		// Check if next token is a time travel keyword (not LParen).
		if j+1 < len(sig) && sig[j+1].Kind != sqltok.LParen {
			nextU := tokUpper(sig[j+1], stripped)
			for _, kw := range ttKWs {
				if nextU == kw {
					markers = append(markers, diagMarkerSpan(r,
						"Time Travel clause requires parentheses. Use AT (TIMESTAMP => ...) or BEFORE (STATEMENT => ...)."))
					break
				}
			}
		}
	}

	// Find each AT(...) / BEFORE(...) occurrence and validate contents.
	for i := 0; i+1 < len(sig); i++ {
		keyword := tokUpper(sig[i], stripped)
		if keyword != "AT" && keyword != "BEFORE" {
			continue
		}
		if sig[i+1].Kind != sqltok.LParen {
			continue
		}

		// Extract tokens inside the parentheses.
		innerStart, innerEnd, ok := parenInnerRange(sig, i+1)
		if !ok {
			continue // Unbalanced — the syntax checker will flag it.
		}
		innerSig := sig[innerStart:innerEnd]

		// Count how many valid keyword arguments appear (KW =>).
		var args []string
		for k := 0; k+1 < len(innerSig); k++ {
			u := tokUpper(innerSig[k], stripped)
			if (u == "TIMESTAMP" || u == "OFFSET" || u == "STATEMENT" || u == "STREAM") &&
				innerSig[k+1].Kind == sqltok.Operator && innerSig[k+1].Text(stripped) == "=>" {
				args = append(args, u)
			}
		}

		streamExpected := ""
		streamPlain := ""
		if keyword == "AT" {
			streamExpected = ", STREAM =>"
			streamPlain = ", STREAM"
		}

		if len(args) == 0 {
			// Check if the user wrote a keyword without =>
			bareKW := ""
			for k := 0; k < len(innerSig); k++ {
				u := tokUpper(innerSig[k], stripped)
				if u == "TIMESTAMP" || u == "OFFSET" || u == "STATEMENT" || u == "STREAM" {
					bareKW = u
					break
				}
			}
			if bareKW != "" {
				markers = append(markers, diagMarkerSpan(r,
					"Missing '=>' operator in "+keyword+" clause. Use "+bareKW+" => <value>."))
			} else {
				markers = append(markers, diagMarkerSpan(r,
					"Invalid "+keyword+" clause. Expected one of: TIMESTAMP =>, OFFSET =>, STATEMENT =>"+streamExpected+"."))
			}
			continue
		}

		if len(args) > 1 {
			markers = append(markers, diagMarkerSpan(r,
				"Multiple keyword arguments in "+keyword+" clause. Only one of TIMESTAMP, OFFSET, STATEMENT"+streamPlain+" is allowed."))
			continue
		}

		// Exactly one argument — validate STREAM restriction.
		if args[0] == "STREAM" && keyword == "BEFORE" {
			markers = append(markers, diagMarkerSpan(r,
				"STREAM => is not valid in a BEFORE clause. STREAM is only supported with AT."))
		}
	}

	return markers
}
