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
	ExportDir          string `json:"exportDir"`
	RemoteURL          string `json:"remoteURL"`
	Branch             string `json:"branch"`
	AuthorName         string `json:"authorName"`
	AuthorEmail        string `json:"authorEmail"`
	ExportPathTemplate string `json:"exportPathTemplate"`
}

// AIConfig holds AI provider settings.
// APIKey is stored in ~/.config/thaw/config.json (mode 0600).
type AIConfig struct {
	Provider   string `json:"provider"`             // "openai" | "google" | "ollama"
	APIKey     string `json:"apiKey"`
	Model      string `json:"model"`
	Enabled    bool   `json:"enabled"`
	OllamaPort int    `json:"ollamaPort,omitempty"` // 0 means default (11434)
}

// SnowparkConfig holds Snowpark environment settings.
type SnowparkConfig struct {
	Backend    string `json:"backend"`    // "conda" | "venv" | "" (empty = default to conda)
	VenvPath   string `json:"venvPath"`   // custom venv path; empty = use computed default
	PythonPath string `json:"pythonPath"` // explicit python binary for venv creation; empty = auto-detect
}

// EditorPrefs holds SQL formatting preferences for the Monaco editor.
type EditorPrefs struct {
	// KeywordCase controls casing for SQL reserved words (SELECT, FROM, …).
	// Valid values: "UPPER" | "lower" | "Title" | "Preserve"
	KeywordCase string `json:"keywordCase"`
	// IdentifierCase controls casing for unquoted table/column names.
	// Double-quoted identifiers are never modified.
	// Valid values: "Preserve" | "UPPER" | "lower"
	IdentifierCase string `json:"identifierCase"`
	// FunctionCase controls casing for built-in and user-defined function calls.
	// Valid values: "UPPER" | "lower"
	FunctionCase string `json:"functionCase"`
	// IndentStyle is the character used for indentation.
	// Valid values: "spaces" | "tabs"
	IndentStyle string `json:"indentStyle"`
	// IndentSize is the number of spaces per indent level (ignored when IndentStyle is "tabs").
	// Valid values: 2 | 4
	IndentSize int `json:"indentSize"`
	// CommaPosition controls where commas appear in multi-value lists.
	// Valid values: "trailing" | "leading"
	CommaPosition string `json:"commaPosition"`
	// OperatorPosition controls whether AND/OR operators appear before or after the line break.
	// Valid values: "before" | "after"
	OperatorPosition string `json:"operatorPosition"`
}

// DefaultEditorPrefs returns sensible defaults for SQL editing in Snowflake.
func DefaultEditorPrefs() EditorPrefs {
	return EditorPrefs{
		KeywordCase:      "UPPER",
		IdentifierCase:   "Preserve",
		FunctionCase:     "UPPER",
		IndentStyle:      "spaces",
		IndentSize:       2,
		CommaPosition:    "trailing",
		OperatorPosition: "before",
	}
}

// AppConfig is the on-disk configuration for Thaw.
type AppConfig struct {
	Connections            []Connection   `json:"connections"`
	Git                    GitConfig      `json:"git"`
	AI                     AIConfig       `json:"ai"`
	Snowpark               SnowparkConfig `json:"snowpark"`
	Editor                 EditorPrefs    `json:"editor"`
	SnowflakeCLIConfigPath string         `json:"snowflakeCliConfigPath"`
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
