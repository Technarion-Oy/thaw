# internal/datametricfunction

Builds SQL for Snowflake **DATA METRIC FUNCTION** (DMF) objects — user-defined
functions that encode a data-quality rule and return a single `NUMBER` metric
(e.g. a count of NULLs, of rows failing a regular expression, or of duplicate
keys). DMFs are scheduled against tables and views via
`ALTER TABLE … ADD DATA METRIC FUNCTION` and their results feed monitoring and
alerting; this package only covers the lifecycle of the DMF **definition**.

Unlike a regular UDF, a DMF's arguments are one or more named **TABLE** arguments
(each a set of typed columns) rather than scalar parameters, it always `RETURNS
NUMBER`, and its body is a deterministic scalar SQL expression that aggregates
over those arguments.

## What it does

`BuildCreateDataMetricFunctionSql(db, schema, cfg)` renders a
`CREATE DATA METRIC FUNCTION` statement from a `DataMetricFunctionConfig`:

```
CREATE [OR REPLACE] [SECURE] DATA METRIC FUNCTION [IF NOT EXISTS] <fqn>
  ( <arg_name> TABLE ( <col> <type> [, ...] ) [, <arg_name> TABLE ( ... ) ] )
  RETURNS NUMBER [NOT NULL]
  [ COMMENT = '<string>' ]
  AS
$$
<expression>
$$;
```

## Types & builders

- `DataMetricFunctionConfig` — name + case sensitivity, `OrReplace`,
  `IfNotExists`, `Secure`, `Args` (`[]DataMetricFunctionTableArg`), `NotNull`,
  `Comment`, `Body`.
- `DataMetricFunctionTableArg` — one TABLE argument: a `Name` plus its `Columns`
  (`[]DataMetricFunctionColumn`).
- `DataMetricFunctionColumn` — a single `{Name, Type}` column of a TABLE argument.
- `BuildCreateDataMetricFunctionSql` — the only exported builder.

## Gotchas

- **`OR REPLACE` and `IF NOT EXISTS` are mutually exclusive.** The builder drops
  `IF NOT EXISTS` when `OrReplace` is set (the create modal also clears the other
  checkbox).
- The return type is **always `NUMBER`** — there is no return-type field, only the
  optional `NOT NULL` modifier.
- The body is emitted with **`$$` dollar-quoting** so multi-line SQL expressions
  containing single quotes (e.g. `REGEXP` literals) need no escaping.
- Data metric functions share the **regular `FUNCTION` management commands** —
  there is no `ALTER`/`DROP DATA METRIC FUNCTION`. Mutations
  (`SET`/`UNSET COMMENT`, `SET`/`UNSET SECURE`, `RENAME TO`) are issued as
  free-form `ALTER FUNCTION <fqn>(<args>) <clause>` via
  `App.AlterDataMetricFunction`; the Sidebar drops them with
  `DROP FUNCTION <fqn>(<args>)`. All of these need the **TABLE argument
  signature** (e.g. `TABLE(NUMBER)`), which the tree carries as `objArgs`.
- `SHOW DATA METRIC FUNCTIONS` omits the **body**. `App.DescribeDataMetricFunction`
  runs `DESCRIBE FUNCTION <fqn>(<args>)` to supply it for the properties panel.
- DMFs also surface in `SHOW FUNCTIONS` with `is_data_metric = Y`;
  `internal/snowflake` (`showInSchema`) **relabels** those to kind
  `"DATA METRIC FUNCTION"` so they group under **Data Metric Functions** even when
  the dedicated `SHOW DATA METRIC FUNCTIONS` command fails, and
  `dedupeDataMetricFunctions` collapses the resulting duplicates (and drops a
  plain `FUNCTION` that collides on column-absent editions).
- `GET_DDL` has **no `DATA_METRIC_FUNCTION` object type** — DMFs are retrieved via
  the `'FUNCTION'` type with the TABLE argument signature appended. The
  `"DATA METRIC FUNCTION"` → `FUNCTION` normalization lives in `internal/snowflake`
  (`buildGetDDLQuery`), not here.
- The DMF `arguments` column nests parentheses (`MY_DMF(TABLE(NUMBER)) RETURN
  NUMBER`). `internal/snowflake`'s `extractArgTypes` matches the outer parens by
  depth so the `TABLE(...)` type survives intact (a naive first-`)` scan would
  truncate it).
