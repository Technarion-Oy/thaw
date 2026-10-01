# frontend/src/components/containerservices

> Account-wide **Container Services** control center for Snowpark Container
> Services (SPCS). Issue #939.

Until this dialog existed, SPCS was reachable only through schema-node context
menus (Service, Image Repository, Gateway), and compute pools, job services and
volume snapshots had no UI at all. This is the cross-schema view the object tree
cannot give: every list is a `SHOW … IN ACCOUNT`.

| File | What it does |
|------|--------------|
| `ContainerServicesModal.tsx` | The dialog: one wide modal, six tabs (Compute pools / Services / Jobs / Images / Snapshots / Gateways), one per Snowflake SPCS command group. Opened from `containerServicesStore`, mounted once in `QueryPage.tsx` (lazy, next to `TagManagementModal`). `LABELS` holds each tab's title; each tab component owns its plural object name and primary action. |
| `ComputePoolsTab.tsx` | **Compute pools** tab (#941): `SHOW COMPUTE POOLS` table (state dot, `active (min–max)` nodes, family, …), State facet, row ⋯ menu (Properties / Suspend·Resume / Stop all services… / Drop — confirms via `App.useApp()`), **New compute pool…** and **Instance families…**. Holds the instance-family list: `loadFamilies()` caches one promise per dialog open (cleared on failure, so the next need retries), shared by the three modals below. Stop-all workload types are checked before the confirm closes. |
| `ServicesTab.tsx` | **Services** tab (#944): `SHOW SERVICES EXCLUDE JOBS IN ACCOUNT` table keyed by `db.schema.name` (`App.ListServicesInAccount`; job services are `JobsTab`'s). Status dot + word (`computePools.serviceStatusColor`, shared with `JobsTab` — the two SHOW commands share the same status vocabulary; `SUSPENDING`/`DELETING` show amber, not failure red), name, `database.schema`, compute pool, `current (min–max)` instances, auto-resume, query warehouse, owner, updated; Status / Compute pool / Database facets. Row ⋯ menu: Properties… (`service/ServicePropertiesModal`), Suspend/Resume (`AlterService`; `computePools.serviceLifecycle` picks the label and disables it while `SUSPENDING`/`DELETING`; same confirm copy as the object-tree context menu in `layout/Sidebar.tsx`), Redeploy… (opens the same modal with `focusSpec` so it scrolls straight to the Specification section, once — not again after each in-modal reload), Drop (`DropService`, confirm-guarded). Detail panel: a status line plus `shared/LazyResultTable` for Endpoints and Instances — the same lazy sections Properties has, reused directly rather than duplicated. **New service…** through `ScopedCreate`. |
| `JobsTab.tsx` | **Jobs** tab (#942): `SHOW JOB SERVICES IN ACCOUNT` table keyed by `db.schema.name` (status dot, pool, async, created, owner), Status facet, row ⋯ menu (Logs… → `service/ServiceLogs` in a small modal, Properties… → `service/ServicePropertiesModal`, Drop → `DropJobService`, confirm-guarded), **Run job…**. After a run it refreshes for an async job, or closes the whole dialog for a synchronous one so its query tab is visible. |
| `RunJobModal.tsx` | **Job / Inference job** `Segmented` switch over `CreateModalShell` with live `SqlPreview` (`BuildExecuteJobServiceSql` / `BuildExecuteInferenceJobServiceSql`). Job reuses `service/ServiceSpecFields`; Inference takes spec YAML, a query (`shared/MonacoSqlField`) or stage path (`shared/StageFilePicker`), a model picker (`model/ModelSourcePicker`'s cached `loadModelsCached`), version, function. **Run** hands the SQL to `queryStore.executeInNewTab`. |
| `ImagesTab.tsx` | **Images** tab (#943): `SHOW IMAGE REPOSITORIES IN ACCOUNT` table (copy-URL per row), detail = that repository's images (`ListImagesInRepository`, **Copy pull command** from `image_path`) plus `imagerepository/RegistryCommands`. Row menu: Properties… / Drop. **New repository…** goes through `ScopedCreate`. |
| `SnapshotsTab.tsx` | **Snapshots** tab (#945): `SHOW SNAPSHOTS IN ACCOUNT` table keyed by `db.schema.name` (name, database.schema, service, volume, instance, size, state, comment — service/instance read under their likely column aliases, unverified live); Service / Database facets plus a **Restore dropped…** button (`ScopedCreate` → `snapshot/RestoreSnapshotModal`). Row menu: Properties… (`snapshot/SnapshotPropertiesModal`), Drop (`DropSnapshot`; the success toast has an **Undo** → `UndropSnapshot`). **New snapshot…** through `ScopedCreate` → `snapshot/CreateSnapshotModal`. |
| `GatewaysTab.tsx` | **Gateways** tab (#943): `SHOW GATEWAYS IN ACCOUNT` table; row menu Copy ingress URL (falls back to `DescribeGateway` when SHOW omits it) / Properties… / Drop; **New gateway…** through `ScopedCreate`. |
| `ScopedCreate.tsx` | Account-scope entry for schema-scoped create modals: database + schema picker (session defaults; schemas from `ListUserSchemas`, so no `INFORMATION_SCHEMA`), then renders the real modal. |
| `CreateComputePoolModal.tsx` | `CREATE COMPUTE POOL` form on `CreateModalShell` with live `SqlPreview` (`BuildCreateComputePoolSql`); instance family picker labelled with vCPU / memory / GPU / usage, backup families multi-select, `shared/TagInput`. |
| `ComputePoolPropertiesModal.tsx` | `DESCRIBE COMPUTE POOL` merged over the SHOW row; `common/PropertyRows` `EditRow`s from a keyword-keyed `SETTABLE` list → `AlterComputePoolProperty` (SET, or UNSET on blank; MIN ≤ MAX checked first; the INSTANCE_FAMILY select loads lazily and always includes the pool's current family), `TagsRow` via `useObjectTags` (kind `COMPUTE POOL`), read-only rest, lazy **Nodes** `shared/LazyResultTable`. |
| `InstanceFamiliesModal.tsx` | Read-only `SHOW COMPUTE POOL INSTANCE FAMILIES` table. |
| `computePools.ts` | Pure helpers: `rowsOf` (QueryResult → lower-cased-key rows), `familyLabel` / `familyOptions`, `familyListText`, `stateColor`. Tested in `computePools.test.ts`. |
| `ContainerTabLayout.tsx` | Shared chrome every tab renders, so the six read as one system: toolbar (free-text filter → facet selects → Refresh → one primary **New …** action), dense antd table, detail panel for the selected row. It owns the search text, the selection, the error banner and the empty state — which names the active role and the fix ("No compute pools are visible to ROLE. Create one or switch role."). |

## How a tab gets filled in

Each tab was its own issue: #941 compute pools, #942 jobs, #943 images +
gateways, #944 services, #945 snapshots — all landed. A tab passes its
`columns`, `rows`, `loading`/`error`, a `filter`, an optional `detail` renderer
and a row `⋯` menu to `ContainerTabLayout`. The
shell deliberately ships no per-object modals of its own: the tabs reuse the
existing `CreateServiceModal` / `ServicePropertiesModal` /
`CreateImageRepositoryModal` / `ImageRepositoryPropertiesModal` /
`CreateGatewayModal` / `GatewayPropertiesModal` / `CreateSnapshotModal` /
`SnapshotPropertiesModal` rather than duplicating them.
Those modals take `database`/`schema` props, so a tab opening one from account
scope has to pick a database and schema first (`CreateModalShell` itself is pure
chrome and knows nothing about either).

## Entry points

- **Snowpark → Container Services ▸** — six native menu items, all emitting
  `menu:container-services` with the tab name as payload (`internal/app/menu.go`).
  `QueryPage.tsx` has the single listener, next to `menu:tag-management`;
  `toContainerTab` narrows the payload.
- Greyed out while disconnected **and** while the `containerServices` feature
  flag is off (View → Enabled Features… → Developer Environments).
- Right-clicking the **Services** group header in the object tree also opens
  the dialog, on the Services tab (`layout/Sidebar.tsx`'s `openManageServices`,
  via `containerServicesStore.openView("services")`) — mirrors Tags → **Manage
  Tags…**. Greyed out (with a tooltip) while the feature flag is off.

## Conventions

- Confirmation dialogs use `App.useApp()`'s `modal`, never static
  `Modal.confirm` — see [`../README.md`](../README.md#conventions).
- Destructive lifecycle verbs say what disappears ("Suspending deletes the
  containers; Resume rebuilds them from the spec").
- Every create/run form keeps a live `SqlPreview`
  ([`../shared/`](../shared/README.md)).
