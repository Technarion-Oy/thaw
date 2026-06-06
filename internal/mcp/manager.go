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
	"fmt"
	"net"
	"sort"
	"sync"

	"thaw/internal/fnmeta"
	"thaw/internal/snowflake"
)

// basePort is the first port tried when auto-assigning a session port.
const basePort = 9100

// ExecutionMode values control how much a session is permitted to do.
const (
	ExecutionModeMetadata    = "metadata"
	ExecutionModeReadonly    = "readonly"
	ExecutionModeExplainOnly = "explain_only"
)

// validModes is the set of accepted execution modes. Start rejects any
// mode not in this set so a session cannot report a capability it does
// not actually enforce.
var validModes = map[string]bool{
	ExecutionModeMetadata:    true,
	ExecutionModeReadonly:    true,
	ExecutionModeExplainOnly: true,
}

// SessionConfig holds optional per-session configuration applied at startup.
// It controls role/warehouse pinning, secondary-role restrictions, and
// workspace sandboxing.
type SessionConfig struct {
	PinnedRole      bool   `json:"pinnedRole"`
	PinnedWarehouse bool   `json:"pinnedWarehouse"`
	Role            string `json:"role,omitempty"`
	Warehouse       string `json:"warehouse,omitempty"`
	SecondaryRoles  string `json:"secondaryRoles,omitempty"`
	// WorkspaceRoot is the directory that workspace tools (read_file,
	// list_directory, search_files, git_*) are sandboxed to. When empty,
	// workspace tools are not registered at all.
	WorkspaceRoot string `json:"workspaceRoot,omitempty"`
}

// SessionInfo is the serializable view of a session exposed to the frontend.
// Sessions are removed from the Manager map on stop/unexpected failure, so
// List() only ever returns running sessions — there is no "Stopped" state.
type SessionInfo struct {
	Label           string `json:"label"`
	Port            int    `json:"port"`
	ExecutionMode   string `json:"executionMode"`
	URL             string `json:"url"`
	ConnectionLabel string `json:"connectionLabel"`
	PinnedRole      string `json:"pinnedRole,omitempty"`
	PinnedWarehouse string `json:"pinnedWarehouse,omitempty"`
}

// Manager owns the set of running MCP sessions. It is safe for concurrent use.
type Manager struct {
	mu        sync.Mutex
	sessions  map[string]*session
	editorCtx *EditorContextStore
	emit      func(string, interface{}) // Wails event emitter; nil when running outside the app (tests)
	fnStore   *fnmeta.Store             // local function metadata cache; nil until set via SetFnStore
	nb        NotebookBackend           // notebook/Snowpark backend; nil until set via SetNotebookBackend
}

// NewManager returns an empty Manager with an initialized EditorContextStore.
// emit is an optional callback for emitting Wails events from MCP tool handlers
// (e.g. opening a SQL tab in the frontend). Pass nil in tests or when the Wails
// runtime is not available.
func NewManager(emit func(string, interface{})) *Manager {
	return &Manager{
		sessions:  make(map[string]*session),
		editorCtx: NewEditorContextStore(),
		emit:      emit,
	}
}

// EditorContext returns the shared editor context store. MCP tool handlers
// read from this store; the frontend pushes state into it via App IPC methods.
func (m *Manager) EditorContext() *EditorContextStore {
	return m.editorCtx
}

// SetFnStore sets the function metadata store on the manager so new MCP
// sessions can expose function/procedure lookup tools. Existing sessions
// are unaffected — only sessions started after this call will see the store.
func (m *Manager) SetFnStore(store *fnmeta.Store) {
	m.mu.Lock()
	m.fnStore = store
	m.mu.Unlock()
}

// SetNotebookBackend sets the notebook/Snowpark backend on the manager so
// new MCP sessions can expose notebook tools (get_notebook_completions,
// check_python_syntax). Existing sessions are unaffected — only sessions
// started after this call will see the backend.
func (m *Manager) SetNotebookBackend(nb NotebookBackend) {
	m.mu.Lock()
	m.nb = nb
	m.mu.Unlock()
}

// Start creates and starts a new session bound to the supplied client. The
// label must be unique among running sessions. If port is 0 a free port is
// auto-assigned starting at basePort. The session takes ownership of client
// and closes it when stopped. cfg controls optional role/warehouse pinning
// applied at session startup. The context is used for the initial session
// setup (USE ROLE, USE WAREHOUSE, etc.) and can be canceled by the caller.
func (m *Manager) Start(ctx context.Context, label, connLabel, mode string, port int, client *snowflake.Client, cfg SessionConfig) (SessionInfo, error) {
	if label == "" {
		return SessionInfo{}, fmt.Errorf("mcp: session label is required")
	}
	if mode == "" {
		mode = ExecutionModeMetadata
	}
	if !validModes[mode] {
		return SessionInfo{}, fmt.Errorf("mcp: unsupported execution mode %q", mode)
	}

	// Validate secondaryRoles early — only "" and "none" are accepted.
	if cfg.SecondaryRoles != "" && cfg.SecondaryRoles != "none" {
		return SessionInfo{}, fmt.Errorf("mcp: unsupported secondaryRoles value %q (must be \"\" or \"none\")", cfg.SecondaryRoles)
	}

	// Apply session configuration for non-metadata modes. In metadata mode
	// no SQL tools are registered, so role/warehouse pinning is a no-op —
	// skip the Snowflake round-trips to avoid confusion.
	if mode != ExecutionModeMetadata {
		if err := applySessionConfig(ctx, client, cfg); err != nil {
			return SessionInfo{}, err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[label]; exists {
		return SessionInfo{}, fmt.Errorf("mcp: a session named %q is already running", label)
	}

	ln, err := m.allocatePortLocked(port)
	if err != nil {
		return SessionInfo{}, err
	}
	assigned := ln.Addr().(*net.TCPAddr).Port

	token, err := newSessionToken()
	if err != nil {
		_ = ln.Close()
		return SessionInfo{}, fmt.Errorf("mcp: failed to generate session token: %w", err)
	}

	s := newSession(m, label, connLabel, mode, token, assigned, client, ln, cfg, m.editorCtx)
	if err := s.start(); err != nil {
		_ = ln.Close()
		return SessionInfo{}, err
	}
	m.sessions[label] = s
	return s.info(), nil
}

// Stop stops and removes the named session, closing its connection.
func (m *Manager) Stop(label string) error {
	m.mu.Lock()
	s, ok := m.sessions[label]
	if ok {
		delete(m.sessions, label)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("mcp: no session named %q", label)
	}
	return s.stop()
}

// removeIfPresent deletes label from the registry only if it still maps to the
// exact session s. It is called by a session's serve goroutine when the server
// dies unexpectedly, so a dead session does not linger in the map. The identity
// check avoids racing a newer session that reused the label.
func (m *Manager) removeIfPresent(label string, s *session) {
	m.mu.Lock()
	if cur, ok := m.sessions[label]; ok && cur == s {
		delete(m.sessions, label)
	}
	m.mu.Unlock()
}

// List returns a snapshot of all sessions sorted by label.
func (m *Manager) List() []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s.info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// AuthenticatedURL returns the SSE endpoint URL for the named session with its
// per-session token embedded as a query parameter, suitable for handing to an
// MCP client. The bare SessionInfo.URL is token-free (for display); the token
// is surfaced only here so it is not broadcast in every List() snapshot. Both
// port and token are immutable after the session is created, so reading them
// under m.mu (without s.mu) is safe.
func (m *Manager) AuthenticatedURL(label string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[label]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("http://127.0.0.1:%d/sse?token=%s", s.port, s.token), true
}

// UpdateMode changes the execution mode of a running session, rebuilding its
// tool set. The session's dedicated connection and port are unaffected. New MCP
// client connections will see the updated tools; existing connections keep old
// tools until they reconnect (standard MCP behavior). When switching to a
// non-metadata mode, session config (role/warehouse pinning, secondary roles)
// is re-applied via Snowflake round-trips using ctx. Returns the updated
// SessionInfo.
func (m *Manager) UpdateMode(ctx context.Context, label, newMode string) (SessionInfo, error) {
	if !validModes[newMode] {
		return SessionInfo{}, fmt.Errorf("mcp: unsupported execution mode %q", newMode)
	}

	m.mu.Lock()
	s, ok := m.sessions[label]
	m.mu.Unlock()

	if !ok {
		return SessionInfo{}, fmt.Errorf("mcp: no session named %q", label)
	}

	if err := s.updateMode(ctx, newMode); err != nil {
		return SessionInfo{}, err
	}
	return s.info(), nil
}

// StopAll stops every session. It is called on application shutdown and on
// disconnect. Errors are ignored since the process is tearing down.
func (m *Manager) StopAll() {
	m.mu.Lock()
	sessions := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = make(map[string]*session)
	m.mu.Unlock()

	for _, s := range sessions {
		_ = s.stop()
	}
}

// allocatePortLocked binds and returns a held loopback listener. Must be
// called with m.mu held. If preferred is non-zero that exact port is bound;
// otherwise ports are tried sequentially from basePort. Returning the *held*
// listener (rather than a port number) closes the TOCTOU window where another
// process could claim the port between the availability check and the real
// bind in session.start.
func (m *Manager) allocatePortLocked(preferred int) (net.Listener, error) {
	inUse := func(p int) bool {
		for _, s := range m.sessions {
			if s.port == p {
				return true
			}
		}
		return false
	}

	if preferred != 0 {
		if inUse(preferred) {
			return nil, fmt.Errorf("mcp: port %d is already in use by another session", preferred)
		}
		ln, err := listenLoopback(preferred)
		if err != nil {
			return nil, fmt.Errorf("mcp: port %d is not available", preferred)
		}
		return ln, nil
	}

	for p := basePort; p < basePort+1000; p++ {
		if inUse(p) {
			continue
		}
		ln, err := listenLoopback(p)
		if err != nil {
			continue
		}
		return ln, nil
	}
	return nil, fmt.Errorf("mcp: no free port available in range %d-%d", basePort, basePort+1000)
}

// listenLoopback binds a TCP listener on the loopback interface at port.
func listenLoopback(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}
