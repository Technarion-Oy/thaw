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
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedTools is the full tool set the server exposes (alphabetically sorted).
var expectedTools = []string{
	"describe_table",
	"format_sql",
	"get_ddl",
	"get_session_context",
	"get_snowflake_keywords",
	"get_table_foreign_keys",
	"list_databases",
	"list_objects",
	"list_schemas",
	"suggest_join_conditions",
	"validate_sql",
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

// TestTokenGuard verifies the per-session token is required on session-creating
// GET requests (via header or query param) but POSTs pass through unchecked
// (they are authorized by the SDK-issued sessionid).
func TestTokenGuard(t *testing.T) {
	const token = "s3cret-token"
	guard := tokenGuard(token, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name    string
		method  string
		query   string
		authHdr string
		want    int
	}{
		{"get no token", http.MethodGet, "", "", http.StatusUnauthorized},
		{"get wrong query token", http.MethodGet, "?token=nope", "", http.StatusUnauthorized},
		{"get correct query token", http.MethodGet, "?token=" + token, "", http.StatusOK},
		{"get correct bearer token", http.MethodGet, "", "Bearer " + token, http.StatusOK},
		{"get wrong bearer token", http.MethodGet, "", "Bearer nope", http.StatusUnauthorized},
		// POSTs are authorized by the sessionid the SDK only emits over the
		// authenticated GET stream, so the guard lets them through.
		{"post no token", http.MethodPost, "?sessionid=abc", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://localhost:9100/sse"+tc.query, nil)
			if tc.authHdr != "" {
				req.Header.Set("Authorization", tc.authHdr)
			}
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestNewSessionTokenUnique verifies tokens are non-empty and not repeated.
func TestNewSessionTokenUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		tok, err := newSessionToken()
		if err != nil {
			t.Fatalf("newSessionToken failed: %v", err)
		}
		if tok == "" {
			t.Fatal("newSessionToken returned empty string")
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = true
	}
}

// TestAuthenticatedSSERoundTrip verifies a client presenting the per-session
// token can complete the full SSE handshake (GET stream + POSTed message
// endpoint) through the loopbackGuard+tokenGuard stack, and that a client
// without the token is rejected at connect.
func TestAuthenticatedSSERoundTrip(t *testing.T) {
	const token = "round-trip-token"
	srv := buildServer(nil, ExecutionModeMetadata)
	sse := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	handler := loopbackGuard(tokenGuard(token, sse))
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()

	// Without the token, connect must fail (the GET returns 401).
	noAuth := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	if _, err := noAuth.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL + "/sse"}, nil); err == nil {
		t.Fatal("expected connect without token to fail, got nil error")
	}

	// With the token in the URL, the full round-trip (GET + message POSTs)
	// succeeds even though the SDK drops the token from the message endpoint.
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL + "/sse?token=" + token}, nil)
	if err != nil {
		t.Fatalf("client connect with token failed: %v", err)
	}
	defer func() { _ = cs.Close() }()

	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatalf("ListTools over authenticated SSE failed: %v", err)
	}
}

// TestAllowedDDLKinds verifies the kind whitelist accepts standard Snowflake
// object kinds and rejects unknown/malicious strings.
func TestAllowedDDLKinds(t *testing.T) {
	for _, kind := range []string{"TABLE", "VIEW", "FUNCTION", "PROCEDURE", "STAGE"} {
		if !allowedDDLKinds[kind] {
			t.Errorf("expected %q to be allowed", kind)
		}
	}
	for _, kind := range []string{"", "EVIL", "TABLE'; DROP", "table", "DATABASE", "SCHEMA"} {
		if allowedDDLKinds[kind] {
			t.Errorf("expected %q to be rejected", kind)
		}
	}
}

// TestValidateSqlPureMarkers calls validate_sql with a nil client and SQL that
// has syntax errors, verifying that pure (phase-1) markers are returned even
// without a live Snowflake connection.
func TestValidateSqlPureMarkers(t *testing.T) {
	// Unmatched parenthesis produces a syntax error marker.
	markers := validateSQL(context.Background(), nil, "SELECT (1")
	if len(markers) == 0 {
		t.Fatal("expected at least one diagnostic marker for unmatched paren, got 0")
	}
	// All markers from the pure phase should have Severity 8 (Error) or 4 (Warning).
	for i, m := range markers {
		if m.Severity != 8 && m.Severity != 4 {
			t.Errorf("marker[%d].Severity = %d, want 8 or 4", i, m.Severity)
		}
	}
}

// TestFormatSqlTool calls ApplyCasing through the format_sql path and verifies
// keyword casing is applied.
func TestFormatSqlTool(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		kwCase   string
		idCase   string
		fnCase   string
		contains string
	}{
		{"uppercase keywords", "select 1", "UPPER", "", "", "SELECT"},
		{"lowercase keywords", "SELECT 1", "lower", "", "", "select"},
		{"preserve empty", "SELECT 1", "", "", "", "SELECT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := buildServer(nil, ExecutionModeMetadata)
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
				Name: "format_sql",
				Arguments: formatSqlInput{
					SQL:            tc.sql,
					KeywordCase:    tc.kwCase,
					IdentifierCase: tc.idCase,
					FunctionCase:   tc.fnCase,
				},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.contains) {
				t.Errorf("result %q does not contain %q", text, tc.contains)
			}
		})
	}
}

// TestGetSnowflakeKeywordsTool verifies the keyword list is non-empty.
func TestGetSnowflakeKeywordsTool(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata)
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
		Name:      "get_snowflake_keywords",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	text := extractText(t, res)
	var keywords []string
	if err := json.Unmarshal([]byte(text), &keywords); err != nil {
		t.Fatalf("payload is not valid JSON array: %v", err)
	}
	if len(keywords) < 10 {
		t.Errorf("expected at least 10 keywords, got %d", len(keywords))
	}
}

// extractText extracts the text from a single-content CallToolResult.
func extractText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcpsdk.TextContent", res.Content[0])
	}
	return tc.Text
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
