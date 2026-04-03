# Thaw — Gemini CLI Context & Mandates

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend).

## 💡 Critical Context
- **Nature of App**: This is a **Snowflake SQL Editor** and management tool.
- **Authentication**: Authentication is handled by parsing connection parameters from the **Snowflake CLI configuration file** (defaults to `~/.snowflake/config.toml` or `connections.toml`). Users can select a custom path during sign-in, which is persisted in the app configuration.
- **Tech Stack**: Go 1.22, Wails v2, React 18, TypeScript 5.6, Monaco Editor, Ant Design 5, Zustand 5.

## 🏗 Architecture Overview
- **Go Backend**: Core logic is in `app.go` (Wails IPC bindings) and `internal/`.
- **Snowflake Client**: Located in `internal/snowflake/client.go`. Enriched `ColumnInfo` here for metadata-heavy tasks.
- **Frontend**: React application in `frontend/src/`.
- **State Management**: Zustand stores are in `frontend/src/store/`.
- **IPC Flow**: Frontend calls `wailsjs/go/main/App.ts` → Go `*App` methods in `app.go`.

## 🛠 Engineering Standards
- **Surgical Edits**: Prefer `replace` over `write_file` for large files like `app.go` and `Sidebar.tsx`.
- **Wails Bindings**: After modifying Go method signatures in `app.go`, you MUST run `wails generate module` to update frontend bindings.
- **New Feature Pattern**:
    1. Define state in a new `zustand` store in `frontend/src/store/` (optional).
    2. Create UI components in `frontend/src/components/` (e.g., `database/CreateTableModal.tsx`, `layout/`).
    3. Register context menu actions in `frontend/src/components/layout/Sidebar.tsx`.
- **SQL Generation**: Use double quotes for identifiers (`"DATABASE"."SCHEMA"."TABLE"`) and handle escaping (`" -> ""`).

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
1. Define a public method on `*App` in `app.go`.
2. Run `wails generate module`.
3. Import and use the method in the React component from `../../../wailsjs/go/main/App`.

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
- `ObjectSummariesModal` fetches detailed table metadata via `GetDatabaseTableSummary` in `app.go`.
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
