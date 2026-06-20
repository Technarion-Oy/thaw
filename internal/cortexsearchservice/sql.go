// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package cortexsearchservice

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// Index-mode constants for CortexSearchServiceConfig.IndexMode. They select which
// of the two documented CREATE CORTEX SEARCH SERVICE shapes the builder emits.
const (
	// IndexModeSingle is the single-index form (ON <search_column>).
	IndexModeSingle = "single"
	// IndexModeMulti is the multi-index form (TEXT INDEXES / VECTOR INDEXES).
	IndexModeMulti = "multi"
)

// CortexSearchServiceConfig holds the parameters for creating a Snowflake CORTEX
// SEARCH SERVICE object. The defining query (Query) is appended verbatim after
// the AS keyword; the remaining fields map to the CREATE CORTEX SEARCH SERVICE
// options in the order Snowflake documents them.
//
// Both documented shapes are modelled, selected by IndexMode:
//   - "single": ON <search_column> (+ optional EMBEDDING_MODEL).
//   - "multi":  TEXT INDEXES … / VECTOR INDEXES … (no EMBEDDING_MODEL / no
//     IF NOT EXISTS, which Snowflake does not allow for this form).
//
// Scoring profiles and per-vector-index advanced tuning beyond the documented
// clauses are still left to raw SQL / post-create ALTER.
type CortexSearchServiceConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// IndexMode selects the CREATE shape: "single" (default) or "multi".
	IndexMode string `json:"indexMode"`

	// SearchColumn is the text column indexed for search (ON <search_column>).
	// Single-index form only; required there and emitted as a placeholder when
	// blank.
	SearchColumn string `json:"searchColumn"`

	// TextIndexes are the text columns indexed for keyword search
	// (TEXT INDEXES col, …). Multi-index form only; optional.
	TextIndexes []string `json:"textIndexes"`

	// VectorIndexes are the column specifications indexed for vector similarity
	// (VECTOR INDEXES spec, …) — e.g. "BODY (model='snowflake-arctic-embed-m')"
	// or a user-provided vector column. Multi-index form only; at least one is
	// required there (emitted as a placeholder when blank).
	VectorIndexes []string `json:"vectorIndexes"`

	// PrimaryKey are the columns forming the service primary key
	// (PRIMARY KEY ( col, … )). Optional in both forms.
	PrimaryKey []string `json:"primaryKey"`

	// Attributes are the columns exposed for filtering (ATTRIBUTES col, …).
	// Optional; omitted when empty.
	Attributes []string `json:"attributes"`

	// Warehouse refreshes/serves the service (WAREHOUSE = …). Required.
	Warehouse string `json:"warehouse"`

	// TargetLag is the maximum data staleness, e.g. "1 hour" (TARGET_LAG = '…').
	// Required.
	TargetLag string `json:"targetLag"`

	// EmbeddingModel selects the vectorization model (EMBEDDING_MODEL = '…').
	// Single-index form only; Snowflake applies a default when omitted, and the
	// model cannot be altered after creation.
	EmbeddingModel string `json:"embeddingModel"`

	// RefreshMode is the index refresh strategy: "" (default), "FULL" or
	// "INCREMENTAL" (REFRESH_MODE = …).
	RefreshMode string `json:"refreshMode"`

	// Initialize controls when the index is first built: "" (default),
	// "ON_CREATE" or "ON_SCHEDULE" (INITIALIZE = …).
	Initialize string `json:"initialize"`

	// FullIndexBuildIntervalDays sets FULL_INDEX_BUILD_INTERVAL_DAYS when > 0.
	FullIndexBuildIntervalDays int `json:"fullIndexBuildIntervalDays"`

	// RequestLogging emits REQUEST_LOGGING = TRUE when set (FALSE is the default,
	// so it is not emitted explicitly).
	RequestLogging bool `json:"requestLogging"`

	// AutoSuspend sets AUTO_SUSPEND = <seconds> when > 0.
	AutoSuspend int `json:"autoSuspend"`

	Comment string `json:"comment"`

	// Query is the base query whose rows are indexed (AS <query>). Required.
	Query string `json:"query"`
}

// BuildCreateCortexSearchServiceSql constructs a CREATE CORTEX SEARCH SERVICE
// statement from the given config, emitting either the single-index
// (ON <search_column>) or multi-index (TEXT INDEXES / VECTOR INDEXES) shape per
// cfg.IndexMode. The index locator, WAREHOUSE, TARGET_LAG and the defining query
// are required by Snowflake; when they are empty the builder emits placeholders
// so the preview remains a syntactically obvious template the user can complete.
// Optional clauses are emitted only when set, in the order Snowflake documents
// them. OR REPLACE and IF NOT EXISTS are mutually exclusive; the create modal
// prevents selecting both, and if both are set here OR REPLACE wins. The
// multi-index form does not support IF NOT EXISTS or EMBEDDING_MODEL, so those
// are dropped in that mode.
//
//	CREATE [OR REPLACE] CORTEX SEARCH SERVICE [IF NOT EXISTS] <fqn>
//	  { ON <search_column> | TEXT INDEXES … VECTOR INDEXES … }
//	  [PRIMARY KEY ( <col>, … )]
//	  [ATTRIBUTES <col>, …]
//	  WAREHOUSE = <warehouse>
//	  TARGET_LAG = '<lag>'
//	  [EMBEDDING_MODEL = '<model>']         -- single-index only
//	  [REFRESH_MODE = FULL|INCREMENTAL]
//	  [INITIALIZE = ON_CREATE|ON_SCHEDULE]
//	  [FULL_INDEX_BUILD_INTERVAL_DAYS = <n>]
//	  [REQUEST_LOGGING = TRUE]
//	  [AUTO_SUSPEND = <seconds>]
//	  [COMMENT = '<comment>']
//	  AS
//	  <query>;
func BuildCreateCortexSearchServiceSql(db, schema string, cfg CortexSearchServiceConfig) (string, error) {
	var sb strings.Builder

	multi := strings.EqualFold(strings.TrimSpace(cfg.IndexMode), IndexModeMulti)

	// The multi-index form does not support IF NOT EXISTS.
	ifNotExists := cfg.IfNotExists && !multi
	createClause := snowflake.CreateClause("CORTEX SEARCH SERVICE", cfg.OrReplace, ifNotExists)

	name := cfg.Name
	if name == "" {
		name = "search_service_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Index locator. Columns/specs are emitted verbatim so the user controls any
	// quoting of case-sensitive names (mirrors how CLUSTER BY expressions are
	// handled) and so vector index specs like "BODY (model='…')" pass through.
	if multi {
		if texts := cleanList(cfg.TextIndexes); len(texts) > 0 {
			fmt.Fprintf(&sb, "\n  TEXT INDEXES %s", strings.Join(texts, ", "))
		}
		vecs := cleanList(cfg.VectorIndexes)
		if len(vecs) == 0 {
			// VECTOR INDEXES requires at least one column for a multi-index service.
			vecs = []string{"<vector_column>"}
		}
		fmt.Fprintf(&sb, "\n  VECTOR INDEXES %s", strings.Join(vecs, ", "))
	} else {
		searchColumn := strings.TrimSpace(cfg.SearchColumn)
		if searchColumn == "" {
			searchColumn = "<search_column>"
		}
		fmt.Fprintf(&sb, "\n  ON %s", searchColumn)
	}

	if pk := cleanList(cfg.PrimaryKey); len(pk) > 0 {
		fmt.Fprintf(&sb, "\n  PRIMARY KEY ( %s )", strings.Join(pk, ", "))
	}

	if attrs := cleanList(cfg.Attributes); len(attrs) > 0 {
		fmt.Fprintf(&sb, "\n  ATTRIBUTES %s", strings.Join(attrs, ", "))
	}

	warehouse := strings.TrimSpace(cfg.Warehouse)
	if warehouse == "" {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = <warehouse>")
	} else {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", snowflake.QuoteIdent(warehouse))
	}

	lag := strings.TrimSpace(cfg.TargetLag)
	if lag == "" {
		lag = "1 hour"
	}
	fmt.Fprintf(&sb, "\n  TARGET_LAG = '%s'", snowflake.EscapeStringLit(lag))

	// EMBEDDING_MODEL is single-index only. Model names contain hyphens (e.g.
	// snowflake-arctic-embed-m-v1.5), so they must be passed as a quoted string
	// literal.
	if !multi {
		if em := strings.TrimSpace(cfg.EmbeddingModel); em != "" {
			fmt.Fprintf(&sb, "\n  EMBEDDING_MODEL = '%s'", snowflake.EscapeStringLit(em))
		}
	}

	if rm := strings.ToUpper(strings.TrimSpace(cfg.RefreshMode)); rm != "" {
		fmt.Fprintf(&sb, "\n  REFRESH_MODE = %s", rm)
	}
	if init := strings.ToUpper(strings.TrimSpace(cfg.Initialize)); init != "" {
		fmt.Fprintf(&sb, "\n  INITIALIZE = %s", init)
	}
	if cfg.FullIndexBuildIntervalDays > 0 {
		fmt.Fprintf(&sb, "\n  FULL_INDEX_BUILD_INTERVAL_DAYS = %d", cfg.FullIndexBuildIntervalDays)
	}
	if cfg.RequestLogging {
		sb.WriteString("\n  REQUEST_LOGGING = TRUE")
	}
	if cfg.AutoSuspend > 0 {
		fmt.Fprintf(&sb, "\n  AUTO_SUSPEND = %d", cfg.AutoSuspend)
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	query := strings.TrimSpace(cfg.Query)
	query = strings.TrimRight(query, ";")
	query = strings.TrimSpace(query)
	if query == "" {
		query = "SELECT id, text_column FROM <source_table>"
	}
	fmt.Fprintf(&sb, "\n  AS\n%s", query)

	return sb.String() + ";", nil
}

// cleanList trims each entry and drops blanks, preserving order.
func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// FormatAttributes renders a comma-separated ATTRIBUTES column list (without the
// surrounding parentheses) for the ALTER … SET ATTRIBUTES ( … ) clause. It is
// exposed over IPC so the properties modal can build the clause without
// duplicating the trim/skip-blank logic. Returns "" when no non-blank columns
// remain.
func FormatAttributes(columns []string) string {
	return strings.Join(cleanList(columns), ", ")
}
