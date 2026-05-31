// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: MCP Server

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
  message,
} from "antd";
import { CopyOutlined, PoweroffOutlined } from "@ant-design/icons";
import {
  GetMCPSessionConfig,
  StartMCPSession,
  StopMCPSession,
} from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useMCPStore } from "../../store/mcpStore";
import { useConnectionStore } from "../../store/connectionStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

// Execution modes. Only metadata browsing is supported in the foundation
// milestone; the value must match internal/mcp.ExecutionModeMetadata.
const EXECUTION_MODES = [
  { value: "metadata", label: "Metadata Only" },
];

export default function MCPSessionsModal({ onClose }: Props) {
  const sessions = useMCPStore((s) => s.sessions);
  const refresh = useMCPStore((s) => s.refresh);
  const isConnected = useConnectionStore((s) => s.isConnected);
  const mcpEnabled = useFeatureFlagsStore((s) => s.flags.mcpServer);

  // The feature can be off and admin-locked; the native menu can still open
  // this modal, so disable starting sessions and explain why.
  const canStart = isConnected && mcpEnabled;

  const [label, setLabel] = useState("");
  const [mode, setMode] = useState("metadata");
  const [port, setPort] = useState<number | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function handleStart() {
    const trimmed = label.trim();
    if (!trimmed) {
      setError("A session label is required.");
      return;
    }
    setStarting(true);
    setError(null);
    try {
      await StartMCPSession(trimmed, mode, port ?? 0);
      await refresh();
      window.dispatchEvent(new Event("thaw:mcp-changed"));
      setLabel("");
      setPort(null);
      message.success(`MCP session "${trimmed}" started`);
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setStarting(false);
    }
  }

  async function handleStop(sessionLabel: string) {
    try {
      await StopMCPSession(sessionLabel);
      await refresh();
      window.dispatchEvent(new Event("thaw:mcp-changed"));
      message.success(`MCP session "${sessionLabel}" stopped`);
    } catch (e: unknown) {
      message.error(String(e));
    }
  }

  async function handleCopy(sessionLabel: string) {
    try {
      const cfg = await GetMCPSessionConfig(sessionLabel);
      await ClipboardSetText(cfg);
      message.success("Client configuration copied to clipboard");
    } catch (e: unknown) {
      message.error(String(e));
    }
  }

  return (
    <Modal
      open
      title="MCP Sessions"
      onCancel={onClose}
      width={560}
      styles={{ body: { paddingTop: 8, maxHeight: "70vh", overflowY: "auto" } }}
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {!mcpEnabled && (
        <Alert
          type="warning"
          showIcon
          message="MCP Server is disabled. Enable it under View → Enabled Features… (an IT administrator may have locked this)."
          style={{ marginBottom: 12 }}
        />
      )}
      {mcpEnabled && !isConnected && (
        <Alert
          type="info"
          showIcon
          message="Connect to Snowflake to start an MCP session."
          style={{ marginBottom: 12 }}
        />
      )}
      {error && (
        <Alert
          type="error"
          message={error}
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 12 }}
        />
      )}

      {/* The SSE endpoint is protected by a per-session token, so the copied
          client configuration is a secret. A local administrator can still
          bypass this (process memory, loopback capture), so stop sessions when
          you're done. */}
      <Alert
        type="warning"
        showIcon
        message="A running session exposes your connection's schema metadata to any MCP client holding this session's token. The copied configuration contains that token — treat it like a password and don't share it. Stop sessions when you're done."
        style={{ marginBottom: 12 }}
      />

      {/* ── Start session form ── */}
      <Form layout="vertical" size="small" style={{ marginBottom: 20 }}>
        <Space align="end" wrap style={{ width: "100%" }}>
          <Form.Item
            label={<Text style={{ fontSize: 12 }}>Label</Text>}
            style={{ marginBottom: 0 }}
          >
            <Input
              placeholder="e.g. analytics"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              style={{ width: 180 }}
              disabled={!canStart}
            />
          </Form.Item>
          <Form.Item
            label={<Text style={{ fontSize: 12 }}>Execution mode</Text>}
            style={{ marginBottom: 0 }}
          >
            <Select
              value={mode}
              onChange={setMode}
              options={EXECUTION_MODES}
              style={{ width: 150 }}
              disabled={!canStart}
            />
          </Form.Item>
          <Form.Item
            label={
              <Tooltip title="Leave blank to auto-assign from 9100.">
                <Text style={{ fontSize: 12 }}>Port</Text>
              </Tooltip>
            }
            style={{ marginBottom: 0 }}
          >
            <InputNumber
              min={1}
              max={65535}
              placeholder="auto"
              value={port}
              onChange={(v) => setPort(v)}
              style={{ width: 90 }}
              disabled={!canStart}
            />
          </Form.Item>
          <Button
            type="primary"
            loading={starting}
            onClick={handleStart}
            disabled={!canStart}
          >
            Start Session
          </Button>
        </Space>
      </Form>

      {/* ── Running sessions ── */}
      {sessions.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="No MCP sessions running"
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {sessions.map((s) => (
            <div
              key={s.label}
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                gap: 12,
                padding: "10px 12px",
                border: "1px solid var(--border-color, rgba(0,0,0,0.08))",
                borderRadius: 6,
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <Text strong style={{ fontSize: 13 }}>{s.label}</Text>
                  <Tag color={s.running ? "green" : "default"}>
                    {s.running ? "Running" : "Stopped"}
                  </Tag>
                </div>
                <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                  {s.connectionLabel} · port {s.port} · {s.executionMode}
                </div>
                <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {/* Token-free endpoint, shown for reference only. Use "Copy
                      client configuration" to get the URL with the auth token. */}
                  <Text code style={{ fontSize: 11 }}>{s.url}</Text>
                </div>
              </div>
              <Space>
                <Tooltip title="Copy client configuration">
                  <Button
                    size="small"
                    icon={<CopyOutlined />}
                    onClick={() => handleCopy(s.label)}
                  />
                </Tooltip>
                <Tooltip title="Stop session">
                  <Button
                    size="small"
                    danger
                    icon={<PoweroffOutlined />}
                    onClick={() => handleStop(s.label)}
                  />
                </Tooltip>
              </Space>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}
