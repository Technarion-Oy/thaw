import { useState, useRef, useEffect } from "react";
import { Layout } from "antd";
import Sidebar from "./Sidebar";
import QueryPage from "../../pages/QueryPage";

const { Content } = Layout;

const MIN_WIDTH     = 160;
const MAX_WIDTH     = 600;
const DEFAULT_WIDTH = 240;

export default function AppLayout() {
  const [sidebarWidth, setSidebarWidth] = useState(DEFAULT_WIDTH);
  const [resizing, setResizing]         = useState(false);
  const startX     = useRef(0);
  const startWidth = useRef(0);

  const onHandleMouseDown = (e: React.MouseEvent) => {
    startX.current     = e.clientX;
    startWidth.current = sidebarWidth;
    setResizing(true);
    e.preventDefault();
  };

  useEffect(() => {
    if (!resizing) return;

    document.body.style.cursor     = "col-resize";
    document.body.style.userSelect = "none";

    const onMove = (e: MouseEvent) => {
      const w = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth.current + e.clientX - startX.current));
      setSidebarWidth(w);
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

  return (
    <Layout style={{ height: "100vh", flexDirection: "row" }}>
      {/* macOS traffic-light drag area */}
      <div
        className="titlebar-drag"
        style={{ height: 28, background: "#161b22", position: "fixed", top: 0, left: 0, right: 0, zIndex: 100 }}
      />

      {/* Sidebar */}
      <div
        style={{
          width:     sidebarWidth,
          minWidth:  sidebarWidth,
          maxWidth:  sidebarWidth,
          background: "#161b22",
          paddingTop: 28,
          overflow:  "auto",
          flexShrink: 0,
        }}
      >
        <Sidebar />
      </div>

      {/* Resize handle */}
      <div
        onMouseDown={onHandleMouseDown}
        style={{
          width:      5,
          flexShrink: 0,
          cursor:     "col-resize",
          background: resizing ? "#388bfd" : "transparent",
          borderLeft: "1px solid #30363d",
          transition: resizing ? "none" : "background 0.15s",
          zIndex:     10,
        }}
        onMouseEnter={(e) => { if (!resizing) e.currentTarget.style.background = "#388bfd44"; }}
        onMouseLeave={(e) => { if (!resizing) e.currentTarget.style.background = "transparent"; }}
      />

      {/* Content */}
      <Content
        style={{ paddingTop: 28, overflow: "hidden", display: "flex", flexDirection: "column", flex: 1, minWidth: 0 }}
      >
        <QueryPage />
      </Content>
    </Layout>
  );
}
