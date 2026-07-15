// SPDX-License-Identifier: GPL-3.0-or-later

package eventtable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// EventTableConfig holds the parameters for creating a Snowflake EVENT TABLE
// object. Event tables have a fixed, predefined schema, so there are no column
// definitions; CLUSTER BY (on the predefined columns) and the table-level
// properties below are the configurable knobs. Every field is optional — an
// empty/zero value means the corresponding clause is omitted and Snowflake's
// default applies.
type EventTableConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	ClusterBy                  string              `json:"clusterBy"`                  // CLUSTER BY ( <expr>, ... ) — comma-separated expressions (or "")
	DataRetentionTimeInDays    string              `json:"dataRetentionTimeInDays"`    // DATA_RETENTION_TIME_IN_DAYS = <int> (or "")
	MaxDataExtensionTimeInDays string              `json:"maxDataExtensionTimeInDays"` // MAX_DATA_EXTENSION_TIME_IN_DAYS = <int> (or "")
	ChangeTracking             string              `json:"changeTracking"`             // TRUE | FALSE (or "")
	DefaultDdlCollation        string              `json:"defaultDdlCollation"`        // DEFAULT_DDL_COLLATION = '<spec>' (or "")
	CopyGrants                 bool                `json:"copyGrants"`                 // COPY GRANTS
	Comment                    string              `json:"comment"`
	Tags                       []snowflake.TagPair `json:"tags"` // table-level TAG (name = 'value', ...)
}

// BuildCreateEventTableSql constructs a CREATE EVENT TABLE statement from the
// given config. Event tables carry no column list (their schema is fixed), so
// the statement is the qualified name followed by the optional table-level
// properties in the order Snowflake documents them. Each property is emitted
// only when set. OR REPLACE and IF NOT EXISTS are mutually exclusive; if both
// are set OR REPLACE wins.
//
//	CREATE [OR REPLACE] EVENT TABLE [IF NOT EXISTS] <fqn>
//	  [CLUSTER BY ( <expr>, ... )]
//	  [DATA_RETENTION_TIME_IN_DAYS = <int>]
//	  [MAX_DATA_EXTENSION_TIME_IN_DAYS = <int>]
//	  [CHANGE_TRACKING = { TRUE | FALSE }]
//	  [DEFAULT_DDL_COLLATION = '<spec>']
//	  [COPY GRANTS]
//	  [COMMENT = '<string>']
//	  [TAG ( <name> = '<value>', ... )];
func BuildCreateEventTableSql(db, schema string, cfg EventTableConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("EVENT TABLE", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "event_table_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// CLUSTER BY clusters the event table on its predefined columns (e.g. the
	// timestamp column); it comes first after the name in the documented grammar.
	if cb := strings.TrimSpace(cfg.ClusterBy); cb != "" {
		fmt.Fprintf(&sb, "\n  CLUSTER BY (%s)", cb)
	}
	if v := strings.TrimSpace(cfg.DataRetentionTimeInDays); v != "" {
		fmt.Fprintf(&sb, "\n  DATA_RETENTION_TIME_IN_DAYS = %s", v)
	}
	if v := strings.TrimSpace(cfg.MaxDataExtensionTimeInDays); v != "" {
		fmt.Fprintf(&sb, "\n  MAX_DATA_EXTENSION_TIME_IN_DAYS = %s", v)
	}
	if ct := strings.TrimSpace(cfg.ChangeTracking); ct != "" {
		fmt.Fprintf(&sb, "\n  CHANGE_TRACKING = %s", strings.ToUpper(ct))
	}
	if dc := strings.TrimSpace(cfg.DefaultDdlCollation); dc != "" {
		fmt.Fprintf(&sb, "\n  DEFAULT_DDL_COLLATION = '%s'", snowflake.EscapeStringLit(dc))
	}
	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  %s", tc)
	}

	return sb.String() + ";", nil
}
