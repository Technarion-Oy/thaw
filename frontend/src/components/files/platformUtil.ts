// SPDX-License-Identifier: GPL-3.0-or-later

import { GetPlatformOS } from "../../../wailsjs/go/app/App";

// Module-level cache for platform OS (compile-time constant, fetched once).
let _platformOS: string | null = null;

/** Fetches the platform OS once and caches the result.
 *  On failure, returns "unknown" WITHOUT caching — so the next call retries. */
export function getPlatformOS(): Promise<string> {
  if (_platformOS) return Promise.resolve(_platformOS);
  return GetPlatformOS()
    .then((os) => { _platformOS = os; return os; })
    .catch(() => "unknown"); // intentionally not cached: _platformOS stays null
}

// Eagerly fetch on module load so the cache is populated before components mount.
// If Wails runtime isn't ready yet, the catch fallback applies and components
// will re-fetch via their useEffect hooks once mounted.
getPlatformOS();

/** Returns the cached value synchronously, or null if not yet fetched. */
export function getCachedPlatformOS(): string | null {
  return _platformOS;
}

/** Platform-appropriate label for the "reveal in file manager" action. */
export function revealLabel(os: string | null): string {
  if (os === "windows") return "Show in Explorer";
  if (os === "darwin") return "Reveal in Finder";
  return "Show in File Manager";
}
