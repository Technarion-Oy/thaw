# internal/storagelifecyclepolicy

> SQL builder for Snowflake STORAGE LIFECYCLE POLICY objects.

## Responsibility

Builds the `CREATE STORAGE LIFECYCLE POLICY` DDL from a structured config.
Storage lifecycle policies automate **data retention, archival, and deletion**:
rows of a table the policy is attached to become eligible for the lifecycle
action (archive to a `COOL` / `COLD` tier, then expire) once the policy body
evaluates to `TRUE` for them. Like masking and row access policies, a storage
lifecycle policy has a **real signature** — `AS (<arg> <type>, …) RETURNS BOOLEAN
-> <body>` — mapping each argument to a column of the attached table (Snowflake
requires at least one argument); unlike them it also carries the archival options
`ARCHIVE_TIER` and `ARCHIVE_FOR_DAYS`. The
lifecycle / edit commands (`RENAME TO`, `SET BODY`, `SET ARCHIVE_TIER`,
`SET` / `UNSET ARCHIVE_FOR_DAYS`, `SET` / `UNSET COMMENT`, `SET` / `UNSET TAG`)
are simple enough that they are issued as free-form
`ALTER STORAGE LIFECYCLE POLICY <fqn> <clause>` statements directly from
`internal/app/storagelifecyclepolicy.go` (`App.AlterStorageLifecyclePolicy`)
without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `StorageLifecycleArg`, `StorageLifecyclePolicyConfig`, `BuildCreateStorageLifecyclePolicySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `StorageLifecycleArg` | One signature entry: `Name` + SQL `Type` (the column the body evaluates) |
| `StorageLifecyclePolicyConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Args` (signature), `Body` (the boolean expression), `ArchiveTier` (`""` / `COOL` / `COLD`), `ArchiveForDays`, `Comment` |
| `BuildCreateStorageLifecyclePolicySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] STORAGE LIFECYCLE POLICY [IF NOT EXISTS] <fqn> AS (<args>) RETURNS BOOLEAN -> <body> [ARCHIVE_TIER=…] [ARCHIVE_FOR_DAYS=…] [COMMENT='…'];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `storage_lifecycle_policy_name`, an
  empty signature emits `(val TIMESTAMP_NTZ)` (Snowflake requires at least one
  argument), and an empty body emits `TRUE`.
- `ARCHIVE_TIER` is emitted as an unquoted keyword (upper-cased); when unset, rows
  expire without being archived. `ARCHIVE_FOR_DAYS` is omitted when `<= 0` (it is
  only meaningful alongside a tier, and the documented minimums are 90 days for
  `COOL` and 180 for `COLD`).
- `App.BuildCreateStorageLifecyclePolicySql` (in `internal/app/builders.go`) is
  the thin IPC delegator; `App.AlterStorageLifecyclePolicy` (in
  `internal/app/storagelifecyclepolicy.go`) runs the edit clauses,
  `App.GetStorageLifecyclePolicyTags` lists the current tags, and
  `App.GetStorageLifecyclePolicyReferences` lists the tables a policy is applied
  to.
- Discovery: `Client.ListExtendedObjects` runs `SHOW STORAGE LIFECYCLE POLICIES
  IN SCHEMA` with the fixed kind `"STORAGE LIFECYCLE POLICY"`. Storage lifecycle
  policies are not surfaced by `SHOW OBJECTS`, so — unlike dynamic / external
  tables and materialized views — no dedupe pass is needed.
- DDL export: storage lifecycle policies are retrieved via the `GET_DDL` object
  type `POLICY` (which covers all policy kinds including storage lifecycle), so
  `buildGetDDLQuery` maps the SHOW kind `"STORAGE LIFECYCLE POLICY"` to `POLICY`.
- Properties panel: `internal/objects` runs `SHOW STORAGE LIFECYCLE POLICIES LIKE
  …` for the `STORAGE LIFECYCLE POLICY` kind and enriches it via `DESCRIBE STORAGE
  LIFECYCLE POLICY` (signature, return type, body, and the archive settings — none
  of which SHOW reports); the panel also surfaces the policy references.

## Gotchas

- **The body returns BOOLEAN** — a row is eligible for the lifecycle action when
  the body evaluates to `TRUE`. Each signature argument is mapped to a column of
  the table the policy is attached to via `ALTER TABLE … ADD STORAGE LIFECYCLE
  POLICY … ON (col, …)`.
- **`ARCHIVE_TIER` and `ARCHIVE_FOR_DAYS` must be set together** — Snowflake
  validates the *combined* state and rejects a half-set pair (`ARCHIVE_TIER` with
  no `ARCHIVE_FOR_DAYS`, or vice-versa) with "invalid property combination". The
  builder therefore emits either both or neither, and the properties modal edits
  them as one unit, issuing a single `ALTER … SET ARCHIVE_TIER = … ARCHIVE_FOR_DAYS
  = …` (or `UNSET ARCHIVE_FOR_DAYS` to disable).
- **`ARCHIVE_TIER` is immutable once set** — Snowflake rejects changing the tier
  of a policy that already has one (the properties modal still offers it, but the
  ALTER will error in that case). There is no `UNSET ARCHIVE_TIER` in the grammar.
- **References require ACCOUNT_USAGE** — `App.GetStorageLifecyclePolicyReferences`
  queries `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES` (filtered to `POLICY_KIND =
  'STORAGE_LIFECYCLE_POLICY'`), which needs governance privileges and has
  propagation latency (newly-applied policies may take time to appear).
