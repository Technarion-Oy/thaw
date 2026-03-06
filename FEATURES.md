# Thaw — Feature Overview

Thaw is a native desktop application for Snowflake — built for analysts, engineers, and administrators who need a fast, capable SQL environment beyond the Snowflake web UI.

---

## SQL Editor

- **Monaco-based editor** with full SQL syntax highlighting and rich keyboard shortcuts
- **Multi-tab editing** — open multiple files simultaneously; each tab remembers its SQL, results, and scroll position
- **Run selected text** — highlight any portion of a query and run only that part (`⌘ Enter` / `Ctrl+Enter`)
- **Cancel queries** — cancel a running query at any time; Thaw issues `SYSTEM$CANCEL_QUERY` so it also stops consuming Snowflake credits
- **Query ID** — the Snowflake Query ID is shown while running and after completion; click to copy
- **Selection highlight** — selecting text highlights every other occurrence in the document; overview-ruler markers show occurrences in long files
- **Hover definitions** — hover over any table or view name to see its DDL in a tooltip; definitions are cached per session
- **SQL autocomplete** — context-aware completions:
  - `db.` → schemas in that database
  - `db.schema.` → tables, views, functions, and other objects in that schema
  - `db.schema.table.` → columns of that table or view
  - `Ctrl+Space` inside a query → columns from all tables referenced in the current `FROM`/`JOIN` clauses
- **AI inline completions** — ghost-text SQL suggestions powered by OpenAI or Google AI Studios (Gemini); press `Tab` to accept
- **AI Chat** — an agentic assistant in the results area that can query your live Snowflake connection to answer questions about your data (see [AI Features](#ai-features))
- **Unsaved-change indicator** — a `•` dot in the tab title shows unsaved work at a glance
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the insertion point

---

## Object Browser

- Browse all databases → schemas → tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, and pipes
- **Search** — filter objects by name across all databases and schemas in real time
- **Right-click tables and views** to:
  - Select the top 1,000 rows (opens in a new tab)
  - **Time Travel Query** — drag a timeline slider to query data at any past point within the retention window
  - **Export Data** — download table data as CSV, JSON, or Parquet via a temporary Snowflake stage
  - **Import Data** — upload a local file into Snowflake; supports CSV, JSON, and Parquet; can create a new table automatically by inferring the schema
  - **Insert Full Name** — insert the fully-qualified `"DB"."SCHEMA"."OBJECT"` identifier at the cursor
  - View DDL definition inline
  - **Rename** the object
  - **Drop** the object (with confirmation)
  - **Select for Comparison** / **Compare with** — side-by-side DDL diff (see [Text Comparison](#text-comparison))
- **Right-click a database** to export its DDL, generate an ER Diagram, or view dropped schemas recoverable via Time Travel
- **Right-click a schema** to view dropped tables or create a new Snowflake Task
- **Drag and drop** — drag any table or view into the editor to insert a `SELECT` statement with all column names listed individually
- **Hover tooltips** — hovering any object in the tree shows its DDL definition
- **View Definition** — opens the DDL in a modal with a Copy button
- **Properties** — opens a key/value panel of object metadata populated from the relevant `SHOW` command
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

**Tools available to the assistant:**

| Tool | What it does |
|------|-------------|
| `get_session_context` | Returns the active role, warehouse, database, and schema |
| `list_databases` | Lists all databases accessible to the current role |
| `list_schemas` | Lists all schemas in a database |
| `list_tables` | Lists all tables and views in a schema |
| `describe_table` | Returns column names and data types |
| `run_sql` | Executes a SQL query and returns up to 50 rows |

- **Context injection** — the current SQL in the editor and the most recent query result are automatically included so the assistant has full context
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
- Parallel export — up to 8 databases fetched concurrently
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

## Account & User Management

- View all roles, warehouses, and users in the account
- **User Management** — search users by name, login, display name, or email; view disabled accounts at a glance
- **Create User** — dialog with all user properties and a live `CREATE USER` SQL preview
- **Edit User** — pre-populated form that generates only the `ALTER USER … SET/UNSET` statements needed for the changed fields
- **Enable / Disable / Drop** users with a single right-click action
- All user management actions are automatically hidden or greyed out when the current role lacks the required privileges

---

## Results & Export

- Query results displayed in a virtualised grid — handles large result sets smoothly
- **Export results** — CSV (RFC 4180) and Excel (`.xlsx`) export with a native save dialog
- Column sorting and horizontal scrolling

---

## Snowflake Connectivity

- Connect with account / user / password / warehouse / role
- **Auto-fill from Snowflake CLI** — reads `~/.snowflake/config.toml` and populates the connection form from any saved profile, including key-pair (`SNOWFLAKE_JWT`) profiles; authenticator values are matched case-insensitively so both `snowflake_jwt` and `SNOWFLAKE_JWT` work
- **Cancel connection** — abort an in-progress connection attempt
- **Switch role or warehouse** from the toolbar without disconnecting — all subsequent queries, privilege checks, and object browsing immediately reflect the new session state
- Role dropdown shows only roles the current user can actually assume

---

## UI & Theming

- **Light, Dark, and System** themes — switch via **View → Appearance**; preference is saved across sessions
- **Resizable sidebars** — drag either sidebar edge to any width between 160 px and 600 px
- **Resizable editor/results split** — drag the horizontal divider between the SQL editor and the results pane to any ratio; position is saved across sessions
- **Drag-and-drop panel layout** — every sidebar panel (Export DDL, File Browser, Git, Object Browser, Account Objects) has a drag handle at its top edge; drag panels between the left and right sidebars or reorder them within a sidebar; layout is persisted across sessions
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

---

*Thaw is built with Go, Wails, React, Ant Design, Monaco Editor, and Ag-Grid.*
