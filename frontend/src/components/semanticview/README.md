# components/semanticview

UI for Snowflake **SEMANTIC VIEW** objects.

A semantic view defines a semantic layer over physical tables for
natural-language querying with **Cortex Analyst**: logical `TABLES`, the
`RELATIONSHIPS` between them, and the `FACTS` / `DIMENSIONS` / `METRICS` that
describe the data.

## Components

- **`CreateSemanticViewModal.tsx`** — name + `OR REPLACE` / `IF NOT EXISTS`
  (mutually exclusive) + case control + a Monaco SQL editor for the definition
  body (`TABLES` → `RELATIONSHIPS` → `FACTS` → `DIMENSIONS` → `METRICS`, in that
  required order) + an optional comment + a `COPY GRANTS` checkbox. Calls
  `BuildCreateSemanticViewSql` for the live preview and `ExecDDL` to run it.
- **`SemanticViewPropertiesModal.tsx`** — Overview (owner, created, editable
  comment via `AlterSemanticView`), a **Tags** section (chips from
  `GetSemanticViewTags`; add / remove via `SET` / `UNSET TAG`), and lazily-loaded
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
