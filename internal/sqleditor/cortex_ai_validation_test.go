package sqleditor

import (
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
