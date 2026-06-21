// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package view

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ViewConfig holds the parameters for creating a Snowflake VIEW object. The
// defining query (Query) is appended verbatim after the AS keyword; the
// remaining fields map to the view-level CREATE VIEW options in the order
// Snowflake documents them. Policy attachments (masking / row access), per-column
// comments, and CONTACT are intentionally out of scope for the visual builder and
// are left to raw SQL.
type ViewConfig struct {
	Name          string              `json:"name"`
	CaseSensitive bool                `json:"caseSensitive"`
	OrReplace     bool                `json:"orReplace"`
	Secure        bool                `json:"secure"`      // SECURE view
	Recursive     bool                `json:"recursive"`   // RECURSIVE view
	IfNotExists   bool                `json:"ifNotExists"` //
	CopyGrants    bool                `json:"copyGrants"`  // COPY GRANTS (meaningful with OR REPLACE)
	Comment       string              `json:"comment"`
	Columns       string              `json:"columns"` // optional comma-separated explicit column name list
	Tags          []snowflake.TagPair `json:"tags"`    // view-level TAG (name = 'value', ...)
	Query         string              `json:"query"`   // AS <select statement>
}

// BuildCreateViewSql constructs a CREATE VIEW statement from the given config. A
// defining query is required by Snowflake; when it is empty the builder emits a
// placeholder so the preview remains a syntactically obvious template the user
// can complete. Optional clauses are emitted only when set, in the order
// Snowflake documents them.
func BuildCreateViewSql(db, schema string, cfg ViewConfig) (string, error) {
	var sb strings.Builder

	body := "VIEW"
	if cfg.Recursive {
		body = "RECURSIVE " + body
	}
	if cfg.Secure {
		body = "SECURE " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "view_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// The explicit column list is emitted verbatim (like the defining Query):
	// the CREATE VIEW column-list grammar allows per-column extras such as
	// `<col> COMMENT '...'`, so the builder cannot naively quote each token. The
	// create modal's help text tells the user to double-quote names that need it.
	if cols := strings.TrimSpace(cfg.Columns); cols != "" {
		fmt.Fprintf(&sb, "\n  (%s)", cols)
	}
	// Clause order per the CREATE VIEW grammar: [WITH] TAG → COPY GRANTS →
	// COMMENT (CREATE VIEW's parser is order-sensitive, and this order differs
	// from the dynamic-table builder's COMMENT → COPY GRANTS → TAG).
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  %s", tc)
	}
	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	query := strings.TrimSpace(cfg.Query)
	query = strings.TrimRight(query, ";")
	query = strings.TrimSpace(query)
	if query == "" {
		query = "SELECT * FROM <source_table>"
	}
	fmt.Fprintf(&sb, "\n  AS\n%s", query)

	return sb.String() + ";", nil
}
