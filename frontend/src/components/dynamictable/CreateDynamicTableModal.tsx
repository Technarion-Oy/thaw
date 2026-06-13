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

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Checkbox, Select, Space,
  Typography, Button, Alert,
} from "antd";
import { RetweetOutlined } from "@ant-design/icons";
import { BuildCreateDynamicTableSql, ExecDDL, GetQuotedIdentifiersIgnoreCase, ListWarehouses } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { dynamictable } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_QUERY = "SELECT *\n  FROM my_source_table";

export default function CreateDynamicTableModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<dynamictable.DynamicTableConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    transient: false,
    targetLag: "1 minute",
    warehouse: "",
    refreshMode: "",
    initialize: "",
    clusterBy: "",
    comment: "",
    query: DEFAULT_QUERY,
  });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});

    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  useEffect(() => {
    BuildCreateDynamicTableSql(db, schema, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof dynamictable.DynamicTableConfig>(key: K, value: dynamictable.DynamicTableConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.warehouse.trim().length > 0 &&
    cfg.targetLag.trim().length > 0 &&
    cfg.query.trim().length > 0;

  const handleRun = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <RetweetOutlined style={{ color: "var(--link)" }} />
          <span>Create Dynamic Table</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<RetweetOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={720}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Dynamic table creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Dynamic table name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_DYNAMIC_TABLE"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Space direction="vertical" size={4}>
              <Checkbox
                checked={cfg.orReplace}
                onChange={(e) => {
                  set("orReplace", e.target.checked);
                  if (e.target.checked) set("ifNotExists", false);
                }}
              >
                OR REPLACE
              </Checkbox>
              <Checkbox
                checked={cfg.ifNotExists}
                disabled={cfg.orReplace}
                onChange={(e) => set("ifNotExists", e.target.checked)}
              >
                IF NOT EXISTS
              </Checkbox>
              <Checkbox
                checked={cfg.transient}
                onChange={(e) => set("transient", e.target.checked)}
              >
                TRANSIENT
              </Checkbox>
            </Space>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Target Lag" required style={itemStyle} help="Maximum staleness, e.g. '1 minute', '2 hours', or DOWNSTREAM">
            <Input
              value={cfg.targetLag}
              onChange={(e) => set("targetLag", e.target.value)}
              placeholder="1 minute"
            />
          </Form.Item>
          <Form.Item label="Warehouse" required style={itemStyle} help="Warehouse used to refresh the table">
            <Select
              showSearch
              loading={loadingWarehouses}
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              placeholder="Select warehouse…"
              options={warehouseOptions}
              style={{ width: "100%" }}
              notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
            />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Refresh Mode" style={itemStyle} help="How rows are refreshed">
            <Select
              allowClear
              value={cfg.refreshMode || undefined}
              onChange={(v) => set("refreshMode", v ?? "")}
              placeholder="AUTO (default)"
              style={{ width: "100%" }}
              options={[
                { value: "AUTO", label: "AUTO" },
                { value: "FULL", label: "FULL" },
                { value: "INCREMENTAL", label: "INCREMENTAL" },
              ]}
            />
          </Form.Item>
          <Form.Item label="Initialize" style={itemStyle} help="When the first refresh runs">
            <Select
              allowClear
              value={cfg.initialize || undefined}
              onChange={(v) => set("initialize", v ?? "")}
              placeholder="ON_CREATE (default)"
              style={{ width: "100%" }}
              options={[
                { value: "ON_CREATE", label: "ON_CREATE" },
                { value: "ON_SCHEDULE", label: "ON_SCHEDULE" },
              ]}
            />
          </Form.Item>
        </div>

        <Form.Item label="Cluster By" style={itemStyle} help="Optional comma-separated clustering expressions">
          <Input
            value={cfg.clusterBy}
            onChange={(e) => set("clusterBy", e.target.value)}
            placeholder="col1, col2"
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Form.Item label="Defining Query (AS)" required style={itemStyle}>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={140}
              language="sql"
              theme={editorTheme}
              value={cfg.query}
              onChange={(v) => set("query", v ?? "")}
              onMount={(editor) => patchMonacoClipboard(editor)}
              options={{
                minimap: { enabled: false },
                lineNumbers: "off",
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
        </Form.Item>

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
            {preview}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
