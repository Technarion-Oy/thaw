# components/semanticview

UI for Snowflake **SEMANTIC VIEW** objects.

A semantic view defines a semantic layer over physical tables for
natural-language querying with **Cortex Analyst**: logical `TABLES`, the
`RELATIONSHIPS` between them, and the `FACTS` / `DIMENSIONS` / `METRICS` that
describe the data.

## Components

- **`CreateSemanticViewModal.tsx`** — form-driven `CREATE SEMANTIC VIEW`: name +
  `OR REPLACE` / `IF NOT EXISTS` (mutually exclusive) + case control + comment,
  then one section per definition clause (`TABLES`, `RELATIONSHIPS`, `FACTS`,
  `DIMENSIONS`, `METRICS`) built from pickers, a **View options** panel
  (`MAX_STALENESS`, `AI_SQL_GENERATION`, `AI_QUESTION_CATEGORIZATION`,
  `AI_VERIFIED_QUERIES`, tags, `COPY GRANTS`), and an **Advanced — raw SQL
  definition** panel holding the original Monaco editor as an escape hatch
  (anything typed there replaces the structured definition). Create is disabled
  until the view has a name, a logical table, and at least one dimension or
  metric — Snowflake's rule, where a complete expression is
  `<table_alias>.<name> AS <sql_expr>` (the grammar has no alias-less form).
  Calls `BuildCreateSemanticViewSql` for the live preview and `ExecDDL` to run
  it; the builder, not the user, emits the clauses in the order Snowflake
  requires. It owns every section's state, so it is also where `updateTables`
  keeps alias references honest (see `semanticViewAliases.ts`).
- **`semanticViewForm.tsx`** — the section editors behind that modal:
  `TablesSection` (database → schema → table picker per row — a semantic view
  may reference tables in **any** database, so each row carries its own; the
  view's own db/schema only seed a new row — plus alias, `PRIMARY KEY` /
  `UNIQUE` column multi-selects, synonyms, comment, tags, and the preview-only
  `CONSTRAINT … DISTINCT RANGE` bounds), `RelationshipsSection` (alias →
  columns → `REFERENCES` alias, with the referenced columns constrained to the
  target's declared keys, and a standard / `ASOF` / `BETWEEN … EXCLUSIVE` join
  dropdown), `ExpressionsSection` (one component for facts, dimensions and
  metrics — the `kind` prop gates visibility, `LABELS = (FILTER)`, `USING` /
  `NON ADDITIVE BY`, and the Cortex Search binding), and
  `VerifiedQueriesSection`.

  Two caches keep the pickers cheap, both built on the shared `useKeyedFetch`
  (fetch-once per key, and forget a key when its fetch fails so a transient
  error can be retried instead of leaving a picker permanently empty):
  `useTableColumns` fetches each picked table's columns (`GetTableColumns`),
  keyed by the full `database.schema.table` path so rows in different databases
  don't collide, and `useObjectCache` is the one schema/object cache
  (`ListUserSchemas` / `ListObjects`) every `ObjectPicker` in the modal shares —
  without it five logical tables in one schema would mean five identical
  round-trips. The table picker offers every table-like kind (`TABLE`, `VIEW`,
  `MATERIALIZED VIEW`, `DYNAMIC TABLE`, `EXTERNAL TABLE`, `ICEBERG TABLE`,
  `HYBRID TABLE`, `EVENT TABLE`), the same set the model-monitor source picker
  uses.

  `TableRow` and `ExpressionRow` extend the generated config types with the
  picked `db` / `schema` / object parts, and `toLogicalTable` / `toExpression`
  derive the quoted `"db"."schema"."name"` reference the Go builder emits
  verbatim (via the shared `quoteIdent`). Keeping the parts on the row is what
  lets every cascade be a fully controlled component with no local state to
  fall out of step with its row.
- **`semanticViewAliases.ts`** — `diffAliases` / `remapAlias` /
  `remapQualified`. The other sections refer to logical tables by alias — a
  copied string, not a live reference — so renaming or deleting a table row
  would leave them pointing at an alias that no longer exists in
  `TABLES ( … )`, which neither the preview nor the builder can catch (the SQL
  is well-formed; only Snowflake rejects it). Every edit to the table list is
  diffed and the dependent rows rewritten: a renamed alias is followed, a
  removed one cleared. An alias two rows share is deliberately left alone — a
  reference is a bare string, so it can't be attributed to one of them, and
  remapping would silently repoint the row the user didn't touch. Covered by
  `semanticViewAliases.test.ts`.
- **`SemanticViewPropertiesModal.tsx`** — Overview (owner, created, editable
  comment via `AlterSemanticView`), a **Tags** section (the shared
  `TagsRow` + `useObjectTags` hook — tags read via `GetObjectTagReferences`, add /
  remove via `AlterSemanticView`), and lazily-loaded
  sections that surface the view's structure on demand: **Structure**
  (`DescribeSemanticView`), **Dimensions** (`ListSemanticDimensions`), **Facts**
  (`ListSemanticFacts`), **Metrics** (`ListSemanticMetrics`), and a
  **Dimensions for metric** lookup (`ListSemanticDimensionsForMetric`).

## Lifecycle

`ALTER SEMANTIC VIEW` only changes the comment, tags, or name — the definition
body is changed via `CREATE OR REPLACE`. **Rename** (context-menu) and **View
Definition** / object **comparison** are offered, because `GET_DDL` supports
semantic views (object_type `'SEMANTIC VIEW'`). Semantic views are queried with
the special `SELECT … FROM SEMANTIC_VIEW(…)` syntax, so they are not offered in
the "Select Top 1000" action.

See also: `components/mcpserver` (MCP servers can expose semantic views to Cortex
Analyst) and `components/materializedview` (another view type).
