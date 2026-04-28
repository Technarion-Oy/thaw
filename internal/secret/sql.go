// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package secret

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

type SecretType string

const (
	SecretTypeOAuth2             SecretType = "OAUTH2"
	SecretTypeCloudProviderToken SecretType = "CLOUD_PROVIDER_TOKEN"
	SecretTypePassword           SecretType = "PASSWORD"
	SecretTypeGenericString      SecretType = "GENERIC_STRING"
	SecretTypeSymmetricKey       SecretType = "SYMMETRIC_KEY"
)

type SecretConfig struct {
	Name                    string     `json:"name"`
	CaseSensitive           bool       `json:"caseSensitive"`
	OrReplace               bool       `json:"orReplace"`
	IfNotExists             bool       `json:"ifNotExists"`
	Type                    SecretType `json:"type"`
	// OAUTH2
	OAuthFlow               string     `json:"oauthFlow"` // CLIENT_CREDENTIALS or AUTHORIZATION_CODE
	ApiAuthentication       string     `json:"apiAuthentication"`
	OAuthScopes             string     `json:"oauthScopes"`
	OAuthRefreshToken       string     `json:"oauthRefreshToken"`
	OAuthRefreshTokenExpiry string     `json:"oauthRefreshTokenExpiry"` // ISO string from frontend
	// CLOUD_PROVIDER_TOKEN
	Enabled                 bool       `json:"enabled"`
	// PASSWORD
	Username                string     `json:"username"`
	Password                string     `json:"password"`
	// GENERIC_STRING
	SecretString            string     `json:"secretString"`
	// Common
	Comment                 string     `json:"comment"`
}

func escLit(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// BuildCreateSecretSql constructs a CREATE SECRET SQL statement.
func BuildCreateSecretSql(db, schema string, cfg SecretConfig) string {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " SECRET"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "secret_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s\n", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)
	fmt.Fprintf(&sb, "  TYPE = %s", cfg.Type)

	switch cfg.Type {
	case SecretTypeOAuth2:
		fmt.Fprintf(&sb, "\n  API_AUTHENTICATION = %s", snowflake.QuoteIdent(cfg.ApiAuthentication))
		if cfg.OAuthFlow == "CLIENT_CREDENTIALS" {
			if cfg.OAuthScopes != "" {
				parts := strings.Split(cfg.OAuthScopes, ",")
				var quoted []string
				for _, p := range parts {
					quoted = append(quoted, fmt.Sprintf("'%s'", escLit(strings.TrimSpace(p))))
				}
				fmt.Fprintf(&sb, "\n  OAUTH_SCOPES = (%s)", strings.Join(quoted, ", "))
			}
		} else {
			fmt.Fprintf(&sb, "\n  OAUTH_REFRESH_TOKEN = '%s'", escLit(cfg.OAuthRefreshToken))
			if cfg.OAuthRefreshTokenExpiry != "" {
				// Snowflake accepts ISO8601/RFC3339
				fmt.Fprintf(&sb, "\n  OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '%s'", escLit(cfg.OAuthRefreshTokenExpiry))
			}
		}
	case SecretTypeCloudProviderToken:
		fmt.Fprintf(&sb, "\n  API_AUTHENTICATION = '%s'", escLit(cfg.ApiAuthentication))
		enabled := "TRUE"
		if !cfg.Enabled {
			enabled = "FALSE"
		}
		fmt.Fprintf(&sb, "\n  ENABLED = %s", enabled)
	case SecretTypePassword:
		fmt.Fprintf(&sb, "\n  USERNAME = '%s'", escLit(cfg.Username))
		fmt.Fprintf(&sb, "\n  PASSWORD = '%s'", escLit(cfg.Password))
	case SecretTypeGenericString:
		fmt.Fprintf(&sb, "\n  SECRET_STRING = '%s'", escLit(cfg.SecretString))
	case SecretTypeSymmetricKey:
		fmt.Fprintf(&sb, "\n  ALGORITHM = GENERIC")
	}

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", escLit(cfg.Comment))
	}

	return sb.String() + ";"
}

// BuildModifySecretSql constructs one or more ALTER SECRET statements.
func BuildModifySecretSql(db, schema, name string, cfg SecretConfig, originalComment string) []string {
	secretRef := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	var statements []string
	var setClauses []string

	switch cfg.Type {
	case SecretTypeOAuth2:
		if cfg.OAuthFlow == "CLIENT_CREDENTIALS" {
			if cfg.OAuthScopes != "" {
				parts := strings.Split(cfg.OAuthScopes, ",")
				var quoted []string
				for _, p := range parts {
					quoted = append(quoted, fmt.Sprintf("'%s'", escLit(strings.TrimSpace(p))))
				}
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_SCOPES = (%s)", strings.Join(quoted, ", ")))
			}
		} else {
			if cfg.OAuthRefreshToken != "" {
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_REFRESH_TOKEN = '%s'", escLit(cfg.OAuthRefreshToken)))
			}
			if cfg.OAuthRefreshTokenExpiry != "" {
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '%s'", escLit(cfg.OAuthRefreshTokenExpiry)))
			}
		}
	case SecretTypeCloudProviderToken:
		if cfg.ApiAuthentication != "" {
			setClauses = append(setClauses, fmt.Sprintf("API_AUTHENTICATION = '%s'", escLit(cfg.ApiAuthentication)))
		}
	case SecretTypePassword:
		if cfg.Username != "" {
			setClauses = append(setClauses, fmt.Sprintf("USERNAME = '%s'", escLit(cfg.Username)))
		}
		if cfg.Password != "" {
			setClauses = append(setClauses, fmt.Sprintf("PASSWORD = '%s'", escLit(cfg.Password)))
		}
	case SecretTypeGenericString:
		if cfg.SecretString != "" {
			setClauses = append(setClauses, fmt.Sprintf("SECRET_STRING = '%s'", escLit(cfg.SecretString)))
		}
	}

	if cfg.Comment != "" {
		setClauses = append(setClauses, fmt.Sprintf("COMMENT = '%s'", escLit(cfg.Comment)))
	}

	if len(setClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER SECRET %s SET %s;", secretRef, strings.Join(setClauses, "\n    ")))
	}

	if originalComment != "" && cfg.Comment == "" {
		statements = append(statements, fmt.Sprintf("ALTER SECRET %s UNSET COMMENT;", secretRef))
	}

	return statements
}
