// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/apperrors"
	"thaw/internal/backup"
)

// ListBackupSets returns backup sets whose backed-up object matches the
// right-clicked item, optionally filtered by the backup set's name.
func (a *App) ListBackupSets(scopeType, db, schema, table, nameFilter string) ([]backup.BackupSetRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return backup.ListBackupSets(a.fctx(FeatureBackup), client, scopeType, db, schema, table, nameFilter)
}

// CreateBackupSet creates a new backup set for a DATABASE, SCHEMA, or TABLE.
func (a *App) CreateBackupSet(name, nameDb, nameSchema, forType, objectFQN, db string, orReplace, ifNotExists, caseSensitive bool) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.CreateBackupSet(a.fctx(FeatureBackup), client, name, nameDb, nameSchema, forType, objectFQN, db, orReplace, ifNotExists, caseSensitive)
}

// DropBackupSet drops the named backup set.
func (a *App) DropBackupSet(name, bsDb, bsSchema string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.DropBackupSet(a.fctx(FeatureBackup), client, name, bsDb, bsSchema)
}

// AlterBackupSet executes ALTER BACKUP SET <fqn> <alteration>.
func (a *App) AlterBackupSet(name, bsDb, bsSchema, alteration string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.AlterBackupSet(a.fctx(FeatureBackup), client, name, bsDb, bsSchema, alteration)
}

// ListBackupPolicies runs SHOW BACKUP POLICIES and returns all visible policies.
func (a *App) ListBackupPolicies() ([]backup.BackupPolicyRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return backup.ListBackupPolicies(a.fctx(FeatureBackup), client)
}

// CreateBackupPolicy creates a new backup policy.
func (a *App) CreateBackupPolicy(name, schedule string, expireAfterDays int64, retentionLock bool, comment, tags string, orReplace, ifNotExists, caseSensitive bool) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.CreateBackupPolicy(a.fctx(FeatureBackup), client, name, schedule, expireAfterDays, retentionLock, comment, tags, orReplace, ifNotExists, caseSensitive)
}

// DropBackupPolicy drops the named backup policy.
func (a *App) DropBackupPolicy(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.DropBackupPolicy(a.fctx(FeatureBackup), client, name)
}

// AlterBackupPolicy executes ALTER BACKUP POLICY <name> <alteration>.
func (a *App) AlterBackupPolicy(name, alteration string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.AlterBackupPolicy(a.fctx(FeatureBackup), client, name, alteration)
}

// ListBackups runs SHOW BACKUPS IN BACKUP SET <fqn> and returns the result.
func (a *App) ListBackups(backupSetName, bsDb, bsSchema string) ([]backup.BackupRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return backup.ListBackups(a.fctx(FeatureBackup), client, backupSetName, bsDb, bsSchema)
}

// AddBackup triggers ALTER BACKUP SET <fqn> ADD BACKUP to create a new snapshot.
func (a *App) AddBackup(backupSetName, bsDb, bsSchema string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.AddBackup(a.fctx(FeatureBackup), client, backupSetName, bsDb, bsSchema)
}

// RestoreFromBackup creates a new object from a specific backup.
func (a *App) RestoreFromBackup(objectType, targetName, backupSetName, bsDb, bsSchema, backupID, db string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.RestoreFromBackup(a.fctx(FeatureBackup), client, objectType, targetName, backupSetName, bsDb, bsSchema, backupID, db)
}

// DeleteOldestBackup finds the oldest backup in the set with no legal hold and
// deletes it.
func (a *App) DeleteOldestBackup(backupSetName, bsDb, bsSchema string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return backup.DeleteOldestBackup(a.fctx(FeatureBackup), client, backupSetName, bsDb, bsSchema)
}
