// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

// insertSpecTarget drops a YAML `targets` block into a Monaco editor at the
// caret. If the current line already has content it prefixes a newline so the
// inserted entry starts on its own line. When the editor instance is not yet
// available it falls back to appending the block to the spec text via the
// supplied setter. Shared by the gateway create and properties spec editors.
//
// editor is the Monaco IStandaloneCodeEditor handed to onMount (typed loosely as
// `any` to avoid pulling in the monaco type just for two method calls).
export function insertSpecTarget(
  editor: any,
  block: string,
  fallbackAppend: (block: string) => void,
): void {
  if (editor) {
    const model = editor.getModel?.();
    const pos = editor.getPosition?.();
    const sel = editor.getSelection?.();
    const lineContent = model && pos ? model.getLineContent(pos.lineNumber) : "";
    const prefix = lineContent.trim().length > 0 ? "\n" : "";
    if (sel) {
      editor.executeEdits("insert-endpoint-target", [
        { range: sel, text: prefix + block, forceMoveMarkers: true },
      ]);
      editor.focus();
      return;
    }
  }
  fallbackAppend(block);
}
