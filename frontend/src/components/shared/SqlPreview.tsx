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
  /** Heading shown above the code box. Defaults to "SQL Preview". */
  label?: string;
  /**
   * `compact` (default) — the small inline preview used by most create modals.
   * `prominent` — the larger "Generated SQL" panel used inside multi-column
   * layouts (file format / stage inline builders).
   */
  variant?: "compact" | "prominent";
  /** Extra styles merged into the outer container (e.g. `flexGrow`). */
  style?: React.CSSProperties;
}

export default function SqlPreview({
  sql,
  placeholder = "-- Fill in required fields",
  label = "SQL Preview",
  variant = "compact",
  style,
}: Props) {
  const prominent = variant === "prominent";
  return (
    <div
      style={{
        padding: prominent ? "12px 14px" : "10px 12px",
        background: "var(--bg)",
        borderRadius: prominent ? 8 : 6,
        border: "1px solid var(--border)",
        marginTop: prominent ? undefined : 4,
        ...style,
      }}
    >
      <Text
        type="secondary"
        style={
          prominent
            ? { fontSize: 11, display: "block", marginBottom: 8, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em" }
            : { fontSize: 11, display: "block", marginBottom: 4 }
        }
      >
        {label}
      </Text>
      <pre
        style={{
          margin: 0,
          color: "var(--text)",
          fontSize: prominent ? 12 : 11,
          fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
          lineHeight: prominent ? 1.6 : undefined,
        }}
      >
        {sql || placeholder}
      </pre>
    </div>
  );
}
