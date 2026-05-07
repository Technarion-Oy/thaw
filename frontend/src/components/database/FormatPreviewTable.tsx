// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { Tooltip } from "antd";
import type { fileformat } from "../../../wailsjs/go/models";

interface Props {
  previewData: fileformat.PreviewResult | null;
}

export default function FormatPreviewTable({ previewData }: Props) {
  if (!previewData) return null;
  if (!previewData.columns || previewData.columns.length === 0) {
    return (
      <div style={{ padding: "12px 0", textAlign: "center", color: "var(--text-muted)", fontSize: 12 }}>
        No data to preview
      </div>
    );
  }

  return (
    <div style={{
      marginTop: 10,
      border: "1px solid var(--border)",
      borderRadius: 6,
      overflow: "auto",
      maxHeight: 280,
      background: "var(--bg)",
    }}>
      <table style={{ borderCollapse: "separate", borderSpacing: 0, width: "100%", fontSize: 11, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace" }}>
        <thead>
          <tr>
            {previewData.columns.map((c, i) => (
              <th key={i} style={{ 
                position: "sticky",
                top: 0,
                zIndex: 10,
                background: "var(--bg-secondary)",
                padding: "6px 8px", 
                textAlign: "left", 
                whiteSpace: "nowrap",
                fontWeight: 600,
                boxShadow: `inset 0 -1px 0 var(--border), ${i < previewData.columns!.length - 1 ? "inset -1px 0 0 var(--border)" : "none"}`,
              }}>
                <div style={{ position: "absolute", inset: 0, background: "var(--bg)", zIndex: -1 }} />
                {c || <em style={{ color: "var(--text-muted)", fontWeight: 400 }}>(empty)</em>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(previewData.rows ?? []).map((row, ri) => (
            <tr key={ri}>
              {previewData.columns!.map((col, ci) => (
                <td key={ci} style={{ 
                  padding: "4px 8px", 
                  borderBottom: ri < (previewData.rows?.length ?? 0) - 1 ? "1px solid var(--border)" : "none",
                  borderRight: ci < previewData.columns!.length - 1 ? "1px solid var(--border)" : "none", 
                  whiteSpace: "pre", 
                  maxWidth: 200, 
                  overflow: "hidden", 
                  textOverflow: "ellipsis" 
                }}>
                  <Tooltip title={row[col]} placement="topLeft">
                    {row[col] === "" ? <em style={{ color: "var(--text-muted)", fontSize: 10 }}>(empty)</em> : row[col]}
                  </Tooltip>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}