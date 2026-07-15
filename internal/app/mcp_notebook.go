// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/mcp"
	"thaw/internal/snowpark"
)

// notebookBackendAdapter bridges snowpark.Service to the mcp.NotebookBackend
// interface. It maps snowpark types to the MCP-local duplicates so the mcp
// package does not import snowpark.
type notebookBackendAdapter struct {
	svc *snowpark.Service
}

func (a *notebookBackendAdapter) GetNotebookCompletions(tabId, code string, line, col int) ([]mcp.NotebookCompletion, error) {
	comps, err := a.svc.GetNotebookCompletions(tabId, code, line, col)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.NotebookCompletion, len(comps))
	for i, c := range comps {
		result[i] = mcp.NotebookCompletion{
			Label:         c.Label,
			Type:          c.Type,
			Detail:        c.Detail,
			Documentation: c.Documentation,
		}
	}
	return result, nil
}

func (a *notebookBackendAdapter) CheckPythonSyntax(tabId, code, mode string) ([]mcp.NotebookSyntaxError, error) {
	errs, err := a.svc.CheckPythonSyntax(tabId, code, mode)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.NotebookSyntaxError, len(errs))
	for i, e := range errs {
		result[i] = mcp.NotebookSyntaxError{
			Severity: e.Severity,
			Line:     e.Line,
			Col:      e.Col,
			EndCol:   e.EndCol,
			Msg:      e.Msg,
		}
	}
	return result, nil
}
