// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Connection is a saved Snowflake connection profile.
type Connection struct {
	Name      string `json:"name"`
	Account   string `json:"account"`
	User      string `json:"user"`
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// GitConfig holds the persisted git / export settings.
// Token is intentionally excluded — it must not be written to disk.
type GitConfig struct {
	ExportDir   string `json:"exportDir"`
	RemoteURL   string `json:"remoteURL"`
	Branch      string `json:"branch"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
}

// AIConfig holds AI provider settings.
// APIKey is stored in ~/.config/thaw/config.json (mode 0600).
type AIConfig struct {
	Provider string `json:"provider"` // "openai" | "google"
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
}

// AppConfig is the on-disk configuration for Thaw.
type AppConfig struct {
	Connections []Connection `json:"connections"`
	Git         GitConfig    `json:"git"`
	AI          AIConfig     `json:"ai"`
}

// configPath returns the absolute path to the application configuration file,
// typically $HOME/.config/thaw/config.json on Linux/macOS.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "thaw", "config.json"), nil
}

// Load reads the config file, returning an empty config if it doesn't exist yet.
func Load() (*AppConfig, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &AppConfig{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to disk, creating directories as needed.
func Save(cfg *AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
