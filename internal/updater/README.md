# internal/updater

> Notification-only application update check: fetch the latest GitHub release,
> compare it against the running build with semantic versioning, and resolve the
> release page URL for a manual download.

## Responsibility

Tells the user when a newer Thaw release is available. It deliberately contains
**no** download-and-apply/self-update logic — Wails v2 has no updater or restart
API, so full self-update is deferred to the Wails v3 migration (see issue #568).
The one job here is: fetch → compare → hand the frontend a `CheckResult`.

## Key files

| File | Purpose |
|------|---------|
| `updater.go` | `Check(ctx, currentVersion)` fetches `…/releases/latest`, parses the release JSON, and returns a `CheckResult`. `IsNewer(latest, current)` does the semver comparison via `golang.org/x/mod/semver` (both accept an optional leading `v`). |
| `proxy.go` | `doWithProxyFallback` — proxy-aware HTTP with fallback. Detects the OS proxy via `github.com/mattn/go-ieproxy` (WinHTTP/IE registry + PAC on Windows, `CFNetworkCopySystemProxySettings` on macOS, env vars on Linux) and, on failure, retries with a direct connection so a stale/misconfigured proxy never hard-fails the check. Logs which path succeeded. |
| `doc.go` | Package doc + `// thaw:domain: Core IPC & App Lifecycle`. |
| `updater_test.go` | `IsNewer` table (numeric vs lexical, dev/invalid versions, prereleases), `normalizeSemver`, and an httptest-backed transport exercise. |

## Key types & functions

| Symbol | Description |
|--------|-------------|
| `CheckResult` | `{Available, CurrentVersion, LatestVersion, ReleaseNotes, ReleasePageURL}` — JSON-tagged, returned to the frontend. |
| `Check(ctx, current) (CheckResult, error)` | Live check. Network/proxy/parse failures are returned as errors for the caller to log or surface. |
| `IsNewer(latest, current) bool` | Strict semver "greater than". Returns false for a `dev`/non-semver current version (local builds are never nagged) and for an unparseable latest tag. |

## Patterns & integration

- **Proxy behaviour** (issue #568 requirement): a GUI app launched from
  Finder/Dock/Start menu does **not** inherit shell env vars, so
  `HTTP_PROXY`/`HTTPS_PROXY` alone miss most corporate desktop users. `go-ieproxy`
  reads the real OS proxy settings; explicit env vars still take precedence. Both
  a proxied and a direct attempt are tried before giving up.
- **Rate limit**: unauthenticated GitHub API allows 60 req/hour. The background
  check is throttled (see `internal/app/updater.go`, `updateCheckInterval`) and
  the result is cached in `config.UpdateCheckState`, so repeated app launches
  stay well under the limit. The on-demand **Help → Check for Updates…** action
  bypasses the throttle and always hits the network.

Consumers:
- `internal/app/updater.go` — `CheckForUpdate` IPC + `startUpdateChecker`
  background goroutine; compares against `internal/version.Version`.
- `frontend/src/components/help/UpdateNotification.tsx` — banner + modal.

## Gotchas

- `golang.org/x/mod/semver` **requires** a leading `v`; `normalizeSemver` adds
  one. Snowflake/GitHub tags may or may not carry it, and `version.Version` does
  not — always route comparisons through `IsNewer`.
- `Check` never applies its own throttle. Throttling lives in the app layer so
  the on-demand menu path can opt out.
