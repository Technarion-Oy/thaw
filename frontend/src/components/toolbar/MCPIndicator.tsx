// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: MCP Server

import { useEffect } from "react";
import { Tooltip } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { useMCPStore } from "../../store/mcpStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";

// MCPIndicator is a self-contained toolbar widget showing running MCP session
// info. It is hidden when the MCP Server feature is disabled or when there are
// no running sessions. Clicking it opens the MCP Sessions panel.
export default function MCPIndicator() {
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
  if (!enabled || active === 0) return null;

  return (
    <Tooltip title="Click to manage MCP sessions">
      <div
        style={{
          border: "1px solid #9254de",
          borderRadius: 5,
          overflow: "hidden",
          cursor: "pointer",
          fontSize: 11,
          lineHeight: 1.4,
        }}
        onClick={() => window.dispatchEvent(new Event("thaw:open-mcp-sessions"))}
      >
        {/* ── Thick header bar ── */}
        <div style={{
          background: "#f9f0ff",
          borderBottom: "1px solid #9254de",
          padding: "2px 8px",
          color: "#531dab",
          fontWeight: 500,
          display: "flex",
          alignItems: "center",
          gap: 4,
          whiteSpace: "nowrap",
        }}>
          <ApiOutlined style={{ fontSize: 11 }} />
          MCP: {active} active {active === 1 ? "session" : "sessions"}
        </div>
        {/* ── Session details ── */}
        <div style={{ padding: "3px 8px", color: "var(--text-muted)" }}>
          {sessions.map((s) => (
            <div key={s.label} style={{ whiteSpace: "nowrap" }}>
              {s.connectionLabel} · {s.label} · {s.executionMode}
            </div>
          ))}
        </div>
      </div>
    </Tooltip>
  );
}
