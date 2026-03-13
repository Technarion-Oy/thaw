# Thaw — Snowflake Manager

A desktop application for Snowflake management: browsing objects, running SQL queries, exporting DDL to a git repository, and pushing changes via CI/CD workflows.

**Stack:** Go · Wails v2 · React · Ant Design · Monaco Editor · Ag-Grid

---

## Features

### Snowflake connectivity
- Connect with account / user / password / warehouse / role
- Auto-fill connection form from `~/.snowflake/config.toml` (Snowflake CLI profiles), including key-pair (`SNOWFLAKE_JWT`) profiles; authenticator values are matched case-insensitively
- Cancel an in-progress connection attempt
- Switch role or warehouse from the query toolbar without reconnecting
- Role dropdown shows only roles the current user can actually `USE ROLE` to — not all account-visible roles

### SQL editor
- Monaco editor with full SQL syntax highlighting
- Multi-tab editing — each open file gets its own tab; tabs restore their SQL, results and error state when switched back to
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the drop position
- **Split view** — right-click any tab and choose **Split with: [tab name]** to view two editors side by side; a draggable vertical divider separates them and the ratio is persisted across sessions; each editor is fully independent with its own completions, hover definitions, and editing history; close the split with the × button in the secondary editor header, via **Close split view** in the right-click menu, or by closing either of the two tabs
- Unsaved changes shown with a `•` prefix in the tab title
- Run the full query or just the selected text (`⌘ Enter` / `Ctrl Enter`)
- **Multi-statement scripts** — separate statements with `;`; all statements execute sequentially on the same Snowflake session so `LAST_QUERY_ID(-1)` and `RESULT_SCAN` work correctly across statements, matching Snowsight behaviour
- **Cancel query** — while a query is running the Run button becomes a **Cancel** button; pressing it (or `Esc`) cancels client-side polling *and* issues `SYSTEM$CANCEL_QUERY` so the query stops consuming credits in Snowflake
- **Query ID** — the Snowflake query ID is shown in the loading spinner while the query runs and in the results status bar after it completes; click the copy icon to copy it to the clipboard
- Query SQL, results, tab state, and the active connection (account · user tag) survive Vite / WebView page reloads (persisted to `sessionStorage`; credentials are never stored); the connection state is verified against the backend on every reload so a backend restart shows ConnectModal immediately rather than a broken UI; the UI waits for the persisted state to hydrate before rendering, eliminating the brief ConnectModal flash that occurred on HMR reloads
- **Selection highlight** — selecting any text highlights every other occurrence in the document with a blue background; overview-ruler markers make occurrences visible in long files
- Word-under-cursor highlight when nothing is selected
- **Toggle line comment** — right-click in the editor and choose **Toggle Line Comment** to add or remove `--` on the current line or every line in the selection
- **Font size zoom** — `⌘+` / `Ctrl++` increases the editor font size, `⌘-` / `Ctrl+-` decreases it, `⌘0` / `Ctrl+0` resets to the default; uses the printed character so shortcuts work correctly on non-US keyboard layouts
- **Hover definition** — hovering over a table or view name shows its DDL in a custom scrollable overlay tooltip; the tooltip stays open when the cursor moves into it; entries are cached and automatically refreshed after 60 seconds so stale definitions are never shown indefinitely:
  - **Copy button** — copies the full DDL to the clipboard
  - **Text selection** — paint to select any portion of the DDL, then copy with `⌘C` / `Ctrl+C`
  - **Right-click → Copy** — right-clicking inside the tooltip opens a context menu; choosing Copy copies the selected text to the clipboard
- **SQL autocomplete** — context-aware completions triggered by `.` or `Ctrl+Space`:
  - After `db.` → schemas of that database
  - After `db.schema.` → objects (tables, views, functions, …) in that schema
  - After `db.schema.table.` or `schema.table.` or `table.` → columns of that table/view
  - `Ctrl+Space` anywhere in a query (SELECT list, WHERE clause, etc.) → columns from all tables/views referenced in the `FROM`/`JOIN` clauses of the current statement; both quoted (`"TABLE"`) and unquoted identifiers are recognised; works above the FROM clause (e.g. inside the SELECT column list)
  - Column lists are fetched once via `DESCRIBE TABLE` and cached for the session; subsequent invocations are instant
- **AI inline completions** — ghost-text SQL suggestions powered by OpenAI or Google AI Studios (Gemini); appears automatically as you type and is accepted with `Tab`; configure via **AI → Configure AI…** in the menu bar
- **AI Chat** — agentic chat panel in the results area (Results / AI Chat / Terminal tabs); the assistant operates in **Chat** or **Agent** mode (toggle above the input); in agent mode it calls tools against the live Snowflake connection and the local file system — see [AI Chat](#ai-chat) below
- **Code Snippets** — open **Tools → Code Snippets…** in the menu bar to browse 24 curated `CREATE OR REPLACE` templates across six categories (Data Objects, Code, Automation, Storage, Governance, Infrastructure); live search filters by name; selecting a snippet shows a read-only preview; clicking **Open in New Tab** loads the SQL into a new scratch tab for review and customisation before running
- Results displayed in a virtualised Ag-Grid table
- **NULL display** — `NULL` values are rendered as a faded italic `NULL` label so they are never confused with empty strings
- **Copy from results** — right-click any cell to open a context menu with three options: **Copy cell value**, **Copy row (tab-separated)**, and **Copy row with headers**; all three write to the native OS clipboard via the Wails runtime so they work reliably on macOS (WKWebView suppresses standard browser clipboard access)
- **Result history** — the last 10 successful result sets are kept in memory; a dropdown in the results status bar lets you switch between them (analogous to `LAST_QUERY_ID(-n)`); after a query failure the dropdown becomes a standalone **Previous results** picker — the grid is hidden until a result is explicitly selected, keeping the error visible and unambiguous
- **Export results** — CSV and Excel (`.xlsx`) export buttons in the results status bar; CSV uses RFC 4180 quoting; Excel uses SheetJS to produce a native `.xlsx` file; both open a native save dialog with format-appropriate file filters; exports reflect whichever historical result is currently selected

### Embedded terminal

- **Terminal tab** appears in the results area alongside Results and AI Chat; open via **Terminal → New Terminal** (`⌘ \`` / `Ctrl+\``) in the menu bar
- **Shell picker** — a dropdown lists every shell from `/etc/shells`; switching shells immediately restarts the session in the chosen binary
- **New** button restarts the current shell; **Kill** stops it without closing the tab; **×** closes the tab and returns to Results
- The terminal opens in the configured export / working directory by default
- Resizes automatically when the results pane is resized (ResizeObserver → `FitAddon`)
- Full ANSI colour, cursor blink, and mouse support via xterm.js (`@xterm/xterm`, `@xterm/addon-fit`)
- PTY managed by the Go backend via `github.com/creack/pty`

### File management
- **Open…** (`⌘O` / `Ctrl+O`) — native OS open-file dialog filtered to `.sql`; re-activates an existing tab if the file is already open
- **Save** (`⌘S` / `Ctrl+S`) — writes back to the file's original path
- **Save As…** (`⌘⇧S` / `Ctrl+Shift+S`) — native OS save dialog with `.sql` filter; also promotes a scratch tab to a named file tab
- **New Tab** (`⌘T` / `Ctrl+T`) — opens a blank scratch tab
- All four actions are available in the **File** menu in the macOS/Windows menu bar as well as in the toolbar

### Object browser (sidebar)
- Browse databases → schemas → objects (tables, views, functions, procedures, …)
- **Filter objects** — type in the search box at the top of the sidebar to filter objects by name across all databases and schemas; the tree cascade-loads all schemas and objects automatically and collapses back to the database list when the search is cleared
- **Refresh** button (`↺`) in the sidebar header reloads the entire database tree from Snowflake
- Right-click a **database** to refresh, export its DDL, **insert its name** at the editor cursor, generate an **ER Diagram**, **Show Dropped Schemas…**, or open **Backup Sets…** — lists schemas recoverable via Time Travel with an **Undrop** button for each
- **Dropped Databases** button (`⏪`) in the sidebar header lists databases within their Time Travel retention window; click **Undrop** to restore any of them
- Right-click a **schema** to browse dropped tables recoverable via Snowflake Time Travel, **insert its fully-qualified name** at the editor cursor, open the **Create Object** cascading submenu, or open **Backup Sets…**; the **Create Object** submenu currently contains **Task…** — opens a dialog to configure and generate a `CREATE OR REPLACE TASK` statement with:
  - Compute: warehouse (searchable dropdown) or serverless with initial warehouse size
  - Schedule: none, fixed interval (seconds/minutes/hours), or cron expression with timezone
  - Dependencies: predecessor tasks (AFTER), boolean condition (WHEN)
  - Execution: allow overlapping, timeout, suspend-after-failures, auto-retry attempts
  - Integrations: error and success notification integrations (searchable dropdowns populated from `SHOW NOTIFICATION INTEGRATIONS`; default is none)
  - Other: comment, finalize task
  - SQL body (AS); live `CREATE TASK` preview updates as you type
- Right-click an **object** to:
  - Select the top 1 000 rows (tables and views) — opens in a new tab
  - **Time Travel Query…** (tables) — opens a dialog with a timeline slider spanning the table's full retention window; drag to choose a point in time and run `SELECT … AT(TIMESTAMP => …) LIMIT 1000` in a new tab
  - **Export Data…** (tables) — export table data to the local machine via a temporary internal Snowflake stage; choose format (CSV, JSON, PARQUET), compression, delimiter, header row, and output directory; the stage is dropped automatically after the download
  - **Import Data…** (tables) — import a local file into a Snowflake table via a temporary internal stage; choose format (CSV, JSON, PARQUET) with format-specific options; the file picker filters to the selected format's extensions automatically; supports two modes:
    - **Import into existing table** — optionally truncate before loading (overwrite mode)
    - **Create new table from data** — derives the schema from the file using `INFER_SCHEMA` (CSV with headers and PARQUET) or creates a `VARIANT` column table (JSON); the object browser refreshes automatically on success
  - Call the procedure with auto-generated parameter fields (procedures) — opens a parameter dialog; clicking **Execute** opens a new tab with the generated `CALL` statement and runs it immediately
  - **Call Function…** (functions) — opens a parameter dialog with auto-generated fields; detects scalar vs. table functions from the DDL and generates the correct SQL (`SELECT func(args) AS result` or `SELECT * FROM TABLE(func(args))`); clicking **Execute** opens a new tab and runs it immediately
  - **View Dependencies…** (views, procedures, functions) — opens a modal with a fully recursive dependency tree built by parsing DDL — no dynamic SQL or Snowflake lineage service required; each node shows the object kind (icon + colour-coded tag), fully-qualified name, and optional error/circular badges; hover any node to see its DDL in a tooltip (fetched lazily, cached for 60 seconds); circular references are detected automatically and labelled "already shown" to prevent infinite expansion; SQL-language objects are expanded recursively up to 8 levels deep; tables and non-SQL objects are shown as leaf nodes; the tree is fully expanded on load and can be collapsed/expanded manually
  - **Insert Full Name** — inserts the fully-qualified `"DB"."SCHEMA"."NAME"` at the current editor cursor position
  - View the DDL definition inline
  - **Rename** the object (`ALTER … RENAME TO`) — available for tables, views, sequences, stages, streams, tasks, file formats, and pipes
  - **Delete** the object (`DROP …`) — with a confirmation dialog
- **Drag and drop** — drag any table or view node from the sidebar into the editor to insert a fully-qualified `SELECT` with all column names (fetched from Snowflake and listed individually, not `*`) at the drop position; drag a user from the User Management panel to insert a `CREATE USER` DDL statement
- **Empty table indicator** — table names with zero rows are shown in a faded colour in the object tree, making it easy to spot unpopulated tables at a glance
- **Hover tooltip** — hovering over any object in the tree shows its DDL definition; cached with a 60-second TTL so changes made outside the app are visible promptly
- **View Definition** — right-click any object → **View Definition** opens a modal with the full DDL; a **Copy** button copies the SQL to the clipboard
- **Properties** — right-click any database, schema, or object → **Properties** opens a key/value panel populated by the corresponding `SHOW` command; a **Copy** button copies all rows as `property: value` lines; for **tables** the panel includes two additional inline-editable sections:
  - **Table Settings** — cluster key, schema evolution, change tracking, data retention days, max data extension days, default DDL collation, and comment; booleans are toggled with a switch, other fields open an inline input with Save / Cancel; changes apply via `ALTER TABLE SET`
  - **Column Comments** — lists every column with its current comment; click the pencil icon to edit inline; saving runs `ALTER TABLE … MODIFY COLUMN … COMMENT`
- **Text Comparison** — right-click any object, role, warehouse, or file → **Select for Comparison**; then right-click a second item → **Compare with: …** to open a Monaco side-by-side diff view; works across categories (e.g. compare a table DDL against a local `.sql` file); both sides are fetched concurrently and trailing whitespace is trimmed before diffing
- Tree automatically refreshes the affected database after any rename, drop, or undrop operation
- **ER Diagram** — right-click a database and choose **ER Diagram…** to generate an Entity Relationship Diagram from `INFORMATION_SCHEMA.COLUMNS`, `SHOW PRIMARY KEYS`, and `SHOW IMPORTED KEYS`; only base tables are shown (views excluded); filter visible schemas with checkboxes, zoom in/out, drag to pan, and copy the Mermaid source to the clipboard
- **Visual ER Designer** — click **Design Tables…** in the ER Diagram toolbar to open an interactive designer at the database level:
  - Pre-populated with all existing base tables and their columns, data types, primary keys, and foreign keys
  - Add new tables or edit existing ones; each table has its own schema selector to support cross-schema designs
  - Define columns with name, data type (NUMBER, VARCHAR, BOOLEAN, DATE, TIMESTAMP_NTZ, TIMESTAMP_LTZ, FLOAT, VARIANT, ARRAY, OBJECT), Primary Key, and Not Null flags
  - Set Foreign Key references across any table in any schema; FK arrows appear in the live preview automatically
  - Resizable left panel (drag the divider) for comfortable editing alongside the live preview
  - Live Mermaid ER diagram preview (300 ms debounce) with zoom and drag-to-pan
  - **Review & Apply Changes** — diffs the current diagram against the existing Snowflake schema and generates only the necessary SQL: `DROP TABLE` for removed tables, `CREATE TABLE` for new ones, and `ALTER TABLE` statements for column additions/removals, type changes, nullability changes, and PK/FK updates; the sidebar refreshes automatically on success
  - Closing the designer with unapplied changes prompts a confirmation dialog to prevent accidental data loss

### Administration panel

The **Administration** collapsible panel in the sidebar shows roles, warehouses, and users. It lazy-loads on first expand.

#### Warehouse Credit Usage

Click the bar-chart icon in the Administration panel header (always visible, even before expanding) to open the **Warehouse Credit Usage** modal — backed by `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`:

- The button is only shown to users whose current role has `SELECT` access to `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`; a zero-row probe query is run on mount and the button is hidden automatically for roles without access
- **Warehouse** — select a specific warehouse or leave as *All warehouses* to aggregate across all
- **Date range** — pick any start/end date; defaults to the last 30 days
- **Apply** — re-fetches with the current filters; the modal also auto-fetches on open
- **Summary cards** — total credits used, compute credits, and cloud services credits across the selected filters
- **Stacked bar chart** — toggle between **Daily** and **Hourly** granularity with a segmented control above the chart; stacked bars show Compute (blue) and Cloud Services (orange) separately so the credit split is immediately visible; X-axis labels are angled and thinned automatically so they remain legible at any date range; built with recharts inside a responsive container
- **Hourly detail table** — one row per hourly metering record; columns: Start Time, Warehouse, Total Credits, Compute Credits, Cloud Svc Credits (all credit values shown to 4 decimal places); paginated at 20 rows/page
- **Collapse / Expand table** — a button in the table header hides the detail rows while keeping the summary cards and chart visible; useful when the chart is all you need

#### Query Activity

Click the clock icon (⏱) in the Administration panel header to open the **Query Activity** modal — available even before expanding the panel:

- **Scope** — filter by *Current Session*, *By User*, *By Warehouse*, or *All*
  - **By User** — autocomplete dropdown populated from `SHOW USERS`; accepts free-typed names for users that no longer exist
  - **By Warehouse** — autocomplete dropdown populated from the live warehouse list; accepts free-typed names for dropped/renamed warehouses
- **Time range** — optional date/time range picker (`END_TIME_RANGE_START` / `END_TIME_RANGE_END`)
- **Limit** — result row cap (1 – 10 000, default 100)
- **Include client-generated** — optionally include Thaw's own internal statements
- **Run** — re-fetches with the current filter settings; the modal also auto-fetches on open using the current session scope
- Results table shows status (colour-coded tag), query type, query preview, user, warehouse, database, start time, and duration
- **Query text search** — a live filter bar above the table narrows rows by query text as you type; matches are highlighted in the preview column and in the expanded full-SQL view; the row count shows `N of M rows` when a filter is active
- Expand any row to see the full SQL with match highlighting and an **Error** message if the query failed
- **Load in Editor** — inserts the selected query into the active editor tab and closes the modal
- Backed by `SNOWFLAKE.INFORMATION_SCHEMA.QUERY_HISTORY_BY_SESSION / _BY_USER / _BY_WAREHOUSE / QUERY_HISTORY` table functions

#### User Management

- Expandable scrollable list of all users in the account, with a live **search** box that filters by username, login name, display name, and email
- **Disabled** users shown with a greyed-out `disabled` tag
- **Create user** — opens a dialog to generate and execute a `CREATE USER` statement with:
  - Username (required), masked password, identity fields (login name, display name, first/last name, email)
  - Default warehouse and role (searchable dropdowns), default namespace
  - Security options: must-change-password, days-to-expiry, create-as-disabled
  - Live `CREATE USER` SQL preview
  - Button is greyed out with a tooltip if the current role lacks the `CREATE USER` or `MANAGE GRANTS` privilege
- **Right-click a user** to:
  - **Edit…** — opens a pre-populated form to modify all user properties; generates `ALTER USER … SET / UNSET` SQL with a live preview; only changed fields are included
  - **Enable / Disable** — runs `ALTER USER … SET DISABLED = TRUE/FALSE` immediately
  - **Drop…** — confirmation dialog before `DROP USER`
  - All three actions are greyed out if the current role lacks `MANAGE GRANTS`
- **Drag a user** from the list into the editor to insert a `CREATE USER` DDL statement built from `DESCRIBE USER`
- The panel hides itself entirely if the current role cannot access `SHOW USERS`
- All content and privilege buttons **auto-refresh** when the active role is switched — no manual reload needed

#### Backup Policies

A **Backup Policies** section in the Administration panel lets you manage account-level backup policies:

- List all backup policies with schedule, expiry, retention lock status, owner, and comment
- **Create** — configure `CREATE BACKUP POLICY` with:
  - Schedule (e.g. `60 MINUTE`, `USING CRON 0 2 * * * UTC`)
  - Expire after days
  - Optional tags, comment, and `WITH RETENTION LOCK`
  - `OR REPLACE` / `IF NOT EXISTS` modifiers
- **Alter** — rename, set/unset schedule, expiry, comment, and retention lock via a dropdown action picker
- **Drop** — with a Popconfirm confirmation

#### Backup Sets

Right-click any **database**, **schema**, or **table** in the object browser and choose **Backup Sets…** to open the Backup Sets modal:

- **Object-scoped listing** — backup sets are filtered by the actual backed-up object, not just storage location: uses `SHOW BACKUP SETS IN DATABASE <db>` and post-filters by `object_kind`, `object_name`, `object_database_name`, and `object_schema_name` so only backup sets that back up the right-clicked object are shown
- **Create** — configure `CREATE BACKUP SET FOR DATABASE|SCHEMA|TABLE <fqn>`:
  - Backup set name is fully qualified: select the **database** and **schema** from dropdowns (pre-filled from the source object's location; `INFORMATION_SCHEMA` is excluded), then type only the name — the full `"db"."schema"."name"` is assembled and sent to Snowflake
  - Optional backup policy applied immediately after creation
- **Alter** — rename, set/unset comment, apply/suspend/resume backup policy
- **Drop** — with Popconfirm confirmation
- All backup-set operations (list, add, alter, drop, restore) use the fully-qualified name (`"db"."schema"."name"`) to avoid schema-resolution ambiguity regardless of the session's current schema
- The **Name** column displays the full `db.schema.name` qualified name so the storage location is always visible
- **Delete oldest backup** — each backup set row has a **Delete oldest backup** button (`−` icon) that finds and deletes the oldest backup without a legal hold via `ALTER BACKUP SET … DELETE BACKUP IDENTIFIER '<uuid>'`; the button is greyed out automatically when the set has no backups (counts are pre-loaded in the background when the modal opens, so no row expansion is required)
- **Expand** any backup set row to see its individual backups (`SHOW BACKUPS IN BACKUP SET`):
  - Columns: backup name, status (colour-coded tag), created date, size, comment
  - **Add Backup** — runs `ALTER BACKUP SET … ADD BACKUP`, waits for Snowflake to complete the operation, then refreshes the backup list automatically; the button shows a loading spinner while in progress to prevent accidental double-clicks
  - **Restore** — opens a dialog to create a new object from the selected backup:
    - Auto-detects the object type (DATABASE / SCHEMA / TABLE) from the backup set
    - Requires a new target name (Snowflake does not support restoring over an existing object)
    - For **TABLE** restores: select the target **database** and **schema** from dropdowns (pre-filled from the source object's location), then enter only the new table name
    - For **DATABASE** / **SCHEMA** restores: enter the new name directly
    - Executes `CREATE <type> <new_name> FROM BACKUP SET "<set>" IDENTIFIER '<uuid>'`

#### Role switching and session state

Role and warehouse switches (via the toolbar dropdowns) are applied to a **single persistent connection**, so every subsequent query — including user management operations, privilege checks, and all SQL editor queries — immediately reflects the new role without needing a manual refresh.

#### Session Properties

Right-click the **account · user** tag in the query toolbar to open the **Session Properties** modal:

- **Parameters** — all rows from `SHOW PARAMETERS IN SESSION`; boolean parameters render as a toggle switch (saves immediately via `ALTER SESSION SET`); other parameters show a pencil button that opens an inline input with Save / Cancel; hovering the parameter name shows its description
- **Variables** — all rows from `SHOW VARIABLES`; editing works identically; changes apply via `SET variable = value`
- String-type values are automatically single-quoted in the generated SQL; booleans and numbers are passed through raw
- **Copy** button exports all parameters and variables as `key: value` lines to the clipboard


- Export DDL for every database (or a single one) with one file per object
- Fully qualified names (`db.schema.object`) in every CREATE statement
- Shared / imported databases (e.g. `SNOWFLAKE_SAMPLE_DATA`) are automatically skipped
- Files are organised on disk by schema and object type (default layout):
  ```
  <outputDir>/<DATABASE>/
      _database.sql
      schemas/<SCHEMA>.sql
      <SCHEMA>/
          tables/
          views/
          functions/
          procedures/
          sequences/
          stages/
          streams/
          tasks/
          file_formats/
          pipes/
  ```
- **Configurable export path format** — open **Tools → Export Path Format…** in the menu bar to customise the file path template used for each exported object; supported placeholders: `{database}`, `{schema}`, `{object_type}`, `{object_name}`; a live preview shows an example path as you type; the setting is persisted to `config.json`
- Parallel fetch (up to 16 databases concurrently) and parallel atomic writes; each database is fetched with a single `GET_DDL('DATABASE', name, true)` call
- Live progress bar driven by Wails events from the Go backend
- **Cancel export** — a Cancel button appears next to the Export button while a run is in progress; cancels both the in-flight Snowflake DDL fetch and the local file writes
- Export directory can be changed directly from the Export DDL panel without opening the Git section
- Results list (per-database file counts and errors) can be collapsed/expanded with a caret button; the summary tags (total files, skipped, errors) always remain visible

### File browser
- Browse the export working directory in the sidebar
- Lazy-loads subdirectories on demand
- Click any file to open it in a new editor tab
- Auto-refreshes after a DDL export completes
- Highlights the file that matches the currently active tab

### Git integration
- View git status for the working directory (staged / unstaged files)
- **Pull** — fetch and merge from the configured remote branch
- **Commit & Push** — opens a modal where you can:
  - Select individual files to stage (with Select All / None buttons)
  - Filter files by extension (`.sql`, `.json`, …)
  - Enter a commit message and a personal-access token
- Git credentials are **never persisted to disk** — the token is used in-memory only
- OS junk files (`.DS_Store`, `Thumbs.db`, `desktop.ini`) are automatically excluded from commits and appended to `.gitignore`

### UI
- **Drag-and-drop panel layout** — every sidebar panel (Export DDL, File Browser, Git, Object Browser, Administration) has a drag handle at its top edge; drag panels between the left and right sidebars or reorder them within a sidebar; layout is persisted across sessions
- **Reset Layout** — restore default panel positions and split ratio from the **Customize Layout…** dialog
- Resizable sidebars — drag either edge to any width between 160 px and 600 px
- **Resizable editor/results split** — drag the horizontal divider between the SQL editor and the results pane; ratio is persisted across sessions
- **Object browser height** — the Objects panel is collapsible (click the label or the ▶/▼ chevron) and vertically resizable (drag the handle below the tree, 80 – 800 px); the Administration panel fills the remaining space
- **Theming** — light, dark, and system-default themes; switch via **View → Appearance** in the native menu bar; preference is persisted across sessions
- Native application menu bar with **File** (open / save / new tab), **View → Appearance** (System / Light / Dark), **AI → Configure AI…**, and **Tools** (**Code Snippets…**, **Export Path Format…**) menus
- Object browser scrolls horizontally when object names are wider than the sidebar
- Right-click context menu is always clamped inside the viewport — never overflows the screen edges
- Closing the app while a query is running shows a confirmation dialog; if confirmed, the query is cancelled in Snowflake before exit

### Logging

Thaw writes a structured, rotating log file automatically on every launch.

| Build | Path |
|---|---|
| Development (`wails dev`) | `./logs/thaw.log` (also echoed to stderr) |
| macOS production | `~/Library/Logs/thaw/thaw.log` |
| Windows production | `%APPDATA%\thaw\logs\thaw.log` |
| Linux production | `~/.local/state/thaw/thaw.log` (or `$XDG_STATE_HOME/thaw/thaw.log`) |

Log files rotate at 10 MB, keeping 5 compressed backups for up to 30 days. The Snowflake driver's own log output (connection errors, async polling) is redirected into the same file.

### Telemetry

Anonymous usage events (app started/stopped, connections, query lifecycle, feature usage) are logged at DEBUG level. No SQL content, credentials, or account identifiers are ever recorded. A remote backend placeholder is provided in `internal/telemetry/telemetry.go` for future wiring to PostHog, Segment, or Mixpanel.

### Crash reporting

Unexpected panics are caught by a deferred `crashreport.Recover()` in `main()`. On crash, a timestamped JSON file (e.g. `crash_20260303T120000Z.json`) is written alongside the rotating log files. A remote delivery placeholder is provided in `internal/crashreport/crashreport.go` for future wiring to Sentry or Bugsnag.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.22 | `brew install go` |
| Node.js | ≥ 20 | `brew install node` |
| Wails CLI | ≥ 2.9 | see below |

### Install Wails

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Verify the installation:

```bash
wails doctor
```

---

## Getting started

### 1. Install Go dependencies

```bash
go mod tidy
```

### 2. Install frontend dependencies

```bash
cd frontend && npm install && cd ..
```

### 3. Run in development mode

```bash
wails dev
```

Both the Go backend and the React frontend support hot-reload. The first run also regenerates `frontend/wailsjs/` from your Go structs — the hand-written stubs in that folder can be deleted afterwards.

### 4. Build a production binary

```bash
wails build
```

The output binary is placed in `build/bin/`.

---

## Project structure

```
thaw/
├── main.go                        # Wails entry point, window config, native menu
├── app.go                         # Methods bound to the frontend (Connect, ExecuteQuery, …)
├── errors.go                      # Sentinel errors
├── version.go                     # Version string (overridable via -ldflags at build time)
├── go.mod
├── wails.json                     # Wails project configuration
├── build/
│   ├── darwin/                    # macOS app icons
│   └── windows/                   # Windows resources
├── internal/
│   ├── ai/ai.go                   # AI provider HTTP clients (OpenAI, Google AI Studios); model listing; agentic chat loop with tool-calling
│   ├── config/config.go           # Saved git / export / AI settings
│   ├── crashreport/crashreport.go # Panic handler; writes JSON crash file; remote-send placeholder
│   ├── ddl/
│   │   ├── parser.go              # SQL statement splitter (state machine)
│   │   ├── object.go              # Metadata extraction + file-path generation
│   │   ├── exporter.go            # Parallel DDL export orchestration (cancellable)
│   │   ├── parser_test.go
│   │   └── object_test.go
│   ├── filesystem/fs.go           # Directory listing, file reading and writing
│   ├── gitrepo/repo.go            # Git status, commit/push, pull
│   ├── integration/
│   │   └── export_test.go         # End-to-end tests (require live Snowflake account)
│   ├── logger/
│   │   ├── logger.go              # slog + lumberjack setup; logrus redirect for gosnowflake
│   │   ├── path_dev.go            # Log path for dev builds (./logs/thaw.log)
│   │   └── path_prod.go           # Log path for production builds (OS-specific)
│   ├── sfconfig/reader.go         # Snowflake CLI config (~/.snowflake/config.toml)
│   ├── snowflake/client.go        # Snowflake driver wrapper
│   ├── snowflake/lineage.go       # DDL-based dependency/lineage parser (recursive, cycle-safe)
│   └── telemetry/telemetry.go     # Anonymous event tracking; remote-send placeholder
└── frontend/
    ├── index.html
    ├── vite.config.ts
    ├── package.json
    ├── src/
    │   ├── App.tsx                # Root component, Ant Design dark theme
    │   ├── main.tsx               # React entry point; suppresses WebView context menu
    │   ├── styles/global.css      # Global styles incl. Monaco occurrence-highlight class
    │   ├── store/
    │   │   ├── connectionStore.ts  # Connection state (Zustand)
    │   │   ├── diffStore.ts        # Text comparison pending item + fetch state (Zustand)
    │   │   ├── gitStore.ts         # Git / export directory state (Zustand)
    │   │   ├── objectStore.ts      # Object browser state (Zustand)
    │   │   ├── panelLayoutStore.ts # Sidebar panel order, widths, editor split (Zustand, persisted)
    │   │   ├── queryStore.ts       # Multi-tab editor state (Zustand)
    │   │   ├── sessionStore.ts     # Active role & warehouse (Zustand)
    │   │   └── themeStore.ts       # Light/dark/system theme preference (Zustand, persisted)
    │   ├── pages/
    │   │   └── QueryPage.tsx      # Main query workspace; save handlers; menu event wiring
    │   └── components/
    │       ├── connection/ConnectModal.tsx
    │       ├── editor/
    │       │   ├── monacoSetup.ts # Shared Monaco theme/language registration
    │       │   ├── SqlEditor.tsx  # Monaco editor with completions, selection highlight
    │       │   └── TabBar.tsx     # File/scratch tab strip with dirty indicator
    │       ├── export/
    │       │   ├── ExportPanel.tsx         # DDL export panel
    │       │   ├── ExportTableModal.tsx    # Table data export dialog (CSV/JSON/PARQUET)
    │       │   └── ImportTableModal.tsx    # Table data import dialog (CSV/JSON/PARQUET)
    │       ├── files/FileBrowser.tsx
    │       ├── git/
    │       │   ├── GitPanel.tsx
    │       │   └── CommitModal.tsx
    │       ├── er/
    │       │   ├── ERDiagramModal.tsx  # Read-only ER diagram viewer (from existing DB)
    │       │   ├── ERDesigner.tsx      # Visual ER schema designer (create new tables)
    │       │   └── buildMermaid.ts    # Mermaid source generator for the diagram viewer
    │       ├── account/
    │       │   ├── AccountPanel.tsx           # Administration panel: roles, warehouses, user management, backup policies
    │       │   ├── QueryHistoryModal.tsx       # Query Activity modal (INFORMATION_SCHEMA.QUERY_HISTORY_*)
    │       │   ├── WarehouseMeteringModal.tsx  # Warehouse Credit Usage modal (ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY)
    │       │   ├── UserManagementPanel.tsx     # User list, search, right-click menu
    │       │   ├── EditUserModal.tsx           # ALTER USER dialog with live SQL preview
    │       │   ├── CreateUserModal.tsx         # CREATE USER dialog with live SQL preview
    │       │   └── BackupPoliciesPanel.tsx     # Backup policies list with create/alter/drop
    │       ├── backup/
    │       │   └── BackupSetsModal.tsx     # Backup sets + nested backups with add/drop/restore
    │       ├── chat/AiChat.tsx        # AI Chat panel with tool-call display and Run/Copy buttons
    │       ├── lineage/DependenciesModal.tsx  # Recursive dependency tree modal with DDL hover tooltips
    │       ├── procedure/CallProcedureModal.tsx
    │       ├── results/ResultGrid.tsx
    │       ├── settings/
    │       │   ├── AISettingsModal.tsx    # AI provider / API key / model configuration
    │       │   └── LayoutSettingsModal.tsx
    │       ├── snippets/SnippetsModal.tsx  # Code Snippets browser (Tools menu)
    │       ├── task/CreateTaskModal.tsx    # CREATE OR REPLACE TASK dialog
    │       └── layout/
    │           ├── AppLayout.tsx  # Two-sidebar layout with drag-and-drop panel reordering and resize handles
    │           └── Sidebar.tsx    # Object browser: lazy tree, right-click actions (rename, drop, undrop, DDL)
    └── wailsjs/                   # Auto-generated Go→JS bridge (do not edit)
```

---

## Testing

Tests live alongside the production code inside each package. No external test
framework is used — only the standard `testing` package.

### Run all tests

```bash
go test ./...
```

### Run tests for a specific package

```bash
go test ./internal/ddl/...
```

### Verbose output (see each sub-test name)

```bash
go test -v ./internal/ddl/...
```

### Run a single named test

```bash
go test -v -run TestSplit ./internal/ddl/
go test -v -run TestParse_Kinds ./internal/ddl/
go test -v -run TestParseAfterSplit ./internal/ddl/
```

### Run with the race detector

```bash
go test -race ./...
```

The race detector is particularly useful for `TestNameTracker_ConcurrentSafety`,
which exercises the collision resolver under concurrent load.

### Coverage report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Integration tests

Integration tests live in `internal/integration/` and are gated behind the
`integration` build tag, so they are **never run** by `go test ./...`.  They
require a real Snowflake account.

### What they do

Each test run:

1. Connects to Snowflake using environment variables.
2. Creates a temporary database named `THAW_TEST_<random>` with two schemas
   (`ALPHA`, `BETA`) containing objects of every supported DDL type — tables,
   views, JavaScript functions (including overloads), stored procedures,
   sequences, internal stages, streams, and file formats.
3. Runs the full parallel export pipeline (`ddl.ExportDatabases`).
4. Validates the file-system output: file existence, directory structure,
   content correctness, and that function overloads land at distinct paths.
5. Drops the temporary database unconditionally, even when the test fails.

### Required environment variables

| Variable | Description |
|---|---|
| `SNOWFLAKE_ACCOUNT` | Account identifier, e.g. `myorg-myaccount` |
| `SNOWFLAKE_USER` | Login name |
| `SNOWFLAKE_PASSWORD` | Password |
| `SNOWFLAKE_WAREHOUSE` | Warehouse to use, e.g. `COMPUTE_WH` |
| `SNOWFLAKE_ROLE` | *(optional)* Role to assume |

If any required variable is missing the tests are **skipped**, not failed —
safe to run in CI environments that lack Snowflake access.

### Running the integration tests

```bash
export SNOWFLAKE_ACCOUNT=myorg-myaccount
export SNOWFLAKE_USER=my_user
export SNOWFLAKE_PASSWORD=secret
export SNOWFLAKE_WAREHOUSE=COMPUTE_WH
export SNOWFLAKE_ROLE=SYSADMIN   # optional

go test -v -tags integration -timeout 10m ./internal/integration/
```

Run a single test by name:

```bash
go test -v -tags integration -timeout 10m \
  -run TestExportDatabase \
  ./internal/integration/
```

With the race detector (recommended before merging):

```bash
go test -v -tags integration -race -timeout 10m ./internal/integration/
```

> **Note** — Snowflake DDL operations are not instant. Allow up to 10 minutes
> for a full run against an account with slow warehouse start-up. The `-timeout`
> flag above prevents the test binary from hanging indefinitely.

### Permissions required

The Snowflake user needs the following privileges (or a role that grants them):

```sql
GRANT CREATE DATABASE ON ACCOUNT TO ROLE <role>;
GRANT USAGE ON WAREHOUSE <warehouse> TO ROLE <role>;
```

All other privileges (CREATE TABLE, CREATE FUNCTION, etc.) are automatically
granted to the owner of the database created by the test.

---

## Development workflow

- **Backend changes** — edit any `.go` file; `wails dev` recompiles automatically.
- **Frontend changes** — edit files under `frontend/src/`; Vite HMR updates the UI instantly.
- **Adding a new backend method** — add the method to `app.go`, then run `wails generate module` to regenerate the JS bindings in `frontend/wailsjs/`.
- **Adding a new Go package** — place it under `internal/` and import it from `app.go`.
- **Adding a native menu item** — extend `buildMenu` in `main.go`; emit a Wails event from the callback and listen with `EventsOn` in the relevant frontend component.
- **GoDoc coverage** — every exported identifier and every significant unexported function carries a GoDoc comment; run `go doc ./...` or hover in any LSP-enabled editor to browse them.

---

## Keyboard shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘ Enter` / `Ctrl+Enter` | Run the current query (or selected text) |
| `Esc` | Cancel a running query |
| `⌘O` / `Ctrl+O` | Open a SQL file |
| `⌘S` / `Ctrl+S` | Save the active file |
| `⌘⇧S` / `Ctrl+Shift+S` | Save As… (always opens a dialog) |
| `⌘T` / `Ctrl+T` | New scratch tab |
| `⌘\`` / `Ctrl+\`` | Open embedded terminal |
| `⌘+` / `Ctrl++` | Increase editor font size |
| `⌘-` / `Ctrl+-` | Decrease editor font size |
| `⌘0` / `Ctrl+0` | Reset editor font size to default |

---

## Configuration

Git and export settings are stored at:

- **macOS** — `~/Library/Application Support/thaw/config.json`
- **Linux** — `~/.config/thaw/config.json`
- **Windows** — `%APPDATA%\thaw\config.json`

The file stores the remote URL, branch, export directory, export path template, author info, and AI provider settings (provider, model, enabled flag, and API key).
**Git tokens are never written to disk.** The AI API key is written to `config.json` with mode `0600` (owner-read-only).

Log and crash files are written to:

- **macOS** — `~/Library/Logs/thaw/`
- **Linux** — `~/.local/state/thaw/` (or `$XDG_STATE_HOME/thaw/`)
- **Windows** — `%APPDATA%\thaw\logs\`

Snowflake CLI connection profiles are read from `~/.snowflake/config.toml` and
pre-fill the connection form, but are never modified by Thaw.

### AI Chat

When AI is enabled, an **AI Chat** tab appears next to the **Results** tab in the bottom half of the query workspace.

**Chat mode vs Agent mode** — a toggle above the input switches between:
- **Chat mode** (default) — single API call, no tools; the assistant sees the current SQL and last query result but makes no live Snowflake calls
- **Agent mode** — a tool-calling loop (up to 8 iterations) that gives the assistant access to the live Snowflake session and the local file system

**Tools available in Agent mode:**

| Tool | What it does |
|------|-------------|
| `get_session_context` | Returns the active role, warehouse, database, and schema |
| `list_databases` | Lists all databases accessible to the current role |
| `list_schemas(database)` | Lists all schemas in a database |
| `list_tables(database, schema)` | Lists all tables and views in a schema (with kind) |
| `describe_table(database, schema, table)` | Returns each column's name and data type |
| `run_sql(query)` | Executes a SQL query and returns up to 50 rows |
| `list_directory(path)` | Lists files and subdirectories relative to the working directory |
| `read_file(path)` | Reads a local text file (SQL scripts, configs, …); capped at 50 000 characters |
| `run_command(command)` | Runs a shell command in the working directory; returns combined stdout/stderr |

The assistant always looks up real names before writing SQL — it will not guess database, schema, table, or column names.

**Working directory:** The assistant is told the configured export/working directory so it can reference local SQL files by path.

**Stop generation:** A **Stop** button appears while the assistant is thinking; clicking it cancels the in-flight API request immediately without showing an error.

**Context injection:** The current SQL in the editor and the most recent query result are automatically included in each turn so the assistant has full context without the user needing to paste them.

**Run button:** SQL code blocks in the assistant's response include a **Run** button that inserts the SQL into the active editor tab and executes it immediately.

**Copy button:** Every message and error has a **Copy** button that writes the text to the clipboard via the native OS clipboard API.

Chat history is preserved when switching between the Results and AI Chat tabs; it resets when the page is reloaded.

### AI inline completions

Open **AI → Configure AI…** in the menu bar to configure:

| Setting | Description |
|---------|-------------|
| **Enable AI suggestions** | Master on/off toggle |
| **Provider** | `OpenAI` or `Google AI Studios` |
| **API Key** | Stored locally in `config.json` (mode `0600`) |
| **Model** | Auto-fetched from the provider after entering a valid key; falls back to built-in defaults if the key is not yet valid |
| **Model status** | A live indicator appears below the model dropdown: `● Model OK` (green) confirms the selected model is reachable; `● <error>` (red) shows the exact API error (e.g. invalid model name) within 10 seconds of selection |

Once enabled, the Monaco editor fetches ghost-text suggestions as you type (triggered automatically after a short pause). Press `Tab` to accept a suggestion.

---

## License

Copyright © 2026 Technarion Oy. All rights reserved.

This software is proprietary and confidential. Unauthorized copying, distribution,
modification, or use — in whole or in part — is strictly prohibited without prior
written permission from Technarion Oy. Commercial use is restricted to parties
holding a valid license agreement with Technarion Oy.
