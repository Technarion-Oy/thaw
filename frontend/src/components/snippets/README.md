# frontend/src/components/snippets

> Two-panel SQL snippet browser for Snowflake boilerplate templates.

## Responsibility

Presents a curated library of SQL snippet templates in a **cascading (collapsible)
left-panel menu**. Users expand a category to reveal its items, preview the formatted SQL
in the right panel, and open any snippet in a new SQL editor tab with one click.

The six DDL-template categories (Data Objects, Code, Automation, Storage, Governance,
Infrastructure) each hold a flat item list; a final **Built-in Functions** category nests
one level deeper into sub-categories (Context, Aggregate, Window, String, Date & Time,
Conversion & Cast, Conditional & NULL, Math, Semi-structured / JSON, Hash & Crypto,
System & Table, Geospatial). Function items are sourced from the editor's single-source catalogue —
`BUILTIN_FUNCTION_CATEGORIES` and the context `constants` in `../editor/snowflakeSql.ts`
(the same data that drives the Monarch `@builtins` tokenizer rule) — with the callable
form (`NAME()`) as the snippet body. No duplicate function list is maintained here.

## Files

| File | Purpose |
|------|---------|
| `SnippetsModal.tsx` | Two-panel snippet browser: a cascading AntD `Menu` (`mode="inline"`) built from a recursive `Category` tree (`items` = leaf group, `children` = nested sub-categories) plus SQL preview. Opens snippets in a new tab via `queryStore.loadInNewTab`. |

## Patterns & integration

**No IPC calls.** DDL snippet templates are defined statically in `SnippetsModal.tsx`; built-in-function items are derived from `BUILTIN_FUNCTION_CATEGORIES` + context `constants` in `../editor/snowflakeSql.ts` (no duplicate list).

**Cascading menu:** `buildMenu` walks the `Category` tree into AntD menu `items` and fills a `key → snippet` map (leaf keys are full slash-paths); `openKeys` is controlled so the first top-level category is expanded by default and every matching category auto-expands while searching.

**Execution:** "Open in New Tab" calls `loadInNewTab(sql)` from `queryStore`, which creates a new SQL tab pre-populated with the snippet text (not auto-executed).

**Search:** `filterTree` prunes the tree to items whose name matches the query, then the menu is rebuilt and fully expanded with the first hit auto-selected.

**Stores used:** `queryStore` (`loadInNewTab`).
