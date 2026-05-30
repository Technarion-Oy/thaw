# internal/sfconfig

> Reader and text-level writer for the Snowflake CLI configuration file (`~/.snowflake/config.toml`).

## Responsibility

Reads and writes connection profiles in the Snowflake CLI format. The reader parses TOML
and exposes typed `Connection` structs. The writer operates at the raw text level
(line-by-line splice) to preserve user comments, blank lines, and TOML keys that Thaw
does not model. Writes are atomic (write to temp file, then rename) with `0600`
permissions to protect credentials.

## Key files

| File | Purpose |
|---|---|
| `reader.go` | `Load(path)` — TOML decode via `BurntSushi/toml`, returns `*Config` with sorted `[]Connection`. Defaults to `~/.snowflake/config.toml` when `path` is `""`. |
| `writer.go` | `SaveProfile`, `DeleteProfile`, `CloneProfile`, `RenameProfile`, `SetDefaultProfile`, `ValidateProfileName`, and all text-level splice helpers. |

## Key types & functions

| Symbol | Description |
|---|---|
| `Connection` | A single named profile: `account`, `user`, `password`, `role`, `warehouse`, `database`, `schema`, `authenticator`, `passcode`, `oktaUrl`, `privateKeyPath`, `privateKeyPassphrase`. |
| `Config` | `{ defaultConnection string; connections []Connection }` sorted alphabetically. |
| `Load(path)` | Returns empty `Config` (not error) if the file does not exist. |
| `ValidateProfileName(name)` | Rejects empty names and names with characters outside `[A-Za-z0-9_-]`. |
| `SaveProfile(path, conn)` | Upserts a `[connections.<name>]` section, trimming trailing blank lines before the next section. |
| `DeleteProfile(path, name)` | Removes the section; clears `default_connection_name` if it matches. |
| `CloneProfile(path, src, dst)` | Copies the section body under a new name. |
| `RenameProfile(path, old, new)` | Renames the section header and updates `default_connection_name` if needed. |
| `SetDefaultProfile(path, name)` | Updates or inserts the `default_connection_name` key. |

## Patterns & integration

- IPC methods in `internal/app/profiles.go` are thin delegators that call `sfconfig.Load` / `sfconfig.Save*` / etc.
- The `ConnectModal.tsx` frontend uses the loaded profiles to pre-fill connection fields; the profile management UI (New / Save / Rename / Clone / Set Default / Delete) calls the corresponding IPC methods.
- All writer functions are gated by `ValidateProfileName` and read-parse-modify-write the file on every call — no in-memory state is maintained.

## Gotchas

- Profile name validation rejects names outside `[A-Za-z0-9_-]`. Snowflake CLI also accepts names with other characters, but Thaw enforces a stricter subset for safety.
- The writer does **not** use a TOML library for output — it splices raw text to preserve comments and unknown keys. This means the output format is not re-indented or normalised.
- `atomicWriteFile` writes to a temp file in the same directory then renames. On Windows, the rename can fail if the target file is locked by another process.
- Deleting the current default profile sets `default_connection_name = ""` (empty string), not removing the key entirely, to keep the file valid TOML.
