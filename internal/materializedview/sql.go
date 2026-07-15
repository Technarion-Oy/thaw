// SPDX-License-Identifier: GPL-3.0-or-later

package materializedview

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// MaterializedViewConfig holds the parameters for creating a Snowflake
// MATERIALIZED VIEW object. The defining query (Query) is appended verbatim
// after the AS keyword; the remaining fields map to the view-level CREATE
// MATERIALIZED VIEW options in the order Snowflake documents them. Column-level
// definitions and policy attachments (masking / row access / aggregation),
// per-column comments, and CONTACT are intentionally out of scope for the visual
// builder and are left to raw SQL.
type MaterializedViewConfig struct {
	Name          string              `json:"name"`
	CaseSensitive bool                `json:"caseSensitive"`
	OrReplace     bool                `json:"orReplace"`
	Secure        bool                `json:"secure"` // SECURE materialized view
	IfNotExists   bool                `json:"ifNotExists"`
	CopyGrants    bool                `json:"copyGrants"` // COPY GRANTS (meaningful with OR REPLACE)
	Comment       string              `json:"comment"`
	ClusterBy     string              `json:"clusterBy"` // comma-separated clustering expressions or ""
	Tags          []snowflake.TagPair `json:"tags"`      // view-level TAG (name = 'value', ...)
	Query         string              `json:"query"`     // AS <select statement>
}

// BuildCreateMaterializedViewSql constructs a CREATE MATERIALIZED VIEW statement
// from the given config. A defining query is required by Snowflake; when it is
// empty the builder emits a placeholder so the preview remains a syntactically
// obvious template the user can complete. Optional clauses are emitted only when
// set, in the order Snowflake documents them.
func BuildCreateMaterializedViewSql(db, schema string, cfg MaterializedViewConfig) (string, error) {
	var sb strings.Builder

	body := "MATERIALIZED VIEW"
	if cfg.Secure {
		body = "SECURE " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "materialized_view_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if cb := strings.TrimSpace(cfg.ClusterBy); cb != "" {
		fmt.Fprintf(&sb, "\n  CLUSTER BY (%s)", cb)
	}
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  %s", tc)
	}

	query := strings.TrimSpace(cfg.Query)
	query = strings.TrimRight(query, ";")
	query = strings.TrimSpace(query)
	if query == "" {
		query = "SELECT * FROM <source_table>"
	}
	fmt.Fprintf(&sb, "\n  AS\n%s", query)

	return sb.String() + ";", nil
}
