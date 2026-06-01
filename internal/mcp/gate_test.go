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
	"fmt"
	"testing"

	"thaw/internal/snowflake"
)

// fakeQueryRunner returns a canned QueryResult or error for any query.
type fakeQueryRunner struct {
	result *snowflake.QueryResult
	err    error
}

func (f *fakeQueryRunner) QuerySingle(_ context.Context, _ string) (*snowflake.QueryResult, error) {
	return f.result, f.err
}

// explainResult builds a QueryResult that mimics EXPLAIN USING TABULAR output
// with the given operation names.
func explainResult(ops ...string) *snowflake.QueryResult {
	rows := make([][]any, len(ops))
	for i, op := range ops {
		rows[i] = []any{"step", op, float64(i)}
	}
	return &snowflake.QueryResult{
		Columns: []string{"step", "operation", "id"},
		Rows:    rows,
	}
}

// ── SplitStatements pre-check ───────────────────────────────────────────────

func TestSingleStatementPreCheck(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantN   int // expected number of statements from SplitStatements
		allowed bool
	}{
		{"empty", "", 0, false},
		{"single", "SELECT 1", 1, true},
		{"trailing semicolon", "SELECT 1;", 1, true},
		{"multi-statement", "SELECT 1; SELECT 2", 2, false},
		{"semicolon in string", "SELECT 'a;b'", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeQueryRunner{result: explainResult("Result", "TableScan")}
			verdict, err := CheckGate(context.Background(), runner, tc.sql)
			if err != nil {
				t.Fatalf("CheckGate error: %v", err)
			}
			if verdict.Allowed != tc.allowed {
				t.Errorf("Allowed = %v, want %v (reason: %s)", verdict.Allowed, tc.allowed, verdict.Reason)
			}
		})
	}
}

// ── USE statement detection ─────────────────────────────────────────────────

func TestIsUSEStatement(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"USE ROLE SYSADMIN", true},
		{"use role public", true},
		{"USE WAREHOUSE COMPUTE_WH", true},
		{"USE DATABASE MYDB", true},
		{"USE SCHEMA PUBLIC", true},
		{"USE SECONDARY ROLES NONE", true},
		{"  USE ROLE FOO  ", true},
		// Leading comments should be stripped.
		{"/* bypass */ USE ROLE SYSADMIN", true},
		{"-- comment\nUSE ROLE SYSADMIN", true},
		{"/* a */ /* b */ USE ROLE X", true},
		{"-- line1\n-- line2\nUSE DATABASE DB", true},
		// Non-USE statements.
		{"SELECT 1", false},
		{"CREATE TABLE t (id INT)", false},
		{"USELESS_FUNC()", false},
		{"/* USE ROLE X */ SELECT 1", false},
	}
	for _, tc := range cases {
		t.Run(tc.sql, func(t *testing.T) {
			got := isUSEStatement(tc.sql)
			if got != tc.want {
				t.Errorf("isUSEStatement(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

// ── readOnlyOps default-deny ────────────────────────────────────────────────

func TestReadOnlyOpsDefaultDeny(t *testing.T) {
	allowed := []string{
		"Result", "Filter", "TableScan", "Join", "JoinFilter",
		"Aggregate", "Sort", "SortWithLimit", "Limit", "UnionAll",
		"WithClause", "WithReference", "Subquery",
		"InMemoryTableScan", "ValuesClause", "Generator", "Flatten",
		"ExternalScan", "WindowFunction", "Projection", "CartesianJoin",
		"SetOperation", "GroupingSets", "GlobalStats",
	}
	for _, op := range allowed {
		if !readOnlyOps[op] {
			t.Errorf("expected %q to be in readOnlyOps", op)
		}
	}
	denied := []string{
		"Insert", "Update", "Delete", "Merge", "CreateTable",
		"CreateView", "DropTable", "AlterTable", "Copy", "Put",
		"ExternalFunction",
	}
	for _, op := range denied {
		if readOnlyOps[op] {
			t.Errorf("expected %q to NOT be in readOnlyOps", op)
		}
	}
}

// ── CheckGate end-to-end scenarios ──────────────────────────────────────────

func TestCheckGateAllowed(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result", "TableScan", "Filter")}
	v, err := CheckGate(context.Background(), runner, "SELECT * FROM t WHERE x > 1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !v.Allowed {
		t.Fatalf("expected allowed, got rejected: %s", v.Reason)
	}
	if len(v.Operations) != 3 {
		t.Errorf("operations = %v, want 3 items", v.Operations)
	}
}

func TestCheckGateRejectDML(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Insert", "TableScan")}
	v, err := CheckGate(context.Background(), runner, "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for DML")
	}
	found := false
	for _, r := range v.Rejected {
		if r == "Insert" {
			found = true
		}
	}
	if !found {
		t.Errorf("Rejected = %v, want Insert in list", v.Rejected)
	}
}

func TestCheckGateRejectMultiStatement(t *testing.T) {
	// Runner should never be called for multi-statement SQL.
	runner := &fakeQueryRunner{result: explainResult("Result")}
	v, err := CheckGate(context.Background(), runner, "SELECT 1; SELECT 2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for multi-statement")
	}
	if v.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCheckGateRejectUSE(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result")}
	v, err := CheckGate(context.Background(), runner, "USE ROLE SYSADMIN")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for USE statement")
	}
}

func TestCheckGateRejectUSEWithComments(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result")}
	v, err := CheckGate(context.Background(), runner, "/* bypass */ USE ROLE SYSADMIN")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for USE statement with leading comment")
	}
}

func TestCheckGateEmptySQL(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result")}
	v, err := CheckGate(context.Background(), runner, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for empty SQL")
	}
}

func TestCheckGateUnknownOp(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result", "FUTURISTIC_OP")}
	v, err := CheckGate(context.Background(), runner, "SELECT 1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for unknown operation (default-deny)")
	}
	found := false
	for _, r := range v.Rejected {
		if r == "FUTURISTIC_OP" {
			found = true
		}
	}
	if !found {
		t.Errorf("Rejected = %v, want FUTURISTIC_OP in list", v.Rejected)
	}
}

func TestCheckGateRejectExternalFunction(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result", "ExternalFunction")}
	v, err := CheckGate(context.Background(), runner, "SELECT exfil_func(CURRENT_USER())")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for ExternalFunction (can invoke arbitrary HTTP endpoints)")
	}
}

func TestCheckGateExplainError(t *testing.T) {
	runner := &fakeQueryRunner{err: fmt.Errorf("snowflake: compilation error")}
	_, err := CheckGate(context.Background(), runner, "SELECT INVALID SYNTAX")
	if err == nil {
		t.Fatal("expected error propagation from EXPLAIN failure")
	}
}

// ── extractOperations edge cases ────────────────────────────────────────────

func TestExtractOperationsMissingColumn(t *testing.T) {
	result := &snowflake.QueryResult{
		Columns: []string{"step", "id"},
		Rows:    [][]any{{"step1", float64(0)}},
	}
	_, err := extractOperations(result)
	if err == nil {
		t.Fatal("expected error for missing 'operation' column")
	}
}

func TestExtractOperationsDedup(t *testing.T) {
	result := explainResult("TableScan", "Filter", "TableScan", "Result", "Filter")
	ops, err := extractOperations(result)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(ops) != 3 {
		t.Errorf("expected 3 unique ops, got %d: %v", len(ops), ops)
	}
	// Verify order preserved: TableScan, Filter, Result.
	want := []string{"TableScan", "Filter", "Result"}
	for i, w := range want {
		if i >= len(ops) || ops[i] != w {
			t.Errorf("ops[%d] = %q, want %q", i, ops[i], w)
		}
	}
}

// ── checkExplainPlan ────────────────────────────────────────────────────────

func TestCheckExplainPlanAllowed(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Result", "TableScan")}
	v, err := checkExplainPlan(context.Background(), runner, "SELECT 1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !v.Allowed {
		t.Fatalf("expected allowed, got rejected: %s", v.Reason)
	}
}

func TestCheckExplainPlanRejected(t *testing.T) {
	runner := &fakeQueryRunner{result: explainResult("Insert", "TableScan")}
	v, err := checkExplainPlan(context.Background(), runner, "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection")
	}
	found := false
	for _, r := range v.Rejected {
		if r == "Insert" {
			found = true
		}
	}
	if !found {
		t.Errorf("Rejected = %v, want Insert in list", v.Rejected)
	}
}

func TestCheckExplainPlanError(t *testing.T) {
	runner := &fakeQueryRunner{err: fmt.Errorf("snowflake: compilation error")}
	_, err := checkExplainPlan(context.Background(), runner, "SELECT BAD SYNTAX")
	if err == nil {
		t.Fatal("expected error propagation from EXPLAIN failure")
	}
}

// ── stripLeadingComments ────────────────────────────────────────────────────

func TestStripLeadingComments(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no comment", "SELECT 1", "SELECT 1"},
		{"line comment", "-- comment\nSELECT 1", "SELECT 1"},
		{"block comment", "/* comment */ SELECT 1", "SELECT 1"},
		{"multiple line comments", "-- a\n-- b\nSELECT 1", "SELECT 1"},
		{"nested block comments", "/* a */ /* b */ SELECT 1", "SELECT 1"},
		{"mixed", "-- line\n/* block */ SELECT 1", "SELECT 1"},
		{"only comment", "-- just a comment", ""},
		{"unclosed block", "/* unclosed", ""},
		{"comment then USE", "/* x */ USE ROLE FOO", "USE ROLE FOO"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripLeadingComments(tc.in)
			if got != tc.want {
				t.Errorf("stripLeadingComments(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
