// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"context"
	"fmt"
	"strings"
	"time"

	"thaw/internal/snowflake"
)

// TableSummary represents detailed information about a table in a database.
type TableSummary struct {
	Name          string `json:"name"`
	Schema        string `json:"schema"`
	Kind          string `json:"kind"`  // BASE TABLE, VIEW, DYNAMIC TABLE, etc.
	Rows          *int64 `json:"rows"`  // nil when Snowflake reports no count
	Bytes         *int64 `json:"bytes"` // nil when Snowflake reports no count
	Owner         string `json:"owner"`
	RetentionTime *int64 `json:"retentionTime"` // nil when Snowflake reports none
	// Use string for Wails binding compatibility with time.Time
	Created     string `json:"created"`
	LastAltered string `json:"lastAltered"`
	Comment     string `json:"comment"`
}

// TableSettings holds the modifiable table-level properties that can be
// changed via ALTER TABLE ... SET without re-creating the table.
type TableSettings struct {
	ClusterBy             string `json:"clusterBy"`
	EnableSchemaEvolution bool   `json:"enableSchemaEvolution"`
	DataRetentionDays     int    `json:"dataRetentionDays"`
	MaxDataExtensionDays  int    `json:"maxDataExtensionDays"`
	ChangeTracking        bool   `json:"changeTracking"`
	DefaultDDLCollation   string `json:"defaultDDLCollation"`
	Comment               string `json:"comment"`
}

// BuildDatabaseTableSummaryQuery returns the INFORMATION_SCHEMA.TABLES query
// that lists every table and view in the given database. TABLE_TYPE is one of
// BASE TABLE, TEMPORARY TABLE, EXTERNAL TABLE, EVENT TABLE, VIEW or
// MATERIALIZED VIEW — all of them are listed, and the report filters client
// side. Transient, dynamic, iceberg and hybrid tables all report TABLE_TYPE =
// BASE TABLE and are told apart only by the IS_* flags, so those are selected
// too. ROW_COUNT / BYTES / RETENTION_TIME are NULL whenever Snowflake keeps no
// statistics (views, and a freshly created or truncated table).
func BuildDatabaseTableSummaryQuery(database string) string {
	return fmt.Sprintf(`
		SELECT
			TABLE_NAME,
			TABLE_SCHEMA,
			TABLE_TYPE,
			ROW_COUNT,
			BYTES,
			TABLE_OWNER,
			RETENTION_TIME,
			CREATED,
			LAST_ALTERED,
			COMMENT,
			IS_TRANSIENT,
			IS_DYNAMIC,
			IS_ICEBERG,
			IS_HYBRID
		FROM %s.INFORMATION_SCHEMA.TABLES
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`, snowflake.QuoteIdent(database))
}

// ParseDatabaseTableSummary projects a table-summary query result into
// TableSummary rows.
func ParseDatabaseTableSummary(res *snowflake.QueryResult) []TableSummary {
	if res == nil {
		return nil
	}
	var tables []TableSummary
	for _, row := range res.Rows {
		// The query selects a fixed 14 columns; anything shorter is malformed.
		if len(row) < 14 {
			continue
		}
		t := TableSummary{
			Name:    snowflake.CellString(row[0]),
			Schema:  snowflake.CellString(row[1]),
			Kind:    snowflake.CellString(row[2]),
			Owner:   snowflake.CellString(row[5]),
			Comment: snowflake.CellString(row[9]),
		}

		// Dynamic, iceberg, hybrid and transient tables all report TABLE_TYPE =
		// BASE TABLE and are told apart only by their own flag, so fold the flag
		// into Kind the way SHOW TABLES reports it. Only a BASE TABLE is
		// reclassified: any other TABLE_TYPE keeps its own kind, so a transient
		// temporary table is not mislabelled. The first flag set wins — a dynamic
		// table reads as such even when it is also transient.
		if strings.EqualFold(t.Kind, "BASE TABLE") {
			for _, f := range []struct {
				col  int
				kind string
			}{
				{12, "ICEBERG TABLE"},
				{13, "HYBRID TABLE"},
				{11, "DYNAMIC TABLE"},
				{10, "TRANSIENT"},
			} {
				if snowflake.CellBool(row[f.col]) {
					t.Kind = f.kind
					break
				}
			}
		}

		// ROW_COUNT / BYTES / RETENTION_TIME are NULL whenever Snowflake keeps no
		// statistics — for views, and for a freshly created or truncated table — so
		// they stay nil rather than reading back as a real 0.
		t.Rows = snowflake.CellInt64Ptr(row[3])
		t.Bytes = snowflake.CellInt64Ptr(row[4])
		t.RetentionTime = snowflake.CellInt64Ptr(row[6])

		// Parsing times and converting to string for Wails compatibility
		if row[7] != nil {
			if ts, ok := row[7].(time.Time); ok {
				t.Created = ts.Format(time.RFC3339)
			}
		}
		if row[8] != nil {
			if ts, ok := row[8].(time.Time); ok {
				t.LastAltered = ts.Format(time.RFC3339)
			}
		}

		tables = append(tables, t)
	}
	return tables
}

// GetDatabaseTableSummary returns detailed information about every table and
// view in the specified database.
func GetDatabaseTableSummary(ctx context.Context, client *snowflake.Client, database string) ([]TableSummary, error) {
	res, err := client.QuerySingle(ctx, BuildDatabaseTableSummaryQuery(database))
	if err != nil {
		return nil, err
	}
	return ParseDatabaseTableSummary(res), nil
}

// GetTableSettings reads the current values of all modifiable table properties
// by running SHOW TABLES and (for collation) SHOW PARAMETERS.
func GetTableSettings(ctx context.Context, client *snowflake.Client, database, schema, tbl string) (TableSettings, error) {
	res, err := client.Execute(ctx, fmt.Sprintf(
		"SHOW TABLES LIKE '%s' IN SCHEMA %s.%s",
		snowflake.EscapeStringLit(tbl), snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema),
	))
	if err != nil {
		return TableSettings{}, err
	}

	// Build column-name → index map (case-insensitive).
	colIdx := make(map[string]int, len(res.Columns))
	for i, c := range res.Columns {
		colIdx[strings.ToLower(c)] = i
	}

	// Find the row whose name matches exactly (LIKE can return partial matches).
	var row []interface{}
	for _, r := range res.Rows {
		idx, ok := colIdx["name"]
		if ok && idx < len(r) && r[idx] != nil && strings.EqualFold(fmt.Sprint(r[idx]), tbl) {
			row = r
			break
		}
	}
	if row == nil {
		return TableSettings{}, fmt.Errorf("table %q not found", tbl)
	}

	get := func(name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(row) || row[idx] == nil {
			return ""
		}
		return fmt.Sprint(row[idx])
	}
	parseBool := func(s string) bool {
		s = strings.ToLower(strings.TrimSpace(s))
		return s == "y" || s == "true" || s == "on" || s == "1"
	}
	parseInt := func(s string) int {
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}

	settings := TableSettings{
		ClusterBy:             get("cluster_by"),
		EnableSchemaEvolution: parseBool(get("enable_schema_evolution")),
		DataRetentionDays:     parseInt(get("retention_time")),
		MaxDataExtensionDays:  parseInt(get("max_data_extension_time_in_days")),
		ChangeTracking:        parseBool(get("change_tracking")),
		Comment:               get("comment"),
		DefaultDDLCollation:   get("default_ddl_collation"),
	}

	// Fallback: read DEFAULT_DDL_COLLATION from SHOW PARAMETERS if not in SHOW TABLES.
	if settings.DefaultDDLCollation == "" {
		pres, perr := client.Execute(ctx, fmt.Sprintf(
			"SHOW PARAMETERS LIKE 'DEFAULT_DDL_COLLATION' IN TABLE %s.%s.%s",
			snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(tbl),
		))
		if perr == nil && len(pres.Rows) > 0 {
			pidx := make(map[string]int, len(pres.Columns))
			for i, c := range pres.Columns {
				pidx[strings.ToLower(c)] = i
			}
			if vi, ok := pidx["value"]; ok && vi < len(pres.Rows[0]) && pres.Rows[0][vi] != nil {
				settings.DefaultDDLCollation = fmt.Sprint(pres.Rows[0][vi])
			}
		}
	}

	return settings, nil
}

// BuildAlterTablePropertySQL builds an ALTER TABLE statement for a single
// property change. property must be one of: clusterBy, enableSchemaEvolution,
// dataRetentionDays, maxDataExtensionDays, changeTracking, defaultDDLCollation,
// comment.
func BuildAlterTablePropertySQL(database, schema, tbl, property, value string) (string, error) {
	t := snowflake.Qualify(database, schema, tbl)

	switch property {
	case "clusterBy":
		if strings.TrimSpace(value) == "" {
			return fmt.Sprintf(`ALTER TABLE %s DROP CLUSTERING KEY`, t), nil
		}
		return fmt.Sprintf(`ALTER TABLE %s CLUSTER BY (%s)`, t, value), nil
	case "enableSchemaEvolution":
		return fmt.Sprintf(`ALTER TABLE %s SET ENABLE_SCHEMA_EVOLUTION = %s`, t, strings.ToUpper(value)), nil
	case "dataRetentionDays":
		return fmt.Sprintf(`ALTER TABLE %s SET DATA_RETENTION_TIME_IN_DAYS = %s`, t, value), nil
	case "maxDataExtensionDays":
		return fmt.Sprintf(`ALTER TABLE %s SET MAX_DATA_EXTENSION_TIME_IN_DAYS = %s`, t, value), nil
	case "changeTracking":
		return fmt.Sprintf(`ALTER TABLE %s SET CHANGE_TRACKING = %s`, t, strings.ToUpper(value)), nil
	case "defaultDDLCollation":
		return fmt.Sprintf(`ALTER TABLE %s SET DEFAULT_DDL_COLLATION = '%s'`, t, snowflake.EscapeStringLit(value)), nil
	case "comment":
		return fmt.Sprintf(`ALTER TABLE %s SET COMMENT = '%s'`, t, snowflake.EscapeStringLit(value)), nil
	default:
		return "", fmt.Errorf("unknown property: %s", property)
	}
}

// AlterProperty applies a single ALTER TABLE SET change.
func AlterProperty(ctx context.Context, client *snowflake.Client, database, schema, tbl, property, value string) error {
	query, err := BuildAlterTablePropertySQL(database, schema, tbl, property, value)
	if err != nil {
		return err
	}
	_, err = client.Execute(ctx, query)
	return err
}
