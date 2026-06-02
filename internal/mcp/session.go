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
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/logger"
	"thaw/internal/snowflake"
)

// session is a single running MCP server bound to one Snowflake client.
type session struct {
	label     string
	connLabel string
	mode      string
	port      int
	token     string
	cfg       SessionConfig

	client    *snowflake.Client
	editorCtx *EditorContextStore
	server    *mcpsdk.Server
	ln        net.Listener
	http      *http.Server
	mgr       *Manager

	mu      sync.Mutex
	running bool
}

// newSession constructs a session; it is not started until start() is called.
// ln is the already-bound loopback listener the HTTP server will serve on;
// mgr is the owning Manager, used to self-remove if the server dies. token is
// the per-session secret required to open the SSE GET (see tokenGuard).
func newSession(mgr *Manager, label, connLabel, mode, token string, port int, client *snowflake.Client, ln net.Listener, cfg SessionConfig, editorCtx *EditorContextStore) *session {
	return &session{
		label:     label,
		connLabel: connLabel,
		mode:      mode,
		token:     token,
		port:      port,
		client:    client,
		editorCtx: editorCtx,
		ln:        ln,
		mgr:       mgr,
		cfg:       cfg,
	}
}

// start builds the MCP server, binds the loopback listener, and serves the SSE
// handler in a background goroutine.
func (s *session) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("mcp: session %q already running", s.label)
	}

	s.server = buildServer(s.client, s.mode, s.cfg, s.editorCtx)
	sse := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return s.server }, nil)
	// loopbackGuard (DNS-rebinding defense) runs first, then tokenGuard
	// authenticates the session-creating GET against the per-session token.
	handler := loopbackGuard(tokenGuard(s.token, sse))

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.http = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		// ErrServerClosed is the normal result of a graceful Shutdown; any
		// other error means the server died unexpectedly. Tear the session
		// down completely: stop() releases the client/connection pool, and
		// self-removing from the manager avoids stranding a dead "Stopped"
		// row in the UI. stop() releases s.mu before removeIfPresent acquires
		// m.mu, so the m.mu→s.mu lock ordering is preserved (no deadlock).
		if err := s.http.Serve(s.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L.Error("mcp session server stopped unexpectedly", "label", s.label, "port", s.port, "err", err)
			_ = s.stop()
			if s.mgr != nil {
				s.mgr.removeIfPresent(s.label, s)
			}
		}
	}()

	// Set running under the same s.mu lock that the goroutine's stop() call
	// contends for, so stop() blocks until this flag is set. This guarantees
	// stop() always observes running == true if the goroutine races here.
	s.running = true
	return nil
}

// stop gracefully shuts down the HTTP server and closes the owned client.
func (s *session) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Always release the dedicated client/connection pool, even when the
	// server already died unexpectedly (running == false). Gating this on
	// running would leak the Snowflake connection on the unexpected-failure
	// branch, since Manager.Stop/StopAll calls stop() exactly once.
	defer func() {
		if s.client != nil {
			_ = s.client.Close()
			s.client = nil
		}
	}()

	if !s.running {
		return nil
	}
	s.running = false

	// Shutdown does not wait for hijacked/long-lived SSE connections, so a
	// tool call in flight at teardown may hit the client closed above and
	// error out. That is expected on Disconnect/shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.http != nil {
		return s.http.Shutdown(ctx)
	}
	return nil
}

// updateMode rebuilds the MCP server with a new execution mode, swapping the
// server pointer atomically under s.mu. The existing SSE handler closure reads
// s.server per-request, so new connections automatically get the new tool set.
// Existing connections keep old tools until reconnect (standard MCP behavior).
func (s *session) updateMode(newMode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.server = buildServer(s.client, newMode, s.cfg, s.editorCtx)
	s.mode = newMode
}

// info returns the serializable snapshot for this session.
func (s *session) info() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := SessionInfo{
		Label:           s.label,
		Port:            s.port,
		ExecutionMode:   s.mode,
		URL:             fmt.Sprintf("http://127.0.0.1:%d/sse", s.port),
		ConnectionLabel: s.connLabel,
	}
	if s.cfg.PinnedRole && s.cfg.Role != "" {
		info.PinnedRole = s.cfg.Role
	}
	if s.cfg.PinnedWarehouse && s.cfg.Warehouse != "" {
		info.PinnedWarehouse = s.cfg.Warehouse
	}
	return info
}
