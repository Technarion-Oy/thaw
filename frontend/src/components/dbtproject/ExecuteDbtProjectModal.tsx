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
  Modal, Form, Input, Select, Space,
  Typography, Button,
} from "antd";
import { PlayCircleOutlined } from "@ant-design/icons";
import { BuildExecuteDbtProjectSql, ListSupportedDbtVersions } from "../../../wailsjs/go/main/App";
import { dbtproject } from "../../../wailsjs/go/models";
import type { main } from "../../../wailsjs/go/models";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function ExecuteDbtProjectModal({ db, schema, name, onClose }: Props) {
  const [mode, setMode] = useState<"direct" | "workspace">("direct");
  const [cfg, setCfg] = useState<dbtproject.ExecuteConfig>({
    args: "",
    dbtVersion: "",
    fromWorkspace: "",
    projectRoot: "",
  });
  const [dbtVersions, setDbtVersions] = useState<main.DbtVersionInfo[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [preview, setPreview] = useState("");
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    setLoadingVersions(true);
    ListSupportedDbtVersions()
      .then((v) => setDbtVersions(v ?? []))
      .catch(() => {})
      .finally(() => setLoadingVersions(false));
  }, []);

  useEffect(() => {
    const execCfg = mode === "direct"
      ? { ...cfg, fromWorkspace: "", projectRoot: "" }
      : cfg;
    BuildExecuteDbtProjectSql(db, schema, name, execCfg)
      .then(setPreview)
      .catch(() => setPreview(""));
  }, [db, schema, name, cfg, mode]);

  const set = <K extends keyof dbtproject.ExecuteConfig>(key: K, value: dbtproject.ExecuteConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const handleRun = () => {
    if (!preview) return;
    executeInNewTab(preview);
    onClose();
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <PlayCircleOutlined style={{ color: "var(--link)" }} />
          <span>Execute DBT Project: {name}</span>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            type="primary"
            icon={<PlayCircleOutlined />}
            onClick={handleRun}
            disabled={!preview}
          >
            Execute
          </Button>
        </Space>
      }
      width={620}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      <Form layout="vertical" size="small">
        <Form.Item label="Execution Mode" style={itemStyle}>
          <Select
            value={mode}
            onChange={(v) => setMode(v)}
            options={[
              { value: "direct", label: "Direct" },
              { value: "workspace", label: "From Workspace" },
            ]}
          />
        </Form.Item>

        <Form.Item label="Args" style={itemStyle} help="dbt CLI arguments (e.g. run --models my_model)">
          <Input.TextArea
            value={cfg.args}
            onChange={(e) => set("args", e.target.value)}
            placeholder="run --models my_model"
            rows={2}
          />
        </Form.Item>

        <Form.Item label="dbt Version" style={itemStyle}>
          <Select
            value={cfg.dbtVersion || undefined}
            onChange={(v) => set("dbtVersion", v ?? "")}
            placeholder="Select version (optional override)"
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

        {mode === "workspace" && (
          <>
            <Form.Item label="Workspace Name" style={itemStyle}>
              <Input
                value={cfg.fromWorkspace}
                onChange={(e) => set("fromWorkspace", e.target.value)}
                placeholder="MY_WORKSPACE"
              />
            </Form.Item>

            <Form.Item label="Project Root" style={itemStyle}>
              <Input
                value={cfg.projectRoot}
                onChange={(e) => set("projectRoot", e.target.value)}
                placeholder="/project (optional)"
              />
            </Form.Item>
          </>
        )}

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
            {preview || "-- Configure execution options"}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
