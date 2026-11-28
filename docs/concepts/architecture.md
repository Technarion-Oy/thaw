# Architecture

Thaw is a native desktop Snowflake manager built with **Wails v3**: a Go backend and a React/TypeScript frontend compiled into a single binary. The frontend is embedded via `//go:embed all:frontend/dist` in `main.go`.

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (React 19 + TS, Ant Design, Monaco, TanStack)      │
│  pages/ · components/ · store/ (Zustand) · utils/            │
└───────────────▲─────────────────────────────┬───────────────┘
                │  @wailsio/runtime           │  frontend/bindings/ (auto-generated)
                │  EventsOn(...)              │  app.ts / service.ts calls
┌───────────────┴─────────────────────────────▼───────────────┐
│  Wails v3 runtime bridge                                     │
└───────────────▲─────────────────────────────┬───────────────┘
                │  a.emit(...)                │  method dispatch
┌───────────────┴─────────────────────────────▼───────────────┐
│  Go backend                                                  │
│  internal/app  (App service — all IPC methods, thin delegators)│
│  internal/sqleditor.Service (separate Wails-bound service)   │
│  internal/<domain> packages (SQL builders, parsers, logic)   │
│  internal/snowflake (gosnowflake driver wrapper + pool)      │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
                      Snowflake account
```

## IPC flow

1. A component calls a generated binding in `frontend/bindings/thaw/internal/app/app.ts` (or `frontend/bindings/thaw/internal/sqleditor/service.ts` for SQL-editor analysis).
2. The Wails v3 runtime dispatches to the matching Go method on `*App` (package `internal/app`) or the bound `Service`.
3. `*App` methods are **thin delegators**: nil-check the connection, then call into an `internal/<domain>` package that does the real work.
4. Results flow back as JSON; long-running work emits Wails events (`a.emit("event:name", payload)`) which components subscribe to via `EventsOn(...)` from `frontend/src/utils/wailsRuntime.ts` (which unwraps Wails v3's `event.data` payload).

`frontend/bindings/` is **auto-generated** by `wails3 generate bindings -ts -i -clean=true` — never edit it by hand. After changing any `*App` or `sqleditor.Service` method signature, regenerate.

## Backend package map

`internal/app/` holds the single `App` service (lifecycle, tab-session management, `Connect`/`Disconnect`) plus all IPC methods, split across files by domain (`query.go`, `objects.go`, `warehouse.go`, …). `run.go` wires `application.New`, `Services`, and the main window; `menu.go` builds the native menu; `runtime.go` isolates Wails v3 runtime helpers.

Everything else under `internal/` is a focused domain package. Each has its own `README.md`; highlights:

| Package | Role |
|---------|------|
| `snowflake` | gosnowflake driver wrapper, session-state-aware pool, object-listing TTL cache, result-parsing helpers |
| `objectkind` | the canonical object-kind registry (KIND, SHOW plural, label, order, `GET_DDL` type) every backend consumer derives from, and the source of the generated frontend artifact — a leaf package with no Thaw imports |
| `sqleditor` | SQL diagnostics & JOIN-suggestion engine (its own Wails-bound `Service`) |
| `ddl` | statement splitter + DDL metadata extraction + parallel export pipeline |
| `config` | TOML/JSON app config, feature flags, IT-admin enforced policies (secrets scrubbed out — see `secrets`) |
| `secrets` | OS-native credential store (macOS Keychain / Windows Credential Manager / Linux Secret Service; `0600` file fallback) for Thaw-owned secrets kept out of `config.json` |
| `tasks`, `warehouse`, `table`, `column`, `pipe`, `stage`, `secret`, `backup`, `objects`, `queryhistory`, `queryprofile`, `dbtproject`, `snowgitrepo`, `integrations`, `keypair`, `fileformat`, `procedure` | per-object SQL builders + parsers (thin-delegator domains) |
| `migration`, `snowpark` | larger features exposing their own `Service` |
| `gitrepo` | local git via go-git; `filesystem` | file I/O + FS watcher; `sfconfig` | `~/.snowflake/config.toml` reader/writer |
| `ai` | OpenAI/Google/Ollama HTTP clients; `fnmeta` | function-catalog SQLite cache |
| `logger`, `crashreport`, `session`, `sysinfo`, `version`, `apperrors`, `architecture` | cross-cutting infrastructure |

See [`internal/app/README.md`](../../internal/app/README.md) for the full file-by-file breakdown.

## Frontend layout

- **`store/`** — ~14 Zustand stores. Core ones: `connectionStore`, `queryStore` (SQL tabs/results), `objectStore` (sidebar tree + cache), `sessionStore`, `themeStore`, `panelLayoutStore`, `gridStore` (results grid — a singleton, see gotchas), `featureFlagsStore`, `mcpStore`, `notebookToolbarStore`.
- **`pages/`** — top-level page components (`QueryPage` orchestrates the workspace).
- **`components/`** — feature UI grouped by domain (`editor/`, `results/`, `layout/`, `sidebar/`, `toolbar/`, `notebook/`, plus per-object-type modal folders).
- **`utils/`, `schemas/`, `styles/`** — helpers, validation/JSON schemas, and global CSS.

Each folder has its own `README.md`.

## Where data lives

- **App config** (connection, git, AI, feature flags, session tuning): `config.json` (`0600` for the API key) in the OS app-support dir.
- **Window/session state**: `session.json` in the OS data dir.
- **Snowflake CLI profiles**: `~/.snowflake/config.toml` (read and text-level-edited, preserving comments).
- **Function-catalog cache**: SQLite via `internal/fnmeta`.
- **Logs & crash reports**: OS-specific log dir via `internal/logger` / `internal/crashreport`.

See `README.md` → Configuration for exact paths per platform.

## Related

- [Patterns](patterns.md) · [Gotchas](gotchas.md) · [Onboarding](onboarding.md)
- The Draw.io system diagram and its maintenance rules: [`ARCHITECTURE_DIAGRAM.md`](ARCHITECTURE_DIAGRAM.md).
