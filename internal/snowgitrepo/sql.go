// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowgitrepo

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

// GitRepositoryConfig holds the parameters for creating a Snowflake GIT REPOSITORY object.
type GitRepositoryConfig struct {
	Name           string              `json:"name"`
	CaseSensitive  bool                `json:"caseSensitive"`
	OrReplace      bool                `json:"orReplace"`
	IfNotExists    bool                `json:"ifNotExists"`
	OriginUrl      string              `json:"originUrl"`
	ApiIntegration string              `json:"apiIntegration"`
	GitCredentials string              `json:"gitCredentials"` // fully qualified "db"."schema"."secret" or ""
	Comment        string              `json:"comment"`
	Tags           []snowflake.TagPair `json:"tags"`
}

// BuildCreateGitRepositorySql constructs a CREATE GIT REPOSITORY SQL statement.
func BuildCreateGitRepositorySql(db, schema string, cfg GitRepositoryConfig) (string, error) {
	if cfg.OriginUrl == "" {
		return "", fmt.Errorf("originUrl is required")
	}
	if cfg.ApiIntegration == "" {
		return "", fmt.Errorf("apiIntegration is required")
	}

	var sb strings.Builder

	createClause := snowflake.CreateClause("GIT REPOSITORY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "repo_name"
	}

	fmt.Fprintf(&sb, "%s %s\n", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))
	fmt.Fprintf(&sb, "  ORIGIN = '%s'", snowflake.EscapeStringLit(cfg.OriginUrl))
	fmt.Fprintf(&sb, "\n  API_INTEGRATION = %s", snowflake.QuoteIdent(cfg.ApiIntegration))

	if cfg.GitCredentials != "" {
		fmt.Fprintf(&sb, "\n  GIT_CREDENTIALS = %s", cfg.GitCredentials)
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  WITH %s", tc)
	}

	return sb.String() + ";", nil
}

// BuildModifyGitRepositorySql constructs one or more ALTER GIT REPOSITORY statements.
// originalComment, originalIntegration, and originalCredentials are the values currently
// set on the object (used to detect UNSET operations).
func BuildModifyGitRepositorySql(db, schema, name string, cfg GitRepositoryConfig, originalComment, originalIntegration, originalCredentials string) ([]string, error) {
	repoRef := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	var statements []string
	var setClauses []string

	// API_INTEGRATION can only be SET (cannot UNSET per Snowflake limitation)
	if cfg.ApiIntegration != "" && cfg.ApiIntegration != originalIntegration {
		setClauses = append(setClauses, fmt.Sprintf("API_INTEGRATION = %s", snowflake.QuoteIdent(cfg.ApiIntegration)))
	}

	// GIT_CREDENTIALS: set new value if non-empty
	if cfg.GitCredentials != "" && cfg.GitCredentials != originalCredentials {
		setClauses = append(setClauses, fmt.Sprintf("GIT_CREDENTIALS = %s", cfg.GitCredentials))
	}

	// COMMENT: set if non-empty
	if cfg.Comment != "" {
		setClauses = append(setClauses, "COMMENT = "+snowflake.QuoteTextLit(cfg.Comment))
	}

	if len(setClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER GIT REPOSITORY %s SET\n  %s;", repoRef, strings.Join(setClauses, "\n  ")))
	}

	// UNSET GIT_CREDENTIALS if cleared
	var unsetClauses []string
	if originalCredentials != "" && cfg.GitCredentials == "" {
		unsetClauses = append(unsetClauses, "GIT_CREDENTIALS")
	}
	// UNSET COMMENT if cleared
	if originalComment != "" && cfg.Comment == "" {
		unsetClauses = append(unsetClauses, "COMMENT")
	}
	if len(unsetClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER GIT REPOSITORY %s UNSET %s;", repoRef, strings.Join(unsetClauses, ", ")))
	}

	return statements, nil
}
