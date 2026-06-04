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

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// Tool input types for account & infrastructure tools.

type nameInput struct {
	Name string `json:"name" jsonschema:"the object name"`
}

type integrationKindInput struct {
	Kind string `json:"kind" jsonschema:"the integration kind: API, NOTIFICATION, SECURITY, STORAGE, or CATALOG"`
}

// registerAccountTools wires account-level and infrastructure browsing tools
// onto srv. All tools are read-only metadata operations registered in every
// execution mode (metadata, readonly, explain_only).
func registerAccountTools(srv *mcpsdk.Server, client *snowflake.Client) {

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_roles",
		Description: "List all roles visible to the current session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		roles, err := client.ListRoles(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(roles), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_available_roles",
		Description: "List roles available (grantable) to the current user.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		roles, err := client.ListAvailableRoles(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(roles), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_role_ddl",
		Description: "Return the CREATE ROLE DDL for a role, including granted privileges.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in nameInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		ddl, err := client.GetRoleDDL(ctx, in.Name)
		if err != nil {
			return nil, nil, err
		}
		return textResult(ddl), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_warehouses",
		Description: "List all warehouses accessible to the current session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		whs, err := client.ListWarehouses(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(whs), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_warehouse_ddl",
		Description: "Return the CREATE WAREHOUSE DDL for a warehouse.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in nameInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		ddl, err := client.GetWarehouseDDL(ctx, in.Name)
		if err != nil {
			return nil, nil, err
		}
		return textResult(ddl), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "list_integrations",
		Description: "List integrations of a given kind. " +
			"Valid kinds: API, NOTIFICATION, SECURITY, STORAGE, CATALOG.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in integrationKindInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Kind == "" {
			return nil, nil, fmt.Errorf("kind is required")
		}
		rows, err := client.ListIntegrations(ctx, in.Kind)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(rows), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_secrets",
		Description: "List all secrets visible in the account (name, database, schema).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		secrets, err := client.ListSecretsInAccount(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(secrets), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_file_formats",
		Description: "List file formats defined in a schema.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in schemaInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		fmts, err := client.ListFileFormats(ctx, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(fmts), nil, nil
	})
}
