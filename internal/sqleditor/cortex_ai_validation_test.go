package sqleditor

import (
	"strings"
	"testing"
)

// ── Cortex AI function call tests ─────────────────────────────────────────────

func TestCortexAI_ValidateTablesExist_NoFalsePositives(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', user_prompt) FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(review_body) AS score FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(doc_text, 'What is the deadline?') FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', chunk) FROM LIVE_TABLE",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(article_text) FROM LIVE_TABLE",
	}

	req := ValidateTablesExistRequest{
		ResolvedRefs:   getLiveRefs(),
		KnownDatabases: []string{"DB", "GOVERNANCE"},
		KnownSchemas:   []SchemaEntry{{DB: "DB", Name: "SCH"}, {DB: "GOVERNANCE", Name: "PUBLIC"}},
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateTablesExist(req)
			errs := getErrors(markers)
			if len(errs) > 0 {
				t.Errorf("Expected 0 errors for %q, got %d: %v", sql, len(errs), errs[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateBareColumnRefs_NoFalsePositives(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', ID) FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(FIRST_NAME) AS score FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(FIRST_NAME, 'What is the deadline?') FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', FIRST_NAME) FROM DB.SCH.EMPLOYEES",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(LAST_NAME) FROM DB.SCH.EMPLOYEES",
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warns := getWarnings(markers)
			if len(warns) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warns), warns[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateSnowflakePatterns_KnownFunctions(t *testing.T) {
	validQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', user_prompt) FROM prompts",
		"SELECT SNOWFLAKE.CORTEX.SENTIMENT(review_body) AS score FROM reviews",
		"SELECT SNOWFLAKE.CORTEX.EXTRACT_ANSWER(doc_text, 'What is the deadline?') FROM contracts",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_768('snowflake-arctic-embed-m', chunk) FROM corpus",
		"SELECT SNOWFLAKE.CORTEX.SUMMARIZE(article_text) FROM news",
		"SELECT SNOWFLAKE.CORTEX.TRANSLATE(text_col, 'en', 'fr') FROM docs",
		"SELECT SNOWFLAKE.CORTEX.CLASSIFY_TEXT(review, ARRAY_CONSTRUCT('positive', 'negative')) FROM reviews",
		"SELECT SNOWFLAKE.CORTEX.EMBED_TEXT_1024('model', chunk) FROM corpus",
		"SELECT SNOWFLAKE.CORTEX.TRY_COMPLETE('model', prompt) FROM t",
		"SELECT SNOWFLAKE.CORTEX.SEARCH_PREVIEW('service', query) FROM t",
		"SELECT SNOWFLAKE.CORTEX.FINETUNE('model', 'train_data') FROM t",
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			allMarkers := append(getErrors(markers), getWarnings(markers)...)
			if len(allMarkers) > 0 {
				t.Errorf("Expected 0 markers for %q, got %d: %v", sql, len(allMarkers), allMarkers[0].Message)
			}
		})
	}
}

func TestCortexAI_ValidateSnowflakePatterns_UnknownFunction(t *testing.T) {
	invalidQueries := []struct {
		sql     string
		wantMsg string
	}{
		{
			sql:     "SELECT SNOWFLAKE.CORTEX.MAGIC_ANSWER(col) FROM t",
			wantMsg: "Unknown Cortex function",
		},
		{
			sql:     "SELECT SNOWFLAKE.CORTEX.DOES_NOT_EXIST(col) FROM t",
			wantMsg: "Unknown Cortex function",
		},
	}

	for _, tc := range invalidQueries {
		t.Run(tc.sql[:min(len(tc.sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Fatalf("Expected warning for %q, got none", tc.sql)
			}
			found := false
			for _, w := range warns {
				if strings.Contains(w.Message, tc.wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns[0].Message)
			}
		})
	}
}

func TestCortexAI_DuplicateUnknownFunction_DistinctMarkers(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.MAGIC(a), SNOWFLAKE.CORTEX.MAGIC(b) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 2 {
		t.Fatalf("Expected 2 warnings for duplicate unknown Cortex function, got %d", len(warns))
	}
	// The two markers must have distinct column positions.
	if warns[0].StartColumn == warns[1].StartColumn {
		t.Errorf("Both markers have the same StartColumn=%d; expected distinct positions", warns[0].StartColumn)
	}
}

func TestCortexAI_NoFalsePositivesInCommentsAndStrings(t *testing.T) {
	noWarnQueries := []string{
		// Line comment
		"-- SELECT SNOWFLAKE.CORTEX.MAGIC_FUNC(x)\nSELECT 1",
		// Block comment
		"/* SNOWFLAKE.CORTEX.FAKE_FUNC(x) */ SELECT 1",
		// String literal
		"SELECT 'SNOWFLAKE.CORTEX.FAKE_FUNC(x)' FROM t",
		// Dollar-quoted string
		"EXECUTE IMMEDIATE $$SELECT SNOWFLAKE.CORTEX.FAKE_FUNC(x) FROM t$$",
		// Dollar-quoted procedure body with tagged delimiter
		"CREATE PROCEDURE p() RETURNS STRING LANGUAGE SQL AS $body$\n  SELECT SNOWFLAKE.CORTEX.NOT_REAL(col) FROM t;\n$body$",
	}

	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			for _, w := range warns {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_CaseInsensitive(t *testing.T) {
	// Known function in lowercase — should produce no warning
	validQueries := []string{
		"SELECT snowflake.cortex.complete('mistral-7b', prompt) FROM t",
		"SELECT Snowflake.Cortex.Sentiment(review) FROM t",
		"SELECT SNOWFLAKE.cortex.SUMMARIZE(text) FROM t",
	}
	for _, sql := range validQueries {
		t.Run("valid/"+sql[:min(len(sql), 40)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for %q, got: %v", sql, w.Message)
				}
			}
		})
	}

	// Unknown function in lowercase — should still produce a warning
	invalidSQL := "SELECT snowflake.cortex.magic_answer(col) FROM t"
	t.Run("invalid/"+invalidSQL[:40], func(t *testing.T) {
		stmtRanges := GetStatementRanges(invalidSQL)
		markers := ValidateSnowflakePatterns(invalidSQL, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected unknown Cortex function warning for %q, got none", invalidSQL)
		}
	})
}

func TestCortexAI_GrantRevokeSkipped(t *testing.T) {
	// GRANT/REVOKE statements referencing Cortex functions should not trigger warnings.
	noWarnQueries := []string{
		"GRANT USAGE ON PROCEDURE SNOWFLAKE.CORTEX.FAKE_FUNC(VARCHAR) TO ROLE analyst",
		"GRANT USAGE ON FUNCTION SNOWFLAKE.CORTEX.NOT_REAL(VARCHAR, VARCHAR) TO ROLE data_eng",
		"REVOKE USAGE ON PROCEDURE SNOWFLAKE.CORTEX.MAGIC(INT) FROM ROLE analyst",
		"REVOKE ALL PRIVILEGES ON FUNCTION SNOWFLAKE.CORTEX.BOGUS(VARCHAR) FROM ROLE public",
	}

	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			warns := getWarnings(markers)
			for _, w := range warns {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for GRANT/REVOKE %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_WhitespaceAroundDots(t *testing.T) {
	// The regex allows flexible whitespace around dots — ensure these are still validated.
	validSQL := "SELECT SNOWFLAKE . CORTEX . COMPLETE('model', prompt) FROM t"
	t.Run("known/spaces", func(t *testing.T) {
		stmtRanges := GetStatementRanges(validSQL)
		markers := ValidateSnowflakePatterns(validSQL, stmtRanges)
		for _, w := range getWarnings(markers) {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				t.Errorf("Expected no Cortex warning for spaced known function, got: %v", w.Message)
			}
		}
	})

	invalidSQL := "SELECT SNOWFLAKE . CORTEX . FAKE_THING('x') FROM t"
	t.Run("unknown/spaces", func(t *testing.T) {
		stmtRanges := GetStatementRanges(invalidSQL)
		markers := ValidateSnowflakePatterns(invalidSQL, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected unknown Cortex function warning for spaced unknown function, got none")
		}
	})
}

func TestCortexAI_MultiLine_PositionAccuracy(t *testing.T) {
	sql := "SELECT 1;\nSELECT SNOWFLAKE.CORTEX.BOGUS_FUNC(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected at least one warning for BOGUS_FUNC on second line")
	}
	// The unknown function is on the second statement (line 2)
	if warns[0].StartLineNumber != 2 {
		t.Errorf("Expected warning on line 2, got line %d", warns[0].StartLineNumber)
	}
}

func TestCortexAI_InSubquery(t *testing.T) {
	sql := "SELECT * FROM (SELECT SNOWFLAKE.CORTEX.UNKNOWN_AI(col) FROM t) sub"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected unknown Cortex function warning inside subquery, got none")
	}
}

func TestCortexAI_NotTriggeredByNonSnowflakePrefix(t *testing.T) {
	// Patterns that look Cortex-like but use a different database prefix should not trigger.
	noWarnQueries := []string{
		"SELECT MYDB.CORTEX.SOME_FUNC(col) FROM t",
		"SELECT SNOWFLAKE.NOTCORTEX.COMPLETE(col) FROM t",
		"SELECT OTHER.CORTEX.SENTIMENT(col) FROM t",
	}

	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for non-SNOWFLAKE.CORTEX pattern %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_MixedKnownAndUnknown(t *testing.T) {
	// Multiple statements: one valid, one invalid — only the invalid one should warn.
	sql := "SELECT SNOWFLAKE.CORTEX.COMPLETE('model', p) FROM t;\nSELECT SNOWFLAKE.CORTEX.NONEXISTENT(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected exactly 1 warning (for NONEXISTENT), got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "NONEXISTENT") {
		t.Errorf("Expected warning about NONEXISTENT, got: %s", warns[0].Message)
	}
}

func TestCortexAI_MarkerPositionColumnsAccurate(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.FAKE_FUNC(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(warns))
	}
	w := warns[0]
	// "SELECT " = 7 chars → StartColumn = 8 (1-based)
	if w.StartColumn != 8 {
		t.Errorf("StartColumn: want 8, got %d", w.StartColumn)
	}
	// "SNOWFLAKE.CORTEX.FAKE_FUNC" = 26 chars → EndColumn = 8+26 = 34
	if w.EndColumn != 34 {
		t.Errorf("EndColumn: want 34, got %d", w.EndColumn)
	}
	if w.StartLineNumber != 1 || w.EndLineNumber != 1 {
		t.Errorf("Line numbers: want 1:1, got %d:%d", w.StartLineNumber, w.EndLineNumber)
	}
}

func TestCortexAI_WithinStatementMultilinePosition(t *testing.T) {
	sql := "SELECT\n    SNOWFLAKE.CORTEX.FAKE(col)\nFROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected warning for FAKE")
	}
	w := warns[0]
	if w.StartLineNumber != 2 {
		t.Errorf("StartLineNumber: want 2, got %d", w.StartLineNumber)
	}
	// "    " = 4 chars of indent → column 5
	if w.StartColumn != 5 {
		t.Errorf("StartColumn: want 5, got %d", w.StartColumn)
	}
}

func TestCortexAI_NoMatchWithoutParenthesis(t *testing.T) {
	// SNOWFLAKE.CORTEX.funcname without opening ( should not trigger the validator.
	noWarnQueries := []string{
		"SELECT SNOWFLAKE.CORTEX.FAKE_FUNC FROM t",
		"DESC FUNCTION SNOWFLAKE.CORTEX.COMPLETE",
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning without parenthesis for %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_SeverityIsWarning(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.UNKNOWN_FUNC(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	var cortexMarker *DiagMarker
	for i, m := range markers {
		if strings.Contains(m.Message, "Unknown Cortex function") {
			cortexMarker = &markers[i]
			break
		}
	}
	if cortexMarker == nil {
		t.Fatalf("No Cortex warning marker found")
		return
	}
	// Severity 4 = Warning in Monaco editor
	if cortexMarker.Severity != 4 {
		t.Errorf("Expected Severity=4 (Warning), got %d", cortexMarker.Severity)
	}
}

func TestCortexAI_InsideCTE(t *testing.T) {
	sql := "WITH cte AS (SELECT SNOWFLAKE.CORTEX.NOT_REAL(col) FROM t) SELECT * FROM cte"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning inside CTE, got none")
	}
}

func TestCortexAI_MessagePreservesOriginalCase(t *testing.T) {
	sql := "SELECT snowflake.cortex.My_Custom_Func(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected warning for My_Custom_Func")
	}
	// The message should preserve the original case of the function name.
	if !strings.Contains(warns[0].Message, "My_Custom_Func") {
		t.Errorf("Expected message to preserve original case 'My_Custom_Func', got: %s", warns[0].Message)
	}
}

func TestCortexAI_GrantDoesNotSuppressNextStatement(t *testing.T) {
	// GRANT in the first statement must not suppress the warning in the second.
	sql := "GRANT USAGE ON FUNCTION SNOWFLAKE.CORTEX.FAKE(VARCHAR) TO ROLE analyst;\nSELECT SNOWFLAKE.CORTEX.FAKE(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			found = true
			if w.StartLineNumber != 2 {
				t.Errorf("Expected warning on line 2, got line %d", w.StartLineNumber)
			}
		}
	}
	if !found {
		t.Error("Expected Cortex warning for FAKE in second (non-GRANT) statement")
	}
}

func TestCortexAI_EmptyAndWhitespaceSQL(t *testing.T) {
	inputs := []string{
		"",
		"   ",
		"\n\n",
		"\t\t",
		";",
		"  ;  ;  ",
	}
	for _, sql := range inputs {
		t.Run(sql, func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for empty/whitespace SQL %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_NestedInsideAnotherFunction(t *testing.T) {
	// Unknown Cortex function nested inside another function call.
	sql := "SELECT UPPER(SNOWFLAKE.CORTEX.FAKE_FUNC(col)) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "FAKE_FUNC") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning when nested inside UPPER(), got none")
	}
}

func TestCortexAI_KnownFunctionNestedInsideAnotherFunction(t *testing.T) {
	// Known Cortex function nested inside another function — no warning expected.
	sql := "SELECT COALESCE(SNOWFLAKE.CORTEX.SENTIMENT(review), 0) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			t.Errorf("Expected no Cortex warning for known function inside COALESCE, got: %v", w.Message)
		}
	}
}

func TestCortexAI_MultipleUnknownsAcrossLines_PositionAccuracy(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.AAA(col) FROM t;\nSELECT SNOWFLAKE.CORTEX.BBB(col) FROM t;\nSELECT SNOWFLAKE.CORTEX.CCC(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 3 {
		t.Fatalf("Expected 3 warnings, got %d", len(warns))
	}
	expectedLines := []int{1, 2, 3}
	expectedFuncs := []string{"AAA", "BBB", "CCC"}
	for i, w := range warns {
		if w.StartLineNumber != expectedLines[i] {
			t.Errorf("Warning %d: expected line %d, got %d", i, expectedLines[i], w.StartLineNumber)
		}
		if !strings.Contains(w.Message, expectedFuncs[i]) {
			t.Errorf("Warning %d: expected message containing %q, got %q", i, expectedFuncs[i], w.Message)
		}
	}
}

func TestCortexAI_InWhereClause(t *testing.T) {
	sql := "SELECT * FROM t WHERE SNOWFLAKE.CORTEX.NOT_REAL(col) > 0.5"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "NOT_REAL") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning in WHERE clause, got none")
	}
}

func TestCortexAI_InCaseExpression(t *testing.T) {
	sql := "SELECT CASE WHEN SNOWFLAKE.CORTEX.FAKE_SCORE(col) > 0 THEN 'pos' ELSE 'neg' END FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "FAKE_SCORE") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning inside CASE expression, got none")
	}
}

func TestCortexAI_KnownAndUnknownInSameStatement(t *testing.T) {
	// A known and unknown Cortex function in the same SELECT must produce
	// exactly one warning — for the unknown one. Ensures no short-circuit.
	sql := "SELECT SNOWFLAKE.CORTEX.COMPLETE('model', col), SNOWFLAKE.CORTEX.BOGUS(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected exactly 1 warning, got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "BOGUS") {
		t.Errorf("Expected warning about BOGUS, got: %s", warns[0].Message)
	}
}

func TestCortexAI_TabAndNewlineWhitespace(t *testing.T) {
	// The regex uses \s* around dots — tabs and newlines should still match.
	unknownSQL := "SELECT SNOWFLAKE\t.\tCORTEX\t.\tFAKE_FUNC(col) FROM t"
	t.Run("tabs/unknown", func(t *testing.T) {
		stmtRanges := GetStatementRanges(unknownSQL)
		markers := ValidateSnowflakePatterns(unknownSQL, stmtRanges)
		found := false
		for _, w := range getWarnings(markers) {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				found = true
			}
		}
		if !found {
			t.Error("Expected Cortex warning with tab separators, got none")
		}
	})

	newlineSQL := "SELECT SNOWFLAKE.\nCORTEX.\nFAKE_FUNC(col) FROM t"
	t.Run("newlines/unknown", func(t *testing.T) {
		stmtRanges := GetStatementRanges(newlineSQL)
		markers := ValidateSnowflakePatterns(newlineSQL, stmtRanges)
		found := false
		for _, w := range getWarnings(markers) {
			if strings.Contains(w.Message, "Unknown Cortex function") {
				found = true
			}
		}
		if !found {
			t.Error("Expected Cortex warning with newline separators, got none")
		}
	})
}

func TestCortexAI_QuotedIdentifiersNoFalsePositive(t *testing.T) {
	// Quoted identifiers like "SNOWFLAKE"."CORTEX"."FUNC"(x) should NOT
	// trigger the validator because the quotes break the regex pattern.
	noWarnQueries := []string{
		`SELECT "SNOWFLAKE"."CORTEX"."FAKE_FUNC"(col) FROM t`,
		`SELECT "SNOWFLAKE".CORTEX.FAKE_FUNC(col) FROM t`,
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for quoted identifiers %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_KnownFunctionNoFromClause(t *testing.T) {
	// Known Cortex function in a standalone SELECT without FROM — no warning.
	sql := "SELECT SNOWFLAKE.CORTEX.COMPLETE('mistral-7b', 'Hello world')"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			t.Errorf("Expected no Cortex warning for known function without FROM, got: %v", w.Message)
		}
	}
}

func TestCortexAI_CreateViewWithCortexCall(t *testing.T) {
	// Unknown Cortex function inside a CREATE VIEW body should still warn.
	sql := "CREATE VIEW v AS SELECT SNOWFLAKE.CORTEX.NOT_REAL(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "NOT_REAL") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning inside CREATE VIEW body, got none")
	}
}

func TestCortexAI_SchemaVariantNoFalsePositive(t *testing.T) {
	// SNOWFLAKE.CORTEX_V2.FUNC or SNOWFLAKE.CORTEX_ML.FUNC should NOT match
	// because the regex expects CORTEX followed by a dot, not CORTEX_*.
	noWarnQueries := []string{
		"SELECT SNOWFLAKE.CORTEX_V2.SOME_FUNC(col) FROM t",
		"SELECT SNOWFLAKE.CORTEX_ML.PREDICT(col) FROM t",
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for schema variant %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_NestedCortexCalls(t *testing.T) {
	// A Cortex call as an argument to another Cortex call.
	// The outer is known but the inner is unknown — should warn on inner.
	sql := "SELECT SNOWFLAKE.CORTEX.COMPLETE('model', SNOWFLAKE.CORTEX.FAKE_PREP(col)) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) != 1 {
		t.Fatalf("Expected 1 warning for nested unknown Cortex call, got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "FAKE_PREP") {
		t.Errorf("Expected warning about FAKE_PREP, got: %s", warns[0].Message)
	}
}

func TestCortexAI_MessageListsAllKnownFunctions(t *testing.T) {
	sql := "SELECT SNOWFLAKE.CORTEX.BOGUS(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	warns := getWarnings(markers)
	if len(warns) == 0 {
		t.Fatal("Expected at least one warning")
	}
	msg := warns[0].Message
	expectedFuncs := []string{
		"COMPLETE", "EXTRACT_ANSWER", "SENTIMENT", "SUMMARIZE",
		"TRANSLATE", "CLASSIFY_TEXT", "EMBED_TEXT_768", "EMBED_TEXT_1024",
		"FINETUNE", "SEARCH_PREVIEW", "TRY_COMPLETE",
	}
	for _, fn := range expectedFuncs {
		if !strings.Contains(msg, fn) {
			t.Errorf("Warning message missing known function %q: %s", fn, msg)
		}
	}
}

func TestCortexAI_InOrderByAndHaving(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"ORDER BY", "SELECT * FROM t ORDER BY SNOWFLAKE.CORTEX.FAKE_SORT(col)"},
		{"HAVING", "SELECT col, COUNT(*) FROM t GROUP BY col HAVING SNOWFLAKE.CORTEX.FAKE_FILTER(col) > 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmtRanges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
			found := false
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected Cortex warning in %s clause, got none", tc.name)
			}
		})
	}
}

func TestCortexAI_WordBoundary_SuffixedSnowflake(t *testing.T) {
	// The regex uses \b before SNOWFLAKE — identifiers that merely end with
	// SNOWFLAKE (e.g. NOTSNOWFLAKE, MY_SNOWFLAKE) must not match.
	noWarnQueries := []string{
		"SELECT NOTSNOWFLAKE.CORTEX.COMPLETE('model', col) FROM t",
		"SELECT MYSNOWFLAKE.CORTEX.SENTIMENT(col) FROM t",
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for suffixed SNOWFLAKE in %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_EscapedQuotesInStringLiteral(t *testing.T) {
	// buildInertMask handles SQL's '' escape inside single-quoted strings.
	// A Cortex-like pattern inside such a string must not trigger a warning.
	noWarnQueries := []string{
		"SELECT 'It''s SNOWFLAKE.CORTEX.FAKE(x)' FROM t",
		"SELECT 'prefix''SNOWFLAKE.CORTEX.BOGUS(col)''suffix' FROM t",
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning inside escaped-quote string %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_AdjacentToClosedComment(t *testing.T) {
	// Cortex call immediately after a closed block comment — the inert mask
	// must end at the closing */ so the call is still flagged.
	cases := []struct {
		name string
		sql  string
	}{
		{"block comment then call", "SELECT /*comment*/SNOWFLAKE.CORTEX.FAKE(col) FROM t"},
		{"line comment then call on next line", "-- comment\nSELECT SNOWFLAKE.CORTEX.FAKE(col) FROM t"},
		{"string literal then call", "SELECT 'hello' || SNOWFLAKE.CORTEX.FAKE(col) FROM t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmtRanges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
			found := false
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "FAKE") {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected Cortex warning adjacent to closed inert region for %q, got none", tc.sql)
			}
		})
	}
}

func TestCortexAI_BareCortexPrefixNoSnowflake(t *testing.T) {
	// Without the SNOWFLAKE database prefix, the regex should not match.
	noWarnQueries := []string{
		"SELECT CORTEX.COMPLETE('model', col) FROM t",
		"SELECT CORTEX.FAKE_FUNC(col) FROM t",
	}
	for _, sql := range noWarnQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			stmtRanges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, stmtRanges)
			for _, w := range getWarnings(markers) {
				if strings.Contains(w.Message, "Unknown Cortex function") {
					t.Errorf("Expected no Cortex warning for bare CORTEX prefix %q, got: %v", sql, w.Message)
				}
			}
		})
	}
}

func TestCortexAI_MultipleKnownFunctionsNoWarnings(t *testing.T) {
	// Multiple known Cortex functions in the same SELECT — zero warnings expected.
	sql := "SELECT SNOWFLAKE.CORTEX.COMPLETE('model', col), SNOWFLAKE.CORTEX.SENTIMENT(col), SNOWFLAKE.CORTEX.SUMMARIZE(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			t.Errorf("Expected no Cortex warning for multiple known functions, got: %v", w.Message)
		}
	}
}

func TestCortexAI_InsertSelectContext(t *testing.T) {
	sql := "INSERT INTO results SELECT SNOWFLAKE.CORTEX.FAKE_AI(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "FAKE_AI") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning inside INSERT...SELECT, got none")
	}
}

func TestCortexAI_CreateTableAsSelectContext(t *testing.T) {
	sql := "CREATE TABLE results AS SELECT SNOWFLAKE.CORTEX.NOT_REAL(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	found := false
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") && strings.Contains(w.Message, "NOT_REAL") {
			found = true
		}
	}
	if !found {
		t.Error("Expected unknown Cortex function warning inside CREATE TABLE AS SELECT, got none")
	}
}

func TestCortexAI_UnterminatedBlockComment(t *testing.T) {
	// An unterminated block comment masks everything after it — a Cortex
	// call inside the unclosed comment must not produce a warning.
	sql := "SELECT 1; /* unclosed comment SNOWFLAKE.CORTEX.FAKE(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			t.Errorf("Expected no Cortex warning inside unterminated block comment, got: %v", w.Message)
		}
	}
}

func TestCortexAI_UnterminatedStringLiteral(t *testing.T) {
	// An unterminated string literal masks everything after the opening quote.
	sql := "SELECT 'unclosed SNOWFLAKE.CORTEX.FAKE(col) FROM t"
	stmtRanges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, stmtRanges)
	for _, w := range getWarnings(markers) {
		if strings.Contains(w.Message, "Unknown Cortex function") {
			t.Errorf("Expected no Cortex warning inside unterminated string literal, got: %v", w.Message)
		}
	}
}

// ── Notebook Tests ──────────────────────────────────────────────────────────
