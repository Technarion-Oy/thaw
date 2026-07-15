// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/filesystem"
	"thaw/internal/logger"
)

// NotebookBackend abstracts the kernel-dependent notebook operations so that
// the MCP package does not import internal/snowpark. The adapter lives in
// internal/app and delegates to snowpark.Service.
type NotebookBackend interface {
	GetNotebookCompletions(tabId, code string, line, col int) ([]NotebookCompletion, error)
	// CheckPythonSyntax validates Python code. The MCP tool handler always
	// passes mode="static" to restrict to static analysis only — kernel-based
	// diagnostics are not exposed to MCP clients because they can execute code.
	CheckPythonSyntax(tabId, code, mode string) ([]NotebookSyntaxError, error)
}

// MCP-local type duplicates — mirror the snowpark types without importing
// the snowpark package. The adapter in internal/app maps between them.

// NotebookCompletion is a single intellisense completion item.
type NotebookCompletion struct {
	Label         string `json:"label"`
	Type          string `json:"type"`
	Detail        string `json:"detail"`
	Documentation string `json:"documentation"`
}

// NotebookSyntaxError describes a single Python diagnostic.
type NotebookSyntaxError struct {
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	EndCol   *int   `json:"endCol"`
	Msg      string `json:"msg"`
}

// OpenNotebookTabPayload is the Wails event payload for "mcp:open-notebook-tab".
type OpenNotebookTabPayload struct {
	Title   string `json:"title"`
	Content string `json:"content"` // nbformat v4 JSON
}

// Tool input types for notebook tools.

type readNotebookInput struct {
	Path string `json:"path" jsonschema:"path to the .ipynb file within the workspace"`
}

type getNotebookCompletionsInput struct {
	TabID string `json:"tab_id" jsonschema:"the notebook tab ID"`
	Code  string `json:"code" jsonschema:"the Python source code"`
	Line  int    `json:"line" jsonschema:"1-indexed line number"`
	Col   int    `json:"col" jsonschema:"0-indexed column number"`
}

type checkPythonSyntaxInput struct {
	TabID string `json:"tab_id" jsonschema:"the notebook tab ID"`
	Code  string `json:"code" jsonschema:"the Python source code to check"`
}

type notebookCellInput struct {
	Kind   string `json:"kind" jsonschema:"cell kind: python, markdown, or sql"`
	Source string `json:"source" jsonschema:"the cell source text"`
}

type openNotebookTabInput struct {
	Title string              `json:"title,omitempty" jsonschema:"tab title (default: AI Notebook)"`
	Cells []notebookCellInput `json:"cells" jsonschema:"the notebook cells"`
}

// registerNotebookTools wires the notebook/Snowpark tools onto srv.
// read_notebook requires a workspace root; kernel-dependent tools require
// a non-nil NotebookBackend; open_notebook_tab requires a non-nil emit.
func registerNotebookTools(srv *mcpsdk.Server, nb NotebookBackend, workspaceRoot string, emit func(string, interface{})) {

	// ── read_notebook (workspace-gated) ──────────────────────────────────
	if workspaceRoot != "" {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "read_notebook",
			Description: "Read a Jupyter notebook (.ipynb) file from the workspace. " +
				"Returns the raw JSON content (up to 5 MB). The file must be inside the configured workspace root " +
				"and have a .ipynb extension.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in readNotebookInput) (*mcpsdk.CallToolResult, any, error) {
			if in.Path == "" {
				return nil, nil, fmt.Errorf("path is required")
			}
			if !strings.HasSuffix(in.Path, ".ipynb") {
				return nil, nil, fmt.Errorf("path must have a .ipynb extension")
			}
			if err := filesystem.ValidateInsideOrEqual(in.Path, workspaceRoot); err != nil {
				return nil, nil, fmt.Errorf("access denied: %w", err)
			}
			const maxBytes int64 = 5 * 1024 * 1024 // 5 MB
			f, err := os.Open(in.Path)
			if err != nil {
				return nil, nil, err
			}
			defer func() { _ = f.Close() }()
			// Read up to maxBytes+1 in a single open to avoid TOCTOU between
			// stat and read. If we get more than maxBytes, the file is too large.
			data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
			if err != nil {
				return nil, nil, err
			}
			if int64(len(data)) > maxBytes {
				return nil, nil, fmt.Errorf("file exceeds 5 MB limit")
			}
			return textResult(string(data)), nil, nil
		})
	}

	// ── kernel-dependent tools (backend-gated) ───────────────────────────
	if nb != nil {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "get_notebook_completions",
			Description: "Get Python intellisense completions from the running notebook kernel at the given cursor position. " +
				"Returns a list of completion items with label, type, detail, and documentation.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in getNotebookCompletionsInput) (*mcpsdk.CallToolResult, any, error) {
			if in.TabID == "" {
				return nil, nil, fmt.Errorf("tab_id is required")
			}
			if in.Code == "" {
				return nil, nil, fmt.Errorf("code is required")
			}
			if in.Line < 1 {
				return nil, nil, fmt.Errorf("line must be >= 1 (1-indexed)")
			}
			completions, err := nb.GetNotebookCompletions(in.TabID, in.Code, in.Line, in.Col)
			if err != nil {
				return nil, nil, err
			}
			if completions == nil {
				completions = []NotebookCompletion{}
			}
			return jsonResult(completions), nil, nil
		})

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "check_python_syntax",
			Description: "Validate Python syntax and check for common errors using static analysis (no code execution). " +
				"Returns a list of diagnostics with severity, line, column, and message.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in checkPythonSyntaxInput) (*mcpsdk.CallToolResult, any, error) {
			if in.TabID == "" {
				return nil, nil, fmt.Errorf("tab_id is required")
			}
			if in.Code == "" {
				return nil, nil, fmt.Errorf("code is required")
			}
			errors, err := nb.CheckPythonSyntax(in.TabID, in.Code, "static")
			if err != nil {
				return nil, nil, err
			}
			if errors == nil {
				errors = []NotebookSyntaxError{}
			}
			return jsonResult(errors), nil, nil
		})
	}

	// ── open_notebook_tab (emit-gated) ───────────────────────────────────
	if emit != nil {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "open_notebook_tab",
			Description: "Open a new notebook tab in Thaw with pre-filled cells. " +
				"Supported cell kinds: python, markdown, sql. " +
				"The user must manually run cells (human-in-the-loop). " +
				"Returns a success message.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in openNotebookTabInput) (*mcpsdk.CallToolResult, any, error) {
			if len(in.Cells) == 0 {
				return nil, nil, fmt.Errorf("cells is required and must not be empty")
			}

			// Validate cell kinds.
			for i, cell := range in.Cells {
				switch cell.Kind {
				case "python", "markdown", "sql":
				default:
					return nil, nil, fmt.Errorf("cells[%d].kind %q is invalid (must be python, markdown, or sql)", i, cell.Kind)
				}
			}

			title := in.Title
			if title == "" {
				title = "AI Notebook"
			}
			if utf8.RuneCountInString(title) > 100 {
				title = string([]rune(title)[:100])
			}

			content, err := buildNbformat(in.Cells)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to build notebook: %w", err)
			}

			// Emit the event to the frontend. Recover from panics so that a
			// torn-down Wails context during shutdown doesn't kill the MCP
			// server goroutine.
			var emitFailed bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.L.Error("mcp open_notebook_tab: emit panicked", "err", r)
						emitFailed = true
					}
				}()
				emit("mcp:open-notebook-tab", OpenNotebookTabPayload{
					Title:   title,
					Content: content,
				})
			}()
			if emitFailed {
				return textResult("Failed to open notebook tab: internal error"), nil, nil
			}

			return textResult("Notebook tab opened successfully."), nil, nil
		})
	}
}

// nbformatNotebook is the top-level structure of a Jupyter notebook (nbformat v4).
type nbformatNotebook struct {
	Cells         []nbformatCell    `json:"cells"`
	Metadata      map[string]any    `json:"metadata"`
	Nbformat      int               `json:"nbformat"`
	NbformatMinor int               `json:"nbformat_minor"`
}

// nbformatCell is a single cell in an nbformat v4 notebook.
// Outputs and ExecutionCount use omitempty so they are only present on code
// cells (set explicitly in buildNbformat). Markdown and raw cells omit them.
type nbformatCell struct {
	CellType       string         `json:"cell_type"`
	Source         string         `json:"source"`
	Metadata       map[string]any `json:"metadata"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
	ExecutionCount json.RawMessage `json:"execution_count,omitempty"`
}

// buildNbformat builds a Jupyter nbformat v4 JSON string from the given cells.
func buildNbformat(cells []notebookCellInput) (string, error) {
	nb := nbformatNotebook{
		Cells:         make([]nbformatCell, 0, len(cells)),
		Metadata:      map[string]any{},
		Nbformat:      4,
		NbformatMinor: 5,
	}

	for _, c := range cells {
		var cellType string
		meta := map[string]any{}

		switch c.Kind {
		case "python":
			cellType = "code"
		case "markdown":
			cellType = "markdown"
		case "sql":
			cellType = "raw"
			meta["thaw_cell_type"] = "sql"
		}

		cell := nbformatCell{
			CellType: cellType,
			Source:   c.Source,
			Metadata: meta,
		}
		// nbformat v4 requires outputs and execution_count on code cells.
		if cellType == "code" {
			cell.Outputs = json.RawMessage(`[]`)
			cell.ExecutionCount = json.RawMessage(`null`)
		}

		nb.Cells = append(nb.Cells, cell)
	}

	b, err := json.Marshal(nb)
	if err != nil {
		return "", fmt.Errorf("marshal notebook: %w", err)
	}
	return string(b), nil
}
