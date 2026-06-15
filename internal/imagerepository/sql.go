// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package imagerepository

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ImageRepositoryConfig holds the parameters for creating a Snowflake IMAGE
// REPOSITORY object. Image repositories have a minimal grammar — beyond the
// name they only accept OR REPLACE / IF NOT EXISTS and an optional COMMENT — so
// the config is correspondingly small.
type ImageRepositoryConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Comment       string `json:"comment"`
}

// BuildCreateImageRepositorySql constructs a CREATE IMAGE REPOSITORY statement
// from the given config. When the name is blank the builder substitutes a
// placeholder so the live preview reads as a completable template rather than
// invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive in
// Snowflake; the create modal prevents selecting both, and if both are set here
// OR REPLACE wins (IF NOT EXISTS is dropped).
//
//	CREATE [OR REPLACE] IMAGE REPOSITORY [IF NOT EXISTS] <fqn>
//	  [COMMENT = '…'];
func BuildCreateImageRepositorySql(db, schema string, cfg ImageRepositoryConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " IMAGE REPOSITORY"
	// IF NOT EXISTS is incompatible with OR REPLACE; only emit it when OR
	// REPLACE is not set.
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "image_repository_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}
