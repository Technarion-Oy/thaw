# internal/backup

> SQL builders, SHOW result parsers, and high-level orchestration for Snowflake backup-set and backup-policy management.

## Responsibility

Owns all logic for interacting with Snowflake's native backup objects: building DDL/DML SQL strings, parsing `SHOW BACKUP SETS / SHOW BACKUP POLICIES / SHOW BACKUPS` result sets into typed rows, and providing high-level functions that combine a SQL execution with the corresponding parse step. The `*App` in `internal/app` calls these functions via thin delegators; no backup SQL is constructed or parsed in `internal/app`.

## Key files

| File | Purpose |
|---|---|
| `backup.go` | All types, SQL builders, parsers, and high-level functions |
| `backup_test.go` | Unit tests for builders and parsers |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

### Row types (returned over IPC to the frontend)

| Type | Source command |
|---|---|
| `BackupSetRow` | `SHOW BACKUP SETS` |
| `BackupPolicyRow` | `SHOW BACKUP POLICIES` |
| `BackupRow` | `SHOW BACKUPS IN BACKUP SET` |

### SQL builders

| Function | Emits |
|---|---|
| `BuildListBackupSetsSql(nameFilter)` | `SHOW BACKUP SETS [LIKE ...] IN ACCOUNT` (filter escaped via `EscapeLikePattern`) |
| `BuildCreateBackupSetSql(...)` | `CREATE [OR REPLACE] BACKUP SET ... FOR <type> <fqn>` |
| `BuildCreateBackupPolicySql(...)` | `CREATE [OR REPLACE] BACKUP POLICY ...` (SCHEDULE/COMMENT via `QuoteTextLit`) |
| `BuildApplyBackupPolicySql(setFQN, policy)` | `ALTER BACKUP SET <setFQN> APPLY BACKUP POLICY "<policy>"` (policy always quoted) |
| `BuildRestoreFromBackupSql(...)` | `CREATE <type> <target> FROM BACKUP SET ... IDENTIFIER '...'` |
| `BuildDeleteOldestBackupSql(...)` | `ALTER BACKUP SET ... DELETE BACKUP IDENTIFIER '...'` |

Free-text values (COMMENT, SCHEDULE, the user-editable backup `IDENTIFIER`, the `LIKE` name filter) are escaped as text literals (`QuoteTextLit` / `EscapeLikePattern`), which double backslashes as well as single-quotes — a value ending in `\` or containing `\'` can no longer break out of its literal.

### Parsers

| Function | Input | Output |
|---|---|---|
| `ParseBackupSets(res, scopeType, db, schema, table)` | `*snowflake.QueryResult` | `[]BackupSetRow` (post-filtered by object scope) |
| `ParseBackupPolicies(res)` | `*snowflake.QueryResult` | `[]BackupPolicyRow` |
| `ParseBackups(res)` | `*snowflake.QueryResult` | `[]BackupRow` |
| `FindOldestEligibleBackup(res)` | `*snowflake.QueryResult` | `(id string, ok bool)` |

### High-level functions

`ListBackupSets`, `CreateBackupSet`, `DropBackupSet`, `AlterBackupSet`, `ListBackupPolicies`, `CreateBackupPolicy`, `DropBackupPolicy`, `AlterBackupPolicy`, `ListBackups`, `AddBackup`, `RestoreFromBackup`, `DeleteOldestBackup` — each takes `(ctx, *snowflake.Client, ...)` and handles execution + parsing internally.

## Patterns & integration

`*App` in `internal/app/backup.go` is a thin delegator: nil-check → call into this package → return. The frontend receives the typed row slices over Wails IPC.

`CreateBackupSet` takes an optional `backupPolicy` argument: when non-blank, the policy is applied to the new set in a follow-up `ALTER` **within the same call**, reusing the exact name reference `CREATE` stored (`createBackupSetNameFQN`). Doing this backend-side is deliberate — a separate frontend `AlterBackupSet` call would re-quote the name via `bsFQN` (which always double-quotes) and fail to match a bare, case-insensitively created set name.

`ParseBackupSets` post-filters the full account SHOW result server-side so only backup sets that cover the right-clicked database, schema, or table are returned. The `scopeType` parameter (`"DATABASE"`, `"SCHEMA"`, `"TABLE"`) drives the filter logic.

## Gotchas

`RestoreFromBackup` uses `client.QuerySingle` (plain `db.QueryContext`) rather than `client.Execute` because Snowflake's multi-statement mode breaks the `FROM BACKUP SET ... IDENTIFIER` syntax.

`FindOldestEligibleBackup` skips any backup marked `is_under_legal_hold`. Snowflake only allows deleting the single oldest eligible backup at a time — attempting to delete a non-oldest or legally-held backup will error at the server. It parses each `created_on` to a `time.Time` (`parseBackupTime`, several layouts) and compares instants, so rows whose timestamps carry different UTC offsets (e.g. across a DST transition) still order chronologically; it falls back to a lexicographic compare only when a value fails to parse.

`ParseBackupSets` records the backup set's own `database_name`/`schema_name` exactly as SHOW reports them and leaves them empty when SHOW returns them empty — it does **not** fall back to the backed-up object's database, which is unrelated to where the set lives. An empty location makes follow-up calls emit a bare quoted name resolved against the session context (better than a fabricated two-part FQN that Snowflake would misresolve as `schema.object`).

Column names in `SHOW BACKUPS` vary across Snowflake versions (`backup_id` / `snapshot_id` / `id` / `identifier` / `uuid`); `ColIdx` is called with all known aliases to handle version differences.
