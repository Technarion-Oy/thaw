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
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestOpenSqlTabNotRegisteredWithNilEmit verifies that the open_sql_tab tool
// is not registered when emit is nil (graceful degradation in tests).
func TestOpenSqlTabNotRegisteredWithNilEmit(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil)
	names := toolNames(t, srv)

	if hasToolName(names, "open_sql_tab") {
		t.Error("open_sql_tab should not be registered when emit is nil")
	}
}

// TestOpenSqlTabRegisteredWithEmit verifies that the open_sql_tab tool is
// registered when a non-nil emit function is provided.
func TestOpenSqlTabRegisteredWithEmit(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil)
	names := toolNames(t, srv)

	if !hasToolName(names, "open_sql_tab") {
		t.Errorf("open_sql_tab should be registered when emit is non-nil (got: %v)", names)
	}
}

// TestOpenSqlTabEmptySQL verifies that calling open_sql_tab with empty SQL
// returns an error.
func TestOpenSqlTabEmptySQL(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil)

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
		Name:      "open_sql_tab",
		Arguments: openSqlTabInput{SQL: ""},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty SQL, got false")
	}
}

// TestOpenSqlTabEmitsEvent verifies that calling open_sql_tab with valid SQL
// emits the correct event name and payload shape.
func TestOpenSqlTabEmitsEvent(t *testing.T) {
	var mu sync.Mutex
	var emittedName string
	var emittedData interface{}
	emit := func(name string, data interface{}) {
		mu.Lock()
		emittedName = name
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil)

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
		Name:      "open_sql_tab",
		Arguments: openSqlTabInput{SQL: "SELECT 1", Title: "Test Tab"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		text := extractText(t, res)
		t.Fatalf("unexpected error: %s", text)
	}

	mu.Lock()
	defer mu.Unlock()

	if emittedName != "mcp:open-sql-tab" {
		t.Errorf("emitted event name = %q, want %q", emittedName, "mcp:open-sql-tab")
	}

	payload, ok := emittedData.(OpenSqlTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenSqlTabPayload", emittedData)
	}
	if payload.Title != "Test Tab" {
		t.Errorf("payload.Title = %q, want %q", payload.Title, "Test Tab")
	}
	if payload.SQL == "" {
		t.Error("payload.SQL is empty")
	}
	if payload.Markers == nil {
		t.Error("payload.Markers should be non-nil (empty slice, not nil)")
	}

	text := extractText(t, res)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success message, got: %s", text)
	}
}

// TestOpenSqlTabDefaultTitle verifies that an empty title defaults to "AI Query".
func TestOpenSqlTabDefaultTitle(t *testing.T) {
	var mu sync.Mutex
	var emittedData interface{}
	emit := func(_ string, data interface{}) {
		mu.Lock()
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil)

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

	_, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "open_sql_tab",
		Arguments: openSqlTabInput{SQL: "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	payload, ok := emittedData.(OpenSqlTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenSqlTabPayload", emittedData)
	}
	if payload.Title != "AI Query" {
		t.Errorf("payload.Title = %q, want %q", payload.Title, "AI Query")
	}
}

// TestOpenSqlTabTitleTruncation verifies that titles longer than 100 runes are
// truncated without splitting multi-byte UTF-8 characters.
func TestOpenSqlTabTitleTruncation(t *testing.T) {
	var mu sync.Mutex
	var emittedData interface{}
	emit := func(_ string, data interface{}) {
		mu.Lock()
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil)

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

	// Build a title with 105 runes including multi-byte characters.
	// "日本語テスト" = 5 runes; repeated 21 times = 105 runes (315 bytes).
	longTitle := strings.Repeat("日本語テスト", 21) // 105 runes

	_, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "open_sql_tab",
		Arguments: openSqlTabInput{SQL: "SELECT 1", Title: longTitle},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	payload, ok := emittedData.(OpenSqlTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenSqlTabPayload", emittedData)
	}
	runes := []rune(payload.Title)
	if len(runes) != 100 {
		t.Errorf("payload.Title has %d runes, want 100", len(runes))
	}
	// Verify the truncated string is valid UTF-8 (no split multi-byte chars).
	for i, r := range runes {
		if r == '\uFFFD' {
			t.Errorf("replacement character at rune index %d — UTF-8 was split", i)
		}
	}
}
