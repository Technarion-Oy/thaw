// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/apperrors"
	"thaw/internal/computepool"
	"thaw/internal/snowflake"
)

// Compute pool IPC for the Container Services dialog's Compute pools tab.
// Listing / describe / nodes / families return the raw QueryResult so the tab
// renders whatever columns the account's Snowflake version reports.

func (a *App) execPoolSQL(sql string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.Execute(a.fctx(FeatureContainerServices), sql)
}

// ListComputePoolsDetailed runs SHOW COMPUTE POOLS (every column).
func (a *App) ListComputePoolsDetailed() (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW COMPUTE POOLS")
}

// DescribeComputePool runs DESCRIBE COMPUTE POOL <name>.
func (a *App) DescribeComputePool(name string) (*snowflake.QueryResult, error) {
	return a.execPoolSQL("DESCRIBE COMPUTE POOL " + snowflake.QuoteIdent(name))
}

// ListComputePoolNodes runs SHOW NODES IN COMPUTE POOL <name>.
func (a *App) ListComputePoolNodes(name string) (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW NODES IN COMPUTE POOL " + snowflake.QuoteIdent(name))
}

// ListComputePoolInstanceFamilies runs SHOW COMPUTE POOL INSTANCE FAMILIES
// (name, vcpu, memory_gib, gpu, gpu_count, gpu_memory_gib, current_node_usage, …).
func (a *App) ListComputePoolInstanceFamilies() (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW COMPUTE POOL INSTANCE FAMILIES")
}

// AlterComputePool runs ALTER COMPUTE POOL <name> <clause> — SUSPEND, RESUME,
// SET TAG / UNSET TAG. The caller quotes anything inside clause.
func (a *App) AlterComputePool(name, clause string) error {
	_, err := a.execPoolSQL("ALTER COMPUTE POOL " + snowflake.QuoteIdent(name) + " " + clause)
	return err
}

// AlterComputePoolProperty sets (or, for a blank value, unsets) one validated
// property; see computepool.BuildAlterComputePoolPropertySql.
func (a *App) AlterComputePoolProperty(name, property, value string) error {
	sql, err := computepool.BuildAlterComputePoolPropertySql(name, property, value)
	if err != nil {
		return err
	}
	_, err = a.execPoolSQL(sql)
	return err
}

// StopAllComputePoolServices runs ALTER COMPUTE POOL <name> STOP ALL
// [OF TYPE …], terminating every service and job on the pool.
func (a *App) StopAllComputePoolServices(name string, workloadTypes []string) error {
	sql, err := computepool.BuildStopAllSql(name, workloadTypes)
	if err != nil {
		return err
	}
	_, err = a.execPoolSQL(sql)
	return err
}

// DropComputePool runs DROP COMPUTE POOL <name>.
func (a *App) DropComputePool(name string) error {
	_, err := a.execPoolSQL("DROP COMPUTE POOL " + snowflake.QuoteIdent(name))
	return err
}
