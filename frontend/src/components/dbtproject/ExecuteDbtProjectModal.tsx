// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useRef } from "react";
import {
  Modal, Form, Input, Select, Space, Button,
} from "antd";
import { PlayCircleOutlined } from "@ant-design/icons";
import { BuildExecuteDbtProjectSql, ListSupportedDbtVersions } from "../../../wailsjs/go/app/App";
import { dbtproject } from "../../../wailsjs/go/models";
import { useQueryStore } from "../../store/queryStore";
import SqlPreview from "../shared/SqlPreview";

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function ExecuteDbtProjectModal({ db, schema, name, onClose }: Props) {
  const [mode, setMode] = useState<"direct" | "workspace">("direct");
  const [cfg, setCfg] = useState<dbtproject.ExecuteConfig>(new dbtproject.ExecuteConfig({
    args: "",
    dbtVersion: "",
    fromWorkspace: "",
    projectRoot: "",
  }));
  const [dbtVersions, setDbtVersions] = useState<dbtproject.DbtVersionInfo[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [preview, setPreview] = useState("");
  const previewTimer = useRef<ReturnType<typeof setTimeout>>();
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    setLoadingVersions(true);
    ListSupportedDbtVersions()
      .then((v) => setDbtVersions(v ?? []))
      .catch((err) => console.warn("ListSupportedDbtVersions failed:", err))
      .finally(() => setLoadingVersions(false));
  }, []);

  useEffect(() => {
    let stale = false;
    clearTimeout(previewTimer.current);
    previewTimer.current = setTimeout(() => {
      const execCfg = mode === "direct"
        ? { ...cfg, fromWorkspace: "", projectRoot: "" }
        : cfg;
      BuildExecuteDbtProjectSql(db, schema, name, execCfg)
        .then((sql) => { if (!stale) setPreview(sql); })
        .catch(() => { if (!stale) setPreview(""); });
    }, 200);
    return () => { stale = true; clearTimeout(previewTimer.current); };
  }, [db, schema, name, cfg, mode]);

  // Spread loses the Wails class prototype, but this is fine — Wails uses JSON
  // serialization for IPC so only the field values matter, not the prototype.
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
            onChange={(v) => {
              setMode(v);
              // Clear workspace fields when switching to direct; the preview effect
              // also masks them, but clearing state avoids stale values reappearing
              // if the user switches back to workspace mode.
              if (v === "direct") setCfg((prev) => ({ ...prev, fromWorkspace: "", projectRoot: "" }));
            }}
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

        <SqlPreview sql={preview} placeholder="-- Configure execution options" />
      </Form>
    </Modal>
  );
}
