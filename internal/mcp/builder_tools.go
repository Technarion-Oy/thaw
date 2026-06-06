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

	"thaw/internal/fileformat"
	"thaw/internal/integrations"
	"thaw/internal/pipe"
	"thaw/internal/secret"
	"thaw/internal/stage"
)

// Tool input types for builder tools that need db/schema wrappers around
// domain config structs.

type buildCreateFileFormatInput struct {
	Database string                    `json:"database" jsonschema:"the database name"`
	Schema   string                    `json:"schema" jsonschema:"the schema name"`
	Config   fileformat.FileFormatConfig `json:"config" jsonschema:"the file format configuration"`
}

type buildCreatePipeInput struct {
	Database string          `json:"database" jsonschema:"the database name"`
	Schema   string          `json:"schema" jsonschema:"the schema name"`
	Config   pipe.PipeConfig `json:"config" jsonschema:"the pipe configuration including COPY INTO statement"`
}

type buildRefreshPipeInput struct {
	Database string                `json:"database" jsonschema:"the database name"`
	Schema   string                `json:"schema" jsonschema:"the schema name"`
	Name     string                `json:"name" jsonschema:"the pipe name"`
	Config   pipe.RefreshPipeConfig `json:"config" jsonschema:"the refresh configuration (optional prefix and modifiedAfter)"`
}

type buildCreateSecretInput struct {
	Database string              `json:"database" jsonschema:"the database name"`
	Schema   string              `json:"schema" jsonschema:"the schema name"`
	Config   secret.SecretConfig `json:"config" jsonschema:"the secret configuration"`
}

// registerBuilderTools wires DDL builder tools onto srv. All tools are pure
// SQL generators — no Snowflake client needed, no SQL execution. They are
// registered in every execution mode.
func registerBuilderTools(srv *mcpsdk.Server) {

	// ── Stage tools ──────────────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_stage_sql",
		Description: "Generate a CREATE STAGE DDL statement from a stage configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in stage.StageConfig) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql := stage.BuildCreateStageSql(in)
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_alter_stage_sql",
		Description: "Generate an ALTER STAGE DDL statement from an alter-stage configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in stage.AlterStageConfig) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql := stage.BuildAlterStageSql(in)
		return textResult(sql), nil, nil
	})

	// ── File format tool ─────────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_file_format_sql",
		Description: "Generate a CREATE FILE FORMAT DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateFileFormatInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql := fileformat.BuildCreateFileFormatSql(in.Database, in.Schema, in.Config)
		return textResult(sql), nil, nil
	})

	// ── Pipe tools ───────────────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_pipe_sql",
		Description: "Generate a CREATE PIPE DDL statement with a COPY INTO definition. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreatePipeInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql, err := pipe.BuildCreatePipeSql(in.Database, in.Schema, in.Config)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_refresh_pipe_sql",
		Description: "Generate an ALTER PIPE ... REFRESH statement with optional prefix and modifiedAfter filters. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildRefreshPipeInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		sql, err := pipe.BuildRefreshPipeSql(in.Database, in.Schema, in.Name, in.Config)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	// ── Secret tool ──────────────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_secret_sql",
		Description: "Generate a CREATE SECRET DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateSecretInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql, err := secret.BuildCreateSecretSql(in.Database, in.Schema, in.Config)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	// ── Integration tools ────────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_storage_integration_sql",
		Description: "Generate a CREATE STORAGE INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.StorageIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildStorageIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_api_integration_sql",
		Description: "Generate a CREATE API INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.ApiIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildApiIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_catalog_integration_sql",
		Description: "Generate a CREATE CATALOG INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.CatalogIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildCatalogIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_external_access_integration_sql",
		Description: "Generate a CREATE EXTERNAL ACCESS INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.ExternalAccessIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildExternalAccessIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_notification_integration_sql",
		Description: "Generate a CREATE NOTIFICATION INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.NotificationIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildNotificationIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_security_integration_sql",
		Description: "Generate a CREATE SECURITY INTEGRATION DDL statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in integrations.SecurityIntegrationParams) (*mcpsdk.CallToolResult, any, error) {
		sql, err := integrations.BuildSecurityIntegrationSQL(in)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})
}
