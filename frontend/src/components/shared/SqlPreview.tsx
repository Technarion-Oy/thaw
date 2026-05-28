// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { Typography } from "antd";

const { Text } = Typography;

interface Props {
  sql: string;
  placeholder?: string;
}

export default function SqlPreview({ sql, placeholder = "-- Fill in required fields" }: Props) {
  return (
    <div
      style={{
        padding: "10px 12px",
        background: "var(--bg)",
        borderRadius: 6,
        border: "1px solid var(--border)",
        marginTop: 4,
      }}
    >
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
        SQL Preview
      </Text>
      <pre
        style={{
          margin: 0,
          color: "var(--text)",
          fontSize: 11,
          fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}
      >
        {sql || placeholder}
      </pre>
    </div>
  );
}
