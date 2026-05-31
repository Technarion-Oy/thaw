// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: MCP Server

import { useEffect } from "react";
import { Tag, Tooltip } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { useMCPStore } from "../../store/mcpStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";

// MCPIndicator is a self-contained toolbar pill showing the number of running
// MCP sessions. It is hidden when the MCP Server feature is disabled or when
// there are no running sessions. Clicking it opens the MCP Sessions panel.
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
    <Tooltip title="Open MCP Sessions">
      <Tag
        icon={<ApiOutlined />}
        color="purple"
        style={{ margin: 0, cursor: "pointer", fontSize: 11 }}
        onClick={() => window.dispatchEvent(new Event("thaw:open-mcp-sessions"))}
      >
        MCP: {active} active
      </Tag>
    </Tooltip>
  );
}
