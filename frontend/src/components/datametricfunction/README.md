# components/datametricfunction

UI for Snowflake **DATA METRIC FUNCTION** (DMF) objects in the object browser —
user-defined functions that encode a data-quality rule and return a single
`NUMBER` metric, scheduled against tables and views for monitoring and alerting.

## Components

- **`CreateDataMetricFunctionModal`** — builds a `CREATE DATA METRIC FUNCTION`
  statement. Name + `OR REPLACE` / `IF NOT EXISTS` (mutually exclusive — toggling
  one clears the other) / `SECURE`, a **table-arguments editor** (one or more
  named TABLE arguments, each with its own name + column rows), the fixed
  `RETURNS NUMBER`
  return type + `NOT NULL`, a monospace **body** editor (the scalar SQL
  expression), and an optional comment. Live SQL preview; submission is gated on
  name + body + at least one named column. SQL is built by
  `BuildCreateDataMetricFunctionSql` (`internal/datametricfunction`).
- **`DataMetricFunctionPropertiesModal`** — overview (owner, created-on, language)
  + an editable **Settings** section that exposes every `ALTER FUNCTION` clause a
  DMF supports, all via `AlterDataMetricFunction`: **rename** (`RENAME TO` — on
  success calls the `onChanged` prop to refresh the tree, then closes since the
  identity changed), inline-editable **comment** (`SET`/`UNSET COMMENT`), a
  `SECURE` toggle (`SET`/`UNSET SECURE`), and a **Tags** editor (current tags from
  `GetDataMetricFunctionTags` as removable chips → `UNSET TAG`, plus an add form →
  `SET TAG <tag> = '<value>'`). Then a **Data Metric Function Detail** section
  backed by `DescribeDataMetricFunction` (`DESCRIBE FUNCTION`): signature, returns,
  language, and the **body** (the metric expression `SHOW DATA METRIC FUNCTIONS`
  omits), and an on-demand **Associated Tables & Views** list from
  `GetDataMetricFunctionReferences`. Takes the TABLE argument signature (`args`,
  e.g. `TABLE(NUMBER)`) needed to resolve the overload for `DESCRIBE` / `ALTER
  FUNCTION`.

## Notes

- DMFs are dropped with `DROP FUNCTION <fqn>(<args>)` and altered with
  `ALTER FUNCTION <fqn>(<args>) …` — all of which need the TABLE argument
  signature, carried through the sidebar tree as `objArgs`.
- `GET_DDL` (View Definition / Compare) works via the `'FUNCTION'` object type;
  the kind normalization lives in `internal/snowflake`.
