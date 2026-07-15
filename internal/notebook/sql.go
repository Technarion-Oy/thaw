// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package notebook

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// NotebookConfig holds the parameters for creating a Snowflake NOTEBOOK object.
// A Notebook can be created empty (leave SourceLocation/MainFile blank — Snowflake
// provisions an editable notebook on first open) or from files already staged in
// an internal stage or git repository (set SourceLocation + MainFile). OR REPLACE
// and IF NOT EXISTS are mutually exclusive.
type NotebookConfig struct {
	Name           string `json:"name"`
	CaseSensitive  bool   `json:"caseSensitive"`
	OrReplace      bool   `json:"orReplace"`
	IfNotExists    bool   `json:"ifNotExists"`
	SourceLocation string `json:"sourceLocation"` // FROM '<stage or git path>'
	MainFile       string `json:"mainFile"`       // MAIN_FILE = '<relative .ipynb path>'
	QueryWarehouse string `json:"queryWarehouse"` // QUERY_WAREHOUSE = <warehouse>
	Comment        string `json:"comment"`
}

// BuildCreateNotebookSql constructs a CREATE NOTEBOOK statement from the given
// config. Optional clauses are emitted only when set, in the order Snowflake
// documents them. OR REPLACE and IF NOT EXISTS are mutually exclusive; if both are
// set OR REPLACE wins (via snowflake.CreateClause).
//
//	CREATE [OR REPLACE] NOTEBOOK [IF NOT EXISTS] <fqn>
//	  [FROM '<source location>']
//	  [MAIN_FILE = '<relative path>']
//	  [COMMENT = '…']
//	  [QUERY_WAREHOUSE = <warehouse>];
func BuildCreateNotebookSql(db, schema string, cfg NotebookConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("NOTEBOOK", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "notebook_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// FROM '<source location>' is a quoted string literal (unlike STREAMLIT's bare
	// stage reference). Emitted only when the notebook is created from staged files.
	if src := strings.TrimSpace(cfg.SourceLocation); src != "" {
		fmt.Fprintf(&sb, "\n  FROM '%s'", snowflake.EscapeStringLit(src))
	}
	if main := strings.TrimSpace(cfg.MainFile); main != "" {
		fmt.Fprintf(&sb, "\n  MAIN_FILE = '%s'", snowflake.EscapeStringLit(main))
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if qw := strings.TrimSpace(cfg.QueryWarehouse); qw != "" {
		fmt.Fprintf(&sb, "\n  QUERY_WAREHOUSE = %s", snowflake.QuoteIdent(qw))
	}

	return sb.String() + ";", nil
}
