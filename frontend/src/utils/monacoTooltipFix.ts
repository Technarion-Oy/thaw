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
//
// The hover service is a shared singleton, so patching it once covers every
// editor for the rest of the session — but it must run AFTER an editor exists
// (the service is instantiated in the StandaloneCodeEditor constructor). Called
// from the onDidCreateEditor hook below, so every call is post-creation.
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

interface EditorNamespace {
  onDidCreateEditor: (listener: () => void) => void;
}

let hookRegistered = false;

/**
 * Register a global `onDidCreateEditor` hook that applies the find-widget tooltip
 * fix to every editor Monaco creates (issue #593). Idempotent; call once with the
 * `monaco.editor` namespace (from `ensureMonacoSetup`, which runs before the first
 * editor is created). Because the hook fires on every creation and the patch
 * targets a shared singleton, this covers all editors — SqlEditor, notebook
 * cells, modals, diff views — independent of any per-mount clipboard wiring.
 */
export function registerFindWidgetTooltipFix(editorNs: EditorNamespace): void {
  if (hookRegistered) return;
  hookRegistered = true;
  editorNs.onDidCreateEditor(() => applyHoverPatch());
}
