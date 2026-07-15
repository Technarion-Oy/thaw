// SPDX-License-Identifier: GPL-3.0-or-later

// Deep internals — needed to patch Monaco's base-layer hover service.
// @ts-expect-error no type declarations for this deep Monaco path
import { StandaloneServices } from "monaco-editor/esm/vs/editor/standalone/browser/standaloneServices.js";
// @ts-expect-error no type declarations for this deep Monaco path
import { IHoverService } from "monaco-editor/esm/vs/platform/hover/browser/hover.js";

let patched = false;

interface HoverOptions {
  position?: { hoverPosition?: unknown };
  target?: HTMLElement | { targetElements?: HTMLElement[] };
}

// The find widget is pinned to the editor's top edge, and Monaco's base-layer
// hover tooltips default to rendering ABOVE their target — so its button
// tooltips (the Aa/ab/.* toggles, prev/next, close) land in the tab-bar band
// where the editor pane's `overflow: hidden` clips them away (issue #593).
// Whether the tooltip's target lives inside the find widget.
function isFindWidgetHover(options: HoverOptions): boolean {
  const t = options.target;
  const el = t instanceof HTMLElement ? t : t?.targetElements?.[0];
  return !!el && !!el.closest(".find-widget");
}

// Patch Monaco's base-layer hover service so FIND-WIDGET tooltips render BELOW.
// `_createHover` is the single choke point both showInstantHover and
// showDelayedHover funnel through — but it backs every base-layer tooltip in the
// app (toolbar buttons, rename widget, etc.), so we flip to BELOW only for
// hovers whose target is inside the find widget, leaving all others at Monaco's
// default. Monaco still auto-flips back to ABOVE if BELOW would overflow the
// window. The code hover uses a content widget (not this service), so it's
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
    _createHover?: (options: HoverOptions, skip?: unknown) => unknown;
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
    if (options && options.position?.hoverPosition === undefined && isFindWidgetHover(options)) {
      options.position = { ...options.position, hoverPosition: HOVER_POSITION_BELOW };
    }
    return original(options, skip);
  };
  patched = true;
}

interface EditorNamespace {
  onDidCreateEditor: (listener: () => void) => void;
}

/**
 * Register a global `onDidCreateEditor` hook that applies the find-widget tooltip
 * fix to every editor Monaco creates (issue #593). Call once with the
 * `monaco.editor` namespace from `ensureMonacoSetup` (its module-level `registered`
 * guard already makes a second call structurally impossible). The hook fires on
 * every editor creation and the underlying patch (`patched`) is a one-time
 * singleton mutation, so this covers all editors — SqlEditor, notebook cells,
 * modals, diff views — independent of any per-mount clipboard wiring.
 */
export function registerFindWidgetTooltipFix(editorNs: EditorNamespace): void {
  editorNs.onDidCreateEditor(() => applyHoverPatch());
}
