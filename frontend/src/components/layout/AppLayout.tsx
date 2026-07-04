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
import { HolderOutlined } from "@ant-design/icons";
import Sidebar from "./Sidebar";
import QueryPage from "../../pages/QueryPage";
import FileBrowser from "../files/FileBrowser";
import GitOperationsDialog from "../git/GitOperationsDialog";
import AccountPanel from "../account/AccountPanel";
import { usePanelLayoutStore, type PanelId, type SidebarId } from "../../store/panelLayoutStore";
import { useGitStore } from "../../store/gitStore";
import { EventsOn } from "../../../wailsjs/runtime/runtime";

const { Content } = Layout;

const MIN_WIDTH = 160;
const MAX_WIDTH = 600;

const IS_MAC = /Macintosh/i.test(navigator.userAgent);
const TITLEBAR_HEIGHT = IS_MAC ? 40 : 0;

// ── Sidebar width resize ───────────────────────────────────────────────────────

function useResize(
  initialWidth: number,
  direction: "left" | "right",
  onCommit: (w: number) => void,
) {
  const [width, setWidth]       = useState(initialWidth);
  const [resizing, setResizing] = useState(false);
  const startX      = useRef(0);
  const startWidth  = useRef(0);
  const liveWidth   = useRef(initialWidth);
  const elRef       = useRef<HTMLDivElement>(null);

  useEffect(() => { setWidth(initialWidth); liveWidth.current = initialWidth; }, [initialWidth]);

  const onMouseDown = (e: React.MouseEvent) => {
    startX.current     = e.clientX;
    startWidth.current = liveWidth.current;
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
        ? startWidth.current + delta
        : startWidth.current - delta;
      const clamped = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, w));
      liveWidth.current = clamped;
      if (elRef.current) {
        elRef.current.style.width    = `${clamped}px`;
        elRef.current.style.minWidth = `${clamped}px`;
        elRef.current.style.maxWidth = `${clamped}px`;
      }
    };
    const onUp = () => {
      setResizing(false);
      const w = liveWidth.current;
      setWidth(w);
      onCommit(w);
    };

    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup",   onUp);
    return () => {
      document.body.style.cursor     = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup",   onUp);
    };
  }, [resizing]);

  return { width, resizing, onMouseDown, elRef };
}

function ResizeHandle({ resizing, onMouseDown }: { resizing: boolean; onMouseDown: (e: React.MouseEvent) => void }) {
  return (
    <div
      onMouseDown={onMouseDown}
      style={{
        width:      5,
        flexShrink: 0,
        cursor:     "col-resize",
        background: resizing ? "var(--accent)" : "color-mix(in srgb, var(--text) 7%, transparent)",
        borderLeft: "1px solid var(--border-strong)",
        transition: resizing ? "none" : "background 0.15s",
        zIndex:     10,
      }}
      onMouseEnter={(e) => { if (!resizing) e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 26%, transparent)"; }}
      onMouseLeave={(e) => { if (!resizing) e.currentTarget.style.background = "color-mix(in srgb, var(--text) 7%, transparent)"; }}
    />
  );
}

// ── Panel drag-and-drop ────────────────────────────────────────────────────────

// Module-level variable tracks which panel is currently being dragged so
// PanelWrappers can avoid showing drop indicators when nothing is in flight.
let _draggingId: PanelId | null = null;

function renderPanelContent(id: PanelId) {
  switch (id) {
    case "files":   return <FileBrowser />;
    case "objects": return <Sidebar hideAccountPanel />;
    case "account": return <AccountPanel />;
  }
}

function PanelWrapper({ id, sidebar }: { id: PanelId; sidebar: SidebarId }) {
  const movePanel = usePanelLayoutStore((s) => s.movePanel);
  const [dropPos, setDropPos] = useState<"before" | "after" | null>(null);

  return (
    <div
      style={{ position: "relative", flexShrink: 0 }}
      onDragOver={(e) => {
        if (!_draggingId || _draggingId === id) return;
        e.preventDefault();
        e.stopPropagation();
        const rect = e.currentTarget.getBoundingClientRect();
        setDropPos(e.clientY < rect.top + rect.height / 2 ? "before" : "after");
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setDropPos(null);
      }}
      onDrop={(e) => {
        e.preventDefault();
        e.stopPropagation();
        const droppedId = e.dataTransfer.getData("text/plain") as PanelId;
        if (droppedId && droppedId !== id) {
          movePanel(droppedId, id, sidebar, dropPos === "before");
        }
        setDropPos(null);
        _draggingId = null;
      }}
    >
      {/* Drop indicator — before */}
      {dropPos === "before" && (
        <div style={{ height: 2, background: "var(--accent)", position: "absolute", top: 0, left: 0, right: 0, zIndex: 100, pointerEvents: "none" }} />
      )}

      {/* Drag handle bar */}
      <div
        draggable
        onDragStart={(e) => {
          _draggingId = id;
          e.dataTransfer.setData("text/plain", id);
          e.dataTransfer.effectAllowed = "move";
        }}
        onDragEnd={() => { _draggingId = null; }}
        style={{
          height: 18,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          cursor: "grab",
          userSelect: "none",
          flexShrink: 0,
          background: "color-mix(in srgb, var(--text) 5%, transparent)",
          borderTop: "1px solid var(--border)",
          transition: "background 0.1s",
        }}
        onMouseEnter={(e) => { e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 18%, transparent)"; }}
        onMouseLeave={(e) => { e.currentTarget.style.background = "color-mix(in srgb, var(--text) 5%, transparent)"; }}
      >
        <HolderOutlined style={{ fontSize: 11, color: "var(--text-muted)", opacity: 0.75 }} />
      </div>

      {renderPanelContent(id)}

      {/* Drop indicator — after */}
      {dropPos === "after" && (
        <div style={{ height: 2, background: "var(--accent)", position: "absolute", bottom: 0, left: 0, right: 0, zIndex: 100, pointerEvents: "none" }} />
      )}
    </div>
  );
}

// Fills remaining sidebar space; acts as a drop target so panels can be moved
// to the bottom of a sidebar or into an otherwise-empty sidebar.
function SidebarDropZone({ sidebar }: { sidebar: SidebarId }) {
  const movePanel = usePanelLayoutStore((s) => s.movePanel);
  const [over, setOver] = useState(false);

  return (
    <div
      style={{
        flex: 1,
        minHeight: 24,
        transition: "background 0.15s",
        background: over ? "color-mix(in srgb, var(--accent) 8%, transparent)" : "transparent",
      }}
      onDragOver={(e) => {
        if (!_draggingId) return;
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        const droppedId = e.dataTransfer.getData("text/plain") as PanelId;
        if (droppedId) movePanel(droppedId, null, sidebar, false);
        setOver(false);
        _draggingId = null;
      }}
    />
  );
}

// ── AppLayout ──────────────────────────────────────────────────────────────────

export default function AppLayout() {
  const storedLeftWidth  = usePanelLayoutStore((s) => s.leftWidth);
  const storedRightWidth = usePanelLayoutStore((s) => s.rightWidth);
  const setLeftWidth     = usePanelLayoutStore((s) => s.setLeftWidth);
  const setRightWidth    = usePanelLayoutStore((s) => s.setRightWidth);
  const leftPanels       = usePanelLayoutStore((s) => s.left);
  const rightPanels      = usePanelLayoutStore((s) => s.right);
  const leftHidden       = usePanelLayoutStore((s) => s.leftHidden);

  const left  = useResize(storedLeftWidth,  "left",  setLeftWidth);
  const right = useResize(storedRightWidth, "right", setRightWidth);

  const anyResizing = left.resizing || right.resizing;

  // Listen for "Git Operations…" menu item
  const openGitOps = useGitStore((s) => s.openGitOps);
  const gitOpsOpen = useGitStore((s) => s.gitOpsOpen);
  useEffect(() => {
    const cleanup = EventsOn("menu:git-operations", () => openGitOps());
    return cleanup;
  }, [openGitOps]);

  // Listen for "Open Folder…" (Cmd+Shift+O) — the top-level, discoverable way to
  // change the working directory, mirroring VS Code's Open Folder.
  const pickExportDir = useGitStore((s) => s.pickExportDir);
  useEffect(() => {
    const cleanup = EventsOn("menu:open-folder", () => pickExportDir());
    return cleanup;
  }, [pickExportDir]);

  // Listen for "Open Folder in New Window…" — spawns a second instance.
  const openInNewWindow = useGitStore((s) => s.openInNewWindow);
  useEffect(() => {
    const cleanup = EventsOn("menu:open-folder-new-window", () => openInNewWindow());
    return cleanup;
  }, [openInNewWindow]);

  const sidebarStyle = (width: number): React.CSSProperties => ({
    width,
    minWidth: width,
    maxWidth: width,
    background:     "var(--bg-raised)",
    paddingTop:     TITLEBAR_HEIGHT,
    overflow:       "auto",
    flexShrink:     0,
    display:        "flex",
    flexDirection:  "column",
  });

  return (
    <Layout style={{ height: "100vh", flexDirection: "row" }}>
      {IS_MAC && (
        <div
          className="titlebar-drag"
          style={{ height: TITLEBAR_HEIGHT, background: "var(--bg-raised)", position: "fixed", top: 0, left: 0, right: 0, zIndex: 100 }}
        />
      )}

      {/* Left sidebar — hidden when ⌘B is toggled */}
      {!leftHidden && (
        <div ref={left.elRef} style={sidebarStyle(left.width)}>
          {leftPanels.map((id) => <PanelWrapper key={id} id={id} sidebar="left" />)}
          <SidebarDropZone sidebar="left" />
        </div>
      )}
      {!leftHidden && <ResizeHandle resizing={left.resizing} onMouseDown={left.onMouseDown} />}

      {/* Center content */}
      <Content
        style={{
          paddingTop:     TITLEBAR_HEIGHT,
          overflow:       "hidden",
          display:        "flex",
          flexDirection:  "column",
          flex:           1,
          minWidth:       0,
          userSelect:     anyResizing ? "none" : undefined,
        }}
        onDragOverCapture={(e) => {
          if (!_draggingId) return;
          e.preventDefault();
          e.stopPropagation();
          e.dataTransfer.dropEffect = "none";
        }}
        onDropCapture={(e) => {
          if (!_draggingId) return;
          e.preventDefault();
          e.stopPropagation();
          _draggingId = null;
        }}
      >
        <QueryPage />
      </Content>

      <ResizeHandle resizing={right.resizing} onMouseDown={right.onMouseDown} />

      {/* Right sidebar */}
      <div ref={right.elRef} style={sidebarStyle(right.width)}>
        {rightPanels.map((id) => <PanelWrapper key={id} id={id} sidebar="right" />)}
        <SidebarDropZone sidebar="right" />
      </div>

      {/* Git Operations Dialog — rendered at layout root so it floats above all panels.
          Mounted only while open so a dragged/resized dialog resets on reopen (#572). */}
      {gitOpsOpen && <GitOperationsDialog />}
    </Layout>
  );
}
