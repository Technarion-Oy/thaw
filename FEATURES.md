# Thaw — Feature Overview

Thaw is a native desktop application for Snowflake — built for analysts, engineers, and administrators who need a fast, capable SQL environment beyond the Snowflake web UI.

---

## SQL Editor

- **Monaco-based editor** with full SQL syntax highlighting and rich keyboard shortcuts
- **Multi-tab editing** — open multiple files simultaneously; each tab remembers its SQL, results, and scroll position
- **Run selected text** — highlight any portion of a query and run only that part (`⌘ Enter` / `Ctrl+Enter`)
- **Multi-statement scripts** — separate statements with `;`; all statements execute sequentially on a dedicated Snowflake session so `LAST_QUERY_ID(-1)` and `RESULT_SCAN` work correctly across statements, matching Snowsight behaviour; the spinner shows **statement N of M** and the Snowflake query ID for the active statement while the script runs; the currently-executing statement is highlighted in the editor with an amber background and a gutter indicator so you always know exactly where execution is — works whether running the full buffer or a painted selection of multiple statements
- **Cancel queries** — cancel a running query at any time; Thaw issues `SYSTEM$CANCEL_QUERY` so it also stops consuming Snowflake credits
- **Query ID** — the Snowflake Query ID is shown in the spinner while running (per-statement for multi-statement scripts) and in the results status bar after completion; click the copy icon to copy it to the clipboard
- **Selection highlight** — selecting text highlights every other occurrence in the document; overview-ruler markers show occurrences in long files
- **Toggle line comment** — right-click in the editor and choose **Toggle Line Comment** to add or remove `--` on the current line or on every line in the selection
- **Font size zoom** — `⌘+` / `Ctrl++` increases the editor font size, `⌘-` / `Ctrl+-` decreases it, `⌘0` / `Ctrl+0` resets to the default
- **Hover definitions** — hover over any table or view name — including fully-qualified three-part identifiers (`DB.SCHEMA.TABLE`) — to see its DDL in a scrollable overlay tooltip; the tooltip stays open when the cursor moves into it:
  - **Copy button** — copies the full DDL to the clipboard
  - **Text selection** — paint any portion of the DDL and copy with `⌘C` / `Ctrl+C`
  - **Right-click → Copy** — right-click inside the tooltip to copy the selected text via a context menu
  - Definitions are cached per session and refreshed automatically after 60 seconds
- **SQL autocomplete** — context-aware completions:
  - `db.` → schemas in that database
  - `db.schema.` → tables, views, functions, and other objects in that schema
  - `db.schema.table.` → columns of that table or view
  - `Ctrl+Space` inside a query → columns from all tables referenced in the current `FROM`/`JOIN` clauses
  - After `ON` in a `JOIN` clause → join conditions: FK relationships listed first (sourced from `SHOW IMPORTED KEYS IN TABLE`), followed by columns that share the same name across the joined tables; works with full three-part identifiers or bare table names, with or without aliases
- **AI inline completions** — ghost-text SQL suggestions powered by OpenAI or Google AI Studios (Gemini); press `Tab` to accept
- **AI Chat** — an agentic assistant in the results area that can query your live Snowflake connection to answer questions about your data (see [AI Features](#ai-features))
- **Code Snippets** — open **Tools → Code Snippets…** in the menu bar to browse 24 curated `CREATE OR REPLACE` templates across six categories:
  - **Data Objects** — Table, View, Materialized View, Dynamic Table, Sequence
  - **Code** — Stored Procedure (Snowflake Scripting), Stored Procedure (Python), UDF (SQL), UDF (JavaScript), UDF (Python)
  - **Automation** — Task, Stream on Table, Pipe, Alert
  - **Storage** — Stage (Internal), Stage (External S3), File Format (CSV), File Format (Parquet)
  - **Governance** — Network Policy, Resource Monitor
  - **Infrastructure** — Database, Schema, Warehouse
  - Live search filters by snippet name across all categories; the first match is auto-selected; clicking **Open in New Tab** loads the SQL into a new scratch tab for review and customisation — not auto-executed
- **Unsaved-change indicator** — a `•` dot in the tab title shows unsaved work at a glance
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the insertion point
- **Split view** — right-click any tab and choose **Split with: [tab name]** to view two editors side by side; a draggable vertical divider separates them and the ratio is persisted across sessions; each editor is fully independent with its own completions, hover definitions, and editing history; close the split with the × button in the secondary editor header, via **Close split view** in the right-click menu, or by closing either of the two tabs

---

## Object Browser

- Browse all databases → schemas → tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, and pipes
- **Search** — filter objects by name across all databases and schemas in real time
- **Right-click procedures** to open a parameter dialog; clicking **Execute** generates the `CALL` statement, opens a new tab, and runs it immediately — no manual Run press needed
- **Right-click functions** (**Call Function…**) to open a parameter dialog; detects scalar vs. table functions and generates the correct SQL; clicking **Execute** opens a new tab and runs it immediately
- **View Dependencies…** (views, procedures, functions) — right-click any view, procedure, or function and choose **View Dependencies…** to open a recursive dependency tree built by parsing DDL:
  - Every referenced object (tables, views, procedures, functions) appears as a node with its kind icon, colour-coded type tag, and fully-qualified name
  - The tree is recursive — each SQL-language object's own dependencies are expanded as children, up to 8 levels deep
  - **Circular reference detection** — objects that have already appeared higher in the tree are marked with an "already shown" badge and shown as leaf nodes to prevent infinite expansion
  - **Hover for DDL** — hovering any node shows its DDL definition in a tooltip; content is fetched lazily on first hover and cached for 60 seconds
  - Tables and non-SQL objects (non-SQL procedures, external functions) are shown as leaf nodes
  - The tree is fully expanded on load; nodes can be collapsed and re-expanded manually
- **Right-click tables and views** to:
  - Select the top 1,000 rows — opens a new tab and executes immediately
  - **Time Travel Query** — drag a timeline slider to query data at any past point within the retention window
  - **Export Data** — download table data as CSV, JSON, or Parquet via a temporary Snowflake stage
  - **Import Data** — upload a local file into Snowflake; supports CSV, JSON, and Parquet; can create a new table automatically by inferring the schema
  - **Insert Full Name** — insert the fully-qualified `"DB"."SCHEMA"."OBJECT"` identifier at the cursor
  - View DDL definition inline
  - **Rename** the object
  - **Drop** the object (with confirmation)
  - **Select for Comparison** / **Compare with** — side-by-side DDL diff (see [Text Comparison](#text-comparison))
- **Right-click a database** to export its DDL, generate an ER Diagram, view dropped schemas recoverable via Time Travel, or open **Backup Sets…**
- **Right-click a schema** to view dropped tables, open **Backup Sets…**, or use the **Create Object** cascading submenu (opens left or right depending on available screen space); currently contains **Task…** to create a new Snowflake Task
- **Right-click a table** to open **Backup Sets…** (shows backup sets scoped to its schema)
- **Drag and drop** — drag any table or view into the editor to insert a `SELECT` statement with all column names listed individually
- **Empty table indicator** — table names with zero rows appear in a faded colour so unpopulated tables are immediately visible in the tree
- **Hover tooltips** — hovering any object in the tree shows its DDL definition
- **View Definition** — opens the DDL in a modal with a Copy button
- **Properties** — opens a key/value panel of object metadata populated from the relevant `SHOW` command; for tables the panel additionally provides two inline-editable sections:
  - **Table Settings** — view and edit cluster key, schema evolution, change tracking, data retention days, max data extension days, default DDL collation, and comment; booleans are toggled with a switch, numeric and text fields open an inline input with Save / Cancel; changes are applied immediately via `ALTER TABLE SET`
  - **Column Comments** — view and edit the comment on every column; each row shows the column name, its current comment (or a dash if empty), and a pencil icon to edit inline
- **Refresh** — reload the full object tree with one click
- **Time Travel / Undrop** — list dropped databases, schemas, and tables within their retention window and restore them with a single click
- **ER Diagram** — generate an Entity Relationship Diagram for any database; filter by schema, zoom, pan, and copy the Mermaid source
- **Visual ER Designer** — interactively design or modify tables: add columns, set data types, define primary and foreign keys, preview the live Mermaid diagram, then generate and apply the necessary `CREATE TABLE` / `ALTER TABLE` SQL in one step

---

## Text Comparison

Compare the DDL or content of any two database objects, files, roles, or warehouses side by side:

1. Right-click any object, file, role, or warehouse and choose **Select for Comparison**.
2. Right-click a second item (any category) and choose **Compare with: …** — the label of the first item is shown so you always know what you are comparing against.
3. A Monaco side-by-side diff view opens, showing additions and deletions highlighted inline.

- Works across categories — compare a table's DDL against a local `.sql` file, a role against a warehouse, etc.
- Both sides are fetched concurrently so the modal opens without delay.
- The diff editor respects the active light/dark theme and the configured editor font and size.
- Trailing whitespace is trimmed from both sides before diffing to avoid spurious empty-line differences.

---

## AI Features

### AI Chat

An agentic chat panel lives alongside the SQL results. The assistant has access to your live Snowflake connection and calls tools autonomously to answer questions about your data — without you having to paste schema or query results.

**Chat mode vs Agent mode** — a toggle above the input switches between:
- **Chat mode** (default) — conversational only; the assistant sees the current SQL and last query result but makes no live calls
- **Agent mode** — the assistant calls tools autonomously against the live Snowflake session and the local file system

**Tools available in Agent mode:**

| Tool | What it does |
|------|-------------|
| `get_session_context` | Returns the active role, warehouse, database, and schema |
| `list_databases` | Lists all databases accessible to the current role |
| `list_schemas` | Lists all schemas in a database |
| `list_tables` | Lists all tables and views in a schema |
| `describe_table` | Returns column names and data types |
| `run_sql` | Executes a SQL query and returns up to 50 rows |
| `list_directory` | Lists files and subdirectories in the project working directory |
| `read_file` | Reads the content of a local file (SQL scripts, configs, etc.) |
| `run_command` | Runs a shell command in the project working directory |

- **Working directory** — the assistant always knows the configured export directory so it can refer to local SQL files by path
- **Context injection** — the current SQL in the editor and the most recent query result are automatically included so the assistant has full context
- **Stop generation** — a **Stop** button appears while the assistant is thinking; clicking it immediately cancels the in-flight API request
- **Run button** — SQL code blocks in the assistant's response include a **Run** button that loads the query into the editor and executes it immediately
- **Copy button** — every message and error has a **Copy** button using the native OS clipboard

### AI Inline Completions

Ghost-text SQL suggestions appear automatically as you type in the editor. Press `Tab` to accept. Powered by OpenAI or Google AI Studios.

### Model Validation

When configuring AI, a live **model status indicator** appears next to the model selector: a green `● Model OK` confirms the model is reachable, while a red indicator shows the exact API error — so misconfigured model names are caught immediately rather than at runtime.

### Configuration

Open **AI → Configure AI…** in the menu bar to set your provider, API key, and model. The API key is stored locally with restricted file permissions (`0600`) and never transmitted anywhere other than the selected AI provider.

---

## File Management

- **Open** (`⌘O` / `Ctrl+O`) — native OS file dialog; re-activates an existing tab if the file is already open
- **Save** (`⌘S` / `Ctrl+S`) — writes back to the file's original path
- **Save As…** (`⌘⇧S` / `Ctrl+Shift+S`) — native OS save dialog; promotes a scratch tab to a named file
- **New Tab** (`⌘T` / `Ctrl+T`) — opens a blank scratch tab
- **File Browser** — browse the working directory in the sidebar; click any file to open it; auto-refreshes after a DDL export; right-click any file to **Select for Comparison** or **Compare with** another item

---

## DDL Export

- Export DDL for every database (or a specific one) as individual files, one per object
- Fully qualified object names (`db.schema.object`) in every `CREATE` statement
- Shared / imported databases (e.g. `SNOWFLAKE_SAMPLE_DATA`) are automatically skipped
- Files are organised on disk by schema and object type (tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, pipes)
- **Configurable export path format** — open **Tools → Export Path Format…** to define a custom file path template; supported placeholders: `{database}`, `{schema}`, `{object_type}`, `{object_name}`; leave blank to use the default `{database}/{schema}/{object_type}/{object_name}.sql`; a live preview shows an example path as you type; the template is persisted across sessions
- Parallel export — up to 16 databases fetched concurrently; each database uses a single `GET_DDL('DATABASE', name, true)` call for maximum throughput
- **Live progress bar** while the export runs
- **Cancel** — stop an in-progress export at any time
- Results summary shows file counts, skipped databases, and any errors

---

## Git Integration

- View git status for the working directory (staged and unstaged files)
- **Pull** — fetch and merge from the configured remote branch
- **Commit & Push** — select individual files to stage, filter by extension, enter a commit message and personal access token
- Git credentials are **never saved to disk** — the token is held in memory only for the duration of the push
- OS junk files (`.DS_Store`, `Thumbs.db`, `desktop.ini`) are automatically excluded and added to `.gitignore`

---

## Administration

- View all roles, warehouses, and users in the account from the **Administration** panel in the sidebar

### Warehouse Credit Usage

Click the bar-chart icon in the Administration panel header (always visible, even before expanding) to open the **Warehouse Credit Usage** modal — backed by `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`:

- The button is only shown to users whose current role has `SELECT` access to `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`; a zero-row probe query runs on mount and hides the button automatically for roles without access
- **Warehouse** — select a specific warehouse or *All warehouses* to aggregate across the account
- **Date range** — defaults to the last 30 days; pick any custom range and click **Apply** to refresh
- **Summary cards** — total credits used, compute credits, and cloud services credits for the selected scope
- **Stacked bar chart** — toggle between **Daily** and **Hourly** granularity with the segmented control above the chart; Compute (blue) and Cloud Services (orange) are stacked so the credit split is immediately visible; X-axis labels are angled and thinned automatically so they remain legible at any date range; built with recharts inside a responsive container
- **Hourly detail table** — one row per metering record; columns: Start Time, Warehouse, Total Credits, Compute Credits, Cloud Svc Credits; paginated at 20 rows/page
- **Collapse / Expand table** — a toggle button in the table header hides the row detail while keeping the summary cards and chart visible

### Query Activity

Click the clock icon in the Administration panel header (always visible, even before expanding) to open the **Query Activity** modal:

- **Scope** — *Current Session*, *By User*, *By Warehouse*, or *All*
  - *By User* — autocomplete dropdown from `SHOW USERS`; accepts free-typed names for users that no longer exist
  - *By Warehouse* — autocomplete dropdown from the live warehouse list; accepts free-typed names for dropped or renamed warehouses
- **Time range** — optional date/time range picker to bound the history window
- **Limit** — cap results from 1 to 10 000 (default 100)
- **Include client-generated** — toggle to include Thaw's own internal statements
- **Run** — re-fetches with the current filters; auto-runs on open with current session scope
- **Query text search** — live filter bar narrows the loaded results by query text as you type; matches are highlighted in the table and in expanded rows; row count shows `N of M rows` when a filter is active
- Results table shows status (colour-coded), query type, query preview, user, warehouse, database, start time, and duration
- Expand any row to see the full SQL and any error message
- **Load in Editor** — inserts the query into the active editor tab and closes the modal

### Backup Policies

- List all backup policies with schedule, expiry, retention lock, owner, and comment
- **Create** — full `CREATE BACKUP POLICY` support: schedule, expire after days, tags, comment, `WITH RETENTION LOCK`, and `OR REPLACE` / `IF NOT EXISTS` modifiers
- **Alter** — rename, set/unset schedule, expiry, comment, and retention lock via an action dropdown
- **Drop** — with confirmation

### Backup Sets

Right-click any **database**, **schema**, or **table** in the object browser and choose **Backup Sets…**:

- **Object-scoped listing** — backup sets shown are those that actually back up the right-clicked object: `SHOW BACKUP SETS IN DATABASE <db>` is issued and the results are post-filtered by `object_kind`, `object_name`, `object_database_name`, and `object_schema_name` — so right-clicking a table returns only backup sets covering that exact table, not all backup sets stored in that database
- **Create** — `CREATE BACKUP SET FOR DATABASE|SCHEMA|TABLE <fqn>` with optional backup policy applied after creation:
  - Backup set name is fully qualified: choose the **database** and **schema** from dropdowns (defaulting to the source object's database and schema; `INFORMATION_SCHEMA` is excluded from the schema list), then enter just the name — the full `db.schema.name` is assembled automatically
- **Alter** — rename, set/unset comment, apply/suspend/resume backup policy
- **Drop** — with confirmation
- All operations (list, add, alter, drop, restore) reference backup sets by their fully-qualified name (`"db"."schema"."name"`) to avoid schema-resolution ambiguity
- The **Name** column in the backup sets list shows the full `db.schema.name` qualified name so the storage location is always visible at a glance
- **Delete oldest backup** — each backup set row has a **Delete oldest backup** button that identifies and removes the oldest backup without a legal hold via `ALTER BACKUP SET … DELETE BACKUP IDENTIFIER '<uuid>'`; the button is automatically greyed out when the set contains no backups; counts are pre-fetched in the background when the modal opens so no row expansion is needed
- **Expand any row** to see its individual backups:
  - Backup name, status, created date, size, and comment
  - **Add Backup** — runs `ALTER BACKUP SET … ADD BACKUP`, waits for completion, then refreshes the backup list automatically; the button shows a loading spinner while the operation is in progress to prevent accidental double-submission
  - **Restore** — create a new object from a backup snapshot:
    - Object type auto-detected from the backup set
    - Requires a new name (Snowflake does not allow restoring over an existing object)
    - For **TABLE** restores: choose the target **database** and **schema** from dropdowns (defaulting to the source object's location), then enter only the new table name
    - For **DATABASE** and **SCHEMA** restores: enter the new name directly
    - Executes `CREATE <type> <new_name> FROM BACKUP SET "<set>" IDENTIFIER '<uuid>'`

### User Management

- **User Management** — search users by name, login, display name, or email; view disabled accounts at a glance
- **Create User** — dialog with all user properties and a live `CREATE USER` SQL preview
- **Edit User** — pre-populated form that generates only the `ALTER USER … SET/UNSET` statements needed for the changed fields
- **Enable / Disable / Drop** users with a single right-click action
- All user management actions are automatically hidden or greyed out when the current role lacks the required privileges

---

## Results & Export

- Query results displayed in a virtualised grid — handles large result sets smoothly
- **NULL display** — `NULL` values are rendered as a faded italic `NULL` label so they are never confused with empty strings
- **Copy from results** — right-click any cell to open a context menu with: **Copy cell value**, **Copy row (tab-separated)**, and **Copy row with headers**; all three write to the native OS clipboard so they work reliably on macOS
- **Result history** — the last 10 successful result sets are kept in memory for the session; a dropdown in the results status bar (visible after two or more runs) lets you switch between them instantly, similar to `LAST_QUERY_ID(-n)` in SQL; after a query failure the error is shown and the dropdown appears as a standalone **Previous results** picker — the last result grid is not auto-displayed so the failure is immediately obvious, but any historical result can be recalled on demand
- **Export results** — CSV (RFC 4180) and Excel (`.xlsx`) export with a native save dialog; exports always reflect whichever result is currently selected in the history dropdown
- Column sorting and horizontal scrolling

---

## Snowflake Connectivity

- Connect with account / user / password / warehouse / role
- **Auto-fill from Snowflake CLI** — reads `~/.snowflake/config.toml` and populates the connection form from any saved profile, including key-pair (`SNOWFLAKE_JWT`) profiles; authenticator values are matched case-insensitively so both `snowflake_jwt` and `SNOWFLAKE_JWT` work
- **Cancel connection** — abort an in-progress connection attempt
- **Switch role, warehouse, database, or schema** from the toolbar without disconnecting — all subsequent queries, privilege checks, and object browsing immediately reflect the new session state
- Role dropdown shows only roles the current user can actually assume
- Schema dropdown lists only schemas belonging to the currently selected database; the list resets automatically when the database is changed
- After any `USE DATABASE`, `USE SCHEMA`, `USE ROLE`, or `USE WAREHOUSE` command runs in the editor, all four toolbar dropdowns update automatically to reflect the resulting session state
- **Session state persisted across reloads** — the account · user tag and non-sensitive connection details survive a page reload via `sessionStorage`; credentials (password, passcode, private key passphrase) are never written to storage; the connected state is verified against the backend on every reload so a backend restart correctly shows ConnectModal pre-filled with the last-used parameters rather than a broken UI; the UI waits for `sessionStorage` hydration to complete before rendering, preventing a spurious ConnectModal flash on HMR page reloads
- **Session Properties** — right-click the account · user tag in the toolbar to open a **Session Properties** modal:
  - **Parameters** section — all rows from `SHOW PARAMETERS IN SESSION`; boolean parameters render as a toggle switch (saves immediately); all other parameters show a pencil button that opens an inline input with Save / Cancel; changes apply via `ALTER SESSION SET`
  - **Variables** section — all rows from `SHOW VARIABLES`; editing works identically; changes apply via `SET variable = value`
  - String-type values are automatically single-quoted in the generated SQL; booleans and numbers are passed raw
  - **Copy** button copies all parameters and variables to the clipboard

---

## Embedded Terminal

An OS shell terminal is available as a tab in the results area alongside Results and AI Chat.

- **Open** via **Terminal → New Terminal** in the menu bar (`⌘ \`` / `Ctrl+\``)
- **Shell picker** — a dropdown lists all shells from `/etc/shells`; switching shells immediately restarts the session in the selected shell
- **New** button restarts the current shell; **Kill** stops it without closing the tab; **×** closes the tab and returns to the Results tab
- The terminal opens in the configured export directory so file operations run in context
- Resizes automatically when the results pane is resized
- Full ANSI colour and cursor support via xterm.js

---

## Snowpark & Jupyter Notebooks

Open the **Snowpark** menu to set up a local Python environment and run Jupyter-style notebooks directly inside Thaw.

### Environment setup

- **Check Environment** (`Snowpark → Check Environment…`) — scans the local machine and shows the status of system Python, the selected backend (conda env or venv), `snowflake-snowpark-python`, `notebook`, `ipython-sql`, and `sqlalchemy`; offers a direct shortcut to the setup wizard when anything is missing
- **Setup Environment** (`Snowpark → Setup Environment…`) — three-step guided wizard that streams command output line-by-line into a scrollable log:
  1. Create a conda environment (`thaw_snowpark`, Python 3.12, Snowflake channel) **or** a Python venv
  2. Install `snowflake-snowpark-python` (with optional `[pandas]` extras for venv)
  3. Install `notebook`, `ipython-sql`, and `sqlalchemy`
- **Backend choice** — radio group selects **conda** or **venv**; all commands adapt accordingly
- **Python interpreter selector** (venv only) — dropdown lists every Python interpreter found on the system (`/usr/bin`, Homebrew, pyenv, etc.); duplicates are removed by resolving symlinks; the selection is saved to `config.json`
- **Apple Silicon warning** (conda only) — `CONDA_SUBDIR=osx-64` is applied automatically on Apple M-series chips to work around a known `pyOpenSSL` incompatibility; a banner explains this
- **Delete venv folder** — danger button with a confirmation dialog removes the venv directory and resets all steps
- The project directory (same path used for DDL export and the terminal) is shown for reference
- **Manage Packages** — a 4th step in the setup wizard is always accessible (via the stepper or the "Manage Packages" footer button) regardless of whether the setup steps have been run in the current session:
  - **Install** — enter any package name and press Install or hit Enter; output streams line-by-line into a log panel; the package list refreshes automatically on success
  - **Uninstall** — all installed packages are listed with their versions; click Uninstall on any row (with confirmation) to remove it; the list refreshes after removal
  - Backed by `pip list --format=json` and `pip install` / `pip uninstall -y` inside the active conda or venv environment

### Notebook tabs

- **New Notebook** (`Snowpark → New Notebook…`) — native save dialog writes a blank `nbformat v4` file and opens it as a new notebook tab
- **Open Notebook** (`Snowpark → Open Notebook…`) — file picker filtered to `.ipynb`; opens alongside SQL tabs
- Notebooks are saved as standard `.ipynb` files compatible with JupyterLab and VS Code

### Cell editor

- **Monaco editor per cell** with full syntax highlighting:
  - **Code cells** → Python syntax (keywords, builtins, decorators, strings, comments)
  - **SQL cells** → custom Snowflake SQL tokenizer (same as the main SQL editor)
  - **Markdown cells** → Markdown syntax highlighting
- Editor auto-sizes vertically to its content
- Native undo/redo (`⌘Z` / `⌘⇧Z`) and clipboard (`⌘C` / `⌘V` / `⌘X`) via Monaco and Wails native APIs
- `Shift+Enter` runs the current cell; cell kind (Code / SQL / Markdown) can be changed at any time

### Python code cells

- Cells share a **persistent Python kernel** subprocess per notebook tab — variables and imports carry across cells
- The kernel uses the `snowflake-snowpark-python` environment (conda or venv)
- Output shows stdout, stderr, and tracebacks in colour-coded blocks with a per-block copy button
- **Inline plots** — matplotlib figures (e.g. from `plt.show()`) are captured as PNG images after each cell run and rendered inline below the cell output; no separate window opens; the kernel automatically configures the `Agg` backend on startup; multiple figures per cell are each rendered in order

### SQL cells

- SQL cells execute directly against the **active Snowflake connection** — no Python kernel required
- Results render in a **sticky-header scrollable table** (up to 1 000 rows)
- DDL / DML with no result set shows "OK — N rows affected · queryID"

### Notebook management

- **Run All**, **Restart Kernel**, **Save**, **Add Cell** in the toolbar
- Per-cell controls: run, move up/down, add below, delete
- Kernel status indicator: starting spinner → "Kernel ready" → "Kernel error"

---

## UI & Theming

- **Light, Dark, and System** themes — switch via **View → Appearance**; preference is saved across sessions
- **Tools menu** — native menu bar **Tools** entry provides **Code Snippets…** and **Export Path Format…**
- **Snowpark menu** — native menu bar **Snowpark** entry provides **Check Environment…**, **Setup Environment…**, **New Notebook…**, and **Open Notebook…**
- **Resizable sidebars** — drag either sidebar edge to any width between 160 px and 600 px
- **Resizable editor/results split** — drag the horizontal divider between the SQL editor and the results pane to any ratio; position is saved across sessions
- **Drag-and-drop panel layout** — every sidebar panel (Export DDL, File Browser, Git, Object Browser, Administration) has a drag handle at its top edge; drag panels between the left and right sidebars or reorder them within a sidebar; layout is persisted across sessions
- **Reset Layout** — restore the default panel positions and editor/results split via the **Customize Layout…** dialog (accessible from the **View** menu)
- **Resizable object browser** — collapse, expand, or drag to resize the object tree panel
- Right-click context menus are always clamped inside the viewport
- Closing the app while a query is running prompts a confirmation dialog; the query is cancelled in Snowflake before exit

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘ Enter` / `Ctrl+Enter` | Run query (or selected text) |
| `Esc` | Cancel running query |
| `⌘O` / `Ctrl+O` | Open SQL file |
| `⌘S` / `Ctrl+S` | Save active file |
| `⌘⇧S` / `Ctrl+Shift+S` | Save As… |
| `⌘T` / `Ctrl+T` | New scratch tab |
| `⌘\`` / `Ctrl+\`` | Open embedded terminal |
| `⌘+` / `Ctrl++` | Increase editor font size |
| `⌘-` / `Ctrl+-` | Decrease editor font size |
| `⌘0` / `Ctrl+0` | Reset editor font size to default |

---

*Thaw is built with Go, Wails, React, Ant Design, Monaco Editor, and Ag-Grid.*
