// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { Modal, Spin, Button, message } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { main } from "../../../wailsjs/go/models";

interface Props {
  title: string;
  rows: main.PropertyPair[] | null;  // null = loading
  error: string | null;
  onClose: () => void;
}

export default function PropertiesModal({ title, rows, error, onClose }: Props) {
  const loading = rows === null && !error;

  const copyAll = () => {
    if (!rows) return;
    const text = rows.map((r) => `${r.key}: ${r.value}`).join("\n");
    ClipboardSetText(text).then(() => message.success("Copied to clipboard"));
  };

  return (
    <Modal
      open
      title={title}
      onCancel={onClose}
      width={620}
      footer={[
        <Button
          key="copy"
          icon={<CopyOutlined />}
          disabled={!rows || rows.length === 0}
          onClick={copyAll}
        >
          Copy
        </Button>,
        <Button key="close" onClick={onClose}>
          Close
        </Button>,
      ]}
    >
      {loading && (
        <div style={{ textAlign: "center", padding: "32px 0" }}>
          <Spin />
        </div>
      )}

      {error && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
          {error}
        </div>
      )}

      {rows && rows.length === 0 && !error && (
        <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 8 }}>
          No properties found.
        </div>
      )}

      {rows && rows.length > 0 && (
        <div style={{ maxHeight: "60vh", overflowY: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.key}
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  <td
                    style={{
                      padding: "5px 12px 5px 0",
                      color: "var(--text-muted)",
                      fontFamily: "monospace",
                      whiteSpace: "nowrap",
                      verticalAlign: "top",
                      width: 200,
                      minWidth: 160,
                    }}
                  >
                    {row.key}
                  </td>
                  <td
                    style={{
                      padding: "5px 0",
                      color: "var(--text)",
                      fontFamily: "monospace",
                      wordBreak: "break-word",
                      verticalAlign: "top",
                    }}
                  >
                    {row.value || <span style={{ color: "var(--text-muted)", fontStyle: "italic" }}>—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}
