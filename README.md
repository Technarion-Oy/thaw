# Thaw — Snowflake Manager

A desktop application for Snowflake management: browsing objects, running SQL queries, exporting DDL to a git repository, and pushing changes via CI/CD workflows.

**Stack:** Go · Wails v2 · React · Ant Design · Monaco Editor · Ag-Grid

---

## Features

### Snowflake connectivity
- Connect with account / user / password / warehouse / role
- Auto-fill connection form from `~/.snowflake/config.toml` (Snowflake CLI profiles)
- Cancel an in-progress connection attempt
- Switch role or warehouse from the query toolbar without reconnecting

### SQL editor
- Monaco editor with full SQL syntax highlighting
- Multi-tab editing — each open file gets its own tab; tabs restore their SQL, results and error state when switched back to
- Unsaved changes shown with a `•` prefix in the tab title
- Run the full query or just the selected text (`⌘ Enter` / `Ctrl Enter`)
- **Query ID** — the Snowflake query ID is shown in the loading spinner while the query runs and in the results status bar after it completes; click the copy icon to copy it to the clipboard
- **Selection highlight** — selecting any text highlights every other occurrence in the document with a blue background; overview-ruler markers make occurrences visible in long files
- Word-under-cursor highlight when nothing is selected
- **Hover definition** — hovering over a table or view name shows its DDL in a Monaco tooltip; definitions are cached per session so subsequent hovers are instant
- Results displayed in a virtualised Ag-Grid table

### File management
- **Open…** (`⌘O` / `Ctrl+O`) — native OS open-file dialog filtered to `.sql`; re-activates an existing tab if the file is already open
- **Save** (`⌘S` / `Ctrl+S`) — writes back to the file's original path
- **Save As…** (`⌘⇧S` / `Ctrl+Shift+S`) — native OS save dialog with `.sql` filter; also promotes a scratch tab to a named file tab
- **New Tab** (`⌘T` / `Ctrl+T`) — opens a blank scratch tab
- All four actions are available in the **File** menu in the macOS/Windows menu bar as well as in the toolbar

### Object browser (sidebar)
- Browse databases → schemas → objects (tables, views, functions, procedures, …)
- **Filter objects** — type in the search box at the top of the sidebar to filter objects by name across all databases and schemas; the tree cascade-loads all schemas and objects automatically and collapses back to the database list when the search is cleared
- Right-click a **database** to refresh, export its DDL, **insert its name** at the editor cursor, or **generate an ER Diagram**
- Right-click a **schema** to browse dropped tables recoverable via Snowflake Time Travel or **insert its fully-qualified name** at the editor cursor
- Right-click an **object** to:
  - Select the top 1 000 rows (tables and views) — opens in a new tab
  - **Time Travel Query…** (tables) — opens a dialog with a timeline slider spanning the table's full retention window; drag to choose a point in time and run `SELECT … AT(TIMESTAMP => …) LIMIT 1000` in a new tab
  - **Export Data…** (tables) — export table data to the local machine via a temporary internal Snowflake stage; choose format (CSV, JSON, PARQUET), compression, delimiter, header row, and output directory; the stage is dropped automatically after the download
  - **Import Data…** (tables) — import a local file into a Snowflake table via a temporary internal stage; choose format (CSV, JSON, PARQUET) with format-specific options; the file picker filters to the selected format's extensions automatically; supports two modes:
    - **Import into existing table** — optionally truncate before loading (overwrite mode)
    - **Create new table from data** — derives the schema from the file using `INFER_SCHEMA` (CSV with headers and PARQUET) or creates a `VARIANT` column table (JSON); the object browser refreshes automatically on success
  - Call the procedure with auto-generated parameter fields (procedures) — opens in a new tab
  - **Call Function…** (functions) — opens a parameter dialog with auto-generated fields; detects scalar vs. table functions from the DDL and generates the correct SQL (`SELECT func(args) AS result` or `SELECT * FROM TABLE(func(args))`); opens in a new tab
  - **Insert Full Name** — inserts the fully-qualified `"DB"."SCHEMA"."NAME"` at the current editor cursor position
  - View the DDL definition inline
  - **Rename** the object (`ALTER … RENAME TO`) — available for tables, views, sequences, stages, streams, tasks, file formats, and pipes
  - **Delete** the object (`DROP …`) — with a confirmation dialog
- **Hover tooltip** — hovering over any object in the tree shows its DDL definition; fetched once and cached for the session
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

### DDL export
- Export DDL for every database (or a single one) with one file per object
- Fully qualified names (`db.schema.object`) in every CREATE statement
- Shared / imported databases (e.g. `SNOWFLAKE_SAMPLE_DATA`) are automatically skipped
- Files are organised on disk by schema and object type:
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
- Parallel fetch (up to 8 databases concurrently) and parallel atomic writes
- Live progress bar driven by Wails events from the Go backend
- Export directory can be changed directly from the Export DDL panel without opening the Git section

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

### UI
- Resizable sidebar — drag the divider to any width between 160 px and 600 px
- **Theming** — light, dark, and system-default themes; switch via **View → Appearance** in the native menu bar; preference is persisted across sessions
- Native application menu bar with **File** (open / save / new tab) and **View → Appearance** (System / Light / Dark) menus
- Object browser scrolls horizontally when object names are wider than the sidebar
- Right-click context menu is always clamped inside the viewport — never overflows the screen edges

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
├── go.mod
├── wails.json                     # Wails project configuration
├── build/
│   ├── darwin/                    # macOS app icons
│   └── windows/                   # Windows resources
├── internal/
│   ├── config/config.go           # Saved git / export settings
│   ├── ddl/
│   │   ├── parser.go              # SQL statement splitter (state machine)
│   │   ├── object.go              # Metadata extraction + file-path generation
│   │   ├── exporter.go            # Parallel DDL export orchestration
│   │   ├── parser_test.go
│   │   └── object_test.go
│   ├── filesystem/fs.go           # Directory listing, file reading and writing
│   ├── gitrepo/repo.go            # Git status, commit/push, pull
│   ├── integration/
│   │   └── export_test.go         # End-to-end tests (require live Snowflake account)
│   ├── sfconfig/reader.go         # Snowflake CLI config (~/.snowflake/config.toml)
│   └── snowflake/client.go        # Snowflake driver wrapper
└── frontend/
    ├── index.html
    ├── vite.config.ts
    ├── package.json
    ├── src/
    │   ├── App.tsx                # Root component, Ant Design dark theme
    │   ├── main.tsx               # React entry point; suppresses WebView context menu
    │   ├── styles/global.css      # Global styles incl. Monaco occurrence-highlight class
    │   ├── store/
    │   │   ├── connectionStore.ts # Connection state (Zustand)
    │   │   ├── gitStore.ts        # Git / export directory state (Zustand)
    │   │   ├── objectStore.ts     # Object browser state (Zustand)
    │   │   ├── queryStore.ts      # Multi-tab editor state (Zustand)
    │   │   ├── sessionStore.ts    # Active role & warehouse (Zustand)
    │   │   └── themeStore.ts      # Light/dark/system theme preference (Zustand, persisted)
    │   ├── pages/
    │   │   └── QueryPage.tsx      # Main query workspace; save handlers; menu event wiring
    │   └── components/
    │       ├── connection/ConnectModal.tsx
    │       ├── editor/
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
    │       ├── procedure/CallProcedureModal.tsx
    │       ├── results/ResultGrid.tsx
    │       └── layout/
    │           ├── AppLayout.tsx  # Resizable sidebar
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

---

## Keyboard shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘ Enter` / `Ctrl+Enter` | Run the current query (or selected text) |
| `⌘O` / `Ctrl+O` | Open a SQL file |
| `⌘S` / `Ctrl+S` | Save the active file |
| `⌘⇧S` / `Ctrl+Shift+S` | Save As… (always opens a dialog) |
| `⌘T` / `Ctrl+T` | New scratch tab |

---

## Configuration

Git and export settings are stored at:

- **macOS** — `~/Library/Application Support/thaw/config.json`
- **Linux** — `~/.config/thaw/config.json`
- **Windows** — `%APPDATA%\thaw\config.json`

The file stores the remote URL, branch, export directory, and author info.
**Git tokens are never written to disk.**

Snowflake CLI connection profiles are read from `~/.snowflake/config.toml` and
pre-fill the connection form, but are never modified by Thaw.

---

## License

Copyright © 2026 Technarion Oy. All rights reserved.

This software is proprietary and confidential. Unauthorized copying, distribution,
modification, or use — in whole or in part — is strictly prohibited without prior
written permission from Technarion Oy. Commercial use is restricted to parties
holding a valid license agreement with Technarion Oy.
