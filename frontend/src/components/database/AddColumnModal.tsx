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
  Modal, Form, Input, Checkbox, Space, Radio, Select, InputNumber, Divider,
  Typography, Button, Alert,
} from "antd";
import { PlusOutlined, TableOutlined } from "@ant-design/icons";
import { ExecDDL, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl, { identToken, quoteIdent } from "../shared/ObjectNameCaseControl";
import DataTypeSelect from "../shared/DataTypeSelect";

const { Text } = Typography;
const { TextArea } = Input;

type ValueMode = "none" | "default" | "autoincrement" | "computed";
type IdentityOrder = "ORDER" | "NOORDER" | "";
type ConstraintKind = "none" | "unique" | "primary_key" | "foreign_key";

interface ColumnConfig {
  name: string;
  caseSensitive: boolean;
  ifNotExists: boolean;
  dataType: string;
  // Value mode
  valueMode: ValueMode;
  defaultValue: string;
  computedExpr: string;
  // Autoincrement / Identity
  identityStart: number;
  identityStep: number;
  identityOrder: IdentityOrder;
  // Inline constraint
  notNull: boolean;
  constraintKind: ConstraintKind;
  constraintName: string;
  fkTable: string;
  fkColumn: string;
  // Collation & comment
  collation: string;
  comment: string;
}

const DEFAULTS: ColumnConfig = {
  name: "",
  caseSensitive: false,
  ifNotExists: false,
  dataType: "VARCHAR",
  valueMode: "none",
  defaultValue: "",
  computedExpr: "",
  identityStart: 1,
  identityStep: 1,
  identityOrder: "",
  notNull: false,
  constraintKind: "none",
  constraintName: "",
  fkTable: "",
  fkColumn: "",
  collation: "",
  comment: "",
};

function buildSql(db: string, schema: string, table: string, cfg: ColumnConfig): string {
  const q = (s: string) => quoteIdent(s);
  const sq = (s: string) => "'" + s.replace(/'/g, "''") + "'";

  const colToken = identToken(cfg.name || "column_name", cfg.caseSensitive);
  const parts: string[] = [`ALTER TABLE ${q(db)}.${q(schema)}.${q(table)} ADD COLUMN`];

  if (cfg.ifNotExists) parts.push("IF NOT EXISTS");
  parts.push(colToken);

  // Data type (omitted for computed columns that derive their type from the expression)
  if (cfg.valueMode !== "computed") {
    parts.push(cfg.dataType);
  }

  // Computed expression: AS ( <expr> )
  if (cfg.valueMode === "computed" && cfg.computedExpr.trim()) {
    parts.push(`AS (${cfg.computedExpr.trim()})`);
  }

  // Default / Autoincrement
  if (cfg.valueMode === "default" && cfg.defaultValue.trim()) {
    parts.push(`DEFAULT ${cfg.defaultValue.trim()}`);
  } else if (cfg.valueMode === "autoincrement") {
    parts.push(`AUTOINCREMENT (${cfg.identityStart}, ${cfg.identityStep})`);
    if (cfg.identityOrder) parts.push(cfg.identityOrder);
  }

  // Inline constraint
  if (cfg.constraintName.trim()) {
    parts.push(`CONSTRAINT ${q(cfg.constraintName.trim())}`);
  }
  if (cfg.notNull) parts.push("NOT NULL");
  if (cfg.constraintKind === "unique") parts.push("UNIQUE");
  if (cfg.constraintKind === "primary_key") parts.push("PRIMARY KEY");
  if (cfg.constraintKind === "foreign_key") {
    let ref = "REFERENCES";
    if (cfg.fkTable.trim()) {
      ref += ` ${cfg.fkTable.trim()}`;
      if (cfg.fkColumn.trim()) ref += ` (${cfg.fkColumn.trim()})`;
    }
    parts.push(ref);
  }

  // Collation
  if (cfg.collation.trim()) {
    parts.push(`COLLATE '${cfg.collation.trim()}'`);
  }

  // Comment
  if (cfg.comment.trim()) {
    parts.push(`COMMENT ${sq(cfg.comment.trim())}`);
  }

  return parts.join(" ") + ";";
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

  const canSubmit = cfg.name.trim() !== "" && (cfg.valueMode === "computed" ? cfg.computedExpr.trim() !== "" : cfg.dataType.trim() !== "");

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

  const isNumericType = /^(NUMBER|DECIMAL|NUMERIC|INT|INTEGER|BIGINT|SMALLINT|TINYINT|BYTEINT|FLOAT|DOUBLE|REAL)/i.test(cfg.dataType);

  const divider = (label: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "12px 0 6px" }}>
      {label}
    </Divider>
  );

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
      width={620}
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
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "start" }}>
          <Form.Item label="Column name" required style={{ marginBottom: 8 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_COLUMN"
              autoFocus
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 8, paddingTop: 24 }}>
            <Checkbox
              checked={cfg.ifNotExists}
              onChange={(e) => set("ifNotExists", e.target.checked)}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        {cfg.valueMode !== "computed" && (
          <Form.Item label="Data type" required style={{ marginBottom: 12 }}>
            <DataTypeSelect value={cfg.dataType} onChange={(v) => set("dataType", v)} />
          </Form.Item>
        )}

        {divider("Value")}

        <Form.Item style={{ marginBottom: 8 }}>
          <Radio.Group
            value={cfg.valueMode}
            onChange={(e) => set("valueMode", e.target.value)}
            size="small"
          >
            <Radio value="none">None</Radio>
            <Radio value="default">Default</Radio>
            <Radio value="autoincrement">Autoincrement / Identity</Radio>
            <Radio value="computed">Computed (AS)</Radio>
          </Radio.Group>
        </Form.Item>

        {cfg.valueMode === "default" && (
          <Form.Item label="Default value" style={{ marginBottom: 12 }}>
            <Input
              value={cfg.defaultValue}
              onChange={(e) => set("defaultValue", e.target.value)}
              placeholder="e.g. 0, '', CURRENT_TIMESTAMP()"
            />
          </Form.Item>
        )}

        {cfg.valueMode === "autoincrement" && (
          <>
            {!isNumericType && (
              <Alert
                type="warning"
                message="AUTOINCREMENT is only supported for numeric data types"
                style={{ marginBottom: 8 }}
                showIcon
              />
            )}
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
              <Form.Item label="Start" style={{ marginBottom: 8 }}>
                <InputNumber
                  value={cfg.identityStart}
                  onChange={(v) => set("identityStart", v ?? 1)}
                  style={{ width: "100%" }}
                />
              </Form.Item>
              <Form.Item label="Increment" style={{ marginBottom: 8 }}>
                <InputNumber
                  value={cfg.identityStep}
                  onChange={(v) => set("identityStep", v ?? 1)}
                  style={{ width: "100%" }}
                />
              </Form.Item>
            </div>
            <Form.Item label="Ordering" style={{ marginBottom: 12 }}>
              <Radio.Group
                value={cfg.identityOrder}
                onChange={(e) => set("identityOrder", e.target.value)}
                size="small"
              >
                <Radio value="">Default</Radio>
                <Radio value="ORDER">ORDER</Radio>
                <Radio value="NOORDER">NOORDER</Radio>
              </Radio.Group>
            </Form.Item>
          </>
        )}

        {cfg.valueMode === "computed" && (
          <Form.Item label="Expression" required style={{ marginBottom: 12 }}
            help="The column value is computed from this expression. Data type is derived automatically."
          >
            <Input
              value={cfg.computedExpr}
              onChange={(e) => set("computedExpr", e.target.value)}
              placeholder='e.g. col_a + col_b, UPPER("NAME")'
            />
          </Form.Item>
        )}

        {divider("Constraints")}

        <Form.Item style={{ marginBottom: 8 }}>
          <Checkbox checked={cfg.notNull} onChange={(e) => set("notNull", e.target.checked)}>
            NOT NULL
          </Checkbox>
        </Form.Item>

        <Form.Item label="Inline constraint" style={{ marginBottom: 8 }}>
          <Radio.Group
            value={cfg.constraintKind}
            onChange={(e) => set("constraintKind", e.target.value)}
            size="small"
          >
            <Radio value="none">None</Radio>
            <Radio value="unique">UNIQUE</Radio>
            <Radio value="primary_key">PRIMARY KEY</Radio>
            <Radio value="foreign_key">FOREIGN KEY</Radio>
          </Radio.Group>
        </Form.Item>

        {cfg.constraintKind !== "none" && (
          <Form.Item label="Constraint name (optional)" style={{ marginBottom: 8 }}>
            <Input
              value={cfg.constraintName}
              onChange={(e) => set("constraintName", e.target.value)}
              placeholder="Leave empty for auto-generated name"
            />
          </Form.Item>
        )}

        {cfg.constraintKind === "foreign_key" && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="References table" required style={{ marginBottom: 8 }}>
              <Input
                value={cfg.fkTable}
                onChange={(e) => set("fkTable", e.target.value)}
                placeholder='"DB"."SCHEMA"."TABLE"'
              />
            </Form.Item>
            <Form.Item label="References column" style={{ marginBottom: 8 }}>
              <Input
                value={cfg.fkColumn}
                onChange={(e) => set("fkColumn", e.target.value)}
                placeholder="column_name"
              />
            </Form.Item>
          </div>
        )}

        {divider("Options")}

        <Form.Item label="Collation" style={{ marginBottom: 8 }}>
          <Select
            showSearch
            allowClear
            value={cfg.collation || undefined}
            onChange={(v) => set("collation", v ?? "")}
            placeholder="Default (no collation)"
            style={{ width: "100%" }}
            options={[
              { value: "utf8", label: "utf8" },
              { value: "en", label: "en" },
              { value: "en-ci", label: "en-ci (case-insensitive)" },
              { value: "en-ci-ai", label: "en-ci-ai (case & accent insensitive)" },
              { value: "en-cs", label: "en-cs (case-sensitive)" },
              { value: "en-cs-ai", label: "en-cs-ai (case-sensitive, accent insensitive)" },
              { value: "fr", label: "fr" },
              { value: "fr-ci", label: "fr-ci" },
              { value: "de", label: "de" },
              { value: "de-ci", label: "de-ci" },
              { value: "es", label: "es" },
              { value: "ja", label: "ja" },
              { value: "ko", label: "ko" },
              { value: "zh", label: "zh" },
            ]}
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
