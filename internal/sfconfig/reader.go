// Package sfconfig reads connection profiles from the Snowflake CLI
// configuration file (~/.snowflake/config.toml).
package sfconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Connection is a single named connection profile.
type Connection struct {
	Name                string `json:"name"`
	Account             string `json:"account"`
	User                string `json:"user"`
	Password            string `json:"password"`
	Role                string `json:"role"`
	Warehouse           string `json:"warehouse"`
	Database            string `json:"database"`
	Schema              string `json:"schema"`
	Authenticator       string `json:"authenticator"`
	Passcode            string `json:"passcode"`
	OktaURL             string `json:"oktaUrl"`
	PrivateKeyPath      string `json:"privateKeyPath"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
}

// Config is the parsed result of config.toml.
type Config struct {
	DefaultConnection string       `json:"defaultConnection"`
	Connections       []Connection `json:"connections"`
}

// ── internal TOML shapes ──────────────────────────────────────────────────────

type rawConfig struct {
	DefaultConnectionName string                    `toml:"default_connection_name"`
	Connections           map[string]rawConnection  `toml:"connections"`
}

type rawConnection struct {
	Account              string `toml:"account"`
	User                 string `toml:"user"`
	Password             string `toml:"password"`
	Role                 string `toml:"role"`
	Warehouse            string `toml:"warehouse"`
	Database             string `toml:"database"`
	Schema               string `toml:"schema"`
	Authenticator        string `toml:"authenticator"`
	Passcode             string `toml:"passcode"`
	OktaURL              string `toml:"okta_url"`
	PrivateKeyPath       string `toml:"private_key_path"`
	PrivateKeyPassphrase string `toml:"private_key_passphrase"`
}

// ─────────────────────────────────────────────────────────────────────────────

// Load reads and parses ~/.snowflake/config.toml.
// Returns an empty Config (not an error) if the file does not exist.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, nil
	}
	path := filepath.Join(home, ".snowflake", "config.toml")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg := &Config{DefaultConnection: raw.DefaultConnectionName}
	for name, c := range raw.Connections {
		cfg.Connections = append(cfg.Connections, Connection{
			Name:                name,
			Account:             c.Account,
			User:                c.User,
			Password:            c.Password,
			Role:                c.Role,
			Warehouse:           c.Warehouse,
			Database:            c.Database,
			Schema:              c.Schema,
			Authenticator:       c.Authenticator,
			Passcode:            c.Passcode,
			OktaURL:             c.OktaURL,
			PrivateKeyPath:      c.PrivateKeyPath,
			PrivateKeyPassphrase: c.PrivateKeyPassphrase,
		})
	}

	sort.Slice(cfg.Connections, func(i, j int) bool {
		return cfg.Connections[i].Name < cfg.Connections[j].Name
	})

	return cfg, nil
}
