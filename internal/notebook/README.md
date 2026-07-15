# internal/notebook

Builds SQL for Snowflake **NOTEBOOK** objects — schema-level notebooks that run
cells of Python/SQL/Scala on a warehouse.

## What it does

`BuildCreateNotebookSql(db, schema, cfg)` renders a `CREATE NOTEBOOK` statement
from a `NotebookConfig`:

```sql
CREATE [OR REPLACE] NOTEBOOK [IF NOT EXISTS] <fqn>
  [FROM '<source location>']
  [MAIN_FILE = '<relative path>']
  [COMMENT = '…']
  [QUERY_WAREHOUSE = <warehouse>];
```

The builder backs the **Create Object → Projects → Notebook…** schema
context-menu form (`frontend/src/components/notebook/CreateNotebookModal.tsx`),
which lets a user create an empty notebook or one seeded from staged `.ipynb`
files.

## Types & builders

- `NotebookConfig` — name + case sensitivity, `OrReplace`/`IfNotExists`
  (mutually exclusive), `SourceLocation` (the `FROM` source), `MainFile`,
  `QueryWarehouse`, `Comment`.
- `BuildCreateNotebookSql` — the only exported builder.

## Gotchas

- **No "main language" clause.** `CREATE NOTEBOOK` has no DDL parameter for the
  notebook's default cell language (Python/SQL/Scala) — that lives in the
  notebook file's metadata, not the DDL — so the builder emits none.
- **`FROM` is a quoted string literal** (e.g. `FROM '@db.schema.stage/dir'`),
  unlike `CREATE STREAMLIT`'s bare `FROM @stage` reference. `MAIN_FILE` is also a
  quoted literal. When `SourceLocation` is empty an empty notebook is created and
  neither clause is emitted.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive; when both are set
  `snowflake.CreateClause` drops `IF NOT EXISTS`.
- This package builds only the schema-menu **create-from-scratch / from-stage**
  path. Deploying a *local* `.ipynb` (uploading its bytes to a temp stage, then
  `CREATE NOTEBOOK … FROM` it) is a separate flow in
  `internal/snowflake` (`Client.DeployNotebook`) driven by the notebook editor's
  Deploy action — it is not built here.
- `GET_DDL('NOTEBOOK', …)` is supported directly (single-word kind), so DDL
  export needs no normalization.
