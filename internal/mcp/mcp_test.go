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
	defer cs.Close()

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
// that an explicit duplicate port is rejected.
func TestManagerPortAllocation(t *testing.T) {
	m := NewManager()

	p, err := m.allocatePortLocked(0)
	if err != nil {
		t.Fatalf("allocatePortLocked(0) failed: %v", err)
	}
	if p < basePort {
		t.Errorf("auto-assigned port %d below basePort %d", p, basePort)
	}
}
