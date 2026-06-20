// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package sfconfig reads connection profiles from the Snowflake CLI
// configuration file (~/.snowflake/config.toml).
//
// thaw:domain: Core IPC & App Lifecycle
package sfconfig

import (
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

// Connection is a single named connection profile.
type Connection struct {
	Name                 string `json:"name"`
	Account              string `json:"account"`
	User                 string `json:"user"`
	Password             string `json:"password"`
	Role                 string `json:"role"`
	Warehouse            string `json:"warehouse"`
	Database             string `json:"database"`
	Schema               string `json:"schema"`
	Authenticator        string `json:"authenticator"`
	Passcode             string `json:"passcode"`
	OktaURL              string `json:"oktaUrl"`
	PrivateKeyPath       string `json:"privateKeyPath"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`

	// Token-based and OAuth2 / WIF authenticators (see snowflake.ConnectParams).
	Token                             string `json:"token"`
	TokenFilePath                     string `json:"tokenFilePath"`
	OAuthClientID                     string `json:"oauthClientId"`
	OAuthClientSecret                 string `json:"oauthClientSecret"`
	OAuthTokenRequestURL              string `json:"oauthTokenRequestUrl"`
	OAuthAuthorizationURL             string `json:"oauthAuthorizationUrl"`
	OAuthRedirectURI                  string `json:"oauthRedirectUri"`
	OAuthScope                        string `json:"oauthScope"`
	EnableSingleUseRefreshTokens      bool   `json:"enableSingleUseRefreshTokens"`
	WorkloadIdentityProvider          string `json:"workloadIdentityProvider"`
	WorkloadIdentityEntraResource     string `json:"workloadIdentityEntraResource"`
	WorkloadIdentityImpersonationPath string `json:"workloadIdentityImpersonationPath"`

	// Forward-proxy configuration (see snowflake.ConnectParams).
	ProxyHost     string `json:"proxyHost"`
	ProxyPort     int    `json:"proxyPort"`
	ProxyUser     string `json:"proxyUser"`
	ProxyPassword string `json:"proxyPassword"`
	ProxyProtocol string `json:"proxyProtocol"`
	NoProxy       string `json:"noProxy"`
}

// Config is the parsed result of config.toml.
type Config struct {
	DefaultConnection string       `json:"defaultConnection"`
	Connections       []Connection `json:"connections"`
}

// ── internal TOML shapes ──────────────────────────────────────────────────────

type rawConfig struct {
	DefaultConnectionName string                   `toml:"default_connection_name"`
	Connections           map[string]rawConnection `toml:"connections"`
}

type rawConnection struct {
	Account                           string `toml:"account"`
	User                              string `toml:"user"`
	Password                          string `toml:"password"`
	Role                              string `toml:"role"`
	Warehouse                         string `toml:"warehouse"`
	Database                          string `toml:"database"`
	Schema                            string `toml:"schema"`
	Authenticator                     string `toml:"authenticator"`
	Passcode                          string `toml:"passcode"`
	OktaURL                           string `toml:"okta_url"`
	PrivateKeyPath                    string `toml:"private_key_path"`
	PrivateKeyPassphrase              string `toml:"private_key_passphrase"`
	Token                             string `toml:"token"`
	TokenFilePath                     string `toml:"token_file_path"`
	OAuthClientID                     string `toml:"oauth_client_id"`
	OAuthClientSecret                 string `toml:"oauth_client_secret"`
	OAuthTokenRequestURL              string `toml:"oauth_token_request_url"`
	OAuthAuthorizationURL             string `toml:"oauth_authorization_url"`
	OAuthRedirectURI                  string `toml:"oauth_redirect_uri"`
	OAuthScope                        string `toml:"oauth_scope"`
	EnableSingleUseRefreshTokens      bool   `toml:"enable_single_use_refresh_tokens"`
	WorkloadIdentityProvider          string `toml:"workload_identity_provider"`
	WorkloadIdentityEntraResource     string `toml:"workload_identity_entra_resource"`
	WorkloadIdentityImpersonationPath string `toml:"workload_identity_impersonation_path"`
	ProxyHost                         string `toml:"proxy_host"`
	ProxyPort                         int    `toml:"proxy_port"`
	ProxyUser                         string `toml:"proxy_user"`
	ProxyPassword                     string `toml:"proxy_password"`
	ProxyProtocol                     string `toml:"proxy_protocol"`
	NoProxy                           string `toml:"no_proxy"`
}

// ─────────────────────────────────────────────────────────────────────────────

// Load reads and parses the given Snowflake CLI configuration file.
// Returns an empty Config (not an error) if the file does not exist.
func Load(path string) (*Config, error) {
	path, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

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
			Name:                              name,
			Account:                           c.Account,
			User:                              c.User,
			Password:                          c.Password,
			Role:                              c.Role,
			Warehouse:                         c.Warehouse,
			Database:                          c.Database,
			Schema:                            c.Schema,
			Authenticator:                     c.Authenticator,
			Passcode:                          c.Passcode,
			OktaURL:                           c.OktaURL,
			PrivateKeyPath:                    c.PrivateKeyPath,
			PrivateKeyPassphrase:              c.PrivateKeyPassphrase,
			Token:                             c.Token,
			TokenFilePath:                     c.TokenFilePath,
			OAuthClientID:                     c.OAuthClientID,
			OAuthClientSecret:                 c.OAuthClientSecret,
			OAuthTokenRequestURL:              c.OAuthTokenRequestURL,
			OAuthAuthorizationURL:             c.OAuthAuthorizationURL,
			OAuthRedirectURI:                  c.OAuthRedirectURI,
			OAuthScope:                        c.OAuthScope,
			EnableSingleUseRefreshTokens:      c.EnableSingleUseRefreshTokens,
			WorkloadIdentityProvider:          c.WorkloadIdentityProvider,
			WorkloadIdentityEntraResource:     c.WorkloadIdentityEntraResource,
			WorkloadIdentityImpersonationPath: c.WorkloadIdentityImpersonationPath,
			ProxyHost:                         c.ProxyHost,
			ProxyPort:                         c.ProxyPort,
			ProxyUser:                         c.ProxyUser,
			ProxyPassword:                     c.ProxyPassword,
			ProxyProtocol:                     c.ProxyProtocol,
			NoProxy:                           c.NoProxy,
		})
	}

	sort.Slice(cfg.Connections, func(i, j int) bool {
		return cfg.Connections[i].Name < cfg.Connections[j].Name
	})

	return cfg, nil
}
