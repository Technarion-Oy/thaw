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
  Modal, Form, Input, Select, Checkbox, Space,
  Typography, Button, Alert,
} from "antd";
import { BuildOutlined } from "@ant-design/icons";
import {
  BuildCreateDbtProjectSql,
  ExecDDL,
  GetQuotedIdentifiersIgnoreCase,
  ListExternalAccessIntegrations,
  ListSupportedDbtVersions,
} from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import { dbtproject } from "../../../wailsjs/go/models";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateDbtProjectModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<dbtproject.CreateConfig>(new dbtproject.CreateConfig({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    sourceLocation: "",
    comment: "",
    dbtVersion: "",
    defaultTarget: "",
    externalAccessIntegrations: [],
  }));
  const [eaiList, setEaiList] = useState<snowflake.IntegrationRow[]>([]);
  const [dbtVersions, setDbtVersions] = useState<dbtproject.DbtVersionInfo[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [loadingEai, setLoadingEai] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    setLoadingEai(true);
    ListExternalAccessIntegrations()
      .then((rows) => setEaiList(rows ?? []))
      .catch(() => {})
      .finally(() => setLoadingEai(false));

    setLoadingVersions(true);
    ListSupportedDbtVersions()
      .then((v) => setDbtVersions(v ?? []))
      .catch(() => {})
      .finally(() => setLoadingVersions(false));

    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

  useEffect(() => {
    BuildCreateDbtProjectSql(db, schema, cfg).then(setPreview).catch(() => setPreview(""));
  }, [db, schema, cfg]);

  const set = <K extends keyof dbtproject.CreateConfig>(key: K, value: dbtproject.CreateConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim() !== "" && cfg.sourceLocation.trim() !== "";

  const handleRun = async () => {
    if (!canSubmit || !preview) return;
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

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <BuildOutlined style={{ color: "var(--link)" }} />
          <span>Create DBT Project</span>
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
            icon={<BuildOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={620}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">
        {/* Name row */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Project name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_DBT_PROJECT"
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

        <Form.Item label="Source Location" required style={itemStyle}>
          <Input
            value={cfg.sourceLocation}
            onChange={(e) => set("sourceLocation", e.target.value)}
            placeholder="@stage/path or git URL"
          />
        </Form.Item>

        <Form.Item label="dbt Version" style={itemStyle}>
          <Select
            value={cfg.dbtVersion || undefined}
            onChange={(v) => set("dbtVersion", v ?? "")}
            placeholder="Select version (optional)"
            loading={loadingVersions}
            allowClear
            showSearch
            optionFilterProp="label"
            options={dbtVersions.map((v) => ({
              value: v.dbt_version,
              label: `${v.dbt_version} (${v.type})`,
            }))}
          />
        </Form.Item>

        <Form.Item label="Default Target" style={itemStyle}>
          <Input
            value={cfg.defaultTarget}
            onChange={(e) => set("defaultTarget", e.target.value)}
            placeholder="e.g. prod (optional)"
          />
        </Form.Item>

        <Form.Item label="External Access Integrations" style={itemStyle}>
          <Select
            mode="multiple"
            value={cfg.externalAccessIntegrations}
            onChange={(v) => set("externalAccessIntegrations", v)}
            placeholder="Select integrations (optional)"
            loading={loadingEai}
            options={eaiList.map((i) => ({ value: i.name, label: i.name }))}
            showSearch
            optionFilterProp="label"
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        {/* SQL Preview */}
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
            {preview || "-- Fill in required fields"}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
