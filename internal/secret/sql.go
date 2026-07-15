// SPDX-License-Identifier: GPL-3.0-or-later

package secret

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

// isValid reports whether t is one of the declared SecretType constants.
func (t SecretType) isValid() bool {
	switch t {
	case SecretTypeOAuth2, SecretTypeCloudProviderToken, SecretTypePassword, SecretTypeGenericString, SecretTypeSymmetricKey:
		return true
	}
	return false
}

type SecretType string

const (
	SecretTypeOAuth2             SecretType = "OAUTH2"
	SecretTypeCloudProviderToken SecretType = "CLOUD_PROVIDER_TOKEN"
	SecretTypePassword           SecretType = "PASSWORD"
	// #nosec G101 -- False positive: these are Snowflake Secret TYPE enum values, not actual secrets.
	SecretTypeGenericString SecretType = "GENERIC_STRING"
	// #nosec G101 -- False positive: these are Snowflake Secret TYPE enum values, not actual secrets.
	SecretTypeSymmetricKey SecretType = "SYMMETRIC_KEY"
)

type SecretConfig struct {
	Name          string     `json:"name"`
	CaseSensitive bool       `json:"caseSensitive"`
	OrReplace     bool       `json:"orReplace"`
	IfNotExists   bool       `json:"ifNotExists"`
	Type          SecretType `json:"type"`
	// OAUTH2
	OAuthFlow               string `json:"oauthFlow"` // CLIENT_CREDENTIALS or AUTHORIZATION_CODE
	ApiAuthentication       string `json:"apiAuthentication"`
	OAuthScopes             string `json:"oauthScopes"`
	OAuthRefreshToken       string `json:"oauthRefreshToken"`
	OAuthRefreshTokenExpiry string `json:"oauthRefreshTokenExpiry"` // ISO string from frontend
	// CLOUD_PROVIDER_TOKEN
	Enabled bool `json:"enabled"`
	// PASSWORD
	Username string `json:"username"`
	Password string `json:"password"`
	// GENERIC_STRING
	SecretString string `json:"secretString"`
	// Common
	Comment string `json:"comment"`
}

// BuildCreateSecretSql constructs a CREATE SECRET SQL statement.
// It returns an error if cfg.Type is not a known SecretType constant.
func BuildCreateSecretSql(db, schema string, cfg SecretConfig) (string, error) {
	if !cfg.Type.isValid() {
		return "", fmt.Errorf("invalid secret type: %q", cfg.Type)
	}

	var sb strings.Builder

	createClause := snowflake.CreateClause("SECRET", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "secret_name"
	}

	fmt.Fprintf(&sb, "%s %s\n", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))
	fmt.Fprintf(&sb, "  TYPE = %s", cfg.Type)

	switch cfg.Type {
	case SecretTypeOAuth2:
		fmt.Fprintf(&sb, "\n  API_AUTHENTICATION = %s", snowflake.QuoteIdent(cfg.ApiAuthentication))
		if cfg.OAuthFlow == "CLIENT_CREDENTIALS" {
			if cfg.OAuthScopes != "" {
				parts := strings.Split(cfg.OAuthScopes, ",")
				var quoted []string
				for _, p := range parts {
					quoted = append(quoted, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(strings.TrimSpace(p))))
				}
				fmt.Fprintf(&sb, "\n  OAUTH_SCOPES = (%s)", strings.Join(quoted, ", "))
			}
		} else {
			fmt.Fprintf(&sb, "\n  OAUTH_REFRESH_TOKEN = '%s'", snowflake.EscapeStringLit(cfg.OAuthRefreshToken))
			if cfg.OAuthRefreshTokenExpiry != "" {
				// Snowflake accepts ISO8601/RFC3339
				fmt.Fprintf(&sb, "\n  OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '%s'", snowflake.EscapeStringLit(cfg.OAuthRefreshTokenExpiry))
			}
		}
	case SecretTypeCloudProviderToken:
		fmt.Fprintf(&sb, "\n  API_AUTHENTICATION = '%s'", snowflake.EscapeStringLit(cfg.ApiAuthentication))
		enabled := "TRUE"
		if !cfg.Enabled {
			enabled = "FALSE"
		}
		fmt.Fprintf(&sb, "\n  ENABLED = %s", enabled)
	case SecretTypePassword:
		fmt.Fprintf(&sb, "\n  USERNAME = '%s'", snowflake.EscapeStringLit(cfg.Username))
		fmt.Fprintf(&sb, "\n  PASSWORD = '%s'", snowflake.EscapeStringLit(cfg.Password))
	case SecretTypeGenericString:
		fmt.Fprintf(&sb, "\n  SECRET_STRING = '%s'", snowflake.EscapeStringLit(cfg.SecretString))
	case SecretTypeSymmetricKey:
		fmt.Fprintf(&sb, "\n  ALGORITHM = GENERIC")
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}

// BuildModifySecretSql constructs one or more ALTER SECRET statements.
// It returns an error if cfg.Type is not a known SecretType constant.
func BuildModifySecretSql(db, schema, name string, cfg SecretConfig, originalComment string) ([]string, error) {
	if !cfg.Type.isValid() {
		return nil, fmt.Errorf("invalid secret type: %q", cfg.Type)
	}

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
					quoted = append(quoted, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(strings.TrimSpace(p))))
				}
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_SCOPES = (%s)", strings.Join(quoted, ", ")))
			}
		} else {
			if cfg.OAuthRefreshToken != "" {
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_REFRESH_TOKEN = '%s'", snowflake.EscapeStringLit(cfg.OAuthRefreshToken)))
			}
			if cfg.OAuthRefreshTokenExpiry != "" {
				setClauses = append(setClauses, fmt.Sprintf("OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '%s'", snowflake.EscapeStringLit(cfg.OAuthRefreshTokenExpiry)))
			}
		}
	case SecretTypeCloudProviderToken:
		if cfg.ApiAuthentication != "" {
			setClauses = append(setClauses, fmt.Sprintf("API_AUTHENTICATION = '%s'", snowflake.EscapeStringLit(cfg.ApiAuthentication)))
		}
	case SecretTypePassword:
		if cfg.Username != "" {
			setClauses = append(setClauses, fmt.Sprintf("USERNAME = '%s'", snowflake.EscapeStringLit(cfg.Username)))
		}
		if cfg.Password != "" {
			setClauses = append(setClauses, fmt.Sprintf("PASSWORD = '%s'", snowflake.EscapeStringLit(cfg.Password)))
		}
	case SecretTypeGenericString:
		if cfg.SecretString != "" {
			setClauses = append(setClauses, fmt.Sprintf("SECRET_STRING = '%s'", snowflake.EscapeStringLit(cfg.SecretString)))
		}
	}

	if cfg.Comment != "" {
		setClauses = append(setClauses, "COMMENT = "+snowflake.QuoteTextLit(cfg.Comment))
	}

	if len(setClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER SECRET %s SET %s;", secretRef, strings.Join(setClauses, "\n    ")))
	}

	if originalComment != "" && cfg.Comment == "" {
		statements = append(statements, fmt.Sprintf("ALTER SECRET %s UNSET COMMENT;", secretRef))
	}

	return statements, nil
}
