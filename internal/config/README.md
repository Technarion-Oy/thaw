# internal/config

> Persistent application configuration (JSON), feature flags with version migration, and IT-admin policy enforcement via platform-native mechanisms.

## Responsibility

- Define every top-level configuration struct that is serialised to `~/.config/thaw/config.json` (mode 0600).
- Provide `Load()` / `Save()` helpers for the JSON config file.
- Manage the `FeatureFlags` schema: versioned bool fields, `Initialized` sentinel, `DefaultFeatureFlags()`, and `MigrateFlags()` for forward migration.
- Read and apply IT-admin enforced policies from platform-specific sources (macOS managed plist, Windows Group Policy registry, Linux/other `features.json`).
- Validate and clamp `SessionConfig` values to safe ranges.
- Prevent frontend bypass of admin-locked flags via `RestoreAdminLockedFields`.

## Key files

| File | Purpose |
|------|---------|
| `config.go` | `AppConfig` and all nested config structs; `Load`, `Save`, `DefaultFeatureFlags`, `MigrateFlags` |
| `secretsync.go` | Keeps Thaw-owned secrets out of `config.json`: `buildDiskConfig` persists each secret to the OS secure store (`internal/secrets`) **without overwriting an existing store value** and returns a scrubbed copy for disk; `hasPlaintextSecret`/`blankSecrets` drive one-time migration on `Load()`. See below. |
| `adminconfig.go` | `adminConfigJSON` schema, `systemFeaturesPath`, `loadAdminJSON`, `LoadAdminConfig`, `mergeAdminOverrides` |
| `adminconfig_darwin.go` | macOS: reads managed plist via `plutil`, `applyPlatformOverrides` |
| `adminconfig_windows.go` | Windows: reads Group Policy registry (`HKLM`/`HKCU`), `applyPlatformOverrides` |
| `adminconfig_other.go` | Linux/other: no-op `applyPlatformOverrides`; admin config from JSON only |
| `restore.go` | `RestoreAdminLockedFields`, `ValidateSessionConfig` |
| `doc.go` | Package doc and `// thaw:domain: Core IPC & App Lifecycle` annotation |

## Key types & functions

```go
// config.go:186
type FeatureFlags struct {
    Initialized bool `json:"initialized"`
    Version     int  `json:"version"`
    // ... 30+ feature bool fields
}

// config.go:253
func DefaultFeatureFlags() FeatureFlags   // all flags true
func MigrateFlags(f FeatureFlags) FeatureFlags  // fills zero fields for new flags

// config.go:361
type AppConfig struct {
    Connections     []Connection
    Git             GitConfig
    AI              AIConfig
    Snowpark        SnowparkConfig
    PipRegistry     PipRegistryConfig
    Editor          EditorPrefs
    NotebookPrefs   NotebookPrefs
    Session         SessionConfig
    FeatureFlags    FeatureFlags
    LogPrefs        LogPrefs         // runtime log level + SQL-to-file logging switches
    UpdateCheck     UpdateCheckState // cached last update-check result (throttles the startup GitHub check)
    LicenseAccepted bool             // first-launch license gate: false (fresh install or key absent) → prompt to accept
    // ...
}

// config.go — cached update-check state (see internal/updater + internal/app/updater.go)
type UpdateCheckState struct {
    LastCheckUnix  int64  // Unix seconds of the last successful check; throttles the background check
    LatestVersion  string // latest release version seen (leading "v" stripped)
    ReleaseNotes   string // cached release body, so the notification shows without a re-fetch
    ReleasePageURL string // GitHub release page opened by "Download update"
}

// config.go — file-logging preferences (see internal/logger)
type LogPrefs struct {
    LogLevel               string // "debug"|"info"|"warn"|"error"; "" = build default
    IncludeQuerySQL        bool   // write executed SQL text to thaw.log (default false — SQL can be sensitive)
    IncludeInternalQueries bool   // also log internal/background queries (requires IncludeQuerySQL)
}
func DefaultLogPrefs() LogPrefs
func ValidLogLevel(name string) bool
func ValidateLogPrefs(p LogPrefs) LogPrefs
func RestoreAdminLockedLogPrefs(user, effective LogPrefs, locked LogPrefsLocked) LogPrefs

// config.go:386 / 416
func Load() (*AppConfig, error)
func Save(cfg *AppConfig) error
// Update runs a read-modify-write under a process mutex so concurrent config
// writes in this process can't lose each other's change; use it whenever the new
// value depends on the old (e.g. appending to a list). Save writes atomically
// (temp file + rename) so a second Thaw process never reads a half-written file.
func Update(fn func(*AppConfig) error) error

// adminconfig.go:180
func LoadAdminConfig(user FeatureFlags) (effective FeatureFlags, locked FeatureFlags)
// adminconfig.go — the "logging" features.json category enforces LogPrefs
// (log level + SQL-logging switches) so IT can force-disable SQL logging for
// privacy or force-enable it for audit.
func LoadAdminLogPrefs(user LogPrefs) (effective LogPrefs, locked LogPrefsLocked)

// restore.go:21
func RestoreAdminLockedFields(user, effective, locked FeatureFlags) FeatureFlags
func ValidateSessionConfig(sc SessionConfig) SessionConfig
```

## Patterns & integration

- `Load()` returns a zero-value `AppConfig` when the file does not exist (fresh install); callers apply `DefaultFeatureFlags()` / `MigrateFlags()` from `internal/app/config.go`.
- `FeatureFlags.Initialized` is a sentinel: a `false` value means the config file predates feature flags; `GetFeatureFlags()` in `internal/app/config.go` substitutes `DefaultFeatureFlags()` in that case.
- `flagsVersion` (currently 16) is bumped each time a new flag is added, and a corresponding `setIfZero` call is added to `MigrateFlags` so existing users get the new flag enabled by default.
- Admin enforcement: `LoadAdminConfig` chains `loadAdminJSON` → `applyPlatformOverrides` → `mergeAdminOverrides`. Platform files are selected at compile time via build tags (`//go:build darwin`, `//go:build windows`, `//go:build !darwin && !windows`).
- macOS platform override uses `plutil -convert json` (always available, no CGo) to parse plists.
- Windows platform override uses `golang.org/x/sys/windows/registry`; registry DWORD `1` = disabled.
- `RestoreAdminLockedFields` uses `reflect` to iterate `FeatureFlags` fields and overwrite any field where `locked.Field == true`, preventing frontend clients from submitting policy-bypassing flag saves.

## Secret handling

Thaw-owned secrets (AI API key, Git OAuth client secrets, pip credential/proxy passwords, MCP session tokens) are **never persisted to `config.json`** — the authoritative copy lives in the OS secure store (`internal/secrets`). The secret-bearing struct fields (`AI.APIKey`, `OAuth.GithubClientSecret`/`GitlabClientSecret`, `PipRegistry.ProxyPassword`, `PipRegistry.Credentials[].Password`, `MCPCredentials[].Token`) are in-memory transport only:

- `save()` writes what `buildDiskConfig` returns: each secret is persisted to the store and blanked on disk **only once it is safely in the store**. Crucially, the store is **never overwritten** from a value read out of `config.json` (it writes only when the store lacks the key), so a stale/synced/restored `config.json` can't clobber a newer secret already in the OS store. A secret that can't be stored is left on disk (`0600`) rather than lost.
- `Load()` migrates a legacy plaintext `config.json` once (`hasPlaintextSecret` → `save()` persists + scrubs), and `blankSecrets` ensures it never hands back a secret the store doesn't own. Empty fields trigger **zero** store access, so once migrated the hot load path never touches the keychain.
- **Authoritative updates** (the user changing a secret in Settings) go through the owning IPC seam via `secrets.Set`/`Delete`, which *do* overwrite — for those, `buildDiskConfig` is only the migration/first-write safety net, never the update path.
- **Exception — OAuth client secrets** (`OAuth.GithubClientSecret`/`GitlabClientSecret`) have no UI/app write seam, so `config.json` is their authoritative source. `buildDiskConfig` uses `storedFromDisk` (set-if-changed) for them so a hand-edited rotation is persisted to the store, not dropped by the anti-clobber guard.

Reads and clears of the actual values happen at each consumer's IPC seam (see `internal/secrets/README.md`), not here — this file only keeps the on-disk copy clean.

## Gotchas

- Config is written with `os.WriteFile(..., 0o600)` — never 0644. It no longer holds secrets (see **Secret handling**), but still contains connection profiles and other private settings.
- Secrets are scrubbed on every `save()`; do not add a new secret-bearing field without also handling it in `secretsync.go` (`buildDiskConfig`, `hasPlaintextSecret`, `blankSecrets`) and a `secrets` key.
- Do not edit `flagsVersion` without also adding a `setIfZero` block in `MigrateFlags`; forgetting this silently leaves new flags as `false` for existing users.
- After adding a new `FeatureFlags` field, run `wails generate module` to regenerate `frontend/wailsjs/go/models.ts`, then add a `<FlagRow>` in `FeatureFlagsModal.tsx`.
- The macOS plist priority order is highest-priority-last (reversed iteration); the managed pref at `/Library/Managed Preferences/` wins over the user pref at `~/Library/Preferences/`.
- `SessionConfig.MaxIdleConnsPerSession` is clamped to never exceed `MaxOpenConnsPerSession` by `ValidateSessionConfig` (`restore.go:55`).
- Admin `features.json` `"logging"`: forcing `"includeInternalQueries": true` automatically implies and locks `"includeQuerySQL": true` (via `mergeAdminLogPrefs`), so the audit policy works from a single key rather than silently no-opping. An explicit `"includeQuerySQL": false` alongside it is honored as-is (a contradictory config), and `ValidateLogPrefs` then normalizes internal logging off since it has no effect without SQL logging.
