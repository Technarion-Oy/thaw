// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore – internal Monaco paths; no public type declarations
import { MenuId } from "monaco-editor/esm/vs/platform/actions/common/actions.js";

/**
 * Get an existing Monaco `MenuId` by string key, or create it. Monaco's
 * `MenuId` constructor throws when the key already exists, so we fall back to
 * its (unexported, private) `_instances` registry. Centralized here so a Monaco
 * bump that changes this internal only needs fixing in one place rather than at
 * every context-menu registration site. Returns `undefined` if neither works.
 */
export function getOrCreateMenuId(key: string): unknown {
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return new (MenuId as any)(key);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return (MenuId as any)._instances?.get(key);
  }
}
