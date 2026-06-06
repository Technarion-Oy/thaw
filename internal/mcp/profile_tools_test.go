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
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestProfileToolsRegistered verifies that both profiling tools are registered
// on a server built with a nil client (tool listing does not invoke handlers).
func TestProfileToolsRegistered(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	expected := []string{"explain_query", "get_explain_diagnostics"}
	for _, name := range expected {
		if !hasToolName(names, name) {
			t.Errorf("expected tool %q to be registered, got tools: %v", name, names)
		}
	}
}

// TestProfileToolsRegisteredInAllModes verifies that profiling tools are
// present in metadata, readonly, and explain_only modes.
func TestProfileToolsRegisteredInAllModes(t *testing.T) {
	profileTools := []string{"explain_query", "get_explain_diagnostics"}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range profileTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered", mode, tool)
				}
			}
		})
	}
}

// TestExplainQueryNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestExplainQueryNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "explain_query",
		Arguments: explainInput{SQL: "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestExplainQueryEmptySQL verifies the tool rejects empty SQL input.
func TestExplainQueryEmptySQL(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "explain_query",
		Arguments: explainInput{SQL: "   "},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty SQL")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "sql is required") {
		t.Errorf("error message should mention sql requirement, got: %s", text)
	}
}

// TestGetExplainDiagnosticsNilClient verifies the tool returns an error when
// no Snowflake client is available.
func TestGetExplainDiagnosticsNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_explain_diagnostics",
		Arguments: explainInput{SQL: "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetExplainDiagnosticsEmptySQL verifies the tool rejects empty SQL input.
func TestGetExplainDiagnosticsEmptySQL(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_explain_diagnostics",
		Arguments: explainInput{SQL: ""},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty SQL")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "sql is required") {
		t.Errorf("error message should mention sql requirement, got: %s", text)
	}
}
