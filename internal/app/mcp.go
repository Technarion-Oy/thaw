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
	"errors"
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/config"
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
// auto-assigned starting at 9100. Credentials (port + token) are persisted
// per label so restarting Thaw and re-launching the same label reuses the
// same URL, keeping external AI client configs valid. The role, warehouse,
// and secondaryRoles parameters configure optional session pinning for
// non-metadata modes.
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

	// Load persisted credentials for this label so the session reuses the
	// same port+token across app restarts.
	var savedPort int
	var savedToken string
	if appCfg, err := config.Load(); err == nil && appCfg.MCPCredentials != nil {
		if cred, ok := appCfg.MCPCredentials[label]; ok {
			savedToken = cred.Token
			if port == 0 {
				savedPort = cred.Port
			}
		}
	}

	preferredPort := port
	if preferredPort == 0 && savedPort != 0 {
		preferredPort = savedPort
	}

	// Each session owns an isolated client so it survives independently of the
	// UI tab sessions and is closed when the session stops.
	client, err := snowflake.NewClient(a.ctx, *params)
	if err != nil {
		return mcp.SessionInfo{}, fmt.Errorf("mcp: failed to open connection: %w", err)
	}

	// Read the cached export directory (non-blocking). An empty value means
	// no workspace is configured — workspace tools will not be registered.
	a.exportDirMu.RLock()
	workspaceRoot := a.cachedExportDir
	a.exportDirMu.RUnlock()

	cfg := mcp.SessionConfig{
		PinnedRole:      role != "",
		PinnedWarehouse: warehouse != "",
		Role:            role,
		Warehouse:       warehouse,
		SecondaryRoles:  secondaryRoles,
		WorkspaceRoot:   workspaceRoot,
	}

	connLabel := fmt.Sprintf("%s / %s", params.Account, params.User)
	info, err := a.mcpManager.Start(a.ctx, label, connLabel, mode, preferredPort, client, cfg, savedToken)
	if err != nil {
		// If we used a saved port and it's now unavailable, retry with auto-assign
		// but keep the saved token so the URL only changes the port portion.
		if savedPort != 0 && preferredPort == savedPort && isPortConflict(err) {
			info, err = a.mcpManager.Start(a.ctx, label, connLabel, mode, 0, client, cfg, savedToken)
		}
		if err != nil {
			_ = client.Close()
			return mcp.SessionInfo{}, err
		}
	}

	// Persist the actual port and token so subsequent restarts reuse them.
	a.saveMCPCredential(label, info.Port)

	logger.L.Info("mcp session started", "label", info.Label, "port", info.Port, "mode", info.ExecutionMode)
	return info, nil
}

// saveMCPCredential persists the port and token for a session label to config.
func (a *App) saveMCPCredential(label string, port int) {
	token, ok := a.mcpManager.SessionToken(label)
	if !ok {
		return
	}
	appCfg, err := config.Load()
	if err != nil {
		logger.L.Warn("mcp: failed to load config for credential save", "err", err)
		return
	}
	if appCfg.MCPCredentials == nil {
		appCfg.MCPCredentials = make(map[string]config.MCPSessionCredential)
	}
	appCfg.MCPCredentials[label] = config.MCPSessionCredential{Port: port, Token: token}
	if err := config.Save(appCfg); err != nil {
		logger.L.Warn("mcp: failed to save credential", "label", label, "err", err)
	}
}

// isPortConflict reports whether the error indicates the requested port was unavailable.
func isPortConflict(err error) bool {
	return errors.Is(err, mcp.ErrPortUnavailable)
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
