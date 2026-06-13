// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package dynamictable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// DynamicTableConfig holds the parameters for creating a Snowflake DYNAMIC TABLE
// object. The defining query (Query) is appended verbatim after the AS keyword;
// the remaining fields map to the CREATE DYNAMIC TABLE options.
type DynamicTableConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Transient     bool   `json:"transient"`
	TargetLag     string `json:"targetLag"`     // e.g. "1 minute", "1 hour", or "DOWNSTREAM"
	Warehouse     string `json:"warehouse"`     // warehouse used to refresh the table
	RefreshMode   string `json:"refreshMode"`   // AUTO | FULL | INCREMENTAL (or "" for default)
	Initialize    string `json:"initialize"`    // ON_CREATE | ON_SCHEDULE (or "" for default)
	ClusterBy     string `json:"clusterBy"`     // comma-separated clustering expressions or ""
	Comment       string `json:"comment"`
	Query         string `json:"query"` // AS <select statement>
}

func escLit(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// targetLagClause renders the TARGET_LAG assignment. The special value
// DOWNSTREAM is a keyword and must not be quoted; every other value is a string
// literal such as '1 minute'.
func targetLagClause(lag string) string {
	lag = strings.TrimSpace(lag)
	if strings.EqualFold(lag, "DOWNSTREAM") {
		return "TARGET_LAG = DOWNSTREAM"
	}
	return fmt.Sprintf("TARGET_LAG = '%s'", escLit(lag))
}

// BuildCreateDynamicTableSql constructs a CREATE DYNAMIC TABLE statement from the
// given config. TARGET_LAG, WAREHOUSE, and a defining query are required by
// Snowflake; when they are empty the builder emits placeholders so the preview
// remains a syntactically obvious template the user can complete.
func BuildCreateDynamicTableSql(db, schema string, cfg DynamicTableConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	if cfg.Transient {
		createClause += " TRANSIENT"
	}
	createClause += " DYNAMIC TABLE"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "dynamic_table_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	lag := strings.TrimSpace(cfg.TargetLag)
	if lag == "" {
		lag = "1 minute"
	}
	fmt.Fprintf(&sb, "\n  %s", targetLagClause(lag))

	warehouse := strings.TrimSpace(cfg.Warehouse)
	if warehouse == "" {
		warehouse = "<warehouse>"
		fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", warehouse)
	} else {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", snowflake.QuoteIdent(warehouse))
	}

	if rm := strings.TrimSpace(cfg.RefreshMode); rm != "" {
		fmt.Fprintf(&sb, "\n  REFRESH_MODE = %s", strings.ToUpper(rm))
	}
	if init := strings.TrimSpace(cfg.Initialize); init != "" {
		fmt.Fprintf(&sb, "\n  INITIALIZE = %s", strings.ToUpper(init))
	}
	if cb := strings.TrimSpace(cfg.ClusterBy); cb != "" {
		fmt.Fprintf(&sb, "\n  CLUSTER BY (%s)", cb)
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", escLit(cfg.Comment))
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
