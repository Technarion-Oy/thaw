// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Select, Space,
  Typography, Button, Alert,
} from "antd";
import { BranchesOutlined } from "@ant-design/icons";
import {
  GetObjectProperties,
  ListApiIntegrations,
  ListSecretsInAccount,
  BuildModifyGitRepositorySql,
  ExecDDL,
} from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface GitRepoConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  originUrl: string;
  apiIntegration: string;
  gitCredentials: string;
  comment: string;
  tags: Array<{ name: string; value: string }>;
}

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function ModifyGitRepositoryModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<GitRepoConfig | null>(null);
  const [originUrl, setOriginUrl] = useState("");
  const [originalComment, setOriginalComment] = useState("");
  const [originalIntegration, setOriginalIntegration] = useState("");
  const [originalCredentials, setOriginalCredentials] = useState("");
  const [apiIntegrations, setApiIntegrations] = useState<snowflake.ApiIntegration[]>([]);
  const [accountSecrets, setAccountSecrets] = useState<snowflake.AccountSecret[]>([]);
  const [loading, setLoading] = useState(true);
  const [modifying, setModifying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    const init = async () => {
      try {
        const [props, ints, secrets] = await Promise.all([
          GetObjectProperties(db, schema, "GIT REPOSITORY", name),
          ListApiIntegrations(),
          ListSecretsInAccount(),
        ]);
        setApiIntegrations(ints ?? []);
        setAccountSecrets(secrets ?? []);

        const pMap = new Map((props ?? []).map((p) => [p.key.toUpperCase(), p.value]));
        const origin = pMap.get("ORIGIN") || pMap.get("ORIGIN_URL") || "";
        const integration = pMap.get("API_INTEGRATION") || pMap.get("API_INTEGRATION_NAME") || "";
        const creds = pMap.get("GIT_CREDENTIALS") || "";
        const comment = pMap.get("COMMENT") || "";

        setOriginUrl(origin);
        setOriginalIntegration(integration);
        setOriginalCredentials(creds);
        setOriginalComment(comment);

        setCfg({
          name,
          caseSensitive: true,
          orReplace: false,
          ifNotExists: false,
          originUrl: origin,
          apiIntegration: integration,
          gitCredentials: creds,
          comment,
          tags: [],
        });
      } catch (err) {
        setError(String(err));
      } finally {
        setLoading(false);
      }
    };
    init();
  }, [db, schema, name]);

  useEffect(() => {
    if (cfg) {
      BuildModifyGitRepositorySql(db, schema, name, cfg as any, originalComment, originalIntegration, originalCredentials)
        .then((sqls) => setPreview((sqls ?? []).join("\n\n")))
        .catch(() => setPreview(""));
    }
  }, [db, schema, name, cfg, originalComment, originalIntegration, originalCredentials]);

  const set = <K extends keyof GitRepoConfig>(key: K, value: GitRepoConfig[K]) =>
    setCfg((prev) => prev ? ({ ...prev, [key]: value }) : null);

  const handleRun = async () => {
    if (!cfg) return;
    const sqls = preview.split("\n\n").filter((s) => s.trim() !== "");
    if (sqls.length === 0) {
      onClose();
      return;
    }
    setModifying(true);
    setError(null);
    try {
      for (const sql of sqls) {
        await ExecDDL(sql);
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
      <Modal open title="Modify Git Repository" onCancel={onClose} footer={null}>
        <div style={{ padding: 20, textAlign: "center" }}>Loading properties…</div>
      </Modal>
    );
  }

  if (!cfg) return null;

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // Build FQN label for secret dropdown
  const secretOptions = [
    { value: "", label: "— None / Clear —" },
    ...accountSecrets.map((s) => {
      const fqn = `"${s.databaseName}"."${s.schemaName}"."${s.name}"`;
      return { value: fqn, label: fqn };
    }),
  ];

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <BranchesOutlined style={{ color: "var(--link)" }} />
          <span>Modify Git Repository: {name}</span>
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
        {/* Read-only info */}
        <Form.Item label="Origin URL" style={itemStyle} help="Origin URL cannot be changed after creation">
          <Input value={originUrl} disabled />
        </Form.Item>

        {/* Editable fields */}
        <Form.Item label="API Integration" required style={itemStyle} help="API_INTEGRATION cannot be unset; select a new value to change it">
          <Select
            value={cfg.apiIntegration || undefined}
            onChange={(v) => set("apiIntegration", v ?? "")}
            placeholder="Select API integration"
            options={apiIntegrations.map((i) => ({ value: i.name, label: i.name }))}
            showSearch
            optionFilterProp="label"
          />
        </Form.Item>

        <Form.Item label="Git Credentials" style={itemStyle} help="Select '— None / Clear —' to remove credentials">
          <Select
            value={cfg.gitCredentials ?? ""}
            onChange={(v) => set("gitCredentials", v ?? "")}
            options={secretOptions}
            showSearch
            optionFilterProp="label"
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
            allowClear
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
            {preview || "-- No changes"}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
