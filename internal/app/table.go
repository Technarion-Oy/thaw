// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"thaw/internal/table"
)

// GetDatabaseTableSummary returns detailed information about all tables in the
// specified database.
func (a *App) GetDatabaseTableSummary(dbName string) ([]table.TableSummary, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return table.GetDatabaseTableSummary(a.ctx, client, dbName)
}

// GetTableSettings reads the current values of all modifiable table properties
// by running SHOW TABLES and (for collation) SHOW PARAMETERS.
func (a *App) GetTableSettings(database, schema, tbl string) (table.TableSettings, error) {
	client := a.currentClient()
	if client == nil {
		return table.TableSettings{}, apperrors.ErrNotConnected
	}
	return table.GetTableSettings(a.ctx, client, database, schema, tbl)
}

// AlterTableProperty applies a single ALTER TABLE SET change.
// property must be one of: clusterBy, enableSchemaEvolution, dataRetentionDays,
// maxDataExtensionDays, changeTracking, defaultDDLCollation, comment.
func (a *App) AlterTableProperty(database, schema, tbl, property, value string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return table.AlterProperty(a.ctx, client, database, schema, tbl, property, value)
}

// ExportTableData exports a Snowflake table to the local filesystem using a
// temporary internal stage. The stage is dropped automatically after the
// download completes or on error.
func (a *App) ExportTableData(params snowflake.ExportTableParams) (snowflake.ExportTableResult, error) {
	client := a.currentClient()
	if client == nil {
		return snowflake.ExportTableResult{}, apperrors.ErrNotConnected
	}
	return client.ExportTableData(a.ctx, params)
}

// ImportTableData imports a local file into a Snowflake table using a temporary
// internal stage. The stage is dropped automatically after the upload completes
// or on error.
func (a *App) ImportTableData(params snowflake.ImportTableParams) (snowflake.ImportTableResult, error) {
	client := a.currentClient()
	if client == nil {
		return snowflake.ImportTableResult{}, apperrors.ErrNotConnected
	}
	return client.ImportTableData(a.ctx, params)
}

// ExecDDL executes an arbitrary DDL/DML statement and discards the result set.
// It is intended for one-shot statements (CREATE, ALTER, DROP, etc.) where the
// caller needs to know whether the statement succeeded without routing the SQL
// through the editor's query pipeline.
func (a *App) ExecDDL(sql string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := client.Execute(a.ctx, sql)
	return err
}
