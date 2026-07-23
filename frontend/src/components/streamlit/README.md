# components/streamlit

UI for Snowflake **STREAMLIT** objects (interactive Python data apps) in the
object browser.

## Components

- **`CreateStreamlitModal.tsx`** — `CREATE STREAMLIT` builder. Name +
  case-sensitivity, mutually-exclusive `OR REPLACE` / `IF NOT EXISTS`, a stage
  browser (the shared `components/shared/StageFilePicker`) that fills the **source
  location** (`FROM`) and **main file** when a Python file is picked, a query-warehouse
  picker (`ListWarehouses`), title, external-access integrations, and comment.
  Live SQL preview via `BuildCreateStreamlitSql`; runs through `ExecDDL`. Only
  the modern `FROM <stage location>` grammar is emitted (no legacy
  `ROOT_LOCATION`).
- **`DeployStreamlitModal.tsx`** — deploys a **local** Streamlit app folder to
  Snowflake (distinct from `CreateStreamlitModal`, which points `FROM` at files
  already in a stage). Native folder picker (`PickDirectory`) → main-file
  detection (`DetectStreamlitMainFile`: pre-fills `streamlit_app.py` / `app.py`,
  else offers the root `*.py` candidates in an `AutoComplete`) → name (+ case
  control, defaulting to the folder name), `OR REPLACE` toggle, query-warehouse
  picker (`ListWarehouses`), title, and comment. Submits via `DeployStreamlit`
  (upload → temp stage → `CREATE STREAMLIT` → drop stage, all backend). Uses the
  shared `CreateModalShell` with `lockWhileBusy` so the upload isn't orphaned by
  a mid-flight dismiss. No SQL preview — the backend builds the statement inline
  around the temporary stage it creates. An `initialLocalDir` prop opens the modal
  with a folder already selected and its main file auto-detected (the "Deploy now"
  hand-off from `NewStreamlitFromTemplateModal`). **Update-existing path:** with an
  `initialName` prop the modal runs in "redeploy" mode — the name is fixed to the
  target app and `OR REPLACE` is enforced (Streamlit snapshots files at CREATE
  time, so a plain re-upload can't refresh a running app); it re-uploads to a
  fresh temp stage and issues `CREATE OR REPLACE STREAMLIT`, consistent with
  notebook redeploy.
- **`StreamlitPreviewControl.tsx`** — a compact "Preview locally" control embedded
  in `DeployStreamlitModal`. Runs `streamlit run <main file>` in the Snowpark
  Python environment via `StartStreamlitPreview` (backend `internal/snowpark`),
  streams `snowpark:streamlit-*` events, opens the browser when the server is
  ready, and offers Stop / Open-in-browser. Stops the process on unmount / Stop.
  Surfaces the **runtime-parity caveat** (Snowflake pins Python/Streamlit versions
  and an allow-listed Anaconda set, so local ≠ Snowflake).
- **`NewStreamlitFromTemplateModal.tsx`** — scaffolds a new **local** Streamlit
  app from a [`Snowflake-Labs/snowflake-demo-streamlit`](https://github.com/Snowflake-Labs/snowflake-demo-streamlit)
  template (Apache-2.0), then the user deploys it with the local-deploy path.
  Loads the catalog via `ListStreamlitTemplates` (searchable name + description
  list; surfaces the `Degraded` fallback state as a warning), picks a destination
  folder (`PickDirectory`), and scaffolds via `CreateStreamlitFromTemplate` with
  progress/error/success states. On success it offers **Deploy now** — which
  hands the scaffolded folder to `onDeployNow`, opening `DeployStreamlitModal`
  pre-filled (`initialLocalDir`, main file auto-detected) — and **Open folder**
  via `RevealInFinder`. Shows the **required attribution** line linking to the
  source repo (`BrowserOpenURL`).
- **`StreamlitPropertiesModal.tsx`** — `GetObjectProperties("STREAMLIT", …)`
  (SHOW STREAMLITS enriched with DESCRIBE `root_location`/`main_file`). Surfaces
  the **URL endpoint** — a clickable Snowsight deep-link built from the account
  base (`GetSnowsightURL`, reused) + `#/streamlit-apps/<DB>.<SCHEMA>.<NAME>`, with
  open (`BrowserOpenURL`) and copy buttons, plus the raw `url_id` as a secondary
  line — and inline-editable **title**, **main file**, **query warehouse**, and
  **comment** via `AlterStreamlit` `SET`/`UNSET` clauses; the rest is shown in a
  generic properties table. **External access integrations** are settable at
  create time only; the properties modal does not edit them (change them via raw
  `ALTER STREAMLIT … SET/UNSET EXTERNAL_ACCESS_INTEGRATIONS` in the SQL editor).

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `STREAMLIT`): Create-Object →
Projects submenu, type-node "Create Streamlit…" and "Deploy local Streamlit…"
(opens `DeployStreamlitModal`; on success refreshes the schema's `STREAMLIT`
list via `refreshDatabaseByName`), "New Streamlit app from template…" (opens
`NewStreamlitFromTemplateModal`), object-node "Properties…" and "Redeploy from
local folder…" (opens `DeployStreamlitModal` with `initialName` set → redeploy
mode), plus DROP / RENAME.
Icon + colour live in `components/sidebar/objectIcons.tsx` (`AppstoreOutlined`,
`--icon-streamlit`). Streamlit supports `GET_DDL`, so View Definition /
comparison / rename are all available.
