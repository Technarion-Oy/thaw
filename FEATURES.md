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
- **Multi-cursor editing** — `⌘⌥↑` / `Ctrl+Alt+↑` adds a cursor on the line above; `⌘⌥↓` / `Ctrl+Alt+↓` adds one below; works in the SQL editor, YAML editor, and all notebook cell editors; matches VS Code behaviour
- **Selection highlight** — selecting text highlights every other occurrence in the document; overview-ruler markers show occurrences in long files
- **Toggle line comment** — `⌘/` / `Ctrl+/` (or right-click → **Toggle Line Comment**) adds or removes `--` on the current line or on every line in the selection
- **Font size zoom** — `⌘+` / `Ctrl++` increases the editor font size, `⌘-` / `Ctrl+-` decreases it, `⌘0` / `Ctrl+0` resets to the default
- **Code folding** — fold arrows are always visible in the editor gutter; click to collapse or expand any SQL block — CTEs, `BEGIN…END` blocks, subqueries, and multi-line expressions
- **Hover definitions** — move the cursor over any table or view name — including fully-qualified three-part identifiers (`DB.SCHEMA.TABLE`) and double-quoted identifiers (`"MY_TABLE"`, `"DB"."SCHEMA"."TABLE"`) — to see its DDL in a scrollable overlay tooltip; the tooltip fires as the cursor enters the token (not just when stationary at the end), stays open when the cursor moves into it, and auto-loads object metadata for schemas not yet expanded in the sidebar:
  - **Copy button** — copies the full DDL to the clipboard
  - **Text selection** — paint any portion of the DDL and copy with `⌘C` / `Ctrl+C`
  - **Right-click → Copy** — right-click inside the tooltip to copy the selected text via a context menu
  - Definitions are cached per session and refreshed automatically after 60 seconds
  - **Function tooltips** — hovering over a bare function name (e.g. `DATEADD`, `FLATTEN`, or a UDF) shows all overloads with their full signatures and descriptions in the same overlay; backed by an embedded catalogue of ~320 built-in functions that is always available offline, and refreshed with live metadata after each Snowflake connection
- **Function call highlighting** — every function call in the editor is syntax-coloured by kind: built-in Snowflake functions appear in **gold** and user-defined functions appear in **teal**, making it easy to distinguish system functions from custom logic at a glance; highlighting updates as you type (200 ms debounce) and is seeded from a local SQLite cache on editor mount so it works without a live connection
- **SQL autocomplete** — context-aware completions:
  - `db.` → schemas in that database
  - `db.schema.` → tables, views, functions, and other objects in that schema
  - `db.schema.table.` → columns of that table or view
  - `Ctrl+Space` inside a query → columns from all tables referenced in the current `FROM`/`JOIN` clauses
  - After `ON` in a `JOIN` clause → join conditions in three tiers: **(1)** FK relationships — composite multi-column constraints produce a single `col1 = ref.col1 AND col2 = ref.col2` expression (sourced from `SHOW IMPORTED KEYS`); **(2)** PK-naming-convention heuristic (`orders.CUSTOMER_ID = customers.ID`) when no FK constraint exists; **(3)** type-compatible same-name columns with both `a.col = b.col` equality and `USING (col)` alternatives; works with quoted/unquoted identifiers, full three-part names, and optional table aliases
  - **Ghost text before ON** — after `JOIN table ` (before typing `ON`), an inline ghost-text suggestion `ON <condition>` appears and can be accepted with `Tab` (FK-cache-backed, instant)
  - **Ctrl+Space before ON** — pressing `Ctrl+Space` after a JOIN table reference but before typing `ON` opens a full dropdown of `ON <condition>` suggestions covering all three tiers
  - **Function completions** — typing two or more characters outside a dotted context also suggests matching Snowflake built-in and user-defined functions from the local cache; UDFs sort above built-ins so custom functions surface first; instant and available offline
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
- **Close confirmation** — closing a tab with unsaved changes (via the `×` button or `⌘W` / `Ctrl+W`) shows a dialog with three choices: **Save**, **Close without Saving**, or **Cancel**; for new scratch tabs or files not yet saved to disk, **Save** opens a native Save As dialog first; applies to SQL files, notebooks, and any scratch tab that has been edited
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the insertion point
- **Split view** — right-click any tab and choose **Split with: [tab name]** to view two editors side by side; a draggable vertical divider separates them and the ratio is persisted across sessions; each editor is fully independent with its own completions, hover definitions, and editing history; close the split with the × button in the secondary editor header, via **Close split view** in the right-click menu, or by closing either of the two tabs

---

## Object Browser

- Browse all databases → schemas → tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, pipes, and notebooks
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
  - **Multi-CTE correctness** — all CTE aliases in a `WITH` clause (first and subsequent `cte_name AS (...)` entries) are correctly excluded from the dependency list; only real table/view references produce dependency nodes
- **Right-click tables and views** to:
  - Select the top 1,000 rows — opens a new tab and executes immediately
  - **Time Travel Query** — drag a timeline slider to query data at any past point within the retention window
  - **Export Data** — download table data as CSV, JSON, or Parquet via a temporary Snowflake stage
  - **Import Data** — upload one or more local files into Snowflake; supports CSV, JSON, AVRO, ORC, and Parquet; exposes all Snowflake `FORMAT_TYPE_OPTIONS` with defaults pre-filled; can create a new table automatically by inferring the schema; **file preview** for CSV and JSON shows the first 10 rows of up to 5 files — CSV offers **Parsed** (table respecting current delimiter and header settings) and **Raw** views; JSON offers **Parsed** (tabular, supports arrays-of-objects and NDJSON) and **Raw** views; multiple files shown in a tabbed layout; **✨ AI Suggest** button (CSV and JSON, requires AI configured) — clicking shows a confirmation dialog disclosing that up to 64 KB of file content will be sent to the configured AI provider and warning against use with sensitive data; on confirmation, format options (delimiter, header detection, quoting, encoding, compression, etc.) are auto-filled and a one-sentence AI explanation is shown; a ⓘ icon next to the button also surfaces the data-sharing notice on hover
  - **Insert Full Name** — insert the fully-qualified `"DB"."SCHEMA"."OBJECT"` identifier at the cursor
  - View DDL definition inline
  - **Rename** the object
  - **Drop** the object (with confirmation)
  - **Select for Comparison** / **Compare with** — side-by-side DDL diff (see [Text Comparison](#text-comparison))
- **Right-click a database** to export its DDL, generate an ER Diagram, view dropped schemas recoverable via Time Travel, or open **Backup Sets…**
- **Right-click a schema** to view dropped tables, **Export Data…** or **Import Data…** without needing an existing table (schema-level launch opens the same modals with a table selector or name field), open **Backup Sets…**, or use the **Create Object** cascading submenu (opens left or right depending on available screen space); contains **Task…** to open the Create Task dialog
- **Right-click the Tasks folder** inside any schema to open **Create Task…** directly — the dialog covers the full `CREATE TASK` syntax:
  - **Create options**: `OR REPLACE` / `IF NOT EXISTS` checkboxes (mutually exclusive)
  - **Compute**: warehouse dropdown or serverless with initial size and optional min/max statement size selects
  - **Schedule**: visual editor — **None**, **Interval** (validated number + unit: seconds `10–691,200`, minutes `1–11,520`, hours `1–192`; out-of-range values highlighted red), or **Cron** (5-field expression + searchable timezone dropdown, ~440 Snowflake-supported timezones)
  - **Configuration**: `CONFIG` JSON string (dollar-quoted in the generated SQL)
  - **Dependencies**: predecessor task picker — type to search tasks in the current schema, hit **+** to add each one as a removable tag; already-added tasks are hidden from the dropdown; the preview emits fully-qualified `"db"."schema"."task"` references; **WHEN condition** — visual boolean expression builder with `SYSTEM$STREAM_HAS_DATA` (stream selector), `SYSTEM$GET_PREDECESSOR_RETURN_VALUE` (task selector, optional cast to BOOLEAN/FLOAT/STRING, comparison operator + value), and custom SQL condition rows; combine with AND/OR; negate with NOT; Visual/Raw SQL toggle; live WHEN preview below the builder
  - **Execution**: overlap policy (`NO_OVERLAP` / `ALLOW_CHILD_OVERLAP` / `ALLOW_ALL_OVERLAP`), execute as (Default / Caller / User), timeout, suspend-after-failures, auto-retry, minimum trigger interval, target completion interval
  - **Notifications**: error and success notification integration dropdowns (populated from `SHOW NOTIFICATION INTEGRATIONS`)
  - **Other**: log level (TRACE…OFF), comment; **finalize task** — AutoComplete dropdown listing only standalone tasks (no predecessors, not referenced as predecessor by any other task); disabled with a tooltip when the current task has child tasks
  - **SQL body** (`AS`) with live `CREATE TASK` preview; a yellow warning alert appears when the task has no trigger defined (no SCHEDULE, AFTER, FINALIZE, or WHEN)
- **Right-click a task** to:
  - **Execute Task…** — opens a dialog with two modes:
    - **Execute** — issues `EXECUTE TASK <name>` immediately; accepts an optional CONFIG JSON override (`USING CONFIG = $json$`); validates JSON on the fly and blocks execution while the input is invalid
    - **Retry Last** — issues `EXECUTE TASK <name> RETRY LAST` to resume the last failed or cancelled task graph run from the point of failure (requires the run to be `FAILED` or `CANCELED`, the graph to be unchanged, and the original attempt to be within 14 days)
    - A live SQL preview shows the exact statement before it is sent
  - **View Task Graph…** — opens an interactive DAG visualisation of the complete task graph rooted at the selected task:
    - Left-to-right layout computed automatically via Dagre; each node shows the task name, schedule state badge (STARTED / SUSPENDED), last-run state badge (Running, Succeeded, Failed, Skipped, Scheduled, Cancelled, Waiting…), and — for completed or failed runs — a completion timestamp (HH:MM:SS for today, "Jan 15 HH:MM" for earlier dates)
    - **Real-time status** — polls Snowflake every 3 seconds and updates all node states in place without re-running the layout or losing drag positions; a pulsing green **Live** indicator and last-updated timestamp are shown in the top-right of the canvas
    - **Skipped inference** — tasks with no `TASK_HISTORY` row for the current run (because a predecessor failed before they could be scheduled) are automatically shown as Skipped; transitive chains are resolved so every downstream dependent also shows Skipped; a stale Succeeded row from a previous run is correctly overridden when the predecessor's failure is more recent (timestamp guard prevents false overrides when the predecessor was fixed in a later run); timestamps are suppressed on Skipped nodes since the stored time would be from the task's last actual run, not the current skipped run
    - **Run Graph** button — calls `EXECUTE TASK <root>` immediately to start the whole graph; all child nodes switch to "Waiting…" optimistically the moment the call returns so stale last-run states no longer show; the next poll tick replaces them with real states
    - **Retry Failed** button — calls `EXECUTE TASK <root> RETRY LAST`; enabled only when the last graph run has at least one FAILED task AND the first attempt was within the last 14 days (mirrors Snowflake's eligibility conditions for `RETRY LAST`); disabled with a descriptive tooltip when conditions are not met (e.g. "Last graph run did not fail or get cancelled" / "Last failed run was more than 14 days ago"); root task's own run state is not required to be FAILED — a child task failure is sufficient
    - **Finalizer task display** — a task created with `FINALIZE = <root>` appears with a dashed purple border, a purple "Finalizer" badge (replacing the STARTED/SUSPENDED tag), and a dashed purple **finalizes** edge from the root node; the node is placed at the far right of the Dagre layout, after all leaf tasks; finalize relationship is detected via `GET_DDL('TASK', ...)` as a reliable fallback when the `task_relations` SHOW TASKS column is absent or in an unexpected format
    - **Right-click any node** for a context menu:
      - **Suspend / Resume** — issues `ALTER TASK IF EXISTS … SUSPEND/RESUME`; shows the applicable action based on the task's current state (STARTED → Suspend, SUSPENDED → Resume); schedule state badge updates immediately without waiting for the next poll
      - **Add Child Task…** — opens the Create Task dialog pre-configured for child mode (SCHEDULE field replaced by an info note, AFTER pre-filled with the right-clicked task name, FINALIZE field hidden); disabled on finalizer nodes
      - **Add Finalizer Task…** — opens Create Task dialog pre-configured for finalizer mode (SCHEDULE and AFTER fields replaced by info notes, FINALIZE pre-filled with the root task fully-qualified name); enabled only when right-clicking the root node and no finalizer task already exists; label reads "(already has one)" when the root already has a finalizer; reads "(root only)" on non-root nodes
  - **Properties** — opens a dedicated editable modal covering the full `ALTER TASK` syntax:
    - **Status**: RESUME / SUSPEND; when the task has child tasks, **Resume with children** (calls `SYSTEM$TASK_DEPENDENTS_ENABLE` — resumes descendants first, then the root) and **Suspend with children** (suspends root first, then all descendants) are also shown; Resume buttons are disabled when the task has no trigger configured
    - **Compute**: warehouse (select from available warehouses)
    - **Schedule**: inline visual schedule editor (None/Interval/Cron with validated interval ranges and searchable timezone dropdown; UNSET supported)
    - **Dependencies**: list of predecessor tasks; add with `ADD AFTER` or remove per row with `REMOVE AFTER`
    - **Condition**: WHEN expression — visual boolean expression builder (`STREAM_HAS_DATA`, `GET_PREDECESSOR_RETURN_VALUE`, custom SQL condition rows; Visual/Raw SQL toggle; Save / Cancel / Remove WHEN)
    - **SQL Body**: task SQL (multi-line editor with Save / Cancel via `MODIFY AS`)
    - **Configuration**: CONFIG JSON string (inline edit, UNSET supported)
    - **Limits**: user task timeout (ms) and overlap policy (ALLOW / DISALLOW)
    - **Notifications**: ERROR_INTEGRATION and SUCCESS_INTEGRATION selected from dropdowns of available notification integrations (UNSET supported)
    - **General**: comment (inline edit, UNSET) and EXECUTE AS (caller / user)
    - Every change is applied immediately via `ALTER TASK IF EXISTS … <clause>` and values reload after each save
- **Right-click a notebook** to:
  - **Open Notebook** — pulls the latest version from Snowflake using `DESC NOTEBOOK` and `GET`, then opens it in a new unsaved notebook tab
  - **Execute Notebook…** — opens a dialog to run `EXECUTE NOTEBOOK` with optional string parameters (each value is automatically single-quoted); the dialog shows the notebook's current Query Warehouse fetched from `SHOW NOTEBOOKS`; if none is set a warning alert offers a **Set Warehouse** button that opens a separate dialog with a warehouse selector and explicit **Save** / **Cancel** buttons (saves via `ALTER NOTEBOOK … SET QUERY_WAREHOUSE`); the execute dialog updates live once the warehouse is saved; a live SQL preview shows the exact statement that will run
- **Right-click a table** to open **Backup Sets…** (shows backup sets scoped to its schema)
- **Drag and drop** — drag any table or view into the editor to insert a `SELECT` statement with all column names listed individually
- **Empty table indicator** — table names with zero rows appear in a faded colour so unpopulated tables are immediately visible in the tree
- **Hover tooltips** — hovering any object in the tree shows its DDL definition
- **View Definition** — opens the DDL in a modal with a Copy button
- **Properties** — opens a key/value panel of object metadata populated from the relevant `SHOW` command; for tables the panel additionally provides two inline-editable sections:
  - **Table Settings** — view and edit cluster key, schema evolution, change tracking, data retention days, max data extension days, default DDL collation, and comment; booleans are toggled with a switch, numeric and text fields open an inline input with Save / Cancel; changes are applied immediately via `ALTER TABLE SET`
  - **Column Comments** — view and edit the comment on every column; each row shows the column name, its current comment (or a dash if empty), and a pencil icon to edit inline
  - For **tasks** the Properties entry opens the full Task Properties modal described above instead of the generic read-only panel
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

### Function Catalog AI Chat

Open **Help → Function Catalog…** and select any function to get:

- **Details tab** — all overload signatures and descriptions from the local function cache
- **Ask AI tab** — a chat panel for asking questions about the selected function:
  - The function's full signatures and descriptions are automatically injected as context — no need to paste anything
  - Ask usage questions, request examples, compare overloads, or explore edge cases
  - For built-in Snowflake functions, a **📖 Snowflake documentation** link at the top of the tab opens the official docs page in the system browser
  - Chat history resets automatically when you switch to a different function
  - Requires AI to be configured via **AI → Configure AI…**

### Model Validation

When configuring AI, a live **model status indicator** appears next to the model selector: a green `● Model OK` confirms the model is reachable, while a red indicator shows the exact API error — so misconfigured model names are caught immediately rather than at runtime.

### Configuration

Open **AI → Configure AI…** in the menu bar to set your provider, API key, and model. The API key is stored locally with restricted file permissions (`0600`) and never transmitted anywhere other than the selected AI provider.

---

## File Management

- **Open** (`⌘O` / `Ctrl+O`) — native OS file dialog filtered to `.sql`, `.yml`, `.yaml`, and `.py`; opens in the configured export directory by default; re-activates an existing tab if the file is already open; the editor automatically uses YAML or Python syntax highlighting based on the file extension
- **YAML intelligence** — dbt YAML files opened in the editor receive schema-driven autocompletions, hover documentation, and real-time validation (red squiggles) powered by the bundled dbt-jsonschema schemas — all schemas are embedded locally, no network requests at runtime; covers `dbt_project.yml`, `packages.yml`, `dependencies.yml`, `selectors.yml`, and all model/source/seed/snapshot/exposure YAML files; property names, allowed values, and inline documentation strings are surfaced as you type; non-dbt YAML files (`profiles.yml`, CI configs, etc.) are not falsely flagged with schema validation warnings
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

## Schema Migration

Open **Tools → Schema Migration…** to deploy local `.sql` DDL files to a Snowflake database. A 5-step wizard guides the process from source directory to live deployment.

### Step 1 — Configure
- Add one or more **source directory → target database** mappings using the mapping list:
  - Each row has a source directory (type or **Browse…**) and a **Target DB** dropdown (optional fallback)
  - The target database is used for objects in that directory that have no explicit `USE DATABASE` context in the SQL files
  - Click **Add Database** to add a row; click the delete button to remove one; at least one directory is required to scan
  - Multiple mappings let you migrate several databases in a single wizard run

### Step 2 — Scan
- Recursively reads every `.sql` file in all source directories
- Handles multi-statement files; tracks `USE DATABASE` / `USE SCHEMA` context; applies each mapping's fallback database for unqualified objects
- Merges and deduplicates objects across all sources by kind + name (last definition wins); shows a count breakdown by object type (TABLE: N, VIEW: N, …)

### Step 3 — Review
- **Ag-Grid diff table** with status tags:
  - **New** — object exists locally but not in Snowflake
  - **Changed** — DDL differs after normalisation (comments stripped, whitespace collapsed, uppercased, trailing `;` removed)
  - **Unchanged** — identical; hidden from selection by default
  - **Removed** — exists in Snowflake but not in the local source
- **Monaco DiffEditor** below the grid shows the local vs remote DDL for the selected row
- **Dependency auto-select** — selecting a VIEW or PROCEDURE automatically selects any referenced TABLE that is also "new" or "changed"; unchecking a TABLE that a selected VIEW or PROCEDURE depends on is blocked with an inline warning ("Required by: VIEW_NAME")

### Step 4 — Strategy & Protect
Choose how existing TABLE objects with data are handled, then optionally create safety snapshots before deploying.

#### Table Migration Strategies
Only applies to TABLE objects that already exist in Snowflake and have rows. Empty tables (`SHOW TABLES` reports 0 rows) always use a fast `DROP + CREATE` regardless of the selected strategy.

| Strategy | How it works |
|---|---|
| **Smart In-Place** *(default)* | Diffs local vs remote column definitions; issues `ALTER TABLE ADD COLUMN`, `DROP COLUMN`, and `ALTER COLUMN TYPE` — no data movement |
| **Blue/Green Swap** | Creates a temp table with the new schema, copies shared columns via `INSERT … SELECT`, atomically swaps with `ALTER TABLE … SWAP WITH`, drops the temp; non-shared columns are discarded |
| **View-Based Soft Cutover** | Renames the original table to `<name>_v1`, creates the new table, and creates a compatibility view `<name>_compat` that exposes the shared columns from the archived data |
| **Destructive Rebuild** | `DROP TABLE IF EXISTS` + `CREATE TABLE`; all existing data is permanently lost; a red warning banner is shown when this strategy is selected |

- **Open in SQL Editor** — generates a strategy-aware SQL script and opens it in a new editor tab for review and editing before running:
  - Smart In-Place → `ALTER TABLE ADD/DROP/ALTER COLUMN TYPE` statements
  - Blue/Green Swap → `CREATE TABLE tmp; INSERT … SELECT; ALTER TABLE SWAP WITH; DROP TABLE`
  - View-Based Soft Cutover → `ALTER TABLE RENAME TO _v1; CREATE TABLE; CREATE VIEW _compat AS SELECT …`
  - Destructive Rebuild → `DROP TABLE IF EXISTS; CREATE TABLE`

#### Safety Snapshots (optional, per target database)
- The snapshot section shows one block per unique target database involved in the selected objects
- **Create Backup Set** — `CREATE BACKUP SET FOR DATABASE <db>` targeting a chosen database / schema / name
- **Create Zero-Copy Clone** — `CREATE DATABASE <clone> CLONE <db>` for a point-in-time snapshot
- Each database's backup and clone settings are independent; databases with no snapshot options checked are skipped

### Step 5 — Deploy
- Objects execute in dependency order: DATABASE → SCHEMA → SEQUENCE → TABLE → FILE FORMAT → STAGE → VIEW → MATERIALIZED VIEW → FUNCTION → PROCEDURE → STREAM → TASK → PIPE
- Up to **5 retry passes** — objects that fail with a dependency error ("does not exist" / "not authorized") are automatically re-queued for the next pass; once a pass produces no progress the remaining objects are marked as failed
- **Live progress table** — pass number, object kind, fully-qualified name, and per-object status tag (running / success / failed / skipped) update in real time as events arrive
- **Cancel** — stops the deployment cleanly mid-run

---

## dbt Project Scaffolding

Open **Tools → Create dbt Project…** to scaffold a complete dbt project pre-wired to the active Snowflake connection — no dbt CLI required during generation.

### Step 1 — Configure
- Set the **project name** and **profile name** (mirrors the project name by default, independently editable once changed)
- Choose the **output directory** with a native directory picker or type a path directly
- Thaw warns when the target `<dir>/<name>` directory already exists to prevent accidental overwrites
- **Inline view SQL definitions** toggle (off by default) — when enabled, Thaw fetches the `GET_DDL` for each view in the selected schemas and embeds the actual `SELECT` body into the staging stub instead of a generic `{{ source() }}` pass-through; one extra `GET_DDL` call per view is made at generation time
- **Automatic reference rewriting** (active whenever inline view SQL is enabled) — after all schemas are fetched, Thaw scans every inlined view body for multi-part Snowflake identifiers and rewrites them to correct dbt Jinja calls:
  - Three-part references to **tables** in selected schemas → `{{ source('db_schema', 'TABLE') }}`
  - Three-part references to **views** in selected schemas → `{{ ref('stg_model_name') }}`
  - References to objects **outside** the selected schemas → left unchanged
  - CTE aliases are excluded to prevent false-positive replacements; single-part names are never replaced to avoid collisions with column aliases
- **Use dbt variables for database names** toggle (off by default) — when enabled, adds a `vars:` block to `dbt_project.yml` with one entry per selected database (e.g. `db_mydb: MYDB`, sorted alphabetically) and replaces hardcoded database names in `_sources.yml` with `{{ var('db_mydb', 'MYDB') }}` calls; the default value in the var preserves the original database name casing; retargeting the project at a different database then only requires overriding the relevant variable

### Step 2 — Select Sources
- Databases load lazily from the live Snowflake connection
- Expand any database to fetch and display its schemas as a checkbox list
- **Select all / Deselect all** link per database for quick selection
- `INFORMATION_SCHEMA` is shown with a warning icon and descriptive tooltip, excluded from **Select all**; when checked, it is added to `_sources.yml` as a system schema entry but no staging stubs or `ListObjects` calls are made — this matches dbt convention for referencing virtual Snowflake schemas
- **Cross-schema dependency hints** — checking a schema triggers a background analysis of all views in that schema (via `SHOW VIEWS IN SCHEMA`, which returns the full `CREATE VIEW` DDL); view bodies are scanned for `FROM` / `JOIN` references to other schemas; any referenced schema not yet selected is highlighted in the list with an amber indicator and a tooltip listing the selected schemas that reference it; "Select all" for a database triggers a single batched analysis of all schemas at once; analysis is non-blocking — the spinner shows "Analysing dependencies…" per schema while in flight and disappears silently when done; results are cached for the lifetime of the wizard
- At least one schema must be selected to proceed

### Step 3 — Generate
- Summary shows project path, number of databases and schemas selected, and estimated file count
- **Generate Project** creates all files on disk; a spinner shows "Creating project files…" while in flight
- **Success** — collapsible file list grouped by directory; a note below the list reminds you to copy `profiles.yml` to `~/.dbt/` before running dbt commands
- **Error** — red alert with message and a back button to return to Step 1

### Generated files

| File | Description |
|------|-------------|
| `dbt_project.yml` | Project config: name, profile reference, materialization defaults (staging → view, marts → table); optional `vars:` block when **Use dbt variables** is enabled |
| `profiles.yml` | Pre-filled from the live session: account, user, role, warehouse, database, schema |
| `models/staging/_sources.yml` | One `source:` entry per selected (database, schema) |
| `models/staging/stg_<table>.sql` | CTE stub per table/view (`with source as … renamed as … select * from renamed`) |
| `models/marts/.gitkeep` | Directory placeholder |
| `seeds/.gitkeep` | Directory placeholder |
| `macros/.gitkeep` | Directory placeholder |

When multiple databases or schemas are selected, stub filenames are prefixed with `db_schema_` (e.g. `stg_mydb_public_orders.sql`) to prevent collisions. Single-scope projects use the shorter `stg_<table>.sql` form.

---

## Git Integration

- View git status for the working directory (staged and unstaged files)
- **Pull** — fetch and merge from the configured remote branch
- **Commit & Push** — select individual files to stage, filter by extension, enter a commit message and personal access token
- Git credentials are **never saved to disk** — the token is held in memory only for the duration of the push
- OS junk files (`.DS_Store`, `Thumbs.db`, `desktop.ini`) are automatically excluded and added to `.gitignore`

---

## Administration

- View all roles, warehouses, users, and Snowflake integrations from the **Administration** panel in the sidebar

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
- Results table shows status (colour-coded), query type, query preview, start time, end time, and duration
- Expand any row to see the full SQL plus a detail grid with user, warehouse, database, schema, rows produced, bytes scanned, and query ID
- **Load in Editor** — inserts the query into the active editor tab and closes the modal
- **Copy** — copies the full query text to the clipboard with a brief "Copied!" confirmation

### Backup Policies

- List all backup policies with schedule, expiry, retention lock, owner, and comment
- **Create** — full `CREATE BACKUP POLICY` support: schedule, expire after days, tags, comment, `WITH RETENTION LOCK`, and `OR REPLACE` / `IF NOT EXISTS` modifiers
- **Alter** — rename, set/unset schedule, expiry, comment, and retention lock via an action dropdown
- **Drop** — with confirmation

### Integrations

Browse, create, modify, and drop all six Snowflake integration types from a lazy-loading tree in the Administration panel:

| Kind | Supported Subtypes / Providers |
|------|-------------------------------|
| **Storage** | Amazon S3, S3 GovCloud, Google Cloud Storage, Azure Blob Storage |
| **API** | AWS API Gateway, AWS Private API Gateway, Azure API Management, Google API Gateway, Git HTTPS API |
| **Catalog** | AWS Glue, Object Store, Polaris, Iceberg REST, SAP BDC |
| **External Access** | Network-rule-based (allowed network rules + optional authentication secrets) |
| **Notification** | Email, Webhook, Azure Storage Queue (inbound), GCP Pub/Sub (inbound/outbound), AWS SNS (outbound), Azure Event Grid (outbound) |
| **Security** | API Authentication (AWS IAM / OAuth2), External OAuth, OAuth partner (Looker, Tableau, Power BI), OAuth custom, SAML2, SCIM |

- **Lazy loading** — each category's integrations are fetched from Snowflake only when the node is first expanded
- **Create** — right-click any category to open a structured form; fields change dynamically based on the selected integration type and subtype; cloud provider defaults (S3 / GCS / Azure for Storage; equivalent defaults for API) are pre-selected based on the current Snowflake region; the option is automatically disabled when the current role lacks `CREATE INTEGRATION`
- **Properties** — right-click any integration and choose **Properties** to see its `DESCRIBE INTEGRATION` output as a key/value table
- **Modify** — right-click and choose **Modify** to open a modal showing current DESCRIBE properties alongside an editable ALTER SQL textarea; click **Run** to execute the statement
- **Drop** — right-click and choose **Drop** with a Popconfirm confirmation; the category reloads automatically on success

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

### Warehouse Properties

Right-click any warehouse in the Administration panel and choose **Properties** to open an editable properties modal:

- **Status** — current state badge (STARTED / SUSPENDED / RESUMING / QUIESCING) with type, size, and owner; action buttons:
  - **Suspend** / **Resume** — toggle warehouse state immediately
  - **Abort All Queries** — cancel all running queries (confirmation required)
  - **Rename** — inline name input; the sidebar warehouse list updates live
- **Compute** — warehouse size (X-Small → 6X-Large), warehouse type (Standard / Snowpark-Optimized); multi-cluster warehouses also expose max/min cluster count and scaling policy
- **Behavior** — auto-suspend seconds (0 = disabled), auto-resume toggle
- **Query Acceleration** — enable/disable, max scale factor (0–100)
- **Resource & Timeouts** — resource monitor, max concurrency level, statement queued timeout, statement timeout (from `SHOW PARAMETERS IN WAREHOUSE`)
- **General** — comment
- Each property saves immediately via `ALTER WAREHOUSE … SET` on confirm
- **Inline privilege errors** — `ALTER WAREHOUSE` failures (e.g. insufficient privileges) are shown inline below the field in red rather than silently discarded; toggle-switch errors appear as message toasts; rename errors appear below the name input; the "Insufficient privileges" phrase is extracted for a short, readable message

### User Management

- **User Management** — search users by name, login, display name, or email; view disabled accounts at a glance
- **Create User** — dialog with all user properties and a live `CREATE USER` SQL preview
- **Edit User** — pre-populated form that generates only the `ALTER USER … SET/UNSET` statements needed for the changed fields
- **Enable / Disable / Drop** users with a single right-click action
- All user management actions are automatically hidden or greyed out when the current role lacks the required privileges
- **Key Pair Authentication** — right-click any user and choose **Key Pair Auth…** to set up Snowflake key-pair authentication without leaving the app:
  - Choose a key generation method: **Go built-in crypto** (always available, no passphrase), **OpenSSL** (passphrase-encrypted private key), or **ssh-keygen** (passphrase-encrypted private key); only tools present on PATH are shown
  - Set the private key output path (type or browse); the public key is saved alongside with `_pub.pem` appended; the private key file is written with mode `0600`
  - Optionally enter a passphrase (disabled for Go built-in)
  - Click **Generate key pair** to produce an RSA-2048 PKCS#8 PEM key pair; the stripped public key content (no PEM header/footer) is shown for review
  - Click **Apply to \<username\>** to run `ALTER USER … SET RSA_PUBLIC_KEY='…'` immediately
  - The menu item is greyed out automatically when the current role lacks OWNERSHIP or MODIFY PROGRAMMATIC AUTHENTICATION METHODS on that user
- **Key pair auth in Create User** — the **Create User** dialog includes an **RSA public key** field and a **Generate key pair…** button; clicking the button opens the key pair generator in "pick" mode so you can generate a key pair and auto-fill the public key without leaving the create flow

---

## Results & Export

- Query results displayed in a virtualised grid — handles large result sets smoothly
- **NULL display** — `NULL` values are rendered as a faded italic `NULL` label so they are never confused with empty strings
- **Copy from results** — right-click any cell to open a context menu with: **Copy cell value**, **Copy row (tab-separated)**, and **Copy row with headers**; all three write to the native OS clipboard so they work reliably on macOS
- **Result history** — the last 10 successful result sets are kept in memory for the session; a dropdown in the results status bar (visible after two or more runs) lets you switch between them instantly, similar to `LAST_QUERY_ID(-n)` in SQL; after a query failure the error is shown and the dropdown appears as a standalone **Previous results** picker — the last result grid is not auto-displayed so the failure is immediately obvious, but any historical result can be recalled on demand; click the **pin** icon next to any entry in the dropdown to keep it indefinitely — pinned results are exempt from the 10-entry cap and always appear at the top of the list (click again to unpin); **right-click** any entry and choose **View side by side** to open it alongside the current result in a horizontally split view — both grids scroll in sync so corresponding rows stay level, column headers align, and the compare panel's SQL snippet, query ID, and row count appear on a second line of the status bar (right-aligned for clarity); close the compare panel with the × button in the status bar
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
- **Current username** — the active Snowflake username (from `CURRENT_USER()`, preserving exact case) is displayed above the toolbar session selectors and above the account · user tag so the connected identity is always visible
- **Session state persisted across reloads** — the account · user tag and non-sensitive connection details survive a page reload; credentials (password, passcode, private key passphrase) are never written to storage; the connected state is verified against the backend on every reload so a backend restart correctly shows ConnectModal pre-filled with the last-used parameters rather than a broken UI; the UI waits for state hydration to complete before rendering, preventing a spurious ConnectModal flash on HMR page reloads
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
- **Open from Snowflake** — right-click any notebook in the object browser and choose **Open Notebook**; the latest version is downloaded from Snowflake and opened as a new unsaved notebook tab
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
- **Auto-connected Snowpark session** — a Snowpark session is automatically created on kernel startup using the same account, role, warehouse, database, and schema as the active app connection; `get_active_session()` (from `snowflake.snowpark.context`) works in every Python cell with no `Session.builder` boilerplate — matching Snowflake's native notebook behaviour; session init errors (e.g. wrong credentials or missing private key) are surfaced in the first cell's stderr
- **Session kept in sync — bidirectional** — changing role, warehouse, database, or schema via the toolbar automatically applies the change to the kernel session via `get_active_session()`; switching to a notebook tab also triggers a sync; conversely, when a Python or SQL cell runs a `USE` command the change propagates back to the main Snowflake connection pool — all four toolbar dropdowns update automatically and subsequent queries in SQL editor tabs immediately reflect the new context; Python cells, SQL cells, and SQL editor tabs always see the same session state
- **DDL executes immediately** — `session.sql("USE DATABASE X")` takes effect without an explicit `.collect()` call, matching Snowflake native notebook behaviour; USE, CREATE, ALTER, DROP, TRUNCATE, COMMENT, GRANT, and REVOKE are auto-collected on the session instance at startup
- **Python intellisense** — [Jedi](https://jedi.readthedocs.io/)-powered completions and hover documentation in every code cell, sourced from the running kernel so the live namespace (all variables defined in previous cells) is available:
  - **Autocomplete** — trigger with `.` or `Ctrl+Space`; shows function, class, module, keyword, variable, and property completions with kind icons, fully-qualified name detail, and docstring popovers; runtime-aware so `df.` on a Pandas DataFrame shows all DataFrame methods
  - **Hover documentation** — hover any name to see its signature and docstring; function calls show the full parameter signature first; content is fetched from the kernel on demand

### SQL cells

- SQL cells execute through the **Snowpark kernel session** — the same session Python cells use — so `USE` commands in SQL cells affect Python cells and vice versa, and `SELECT CURRENT_DATABASE()` always returns the same value in both cell types
- SQL is split into individual statements by a parser that handles `--` line comments, `/* */` block comments, single-quoted strings, and `$$`-dollar-quoted strings; each statement runs in order and the last result is displayed
- **Run selection** — if text is selected in a SQL cell, only the selected SQL is executed
- `USE DATABASE X;` in a SQL cell updates the toolbar dropdowns and the Python session automatically
- Results render in a **sticky-header scrollable table** (up to 1 000 rows)
- DDL / DML with no result set shows "OK — N rows affected"

### Notebook management

- **Run All**, **Restart Kernel**, **Save**, **Add Cell** in the toolbar
- **Deploy** — deploys the notebook to Snowflake via a dialog with all `CREATE NOTEBOOK` options (database, schema, name, `OR REPLACE` / `IF NOT EXISTS`, comment, query warehouse, Python runtime warehouse, idle auto-shutdown seconds, runtime name, compute pool); works for both saved and unsaved notebooks — unsaved content is serialised and written to a temporary file automatically
- Per-cell controls: run, move up/down, add below, **delete** (confirmation dialog)
- **Command mode** — when no cell Monaco editor is focused, the selected cell (last clicked or focused, shown with an accent left border) can be operated on with single-key shortcuts:
  - `B` — add a new code cell below the selected cell
  - `A` — add a new code cell above the selected cell
  - `D D` — delete the selected cell (a confirmation dialog is always shown)
  - `Y` / `M` / `S` — change the selected cell's type to Code / Markdown / SQL
- Kernel status indicator: starting spinner → "Kernel ready" → "Kernel error"

---

## UI & Theming

- **Light, Dark, and System** themes — switch via **View → Appearance**; preference is saved across sessions
- **Session restoration across app restarts** — all open tabs (scratch SQL, file tabs, notebook tabs) and their SQL content are restored exactly when the app is relaunched; file-backed tabs re-read their content from disk on startup so they always show the current file; if a file has been deleted or moved the tab becomes a scratch tab (prefixed `↺`) so the last-known SQL content is not lost; window size is saved on quit and restored on the next launch
- **Tools menu** — native menu bar **Tools** entry provides **Code Snippets…**, **Export Path Format…**, **Schema Migration…**, and **Create dbt Project…**
- **Snowpark menu** — native menu bar **Snowpark** entry provides **Check Environment…**, **Setup Environment…**, **New Notebook…**, and **Open Notebook…**
- **Help menu** — **Function Catalog…** opens the built-in Snowflake function reference with an **Ask AI** tab for chatting about any selected function (see below); **Keyboard Shortcuts…** opens a searchable modal listing every shortcut with macOS and Windows columns
- **Resizable sidebars** — drag either sidebar edge to any width between 160 px and 600 px
- **Resizable editor/results split** — drag the horizontal divider between the SQL editor and the results pane to any ratio; position is saved across sessions
- **Drag-and-drop panel layout** — every sidebar panel (Export DDL, File Browser, Git, Object Browser, Administration) has a drag handle at its top edge; drag panels between the left and right sidebars or reorder them within a sidebar; layout is persisted across sessions
- **Reset Layout** — restore the default panel positions and editor/results split via the **Customize Layout…** dialog (accessible from the **View** menu)
- **Resizable object browser** — collapse, expand, or drag to resize the object tree panel
- Right-click context menus are always clamped inside the viewport
- Closing the app while a query is running prompts a confirmation dialog; the query is cancelled in Snowflake before exit

---

## Keyboard Shortcuts

Open **Help → Keyboard Shortcuts…** in the menu bar for a searchable, always-up-to-date reference.

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

*Thaw is built with Go, Wails, React, Ant Design, Monaco Editor, and Ag-Grid.*
