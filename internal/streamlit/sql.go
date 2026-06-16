// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package streamlit

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// StreamlitConfig holds the parameters for creating a Snowflake STREAMLIT object.
// A Streamlit app renders an interactive Python data app from files in a stage.
// Only the modern FROM <stage location> + MAIN_FILE grammar is supported; the
// deprecated ROOT_LOCATION form is intentionally not offered. OR REPLACE and IF
// NOT EXISTS are mutually exclusive.
type StreamlitConfig struct {
	Name                       string `json:"name"`
	CaseSensitive              bool   `json:"caseSensitive"`
	OrReplace                  bool   `json:"orReplace"`
	IfNotExists                bool   `json:"ifNotExists"`
	StageLocation              string `json:"stageLocation"`              // FROM <stage path> (bare @ reference, e.g. @db.schema.stage/dir)
	MainFile                   string `json:"mainFile"`                   // MAIN_FILE = '<relative path>'
	QueryWarehouse             string `json:"queryWarehouse"`             // QUERY_WAREHOUSE = <warehouse>
	ExternalAccessIntegrations string `json:"externalAccessIntegrations"` // comma-separated EAI names
	Title                      string `json:"title"`                      // TITLE = '<display title>'
	Comment                    string `json:"comment"`
}

// BuildCreateStreamlitSql constructs a CREATE STREAMLIT statement from the given
// config. The source location and main file locate the app's code; when they are
// empty the builder substitutes placeholders so the live preview reads as a
// completable template rather than invalid SQL. Optional clauses are emitted only
// when set, in the order Snowflake documents them. OR REPLACE and IF NOT EXISTS
// are mutually exclusive; if both are set OR REPLACE wins.
//
//	CREATE [OR REPLACE] STREAMLIT [IF NOT EXISTS] <fqn>
//	  FROM <stage location>
//	  MAIN_FILE = '<relative path>'
//	  [QUERY_WAREHOUSE = <warehouse>]
//	  [EXTERNAL_ACCESS_INTEGRATIONS = ( … )]
//	  [TITLE = '<title>']
//	  [COMMENT = '…'];
func BuildCreateStreamlitSql(db, schema string, cfg StreamlitConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " STREAMLIT"
	// IF NOT EXISTS is incompatible with OR REPLACE; only emit it when OR
	// REPLACE is not set.
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "streamlit_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	// FROM <stage location> is a bare stage-path reference (the modern form),
	// not a quoted string literal. The legacy ROOT_LOCATION grammar is not used.
	fmt.Fprintf(&sb, "\n  FROM %s", normalizeStagePath(cfg.StageLocation))

	main := strings.TrimSpace(cfg.MainFile)
	if main == "" {
		main = "streamlit_app.py"
	}
	fmt.Fprintf(&sb, "\n  MAIN_FILE = '%s'", snowflake.EscapeStringLit(main))

	if qw := strings.TrimSpace(cfg.QueryWarehouse); qw != "" {
		fmt.Fprintf(&sb, "\n  QUERY_WAREHOUSE = %s", snowflake.QuoteIdent(qw))
	}
	// EXTERNAL_ACCESS_INTEGRATIONS names are free-text; quote each only when it
	// can't be a bare identifier (caseSensitive=false) so unquoted input resolves
	// case-insensitively as it would in plain SQL.
	if eai := snowflake.SplitIdentList(cfg.ExternalAccessIntegrations, false); len(eai) > 0 {
		fmt.Fprintf(&sb, "\n  EXTERNAL_ACCESS_INTEGRATIONS = (%s)", strings.Join(eai, ", "))
	}
	if t := strings.TrimSpace(cfg.Title); t != "" {
		fmt.Fprintf(&sb, "\n  TITLE = '%s'", snowflake.EscapeStringLit(t))
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}

// normalizeStagePath returns the source location for the FROM clause, ensuring a
// single leading '@' so callers can supply the path with or without it. An empty
// path becomes an obvious placeholder so the preview stays a completable
// template.
func normalizeStagePath(s string) string {
	v := strings.TrimSpace(s)
	if v == "" {
		return "@<stage>"
	}
	if !strings.HasPrefix(v, "@") {
		v = "@" + v
	}
	return v
}
