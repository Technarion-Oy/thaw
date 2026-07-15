// SPDX-License-Identifier: GPL-3.0-or-later

package dynamictable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// DynamicTableConfig holds the parameters for creating a Snowflake DYNAMIC TABLE
// object. The defining query (Query) is appended verbatim after the AS keyword;
// the remaining fields map to the table-level CREATE DYNAMIC TABLE options in
// the order Snowflake documents them. Column-level definitions and policy
// attachments (row access / aggregation / masking / projection), BACKFILL FROM,
// START AT, EXECUTE AS USER, and REFRESH USING are intentionally out of scope
// for the visual builder and are left to raw SQL.
type DynamicTableConfig struct {
	Name                       string              `json:"name"`
	CaseSensitive              bool                `json:"caseSensitive"`
	OrReplace                  bool                `json:"orReplace"`
	IfNotExists                bool                `json:"ifNotExists"`
	Transient                  bool                `json:"transient"`
	TargetLag                  string              `json:"targetLag"`                  // e.g. "1 minute", "2 hours", or "DOWNSTREAM"
	Scheduler                  string              `json:"scheduler"`                  // ENABLE | DISABLE (or "" for default)
	Warehouse                  string              `json:"warehouse"`                  // warehouse used to refresh the table
	InitializationWarehouse    string              `json:"initializationWarehouse"`    // warehouse used for the initial refresh (or "")
	RefreshMode                string              `json:"refreshMode"`                // AUTO | FULL | INCREMENTAL | ADAPTIVE | CUSTOM_INCREMENTAL (or "")
	Initialize                 string              `json:"initialize"`                 // ON_CREATE | ON_SCHEDULE (or "")
	ClusterBy                  string              `json:"clusterBy"`                  // comma-separated clustering expressions or ""
	DataRetentionTimeInDays    string              `json:"dataRetentionTimeInDays"`    // integer string or ""
	MaxDataExtensionTimeInDays string              `json:"maxDataExtensionTimeInDays"` // integer string or ""
	Comment                    string              `json:"comment"`
	CopyGrants                 bool                `json:"copyGrants"`   // COPY GRANTS (meaningful with OR REPLACE)
	RequireUser                bool                `json:"requireUser"`  // REQUIRE USER
	RowTimestamp               string              `json:"rowTimestamp"` // TRUE | FALSE (or "" for default)
	Tags                       []snowflake.TagPair `json:"tags"`         // table-level TAG (name = 'value', ...)
	Query                      string              `json:"query"`        // AS <select statement>
}

// targetLagClause renders the TARGET_LAG assignment. The special value
// DOWNSTREAM is a keyword and must not be quoted; every other value is a string
// literal such as '1 minute'.
func targetLagClause(lag string) string {
	lag = strings.TrimSpace(lag)
	if strings.EqualFold(lag, "DOWNSTREAM") {
		return "TARGET_LAG = DOWNSTREAM"
	}
	return fmt.Sprintf("TARGET_LAG = '%s'", snowflake.EscapeStringLit(lag))
}

// BuildCreateDynamicTableSql constructs a CREATE DYNAMIC TABLE statement from the
// given config. TARGET_LAG, WAREHOUSE, and a defining query are required by
// Snowflake; when they are empty the builder emits placeholders so the preview
// remains a syntactically obvious template the user can complete. Optional
// clauses are emitted only when set, in the order Snowflake documents them.
func BuildCreateDynamicTableSql(db, schema string, cfg DynamicTableConfig) (string, error) {
	var sb strings.Builder

	body := "DYNAMIC TABLE"
	if cfg.Transient {
		body = "TRANSIENT " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "dynamic_table_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	lag := strings.TrimSpace(cfg.TargetLag)
	if lag == "" {
		lag = "1 minute"
	}
	fmt.Fprintf(&sb, "\n  %s", targetLagClause(lag))

	if sched := strings.TrimSpace(cfg.Scheduler); sched != "" {
		fmt.Fprintf(&sb, "\n  SCHEDULER = %s", strings.ToUpper(sched))
	}

	warehouse := strings.TrimSpace(cfg.Warehouse)
	if warehouse == "" {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = <warehouse>")
	} else {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", snowflake.QuoteIdent(warehouse))
	}

	if iw := strings.TrimSpace(cfg.InitializationWarehouse); iw != "" {
		fmt.Fprintf(&sb, "\n  INITIALIZATION_WAREHOUSE = %s", snowflake.QuoteIdent(iw))
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
	if dr := strings.TrimSpace(cfg.DataRetentionTimeInDays); dr != "" {
		fmt.Fprintf(&sb, "\n  DATA_RETENTION_TIME_IN_DAYS = %s", dr)
	}
	if mde := strings.TrimSpace(cfg.MaxDataExtensionTimeInDays); mde != "" {
		fmt.Fprintf(&sb, "\n  MAX_DATA_EXTENSION_TIME_IN_DAYS = %s", mde)
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  %s", tc)
	}
	if cfg.RequireUser {
		fmt.Fprintf(&sb, "\n  REQUIRE USER")
	}
	if rt := strings.TrimSpace(cfg.RowTimestamp); rt != "" {
		fmt.Fprintf(&sb, "\n  ROW_TIMESTAMP = %s", strings.ToUpper(rt))
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
