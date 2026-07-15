// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestLineageToolsRegistered verifies that all 3 lineage tools are registered
// on a server built with a nil client (tool listing does not invoke handlers).
func TestLineageToolsRegistered(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	expected := []string{"get_object_lineage", "get_schema_cross_deps", "get_database_cross_deps"}
	for _, name := range expected {
		if !hasToolName(names, name) {
			t.Errorf("expected tool %q to be registered, got tools: %v", name, names)
		}
	}
}

// TestLineageToolsRegisteredInAllModes verifies that lineage tools are present
// in metadata, readonly, and explain_only modes.
func TestLineageToolsRegisteredInAllModes(t *testing.T) {
	lineageTools := []string{"get_object_lineage", "get_schema_cross_deps", "get_database_cross_deps"}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range lineageTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered", mode, tool)
				}
			}
		})
	}
}

// TestGetObjectLineageNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestGetObjectLineageNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "get_object_lineage",
		Arguments: objectLineageInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Kind:     "VIEW",
			Name:     "V_USERS",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetObjectLineageMissingFields verifies the tool rejects requests with
// missing required fields.
func TestGetObjectLineageMissingFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   objectLineageInput
		wantMsg string
	}{
		{"missing database", objectLineageInput{Schema: "PUBLIC", Kind: "VIEW", Name: "V"}, "database is required"},
		{"missing schema", objectLineageInput{Database: "DB", Kind: "VIEW", Name: "V"}, "schema is required"},
		{"missing name", objectLineageInput{Database: "DB", Schema: "PUBLIC", Kind: "VIEW"}, "name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_object_lineage",
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

// TestGetObjectLineageInvalidKind verifies the tool rejects unsupported object
// kinds.
func TestGetObjectLineageInvalidKind(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	for _, kind := range []string{"TABLE", "STAGE", "STREAM", "", "view"} {
		t.Run(kind, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name: "get_object_lineage",
				Arguments: objectLineageInput{
					Database: "DB",
					Schema:   "PUBLIC",
					Kind:     kind,
					Name:     "OBJ",
				},
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Errorf("expected IsError=true for kind %q", kind)
			}
		})
	}
}

// TestAllowedLineageKinds verifies the kind whitelist accepts the expected
// kinds and rejects others.
func TestAllowedLineageKinds(t *testing.T) {
	for _, kind := range []string{"VIEW", "PROCEDURE", "FUNCTION"} {
		if !allowedLineageKinds[kind] {
			t.Errorf("expected %q to be allowed", kind)
		}
	}
	for _, kind := range []string{"", "TABLE", "STAGE", "view", "EVIL"} {
		if allowedLineageKinds[kind] {
			t.Errorf("expected %q to be rejected", kind)
		}
	}
}

// TestGetSchemaCrossDepsNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestGetSchemaCrossDepsNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_schema_cross_deps",
		Arguments: schemaInput{Database: "DB", Schema: "PUBLIC"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetSchemaCrossDepsMissingFields verifies the tool rejects missing fields.
func TestGetSchemaCrossDepsMissingFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   schemaInput
		wantMsg string
	}{
		{"missing database", schemaInput{Schema: "PUBLIC"}, "database is required"},
		{"missing schema", schemaInput{Database: "DB"}, "schema is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_schema_cross_deps",
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

// TestGetDatabaseCrossDepsNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestGetDatabaseCrossDepsNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "get_database_cross_deps",
		Arguments: databaseCrossDepsInput{
			Database: "DB",
			Schemas:  []string{"PUBLIC"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetDatabaseCrossDepsMissingFields verifies the tool rejects missing fields.
func TestGetDatabaseCrossDepsMissingFields(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   databaseCrossDepsInput
		wantMsg string
	}{
		{"missing database", databaseCrossDepsInput{Schemas: []string{"PUBLIC"}}, "database is required"},
		{"missing schemas", databaseCrossDepsInput{Database: "DB"}, "schemas is required"},
		{"empty schemas", databaseCrossDepsInput{Database: "DB", Schemas: []string{}}, "schemas is required"},
		{"too many schemas", databaseCrossDepsInput{Database: "DB", Schemas: make([]string, 21)}, "too many schemas"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_database_cross_deps",
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
