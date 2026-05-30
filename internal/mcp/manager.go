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
	"fmt"
	"net"
	"sort"
	"sync"

	"thaw/internal/snowflake"
)

// basePort is the first port tried when auto-assigning a session port.
const basePort = 9100

// ExecutionMode values control how much a session is permitted to do.
// Only metadata browsing is supported in the foundation milestone.
const (
	ExecutionModeMetadata = "metadata"
)

// SessionInfo is the serializable view of a session exposed to the frontend.
type SessionInfo struct {
	Label           string `json:"label"`
	Port            int    `json:"port"`
	ExecutionMode   string `json:"executionMode"`
	Running         bool   `json:"running"`
	URL             string `json:"url"`
	ConnectionLabel string `json:"connectionLabel"`
}

// Manager owns the set of running MCP sessions. It is safe for concurrent use.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*session)}
}

// Start creates and starts a new session bound to the supplied client. The
// label must be unique among running sessions. If port is 0 a free port is
// auto-assigned starting at basePort. The session takes ownership of client
// and closes it when stopped.
func (m *Manager) Start(label, connLabel, mode string, port int, client *snowflake.Client) (SessionInfo, error) {
	if label == "" {
		return SessionInfo{}, fmt.Errorf("mcp: session label is required")
	}
	if mode == "" {
		mode = ExecutionModeMetadata
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[label]; exists {
		return SessionInfo{}, fmt.Errorf("mcp: a session named %q is already running", label)
	}

	assigned, err := m.allocatePortLocked(port)
	if err != nil {
		return SessionInfo{}, err
	}

	s := newSession(label, connLabel, mode, assigned, client)
	if err := s.start(); err != nil {
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

// allocatePortLocked returns a usable port. Must be called with m.mu held.
// If preferred is non-zero it is validated and returned; otherwise ports are
// tried sequentially from basePort. A port is usable if it is not already
// claimed by a session and can be bound on the loopback interface.
func (m *Manager) allocatePortLocked(preferred int) (int, error) {
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
			return 0, fmt.Errorf("mcp: port %d is already in use by another session", preferred)
		}
		if !portFree(preferred) {
			return 0, fmt.Errorf("mcp: port %d is not available", preferred)
		}
		return preferred, nil
	}

	for p := basePort; p < basePort+1000; p++ {
		if inUse(p) || !portFree(p) {
			continue
		}
		return p, nil
	}
	return 0, fmt.Errorf("mcp: no free port available in range %d-%d", basePort, basePort+1000)
}

// portFree reports whether a TCP port can be bound on the loopback interface.
func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
