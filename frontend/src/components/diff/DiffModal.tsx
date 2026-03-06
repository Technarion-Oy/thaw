// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { Modal, Spin } from "antd";
import { DiffEditor } from "@monaco-editor/react";
import { useDiffStore } from "../../store/diffStore";
import { useThemeStore } from "../../store/themeStore";
import { ensureMonacoSetup } from "../editor/monacoSetup";

interface Props {
  onClose: () => void;
}

export default function DiffModal({ onClose }: Props) {
  const { isOpen, leftText, rightText, leftLabel, rightLabel, loading, error } = useDiffStore();
  const resolved       = useThemeStore((s) => s.resolved);
  const editorFont     = useThemeStore((s) => s.editorFont);
  const editorFontSize = useThemeStore((s) => s.editorFontSize);

  return (
    <Modal
      open={isOpen}
      title="Compare"
      onCancel={onClose}
      footer={null}
      width="90vw"
      styles={{ body: { padding: 0, height: "70vh", display: "flex", flexDirection: "column" } }}
      destroyOnClose
    >
      {/* Column headers */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          background: "var(--bg-raised)",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <div
          style={{
            padding: "4px 12px",
            fontSize: 12,
            color: "var(--text-muted)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            borderRight: "1px solid var(--border)",
          }}
          title={leftLabel}
        >
          {leftLabel}
        </div>
        <div
          style={{
            padding: "4px 12px",
            fontSize: 12,
            color: "var(--text-muted)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
          title={rightLabel}
        >
          {rightLabel}
        </div>
      </div>

      {/* Body */}
      <div style={{ flex: 1, position: "relative", minHeight: 0 }}>
        {loading && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              zIndex: 10,
            }}
          >
            <Spin tip="Loading…" />
          </div>
        )}

        {!loading && error && (
          <div
            style={{
              padding: 16,
              color: "#f85149",
              fontFamily: "monospace",
              fontSize: 12,
              whiteSpace: "pre-wrap",
            }}
          >
            {error}
          </div>
        )}

        {!loading && !error && (
          <DiffEditor
            height="100%"
            language="sql"
            theme={resolved === "dark" ? "thaw-dark" : "thaw-light"}
            original={leftText}
            modified={rightText}
            beforeMount={ensureMonacoSetup}
            options={{
              readOnly: true,
              renderSideBySide: true,
              minimap: { enabled: false },
              fontSize: editorFontSize,
              fontFamily: editorFont,
              scrollBeyondLastLine: false,
            }}
          />
        )}
      </div>
    </Modal>
  );
}
