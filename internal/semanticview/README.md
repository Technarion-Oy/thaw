# internal/semanticview

Builds SQL for Snowflake **SEMANTIC VIEW** objects.

A semantic view defines a semantic layer over physical tables for
natural-language querying with **Cortex Analyst**. It names logical `TABLES`,
the `RELATIONSHIPS` between them, and the `FACTS` / `DIMENSIONS` / `METRICS`
that describe the business meaning of the data.

## Types & functions

- `SemanticViewConfig` — name, case flag, `OR REPLACE` / `IF NOT EXISTS`, the
  `Body` (the order-sensitive `TABLES` / `RELATIONSHIPS` / `FACTS` /
  `DIMENSIONS` / `METRICS` clauses, taken verbatim from the create modal's
  Monaco editor), an optional `Comment`, and a `CopyGrants` flag.
- `BuildCreateSemanticViewSql(db, schema, cfg)` — renders:

  ```sql
  CREATE [OR REPLACE] SEMANTIC VIEW [IF NOT EXISTS] <fqn>
    TABLES ( … )
    [RELATIONSHIPS ( … )]
    [FACTS ( … )]
    [DIMENSIONS ( … )]
    [METRICS ( … )]
    [COMMENT = '…']
    [COPY GRANTS];
  ```

  The `Body` is emitted verbatim — the order of the clauses matters to Snowflake
  (e.g. `FACTS` must precede `DIMENSIONS`), and the builder does not reorder or
  validate it. A blank body falls back to a minimal `TABLES` placeholder so the
  live preview reads as a completable template.

## ALTER / lifecycle

`ALTER SEMANTIC VIEW` can only **rename** the view, set/unset its **comment**,
or set/unset **tags** — the definition body cannot be altered (change it via
`CREATE OR REPLACE`). These run through `App.AlterSemanticView(db, schema, name,
clause)` in `internal/app/semanticview.go` (a thin wrapper over the shared
`alterObject` helper).

`SHOW SEMANTIC VIEWS` reports only `created_on` / `name` / `database_name` /
`schema_name` / `comment` / `owner` / `owner_role_type`, so the structure is
read from:

- `App.DescribeSemanticView` — `DESCRIBE SEMANTIC VIEW` (one row per logical
  table / relationship / dimension / fact / metric property).
- `App.ListSemanticDimensions` / `ListSemanticFacts` / `ListSemanticMetrics` —
  the `SHOW SEMANTIC DIMENSIONS|FACTS|METRICS IN <fqn>` commands.
- `App.ListSemanticDimensionsForMetric` — `SHOW SEMANTIC DIMENSIONS IN <fqn> FOR
  METRIC <metric>` (which dimensions are queryable alongside a given metric).
- `App.GetSemanticViewTags` — the tags applied to the view, for the tag editor.

`GET_DDL` **supports** semantic views directly (object_type `'SEMANTIC VIEW'`),
so View Definition / object comparison work without any special handling in
`internal/snowflake`.

See also: `internal/mcpserver` (MCP servers can expose semantic views to Cortex
Analyst) and `internal/view` / `internal/materializedview` (other view types).
