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
      switch (e.key.toLowerCase()) {
        case "v": e.preventDefault(); e.stopPropagation(); doPaste(); break;
        case "c": e.preventDefault(); e.stopPropagation(); doCopy(); break;
        case "x": e.preventDefault(); e.stopPropagation(); doCut(); break;
      }
    }, true /* capture */);
  }
}
