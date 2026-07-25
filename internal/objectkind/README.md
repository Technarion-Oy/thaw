# internal/objectkind

> The single canonical registry of Snowflake object kinds: canonical KIND string, SHOW plural, display label, ordering, and GET_DDL object type.

## Responsibility

One entry per object kind the object browser supports, and every consumer — backend and frontend — derives from it.

Before this package the same facts were spelled out independently in the extended `SHOW` command list, the Properties query `switch`, the `GET_DDL` normalization `switch`, its reject-list, and the frontend's `KIND_LABEL` / `KIND_ORDER` maps. Adding a kind meant editing six or seven places and nothing failed when one was missed — a kind could be searchable but have no Properties query, or shown in the tree with no icon.

**The package is a leaf**: it imports nothing else from Thaw, so `internal/snowflake` (and anything else) can depend on it without an import cycle.

## Key files

| File | Purpose |
|---|---|
| `kinds.go` | The `Kind` struct, the ordered `Kinds` registry, and the lookups |
| `generate.go` | `go:generate` directive for the frontend artifact |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

```go
type Kind struct {
    Name              string // canonical KIND, e.g. "MATERIALIZED VIEW"
    Plural            string // SHOW noun, e.g. "MATERIALIZED VIEWS"
    Label             string // display label, e.g. "Materialized Views"
    Basic             bool   // sourced from SHOW OBJECTS (TABLE/VIEW/SEQUENCE) vs a dedicated SHOW
    GetDDLType        string // GET_DDL object type ("" = GET_DDL does not support the kind)
    Routine           bool   // overloads by argument signature (functions / procedures)
    NoPropertiesQuery bool   // opts out of the generic Properties SHOW (NOTEBOOK only)
}
```

| Function | Description |
|---|---|
| `Kinds` | The ordered registry. Order is the **display** order (tree grouping, search grouping, type-filter options) |
| `ByName(kind)` | Registry lookup, case-insensitive and whitespace-tolerant; `false` for unknown kinds |
| `Extended()` | The non-`Basic` kinds — those needing their own `SHOW` — as a fresh slice, in registry order |
| `IsExtended(kind)` | Whether a kind comes from its own `SHOW` rather than `SHOW OBJECTS`. Unknown kinds → `false` |
| `DDLUnsupported()` | Set of kinds with no `GetDDLType`, keyed by canonical name |

## Consumers

| Consumer | Derives |
|---|---|
| `internal/snowflake` `ListExtendedObjects` / `search.go` | One `SHOW <Plural> IN SCHEMA` / `IN ACCOUNT` per `Extended()` kind (`extendedShowKinds`) |
| `internal/snowflake` `buildGetDDLQuery` | The `GET_DDL` object type, and whether to append the argument signature (`Routine`) |
| `internal/snowflake` `DDLUnsupportedKinds` | `DDLUnsupported()` — the `GetObjectDDL` reject-list |
| `internal/objects` `BuildObjectPropertiesQuery` | `SHOW <Plural> LIKE … IN SCHEMA` for every schema-scoped kind |
| `frontend/src/generated/objectKinds.ts` | Generated: `OBJECT_KINDS`, `OBJECT_KIND_ORDER`, `OBJECT_KIND_LABEL`, `DDL_UNSUPPORTED_KINDS` |

## Adding an object kind

1. Add one entry to `Kinds` in `kinds.go`, positioned where the kind should appear in the tree.
2. `go generate ./internal/objectkind/` to refresh `frontend/src/generated/objectKinds.ts`.
3. Add an icon and a color variable for the kind in `frontend/src/components/sidebar/objectIcons.tsx` (and the `--icon-*` variable in `frontend/src/styles/global.css`, in both the light and dark blocks).

That is the whole list — the SHOW commands, the account-wide search, the Properties query, the GET_DDL mapping, the tree label and ordering, and the search type filter all follow. Tests fail if any step is skipped:

| Test | Catches |
|---|---|
| `objectkind.TestKindInvariants` / `TestBasicKinds` | A malformed entry (empty label, underscore in the name, a plural that isn't one, a mistakenly `Basic` kind) |
| `objectkind.TestGeneratedObjectKindsInSync` | A stale `objectKinds.ts` (step 2 skipped) |
| `snowflake.TestEveryKindIsSearchable` / `TestEveryKindIsListable` | A kind with no `SHOW` command |
| `snowflake.TestEveryKindHasDDLDisposition` | A kind that would emit an invalid `GET_DDL` instead of being rejected |
| `objects.TestEveryKindHasPropertiesQuery` | A kind whose Properties modal would error |
| frontend `objectIcons.test.ts` | A kind with no icon, no color, or a color variable missing from `global.css` |

## Gotchas

- **Order is display order, not command order.** The extended `SHOW` commands run concurrently and their results are regrouped per kind, so their order is immaterial; the registry order is chosen for the UI.
- **The registry is schema-scoped only.** `DATABASE`, `SCHEMA`, `WAREHOUSE`, `ROLE` and `USER` are deliberately absent — they are never objects in the tree, `SHOW OBJECTS` doesn't return them, and each needs a different scope clause. `ByName` returns `false` for them and their few consumers name them explicitly.
- **`GetDDLType` is not cosmetic.** An empty value means "reject before querying": `GET_DDL` fails server-side for these kinds and the gosnowflake driver logs every attempt as error noise. Some non-empty values are deliberately not the underscore form (`FILE FORMAT`, `GIT REPOSITORY`, `SEMANTIC VIEW`) — they are what `GET_DDL` actually accepts.
- **`Basic` is not "simple".** It means precisely "returned by `SHOW OBJECTS`". Marking a kind `Basic` by mistake removes its dedicated `SHOW` and the kind silently disappears from the tree.
