# internal/streamlit

Builds SQL for Snowflake **STREAMLIT** objects — schema-level interactive Python
data apps.

## What it does

`BuildCreateStreamlitSql(db, schema, cfg)` renders a `CREATE STREAMLIT` statement
from a `StreamlitConfig`. Only the **modern** grammar is emitted:

```sql
CREATE [OR REPLACE] STREAMLIT [IF NOT EXISTS] <fqn>
  FROM <stage location>
  MAIN_FILE = '<relative path>'
  [QUERY_WAREHOUSE = <warehouse>]
  [EXTERNAL_ACCESS_INTEGRATIONS = ( … )]
  [TITLE = '<title>']
  [COMMENT = '…'];
```

## Types & builders

- `StreamlitConfig` — name + case sensitivity, `OrReplace`/`IfNotExists`
  (mutually exclusive), `StageLocation` (the `FROM` source), `MainFile`,
  `QueryWarehouse`, `ExternalAccessIntegrations` (comma-separated), `Title`,
  `Comment`.
- `BuildCreateStreamlitSql` — the only exported builder.

## Local-app deploy

`DeployStreamlit(ctx, client, DeployStreamlitParams)` (`deploy.go`) stands a local
app folder up as a `STREAMLIT` object: `CREATE TEMPORARY STAGE` → recursive upload
via `stage.UploadDirToStage` → `CREATE [OR REPLACE] STREAMLIT … FROM @stage
MAIN_FILE = …` via `BuildCreateStreamlitSql` → deferred `DROP STAGE`. A temporary
stage suffices because Streamlit copies files once at creation time. It lives here
(not in `internal/snowflake`) so it can reuse both `internal/stage` and the CREATE
builder — `streamlit → stage → snowflake`, no import cycle. The `deployConfig`
mapping (params + temp-stage location → `StreamlitConfig`) is unit-tested; live
coverage is `internal/integration` `TestDeployStreamlit` (`-tags integration`).

- `DetectStreamlitMainFile(dir) (MainFileResult, error)` — inspects the **root**
  of a local app folder and picks the entrypoint, preferring `streamlit_app.py`
  then `app.py`. When neither is present `MainFile` is empty and the caller picks
  from `Candidates` (all root-level `*.py` base names, sorted — always populated
  so the UI can offer overrides). Only the root is scanned (pages under `pages/`
  are not entrypoints); hidden files are skipped and the `.py` extension is
  matched case-insensitively.

## Gotchas

- **No legacy `ROOT_LOCATION`.** Snowflake's `ROOT_LOCATION = '…'` form is
  deprecated; this package only emits `FROM <stage location>`. `FROM` is a **bare
  stage-path reference** (e.g. `@db.schema.stage/dir`), not a quoted string —
  `MAIN_FILE` *is* a quoted string literal. `normalizeStagePath` guarantees a
  single leading `@`.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive; when both are set the
  builder drops `IF NOT EXISTS`.
- Mutations (`MAIN_FILE` / `QUERY_WAREHOUSE` / `TITLE` / `COMMENT` /
  `EXTERNAL_ACCESS_INTEGRATIONS` `SET`/`UNSET`, and `RENAME TO`) are issued as
  free-form `ALTER STREAMLIT` statements via `App.AlterStreamlit` in
  `internal/app/streamlit.go`, not built here.
- `GET_DDL('STREAMLIT', …)` is supported directly (single-word kind), so DDL
  export needs no normalization in `internal/snowflake`.
- `SHOW STREAMLITS` omits `root_location`/`main_file`; the properties panel
  enriches them via `DESCRIBE STREAMLIT` (see `internal/objects`).
