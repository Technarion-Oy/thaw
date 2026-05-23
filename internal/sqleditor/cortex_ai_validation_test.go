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

// ── Notebook Tests ──────────────────────────────────────────────────────────


