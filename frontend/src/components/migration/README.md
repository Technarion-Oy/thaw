# frontend/src/components/migration

> Five-step schema migration wizard: scan local SQL files, diff against live Snowflake, review changes, protect databases, and deploy.

## Responsibility

Orchestrates an end-to-end schema migration workflow. The user maps local SQL source
directories to target Snowflake databases, scans for objects, compares local DDL against
the live schema, selects which changes to apply, configures table migration strategy and
optional safety snapshots, then executes the migration with real-time progress streaming.

## Files

| File | Purpose |
|------|---------|
| `MigrationModal.tsx` | Five-step modal (Configure → Scan Results → Review → Strategy & Protect → Deploy). Orchestrates all IPC calls and step transitions. |
| `ReviewGrid.tsx` | TanStack Table grid for Step 2: shows diff items (new/changed/unchanged) with checkboxes, status badges, and click-to-preview DDL. |
| `ExecGrid.tsx` | Real-time execution log grid for Step 4: renders streaming `MigrationExecEvent` rows with status icons and elapsed time. |
| `migrationUtils.ts` | Shared types (`MigrationObject`, `MigrationDiffItem`, `MigrationExecEvent`) and the `objectLabel(obj)` key helper. |

## Patterns & integration

**IPC calls:**
- `ListDatabases` — populates target DB selects on mount
- `PickDirectory` — native OS directory picker for source mappings
- `ScanMigrationSource(dir)` — scans a local directory for SQL objects; returns `MigrationObject[]`
- `AnalyzeMigration(objects, fallbackDB)` — diffs local DDL against live schema; returns `MigrationDiffItem[]`; emits `migration:analyze:progress` events
- `GenerateMigrationScript(items, db, strategy)` — generates a deployable SQL script; result is opened in a new editor tab via `queryStore.loadInNewTab`
- `CreateMigrationSnapshot(db, ...)` — creates backup set or zero-copy clone before deploying
- `ExecuteMigration(objects, db, concurrency, strategy)` — deploys selected objects; emits `migration:exec:progress` events
- `CancelMigration` — signals the backend to stop an in-progress deploy

**Event streaming:** Both analysis and execution phases subscribe to Wails events (`EventsOn`) and update progress state in real time. The `off()` cleanup function returned by `EventsOn` is called in `finally` blocks.

**Step 2 — dependency auto-selection:** When a row is checked, `extractReferencedNames` parses `FROM`/`JOIN` references from the local DDL and auto-selects any new/changed dependencies. Unchecking blocks if a currently-selected view/procedure depends on the table being unchecked.

**Step 3 — table strategies:** `in_place` (ADD/DROP/ALTER COLUMN), `blue_green_swap`, `view_abstraction`, `destructive_rebuild`. A destructive-rebuild warning alert is shown when that option is selected.

**Stores used:** `themeStore` (Monaco diff editor theme), `queryStore` (`loadInNewTab` for "Open in SQL Editor").

## Gotchas

- Multiple source directories are supported via the `SourceMapping[]` list. When "Select All" is clicked, objects from all mappings are merged into a single `Map` keyed by `objectLabel` to deduplicate.
- `AnalyzeMigration` and `ExecuteMigration` receive arrays cast via `as any` because Wails generates stricter TS types than the runtime accepts for complex nested types.
