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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestGetCurrentEditorSQLEmpty verifies the tool returns a graceful message
// when no SQL is in the active editor.
func TestGetCurrentEditorSQLEmpty(t *testing.T) {
	store := NewEditorContextStore()
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_current_editor_sql",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if !strings.Contains(text, "No SQL") {
		t.Errorf("expected 'No SQL' message, got: %s", text)
	}
}

// TestGetCurrentEditorSQLReturnsContent verifies the tool returns the active
// editor SQL when it is set.
func TestGetCurrentEditorSQLReturnsContent(t *testing.T) {
	store := NewEditorContextStore()
	store.SetActiveTab("tab1", "SELECT * FROM users")
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_current_editor_sql",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if text != "SELECT * FROM users" {
		t.Errorf("expected editor SQL, got: %s", text)
	}
}

// TestGetQueryResultsSummaryEmpty verifies the tool returns a graceful message
// when no results are available.
func TestGetQueryResultsSummaryEmpty(t *testing.T) {
	store := NewEditorContextStore()
	store.SetActiveTab("tab1", "SELECT 1")
	srv := buildServer(nil, ExecutionModeReadonly, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_query_results_summary",
		Arguments: resultSummaryInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if !strings.Contains(text, "No query results") {
		t.Errorf("expected 'No query results' message, got: %s", text)
	}
}

// TestGetQueryResultsSummaryReturnsData verifies the tool returns result data
// when available.
func TestGetQueryResultsSummaryReturnsData(t *testing.T) {
	store := NewEditorContextStore()
	store.SetActiveTab("tab1", "SELECT 1")
	store.SetTabResult("tab1", &ResultSummary{
		TabID:      "tab1",
		Columns:    []string{"ID", "NAME"},
		RowCount:   100,
		Truncated:  true,
		SampleRows: [][]any{{1, "Alice"}, {2, "Bob"}},
		QueryID:    "qid-abc",
	})
	srv := buildServer(nil, ExecutionModeReadonly, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_query_results_summary",
		Arguments: resultSummaryInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if !strings.Contains(text, "qid-abc") {
		t.Errorf("expected query ID in result, got: %s", text)
	}
	if !strings.Contains(text, "Alice") {
		t.Errorf("expected sample data in result, got: %s", text)
	}
}

// TestGetQueryResultsSummaryExplicitTabID verifies the tool returns results
// for a non-active tab when an explicit tabId is provided.
func TestGetQueryResultsSummaryExplicitTabID(t *testing.T) {
	store := NewEditorContextStore()
	store.SetActiveTab("tab1", "SELECT 1")
	store.SetTabResult("tab2", &ResultSummary{
		TabID:      "tab2",
		Columns:    []string{"COL_A"},
		RowCount:   42,
		SampleRows: [][]any{{"hello"}},
		QueryID:    "qid-tab2",
	})
	srv := buildServer(nil, ExecutionModeReadonly, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_query_results_summary",
		Arguments: resultSummaryInput{TabID: "tab2"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if !strings.Contains(text, "qid-tab2") {
		t.Errorf("expected query ID qid-tab2 in result, got: %s", text)
	}
	if !strings.Contains(text, "hello") {
		t.Errorf("expected sample data in result, got: %s", text)
	}
}

// TestGetQueryResultsSummaryModeGating verifies get_query_results_summary is
// NOT registered in metadata mode (it exposes data rows).
func TestGetQueryResultsSummaryModeGating(t *testing.T) {
	store := NewEditorContextStore()
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, store, nil, nil, nil)
	names := toolNames(t, srv)

	if hasToolName(names, "get_query_results_summary") {
		t.Error("get_query_results_summary should not be registered in metadata mode")
	}
}

// TestEditorToolsRegisteredInAllModes verifies that get_current_editor_sql and
// get_query_history are registered in all modes.
func TestEditorToolsRegisteredInAllModes(t *testing.T) {
	store := NewEditorContextStore()
	modes := []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly}
	always := []string{"get_current_editor_sql", "get_query_history"}

	for _, mode := range modes {
		srv := buildServer(nil, mode, SessionConfig{}, store, nil, nil, nil)
		names := toolNames(t, srv)

		for _, tool := range always {
			if !hasToolName(names, tool) {
				t.Errorf("mode %q should expose %q (got: %v)", mode, tool, names)
			}
		}
	}
}

// TestEditorToolsNotRegisteredWithNilStore verifies that no editor tools are
// registered when the store is nil (graceful degradation in tests).
func TestEditorToolsNotRegisteredWithNilStore(t *testing.T) {
	srv := buildServer(nil, ExecutionModeReadonly, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	editorTools := []string{"get_current_editor_sql", "get_query_results_summary", "get_query_history"}
	for _, tool := range editorTools {
		if hasToolName(names, tool) {
			t.Errorf("%q should not be registered with nil store", tool)
		}
	}
}

// TestGetQueryHistoryNilClient verifies the tool returns an error result
// when no Snowflake client is available.
func TestGetQueryHistoryNilClient(t *testing.T) {
	store := NewEditorContextStore()
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, store, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_query_history",
		Arguments: queryHistoryInput{Limit: 10},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}
