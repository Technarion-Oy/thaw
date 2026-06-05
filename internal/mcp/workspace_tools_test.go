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
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/filesystem"
	"thaw/internal/gitrepo"
	"thaw/internal/sqleditor"
)

// TestWorkspaceToolsRegistered verifies that all 7 workspace tools are
// registered in metadata, readonly, and explain_only modes.
func TestWorkspaceToolsRegistered(t *testing.T) {
	workspaceTools := []string{
		"git_status",
		"git_list_branches",
		"git_get_head_file",
		"git_diff_lines",
		"list_directory",
		"read_file",
		"search_files",
	}

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(nil, mode, SessionConfig{}, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range workspaceTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestGitStatusEmptyDir verifies that an empty dir returns an error.
func TestGitStatusEmptyDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "git_status",
		Arguments: dirInput{Dir: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty dir")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "dir is required") {
		t.Errorf("error should mention dir requirement, got: %s", text)
	}
}

// TestGitStatusNonRepo verifies that git_status on a non-repo directory
// returns isRepo=false without an error.
func TestGitStatusNonRepo(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()
	tmp := t.TempDir()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "git_status",
		Arguments: dirInput{Dir: tmp},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	var status gitrepo.RepoStatus
	if err := json.Unmarshal([]byte(extractText(t, res)), &status); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if status.IsRepo {
		t.Error("expected isRepo=false for non-repo directory")
	}
}

// TestGitListBranchesEmptyDir verifies input validation.
func TestGitListBranchesEmptyDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "git_list_branches",
		Arguments: dirInput{Dir: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty dir")
	}
}

// TestGitGetHeadFileEmptyPath verifies input validation.
func TestGitGetHeadFileEmptyPath(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "git_get_head_file",
		Arguments: pathInput{Path: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty path")
	}
}

// TestGitDiffLinesKnownInput verifies git_diff_lines with known inputs.
func TestGitDiffLinesKnownInput(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "git_diff_lines",
		Arguments: diffLinesInput{
			HeadLines:    []string{"line1", "line2", "line3"},
			CurrentLines: []string{"line1", "modified", "line3", "line4"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	var diff sqleditor.LineDiff
	if err := json.Unmarshal([]byte(extractText(t, res)), &diff); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// line2→modified is a modification, line4 is an addition
	if len(diff.Modified) == 0 && len(diff.Added) == 0 {
		t.Error("expected at least one modified or added line")
	}
}

// TestGitDiffLinesNilHeadLines verifies that nil head_lines returns an error.
func TestGitDiffLinesNilHeadLines(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "git_diff_lines",
		// Only send current_lines, omitting head_lines entirely.
		Arguments: map[string]any{"current_lines": []string{"a"}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil head_lines")
	}
}

// TestListDirectoryEmptyDir verifies input validation.
func TestListDirectoryEmptyDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_directory",
		Arguments: dirInput{Dir: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty dir")
	}
}

// TestListDirectoryTempDir verifies list_directory returns correct entries.
func TestListDirectoryTempDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_directory",
		Arguments: dirInput{Dir: tmp},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	var entries []filesystem.FileEntry
	if err := json.Unmarshal([]byte(extractText(t, res)), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	// Directories come first.
	if entries[0].Name != "subdir" || !entries[0].IsDir {
		t.Errorf("entries[0] = %+v, want subdir dir", entries[0])
	}
	if entries[1].Name != "hello.txt" || entries[1].IsDir {
		t.Errorf("entries[1] = %+v, want hello.txt file", entries[1])
	}
}

// TestReadFileEmptyPath verifies input validation.
func TestReadFileEmptyPath(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "read_file",
		Arguments: pathInput{Path: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty path")
	}
}

// TestReadFileTempFile verifies read_file returns the file content.
func TestReadFileTempFile(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "test.sql")
	content := "SELECT 1;\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "read_file",
		Arguments: pathInput{Path: filePath},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if text != content {
		t.Errorf("read_file returned %q, want %q", text, content)
	}
}

// TestSearchFilesEmptyDir verifies input validation for empty dir.
func TestSearchFilesEmptyDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_files",
		Arguments: searchFilesInput{Dir: "", Query: "test"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty dir")
	}
}

// TestSearchFilesEmptyQuery verifies input validation for empty query.
func TestSearchFilesEmptyQuery(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_files",
		Arguments: searchFilesInput{Dir: "/tmp", Query: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty query")
	}
}

// TestSearchFilesTempDir verifies search_files finds content in a temp directory.
func TestSearchFilesTempDir(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.sql"), []byte("SELECT * FROM orders;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("no match here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_files",
		Arguments: searchFilesInput{Dir: tmp, Query: "orders"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	var matches []filesystem.SearchMatch
	if err := json.Unmarshal([]byte(extractText(t, res)), &matches); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if !strings.HasSuffix(matches[0].Path, "a.sql") {
		t.Errorf("match path = %q, want suffix a.sql", matches[0].Path)
	}
}
