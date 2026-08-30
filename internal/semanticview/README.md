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
  SQL, so the live preview stays valid while a row is being filled in. That
  tolerance is deliberate and extends to the statement as a whole: Snowflake's
  "at least one dimension or metric" rule is enforced by the create modal's own
  `structuredValid` (its only current caller), not here — this function only
  rejects a value that's actively *wrong* (`MaxStaleness` below the floor), not
  one that's merely *not yet provided*, since erroring on the latter would mean
  a blank live preview for the ordinary mid-edit state of having added a table
  but no dimension or metric yet.

  `CaseSensitive` governs **every** identifier in the statement — the view's own
  name plus the aliases, columns and entity names inside the clauses, including
  each half of a dotted `NON ADDITIVE BY` reference — matching the other CREATE
  builders (hybrid / iceberg / external tables all quote their column names with
  `cfg.CaseSensitive`). The per-entity renderers hang off a `renderer` value
  that carries the flag. `qualifiedIdent` splits a `NON ADDITIVE BY` reference
  on its *last* dot, not its first — the table alias half is a free-text form
  field with no restriction against containing one (e.g. `ord.v2`), so
  splitting on the first dot would fold the rest of the alias into the name
  half and quote it as one nonexistent identifier. Human-entered free text —
  comments, synonyms, the AI instruction fields, the verified-query strings —
  goes through `snowflake.QuoteTextLit`, which doubles backslashes as well as
  quotes.

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

`ALTER SEMANTIC VIEW` can **rename** the view, set/unset its **comment**,
set/unset **tags**, set/unset **MAX_STALENESS**, and add / drop / suspend /
resume / refresh **materializations** — the definition body itself
(`TABLES`/`RELATIONSHIPS`/`FACTS`/`DIMENSIONS`/`METRICS`) still cannot be
altered (change it via `CREATE OR REPLACE`). The rename / comment / tag /
MAX_STALENESS / suspend / resume / refresh / drop actions are single, fixed
clauses, so — like the equivalent single-clause actions in every other
Properties modal (`DynamicTablePropertiesModal.tsx`'s target lag/warehouse,
`ServicePropertiesModal.tsx`'s numeric settings, …) — they're assembled as a
plain string by the frontend (`SemanticViewPropertiesModal.tsx`) and run
through `App.AlterSemanticView(db, schema, name, clause)` in
`internal/app/semanticview.go` (a thin wrapper over the shared `alterObject`
helper).

`ADD MATERIALIZATION` is different: a multi-part statement (required
warehouse, optional `REFRESH_MODE`/`IMMUTABLE WHERE`, required dimension/metric
lists, optional `WHERE`) on par with the CREATE builder above, not a one-line
`SET`. So it gets its own Go builder, `BuildAddMaterializationSql`, exposed as
`App.BuildAddSemanticViewMaterializationSql` in `internal/app/builders.go` —
the frontend calls it for the SQL, then `ExecDDL` to run it, the same
two-step split `internal/dbtproject`'s `BuildAddVersionSql` /
`AddDbtProjectVersionModal.tsx` use for `ADD VERSION`.

`MAX_STALENESS` must be set (minimum 120 seconds, matching `MinMaxStaleness`
above) before a materialization can be added, and Snowflake rejects `UNSET
MAX_STALENESS` while one exists. Adding a materialization
(`ADD MATERIALIZATION <name> WAREHOUSE = <wh> [REFRESH_MODE = ...] [IMMUTABLE
WHERE (...)] AS DIMENSIONS ... METRICS ... [WHERE (...)]`) requires a
warehouse and at least one dimension and one metric — each qualified by its
logical table (`DIMENSIONS customers.customer_name`, per Snowflake's own
example), since a bare name is ambiguous once a view joins more than one
logical table. `BuildAddMaterializationSql` quotes each `table.name` reference
via `quoteMaterializationRef`, which — like the CREATE builder's
`renderer.qualifiedIdent` just above — splits on the *last* dot and quotes
both halves; both now share that splitting rule through the package-level
`splitLastDotQuoted(ref, quote)`, parameterized only by which quoting function
to apply (`renderer.ident`, gated by the config's case-sensitivity flag, vs.
the materialization builder's unconditional `snowflake.QuoteIdent` — these
names come from live `SHOW` output, not a user-typed identifier). The
dimension/metric emptiness check runs *after* that quoting/filtering, not
before: an all-whitespace entry survives a raw `len()` check but
`quoteMaterializationRef` drops it, so checking the filtered slice is what
actually catches a materialization that would otherwise render an empty
`DIMENSIONS`/`METRICS` clause. Snowflake exposes no `SHOW`/`DESCRIBE` for a
view's existing materializations, so `DROP`/`SUSPEND`/`RESUME`/`REFRESH
MATERIALIZATION` act on a typed-in name rather than a picked row.

The SQL editor's own grammar validator (`internal/sqlgrammar`) has to know
this syntax independently of the builders above — `ParseAlterSemanticView`
models the full `SET`/`UNSET MAX_STALENESS` and
`ADD`/`DROP`/`SUSPEND`/`RESUME`/`REFRESH MATERIALIZATION` forms (`WHERE ( … )`
and `IMMUTABLE WHERE ( … )` accepted as a balanced parenthesized span via the
shared `consumeBalancedParens`, the same tolerance `ParseCreateSemanticView`
gives its own clause bodies), so a statement this package's builders and the
Properties modal produce doesn't get flagged as a syntax error in the editor.

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
