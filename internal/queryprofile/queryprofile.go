// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package queryprofile fetches and parses Snowflake query execution profiles.
//
// It wraps the GET_QUERY_OPERATOR_STATS table function and returns typed
// [OperatorStat] rows so callers never have to deal with raw column indices or
// JSON string parsing. It also provides tools to parse EXPLAIN USING JSON outputs.
//
// thaw:domain: SQL Editor & Diagnostics
package queryprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ── Diagnostic Messages ──────────────────────────────────────────────────────

// DiagCode represents a unique identifier for a diagnostic message.
type DiagCode string

const (
	CodeFullTableScan DiagCode = "FULL_TABLE_SCAN"
	CodeCartesianJoin DiagCode = "CARTESIAN_JOIN"
	CodeRowExplosion  DiagCode = "ROW_EXPLOSION"
)

// diagMessageTemplates holds the raw string templates for all diagnostics.
var diagMessageTemplates = map[DiagCode]string{
	CodeFullTableScan: "Full Table Scan: %s\nScanning %d partitions. Consider adding a filter.",
	CodeCartesianJoin: "Cartesian Join Detected: This will multiply rows exponentially.",
	CodeRowExplosion:  "Row Explosion Warning: This join is estimated to produce %d rows. Verify your ON conditions are selective enough.",
}

// GetDiagMessage retrieves the message template for the given code and formats
// it with any optional arguments provided.
func GetDiagMessage(code DiagCode, args ...any) string {
	template, exists := diagMessageTemplates[code]
	if !exists {
		// Fallback for missing definitions during development
		return fmt.Sprintf("Unknown diagnostic code: %s", string(code))
	}

	// If arguments are provided, format the string; otherwise return it raw.
	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}
	return template
}

// ── Query Operator Stats ─────────────────────────────────────────────────────

// OperatorStat is one row returned by GET_QUERY_OPERATOR_STATS.
//
// The three JSON object columns (OperatorStatistics, ExecutionTimeBreakdown,
// OperatorAttributes) are parsed from the JSON strings the Snowflake driver
// returns and stored as Go values so they serialize as JSON objects (not
// strings) when sent to the frontend over the Wails IPC layer.
type OperatorStat struct {
	QueryID                string  `json:"queryId"`
	StepID                 int64   `json:"stepId"`
	OperatorID             int64   `json:"operatorId"`
	ParentOperators        []int64 `json:"parentOperators"`
	OperatorType           string  `json:"operatorType"`
	OperatorStatistics     any     `json:"operatorStatistics,omitempty"`
	ExecutionTimeBreakdown any     `json:"executionTimeBreakdown,omitempty"`
	OperatorAttributes     any     `json:"operatorAttributes,omitempty"`
}

// GetOperatorStats runs GET_QUERY_OPERATOR_STATS for the given Snowflake
// query ID and returns the parsed rows.
//
// queryID must contain only hex characters and hyphens (the standard
// Snowflake query-ID format).  An error is returned immediately if the
// value contains characters that could be used in a SQL injection attack.
func GetOperatorStats(ctx context.Context, client *snowflake.Client, queryID string) ([]OperatorStat, error) {
	if !isValidQueryID(queryID) {
		return nil, fmt.Errorf("queryprofile: invalid query ID %q", queryID)
	}
	result, err := client.Execute(ctx,
		"SELECT * FROM TABLE(GET_QUERY_OPERATOR_STATS('"+queryID+"'))",
	)
	if err != nil {
		return nil, err
	}
	return parseResult(result)
}

// ── Explain Plan ─────────────────────────────────────────────────────────────

// ExplainPlan represents the parsed JSON output from Snowflake's EXPLAIN command.
type ExplainPlan struct {
	GlobalStats ExplainGlobalStats `json:"GlobalStats"`
	// Operations is a 2D array. The outer array represents execution steps,
	// and the inner array represents the flat list of nodes in that step.
	Operations [][]ExplainNode `json:"Operations"`
}

// ExplainGlobalStats contains the top-level execution estimates.
type ExplainGlobalStats struct {
	PartitionsTotal   int64 `json:"partitionsTotal"`
	PartitionsScanned int64 `json:"partitionsScanned"`
	BytesAssigned     int64 `json:"bytesAssigned"`
}

// ExplainNode represents a single logical operation in the query plan.
type ExplainNode struct {
	ID                int64    `json:"id"`
	Parent            *int64   `json:"parent,omitempty"`  // Pointer because root nodes have null parents
	Operation         string   `json:"operation"`         // e.g., "TableScan", "Join"
	Objects           []string `json:"objects,omitempty"` // e.g., ["MY_DB.MY_SCHEMA.SALES_DATA"]
	PartitionsScanned int64    `json:"partitionsScanned,omitempty"`
	PartitionsTotal   int64    `json:"partitionsTotal,omitempty"`
	JoinType          string   `json:"joinType,omitempty"`    // e.g., "Inner", "Cartesian"
	EstimatedRows     int64    `json:"estimatedRows,omitempty"` // Snowflake compiler row estimate
}

// GetExplainPlan runs EXPLAIN USING JSON for the provided SQL query
// and parses the result into a typed ExplainPlan.
func GetExplainPlan(ctx context.Context, client *snowflake.Client, query string) (*ExplainPlan, error) {
	// 1. Wrap the raw query
	explainQuery := "EXPLAIN USING JSON " + query

	// 2. Execute against Snowflake
	result, err := client.Execute(ctx, explainQuery)
	if err != nil {
		// If the SQL is invalid, Snowflake returns an error here
		return nil, err
	}
	return parseExplainResult(result)
}

// GetExplainPlanOnConn is the same as GetExplainPlan but runs on a pinned connection.
func GetExplainPlanOnConn(ctx context.Context, client *snowflake.Client, conn *sql.Conn, query string) (*ExplainPlan, error) {
	// 1. Wrap the raw query
	explainQuery := "EXPLAIN USING JSON " + query

	// 2. Execute against Snowflake on the pinned connection
	result, err := client.ExecuteOnConn(ctx, conn, explainQuery)
	if err != nil {
		return nil, err
	}
	return parseExplainResult(result)
}

func parseExplainResult(result *snowflake.QueryResult) (*ExplainPlan, error) {
	// 3. Extract the JSON string from the result
	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("queryprofile: explain returned no rows")
	}

	row := result.Rows[0]
	if len(row) == 0 {
		return nil, fmt.Errorf("queryprofile: explain returned empty row")
	}

	// EXPLAIN USING JSON always returns the JSON string in the first column
	jsonStr := asString(row, 0)
	if jsonStr == "" {
		return nil, fmt.Errorf("queryprofile: explain returned empty JSON string")
	}

	// 4. Unmarshal into our typed structs
	var plan ExplainPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("queryprofile: failed to parse explain JSON: %w", err)
	}

	return &plan, nil
}

// ── Explain Diagnostics ──────────────────────────────────────────────────────

// ExplainData carries structured data from EXPLAIN USING JSON for rich
// hover tooltips in the editor. Fields mirror the TypeScript spec interface.
type ExplainData struct {
	Operation         string `json:"operation"`
	ObjectName        string `json:"objectName,omitempty"`
	BytesAssigned     int64  `json:"bytesAssigned,omitempty"`
	PartitionsScanned int64  `json:"partitionsScanned,omitempty"`
	PartitionsTotal   int64  `json:"partitionsTotal,omitempty"`
	JoinType          string `json:"joinType,omitempty"`
	EstimatedRows     int64  `json:"estimatedRows,omitempty"`
}

// ExplainMarker is a Monaco editor marker (DiagMarker-compatible) enriched
// with optional structured EXPLAIN data for the tooltip hover provider.
type ExplainMarker struct {
	StartLineNumber int          `json:"startLineNumber"`
	StartColumn     int          `json:"startColumn"`
	EndLineNumber   int          `json:"endLineNumber"`
	EndColumn       int          `json:"endColumn"`
	Message         string       `json:"message"`
	Severity        int          `json:"severity"` // 8 = Error, 4 = Warning, 2 = Info
	ExplainData     *ExplainData `json:"explainData,omitempty"`
}

// ExplainResult combines the full plan tree and performance diagnostics so
// the front-end Explain modal can display everything from a single IPC call.
type ExplainResult struct {
	Plan        *ExplainPlan    `json:"plan"`
	Diagnostics []ExplainMarker `json:"diagnostics"`
}

// RunExplain runs EXPLAIN USING JSON for sql and returns both the parsed plan
// tree (for display) and any detected performance issues (for highlighting).
func RunExplain(ctx context.Context, client *snowflake.Client, sql string) (*ExplainResult, error) {
	plan, err := GetExplainPlan(ctx, client, sql)
	if err != nil {
		return nil, err
	}
	return &ExplainResult{
		Plan:        plan,
		Diagnostics: analyzePlan(plan, sql),
	}, nil
}

// RunExplainOnConn is the same as RunExplain but runs on a pinned connection.
func RunExplainOnConn(ctx context.Context, client *snowflake.Client, conn *sql.Conn, sql string) (*ExplainResult, error) {
	plan, err := GetExplainPlanOnConn(ctx, client, conn, sql)
	if err != nil {
		return nil, err
	}
	return &ExplainResult{
		Plan:        plan,
		Diagnostics: analyzePlan(plan, sql),
	}, nil
}

// GetExplainDiagnostics runs EXPLAIN USING JSON for sql and walks the
// resulting plan to emit performance markers (full table scans, cartesian
// joins).  Returns nil (not an error) when no issues are found.
func GetExplainDiagnostics(ctx context.Context, client *snowflake.Client, sql string) ([]ExplainMarker, error) {
	plan, err := GetExplainPlan(ctx, client, sql)
	if err != nil {
		return nil, err
	}
	return analyzePlan(plan, sql), nil
}

// analyzePlan walks plan nodes and emits performance markers.
// Extracted so RunExplain and GetExplainDiagnostics share the analysis
// without calling EXPLAIN twice.
// analyzePlan walks plan nodes and emits performance markers.
func analyzePlan(plan *ExplainPlan, sql string) []ExplainMarker {
	var markers []ExplainMarker
	for _, step := range plan.Operations {
		for _, node := range step {
			// ── Full table scan ──────────────────────────────────────────
			// ── Full table scan ──────────────────────────────────────────
			if isTableScanOp(node.Operation) && node.PartitionsTotal >= 10 {

				// Trust the math: only warn if it EXPLICITLY plans to scan >= 50%
				pct := float64(node.PartitionsScanned) / float64(node.PartitionsTotal)

				if pct >= 0.5 {
					objName := ""
					if len(node.Objects) > 0 {
						objName = node.Objects[0]
					}
					shortName := lastPart(objName)
					sl, sc, el, ec := findTokenPos(sql, shortName)

					severity := 4 // Warning
					if pct >= 0.9 {
						severity = 8 // Error
					}

					markers = append(markers, ExplainMarker{
						StartLineNumber: sl, StartColumn: sc,
						EndLineNumber: el, EndColumn: ec,
						Message:  GetDiagMessage(CodeFullTableScan, shortName, node.PartitionsScanned),
						Severity: severity,
						ExplainData: &ExplainData{
							Operation:         node.Operation,
							ObjectName:        objName,
							BytesAssigned:     plan.GlobalStats.BytesAssigned,
							PartitionsScanned: node.PartitionsScanned,
							PartitionsTotal:   node.PartitionsTotal,
						},
					})
				}
			}

			// ── Cartesian join ───────────────────────────────────────────
			opUpper := strings.ToUpper(node.Operation)
			jtUpper := strings.ToUpper(node.JoinType)

			// Catch any variation of Cartesian or Cross joins in either field
			isCartesian := strings.Contains(opUpper, "CARTESIAN") || strings.Contains(opUpper, "CROSS") ||
				strings.Contains(jtUpper, "CARTESIAN") || strings.Contains(jtUpper, "CROSS")

			if isCartesian {
				// Try to attach the red squiggly to a relevant keyword
				sl, sc, el, ec := findTokenPos(sql, "JOIN")

				if sl == 1 && sc == 1 && ec == 9999 { // Fallback hit, try CROSS
					sl, sc, el, ec = findTokenPos(sql, "CROSS")
				}
				if sl == 1 && sc == 1 && ec == 9999 { // Fallback hit, try FROM (for comma joins)
					sl, sc, el, ec = findTokenPos(sql, "FROM")
				}

				markers = append(markers, ExplainMarker{
					StartLineNumber: sl,
					StartColumn:     sc,
					EndLineNumber:   el,
					EndColumn:       ec,
					Message:         GetDiagMessage(CodeCartesianJoin),
					Severity:        8,
					ExplainData: &ExplainData{
						Operation: node.Operation,
						JoinType:  node.JoinType,
					},
				})
			}

			// ── Row explosion (Many-to-Many equi-join) ───────────────────
			// Catches equi-joins on low-cardinality columns where Snowflake
			// plans an InnerJoin (not Cartesian) but the output is enormous.
			// Only fire when the node isn't already flagged as Cartesian to
			// avoid duplicate markers on the same JOIN keyword.
			const rowExplosionThreshold = 10_000_000
			if !isCartesian &&
				strings.Contains(opUpper, "JOIN") &&
				node.EstimatedRows > rowExplosionThreshold {

				sl, sc, el, ec := findTokenPos(sql, "JOIN")
				severity := 4 // Warning
				if node.EstimatedRows > 1_000_000_000 {
					severity = 8 // Error for truly catastrophic estimates
				}
				markers = append(markers, ExplainMarker{
					StartLineNumber: sl, StartColumn: sc,
					EndLineNumber:   el, EndColumn: ec,
					Message:  GetDiagMessage(CodeRowExplosion, node.EstimatedRows),
					Severity: severity,
					ExplainData: &ExplainData{
						Operation:     node.Operation,
						JoinType:      node.JoinType,
						EstimatedRows: node.EstimatedRows,
					},
				})
			}
		}
	}
	return markers
}

// isTableScanOp returns true for plan operations that represent a table scan.
func isTableScanOp(op string) bool {
	up := strings.ToUpper(op)
	return up == "TABLESCAN" || up == "INMEMTABLESCAN" || strings.HasSuffix(up, "SCAN")
}

// lastPart extracts the table name from a fully-qualified dotted identifier,
// stripping surrounding double-quotes.
// "MY_DB.MY_SCHEMA.MY_TABLE" → "MY_TABLE"
func lastPart(dotted string) string {
	parts := strings.Split(dotted, ".")
	return strings.Trim(parts[len(parts)-1], `"`)
}

// findTokenPos searches for token (case-insensitive) in sql and returns the
// 1-based line/column range of the first occurrence.  Falls back to (1,1,1,9999)
// when the token is empty or not found.
func findTokenPos(sql, token string) (sl, sc, el, ec int) {
	if token == "" {
		return 1, 1, 1, 9999
	}
	tokenUpper := strings.ToUpper(token)
	line := 1
	lineStart := 0
	sqlUpper := strings.ToUpper(sql)
	for i := range sql {
		if i+len(tokenUpper) <= len(sqlUpper) && sqlUpper[i:i+len(tokenUpper)] == tokenUpper {
			col := i - lineStart + 1
			return line, col, line, col + len(token)
		}
		if sql[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return 1, 1, 1, 9999
}

// ── helpers ──────────────────────────────────────────────────────────────────

// isValidQueryID returns true when s consists entirely of hex digits and
// hyphens — the standard format Snowflake uses for query IDs.
func isValidQueryID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-') {
			return false
		}
	}
	return true
}

// parseResult maps a raw QueryResult into typed OperatorStat rows.
func parseResult(result *snowflake.QueryResult) ([]OperatorStat, error) {
	idx := make(map[string]int, len(result.Columns))
	for i, c := range result.Columns {
		idx[strings.ToUpper(c)] = i
	}

	stats := make([]OperatorStat, 0, len(result.Rows))
	for _, row := range result.Rows {
		s := OperatorStat{
			ParentOperators: []int64{},
		}
		if i, ok := idx["QUERY_ID"]; ok {
			s.QueryID = asString(row, i)
		}
		if i, ok := idx["STEP_ID"]; ok {
			s.StepID = asInt64(row, i)
		}
		if i, ok := idx["OPERATOR_ID"]; ok {
			s.OperatorID = asInt64(row, i)
		}
		if i, ok := idx["PARENT_OPERATORS"]; ok {
			s.ParentOperators = asInt64Slice(asString(row, i))
		}
		if i, ok := idx["OPERATOR_TYPE"]; ok {
			s.OperatorType = asString(row, i)
		}
		if i, ok := idx["OPERATOR_STATISTICS"]; ok {
			s.OperatorStatistics = asJSONValue(asString(row, i))
		}
		if i, ok := idx["EXECUTION_TIME_BREAKDOWN"]; ok {
			s.ExecutionTimeBreakdown = asJSONValue(asString(row, i))
		}
		if i, ok := idx["OPERATOR_ATTRIBUTES"]; ok {
			s.OperatorAttributes = asJSONValue(asString(row, i))
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func asString(row []any, i int) string {
	if i >= len(row) || row[i] == nil {
		return ""
	}
	return fmt.Sprintf("%v", row[i])
}

func asInt64(row []any, i int) int64 {
	if i >= len(row) || row[i] == nil {
		return 0
	}
	switch v := row[i].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		var n int64
		_, _ = fmt.Sscan(v, &n)
		return n
	default:
		return 0
	}
}

// asInt64Slice parses a JSON array of numbers (e.g. "[1,2,3]") into []int64.
// Snowflake may return the numbers as floats, so both integer and float arrays
// are handled.
func asInt64Slice(s string) []int64 {
	if s == "" {
		return []int64{}
	}
	var ints []int64
	if err := json.Unmarshal([]byte(s), &ints); err == nil {
		return ints
	}
	var floats []float64
	if err := json.Unmarshal([]byte(s), &floats); err == nil {
		out := make([]int64, len(floats))
		for i, f := range floats {
			out[i] = int64(f)
		}
		return out
	}
	return []int64{}
}

// asJSONValue parses s as JSON and returns the decoded Go value (map, slice,
// number, string, bool, or nil).  Returns nil for empty or invalid JSON.
func asJSONValue(s string) any {
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}
