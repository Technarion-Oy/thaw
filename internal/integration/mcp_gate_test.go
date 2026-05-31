// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"thaw/internal/mcp"
)

// TestExplainGateSelectAllowed verifies that a simple SELECT passes the gate.
func TestExplainGateSelectAllowed(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v, err := mcp.CheckGate(ctx, client, "SELECT 1")
	if err != nil {
		t.Fatalf("CheckGate error: %v", err)
	}
	if !v.Allowed {
		t.Fatalf("expected allowed, got rejected: %s (ops: %v)", v.Reason, v.Operations)
	}
	if len(v.Operations) == 0 {
		t.Error("expected at least one operation in allowed plan")
	}
	for _, op := range v.Operations {
		if !mcp.IsReadOnlyOp(op) {
			t.Errorf("operation %q is not in readOnlyOps allow-list", op)
		}
	}
}

// TestExplainGateComplexSelectAllowed verifies that a more complex read-only
// query (GENERATOR) passes the gate.
func TestExplainGateComplexSelectAllowed(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v, err := mcp.CheckGate(ctx, client, "SELECT SEQ4() FROM TABLE(GENERATOR(ROWCOUNT=>10))")
	if err != nil {
		t.Fatalf("CheckGate error: %v", err)
	}
	if !v.Allowed {
		t.Fatalf("expected allowed, got rejected: %s (ops: %v)", v.Reason, v.Operations)
	}
}

// TestExplainGateMultiStatementRejected verifies multi-statement SQL is
// rejected before EXPLAIN is attempted.
func TestExplainGateMultiStatementRejected(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v, err := mcp.CheckGate(ctx, client, "SELECT 1; SELECT 2")
	if err != nil {
		t.Fatalf("CheckGate error: %v", err)
	}
	if v.Allowed {
		t.Fatal("expected rejection for multi-statement SQL")
	}
}

// TestExplainGateUSERejected verifies that USE statements are rejected.
func TestExplainGateUSERejected(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cases := []string{
		"USE ROLE PUBLIC",
		"USE WAREHOUSE COMPUTE_WH",
		"USE DATABASE SNOWFLAKE",
	}
	for _, sql := range cases {
		v, err := mcp.CheckGate(ctx, client, sql)
		if err != nil {
			t.Fatalf("CheckGate(%q) error: %v", sql, err)
		}
		if v.Allowed {
			t.Errorf("expected rejection for %q", sql)
		}
	}
}

// TestExplainGateInsertRejected verifies that INSERT (DML) is rejected.
// The EXPLAIN may succeed with non-read-only ops, or Snowflake may error
// for a non-existent table — either way the gate should not allow it.
func TestExplainGateInsertRejected(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a CTE-based INSERT that Snowflake can EXPLAIN without a real table.
	v, err := mcp.CheckGate(ctx, client, "INSERT INTO INFORMATION_SCHEMA.TABLES SELECT 1")
	if err != nil {
		// An EXPLAIN error (e.g. "table not found") is also an acceptable
		// outcome — the gate did not allow execution.
		t.Logf("CheckGate returned error (acceptable): %v", err)
		return
	}
	if v.Allowed {
		t.Fatalf("expected rejection for INSERT, got allowed (ops: %v)", v.Operations)
	}
}

// TestExplainGateDeleteRejected verifies that DELETE (DML) is rejected.
func TestExplainGateDeleteRejected(t *testing.T) {
	client := keyPairConnFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v, err := mcp.CheckGate(ctx, client, "DELETE FROM INFORMATION_SCHEMA.TABLES WHERE 1=0")
	if err != nil {
		t.Logf("CheckGate returned error (acceptable): %v", err)
		return
	}
	if v.Allowed {
		t.Fatalf("expected rejection for DELETE, got allowed (ops: %v)", v.Operations)
	}
}
