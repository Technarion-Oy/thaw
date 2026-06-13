# internal/alert

> SQL builder for Snowflake ALERT objects.

## Responsibility

Builds the `CREATE ALERT` DDL from a structured config. The lifecycle commands
(`RESUME`, `SUSPEND`, `SET`/`UNSET`, `MODIFY CONDITION`, `MODIFY ACTION`) are
simple enough that they are issued as free-form `ALTER ALERT <fqn> <clause>`
statements directly from `internal/app/alert.go` (`App.AlterAlert`) without a
dedicated builder. Manual execution uses the standalone `EXECUTE ALERT <fqn>`
statement (`App.ExecuteAlert`) — `EXECUTE` is its own SQL command, not an
`ALTER ALERT` clause.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `AlertConfig`, `BuildCreateAlertSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `AlertConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Warehouse` (empty ⇒ serverless), `Schedule`, comment, `Tags` (`[]snowflake.TagPair`), the `Condition` query, and the `Action` statement |
| `snowflake.TagClause` | Shared `TAG (...)` clause builder (in `internal/snowflake`); the alert grammar uses the `WITH TAG (...)` form, so the builder prepends `WITH ` to a non-empty result |
| `BuildCreateAlertSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] ALERT [IF NOT EXISTS] <fqn> [WITH TAG (…)] SCHEDULE='…' [WAREHOUSE=…] [COMMENT='…'] IF (EXISTS (<condition>)) THEN <action>;` — optional clauses emitted only when set, in documented order |

## Patterns & integration

- Required fields left empty (`Schedule`, `Condition`, `Action`) emit obvious
  placeholders so the live SQL preview reads as a completable template.
- `App.BuildCreateAlertSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterAlert` (in `internal/app/alert.go`) runs the lifecycle
  clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW ALERTS IN SCHEMA` with the
  fixed kind `"ALERT"`. Alerts are not surfaced by `SHOW OBJECTS`, so — unlike
  dynamic / external tables and materialized views — no dedupe pass is needed.
- DDL export: alerts are retrieved via the `GET_DDL` object type `ALERT`
  (`buildGetDDLQuery` needs no special-casing — the SHOW kind already matches the
  `GET_DDL` object type).
- Properties panel: `internal/objects` runs `SHOW ALERTS LIKE …` for the
  `ALERT` kind.

## Gotchas

- **No RENAME** — `ALTER ALERT` has no `RENAME TO` variant (like external
  tables), so Rename is not offered for alerts.
- **Warehouse is optional** — omitting `WAREHOUSE` creates a serverless alert
  that runs on Snowflake-managed compute. The app's own CREATE-ALERT editor
  diagnostic still flags a missing `WAREHOUSE`; that is a separate, stricter
  lint and does not affect the visual builder.
- The `Condition` query and `Action` statement are appended verbatim (after
  trimming trailing semicolons); their SQL validity surfaces at execution time,
  not in the builder.
