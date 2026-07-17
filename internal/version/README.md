# internal/version

> Application version string, overridable at build time via linker flags.

## Responsibility

Holds a single exported variable, `Version`, that defaults to `"dev"` and is replaced
with the real semver string during release builds by passing `-ldflags` to the Go linker.

## Key files

| File | Purpose |
|---|---|
| `version.go` | `var Version = "dev"` |

## Key types & functions

| Symbol | Description |
|---|---|
| `Version string` | The application version string. Defaults to `"dev"`. |

## Patterns & integration

Set the version at build time:

```bash
# Wails release build
wails build -ldflags "-X thaw/internal/version.Version=1.2.3"

# Plain go build
go build -ldflags "-X thaw/internal/version.Version=1.2.3" .
```

Consumers:
- `internal/app/run.go` — passes `version.Version` to `wails.Run` (window title / about box).
- `internal/crashreport` — embeds version in crash JSON via `crashreport.Init(version.Version)`.
- `internal/telemetry` — embeds version in every telemetry event via `telemetry.Init(version.Version)`.
- `internal/app/app.go` — returns `version.Version` as part of the `AppInfo` IPC response.
- `internal/updater` / `internal/app/updater.go` — compares `version.Version` against the latest GitHub release tag; the update check is **skipped entirely when `Version == "dev"`**.

## Gotchas

- The default `"dev"` value is intentional and visible in the UI for local development builds. CI/CD release pipelines must explicitly set this via `-ldflags`.
- The update checker keys off the exact `"dev"` string to suppress "update available" nags on local builds. `internal/updater.IsNewer` also treats any non-semver current version as never-older as a second line of defense.
