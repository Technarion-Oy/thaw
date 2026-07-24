// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"thaw/internal/warehouse"
)

// GetRoleDDL returns the DDL definition of a Snowflake role.
func (a *App) GetRoleDDL(name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetRoleDDL(a.fctx(FeatureWarehouses), name)
}

// GetWarehouseDDL returns the DDL definition of a Snowflake warehouse.
func (a *App) GetWarehouseDDL(name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetWarehouseDDL(a.fctx(FeatureWarehouses), name)
}

// AlterWarehouseProperty applies a single SET property to a warehouse.
// property must be one of: size, warehouseType, autoSuspend, autoResume, comment,
// maxClusterCount, minClusterCount, scalingPolicy, resourceMonitor,
// enableQueryAcceleration, queryAccelerationMaxScaleFactor,
// maxConcurrencyLevel, statementQueuedTimeout, statementTimeout.
func (a *App) AlterWarehouseProperty(name, property, value string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.AlterProperty(a.fctx(FeatureWarehouses), client, name, property, value)
}

// AlterWarehouse runs an `ALTER WAREHOUSE <name> <clause>` statement. clause is
// everything after the warehouse name, e.g. "SET TAG "DB"."S"."COST_CENTER" = 'x'"
// or "UNSET TAG "DB"."S"."COST_CENTER"". A warehouse is account-level, so the name
// is a bare double-quoted identifier (no database / schema qualification). It backs
// the Tags section of the Warehouse Properties modal; the caller is responsible for
// correct SQL quoting inside the clause.
func (a *App) AlterWarehouse(name, clause string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := client.Execute(a.fctx(FeatureWarehouses),
		fmt.Sprintf("ALTER WAREHOUSE %s %s", snowflake.QuoteIdent(name), clause))
	return err
}

// AlterWarehouseSuspend suspends the named warehouse.
func (a *App) AlterWarehouseSuspend(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Suspend(a.fctx(FeatureWarehouses), client, name)
}

// AlterWarehouseResume resumes the named warehouse if it is suspended.
func (a *App) AlterWarehouseResume(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Resume(a.fctx(FeatureWarehouses), client, name)
}

// AlterWarehouseAbortAllQueries issues ABORT ALL QUERIES on the named warehouse.
func (a *App) AlterWarehouseAbortAllQueries(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.AbortAllQueries(a.fctx(FeatureWarehouses), client, name)
}

// AlterWarehouseRename renames a warehouse and returns the new name.
func (a *App) AlterWarehouseRename(name, newName string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return warehouse.Rename(a.fctx(FeatureWarehouses), client, name, newName)
}

// GetWarehouseParameters returns per-warehouse parameter overrides (MAX_CONCURRENCY_LEVEL,
// STATEMENT_QUEUED_TIMEOUT_IN_SECONDS, STATEMENT_TIMEOUT_IN_SECONDS) sourced from
// SHOW PARAMETERS IN WAREHOUSE.
func (a *App) GetWarehouseParameters(name string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return warehouse.GetParameters(a.fctx(FeatureWarehouses), client, name)
}

// GetWarehouseMeteringHistory returns hourly credit usage records from
// SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY. Rows are ordered by
// START_TIME ascending. warehouse, startDate, and endDate are all optional
// filters; dates must be RFC3339 strings when provided.
func (a *App) GetWarehouseMeteringHistory(wh, startDate, endDate string) ([]warehouse.WarehouseMeteringRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return warehouse.GetMeteringHistory(a.fctx(FeatureWarehouses), client, wh, startDate, endDate)
}
