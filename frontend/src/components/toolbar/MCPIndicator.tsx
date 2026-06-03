// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: MCP Server

import type { ReactNode } from "react";
import { useEffect } from "react";
import { Tooltip } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { useMCPStore } from "../../store/mcpStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";

// MCPIndicator wraps the connection-details column in a purple frame when MCP
// sessions are active. The frame header shows session count and per-session
// info; clicking it opens the MCP Sessions panel. When no sessions are running
// (or the feature is disabled) children render unwrapped.
export default function MCPIndicator({ children }: { children: ReactNode }) {
  const sessions = useMCPStore((s) => s.sessions);
  const refresh = useMCPStore((s) => s.refresh);
  const enabled = useFeatureFlagsStore((s) => s.flags.mcpServer);

  useEffect(() => {
    void refresh();
    const onChange = () => void refresh();
    window.addEventListener("thaw:mcp-changed", onChange);
    return () => window.removeEventListener("thaw:mcp-changed", onChange);
  }, [refresh]);

  // Sessions in the list are always running — stopped sessions are removed
  // from the Manager map before List() returns.
  const active = sessions.length;
  if (!enabled || active === 0) return <>{children}</>;

  return (
    <div
      style={{
        border: "1px solid #9254de",
        borderRadius: 5,
        overflow: "hidden",
      }}
    >
      {/* ── Header bar ── */}
      <Tooltip title="Click to manage MCP sessions">
        <div
          style={{
            background: "#f9f0ff",
            borderBottom: "1px solid #9254de",
            padding: "2px 8px",
            color: "#531dab",
            fontWeight: 500,
            fontSize: 11,
            lineHeight: 1.4,
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            gap: 4,
            whiteSpace: "nowrap",
          }}
          onClick={() => window.dispatchEvent(new Event("thaw:open-mcp-sessions"))}
        >
          <ApiOutlined style={{ fontSize: 11 }} />
          MCP: {active} active {active === 1 ? "session" : "sessions"}
        </div>
      </Tooltip>
      {/* ── Wrapped connection details ── */}
      <div style={{ padding: "4px 6px" }}>
        {children}
      </div>
    </div>
  );
}
