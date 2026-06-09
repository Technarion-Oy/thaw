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

// TestAccountToolsRegistered verifies that all 8 account tools are registered
// in metadata, readonly, and explain_only modes.
func TestAccountToolsRegistered(t *testing.T) {
	accountTools := []string{
		"list_roles",
		"list_available_roles",
		"get_role_ddl",
		"list_warehouses",
		"get_warehouse_ddl",
		"list_integrations",
		"list_secrets",
		"list_file_formats",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range accountTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestListRolesNilClient verifies the tool returns an error when no Snowflake
// client is available (no-input tool).
func TestListRolesNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_roles",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetRoleDDLNilClient verifies the tool returns an error when no Snowflake
// client is available (tool with input validation).
func TestGetRoleDDLNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_role_ddl",
		Arguments: nameInput{Name: "SYSADMIN"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestListIntegrationsEmptyKind verifies that an empty kind returns an error.
func TestListIntegrationsEmptyKind(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_integrations",
		Arguments: integrationKindInput{Kind: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty kind")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "kind is required") {
		t.Errorf("error message should mention kind requirement, got: %s", text)
	}
}

// TestGetRoleDDLEmptyName verifies that an empty name returns an error.
func TestGetRoleDDLEmptyName(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_role_ddl",
		Arguments: nameInput{Name: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty name")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "name is required") {
		t.Errorf("error message should mention name requirement, got: %s", text)
	}
}

// TestGetWarehouseDDLEmptyName verifies that an empty name returns an error.
func TestGetWarehouseDDLEmptyName(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_warehouse_ddl",
		Arguments: nameInput{Name: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty name")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "name is required") {
		t.Errorf("error message should mention name requirement, got: %s", text)
	}
}

// TestListFileFormatsRequiresInputs verifies that empty database and schema
// return errors.
func TestListFileFormatsRequiresInputs(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing both database and schema.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_file_formats",
		Arguments: schemaInput{Database: "", Schema: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error message should mention database requirement, got: %s", text)
	}

	// Database present but schema missing.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_file_formats",
		Arguments: schemaInput{Database: "MYDB", Schema: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty schema")
	}
	text = extractText(t, res)
	if !strings.Contains(text, "schema is required") {
		t.Errorf("error message should mention schema requirement, got: %s", text)
	}
}
