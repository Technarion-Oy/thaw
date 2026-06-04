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

// Tool input types for the lineage tools.

type objectLineageInput struct {
	Database  string `json:"database" jsonschema:"the database name"`
	Schema    string `json:"schema" jsonschema:"the schema name"`
	Kind      string `json:"kind" jsonschema:"the object kind: VIEW, PROCEDURE, or FUNCTION"`
	Name      string `json:"name" jsonschema:"the object name"`
	Arguments string `json:"arguments,omitempty" jsonschema:"argument signature for overloaded procedures/functions, if any"`
}

type databaseCrossDepsInput struct {
	Database string   `json:"database" jsonschema:"the database name"`
	Schemas  []string `json:"schemas" jsonschema:"list of schema names to scan for cross-schema references"`
}

// allowedLineageKinds is the whitelist of Snowflake object kinds accepted by
// the get_object_lineage tool. Restricting to dependency-bearing kinds prevents
// meaningless queries (tables have no upstream refs) and untrusted input from
// reaching the dependency query.
var allowedLineageKinds = map[string]bool{
	"VIEW":      true,
	"PROCEDURE": true,
	"FUNCTION":  true,
}

// registerLineageTools wires the object lineage and cross-dependency tools
// onto srv. All tools are read-only metadata operations registered in every
// execution mode (metadata, readonly, explain_only).
func registerLineageTools(srv *mcpsdk.Server, client *snowflake.Client) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_object_lineage",
		Description: "Return the recursive dependency tree for a VIEW, PROCEDURE, or FUNCTION. " +
			"Shows which tables, views, and routines the object depends on (upstream impact analysis).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in objectLineageInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if !allowedLineageKinds[in.Kind] {
			return nil, nil, fmt.Errorf("unsupported object kind %q: must be VIEW, PROCEDURE, or FUNCTION", in.Kind)
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		node, err := client.GetObjectDependencies(ctx, in.Database, in.Schema, in.Kind, in.Name, in.Arguments)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(node), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_schema_cross_deps",
		Description: "Return the list of external schemas referenced by views in a schema. " +
			"Useful for understanding cross-schema dependencies before refactoring or migrating.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in schemaInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		refs, err := client.GetSchemaCrossDeps(ctx, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		// Ensure non-nil so JSON serializes as [] not null.
		if refs == nil {
			refs = []snowflake.SchemaRef{}
		}
		return jsonResult(refs), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_database_cross_deps",
		Description: "Return the combined cross-schema references from views across multiple schemas in a database. " +
			"Aggregates results from get_schema_cross_deps for each schema, deduplicated.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in databaseCrossDepsInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if len(in.Schemas) == 0 {
			return nil, nil, fmt.Errorf("schemas is required (at least one schema name)")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		refs, err := client.GetDatabaseCrossDeps(ctx, in.Database, in.Schemas)
		if err != nil {
			return nil, nil, err
		}
		// Ensure non-nil so JSON serializes as [] not null.
		if refs == nil {
			refs = []snowflake.SchemaRef{}
		}
		return jsonResult(refs), nil, nil
	})
}
