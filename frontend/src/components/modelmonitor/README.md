# components/modelmonitor

> UI for creating and managing Snowflake MODEL MONITOR objects (ML observability).

A model monitor tracks performance metrics, prediction quality, and data drift
for a model registered in the Snowpark ML Model Registry, by aggregating a source
table/view (and optionally a baseline) on a refresh schedule.

## Components

| File | Purpose |
|---|---|
| `CreateModelMonitorModal.tsx` | Create form, mostly dropdown-driven from the schema's own objects: **Model** picker (`ListObjects` filtered to `MODEL`), **Version** picker (`ListModelVersions` for the chosen model — disabled until a model is selected), **Source** / **Baseline** pickers (`ListObjects` filtered to table-like kinds), **Warehouse** picker (`ListWarehouses`), and **Timestamp column** (an AutoComplete that only suggests `TIMESTAMP_NTZ` columns, per Snowflake's requirement) + all seven **column-array** parameters as Selects offering the source table's columns (`GetTableColumnsWithTypes` — all of these stay free-typeable so a name not in the list, or one when `DESCRIBE` returns nothing, can still be entered; `SEGMENT_COLUMNS` is capped at 5). **Refresh interval** is a number + unit (seconds/minutes/hours/days) composer; **Aggregation window** is a number with a fixed `days` suffix. **Function** stays a free-text input (no list source). Validates that the eight required fields are filled and at least one prediction column (score or class) is set. Builds SQL via `BuildCreateModelMonitorSql` with a live preview. |
| `ModelMonitorPropertiesModal.tsx` | Properties panel: Suspend/Resume lifecycle buttons, editable Baseline / Refresh interval / Warehouse rows, a Segment Columns section (add / drop chips), and the raw `SHOW MODEL MONITORS` property table. Edits run through `AlterModelMonitor`. |

## Integration

- Wired into `components/layout/Sidebar.tsx`: a "Model Monitor…" item in the
  Create-Object **Machine Learning** submenu and a type-node "Create Model
  Monitor…" item open the create modal; an obj-node "Properties…", "Suspend", and
  "Resume" items drive the properties modal and lifecycle.
- The kind icon (`LineChartOutlined`, `--icon-modelmonitor`) is registered in
  `components/sidebar/objectIcons.tsx` and `styles/global.css`.

## Gotchas

- **No `GET_DDL`** for model monitors — they are excluded from View Definition,
  comparison, and the DDL-hover tooltip in the sidebar.
- **No `RENAME`** — `ALTER MODEL MONITOR` has no `RENAME TO`, so model monitors
  are in the sidebar's Rename-exclusion. The create config arrays mean the Wails
  config class needs an `as any` cast at the `BuildCreateModelMonitorSql` boundary
  (the modal uses a plain local `MMConfig` interface).
- **Limited ALTER** — only Suspend/Resume, SET Baseline/Refresh interval/Warehouse,
  and ADD/DROP segment_column are editable; everything else is fixed at creation
  (use Create with OR REPLACE to change it).
- **Source / Baseline are scoped to the monitor's own schema** — the create
  modal's pickers only list objects from `db.schema` and the builder hard-qualifies
  them with `db.schema`. Snowflake permits a source in another schema/database;
  lifting that would mean making these fields free-typeable. Reasonable v1 limit.
