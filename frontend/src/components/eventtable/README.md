# components/eventtable

UI for Snowflake **EVENT TABLE** objects (special tables with a fixed,
predefined schema that capture telemetry — logs, traces, metrics — from UDFs,
stored procedures, and Snowpark Container Services) in the object browser.

## Components

- **`CreateEventTableModal.tsx`** — `CREATE EVENT TABLE` builder. Name +
  case-sensitivity, mutually-exclusive `OR REPLACE` / `IF NOT EXISTS`, a
  top-level comment, and the supported table-level properties (under **Advanced
  options**): `CLUSTER BY` (on the predefined columns),
  `DATA_RETENTION_TIME_IN_DAYS`, `MAX_DATA_EXTENSION_TIME_IN_DAYS`,
  `CHANGE_TRACKING`, `DEFAULT_DDL_COLLATION` (searchable dropdown sourced from
  `GetCollations` / `internal/snowflake`), `COPY GRANTS`, and tags (`TagInput`).
  **No column editor** — event tables have a fixed schema. Live SQL preview via
  `BuildCreateEventTableSql`; runs through `ExecDDL`.
- **`EventTablePropertiesModal.tsx`** — `GetObjectProperties("EVENT TABLE", …)`
  (SHOW EVENT TABLES) for the overview (owner / created-on) plus
  `GetEventTableParameters` (`SHOW PARAMETERS IN TABLE`) for the configurable
  parameters that `SHOW EVENT TABLES` omits. Editable **Settings** routed through
  `AlterEventTable`: inline **comment** (`SET`/`UNSET COMMENT`), a **change
  tracking** toggle (`SET CHANGE_TRACKING = TRUE|FALSE`), **data retention** and
  **max data extension** day counts (`SET`/`UNSET …_TIME_IN_DAYS`, integer
  validated), and **search optimization** add/drop buttons (`ADD`/`DROP SEARCH
  OPTIMIZATION`). Row access policies, tags, contacts, and clustering keys are
  left to the SQL editor. The remaining columns render in a generic properties
  table.

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `EVENT TABLE`): Create-Object
→ Tables & Views submenu, type-node "Create Event Table…", object-node
"Properties…", plus DROP / RENAME. Icon + colour live in
`components/sidebar/objectIcons.tsx` (`AuditOutlined`, `--icon-eventtable`).
Event tables use the plain `TABLE` grammar for `ALTER` / `DROP` / `RENAME`,
support `GET_DDL` (via the dedicated `EVENT_TABLE` type), and are queryable, so
View Definition / comparison / rename / Select Top 1000 are all available.
