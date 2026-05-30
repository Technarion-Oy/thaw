# frontend/src/components/snippets

> Two-panel SQL snippet browser for Snowflake boilerplate templates.

## Responsibility

Presents a curated library of SQL snippet templates organised into six categories. Users
browse categories and items in the left panel, preview the formatted SQL in the right panel,
and open any snippet in a new SQL editor tab with one click.

## Files

| File | Purpose |
|------|---------|
| `SnippetsModal.tsx` | Two-panel snippet browser with searchable category/item list and SQL preview. Opens snippets in a new tab via `queryStore.loadInNewTab`. |

## Patterns & integration

**No IPC calls.** All snippet data is defined statically in `snowflakeSnippets.ts` (under `frontend/src/components/editor/`). The six categories (Data Objects, Code, Automation, Storage, Governance, Infrastructure) mirror `SNIPPET_CATEGORIES` used by the Monaco context-menu snippet submenu.

**Execution:** "Open in New Tab" calls `loadInNewTab(sql)` from `queryStore`, which creates a new SQL tab pre-populated with the snippet text (not auto-executed).

**Search:** Filters across category name and item label/description client-side.

**Stores used:** `queryStore` (`loadInNewTab`).
