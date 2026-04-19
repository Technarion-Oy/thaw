// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties provided a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { Modal, Button, Tooltip, message } from "antd";
import { InfoCircleOutlined, CopyOutlined } from "@ant-design/icons";
import { GetAppInfo } from "../../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { main } from "../../../wailsjs/go/models";

interface Props { onClose: () => void; }

export default function AboutModal({ onClose }: Props) {
  const [info, setInfo] = useState<main.AppInfo | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    GetAppInfo().then(setInfo).catch(() => {});
  }, []);

  const copyText = info
    ? [
        `${info.productName}`,
        `Version: ${info.productVersion}`,
        `${info.comments}`,
        `${info.copyright}`,
        `${info.companyName}`,
      ].join("\n")
    : "";

  const handleCopy = () => {
    ClipboardSetText(copyText)
      .then(() => {
        setCopied(true);
        void message.success("Copied to clipboard");
        setTimeout(() => setCopied(false), 2000);
      })
      .catch(() => void message.error("Failed to copy"));
  };

  const rowStyle: React.CSSProperties = {
    display: "flex",
    justifyContent: "space-between",
    padding: "6px 0",
    borderBottom: "1px solid var(--border-color, #303030)",
    fontSize: 13,
  };

  const labelStyle: React.CSSProperties = {
    color: "var(--text-muted, #888)",
    fontWeight: 500,
    minWidth: 120,
  };

  const valueStyle: React.CSSProperties = {
    color: "var(--text, #ccc)",
    textAlign: "right",
  };

  return (
    <Modal
      open
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <InfoCircleOutlined />
          About Thaw
        </span>
      }
      onCancel={onClose}
      width={420}
      styles={{ body: { padding: "16px 20px 8px" } }}
      footer={
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Tooltip title={copied ? "Copied!" : "Copy all info"}>
            <Button
              icon={<CopyOutlined />}
              onClick={handleCopy}
              disabled={!info}
            >
              Copy
            </Button>
          </Tooltip>
          <Button type="primary" onClick={onClose}>
            Close
          </Button>
        </div>
      }
    >
      {info ? (
        <div>
          <div style={rowStyle}>
            <span style={labelStyle}>Product</span>
            <span style={valueStyle}>{info.productName}</span>
          </div>
          <div style={rowStyle}>
            <span style={labelStyle}>Version</span>
            <span style={valueStyle}>{info.productVersion}</span>
          </div>
          <div style={rowStyle}>
            <span style={labelStyle}>Description</span>
            <span style={valueStyle}>{info.comments}</span>
          </div>
          <div style={rowStyle}>
            <span style={labelStyle}>Company</span>
            <span style={valueStyle}>{info.companyName}</span>
          </div>
          <div style={{ ...rowStyle, borderBottom: "none" }}>
            <span style={labelStyle}>Copyright</span>
            <span style={{ ...valueStyle, maxWidth: 240 }}>{info.copyright}</span>
          </div>
        </div>
      ) : (
        <div style={{ padding: "16px 0", textAlign: "center", color: "var(--text-muted, #888)" }}>
          Loading…
        </div>
      )}
    </Modal>
  );
}
