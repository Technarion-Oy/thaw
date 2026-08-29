# internal/semanticview

Builds SQL for Snowflake **SEMANTIC VIEW** objects.

A semantic view defines a semantic layer over physical tables for
natural-language querying with **Cortex Analyst**. It names logical `TABLES`,
the `RELATIONSHIPS` between them, and the `FACTS` / `DIMENSIONS` / `METRICS`
that describe the business meaning of the data.

## Types & functions

- `SemanticViewConfig` — name, case flag, `OR REPLACE` / `IF NOT EXISTS`, the
  structured definition (`Tables`, `Relationships`, `Facts`, `Dimensions`,
  `Metrics`), the view-level options (`Comment`, `MaxStaleness`,
  `AISqlGeneration`, `AIQuestionCategorization`, `VerifiedQueries`, `Tags`,
  `CopyGrants`), and `Body` — the raw-SQL escape hatch.
- `LogicalTable` — one `TABLES ( … )` entry: alias, the physical table
  reference (emitted verbatim; the modal supplies a quoted FQN), `PRIMARY KEY`
  and `UNIQUE` column lists, the preview-only
  `CONSTRAINT … DISTINCT RANGE BETWEEN … EXCLUSIVE` bounds, synonyms, comment,
  tags. *Only one `UNIQUE` clause is modeled*; a table needing several distinct
  unique constraints is served by `Body`.
- `Relationship` — `[ name AS ] alias ( cols ) REFERENCES ref_alias ( … )`, with
  `JoinType` selecting the reference form: `""` (standard), `"ASOF"`, or
  `"BETWEEN"` (the latter two are Snowflake preview features). `ASOF` and
  `BETWEEN` require their reference form, so a row missing it is dropped rather
  than silently downgraded to a standard relationship; only the standard form
  may omit the columns (Snowflake then matches the target's declared key).
- `Expression` — one `FACTS` / `DIMENSIONS` / `METRICS` entry. The grammar is
  `[ visibility ] <table_alias>.<name> … AS <sql_expr>`, so `TableAlias`,
  `Name` and `Expr` are all required. The three
  grammars overlap almost entirely, so one type covers all of them and the
  renderer gates the clause-specific parts: `Visibility` (`PRIVATE` is dropped
  for dimensions, which are always public), `FilterLabel` →
  `LABELS = (FILTER)` (facts & dimensions), `Using` / `NonAdditiveBy`
  (metrics), `CortexSearchService` / `CortexSearchColumn` (dimensions). A
  window-function metric is just an `Expr` carrying its own `OVER ( … )` —
  Snowflake's separate window-metric production adds no surrounding keywords.
- `NonAdditiveDim` — one `NON ADDITIVE BY` entry with its `ASC`/`DESC` and
  `NULLS FIRST`/`LAST` ordering.
- `VerifiedQuery` — one `AI_VERIFIED_QUERIES` entry (`QUESTION`, `VERIFIED_AT`,
  `ONBOARDING_QUESTION`, `VERIFIED_BY`, `SQL`). `VERIFIED_AT` is the one value in
  the grammar embedded unquoted, so it is emitted only when
  `snowflake.IsNumericID` accepts it; anything else drops the clause rather than
  splicing raw text into the statement.
- `BuildCreateSemanticViewSql(db, schema, cfg)` — renders:

  ```sql
  CREATE [OR REPLACE] SEMANTIC VIEW [IF NOT EXISTS] <fqn>
    TABLES ( … )
    [RELATIONSHIPS ( … )]
    [FACTS ( … )]
    [DIMENSIONS ( … )]
    [METRICS ( … )]
    [COMMENT = '…']
    [MAX_STALENESS = <n>]
    [AI_SQL_GENERATION '…']
    [AI_QUESTION_CATEGORIZATION '…']
    [AI_VERIFIED_QUERIES ( … )]
    [WITH TAG ( … )]
    [COPY GRANTS];
  ```

  The clause order matters to Snowflake (e.g. `FACTS` must precede
  `DIMENSIONS`) and is guaranteed by the builder, not the user. An incomplete
  entry — a table row with no table, a relationship with no target, an
  expression with no alias, name or SQL — renders as nothing rather than broken
  SQL, so the live preview stays valid while a row is being filled in.

  `CaseSensitive` governs **every** identifier in the statement — the view's own
  name plus the aliases, columns and entity names inside the clauses, including
  each half of a dotted `NON ADDITIVE BY` reference — matching the other CREATE
  builders (hybrid / iceberg / external tables all quote their column names with
  `cfg.CaseSensitive`). The per-entity renderers hang off a `renderer` value
  that carries the flag. Human-entered free text — comments, synonyms, the AI
  instruction fields, the verified-query strings — goes through
  `snowflake.QuoteTextLit`, which doubles backslashes as well as quotes.

  The trailing clauses are ordered per production, and the two orders differ:
  `logicalTable` is `WITH SYNONYMS` → `COMMENT` → `WITH TAG`, while the
  fact / dimension / metric productions are `WITH SYNONYMS` → `WITH TAG` →
  `COMMENT`. Both are asserted by the tests so the divergence can't drift.

  A non-blank `Body` **replaces the whole structured definition** and is emitted
  verbatim, so anything the create form doesn't cover can still be typed into
  the modal's Monaco editor; the clause order is then the user's responsibility.
  With neither a body nor any logical table, a minimal `TABLES` placeholder
  keeps the live preview a completable template.

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
- The Tags editor reads the view's tags through the shared `App.GetObjectTagReferences`.

`GET_DDL` **supports** semantic views directly (object_type `'SEMANTIC VIEW'`),
so View Definition / object comparison work without any special handling in
`internal/snowflake`.

See also: `internal/mcpserver` (MCP servers can expose semantic views to Cortex
Analyst) and `internal/view` / `internal/materializedview` (other view types).
