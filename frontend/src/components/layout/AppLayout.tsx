// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useRef, useEffect } from "react";
import { Layout } from "antd";
import LeftPanel from "./LeftPanel";
import Sidebar from "./Sidebar";
import QueryPage from "../../pages/QueryPage";

const { Content } = Layout;

const MIN_WIDTH      = 160;
const MAX_WIDTH      = 600;
const DEFAULT_LEFT   = 220;
const DEFAULT_RIGHT  = 260;

// Detect macOS at module load: Wails uses WKWebView on macOS whose user-agent
// string contains "Macintosh". Edge WebView2 (Windows) and WebKitGTK (Linux)
// do not. On macOS we render a 28 px drag area for the traffic-light buttons
// (which overlap the WebView when TitleBarHiddenInset is active); on other
// platforms the native OS window frame handles all window chrome.
const IS_MAC = /Macintosh/i.test(navigator.userAgent);
const TITLEBAR_HEIGHT = IS_MAC ? 28 : 0;

// Generic hook for a resizable panel. direction controls whether dragging
// right increases ("left" panel) or decreases ("right" panel) the width.
function useResize(initial: number, direction: "left" | "right") {
  const [width, setWidth]     = useState(initial);
  const [resizing, setResizing] = useState(false);
  const startX     = useRef(0);
  const startWidth = useRef(0);

  const onMouseDown = (e: React.MouseEvent) => {
    startX.current     = e.clientX;
    startWidth.current = width;
    setResizing(true);
    e.preventDefault();
  };

  useEffect(() => {
    if (!resizing) return;
    document.body.style.cursor     = "col-resize";
    document.body.style.userSelect = "none";

    const onMove = (e: MouseEvent) => {
      const delta = e.clientX - startX.current;
      const w = direction === "left"
        ? startWidth.current + delta          // left panel grows rightward
        : startWidth.current - delta;         // right panel grows leftward
      setWidth(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, w)));
    };
    const onUp = () => setResizing(false);

    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup",   onUp);
    return () => {
      document.body.style.cursor     = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup",   onUp);
    };
  }, [resizing]);

  return { width, resizing, onMouseDown };
}

function ResizeHandle({ resizing, onMouseDown }: { resizing: boolean; onMouseDown: (e: React.MouseEvent) => void }) {
  return (
    <div
      onMouseDown={onMouseDown}
      style={{
        width:      5,
        flexShrink: 0,
        cursor:     "col-resize",
        background: resizing ? "var(--accent)" : "transparent",
        borderLeft: "1px solid var(--border)",
        transition: resizing ? "none" : "background 0.15s",
        zIndex:     10,
      }}
      onMouseEnter={(e) => { if (!resizing) e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 26%, transparent)"; }}
      onMouseLeave={(e) => { if (!resizing) e.currentTarget.style.background = "transparent"; }}
    />
  );
}

export default function AppLayout() {
  const left  = useResize(DEFAULT_LEFT,  "left");
  const right = useResize(DEFAULT_RIGHT, "right");

  const anyResizing = left.resizing || right.resizing;

  return (
    <Layout style={{ height: "100vh", flexDirection: "row" }}>
      {/* macOS traffic-light drag area — only rendered on macOS */}
      {IS_MAC && (
        <div
          className="titlebar-drag"
          style={{ height: TITLEBAR_HEIGHT, background: "var(--bg-raised)", position: "fixed", top: 0, left: 0, right: 0, zIndex: 100 }}
        />
      )}

      {/* Left panel — file browser + git */}
      <div
        style={{
          width:      left.width,
          minWidth:   left.width,
          maxWidth:   left.width,
          background: "var(--bg-raised)",
          paddingTop: TITLEBAR_HEIGHT,
          overflow:   "auto",
          flexShrink: 0,
        }}
      >
        <LeftPanel />
      </div>

      <ResizeHandle resizing={left.resizing} onMouseDown={left.onMouseDown} />

      {/* Content */}
      <Content
        style={{
          paddingTop: TITLEBAR_HEIGHT,
          overflow: "hidden",
          display: "flex",
          flexDirection: "column",
          flex: 1,
          minWidth: 0,
          // Prevent text selection bleed during either resize
          userSelect: anyResizing ? "none" : undefined,
        }}
      >
        <QueryPage />
      </Content>

      <ResizeHandle resizing={right.resizing} onMouseDown={right.onMouseDown} />

      {/* Right sidebar — database explorer + account objects + export */}
      <div
        style={{
          width:      right.width,
          minWidth:   right.width,
          maxWidth:   right.width,
          background: "var(--bg-raised)",
          paddingTop: TITLEBAR_HEIGHT,
          overflow:   "auto",
          flexShrink: 0,
        }}
      >
        <Sidebar />
      </div>
    </Layout>
  );
}
