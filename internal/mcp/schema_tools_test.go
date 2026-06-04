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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSchemaToolsRegistered verifies that all 9 extended schema tools are
// registered on a server built with a nil client (tool listing does not invoke
// handlers).
func TestSchemaToolsRegistered(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
	names := toolNames(t, srv)

	expected := []string{
		"get_schema_foreign_keys",
		"get_database_ddl",
		"get_er_model",
		"search_objects",
		"get_all_data_types",
		"validate_data_type",
		"list_dropped_tables",
		"list_dropped_schemas",
		"get_data_retention",
	}
	for _, name := range expected {
		if !hasToolName(names, name) {
			t.Errorf("expected tool %q to be registered, got tools: %v", name, names)
		}
	}
}

// TestValidateDataTypeToolValid exercises validate_data_type through SSE with
// a valid data type and verifies the structured result.
func TestValidateDataTypeToolValid(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
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
		Name:      "validate_data_type",
		Arguments: validateDataTypeInput{DataType: "varchar(256)"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	text := extractText(t, res)
	var result struct {
		Input      string `json:"input"`
		Normalized string `json:"normalized"`
		Valid      bool   `json:"valid"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, text)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, got false")
	}
	if result.Normalized != "VARCHAR(256)" {
		t.Errorf("normalized = %q, want VARCHAR(256)", result.Normalized)
	}
}

// TestValidateDataTypeToolInvalid exercises validate_data_type with an invalid
// data type and verifies the structured error result (not a Go error).
func TestValidateDataTypeToolInvalid(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
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
		Name:      "validate_data_type",
		Arguments: validateDataTypeInput{DataType: "NOTAREAL_TYPE"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	// Invalid input should NOT produce IsError — it's a structured result.
	if res.IsError {
		t.Errorf("expected IsError=false for invalid data type (structured result), got true")
	}

	text := extractText(t, res)
	var result struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, text)
	}
	if result.Valid {
		t.Errorf("expected valid=false for invalid type, got true")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty error message")
	}
}

// TestGetDataRetentionTableWithoutSchema verifies that specifying a table
// without a schema returns an error.
func TestGetDataRetentionTableWithoutSchema(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
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
		Name: "get_data_retention",
		Arguments: dataRetentionInput{
			Database: "MYDB",
			Table:    "MYTABLE",
			// Schema intentionally omitted.
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !res.IsError {
		t.Errorf("expected IsError=true when table is specified without schema")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "schema is required") {
		t.Errorf("error message should mention schema requirement, got: %s", text)
	}
}

// TestSearchObjectsEmptyPattern verifies that an empty pattern is rejected.
func TestSearchObjectsEmptyPattern(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
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
		Name:      "search_objects",
		Arguments: searchObjectsInput{Pattern: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !res.IsError {
		t.Errorf("expected IsError=true for empty pattern")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "pattern is required") {
		t.Errorf("error message should mention pattern requirement, got: %s", text)
	}
}

// TestGetAllDataTypesTool verifies the tool returns a non-empty JSON array
// of data types.
func TestGetAllDataTypesTool(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
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
		Name:      "get_all_data_types",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	text := extractText(t, res)
	if !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("expected JSON array, got: %.80s", text)
	}

	var types []struct {
		Name string `json:"Name"`
	}
	if err := json.Unmarshal([]byte(text), &types); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(types) < 10 {
		t.Errorf("expected at least 10 data types, got %d", len(types))
	}
}

// TestSchemaToolsRegisteredInAllModes verifies that schema tools are present
// in metadata, readonly, and explain_only modes.
func TestSchemaToolsRegisteredInAllModes(t *testing.T) {
	schemaTools := []string{
		"get_schema_foreign_keys",
		"get_database_ddl",
		"get_er_model",
		"search_objects",
		"get_all_data_types",
		"validate_data_type",
		"list_dropped_tables",
		"list_dropped_schemas",
		"get_data_retention",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range schemaTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered", mode, tool)
				}
			}
		})
	}
}
