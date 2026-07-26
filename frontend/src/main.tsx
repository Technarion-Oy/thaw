// SPDX-License-Identifier: GPL-3.0-or-later

import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import AppErrorBoundary from "./components/common/AppErrorBoundary";
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

// AppErrorBoundary is outside <App /> so a render-phase throw anywhere in the
// tree (including the theme provider) shows a recoverable message instead of
// unmounting the root and leaving a blank window (issue #875).
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AppErrorBoundary>
      <App />
    </AppErrorBoundary>
  </React.StrictMode>
);
