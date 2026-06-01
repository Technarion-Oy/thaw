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

import "sync"

// EditorContextStore is a concurrency-safe in-memory store that holds
// per-tab snapshots of the editor SQL and query result summaries. The
// frontend pushes state into this store via IPC, and MCP tool handlers
// read from it. This bridges the frontend Zustand queryStore (which
// owns the editor state) with the MCP server (which cannot import
// internal/app).
type EditorContextStore struct {
	mu        sync.RWMutex
	activeTab string
	tabs      map[string]*tabContext
}

// tabContext holds the editor state for a single tab.
type tabContext struct {
	sql    string
	result *ResultSummary
}

// ResultSummary is a condensed view of a query result, safe to expose
// to external AI clients via MCP. It contains column names, row count,
// and a small sample of rows (typically the first 5).
type ResultSummary struct {
	TabID      string     `json:"tabId"`
	Columns    []string   `json:"columns"`
	RowCount   int        `json:"rowCount"`
	Truncated  bool       `json:"truncated"`
	SampleRows [][]any    `json:"sampleRows"`
	QueryID    string     `json:"queryId,omitempty"`
}

// QueryHistoryEntry is a condensed view of a query history row,
// suitable for returning from the get_query_history MCP tool.
type QueryHistoryEntry struct {
	QueryID      string `json:"queryId"`
	QueryText    string `json:"queryText"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	ElapsedMs    int64  `json:"elapsedMs"`
	RowsProduced int64  `json:"rowsProduced"`
	StartTime    string `json:"startTime"`
	DatabaseName string `json:"databaseName,omitempty"`
	SchemaName   string `json:"schemaName,omitempty"`
}

// NewEditorContextStore returns an empty store.
func NewEditorContextStore() *EditorContextStore {
	return &EditorContextStore{tabs: make(map[string]*tabContext)}
}

// SetActiveTab sets the active tab and its SQL content. This is called
// when the user switches tabs.
func (s *EditorContextStore) SetActiveTab(tabID, sql string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeTab = tabID
	tc := s.tabs[tabID]
	if tc == nil {
		tc = &tabContext{}
		s.tabs[tabID] = tc
	}
	tc.sql = sql
}

// SetTabSQL updates the SQL content for a specific tab without changing
// the active tab. Called on debounced text changes.
func (s *EditorContextStore) SetTabSQL(tabID, sql string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tc := s.tabs[tabID]
	if tc == nil {
		tc = &tabContext{}
		s.tabs[tabID] = tc
	}
	tc.sql = sql
}

// SetTabResult stores the latest result summary for a tab.
func (s *EditorContextStore) SetTabResult(tabID string, result *ResultSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tc := s.tabs[tabID]
	if tc == nil {
		tc = &tabContext{}
		s.tabs[tabID] = tc
	}
	tc.result = result
}

// RemoveTab removes all state for a tab. Called on tab close.
func (s *EditorContextStore) RemoveTab(tabID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tabs, tabID)
	if s.activeTab == tabID {
		s.activeTab = ""
	}
}

// ActiveEditorSQL returns the SQL content of the currently active tab.
// The second return value is false when no active tab is set or the tab
// has no content.
func (s *EditorContextStore) ActiveEditorSQL() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeTab == "" {
		return "", false
	}
	tc := s.tabs[s.activeTab]
	if tc == nil {
		return "", false
	}
	return tc.sql, true
}

// QueryResultSummary returns the latest result summary for a tab. If
// tabID is empty, the active tab is used. Returns nil when no result
// is available.
func (s *EditorContextStore) QueryResultSummary(tabID string) *ResultSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if tabID == "" {
		tabID = s.activeTab
	}
	if tabID == "" {
		return nil
	}
	tc := s.tabs[tabID]
	if tc == nil {
		return nil
	}
	return tc.result
}
