# Thaw — Snowflake Manager

A desktop application for Snowflake management: browsing objects, running SQL queries, exporting DDL to a git repository, and pushing changes via CI/CD workflows.

**Stack:** Go · Wails v2 · React · Ant Design · Monaco Editor · Ag-Grid

---

## Features

### Snowflake connectivity
- Connect with account / user / password / warehouse / role
- Auto-fill connection form from `~/.snowflake/config.toml` (Snowflake CLI profiles), including key-pair (`SNOWFLAKE_JWT`) profiles; authenticator values are matched case-insensitively
- Cancel an in-progress connection attempt
- Switch role, warehouse, database, or schema from the query toolbar without reconnecting
- Role dropdown shows only roles the current user can actually `USE ROLE` to — not all account-visible roles
- Schema dropdown lists only schemas belonging to the currently selected database; resets automatically when the database is switched
- After any `USE` command runs in the editor, all four toolbar dropdowns (role, warehouse, database, schema) update automatically to reflect the new session state
- **Current username** — the active Snowflake username (from `SELECT CURRENT_USER()`, preserving the exact case Snowflake stores) is displayed above the toolbar session selectors and above the account · user tag so the connected identity is always visible at a glance

### SQL editor
- Monaco editor with full SQL syntax highlighting
- Multi-tab editing — each open file gets its own tab; tabs restore their SQL, results and error state when switched back to
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the drop position
- **Split view** — right-click any tab and choose **Split with: [tab name]** to view two editors side by side; a draggable vertical divider separates them and the ratio is persisted across sessions; each editor is fully independent with its own completions, hover definitions, and editing history; close the split with the × button in the secondary editor header, via **Close split view** in the right-click menu, or by closing either of the two tabs
- Unsaved changes shown with a `•` prefix in the tab title
- **Close confirmation** — closing a tab with unsaved changes (via the `×` button or `⌘W` / `Ctrl+W`) shows a dialog with three choices: **Save** (saves to the existing path, or opens a Save As dialog for new unsaved files), **Close without Saving**, or **Cancel**; applies to SQL files, notebooks, and scratch tabs that have been edited
- Run the full query or just the selected text (`⌘ Enter` / `Ctrl Enter`)
- **Multi-statement scripts** — separate statements with `;`; all statements execute sequentially on the same Snowflake session so `LAST_QUERY_ID(-1)` and `RESULT_SCAN` work correctly across statements, matching Snowsight behaviour; while the script runs the spinner shows **statement N of M** and the Snowflake query ID for the active statement; the currently-executing statement is highlighted in the editor with an amber background and a gutter indicator — works whether running the full buffer or a painted selection of multiple statements
- **Cancel query** — while a query is running the Run button becomes a **Cancel** button; pressing it (or `Esc`) cancels client-side polling *and* issues `SYSTEM$CANCEL_QUERY` so the query stops consuming credits in Snowflake
- **Query ID** — the Snowflake query ID is shown in the loading spinner while the query runs (per-statement for multi-statement scripts) and in the results status bar after it completes; click the copy icon to copy it to the clipboard
- Query SQL, results, tab state, and the active connection (account · user tag) survive Vite / WebView page reloads (persisted to `localStorage`; credentials are never stored); the connection state is verified against the backend on every reload so a backend restart shows ConnectModal immediately rather than a broken UI; the UI waits for the persisted state to hydrate before rendering, eliminating the brief ConnectModal flash that occurred on HMR reloads
- **Session restoration across app restarts** — all open tabs (scratch SQL, file tabs, notebook tabs) and their SQL content are restored exactly when the app is relaunched; file-backed tabs re-read their content from disk on startup so they are always current; if a file has been deleted or moved while the app was closed the tab becomes a scratch tab (prefixed `↺`) so the last-known SQL is not lost; window size is also saved and restored on the next launch
- **Multi-cursor editing** — `⌘⌥↑` / `Ctrl+Alt+↑` adds a cursor on the line above; `⌘⌥↓` / `Ctrl+Alt+↓` adds one below; works in the SQL editor, YAML editor, and all notebook cell editors; matches VS Code behaviour
- **Selection highlight** — selecting any text highlights every other occurrence in the document with a blue background; overview-ruler markers make occurrences visible in long files
- Word-under-cursor highlight when nothing is selected
- **Toggle line comment** — `⌘/` / `Ctrl+/` (or right-click → **Toggle Line Comment**) adds or removes `--` on the current line or every line in the selection
- **Font size zoom** — `⌘+` / `Ctrl++` increases the editor font size, `⌘-` / `Ctrl+-` decreases it, `⌘0` / `Ctrl+0` resets to the default; uses the printed character so shortcuts work correctly on non-US keyboard layouts
- **Code folding** — fold arrows are always visible in the editor gutter; click to collapse or expand any SQL block — CTEs, `BEGIN…END` blocks, subqueries, and multi-line expressions; keyboard shortcut `⌘K ⌘[` / `Ctrl+K Ctrl+[` folds the current block and `⌘K ⌘]` / `Ctrl+K Ctrl+]` unfolds it
- **Hover definition** — move the cursor over any table or view name — including fully-qualified three-part identifiers (`DB.SCHEMA.TABLE`) and double-quoted identifiers (`"MY_TABLE"`, `"DB"."SCHEMA"."TABLE"`) — to see its DDL in a custom scrollable overlay tooltip; the tooltip fires as the cursor enters the token (not just when stationary at its end), stays open when the cursor moves into it, and auto-loads object metadata for schemas not yet expanded in the sidebar; entries are cached and automatically refreshed after 60 seconds so stale definitions are never shown indefinitely:
  - **Copy button** — copies the full DDL to the clipboard
  - **Text selection** — paint to select any portion of the DDL, then copy with `⌘C` / `Ctrl+C`
  - **Right-click → Copy** — right-clicking inside the tooltip opens a context menu; choosing Copy copies the selected text to the clipboard
  - **Function tooltips** — hovering over any bare function name (e.g. `DATEADD`, `COALESCE`, or a UDF) shows all overloads with their full signatures and descriptions in the same overlay; works offline from an embedded catalogue of ~320 built-in functions and is refreshed with live function metadata after each connection
- **Function call highlighting** — every function call in the editor is coloured based on what it is: built-in Snowflake functions appear in **gold** (matching the keyword colour palette) and user-defined functions appear in **teal**; highlighting is applied as you type (200 ms debounce) so it stays current without any manual refresh; the colour set is populated from the local SQLite function cache on editor mount and includes all functions from the embedded fallback catalogue plus any UDFs discovered after a live connection
- **SQL autocomplete** — context-aware completions triggered by `.` or `Ctrl+Space`:
  - After `db.` → schemas of that database
  - After `db.schema.` → objects (tables, views, functions, …) in that schema
  - After `db.schema.table.` or `schema.table.` or `table.` → columns of that table/view
  - `Ctrl+Space` anywhere in a query (SELECT list, WHERE clause, etc.) → columns from all tables/views referenced in the `FROM`/`JOIN` clauses of the current statement; both quoted (`"TABLE"`) and unquoted identifiers are recognised; works above the FROM clause (e.g. inside the SELECT column list)
  - After `ON` in a `JOIN` clause → join conditions in three tiers: **(1)** FK relationships (composite multi-column constraints produce `col1 = col1 AND col2 = col2` expressions, sourced from `SHOW IMPORTED KEYS`), **(2)** PK-naming-convention heuristic (`orders.CUSTOMER_ID = customers.ID`) when no FK constraint exists, **(3)** type-compatible same-name columns with both equality (`a.col = b.col`) and `USING (col)` alternatives; works with quoted and unquoted identifiers, full three-part names, and optional table aliases
  - **Ghost text (Trigger A)** — after `JOIN table ` (before typing `ON`), an inline ghost-text suggestion appears with the most likely `ON <condition>`; accept with `Tab`; powered by the FK cache so it's instant when data is already loaded
  - **Ctrl+Space before ON (Trigger C)** — press `Ctrl+Space` right after a JOIN table reference (before typing `ON`) to get a full dropdown of `ON <condition>` options (all three tiers: FK, heuristic, same-name)
  - Column lists are fetched once via `DESCRIBE TABLE` and cached for the session; subsequent invocations are instant
  - **Function completions** — typing any two or more characters outside a dotted context also returns matching Snowflake built-in and user-defined functions from the local cache; UDFs are sorted above built-ins so custom functions are easy to find; backed by the same SQLite store as hover tooltips, so results are instant and available offline
- **AI inline completions** — ghost-text SQL suggestions powered by OpenAI or Google AI Studios (Gemini); appears automatically as you type and is accepted with `Tab`; configure via **AI → Configure AI…** in the menu bar
- **AI Chat** — agentic chat panel in the results area (Results / AI Chat / Terminal tabs); the assistant operates in **Chat** or **Agent** mode (toggle above the input); in agent mode it calls tools against the live Snowflake connection and the local file system — see [AI Chat](#ai-chat) below
- **Code Snippets** — open **Tools → Code Snippets…** in the menu bar to browse 24 curated `CREATE OR REPLACE` templates across six categories (Data Objects, Code, Automation, Storage, Governance, Infrastructure); live search filters by name; selecting a snippet shows a read-only preview; clicking **Open in New Tab** loads the SQL into a new scratch tab for review and customisation before running
- **Function Catalog AI Chat** — open **Help → Function Catalog…** and switch to the **Ask AI** tab to chat with the AI about any selected function; the function's full signatures and descriptions are automatically injected as context so you can ask usage questions, request examples, or compare overloads without pasting anything; for built-in Snowflake functions a **📖 Snowflake documentation** link opens the official docs page in the system browser; chat history resets automatically when switching to a different function
- Results displayed in a virtualised Ag-Grid table
- **NULL display** — `NULL` values are rendered as a faded italic `NULL` label so they are never confused with empty strings
- **Copy from results** — right-click any cell to open a context menu with three options: **Copy cell value**, **Copy row (tab-separated)**, and **Copy row with headers**; all three write to the native OS clipboard via the Wails runtime so they work reliably on macOS (WKWebView suppresses standard browser clipboard access)
- **Result history** — the last 10 successful result sets are kept in memory; a dropdown in the results status bar lets you switch between them (analogous to `LAST_QUERY_ID(-n)`); after a query failure the dropdown becomes a standalone **Previous results** picker — the grid is hidden until a result is explicitly selected, keeping the error visible and unambiguous; click the **pin** icon next to any entry to keep it indefinitely (pinned entries are exempt from the 10-result cap and float to the top of the dropdown); **right-click** any entry and choose **View side by side** to split the results area horizontally — both grids scroll in sync so rows stay aligned; the compare panel's query ID and row count appear on a second line of the status bar (right-aligned); close the split with the × button
- **Export results** — CSV and Excel (`.xlsx`) export buttons in the results status bar; CSV uses RFC 4180 quoting; Excel uses SheetJS to produce a native `.xlsx` file; both open a native save dialog with format-appropriate file filters; exports reflect whichever historical result is currently selected

### Embedded terminal

- **Terminal tab** appears in the results area alongside Results and AI Chat; open via **Terminal → New Terminal** (`⌘ \`` / `Ctrl+\``) in the menu bar
- **Shell picker** — a dropdown lists every shell from `/etc/shells`; switching shells immediately restarts the session in the chosen binary
- **New** button restarts the current shell; **Kill** stops it without closing the tab; **×** closes the tab and returns to Results
- The terminal opens in the configured export / working directory by default
- Resizes automatically when the results pane is resized (ResizeObserver → `FitAddon`)
- Full ANSI colour, cursor blink, and mouse support via xterm.js (`@xterm/xterm`, `@xterm/addon-fit`)
- PTY managed by the Go backend via `github.com/creack/pty`

### Snowpark & Jupyter notebooks

Open the **Snowpark** menu to set up a local Python environment and run Jupyter-style notebooks directly inside Thaw.

#### Environment setup

- **Check Environment** — scans the local machine and reports which components are present: system Python, conda / venv, `snowflake-snowpark-python`, `notebook`, `ipython-sql`, and `sqlalchemy`; shows a "Setup Environment…" shortcut when anything is missing
- **Setup Environment** — three-step guided wizard:
  1. **Create environment** — conda (`thaw_snowpark`, Python 3.12, Snowflake channel) or venv (uses a chosen system Python)
  2. **Install Snowpark** — `snowflake-snowpark-python` (with optional `[pandas]` extras for venv)
  3. **Install Jupyter & SQL** — `notebook`, `ipython-sql`, `sqlalchemy`
- **Backend choice** — select **conda** or **venv** from a radio group; the wizard adapts all commands accordingly
- **Python interpreter selector** (venv only) — a dropdown lists every Python interpreter found on the system (`/usr/bin`, `/usr/local/bin`, `/opt/homebrew/bin`, Homebrew formula dirs, `~/.pyenv/versions/*/bin`); duplicates are removed by resolving symlinks; selection is persisted to `config.json`
- **Apple Silicon warning** (conda only) — when an Apple M-series chip is detected, the conda environment is created with `CONDA_SUBDIR=osx-64` to work around a known `pyOpenSSL` incompatibility; a warning banner explains this automatically
- **Delete venv folder** — danger button with confirmation dialog removes the venv directory and resets all steps, letting the user reinstall cleanly
- Each step streams its output line-by-line into a scrollable log panel as the command runs; errors are surfaced immediately with retry support
- The project directory (same location used for DDL export and the embedded terminal) is shown in the setup dialog for reference
- Environment and backend settings are persisted to `~/.config/thaw/config.json`
- **Manage Packages** — a 4th step in the setup wizard (always accessible via the stepper or the "Manage Packages" footer button) provides a persistent package manager for the Snowpark environment:
  - **Install** — type any package name (e.g. `scikit-learn`) and press Install or Enter; installation streams line-by-line output into a scrollable log and refreshes the package list on completion
  - **Uninstall** — every installed package is listed with its version and an Uninstall button; a confirmation dialog is shown before removal
  - The package list is loaded automatically on each visit by running `pip list --format=json` inside the active environment (conda or venv); works without completing the setup steps on return visits

#### Notebook tabs

- **New Notebook** (`Snowpark → New Notebook…`) — shows a native save dialog, writes a blank `nbformat v4` file, and opens it as a new tab
- **Open Notebook** (`Snowpark → Open Notebook…`) — file picker filtered to `.ipynb`; opens the notebook as a tab alongside SQL tabs
- **Open from Snowflake** — right-click any notebook in the object browser and choose **Open Notebook** to pull the latest version directly from Snowflake; `DESC NOTEBOOK` locates the stage URI and `GET` downloads the file to a temporary directory; the content opens in a new unsaved notebook tab (the unsaved indicator is shown so you can choose to save it locally)
- Notebook tabs are identified by an experiment icon in the tab strip
- Notebooks are saved back to disk as standard `.ipynb` files compatible with JupyterLab and VS Code

#### Cell editor

- **Monaco editor per cell** — each cell uses a Monaco editor with full syntax highlighting:
  - **Code cells** → Python (keywords, builtins, decorators, strings, comments)
  - **SQL cells** → custom Snowflake SQL tokenizer (same as the main SQL editor)
  - **Markdown cells** → Markdown syntax highlighting
- Editor auto-sizes vertically to its content — no internal scrollbar for short cells
- Native undo/redo (`⌘Z` / `⌘⇧Z`) via Monaco's built-in history
- Clipboard operations (`⌘C` / `⌘V` / `⌘X`) routed through the Wails native API to work correctly inside WKWebView
- `Shift+Enter` runs the current cell (code and SQL cells)
- Cell kind (Code / SQL / Markdown) can be changed at any time via a dropdown in the cell header

#### Python code cells

- Cells run in a **persistent Python kernel** subprocess — variables and imports are shared across all cells in the same tab
- The kernel uses `snowflake-snowpark-python` from the configured conda or venv environment
- Per-cell output shows stdout, stderr, and tracebacks in colour-coded blocks
- **Copy output** — each output block has a copy button that writes the text to the native clipboard
- **Inline plots** — matplotlib figures are captured as PNG images and rendered directly below the cell output; `plt.show()` works as expected without opening a separate window; the matplotlib `Agg` backend is configured automatically by the kernel; multiple figures per cell are supported
- **Auto-connected Snowpark session** — a Snowpark session is automatically created on kernel startup using the same account, role, warehouse, database, and schema as the active app connection; the session registers itself as the active session so `get_active_session()` (from `snowflake.snowpark.context`) works in every Python cell without any `Session.builder` boilerplate — matching the behaviour of Snowflake's native notebooks; supports password, key-pair (`snowflake_jwt`), Okta, and MFA authenticators; `externalbrowser` SSO requires manual session creation; session init errors are surfaced in the first cell's stderr rather than silently swallowed
- **Session kept in sync — bidirectional** — whenever role, warehouse, database, or schema is changed via the toolbar dropdowns, `get_active_session()` is used to apply the update to the live kernel session; switching to a notebook tab also triggers a sync; conversely, when a Python or SQL cell runs a `USE` command the change is propagated back to the main Snowflake connection pool — all four toolbar dropdowns update automatically and subsequent queries in SQL editor tabs immediately reflect the new database, schema, role, or warehouse; Python cells, SQL cells, and SQL editor tabs always see the same session state
- **DDL executes immediately** — `session.sql("USE DATABASE X")` takes effect without an explicit `.collect()` call, matching Snowflake native notebook behaviour; USE, CREATE, ALTER, DROP, TRUNCATE, COMMENT, GRANT, and REVOKE statements are auto-collected on the session instance at startup
- **Python intellisense** — powered by [Jedi](https://jedi.readthedocs.io/) running inside the live kernel subprocess, giving runtime-aware completions and documentation in every code cell:
  - **Autocomplete** — triggered by `.` or `Ctrl+Space`; completions are sourced from the kernel's live namespace so variables and objects defined in earlier cells are fully reflected (e.g. `df.` on a Pandas DataFrame shows all DataFrame methods); completion items display the kind icon (function, class, module, keyword, variable, …), the fully-qualified name as detail, and the raw docstring in a documentation popover; up to 200 items are returned per request
  - **Hover documentation** — move the cursor over any name to see its documentation tooltip; function calls show the full signature with parameter names and types first, followed by the docstring; for other names the fully-qualified name and docstring are shown; content is fetched live from the kernel on each hover

#### SQL cells

- SQL cells execute through the **Snowpark kernel session** — the same session Python cells use — so `USE` commands in SQL cells affect Python cells and vice versa, and `SELECT CURRENT_DATABASE()` always returns the same value in both cell types
- SQL is split into individual statements by a parser that correctly handles `--` line comments, `/* */` block comments, single-quoted strings, and `$$`-dollar-quoted strings; each statement runs in order and the last result is displayed
- **Run selection** — if text is selected in a SQL cell, only the selected SQL is sent for execution
- Results are rendered in a **sticky-header scrollable table** (up to 1 000 rows displayed)
- DDL / DML statements with no result set show an "OK — N rows affected" line
- `Shift+Enter` runs the SQL (or selection) and displays the result inline below the cell
- `USE DATABASE X;` in a SQL cell updates the toolbar dropdowns and the Python session automatically

#### Notebook management

- **Run All** — executes all code and SQL cells sequentially
- **Restart Kernel** — stops and relaunches the Python kernel subprocess; existing SQL cell results are preserved
- **Save** — writes the notebook to disk at its original path; the tab's unsaved-change indicator clears
- **Add Cell** — inserts a new code cell at the bottom or below a specific cell
- **Deploy** — deploys the notebook as a Snowflake Notebook object; opens a dialog with all `CREATE NOTEBOOK` options: database, schema, name, `OR REPLACE` / `IF NOT EXISTS`, comment, query warehouse (for SQL queries), Python runtime warehouse, idle auto-shutdown seconds, runtime name, and compute pool; works for both saved notebooks (uploaded from their file path) and unsaved notebooks (the current in-memory content is serialised and written to a temporary file before upload; the temp file is removed after the stage transfer)
- Per-cell controls: run, move up, move down, add below, **delete** (with confirmation dialog)
- **Command mode** — when no cell editor is focused, single-key shortcuts operate on the selected cell (the last clicked or focused cell, highlighted with an accent left border):
  - `B` — add a new code cell below
  - `A` — add a new code cell above
  - `D D` — delete the selected cell (confirmation dialog required)
  - `Y` — change cell type to Code
  - `M` — change cell type to Markdown
  - `S` — change cell type to SQL
- Kernel status indicator in the toolbar: "Starting kernel…" spinner, "Kernel ready" tag, or "Kernel error" tag

### File management
- **Open…** (`⌘O` / `Ctrl+O`) — native OS open-file dialog filtered to `.sql`, `.yml`, `.yaml`, and `.py`; opens in the configured export directory by default; re-activates an existing tab if the file is already open; the editor automatically uses YAML or Python syntax highlighting based on the file extension
- **YAML intelligence** — dbt YAML files opened in the editor receive schema-driven autocompletions, hover documentation, and real-time validation (red squiggles) powered by bundled dbt-jsonschema schemas — no network requests; covers `dbt_project.yml`, `packages.yml`, `dependencies.yml`, `selectors.yml`, and all model/source/seed/snapshot/exposure YAML files; property names, allowed values, and documentation strings appear as you type; non-dbt YAML files (`profiles.yml`, CI configs, etc.) are not falsely flagged with "Property X is not allowed" warnings
- **Save** (`⌘S` / `Ctrl+S`) — writes back to the file's original path
- **Save As…** (`⌘⇧S` / `Ctrl+Shift+S`) — native OS save dialog with `.sql` filter; also promotes a scratch tab to a named file tab
- **New Tab** (`⌘T` / `Ctrl+T`) — opens a blank scratch tab
- All four actions are available in the **File** menu in the macOS/Windows menu bar as well as in the toolbar

### Object browser (sidebar)
- Browse databases → schemas → objects (tables, views, functions, procedures, notebooks, …)
- **Filter objects** — type in the search box at the top of the sidebar to filter objects by name across all databases and schemas; the tree cascade-loads all schemas and objects automatically and collapses back to the database list when the search is cleared
- **Refresh** button (`↺`) in the sidebar header reloads the entire database tree from Snowflake
- Right-click a **database** to refresh, export its DDL, **insert its name** at the editor cursor, generate an **ER Diagram**, **Show Dropped Schemas…**, or open **Backup Sets…** — lists schemas recoverable via Time Travel with an **Undrop** button for each
- **Dropped Databases** button (`⏪`) in the sidebar header lists databases within their Time Travel retention window; click **Undrop** to restore any of them
- Right-click a **schema** to browse dropped tables recoverable via Snowflake Time Travel, **insert its fully-qualified name** at the editor cursor, **Export Data…** or **Import Data…** (opens the same export/import modals with a table selector — no need to expand the schema first), open the **Create Object** cascading submenu, or open **Backup Sets…**; the **Create Object** submenu currently contains **Task…** — opens a dialog to configure and generate a `CREATE OR REPLACE TASK` statement with:
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
  - **Import Data…** (tables) — import one or more local files into a Snowflake table via a temporary internal stage; supports CSV, JSON, AVRO, ORC, and PARQUET; all Snowflake `FORMAT_TYPE_OPTIONS` are exposed with defaults pre-filled in a collapsible panel; the file picker filters to the selected format's extensions; supports two modes:
    - **Import into existing table** — optionally truncate before loading (overwrite mode)
    - **Create new table from data** — derives the schema from the file using `INFER_SCHEMA` (CSV with headers and PARQUET) or creates a `VARIANT` column table (JSON); the object browser refreshes automatically on success
    - **File preview** (CSV and JSON) — after selecting files a preview section appears showing the first 10 rows of each file (up to 5 files); CSV preview respects the current delimiter and "Parse header" settings and updates live as options change; JSON preview offers a **Parsed** tab (tabular view of the first 10 records) and a **Raw** tab (first 4 KB of the raw text); multiple files are shown in a tabbed layout
    - **AI Suggest** (CSV and JSON, requires AI configured) — an **✨ AI Suggest** button appears in the Format options panel header; clicking it shows a confirmation dialog warning that up to 64 KB of file content will be sent to the configured AI provider and advising against use with sensitive or confidential data; confirming proceeds with the call and suggested values for delimiter, header detection, quoting, encoding, compression, and other format options are applied automatically; the panel opens to show the changes and a one-sentence AI explanation is shown below; an ⓘ info icon next to the button also discloses the data-sharing behaviour on hover
  - Call the procedure with auto-generated parameter fields (procedures) — opens a parameter dialog; clicking **Execute** opens a new tab with the generated `CALL` statement and runs it immediately
  - **Call Function…** (functions) — opens a parameter dialog with auto-generated fields; detects scalar vs. table functions from the DDL and generates the correct SQL (`SELECT func(args) AS result` or `SELECT * FROM TABLE(func(args))`); clicking **Execute** opens a new tab and runs it immediately
  - **View Dependencies…** (views, procedures, functions) — opens a modal with a fully recursive dependency tree built by parsing DDL — no dynamic SQL or Snowflake lineage service required; each node shows the object kind (icon + colour-coded tag), fully-qualified name, and optional error/circular badges; hover any node to see its DDL in a tooltip (fetched lazily, cached for 60 seconds); circular references are detected automatically and labelled "already shown" to prevent infinite expansion; SQL-language objects are expanded recursively up to 8 levels deep; tables and non-SQL objects are shown as leaf nodes; the tree is fully expanded on load and can be collapsed/expanded manually
  - **Open Notebook** (notebooks) — downloads the notebook source from Snowflake via `DESC NOTEBOOK` → `GET`, opens it in a new unsaved notebook tab; the `•` unsaved indicator is shown immediately so it's clear the file hasn't been saved locally yet
  - **Execute Notebook…** (notebooks) — opens a dialog to run `EXECUTE NOTEBOOK` with optional string parameters (each value is automatically single-quoted); the dialog fetches the notebook's current `QUERY_WAREHOUSE` via `SHOW NOTEBOOKS` and displays it read-only; if no warehouse is configured a warning alert is shown with a **Set Warehouse** button that opens a separate dialog where the warehouse can be selected from the session warehouse list and saved via `ALTER NOTEBOOK … SET QUERY_WAREHOUSE`; the Set Warehouse dialog has explicit **Save** and **Cancel** buttons and updates the execute dialog live on save; a live SQL preview shows the exact `EXECUTE NOTEBOOK` statement that will be sent
  - **Execute Task** (tasks) — triggers an immediate single run of the task via `EXECUTE TASK`; a loading indicator is shown while the command is in flight and a success or error message is displayed on completion
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

The **Administration** collapsible panel in the sidebar shows roles, warehouses, users, and Snowflake integrations. It lazy-loads on first expand.

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
- Results table shows status (colour-coded tag), query type, query preview, start time, end time, and duration
- **Query text search** — a live filter bar above the table narrows rows by query text as you type; matches are highlighted in the preview column and in the expanded full-SQL view; the row count shows `N of M rows` when a filter is active
- Expand any row to see the full SQL with match highlighting plus a detail grid with user, warehouse, database, schema, rows produced, bytes scanned, and query ID
- **Load in Editor** — inserts the selected query into the active editor tab and closes the modal
- **Copy** — copies the full query text to the clipboard; the button briefly shows "Copied!" as confirmation
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
- **Key Pair Auth…** — right-click any user and choose **Key Pair Auth…** to open the key pair authentication dialog (requires OWNERSHIP or MODIFY PROGRAMMATIC AUTHENTICATION METHODS privilege on that user; the menu item is greyed out automatically when the privilege is absent):
  - Choose a key generation method: **Go built-in crypto** (always available, no passphrase), **OpenSSL** (passphrase-encrypted private key), or **ssh-keygen** (passphrase-encrypted private key); the dropdown lists only the tools that are actually present on PATH
  - Set the private key output path (type or **Browse…** to pick a directory); the public key is saved alongside with `_pub.pem` appended
  - Optionally enter a passphrase (disabled for Go built-in)
  - Click **Generate key pair** to produce an RSA-2048 PKCS#8 PEM key pair; the private key is written with mode `0600`; the stripped public key content (no PEM header/footer) is shown for inspection
  - Click **Apply to \<username\>** to run `ALTER USER "<name>" SET RSA_PUBLIC_KEY='…'` immediately
- **Key pair auth in Create User** — the **Create User** dialog includes an **RSA public key** field and a **Generate key pair…** button; clicking the button opens the key pair dialog in "pick" mode so you can generate a key pair and use its public key without leaving the create flow
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

#### Integrations

An **Integrations** section in the Administration panel lets you browse, create, modify, and drop all six Snowflake integration types — each as a lazy-loading category in an expandable tree:

- **Storage** — S3, S3 GovCloud, GCS, and Azure Blob external stage integrations
- **API** — AWS API Gateway, AWS Private API Gateway, Azure API Management, Google API Gateway, and Git HTTPS API integrations
- **Catalog** — Glue, Object Store, Polaris, Iceberg REST, and SAP BDC catalog integrations
- **External Access** — network-rule-based external access integrations
- **Notification** — Email, Webhook, Azure Storage Queue (inbound), GCP Pub/Sub (inbound/outbound), AWS SNS (outbound), and Azure Event Grid (outbound) integrations
- **Security** — API Authentication, External OAuth, OAuth (partner and custom), SAML2, and SCIM integrations

Right-click a **category** to **Create** a new integration (the option is disabled automatically if the current role lacks `CREATE INTEGRATION`). Right-click any **integration** to:
- **Properties** — `DESCRIBE INTEGRATION` output as a read-only key/value table
- **Modify** — shows current DESCRIBE properties alongside an editable ALTER SQL textarea; click **Run** to apply
- **Drop** — with a Popconfirm confirmation that reloads the category on success

The **Create** dialog adapts its form fields dynamically based on the selected kind and subtype/provider. Cloud provider defaults (S3 / GCS / Azure for Storage; matching defaults for API) are pre-selected based on `SELECT CURRENT_REGION()` at the time the dialog opens.

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

#### Warehouse Properties

Right-click any warehouse in the Administration panel and choose **Properties** to open a dedicated editable properties modal:

- **Status bar** — shows the current state (STARTED / SUSPENDED / RESUMING / QUIESCING) as a colour-coded badge alongside the type, size, and owner; action buttons live here:
  - **Suspend** (visible when started) and **Resume** (visible when suspended) toggle the warehouse state immediately
  - **Abort All Queries** cancels all currently running queries on the warehouse (with a confirmation prompt)
  - **Rename** — opens an inline name input; the warehouse list in the sidebar updates live on save
- **Compute** — warehouse size (dropdown: X-Small → 6X-Large), warehouse type (Standard / Snowpark-Optimized); for multi-cluster warehouses: max and min cluster count, scaling policy (Standard / Economy)
- **Behavior** — auto-suspend timeout in seconds (0 = disabled), auto-resume toggle
- **Query Acceleration** — enable/disable toggle, max scale factor (0–100)
- **Resource & Timeouts** — resource monitor name, max concurrency level, statement queued timeout, statement timeout (sourced from `SHOW PARAMETERS IN WAREHOUSE`)
- **General** — comment
- **Info** — read-only: owner, created_on, resumed_on, updated_on, running/queued query counts
- All editable fields use inline pencil-click editing (text/number fields) or instant toggle switches (booleans) — each save runs the corresponding `ALTER WAREHOUSE … SET` statement immediately
- **Inline privilege errors** — if an `ALTER WAREHOUSE` operation fails (e.g. insufficient privileges), the error is shown inline below the field in red rather than silently printed to the log; toggle switches surface the error as a message toast; rename errors appear inline below the name input; the "Insufficient privileges" phrase is extracted from the full Snowflake error string for a concise, readable message

#### Role switching and session state

Role, warehouse, database, and schema switches (via the toolbar dropdowns) are applied to a **single persistent connection**, so every subsequent query — including user management operations, privilege checks, and all SQL editor queries — immediately reflects the new session state without needing a manual refresh. Running any `USE` command in the SQL editor has the same effect: all four dropdowns sync automatically when the query completes.

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

### dbt Project Scaffolding

Open **Tools → Create dbt Project…** in the menu bar to scaffold a new dbt project pre-wired to the active Snowflake connection — no dbt CLI required during generation.

A 3-step wizard guides the process:

1. **Configure** — set the project name, profile name (mirrors the project name by default, independently editable), and output directory (Browse… uses a native directory picker); Thaw warns if the target directory already exists; an **Inline view SQL definitions** toggle (off by default) embeds the actual `SELECT` body of each Snowflake view into its staging stub instead of a generic pass-through — requires one extra `GET_DDL` call per view; when enabled, Thaw also **auto-rewrites raw Snowflake identifiers** to dbt Jinja calls: three-part references (`DB.SCHEMA.TABLE`) to tables become `{{ source('db_schema', 'TABLE') }}`, references to views become `{{ ref('stg_model_name') }}`, and references to objects outside the selected schemas are left unchanged; CTE aliases are excluded from replacement; a **Use dbt variables for database names** toggle (off by default) adds a `vars:` block to `dbt_project.yml` (e.g. `db_mydb: MYDB`) and replaces hardcoded database names in `_sources.yml` with `{{ var('db_mydb', 'MYDB') }}` calls, making it trivial to retarget the project at a different database
2. **Select Sources** — expand any database to load its schemas on demand; check the schemas to include as dbt sources; `INFORMATION_SCHEMA` is listed but marked as exceptional (warning icon, excluded from "Select all") — when included, Thaw adds it to `_sources.yml` as a system schema entry without generating any staging stubs; **Cross-schema dependency hints** — when a schema is checked, Thaw silently analyses the views in that schema (via `SHOW VIEWS IN SCHEMA`) and highlights any other schemas those views reference that are not yet selected; suggested schemas appear with an amber indicator and a tooltip naming the selected schema that references them, helping ensure your dbt project includes all transitively-needed sources
3. **Generate** — shows a summary (project path, database count, schema count, estimated file count); click **Generate Project** to create all files; on success a collapsible file list is shown grouped by directory; on failure a back button returns to Step 1

**Generated file tree** under `<OutputDir>/<ProjectName>/`:
```
dbt_project.yml          # project config with profile reference, materialization defaults
profiles.yml             # pre-filled from the live session (account, user, role, warehouse, database, schema)
models/
  staging/
    _sources.yml         # one source entry per selected (database, schema)
    stg_<table>.sql      # one CTE stub per table/view
  marts/
    .gitkeep
seeds/
  .gitkeep
macros/
  .gitkeep
```

When multiple databases or schemas are selected the staging stub filenames are prefixed with `db_schema_` (e.g. `stg_mydb_public_orders.sql`) to avoid collisions.

`profiles.yml` is written to the project root for inspection. Copy it to `~/.dbt/profiles.yml` when you are ready to run dbt commands.

### Schema Migration

Open **Tools → Schema Migration…** in the menu bar to deploy local `.sql` DDL files to Snowflake with conflict detection, dependency ordering, and safety snapshots. A 5-step wizard guides the process:

1. **Configure** — add one or more source directory → target database mappings; each mapping associates a local `.sql` directory with a fallback Snowflake database used for objects that have no explicit `USE DATABASE` context; multiple mappings let you migrate several databases in a single wizard run
2. **Scan** — reads every `.sql` file in all source directories, splits multi-statement files, tracks `USE DATABASE` / `USE SCHEMA` context, applies each mapping's fallback database, and deduplicates objects across all sources by kind + name; the summary shows total counts by object type
3. **Review** — shows an Ag-Grid diff table with a status tag for each object:
   - **New** — exists locally but not in Snowflake
   - **Changed** — exists in both; DDL is normalised (comments stripped, whitespace collapsed, uppercased, trailing `;` removed) before comparison to eliminate cosmetic noise
   - **Unchanged** — identical DDL; hidden from selection by default
   - **Removed** — exists in Snowflake but not locally
   - **Monaco DiffEditor** below the grid shows local vs remote DDL for the selected row
   - **Auto-dependency selection** — when a VIEW or PROCEDURE is checked, any referenced TABLE that is also "new" or "changed" is selected automatically and a toast is shown; unchecking a TABLE that a selected VIEW depends on is blocked with an inline warning
4. **Strategy & Protect** — choose how existing TABLE objects with data are migrated, then optionally create safety snapshots:
   - **Smart In-Place** *(default)* — diffs local vs remote column definitions and issues `ALTER TABLE ADD COLUMN` / `DROP COLUMN` / `ALTER COLUMN TYPE`; no data movement; safest for compatible schema changes
   - **Blue/Green Swap** — creates a temporary table with the new schema, copies shared columns with `INSERT … SELECT`, atomically swaps the two tables with `ALTER TABLE … SWAP WITH`, then drops the temp; non-shared columns are discarded
   - **View-Based Soft Cutover** — renames the original table to `<name>_v1`, creates the new table from local DDL, and creates a compatibility view `<name>_compat` exposing the shared columns from the archived data
   - **Destructive Rebuild** — `DROP TABLE IF EXISTS` + `CREATE TABLE`; all existing data is permanently lost; shows a red warning banner when selected
   - **Empty-table shortcut** — if `SHOW TABLES` reports 0 rows for a table, the data-preserving strategies are skipped and a direct `DROP + CREATE` is used instead, regardless of the chosen strategy
   - **Open in SQL Editor** — generates a strategy-aware SQL script (ALTER TABLE statements for in-place, multi-step sequences for the others) and loads it into a new editor tab for review before running
   - **Safety snapshots** (optional, per target database): Create a Backup Set (`CREATE BACKUP SET FOR DATABASE …`) and/or a zero-copy clone database (`CREATE DATABASE … CLONE …`) for each unique target database involved in the selected objects
5. **Deploy** — executes the selected objects in dependency order (DATABASE → SCHEMA → SEQUENCE → TABLE → FILE FORMAT → STAGE → VIEW → MATERIALIZED VIEW → FUNCTION → PROCEDURE → STREAM → TASK → PIPE) with up to 5 retry passes; objects that fail with a dependency error ("does not exist" / "not authorized") are automatically retried in subsequent passes; a live progress table shows pass number, object name, and per-object status; **Cancel** stops the run cleanly

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
- Native application menu bar with **File** (open / save / new tab), **View → Appearance** (System / Light / Dark), **AI → Configure AI…**, **Tools** (**Code Snippets…**, **Export Path Format…**, **Schema Migration…**, **Create dbt Project…**), **Snowpark** (**Check Environment…**, **Setup Environment…**, **New Notebook…**, **Open Notebook…**), and **Help** (**Function Catalog…**, **Keyboard Shortcuts…**) menus
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

Log files rotate at 10 MB, keeping 5 compressed backups for up to 30 days. The Snowflake driver (gosnowflake v2) uses `slog.Default()` for its own log output, so driver messages (connection errors, async polling, etc.) appear in the application log automatically.

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
├── snowpark.go                    # Snowpark IPC: env check/setup, notebook kernel, SQL cells
├── dbt.go                         # dbt project scaffolding IPC (CreateDbtProject)
├── session.go                     # Window state persistence (WindowState, load/save)
├── session_path_dev.go            # Session file path for dev builds (./thaw-session.json)
├── session_path_prod.go           # Session file path for production builds (OS-specific)
├── errors.go                      # Sentinel errors
├── version.go                     # Version string (overridable via -ldflags at build time)
├── migration_test.go              # Unit tests for migration helper functions (no Snowflake required)
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
│   ├── dbt/
│   │   ├── generator.go           # Pure dbt project file generator (no Snowflake calls)
│   │   └── generator_test.go      # 56 unit tests for generator, source names, and SQL stubs
│   ├── filesystem/fs.go           # Directory listing, file reading and writing
│   ├── gitrepo/repo.go            # Git status, commit/push, pull
│   ├── integration/
│   │   ├── basic_test.go          # Connectivity + result-shape integration tests (key-pair auth)
│   │   ├── export_test.go         # DDL export end-to-end tests (require live Snowflake account)
│   │   └── migration_test.go      # Schema migration strategy integration tests (key-pair auth)
│   ├── logger/
│   │   ├── logger.go              # slog + lumberjack setup; sets slog.Default so gosnowflake v2 logs flow in automatically
│   │   ├── path_dev.go            # Log path for dev builds (./logs/thaw.log)
│   │   └── path_prod.go           # Log path for production builds (OS-specific)
│   ├── sfconfig/reader.go         # Snowflake CLI config (~/.snowflake/config.toml)
│   ├── snowflake/client.go        # Snowflake driver wrapper
│   ├── snowflake/lineage.go       # DDL-based dependency/lineage parser (recursive, cycle-safe)
│   ├── snowflake/lineage_test.go  # Unit tests for lineage parser (56 cases; no Snowflake required)
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
    │       │   └── ImportTableModal.tsx    # Table data import dialog (CSV/JSON/AVRO/ORC/PARQUET)
    │       ├── files/FileBrowser.tsx
    │       ├── git/
    │       │   ├── GitPanel.tsx
    │       │   └── CommitModal.tsx
    │       ├── er/
    │       │   ├── ERDiagramModal.tsx  # Read-only ER diagram viewer (from existing DB)
    │       │   ├── ERDesigner.tsx      # Visual ER schema designer (create new tables)
    │       │   └── buildMermaid.ts    # Mermaid source generator for the diagram viewer
    │       ├── account/
    │       │   ├── AccountPanel.tsx              # Administration panel: roles, warehouses, user management, backup policies, integrations
    │       │   ├── QueryHistoryModal.tsx          # Query Activity modal (INFORMATION_SCHEMA.QUERY_HISTORY_*)
    │       │   ├── WarehouseMeteringModal.tsx     # Warehouse Credit Usage modal (ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY)
    │       │   ├── UserManagementPanel.tsx        # User list, search, right-click menu
    │       │   ├── EditUserModal.tsx              # ALTER USER dialog with live SQL preview
    │       │   ├── CreateUserModal.tsx            # CREATE USER dialog with live SQL preview
    │       │   ├── BackupPoliciesPanel.tsx        # Backup policies list with create/alter/drop
    │       │   ├── IntegrationsPanel.tsx          # Integrations tree: lazy-load, right-click create/properties/modify/drop
    │       │   ├── CreateIntegrationModal.tsx     # Dynamic CREATE INTEGRATION form per kind and subtype
    │       │   └── IntegrationModifyModal.tsx     # DESCRIBE properties + editable ALTER SQL editor
    │       ├── backup/
    │       │   └── BackupSetsModal.tsx     # Backup sets + nested backups with add/drop/restore
    │       ├── chat/AiChat.tsx        # AI Chat panel with tool-call display and Run/Copy buttons
    │       ├── lineage/DependenciesModal.tsx  # Recursive dependency tree modal with DDL hover tooltips
    │       ├── procedure/CallProcedureModal.tsx
    │       ├── results/ResultGrid.tsx
    │       ├── settings/
    │       │   ├── AISettingsModal.tsx    # AI provider / API key / model configuration
    │       │   └── LayoutSettingsModal.tsx
    │       ├── snowpark/
    │       │   ├── SnowparkCheckModal.tsx  # Environment check dialog
    │       │   └── SnowparkSetupModal.tsx  # Three-step setup wizard (conda / venv)
    │       ├── notebook/
    │       │   └── NotebookTab.tsx         # Jupyter-style notebook with Monaco cell editors
    │       ├── migration/MigrationModal.tsx # Schema Migration wizard (Tools menu)
    │       ├── dbt/DbtProjectModal.tsx     # dbt Project Scaffolding wizard (Tools menu)
    │       ├── snippets/SnippetsModal.tsx  # Code Snippets browser (Tools menu)
    │       ├── help/KeyboardShortcutsModal.tsx  # Searchable keyboard shortcuts reference (Help menu)
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

### Lineage parser tests

`internal/snowflake/lineage_test.go` contains 56 unit tests for the DDL-based dependency engine — no Snowflake connection required:

- **Pure-function tests** — `stripQuotes`, `splitIdent`, `extractArgTypesFromShow`, `depKey`, `depVisited`, `ExtractDDLBody` for all object kinds
- **`ExtractDDLBody` edge cases** — `SECURE VIEW … COPY GRANTS`, `RECURSIVE VIEW`, many column aliases, semicolons inside string literals, single-quoted procedure bodies, SQL UDFs
- **Reference parsing** — `FROM`, `JOIN` (all variants: `LEFT`/`RIGHT`/`INNER`/`FULL OUTER`), `MERGE INTO`, `UPDATE … FROM`, `INSERT INTO`, `CALL`, fully-qualified three-part names, quoted identifiers, CTE exclusion, `INFORMATION_SCHEMA` exclusion, comment stripping, and deduplication
- **`RewriteSQLReferences` complex cases** — mixed `{{ source() }}`/`{{ ref() }}` in one query, same ref three times, `UNION ALL` across four databases, all JOIN variants, deeply nested CTEs, scalar subqueries, `WHERE IN`, case-insensitive lookup, two-part and three-part of the same table, `LATERAL FLATTEN`, `MERGE INTO`, `UPDATE FROM`, external refs left unchanged, and a documented CTE-alias shadowing limitation
- **Full pipeline tests** — `ExtractDDLBody → RewriteSQLReferences` end-to-end: multi-level dependency graphs, cross-database `MERGE`/`UNION`, complex quoted names, procedures with `CALL` + `SELECT`, commented-out `CALL` statements, ten-table complex view with nested CTEs, window functions, and depth-limit clamping

```bash
go test -v ./internal/snowflake/ -run "^Test"
```

### dbt generator tests

`internal/dbt/generator_test.go` contains 56 unit tests for the dbt project generator — no Snowflake connection required:

- **`TestGenerate`** (25 cases) — end-to-end disk I/O tests using `t.TempDir()`: single and multi-scope sources, system/empty schemas, correct file contents, stub naming, `_sources.yml` structure, profiles.yml session values, and `.gitkeep` stubs
- **`TestSourceName`** (9 cases) — source name construction from (db, schema) pairs
- **`TestStagingModelPath`** (7 cases) — single-scope vs multi-scope stub path generation
- **`TestStagingModelSQL`** (8 cases) — CTE structure, Jinja `{{ source(...) }}` references, comment line; plus inline-body mode: verbatim SQL embed, pre-rewritten Jinja refs, complex multi-CTE bodies, and empty-body fallback to passthrough stub
- **`TestGenerate_InlineViewDefs`** (7 cases) — `InlineViewDefs` integration: view with body → inlined stub; view without body → source() passthrough; tables always use source(); partial `ViewDefs` maps; mixed tables + views; multi-scope prefixed filenames; explicit empty body treated as missing
- **`TestGenerate_DatabaseVars`** (8 cases) — `DatabaseVars` integration: `vars:` block written to `dbt_project.yml`; `_sources.yml` uses `{{ var(...) }}` instead of literal DB names; multiple DBs sorted alphabetically; each schema references its own var; `DatabaseVars=false` emits literals; empty schemas excluded from vars; system schemas included; original DB name case preserved as var default

```bash
go test -v ./internal/dbt/
```

### Migration helper tests

The root package (`migration.go`) contains unit tests for all pure migration helper functions — no Snowflake connection required.

```bash
go test -v -run "^Test(Normalize|SplitTopLevel|MigrQuote|ParseLocal|CommonColumn|ReplaceDDL|IsDependency|ExecutionPriority|RemoteKey|BuildMigration|ScanMigration)" .
```

> **Linux note** — the root package imports Wails (CGO + GTK/WebKit). Install the system headers first:
> ```bash
> sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.0-dev
> go test -v -run "^Test(Normalize|SplitTopLevel|MigrQuote|ParseLocal|CommonColumn|ReplaceDDL|IsDependency|ExecutionPriority|RemoteKey|BuildMigration|ScanMigration)" .
> ```

---

## Integration tests

Integration tests live in `internal/integration/` and are gated behind the
`integration` build tag, so they are **never run** by `go test ./...`.  They
require a real Snowflake account.

### What they do

**DDL export tests** (`export_test.go`) each:

1. Connect to Snowflake using environment variables.
2. Create a temporary database named `THAW_TEST_<random>` with two schemas
   (`ALPHA`, `BETA`) containing objects of every supported DDL type — tables,
   views, JavaScript functions (including overloads), stored procedures,
   sequences, internal stages, streams, and file formats.
3. Run the full parallel export pipeline (`ddl.ExportDatabases`).
4. Validate the file-system output: file existence, directory structure,
   content correctness, and that function overloads land at distinct paths.
5. Drop the temporary database unconditionally, even when the test fails.

**Schema migration tests** (`migration_test.go`) each:

1. Connect to Snowflake using environment variables.
2. Create a temporary database (`THAW_MIGTEST_<random>` if `SNOWFLAKE_TEST_DATABASE` is
   not set — using `CREATE DATABASE` without `OR REPLACE` to avoid clobbering anything;
   retries with a new random name if the candidate name already exists).
3. Exercise all four migration strategies against real tables with both data and empty tables:
   - **Smart In-Place** — `ALTER TABLE ADD/DROP/ALTER COLUMN`; data preserved; covered by:
     - Basic: single-column changes, data verified in kept columns
     - Complex schema (8 columns, 20 rows): widen VARCHAR, add/drop columns; row count, spot-checks, NULL validation for new columns, widened column accepts 150-char string
     - Empty table: schema mutations on 0-row table; insertable after migration
     - Multiple columns: multiple ADD/DROP in one pass
   - **Blue/Green Swap** — temp table + `SWAP WITH`; covered by:
     - Basic: shared columns preserved, non-shared columns discarded
     - Complex data (7 columns, 15 rows): product/region spot-checks, NULL validation on new columns, dropped columns absent
     - Empty table: schema swap with zero rows; insertable after swap
   - **View-Based Soft Cutover** — rename to `_v1` + new table + compat view; covered by:
     - Basic: archive row count, compat view exposes shared columns
     - Complex data (6 columns, 12 rows, 4 departments): archive=12, new table=0, compat view=12, dept count spot-checks, new table insertable with updated schema
     - Empty table: archive/new table/compat view all empty; schema correct
   - **Destructive Rebuild** — `DROP + CREATE`; covered by:
     - Basic: data gone, new table exists
     - Complex schema (10 columns, 25 rows via GENERATOR including VARIANT/ARRAY/OBJECT): new schema completely different; row count=0, new columns present, old columns absent, insertable immediately
   - **Empty-table fast path** — zero-row tables always use `DROP + CREATE` regardless of strategy
   - **Various column types** — NUMBER, FLOAT, VARIANT, ARRAY, OBJECT, TIMESTAMP_NTZ
4. Drop the temporary database unconditionally via `t.Cleanup`, even when the test fails.

### Required environment variables

| Variable | Description |
|---|---|
| `SNOWFLAKE_ACCOUNT` | Account identifier, e.g. `myorg-myaccount` |
| `SNOWFLAKE_USER` | Login name |
| `SNOWFLAKE_PRIVATE_KEY` | PEM-encoded RSA private key (key-pair authentication) |
| `SNOWFLAKE_WAREHOUSE` | Warehouse to use, e.g. `COMPUTE_WH` |
| `SNOWFLAKE_ROLE` | *(optional)* Role to assume; must have `CREATE DATABASE` for migration tests |
| `SNOWFLAKE_TEST_DATABASE` | *(optional)* Database for migration tests; auto-created as `THAW_MIGTEST_<random>` if not set |
| `SNOWFLAKE_TEST_SCHEMA` | *(optional)* Schema within the test database; defaults to `PUBLIC` |

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

All other privileges (CREATE TABLE, CREATE VIEW, CREATE FUNCTION, CREATE STAGE, etc.) are
automatically granted to the owner of the database created by the test.

Migration tests also need `CREATE TABLE` / `ALTER TABLE` / `DROP TABLE` within the
test database — these are covered by the ownership grant above when the test creates
the database itself. If you supply a pre-existing `SNOWFLAKE_TEST_DATABASE`, ensure the
role has at least `CREATE SCHEMA` and `CREATE TABLE` on that database.

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

Open **Help → Keyboard Shortcuts…** in the menu bar for a searchable, always-up-to-date reference. The full list is below.

### Tabs & Navigation

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘T` | `Ctrl+T` | New scratch tab |
| `⌘O` | `Ctrl+O` | Open SQL file |
| `⌘S` | `Ctrl+S` | Save active file |
| `⌘⇧S` | `Ctrl+Shift+S` | Save As… |
| `⌘W` | `Ctrl+W` | Close current tab |
| `⌘⇧T` | `Ctrl+Shift+T` | Reopen last closed tab |
| `⌃Tab` | `Ctrl+Tab` | Switch to next tab |
| `⌃⇧Tab` | `Ctrl+Shift+Tab` | Switch to previous tab |
| `⌘,` | `Ctrl+,` | Open Preferences (AI settings) |

### Query Execution

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘ Enter` | `Ctrl+Enter` | Run query (or selected text) |
| `⌘⇧ Enter` | `Ctrl+Shift+Enter` | Run all statements |
| `Esc` | `Esc` | Cancel running query |
| `⌘↓` | `Ctrl+↓` | Focus results grid |
| `⌘E` | `Ctrl+E` | Export current results as CSV |

### Editor

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘/` | `Ctrl+/` | Toggle line comment |
| `⇧⌥A` | `Shift+Alt+A` | Toggle block comment |
| `⇧⌥F` | `Shift+Alt+F` | Format SQL document |
| `Ctrl+Space` | `Ctrl+Space` | Trigger autocomplete |
| `Tab` | `Tab` | Accept AI suggestion |
| `⌘F` | `Ctrl+F` | Find in document |
| `⌘⌥F` | `Ctrl+H` | Find and replace |
| `⌘D` | `Ctrl+D` | Select next occurrence |
| `⌃G` | `Ctrl+G` | Go to line |
| `⌘⌥↑` | `Ctrl+Alt+↑` | Add cursor above |
| `⌘⌥↓` | `Ctrl+Alt+↓` | Add cursor below |
| `⌘+` | `Ctrl++` | Increase editor font size |
| `⌘-` | `Ctrl+-` | Decrease editor font size |
| `⌘0` | `Ctrl+0` | Reset editor font size to default |

### UI & Panels

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘B` | `Ctrl+B` | Toggle left sidebar |
| `⌘⇧F` | `Ctrl+Shift+F` | Focus object browser search |
| `⌘\` | `Ctrl+\` | Toggle split editor view |
| `⌘L` | `Ctrl+L` | Focus AI Chat |
| `⌘\`` | `Ctrl+\`` | Open embedded terminal |

### Notebook (Command Mode — no cell editor focused)

| Key | Action |
|-----|--------|
| `Shift+Enter` | Run current cell |
| `B` | Add cell below |
| `A` | Add cell above |
| `D D` | Delete current cell (confirmation required) |
| `Y` | Change cell type to Code |
| `M` | Change cell type to Markdown |
| `S` | Change cell type to SQL |

---

## Configuration

Git and export settings are stored at:

- **macOS** — `~/Library/Application Support/thaw/config.json`
- **Linux** — `~/.config/thaw/config.json`
- **Windows** — `%APPDATA%\thaw\config.json`

The file stores the remote URL, branch, export directory, export path template, author info, and AI provider settings (provider, model, enabled flag, and API key).
**Git tokens are never written to disk.** The AI API key is written to `config.json` with mode `0600` (owner-read-only).

Session state (window size and tab list) is stored at:

| Build | Path |
|---|---|
| Development (`wails dev`) | `./thaw-session.json` |
| macOS production | `~/Library/Application Support/thaw/session.json` |
| Windows production | `%LOCALAPPDATA%\thaw\session.json` |
| Linux production | `~/.local/share/thaw/session.json` (or `$XDG_DATA_HOME/thaw/session.json`) |

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
