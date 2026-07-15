// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/filesystem"
	"thaw/internal/gitrepo"
	"thaw/internal/sqleditor"
)

// Tool input types for workspace (local filesystem / git) tools.

type dirInput struct {
	Dir string `json:"dir" jsonschema:"the directory path"`
}

type pathInput struct {
	Path string `json:"path" jsonschema:"the file path"`
}

type diffLinesInput struct {
	HeadLines    []string `json:"head_lines" jsonschema:"lines from the HEAD revision"`
	CurrentLines []string `json:"current_lines" jsonschema:"lines from the current working copy"`
}

type searchFilesInput struct {
	Dir      string `json:"dir" jsonschema:"the directory to search in"`
	Query    string `json:"query" jsonschema:"the search query (substring or regex)"`
	UseRegex bool   `json:"use_regex,omitempty" jsonschema:"if true, treat query as a regular expression (default false)"`
}

// registerWorkspaceTools wires local filesystem and git read-only tools onto
// srv. These tools do NOT require a Snowflake client — they operate on the
// local filesystem and git repositories only. All path inputs are validated
// against workspaceRoot using symlink-resolving defense-in-depth checks.
// Only registered when workspaceRoot is non-empty (see buildServer).
func registerWorkspaceTools(srv *mcpsdk.Server, workspaceRoot string) {

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "git_status",
		Description: "Return the git status for a directory within the workspace: branch, modified/added/deleted files, remote info, ahead count. Non-repo directories return isRepo=false. The directory must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in dirInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Dir == "" {
			return nil, nil, fmt.Errorf("dir is required")
		}
		if err := filesystem.ValidateInsideOrEqual(in.Dir, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		status, err := gitrepo.GetStatus(in.Dir)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(status), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "git_list_branches",
		Description: "List all local and remote branches in the git repository at the given directory. The directory must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in dirInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Dir == "" {
			return nil, nil, fmt.Errorf("dir is required")
		}
		if err := filesystem.ValidateInsideOrEqual(in.Dir, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		branches, err := gitrepo.ListBranches(in.Dir)
		if err != nil {
			return nil, nil, err
		}
		if branches == nil {
			branches = []gitrepo.BranchInfo{}
		}
		return jsonResult(branches), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "git_get_head_file",
		Description: "Return the content of a file as it exists in the HEAD commit. Returns empty string for new/untracked files. The file must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in pathInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Path == "" {
			return nil, nil, fmt.Errorf("path is required")
		}
		if err := filesystem.ValidatePathOrAncestorInsideOrEqual(in.Path, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		content, err := gitrepo.GetHeadFileContent(in.Path)
		if err != nil {
			return nil, nil, err
		}
		return textResult(content), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "git_diff_lines",
		Description: "Compute a line-level diff between HEAD and current file content. " +
			"Returns lists of added, modified, and deleted line numbers.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in diffLinesInput) (*mcpsdk.CallToolResult, any, error) {
		if in.HeadLines == nil {
			return nil, nil, fmt.Errorf("head_lines is required")
		}
		if in.CurrentLines == nil {
			return nil, nil, fmt.Errorf("current_lines is required")
		}
		const maxLines = 10000
		diff := sqleditor.ComputeGitLineDiff(in.HeadLines, in.CurrentLines, maxLines)
		return jsonResult(diff), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_directory",
		Description: "List the direct children (files and directories) of a directory, with name, path, size, and type. The directory must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in dirInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Dir == "" {
			return nil, nil, fmt.Errorf("dir is required")
		}
		if err := filesystem.ValidateInsideOrEqual(in.Dir, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		entries, err := filesystem.ListDir(in.Dir)
		if err != nil {
			return nil, nil, err
		}
		if entries == nil {
			entries = []filesystem.FileEntry{}
		}
		return jsonResult(entries), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "read_file",
		Description: "Read the content of a file (up to 50 KB) within the workspace. Returns the text content or an error if the file cannot be read. The file must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in pathInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Path == "" {
			return nil, nil, fmt.Errorf("path is required")
		}
		if err := filesystem.ValidateInsideOrEqual(in.Path, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		const maxBytes = 50 * 1024
		content, err := filesystem.ReadFileHead(in.Path, maxBytes)
		if err != nil {
			return nil, nil, err
		}
		return textResult(content), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "search_files",
		Description: "Search for a query string (or regex) across all files in a directory within the workspace, recursively. " +
			"Returns matching lines with file path, line number, and match positions. Hidden directories are skipped. The directory must be inside the configured workspace root.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in searchFilesInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Dir == "" {
			return nil, nil, fmt.Errorf("dir is required")
		}
		if in.Query == "" {
			return nil, nil, fmt.Errorf("query is required")
		}
		if err := filesystem.ValidateInsideOrEqual(in.Dir, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		matches, err := filesystem.SearchFiles(in.Dir, in.Query, in.UseRegex)
		if err != nil {
			return nil, nil, err
		}
		if matches == nil {
			matches = []filesystem.SearchMatch{}
		}
		return jsonResult(matches), nil, nil
	})
}
