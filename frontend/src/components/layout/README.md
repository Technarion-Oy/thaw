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
| `AppLayout.tsx` | Root shell. Renders left `Sidebar`, centre `QueryPage`, and the draggable panels (`FileBrowser`, object `Sidebar`, `AccountPanel`) — there is no standalone Git panel (folded into `FileBrowser`) and no Export panel (DDL export lives in the Tools → Export Database DDL… dialog). Implements `useResize` hook for drag-to-resize sidebar widths (clamped 160–600 px), `ResizeHandle` component, and panel drag-and-drop reordering. Reads `panelLayoutStore` for panel order/sizes. Listens for `menu:*` Wails events (incl. `menu:git-operations`, which opens `GitOperationsDialog`; `menu:open-folder` → `gitStore.pickExportDir`; `menu:open-folder-new-window` → `gitStore.openInNewWindow`). Adjusts for macOS title bar (40 px offset). |
| `Sidebar.tsx` | Object browser. Builds and maintains the `DataNode` tree for databases → schemas → object type groups → objects → columns/sub-nodes. Implements `loadData` (lazy expansion), `buildTaskTree` (hierarchical TASK graph), `buildEntryNodes` (stages and DBT projects), `buildSearchTree` (groups backend account-search hits into tree nodes) + the search UI (see below), `removeNode`/`clearNodeChildren` (surgical tree mutations), and `menuItem` (context menu factory with `disabled`/`disabledReason` for feature gating). Owns all inline modals (40+). |
| `objectSearch.ts` | Pure matching logic for the advanced object search — `buildSearchPredicate` (compiles a `{ query, regexMode, caseSensitive, kinds }` config into a memoizable `{ matches, active, regexError, needsFullLoad }` predicate) and `filterTree` (prunes the tree to matches). No React/antd runtime, so it is unit-tested directly (`objectSearch.test.ts`) in the node vitest env. |

## Key patterns in `Sidebar.tsx`

### `menuItem` — context menu factory
```ts
menuItem(label, icon, handler, shortcut?, disabled?, disabledReason?)
```
The 5th parameter (`disabled`) hides or disables the item; the 6th (`disabledReason`) shows a
tooltip explaining why. Feature flags are read from `featureFlagsStore` and passed here — never
invert the gating inside handlers.

### `menuItemSub` — cascading submenu factory
```ts
menuItemSub(label, icon, subKey, children, depth?)
```
Renders a hover-opening submenu panel (150 ms hide delay, viewport clamping via `clampSubPanel`,
left/right auto-flip near the screen edge tracked in `submenuDirs`). `subKey` must be unique among
siblings at the same `depth`; the open panel is the one whose key equals `submenuPath[depth]`.
Disabled child `menuItem`s keep their tooltip, mirrored to the panel's open direction. Used by the
schema node's **Create Object** menu, the database node's **Reports** menu, and the plain **TABLE**
object menu, whose actions are grouped into **Query / Data / Tools** submenus (with the structure
actions — Add Column…, Rename…, View Definition, Properties — and **Delete…** left top-level). Only
the `TABLE` kind is grouped — every other object kind's menu stays
a flat list, so the shared `obj` entries (Tag References…, Insert Full Name, View Definition,
Properties, Select for Comparison, Compare with…, Rename…) carry an explicit `objKind !== "TABLE"`
guard to avoid double-rendering once they also appear inside a table submenu.

### `isInfoSchema` — read-only system-schema guard
`INFORMATION_SCHEMA` is Snowflake-owned and read-only (views + table functions only; no DDL,
Time Travel, or tagging). The module-level `isInfoSchema(nodeKey)` helper reports whether a
`schema:DB:SCHEMA` key points at it, and the schema context menu uses it to hide the infeasible
items (**Create Object**, **Show Dropped Objects…**, **Export Data…**, **Import Data…**,
**Backup Sets…**, **Tag References…**, and **Drop Schema…**), leaving only **Insert Name** and
**Properties**. Properties opens `SchemaPropertiesModal` with `readOnly` set, so it renders as a
value dump with no ALTER controls (issue #854). Mirrors the backend's `ListUserSchemas` exclusion
(`internal/app/objects.go`).

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

### Advanced object search
The sidebar's search box is an advanced, **account-wide** search backed by the server, not a walk of
the loaded tree (issues #855 + follow-up):
- **Backend search, not a cascade.** Typing (debounced ~250 ms) calls `App.SearchAccountObjects(namePattern, kinds)`,
  which runs **one `SHOW <kind> IN ACCOUNT` per kind** (see `internal/snowflake/search.go`). This
  replaced the old per-schema cascade that issued `SHOW SCHEMAS` for every database + `SHOW …` for
  every schema — `O(databases × schemas)` round-trips, pathological on large accounts (a single
  Streamlit meant thousands of queries). Now a Streamlit-only search is **one** query. `buildSearchTree`
  regroups the flat hits (each carries its `database`) back into the normal
  `db → schema → type-group → object` `DataNode` shape, so the renderer, context menus, and on-expand
  column/file loading all work unchanged.
- **Query** — case-insensitive substring by default; **`.*`** toggles regex mode and **`Aa`** toggles
  case sensitivity (both in the input suffix). Invalid regex never throws — the input shows an error
  state and the matcher degrades to a literal substring match. Substring queries are pushed to the
  server as a `LIKE '%…%'` prefilter; **regex** queries fetch all of the selected kinds (no server
  filter) and match client-side.
- **Type filter** — a flat multi-select of object kinds (`KIND_FILTER_OPTIONS`, in `KIND_ORDER`).
  Each selected kind is one `SHOW … IN ACCOUNT`, so any subset is cheap (no fast/slow distinction —
  the old cascade-depth story is gone).
- **The predicate** is built once per query by `buildSearchPredicate` (memoized on
  `{ query, regexMode, caseSensitive, kinds }`) and threaded into `filterTree(nodes, matches)`; the
  `RegExp` compiles a single time, not per node. It is the **precise client-side pass** (exact case,
  regex — things SQL `LIKE` can't express) over results the backend already scoped by kind.
  `filterTree` matches obj: nodes by **name + kind parsed from the key**, prunes empty structural
  parents, and **preserves the full loaded subtree of a matched object** (columns / stage / git / dbt
  files / task subtree) so expanding a hit shows real content instead of an empty node. A matched node
  with empty loaded children is emitted as a leaf so it never renders a dead expander.
- **Fetch lifecycle.** The refetch trigger is `searchServerKey` — `[namePattern, kinds]`, where
  `namePattern` is the **debounced** text (empty in regex mode). Keying off the debounced query means a
  non-empty query never fires until typing settles (no fetch-everything on the first keystroke), and
  refining a **regex** doesn't refetch (server key unchanged) — only the client filter + auto-expand
  recompute. A `searchGenRef` generation counter drops a response from a superseded/cleared search.
  `searchResults` is separate from `treeData` (the user's own expansion), so clearing the search
  leaves the browsed tree intact. `searchActive` (query **or** kind filter present) drives search mode;
  teardown lives in a `searchActive` effect. `searchSettling` (debounce pending or fetch in flight)
  shows a "Searching…" spinner instead of flashing "no match".
- **Focus shortcut** — **⌘⇧F / Ctrl+Shift+F** focuses the search box. A `keydown` listener in
  `Sidebar.tsx` dispatches the `thaw:focus-object-search` window event; the event is kept so menus/MCP
  can focus it too.

Pure matching logic (`buildSearchPredicate`, `filterTree`) lives in `objectSearch.ts` and is
unit-tested in `objectSearch.test.ts`; the backend command planner (`planAccountSearchCommands`) is
tested in `internal/snowflake/search_test.go`.

- **Live updates on mutation.** A catalog mutation (drop / create / rename) calls `refreshActiveSearch()`,
  which bumps `searchRefreshToken` (a dependency of the fetch effect) to re-run the account search when
  one is active, so results reflect the change instead of showing a dropped object or missing a new one.
  It's wired into `refreshDatabaseByName` (the common object create/drop/rename path, incl. bulk delete)
  and the DROP DATABASE / DROP SCHEMA handlers; it's a no-op when no search is active. The re-query
  reflects the committed DDL (SHOW … IN ACCOUNT is uncached), and old results stay visible until the new
  ones arrive (no flash).

### Task tree
`buildTaskTree` builds a nested `DataNode` hierarchy from a flat `SnowflakeObject[]` list using
the `finalize` and predecessor relationship fields. Finalizer nodes are placed as the last child
of their root task with `isFinalizer: true`; root tasks with no predecessors get `isRootTask: true`.

### Object DDL hover cache
`ddlCache` (module-level `Map`, 60 s TTL) caches DDL fetched via `GetObjectDDL` to avoid
repeated IPC calls on tree hover.

### Multi-select (object nodes)
`selectedNodeKeys` (Set) + `selectedNodeArgs` (Map of function/procedure signatures) hold the
selection; the `Tree` is `multiple` with `selectedKeys={Array.from(selectedNodeKeys)}`. The
`onSelect` handler branches on the native modifiers: **Cmd/Ctrl+click** toggles a node (and sets
`objAnchorKey`, the range pivot); **Shift+click** selects every object node between `objAnchorKey`
and the click. Visible order for the range comes from `flattenVisibleNodes(displayData, expandedSet, …)`,
which walks the tree against the controlled `expandedKeys`/`searchExpandedKeys`. A plain (no-modifier)
click on the tree container clears the selection. The tree wrapper sets `userSelect: none` and
`preventDefault`s shift-mousedown so a range click doesn't paint a browser text selection. The
selection drives the context menu's bulk **Delete N selected objects** and **Add N as insert sources**.

### Show Dropped Objects (Time Travel undrop)
Three modals — schema scope (`undropModal`, from the schema context menu's **Show Dropped Objects…**),
database scope (`undropSchemasModal`, from the database context menu), and account scope
(`undropDatabasesModal`, from the Objects-panel toolbar) — all render dropped objects through the
shared module-level `DroppedObjectGroups` component, which sections the list by `DroppedTable.kind`
in a fixed order (`DROPPED_KIND_ORDER`: Tables → Iceberg Tables → Schemas → Databases) with a
pluralised heading + count per group. Each row shows the name, its `dropped_on` timestamp, and an
**Undrop** button; the row key is `name|droppedOn` so an object dropped-and-recreated-and-redropped
(same name, different timestamps) doesn't collide. `onUndrop` receives the whole row so the schema-scope
handler `undropSchemaObject` can pick `UNDROP ICEBERG TABLE` vs `UNDROP TABLE` from `kind`; the
schema/database handlers build `UNDROP SCHEMA` / `UNDROP DATABASE`. All run through `ExecDDL` and then
refresh the affected part of the tree. Only tables (regular + iceberg), schemas, and databases are
enumerable — dynamic tables, tags, streamlits, notebooks, and external volumes have no dropped-object
listing in Snowflake (see `internal/snowflake/README.md`) and are out of scope.

## Stores used

`AppLayout.tsx`: `panelLayoutStore` (panel order, widths), `featureFlagsStore`, `gitStore`.

`Sidebar.tsx`: `queryStore` (open new tab, insert SQL), `objectStore` (schema/object cache),
`connectionStore` (active DB/schema/role), `gitStore`, `diffStore`, `insertMappingStore`,
`featureFlagsStore`.

## IPC calls in `Sidebar.tsx` (representative)

`ListDatabases`, `ListSchemas`, `ListObjects`, `ListBasicObjects`, `SearchAccountObjects`, `ClearObjectCache`,
`ClearObjectCacheForDatabase`, `GetObjectDDL`, `GetObjectProperties`, `ExportDatabaseDDL`,
`ListDroppedTables`, `ListDroppedSchemas`, `ListDroppedDatabases`, `GetTableRetentionDays`,
`GetERDiagramData`, `FetchNotebookContent`, `DropTaskTree`, `GetTableColumnsWithTypes`,
`GetTableForeignKeys`, `ListGitRepoEntries`, `ListGitBranches`, `ListGitTags`, `ExecuteGitFile`,
`DropDatabase`, `DropSchema`, `AlterPipe`, `UploadFileToStage`, `ListStageEntries`,
`ExecuteStageFile`, `ListDbtProjectVersions`, `ListDbtProjectEntries`, `DownloadFileFromStage`,
`RemoveStageFiles`, `BuildDropColumnSql`. (The other column `Build*Sql` IPC methods now live in
`components/column/ColumnPropertiesModal`.)

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
- **Column management actions** — **Add Column…**, **Properties…** (which opens
  `ColumnPropertiesModal`, where Rename / Change Type / Default / Comment / NOT NULL / Masking Policy /
  Tags all live), and **Drop Column…** are gated behind the `columnManagement` feature flag.
  "Insert Column Name" and "Tag References…" are never gated.
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
