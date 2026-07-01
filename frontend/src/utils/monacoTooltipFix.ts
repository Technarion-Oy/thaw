// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// Deep internals — needed to patch Monaco's base-layer hover service.
// @ts-expect-error no type declarations for this deep Monaco path
import { StandaloneServices } from "monaco-editor/esm/vs/editor/standalone/browser/standaloneServices.js";
// @ts-expect-error no type declarations for this deep Monaco path
import { IHoverService } from "monaco-editor/esm/vs/platform/hover/browser/hover.js";
// @ts-expect-error no type declarations for this deep Monaco path
import { ICodeEditorService } from "monaco-editor/esm/vs/editor/browser/services/codeEditorService.js";

let patched = false;

// Force Monaco's base-layer hover tooltips (find-widget button tooltips — the
// Aa/ab/.* toggles, prev/next, close) to render BELOW their target instead of
// the default ABOVE. The find widget is pinned to the editor's top edge, so
// "above" lands in the tab-bar band where the editor pane's `overflow: hidden`
// clips it away (issue #593). `_createHover` is the single choke point both
// showInstantHover and showDelayedHover funnel through; forcing BELOW there
// (only when the caller set no explicit position) drops these tooltips into the
// editor body, unclipped. Monaco still auto-flips back to ABOVE if BELOW would
// overflow the window, so this stays correct for targets that aren't near the
// top. The code hover uses a content widget (not this service), so it's
// unaffected.
function applyHoverPatch(): void {
  if (patched) return;
  const HOVER_POSITION_BELOW = 2; // HoverPosition.BELOW
  const hoverService = StandaloneServices.get(IHoverService) as {
    _createHover?: (options: { position?: { hoverPosition?: unknown } }, skip?: unknown) => unknown;
  } | undefined;
  if (!hoverService || typeof hoverService._createHover !== "function") {
    // Monaco internals changed (e.g. a version bump renamed _createHover). Warn
    // once and stop — don't silently regress to clipped tooltips while retrying
    // on every editor creation for the rest of the session.
    patched = true;
    console.warn(
      "[thaw] find-widget tooltip fix: Monaco hover service `_createHover` unavailable; " +
      "tooltips may render clipped (issue #593).",
    );
    return;
  }
  const original = hoverService._createHover.bind(hoverService);
  hoverService._createHover = (options, skip) => {
    // Mutate the caller's `options` in place — do NOT replace it with a spread
    // copy. The real `_createHover` also writes `options.container` on this same
    // reference (for aux-window support), and the caller reads it back after; a
    // spread copy would swallow that write.
    if (options && options.position?.hoverPosition === undefined) {
      options.position = { ...options.position, hoverPosition: HOVER_POSITION_BELOW };
    }
    return original(options, skip);
  };
  patched = true;
}

let hookRegistered = false;

/**
 * Ensure find-widget button tooltips render below (issue #593). Idempotent, and
 * safe to call from any POST-CREATE Monaco mount hook (never `beforeMount` — the
 * hover-service singleton is only live once an editor has been constructed):
 *
 *  - patches the hover-service singleton immediately for the editor mounting now
 *    (so a modal that is the first Monaco surface of a session is covered without
 *    depending on some other mount having run first), and
 *  - registers a one-time global `onCodeEditorAdd` hook so every future editor
 *    creation re-applies the patch — covering any surface that never calls this.
 */
export function installFindWidgetTooltipFix(): void {
  applyHoverPatch();
  if (hookRegistered) return;
  hookRegistered = true;
  try {
    const codeEditorService = StandaloneServices.get(ICodeEditorService) as {
      onCodeEditorAdd: (listener: () => void) => unknown;
    };
    codeEditorService.onCodeEditorAdd(() => applyHoverPatch());
  } catch {
    /* best-effort: the per-mount applyHoverPatch calls still cover known surfaces */
  }
}
