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

// Tool input types for the extended schema discovery tools.

type searchObjectsInput struct {
	Pattern  string `json:"pattern" jsonschema:"SQL ILIKE pattern to match object and column names"`
	Database string `json:"database,omitempty" jsonschema:"database to search in; defaults to current session database"`
}

type validateDataTypeInput struct {
	DataType string `json:"dataType" jsonschema:"the Snowflake data type to validate, e.g. VARCHAR(256)"`
}

type dataRetentionInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema,omitempty" jsonschema:"the schema name (optional; omit for database-level retention)"`
	Table    string `json:"table,omitempty" jsonschema:"the table name (optional; requires schema; omit for schema-level retention)"`
}

// registerSchemaTools wires the extended schema discovery and object browsing
// tools onto srv. All tools are read-only metadata operations registered in
// every execution mode (metadata, readonly, explain_only).
func registerSchemaTools(srv *mcpsdk.Server, client *snowflake.Client) {

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_schema_foreign_keys",
		Description: "List all foreign-key relationships in a schema (bulk — cheaper than per-table queries).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in schemaInput) (*mcpsdk.CallToolResult, any, error) {
		fks, err := client.GetSchemaForeignKeys(ctx, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(fks), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_database_ddl",
		Description: "Return the complete DDL for a database (all schemas and objects).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in databaseInput) (*mcpsdk.CallToolResult, any, error) {
		ddl, err := client.GetCompleteDatabaseDDL(ctx, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return textResult(ddl), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_er_model",
		Description: "Return ER diagram data for a database: tables with columns, primary keys, nullability, and foreign-key relationships.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in databaseInput) (*mcpsdk.CallToolResult, any, error) {
		data, err := client.GetERDiagramData(ctx, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(data), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "search_objects",
		Description: "Search for objects and columns matching a SQL ILIKE pattern across all schemas in a database. Returns up to 100 matching objects and 100 matching columns.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchObjectsInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Pattern == "" {
			return nil, nil, fmt.Errorf("pattern is required")
		}
		result, err := client.SearchObjects(ctx, in.Pattern, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(result), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_all_data_types",
		Description: "Return the complete list of supported Snowflake data types with parameter syntax hints.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		return jsonResult(snowflake.AllDataTypes()), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "validate_data_type",
		Description: "Validate a Snowflake data type string and return the normalised form. Returns structured validation result (not an error) for invalid input.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in validateDataTypeInput) (*mcpsdk.CallToolResult, any, error) {
		normalized, err := snowflake.ValidateDataType(in.DataType)
		if err != nil {
			// Validation failure is expected user input, not a tool error.
			// Return structured result so the client can inspect it.
			return jsonResult(struct {
				Input      string `json:"input"`
				Normalized string `json:"normalized"`
				Valid      bool   `json:"valid"`
				Error      string `json:"error"`
			}{
				Input: in.DataType,
				Valid: false,
				Error: err.Error(),
			}), nil, nil
		}
		return jsonResult(struct {
			Input      string `json:"input"`
			Normalized string `json:"normalized"`
			Valid      bool   `json:"valid"`
		}{
			Input:      in.DataType,
			Normalized: normalized,
			Valid:      true,
		}), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_dropped_tables",
		Description: "List tables that have been dropped in a schema (available for time-travel undrop).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in schemaInput) (*mcpsdk.CallToolResult, any, error) {
		tables, err := client.ListDroppedTables(ctx, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(tables), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_dropped_schemas",
		Description: "List schemas that have been dropped in a database (available for time-travel undrop).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in databaseInput) (*mcpsdk.CallToolResult, any, error) {
		schemas, err := client.ListDroppedSchemas(ctx, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(schemas), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_data_retention",
		Description: "Return the data retention period (in days) for a database, schema, or table. " +
			"Provide only database for database-level, database+schema for schema-level, " +
			"or database+schema+table for table-level retention.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in dataRetentionInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Table != "" && in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required when table is specified")
		}

		type retentionResult struct {
			Database      string `json:"database"`
			Schema        string `json:"schema,omitempty"`
			Table         string `json:"table,omitempty"`
			RetentionDays int    `json:"retentionDays"`
		}

		var days int
		var err error

		switch {
		case in.Table != "":
			days, err = client.GetTableRetentionDays(ctx, in.Database, in.Schema, in.Table)
		case in.Schema != "":
			days, err = client.GetSchemaRetentionDays(ctx, in.Database, in.Schema)
		default:
			days, err = client.GetDatabaseRetentionDays(ctx, in.Database)
		}
		if err != nil {
			return nil, nil, err
		}

		return jsonResult(retentionResult{
			Database:      in.Database,
			Schema:        in.Schema,
			Table:         in.Table,
			RetentionDays: days,
		}), nil, nil
	})
}
