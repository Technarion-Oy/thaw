# frontend/src/components/fnmeta

> Two-panel function catalog browser for Snowflake built-in functions and UDFs.

## Responsibility

Loads all function names once on mount and presents them in a searchable list on the left.
Selecting a function loads its full metadata (overloads, signatures, return type, description)
into a detail panel on the right. Users can insert the function name into the active SQL
editor or open the Snowflake docs page for built-in functions.

## Files

| File | Purpose |
|------|---------|
| `FunctionCatalogModal.tsx` | Full function catalog with list + detail two-panel layout, search, BUILTIN/UDF/ALL filter, overload display, and editor insertion. |

## Patterns & integration

**IPC calls:**
- `GetAllFunctionNames()` — fetches the complete list of function names on mount; result is filtered/searched client-side
- `GetFunctionTooltip(db, schema, name)` — fetches full metadata for the selected function (returns an array of overloads); called on each selection change

**Race prevention:** A `latestSelectRef` stores the most recently selected function name. The `GetFunctionTooltip` callback checks `latestSelectRef.current === name` before applying state, discarding stale responses from slower in-flight requests when the user navigates quickly.

**Editor insertion:** "Insert into Editor" calls `insertAtCursor(name + "(")` from `editorRef` to insert the function name followed by an opening parenthesis at the Monaco cursor position.

**Docs link:** For `BUILTIN` functions, a "Snowflake Docs" link opens `https://docs.snowflake.com/en/sql-reference/functions/${name.toLowerCase()}` via `BrowserOpenURL` (Wails native browser).

**Stores used:** None — all state is local to the modal.

## Gotchas

- `GetAllFunctionNames` is called unconditionally on mount regardless of whether any search is active, which can be slow on accounts with many UDFs. Results are not persisted between modal opens.
- The segmented filter (ALL / BUILTIN / UDF) operates on the already-loaded list client-side; no additional IPC calls are made when switching filters.
