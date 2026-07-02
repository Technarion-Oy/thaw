// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import type * as monaco from "monaco-editor";
import { ClipboardGetText, ClipboardSetText } from "../../wailsjs/runtime/runtime";

/**
 * Patches a Monaco editor instance to route the CODE BUFFER's copy / cut / paste
 * through Wails' native clipboard APIs. Required in WKWebView (macOS) where
 * `navigator.clipboard` is blocked. The code buffer must go through the editor
 * model (not a value splice), which is why it's handled per-editor here; every
 * other editable inside `.monaco-editor` (find/replace, rename) is an ordinary
 * field handled by the global handler in `App.tsx`.
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

  // Copy is cut without the delete-and-undo-stop step.
  const doCopyOrCut = async (cut: boolean) => {
    // Cut can't delete from a read-only buffer — mirror VS Code and no-op rather
    // than writing the clipboard and silently behaving like copy.
    if (cut && codeEditor.getRawOptions().readOnly) return;
    const selection = codeEditor.getSelection();
    const model = codeEditor.getModel();
    if (!selection || !model) return;
    const text = model.getValueInRange(selection);
    if (!text) return;
    await ClipboardSetText(text);
    if (cut) {
      codeEditor.executeEdits("clipboard-cut", [{ range: selection, text: "", forceMoveMarkers: true }]);
      codeEditor.pushUndoStop();
    }
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
          doCopyOrCut(false);
          return Promise.resolve();
        case "editor.action.clipboardCutAction":
          doCopyOrCut(true);
          return Promise.resolve();
        default:
          return origExec(commandId, ...args);
      }
    };
  }

  // Capture-phase keydown listener so clipboard shortcuts are intercepted
  // before WKWebView or Monaco can swallow them. Only the code buffer is handled
  // here; when a find/replace/rename field is focused we let the event bubble to
  // App.tsx's global handler (which splices native fields) instead.
  const editorDom = codeEditor.getDomNode();
  if (editorDom) {
    editorDom.addEventListener("keydown", (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      const key = e.key.toLowerCase();
      if (key !== "v" && key !== "c" && key !== "x") return;
      // Only handle the code buffer. `hasTextFocus()` (public API) is true only
      // when the editor's own text input is focused — not the find/replace or
      // rename fields — so those bubble to App.tsx's global native-field handler.
      if (!codeEditor.hasTextFocus()) return;
      e.preventDefault();
      e.stopPropagation();
      if (key === "v") doPaste();
      else doCopyOrCut(key === "x");
    }, true /* capture */);
  }
}
