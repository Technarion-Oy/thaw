// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
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
			"SELECT * FROM (\nSELECT * FROM t\n) AS _mcp_limit LIMIT 100",
		},
		{
			"trailing semicolon",
			"SELECT * FROM t;",
			100,
			"SELECT * FROM (\nSELECT * FROM t\n) AS _mcp_limit LIMIT 100",
		},
		{
			"CTE",
			"WITH cte AS (SELECT 1) SELECT * FROM cte",
			50,
			"SELECT * FROM (\nWITH cte AS (SELECT 1) SELECT * FROM cte\n) AS _mcp_limit LIMIT 50",
		},
		{
			"existing limit",
			"SELECT * FROM t LIMIT 5000",
			100,
			"SELECT * FROM (\nSELECT * FROM t LIMIT 5000\n) AS _mcp_limit LIMIT 100",
		},
		{
			"subquery",
			"SELECT * FROM (SELECT id FROM t)",
			100,
			"SELECT * FROM (\nSELECT * FROM (SELECT id FROM t)\n) AS _mcp_limit LIMIT 100",
		},
		{
			"whitespace around semicolon",
			"SELECT 1 ;  ",
			100,
			"SELECT * FROM (\nSELECT 1\n) AS _mcp_limit LIMIT 100",
		},
		{
			// The closing paren and LIMIT must not land inside the trailing
			// line comment.
			"trailing line comment",
			"SELECT 1 -- note",
			100,
			"SELECT * FROM (\nSELECT 1 -- note\n) AS _mcp_limit LIMIT 100",
		},
		{
			"trailing block comment",
			"SELECT 1 /* note */",
			100,
			"SELECT * FROM (\nSELECT 1 /* note */\n) AS _mcp_limit LIMIT 100",
		},
		{
			"order by not propagated to outer query",
			"SELECT * FROM t ORDER BY created_at DESC",
			100,
			"SELECT * FROM (\nSELECT * FROM t ORDER BY created_at DESC\n) AS _mcp_limit LIMIT 100",
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
	if len(runner.queries) != 0 {
		t.Errorf("expected no queries, got %d", len(runner.queries))
	}
}

func TestPipelineExplainErrorRejectsStatement(t *testing.T) {
	// Statements that EXPLAIN doesn't support cause EXPLAIN to error. The
	// pipeline must reject them rather than letting raw SQL through. Only
	// statement types that genuinely fail EXPLAIN are listed here — DML like
	// INSERT/DROP succeed in EXPLAIN (returning a plan with non-read-only
	// ops) and are covered by TestPipelineExplainRejectsNonReadOnly.
	cases := []struct {
		sql    string
		errMsg string
	}{
		{"SHOW TABLES", "snowflake: EXPLAIN does not support SHOW"},
		{"DESCRIBE TABLE t", "snowflake: EXPLAIN does not support DESCRIBE"},
		{"LIST @mystage", "snowflake: EXPLAIN does not support LIST"},
	}
	for _, tc := range cases {
		t.Run(tc.sql, func(t *testing.T) {
			runner := &capturingQueryRunner{err: fmt.Errorf("%s", tc.errMsg)}
			result, err := executeSQLPipeline(context.Background(), runner, tc.sql, ExecutionModeReadonly)
			if err != nil {
				t.Fatalf("pipeline should not return error, got: %v", err)
			}
			text := extractTextFromResult(result)
			if !strings.Contains(text, "not supported") {
				t.Errorf("expected 'not supported' rejection, got: %s", text)
			}
			// Should have attempted EXPLAIN only.
			if len(runner.queries) != 1 {
				t.Fatalf("expected 1 query (EXPLAIN attempt), got %d", len(runner.queries))
			}
			if !strings.HasPrefix(runner.queries[0], "EXPLAIN USING TABULAR") {
				t.Errorf("query should be EXPLAIN-wrapped, got: %s", runner.queries[0])
			}
		})
	}
}

func TestPipelineExplainRejectsNonReadOnly(t *testing.T) {
	// DML that EXPLAIN *does* support — the plan reveals non-read-only ops.
	runner := &capturingQueryRunner{result: explainResult("Insert", "TableScan")}
	result, err := executeSQLPipeline(context.Background(), runner, "INSERT INTO t SELECT * FROM s", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "non-read-only") {
		t.Errorf("expected non-read-only rejection, got: %s", text)
	}
	// Should have called EXPLAIN only — not proceeded to execution.
	if len(runner.queries) != 1 {
		t.Fatalf("expected 1 query (EXPLAIN only), got %d", len(runner.queries))
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

func TestPipelineReadonlyRowCap(t *testing.T) {
	// Build a query result with more rows than maxMCPResultRows.
	rows := make([][]any, maxMCPResultRows+100)
	for i := range rows {
		rows[i] = []any{float64(i)}
	}
	runner := &sequencingQueryRunner{
		results: []*snowflake.QueryResult{
			explainResult("Result", "TableScan"),
			{Columns: []string{"id"}, Rows: rows},
		},
	}
	result, err := executeSQLPipeline(context.Background(), runner, "SELECT * FROM big_table", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
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

func TestPipelineUnsupportedMode(t *testing.T) {
	runner := &capturingQueryRunner{result: explainResult("Result", "TableScan")}
	result, err := executeSQLPipeline(context.Background(), runner, "SELECT 1", "metadata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "unsupported execution mode") {
		t.Errorf("expected unsupported mode rejection, got: %s", text)
	}
	// Should have called EXPLAIN (gate passed) but not executed the query.
	if len(runner.queries) != 1 {
		t.Fatalf("expected 1 query (EXPLAIN only), got %d", len(runner.queries))
	}
}

func TestPipelineQueryExecutionFailure(t *testing.T) {
	// EXPLAIN passes but the actual query fails (e.g. table dropped between
	// EXPLAIN and execution, permissions revoked, etc.). The pipeline should
	// return a structured rejection, not a raw Go error.
	runner := &sequencingQueryRunner{
		results: []*snowflake.QueryResult{
			explainResult("Result", "TableScan"),
			nil,
		},
		errors: []error{
			nil,
			fmt.Errorf("snowflake: table T does not exist"),
		},
	}
	result, err := executeSQLPipeline(context.Background(), runner, "SELECT * FROM t", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("pipeline should not return Go error, got: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "query execution failed") {
		t.Errorf("expected structured execution failure, got: %s", text)
	}
}

func TestPipelineCTEWithDelete(t *testing.T) {
	// WITH ... DELETE is a destructive statement that starts with WITH.
	// Keyword classification would wrongly treat this as a read query.
	// The EXPLAIN gate must catch it via the Delete operation in the plan.
	runner := &capturingQueryRunner{result: explainResult("WithClause", "Delete", "TableScan")}
	result, err := executeSQLPipeline(context.Background(), runner, "WITH target AS (SELECT id FROM t) DELETE FROM t WHERE id IN (SELECT id FROM target)", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextFromResult(result)
	if !strings.Contains(text, "non-read-only") {
		t.Errorf("expected non-read-only rejection for WITH...DELETE, got: %s", text)
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
	if i >= max(len(s.results), len(s.errors)) {
		panic(fmt.Sprintf("sequencingQueryRunner: no result/error for call %d (have %d results, %d errors)", i, len(s.results), len(s.errors)))
	}
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
