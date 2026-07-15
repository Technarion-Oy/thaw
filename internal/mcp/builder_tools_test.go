// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/fileformat"
	"thaw/internal/integrations"
	"thaw/internal/pipe"
	"thaw/internal/secret"
	"thaw/internal/stage"
)

// TestBuilderToolsRegistered verifies that all 12 builder tools are registered
// in metadata, readonly, and explain_only modes.
func TestBuilderToolsRegistered(t *testing.T) {
	builderTools := []string{
		"build_create_stage_sql",
		"build_alter_stage_sql",
		"build_create_file_format_sql",
		"build_create_pipe_sql",
		"build_refresh_pipe_sql",
		"build_create_secret_sql",
		"build_storage_integration_sql",
		"build_api_integration_sql",
		"build_catalog_integration_sql",
		"build_external_access_integration_sql",
		"build_notification_integration_sql",
		"build_security_integration_sql",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range builderTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestBuildCreateStageSqlEmptyFields verifies that missing database/schema
// return errors.
func TestBuildCreateStageSqlEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing database.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_stage_sql",
		Arguments: stage.StageConfig{
			Name:   "MY_STAGE",
			Schema: "PUBLIC",
			Type:   "INTERNAL",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error message should mention database requirement, got: %s", text)
	}

	// Missing schema.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_stage_sql",
		Arguments: stage.StageConfig{
			Name:     "MY_STAGE",
			Database: "MYDB",
			Type:     "INTERNAL",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty schema")
	}
	text = extractText(t, res)
	if !strings.Contains(text, "schema is required") {
		t.Errorf("error message should mention schema requirement, got: %s", text)
	}
}

// TestBuildAlterStageSqlEmptyFields verifies that missing database/schema
// return errors.
func TestBuildAlterStageSqlEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing database.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_alter_stage_sql",
		Arguments: stage.AlterStageConfig{
			Name:   "MY_STAGE",
			Schema: "PUBLIC",
			Action: "RENAME",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error message should mention database requirement, got: %s", text)
	}

	// Missing schema.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_alter_stage_sql",
		Arguments: stage.AlterStageConfig{
			Name:     "MY_STAGE",
			Database: "MYDB",
			Action:   "RENAME",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty schema")
	}
	text = extractText(t, res)
	if !strings.Contains(text, "schema is required") {
		t.Errorf("error message should mention schema requirement, got: %s", text)
	}
}

// TestBuildCreateStageSqlSuccess verifies that a minimal stage config produces
// a CREATE STAGE statement.
func TestBuildCreateStageSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_stage_sql",
		Arguments: stage.StageConfig{
			Name:     "MY_STAGE",
			Database: "MYDB",
			Schema:   "PUBLIC",
			Type:     "INTERNAL",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "CREATE") {
		t.Errorf("expected CREATE in result, got: %s", text)
	}
	if !strings.Contains(text, "MY_STAGE") {
		t.Errorf("expected stage name in result, got: %s", text)
	}
}

// TestBuildAlterStageSqlSuccess verifies that an alter stage config produces
// an ALTER STAGE statement.
func TestBuildAlterStageSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_alter_stage_sql",
		Arguments: stage.AlterStageConfig{
			Name:     "MY_STAGE",
			Database: "MYDB",
			Schema:   "PUBLIC",
			Action:   "RENAME",
			NewName:  "NEW_STAGE",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "ALTER") {
		t.Errorf("expected ALTER in result, got: %s", text)
	}
}

// TestBuildCreateFileFormatEmptyDb verifies that missing db/schema returns an error.
func TestBuildCreateFileFormatEmptyDb(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing database.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_file_format_sql",
		Arguments: buildCreateFileFormatInput{
			Database: "",
			Schema:   "PUBLIC",
			Config:   fileformat.FileFormatConfig{Name: "FF", Type: "CSV"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error message should mention database requirement, got: %s", text)
	}

	// Missing schema.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_file_format_sql",
		Arguments: buildCreateFileFormatInput{
			Database: "MYDB",
			Schema:   "",
			Config:   fileformat.FileFormatConfig{Name: "FF", Type: "CSV"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty schema")
	}
	text = extractText(t, res)
	if !strings.Contains(text, "schema is required") {
		t.Errorf("error message should mention schema requirement, got: %s", text)
	}
}

// TestBuildCreatePipeSqlSuccess verifies that valid config with a COPY INTO
// statement produces SQL.
func TestBuildCreatePipeSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_pipe_sql",
		Arguments: buildCreatePipeInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Config: pipe.PipeConfig{
				Name:          "MY_PIPE",
				CopyStatement: "COPY INTO my_table FROM @my_stage",
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
	if !strings.Contains(strings.ToUpper(text), "CREATE") {
		t.Errorf("expected CREATE in result, got: %s", text)
	}
	if !strings.Contains(text, "MY_PIPE") {
		t.Errorf("expected pipe name in result, got: %s", text)
	}
}

// TestBuildRefreshPipeSqlEmptyFields verifies that missing db/schema/name
// return errors.
func TestBuildRefreshPipeSqlEmptyFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   buildRefreshPipeInput
		wantMsg string
	}{
		{"missing database", buildRefreshPipeInput{Schema: "PUBLIC", Name: "PIPE"}, "database is required"},
		{"missing schema", buildRefreshPipeInput{Database: "DB", Name: "PIPE"}, "schema is required"},
		{"missing name", buildRefreshPipeInput{Database: "DB", Schema: "PUBLIC"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "build_refresh_pipe_sql",
				Arguments: tc.input,
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
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

// TestBuildCreateSecretSqlInvalidType verifies that an invalid secret type
// propagates an error.
func TestBuildCreateSecretSqlInvalidType(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_create_secret_sql",
		Arguments: buildCreateSecretInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Config: secret.SecretConfig{
				Name: "MY_SECRET",
				Type: "INVALID_TYPE",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for invalid secret type")
	}
}

// TestBuildStorageIntegrationSqlSuccess verifies that valid params produce
// a CREATE STORAGE INTEGRATION statement.
func TestBuildStorageIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_storage_integration_sql",
		Arguments: integrations.StorageIntegrationParams{
			Name:             "MY_INTEGRATION",
			Enabled:          true,
			Provider:         "S3",
			AwsRoleArn:       "arn:aws:iam::role/myrole",
			AllowedLocations: "s3://mybucket/",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "STORAGE INTEGRATION") {
		t.Errorf("expected STORAGE INTEGRATION in result, got: %s", text)
	}
}

// TestBuildApiIntegrationSqlSuccess verifies that valid params produce
// a CREATE API INTEGRATION statement.
func TestBuildApiIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_api_integration_sql",
		Arguments: integrations.ApiIntegrationParams{
			Name:            "MY_API",
			Enabled:         true,
			Provider:        "AWS_API_GATEWAY",
			AllowedPrefixes: "https://api.example.com",
			AwsRoleArn:      "arn:aws:iam::role/myrole",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "API INTEGRATION") {
		t.Errorf("expected API INTEGRATION in result, got: %s", text)
	}
}

// TestBuildCatalogIntegrationSqlSuccess verifies that valid params produce
// a CREATE CATALOG INTEGRATION statement.
func TestBuildCatalogIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_catalog_integration_sql",
		Arguments: integrations.CatalogIntegrationParams{
			Name:    "MY_CATALOG",
			Enabled: true,
			Source:  "GLUE",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "CATALOG INTEGRATION") {
		t.Errorf("expected CATALOG INTEGRATION in result, got: %s", text)
	}
}

// TestBuildExternalAccessIntegrationSqlSuccess verifies that valid params
// produce a CREATE EXTERNAL ACCESS INTEGRATION statement.
func TestBuildExternalAccessIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_external_access_integration_sql",
		Arguments: integrations.ExternalAccessIntegrationParams{
			Name:                "MY_EXT_ACCESS",
			Enabled:             true,
			AllowedNetworkRules: "MY_RULE",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "EXTERNAL ACCESS INTEGRATION") {
		t.Errorf("expected EXTERNAL ACCESS INTEGRATION in result, got: %s", text)
	}
}

// TestBuildNotificationIntegrationSqlSuccess verifies that valid params
// produce a CREATE NOTIFICATION INTEGRATION statement.
func TestBuildNotificationIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_notification_integration_sql",
		Arguments: integrations.NotificationIntegrationParams{
			Name:           "MY_NOTIFICATION",
			Enabled:        true,
			Subtype:        "AWS_SNS_OUTBOUND",
			AwsSnsTopicArn: "arn:aws:sns:us-east-1:123:my-topic",
			AwsSnsRoleArn:  "arn:aws:iam::role/myrole",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "NOTIFICATION INTEGRATION") {
		t.Errorf("expected NOTIFICATION INTEGRATION in result, got: %s", text)
	}
}

// TestBuildSecurityIntegrationSqlSuccess verifies that valid params produce
// a CREATE SECURITY INTEGRATION statement.
func TestBuildSecurityIntegrationSqlSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "build_security_integration_sql",
		Arguments: integrations.SecurityIntegrationParams{
			Name:    "MY_SECURITY",
			Enabled: true,
			SecType: "EXTERNAL_OAUTH",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(strings.ToUpper(text), "SECURITY INTEGRATION") {
		t.Errorf("expected SECURITY INTEGRATION in result, got: %s", text)
	}
}
