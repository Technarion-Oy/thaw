# frontend/src/components/settings

> Application settings modals: feature flags, AI inline completions, session pool tuning, and layout customization.

## Responsibility

Provides the modal dialogs accessible from the View menu for configuring application behavior. Each modal is self-contained: it loads its own config on open, allows the user to make changes, and saves on confirm.

## Files

| File | Purpose |
|---|---|
| `FeatureFlagsModal.tsx` | "Enabled Features" modal; renders one `FlagRow` per flag grouped into `Category` sections (Data Export, Governance, AI, Advanced Tools, Developer Environments, Performance, Connection, Results Grid, SQL Editor, Schema Management, Integrations); calls `GetFeatureFlags`, `SaveFeatureFlags`, then reloads `featureFlagsStore`; shows a lock icon (`LockOutlined`) on flags controlled by IT admin policy (`locked` from `featureFlagsStore`). |
| `LoggingPreferencesModal.tsx` | "Logging Preferences" modal (View → Logging Preferences…); a `Select` for the runtime log level and two `Switch`es — write executed SQL to `thaw.log` (shows a warning `Alert` about sensitive data when on) and also log internal/background queries (enabled only when SQL logging is on). Shows an inline hint when SQL logging is on but the log level is Warning/Error, since successful queries are recorded at Info and would otherwise be silently dropped. A "Reveal Log File" footer button calls `RevealLogFile`. Calls `GetLogPrefs`, `GetLogPrefsLocked`, `UpdateLogPrefs`, then reloads `logPrefsStore`. Admin-locked fields render a `LockOutlined` icon + disabled control + tooltip, same as `FeatureFlagsModal`. Tagged `@thaw-domain: Core IPC & App Lifecycle`. |
| `MCPSessionsModal.tsx` | "MCP Sessions" modal (Tools → MCP Sessions); lists running MCP sessions from `mcpStore`, a Start form (label, execution mode, optional port), and per-session Stop and Copy Config actions; calls `StartMCPSession`, `StopMCPSession`, `GetMCPSessionConfig` and uses native `ClipboardSetText`; dispatches `thaw:mcp-changed` after start/stop. Renders `SecretStorageIndicator` under the security alert (the session token is stored in the OS secure store). Gated behind the `mcpServer` flag. Tagged `@thaw-domain: MCP Server`. |
| `AISettingsModal.tsx` | "Configure AI Inline Completions" modal; supports OpenAI, Google AI Studios, and Ollama (local); debounces `ListAIModels` to populate the model dropdown; runs `TestAIModel` on demand via a "Test connection" button (a provider/key/model change clears the prior result rather than re-testing automatically); detects system RAM via `GetSystemRAMGB` to recommend Ollama `num_ctx`; calls `GetAIConfig` and `SaveAIConfig`. Renders `SecretStorageIndicator` under the API-key field (the key is stored in the OS secure store, not `config.json`). Warns (via `featureFlagsStore`) when the enable toggle is on but the `aiInlineCompletions` feature flag gate is off, so the toggle isn't silently overridden. |
| `SecretStorageIndicator.tsx` | Small reusable hint showing where Thaw stores a secret on this OS, driven by `GetSecretStorageInfo` (never guessed from the platform). Renders a lock-icon "Stored in macOS Keychain / Windows Credential Manager / Secret Service" line, or a warning-styled note naming the `0600` fallback file when no OS secure store is available. Reused by `AISettingsModal`, `MCPSessionsModal`, and `snowpark/PipRegistryModal`. Tagged `@thaw-domain: Core IPC & App Lifecycle`. |
| `SessionManagementModal.tsx` | "Session Management" modal (View → Advanced); numeric inputs for max sessions, max open/idle connections per session; radio for lazy vs. eager init mode; idle timeout in minutes; calls `GetSessionConfig`, `SaveSessionConfig`, `GetDefaultSessionConfig`; dispatches `thaw:session-config-saved` DOM event after save so `internal/app/app.go` goroutines can re-read the config. Tagged `@thaw-domain: Core IPC & App Lifecycle`. |
| `LayoutSettingsModal.tsx` | "Customize Layout" modal; font pickers for UI and editor fonts (from `themeStore` constants `UI_FONTS`, `EDITOR_FONTS`); font size `Segmented`; UI density `Segmented` (compact/default/comfortable); three preset buttons (Modern, Classic, Comfortable); "Reset Layout" calls `usePanelLayoutStore`'s `reset`. No IPC calls — all state is Zustand-only. |

## Patterns & integration

- **IPC**: `FeatureFlagsModal` → `GetFeatureFlags`/`SaveFeatureFlags`; `AISettingsModal` → `GetAIConfig`, `SaveAIConfig`, `ListAIModels`, `TestAIModel`, `GetSystemRAMGB`; `SessionManagementModal` → `GetSessionConfig`, `SaveSessionConfig`, `GetDefaultSessionConfig`. All from `wailsjs/go/app/App`.
- **Stores**:
  - `featureFlagsStore` — `FeatureFlagsModal` reads `locked` (admin-enforced flags) and calls `load()` after save to propagate new flags app-wide without a page reload.
  - `logPrefsStore` — `LoggingPreferencesModal` calls `load()` after `UpdateLogPrefs` so the cached logging prefs (and admin-lock mask) stay current.
  - `themeStore` — `LayoutSettingsModal` reads and writes `uiFont`, `editorFont`, `editorFontSize`, `uiDensity` directly; changes apply instantly (Zustand subscribers re-render).
  - `panelLayoutStore` — `LayoutSettingsModal` calls `reset()` to restore default panel sizes.
- **Admin-locked flags**: `FeatureFlagsModal` reads `locked` from `featureFlagsStore` (populated by `GetAdminLockedFlags` IPC at startup); locked flags render a `LockOutlined` icon and a disabled `Switch` with a tooltip explaining admin control. Flags can be locked per-platform via managed plist (macOS), Group Policy (Windows), or `/etc/thaw/features.json` (Linux).
- **`FlagRow` and `Category`**: both are module-internal sub-components; `FlagRow` accepts a `preview` boolean that renders a styled "Preview" badge next to the label (currently used for Git Integration).

## Gotchas

- After `SaveFeatureFlags`, `FeatureFlagsModal` calls `loadStore()` (the Zustand `load` action) before closing. Skipping this step would leave stale flag values in all subscribers until the next app restart.
- `AISettingsModal` debounces the `ListAIModels` fetch and uses a `cancelled` flag to prevent a stale in-flight response from overwriting the model list when the user switches providers quickly. The connectivity test is manual (Test connection button), so it never races on rapid changes.
- `SessionManagementModal` dispatches `thaw:session-config-saved` as a DOM `Event` (not a Wails event) so that `QueryPage.tsx`'s `useEffect` can re-read `GetSessionInitMode` without coupling to the modal directly.
- `LayoutSettingsModal` uses no IPC calls and no "Save" button — changes write to Zustand immediately and are persisted by the store's own persistence layer (`localStorage`). The "Done" button only closes the modal.
- Adding a new feature flag requires changes in four places: `internal/config/config.go`, `wails generate module`, `FeatureFlagsModal.tsx` (add a `FlagRow`), and the component that gates on the flag. See CLAUDE.md for the full checklist.
