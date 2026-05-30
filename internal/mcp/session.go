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
	"net/http"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// session is a single running MCP server bound to one Snowflake client.
type session struct {
	label     string
	connLabel string
	mode      string
	port      int

	client *snowflake.Client
	server *mcpsdk.Server
	http   *http.Server

	mu      sync.Mutex
	running bool
}

// newSession constructs a session; it is not started until start() is called.
func newSession(label, connLabel, mode string, port int, client *snowflake.Client) *session {
	return &session{
		label:     label,
		connLabel: connLabel,
		mode:      mode,
		port:      port,
		client:    client,
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

	s.server = buildServer(s.client, s.mode)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return s.server }, nil)

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mcp: failed to bind %s: %w", addr, err)
	}

	s.http = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		// ErrServerClosed is the normal result of a graceful Shutdown.
		_ = s.http.Serve(ln)
	}()

	s.running = true
	return nil
}

// stop gracefully shuts down the HTTP server and closes the owned client.
func (s *session) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}
	s.running = false

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var httpErr error
	if s.http != nil {
		httpErr = s.http.Shutdown(ctx)
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	return httpErr
}

// info returns the serializable snapshot for this session.
func (s *session) info() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionInfo{
		Label:           s.label,
		Port:            s.port,
		ExecutionMode:   s.mode,
		Running:         s.running,
		URL:             fmt.Sprintf("http://localhost:%d/sse", s.port),
		ConnectionLabel: s.connLabel,
	}
}
