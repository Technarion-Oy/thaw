// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: SQL Editor & Diagnostics

import { ReadFile } from "../../wailsjs/go/app/App";
import { useQueryStore } from "../store/queryStore";

/**
 * Read a file from disk and open it in a new editor tab. Centralises the
 * `ReadFile → openFile` two-step shared by the File menu, the file-tree, and
 * search-result clicks. Returns `null` on success, or the error string on
 * failure — the caller surfaces it via its own `message` instance (the static
 * import in QueryPage vs. the `App.useApp()` hook in FileBrowser).
 */
export async function openFileInTab(path: string): Promise<string | null> {
  try {
    const content = await ReadFile(path);
    useQueryStore.getState().openFile(path, content);
    return null;
  } catch (e) {
    return String(e);
  }
}
