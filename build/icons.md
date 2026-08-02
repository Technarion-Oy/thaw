# Application Icons

`build/appicon.png` (currently **1024 × 1024 px**) is the single source of truth
for the application icon. Wails v2 derives every platform icon from it during
`wails build` — there is **no** `wails generate icons` command (that belongs to
the Wails v3 CLI). To change the icon, replace `appicon.png` and rebuild.

| Platform | What the build produces | Regenerated when? |
|---|---|---|
| macOS | `iconfile.icns` written into `build/bin/thaw.app/Contents/Resources/` | Every `wails build` — never committed, so it can't go stale |
| Windows | `build/windows/icon.ico`, compiled into the `.exe` via a `.syso` resource | Only if the `.ico` is **missing** — delete it after changing the artwork, or the old icon is reused |
| Linux | Nothing — Wails v2 does no Linux icon packaging | n/a |

Because the macOS `.icns` is generated straight into the bundle, this repository
intentionally contains no `build/darwin/iconfile.icns` or `build/linux/` icon.

## Credit

The source artwork (`build/appicon.png`) was generated with
[AppLaunchFlow](https://applaunchflow.com). The credit is surfaced to users in
**Help → About Thaw…** (an *App icon* row linking to the site) and in the intro
of the generated `THIRD_PARTY_NOTICES.md`, which the **Acknowledgements** viewer
renders. If the icon is ever replaced, update both places — the notices intro
lives in `scripts/gen_third_party_notices.go` (regenerate afterwards), not in
the Markdown file.
