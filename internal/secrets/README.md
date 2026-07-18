# internal/secrets

> OS-native credential store for Thaw-owned secrets, keeping them out of `config.json`.

## Responsibility

- Store Thaw's own secrets (AI API key, Git OAuth client secrets, pip registry credential/proxy passwords, MCP session tokens) in the platform secure store instead of `~/.config/thaw/config.json`:
  - **macOS** → Keychain
  - **Windows** → Credential Manager
  - **Linux** → Secret Service / libsecret (when a session bus & collection are available)
  - **Fallback** → a plaintext `~/.config/thaw/secrets.json` (mode `0600`) when no OS store is available (headless Linux, unsupported platforms). This preserves the app's prior on-disk behavior while still keeping secrets out of `config.json`.
- Report the active backend so the Settings UI can show where a secret actually lives (`App.GetSecretStorageInfo` → `secrets.Info`).

This package owns **only Thaw-created secrets**. It does **not** touch `~/.snowflake/config.toml` (shared with the Snowflake CLI — see `internal/sfconfig`).

## Key files

| File | Purpose |
|------|---------|
| `secrets.go` | `Store` interface, `Method`/`Info` types, `ErrNotFound`, the process-wide default store, and package-level `Get`/`Set`/`Delete`/`Keys`/`GetInfo`. `SetDefaultForTesting` swaps the store in tests. |
| `keys.go` | Stable store keys: `KeyAIAPIKey`, `KeyGitHubClientSecret`, `KeyGitLabClientSecret`, `KeyPipProxyPassword`, plus `PipCredentialKey(registry)` / `MCPTokenKey(label)` for the collection-valued secrets. |
| `store_os.go` | `osStore`: wraps `github.com/99designs/keyring`, restricted to the single OS-native backend for the current platform. `newOSStore` returns an error when no such store is available, signalling fallback. |
| `store_file.go` | `fileStore`: the `0600` JSON fallback at `~/.config/thaw/secrets.json` (override dir with `THAW_SECRETS_DIR`). |

## Backend selection

`buildStore()` (lazy, cached) prefers the OS-native store and falls back to the file store when it can't be opened. `THAW_SECRETS_BACKEND=file` forces the fallback — used by tests and headless deployments that must not touch a real keychain. Construction is deferred until the first secret access, so config Load/Save paths that touch no secrets never initialize (or prompt) an OS store.

## How secrets flow (the seams)

`internal/config` guarantees the on-disk `config.json` never holds a secret (see `internal/config/secretsync.go`):

- **`save()`** writes what `buildDiskConfig` returns: each secret is stored here and blanked on disk only once it is safely in the store. The store is **never overwritten** from a value read out of `config.json` (it writes only when this store lacks the key), so a stale/synced `config.json` can't clobber a newer secret already held here.
- **`Load()`** runs a one-time **migration**: plaintext secrets left in an older `config.json` are moved into this store, then the scrubbed file is written back, and the config struct never hands a secret value back to callers. After migration the fields are empty on disk, so the hot load path performs **zero** secure-store access.
- **Authoritative updates** (the user changing a secret in Settings) call `Set`/`Delete` directly at the write seam below — those *do* overwrite; `buildDiskConfig` is only the migration/first-write safety net.

The actual secret **values** are read/written at each consumer's IPC seam, not in `config`:

| Secret | Read seam | Write seam |
|--------|-----------|------------|
| AI API key | `app.GetAIConfig`, `app.GetAISuggestion` | `app.SaveAIConfig` (`storeOrDelete`) |
| GitHub / GitLab OAuth client secret | `gitrepo.GetProviderConfig` | migration only (no UI setter today) |
| pip credential / proxy password | `snowpark.GetPipRegistryConfig`, `buildPipRegistrySetup` (`hydratePipSecrets`) | `snowpark.SavePipRegistryConfig`/`ResetPipRegistryConfig` (`storePipSecrets`/`deletePipSecrets`) |
| MCP session token | `app.StartMCPSession` | `app.saveMCPCredential` |

Setters delete (rather than write empty) when a secret is cleared, and every consumer tolerates `ErrNotFound` — so a secret deleted out-of-band from the OS store (e.g. via Keychain Access / Credential Manager) simply reads as absent and the UI re-prompts.

## Gotchas

- **Never change the key strings** in `keys.go` without a migration — they are the identity of a secret in the OS store.
- Keychain/Credential-Manager `Keys()` enumerates only Thaw's service scope; the pip pruning path (`storePipSecrets`) relies on this to delete secrets for removed registries.
- The file fallback is plaintext by design (matches the app's previous behavior). The UI surfaces a warning-styled indicator when it is active.
