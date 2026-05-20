// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// @thaw-domain: Core IPC & App Lifecycle

import { GetPlatformOS } from "../../../wailsjs/go/main/App";

// Module-level cache for platform OS (compile-time constant, fetched once).
let _platformOS: string | null = null;

/** Fetches the platform OS once and caches the result. */
export function getPlatformOS(): Promise<string> {
  if (_platformOS) return Promise.resolve(_platformOS);
  return GetPlatformOS()
    .then((os) => { _platformOS = os; return os; })
    .catch(() => "darwin");
}

// Eagerly fetch on module load so the cache is populated before components mount.
getPlatformOS();

/** Returns the cached value synchronously, or "darwin" if not yet fetched. */
export function getCachedPlatformOS(): string {
  return _platformOS ?? "darwin";
}

/** Platform-appropriate label for the "reveal in file manager" action. */
export function revealLabel(os: string): string {
  return os === "windows" ? "Show in Explorer" : os === "darwin" ? "Reveal in Finder" : "Show in File Manager";
}
