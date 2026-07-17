# frontend/src/components

> Feature UI components, grouped by domain. Each subfolder has its own `README.md`.

## Layout & shell
- [`layout/`](layout/README.md) — `AppLayout` (two-sidebar drag-and-drop) + `Sidebar` (object tree, context menus)
- [`sidebar/`](sidebar/README.md) — object-kind icon registry
- [`toolbar/`](toolbar/README.md) — unified `Toolbar` (run controls, session selectors, connection info)

## Editing & results
- [`editor/`](editor/README.md) — Monaco SQL editor, TabBar, snippets, cross-tab search
- [`results/`](results/README.md) — TanStack results grid, search, charts, EXPLAIN/profile
- [`notebook/`](notebook/README.md) — Snowpark notebook (per-cell editors, kernel, debugger)
- [`terminal/`](terminal/README.md) — embedded xterm.js shell

## Object management modals
- [`database/`](database/README.md) · [`account/`](account/README.md) · [`backup/`](backup/README.md) · [`pipe/`](pipe/README.md) · [`secret/`](secret/README.md) · [`task/`](task/README.md) · [`procedure/`](procedure/README.md) · [`function/`](function/README.md) · [`fnmeta/`](fnmeta/README.md)

## Git, dbt & integrations
- [`git/`](git/README.md) — local git panel · [`gitrepoobj/`](gitrepoobj/README.md) — Snowflake GIT REPOSITORY objects
- [`dbt/`](dbt/README.md) — local dbt scaffold · [`dbtproject/`](dbtproject/README.md) — Snowflake-native DBT PROJECT

## Tools & visualization
- [`er/`](er/README.md) — ER diagrams · [`lineage/`](lineage/README.md) — dependency tree · [`migration/`](migration/README.md) — schema migration wizard
- [`export/`](export/README.md) — DDL/data export+import · [`files/`](files/README.md) — file browser · [`snippets/`](snippets/README.md) — snippet browser · [`snowpark/`](snowpark/README.md) — Snowpark setup

## Connection, settings & shared
- [`connection/`](connection/README.md) — ConnectModal + profile management
- [`settings/`](settings/README.md) — feature flags, AI, layout, session tuning
- [`common/`](common/README.md) · [`shared/`](shared/README.md) — reusable modals & UI utilities
- [`help/`](help/README.md) — About + keyboard shortcuts
- [`setup/`](setup/README.md) — first-launch license agreement gate

See [`docs/concepts/architecture.md`](../../../docs/concepts/architecture.md) for how components connect to Zustand stores and the Go backend over Wails IPC.
