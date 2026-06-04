// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package queryprofile

import (
	"testing"

	"thaw/internal/snowflake"
)

// ── parseTabularExplainResult ────────────────────────────────────────────────

func TestParseTabularExplainResult_Valid(t *testing.T) {
	result := &snowflake.QueryResult{
		Columns: []string{"id", "parentOperators", "operation", "objects", "partitionsTotal", "partitionsAssigned", "bytesAssigned"},
		Rows: [][]any{
			{int64(0), "[]", "TableScan", "DB.SCHEMA.MY_TABLE", int64(100), int64(100), int64(0)},
			{int64(1), "[0]", "Filter", "", int64(0), int64(0), int64(0)},
			{int64(0), "", "GlobalStats", "", int64(100), int64(100), int64(5000)},
		},
	}

	plan, err := parseTabularExplainResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GlobalStats should be extracted.
	if plan.GlobalStats.PartitionsTotal != 100 {
		t.Errorf("GlobalStats.PartitionsTotal = %d, want 100", plan.GlobalStats.PartitionsTotal)
	}
	if plan.GlobalStats.PartitionsScanned != 100 {
		t.Errorf("GlobalStats.PartitionsScanned = %d, want 100", plan.GlobalStats.PartitionsScanned)
	}
	if plan.GlobalStats.BytesAssigned != 5000 {
		t.Errorf("GlobalStats.BytesAssigned = %d, want 5000", plan.GlobalStats.BytesAssigned)
	}

	// Should have exactly 1 step with 2 nodes (GlobalStats excluded).
	if len(plan.Operations) != 1 {
		t.Fatalf("len(Operations) = %d, want 1", len(plan.Operations))
	}
	nodes := plan.Operations[0]
	if len(nodes) != 2 {
		t.Fatalf("len(Operations[0]) = %d, want 2", len(nodes))
	}

	// First node: TableScan.
	n0 := nodes[0]
	if n0.ID != 0 {
		t.Errorf("node[0].ID = %d, want 0", n0.ID)
	}
	if n0.Operation != "TableScan" {
		t.Errorf("node[0].Operation = %q, want %q", n0.Operation, "TableScan")
	}
	if n0.Parent != nil {
		t.Errorf("node[0].Parent = %v, want nil", n0.Parent)
	}
	if len(n0.Objects) != 1 || n0.Objects[0] != "DB.SCHEMA.MY_TABLE" {
		t.Errorf("node[0].Objects = %v, want [DB.SCHEMA.MY_TABLE]", n0.Objects)
	}
	if n0.PartitionsTotal != 100 {
		t.Errorf("node[0].PartitionsTotal = %d, want 100", n0.PartitionsTotal)
	}
	if n0.PartitionsScanned != 100 {
		t.Errorf("node[0].PartitionsScanned = %d, want 100", n0.PartitionsScanned)
	}

	// Second node: Filter with parent 0.
	n1 := nodes[1]
	if n1.ID != 1 {
		t.Errorf("node[1].ID = %d, want 1", n1.ID)
	}
	if n1.Parent == nil || *n1.Parent != 0 {
		t.Errorf("node[1].Parent = %v, want 0", n1.Parent)
	}

	// JoinType and EstimatedRows must be zero-valued (not available in TABULAR).
	if n0.JoinType != "" {
		t.Errorf("node[0].JoinType = %q, want empty", n0.JoinType)
	}
	if n0.EstimatedRows != 0 {
		t.Errorf("node[0].EstimatedRows = %d, want 0", n0.EstimatedRows)
	}
}

func TestParseTabularExplainResult_MissingOperationColumn(t *testing.T) {
	result := &snowflake.QueryResult{
		Columns: []string{"id", "objects"},
		Rows:    [][]any{{int64(0), "TABLE_A"}},
	}
	_, err := parseTabularExplainResult(result)
	if err == nil {
		t.Fatal("expected error for missing operation column, got nil")
	}
}

func TestParseTabularExplainResult_EmptyRows(t *testing.T) {
	result := &snowflake.QueryResult{
		Columns: []string{"id", "operation"},
		Rows:    [][]any{},
	}
	_, err := parseTabularExplainResult(result)
	if err == nil {
		t.Fatal("expected error for empty rows, got nil")
	}
}

// ── analyzePlan with TABULAR-sourced plan ────────────────────────────────────

func TestAnalyzePlan_TabularFullScan(t *testing.T) {
	plan := &ExplainPlan{
		GlobalStats: ExplainGlobalStats{
			PartitionsTotal:   200,
			PartitionsScanned: 200,
			BytesAssigned:     10000,
		},
		Operations: [][]ExplainNode{{
			{
				ID:                0,
				Operation:         "TableScan",
				Objects:           []string{"DB.SCHEMA.SALES"},
				PartitionsScanned: 200,
				PartitionsTotal:   200,
			},
		}},
	}

	markers := analyzePlan(plan, "SELECT * FROM SALES")
	if len(markers) == 0 {
		t.Fatal("expected full-table-scan marker, got none")
	}
	found := false
	for _, m := range markers {
		if m.Severity == 8 { // >= 90% scan → Error
			found = true
		}
	}
	if !found {
		t.Error("expected Error severity for 100% partition scan")
	}
}

func TestAnalyzePlan_TabularCartesianByOperationName(t *testing.T) {
	plan := &ExplainPlan{
		Operations: [][]ExplainNode{{
			{
				ID:        0,
				Operation: "CartesianJoin",
				// JoinType is empty (TABULAR source) — detection by operation name still works.
			},
		}},
	}

	markers := analyzePlan(plan, "SELECT * FROM a CROSS JOIN b")
	if len(markers) == 0 {
		t.Fatal("expected cartesian-join marker, got none")
	}
	if markers[0].Severity != 8 {
		t.Errorf("severity = %d, want 8", markers[0].Severity)
	}
}

func TestAnalyzePlan_TabularNoJoinTypeCartesian(t *testing.T) {
	// A regular join with JoinType="" (TABULAR) should NOT trigger cartesian.
	plan := &ExplainPlan{
		Operations: [][]ExplainNode{{
			{
				ID:        0,
				Operation: "Join",
				JoinType:  "", // Not available in TABULAR
			},
		}},
	}

	markers := analyzePlan(plan, "SELECT * FROM a JOIN b ON a.id = b.id")
	for _, m := range markers {
		if m.Message == GetDiagMessage(CodeCartesianJoin) {
			t.Error("should not detect cartesian join when JoinType is empty and operation is 'Join'")
		}
	}
}

func TestAnalyzePlan_TabularNoRowExplosion(t *testing.T) {
	// EstimatedRows=0 (TABULAR) means row explosion detection cannot fire.
	plan := &ExplainPlan{
		Operations: [][]ExplainNode{{
			{
				ID:            0,
				Operation:     "Join",
				EstimatedRows: 0, // Not available in TABULAR
			},
		}},
	}

	markers := analyzePlan(plan, "SELECT * FROM a JOIN b ON a.id = b.id")
	for _, m := range markers {
		if m.Severity == 4 || m.Severity == 8 {
			// Row explosion would produce a warning or error
			if m.ExplainData != nil && m.ExplainData.EstimatedRows > 0 {
				t.Error("row explosion should not fire when EstimatedRows is 0")
			}
		}
	}
}

// ── Helper function tests ────────────────────────────────────────────────────

func TestFindTokenPos(t *testing.T) {
	tests := []struct {
		sql   string
		token string
		sl    int
		sc    int
		el    int
		ec    int
	}{
		{"SELECT * FROM users", "FROM", 1, 10, 1, 14},
		{"SELECT *\nFROM users", "FROM", 2, 1, 2, 5},
		{"SELECT * FROM users", "MISSING", 1, 1, 1, 9999},
		{"SELECT * FROM users", "", 1, 1, 1, 9999},
		{"select * from users", "FROM", 1, 10, 1, 14}, // case-insensitive
	}
	for _, tt := range tests {
		sl, sc, el, ec := findTokenPos(tt.sql, tt.token)
		if sl != tt.sl || sc != tt.sc || el != tt.el || ec != tt.ec {
			t.Errorf("findTokenPos(%q, %q) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				tt.sql, tt.token, sl, sc, el, ec, tt.sl, tt.sc, tt.el, tt.ec)
		}
	}
}

func TestIsTableScanOp(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"TableScan", true},
		{"TABLESCAN", true},
		{"InMemTableScan", true}, // HasSuffix("SCAN") matches
		{"INMEMTABLESCAN", true},
		{"ExternalScan", true},
		{"Filter", false},
		{"Join", false},
	}
	for _, tt := range tests {
		got := isTableScanOp(tt.op)
		if got != tt.want {
			t.Errorf("isTableScanOp(%q) = %v, want %v", tt.op, got, tt.want)
		}
	}
}

func TestLastPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MY_DB.MY_SCHEMA.MY_TABLE", "MY_TABLE"},
		{`MY_DB.MY_SCHEMA."My Table"`, "My Table"},
		{"SIMPLE_TABLE", "SIMPLE_TABLE"},
		{"", ""},
	}
	for _, tt := range tests {
		got := lastPart(tt.input)
		if got != tt.want {
			t.Errorf("lastPart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetDiagMessage(t *testing.T) {
	msg := GetDiagMessage(CodeFullTableScan, "SALES", 100)
	if msg == "" {
		t.Error("expected non-empty message")
	}

	msg = GetDiagMessage(CodeCartesianJoin)
	if msg == "" {
		t.Error("expected non-empty message for cartesian join")
	}

	msg = GetDiagMessage("UNKNOWN_CODE")
	if msg == "" {
		t.Error("expected fallback message for unknown code")
	}
}
