# frontend/src/components/help

> Static keyboard shortcut reference, the application About dialog, and the
> third-party license acknowledgements viewer.

## Responsibility

Provides the Help-menu informational modals: a searchable keyboard shortcut
reference grouped by feature area, an About dialog that displays version and
build metadata, and an acknowledgements viewer that lists every bundled
open-source package with its license text.

## Files

| File | Purpose |
|------|---------|
| `KeyboardShortcutsModal.tsx` | Searchable shortcut reference table. Six groups: Tabs & Navigation, Query Execution, Editor, UI & Panels, Results Grid, Notebook. No IPC — all data is static. |
| `AboutModal.tsx` | Displays `app.AppInfo` (product name, version, build comments, company, copyright). **Copy** puts the version string on the clipboard; **Acknowledgements** opens `ThirdPartyNoticesModal`. |
| `ThirdPartyNoticesModal.tsx` | Searchable, collapsible list of every third-party package bundled into Thaw, parsed from the generated `THIRD_PARTY_NOTICES.md`. Groups by Backend (Go modules) / Frontend (npm packages); each entry shows name, version, license tag, and the full license text in an accordion panel. |

## Patterns & integration

**IPC calls:**
- `GetAppInfo()` — called on mount in `AboutModal`; returns `app.AppInfo` with `productName`, `productVersion`, `comments`, `companyName`, `copyright`
- `GetThirdPartyNotices()` — called on mount in `ThirdPartyNoticesModal`; returns the embedded `THIRD_PARTY_NOTICES.md` Markdown, which the component parses into structured package groups
- `ClipboardSetText(text)` — Wails native clipboard API used by the Copy button in `AboutModal` (required because WKWebView blocks `navigator.clipboard`)

**Menu wiring:** `AboutModal` is opened by `App.tsx`, which listens for the
`menu:about` Wails event emitted by the native **Help → About Thaw…** menu item.

**`KeyboardShortcutsModal`** has no IPC calls. Shortcut data is a hardcoded array of `{ action, mac, win }` objects grouped by category. Search filters across all three fields client-side. Shortcut tokens are rendered as `<kbd>` elements joined by `+`.

**`ThirdPartyNoticesModal`** parses the generated Markdown at runtime (it never
mutates it). `THIRD_PARTY_NOTICES.md` is produced by
`scripts/gen_third_party_notices.go` — regenerate it after changing
dependencies; the parser only relies on the `##`/`###`/bullet/fence shapes that
generator emits.

**Stores used:** None.
