# Thaw — Technical Instructions

Deep build, test, and architecture reference for contributors. The [README](../README.md)
covers what Thaw is and how to stand up a developer workspace; this document collects the
detailed material and links out to the canonical docs rather than duplicating them.

| Topic | Canonical source |
|-------|------------------|
| Architecture (IPC bridge, package map, data flow) | [`docs/concepts/architecture.md`](concepts/architecture.md) |
| Getting oriented in the codebase | [`docs/concepts/onboarding.md`](concepts/onboarding.md) |
| Engineering patterns (thin delegators, stores, feature flags, sidebar keys) | [`docs/concepts/patterns.md`](concepts/patterns.md) |
| Critical traps to read before debugging | [`docs/concepts/gotchas.md`](concepts/gotchas.md) |
| Testing (unit, race, frontend, integration) | [`docs/concepts/testing.md`](concepts/testing.md) |
| Branching, commits, PRs, docs-with-code rule | [`CONTRIBUTING.md`](../CONTRIBUTING.md) |
| Full user-facing feature list | [`FEATURES.md`](../FEATURES.md) |
| Per-package reference | `internal/<pkg>/README.md` |
| Per-folder frontend reference | `frontend/src/<dir>/README.md` |
| Generated API reference (TypeDoc + gomarkdoc) | `make docs` → [`docs/`](README.md) |

---

## Build

### Production binary

```bash
wails build
```

The output binary is placed in `build/bin/`.

The frontend build pipeline applies two layers of IP protection automatically:
- **Terser** (2-pass minification, no source maps, dead code and console removal)
- **javascript-obfuscator** (RC4 string-array encoding, hexadecimal identifier renaming,
  wrapper chains) — vendor and Monaco chunks are excluded to avoid breaking their internal
  protocols

The build script allocates 6 GB of Node heap (`--max-old-space-size=6144`) to accommodate
the obfuscator's memory usage.

**CI release builds** are triggered by running the release pipeline, which automatically
creates and pushes the version tag. Artifacts for macOS (arm64), Windows (amd64), and
Linux (amd64) are produced and named after the tag.

### Regenerating the Wails bindings

After changing any exported method signature on the `*App` struct (`internal/app/`), regenerate
the JS bindings in `frontend/wailsjs/`:

```bash
wails generate module
```

---

## Project structure

```
thaw/
├── main.go                        # Thin entry point: //go:embed frontend/dist + app.Run(assets)
├── go.mod
├── wails.json                     # Wails project configuration
├── build/
│   ├── darwin/                    # macOS app icons
│   └── windows/                   # Windows resources
├── internal/
│   ├── ai/ai.go                   # AI provider HTTP clients (OpenAI, Google AI Studios, Ollama); inline completions; model listing and testing
│   ├── app/                        # Wails-bound App struct (package app): app.go (lifecycle), run.go (wails.Run wiring), menu.go (native menu), + IPC methods split by domain (query.go, objects.go, …). Most methods are thin delegators (nil-check → domain-package func → return); real logic lives in the domain packages below
│   ├── apperrors/                  # Sentinel errors (ErrNotConnected etc.)
│   ├── backup/                    # Backup sets/policies: SHOW parsers + CREATE/ALTER/RESTORE SQL builders (BackupSetRow, BackupPolicyRow, BackupRow)
│   ├── config/config.go           # Saved git / export / AI settings
│   ├── crashreport/crashreport.go # Panic handler; writes JSON crash file; remote-send placeholder
│   ├── ddl/
│   │   ├── parser.go              # SQL statement splitter (state machine)
│   │   ├── object.go              # Metadata extraction + file-path generation
│   │   └── exporter.go            # Parallel DDL export orchestration (cancellable)
│   ├── dbt/generator.go           # Pure dbt project file generator (no Snowflake calls)
│   ├── sqleditor/                 # SQL diagnostics & JOIN condition engine (Wails-bound Service)
│   ├── fileformat/                # File format DDL builder and local preview (CSV, JSON, AVRO, ORC, PARQUET, XML)
│   ├── filesystem/fs.go           # Directory listing, file reading and writing
│   ├── fnmeta/                    # Function catalog metadata (SQLite cache + embedded JSON fallback + live sync)
│   ├── gitrepo/repo.go            # Git operations via go-git (status, commit/push, pull, clone, branches)
│   ├── keypair/                   # RSA key-pair generation + ALTER USER RSA_PUBLIC_KEY builder
│   ├── integration/               # Integration tests (build-tag gated; require live Snowflake)
│   ├── integrations/              # CREATE INTEGRATION SQL builders (Storage, API, Catalog, External Access, Notification, Security)
│   ├── logger/                    # slog + lumberjack setup; OS-specific log paths
│   ├── migration/                 # Schema migration engine (Service pattern with NewService)
│   ├── objects/                   # Object-properties query builders + column-comment parse/set
│   ├── pipe/                      # Pipe management: CREATE PIPE SQL builder, copy history, COPY validation
│   ├── procedure/                 # Procedure/function call statement builder (CALL, SELECT)
│   ├── queryhistory/              # QUERY_HISTORY table-function SQL builder + row parser
│   ├── queryprofile/              # Query execution profile and EXPLAIN plan parser
│   ├── secret/                    # Secret management: CREATE/ALTER SECRET SQL builder
│   ├── session/                   # Window state persistence (load/save, OS-specific paths)
│   ├── sfconfig/                  # Snowflake CLI config reader/writer (~/.snowflake/config.toml)
│   ├── snowflake/                 # Snowflake driver wrapper + DDL-based lineage parser
│   ├── snowgitrepo/               # Snowflake Git repository integration SQL builder
│   ├── dbtproject/                # Snowflake-native DBT PROJECT objects: CREATE/ALTER/EXECUTE builders
│   ├── column/                    # Table column DDL builders (ADD/DROP/RENAME/ALTER COLUMN)
│   ├── mcp/                        # MCP servers (Go MCP SDK): multi-session manager, SSE/HTTP transport
│   ├── snowpark/                   # Snowpark/Jupyter support (Service pattern with NewService)
│   ├── stage/                     # Stage creation SQL builder
│   ├── sysinfo/                   # Host system info (MemoryGB via sysctl)
│   ├── table/                     # Table-summary/settings queries + ALTER TABLE property builder
│   ├── tasks/                     # Task graph management: schedule parsing, execution history
│   ├── version/                   # Version string (set via -ldflags)
│   └── warehouse/                 # ALTER WAREHOUSE property builder + metering-history query/parse
└── frontend/
    ├── index.html
    ├── vite.config.ts
    ├── package.json
    ├── src/
    │   ├── App.tsx                # Root component, Ant Design theme
    │   ├── main.tsx               # React entry point
    │   ├── store/                 # Zustand stores (~14 stores)
    │   ├── pages/QueryPage.tsx    # Main query workspace
    │   └── components/            # Feature components by domain (~30 directories)
    └── wailsjs/                   # Auto-generated Go→JS bridge (do not edit)
```

A per-package `README.md` documents the types, builders, parsers, and gotchas of every
`internal/<pkg>/` and `frontend/src/<dir>/`. The authoritative, machine-checked domain map
is `internal/architecture/semantic_map.go` (generated — see [CLAUDE.md](../CLAUDE.md)).

---

## Testing

Tests live alongside the production code inside each package. The Go tests use only the
standard `testing` package; the frontend uses [vitest](https://vitest.dev/). See
[`docs/concepts/testing.md`](concepts/testing.md) for the full guide, including the lint and
security gates.

```bash
go test ./...                      # all Go tests
go test ./internal/ddl/...         # a single package
go test -v -run TestSplit ./internal/ddl/   # a single named test
go test -race ./...                # race detector
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

cd frontend && npm test            # frontend unit tests (vitest)
cd frontend && npm run test:watch  # watch mode
cd frontend && npx tsc --noEmit    # type-check only
```

> **Linux note** — the `migration`, `snowpark`, and other Wails-importing packages need CGO +
> GTK/WebKit headers:
> ```bash
> sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
> ```

### Integration tests

Integration tests live in `internal/integration/` behind the `integration` build tag, so
they are **never** run by `go test ./...`. They require a real Snowflake account and are
**skipped** (not failed) when the required environment variables are absent — safe for CI
without Snowflake access.

```bash
export SNOWFLAKE_ACCOUNT=myorg-myaccount
export SNOWFLAKE_USER=my_user
export SNOWFLAKE_PRIVATE_KEY="$(cat rsa_key.p8)"   # key-pair auth
export SNOWFLAKE_WAREHOUSE=COMPUTE_WH
export SNOWFLAKE_ROLE=SYSADMIN                      # optional

go test -v -tags integration -timeout 10m ./internal/integration/
```

| Variable | Description |
|---|---|
| `SNOWFLAKE_ACCOUNT` | Account identifier, e.g. `myorg-myaccount` |
| `SNOWFLAKE_USER` | Login name |
| `SNOWFLAKE_PRIVATE_KEY` | PEM-encoded RSA private key (key-pair authentication) |
| `SNOWFLAKE_WAREHOUSE` | Warehouse to use |
| `SNOWFLAKE_ROLE` | *(optional)* Role to assume; needs `CREATE DATABASE` for migration tests |
| `SNOWFLAKE_TEST_DATABASE` | *(optional)* Migration test database; auto-created if unset |
| `SNOWFLAKE_TEST_SCHEMA` | *(optional)* Schema within the test database; defaults to `PUBLIC` |

The user (or a role it holds) needs at minimum:

```sql
GRANT CREATE DATABASE ON ACCOUNT TO ROLE <role>;
GRANT USAGE ON WAREHOUSE <warehouse> TO ROLE <role>;
```

All other object privileges are granted to the owner of the database the test creates.

---

## Code quality & security

Three automated checks run on a weekly schedule (Mondays 06:00 UTC) and on demand from the
GitHub Actions UI. All three run locally too. See [`docs/concepts/testing.md`](concepts/testing.md).

```bash
# golangci-lint — static analysis (errcheck, govet, staticcheck, ineffassign, unused, misspell, revive)
golangci-lint run ./...            # config: .golangci.yml · workflow: .github/workflows/lint.yml

# govulncheck — reachable-vulnerability scanning against https://vuln.go.dev/
govulncheck ./...                  # workflow: .github/workflows/govulncheck.yml

# gosec — security static analysis (same exclusions as CI)
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...
                                   # workflow: .github/workflows/gosec.yml
```

---

## Development workflow

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the full workflow (branching, commits, PRs, the
docs-with-code rule, and quality gates) and [`docs/concepts/patterns.md`](concepts/patterns.md)
for engineering patterns. The essentials:

- **Backend changes** — edit any `.go` file; `wails dev` recompiles automatically.
- **Frontend changes** — edit files under `frontend/src/`; Vite HMR updates the UI instantly.
- **Adding a backend method** — add the method (on `*App`) to the matching
  `internal/app/<domain>.go`, then run `wails generate module`.
- **Adding a Go package** — place it under `internal/`, give it a `README.md`, and import it
  from the relevant `internal/app/<domain>.go`.
- **Adding a native menu item** — extend `buildMenu` in `internal/app/menu.go`; emit a Wails
  event from the callback and listen with `EventsOn` in the frontend.

---

## Keyboard shortcuts

Open **Help → Keyboard Shortcuts…** in the app for a searchable, always-up-to-date reference.

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
| `⌘G` | `Ctrl+G` | Toggle grid search (results pane only) |
| `⌘E` | `Ctrl+E` | Export current results as CSV |

### Editor

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘/` | `Ctrl+/` | Toggle line comment |
| `⇧⌥A` | `Shift+Alt+A` | Toggle block comment |
| `⇧⌥F` | `Shift+Alt+F` | Format SQL (selection or full document) |
| `Ctrl+Space` | `Ctrl+Space` | Trigger autocomplete |
| `Tab` | `Tab` | Accept AI suggestion |
| `⌘F` | `Ctrl+F` | Find in document |
| `⌘⌥F` | `Ctrl+H` | Find and replace |
| `⌘⇧H` | `Ctrl+Shift+H` | Find & replace across tabs |
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
| `` ⌘` `` | `` Ctrl+` `` | Open embedded terminal |

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

## Configuration & file locations

Git and export settings (remote URL, branch, export directory, path template, author info,
AI provider settings) are stored at:

- **macOS** — `~/Library/Application Support/thaw/config.json`
- **Linux** — `~/.config/thaw/config.json`
- **Windows** — `%APPDATA%\thaw\config.json`

**Git tokens are never written to disk.** The AI API key is written with mode `0600`
(owner-read-only).

Session state (window size and tab list):

| Build | Path |
|---|---|
| Development (`wails dev`) | `./thaw-session.json` |
| macOS production | `~/Library/Application Support/thaw/session.json` |
| Windows production | `%LOCALAPPDATA%\thaw\session.json` |
| Linux production | `~/.local/share/thaw/session.json` (or `$XDG_DATA_HOME/thaw/session.json`) |

Log and crash files:

- **macOS** — `~/Library/Logs/thaw/`
- **Linux** — `~/.local/state/thaw/` (or `$XDG_STATE_HOME/thaw/`)
- **Windows** — `%APPDATA%\thaw\logs\`

Snowflake CLI connection profiles are read from and written to `~/.snowflake/config.toml`.
The writer uses text-level TOML manipulation to preserve user comments and unknown keys.
