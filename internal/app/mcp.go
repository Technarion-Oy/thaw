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
	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/mcp"
	"thaw/internal/snowflake"
)

// StartMCPSession opens a dedicated Snowflake connection (inheriting the
// current connect params) and starts an MCP server bound to it. The label
// must be unique among running sessions; if port is 0 a free port is
// auto-assigned starting at 9100. The session definition is persisted so it
// is listed on the next launch (sessions never auto-start).
func (a *App) StartMCPSession(label, mode string, port int) (mcp.SessionInfo, error) {
	if a.connectParams == nil {
		return mcp.SessionInfo{}, apperrors.ErrNotConnected
	}
	if mode == "" {
		mode = mcp.ExecutionModeMetadata
	}

	// Each session owns an isolated client so it survives independently of the
	// UI tab sessions and is closed when the session stops.
	client, err := snowflake.NewClient(a.ctx, *a.connectParams)
	if err != nil {
		return mcp.SessionInfo{}, fmt.Errorf("mcp: failed to open connection: %w", err)
	}

	connLabel := fmt.Sprintf("%s / %s", a.connectParams.Account, a.connectParams.User)
	info, err := a.mcpManager.Start(label, connLabel, mode, port, client)
	if err != nil {
		_ = client.Close()
		return mcp.SessionInfo{}, err
	}

	a.persistMCPSession(config.MCPSessionConfig{
		Label:         info.Label,
		Port:          info.Port,
		ExecutionMode: info.ExecutionMode,
	})
	logger.L.Info("mcp session started", "label", info.Label, "port", info.Port, "mode", info.ExecutionMode)
	return info, nil
}

// StopMCPSession stops the named session and removes it from the persisted
// configuration.
func (a *App) StopMCPSession(label string) error {
	if err := a.mcpManager.Stop(label); err != nil {
		return err
	}
	a.removeMCPSession(label)
	logger.L.Info("mcp session stopped", "label", label)
	return nil
}

// ListMCPSessions returns a snapshot of all running MCP sessions.
func (a *App) ListMCPSessions() []mcp.SessionInfo {
	return a.mcpManager.List()
}

// GetMCPSessionConfig returns the MCP client configuration JSON block for the
// named running session, suitable for pasting into an external MCP client.
func (a *App) GetMCPSessionConfig(label string) (string, error) {
	for _, s := range a.mcpManager.List() {
		if s.Label == label {
			cfg := map[string]any{
				"mcpServers": map[string]any{
					"thaw-" + s.Label: map[string]any{
						"url": s.URL,
					},
				},
			}
			b, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
	}
	return "", fmt.Errorf("mcp: no session named %q", label)
}

// persistMCPSession upserts a session definition into config.json by label.
func (a *App) persistMCPSession(sc config.MCPSessionConfig) {
	cfg, err := config.Load()
	if err != nil {
		logger.L.Warn("mcp: load config failed", "err", err)
		return
	}
	found := false
	for i := range cfg.MCP.Sessions {
		if cfg.MCP.Sessions[i].Label == sc.Label {
			cfg.MCP.Sessions[i] = sc
			found = true
			break
		}
	}
	if !found {
		cfg.MCP.Sessions = append(cfg.MCP.Sessions, sc)
	}
	if err := config.Save(cfg); err != nil {
		logger.L.Warn("mcp: save config failed", "err", err)
	}
}

// removeMCPSession deletes a session definition from config.json by label.
func (a *App) removeMCPSession(label string) {
	cfg, err := config.Load()
	if err != nil {
		logger.L.Warn("mcp: load config failed", "err", err)
		return
	}
	out := cfg.MCP.Sessions[:0]
	for _, s := range cfg.MCP.Sessions {
		if s.Label != label {
			out = append(out, s)
		}
	}
	cfg.MCP.Sessions = out
	if err := config.Save(cfg); err != nil {
		logger.L.Warn("mcp: save config failed", "err", err)
	}
}
