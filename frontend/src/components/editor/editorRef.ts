// SPDX-License-Identifier: GPL-3.0-or-later

// Singleton reference to the active Monaco editor instance.
// Kept in a separate module so it can be imported by non-component files
// (e.g. Sidebar.tsx) without mixing component and non-component exports in
// SqlEditor.tsx, which would break Vite Fast Refresh.

import type * as monaco from "monaco-editor";

let _editorInstance: monaco.editor.IStandaloneCodeEditor | null = null;

export function setEditorInstance(editor: monaco.editor.IStandaloneCodeEditor | null) {
  _editorInstance = editor;
}

export function getEditorInstance(): monaco.editor.IStandaloneCodeEditor | null {
  return _editorInstance;
}

export function insertAtCursor(text: string) {
  if (!_editorInstance) return;
  const selection = _editorInstance.getSelection();
  if (!selection) return;
  _editorInstance.executeEdits("sidebar-insert", [{ range: selection, text, forceMoveMarkers: true }]);
  _editorInstance.pushUndoStop();
  _editorInstance.focus();
}
