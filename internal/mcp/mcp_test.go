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
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedTools is the proof-of-life tool set the foundation server exposes.
var expectedTools = []string{
	"describe_table",
	"get_ddl",
	"get_session_context",
	"get_table_foreign_keys",
	"list_databases",
	"list_objects",
	"list_schemas",
}

// TestServerExposesToolsOverSSE verifies that an external MCP client can
// connect to the server over the SSE transport and discover the proof-of-life
// tools. A nil client is sufficient because tool *listing* does not invoke the
// handlers (which would call into Snowflake).
func TestServerExposesToolsOverSSE(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("client connect over SSE failed: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)

	if len(got) != len(expectedTools) {
		t.Fatalf("tool count = %d, want %d (%v)", len(got), len(expectedTools), got)
	}
	for i, name := range expectedTools {
		if got[i] != name {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], name)
		}
	}
}

// TestManagerPortAllocation verifies auto-assignment starts at basePort and
// that an explicit port already claimed by a session is rejected.
func TestManagerPortAllocation(t *testing.T) {
	m := NewManager()

	ln, err := m.allocatePortLocked(0)
	if err != nil {
		t.Fatalf("allocatePortLocked(0) failed: %v", err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	if port < basePort {
		t.Errorf("auto-assigned port %d below basePort %d", port, basePort)
	}

	// Register a session occupying that port and assert an explicit request
	// for the same port is rejected (the inUse path in allocatePortLocked).
	m.sessions["occupied"] = &session{label: "occupied", port: port}
	if _, err := m.allocatePortLocked(port); err == nil {
		t.Errorf("expected duplicate port %d to be rejected, got nil error", port)
	}
}

// TestLoopbackGuard verifies the SSE handler rejects non-loopback Host headers
// and cross-origin browser requests while allowing loopback traffic.
func TestLoopbackGuard(t *testing.T) {
	guard := loopbackGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"loopback host no origin", "localhost:9100", "", http.StatusOK},
		{"loopback ip", "127.0.0.1:9100", "", http.StatusOK},
		{"loopback origin", "localhost:9100", "http://localhost:9100", http.StatusOK},
		{"rebound host", "evil.example.com", "", http.StatusForbidden},
		{"cross origin", "localhost:9100", "https://evil.example.com", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/sse", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestJSONResultShaping verifies the tool result helpers wrap payloads as a
// single text-content block carrying the indented-JSON encoding. The tool
// handlers all funnel their output through jsonResult/textResult, so this
// covers the result-shaping logic without a live Snowflake client.
func TestJSONResultShaping(t *testing.T) {
	res := jsonResult(map[string]string{"role": "SYSADMIN"})
	if len(res.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcpsdk.TextContent", res.Content[0])
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(tc.Text), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["role"] != "SYSADMIN" {
		t.Errorf("decoded[role] = %q, want SYSADMIN", decoded["role"])
	}
}
