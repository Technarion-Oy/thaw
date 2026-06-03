// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"encoding/json"
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/logger"
	"thaw/internal/mcp"
	"thaw/internal/snowflake"
)

// mcpEnabled reports whether the mcpServer feature is enabled. The effective
// flags (with IT-admin overrides applied) are checked so an admin-locked flag
// cannot be bypassed via the native menu.
func (a *App) mcpEnabled() bool {
	return a.GetFeatureFlags().MCPServer
}

// StartMCPSession opens a dedicated Snowflake connection (inheriting the
// current connect params) and starts an MCP server bound to it. The label
// must be unique among running sessions; if port is 0 a free port is
// auto-assigned starting at 9100. Sessions never auto-start and are not
// persisted across restarts. The role, warehouse, and secondaryRoles
// parameters configure optional session pinning for non-metadata modes.
func (a *App) StartMCPSession(label, mode string, port int, role, warehouse, secondaryRoles string) (mcp.SessionInfo, error) {
	if !a.mcpEnabled() {
		return mcp.SessionInfo{}, fmt.Errorf("MCP Server is disabled. Enable it under View → Enabled Features…")
	}
	// Snapshot the pointer into a local so a concurrent Disconnect (which nils
	// a.connectParams) can't turn the nil-check below into a nil-deref panic.
	// The underlying field is still unsynchronised (pre-existing app-wide race
	// tracked in #351); this only guards against the nil-deref.
	params := a.connectParams
	if params == nil {
		return mcp.SessionInfo{}, apperrors.ErrNotConnected
	}
	if mode == "" {
		mode = mcp.ExecutionModeMetadata
	}

	// Each session owns an isolated client so it survives independently of the
	// UI tab sessions and is closed when the session stops.
	client, err := snowflake.NewClient(a.ctx, *params)
	if err != nil {
		return mcp.SessionInfo{}, fmt.Errorf("mcp: failed to open connection: %w", err)
	}

	cfg := mcp.SessionConfig{
		PinnedRole:      role != "",
		PinnedWarehouse: warehouse != "",
		Role:            role,
		Warehouse:       warehouse,
		SecondaryRoles:  secondaryRoles,
	}

	connLabel := fmt.Sprintf("%s / %s", params.Account, params.User)
	info, err := a.mcpManager.Start(a.ctx, label, connLabel, mode, port, client, cfg)
	if err != nil {
		_ = client.Close()
		return mcp.SessionInfo{}, err
	}

	logger.L.Info("mcp session started", "label", info.Label, "port", info.Port, "mode", info.ExecutionMode)
	return info, nil
}

// UpdateMCPSessionMode changes the execution mode of a running session,
// rebuilding its tool set. New MCP client connections see the updated tools;
// existing connections keep old tools until they reconnect.
func (a *App) UpdateMCPSessionMode(label, mode string) (mcp.SessionInfo, error) {
	if !a.mcpEnabled() {
		return mcp.SessionInfo{}, fmt.Errorf("MCP Server is disabled. Enable it under View → Enabled Features…")
	}

	info, err := a.mcpManager.UpdateMode(a.ctx, label, mode)
	if err != nil {
		return mcp.SessionInfo{}, err
	}
	logger.L.Info("mcp session mode updated", "label", label, "mode", mode)
	return info, nil
}

// StopMCPSession stops the named session, closing its dedicated connection.
func (a *App) StopMCPSession(label string) error {
	if err := a.mcpManager.Stop(label); err != nil {
		return err
	}
	logger.L.Info("mcp session stopped", "label", label)
	return nil
}

// ListMCPSessions returns a snapshot of all running MCP sessions.
func (a *App) ListMCPSessions() []mcp.SessionInfo {
	return a.mcpManager.List()
}

// GetMCPSessionConfig returns the MCP client configuration JSON block for the
// named running session, suitable for pasting into an external MCP client. The
// embedded URL carries the session's per-session auth token, so the returned
// block is a secret — it must be treated like a credential.
func (a *App) GetMCPSessionConfig(label string) (string, error) {
	url, ok := a.mcpManager.AuthenticatedURL(label)
	if !ok {
		return "", fmt.Errorf("mcp: no session named %q", label)
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"thaw-" + label: map[string]any{
				"type": "sse",
				"url":  url,
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
