# internal/modelmonitor

> SQL builder for Snowflake MODEL MONITOR (ML observability) objects.

## Responsibility

Builds the `CREATE MODEL MONITOR` DDL from a structured config. A model monitor
is a schema-level object that provides observability for a model registered in
the Snowpark ML Model Registry: it tracks model performance metrics, prediction
quality, and data drift by aggregating a source table/view (and, optionally, a
baseline) on a refresh schedule.

`CREATE MODEL MONITOR` carries its parameters in a `WITH` clause. Eight are
required — `MODEL`, `VERSION`, `FUNCTION`, `SOURCE`, `WAREHOUSE`,
`REFRESH_INTERVAL`, `AGGREGATION_WINDOW`, `TIMESTAMP_COLUMN`. The rest are
optional: `BASELINE` plus the column-array parameters (`ID_COLUMNS`,
`PREDICTION_CLASS_COLUMNS`, `PREDICTION_SCORE_COLUMNS`, `ACTUAL_CLASS_COLUMNS`,
`ACTUAL_SCORE_COLUMNS`, `SEGMENT_COLUMNS`, `CUSTOM_METRIC_COLUMNS`). At least one
prediction column (score or class) is mandatory; the create modal enforces this.

The mutable surface is small: `ALTER MODEL MONITOR` only supports `SUSPEND` /
`RESUME`, `SET BASELINE` / `REFRESH_INTERVAL` / `WAREHOUSE`, and
`ADD` / `DROP segment_column`. There is no `RENAME`, `COMMENT`, or `TAG`. Those
ALTERs are issued as free-form `ALTER MODEL MONITOR <fqn> <clause>` statements
from `internal/app/modelmonitor.go` (`App.AlterModelMonitor`), without a
dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ModelMonitorConfig`, `BuildCreateModelMonitorSql`, `columnArray` helper |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ModelMonitorConfig` | CREATE parameters: name + case/replace flags, the 8 required WITH fields, and the optional `Baseline` + column arrays |
| `BuildCreateModelMonitorSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] MODEL MONITOR [IF NOT EXISTS] <fqn> WITH …;` |

## Patterns & integration

- A blank name emits the placeholder `model_monitor_name`; blank required fields
  emit per-field placeholders (`model_name`, `version_name`, `source_table`, …)
  so the live SQL preview reads as a completable template while the user types.
- Quoting follows the published grammar exactly: `WAREHOUSE` (account-level) and
  `TIMESTAMP_COLUMN` (a column of the source) are identifiers emitted verbatim;
  `MODEL`, `SOURCE`, and `BASELINE` are schema-level object references and are
  fully qualified with the monitor's own database & schema (the create modal only
  offers objects from `db.schema`, and Snowflake requires the model to live in
  the monitor's schema, so creation works even when the session's current schema
  differs from the monitor's target schema); `VERSION`, `FUNCTION`,
  `REFRESH_INTERVAL`, `AGGREGATION_WINDOW` are single-quoted string literals; the
  column arrays are ARRAY constants — parenthesised, comma-separated lists of
  single-quoted string literals, e.g. `('id', 'region')`. Note the CREATE↔ALTER
  asymmetry for **`BASELINE`** that matches Snowflake's own grammar: it is an
  **identifier** in `CREATE` (`BASELINE = <db.schema.tbl>`) but a **string
  literal** in `ALTER` (`SET BASELINE='…'`). Segment columns are string literals
  in both (`SEGMENT_COLUMNS = ('a','b')` in CREATE, `ADD/DROP segment_column='a'`
  in ALTER).
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateModelMonitorSql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterModelMonitor` (in `internal/app/modelmonitor.go`) runs
  the edit/lifecycle clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW MODEL MONITORS IN SCHEMA`
  with the fixed kind `"MODEL MONITOR"`. Model monitors are not surfaced by
  `SHOW OBJECTS`, so — like models, masking policies, and alerts — no dedupe pass
  is needed.
- Properties panel: `internal/objects` runs `SHOW MODEL MONITORS LIKE …` for the
  `MODEL MONITOR` kind; the modal exposes Suspend/Resume, editable
  Baseline/Refresh interval/Warehouse, and segment-column add/drop.

## Gotchas

- **`GET_DDL` is not supported** for model monitors (the get_ddl object-type
  enumeration omits `MODEL MONITOR`), so there is no DDL export / "View
  Definition" / comparison path and no `buildGetDDLQuery` mapping for this kind.
  `App.GetObjectDDL` rejects the kind up front, and the sidebar excludes model
  monitors from the DDL-driven menu actions.
- **`RENAME` is not supported** — `ALTER MODEL MONITOR` has no `RENAME TO`, so
  model monitors *are* added to the sidebar's Rename-exclusion.
- **Limited ALTER surface** — only `SUSPEND`/`RESUME`, `SET BASELINE`/
  `REFRESH_INTERVAL`/`WAREHOUSE`, and `ADD`/`DROP segment_column` are mutable; the
  remaining configuration (model, version, function, source, columns, aggregation
  window, timestamp column) is fixed at creation. Use `CREATE OR REPLACE` to
  change those.
