# internal/tag

> SQL builder for Snowflake TAG objects.

## Responsibility

Builds the `CREATE TAG` DDL from a structured config. Tags are part of
Snowflake's governance framework — named metadata that is attached to other
objects and columns for classification, lineage, and policy enforcement. The
lifecycle / edit commands (`RENAME TO`, `SET`/`UNSET COMMENT`,
`ADD`/`DROP`/`UNSET ALLOWED_VALUES`, `SET`/`UNSET MASKING POLICY`) are simple
enough that they are issued as free-form `ALTER TAG <fqn> <clause>` statements
directly from `internal/app/tag.go` (`App.AlterTag`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `TagConfig`, `BuildCreateTagSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `TagConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `AllowedValues` (optional whitelist), `Propagate` (tag-lineage mode) + `OnConflict` (conflict resolution), and comment |
| `BuildCreateTagSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] TAG [IF NOT EXISTS] <fqn> [ALLOWED_VALUES 'v1', …] [PROPAGATE = <mode> [ON_CONFLICT = {'…' \| ALLOWED_VALUES_SEQUENCE}]] [COMMENT='…'];` — optional clauses emitted only when set, in documented order |
| `AllowedValuesSequence` | The `ON_CONFLICT` sentinel emitted as the bare keyword `ALLOWED_VALUES_SEQUENCE` (rather than a quoted string) to resolve conflicts by allowed-values order |

## Patterns & integration

- An empty name emits the placeholder `tag_name` so the live SQL preview reads as
  a completable template.
- `PROPAGATE` is only emitted for a valid mode (case-normalized); `ON_CONFLICT`
  is nested inside it and never emitted on its own.
- `App.BuildCreateTagSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterTag` (in `internal/app/tag.go`) runs the edit clauses and
  `App.GetTagReferences` lists where a tag is applied.
- Discovery: `Client.ListExtendedObjects` runs `SHOW TAGS IN SCHEMA` with the
  fixed kind `"TAG"`. Tags are not surfaced by `SHOW OBJECTS`, so — unlike
  dynamic / external tables and materialized views — no dedupe pass is needed.
- DDL export: tags are retrieved via the `GET_DDL` object type `TAG`
  (`buildGetDDLQuery` needs no special-casing — the SHOW kind already matches the
  `GET_DDL` object type).
- Properties panel: `internal/objects` runs `SHOW TAGS LIKE …` for the `TAG`
  kind; the panel also surfaces `ALLOWED_VALUES` and the tag's references.

## Gotchas

- **ALLOWED_VALUES is a value whitelist, not a column** — a tag with no allowed
  values accepts any string; supplying values restricts what may be assigned when
  the tag is applied. Blank entries are skipped so a stray empty input row never
  emits `''` as a permitted value.
- **References require ACCOUNT_USAGE** — `App.GetTagReferences` queries
  `SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES`, which needs governance privileges and
  has propagation latency (newly-applied tags may take time to appear).
