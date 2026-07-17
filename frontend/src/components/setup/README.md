# components/setup

> First-launch setup gates shown before the main workspace.

## Components

| File | Purpose |
|------|---------|
| `LicenseAgreement.tsx` | Non-dismissible, full-screen license gate shown on first launch. Fetches the license text via `GetLicenseText`, and offers **Accept** (persists the choice via `AcceptLicense`, then calls `onAccept` to reveal the workspace) or **Decline & Quit** (`DeclineLicense` quits the app). No close affordance — `closable`/`keyboard`/`maskClosable` are all off — so the agreement cannot be bypassed. |

## Integration

- Mounted from `App.tsx`. On startup `IsLicenseAccepted()` is called: `null` while checking, `false` renders `<LicenseAgreement>` and withholds `<AppLayout>` (and the connect modal), `true` reveals the workspace.
- Backend: `internal/app/license.go` (`GetLicenseText`, `IsLicenseAccepted`, `AcceptLicense`, `DeclineLicense`); persisted flag `config.AppConfig.LicenseAccepted`; the license text is the repo-root `LICENSE` embedded in `main.go` and threaded through `Run` → `NewApp`.
- Because the persisted flag defaults to `false`, both fresh installs and existing installs upgraded to this version are prompted once; accepting clears the gate on subsequent launches.
