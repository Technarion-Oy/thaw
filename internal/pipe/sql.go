// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package pipe

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

// TagPair is a single tag name/value pair used in SET TAG / UNSET TAG clauses.
type TagPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PipeConfig holds the parameters for creating a Snowflake PIPE object.
type PipeConfig struct {
	Name             string `json:"name"`
	CaseSensitive    bool   `json:"caseSensitive"`
	OrReplace        bool   `json:"orReplace"`
	IfNotExists      bool   `json:"ifNotExists"`
	AutoIngest       bool   `json:"autoIngest"`
	ErrorIntegration string `json:"errorIntegration"` // integration name or ""
	AwsSnsTopic      string `json:"awsSnsTopic"`      // AWS SNS topic ARN or ""
	Integration      string `json:"integration"`      // notification integration name or ""
	Comment          string `json:"comment"`
	CopyStatement    string `json:"copyStatement"` // COPY INTO ... statement
}

// RefreshPipeConfig holds parameters for ALTER PIPE ... REFRESH.
type RefreshPipeConfig struct {
	Prefix        string `json:"prefix"`        // optional PREFIX path
	ModifiedAfter string `json:"modifiedAfter"` // optional ISO timestamp
}

func escLit(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// validateCopyStatement ensures rawStmt contains exactly one SQL statement and
// that it begins with COPY INTO. Returns the trimmed statement on success.
func validateCopyStatement(rawStmt string) (string, error) {
	stmts := snowflake.SplitStatements(rawStmt)
	if len(stmts) != 1 {
		return "", fmt.Errorf("copy statement must contain exactly one SQL statement, got %d", len(stmts))
	}
	stmt := strings.TrimSpace(stmts[0])
	if !strings.HasPrefix(strings.ToUpper(stmt), "COPY INTO ") {
		return "", fmt.Errorf("copy statement must start with COPY INTO")
	}
	return stmt, nil
}

// BuildCreatePipeSql constructs a CREATE PIPE SQL statement from the given config.
func BuildCreatePipeSql(db, schema string, cfg PipeConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " PIPE"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "pipe_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	if cfg.AutoIngest {
		fmt.Fprintf(&sb, "\n  AUTO_INGEST = TRUE")
	}
	if cfg.ErrorIntegration != "" {
		fmt.Fprintf(&sb, "\n  ERROR_INTEGRATION = %s", snowflake.QuoteIdent(cfg.ErrorIntegration))
	}
	if cfg.AwsSnsTopic != "" {
		fmt.Fprintf(&sb, "\n  AWS_SNS_TOPIC = '%s'", escLit(cfg.AwsSnsTopic))
	}
	if cfg.Integration != "" {
		fmt.Fprintf(&sb, "\n  INTEGRATION = '%s'", escLit(cfg.Integration))
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", escLit(cfg.Comment))
	}

	copyStmt := strings.TrimSpace(cfg.CopyStatement)
	if copyStmt == "" {
		copyStmt = "COPY INTO <table> FROM @<stage>"
	} else {
		validated, err := validateCopyStatement(copyStmt)
		if err != nil {
			return "", err
		}
		copyStmt = validated
	}
	fmt.Fprintf(&sb, "\n  AS\n%s", copyStmt)

	return sb.String() + ";", nil
}

// BuildRefreshPipeSql constructs an ALTER PIPE ... REFRESH SQL statement.
func BuildRefreshPipeSql(db, schema, name string, cfg RefreshPipeConfig) (string, error) {
	pipeRef := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))

	var sb strings.Builder
	fmt.Fprintf(&sb, "ALTER PIPE %s REFRESH", pipeRef)

	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix != "" {
		fmt.Fprintf(&sb, "\n  PREFIX = '%s'", escLit(prefix))
	}

	modifiedAfter := strings.TrimSpace(cfg.ModifiedAfter)
	if modifiedAfter != "" {
		fmt.Fprintf(&sb, "\n  MODIFIED_AFTER = '%s'", escLit(modifiedAfter))
	}

	return sb.String() + ";", nil
}
