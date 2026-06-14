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
  Form, Input, Select, Space, Button, Tooltip,
} from "antd";
import { BranchesOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import CreateSecretModal from "../secret/CreateSecretModal";
import {
  BuildCreateGitRepositorySql,
  ExecDDL,
  ListApiIntegrations,
  ListSecretsInAccount,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { snowflake } from "../../../wailsjs/go/models";

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
  const [showCreateSecret, setShowCreateSecret] = useState(false);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateGitRepositorySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

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
  }, []);

  // Keep tags in cfg in sync
  useEffect(() => {
    setCfg((prev) => ({ ...prev, tags: tags.map((t) => ({ name: t.name, value: t.value })) }));
  }, [tags]);

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

  const handleRun = () => {
    if (!canSubmit || !preview) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
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
    <CreateModalShell
      icon={<BranchesOutlined />}
      title="Create Git Repository"
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
        {/* Name row */}
        <NameWithReplaceOptions
          label="Repository name"
          placeholder="MY_REPO"
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
        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
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
