# frontend/src/components/help

> Static keyboard shortcut reference and application About dialog.

## Responsibility

Provides two informational modals: a searchable keyboard shortcut reference grouped by
feature area, and an About dialog that displays version and build metadata.

## Files

| File | Purpose |
|------|---------|
| `KeyboardShortcutsModal.tsx` | Searchable shortcut reference table. Six groups: Tabs & Navigation, Query Execution, Editor, UI & Panels, Results Grid, Notebook. No IPC — all data is static. |
| `AboutModal.tsx` | Displays `app.AppInfo` (product name, version, build comments, company, copyright). Copy button puts the version string on the clipboard. |

## Patterns & integration

**IPC calls:**
- `GetAppInfo()` — called on mount in `AboutModal`; returns `app.AppInfo` with `productName`, `productVersion`, `comments`, `companyName`, `copyright`
- `ClipboardSetText(text)` — Wails native clipboard API used by the Copy button in `AboutModal` (required because WKWebView blocks `navigator.clipboard`)

**`KeyboardShortcutsModal`** has no IPC calls. Shortcut data is a hardcoded array of `{ action, mac, win }` objects grouped by category. Search filters across all three fields client-side. Shortcut tokens are rendered as `<kbd>` elements joined by `+`.

**Stores used:** None.
