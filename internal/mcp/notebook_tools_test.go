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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockNotebookBackend implements NotebookBackend for testing.
type mockNotebookBackend struct {
	completionsFunc func(tabId, code string, line, col int) ([]NotebookCompletion, error)
	syntaxCheckFunc func(tabId, code, mode string) ([]NotebookSyntaxError, error)
}

func (m *mockNotebookBackend) GetNotebookCompletions(tabId, code string, line, col int) ([]NotebookCompletion, error) {
	if m.completionsFunc != nil {
		return m.completionsFunc(tabId, code, line, col)
	}
	return []NotebookCompletion{{Label: "test", Type: "function"}}, nil
}

func (m *mockNotebookBackend) CheckPythonSyntax(tabId, code, mode string) ([]NotebookSyntaxError, error) {
	if m.syntaxCheckFunc != nil {
		return m.syntaxCheckFunc(tabId, code, mode)
	}
	return []NotebookSyntaxError{}, nil
}

// ── Registration gating tests ────────────────────────────────────────────────

func TestNotebookToolsNotRegisteredWhenNilBackend(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	kernelTools := []string{"get_notebook_completions", "check_python_syntax"}
	for _, tool := range kernelTools {
		if hasToolName(names, tool) {
			t.Errorf("%q should not be registered when NotebookBackend is nil", tool)
		}
	}
}

func TestNotebookToolsRegisteredWithBackend(t *testing.T) {
	nb := &mockNotebookBackend{}
	emit := func(string, interface{}) {}
	cfg := SessionConfig{WorkspaceRoot: t.TempDir()}
	srv := buildServer(nil, ExecutionModeMetadata, cfg, nil, emit, nil, nb)
	names := toolNames(t, srv)

	expected := []string{
		"read_notebook",
		"get_notebook_completions",
		"check_python_syntax",
		"open_notebook_tab",
	}
	for _, tool := range expected {
		if !hasToolName(names, tool) {
			t.Errorf("%q should be registered (got: %v)", tool, names)
		}
	}
}

func TestReadNotebookNotRegisteredWithoutWorkspace(t *testing.T) {
	nb := &mockNotebookBackend{}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nb)
	names := toolNames(t, srv)

	if hasToolName(names, "read_notebook") {
		t.Error("read_notebook should not be registered when WorkspaceRoot is empty")
	}
}

func TestOpenNotebookTabNotRegisteredWithNilEmit(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	if hasToolName(names, "open_notebook_tab") {
		t.Error("open_notebook_tab should not be registered when emit is nil")
	}
}

// ── open_notebook_tab tests ──────────────────────────────────────────────────

func TestOpenNotebookTabEmptyCells(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "open_notebook_tab",
		Arguments: openNotebookTabInput{Cells: []notebookCellInput{}},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty cells, got false")
	}
}

func TestOpenNotebookTabInvalidCellKind(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_notebook_tab",
		Arguments: openNotebookTabInput{
			Cells: []notebookCellInput{{Kind: "java", Source: "x"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for invalid cell kind, got false")
	}
}

func TestOpenNotebookTabEmitsEvent(t *testing.T) {
	var mu sync.Mutex
	var emittedName string
	var emittedData interface{}
	emit := func(name string, data interface{}) {
		mu.Lock()
		emittedName = name
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_notebook_tab",
		Arguments: openNotebookTabInput{
			Title: "Test NB",
			Cells: []notebookCellInput{
				{Kind: "python", Source: "print('hello')"},
				{Kind: "markdown", Source: "# Title"},
				{Kind: "sql", Source: "SELECT 1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		text := extractText(t, res)
		t.Fatalf("unexpected error: %s", text)
	}

	mu.Lock()
	defer mu.Unlock()

	if emittedName != "mcp:open-notebook-tab" {
		t.Errorf("emitted event name = %q, want %q", emittedName, "mcp:open-notebook-tab")
	}

	payload, ok := emittedData.(OpenNotebookTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenNotebookTabPayload", emittedData)
	}
	if payload.Title != "Test NB" {
		t.Errorf("payload.Title = %q, want %q", payload.Title, "Test NB")
	}
	if payload.Content == "" {
		t.Error("payload.Content should not be empty")
	}

	text := extractText(t, res)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success message, got: %s", text)
	}
}

func TestOpenNotebookTabDefaultTitle(t *testing.T) {
	var mu sync.Mutex
	var emittedData interface{}
	emit := func(_ string, data interface{}) {
		mu.Lock()
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	_, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_notebook_tab",
		Arguments: openNotebookTabInput{
			Cells: []notebookCellInput{{Kind: "python", Source: "x=1"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	payload, ok := emittedData.(OpenNotebookTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenNotebookTabPayload", emittedData)
	}
	if payload.Title != "AI Notebook" {
		t.Errorf("payload.Title = %q, want %q", payload.Title, "AI Notebook")
	}
}

func TestOpenNotebookTabTitleTruncation(t *testing.T) {
	var mu sync.Mutex
	var emittedData interface{}
	emit := func(_ string, data interface{}) {
		mu.Lock()
		emittedData = data
		mu.Unlock()
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// Build a title with 105 runes including multi-byte characters.
	longTitle := strings.Repeat("日本語テスト", 21) // 105 runes

	_, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_notebook_tab",
		Arguments: openNotebookTabInput{
			Title: longTitle,
			Cells: []notebookCellInput{{Kind: "python", Source: "x=1"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	payload, ok := emittedData.(OpenNotebookTabPayload)
	if !ok {
		t.Fatalf("emitted data is %T, want OpenNotebookTabPayload", emittedData)
	}
	runes := []rune(payload.Title)
	if len(runes) != 100 {
		t.Errorf("payload.Title has %d runes, want 100", len(runes))
	}
}

// ── buildNbformat tests ──────────────────────────────────────────────────────

func TestBuildNbformatCellKindMapping(t *testing.T) {
	cells := []notebookCellInput{
		{Kind: "python", Source: "print('hello')"},
		{Kind: "markdown", Source: "# Title"},
		{Kind: "sql", Source: "SELECT 1"},
	}

	raw, err := buildNbformat(cells)
	if err != nil {
		t.Fatalf("buildNbformat: %v", err)
	}

	var nb nbformatNotebook
	if err := json.Unmarshal([]byte(raw), &nb); err != nil {
		t.Fatalf("buildNbformat output is not valid JSON: %v", err)
	}

	if nb.Nbformat != 4 {
		t.Errorf("nbformat = %d, want 4", nb.Nbformat)
	}
	if len(nb.Cells) != 3 {
		t.Fatalf("cell count = %d, want 3", len(nb.Cells))
	}

	// Python → code with outputs and execution_count
	if nb.Cells[0].CellType != "code" {
		t.Errorf("python cell_type = %q, want %q", nb.Cells[0].CellType, "code")
	}
	if nb.Cells[0].Source != "print('hello')" {
		t.Errorf("python source = %q", nb.Cells[0].Source)
	}
	if string(nb.Cells[0].Outputs) != "[]" {
		t.Errorf("python cell outputs = %s, want []", nb.Cells[0].Outputs)
	}
	if string(nb.Cells[0].ExecutionCount) != "null" {
		t.Errorf("python cell execution_count = %s, want null", nb.Cells[0].ExecutionCount)
	}

	// Markdown → markdown (no outputs/execution_count)
	if nb.Cells[1].CellType != "markdown" {
		t.Errorf("markdown cell_type = %q, want %q", nb.Cells[1].CellType, "markdown")
	}
	if nb.Cells[1].Outputs != nil {
		t.Errorf("markdown cell should not have outputs, got %s", nb.Cells[1].Outputs)
	}

	// SQL → raw with thaw_cell_type metadata (no outputs/execution_count)
	if nb.Cells[2].CellType != "raw" {
		t.Errorf("sql cell_type = %q, want %q", nb.Cells[2].CellType, "raw")
	}
	if v, ok := nb.Cells[2].Metadata["thaw_cell_type"]; !ok || v != "sql" {
		t.Errorf("sql cell metadata thaw_cell_type = %v, want %q", v, "sql")
	}
	if nb.Cells[2].Outputs != nil {
		t.Errorf("sql cell should not have outputs, got %s", nb.Cells[2].Outputs)
	}
}

func TestBuildNbformatPythonNoMetadata(t *testing.T) {
	cells := []notebookCellInput{
		{Kind: "python", Source: "x = 1"},
	}

	raw, err := buildNbformat(cells)
	if err != nil {
		t.Fatalf("buildNbformat: %v", err)
	}

	var nb nbformatNotebook
	if err := json.Unmarshal([]byte(raw), &nb); err != nil {
		t.Fatalf("buildNbformat output is not valid JSON: %v", err)
	}

	// Python cells should have empty metadata (no thaw_cell_type).
	if len(nb.Cells[0].Metadata) != 0 {
		t.Errorf("python cell metadata should be empty, got %v", nb.Cells[0].Metadata)
	}
}

// ── Kernel tool input validation tests ───────────────────────────────────────

func TestGetNotebookCompletionsEmptyFields(t *testing.T) {
	nb := &mockNotebookBackend{}
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nb)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	cases := []struct {
		name string
		args getNotebookCompletionsInput
	}{
		{"empty tab_id", getNotebookCompletionsInput{TabID: "", Code: "x", Line: 1, Col: 0}},
		{"empty code", getNotebookCompletionsInput{TabID: "t1", Code: "", Line: 1, Col: 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "get_notebook_completions",
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true for missing field")
			}
		})
	}
}

func TestCheckPythonSyntaxEmptyFields(t *testing.T) {
	nb := &mockNotebookBackend{}
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nb)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	cases := []struct {
		name string
		args checkPythonSyntaxInput
	}{
		{"empty tab_id", checkPythonSyntaxInput{TabID: "", Code: "x"}},
		{"empty code", checkPythonSyntaxInput{TabID: "t1", Code: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "check_python_syntax",
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("CallTool returned Go error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true for missing field")
			}
		})
	}
}

// ── read_notebook sandbox test ───────────────────────────────────────────────

func TestReadNotebookPathEscapeRejected(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := SessionConfig{WorkspaceRoot: tmpDir}
	srv := buildServer(nil, ExecutionModeMetadata, cfg, nil, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// Use .ipynb extension so the extension check passes and the sandbox
	// guard is the one that rejects the path.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "read_notebook",
		Arguments: readNotebookInput{Path: "/etc/evil.ipynb"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for path outside workspace, got false")
	}
}
