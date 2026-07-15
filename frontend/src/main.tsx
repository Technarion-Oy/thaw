// SPDX-License-Identifier: GPL-3.0-or-later

import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles/global.css";
import "./utils/modalDragResize"; // global drag-to-move for all antd modals (#572)
import { ClipboardGetText, ClipboardSetText } from "../wailsjs/runtime/runtime";

// Suppress the WebView's native browser context menu so that right-clicking
// anywhere in the app does not expose browser actions such as "Reload" that
// would wipe all in-memory state (including the active Snowflake connection).
document.addEventListener("contextmenu", (e) => e.preventDefault());

// WKWebView blocks navigator.clipboard.readText / writeText (async Clipboard
// API).  Patch both methods to use the Wails native runtime equivalents so
// that every clipboard path inside Monaco (keyboard and context-menu) works.
// If navigator.clipboard doesn't exist at all, create a minimal stand-in.
try {
  if (!navigator.clipboard) {
    Object.defineProperty(navigator, "clipboard", {
      value: {},
      configurable: true,
      writable: true,
    });
  }
  (navigator.clipboard as any).readText  = () => ClipboardGetText();
  (navigator.clipboard as any).writeText = (text: string) => ClipboardSetText(text);
} catch { /* ignore — DOM interception in SqlEditor is the primary fix */ }

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
