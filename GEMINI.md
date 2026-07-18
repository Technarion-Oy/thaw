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

## 🏗 Architecture Overview
- **Go Backend**: Wails IPC bindings (all on `*App`, `package app`) live in `internal/app/`, split across `app.go` (struct, lifecycle, session management), `run.go` (`Run(assets)` entry point + wails.Run wiring), `menu.go` (native menu), and per-domain files (e.g. `query.go`, `objects.go`, `backup.go`); business logic lives in the other `internal/` packages. The root `main.go` is a thin entry point that owns `//go:embed all:frontend/dist` and calls `app.Run(assets)`.
- **Thin-delegator pattern**: Most `*App` methods are thin delegators — a nil-check (`apperrors.ErrNotConnected`), a single call into a domain package, then return. SQL building, `snowflake.QueryResult` parsing, validation, and key generation live in `internal/<domain>` packages (e.g. `backup`, `objects`, `warehouse`, `table`, `keypair`, `queryhistory`), where they are independently unit-testable without a live connection. Domain funcs take `(ctx, *snowflake.Client, …)` and return **domain types** (types belong to their domain package, not `internal/app`); pure `Build*Sql`/`Parse*` helpers are split out for fixture-based tests. **Validated-wrapper variant** (reference example: `queryhistory`): when a `Build*Sql` helper embeds a value that must be pre-validated (e.g. a `SESSION_ID` embedded unquoted), keep the builder **unexported** (`buildQueryHistorySql`) and funnel all external access through the exported `Get*` wrapper, which validates first and returns an error — the builder treats the precondition as an invariant and **panics** if violated, rather than silently producing a wrong query. Shared result-parsing helpers (`ColIdx`, `Cell*`, `PropertyPair`, `SessionParam/SessionVar`) live in `internal/snowflake/result.go`. Methods coupled to `App` state / Wails events / goroutine orchestration stay in `internal/app` (`StartQuery`, `WaitForQueryResult`, `CancelQuery`, `RunExplain`, shell PTY, `ExportDatabaseDDL`). Frontend model refs use the new TS namespaces (`backup.`, `objects.`, `warehouse.`, `table.`, `keypair.`, `queryhistory.`, `snowflake.`); only `app.AppInfo` remains in `app`.
- **Snowflake Client**: Located in `internal/snowflake/client.go`. Enriched `ColumnInfo` here for metadata-heavy tasks.
- **SQL Editor Service**: `internal/sqleditor/service.go` — a Wails-bound `Service` struct exposing all SQL diagnostics & autocomplete IPC endpoints (no Snowflake connection required). Registered in `internal/app/run.go`'s `Bind` array. Frontend imports from `wailsjs/go/sqleditor/Service`.
- **Task Management**: `internal/tasks/tasks.go` — task graph operations (suspend/resume graph, drop tree, clone child, manage predecessors), task statuses via `SHOW TASKS` + `INFORMATION_SCHEMA.TASK_HISTORY()`, and run history queries. Frontend task components live in `frontend/src/components/task/` (TaskGraphModal, TaskHistoryModal, TaskPropertiesModal, TaskStatusesModal, CreateTaskModal, ExecuteTaskModal). TaskGraphModal supports Export DDL — graph-level (topological order with optional SUSPEND/RESUME wrapping) and per-node (single task DDL to clipboard).
- **DBT Project Browser**: `internal/dbtproject/` — SQL builders for Snowflake-native `DBT PROJECT` objects (`CREATE`, `ALTER SET/UNSET`, `EXECUTE`, `ADD VERSION`). Frontend modals in `frontend/src/components/dbtproject/` (CreateDbtProjectModal, ExecuteDbtProjectModal, ModifyDbtProjectModal, AddDbtProjectVersionModal). Context menu entries in the sidebar for full lifecycle management. DBT PROJECT objects are expandable in the sidebar — clicking the expand arrow shows versions, and each version expands to a hierarchical file/directory tree with lazy-loading; right-click versions/directories for Refresh. Gated behind the `dbtProjectBrowser` feature flag.
- **Table Column Management**: `internal/column/` — SQL builders for table column DDL (`BuildAddColumnSql`, `BuildDropColumnSql`, `BuildRenameColumnSql`, `BuildSetNotNullSql`, `BuildDropNotNullSql`, `BuildSetColumnCommentSql`, `BuildChangeDataTypeSql`, `BuildSetColumnDefaultSql`, `BuildDropColumnDefaultSql`, `BuildSetColumnMaskingPolicySql`, `BuildUnsetColumnMaskingPolicySql`), exposed via thin `*App` wrappers and unit-tested in `sql_test.go`. The frontend collects config and calls these builders over IPC; no column DDL is constructed in the frontend. `AddColumnModal` renders a debounced backend-generated SQL preview. The per-column edit actions (Rename / Change Type / Default / Comment / Set-Drop NOT NULL / Masking Policy / Tags) are consolidated into `components/column/ColumnPropertiesModal`, opened from the column context menu's **Properties…** item; each section builds SQL via the IPC builders and runs it with `ExecDDL`. Safe edits execute immediately — only a data-loss-risk edit (currently a data-type change) prompts a confirmation dialog with a warning and theme-aware SQL preview. The masking-policy and tag-name fields are dropdowns fed by `App.ListAccountMaskingPolicies` (`SHOW MASKING POLICIES IN ACCOUNT`) and `App.ListAccountTags` (`SHOW TAGS IN ACCOUNT`). Current default/masking-policy values load via `App.GetColumnDetails`. Reference data for the Add dialog also comes from the backend: datatype predicates (`IsNumeric`/`IsBoolean`/`NeedsQuotes`) and the collation registry in `internal/snowflake/collations.go` (`GetCollations`/`GetCollationLocales`/`GetCollationSpecifiers`) — collation lists and datatype checks are never hardcoded in the frontend. **Add Column…**, **Properties…**, and **Drop Column…** are gated behind the `columnManagement` feature flag (admin-lockable via the `schemaManagement` admin category); the passive **Insert Column Name** and **Tag References…** actions are never gated.
- **Stage Sidebar Tree**: Stages are expandable in the sidebar — clicking the expand arrow shows a hierarchical file/directory tree with lazy-loading via `ListStageEntries`. Right-click `.sql` files for Execute File (`EXECUTE IMMEDIATE FROM`), all files for Download and Delete; right-click directories for Refresh and Upload File. The existing **Manage Storage Files…** modal (`StageBrowserModal`) remains available for bulk operations.
- **Frontend**: React application in `frontend/src/`.
- **Unified Toolbar**: `frontend/src/components/toolbar/Toolbar.tsx` — reusable toolbar with execution controls, quick-action buttons (New SQL, New Notebook, Save), session selectors, and a `contextSlot` for tab-type-specific content (e.g. `NotebookToolbarSlot` for kernel status). The `notebookToolbarStore` bridges NotebookTab's internal state to the toolbar.
- **State Management**: Zustand stores are in `frontend/src/store/`. The `gridStore` manages results grid state: selection range, search, column formatting, and conditional formatting.
- **Results Grid**: `frontend/src/components/results/ResultGrid.tsx` — TanStack Table v8 grid with column pinning, auto-size, drag-to-reorder columns (TanStack `columnOrder`; view-only, gated behind the `columnReorder` feature flag), Excel-style filtering, conditional formatting, data-type formatting, range selection, and quick charting. Supporting components: `GridSearch`, `StatusBar`, `CellDetailPanel` (side panel showing the full content of the selected cell; gated behind the `cellDetailPanel` feature flag), `QuickChartModal`, `ColumnFilterDropdown`, `ConditionalFormattingModal`, `DataTypeFormatModal`.
- **Filesystem Operations**: `internal/filesystem/` — file read/write/delete/rename/copy, directory creation, reveal in platform file manager, recursive file search, and file system watcher (`watcher.go`). All mutating operations (`DeleteFile`, `DeleteDirectory`, `RenameFile`, `CopyFile`, `MkDir`, `WriteFileInRoot`) validate that paths are strictly inside an allowed root directory, resolving symlinks to prevent escape. `CopyFile(src, dst, allowedRoot)` copies a file (`io.Copy`+`O_EXCL`) or directory (recursive `os.CopyFS`), rejects symlinks and dir-into-itself, and never overwrites an existing destination. The File Browser sidebar (`frontend/src/components/files/FileBrowser.tsx`) exposes these via a right-click context menu. The file system watcher (`StartFileWatcher`/`StopFileWatcher` IPC methods) monitors the working directory for external changes (including content edits to existing files) and emits debounced `fs:changed` events. `QueryPage` owns the watcher's start/stop lifecycle (it's always mounted, unlike the ⌘B-hideable File Browser) and re-reads open editor tabs from disk so external edits show up in the editor (clean tabs adopt the new content; dirty tabs keep the user's edits but advance their saved baseline, VSCode-style); the File Browser incrementally refreshes only affected directories. Gated behind the `fileWatcher` feature flag.
- **Working Folder**: the git/export working directory (`GitConfig.ExportDir`) is the app's "operating folder". Change it via **File → Open Folder…** (`⌘⇧O`) or the File Browser header folder dropdown, which also lists **Recent** folders (`GitConfig.RecentDirs`, owned by the atomic `AddRecentDir`/`ClearRecentDirs` IPC — never persisted through the whole-config `SaveGitConfig`). **File → Open Folder in New Window…** (`OpenFolderInNewInstance`) re-execs the running binary with `--workdir=<dir>` (arg-only — no env fallback, so a stray env can't silently make a normal launch an override window) — deliberately not `open -n`, which LaunchServices can resolve to a stale duplicate. Such an **override window** (`workdirOverridden`) keeps its folder in memory only: `GetGitConfig` returns it and blanks the per-repo `RemoteURL`/`Branch` so git ops use the actual folder's live status; `SaveGitConfig` persists only the per-repo fields (`ExportDir`/`RemoteURL`/`Branch`, and nothing at all in an override window). The shared fields each have a dedicated **atomic, field-scoped** writer (`SaveGitAuthor`, `SaveGitExportPathTemplate`, `AddRecentDir`/`ClearRecentDirs`) so a whole-struct save of a stale snapshot from another process can't revert them. Snowpark resolves the working dir override-aware via `snowpark.SetWorkdirProvider`. Switching folders in-place (`gitStore.openFolder`) clears the old repo's remote+status so a mid-switch op can't target the previous repo; `refreshStatus` syncs the store's `branch` from the live HEAD.
- **Tab Bar**: `frontend/src/components/editor/TabBar.tsx` — the editor tab strip (drag-to-reorder, right-click context menu, split view). The far-right **Active Files** dropdown (pinned outside the scroll region) is a searchable list of every open tab; opened via the triangle button or `⌘⇧E` / `Ctrl+Shift+E` (`thaw:open-active-files` window event handled in `QueryPage.tsx`). Non-file tabs can be renamed inline (double-click title or context-menu **Rename**) via `queryStore.renameTab`; new scratch tabs are auto-numbered `SQL (n)` (`nextScratchTitle` in `queryStore.ts`).
- **Cross-Tab Search**: `frontend/src/components/editor/CrossTabSearch.tsx` — search/replace panel that searches across all open tabs (SQL, YAML, Python, notebook cells); opened via `⌘⇧H`; gated behind the `crossTabSearch` feature flag. Replace on the active tab routes through Monaco `executeEdits` for undo support; regex replace supports capture-group back-references (`$1`, `$2`). Tab-switch navigation listens for `thaw:editor-ready` (emitted by SqlEditor on mount) instead of a fixed delay. Known limitation: navigating to a notebook tab match switches tabs but does not scroll to or highlight the match within the cell.
- **MCP Server**: `internal/mcp/` — read-only Model Context Protocol servers built on the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk/mcp`), exposing the active Snowflake connection to external AI clients over SSE/HTTP on `localhost`. A `Manager` runs multiple labelled `session`s (ports auto-assigned from `9100`); each session owns a dedicated `*snowflake.Client` (isolated like tab sessions) and registers seven schema-browsing tools (`get_session_context`, `list_databases`, `list_schemas`, `list_objects`, `describe_table`, `get_ddl`, `get_table_foreign_keys`). `*App` delegators live in `internal/app/mcp.go` (`StartMCPSession`/`StopMCPSession`/`ListMCPSessions`/`GetMCPSessionConfig`); `internal/mcp` must not import `internal/app`. Sessions are user-started only and all stopped via `StopAll()` on `shutdown`/`Disconnect`. Frontend: `MCPSessionsModal.tsx` (Tools → MCP Sessions), `MCPIndicator.tsx` (Toolbar tag), `mcpStore.ts`; gated behind the `mcpServer` flag (Integrations category, admin-lockable). **MCP SDK gotcha**: `AddTool` panics if the `Out` type's schema isn't an object, so all tool handlers use `Out = any` (schema omitted) and return JSON-as-text content.
- **Snowpark / Notebooks**: `internal/snowpark/` — Wails-bound `Service` for the local Python environment (conda env or venv), per-tab Jupyter kernel lifecycle, cell execution, and pip package management. The Step-3 package manager supports single-package install/uninstall plus dependency files: install from a `requirements.txt` (`pip install -r`), install a project from a `pyproject.toml` (`pip install <dir>`), and freeze the active environment to a file (`pip freeze`). All install paths go through the shared `pipCmd` helper (venv pip binary or `conda run -n <env> pip`) and apply the corporate `PipRegistryConfig` (registry URLs, auth, proxy, CA cert). Frontend lives in `frontend/src/components/snowpark/` (SnowparkSetupModal, PipRegistryModal, SnowparkCheckModal).
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
- **Modals**: Use `antd` `Modal` with `destroyOnClose`. Prefer conditionally mounting the modal (`{open && <Modal open … />}`) so it unmounts on close — every modal is globally draggable (by its header) and width-resizable (bottom-right corner) via `frontend/src/utils/modalDragResize.ts` + the `.ant-modal` rules in `styles/global.css`, and that global feature relies on the unmount to reset a dragged/resized dialog to its default on reopen (a modal left mounted across close keeps its last position/width). Don't add per-modal drag/resize props.
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
- **Shift+click range**: `onSelect` also handles `nativeEvent.shiftKey`, selecting every object node between `objAnchorKey` (the last Cmd/Ctrl-clicked pivot) and the click. Visible order comes from `flattenVisibleNodes(displayData, expandedSet, …)`, which walks the tree against the controlled `expandedKeys`. The tree wrapper sets `userSelect: none` and `preventDefault`s shift-mousedown so the range click doesn't paint a browser text selection.

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

### Insert Row
- `InsertRowModal` builds a multi-row `INSERT INTO … VALUES (…), (…)` from a per-column grid form (distinct from Insert Mapping's table-to-table `SELECT`). One grid row per record, one column per table column, with Add/Remove row.
- Columns come from `GetTableColumnsWithTypes`; each cell is a literal **Value** (rendered per data type), a raw **Expr** (from the built-in function picker), **NULL**, or **DEFAULT**.
- In **Value** mode `InsertCellInput` picks the widget from the column's parsed type (`insertCellTypes.ts`): numeric field, TRUE/FALSE select, date/time picker, JSON textarea (`VARIANT`/`OBJECT`/`ARRAY`), vector array box, hex/UUID/WKT inputs. The JSON/geospatial textareas add a `SnippetToolbar` — a **Template** picker (`snippetsFor`) plus a JSON **Format** action (`formatJson`). Validation is UX-only; the Go builder does all injection-safe rendering.
- SQL is built in Go by `BuildInsertRowsSql` (`internal/table`) — never inline in the component — shown live via `SqlPreview` and executed with `ExecDDL`. Semi-structured (`PARSE_JSON`/`TO_OBJECT`/`TO_ARRAY`) and vector array casts are illegal in a `VALUES` clause, so any such cell switches the whole statement to `INSERT … SELECT … UNION ALL …`.
- Gated behind the `insertRow` feature flag.

## ⚠️ Gotchas
- **Logs**: `gosnowflake` driver logs errors to `slog.Default` even when caught.
- **Wails Generate**: If `wails generate module` fails, check Go syntax errors first.
- **Persistence**: App state is persisted in `~/.config/thaw/config.json`. Frontend store persistence uses `localStorage`. `config.Save` writes atomically (temp+rename via `filesystem.WriteFileAtomic`) so a second Thaw process never reads a torn file; any read-modify-write of the config MUST go through `config.Update(fn)` (process-locked) rather than a bare `Load()`→mutate→`Save()`, or a concurrent write in the same process can silently revert it.
- **Secrets**: Thaw-owned secrets (AI API key, Git OAuth client secrets, pip registry credential/proxy passwords, MCP session tokens) are **never** written to `config.json` — they live in the OS secure store via `internal/secrets` (macOS Keychain, Windows Credential Manager, Linux Secret Service; `0600` file fallback otherwise). `config.save()` scrubs these fields on every write and `config.Load()` migrates any legacy plaintext once. `App.GetSecretStorageInfo` reports the active backend to the Settings storage indicator. `~/.snowflake/config.toml` is out of scope (shared with the Snowflake CLI — `internal/sfconfig`).

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
3. Updates `wails.json` and `release/CHANGELOG.md`.
4. Pushes a signed version-bump commit and the `vX.Y.Z` tag.
5. Creates the GitHub Release.

Manual tags bypass the changelog and version-file update, breaking the pipeline.
