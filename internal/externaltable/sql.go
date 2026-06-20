// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package externaltable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ExternalTableColumn is one column of an external table. Snowflake derives
// every external-table column from the staged file via an expression, so each
// column carries a transformation: <name> <type> AS (<expression>). Columns
// flagged Partition are additionally emitted in the PARTITION BY clause.
type ExternalTableColumn struct {
	Name       string `json:"name"`       // column identifier
	Type       string `json:"type"`       // Snowflake data type, emitted verbatim
	Expression string `json:"expression"` // AS (<expr>), e.g. (value:c1::varchar) or metadata$filename
	Partition  bool   `json:"partition"`  // include in PARTITION BY (...)
}

// ExternalTableConfig holds the parameters for creating a Snowflake EXTERNAL
// TABLE object. LOCATION is required by Snowflake; FILE_FORMAT defaults to CSV
// when neither a named format nor a type is supplied. Column definitions are
// optional — an external table created without them exposes only the default
// VALUE variant column. Delta Lake / Iceberg table formats, inline constraints,
// and policy attachments are intentionally out of scope for the visual builder
// and are left to raw SQL.
type ExternalTableConfig struct {
	Name            string                `json:"name"`
	CaseSensitive   bool                  `json:"caseSensitive"`
	OrReplace       bool                  `json:"orReplace"`
	IfNotExists     bool                  `json:"ifNotExists"`
	Columns         []ExternalTableColumn `json:"columns"`
	Location        string                `json:"location"`        // @<stage>[/<path>] — required
	RefreshOnCreate string                `json:"refreshOnCreate"` // TRUE | FALSE (or "" for default)
	AutoRefresh     string                `json:"autoRefresh"`     // TRUE | FALSE (or "" for default)
	Pattern         string                `json:"pattern"`         // regex matching file paths to include
	FileFormatName  string                `json:"fileFormatName"`  // FORMAT_NAME = '<name>' (takes precedence)
	FileFormatType  string                `json:"fileFormatType"`  // TYPE = { CSV | JSON | AVRO | ORC | PARQUET }
	AwsSnsTopic     string                `json:"awsSnsTopic"`     // AWS_SNS_TOPIC for S3 auto-refresh
	CopyGrants      bool                  `json:"copyGrants"`      // COPY GRANTS (meaningful with OR REPLACE)
	Comment         string                `json:"comment"`
	Tags            []snowflake.TagPair   `json:"tags"` // table-level TAG (name = 'value', ...)
}

// fileFormatClause renders the FILE_FORMAT clause. A named format takes
// precedence over an inline TYPE. When a TYPE is set it is used directly; when
// neither a name nor a type is set the clause emits a completable
// FORMAT_NAME placeholder so the preview reflects "named format, not yet chosen"
// rather than silently defaulting to a TYPE (the UI always supplies a concrete
// TYPE when the user is in inline-type mode).
func fileFormatClause(cfg ExternalTableConfig) string {
	if name := strings.TrimSpace(cfg.FileFormatName); name != "" {
		return fmt.Sprintf("FILE_FORMAT = (FORMAT_NAME = '%s')", snowflake.EscapeStringLit(name))
	}
	if typ := strings.TrimSpace(cfg.FileFormatType); typ != "" {
		return fmt.Sprintf("FILE_FORMAT = (TYPE = %s)", strings.ToUpper(typ))
	}
	return "FILE_FORMAT = (FORMAT_NAME = '<file_format>')"
}

// BuildCreateExternalTableSql constructs a CREATE EXTERNAL TABLE statement from
// the given config. LOCATION is required by Snowflake; when it is empty the
// builder emits a placeholder so the preview remains a syntactically obvious
// template the user can complete. Optional clauses are emitted only when set, in
// the order Snowflake documents them.
func BuildCreateExternalTableSql(db, schema string, cfg ExternalTableConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("EXTERNAL TABLE", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "external_table_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Column definitions (optional). Each column is derived from the staged file
	// via an expression: <name> <type> AS (<expr>).
	cols := make([]ExternalTableColumn, 0, len(cfg.Columns))
	for _, c := range cfg.Columns {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		cols = append(cols, c)
	}
	if len(cols) > 0 {
		lines := make([]string, 0, len(cols))
		for _, c := range cols {
			typ := strings.TrimSpace(c.Type)
			if typ == "" {
				typ = "VARCHAR"
			}
			expr := strings.TrimSpace(c.Expression)
			if expr == "" {
				expr = "value"
			}
			lines = append(lines, fmt.Sprintf("  %s %s AS (%s)",
				snowflake.QuoteOrBare(c.Name, cfg.CaseSensitive), typ, expr))
		}
		fmt.Fprintf(&sb, " (\n%s\n)", strings.Join(lines, ",\n"))
	}

	// PARTITION BY the flagged partition columns, in declared order.
	partCols := make([]string, 0)
	for _, c := range cols {
		if c.Partition {
			partCols = append(partCols, snowflake.QuoteOrBare(c.Name, cfg.CaseSensitive))
		}
	}
	if len(partCols) > 0 {
		fmt.Fprintf(&sb, "\n  PARTITION BY (%s)", strings.Join(partCols, ", "))
	}

	location := strings.TrimSpace(cfg.Location)
	if location == "" {
		location = "@<stage>/<path>"
	}
	fmt.Fprintf(&sb, "\n  LOCATION = %s", location)

	if roc := strings.TrimSpace(cfg.RefreshOnCreate); roc != "" {
		fmt.Fprintf(&sb, "\n  REFRESH_ON_CREATE = %s", strings.ToUpper(roc))
	}
	if ar := strings.TrimSpace(cfg.AutoRefresh); ar != "" {
		fmt.Fprintf(&sb, "\n  AUTO_REFRESH = %s", strings.ToUpper(ar))
	}
	if p := strings.TrimSpace(cfg.Pattern); p != "" {
		fmt.Fprintf(&sb, "\n  PATTERN = '%s'", snowflake.EscapeStringLit(p))
	}

	fmt.Fprintf(&sb, "\n  %s", fileFormatClause(cfg))

	if topic := strings.TrimSpace(cfg.AwsSnsTopic); topic != "" {
		fmt.Fprintf(&sb, "\n  AWS_SNS_TOPIC = '%s'", snowflake.EscapeStringLit(topic))
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
