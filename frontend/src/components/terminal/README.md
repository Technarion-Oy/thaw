# frontend/src/components/terminal

> xterm.js terminal panel backed by a Go PTY shell, with theme synchronisation, shell selection, and ResizeObserver-driven fit.

## Responsibility

`TerminalPanel` embeds an xterm.js `Terminal` instance inside the app and connects it
to a native shell process managed by the Go backend (`internal/app` shell methods).
PTY output arrives as Base64-encoded Wails events; user keystrokes are sent via `WriteShell` IPC.
The terminal automatically resizes when its container changes dimensions.

## Files

| File | Purpose |
|---|---|
| `TerminalPanel.tsx` | Single component. On mount: calls `GetAvailableShells` to populate a shell selector, creates an xterm.js `Terminal` with `FitAddon`, opens it in the container div, calls `StartShell(shell, exportDir)` to launch the PTY, and registers `EventsOn("terminal:data")` (Base64 chunks decoded to `Uint8Array` and written to xterm) and `EventsOn("terminal:exit")` (writes `[Process exited]`). A `ResizeObserver` calls `fitAddon.fit()` and `ResizeShell(cols, rows)` whenever the container resizes. Toolbar: shell selector `Select`, New Session button (`StopShell` + `StartShell`), Kill button (`StopShell`), Close button. Theme updates (light/dark toggle) are applied without remounting via `termRef.current.options = { theme: ... }`. On unmount: event listeners cleaned up, `Terminal.dispose()` called, `StopShell()` called. |

## Patterns & integration

- **IPC**: `GetAvailableShells`, `StartShell`, `WriteShell`, `ResizeShell`, `StopShell` — all from `wailsjs/go/app/App`.
- **Events**: `terminal:data` (Base64-encoded PTY output from Go → xterm write), `terminal:exit` (PTY process ended).
- **Stores**: `themeStore` for `resolved` (dark/light) and `editorFont` / `editorFontSize` (applied to xterm `fontFamily` and `fontSize`); `gitStore` for `exportDir` (passed as the shell's starting directory to `StartShell`).
- **Base64 transport**: PTY output is Base64-encoded on the Go side before emission to survive the Wails IPC bridge without multi-byte character corruption. The frontend decodes with `atob` and writes a `Uint8Array` to xterm.
- **Theme**: separate dark and light xterm color palettes are defined inline; `resolved` drives which object is used. Theme is updated in a dedicated `useEffect` (not via remount) to avoid losing terminal history.

## Gotchas

- The mount `useEffect` has an empty dependency array — font and theme values are captured once at mount. Font size changes after mount require a full remount (closing and reopening the terminal panel). Theme changes are applied without remount via the options setter.
- `StopShell` is called on unmount; if the panel is closed and reopened, `StartShell` starts a fresh shell process. Any running process in the old shell is terminated.
- `exportDir` is only used at session start (`StartShell`). Changing the export directory while a shell session is active does not change the shell's working directory.
- The PTY exit event writes `[Process exited]` but does **not** automatically restart the shell. The user must click New Session or close and reopen the panel.
