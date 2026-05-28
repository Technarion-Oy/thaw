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
  Modal, Form, Input, Select, Space, Button, Alert, Spin,
} from "antd";
import { EditOutlined } from "@ant-design/icons";
import {
  DescribeDbtProject,
  ListExternalAccessIntegrations,
  ListSupportedDbtVersions,
  BuildAlterDbtProjectSetSql,
  ExecDDL,
} from "../../../wailsjs/go/main/App";
import { dbtproject } from "../../../wailsjs/go/models";
import type { snowflake } from "../../../wailsjs/go/models";
import SqlPreview from "../shared/SqlPreview";

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function ModifyDbtProjectModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [dbtVersion, setDbtVersion] = useState("");
  const [defaultTarget, setDefaultTarget] = useState("");
  const [integrations, setIntegrations] = useState<string[]>([]);
  const [comment, setComment] = useState("");

  const [origDbtVersion, setOrigDbtVersion] = useState("");
  const [origDefaultTarget, setOrigDefaultTarget] = useState("");
  const [origIntegrations, setOrigIntegrations] = useState<string[]>([]);
  const [origComment, setOrigComment] = useState("");

  const [eaiList, setEaiList] = useState<snowflake.IntegrationRow[]>([]);
  const [dbtVersions, setDbtVersions] = useState<dbtproject.DbtVersionInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [modifying, setModifying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statements, setStatements] = useState<string[]>([]);
  const previewTimer = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    const init = async () => {
      try {
        const [props, eais, versions] = await Promise.all([
          DescribeDbtProject(db, schema, name),
          ListExternalAccessIntegrations(),
          ListSupportedDbtVersions(),
        ]);
        setEaiList(eais ?? []);
        setDbtVersions(versions ?? []);

        const pMap = new Map((props ?? []).map((p) => [p.key.toUpperCase(), p.value]));
        const ver = pMap.get("DBT_VERSION") || "";
        const tgt = pMap.get("DEFAULT_TARGET") || "";
        const cmt = pMap.get("COMMENT") || "";
        const eaiRaw = pMap.get("EXTERNAL_ACCESS_INTEGRATIONS") || "";
        // Handle possible formats: comma-separated, bracket-wrapped [A,B], or JSON array ["A","B"].
        // Splitting on comma is safe because Snowflake identifiers cannot contain commas without quoting,
        // and DESCRIBE returns unquoted integration names.
        const eaiClean = eaiRaw.replace(/^\[|\]$/g, "");
        const eaiArr = eaiClean ? eaiClean.split(",").map((s) => s.trim().replace(/^["']|["']$/g, "")).filter(Boolean) : [];

        setDbtVersion(ver);
        setDefaultTarget(tgt);
        setComment(cmt);
        setIntegrations(eaiArr);

        setOrigDbtVersion(ver);
        setOrigDefaultTarget(tgt);
        setOrigComment(cmt);
        setOrigIntegrations(eaiArr);
      } catch (err) {
        setError(String(err));
      } finally {
        setLoading(false);
      }
    };
    init();
  }, [db, schema, name]);

  useEffect(() => {
    if (loading) return;
    clearTimeout(previewTimer.current);
    previewTimer.current = setTimeout(() => {
      const cfg = new dbtproject.AlterSetConfig({
        dbtVersion,
        defaultTarget,
        externalAccessIntegrations: integrations,
        comment,
      });
      BuildAlterDbtProjectSetSql(db, schema, name, cfg, origComment, origDbtVersion, origDefaultTarget, origIntegrations)
        .then((sqls) => setStatements(sqls ?? []))
        .catch(() => setStatements([]));
    }, 200);
    return () => clearTimeout(previewTimer.current);
  }, [db, schema, name, dbtVersion, defaultTarget, integrations, comment, origComment, origDbtVersion, origDefaultTarget, origIntegrations, loading]);

  const handleRun = async () => {
    if (statements.length === 0) {
      onClose();
      return;
    }
    setModifying(true);
    setError(null);
    try {
      for (let i = 0; i < statements.length; i++) {
        try {
          await ExecDDL(statements[i]);
        } catch (err) {
          const prefix = statements.length > 1
            ? `Statement ${i + 1}/${statements.length} failed (partial changes may have been applied): `
            : "";
          throw new Error(prefix + String(err));
        }
      }
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setModifying(false);
    }
  };

  if (loading) {
    return (
      <Modal open title="Modify DBT Project" onCancel={onClose} footer={null}>
        <div style={{ padding: 20, textAlign: "center" }}><Spin /></div>
      </Modal>
    );
  }

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <EditOutlined style={{ color: "var(--link)" }} />
          <span>Modify DBT Project: {name}</span>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={modifying}>Cancel</Button>
          <Button
            type="primary"
            onClick={handleRun}
            loading={modifying}
          >
            Apply Changes
          </Button>
        </Space>
      }
      width={620}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {error && (
        <Alert
          type="error"
          message="Modification failed"
          description={error}
          showIcon
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">
        <Form.Item label="dbt Version" style={itemStyle} help="Clear to unset">
          <Select
            value={dbtVersion || undefined}
            onChange={(v) => setDbtVersion(v ?? "")}
            placeholder="Select version"
            allowClear
            showSearch
            optionFilterProp="label"
            options={dbtVersions.map((v) => ({
              value: v.dbt_version,
              label: `${v.dbt_version} (${v.type})`,
            }))}
          />
        </Form.Item>

        <Form.Item label="Default Target" style={itemStyle} help="Clear to unset">
          <Input
            value={defaultTarget}
            onChange={(e) => setDefaultTarget(e.target.value)}
            placeholder="e.g. prod"
            allowClear
          />
        </Form.Item>

        <Form.Item label="External Access Integrations" style={itemStyle} help="Clear all to unset">
          <Select
            mode="multiple"
            value={integrations}
            onChange={setIntegrations}
            placeholder="Select integrations"
            options={eaiList.map((i) => ({ value: i.name, label: i.name }))}
            showSearch
            optionFilterProp="label"
            allowClear
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle} help="Clear to unset">
          <Input
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="optional comment"
            allowClear
          />
        </Form.Item>

        <SqlPreview sql={statements.join("\n\n")} placeholder="-- No changes" />
      </Form>
    </Modal>
  );
}
