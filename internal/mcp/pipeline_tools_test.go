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

// TestPipelineToolsRegistered verifies that the 6 always-on pipeline tools are
// registered in metadata, readonly, and explain_only modes.
func TestPipelineToolsRegistered(t *testing.T) {
	alwaysOn := []string{
		"list_tasks",
		"get_task_run_history",
		"get_task_dependencies",
		"list_stage_files",
		"get_pipe_status",
		"get_pipe_copy_history",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range alwaysOn {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestPreviewStageFileModeGated verifies that preview_stage_file is absent in
// metadata mode but present in readonly and explain_only modes.
func TestPreviewStageFileModeGated(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
	names := toolNames(t, srv)
	if hasToolName(names, "preview_stage_file") {
		t.Error("preview_stage_file should not be registered in metadata mode")
	}

	for _, mode := range []string{ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil)
			names := toolNames(t, srv)
			if !hasToolName(names, "preview_stage_file") {
				t.Errorf("preview_stage_file should be registered in %s mode (got: %v)", mode, names)
			}
		})
	}
}

// TestOpenTaskGraphNotRegisteredWithNilEmit verifies that open_task_graph is
// not registered when emit is nil.
func TestOpenTaskGraphNotRegisteredWithNilEmit(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil)
	names := toolNames(t, srv)
	if hasToolName(names, "open_task_graph") {
		t.Error("open_task_graph should not be registered when emit is nil")
	}
}

// TestOpenTaskGraphRegisteredWithEmit verifies that open_task_graph is
// registered when a non-nil emit function is provided.
func TestOpenTaskGraphRegisteredWithEmit(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit)
	names := toolNames(t, srv)
	if !hasToolName(names, "open_task_graph") {
		t.Errorf("open_task_graph should be registered when emit is non-nil (got: %v)", names)
	}
}

// TestListTasksNilClient verifies the tool returns an error when no Snowflake
// client is available.
func TestListTasksNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_tasks",
		Arguments: listTasksInput{Database: "DB", Schema: "PUBLIC"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestGetPipeStatusNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestGetPipeStatusNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_pipe_status",
		Arguments: pipeStatusInput{Database: "DB", Schema: "PUBLIC", Name: "MY_PIPE"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestListTasksEmptyInputs verifies that empty database and schema return errors.
func TestListTasksEmptyInputs(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing database.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_tasks",
		Arguments: listTasksInput{Database: "", Schema: "PUBLIC"},
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
		Name:      "list_tasks",
		Arguments: listTasksInput{Database: "DB", Schema: ""},
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

// TestGetTaskRunHistoryEmptyName verifies that an empty task name returns an error.
func TestGetTaskRunHistoryEmptyName(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_task_run_history",
		Arguments: taskRunHistoryInput{Database: "DB", Schema: "PUBLIC", Name: ""},
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

// TestGetTaskDependenciesEmptyTask verifies that an empty task returns an error.
func TestGetTaskDependenciesEmptyTask(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_task_dependencies",
		Arguments: taskDependenciesInput{Database: "DB", Schema: "PUBLIC", Task: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty task")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "task is required") {
		t.Errorf("error message should mention task requirement, got: %s", text)
	}
}

// TestListStageFilesEmptyStage verifies that an empty stage returns an error.
func TestListStageFilesEmptyStage(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_stage_files",
		Arguments: listStageFilesInput{Stage: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty stage")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "stage is required") {
		t.Errorf("error message should mention stage requirement, got: %s", text)
	}
}

// TestGetPipeStatusEmptyInputs verifies that empty database, schema, and name
// return errors.
func TestGetPipeStatusEmptyInputs(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	// Missing database.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_pipe_status",
		Arguments: pipeStatusInput{Database: "", Schema: "PUBLIC", Name: "MY_PIPE"},
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

	// Missing name.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_pipe_status",
		Arguments: pipeStatusInput{Database: "DB", Schema: "PUBLIC", Name: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty name")
	}
	text = extractText(t, res)
	if !strings.Contains(text, "name is required") {
		t.Errorf("error message should mention name requirement, got: %s", text)
	}
}

// TestGetPipeCopyHistoryEmptyInputs verifies that empty required fields return errors.
func TestGetPipeCopyHistoryEmptyInputs(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_pipe_copy_history",
		Arguments: pipeCopyHistoryInput{Database: "", Schema: "PUBLIC", Name: "MY_PIPE"},
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
}
