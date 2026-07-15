// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Space, Typography, Alert,
} from "antd";
import {
  PartitionOutlined, ReloadOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, DescribeMCPServer } from "../../../wailsjs/go/app/App";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};
const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle", width: 200,
};

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

// MCP servers have no ALTER statement, so this panel is read-only: it shows the
// SHOW metadata (owner, comment) plus the server_spec from DESCRIBE MCP SERVER.
// To change a server, recreate it with CREATE OR REPLACE.
export default function MCPServerPropertiesModal({ db, schema, name, onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Specification (DESCRIBE MCP SERVER server_spec) read-only viewer state.
  const [spec, setSpec] = useState<string | null>(null);
  const [specLoading, setSpecLoading] = useState(false);
  const [specError, setSpecError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setRows(null); setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "MCP SERVER", name);
      setRows(props ?? []);
    } catch (e) { setError(String(e)); }
  }, [db, schema, name]);

  const loadSpec = useCallback(async () => {
    setSpecLoading(true); setSpecError(null);
    try {
      const res = await DescribeMCPServer(db, schema, name);
      const cols = res?.columns ?? [];
      const idx = cols.findIndex((c) => c.toLowerCase() === "server_spec");
      const val = idx >= 0 && res?.rows?.[0] ? String(res.rows[0][idx] ?? "") : "";
      setSpec(val);
    } catch (e) {
      setSpecError(String(e));
      setSpec("");
    } finally { setSpecLoading(false); }
  }, [db, schema, name]);

  useEffect(() => { reload(); loadSpec(); }, [reload, loadSpec]);

  const serverRef = `"${db}"."${schema}"."${name}"`;
  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const comment = find("comment");
  const owner = find("owner");
  const handledKeys = new Set(["comment", "owner"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <PartitionOutlined style={{ color: "var(--link)" }} />
          <span>MCP Server Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{serverRef}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={900}
      styles={{ body: { maxHeight: "78vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>
      )}
      {error && <Alert type="error" message="Failed to load properties" description={error} showIcon />}
      {rows && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="MCP servers cannot be altered. To change the name, comment, or specification, recreate the server with CREATE OR REPLACE MCP SERVER."
          />

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Owner</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {owner || <Text type="secondary">(unknown)</Text>}
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Comment</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {comment || <Text type="secondary">(not set)</Text>}
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Specification</div>
          {specError && (
            <Alert type="warning" message="Could not read specification" description={specError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space style={{ marginBottom: 8 }}>
            <Button size="small" icon={<ReloadOutlined />} onClick={loadSpec} loading={specLoading}>Reload</Button>
          </Space>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={320}
              language="json"
              theme={editorTheme}
              value={spec ?? ""}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
                readOnly: true,
              }}
            />
          </div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 6 }}>
            The specification is read-only (DESCRIBE MCP SERVER serializes it as JSON).
          </Text>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
