// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package agent

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AgentConfig holds the parameters for creating a Snowflake AGENT object. The
// fields map to the CREATE AGENT options in the order Snowflake documents them:
// the name, the optional COMMENT and PROFILE, then the required FROM
// SPECIFICATION body. Profile is a JSON object string ({"display_name": …,
// "avatar": …, "color": …}); Specification is the YAML/JSON agent spec (models,
// orchestration, instructions, tools, tool_resources) and is emitted inside a
// $$-delimited block so multi-line YAML needs no escaping.
type AgentConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Comment       string `json:"comment"`
	Profile       string `json:"profile"`       // JSON object string for PROFILE; omitted when blank
	Specification string `json:"specification"` // YAML/JSON agent spec for FROM SPECIFICATION
}

// BuildCreateAgentSql constructs a CREATE AGENT statement from the given config.
// When required parts are blank the builder substitutes placeholders so the live
// preview reads as a completable template rather than invalid SQL. OR REPLACE and
// IF NOT EXISTS are mutually exclusive in Snowflake; the create modal prevents
// selecting both, and if both are set here OR REPLACE wins (IF NOT EXISTS is
// dropped by CreateClause).
//
//	CREATE [OR REPLACE] AGENT [IF NOT EXISTS] <fqn>
//	  [COMMENT = '…']
//	  [PROFILE = '<json>']
//	  FROM SPECIFICATION
//	  $$
//	  <spec>
//	  $$;
func BuildCreateAgentSql(db, schema string, cfg AgentConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("AGENT", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "agent_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	// PROFILE is a JSON object string. Escape as free text (backslashes preserved)
	// since JSON may contain backslash escape sequences that must survive the
	// single-quoted SQL literal verbatim.
	if p := strings.TrimSpace(cfg.Profile); p != "" {
		fmt.Fprintf(&sb, "\n  PROFILE = %s", snowflake.QuoteTextLit(p))
	}

	// The specification is required by CREATE AGENT. Wrap it in $$ … $$ so the
	// multi-line YAML/JSON needs no escaping. Fall back to a minimal placeholder
	// spec so the preview stays a completable template.
	spec := strings.TrimSpace(cfg.Specification)
	if spec == "" {
		spec = "models:\n  orchestration: auto"
	}
	fmt.Fprintf(&sb, "\n  FROM SPECIFICATION\n  $$\n%s\n  $$", spec)

	return sb.String() + ";", nil
}
