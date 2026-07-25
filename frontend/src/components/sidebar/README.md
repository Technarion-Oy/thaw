# frontend/src/components/sidebar

> Snowflake object browser tree with context menus, inline modals, and a three-tier object-listing cache.

## Responsibility

The sidebar folder contains supporting files for the object browser. The primary implementation
is `Sidebar.tsx` which lives in `frontend/src/components/layout/` (it is bound to the layout and
imports helpers from this folder). This folder provides the icon registry used by the tree.

## Files

| File | Purpose |
|------|---------|
| `objectIcons.tsx` | Maps every Snowflake object kind to a distinct Ant Design Outlined icon (`KIND_ICON`) and a CSS variable colour (`KIND_VAR`, `--icon-*`). Exports `objectIcon(kind)`, `databaseIcon()`, `schemaIcon()`, `typeGroupIcon(kind)`, `columnIcon()`, plus both maps for the coverage test. Using `style={{ color: "var(--icon-x)" }}` instead of TwoTone icons lets the palette adapt to dark/light theme via CSS without recompiling TypeScript. |
| `objectIcons.test.ts` | Coverage guard: every kind in the generated registry (`src/generated/objectKinds.ts`) must have an icon **and** a colour variable, that variable must actually be declared in `global.css`, and no icon may map a kind the registry no longer has. |

## Patterns & integration

**Consumed by:** `Sidebar.tsx` (`frontend/src/components/layout/Sidebar.tsx`) which imports all
five icon factory functions:
```ts
import { objectIcon, databaseIcon, schemaIcon, typeGroupIcon, columnIcon } from "../sidebar/objectIcons";
```

**Colour tokens:** Icon colours are defined as CSS custom properties in `global.css`
(`--icon-table`, `--icon-view`, `--icon-function`, etc.). The icon module never hardcodes hex
values — all theming is delegated to CSS.

**Object kinds covered:** every kind in the canonical registry — `KIND_ICON` / `KIND_VAR` are the
one piece of object-kind metadata that is *not* generated from
[`internal/objectkind`](../../../../internal/objectkind/README.md) (icons are React components,
colours are theme-dependent CSS variables), so `objectIcons.test.ts` asserts they cover exactly the
kinds in `src/generated/objectKinds.ts`. An unknown kind falls back to a grey `FileOutlined`.

## Gotchas

- This folder contains only `objectIcons.tsx` and its test. All tree logic, node key formats, context
  menus, and IPC calls live in `frontend/src/components/layout/Sidebar.tsx`.
- **Adding an object kind starts in Go**, not here: one entry in `internal/objectkind/kinds.go`, then
  `go generate ./internal/objectkind/`, then the icon + colour in this folder (and the `--icon-*`
  variable in `global.css`, in both the light and dark blocks).
- **Node key format reference** (documented here for proximity to the icon module):
  - Databases: `db:NAME`
  - Schemas: `schema:DB:SCHEMA`
  - Object type groups: `type:DB:SCHEMA:KIND`
  - Objects: `obj:DB:SCHEMA:KIND:NAME`
  - Columns: `col:DB:SCHEMA:TABLE:COLUMN`
  - Stage dirs/files: `stagedir:DB:SCHEMA:NAME:path` / `stagefile:DB:SCHEMA:NAME:path`
  - Git dirs/files/refs: `gitdir:DB:SCHEMA:REPO:path` / `gitfile:...` / `gitbranches:` / `gittags:` / `gitcommits:`
  - DBT versions/dirs/files: `dbtversion:DB:SCHEMA:NAME:version` / `dbtdir:...` / `dbtfile:...`
- **Do not add logic** to this folder — it is intentionally a pure icon registry. Tree behaviour
  belongs in `layout/Sidebar.tsx`.
