# frontend/src/components/lineage

> Object dependency tree viewer for Snowflake tables, views, procedures, and functions.

## Responsibility

Shows the recursive dependency graph of a Snowflake object as an expandable Ant Design Tree.
Each node displays the object's kind (TABLE/VIEW/PROCEDURE/FUNCTION), qualified name, and
circular-reference or error markers. Hovering any node fetches and displays its DDL in a
60-second-cached tooltip.

## Files

| File | Purpose |
|------|---------|
| `DependenciesModal.tsx` | Full dependency modal: tree build, DDL tooltip sub-component (`DdlTooltip`), node label sub-component (`NodeLabel`). |

## Patterns & integration

**IPC calls:**
- `GetObjectDependencies(db, schema, kind, name, args)` — fetches the full dependency tree as a `snowflake.DependencyNode` recursive struct; called on `open` state change
- `GetObjectDDL(db, schema, kind, name, args)` — fetched lazily on `mouseEnter` inside `DdlTooltip`; results cached in a module-level `Map` with 60s TTL

**Tree construction:** `toTreeNodes` builds `DataNode[]` from the recursive `DependencyNode` tree, populating a `Map<nodeKey, NodeMeta>` in parallel. Node metadata is stored in a `useRef` (not state) so `titleRender` always sees fresh values without extra re-renders.

**Key caveat:** Ant Design Tree strips unknown properties from `DataNode` objects internally. All custom metadata (dep object, args signature) must be looked up from `nodeMetaRef.current` inside `titleRender` — never rely on custom fields on the `DataNode` argument passed to `titleRender`.

**DDL tooltip trigger:** `DdlTooltip` triggers the DDL fetch on `onMouseEnter` rather than `onOpenChange` because Ant Design disables the tooltip entirely when `title={null}`, which prevents `onOpenChange` from firing in the initial state.

## Gotchas

- `nodeCounter` is a module-level mutable counter reset to `0` before each tree build to generate stable, sequential keys. This means the component is not safe for concurrent instances in the same module scope.
- Only SQL-language objects are expanded by the backend; tables and non-SQL procedures appear as leaf nodes even if they have dependents.
