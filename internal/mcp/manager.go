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

// validModes is the set of accepted execution modes. Start rejects any
// mode not in this set so a session cannot report a capability it does
// not actually enforce.
var validModes = map[string]bool{
	ExecutionModeMetadata: true,
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
	if !validModes[mode] {
		return SessionInfo{}, fmt.Errorf("mcp: unsupported execution mode %q", mode)
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

	s := newSession(m, label, connLabel, mode, token, assigned, client, ln)
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
// is surfaced only here so it is not broadcast in every List() snapshot. The
// token is immutable after the session is created, so reading it under m.mu
// (without s.mu) is safe.
func (m *Manager) AuthenticatedURL(label string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[label]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("http://127.0.0.1:%d/sse?token=%s", s.port, s.token), true
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
