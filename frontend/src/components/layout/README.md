# frontend/src/components/layout

> Top-level shell layout with resizable sidebars, panel drag-and-drop, and the full Snowflake object browser tree.

## Responsibility

Composes the outer application shell (`AppLayout`) and the primary object browser (`Sidebar`).
`AppLayout` owns the resizable left/right sidebar regions, panel drag-and-drop, feature-flag
gating of optional panels, and the macOS title bar offset. `Sidebar` renders the Ant Design
`Tree` of databases, schemas, and Snowflake objects, handles all context menus, inline modals,
and the three-tier object-listing cache cascade.

## Files

| File | Purpose |
|------|---------|
| `AppLayout.tsx` | Root shell. Renders left `Sidebar`, centre `QueryPage`, and the draggable panels (`ExportPanel`, `FileBrowser`, object `Sidebar`, `AccountPanel`) — there is no standalone Git panel; git was folded into `FileBrowser`. Implements `useResize` hook for drag-to-resize sidebar widths (clamped 160–600 px), `ResizeHandle` component, and panel drag-and-drop reordering. Reads `panelLayoutStore` for panel order/sizes. Listens for `menu:*` Wails events (incl. `menu:git-operations`, which opens `GitOperationsDialog`). Adjusts for macOS title bar (40 px offset). |
| `Sidebar.tsx` | Object browser. Builds and maintains the `DataNode` tree for databases → schemas → object type groups → objects → columns/sub-nodes. Implements `loadData` (lazy expansion), `buildTaskTree` (hierarchical TASK graph), `buildEntryNodes` (stages and DBT projects), `filterTree` (search), `removeNode`/`clearNodeChildren` (surgical tree mutations), and `menuItem` (context menu factory with `disabled`/`disabledReason` for feature gating). Owns all inline modals (40+). |

## Key patterns in `Sidebar.tsx`

### `menuItem` — context menu factory
```ts
menuItem(label, icon, handler, shortcut?, disabled?, disabledReason?)
```
The 5th parameter (`disabled`) hides or disables the item; the 6th (`disabledReason`) shows a
tooltip explaining why. Feature flags are read from `featureFlagsStore` and passed here — never
invert the gating inside handlers.

### Three-tier object-listing cache
1. `objectStore` — previously expanded schemas (instant, all types).
2. Go TTL cache (`ListObjects` / `ListBasicObjects`) — 30 s backend cache.
3. `ListBasicObjects` fallback — single query, tables/views/sequences only.

`ClearObjectCache()` / `ClearObjectCacheForDatabase(db)` IPC methods reset the backend cache;
called from `refreshAllDatabases` / `refreshDatabaseByName`.

### Node key formats
| Key prefix | Meaning |
|-----------|---------|
| `db:NAME` | Database node |
| `schema:DB:SCHEMA` | Schema node |
| `type:DB:SCHEMA:KIND` | Object type group |
| `obj:DB:SCHEMA:KIND:NAME` | Individual object |
| `col:DB:SCHEMA:TABLE:COLUMN` | Column leaf node |
| `stagedir:DB:SCHEMA:NAME:path` | Stage directory |
| `stagefile:DB:SCHEMA:NAME:path` | Stage file |
| `gitbranches:/gittags:/gitcommits:` | Git ref groups |
| `gitdir:DB:SCHEMA:REPO:path` | Git directory |
| `gitfile:DB:SCHEMA:REPO:path` | Git file |
| `dbtversion:DB:SCHEMA:NAME:ver` | DBT project version |
| `dbtdir:DB:SCHEMA:NAME:path` | DBT directory |
| `dbtfile:DB:SCHEMA:NAME:path` | DBT file |

### Task tree
`buildTaskTree` builds a nested `DataNode` hierarchy from a flat `SnowflakeObject[]` list using
the `finalize` and predecessor relationship fields. Finalizer nodes are placed as the last child
of their root task with `isFinalizer: true`; root tasks with no predecessors get `isRootTask: true`.

### Object DDL hover cache
`ddlCache` (module-level `Map`, 60 s TTL) caches DDL fetched via `GetObjectDDL` to avoid
repeated IPC calls on tree hover.

## Stores used

`AppLayout.tsx`: `panelLayoutStore` (panel order, widths), `featureFlagsStore`, `gitStore`.

`Sidebar.tsx`: `queryStore` (open new tab, insert SQL), `objectStore` (schema/object cache),
`connectionStore` (active DB/schema/role), `gitStore`, `diffStore`, `insertMappingStore`,
`featureFlagsStore`.

## IPC calls in `Sidebar.tsx` (representative)

`ListDatabases`, `ListSchemas`, `ListObjects`, `ListBasicObjects`, `ClearObjectCache`,
`ClearObjectCacheForDatabase`, `GetObjectDDL`, `GetObjectProperties`, `ExportDatabaseDDL`,
`ListDroppedTables`, `ListDroppedSchemas`, `ListDroppedDatabases`, `GetTableRetentionDays`,
`GetERDiagramData`, `FetchNotebookContent`, `DropTaskTree`, `GetTableColumnsWithTypes`,
`GetTableForeignKeys`, `ListGitRepoEntries`, `ListGitBranches`, `ListGitTags`, `ExecuteGitFile`,
`DropDatabase`, `DropSchema`, `AlterPipe`, `UploadFileToStage`, `ListStageEntries`,
`ExecuteStageFile`, `ListDbtProjectVersions`, `ListDbtProjectEntries`, `DownloadFileFromStage`,
`RemoveStageFiles`, `BuildDropColumnSql`, `BuildRenameColumnSql`, `BuildSetColumnNotNullSql`,
`BuildDropColumnNotNullSql`, `BuildSetColumnCommentSql`, `BuildChangeColumnTypeSql`.

## Gotchas

- **Do not call `GetObjectDDL` with a guessed kind.** The gosnowflake driver logs every failed
  DDL attempt at ERROR level even when the caller catches the error. Always resolve the kind from
  the objects store or a prior `ListObjects` call before calling `GetObjectDDL`.
- **`loadingGitNodes` Set** uses namespaced keys so stage, git, and DBT loading states never
  collide despite sharing the same Set.
- **Column DDL** (ADD/DROP/RENAME/ALTER COLUMN) is always built in the backend
  (`internal/column`). `Sidebar.tsx` and `AddColumnModal` only collect config and call the
  `Build*ColumnSql` IPC methods — SQL is never constructed inline in the frontend.
- **`buildEntryNodes`** is the shared helper for both stage file nodes and DBT project file nodes
  (they have identical sub-tree shapes); `emptyChildNode` provides the empty-state placeholder.
- **Column management actions** (Add/Rename/Change Type/Set Comment/Set NOT NULL/Drop NOT NULL/Drop)
  are all gated behind the `columnManagement` feature flag. "Insert Column Name" is never gated.
- **`removeNode`** surgically deletes a file/object node from the tree after DROP so the parent
  directory stays expanded without a full refresh.
- **`refreshDatabaseByName(db, reveal?)` preserves the open path AND scroll position.** Naively
  stripping the whole `db:` subtree drops every descendant `schema:`/`type:`/`obj:` node from
  `treeData` while their keys linger in `expandedKeys`, so Ant Design renders the previously-open
  path collapsed; the tree also briefly shrinks to nothing, resetting the scroll container to the
  top (issue #493). Instead it re-fetches the schema list (`ListSchemas`) and rebuilds the db node's
  children via **`syncDatabaseSchemas`**, which keeps the loaded children of currently-open schemas
  intact (no collapse, no flicker) while picking up new / `UNDROP`-restored schemas, dropping
  removed ones, and resetting collapsed schemas to childless nodes so their objects re-fetch on the
  next expand. It then reloads each open schema's objects in place — fanned out with `Promise.all`
  (the per-schema `setData`s are independent and order-insensitive, so there's no reason to serialize
  the `ListObjects` round-trips). Scroll is captured before the rebuild and restored via a double
  `requestAnimationFrame` afterwards (via `treeScrollRef`): the first frame lets React flush the
  batched commits, the second runs after layout so `scrollTop` sticks. The optional
  `reveal: { schema, kind }` (passed by create/rename handlers) force-expands the object's
  `schema → type` path so a brand-new type group opens automatically — and because `syncDatabaseSchemas`
  materialises the target schema node first, the reveal works even when that schema wasn't in the
  tree before. When the db node itself is collapsed (and there's no reveal) it falls back to
  `clearDatabase` + `clearNodeChildren` so the next expand re-fetches everything. **Pitfall:** do
  not "optimise" this by skipping the `ListSchemas` re-fetch — without it `UNDROP SCHEMA`, externally
  created schemas, and stale collapsed-schema caches are all missed. `expandedKeys` is
  component-local state — the `objectStore` does not track expansion.
- Panel resize widths are clamped to 160–600 px by `useResize`. Committed widths are persisted
  via `panelLayoutStore` to `session.json`.
- The macOS title bar offset (`TITLEBAR_HEIGHT = 40`) is applied only when `IS_MAC` is true;
  do not hard-code this offset elsewhere.
