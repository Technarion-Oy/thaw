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
| `AboutModal.tsx` | Displays `app.AppInfo` (product name, version, build comments, company, copyright) plus a static **App icon** credit row linking to [AppLaunchFlow](https://applaunchflow.com), which generated `build/appicon.png`. The link uses `BrowserOpenURL` rather than an `<a href>` so WKWebView doesn't navigate the app window out of the embedded frontend. **Copy** puts the whole info block (including the icon credit) on the clipboard; **Acknowledgements** opens `ThirdPartyNoticesModal`. |
| `UpdateNotification.tsx` | Notification-only app-update UI. A floating, dismissible banner appears when the Go background check emits `update:available`; **Help → Check for Updates…** (`menu:check-for-update`) runs a live `CheckForUpdate()` and shows either an "up to date" toast or a modal. The modal renders the release notes and a **Download update** button (`BrowserOpenURL` to the GitHub release page). No in-app download/apply — see issue #568. |
| `ThirdPartyNoticesModal.tsx` | Searchable, collapsible list of every third-party package bundled into Thaw, parsed from the generated `THIRD_PARTY_NOTICES.md`. Groups by Backend (Go modules) / Frontend (npm packages); each entry shows name, version, license tag, and the full license text in an accordion panel. Accordion items are keyed by `name@version` because a few packages (immer, react-is, zustand) are bundled at more than one version. |
| `parseNotices.ts` | Pure parser that turns the generated Markdown into `{ intro, groups[] }`. Extracted from the modal so it's unit-testable in isolation. Fence detection is length-aware (matches the generator's variable-length fences), so a license text containing a bare ` ``` ` line can't truncate the block. |
| `parseNotices.test.ts` | Vitest coverage for `parseNotices`: a normal package, a missing-license prose fallback, the empty "Contents" group, a package at two versions, the longer-fence case, and intro paragraph joining. |

## Patterns & integration

**IPC calls:**
- `GetAppInfo()` — called on mount in `AboutModal`; returns `app.AppInfo` with `productName`, `productVersion`, `comments`, `companyName`, `copyright`
- `GetThirdPartyNotices()` — called on mount in `ThirdPartyNoticesModal`; returns the embedded `THIRD_PARTY_NOTICES.md` Markdown, which the component parses into structured package groups
- `ClipboardSetText(text)` — Wails native clipboard API used by the Copy button in `AboutModal` (required because WKWebView blocks `navigator.clipboard`)
- `BrowserOpenURL(url)` — Wails runtime; opens the app-icon credit link from `AboutModal` in the system browser

**Menu wiring:** `AboutModal` is opened by `App.tsx`, which listens for the
`menu:about` Wails event emitted by the native **Help → About Thaw…** menu item.
`UpdateNotification` is always mounted in `App.tsx`; it listens for the
`update:available` (background check) and `menu:check-for-update` (on-demand)
Wails events itself.

**IPC calls (`UpdateNotification`):**
- `CheckForUpdate()` — live update check; returns `updater.CheckResult`.
- `BrowserOpenURL(releasePageURL)` — Wails runtime; opens the GitHub release page.

**`KeyboardShortcutsModal`** has no IPC calls. Shortcut data is a hardcoded array of `{ action, mac, win }` objects grouped by category. Search filters across all three fields client-side. Shortcut tokens are rendered as `<kbd>` elements joined by `+`.

**`ThirdPartyNoticesModal`** parses the generated Markdown at runtime (it never
mutates it) via `parseNotices`. `THIRD_PARTY_NOTICES.md` is produced by
`scripts/gen_third_party_notices.go` — regenerate it after changing
dependencies; the parser only relies on the `##`/`###`/bullet/fence shapes that
generator emits. The Go section is the union across every shipped platform, so
the modal lists a few packages (the macOS/Windows/Linux credential-store
backends, the Windows webview) that are not in the binary the viewer is running
— the file's intro prose says so, and no per-package platform filtering is
applied here. The intro paragraphs shown above the search box come from that
file's prose (including the AppLaunchFlow app-icon credit) and are rendered as
plain text, so the generator writes bare URLs there rather than Markdown links. `TestThirdPartyNoticesUpToDate` (root Go package, run in
`build-check.yml`) fails CI if the committed file drifts from the dependency
tree.

**Stores used:** None.
