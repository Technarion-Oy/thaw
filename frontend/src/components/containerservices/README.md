# frontend/src/components/containerservices

> Account-wide **Container Services** control center for Snowpark Container
> Services (SPCS). Issue #939.

Until this dialog existed, SPCS was reachable only through schema-node context
menus (Service, Image Repository, Gateway), and compute pools, job services and
volume snapshots had no UI at all. This is the cross-schema view the object tree
cannot give: every list is a `SHOW … IN ACCOUNT`.

| File | What it does |
|------|--------------|
| `ContainerServicesModal.tsx` | The dialog: one wide modal, six tabs (Compute pools / Services / Jobs / Images / Snapshots / Gateways), one per Snowflake SPCS command group. Opened from `containerServicesStore`, mounted once in `QueryPage.tsx` (lazy, next to `TagManagementModal`). The per-tab `TABS` table supplies each tab's label, plural object name and primary-action label. |
| `ContainerTabLayout.tsx` | Shared chrome every tab renders, so the six read as one system: toolbar (free-text filter → facet selects → Refresh → one primary **New …** action), dense antd table, detail panel for the selected row. It owns the search text, the selection, the error banner and the empty state — which names the active role and the fix ("No compute pools are visible to ROLE. Create one or switch role."). |

## How a tab gets filled in

Each tab is its own issue: #941 compute pools, #942 jobs, #943 images +
gateways, #944 services, #945 snapshots. A tab that lands replaces its
placeholder `notice` with real `columns`, `rows`, `loading`/`error`, a `filter`,
a `detail` renderer and a row `⋯` menu, all passed to `ContainerTabLayout`. The
shell deliberately ships no per-object modals of its own: the tabs reuse the
existing `CreateServiceModal` / `ServicePropertiesModal` /
`CreateImageRepositoryModal` / `ImageRepositoryPropertiesModal` /
`CreateGatewayModal` / `GatewayPropertiesModal` rather than duplicating them.
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

## Conventions

- Confirmation dialogs use `App.useApp()`'s `modal`, never static
  `Modal.confirm` — see [`../README.md`](../README.md#conventions).
- Destructive lifecycle verbs say what disappears ("Suspending deletes the
  containers; Resume rebuilds them from the spec").
- Every create/run form keeps a live `SqlPreview`
  ([`../shared/`](../shared/README.md)).
