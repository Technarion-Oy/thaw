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
	"thaw/internal/apperrors"
	"thaw/internal/migration"
)

// ScanMigrationSource walks dir and returns one MigrationObject per DDL statement.
func (a *App) ScanMigrationSource(dir string) ([]migration.MigrationObject, error) {
	return a.migrationSvc.ScanSource(dir)
}

// AnalyzeMigration diffs local objects against the live Snowflake database.
func (a *App) AnalyzeMigration(objects []migration.MigrationObject, database string) ([]migration.MigrationDiffItem, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.migrationSvc.Analyze(a.client, objects, database)
}

// CreateMigrationSnapshot optionally creates a backup set and/or a zero-copy
// clone of the target database as a safety net before deployment.
func (a *App) CreateMigrationSnapshot(database, backupSetDB, backupSetSchema, backupSetName string, doBackup bool, cloneDB string, doClone bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return a.migrationSvc.CreateSnapshot(a.client, database, backupSetDB, backupSetSchema, backupSetName, doBackup, cloneDB, doClone)
}

// ExecuteMigration deploys the selected objects to Snowflake.
func (a *App) ExecuteMigration(selected []migration.MigrationObject, database string, maxPasses int, strategy migration.TableMigrationStrategy) ([]migration.MigrationExecEvent, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.migrationSvc.Execute(a.client, selected, database, maxPasses, strategy)
}

// CancelMigration cancels an in-flight schema migration.
func (a *App) CancelMigration() error {
	return a.migrationSvc.Cancel()
}

// GenerateMigrationScript returns a human-readable migration script.
func (a *App) GenerateMigrationScript(items []migration.MigrationDiffItem, database string, strategy migration.TableMigrationStrategy) string {
	return migration.GenerateScript(items, database, strategy)
}
