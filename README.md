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
- Run the full query or just the selected text (`⌘ Enter` / `Ctrl Enter`)
- Open `.sql` files from the file browser and run them directly
- Results displayed in a virtualised Ag-Grid table

### Object browser (sidebar)
- Browse databases → schemas → objects (tables, views, functions, procedures, …)
- View the DDL of any object inline
- Call stored procedures from the UI

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

### File browser
- Browse the export working directory in the sidebar
- Lazy-loads subdirectories on demand
- Click any file to open it in the Monaco editor
- Auto-refreshes after a DDL export completes

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
- Dark theme throughout

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
├── main.go                        # Wails entry point, window configuration
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
│   ├── filesystem/fs.go           # Directory listing and file reading
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
    │   ├── main.tsx
    │   ├── styles/global.css
    │   ├── store/
    │   │   ├── connectionStore.ts # Connection state (Zustand)
    │   │   ├── gitStore.ts        # Git / export directory state (Zustand)
    │   │   ├── objectStore.ts     # Object browser state (Zustand)
    │   │   ├── queryStore.ts      # Query / result / open-file state (Zustand)
    │   │   └── sessionStore.ts    # Active role & warehouse (Zustand)
    │   ├── pages/
    │   │   └── QueryPage.tsx      # Main query workspace
    │   └── components/
    │       ├── connection/ConnectModal.tsx
    │       ├── editor/SqlEditor.tsx
    │       ├── export/ExportPanel.tsx
    │       ├── files/FileBrowser.tsx
    │       ├── git/
    │       │   ├── GitPanel.tsx
    │       │   └── CommitModal.tsx
    │       ├── procedure/CallProcedureModal.tsx
    │       ├── results/ResultGrid.tsx
    │       └── layout/
    │           ├── AppLayout.tsx  # Resizable sidebar
    │           └── Sidebar.tsx
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

---

## Keyboard shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘ Enter` / `Ctrl Enter` | Run the current SQL query (or selected text) |

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
