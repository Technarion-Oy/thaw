# Application Icons

`build/appicon.png` (currently **1024 × 1024 px**) is the single source of truth
for the application icon. `build/Taskfile.yml`'s `generate:icons` task runs
`wails3 generate icons` to derive the macOS and Windows icon files from it. To
change the icon, replace `appicon.png` and rebuild.

| Platform | What the build produces | Regenerated when? |
|---|---|---|
| macOS | `build/darwin/icons.icns`, copied into `build/bin/thaw.app/Contents/Resources/` | Automatically by `common:generate:icons` whenever `appicon.png` changes (git-ignored) |
| Windows | `build/windows/icon.ico`, compiled into `thaw.exe` via `wails3 generate syso` | Automatically by `common:generate:icons` whenever `appicon.png` changes (git-ignored) |
| Linux | Nothing — Thaw ships a standalone Linux binary | n/a |

Neither `build/darwin/icons.icns` nor `build/windows/icon.ico` is committed to git.

## Credit

The source artwork (`build/appicon.png`) was generated with
[AppLaunchFlow](https://applaunchflow.com). The credit is surfaced to users in
**Help → About Thaw…** (an *App icon* row linking to the site) and in the intro
of the generated `THIRD_PARTY_NOTICES.md`, which the **Acknowledgements** viewer
renders. If the icon is ever replaced, update both places — the notices intro
lives in `scripts/gen_third_party_notices.go` (regenerate afterwards), not in
the Markdown file.
