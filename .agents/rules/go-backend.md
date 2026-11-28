---
trigger: glob
globs: "internal/**/*.go, main.go"
description: "Go backend conventions, thin-delegator IPC pattern, SQL builder safety, config/secrets, and Wails/gosnowflake invariants for Thaw."
---

# Thaw Go Backend Rules

Follow these invariants whenever creating or editing Go files in `internal/` or `main.go`. Full context: `docs/concepts/patterns.md` and `docs/concepts/gotchas.md`.

## Architecture & Semantic Map
- Check `internal/architecture/semantic_map.go` before adding or moving packages. Every `internal/<pkg>/` package must declare `// thaw:domain: <Domain Name>` (canonically in `doc.go`), and `main.go` declares `// thaw:file-domain: <Domain Name>`. Run `go generate ./internal/architecture/` after adding/moving annotated paths.
- Keep `internal/app/` methods as **thin delegators**: nil-check (`apperrors.ErrNotConnected`), delegate to `internal/<domain>`, and return domain types (types belong in `internal/<domain>`, not `internal/app`). Put new `*App` bindings in the matching `internal/app/<domain>.go` file; reserve `app.go` for struct/lifecycle/session management, `run.go` for Wails wiring, and `menu.go` for the native menu.
- Never import `internal/app` from `internal/mcp` or other domain packages (creates an import cycle). Use injected callbacks/interfaces (e.g. `emit` closures, `NotebookBackend`).
- Keep `internal/objectkind` and `internal/sqltok` as **leaf packages** (no Thaw imports).
- Never remove `limitAssetConcurrency(32)` in `internal/app/run.go` (`wails dev` WKWebView asset burst workaround).

## SQL Builders & Parsing
- Split pure `Build*Sql(...) (string, error)` builders and `Parse*(res *snowflake.QueryResult)` parsers in `internal/<domain>` so they are unit-testable without a live Snowflake connection.
- Use the **validated-wrapper variant** (see `internal/queryhistory`) whenever a builder embeds an unquoted/pre-validated value: keep the builder unexported, validate in the exported `Get*` wrapper, and panic on invariant violation in the builder.
- Quote all Snowflake identifiers with double quotes (`"DB"."SCHEMA"."OBJ"`) and escape embedded `"` as `""` via `internal/snowflake` helpers. Never concatenate raw user input.
- Reuse shared result-parsing helpers in `internal/snowflake/result.go` (`ColIdx`, `Cell*`, `PropertyPair`, `ResultToPairs`).
- **Never hand-roll SQL lexing** (`strings.HasPrefix`/`ToUpper`/`Split` on SQL). Use `internal/sqltok` (`FirstToken`, `SignificantTokens`) and `internal/snowflake` (`SplitQualifiedName`, `IdentEqual`).

## Wails IPC & Serialization Gotchas
- **Always run `wails generate module`** after changing any exported method signature on `*App` or a Wails-bound `Service` (`internal/sqleditor/service.go`, `internal/snowpark/`, `internal/migration/`). Never hand-edit `frontend/wailsjs/`.
- **Never return a nil Go slice across IPC**: Wails marshals nil slices to JSON `null`, not `[]`. Always initialize with `make([]T, 0, n)` or `[]T{}`.
- **Format `time.Time` as RFC3339 `string`** on IPC structs so Wails generates clean TypeScript `string` fields without `Not found: time.Time` warnings.

## Config, Secrets, Filesystem & macOS TCC
- All read-modify-writes of app config MUST use `config.Update(fn)` (process-locked + atomic write), never `Load()` → mutate → `Save()`.
- Never persist Thaw-owned secrets (AI keys, OAuth client secrets, pip passwords, MCP tokens) in `config.json` — store them in `internal/secrets` (OS keychain / `0600` fallback).
- All mutating filesystem operations under a workspace root must validate containment with `internal/filesystem` symlink-resolving path validators (`ValidateInsideOrEqual` / `ValidatePathOrAncestorInsideOrEqual`).
- On macOS, direct `os.ReadFile` inside TCC-protected folders (`~/Documents`, `~/Desktop`, `~/Downloads`) fails with `EPERM` without prompting — prefer `wailsruntime.OpenFileDialog` and surface actionable hints on `os.ErrPermission`.

## Snowflake Driver (`gosnowflake`) Gotchas
- `gosnowflake` logs ALL query errors at ERROR level via `slog`, even when caught. Always determine an object's kind before calling `GET_DDL` instead of guessing `TABLE` vs `VIEW`.
- `sf.WithQueryIDChan` writes the query ID and **closes the channel itself** — never call `close(qidChan)` manually.

## Native Menus (`internal/app/menu.go`)
- Use `addConnectionItem(...)` (not `AddText`) for menu items requiring a live Snowflake connection.
- Nil-guard runtime menu callbacks (`setMenuConnected`, `setMenuFeatureFlags`, `setQueryLogMenuCheck`) because unit tests construct `App` without a menu.
