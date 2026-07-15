// SPDX-License-Identifier: GPL-3.0-or-later

package mcpserver

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// MCPServerConfig holds the parameters for creating a Snowflake MCP SERVER
// object. The only payload is the YAML Specification (the tools list); MCP
// servers have no COMMENT clause in CREATE. The Specification is emitted inside
// a $$-delimited block so multi-line YAML needs no escaping.
type MCPServerConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Specification string `json:"specification"` // YAML tools spec for FROM SPECIFICATION
}

// BuildCreateMCPServerSql constructs a CREATE MCP SERVER statement from the
// given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
// the create modal prevents selecting both, and if both are set here OR REPLACE
// wins (IF NOT EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] MCP SERVER [IF NOT EXISTS] <fqn>
//	  FROM SPECIFICATION
//	  $THAW$
//	  <spec>
//	  $THAW$;
func BuildCreateMCPServerSql(db, schema string, cfg MCPServerConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("MCP SERVER", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "mcp_server_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// The specification is required by CREATE MCP SERVER. Wrap it in a tagged
	// $THAW$ … $THAW$ dollar-quote so the multi-line YAML needs no escaping and a
	// literal `$$` inside a tool description / title can't prematurely close the
	// block. Fall back to a minimal placeholder spec so the preview stays a
	// completable template.
	spec := strings.TrimSpace(cfg.Specification)
	if spec == "" {
		spec = "tools: []"
	}
	fmt.Fprintf(&sb, "\n  FROM SPECIFICATION\n  $THAW$\n%s\n  $THAW$", spec)

	return sb.String() + ";", nil
}
