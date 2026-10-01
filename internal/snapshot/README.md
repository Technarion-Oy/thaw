# internal/snapshot

> SQL builder for Snowpark Container Services volume SNAPSHOT objects.

## Responsibility

Builds the `CREATE SNAPSHOT` DDL from a structured config. A snapshot is a
point-in-time copy of one service instance's **block-storage** volume, stored as
a schema-level object.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `SnapshotConfig`, `BuildCreateSnapshotSql` |
| `sql_test.go` | Unit tests for the builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `SnapshotConfig` | name, case sensitivity, `OrReplace` / `IfNotExists`, source service (`ServiceDatabase` / `ServiceSchema` / `ServiceName` — it may live in another schema), `Volume`, `Instance`, `Comment`, `Tags` |
| `BuildCreateSnapshotSql(db, schema, cfg)` | `CREATE [OR REPLACE] SNAPSHOT [IF NOT EXISTS] <fqn> FROM SERVICE <svc> VOLUME "<vol>" INSTANCE <id> [COMMENT = '…'] [TAG (…)];` |

## Patterns & integration

- Blank name / service / volume / instance render as placeholders so the live
  preview stays a completable template. `OR REPLACE` wins over `IF NOT EXISTS`.
- The volume name is **always** double-quoted — volume names come from the
  service spec and are case-sensitive. The instance id must be a non-negative
  integer; anything else is an error, never interpolated.
- `App.BuildCreateSnapshotSql` (`internal/app/builders.go`) is the IPC
  delegator. `internal/app/snapshot.go` runs the rest directly:
  `DescribeSnapshot`, `AlterSnapshot` (`SET COMMENT`), `UndropSnapshot`,
  `ListSnapshots` (`SHOW SNAPSHOTS IN ACCOUNT`), `DropSnapshot`.
- Discovery and Properties lookups come from the `objectkind` registry entry
  (`SNAPSHOT` / `SNAPSHOTS`).

## Gotchas

- **`ALTER SNAPSHOT` is `SET COMMENT` only** — no `RENAME`, no `UNSET`, no tags.
  Clearing the comment is `SET COMMENT = ''`. Snapshots are excluded from the
  sidebar Rename action.
- **No `GET_DDL`** object type for snapshots (`GetDDLType` empty) — no View
  Definition / comparison. Unverified on a live account.
- **No `SHOW SNAPSHOTS HISTORY`**, so dropped snapshots can't be listed in Show
  Dropped Objects; restore is a by-name `UNDROP SNAPSHOT` prompt.
- Only `block` volumes can be snapshotted; the frontend picker filters
  `SHOW SERVICE VOLUMES` to them (`components/snapshot/blockVolumes.ts`).
