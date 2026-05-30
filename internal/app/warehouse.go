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
	"thaw/internal/warehouse"
)

// GetRoleDDL returns the DDL definition of a Snowflake role.
func (a *App) GetRoleDDL(name string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	return a.client.GetRoleDDL(a.ctx, name)
}

// GetWarehouseDDL returns the DDL definition of a Snowflake warehouse.
func (a *App) GetWarehouseDDL(name string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	return a.client.GetWarehouseDDL(a.ctx, name)
}

// AlterWarehouseProperty applies a single SET property to a warehouse.
// property must be one of: size, warehouseType, autoSuspend, autoResume, comment,
// maxClusterCount, minClusterCount, scalingPolicy, resourceMonitor,
// enableQueryAcceleration, queryAccelerationMaxScaleFactor,
// maxConcurrencyLevel, statementQueuedTimeout, statementTimeout.
func (a *App) AlterWarehouseProperty(name, property, value string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.AlterProperty(a.ctx, a.client, name, property, value)
}

// AlterWarehouseSuspend suspends the named warehouse.
func (a *App) AlterWarehouseSuspend(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Suspend(a.ctx, a.client, name)
}

// AlterWarehouseResume resumes the named warehouse if it is suspended.
func (a *App) AlterWarehouseResume(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Resume(a.ctx, a.client, name)
}

// AlterWarehouseAbortAllQueries issues ABORT ALL QUERIES on the named warehouse.
func (a *App) AlterWarehouseAbortAllQueries(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.AbortAllQueries(a.ctx, a.client, name)
}

// AlterWarehouseRename renames a warehouse and returns the new name.
func (a *App) AlterWarehouseRename(name, newName string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Rename(a.ctx, a.client, name, newName)
}

// GetWarehouseParameters returns per-warehouse parameter overrides (MAX_CONCURRENCY_LEVEL,
// STATEMENT_QUEUED_TIMEOUT_IN_SECONDS, STATEMENT_TIMEOUT_IN_SECONDS) sourced from
// SHOW PARAMETERS IN WAREHOUSE.
func (a *App) GetWarehouseParameters(name string) ([]snowflake.PropertyPair, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return warehouse.GetParameters(a.ctx, a.client, name)
}

// CanViewWarehouseMeteringHistory returns true when the current role has SELECT
// access to SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.  It runs a
// zero-row probe query so it is fast and never touches real data.
func (a *App) CanViewWarehouseMeteringHistory() (bool, error) {
	if a.client == nil {
		return false, apperrors.ErrNotConnected
	}
	_, err := a.client.QuerySingle(a.ctx,
		"SELECT 1 FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY LIMIT 0")
	if err != nil {
		return false, nil //nolint:nilerr // permission denied is not a caller error
	}
	return true, nil
}

// GetWarehouseMeteringHistory returns hourly credit usage records from
// SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY. Rows are ordered by
// START_TIME ascending. warehouse, startDate, and endDate are all optional
// filters; dates must be RFC3339 strings when provided.
func (a *App) GetWarehouseMeteringHistory(wh, startDate, endDate string) ([]warehouse.WarehouseMeteringRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return warehouse.GetMeteringHistory(a.ctx, a.client, wh, startDate, endDate)
}
