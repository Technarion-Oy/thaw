# frontend/src/components/connection

> Snowflake connection dialog and GPL license modal shown at startup or on explicit disconnect.

## Responsibility

Handles the full connection flow: displaying the connect form, loading and managing Snowflake CLI profiles from `~/.snowflake/config.toml`, authenticating against Snowflake, and showing the GNU GPL v3 license notice on demand.

## Files

| File | Purpose |
|---|---|
| `ConnectModal.tsx` | Primary connection modal: Ant Design `Form` for account, credentials, and auth method; shows an **error** `Alert` when the `username_password_mfa` **or** `snowflake` (password) method is selected, warning that MFA can **lock the account** (Thaw's pooled connections each re-use a one-time MFA code/TOTP passcode and fail) unless an ACCOUNTADMIN sets `ALLOW_CLIENT_MFA_CACHING = TRUE`, and pointing to key-pair auth as a prompt-free alternative — issue #804; inline Snowflake CLI profile manager (New, Save, Rename, Clone, Set Default, Delete); calls `Connect`, `CancelConnect`, `LoadSnowflakeCLIConfig`, `SaveProfile`, `DeleteProfile`, `CloneProfile`, `RenameProfile`, `SetDefaultProfile`, `ClearDefaultProfile`, `GetSnowflakeCLIConfigPath`, `PickSnowflakeCLIConfigPath`, `PickPrivateKeyFile`. Tagged `@thaw-domain: Core IPC & App Lifecycle`. |
| `UserAgreementModal.tsx` | Read-only GPL v3 license/notice modal (`Modal` + `Typography`); no IPC calls; opened via the **License** link in `ConnectModal`'s footer. |

## Patterns & integration

- **IPC**: `ConnectModal` imports from `wailsjs/go/app/App` and calls `internal/app/profiles.go` delegators (profile CRUD) and `internal/sfconfig/writer.go` (TOML text-level mutations) via `internal/app`.
- **Stores**:
  - `useConnectionStore` — `setConnected(values)` is called on successful connection to propagate session context app-wide.
  - `useFeatureFlagsStore` — `flags.snowflakeCLIProfileManager` gates the entire profile manager section (dropdown, action buttons, divider). When disabled, the connect form is still shown but profile management is hidden.
- **Auth methods supported**: `username_password_mfa`, `externalbrowser`, `snowflake` (password + optional TOTP), `okta`, `snowflake_jwt` (key pair), `programmatic_access_token` (PAT — token or token file), `oauth` (token pass-through), `oauth_client_credentials`, `oauth_authorization_code` (both OAuth2 flows: client ID/secret/token-request URL, plus authorization URL + redirect URI for the auth-code flow, optional scope, single-use refresh tokens toggle), and `workload_identity` (AWS/Azure/GCP with provider-conditional Entra-resource and impersonation-path fields). Fields are shown/hidden reactively based on the selected authenticator; `needsPassword`/`needsUsername` helpers (and the `PASSWORDLESS_AUTH`/`USERLESS_AUTH` sets) gate the shared credential fields, and `AUTH_SPECIFIC_FIELDS` is reset on every authenticator switch. Sensitive fields (`token`, `oauthClientSecret`, `proxyPassword`) are excluded from `sessionStorage` persistence in `connectionStore.ts`.
- **Proxy section**: a collapsible `<Collapse>` panel (collapsed by default, independent of the selected authenticator) exposes forward-proxy fields — `proxyHost`, `proxyPort` (number), `proxyProtocol` (`http`/`https`, default `http`), `proxyUser`, `proxyPassword`, `noProxy` (comma-separated host list). These round-trip through CLI profiles (`applyCliConnection` auto-fills them; `buildConnectionFromForm` persists them) and map directly to the gosnowflake `sf.Config` proxy fields; `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` env vars remain a fallback when no explicit proxy is set.
- **Profile busy guard**: a `profileBusyRef` + `profileBusy` state pair prevents concurrent profile mutations from rapid button clicks; all profile actions are wrapped in `withProfileBusy`.
- **Duplicate name validation**: `nameModalHasDuplicate` checks the existing profile name set before enabling the "Create"/"Clone"/"Rename" button in the inner sub-modal.
- **Auto-select**: on initial load the default CLI profile (from `cfg.defaultConnection`) is automatically applied to the form.

## Gotchas

- `navigator.clipboard` is blocked in WKWebView; any clipboard needs in this folder must use `ClipboardSetText` from `wailsjs/runtime/runtime`. Currently `ConnectModal` has no clipboard operations, but child modals that display generated keys should use the native API.
- Profile names are validated against `/^[A-Za-z0-9_-]+$/`; names that violate this pattern display an inline error and block submission.
- `CancelConnect` must be called when the user clicks Cancel during an ongoing connection attempt (especially for `externalbrowser` which opens a browser window and blocks until the OAuth flow completes).
- The TOML file is written at the text level by `internal/sfconfig/writer.go` to preserve user comments and unknown keys; do not attempt to re-parse or rewrite it from the frontend.
- **Key-pair auth (`snowflake_jwt`)**: the private-key field has a **Browse** affordance (`FolderOpenOutlined` in the input's `addonAfter`) that calls `PickPrivateKeyFile`. On macOS this native selection is what grants Thaw scoped read access to the key — a hand-typed path pointing into a TCC-protected folder (`~/Documents`, `~/Desktop`, `~/Downloads`, iCloud Drive) fails to load with a permission error. The input stays editable for power users. The field's `extra` caveat is gated to macOS via `platformOS === "darwin"` (from `../files/platformUtil`) so Windows/Linux users don't see an irrelevant hint. See the macOS TCC gotcha in `docs/concepts/gotchas.md`.
