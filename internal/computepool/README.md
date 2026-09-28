# internal/computepool

> SQL builders for Snowpark Container Services COMPUTE POOL objects.

## Responsibility

A compute pool is an account-level set of VM nodes that SPCS services and jobs
run on. This package builds the statements that need validation; the
Compute pools tab of the Container Services dialog
(`frontend/src/components/containerservices/`) drives them through
`internal/app/computepool.go`.

| Function | SQL |
|---|---|
| `BuildCreateComputePoolSql(cfg)` | `CREATE COMPUTE POOL [IF NOT EXISTS] <name> [FOR APPLICATION …] MIN_NODES … MAX_NODES … INSTANCE_FAMILY … [AUTO_RESUME] [INITIALLY_SUSPENDED] [AUTO_SUSPEND_SECS] [TAG (…)] [COMMENT] [PLACEMENT_GROUP] [BACKUP_INSTANCE_FAMILIES = ('…', …)]` |
| `BuildAlterComputePoolPropertySql(name, property, value)` | `ALTER COMPUTE POOL <name> SET <prop> = …`, or `UNSET <prop>` for a blank value where Snowflake allows it (auto resume, auto suspend secs, placement group, backup families, comment). `property` is the Snowflake keyword (`MIN_NODES`, …); the `settable` table lists them, and `ComputePoolPropertiesModal`'s `SETTABLE` uses the same keys |
| `BuildStopAllSql(name, types)` | `ALTER COMPUTE POOL <name> STOP ALL [OF TYPE t, …]` |

SHOW / DESCRIBE / SHOW NODES / SHOW INSTANCE FAMILIES, SUSPEND / RESUME and DROP
are one-liners in `internal/app/computepool.go`.

## Gotchas

- Instance family and workload type names are emitted **unquoted** (Snowflake
  treats them as keywords, not identifiers), so they must be bare identifiers
  (`snowflake.NeedsQuoting` is false) and are upper-cased. Backup families are string
  literals in the grammar (`('GPU_NV_S')`) but go through the same check.
- A new pool's name uses `QuoteOrBare`, so `my_pool` becomes `MY_POOL`, as it
  would in a worksheet.
- `MIN_NODES` / `MAX_NODES` have no UNSET. CREATE rejects `MAX_NODES < MIN_NODES`; a single-property ALTER can't see the other value, so the Properties modal checks it against the pool's current one.
