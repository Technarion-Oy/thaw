# frontend/src/components/backup

> UI for Snowflake backup policies and backup sets.

## Responsibility

Provides two components for managing Snowflake account-level backup governance: a panel that lists and manages backup policies, and a modal for browsing backup sets attached to individual database objects.

## Files

| File | Purpose |
|---|---|
| `BackupPoliciesPanel.tsx` | Full-page panel showing all backup policies (`SHOW BACKUP POLICIES`); supports Create, Drop, and Alter actions; calls `ListBackupPolicies`, `CreateBackupPolicy`, `DropBackupPolicy`, and `AlterBackupPolicy` IPC methods (backed by `internal/backup`). |
| `BackupSetsModal.tsx` | Modal for listing the backup sets of a specific object (database, schema, or table); calls `ListBackupSets` and related IPC methods; displays `backup.BackupRow` records. |

## Patterns & integration

- **IPC**: All calls go through `wailsjs/go/app/App` to `internal/app/backup.go` delegators, which forward to `internal/backup` SQL builders and parsers.
- **Backend types**: components consume `backup.BackupPolicyRow`, `backup.BackupSetRow`, and `backup.BackupRow` from `wailsjs/go/models`.
- **Feature flag**: both components are gated behind the `backupPoliciesAndSets` flag. `AccountPanel` (`../account/`) checks the flag before rendering `BackupPoliciesPanel`.
- **Entry point**: `BackupPoliciesPanel` is rendered inside `AccountPanel` as a nested view; `BackupSetsModal` is opened from sidebar context menus on database and table nodes.

## Gotchas

- Backup policies and sets are a preview Snowflake feature; availability depends on the account tier. IPC errors should be surfaced to the user rather than silently swallowed.
- These components do not use any Zustand stores beyond `useFeatureFlagsStore`; all data is fetched fresh on open.
- `SHOW BACKUP SETS IN ACCOUNT` can return same-named sets in different schemas, so `BackupSetsModal` keys every per-set cache (`backupCache`/`backupErrors`/`addingBackup`) and the table `rowKey` by the composite `setKey(row)` = `(backupSetDb, backupSetSchema, name)`, never by name alone.
- Applying a backup policy on **create** is handled by passing the policy name to `CreateBackupSet` (the backend applies it with the matching name reference) — not a separate `AlterBackupSet` call. Policy names emitted into `APPLY BACKUP POLICY` and free-text values (`SET COMMENT`, `SET SCHEDULE`) go through `quoteIdent` / a backslash-safe `escTextLit`, mirroring the backend.
- Restore treats `EXTERNAL TABLE` sets like `TABLE` (target db/schema/name pickers) via `isTableLike`; the emitted `CREATE` still uses the set's real object kind.
