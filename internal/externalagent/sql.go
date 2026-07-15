// SPDX-License-Identifier: GPL-3.0-or-later

package externalagent

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ExternalAgentConfig holds the parameters for creating a Snowflake EXTERNAL
// AGENT object. External agents are version-based and have no inline
// specification, so the config is limited to the name, the OR REPLACE / IF NOT
// EXISTS flags, an optional initial version name (WITH VERSION), and an optional
// COMMENT.
type ExternalAgentConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	VersionName   string `json:"versionName"` // WITH VERSION <name>; omitted when blank
	Comment       string `json:"comment"`
}

// BuildCreateExternalAgentSql constructs a CREATE EXTERNAL AGENT statement from
// the given config. When the name is blank the builder substitutes a placeholder
// so the live preview reads as a completable template rather than invalid SQL.
// OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake; the create
// modal prevents selecting both, and if both are set here OR REPLACE wins (IF NOT
// EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] EXTERNAL AGENT [IF NOT EXISTS] <fqn>
//	  [WITH VERSION <version_name>]
//	  [COMMENT = '…'];
func BuildCreateExternalAgentSql(db, schema string, cfg ExternalAgentConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("EXTERNAL AGENT", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "external_agent_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Version names are emitted unquoted so Snowflake's standard identifier
	// resolution applies (folding to uppercase), matching how versions are stored
	// and referenced (V1, V2, …).
	if v := strings.TrimSpace(cfg.VersionName); v != "" {
		fmt.Fprintf(&sb, "\n  WITH VERSION %s", v)
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
