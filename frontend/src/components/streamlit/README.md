# components/streamlit

UI for Snowflake **STREAMLIT** objects (interactive Python data apps) in the
object browser.

## Components

- **`CreateStreamlitModal.tsx`** — `CREATE STREAMLIT` builder. Name +
  case-sensitivity, mutually-exclusive `OR REPLACE` / `IF NOT EXISTS`, a stage
  browser (reused `service/StageFilePicker`) that fills the **source location**
  (`FROM`) and **main file** when a Python file is picked, a query-warehouse
  picker (`ListWarehouses`), title, external-access integrations, and comment.
  Live SQL preview via `BuildCreateStreamlitSql`; runs through `ExecDDL`. Only
  the modern `FROM <stage location>` grammar is emitted (no legacy
  `ROOT_LOCATION`).
- **`StreamlitPropertiesModal.tsx`** — `GetObjectProperties("STREAMLIT", …)`
  (SHOW STREAMLITS enriched with DESCRIBE `root_location`/`main_file`). Surfaces
  the **URL endpoint** — a clickable Snowsight deep-link built from the account
  base (`GetSnowsightURL`, reused) + `#/streamlit-apps/<DB>.<SCHEMA>.<NAME>`, with
  open (`BrowserOpenURL`) and copy buttons, plus the raw `url_id` as a secondary
  line — and inline-editable **title**, **main file**, **query warehouse**, and
  **comment** via `AlterStreamlit` `SET`/`UNSET` clauses; the rest is shown in a
  generic properties table.

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `STREAMLIT`): Create-Object →
Projects submenu, type-node "Create Streamlit…", object-node "Properties…", plus
DROP / RENAME. Icon + colour live in `components/sidebar/objectIcons.tsx`
(`AppstoreOutlined`, `--icon-streamlit`). Streamlit supports `GET_DDL`, so View
Definition / comparison / rename are all available.
