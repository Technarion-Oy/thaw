// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:domain: Core IPC & App Lifecycle

// Package updater performs notification-only application update checks: it
// fetches the latest GitHub release for Thaw, compares its tag against the
// running build using semantic versioning, and resolves the release page URL so
// the user can download the new version manually.
//
// It deliberately contains NO download-and-apply logic. Wails v2 has no updater
// or restart API, so full self-update is deferred to the Wails v3 migration (see
// issue #568). The HTTP call is proxy-aware (system/IE proxy on Windows &
// macOS, environment variables on Linux) with a direct-connection fallback so a
// stale or misconfigured proxy never hard-fails the check.
package updater
