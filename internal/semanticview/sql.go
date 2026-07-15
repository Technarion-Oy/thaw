// SPDX-License-Identifier: GPL-3.0-or-later

package semanticview

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// SemanticViewConfig holds the parameters for creating a Snowflake SEMANTIC
// VIEW object. The Body carries the order-sensitive definition clauses —
// TABLES ( … ) [ RELATIONSHIPS ( … ) ] [ FACTS ( … ) ] [ DIMENSIONS ( … ) ]
// [ METRICS ( … ) ] — verbatim from the create modal's Monaco editor, because
// their per-entity sub-grammar is too large to model as a structured form. The
// COMMENT and COPY GRANTS clauses, which follow the body, are modeled
// separately so the modal can offer them as discrete controls.
type SemanticViewConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	// Body is the TABLES/RELATIONSHIPS/FACTS/DIMENSIONS/METRICS definition,
	// emitted verbatim between the name and the COMMENT clause. The order of the
	// clauses matters to Snowflake (FACTS must precede DIMENSIONS, etc.); the
	// builder does not reorder or validate it.
	Body       string `json:"body"`
	Comment    string `json:"comment"`
	CopyGrants bool   `json:"copyGrants"`
}

// BuildCreateSemanticViewSql constructs a CREATE SEMANTIC VIEW statement from
// the given config. When the body is blank the builder substitutes a minimal
// TABLES placeholder so the live preview reads as a completable template rather
// than invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive in
// Snowflake; the create modal prevents selecting both, and if both are set here
// OR REPLACE wins (IF NOT EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] SEMANTIC VIEW [IF NOT EXISTS] <fqn>
//	  TABLES ( … )
//	  [RELATIONSHIPS ( … )]
//	  [FACTS ( … )]
//	  [DIMENSIONS ( … )]
//	  [METRICS ( … )]
//	  [COMMENT = '…']
//	  [COPY GRANTS];
func BuildCreateSemanticViewSql(db, schema string, cfg SemanticViewConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SEMANTIC VIEW", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "semantic_view_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// The definition body is required by CREATE SEMANTIC VIEW. Emit it verbatim
	// (the order of TABLES/RELATIONSHIPS/FACTS/DIMENSIONS/METRICS is the user's
	// responsibility). Fall back to a minimal TABLES placeholder so the preview
	// stays a completable template.
	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "TABLES (\n    my_table AS <database>.<schema>.<table>\n  )"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	if cfg.CopyGrants {
		sb.WriteString("\n  COPY GRANTS")
	}

	return sb.String() + ";", nil
}
