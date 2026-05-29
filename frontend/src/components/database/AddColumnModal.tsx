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
  Typography, Button, Alert,
} from "antd";
import { PlusOutlined, TableOutlined } from "@ant-design/icons";
import { ExecDDL, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl, { identToken, quoteIdent } from "../shared/ObjectNameCaseControl";

const { Text } = Typography;
const { TextArea } = Input;

interface ColumnConfig {
  name: string;
  caseSensitive: boolean;
  dataType: string;
  notNull: boolean;
  defaultValue: string;
  comment: string;
}

const DEFAULTS: ColumnConfig = {
  name: "",
  caseSensitive: false,
  dataType: "VARCHAR",
  notNull: false,
  defaultValue: "",
  comment: "",
};

function buildSql(db: string, schema: string, table: string, cfg: ColumnConfig): string {
  const q = (s: string) => quoteIdent(s);
  const sq = (s: string) => "'" + s.replace(/'/g, "''") + "'";

  const colToken = identToken(cfg.name || "column_name", cfg.caseSensitive);
  let sql = `ALTER TABLE ${q(db)}.${q(schema)}.${q(table)} ADD COLUMN ${colToken} ${cfg.dataType}`;

  if (cfg.notNull) sql += " NOT NULL";
  if (cfg.defaultValue.trim()) sql += ` DEFAULT ${cfg.defaultValue.trim()}`;
  if (cfg.comment.trim()) sql += ` COMMENT ${sq(cfg.comment.trim())}`;

  return sql + ";";
}

interface Props {
  db: string;
  schema: string;
  table: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function AddColumnModal({ db, schema, table, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ColumnConfig>(DEFAULTS);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

  const set = <K extends keyof ColumnConfig>(key: K, value: ColumnConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim() !== "" && cfg.dataType.trim() !== "";

  const handleCreate = async () => {
    if (!canSubmit) return;
    const sql = buildSql(db, schema, table, cfg);
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const preview = buildSql(db, schema, table, cfg);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <TableOutlined style={{ color: "var(--link)" }} />
          <span>Add column</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{table}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
            disabled={!canSubmit}
            loading={creating}
          >
            Add Column
          </Button>
        </Space>
      }
      width={560}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Column creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <Form.Item label="Column name" required style={{ marginBottom: 8 }}>
          <Input
            value={cfg.name}
            onChange={(e) => set("name", e.target.value)}
            placeholder="MY_COLUMN"
            autoFocus
          />
        </Form.Item>
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Data type" required style={{ marginBottom: 12 }}>
          <Select
            showSearch
            value={cfg.dataType}
            onChange={(v) => set("dataType", v)}
            options={[
              { value: "NUMBER(38,0)", label: "NUMBER(38,0)" },
              { value: "VARCHAR", label: "VARCHAR" },
              { value: "VARCHAR(16777216)", label: "VARCHAR(16777216)" },
              { value: "BOOLEAN", label: "BOOLEAN" },
              { value: "DATE", label: "DATE" },
              { value: "TIMESTAMP_NTZ", label: "TIMESTAMP_NTZ" },
              { value: "TIMESTAMP_TZ", label: "TIMESTAMP_TZ" },
              { value: "VARIANT", label: "VARIANT" },
              { value: "OBJECT", label: "OBJECT" },
              { value: "ARRAY", label: "ARRAY" },
              { value: "FLOAT", label: "FLOAT" },
            ]}
          />
        </Form.Item>

        <Form.Item style={{ marginBottom: 12 }}>
          <Checkbox checked={cfg.notNull} onChange={(e) => set("notNull", e.target.checked)}>
            NOT NULL
          </Checkbox>
        </Form.Item>

        <Form.Item label="Default value" style={{ marginBottom: 12 }}>
          <Input
            value={cfg.defaultValue}
            onChange={(e) => set("defaultValue", e.target.value)}
            placeholder="NULL"
          />
        </Form.Item>

        <Form.Item label="Comment" style={{ marginBottom: 12 }}>
          <TextArea
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="Column comment"
            rows={2}
          />
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
