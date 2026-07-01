// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import type * as monaco from "monaco-editor";
import { ClipboardGetText, ClipboardSetText } from "../../wailsjs/runtime/runtime";

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
  // this module), so we drive it with the Wails native API and fire an `input`
  // event so the find widget re-runs its search after a paste/cut.
  const fieldSelection = (el: HTMLTextAreaElement | HTMLInputElement) => {
    const start = el.selectionStart ?? el.value.length;
    const end = el.selectionEnd ?? el.value.length;
    return start <= end ? [start, end] : [end, start];
  };

  const pasteIntoField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const text = await ClipboardGetText();
    if (!text) return;
    const [start, end] = fieldSelection(el);
    el.value = el.value.slice(0, start) + text + el.value.slice(end);
    const caret = start + text.length;
    el.setSelectionRange(caret, caret);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  };

  const copyFromField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const [start, end] = fieldSelection(el);
    if (start !== end) await ClipboardSetText(el.value.slice(start, end));
  };

  const cutFromField = async (el: HTMLTextAreaElement | HTMLInputElement) => {
    const [start, end] = fieldSelection(el);
    if (start === end) return;
    await ClipboardSetText(el.value.slice(start, end));
    el.value = el.value.slice(0, start) + el.value.slice(end);
    el.setSelectionRange(start, start);
    el.dispatchEvent(new Event("input", { bubbles: true }));
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
