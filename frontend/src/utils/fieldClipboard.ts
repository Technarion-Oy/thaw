// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// Shared native <input>/<textarea> clipboard-splice helpers, used by both the
// app-wide Cmd/Ctrl+V/C/X handler (App.tsx) and the Monaco find/replace
// clipboard routing (monacoClipboard.ts). WKWebView blocks the async Clipboard
// API, so callers read/write text via Wails' native ClipboardGetText/SetText and
// use these helpers to apply it to a focused field. Keeping the splice logic in
// one place means a future fix (e.g. firing additional events) lands once.

/**
 * True if `el` is Monaco's own code-editing surface (the hidden `.inputarea`
 * textarea) rather than an ordinary editable field. Find/replace and rename
 * inputs also live inside `.monaco-editor` but are plain fields, so this is the
 * single source of truth for "leave this to `patchMonacoClipboard` / the editor
 * model" vs. "splice it like any native field".
 */
export function isMonacoCodeSurface(el: Element | null): boolean {
  return !!el && el.classList.contains("inputarea") && !!el.closest(".monaco-editor");
}

/** [lo, hi] selection range of a native field, defaulting to the caret / 0. */
function selectionRange(el: HTMLInputElement | HTMLTextAreaElement): [number, number] {
  const a = el.selectionStart ?? 0;
  const b = el.selectionEnd ?? 0;
  return a <= b ? [a, b] : [b, a];
}

/** The selected substring of a native field ("" if nothing is selected). */
export function fieldSelectionText(el: HTMLInputElement | HTMLTextAreaElement): string {
  const [lo, hi] = selectionRange(el);
  return el.value.slice(lo, hi);
}

/**
 * Replace a native field's current selection with `text`, driving the change
 * through the native value setter so React-controlled inputs (Ant Design etc.)
 * fire onChange, then dispatching `input` so non-React listeners (Monaco's find
 * widget) also react. Leaves the caret after the inserted text.
 */
export function spliceFieldValue(el: HTMLInputElement | HTMLTextAreaElement, text: string): void {
  const [lo, hi] = selectionRange(el);
  const next = el.value.slice(0, lo) + text + el.value.slice(hi);
  const proto = el instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  setter?.call(el, next);
  el.dispatchEvent(new Event("input", { bubbles: true }));
  const caret = lo + text.length;
  el.setSelectionRange(caret, caret);
}
