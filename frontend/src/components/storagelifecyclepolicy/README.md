# components/storagelifecyclepolicy

> Modals for creating and inspecting Snowflake **STORAGE LIFECYCLE POLICY** objects.

Storage lifecycle policies automate data retention, archival, and deletion: rows
of a table the policy is attached to become eligible for the lifecycle action
(archive to a `COOL` / `COLD` tier, then expire) once the policy body evaluates
to `TRUE` for them.

## Components

| File | Purpose |
|---|---|
| `CreateStorageLifecyclePolicyModal.tsx` | Create form — name, OR REPLACE / IF NOT EXISTS, a column-argument **Signature** editor (each arg is a column the body evaluates; at least one is required), a fixed `BOOLEAN` **Returns**, an embedded Monaco editor for the **Body**, an **Archive tier** selector (`COOL` / `COLD` / none) and **Archive for days** (enabled only when a tier is chosen, with the documented per-tier minimum), a comment, and a live SQL preview of the `CREATE STORAGE LIFECYCLE POLICY` statement. |
| `StorageLifecyclePolicyPropertiesModal.tsx` | Properties panel — **Definition** (signature + return type from `DESCRIBE STORAGE LIFECYCLE POLICY`), an editable **Body** (Monaco, applied via `ALTER … SET BODY -> …`), a **Settings** section with a combined **Archiving** control (enabling sets tier + retention days together in one `SET ARCHIVE_TIER = … ARCHIVE_FOR_DAYS = …`, since Snowflake rejects a half-set pair; once a tier is set the dropdown is locked — the tier is immutable — and only the days are editable via `SET ARCHIVE_FOR_DAYS`; `UNSET ARCHIVE_FOR_DAYS` disables it) and an inline-editable **Comment** (`SET` / `UNSET`), a **Tags** section (`SET` / `UNSET TAG` with removable chips, current tags from `INFORMATION_SCHEMA.TAG_REFERENCES`), a generic **Properties** table, and an on-demand **References** section listing the tables the policy is applied to (from `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`). |

## IPC

- `BuildCreateStorageLifecyclePolicySql(db, schema, cfg)` → `internal/storagelifecyclepolicy`
- `AlterStorageLifecyclePolicy(db, schema, name, clause)` → free-form `ALTER STORAGE LIFECYCLE POLICY <fqn> <clause>`
- `GetStorageLifecyclePolicyTags(db, schema, name)` → `INFORMATION_SCHEMA.TAG_REFERENCES`
- `GetStorageLifecyclePolicyReferences(db, schema, name)` → `ACCOUNT_USAGE.POLICY_REFERENCES` (`POLICY_KIND = 'STORAGE_LIFECYCLE_POLICY'`)
- `GetObjectProperties(db, schema, "STORAGE LIFECYCLE POLICY", name)` → `SHOW` + `DESCRIBE STORAGE LIFECYCLE POLICY` enrichment

## Patterns & gotchas

- The config's nested `args` array means the Wails-generated class carries a
  `convertValues` method a plain object literal can't satisfy; the create modal
  keeps plain local state and casts `cfg as any` only at the IPC boundary. The
  `archiveForDays` field is sent as `0` for "unset" (the builder omits
  `ARCHIVE_FOR_DAYS` when `<= 0`). Snowflake requires at least one signature
  argument, so the last arg row can't be removed.
- The body is raw SQL (a boolean expression), interpolated verbatim into `SET
  BODY -> …` — not a quoted string literal.
- **Archive tier is immutable once set** — the properties modal still offers `SET
  ARCHIVE_TIER`, but Snowflake rejects changing the tier of a policy that already
  has one.
- **RENAME** is reached via the sidebar context-menu **Rename…** (the generic
  rename path issues `ALTER STORAGE LIFECYCLE POLICY … RENAME TO`), so it is not
  duplicated here.
- Storage lifecycle policies require Enterprise Edition; the create flow only
  succeeds against an Enterprise-edition account.
