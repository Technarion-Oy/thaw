# Thaw — Gemini CLI Context & Mandates

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend).

## Documentation map

Read the relevant doc before working in an area:
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — branching, commits, PRs, the docs-with-code rule, build/test/lint commands.
- [`docs/concepts/`](docs/concepts/) — [architecture](docs/concepts/architecture.md), [onboarding](docs/concepts/onboarding.md), [patterns](docs/concepts/patterns.md), [gotchas](docs/concepts/gotchas.md), [testing](docs/concepts/testing.md).
- `internal/<pkg>/README.md` and `frontend/src/<dir>/README.md` — per-module reference for every backend package and frontend folder.
- [`FEATURES.md`](FEATURES.md) — full feature catalogue.

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
- **Root-level Go files** (`main.go`): add `// thaw:file-domain: <Domain Name>` to the file. The generator outputs the individual file path.
- **TypeScript / TSX files**: add `// @thaw-domain: <Domain Name>` anywhere in the file. The generator outputs the individual file path.
- **Regenerate**: run `go generate ./internal/architecture/` (or `go run scripts/gen_semantic_map.go` from the project root) after any annotation change. The CI test `TestSemanticMapAccuracy` will fail if any annotated path no longer exists on disk.

## 💡 Critical Context
- **Nature of App**: This is a **Snowflake SQL Editor** and management tool.
- **Authentication**: Authentication is handled by parsing connection parameters from the **Snowflake CLI configuration file** (defaults to `~/.snowflake/config.toml` or `connections.toml`). Users can select a custom path during sign-in, which is persisted in the app configuration. Profiles can be **created, saved, renamed, cloned, set as default, and deleted** directly from the connection dialog via `internal/sfconfig/writer.go` (text-level TOML manipulation that preserves comments and unknown keys). The profile management UI is gated behind the `snowflakeCLIProfileManager` feature flag (toggleable via **View → Enabled Features…**).
- **Tech Stack**: Go 1.22, Wails v2, React 18, TypeScript 5.6, Monaco Editor, Ant Design 5, Zustand 5, TanStack Table v8.

## 🗄 Codebase Vector Database

A ChromaDB vector index of all `.go`, `.ts`, and `.tsx` source files lives at `.chroma_db/` in the repo root. It is **not committed to git**.

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
- Omit `--reset` to append (but prefer `--reset` since chunk IDs are UUIDs and duplicates won't be detected).
- The venv and all dependencies are already installed at `scripts/.venv/`.

## 🏗 Architecture Overview
- **Go Backend**: Wails IPC bindings (all on `*App`, `package app`) live in `internal/app/`, split across `app.go` (struct, lifecycle, session management), `run.go` (`Run(assets)` entry point + wails.Run wiring), `menu.go` (native menu), and per-domain files (e.g. `query.go`, `objects.go`, `backup.go`); business logic lives in the other `internal/` packages. The root `main.go` is a thin entry point that owns `//go:embed all:frontend/dist` and calls `app.Run(assets)`.
- **Thin-delegator pattern**: Most `*App` methods are thin delegators — a nil-check (`apperrors.ErrNotConnected`), a single call into a domain package, then return. SQL building, `snowflake.QueryResult` parsing, validation, and key generation live in `internal/<domain>` packages (e.g. `backup`, `objects`, `warehouse`, `table`, `keypair`, `queryhistory`), where they are independently unit-testable without a live connection. Domain funcs take `(ctx, *snowflake.Client, …)` and return **domain types** (types belong to their domain package, not `internal/app`); pure `Build*Sql`/`Parse*` helpers are split out for fixture-based tests. Shared result-parsing helpers (`ColIdx`, `Cell*`, `PropertyPair`, `SessionParam/SessionVar`) live in `internal/snowflake/result.go`. Methods coupled to `App` state / Wails events / goroutine orchestration stay in `internal/app` (`StartQuery`, `WaitForQueryResult`, `CancelQuery`, `RunExplain`, shell PTY, `ExportDatabaseDDL`). Frontend model refs use the new TS namespaces (`backup.`, `objects.`, `warehouse.`, `table.`, `keypair.`, `queryhistory.`, `snowflake.`); only `app.AppInfo` remains in `app`.
- **Snowflake Client**: Located in `internal/snowflake/client.go`. Enriched `ColumnInfo` here for metadata-heavy tasks.
- **SQL Editor Service**: `internal/sqleditor/service.go` — a Wails-bound `Service` struct exposing all SQL diagnostics & autocomplete IPC endpoints (no Snowflake connection required). Registered in `internal/app/run.go`'s `Bind` array. Frontend imports from `wailsjs/go/sqleditor/Service`.
- **Task Management**: `internal/tasks/tasks.go` — task graph operations (suspend/resume graph, drop tree, clone child, manage predecessors), task statuses via `SHOW TASKS` + `INFORMATION_SCHEMA.TASK_HISTORY()`, and run history queries. Frontend task components live in `frontend/src/components/task/` (TaskGraphModal, TaskHistoryModal, TaskPropertiesModal, TaskStatusesModal, CreateTaskModal, ExecuteTaskModal). TaskGraphModal supports Export DDL — graph-level (topological order with optional SUSPEND/RESUME wrapping) and per-node (single task DDL to clipboard).
- **DBT Project Browser**: `internal/dbtproject/` — SQL builders for Snowflake-native `DBT PROJECT` objects (`CREATE`, `ALTER SET/UNSET`, `EXECUTE`, `ADD VERSION`). Frontend modals in `frontend/src/components/dbtproject/` (CreateDbtProjectModal, ExecuteDbtProjectModal, ModifyDbtProjectModal, AddDbtProjectVersionModal). Context menu entries in the sidebar for full lifecycle management. DBT PROJECT objects are expandable in the sidebar — clicking the expand arrow shows versions, and each version expands to a hierarchical file/directory tree with lazy-loading; right-click versions/directories for Refresh. Gated behind the `dbtProjectBrowser` feature flag.
- **Table Column Management**: `internal/column/` — SQL builders for table column DDL (`BuildAddColumnSql`, `BuildDropColumnSql`, `BuildRenameColumnSql`, `BuildSetNotNullSql`, `BuildDropNotNullSql`, `BuildSetColumnCommentSql`, `BuildChangeDataTypeSql`), exposed via thin `*App` wrappers and unit-tested in `sql_test.go`. The frontend (`AddColumnModal.tsx` + the Sidebar column context-menu handlers) collects config and calls these builders over IPC; no column DDL is constructed in the frontend. `AddColumnModal` renders a debounced backend-generated SQL preview, mirroring the dbtproject modal pattern. Reference data for the dialog also comes from the backend: datatype predicates (`IsNumeric`/`IsBoolean`/`NeedsQuotes`) and the collation registry in `internal/snowflake/collations.go` (`GetCollations`/`GetCollationLocales`/`GetCollationSpecifiers`) — collation lists and datatype checks are never hardcoded in the frontend. All altering actions (Add/Rename/Change Type/Set Comment/Set/Drop NOT NULL/Drop) are gated behind the `columnManagement` feature flag (admin-lockable via the `schemaManagement` admin category); the passive **Insert Column Name** action is never gated.
- **Stage Sidebar Tree**: Stages are expandable in the sidebar — clicking the expand arrow shows a hierarchical file/directory tree with lazy-loading via `ListStageEntries`. Right-click `.sql` files for Execute File (`EXECUTE IMMEDIATE FROM`), all files for Download and Delete; right-click directories for Refresh and Upload File. The existing **Manage Storage Files…** modal (`StageBrowserModal`) remains available for bulk operations.
- **Frontend**: React application in `frontend/src/`.
- **Unified Toolbar**: `frontend/src/components/toolbar/Toolbar.tsx` — reusable toolbar with execution controls, quick-action buttons (New SQL, New Notebook, Save), session selectors, and a `contextSlot` for tab-type-specific content (e.g. `NotebookToolbarSlot` for kernel status). The `notebookToolbarStore` bridges NotebookTab's internal state to the toolbar.
- **State Management**: Zustand stores are in `frontend/src/store/`. The `gridStore` manages results grid state: selection range, search, column formatting, and conditional formatting.
- **Results Grid**: `frontend/src/components/results/ResultGrid.tsx` — TanStack Table v8 grid with column pinning, auto-size, drag-to-reorder columns (TanStack `columnOrder`; view-only, gated behind the `columnReorder` feature flag), Excel-style filtering, conditional formatting, data-type formatting, range selection, and quick charting. Supporting components: `GridSearch`, `StatusBar`, `CellDetailPanel` (side panel showing the full content of the selected cell; gated behind the `cellDetailPanel` feature flag), `QuickChartModal`, `ColumnFilterDropdown`, `ConditionalFormattingModal`, `DataTypeFormatModal`.
- **Filesystem Operations**: `internal/filesystem/` — file read/write/delete/rename, directory creation, reveal in platform file manager, recursive file search, and file system watcher (`watcher.go`). All mutating operations (`DeleteFile`, `DeleteDirectory`, `RenameFile`, `MkDir`, `WriteFileInRoot`) validate that paths are strictly inside an allowed root directory, resolving symlinks to prevent escape. The File Browser sidebar (`frontend/src/components/files/FileBrowser.tsx`) exposes these via a right-click context menu. The file system watcher (`StartFileWatcher`/`StopFileWatcher` IPC methods) monitors the working directory for external changes and emits debounced `fs:changed` events so the File Browser incrementally refreshes only affected directories; gated behind the `fileWatcher` feature flag.
- **Cross-Tab Search**: `frontend/src/components/editor/CrossTabSearch.tsx` — search/replace panel that searches across all open tabs (SQL, YAML, Python, notebook cells); opened via `⌘⇧H`; gated behind the `crossTabSearch` feature flag. Replace on the active tab routes through Monaco `executeEdits` for undo support; regex replace supports capture-group back-references (`$1`, `$2`). Tab-switch navigation listens for `thaw:editor-ready` (emitted by SqlEditor on mount) instead of a fixed delay. Known limitation: navigating to a notebook tab match switches tabs but does not scroll to or highlight the match within the cell.
- **MCP Server**: `internal/mcp/` — read-only Model Context Protocol servers built on the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk/mcp`), exposing the active Snowflake connection to external AI clients over SSE/HTTP on `localhost`. A `Manager` runs multiple labelled `session`s (ports auto-assigned from `9100`); each session owns a dedicated `*snowflake.Client` (isolated like tab sessions) and registers seven schema-browsing tools (`get_session_context`, `list_databases`, `list_schemas`, `list_objects`, `describe_table`, `get_ddl`, `get_table_foreign_keys`). `*App` delegators live in `internal/app/mcp.go` (`StartMCPSession`/`StopMCPSession`/`ListMCPSessions`/`GetMCPSessionConfig`); `internal/mcp` must not import `internal/app`. Sessions are user-started only and all stopped via `StopAll()` on `shutdown`/`Disconnect`. Frontend: `MCPSessionsModal.tsx` (View → MCP Sessions), `MCPIndicator.tsx` (Toolbar tag), `mcpStore.ts`; gated behind the `mcpServer` flag (Integrations category, admin-lockable). **MCP SDK gotcha**: `AddTool` panics if the `Out` type's schema isn't an object, so all tool handlers use `Out = any` (schema omitted) and return JSON-as-text content.
- **IPC Flow**: Frontend calls `wailsjs/go/app/App.ts` → Go `*App` methods in `internal/app/` (connection-dependent), or `wailsjs/go/sqleditor/Service.ts` → Go `*sqleditor.Service` methods (stateless SQL analysis).

## 🛠 Engineering Standards
- **Keep documentation up to date**: Every change that adds, removes, or modifies a user-facing feature, internal package, frontend component/store, or architectural pattern MUST include corresponding documentation updates in the **same commit or PR**. Files to update:
  - `README.md` — feature descriptions, project structure tree, SQL validation list, keyboard shortcuts
  - `FEATURES.md` — feature list; if toggleable, also the "Feasible Optional Features" section
  - `CLAUDE.md` — architecture tree, Zustand store list, key patterns, critical gotchas
  - `GEMINI.md` — architecture overview, engineering standards, common workflows
  Do not defer documentation to a follow-up PR. Outdated docs mislead both humans and LLM agents.
- **Surgical Edits**: Prefer `replace` over `write_file` for large files like `internal/app/app.go` and `Sidebar.tsx`.
- **Wails Bindings**: After modifying Go method signatures in `internal/app/` or any Wails-bound `Service` struct (e.g., `internal/sqleditor/service.go`), you MUST run `wails generate module` to update frontend bindings.
- **New Feature Pattern**:
    1. Define state in a new `zustand` store in `frontend/src/store/` (optional).
    2. Create UI components in `frontend/src/components/` (e.g., `database/CreateTableModal.tsx`, `layout/`).
    3. Register context menu actions in `frontend/src/components/layout/Sidebar.tsx` (including column-level actions via the `col` nodeType and `colMeta` on `ContextMenu`).
    4. If the feature is optional, add a boolean flag to `FeatureFlags` in `internal/config/config.go`, add it to `DefaultFeatureFlags()`, and bump the `flagsVersion`.
    5. Gate all related UI elements (sidebar items, buttons, tabs) using `useFeatureFlagsStore.getState().flags`. When a flag is `false`, the corresponding feature MUST be hidden or disabled across the entire application.
- **SQL Generation**: Use double quotes for identifiers (`"DATABASE"."SCHEMA"."TABLE"`) and handle escaping (`" -> ""`).
- **Feature Documentation**: When implementing or updating a feature, you MUST update the feature list in `FEATURES.md`. If the feature can be toggled, also add it to the **Feasible Optional Features** section in `FEATURES.md`.

## 🛠 Feature Flags & IT Admin Management
Thaw uses a centralized feature flag system to gate optional or experimental capabilities. Features are managed via **View → Enabled Features…**.

### Adding a New Flag
1. **Go Backend**: Add a `bool` field to the `FeatureFlags` struct in `internal/config/config.go`.
2. **Defaults**: Update `DefaultFeatureFlags()` to set the new flag's default state (usually `true`).
3. **Migration**: Bump `flagsVersion` in `config.go` and update `MigrateFlags()` to ensure existing users receive the default value for the new flag.
4. **Wails Bindings**: Run `wails generate module` to update frontend TypeScript models.
5. **UI**: Add a corresponding `<FlagRow />` to `frontend/src/components/settings/FeatureFlagsModal.tsx` in the appropriate category.

### IT Admin Overrides (Enforced Policies)
IT administrators can enforce feature policies (enabling or disabling features) via system-level configuration, making them "locked" (unchangeable) for the end user.
- **Priority**: MDM/Registry/Plist (Highest) > `features.json` (System-level) > User Preferences (Lowest).
- **macOS**: Managed Plist (`Disable<FeatureName> = true`) in `/Library/Managed Preferences/com.thaw.app.plist`.
- **Windows**: Group Policy Registry (`Disable<FeatureName> = 1`) in `HKLM\SOFTWARE\Policies\Thaw\Features`.
- **Linux / Cross-platform**: `features.json` in `/etc/thaw/features.json` or `%PROGRAMDATA%\Thaw\features.json`.

When a feature is admin-controlled, the toggle in **Enabled Features** is automatically greyed out and shows a lock icon.

## 🎨 UI & Ant Design Standards
- **Icons**: Use `@ant-design/icons` (e.g., `SyncOutlined`, `TableOutlined`).
- **Feedback**: Use `antd`'s `message.success`/`error` for immediate feedback.
- **Modals**: Use `antd` `Modal` with `destroyOnClose`.
- **Alerts**: `antd` `Alert` does **not** have a `size` property. Use `showIcon` and `message` (can be a `Space` or `Typography` block).
- **Typography**: Use `Typography.Text` for consistent font styling.
- **Tree Component**:
    - To support row-wide interaction, use `blockNode` and handle selection in `onSelect`.
    - **Gotcha**: In `onSelect(keys, info)`, the `info.event` is a string literal `"select"`. Use `info.nativeEvent` to access `ctrlKey`, `metaKey`, or `stopPropagation()`.

## 📋 Common Workflows
### Adding an IPC Method
1. Define a public method on `*App` in the `internal/app/<domain>.go` file matching its domain (all are `package app`, so the method is bound wherever it lives).
2. Run `wails generate module`.
3. Import and use the method in the React component from `../../../wailsjs/go/app/App`.

### Pull Request Workflow
- **Feature Branches**: Always work in a dedicated branch (`feat/`, `fix/`, etc.).
- **Git History**: NEVER alter git branch history. Do not use `git commit --amend`, `git rebase`, or `git push --force`. Always create new commits and push them normally to update a PR.
- **Submission**: Use GitHub CLI (`gh`) to create pull requests.
- **Target**: Ensure PRs target `Technarion-Oy/thaw:main`.
- **Command**: `gh pr create --repo Technarion-Oy/thaw --base main --title "..." --body "..."`

### Working with Query Tab
- To open SQL in a new tab without running it: `useQueryStore.getState().loadInNewTab(sql)`.
- To open and execute immediately: `useQueryStore.getState().executeInNewTab(sql)`.

### Multi-Selection in Sidebar
- Controlled via `selectedNodeKeys` state (Set of strings) and `selectedNodeArgs` (Map for function/procedure signatures).
- `Tree` component should have `selectedKeys={Array.from(selectedNodeKeys)}` and `multiple` props.
- Logic for toggling selection resides in the `onSelect` handler (checking `nativeEvent.ctrlKey`/`metaKey`).

### Snowflake Scripting Support
- **Syntax Highlighting**: Custom categories `scripting` and `scripting_loop` added to `snowflakeMonarchLanguage` in `snowflakeSql.ts`.
- **Snippets**: Registered via `monaco.languages.registerCompletionItemProvider` in `monacoSetup.ts`. Templates defined in `snowflakeSnippets.ts`.
- **Dollar Quoting**: Treated as transparent delimiters (`delimiter.dollar`) in Monarch and diagnostics (`sqlDiagnostics.ts`) to allow full highlighting and structural error detection inside scripting bodies.

### Database Reports
- Cascading menu in sidebar for database nodes.
- `ObjectSummariesModal` fetches detailed table metadata via `GetDatabaseTableSummary` in `internal/app/table.go`.
- **Wails v2 Gotcha**: `time.Time` fields are formatted as RFC3339 strings in Go before being passed to the frontend to avoid "Not found: time.Time" build warnings and ensure clean TypeScript `any` -> `string` bindings.

### Insert Mapping
- State management in `useInsertMappingStore`.
- Supports one target table and multiple source tables/views.
- Side-by-side mapping UI allows simultaneous mapping of multiple sources.
- SQL generation handles `UNION ALL` / `UNION` combinations.

## ⚠️ Gotchas
- **Logs**: `gosnowflake` driver logs errors to `slog.Default` even when caught.
- **Wails Generate**: If `wails generate module` fails, check Go syntax errors first.
- **Persistence**: App state is persisted in `~/.config/thaw/config.json`. Frontend store persistence uses `localStorage`.

## 🚀 Pull Request Generation with Gemini CLI

Thaw uses **Gemini CLI** as the mandated tool for generating PR titles and bodies. All squash-merged PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/) — they drive automated semantic versioning via `semantic-release`.

### Install & Auth

```bash
# Install
npm install -g @google/generative-ai-cli   # or: pip install gemini-cli

# Authenticate (first run)
gemini auth login
```

### Prompt: PR Title Only

Use this when you only need a Conventional Commit title (single line):

```
You are a commit message expert. Given the following git diff or description of changes,
produce ONE Conventional Commit PR title (max 72 characters, no period at the end).
Use one of: feat, fix, perf, refactor, chore, docs, style, test, build, ci.
Append "!" after the type for breaking changes (e.g. "feat!:").
Output ONLY the title, nothing else.

Changes:
<paste diff or description here>
```

### Prompt: Full PR Body (with optional BREAKING CHANGE footer)

```
You are a pull request expert. Given the following changes, produce a GitHub PR body in
this exact format:

## Summary
- <bullet 1>
- <bullet 2>
- <bullet 3 if needed>

## Test plan
- [ ] <manual test step 1>
- [ ] <manual test step 2>

If the changes are breaking, append this footer (otherwise omit it entirely):
BREAKING CHANGE: <one-line description of what breaks and how to migrate>

Changes:
<paste diff or description here>
```

### Version Bump Mapping

| Commit type | Release | Version bump |
|-------------|---------|--------------|
| `feat` | ✅ | **minor** (0.X.0) |
| `feat!` / `BREAKING CHANGE` footer | ✅ | **major** (X.0.0) |
| `fix`, `perf` | ✅ | **patch** (0.0.X) |
| `refactor`, `chore`, `docs`, `style`, `test`, `build`, `ci` | ❌ | no release |

### ⛔ Never Tag Manually

Do **not** create or push `v*` tags by hand. The `manual-release.yml` workflow runs
`semantic-release`, which:
1. Analyses commits since the last tag.
2. Determines the correct next version.
3. Updates `wails.json` and `CHANGELOG.md`.
4. Pushes a signed version-bump commit and the `vX.Y.Z` tag.
5. Creates the GitHub Release.

Manual tags bypass the changelog and version-file update, breaking the pipeline.
