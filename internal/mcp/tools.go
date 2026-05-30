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

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// Tool input types. Output types reuse the snowflake domain types directly so
// the JSON schema and payload match the rest of the application.

type emptyInput struct{}

type databaseInput struct {
	Database string `json:"database" jsonschema:"the database name"`
}

type schemaInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
}

type tableInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the table or view name"`
}

type foreignKeysInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Table    string `json:"table" jsonschema:"the table name"`
}

type ddlInput struct {
	Database  string `json:"database" jsonschema:"the database name"`
	Schema    string `json:"schema" jsonschema:"the schema name"`
	Kind      string `json:"kind" jsonschema:"the object kind, e.g. TABLE, VIEW, FUNCTION"`
	Name      string `json:"name" jsonschema:"the object name"`
	Arguments string `json:"arguments,omitempty" jsonschema:"argument signature for overloaded routines, if any"`
}

// registerTools wires the proof-of-life schema-browsing tools onto srv. All
// tools are read-only metadata operations backed by the supplied client.
func registerTools(srv *mcpsdk.Server, client *snowflake.Client) {
	// All tools use 'any' as the Out type parameter so the SDK omits output
	// schema inference; the MCP spec requires output schemas to be objects,
	// but several tools return arrays or strings. The payload is delivered as
	// text content (indented JSON) instead.

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_session_context",
		Description: "Return the current Snowflake session context (role, warehouse, database, schema).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		sc, err := client.GetSessionContext(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(sc), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_databases",
		Description: "List all databases accessible to the current session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		dbs, err := client.ListDatabases(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dbs), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_schemas",
		Description: "List the schemas in a database.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in databaseInput) (*mcpsdk.CallToolResult, any, error) {
		schemas, err := client.ListSchemas(ctx, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(schemas), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_objects",
		Description: "List the objects (tables, views, etc.) in a schema.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in schemaInput) (*mcpsdk.CallToolResult, any, error) {
		objs, err := client.ListObjects(ctx, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(objs), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "describe_table",
		Description: "Describe the columns of a table or view, including data types, nullability, and keys.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in tableInput) (*mcpsdk.CallToolResult, any, error) {
		cols, err := client.GetTableColumnsWithTypes(ctx, in.Database, in.Schema, in.Name)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(cols), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_ddl",
		Description: "Return the CREATE DDL for an object.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ddlInput) (*mcpsdk.CallToolResult, any, error) {
		ddl, err := client.GetObjectDDL(ctx, in.Database, in.Schema, in.Kind, in.Name, in.Arguments)
		if err != nil {
			return nil, nil, err
		}
		return textResult(ddl), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_table_foreign_keys",
		Description: "List the foreign-key relationships defined on a table.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in foreignKeysInput) (*mcpsdk.CallToolResult, any, error) {
		fks, err := client.GetTableForeignKeys(ctx, in.Database, in.Schema, in.Table)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(fks), nil, nil
	})
}

// jsonResult marshals v to indented JSON and wraps it as text content so MCP
// clients that only read text content still receive the full payload.
func jsonResult(v any) *mcpsdk.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return textResult(err.Error())
	}
	return textResult(string(b))
}

// textResult wraps a plain string as a single text-content tool result.
func textResult(s string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: s}},
	}
}
