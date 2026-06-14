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

import { useState, useEffect, useRef } from "react";
import {
  Form, Input, Select, Space,
} from "antd";
import { BuildOutlined } from "@ant-design/icons";
import {
  BuildCreateDbtProjectSql,
  ExecDDL,
  ListExternalAccessIntegrations,
  ListSupportedDbtVersions,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useCreateSubmit } from "../shared/createModalHooks";
import SourceLocationPicker from "./SourceLocationPicker";
import { dbtproject } from "../../../wailsjs/go/models";
import type { snowflake } from "../../../wailsjs/go/models";

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
  const [preview, setPreview] = useState("");
  const previewTimer = useRef<ReturnType<typeof setTimeout>>();

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const { creating, error, setError, submit } = useCreateSubmit();

  useEffect(() => {
    setLoadingEai(true);
    ListExternalAccessIntegrations()
      .then((rows) => setEaiList(rows ?? []))
      .catch((err) => console.warn("ListExternalAccessIntegrations failed:", err))
      .finally(() => setLoadingEai(false));

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
      BuildCreateDbtProjectSql(db, schema, cfg)
        .then((sql) => { if (!stale) setPreview(sql); })
        .catch(() => { if (!stale) setPreview(""); });
    }, 200);
    return () => { stale = true; clearTimeout(previewTimer.current); };
  }, [db, schema, cfg]);

  // Spread loses the Wails class prototype, but this is fine — Wails uses JSON
  // serialization for IPC so only the field values matter, not the prototype.
  const set = <K extends keyof dbtproject.CreateConfig>(key: K, value: dbtproject.CreateConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim() !== "" && cfg.sourceLocation.trim() !== "";

  const handleRun = () => submit(async () => {
    await ExecDDL(preview);
    onSuccess?.();
    onClose();
  });

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<BuildOutlined />}
      title="Create DBT Project"
      subtitle={`${db}.${schema}`}
      width={620}
      bodyMaxHeight="72vh"
      error={error}
      errorTitle="Creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit && !!preview}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Project name"
          placeholder="MY_DBT_PROJECT"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />
        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Source Location" required style={itemStyle}>
          <Space direction="vertical" style={{ width: "100%" }}>
            <Input
              value={cfg.sourceLocation}
              onChange={(e) => set("sourceLocation", e.target.value)}
              placeholder="@stage/path or use picker below"
            />
            <SourceLocationPicker
              db={db}
              schema={schema}
              value={cfg.sourceLocation}
              onChange={(v) => set("sourceLocation", v)}
            />
          </Space>
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

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
