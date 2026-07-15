// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import "sync"

// ERDesignerState is a snapshot of the open ER designer's tables and database,
// pushed from the frontend via IPC. MCP tool handlers read from this cache
// to expose the current designer state without importing internal/app.
type ERDesignerState struct {
	Database string               `json:"database"`
	Tables   []ERDesignerTableOut `json:"tables"`
}

// ERDesignerTableOut is the MCP-facing view of a designer table. It omits the
// React-internal UUID id field — MCP clients identify tables by SCHEMA.NAME.
type ERDesignerTableOut struct {
	Schema  string                `json:"schema"`
	Name    string                `json:"name"`
	Columns []ERDesignerColumnOut `json:"columns"`
}

// ERDesignerColumnOut is the MCP-facing view of a designer column.
type ERDesignerColumnOut struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	IsPK     bool   `json:"isPK"`
	NotNull  bool   `json:"notNull"`
	FKRef    string `json:"fkRef,omitempty"`
	Default  string `json:"defaultValue,omitempty"` // column DEFAULT expression, if set
}

// ERDesignerStateStore is a concurrency-safe in-memory cache that holds the
// current ER designer state. The frontend pushes state into this store via
// IPC on mount, state changes (debounced), and unmount. MCP tool handlers
// read from it.
type ERDesignerStateStore struct {
	mu    sync.RWMutex
	state *ERDesignerState
}

// NewERDesignerStateStore returns an empty store (designer not open).
func NewERDesignerStateStore() *ERDesignerStateStore {
	return &ERDesignerStateStore{}
}

// Set replaces the current designer state. Called by the frontend via IPC
// when the designer opens or when tables change.
func (s *ERDesignerStateStore) Set(state *ERDesignerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// Clear removes the designer state, indicating the designer is closed.
func (s *ERDesignerStateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = nil
}

// Get returns a deep copy of the current designer state (including nested
// column slices). Returns nil when the designer is not open.
func (s *ERDesignerStateStore) Get() *ERDesignerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state == nil {
		return nil
	}
	tables := make([]ERDesignerTableOut, len(s.state.Tables))
	for i, t := range s.state.Tables {
		tables[i] = ERDesignerTableOut{
			Schema:  t.Schema,
			Name:    t.Name,
			Columns: append([]ERDesignerColumnOut(nil), t.Columns...),
		}
	}
	return &ERDesignerState{
		Database: s.state.Database,
		Tables:   tables,
	}
}

// IsOpen returns true when the designer is currently open (state is set).
func (s *ERDesignerStateStore) IsOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state != nil
}
