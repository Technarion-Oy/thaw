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
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/fnmeta"
	"thaw/internal/procedure"
)

// TestFunctionToolsRegistered verifies that all 6 function tools are registered
// in metadata, readonly, and explain_only modes.
func TestFunctionToolsRegistered(t *testing.T) {
	functionTools := []string{
		"search_functions",
		"get_function_tooltip",
		"get_procedure_params",
		"get_function_info",
		"build_call_statement",
		"build_function_select",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range functionTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestSearchFunctionsNilStore verifies the tool returns an error when fnStore
// is nil.
func TestSearchFunctionsNilStore(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_functions",
		Arguments: searchFunctionsInput{Prefix: "ABS"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil fnStore")
	}
}

// TestSearchFunctionsEmptyPrefix verifies that an empty prefix returns an error.
func TestSearchFunctionsEmptyPrefix(t *testing.T) {
	store := openTestFnStore(t)
	cs := newFnStoreTestSession(t, store)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_functions",
		Arguments: searchFunctionsInput{Prefix: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty prefix")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "prefix is required") {
		t.Errorf("error message should mention prefix requirement, got: %s", text)
	}
}

// TestGetFunctionTooltipNilStore verifies the tool returns an error when
// fnStore is nil.
func TestGetFunctionTooltipNilStore(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_function_tooltip",
		Arguments: functionLookupInput{Name: "ABS"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil fnStore")
	}
}

// TestGetFunctionTooltipEmptyName verifies that an empty name returns an error.
func TestGetFunctionTooltipEmptyName(t *testing.T) {
	store := openTestFnStore(t)
	cs := newFnStoreTestSession(t, store)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_function_tooltip",
		Arguments: functionLookupInput{Name: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty name")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "name is required") {
		t.Errorf("error message should mention name requirement, got: %s", text)
	}
}

// TestGetProcedureParamsNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestGetProcedureParamsNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "get_procedure_params",
		Arguments: procedureParamsInput{
			Database: "DB",
			Schema:   "PUBLIC",
			Name:     "MY_PROC",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetProcedureParamsEmptyFields verifies that missing db/schema/name
// return errors.
func TestGetProcedureParamsEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   procedureParamsInput
		wantMsg string
	}{
		{"missing database", procedureParamsInput{Schema: "PUBLIC", Name: "PROC"}, "database is required"},
		{"missing schema", procedureParamsInput{Database: "DB", Name: "PROC"}, "schema is required"},
		{"missing name", procedureParamsInput{Database: "DB", Schema: "PUBLIC"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_procedure_params",
				Arguments: tc.input,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true")
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.wantMsg) {
				t.Errorf("error message should contain %q, got: %s", tc.wantMsg, text)
			}
		})
	}
}

// TestGetFunctionInfoEmptyFields verifies that missing db/schema/name return
// errors.
func TestGetFunctionInfoEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   functionInfoInput
		wantMsg string
	}{
		{"missing database", functionInfoInput{Schema: "PUBLIC", Name: "FN"}, "database is required"},
		{"missing schema", functionInfoInput{Database: "DB", Name: "FN"}, "schema is required"},
		{"missing name", functionInfoInput{Database: "DB", Schema: "PUBLIC"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_function_info",
				Arguments: tc.input,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true")
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.wantMsg) {
				t.Errorf("error message should contain %q, got: %s", tc.wantMsg, text)
			}
		})
	}
}

// TestBuildCallStatementEmptyFields verifies that missing fields return errors.
func TestBuildCallStatementEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   buildCallStatementInput
		wantMsg string
	}{
		{"missing database", buildCallStatementInput{Schema: "PUBLIC", Name: "PROC"}, "database is required"},
		{"missing schema", buildCallStatementInput{Database: "DB", Name: "PROC"}, "schema is required"},
		{"missing name", buildCallStatementInput{Database: "DB", Schema: "PUBLIC"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "build_call_statement",
				Arguments: tc.input,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true")
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.wantMsg) {
				t.Errorf("error message should contain %q, got: %s", tc.wantMsg, text)
			}
		})
	}
}

// TestBuildCallStatementSuccess verifies that valid input produces a CALL statement.
func TestBuildCallStatementSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_call_statement",
		Arguments: buildCallStatementInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Name:     "MY_PROC",
			Args: []procedure.Argument{
				{Name: "arg1", DataType: "VARCHAR", Value: "hello"},
				{Name: "arg2", DataType: "NUMBER", Value: "42"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(text, "CALL") {
		t.Errorf("expected CALL in result, got: %s", text)
	}
	if !strings.Contains(text, "MY_PROC") {
		t.Errorf("expected procedure name in result, got: %s", text)
	}
}

// TestBuildFunctionSelectSuccess verifies that valid input produces a SELECT
// statement for both scalar and table function variants.
func TestBuildFunctionSelectSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Scalar function.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_function_select",
		Arguments: buildFunctionSelectInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Name:     "MY_FN",
			Args: []procedure.Argument{
				{Name: "x", DataType: "NUMBER", Value: "1"},
			},
			IsTableFunction: false,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}
	text := extractText(t, res)
	if !strings.Contains(text, "SELECT") {
		t.Errorf("expected SELECT in scalar result, got: %s", text)
	}
	if strings.Contains(text, "TABLE(") {
		t.Errorf("scalar function should not contain TABLE(, got: %s", text)
	}

	// Table function.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_function_select",
		Arguments: buildFunctionSelectInput{
			Database:        "MYDB",
			Schema:          "PUBLIC",
			Name:            "MY_UDTF",
			IsTableFunction: true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}
	text = extractText(t, res)
	if !strings.Contains(text, "TABLE(") {
		t.Errorf("table function should contain TABLE(, got: %s", text)
	}
}

// TestBuildFunctionSelectEmptyFields verifies that missing fields return errors.
func TestBuildFunctionSelectEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   buildFunctionSelectInput
		wantMsg string
	}{
		{"missing database", buildFunctionSelectInput{Schema: "PUBLIC", Name: "FN"}, "database is required"},
		{"missing schema", buildFunctionSelectInput{Database: "DB", Name: "FN"}, "schema is required"},
		{"missing name", buildFunctionSelectInput{Database: "DB", Schema: "PUBLIC"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "build_function_select",
				Arguments: tc.input,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true")
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.wantMsg) {
				t.Errorf("error message should contain %q, got: %s", tc.wantMsg, text)
			}
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// openTestFnStore creates a temporary fnmeta.Store seeded with fallback data
// for use in tests that need a non-nil store.
func openTestFnStore(t *testing.T) *fnmeta.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "fnmeta")
	store, err := fnmeta.Open(dir)
	if err != nil {
		t.Fatalf("fnmeta.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.LoadFallback(); err != nil {
		t.Fatalf("LoadFallback: %v", err)
	}
	return store
}

// newFnStoreTestSession creates a test MCP client session with a non-nil
// fnStore. This is required for tests that exercise function metadata tools
// that depend on the store being available.
func newFnStoreTestSession(t *testing.T, store *fnmeta.Store) *mcpsdk.ClientSession {
	t.Helper()
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, store, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
