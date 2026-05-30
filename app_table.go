// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"fmt"
	"strconv"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"time"
)

// TableSummary represents detailed information about a table in a database.
type TableSummary struct {
	Name          string `json:"name"`
	Schema        string `json:"schema"`
	Kind          string `json:"kind"` // BASE TABLE, VIEW, etc.
	Rows          int64  `json:"rows"`
	Bytes         int64  `json:"bytes"`
	Owner         string `json:"owner"`
	RetentionTime int    `json:"retentionTime"`
	// Use string for Wails binding compatibility with time.Time
	Created     string `json:"created"`
	LastAltered string `json:"lastAltered"`
	Comment     string `json:"comment"`
}

// GetDatabaseTableSummary returns detailed information about all tables in the
// specified database.
func (a *App) GetDatabaseTableSummary(dbName string) ([]TableSummary, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}

	qdb := snowflake.QuoteIdent(dbName)
	query := fmt.Sprintf(`
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
			COMMENT 
		FROM %s.INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE IN ('BASE TABLE', 'TRANSIENT', 'TEMPORARY')
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`, qdb)

	res, err := a.client.QuerySingle(a.ctx, query)
	if err != nil {
		return nil, err
	}

	var tables []TableSummary
	for _, row := range res.Rows {
		if len(row) < 10 {
			continue
		}
		t := TableSummary{
			Name:   fmt.Sprintf("%v", row[0]),
			Schema: fmt.Sprintf("%v", row[1]),
			Kind:   fmt.Sprintf("%v", row[2]),
			Owner:  fmt.Sprintf("%v", row[5]),
		}

		if row[9] != nil {
			t.Comment = fmt.Sprintf("%v", row[9])
		}

		// Parsing numeric values
		t.Rows, _ = strconv.ParseInt(fmt.Sprintf("%v", row[3]), 10, 64)
		t.Bytes, _ = strconv.ParseInt(fmt.Sprintf("%v", row[4]), 10, 64)
		retTime, _ := strconv.Atoi(fmt.Sprintf("%v", row[6]))
		t.RetentionTime = retTime

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

	return tables, nil
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

// GetTableSettings reads the current values of all modifiable table properties
// by running SHOW TABLES and (for collation) SHOW PARAMETERS.
func (a *App) GetTableSettings(database, schema, table string) (TableSettings, error) {
	if a.client == nil {
		return TableSettings{}, apperrors.ErrNotConnected
	}
	res, err := a.client.Execute(a.ctx, fmt.Sprintf(
		"SHOW TABLES LIKE '%s' IN SCHEMA %s.%s",
		snowflake.EscapeStringLit(table), snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema),
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
		if ok && idx < len(r) && r[idx] != nil && strings.EqualFold(fmt.Sprint(r[idx]), table) {
			row = r
			break
		}
	}
	if row == nil {
		return TableSettings{}, fmt.Errorf("table %q not found", table)
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
		pres, perr := a.client.Execute(a.ctx, fmt.Sprintf(
			"SHOW PARAMETERS LIKE 'DEFAULT_DDL_COLLATION' IN TABLE %s.%s.%s",
			snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(table),
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

// AlterTableProperty applies a single ALTER TABLE SET change.
// property must be one of: clusterBy, enableSchemaEvolution, dataRetentionDays,
// maxDataExtensionDays, changeTracking, defaultDDLCollation, comment.
func (a *App) AlterTableProperty(database, schema, table, property, value string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	tbl := snowflake.QuoteIdent(database) + "." + snowflake.QuoteIdent(schema) + "." + snowflake.QuoteIdent(table)

	var query string
	switch property {
	case "clusterBy":
		if strings.TrimSpace(value) == "" {
			query = fmt.Sprintf(`ALTER TABLE %s DROP CLUSTERING KEY`, tbl)
		} else {
			query = fmt.Sprintf(`ALTER TABLE %s CLUSTER BY (%s)`, tbl, value)
		}
	case "enableSchemaEvolution":
		query = fmt.Sprintf(`ALTER TABLE %s SET ENABLE_SCHEMA_EVOLUTION = %s`, tbl, strings.ToUpper(value))
	case "dataRetentionDays":
		query = fmt.Sprintf(`ALTER TABLE %s SET DATA_RETENTION_TIME_IN_DAYS = %s`, tbl, value)
	case "maxDataExtensionDays":
		query = fmt.Sprintf(`ALTER TABLE %s SET MAX_DATA_EXTENSION_TIME_IN_DAYS = %s`, tbl, value)
	case "changeTracking":
		query = fmt.Sprintf(`ALTER TABLE %s SET CHANGE_TRACKING = %s`, tbl, strings.ToUpper(value))
	case "defaultDDLCollation":
		query = fmt.Sprintf(`ALTER TABLE %s SET DEFAULT_DDL_COLLATION = '%s'`, tbl, snowflake.EscapeStringLit(value))
	case "comment":
		query = fmt.Sprintf(`ALTER TABLE %s SET COMMENT = '%s'`, tbl, snowflake.EscapeStringLit(value))
	default:
		return fmt.Errorf("unknown property: %s", property)
	}

	_, err := a.client.Execute(a.ctx, query)
	return err
}

// ExportTableData exports a Snowflake table to the local filesystem using a
// temporary internal stage. The stage is dropped automatically after the
// download completes or on error.
func (a *App) ExportTableData(params snowflake.ExportTableParams) (snowflake.ExportTableResult, error) {
	if a.client == nil {
		return snowflake.ExportTableResult{}, apperrors.ErrNotConnected
	}
	return a.client.ExportTableData(a.ctx, params)
}

// ImportTableData imports a local file into a Snowflake table using a temporary
// internal stage. The stage is dropped automatically after the upload completes
// or on error.
func (a *App) ImportTableData(params snowflake.ImportTableParams) (snowflake.ImportTableResult, error) {
	if a.client == nil {
		return snowflake.ImportTableResult{}, apperrors.ErrNotConnected
	}
	return a.client.ImportTableData(a.ctx, params)
}

// ExecDDL executes an arbitrary DDL/DML statement and discards the result set.
// It is intended for one-shot statements (CREATE, ALTER, DROP, etc.) where the
// caller needs to know whether the statement succeeded without routing the SQL
// through the editor's query pipeline.
func (a *App) ExecDDL(sql string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, sql)
	return err
}
