# frontend/src/components/sidebar

> Snowflake object browser tree with context menus, inline modals, and a three-tier object-listing cache.

## Responsibility

The sidebar folder contains supporting files for the object browser. The primary implementation
is `Sidebar.tsx` which lives in `frontend/src/components/layout/` (it is bound to the layout and
imports helpers from this folder). This folder provides the icon registry used by the tree.

## Files

| File | Purpose |
|------|---------|
| `objectIcons.tsx` | Maps every Snowflake object kind to a distinct Ant Design Outlined icon and a CSS variable colour (`--icon-*`). Exports `objectIcon(kind)`, `databaseIcon()`, `schemaIcon()`, `typeGroupIcon(kind)`, `columnIcon()`. Using `style={{ color: "var(--icon-x)" }}` instead of TwoTone icons lets the palette adapt to dark/light theme via CSS without recompiling TypeScript. |

## Patterns & integration

**Consumed by:** `Sidebar.tsx` (`frontend/src/components/layout/Sidebar.tsx`) which imports all
five icon factory functions:
```ts
import { objectIcon, databaseIcon, schemaIcon, typeGroupIcon, columnIcon } from "../sidebar/objectIcons";
```

**Colour tokens:** Icon colours are defined as CSS custom properties in `global.css`
(`--icon-table`, `--icon-view`, `--icon-function`, etc.). The icon module never hardcodes hex
values — all theming is delegated to CSS.

**Object kinds covered:** TABLE, VIEW, FUNCTION/PROCEDURE/EXTERNAL FUNCTION (function icon),
SEQUENCE, STAGE, PIPE, STREAM, TASK, FILE FORMAT, ALERT, POLICY, NETWORK RULE, BRANCH/TAG (git),
FILE, SECRET, DATA METRIC FUNCTION, INTEGRATION, DYNAMIC TABLE, MATERIALIZED VIEW, EVENT TABLE,
ICEBERG TABLE, APPLICATION, SNAPSHOT, EXTERNAL TABLE, DBT PROJECT, and fallback.

## Gotchas

- This folder contains only `objectIcons.tsx`. All tree logic, node key formats, context menus,
  and IPC calls live in `frontend/src/components/layout/Sidebar.tsx`.
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
