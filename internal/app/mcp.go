// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/mcp"
	"thaw/internal/secrets"
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
	// Snapshot the params under the connection lock so a concurrent Disconnect
	// (which nils a.connectParams) can't race the read or turn the nil-check
	// below into a nil-deref panic.
	params := a.currentConnectParams()
	if params == nil {
		return mcp.SessionInfo{}, apperrors.ErrNotConnected
	}
	if mode == "" {
		mode = mcp.ExecutionModeMetadata
	}

	// Load persisted credentials for this label so the session reuses the
	// same port+token across app restarts. The port is non-secret and lives in
	// config.json; the token lives in the OS secure store.
	var savedPort int
	var savedToken string
	if appCfg, err := config.Load(); err == nil && appCfg.MCPCredentials != nil {
		if cred, ok := appCfg.MCPCredentials[label]; ok && port == 0 {
			savedPort = cred.Port
		}
	}
	savedToken, _ = secrets.Get(secrets.MCPTokenKey(label))

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

// saveMCPCredential persists the port (config.json) and token (OS secure store)
// for a session label so subsequent restarts reuse the same URL. Token reuse is
// best-effort: a failed secure-store write is logged, and the session still runs
// with its in-memory token — it just won't be reused across a restart.
//
// The token is written straight to the store and NOT carried into the config
// map (only the non-secret port persists there), so config.Update → buildDiskConfig
// never has to scrub a token whose store value might differ from a failed write.
func (a *App) saveMCPCredential(label string, port int) {
	token, ok := a.mcpManager.SessionToken(label)
	if !ok {
		return
	}
	if err := secrets.Set(secrets.MCPTokenKey(label), token); err != nil {
		logger.L.Warn("mcp: failed to save session token", "label", label, "err", err)
	}
	if err := config.Update(func(appCfg *config.AppConfig) error {
		if appCfg.MCPCredentials == nil {
			appCfg.MCPCredentials = make(map[string]config.MCPSessionCredential)
		}
		// Only the non-secret port persists here; the token lives in the store.
		appCfg.MCPCredentials[label] = config.MCPSessionCredential{Port: port}
		return nil
	}); err != nil {
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
// block carries the session's per-session auth token, so it is a secret — it
// must be treated like a credential.
//
// The token is placed in an "Authorization: Bearer" header rather than the URL
// query string. Query-string tokens can leak into local proxy logs, process
// listings (ps aux), and shell history; a header keeps the secret out of the
// URL. The tokenGuard middleware still accepts a "?token=<token>" URL, so a
// client that can only pass credentials in the URL can append it as a fallback.
func (a *App) GetMCPSessionConfig(label string) (string, error) {
	endpoint, token, ok := a.mcpManager.SessionEndpoint(label)
	if !ok {
		return "", fmt.Errorf("mcp: no session named %q", label)
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"thaw-" + label: map[string]any{
				"type": "sse",
				"url":  endpoint,
				"headers": map[string]any{
					"Authorization": "Bearer " + token,
				},
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
