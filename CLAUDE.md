# Thaw — Claude Code Guide

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend embedded as a single binary).

## Codebase Navigation & Architecture

Before proposing new features, refactoring, or writing new files, you MUST consult the codebase semantic map.

1. Open and read the file: `internal/architecture/semantic_map.go`.
2. Locate the target domain for the user's request based on the JSON map inside the `GetCodebaseSemanticMap()` function.
3. Restrict your file creation and modification suggestions to the directories specified in that domain.
4. Do not invent new architectural folders unless explicitly instructed by the user.
5. If the user asks to modify the semantic map, add the `thaw:domain` or `thaw:file-domain` annotation to the relevant Go files, and `@thaw-domain` to the relevant TypeScript files, then run `go generate ./internal/architecture/` to regenerate `semantic_map.go`.

### How the semantic map is maintained

The map in `internal/architecture/semantic_map.go` is **generated** — do not edit it by hand.

- **Go packages** (`internal/*/`): add `// thaw:domain: <Domain Name>` anywhere in a `.go` file inside the package (the canonical place is `doc.go`). The generator outputs the package directory path.
- **Root-level Go files** (`main.go`, `app.go`, etc.): add `// thaw:file-domain: <Domain Name>` to the file. The generator outputs the individual file path.
- **TypeScript / TSX files**: add `// @thaw-domain: <Domain Name>` anywhere in the file. The generator outputs the individual file path.
- **Regenerate**: run `go generate ./internal/architecture/` (or `go run scripts/gen_semantic_map.go` from the project root) after any annotation change. The CI test `TestSemanticMapAccuracy` will fail if any annotated path no longer exists on disk.

## Development Workflow

- **Keep documentation up to date**: Every change that adds, removes, or modifies a user-facing feature, internal package, frontend component/store, or architectural pattern MUST include corresponding updates to the relevant documentation files in the **same commit or PR**. The documentation files and what to update in each:
  - `README.md` — feature descriptions, project structure tree (internal packages, frontend components, stores), SQL validation list, keyboard shortcuts
  - `FEATURES.md` — feature list; if the feature is toggleable, also the "Feasible Optional Features" section
  - `CLAUDE.md` — architecture tree, Zustand store list, key patterns, critical gotchas
  - `GEMINI.md` — architecture overview, engineering standards, common workflows
  Do not defer documentation to a follow-up PR. Outdated docs mislead both humans and LLM agents.
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
├── app.go               # App struct, lifecycle (startup/shutdown), tab-session mgmt, Connect/Disconnect
├── app_*.go             # Wails IPC bindings split by domain (all on *App, package main):
│                        #   app_query, app_objects, app_session, app_filesystem, app_git,
│                        #   app_profiles, app_builders, app_stage, app_dbtproject, app_pipe,
│                        #   app_warehouse, app_integrations, app_users, app_tasks, app_table,
│                        #   app_notebook_native, app_ddlexport, app_config, app_ai, app_shell,
│                        #   app_backup, app_migration, app_snowpark
├── internal/
│   ├── apperrors/       # Sentinel errors (ErrNotConnected etc.)
│   ├── version/         # Version string (set via -ldflags at build time)
│   ├── session/         # Window state persistence (load/save, OS-specific paths)
│   ├── migration/       # Schema migration engine (Service pattern)
│   ├── snowpark/        # Snowpark/Jupyter support (Service pattern)
│   ├── snowflake/       # Snowflake client — connection, query, DDL, lineage
│   ├── sqleditor/       # SQL diagnostics & JOIN suggestion engine (Wails-bound Service)
│   │   ├── service.go   # Wails-bound Service struct (IPC endpoints)
│   │   ├── sqleditor.go
│   │   └── sqleditor_test.go
│   ├── tasks/           # Task graph ops, statuses, run history (TASK_HISTORY), DDL export
│   ├── ddl/             # DDL parsing and git-export pipeline
│   ├── dbtproject/      # Snowflake-native DBT PROJECT SQL builders (CREATE, ALTER, EXECUTE)
│   ├── column/          # Table column DDL builders (ADD/DROP/RENAME/ALTER COLUMN)
│   ├── ai/              # AI provider clients (OpenAI, Google, Ollama); inline completions, model management
│   ├── config/          # App config (TOML persistence)
│   ├── gitrepo/         # Git operations via exec
│   ├── filesystem/      # File read/write/delete/rename helpers, reveal in file manager, FS watcher
│   ├── sfconfig/        # Reads ~/.snowflake/config.toml
│   ├── logger/          # Logrus + lumberjack rotation
│   ├── telemetry/       # Usage telemetry
│   └── crashreport/     # Crash reporting
└── frontend/src/
    ├── pages/           # Top-level page components
    ├── components/      # Feature components (editor/, layout/, toolbar/, results/, task/, ...)
    ├── store/           # Zustand stores (10 stores)
    └── wailsjs/         # Auto-generated Wails IPC bindings (DO NOT EDIT)
```

**IPC flow**: Frontend calls `wailsjs/go/main/App.ts` (or `wailsjs/go/sqleditor/Service.ts` for SQL editor methods) → Wails runtime → Go methods on `App` or bound `Service` structs → `internal/` packages.

## Codebase Vector Database

A ChromaDB vector index of all `.go`, `.ts`, and `.tsx` source files lives at `.chroma_db/` in the repo root. It is **not committed to git** (see `.gitignore`).

**Collection details:**
- Name: `thaw_codebase`
- Model: `models/gemini-embedding-2` at 768 dimensions
- Distance: cosine
- Contents: 190 source files → ~3 069 chunks (1 500 char / 150 overlap, language-aware splits)

**When to query it:**
Before writing code for a non-trivial task, query the vector DB to locate the most relevant existing files and functions. This avoids duplicate implementations and surfaces patterns you might not find with a plain `grep`.

**Querying from Python:**
```python
import chromadb, os
from google import genai
from google.genai import types

client = genai.Client(api_key=os.environ["GEMINI_API_KEY"])
db = chromadb.PersistentClient(path=".chroma_db")
col = db.get_collection("thaw_codebase")

def search(query: str, n: int = 8) -> list[dict]:
    vec = client.models.embed_content(
        model="models/gemini-embedding-2",
        contents=query,
        config=types.EmbedContentConfig(
            task_type="RETRIEVAL_QUERY",
            output_dimensionality=768,
        ),
    ).embeddings[0].values
    results = col.query(query_embeddings=[vec], n_results=n)
    return [
        {"file": m["file_path"], "language": m["language"], "text": d}
        for m, d in zip(results["metadatas"][0], results["documents"][0])
    ]
```

**Refreshing the index** (run after significant code changes):
```bash
cd scripts
GEMINI_API_KEY=... .venv/bin/python embed_codebase.py --reset
```

- The `--reset` flag drops and rebuilds the collection from scratch.
- Omit `--reset` to append (useful for incremental updates, but UUIDs are used as IDs so existing chunks are not deduplicated — prefer `--reset` unless the run was partial).
- The venv and all dependencies are already installed at `scripts/.venv/`.

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
1. Add a public method on `*App` (receiver `a *App`) to the `app_*.go` file matching its domain (e.g. query methods → `app_query.go`, object listing → `app_objects.go`). All `app_*.go` files are `package main`, so methods are bound regardless of which file they live in. Keep `app.go` for the `App` struct, lifecycle, and session-management only.
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
- `objectStore` — sidebar tree: databases, schemas, objects; also used as an instant cache by the search cascade (avoids IPC calls for previously expanded schemas)
- `themeStore` — light/dark/system + editor font/size
- `sessionStore` — persisted session state
- `panelLayoutStore` — persisted panel sizes
- `diffStore` — DDL diff comparisons
- `gitStore` — git repo state
- `notebookToolbarStore` — bridges NotebookTab kernel state/callbacks to the unified Toolbar
- `gridStore` — results grid selection range, search state, column formatting, and conditional formatting rules; resets navigation state when a new query result arrives (formatting persists if the column schema is unchanged). **Known limitation**: gridStore is a singleton — formatting, search state, selection range, conditional formatting rules, and the `tableRows` reference are shared across tabs and reset when switching tabs or running a query in another tab. During tab switches there is a brief window where stale state is visible (e.g. selection range from Tab A could be read against Tab B's data, or conditional rules keyed by column ID like `0_ID` could apply across tabs with identically-named columns). In side-by-side compare mode, both `ResultGrid` instances call `setTableRows()` — the last to render wins, so `StatusBar`/`GridSearch` may show data from the compare grid. Notebook SQL cells use `ResultGrid` with the `standalone` prop, which suppresses `setTableRows()`, `resetGrid()`, and `resetNavigation()` calls to avoid contaminating the main query tab's state. In side-by-side compare mode, both `ResultGrid` instances still call `setTableRows()` — the last to render wins, so `StatusBar`/`GridSearch` may show data from the compare grid. The `reset()` call on column schema change mitigates most cases.

### Unified Toolbar
- The application toolbar is implemented as a reusable `<Toolbar />` component in `frontend/src/components/toolbar/Toolbar.tsx`
- It renders: execution controls (Run/Cancel), action buttons (New SQL, New Notebook, Save), session selectors (role, warehouse, database, schema), connection info (username, account tag, disconnect)
- Context-specific additions (e.g. notebook kernel status) are rendered via the `contextSlot` prop
- `NotebookToolbarSlot` (`frontend/src/components/notebook/NotebookToolbarSlot.tsx`) renders the kernel status dot and Restart Kernel button; Deploy is rendered via Toolbar's `primaryAction` prop, and Add Cell is handled inline via hover-reveal bars between cells in `NotebookTab.tsx`
- `notebookToolbarStore` bridges the NotebookTab's internal kernel state and callbacks to the unified Toolbar through QueryPage
- The Toolbar reads session state directly from `connectionStore` and `sessionStore` — no prop-drilling for session selectors

### Sidebar tree node key formats
The sidebar tree uses key prefixes to identify node types. For expandable objects beyond tables/views:
- **Columns**: `col:DB:SCHEMA:TABLE:COLUMN` — leaf nodes under TABLE/VIEW `obj:` nodes; carry custom properties (`colDataType`, `colNullable`, `colIsPrimaryKey`, `colParentKind`, `colComment`) for the context menu (the Set Comment dialog prefills from `colComment`, which is sourced from `ColumnInfo.Comment` via `DESCRIBE TABLE`); TABLE columns get a full context menu (rename, change type, set/drop NOT NULL, set comment, drop); VIEW columns only get "Insert Column Name". **All column DDL (ADD/DROP/RENAME/ALTER COLUMN) is built in the backend `internal/column` package** (`Build*ColumnSql` IPC methods on `*App`); the Sidebar handlers and `AddColumnModal` only collect config and call the builder, never construct SQL inline. `AddColumnModal` mirrors the `column.AddColumnConfig` model and renders a debounced backend-generated SQL preview (same pattern as the dbtproject modals). Datatype/collation reference data also comes from the backend: `IsNumeric`/`IsBoolean`/`NeedsQuotes` (datatype predicates) and `GetCollations`/`GetCollationLocales`/`GetCollationSpecifiers` (collation registry in `internal/snowflake/collations.go`) — never hardcode collation lists or datatype regexes in the frontend. All altering actions (Add/Rename/Change Type/Set Comment/Set/Drop NOT NULL/Drop) are gated behind the `columnManagement` feature flag (admin-lockable via the `schemaManagement` admin-config category); "Insert Column Name" is never gated.
- **Git Repos**: `obj:DB:SCHEMA:GIT REPOSITORY:NAME` → `gitbranches:`, `gittags:`, `gitcommits:` → `gitdir:DB:SCHEMA:REPO:path` → `gitfile:DB:SCHEMA:REPO:path`
- **Stages**: `obj:DB:SCHEMA:STAGE:NAME` → `stagedir:DB:SCHEMA:NAME:path` → `stagefile:DB:SCHEMA:NAME:path`
- **DBT Projects**: `obj:DB:SCHEMA:DBT PROJECT:NAME` → `dbtversion:DB:SCHEMA:NAME:version` → `dbtdir:DB:SCHEMA:NAME:path` → `dbtfile:DB:SCHEMA:NAME:path`

Loading state for all these node types is tracked in the `loadingGitNodes` Set (shared, keys are namespaced so no collision). `buildEntryNodes(db, schema, name, entries, dirPrefix, filePrefix)` is a parameterised helper used for both stage and DBT nodes; `emptyChildNode` provides the empty-state placeholder. Backend IPC methods `ExecuteStageFile` and `ListDbtProjectEntries` delegate to existing client methods (`ExecuteGitFile`, `ListStageEntries` respectively) since the underlying SQL patterns are identical. Stage file Execute File is restricted to `.sql` files only; DBT project files do not have context menu actions. On delete, the file node is surgically removed from the tree via `removeNode` (the parent directory stays expanded).

### Object listing cache (backend)
- `Client` in `internal/snowflake/client.go` has a per-schema TTL cache (30s) for `ListObjects` and `ListBasicObjects` results, keyed by `"DB\x00SCHEMA"` (full) and `"basic\x00DB\x00SCHEMA"` (basic-only)
- `getObjectCache` returns `slices.Clone()` of the cached slice to prevent `append(basic, extended...)` in `ListObjects` from corrupting the backing array
- `ClearObjectCache()` / `ClearObjectCacheForDatabase(db)` are exposed as IPC methods; called from `refreshAllDatabases` / `refreshDatabaseByName` in `Sidebar.tsx`
- The sidebar search cascade uses a three-tier lookup: (1) `objectStore` for previously expanded schemas (instant, all types), (2) Go TTL cache, (3) `ListBasicObjects` fallback (1 query, tables/views/sequences only)
- `ListExtendedObjects` and `ClearObjectCacheForSchema` remain on `Client` for internal use but are **not** exposed as IPC methods

### Monaco editor integration
- The SQL editor is in `frontend/src/components/editor/SqlEditor.tsx`
- Pure helper functions (`quoteIfNecessary`, `getFKs` + cache, `buildVariableSuggestions`, `FKEntry`) live in `frontend/src/components/editor/sqlEditorUtils.ts`
- `getQualifiedIdent(model, pos)` extracts full dot-separated identifiers (e.g. `DB.SCHEMA.TABLE`) from the cursor position
- `getStatementLineRanges(sql)` splits SQL into per-statement line ranges (mirrors Go backend `splitStatements`)
- DDL hover cache: module-level `hoverDDLCache` (Map, 60s TTL)
- Schema object cache: module-level `fetchedSchemaObjects` Set — avoids duplicate `ListObjects` calls
- **Never register completion/hover providers inside the component render** — use module-level disposable refs

### Cross-tab search & replace
- `CrossTabSearch` (`frontend/src/components/editor/CrossTabSearch.tsx`) renders a search/replace panel between the TabBar and the editor area
- Triggered by `⌘⇧H` / `Ctrl+Shift+H` in QueryPage's global keydown handler; gated behind the `crossTabSearch` feature flag
- Searches all tabs (SQL, YAML, Python) by splitting `tab.sql` into lines; for notebook tabs, parses the serialised Jupyter JSON and searches each cell's source
- Navigation dispatches the existing `thaw:scroll-to-line` custom event so SqlEditor can reveal and select the match; after a tab switch, waits for the `thaw:editor-ready` event (emitted from SqlEditor's `handleMount`) with a 500 ms fallback timeout
- Replace on the active non-notebook tab routes edits through `editor.executeEdits()` (via `getEditorInstance()` from `editorRef.ts`) so Monaco's undo stack records the change and Ctrl+Z works; for non-active tabs and notebook tabs, falls back to store-only updates via `setSqlForTab` / `useQueryStore.setState({ sql })`
- Replace buttons are guarded by an `isReplacing` ref+state pair that disables them during the recomputation window (one `requestAnimationFrame`) after a replacement, preventing stale-match race conditions from rapid clicks
- Regex replace supports capture-group back-references (`$1`, `$2`) via `String.prototype.replace`; literal mode uses positional splicing
- Auto-navigate effect on match change preserves the user's position (finds closest match by tab/line/column) instead of snapping to the first match; only navigates to index 0 when there is no previous position
- Supports case-sensitive and regex toggle buttons; match counter shows "N of M in K tabs"
- **Known limitation — notebook navigation**: Navigating to a notebook tab match switches to the correct tab but does not scroll to or highlight the match within the cell, because `thaw:editor-ready` is only emitted by the primary `SqlEditor` on mount — notebook tabs use per-cell editors that don't emit this event
- **Known limitation — panel state**: Search/replace terms and toggle states are lost when the panel is closed (component is unmounted), unlike VS Code which preserves them

### File system watcher
- `internal/filesystem/watcher.go` — `Watcher` struct wrapping `fsnotify.Watcher`; recursively watches all non-hidden directories under a root path
- Events are debounced per-directory (200ms) to coalesce rapid changes (e.g. `git checkout`)
- New directories created externally are automatically added to the watch list
- `app.go` exposes `StartFileWatcher(dir)` / `StopFileWatcher()` IPC methods; change events are emitted as `"fs:changed"` Wails events with `{ dir: string }` payload
- `FileBrowser.tsx` starts/stops the watcher when `exportDir` changes; listens for `fs:changed` and incrementally refreshes only the affected directory node in the tree
- Self-change suppression: in-app mutations (create, rename, delete, duplicate) mark the parent directory in a `selfChangedDirs` Set with a 500ms timeout to prevent redundant double-refresh flicker
- Gated behind the `fileWatcher` feature flag (View → Enabled Features → File Watcher)

### SQL diagnostics & JOIN suggestions (backend)
All proprietary analysis logic lives in `internal/sqleditor/` and is exposed to the frontend via a dedicated Wails-bound `sqleditor.Service` struct (`service.go`). The service is registered in `main.go`'s `Bind` array and its methods are imported from `wailsjs/go/sqleditor/Service` (not from `wailsjs/go/main/App`):
- `AnalyzeSqlSyntax(sql)` → character-by-character tokenizer (strings, comments, parens, dollar-quoting, scripting); inside `$$` blocks it also flags: placeholder tokens (`<>{}` at statement-start), bare unrecognised identifiers at statement-start, and wrong `:=`/`=` assignment syntax
- `ParseJoinTableRefs(sql)` → regex-based FROM/JOIN table-ref extractor (3/2/1-part + alias)
- `AnalyzeSqlSemantics(sql, resolvedRefs, colEntries)` → alias.column validator
- `ComputeJoinOnConditions(req)` → three-tier JOIN ON suggestion engine (FK → PK heuristic → type-compatible same-name columns + USING)
- `GetAutocompleteContext(sql, cursorOffset)` → unified endpoint bundling statement ranges, scripting completions, table refs, CTE column projections, and `UseContext` (accumulated `USE DATABASE/SCHEMA` context from earlier statements) in a single IPC round-trip
- `GetAutocompleteContextFull(req)` → extends `GetAutocompleteContext` with backend ref resolution (`ResolvedRefs`), in-editor CREATE TABLE column extraction (`InEditorTables`), and context-detection flags (`IsDatatypeCtx`, `IsInJoinOnClause`, `UsingClause`); accepts `StoreObject[]`, `SessionContext`, and `LineUpToWord` so the frontend completion provider becomes a thin wrapper with no inline resolution logic
- `ComputeGitLineDiff(headLines, currentLines, maxLines)` → LCS-based line-level diff returning 1-based line numbers for added/modified/deleted regions; used by git gutter decorations
- `IsDatatypeContext(textToCursor, lineUpToWord)` → detects whether cursor expects a data type (after `::`, `CAST AS`, `DECLARE`, `CREATE/ALTER TABLE` column)
- `IsInJoinOnClause(textToCursor)` → detects whether cursor is inside a JOIN ... ON ... clause not yet terminated by a subsequent keyword
- `DetectUsingClause(textToCursor)` → detects USING clause context (`InUsing` for empty USING, `IsPartial` for partial column list)
- `ResolveTableRefs(refs, storeObjects, useCtx, session)` → resolves unqualified/partially-qualified table refs against store objects, UseContext, and session context (priority: fully-qualified → store match → UseContext → session); skips USE refs (Name=="")
- `GetSnowflakeKeywords()` → static list of Snowflake reserved keywords (delegates to `snowflake.ReservedKeywords()`)
- `ValidateTablesExist` markers include a `Code` field with JSON quick-fix metadata (`{"kind":"qualify-table","original":"FOO","suggestions":["DB.SCHEMA.FOO"]}`) when the unresolved table exists in other schemas; the frontend's `CodeActionProvider` parses this to offer lightbulb quick-fix qualification
- `validateWithParser` and `validateBareColumnRefs` still run in the frontend (`sqlDiagnostics.ts`) as they depend on `node-sql-parser` which has no Go equivalent
- The frontend `resolveRefs()` function has been removed — all table ref resolution now goes through the backend `ResolveTableRefs` IPC method, ensuring UseContext and session context are consistently applied across all completion/hover/diagnostics paths
- `InEditorTableDef` exposes columns from CREATE TABLE statements in the editor text for autocomplete before execution; `ExtractInEditorTableDefs` reuses `parseCreateTableColDefs` from `barecolrefs.go`

### Adding a feature flag (Enabled Features)

Feature flags live in `internal/config/config.go` (`FeatureFlags` struct) and are surfaced to users via **View → Enabled Features…**. All flags default to enabled — the `Initialized` sentinel prevents Go's zero-value `false` from silently disabling features on a fresh install.

**When implementing a new feature (regardless of whether it has a flag yet), you MUST update the feature list in `FEATURES.md`. If it is a toggleable feature, also add it to the "Feasible Optional Features" section in `FEATURES.md`.**

**Steps to add a new flag:**

1. **`internal/config/config.go`** — add a `bool` field to `FeatureFlags`, set it `true` in `DefaultFeatureFlags()`, bump `flagsVersion`, and add migration logic to `MigrateFlags()`:
   ```go
   type FeatureFlags struct {
       Initialized  bool `json:"initialized"`
       Version      int  `json:"version"`
       MyNewFeature bool `json:"myNewFeature"` // ← add here
   }

   func DefaultFeatureFlags() FeatureFlags {
       return FeatureFlags{
           Initialized:     true,
           Version:         flagsVersion,
           MyNewFeature:    true, // ← and here
       }
   }
   ```

2. **Run `wails generate module`** — regenerates `frontend/wailsjs/go/models.ts` with the new field.

3. **`frontend/src/components/settings/FeatureFlagsModal.tsx`** — add a `<FlagRow>` inside the modal's appropriate category:
   ```tsx
   <FlagRow
     label="My New Feature"
     description="One-line description shown in the modal."
     checked={flags.myNewFeature}
     locked={locked.myNewFeature} // ← pass the locked state
     onChange={(v) => set("myNewFeature", v)}
   />
   ```

4. **In the component that needs gating** — read the flag from `featureFlagsStore` and pass `disabled` + `disabledReason` to `menuItem` (Sidebar), or conditionally render/disable your own UI element. When a flag is `false`, the feature should be HIDDEN or DISABLED in the app UI:
   ```tsx
   const featureFlags = useFeatureFlagsStore((s) => s.flags);

   // Sidebar context-menu item:
   menuItem("My Action…", <Icon />, handler, undefined,
     !featureFlags.myNewFeature,
     "My New Feature is disabled. Enable it under View → Enabled Features…")

   // Or for a button:
   {featureFlags.myNewFeature && <Button onClick={...}>…</Button>}
   ```

### IT Admin Management (Enforced Policies)

IT administrators can enforce feature policies via platform-specific mechanisms. When a feature is locked by an admin, it appears with a lock icon in the UI and cannot be changed by the user.

- **macOS**: Managed Plist (`Disable<FeatureName> = true`) in `/Library/Managed Preferences/com.thaw.app.plist`.
- **Windows**: Group Policy Registry (`Disable<FeatureName> = 1`) in `HKLM\SOFTWARE\Policies\Thaw\Features`.
- **Linux**: `features.json` in `/etc/thaw/features.json`.

**Key files:**
- `internal/config/config.go` — `FeatureFlags` struct + `DefaultFeatureFlags()`
- `internal/config/adminconfig.go` — hierarchy and JSON loading logic
- `app.go` — `GetFeatureFlags()` / `GetAdminLockedFlags()` / `SaveFeatureFlags()` IPC methods
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

### Snowflake CLI profile management (TOML writer)
The `internal/sfconfig/writer.go` module provides `SaveProfile`, `DeleteProfile`, `CloneProfile`, `RenameProfile`, and `SetDefaultProfile` functions that manipulate `~/.snowflake/config.toml` at the text level (line-by-line splice/insert/remove). This approach preserves user comments, blank lines, and unknown TOML keys that Thaw doesn't model. When updating an existing `[connections.<name>]` section, `sectionBodyEnd()` trims trailing blank lines and comments so they remain attached to the next section visually. `atomicWriteFile` writes to a temp file then renames, ensuring 0600 permissions. When deleting the default profile, `default_connection_name` is cleared to `""`; when renaming, it's updated to the new name. The frontend exposes New, Save, Rename, Clone, Set Default, and Delete buttons below the profile dropdown in `ConnectModal.tsx`, each calling the corresponding `app.go` IPC method. New, Clone, and Rename block submission when a profile with the chosen name already exists. The entire profile management UI section in `ConnectModal.tsx` is gated behind the `snowflakeCLIProfileManager` feature flag; when disabled the profile dropdown, action buttons, and divider are hidden, but profile auto-fill on connect still works if profiles exist.

### Session management (pool tuning & idle eviction)
Per-tab Snowflake sessions are configurable via **View → Advanced → Session Management…** (`SessionManagementModal.tsx`). The backend stores settings in `config.SessionConfig` (persisted in `config.json`). At startup `app.go` calls `applySessionConfig` which sets runtime fields (`sessionMaxSessions`, `sessionMaxOpen`, `sessionMaxIdle`, `sessionInitMode`, `sessionIdleTimeout`) under `sessionConfigMu` and starts/stops the idle eviction goroutine. `evictIfNeeded()` reads `sessionMaxSessions` under RLock; `getOrInitTabSession()` reads `sessionMaxOpen`/`sessionMaxIdle` for `SetPoolLimits`. The idle eviction loop (`runIdleEvictionLoop`) ticks every 30s and evicts sessions whose `lastUsed` exceeds the timeout. The frontend tab lifecycle effect in `QueryPage.tsx` calls `GetSessionInitMode()` on new tab creation — if "eager", it fires `InitTabSession(tabId)` immediately.

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
| Results grid | TanStack Table v8 + `@tanstack/react-virtual` |
| State | Zustand 5 |
| Terminal | xterm.js |
