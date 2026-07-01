// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import type * as monaco from "monaco-editor";
import { ClipboardGetText, ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { spliceFieldValue, fieldSelectionText } from "./fieldClipboard";
// Deep internals — needed to fix find-widget tooltip clipping; see forceHoverTooltipsBelow().
// @ts-expect-error no type declarations for this deep Monaco path
import { StandaloneServices } from "monaco-editor/esm/vs/editor/standalone/browser/standaloneServices.js";
// @ts-expect-error no type declarations for this deep Monaco path
import { IHoverService } from "monaco-editor/esm/vs/platform/hover/browser/hover.js";

let hoverTooltipsPatched = false;

/**
 * Force Monaco's base-layer hover tooltips (find-widget button tooltips — the
 * Aa/ab/.* toggles, prev/next, close) to render BELOW their target instead of
 * the default ABOVE. The find widget is pinned to the editor's top edge, so
 * "above" lands in the tab-bar band where the editor pane's `overflow: hidden`
 * clips it away (issue #593). `_createHover` is the single choke point both
 * showInstantHover and showDelayedHover funnel through; forcing BELOW there
 * (only when the caller set no explicit position) drops these tooltips into the
 * editor body, unclipped. Monaco still auto-flips back to ABOVE if BELOW would
 * overflow the window, so this stays correct for targets that aren't near the
 * top. The code hover uses a content widget (not this service), so it's
 * unaffected.
 *
 * This patches a session-wide singleton, so it only needs to succeed once — but
 * it must run AFTER an editor exists (`setBaseLayerHoverDelegate(hoverService)`
 * runs in the StandaloneCodeEditor constructor). It's called from
 * `patchMonacoClipboard` (every SqlEditor/modal Monaco mount) and from
 * NotebookTab's onMount, all post-mount; idempotent and retryable.
 */
export function forceHoverTooltipsBelow(): void {
  if (hoverTooltipsPatched) return;
  const HOVER_POSITION_BELOW = 2; // HoverPosition.BELOW
  const hoverService = StandaloneServices.get(IHoverService) as {
    _createHover?: (options: { position?: { hoverPosition?: unknown } }, skip?: unknown) => unknown;
  } | undefined;
  if (!hoverService || typeof hoverService._createHover !== "function") return;
  const original = hoverService._createHover.bind(hoverService);
  hoverService._createHover = (options, skip) => {
    if (options && options.position?.hoverPosition === undefined) {
      options = { ...options, position: { ...options.position, hoverPosition: HOVER_POSITION_BELOW } };
    }
    return original(options, skip);
  };
  hoverTooltipsPatched = true;
}

/**
 * Patches a Monaco editor instance to use Wails' native clipboard APIs.
 * This is required in WKWebView (macOS) where navigator.clipboard is blocked.
 */
export function patchMonacoClipboard(editor: monaco.editor.IStandaloneCodeEditor | monaco.editor.IStandaloneDiffEditor) {
  // For DiffEditor, we need to patch both internal editors
  if ((editor as any).getOriginalEditor) {
    patchMonacoClipboard((editor as any).getOriginalEditor());
    patchMonacoClipboard((editor as any).getModifiedEditor());
    return;
  }

  const codeEditor = editor as monaco.editor.IStandaloneCodeEditor;

  // Every Monaco mount routes through here, so this is a reliable post-mount hook
  // to apply the session-wide find-widget tooltip fix (idempotent — no-op after
  // the first successful call).
  forceHoverTooltipsBelow();

  const doPaste = async () => {
    const text = await ClipboardGetText();
    if (!text) return;
    const selection = codeEditor.getSelection();
    if (!selection) return;
    codeEditor.executeEdits("clipboard-paste", [{ range: selection, text, forceMoveMarkers: true }]);
    codeEditor.pushUndoStop();
  };

  const doCopy = async () => {
    const selection = codeEditor.getSelection();
    const model = codeEditor.getModel();
    if (!selection || !model) return;
    const text = model.getValueInRange(selection);
    if (text) await ClipboardSetText(text);
  };

  const doCut = async () => {
    const selection = codeEditor.getSelection();
    const model = codeEditor.getModel();
    if (!selection || !model) return;
    const text = model.getValueInRange(selection);
    if (!text) return;
    await ClipboardSetText(text);
    codeEditor.executeEdits("clipboard-cut", [{ range: selection, text: "", forceMoveMarkers: true }]);
    codeEditor.pushUndoStop();
  };

  // The find / replace widgets host their own <textarea>/<input> inside the
  // editor DOM. Cmd+V/C/X land here (capture-phase, below) even when one of
  // those fields is focused, so we must route clipboard ops to the field rather
  // than the code buffer. WKWebView clipboard is untrusted (the whole reason for
  // this module), so we drive it with the Wails native API; spliceFieldValue
  // fires an `input` event so the find widget re-runs its search after paste/cut.
  const pasteIntoField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const text = await ClipboardGetText();
    if (text) spliceFieldValue(el, text);
  };

  const copyFromField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const text = fieldSelectionText(el);
    if (text) await ClipboardSetText(text);
  };

  const cutFromField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const text = fieldSelectionText(el);
    if (!text) return;
    await ClipboardSetText(text);
    spliceFieldValue(el, "");
  };

  // Monaco's own typing surface is `<textarea class="inputarea">`; anything else
  // editable inside the editor DOM is a find/replace field.
  const findField = (target: EventTarget | null): HTMLTextAreaElement | HTMLInputElement | null =>
    (target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement) &&
    !target.classList.contains("inputarea")
      ? target
      : null;

  const cs = (codeEditor as any)._commandService;
  if (cs && typeof cs.executeCommand === "function") {
    const origExec = cs.executeCommand.bind(cs);
    cs.executeCommand = (commandId: string, ...args: any[]): Promise<any> => {
      switch (commandId) {
        case "editor.action.clipboardPasteAction":
          doPaste();
          return Promise.resolve();
        case "editor.action.clipboardCopyAction":
          doCopy();
          return Promise.resolve();
        case "editor.action.clipboardCutAction":
          doCut();
          return Promise.resolve();
        default:
          return origExec(commandId, ...args);
      }
    };
  }

  // Capture-phase keydown listener so clipboard shortcuts are intercepted
  // before WKWebView or Monaco can swallow them.
  const editorDom = codeEditor.getDomNode();
  if (editorDom) {
    editorDom.addEventListener("keydown", (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      const key = e.key.toLowerCase();
      if (key !== "v" && key !== "c" && key !== "x") return;
      e.preventDefault();
      e.stopPropagation();
      const field = findField(e.target);
      if (field) {
        if (key === "v") pasteIntoField(field);
        else if (key === "c") copyFromField(field);
        else cutFromField(field);
      } else {
        if (key === "v") doPaste();
        else if (key === "c") doCopy();
        else doCut();
      }
    }, true /* capture */);
  }
}
