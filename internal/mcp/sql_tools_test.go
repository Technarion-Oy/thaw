// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// capturingQueryRunner records all queries it receives and returns a canned
// result. It is used to verify that the pipeline sends the correct SQL
// (e.g. with injected LIMIT) to the runner.
type capturingQueryRunner struct {
	queries []string
	result  *snowflake.QueryResult
	err     error
}

func (c *capturingQueryRunner) QuerySingle(_ context.Context, query string) (*snowflake.QueryResult, error) {
	c.queries = append(c.queries, query)
	return c.result, c.err
}

// extractTextFromResult returns the concatenated text content from an MCP
// CallToolResult, or "" if no text content is present.
func extractTextFromResult(r *mcpsdk.CallToolResult) string {
	if r == nil {
		return ""
	}
	var parts []string
	for _, c := range r.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "")
}

// ── injectLimit ─────────────────────────────────────────────────────────────

func TestInjectLimit(t *testing.T) {
	cases := []struct {
		name  string
		sql   string
		limit int
		want  string
	}{
		{
			"basic select",
			"SELECT * FROM t",
			100,
			"SELECT * FROM (SELECT * FROM t) AS _mcp_limit LIMIT 100",
		},
		{
			"trailing semicolon",
			"SELECT * FROM t;",
			100,
			"SELECT * FROM (SELECT * FROM t) AS _mcp_limit LIMIT 100",
		},
		{
			"CTE",
			"WITH cte AS (SELECT 1) SELECT * FROM cte",
			50,
			"SELECT * FROM (WITH cte AS (SELECT 1) SELECT * FROM cte) AS _mcp_limit LIMIT 50",
		},
		{
			"existing limit",
			"SELECT * FROM t LIMIT 5000",
			100,
			"SELECT * FROM (SELECT * FROM t LIMIT 5000) AS _mcp_limit LIMIT 100",
		},
		{
			"subquery",
			"SELECT * FROM (SELECT id FROM t)",
			100,
			"SELECT * FROM (SELECT * FROM (SELECT id FROM t)) AS _mcp_limit LIMIT 100",
		},
		{
			"whitespace around semicolon",
			"SELECT 1 ;  ",
			100,
			"SELECT * FROM (SELECT 1) AS _mcp_limit LIMIT 100",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := injectLimit(tc.sql, tc.limit)
			if got != tc.want {
				t.Errorf("injectLimit(%q, %d) =\n  %q\nwant:\n  %q", tc.sql, tc.limit, got, tc.want)
			}
		})
	}
}

// ── executeSQLPipeline ──────────────────────────────────────────────────────

func TestPipelineEmptySQL(t *testing.T) {
	runner := &capturingQueryRunner{}
	result, err := executeSQLPipeline(context.Background(), runner, "", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "empty SQL") {
		t.Errorf("expected 'empty SQL' in result, got: %s", text)
	}
	if len(runner.queries) != 0 {
		t.Errorf("expected no queries, got %d", len(runner.queries))
	}
}

func TestPipelineMultiStatement(t *testing.T) {
	runner := &capturingQueryRunner{}
	result, err := executeSQLPipeline(context.Background(), runner, "SELECT 1; SELECT 2", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "multi-statement") {
		t.Errorf("expected 'multi-statement' in result, got: %s", text)
	}
	if len(runner.queries) != 0 {
		t.Errorf("expected no queries, got %d", len(runner.queries))
	}
}

func TestPipelineBlockedKeyword(t *testing.T) {
	blocked := []string{
		"INSERT INTO t VALUES (1)",
		"DELETE FROM t",
		"DROP TABLE t",
		"CREATE TABLE t (id INT)",
		"TRUNCATE TABLE t",
		"GRANT SELECT ON t TO r",
	}
	for _, sql := range blocked {
		t.Run(sql, func(t *testing.T) {
			runner := &capturingQueryRunner{}
			result, err := executeSQLPipeline(context.Background(), runner, sql, ExecutionModeReadonly)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			text := extractTextFromResult(result)
			if !strings.Contains(text, "not allowed") {
				t.Errorf("expected 'not allowed' in result, got: %s", text)
			}
			if len(runner.queries) != 0 {
				t.Errorf("expected no queries for blocked keyword, got %d", len(runner.queries))
			}
		})
	}
}

func TestPipelineUSERejection(t *testing.T) {
	runner := &capturingQueryRunner{}
	result, err := executeSQLPipeline(context.Background(), runner, "USE ROLE SYSADMIN", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "USE statements are not allowed") {
		t.Errorf("expected USE rejection in result, got: %s", text)
	}
}

func TestPipelineMetadataPassthrough(t *testing.T) {
	metadataResult := &snowflake.QueryResult{
		Columns: []string{"name"},
		Rows:    [][]any{{"TABLE_A"}, {"TABLE_B"}},
	}
	cases := []struct {
		sql  string
		mode string
	}{
		{"SHOW TABLES", ExecutionModeReadonly},
		{"DESCRIBE TABLE t", ExecutionModeReadonly},
		{"DESC TABLE t", ExecutionModeReadonly},
		{"EXPLAIN SELECT 1", ExecutionModeReadonly},
		{"LIST @mystage", ExecutionModeReadonly},
		// Metadata also works in explain_only mode.
		{"SHOW TABLES", ExecutionModeExplainOnly},
		{"DESCRIBE TABLE t", ExecutionModeExplainOnly},
	}
	for _, tc := range cases {
		t.Run(tc.sql+"_"+tc.mode, func(t *testing.T) {
			runner := &capturingQueryRunner{result: metadataResult}
			result, err := executeSQLPipeline(context.Background(), runner, tc.sql, tc.mode)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// The runner should have received the original SQL (not EXPLAIN-wrapped).
			if len(runner.queries) != 1 {
				t.Fatalf("expected 1 query, got %d", len(runner.queries))
			}
			if strings.HasPrefix(runner.queries[0], "EXPLAIN USING TABULAR") {
				t.Errorf("metadata queries should NOT be EXPLAIN-wrapped, got: %s", runner.queries[0])
			}
			text := extractTextFromResult(result)
			if !strings.Contains(text, "TABLE_A") {
				t.Errorf("expected metadata result, got: %s", text)
			}
		})
	}
}

func TestPipelineMetadataRowCap(t *testing.T) {
	// Build a result with more rows than maxMCPResultRows.
	rows := make([][]any, maxMCPResultRows+100)
	for i := range rows {
		rows[i] = []any{"row"}
	}
	runner := &capturingQueryRunner{result: &snowflake.QueryResult{
		Columns: []string{"name"},
		Rows:    rows,
	}}
	result, err := executeSQLPipeline(context.Background(), runner, "SHOW TABLES", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	// Parse the JSON to check truncation.
	var qr snowflake.QueryResult
	if err := json.Unmarshal([]byte(text), &qr); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(qr.Rows) != maxMCPResultRows {
		t.Errorf("expected %d rows, got %d", maxMCPResultRows, len(qr.Rows))
	}
	if !qr.Truncated {
		t.Error("expected Truncated flag to be set")
	}
}

func TestPipelineExplainOnlyVerdict(t *testing.T) {
	runner := &capturingQueryRunner{result: explainResult("Result", "TableScan")}
	result, err := executeSQLPipeline(context.Background(), runner, "SELECT 1", ExecutionModeExplainOnly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have called EXPLAIN but not the actual query.
	if len(runner.queries) != 1 {
		t.Fatalf("expected 1 query (EXPLAIN), got %d", len(runner.queries))
	}
	if !strings.HasPrefix(runner.queries[0], "EXPLAIN USING TABULAR") {
		t.Errorf("expected EXPLAIN query, got: %s", runner.queries[0])
	}
	text := extractTextFromResult(result)
	var v GateVerdict
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("failed to unmarshal verdict: %v", err)
	}
	if !v.Allowed {
		t.Errorf("expected allowed verdict, got: %s", v.Reason)
	}
}

func TestPipelineReadonlyLimitInjection(t *testing.T) {
	queryResult := &snowflake.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]any{{float64(1)}, {float64(2)}},
	}

	// The capturing runner returns explainResult for the first call (EXPLAIN)
	// and queryResult for the second call (the actual query). We use a
	// sequencing runner for this.
	runner := &sequencingQueryRunner{
		results: []*snowflake.QueryResult{
			explainResult("Result", "TableScan"),
			queryResult,
		},
	}

	result, err := executeSQLPipeline(context.Background(), runner, "SELECT * FROM t", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have two queries: EXPLAIN + the LIMIT-injected query.
	if len(runner.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d: %v", len(runner.queries), runner.queries)
	}
	if !strings.HasPrefix(runner.queries[0], "EXPLAIN USING TABULAR") {
		t.Errorf("first query should be EXPLAIN, got: %s", runner.queries[0])
	}
	if !strings.Contains(runner.queries[1], "_mcp_limit") {
		t.Errorf("second query should contain LIMIT injection, got: %s", runner.queries[1])
	}
	if !strings.Contains(runner.queries[1], "LIMIT 100") {
		t.Errorf("second query should have LIMIT 100, got: %s", runner.queries[1])
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "id") {
		t.Errorf("expected query result, got: %s", text)
	}
}

func TestPipelineDefaultDeny(t *testing.T) {
	unknowns := []string{
		"WHATEVER 1",
		"FOOBAR table t",
		"LATERAL something",
	}
	for _, sql := range unknowns {
		t.Run(sql, func(t *testing.T) {
			runner := &capturingQueryRunner{}
			result, err := executeSQLPipeline(context.Background(), runner, sql, ExecutionModeReadonly)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			text := extractTextFromResult(result)
			if !strings.Contains(text, "not recognized") {
				t.Errorf("expected 'not recognized' in result, got: %s", text)
			}
			if len(runner.queries) != 0 {
				t.Errorf("expected no queries for unknown keyword, got %d", len(runner.queries))
			}
		})
	}
}

func TestPipelineBlockedWithComments(t *testing.T) {
	runner := &capturingQueryRunner{}
	result, err := executeSQLPipeline(context.Background(), runner, "/* bypass */ DROP TABLE t", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "not allowed") {
		t.Errorf("expected rejection even with comments, got: %s", text)
	}
}

func TestPipelineCTE(t *testing.T) {
	runner := &sequencingQueryRunner{
		results: []*snowflake.QueryResult{
			explainResult("WithClause", "WithReference", "Result"),
			{Columns: []string{"x"}, Rows: [][]any{{float64(1)}}},
		},
	}

	result, err := executeSQLPipeline(context.Background(), runner, "WITH cte AS (SELECT 1 AS x) SELECT * FROM cte", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(runner.queries))
	}
	text := extractTextFromResult(result)
	if text == "" {
		t.Error("expected non-empty result")
	}
}

// sequencingQueryRunner returns different results for successive calls.
type sequencingQueryRunner struct {
	queries []string
	results []*snowflake.QueryResult
	errors  []error
	idx     int
}

func (s *sequencingQueryRunner) QuerySingle(_ context.Context, query string) (*snowflake.QueryResult, error) {
	s.queries = append(s.queries, query)
	i := s.idx
	s.idx++
	var result *snowflake.QueryResult
	var err error
	if i < len(s.results) {
		result = s.results[i]
	}
	if i < len(s.errors) {
		err = s.errors[i]
	}
	return result, err
}
