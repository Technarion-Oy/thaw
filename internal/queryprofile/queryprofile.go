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
// JSON string parsing.
package queryprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// OperatorStat is one row returned by GET_QUERY_OPERATOR_STATS.
//
// The three JSON object columns (OperatorStatistics, ExecutionTimeBreakdown,
// OperatorAttributes) are parsed from the JSON strings the Snowflake driver
// returns and stored as Go values so they serialize as JSON objects (not
// strings) when sent to the frontend over the Wails IPC layer.
type OperatorStat struct {
	QueryID                string `json:"queryId"`
	StepID                 int64  `json:"stepId"`
	OperatorID             int64  `json:"operatorId"`
	ParentOperators        []int64 `json:"parentOperators"`
	OperatorType           string `json:"operatorType"`
	OperatorStatistics     any    `json:"operatorStatistics,omitempty"`
	ExecutionTimeBreakdown any    `json:"executionTimeBreakdown,omitempty"`
	OperatorAttributes     any    `json:"operatorAttributes,omitempty"`
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

// ── helpers ──────────────────────────────────────────────────────────────────

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
		fmt.Sscan(v, &n)
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
