# frontend/src/components/lineage

> Unified Dependencies & References viewer for any Snowflake object.

## Responsibility

`DependenciesModal.tsx` is a single, consistent surface for both directions of an object's
dependency graph, reachable from the **Dependencies & References…** context-menu entry on
*every* object kind in the sidebar. It has two tabs:

- **Depends on** (downstream — what this object uses):
  - **Parsed from DDL** (`ParserTreeSection`) — a recursive Ant Design Tree built live from the
    object's DDL for views/procedures/functions (instant, no extra privileges). Each node shows
    the object's kind icon, colour-coded type tag, qualified name, and circular-reference/error
    markers.
  - **From ACCOUNT_USAGE** (`UsageSection`) — a flat list from
    `SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES` that covers kinds the parser cannot read
    (tables, non-SQL bodies). Auto-loads for non-parseable kinds; behind a **Load** button
    otherwise.
- **Referenced by** (upstream — what uses this object) — a flat `UsageSection` from
  `OBJECT_DEPENDENCIES` in the reverse direction, auto-loaded on first visit to the tab.

Hovering any tree node or list row fetches and displays the object's DDL in a 60-second-cached
tooltip (`DdlTooltip`).

## Files

| File | Purpose |
|------|---------|
| `DependenciesModal.tsx` | Unified modal. Sub-components: `ParserTreeSection` (DDL-parsed tree), `UsageSection` (ACCOUNT_USAGE flat list, one instance per direction), `DdlTooltip` (lazy DDL hover), `NodeLabel` (icon + type tag + qualified name). |

## Patterns & integration

**IPC calls:**
- `GetObjectDependencies(db, schema, kind, name, args)` — DDL-parsed recursive `snowflake.DependencyNode` tree; fetched by `ParserTreeSection` (kinds in `PARSER_KINDS` only)
- `GetObjectUsageDependencies(db, schema, name, direction)` — flat `snowflake.ObjectDependencyRef[]` from `OBJECT_DEPENDENCIES`; `direction` is `"depends_on"` or `"referenced_by"`. Governance-privileged and laggy, so `UsageSection` labels the caveats and degrades to a warning alert on error
- `GetObjectDDL(db, schema, kind, name, args)` — fetched lazily on `mouseEnter` inside `DdlTooltip`; results cached in a module-level `Map` with 60s TTL

**Tree construction:** `toTreeNodes` builds `DataNode[]` from the recursive `DependencyNode` tree, populating a `Map<nodeKey, NodeMeta>` in parallel. Node metadata is stored in a `useRef` (not state) so `titleRender` always sees fresh values without extra re-renders.

**Key caveat:** Ant Design Tree strips unknown properties from `DataNode` objects internally. All custom metadata (dep object, args signature) must be looked up from `nodeMetaRef.current` inside `titleRender` — never rely on custom fields on the `DataNode` argument passed to `titleRender`.

**DDL tooltip trigger:** `DdlTooltip` triggers the DDL fetch on `onMouseEnter` rather than `onOpenChange` because Ant Design disables the tooltip entirely when `title={null}`, which prevents `onOpenChange` from firing in the initial state.

**Domain → icon mapping:** `OBJECT_DEPENDENCIES` reports multi-word domains (e.g. `MATERIALIZED VIEW`, `EXTERNAL FUNCTION`). `bucketFor` maps any domain to the closest `TABLE`/`VIEW`/`PROCEDURE`/`FUNCTION` icon/colour bucket by keyword, while the type tag still shows the full domain string.

**Routine DDL hover:** `OBJECT_DEPENDENCIES` does not report argument signatures, and `GET_DDL` for a `PROCEDURE`/`FUNCTION` appends `()` and fails to resolve any parameterized overload. So in the `UsageSection` flat lists, `isRoutineDomain` rows render the plain `NodeLabel` with **no** `DdlTooltip` (avoiding a silently-empty tooltip), and a footnote explains the omission when any routine rows are present. Non-routine kinds (tables, views, …) keep the hover since they need no signature.

## Gotchas

- `nodeCounter` is a module-level mutable counter reset to `0` before each tree build to generate stable, sequential keys. This means the component is not safe for concurrent instances in the same module scope.
- Only SQL-language objects are expanded by the DDL parser; tables and non-SQL procedures appear as leaf nodes even if they have dependents — the **From ACCOUNT_USAGE** section is the fallback for those.
- `UsageSection` and `ParserTreeSection` are keyed by object identity so switching to a different object remounts them (fresh load), rather than showing stale results.
- The specialized policy/tag/DMF **References** sections in their `*PropertiesModal.tsx` are intentionally *not* subsumed here — they show the *applied-to* relationship (`POLICY_REFERENCES`/`TAG_REFERENCES`/`DATA_METRIC_FUNCTION_REFERENCES`) that `OBJECT_DEPENDENCIES` does not record.
