# Thaw — Claude Code Guide

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend embedded as a single binary).

## Development Workflow

- **Branching**: All changes must be made in a feature branch (e.g., `feat/`, `fix/`, `chore/`).
- **Git History**: NEVER alter git branch history. Do not use `git commit --amend`, `git rebase`, or `git push --force`. Always create new commits for updates.
- **Commits**: Use descriptive commit messages with conventional prefixes. The commit type determines whether a release is triggered and what version component is bumped:

  | Commit type | Release | Version bump |
  |-------------|---------|--------------|
  | `feat` | ✅ | **minor** (0.X.0) |
  | `feat!` / `BREAKING CHANGE` footer | ✅ | **major** (X.0.0) |
  | `fix`, `perf` | ✅ | **patch** (0.0.X) |
  | `refactor`, `chore`, `docs`, `style`, `test`, `build`, `ci` | ❌ | no release |
- **Pull Requests**: Create PRs using the GitHub CLI (`gh`). Target the `upstream` repository (`Technarion-Oy/thaw`) if working from a fork.
- **PR Commands**:
  ```bash
  git checkout -b feat/my-new-feature
  # ... make changes ...
  git add .
  git commit -m "feat: my new feature"
  git push -u origin feat/my-new-feature
  gh pr create --repo Technarion-Oy/thaw --base main --title "feat: my new feature" --body "Description..."
  ```

## Architecture

```
thaw/
├── main.go              # Entry point, native menu, Wails runtime setup
├── app.go               # All Wails IPC bindings (~2750 lines)
├── errors.go            # Sentinel errors (ErrNotConnected etc.)
├── version.go           # Version string
├── internal/
│   ├── snowflake/       # Snowflake client — connection, query, DDL, lineage
│   ├── sqleditor/       # SQL diagnostics & JOIN suggestion engine
│   │   ├── sqleditor.go
│   │   └── sqleditor_test.go
│   ├── ddl/             # DDL parsing and git-export pipeline
│   ├── ai/              # AI provider clients (OpenAI, Google)
│   ├── config/          # App config (TOML persistence)
│   ├── gitrepo/         # Git operations via exec
│   ├── filesystem/      # File read/write helpers
│   ├── sfconfig/        # Reads ~/.snowflake/config.toml
│   ├── logger/          # Logrus + lumberjack rotation
│   ├── telemetry/       # Usage telemetry
│   └── crashreport/     # Crash reporting
└── frontend/src/
    ├── pages/           # Top-level page components
    ├── components/      # Feature components (editor/, layout/, results/, ...)
    ├── store/           # Zustand stores (8 stores)
    └── wailsjs/         # Auto-generated Wails IPC bindings (DO NOT EDIT)
```

**IPC flow**: Frontend calls `wailsjs/go/main/App.ts` → Wails runtime → Go `app.go` methods → `internal/` packages.

## Build Commands

```bash
# Type-check frontend (fast, no emit)
cd frontend && npx tsc --noEmit

# Full frontend build
cd frontend && npm run build

# Full app build (frontend + Go binary)
wails build

# Regenerate Wails JS/TS bindings after changing app.go signatures
wails generate module

# Go: tidy dependencies
go mod tidy

# Docs
make docs          # regenerate all docs (TypeDoc + gomarkdoc)
make docs-serve    # serve docs at http://localhost:4000
```

## Key Patterns

### Adding a new Go→Frontend IPC method
1. Add a public method on `*App` in `app.go` (receiver `a *App`)
2. Run `wails generate module` to regenerate `frontend/wailsjs/`
3. Import from `"../../../wailsjs/go/main/App"` in the component

### Emitting events from Go to frontend
```go
wailsruntime.EventsEmit(a.ctx, "event:name", payload)
```
```ts
const cleanup = EventsOn("event:name", (data) => { ... });
// call cleanup() on unmount
```

### Zustand stores (frontend state)
- `connectionStore` — active connection, role, warehouse, database
- `queryStore` — SQL tabs, results, selected SQL, active query
- `objectStore` — sidebar tree: databases, schemas, objects
- `themeStore` — light/dark/system + editor font/size
- `sessionStore` — persisted session state
- `panelLayoutStore` — persisted panel sizes
- `diffStore` — DDL diff comparisons
- `gitStore` — git repo state

### Monaco editor integration
- The SQL editor is in `frontend/src/components/editor/SqlEditor.tsx`
- `getQualifiedIdent(model, pos)` extracts full dot-separated identifiers (e.g. `DB.SCHEMA.TABLE`) from the cursor position
- `getStatementLineRanges(sql)` splits SQL into per-statement line ranges (mirrors Go backend `splitStatements`)
- DDL hover cache: module-level `hoverDDLCache` (Map, 60s TTL)
- Schema object cache: module-level `fetchedSchemaObjects` Set — avoids duplicate `ListObjects` calls
- **Never register completion/hover providers inside the component render** — use module-level disposable refs

### SQL diagnostics & JOIN suggestions (backend)
All proprietary analysis logic lives in `internal/sqleditor/sqleditor.go` and is called via IPC:
- `AnalyzeSqlSyntax(sql)` → character-by-character tokenizer (strings, comments, parens, dollar-quoting, scripting); inside `$$` blocks it also flags: placeholder tokens (`<>{}` at statement-start), bare unrecognised identifiers at statement-start, and wrong `:=`/`=` assignment syntax
- `ParseJoinTableRefs(sql)` → regex-based FROM/JOIN table-ref extractor (3/2/1-part + alias)
- `AnalyzeSqlSemantics(sql, resolvedRefs, colEntries)` → alias.column validator
- `ComputeJoinOnConditions(req)` → three-tier JOIN ON suggestion engine (FK → PK heuristic → type-compatible same-name columns + USING)
- `validateWithParser` and `validateBareColumnRefs` still run in the frontend (`sqlDiagnostics.ts`) as they depend on `node-sql-parser` which has no Go equivalent

### Adding a feature flag (Enabled Features)

Feature flags live in `internal/config/config.go` (`FeatureFlags` struct) and are surfaced to users via **File → Settings → Enabled Features**. All flags default to enabled — the `Initialized` sentinel prevents Go's zero-value `false` from silently disabling features on a fresh install.

**When implementing a new feature (regardless of whether it has a flag yet), you MUST update the feature list in `FEATURES.md`. If it is a toggleable feature, also add it to the "Feasible Optional Features" section in `FEATURES.md`.**

**Steps to add a new flag:**

1. **`internal/config/config.go`** — add a `bool` field to `FeatureFlags` and set it `true` in `DefaultFeatureFlags()`:
   ```go
   type FeatureFlags struct {
       Initialized  bool `json:"initialized"`
       ExportTableData bool `json:"exportTableData"`
       MyNewFeature bool `json:"myNewFeature"` // ← add here
   }

   func DefaultFeatureFlags() FeatureFlags {
       return FeatureFlags{
           Initialized:     true,
           ExportTableData: true,
           MyNewFeature:    true, // ← and here
       }
   }
   ```

2. **Run `wails generate module`** — regenerates `frontend/wailsjs/go/models.ts` with the new field.

3. **`frontend/src/components/settings/FeatureFlagsModal.tsx`** — add a `<FlagRow>` inside the modal's column:
   ```tsx
   <FlagRow
     label="My New Feature"
     description="One-line description shown in the modal."
     checked={flags.myNewFeature}
     onChange={(v) => set("myNewFeature", v)}
   />
   ```

4. **In the component that needs gating** — read the flag from `featureFlagsStore` and pass `disabled` + `disabledReason` to `menuItem` (Sidebar), or conditionally render/disable your own UI element:
   ```tsx
   const featureFlags = useFeatureFlagsStore((s) => s.flags);

   // Sidebar context-menu item:
   menuItem("My Action…", <Icon />, handler, undefined,
     !featureFlags.myNewFeature,
     "My New Feature is disabled. Enable it under File → Settings → Enabled Features.")

   // Or for a button:
   <Button disabled={!featureFlags.myNewFeature}>…</Button>
   ```

**Key files:**
- `internal/config/config.go` — `FeatureFlags` struct + `DefaultFeatureFlags()`
- `app.go` — `GetFeatureFlags()` / `SaveFeatureFlags()` IPC methods
- `frontend/src/store/featureFlagsStore.ts` — Zustand store (loaded on startup, reloaded after modal save)
- `frontend/src/components/settings/FeatureFlagsModal.tsx` — toggle UI (`<FlagRow>` per flag)
- `frontend/src/components/layout/Sidebar.tsx` — `menuItem()` 5th param `disabled`, 6th param `disabledReason`

### Code Snippets cascading context menu
- Implemented via Monaco's internal **`MenuRegistry` + `CommandsRegistry`** (both from `vs/platform/…`); no per-editor patching
- A module-level IIFE (runs once at load) registers:
  1. A `{ submenu: MenuId("thaw.snippets.submenu") }` entry in `MenuId.EditorContext` (group `9_snippets`) → Monaco renders the `▶` indicator and hover cascade natively
  2. Each snippet as a global `CommandsRegistry` command (`thaw.snippet.<label>`)
  3. Each snippet as a `MenuRegistry` item in the submenu `MenuId` with its display title from `SNIPPET_CATEGORIES.titles`
- Per-editor: `editor.onContextMenu` sets `_activeSnippetEditor` so commands always insert into the right editor
- **Snippets respect `editorPrefsRef`** — `applyPrefsToSnippet(text, prefs)` is called at insertion time; handles keyword casing (`keywordCase`) and indentation (`indentStyle` / `indentSize`); no re-registration needed when prefs change
- Snippet definitions and category groupings live in `snowflakeSnippets.ts`; `SNIPPET_CATEGORIES` drives submenu structure; optional `titles` map per category provides human-readable menu labels distinct from internal command IDs
- **Do not use `instanceof SubmenuAction` from an external import** — Monaco's `menu.js` checks its own bundled class; external imports are different module instances and always fail the check; use `MenuRegistry` instead and let Monaco create `SubmenuAction` internally

## Critical Gotchas

### gosnowflake driver logs errors before throwing
The gosnowflake driver logs ALL query errors at ERROR level via slog, even when the caller catches them. Do NOT call `GetObjectDDL` with a guessed object kind (TABLE vs VIEW) — always determine the kind first (from the objects store or a `ListObjects` call) to avoid noisy error logs from failed GET_DDL attempts.

### gosnowflake `sf.WithQueryIDChan`
The driver writes the query ID to the channel and **then closes it**. Never call `close(qidChan)` manually — that panics. Use `case qid := <-ch:` to drain, with `case <-ctx.Done():` as cancellation fallback.

### WKWebView clipboard
`navigator.clipboard` is blocked in WKWebView. All clipboard operations use Wails' `ClipboardGetText` / `ClipboardSetText` native APIs. Monaco's built-in copy/paste is overridden via `_commandService` patch + capture-phase keydown listeners.

### Multi-statement execution
For multi-statement SQL, `Execute` uses an inner `execCtx` (fresh context). The outer `qidChan` (single-statement async mode) never fires. Per-statement query IDs are tracked via per-statement goroutines + `sync.WaitGroup` in `app.go`'s `StartQuery`.

### `wailsjs/` is auto-generated
Never edit files under `frontend/wailsjs/` by hand — they are overwritten by `wails generate module`.

### `frontend/dist/.gitkeep` must stay committed
Go's `//go:embed all:frontend/dist` directive in `main.go` is evaluated during `wails generate module` (binding generation), which runs **before** the frontend build. If `frontend/dist` is empty or missing, the Go build fails with "contains no embeddable files". The committed `.gitkeep` placeholder satisfies the embed on clean checkouts. Never delete it.

### `runDiagnostics` must stay race-safe and exception-safe
`runDiagnostics` in `SqlEditor.tsx` is an async function with three IPC `await` points. Two invariants must hold:
1. **Race safety** — capture `model.getVersionId()` before any async work and check it after each `await`; `return` early if the version advanced (user edited mid-flight). The `return` still executes `finally`, but the version check inside `finally` prevents overwriting a newer run's markers.
2. **Exception safety** — wrap the entire body in `try/catch/finally`. If any IPC call rejects, `catch` logs it and `finally` guarantees `setModelMarkers` is called with whatever was collected (possibly empty), so stale markers are never left stuck.
Also use `editor.onDidChangeModelContent` (not `editor.getModel()?.onDidChangeContent`) — the latter silently skips registration if the model is null at mount time.

### Frontend bundle obfuscation
The production frontend build (`npm run build`) runs `javascript-obfuscator` after Terser via `vite.config.ts`. Vendor and Monaco chunks are explicitly skipped. The build script passes `--max-old-space-size=6144` to Node to prevent V8 heap OOM during obfuscation. `controlFlowFlattening` and `deadCodeInjection` are disabled to keep peak memory within budget; RC4 string-array encoding provides the primary IP protection.

## Testing

```bash
# Go unit tests (DDL parser)
go test ./internal/ddl/...

# Go unit tests (all internal packages)
go test ./internal/...

# TypeScript type check
cd frontend && npx tsc --noEmit

# Frontend unit tests (vitest)
cd frontend && npm test
```

Integration tests live in `internal/integration/` (require live Snowflake connection; gated behind `integration` build tag).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop runtime | Wails v2.11 |
| Backend | Go 1.22 |
| Snowflake driver | gosnowflake v2.0 |
| Frontend | React 18 + TypeScript 5.6 |
| Build tool | Vite 5 |
| UI library | Ant Design 5 |
| SQL editor | Monaco (`@monaco-editor/react`) |
| Results grid | Ag-Grid Community |
| State | Zustand 5 |
| Terminal | xterm.js |
