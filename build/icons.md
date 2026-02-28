# Application Icons

Wails embeds platform-specific icons at build time. To generate all required
icon files from a single source image, place a **512 × 512 px PNG** named
`appicon.png` in this directory, then run:

```bash
wails generate icons
```

This produces:

| File | Used by |
|---|---|
| `build/darwin/iconfile.icns` | macOS `.app` bundle |
| `build/windows/icon.ico` | Windows `.exe` resource |
| `build/linux/icon.png` | Linux AppImage / desktop file |

The `darwin/` directory already contains an `iconfile.icns`. Re-run the
command only when the source artwork changes.
