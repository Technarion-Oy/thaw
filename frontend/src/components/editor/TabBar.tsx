// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import { FileOutlined, CodeOutlined, PlusOutlined, CloseOutlined } from "@ant-design/icons";
import { useQueryStore } from "../../store/queryStore";

const CLR_BORDER       = "var(--border)";
const CLR_BG           = "var(--bg)";
const CLR_BG_ACTIVE    = "var(--bg-raised)";
const CLR_TEXT         = "var(--text-muted)";
const CLR_TEXT_ACTIVE  = "var(--text)";
const CLR_ACCENT       = "var(--accent)";

export default function TabBar() {
  const tabs        = useQueryStore((s) => s.tabs);
  const activeTabId = useQueryStore((s) => s.activeTabId);
  const activateTab = useQueryStore((s) => s.activateTab);
  const closeTab    = useQueryStore((s) => s.closeTab);
  const openScratch = useQueryStore((s) => s.openScratch);

  // Track which tab the pointer is hovering over so the close button
  // only appears on hover (less cluttered when many tabs are open).
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  return (
    <div
      style={{
        display: "flex",
        alignItems: "stretch",
        background: CLR_BG,
        borderBottom: `1px solid ${CLR_BORDER}`,
        overflowX: "auto",
        flexShrink: 0,
        scrollbarWidth: "none",
      }}
    >
      {tabs.map((tab) => {
        const active  = tab.id === activeTabId;
        const hovered = tab.id === hoveredId;

        return (
          <div
            key={tab.id}
            onClick={() => activateTab(tab.id)}
            onMouseEnter={() => setHoveredId(tab.id)}
            onMouseLeave={() => setHoveredId(null)}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 5,
              padding: "0 10px",
              height: 32,
              cursor: "pointer",
              borderRight: `1px solid ${CLR_BORDER}`,
              borderBottom: active ? `2px solid ${CLR_ACCENT}` : "2px solid transparent",
              background: active ? CLR_BG_ACTIVE : hovered ? "color-mix(in srgb, var(--text) 5%, transparent)" : CLR_BG,
              color: active ? CLR_TEXT_ACTIVE : CLR_TEXT,
              fontSize: 12,
              userSelect: "none",
              flexShrink: 0,
              maxWidth: 220,
              boxSizing: "border-box",
            }}
          >
            {tab.path
              ? <FileOutlined style={{ fontSize: 11, flexShrink: 0 }} />
              : <CodeOutlined style={{ fontSize: 11, flexShrink: 0 }} />
            }

            <span style={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              flex: 1,
            }}>
              {tab.path && tab.sql !== tab.savedSql ? "• " : ""}{tab.title}
            </span>

            {/* Close button — always reserve space so layout doesn't shift,
                but only show the icon on hover or when this is the active tab
                (and there is more than one tab). */}
            <span
              style={{
                width: 14,
                height: 14,
                flexShrink: 0,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
              onClick={(e) => {
                if (tabs.length <= 1) return;
                e.stopPropagation();
                closeTab(tab.id);
              }}
            >
              {tabs.length > 1 && (active || hovered) && (
                <CloseOutlined style={{ fontSize: 9, opacity: 0.7 }} />
              )}
            </span>
          </div>
        );
      })}

      {/* New scratch tab */}
      <div
        onClick={openScratch}
        onMouseEnter={() => setHoveredId("__plus__")}
        onMouseLeave={() => setHoveredId(null)}
        style={{
          display: "flex",
          alignItems: "center",
          padding: "0 10px",
          cursor: "pointer",
          color: CLR_TEXT,
          fontSize: 12,
          flexShrink: 0,
          background: hoveredId === "__plus__" ? "color-mix(in srgb, var(--text) 5%, transparent)" : "transparent",
        }}
      >
        <PlusOutlined style={{ fontSize: 11 }} />
      </div>
    </div>
  );
}
