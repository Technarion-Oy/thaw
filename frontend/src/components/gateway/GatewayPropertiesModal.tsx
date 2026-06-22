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

import { useState, useEffect, useCallback, useRef } from "react";
import {
  Modal, Spin, Button, Space, Typography, Alert, Tooltip, message,
} from "antd";
import {
  NodeIndexOutlined, ReloadOutlined, CopyOutlined, SaveOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, DescribeGateway, AlterGateway } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import EndpointTargetPicker from "./EndpointTargetPicker";
import { insertSpecTarget } from "./insertSpecTarget";
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

// A small URL row with a native-clipboard copy button (WKWebView blocks
// navigator.clipboard, so all copies go through the Wails ClipboardSetText API).
function UrlRow({ label, url }: { label: string; url: string }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
        {url ? (
          <Space>
            <Text style={{ fontFamily: "var(--font-mono)", fontSize: 12, wordBreak: "break-all" }}>{url}</Text>
            <Tooltip title="Copy URL">
              <Button
                type="text"
                size="small"
                icon={<CopyOutlined style={{ fontSize: 12 }} />}
                onClick={() => ClipboardSetText(url)}
              />
            </Tooltip>
          </Space>
        ) : (
          <Text type="secondary">(unavailable)</Text>
        )}
      </td>
    </tr>
  );
}

// Gateways front Snowpark Container Services endpoints, splitting ingress
// traffic per the YAML specification. The entire ALTER GATEWAY surface is the
// FROM SPECIFICATION update, so this panel shows the SHOW metadata plus the
// DESCRIBE GATEWAY ingress URL(s) and exposes the live specification in an
// editable Monaco editor — saving runs ALTER GATEWAY … FROM SPECIFICATION.
export default function GatewayPropertiesModal({ db, schema, name, onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Specification (DESCRIBE GATEWAY spec) editor state.
  const [spec, setSpec] = useState<string>("");
  const [loadedSpec, setLoadedSpec] = useState<string>(""); // last value read from Snowflake
  const [ingressUrl, setIngressUrl] = useState("");
  const [privatelinkUrl, setPrivatelinkUrl] = useState("");
  const [specLoading, setSpecLoading] = useState(false);
  const [specError, setSpecError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const editorRef = useRef<any>(null);

  const reload = useCallback(async () => {
    setRows(null); setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "GATEWAY", name);
      setRows(props ?? []);
    } catch (e) { setError(String(e)); }
  }, [db, schema, name]);

  const loadSpec = useCallback(async () => {
    setSpecLoading(true); setSpecError(null);
    try {
      const res = await DescribeGateway(db, schema, name);
      const cols = res?.columns ?? [];
      const row = res?.rows?.[0];
      const get = (col: string) => {
        const idx = cols.findIndex((c) => c.toLowerCase() === col);
        return idx >= 0 && row ? String(row[idx] ?? "") : "";
      };
      const s = get("spec");
      setSpec(s);
      setLoadedSpec(s);
      setIngressUrl(get("ingress_url"));
      setPrivatelinkUrl(get("privatelink_ingress_url"));
    } catch (e) {
      setSpecError(String(e));
    } finally { setSpecLoading(false); }
  }, [db, schema, name]);

  useEffect(() => { reload(); loadSpec(); }, [reload, loadSpec]);

  const saveSpec = async () => {
    if (!spec.trim()) { message.warning("Specification cannot be empty."); return; }
    setSaving(true);
    try {
      await AlterGateway(db, schema, name, spec);
      message.success("Specification updated.");
      await loadSpec();
    } catch (e) {
      message.error(`Failed to update specification: ${String(e)}`);
    } finally { setSaving(false); }
  };

  const gatewayRef = `"${db}"."${schema}"."${name}"`;
  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const owner = find("owner");
  const gatewayType = find("gateway_type");
  const comment = find("comment");
  const handledKeys = new Set(["owner", "gateway_type", "comment"]);
  const dirty = spec !== loadedSpec;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <NodeIndexOutlined style={{ color: "var(--link)" }} />
          <span>Gateway Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{gatewayRef}</Text>
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
            message="A gateway’s only mutable property is its traffic-split specification. Editing the YAML below and saving runs ALTER GATEWAY … FROM SPECIFICATION. To rename a gateway, recreate it with CREATE OR REPLACE."
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
                <td style={LABEL_TD}>Gateway type</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {gatewayType || <Text type="secondary">(not set)</Text>}
                </td>
              </tr>
              <UrlRow label="Ingress URL" url={ingressUrl} />
              <UrlRow label="PrivateLink ingress URL" url={privatelinkUrl} />
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
            <Button
              size="small"
              type="primary"
              icon={<SaveOutlined />}
              onClick={saveSpec}
              loading={saving}
              disabled={!dirty || !spec.trim()}
            >
              Update specification
            </Button>
            {dirty && <Text type="warning" style={{ fontSize: 11 }}>Unsaved changes</Text>}
          </Space>
          <EndpointTargetPicker
            defaultDb={db}
            defaultSchema={schema}
            onInsert={(block) => insertSpecTarget(editorRef.current, block, (b) => setSpec((s) => s.replace(/\s*$/, "") + "\n" + b))}
          />
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={320}
              language="yaml"
              theme={editorTheme}
              value={spec}
              onChange={(v) => setSpec(v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); editorRef.current = editor; }}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 6 }}>
            Traffic-split YAML (weights across all endpoint targets must sum to 100). The spec is only readable with USAGE / MODIFY / OWNERSHIP on the gateway.
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
