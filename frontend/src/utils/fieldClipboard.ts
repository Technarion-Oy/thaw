// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

/**
 * True if `el` is Monaco's own code-editing surface (the hidden `.inputarea`
 * textarea) rather than an ordinary editable field. Find/replace and rename
 * inputs also live inside `.monaco-editor` but are plain fields, so this is the
 * single source of truth — shared by `App.tsx`'s global clipboard handler and
 * `monacoClipboard.ts` — for "leave this to the editor model" vs. "splice it
 * like any native field".
 */
export function isMonacoCodeSurface(el: Element | null): boolean {
  return !!el && el.classList.contains("inputarea") && !!el.closest(".monaco-editor");
}
