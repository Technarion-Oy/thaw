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
  (anything typed there replaces the structured definition — and, with it, the
  form's validation, so Create then only requires a name and a body free of the
  template's `<database>` / `<schema>` / `<table>` tokens). On the structured
  path Create is disabled until the view has a name, a logical table, and at
  least one dimension or metric — Snowflake's rule, where a complete expression
  is `<table_alias>.<name> AS <sql_expr>` (the grammar has no alias-less form).
  Calls `BuildCreateSemanticViewSql` for the live preview (debounced 250ms —
  `cfg` gets a new identity on every keystroke in any of the form's many
  fields) and `ExecDDL` to run it; the builder, not the user, emits the clauses
  in the order Snowflake requires. A failed build (e.g. a `MAX_STALENESS` below
  Snowflake's floor) blanks the preview and disables Create rather than
  executing stale SQL. Submitting rebuilds the statement fresh from `cfg`
  rather than trusting the debounced preview string — an edit made just before
  clicking Create could still be within that 250ms window and not yet
  reflected in it. It owns every section's state, so it is also where
  `updateTables` / `updateRelationships` / `updateDimensions` keep cross-clause
  references honest (see `semanticViewAliases.ts`) — re-validated against both
  identity (renamed/removed) and *completeness* (e.g. a relationship losing its
  columns, or a dimension losing its expression, without any rename) on every
  edit, since the builder drops an incomplete row the same as a removed one.
- **`semanticViewForm.tsx`** — the section editors behind that modal:
  `TablesSection` (database → schema → table picker per row — a semantic view
  may reference tables in **any** database, so each row carries its own; the
  view's own db/schema only seed a new row — plus alias, `PRIMARY KEY` /
  `UNIQUE` column multi-selects, synonyms, comment, tags, and the preview-only
  `CONSTRAINT … DISTINCT RANGE` bounds), `RelationshipsSection` (alias →
  columns → `REFERENCES` alias, with the referenced columns constrained to the
  target's declared keys — except for `ASOF`, which references any
  type-compatible column (e.g. a timestamp) instead — and a standard / `ASOF` /
  `BETWEEN … EXCLUSIVE` join dropdown), `ExpressionsSection` (one component for facts, dimensions and
  metrics — the `kind` prop gates visibility, `LABELS = (FILTER)`, `USING` /
  `NON ADDITIVE BY`, and the Cortex Search binding), and
  `VerifiedQueriesSection`.

  Two caches keep the pickers cheap, both built on the shared `useKeyedFetch`
  (fetch-once per key, and forget a key when its fetch fails so a transient
  error can be retried): `useTableColumns` fetches each picked table's columns
  (`GetTableColumns`), keyed by the full `database.schema.table` path so rows
  in different databases don't collide, and `useObjectCache` is the one
  schema/object cache (`ListUserSchemas` / `ListObjects`) every `ObjectPicker`
  in the modal shares — without it five logical tables in one schema would
  mean five identical round-trips. Forgetting the key only makes a retry
  *eligible*; something still has to call `ensure` again. `useTableColumns`
  gets that for free (its effect depends on the whole `rows` array, which gets
  a new identity on almost any table edit), but `ObjectPicker`'s own effects
  depend on nothing that changes after a failure — so `useKeyedFetch` also
  bumps a `retryTick` counter on failure, and `useObjectCache` exposes it
  purely so `ObjectPicker`'s effects can list it as a dependency and actually
  re-fire. Capped at 3 attempts per key — a *persistent* failure (no privilege
  on the object, a deleted table) would otherwise retry forever, and since
  `retryTick` is shared across the whole cache, one dead lookup would keep
  re-firing every other picker's effects too. The table picker offers every
  table-like kind, from the shared `components/shared/objectKinds.ts`
  (`TABLE_LIKE_KINDS`) the model-monitor source picker also uses.

  `TableRow` and `ExpressionRow` extend the generated config types with the
  picked `db` / `schema` / object parts, and `toLogicalTable` / `toExpression`
  derive the quoted `"db"."schema"."name"` reference the Go builder emits
  verbatim (via the shared `quoteQualifiedIdent`, `ObjectNameCaseControl.tsx` —
  also used by `useObjectTags.ts`, so the quote-each-part-and-join pattern
  lives in one place instead of being rewritten per caller). The `Select`
  options every picker builds go through the shared `opts` from
  `components/shared/PropertyRows.tsx` too (a rest-arg helper, so a call site
  holding an array spreads it: `opts(...columns)`). Keeping the parts
  on the row is what lets every cascade be a fully controlled component with
  no local state to fall out of step with its row. `toLogicalTable` always
  sends the row's resolved `aliasOf(r)`, not the possibly-blank `alias` field
  — every other clause references a table by that resolved value, so the
  builder must declare it with `AS alias` too, or a table left without a typed
  alias would be referenced by an identifier `TABLES ( … )` never actually
  declares — and destructures the form-only `db`/`schema`/`table` parts out
  rather than spreading the whole row, so they can't leak into the
  `LogicalTable` JSON if that struct ever grows a same-named field.

  Two rows can share an alias (nothing stops a user typing the same one
  twice, or an auto-seeded alias colliding with an existing one), which is
  ambiguous for every alias-keyed lookup in this file: `useTableColumns`'
  `columnsFor(alias)` resolves to whichever row comes first. Its own callers
  in `TablesSection` already hold the actual row, so they pass it as a second
  `row` argument to resolve unambiguously; `RelationshipsSection` only has a
  copied alias string (a relationship doesn't reference a row, it references
  an alias) and has no way around the ambiguity. Rather than have that picker
  silently show one row's columns for the other, `duplicateAliases` flags the
  colliding rows — `TablesSection` marks their alias `Input` with an error
  state, and `CreateSemanticViewModal.tsx`'s `structuredValid` blocks Create
  outright while any alias is duplicated, since `semanticViewAliases.ts`
  can't safely remap a reference to one either. `duplicateRelationshipNames`
  is the same check one clause over — two `RELATIONSHIPS` rows sharing a name
  make a metric's `USING` reference to it ambiguous the same way, and get the
  same error-state `Input` plus `structuredValid` block.

  The `WITH CORTEX SEARCH SERVICE … USING <column>` binding is a dimension-only
  `ObjectPicker` cascade (database → schema → service) plus a free-text "USING
  column" `Input`; the whole clause — column included — is gated in
  `renderer.expression` (`internal/semanticview/sql.go`) on the service
  actually being picked, so the column `Input` is disabled until `cortexName`
  is set rather than silently discarding text typed into a field with no
  effect yet.
- **`semanticViewAliases.ts`** — `diffAliases` / `remapAlias` /
  `remapQualified` / `swappedAliases`. The other sections refer to logical tables by alias — a
  copied string, not a live reference — so renaming or deleting a table row
  would leave them pointing at an alias that no longer exists in
  `TABLES ( … )`, which neither the preview nor the builder can catch (the SQL
  is well-formed; only Snowflake rejects it). Every edit to the table list is
  diffed and the dependent rows rewritten: a renamed alias is followed, a
  removed one cleared. An alias two rows share is deliberately left alone — a
  reference is a bare string, so it can't be attributed to one of them, and
  remapping would silently repoint the row the user didn't touch. The same
  diffing runs over two more reference kinds a metric copies by string: the
  relationship names its `USING` holds, and the `alias.name` its
  `NON ADDITIVE BY` holds. Re-pointing a row at a different table is tracked
  separately by `swappedAliases`, because it isn't a rename — columns picked
  against the old table don't carry over, so they are cleared. It compares table
  identity rather than alias: picking a new table also reseeds an untouched
  alias, so the two change together and an alias-based check would miss the
  ordinary case. Covered by `semanticViewAliases.test.ts`. This module only
  tracks *identity* (a rename or removal) — a row can also go stale by
  becoming *incomplete* while its name/alias stays put (a relationship losing
  its columns, a dimension losing its expression), which the modal's
  `updateRelationships` / `updateDimensions` re-check directly against
  `isCompleteRelationship` / `isCompleteExpression` on every edit, since this
  module has no notion of completeness. Note `isCompleteRelationship` isn't
  quite "will the builder emit this row": Snowflake's relationship name is
  optional (the Go renderer emits a nameless one fine), but a metric's `USING`
  refers to a relationship *by* that name, so a nameless relationship can
  never be a valid `USING` option even though it's a perfectly complete row —
  `isCompleteRelationship` answers the *referenceable* question, which is
  strictly the one every one of its callers actually needs. `remapQualified`
  splits a dotted `alias.name` reference on its *last* dot rather than its
  first, for the same reason `qualifiedIdent` in `internal/semanticview/sql.go`
  does: the alias half is a free-text field with no restriction against
  containing one.
- **`SemanticViewPropertiesModal.tsx`** — Overview (owner, created, editable
  comment via `AlterSemanticView`, plus an editable `MAX_STALENESS` — the
  shared `EditRow`'s `numeric` mode, bounded `[MIN_MAX_STALENESS,
  Number.MAX_SAFE_INTEGER]` and integer-only via `precision={0}`), a **Tags**
  section (the shared `TagsRow` + `useObjectTags` hook — tags read via
  `GetObjectTagReferences`, add / remove via `AlterSemanticView`),
  lazily-loaded sections that surface the view's structure on demand:
  **Structure** (`DescribeSemanticView`), **Dimensions**
  (`ListSemanticDimensions`), **Facts** (`ListSemanticFacts`), **Metrics**
  (`ListSemanticMetrics`), a **Materializations** section (Suspend / Resume /
  Refresh / Drop by typed-in name — quoted via `identToken(trimmed, false)`,
  not an unconditional `quoteIdent`, so a name typed exactly as it was
  originally written (and thus uppercase-folded by Snowflake if left
  unquoted) still resolves instead of being forced into a case-sensitive,
  now-nonexistent quoted identifier; `matAction` tracks which *one* of those
  four is in flight, or `null` when idle, so only the clicked button's
  `loading` prop lights up instead of all four sharing one flag; the Add
  form's own `addMatBusy` keeps that group separate too — plus an **Add
  materialization…** form: warehouse, `REFRESH_MODE`, optional `IMMUTABLE
  WHERE` / `WHERE` raw-SQL conditions, and dimension/metric multi-selects.
  Warehouses are fetched when the form opens (not on modal mount), while
  dimensions/metrics reuse `loadDimensions`/`loadMetrics` — the same loaders
  the adjacent Dimensions/Metrics `LazySection`s call — via a `dimsResult`/
  `metricsResult` cache, so expanding those sections first and then opening
  this form doesn't repeat the same two live `SHOW` round-trips. A failed
  fetch here sets `actionError` **and** closes the form (`setAddingMat(false)`)
  rather than leaving it rendered with dead, misleadingly-empty pickers — the
  form was just opened with nothing typed into it yet, so there's nothing to
  preserve, and the user retries with a fresh "Add materialization…" click.
  Submitting calls `BuildAddSemanticViewMaterializationSql` for the SQL, then
  `ExecDDL` to run it — the same two-step split as
  `AddDbtProjectVersionModal.tsx` — since, unlike the single-clause
  `SET`/`UNSET`/`SUSPEND`/… actions elsewhere in this modal, `ADD
  MATERIALIZATION` is a multi-part statement on par with the CREATE builders,
  so it belongs in Go (`internal/semanticview/sql.go`'s
  `BuildAddMaterializationSql`) rather than assembled as a string in this
  file; see `semanticViewMaterialization.ts`), and a **Dimensions for metric**
  lookup (`ListSemanticDimensionsForMetric`).

  `EditRow` (this file's own — not the shared component of the same name used
  elsewhere) grew a `numeric`/`min`/`max`/`precision` mode rather than a
  separate component, following the pattern of `ServicePropertiesModal.tsx`'s
  `numeric?: boolean` flag and `TaskPropertiesModal.tsx`'s `type`/`min`.
  `MAX_STALENESS` has no read path — `SHOW SEMANTIC VIEWS` / `DESCRIBE
  SEMANTIC VIEW` don't report it — so the field only ever reflects what this
  modal itself has just set, not the view's actual current value.
  `saveMaxStaleness` re-checks `!Number.isSafeInteger(n) || n <
  MIN_MAX_STALENESS` rather than trusting `InputNumber`'s `min`/`max` props
  alone — those props don't block every input path (paste, Enter before
  blur-clamp), and a large-enough pasted value serializes through
  `${n}` as JS exponential notation (`"1e+20"`), an invalid Snowflake integer
  literal — matching the floor `BuildCreateSemanticViewSql` already enforces
  in Go for the CREATE path with a client-side error instead of a raw server
  one. Materializations have the same read-path gap one level further:
  Snowflake exposes no `SHOW`/`DESCRIBE` for a view's *existing*
  materializations at all, so the action row acts on a name the user types
  rather than a row picked from a list.
- **`semanticViewMaterialization.ts`** — `qualifiedOptionsFromResult` turns a
  `SHOW SEMANTIC DIMENSIONS`/`METRICS` result into the Add Materialization
  form's multi-select options, case-insensitively reading the `table_name` /
  `name` columns (matching `internal/snowflake/result.go`'s `ColIdx`,
  guarding both with `?? []` rather than assuming a non-null `columns`/`rows`
  the way every other SHOW/DESCRIBE reader in this codebase does) and
  qualifying each option as the unquoted `table.name` — Snowflake's own `ADD
  MATERIALIZATION` example qualifies every reference, since a bare name is
  ambiguous the moment a view joins more than one logical table (the normal
  case). The reference is left unquoted; `BuildAddMaterializationSql`
  (`internal/semanticview/sql.go`) does the quoting server-side when it
  assembles the actual statement, splitting on the last dot the same way that
  file's `qualifiedIdent` does for the CREATE builder. `NewMaterialization` is
  this form's draft state — the same shape as the Go
  `semanticview.MaterializationConfig` the generated `wailsjs` binding for
  `BuildAddSemanticViewMaterializationSql` expects, so the modal can pass it
  straight through. `isMaterializationValid` mirrors
  `BuildAddMaterializationSql`'s own requirement (a name, a warehouse, and at
  least one dimension and one metric) purely to gate the Add button before a
  doomed round-trip — the Go function is what's actually authoritative.
  Covered by `semanticViewMaterialization.test.ts`; the SQL construction
  itself is covered by `internal/semanticview/sql_test.go`'s
  `TestBuildAddMaterializationSql`.

## Lifecycle

`ALTER SEMANTIC VIEW` changes the comment, tags, name, `MAX_STALENESS`, and
materializations — the definition body itself is still only changed via
`CREATE OR REPLACE`. **Rename** (context-menu) and **View Definition** / object
**comparison** are offered, because `GET_DDL` supports semantic views
(object_type `'SEMANTIC VIEW'`). Semantic views are queried with the special
`SELECT … FROM SEMANTIC_VIEW(…)` syntax, so they are not offered in the "Select
Top 1000" action.

See also: `components/mcpserver` (MCP servers can expose semantic views to Cortex
Analyst) and `components/materializedview` (another view type).
