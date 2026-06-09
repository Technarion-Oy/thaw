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
  Modal, Form, Input, Select, Checkbox, Space,
  Typography, Button, Alert, Tooltip,
} from "antd";
import { BranchesOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import CreateSecretModal from "../secret/CreateSecretModal";
import {
  BuildCreateGitRepositorySql,
  ExecDDL,
  GetQuotedIdentifiersIgnoreCase,
  ListApiIntegrations,
  ListSecretsInAccount,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface TagPair {
  name: string;
  value: string;
}

interface GitRepoConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  originUrl: string;
  apiIntegration: string;
  gitCredentials: string;
  comment: string;
  tags: TagPair[];
}

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateGitRepositoryModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<GitRepoConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    originUrl: "",
    apiIntegration: "",
    gitCredentials: "",
    comment: "",
    tags: [],
  });
  const [tags, setTags] = useState<TagPair[]>([]);
  const [apiIntegrations, setApiIntegrations] = useState<snowflake.ApiIntegration[]>([]);
  const [accountSecrets, setAccountSecrets] = useState<snowflake.AccountSecret[]>([]);
  const [loadingInts, setLoadingInts] = useState(false);
  const [loadingSecrets, setLoadingSecrets] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");
  const [showCreateSecret, setShowCreateSecret] = useState(false);

  useEffect(() => {
    setLoadingInts(true);
    ListApiIntegrations()
      .then((ints) => setApiIntegrations(ints ?? []))
      .catch(() => {})
      .finally(() => setLoadingInts(false));

    setLoadingSecrets(true);
    ListSecretsInAccount()
      .then((secrets) => setAccountSecrets(secrets ?? []))
      .catch(() => {})
      .finally(() => setLoadingSecrets(false));

    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

  // Keep tags in cfg in sync
  useEffect(() => {
    setCfg((prev) => ({ ...prev, tags: tags.map((t) => ({ name: t.name, value: t.value })) }));
  }, [tags]);

  useEffect(() => {
    BuildCreateGitRepositorySql(db, schema, cfg as any).then(setPreview).catch(() => setPreview(""));
  }, [db, schema, cfg]);

  const set = <K extends keyof GitRepoConfig>(key: K, value: GitRepoConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const reloadSecrets = () => {
    setLoadingSecrets(true);
    ListSecretsInAccount()
      .then((secrets) => setAccountSecrets(secrets ?? []))
      .catch(() => {})
      .finally(() => setLoadingSecrets(false));
  };

  const handleSecretCreated = (fqn: string) => {
    reloadSecrets();
    set("gitCredentials", fqn);
    setShowCreateSecret(false);
  };

  const canSubmit = cfg.name.trim() !== "" && cfg.originUrl.trim() !== "" && cfg.apiIntegration !== "";

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

  // Build FQN label for secret dropdown
  const secretOptions = [
    { value: "", label: "— None —" },
    ...accountSecrets.map((s) => {
      const fqn = `"${s.databaseName}"."${s.schemaName}"."${s.name}"`;
      return { value: fqn, label: fqn };
    }),
  ];

  return (
    <>
    <Modal
      open
      title={
        <Space size={6}>
          <BranchesOutlined style={{ color: "var(--link)" }} />
          <span>Create Git Repository</span>
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
            icon={<BranchesOutlined />}
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
          <Form.Item label="Repository name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_REPO"
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

        <Form.Item label="Origin URL" required style={itemStyle}>
          <Input
            value={cfg.originUrl}
            onChange={(e) => set("originUrl", e.target.value)}
            placeholder="https://github.com/org/repo.git"
          />
        </Form.Item>

        <Form.Item label="API Integration" required style={itemStyle}>
          <Select
            value={cfg.apiIntegration || undefined}
            onChange={(v) => set("apiIntegration", v ?? "")}
            placeholder="Select API integration"
            loading={loadingInts}
            options={apiIntegrations.map((i) => ({ value: i.name, label: i.name }))}
            showSearch
            optionFilterProp="label"
          />
        </Form.Item>

        <Form.Item label="Git Credentials (optional)" style={itemStyle} help="Select a SECRET containing Git credentials">
          <Space.Compact style={{ width: "100%" }}>
            <Select
              style={{ flex: 1 }}
              value={cfg.gitCredentials || ""}
              onChange={(v) => set("gitCredentials", v ?? "")}
              loading={loadingSecrets}
              options={secretOptions}
              showSearch
              optionFilterProp="label"
            />
            <Tooltip title="Create new secret">
              <Button icon={<PlusOutlined />} onClick={() => setShowCreateSecret(true)} />
            </Tooltip>
          </Space.Compact>
        </Form.Item>

        {/* Tags */}
        <Form.Item label="Tags" style={{ marginBottom: 6 }}>
          {tags.map((tag, i) => (
            <div key={i} style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 6 }}>
              <Input
                value={tag.name}
                onChange={(e) => setTags(tags.map((t, j) => j === i ? { ...t, name: e.target.value } : t))}
                placeholder="tag name"
                style={{ flex: 1 }}
              />
              <span style={{ color: "var(--text-muted)" }}>=</span>
              <Input
                value={tag.value}
                onChange={(e) => setTags(tags.map((t, j) => j === i ? { ...t, value: e.target.value } : t))}
                placeholder="tag value"
                style={{ flex: 1 }}
              />
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => setTags(tags.filter((_, j) => j !== i))}
                danger
              />
            </div>
          ))}
          <Button
            icon={<PlusOutlined />}
            size="small"
            onClick={() => setTags([...tags, { name: "", value: "" }])}
            style={{ marginTop: 4 }}
          >
            Add tag
          </Button>
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
    {showCreateSecret && (
      <CreateSecretModal
        db={db}
        schema={schema}
        onClose={() => setShowCreateSecret(false)}
        onSuccess={handleSecretCreated}
      />
    )}
    </>
  );
}
