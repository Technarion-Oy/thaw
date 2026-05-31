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
	"strings"

	"thaw/internal/snowflake"
)

// queryRunner is the minimal interface needed by the EXPLAIN gate. In
// production it is satisfied by *snowflake.Client; in tests a fake
// implementation returns canned QueryResult values.
type queryRunner interface {
	QuerySingle(ctx context.Context, query string) (*snowflake.QueryResult, error)
}

// GateVerdict is the result of the EXPLAIN precompilation gate.
type GateVerdict struct {
	Allowed    bool     `json:"allowed"`
	Operations []string `json:"operations"`
	Rejected   []string `json:"rejected,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// readOnlyOps is the default-allow set of EXPLAIN plan operations. Any
// operation not in this set causes the gate to reject the statement. The set
// is intentionally conservative — it is better to over-reject than to let a
// mutation through.
var readOnlyOps = map[string]bool{
	"Result":            true,
	"Filter":            true,
	"TableScan":         true,
	"Join":              true,
	"JoinFilter":        true,
	"Aggregate":         true,
	"GroupingSets":       true,
	"Sort":              true,
	"SortWithLimit":     true,
	"Limit":             true,
	"UnionAll":          true,
	"WithClause":        true,
	"WithReference":     true,
	"Subquery":          true,
	"ExternalFunction":  true,
	"InMemoryTableScan": true,
	"ValuesClause":      true,
	"Generator":         true,
	"Flatten":           true,
	"ExternalScan":      true,
	"WindowFunction":    true,
	"Projection":        true,
	"CartesianJoin":     true,
	"SetOperation":      true,
	"GlobalStats":       true,
}

// isUSEStatement returns true if sql is a USE ROLE/WAREHOUSE/DATABASE/SCHEMA
// or USE SECONDARY ROLES statement. These are rejected by the gate because
// they can change session context — context-switching is exposed through
// dedicated trusted tools instead.
func isUSEStatement(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "USE ")
}

// CheckGate runs the three-layer EXPLAIN precompilation gate:
//  1. SplitStatements must return exactly 1 statement (reject multi-stmt).
//  2. Reject USE statements (context-switching via dedicated tools).
//  3. EXPLAIN USING TABULAR and verify all operations are in readOnlyOps.
func CheckGate(ctx context.Context, runner queryRunner, sql string) (GateVerdict, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return GateVerdict{Reason: "empty SQL"}, nil
	}

	// Layer 1: single-statement check.
	stmts := snowflake.SplitStatements(trimmed)
	if len(stmts) != 1 {
		return GateVerdict{
			Reason: fmt.Sprintf("multi-statement SQL not allowed (%d statements)", len(stmts)),
		}, nil
	}
	stmt := strings.TrimSpace(stmts[0])

	// Layer 2: reject USE statements.
	if isUSEStatement(stmt) {
		return GateVerdict{
			Reason: "USE statements are not allowed; use the dedicated context-switching tools",
		}, nil
	}

	// Layer 3: EXPLAIN USING TABULAR.
	result, err := runner.QuerySingle(ctx, "EXPLAIN USING TABULAR "+stmt)
	if err != nil {
		return GateVerdict{}, fmt.Errorf("EXPLAIN gate: %w", err)
	}

	ops, err := extractOperations(result)
	if err != nil {
		return GateVerdict{}, err
	}

	var rejected []string
	for _, op := range ops {
		if !readOnlyOps[op] {
			rejected = append(rejected, op)
		}
	}

	if len(rejected) > 0 {
		return GateVerdict{
			Operations: ops,
			Rejected:   rejected,
			Reason:     fmt.Sprintf("statement contains non-read-only operations: %s", strings.Join(rejected, ", ")),
		}, nil
	}

	return GateVerdict{
		Allowed:    true,
		Operations: ops,
	}, nil
}

// extractOperations scans the "operation" column of an EXPLAIN result and
// returns deduplicated operation names in encounter order.
func extractOperations(result *snowflake.QueryResult) ([]string, error) {
	opIdx := -1
	for i, col := range result.Columns {
		if strings.EqualFold(col, "operation") {
			opIdx = i
			break
		}
	}
	if opIdx < 0 {
		return nil, fmt.Errorf("EXPLAIN result has no 'operation' column (columns: %v)", result.Columns)
	}

	seen := make(map[string]bool)
	var ops []string
	for _, row := range result.Rows {
		if opIdx >= len(row) {
			continue
		}
		op, ok := row[opIdx].(string)
		if !ok || op == "" {
			continue
		}
		if !seen[op] {
			seen[op] = true
			ops = append(ops, op)
		}
	}
	return ops, nil
}
