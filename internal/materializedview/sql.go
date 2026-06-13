// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package materializedview

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// TagPair is a single tag name/value pair used in the view-level TAG clause.
type TagPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MaterializedViewConfig holds the parameters for creating a Snowflake
// MATERIALIZED VIEW object. The defining query (Query) is appended verbatim
// after the AS keyword; the remaining fields map to the view-level CREATE
// MATERIALIZED VIEW options in the order Snowflake documents them. Column-level
// definitions and policy attachments (masking / row access / aggregation),
// per-column comments, and CONTACT are intentionally out of scope for the visual
// builder and are left to raw SQL.
type MaterializedViewConfig struct {
	Name          string    `json:"name"`
	CaseSensitive bool      `json:"caseSensitive"`
	OrReplace     bool      `json:"orReplace"`
	Secure        bool      `json:"secure"` // SECURE materialized view
	IfNotExists   bool      `json:"ifNotExists"`
	CopyGrants    bool      `json:"copyGrants"` // COPY GRANTS (meaningful with OR REPLACE)
	Comment       string    `json:"comment"`
	ClusterBy     string    `json:"clusterBy"` // comma-separated clustering expressions or ""
	Tags          []TagPair `json:"tags"`      // view-level TAG (name = 'value', ...)
	Query         string    `json:"query"`     // AS <select statement>
}

func escLit(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// tagClause renders a view-level TAG (...) clause from the non-empty tag pairs,
// or "" when there are none. Tag names are identifiers; values are string
// literals.
func tagClause(tags []TagPair) string {
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s = '%s'", snowflake.QuoteIdent(name), escLit(t.Value)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "TAG (" + strings.Join(parts, ", ") + ")"
}

// BuildCreateMaterializedViewSql constructs a CREATE MATERIALIZED VIEW statement
// from the given config. A defining query is required by Snowflake; when it is
// empty the builder emits a placeholder so the preview remains a syntactically
// obvious template the user can complete. Optional clauses are emitted only when
// set, in the order Snowflake documents them.
func BuildCreateMaterializedViewSql(db, schema string, cfg MaterializedViewConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	if cfg.Secure {
		createClause += " SECURE"
	}
	createClause += " MATERIALIZED VIEW"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "materialized_view_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", escLit(cfg.Comment))
	}
	if cb := strings.TrimSpace(cfg.ClusterBy); cb != "" {
		fmt.Fprintf(&sb, "\n  CLUSTER BY (%s)", cb)
	}
	if tc := tagClause(cfg.Tags); tc != "" {
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
