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
import { Modal, Tag, Input } from "antd";
import { SearchOutlined, KeyOutlined } from "@ant-design/icons";

interface ShortcutRow {
  action:  string;
  mac:     string;
  win:     string;
  status?: "new";  // highlight newly added shortcuts
}

interface ShortcutGroup {
  title: string;
  rows:  ShortcutRow[];
}

const GROUPS: ShortcutGroup[] = [
  {
    title: "Tabs & Navigation",
    rows: [
      { action: "New Scratch Tab",       mac: "⌘ T",         win: "Ctrl+T" },
      { action: "Open File",             mac: "⌘ O",         win: "Ctrl+O" },
      { action: "Save Active File",      mac: "⌘ S",         win: "Ctrl+S" },
      { action: "Save As…",             mac: "⌘ ⇧ S",       win: "Ctrl+Shift+S" },
      { action: "Close Current Tab",     mac: "⌘ W",         win: "Ctrl+W",        status: "new" },
      { action: "Reopen Closed Tab",     mac: "⌘ ⇧ T",      win: "Ctrl+Shift+T",  status: "new" },
      { action: "Switch to Next Tab",    mac: "⌃ Tab",       win: "Ctrl+Tab",      status: "new" },
      { action: "Switch to Prev Tab",    mac: "⌃ ⇧ Tab",    win: "Ctrl+Shift+Tab",status: "new" },
      { action: "Open Preferences",      mac: "⌘ ,",         win: "Ctrl+,",        status: "new" },
    ],
  },
  {
    title: "Query Execution",
    rows: [
      { action: "Run Query / Selection",        mac: "⌘ Enter",       win: "Ctrl+Enter" },
      { action: "Run All Statements",           mac: "⌘ ⇧ Enter",     win: "Ctrl+Shift+Enter", status: "new" },
      { action: "Cancel Running Query",         mac: "Esc",            win: "Esc" },
      { action: "Focus Results Grid",           mac: "⌘ ↓",           win: "Ctrl+↓",   status: "new" },
      { action: "Export Current Results (CSV)", mac: "⌘ E",           win: "Ctrl+E",   status: "new" },
    ],
  },
  {
    title: "Editor",
    rows: [
      { action: "Toggle Line Comment",        mac: "⌘ /",           win: "Ctrl+/",        status: "new" },
      { action: "Toggle Block Comment",       mac: "⇧ ⌥ A",         win: "Shift+Alt+A",   status: "new" },
      { action: "Format SQL Document",        mac: "⇧ ⌥ F",         win: "Shift+Alt+F",   status: "new" },
      { action: "Trigger Autocomplete",       mac: "Ctrl+Space",     win: "Ctrl+Space" },
      { action: "Accept AI Suggestion",       mac: "Tab",            win: "Tab" },
      { action: "Find in Document",           mac: "⌘ F",           win: "Ctrl+F",        status: "new" },
      { action: "Find and Replace",           mac: "⌘ ⌥ F",        win: "Ctrl+H",        status: "new" },
      { action: "Select Next Occurrence",     mac: "⌘ D",           win: "Ctrl+D",        status: "new" },
      { action: "Go to Line",                mac: "⌃ G",            win: "Ctrl+G",        status: "new" },
      { action: "Zoom In",                   mac: "⌘ +",            win: "Ctrl++" },
      { action: "Zoom Out",                  mac: "⌘ -",            win: "Ctrl+-" },
      { action: "Reset Zoom",               mac: "⌘ 0",             win: "Ctrl+0" },
    ],
  },
  {
    title: "UI & Panels",
    rows: [
      { action: "Toggle Left Sidebar",           mac: "⌘ B",       win: "Ctrl+B",         status: "new" },
      { action: "Focus Object Browser Search",   mac: "⌘ ⇧ F",    win: "Ctrl+Shift+F",   status: "new" },
      { action: "Toggle Split Editor View",      mac: "⌘ \\",      win: "Ctrl+\\",        status: "new" },
      { action: "Open Terminal",                mac: "⌘ `",         win: "Ctrl+`" },
      { action: "Focus AI Chat",                mac: "⌘ L",         win: "Ctrl+L",         status: "new" },
    ],
  },
  {
    title: "Notebook (Command Mode — no cell editor focused)",
    rows: [
      { action: "Run Cell",              mac: "⇧ Enter",  win: "Shift+Enter" },
      { action: "Add Cell Below",        mac: "B",         win: "B",          status: "new" },
      { action: "Add Cell Above",        mac: "A",         win: "A",          status: "new" },
      { action: "Delete Current Cell",   mac: "D D",       win: "D D",        status: "new" },
      { action: "Change Cell to Code",   mac: "Y",         win: "Y",          status: "new" },
      { action: "Change Cell to Markdown", mac: "M",       win: "M",          status: "new" },
      { action: "Change Cell to SQL",    mac: "S",         win: "S",          status: "new" },
    ],
  },
];

interface Props { onClose: () => void; }

export default function KeyboardShortcutsModal({ onClose }: Props) {
  const [search, setSearch] = useState("");

  const q = search.trim().toLowerCase();
  const filtered: ShortcutGroup[] = q
    ? GROUPS.map((g) => ({
        ...g,
        rows: g.rows.filter(
          (r) =>
            r.action.toLowerCase().includes(q) ||
            r.mac.toLowerCase().includes(q) ||
            r.win.toLowerCase().includes(q),
        ),
      })).filter((g) => g.rows.length > 0)
    : GROUPS;

  return (
    <Modal
      open
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <KeyOutlined />
          Keyboard Shortcuts
        </span>
      }
      onCancel={onClose}
      footer={null}
      width={720}
      styles={{ body: { padding: "12px 16px 16px" } }}
    >
      <Input
        prefix={<SearchOutlined style={{ color: "var(--text-muted, #888)" }} />}
        placeholder="Search shortcuts…"
        allowClear
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ marginBottom: 14 }}
        autoFocus
      />

      <div style={{ maxHeight: 520, overflowY: "auto" }}>
        {filtered.map((group) => (
          <div key={group.title} style={{ marginBottom: 20 }}>
            <div style={{
              fontSize: 11,
              fontWeight: 600,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--text-muted, #888)",
              marginBottom: 6,
              paddingBottom: 4,
              borderBottom: "1px solid var(--border-color, #303030)",
            }}>
              {group.title}
            </div>
            <table style={{ width: "100%", borderCollapse: "collapse" }}>
              <colgroup>
                <col style={{ width: "50%" }} />
                <col style={{ width: "25%" }} />
                <col style={{ width: "25%" }} />
              </colgroup>
              <thead>
                <tr>
                  <th style={thStyle}>Action</th>
                  <th style={thStyle}>macOS</th>
                  <th style={thStyle}>Windows / Linux</th>
                </tr>
              </thead>
              <tbody>
                {group.rows.map((row, i) => (
                  <tr key={i} style={{ borderBottom: "1px solid var(--border-color, #1f1f1f)" }}>
                    <td style={tdStyle}>
                      <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
                        {row.action}
                        {row.status === "new" && (
                          <Tag color="blue" style={{ fontSize: 10, lineHeight: "16px", padding: "0 4px" }}>
                            new
                          </Tag>
                        )}
                      </span>
                    </td>
                    <td style={tdStyle}><KbdCell text={row.mac} /></td>
                    <td style={tdStyle}><KbdCell text={row.win} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}

        {filtered.length === 0 && (
          <div style={{ padding: 24, textAlign: "center", color: "var(--text-muted, #888)", fontSize: 13 }}>
            No shortcuts match "{search}"
          </div>
        )}
      </div>
    </Modal>
  );
}

const thStyle: React.CSSProperties = {
  textAlign: "left",
  fontSize: 11,
  fontWeight: 600,
  color: "var(--text-muted, #888)",
  padding: "4px 6px",
};

const tdStyle: React.CSSProperties = {
  padding: "6px 6px",
  fontSize: 12,
  verticalAlign: "middle",
};

/** Render a shortcut string like "⌘ ⇧ T" with each token as a <kbd>. */
function KbdCell({ text }: { text: string }) {
  const parts = text.split(/\s+/);
  return (
    <span style={{ display: "flex", flexWrap: "wrap", gap: 3 }}>
      {parts.map((p, i) => (
        <kbd key={i} style={{
          display: "inline-block",
          padding: "1px 5px",
          borderRadius: 3,
          border: "1px solid var(--border-color, #404040)",
          background: "var(--bg-raised, #252526)",
          fontFamily: "inherit",
          fontSize: 11,
          lineHeight: "18px",
          color: "var(--text, #ccc)",
          whiteSpace: "nowrap",
        }}>
          {p}
        </kbd>
      ))}
    </span>
  );
}
